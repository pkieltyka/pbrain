# Plan: sync throughput, GPU first, with batching and parallelism

Epic #36. Owner's direction of 2026-11-22: the GPU is the primary path for `pbrain sync`; the CPU
is a fallback that still works and says so; the GPU batch is sized at run time from free VRAM; the
CPU path uses every core; tokenizing, embedding and writing overlap; a machine can have several
GPUs, so pbrain picks one by itself and lets a person name one. The #32 decision is folded in: a
search never uses the GPU. This plan also absorbs what was left of #29 (the CUDA contract).

Read against master `18f50a6`, goinfer v0.5.0 with its CUDA module, and aikit v1.79.1. Measurements
are @lead-dev's of 2026-11-22 (`/var/tmp/pbrain-sync-throughput.yqF3OJ/findings.md`; they go into
`internal/cli/README.measure.md` with ticket 7): 128 items of the phase 28 brain, 1,223 passages,
CPUs 0 to 3, RTX PRO 6000; and for the CPU `/var/tmp/pbrain-bge-cores.xgwVO4/findings.md`, the same
slice on 4 cores, 16 cores and 32 threads of the Ryzen 9 7950X.

## What was measured, and what it means for this plan

| Run | Passages a second | Notes |
|---|---:|---|
| bge-small, sync, 4 cores | 10.50 | 354% of 400% CPU; 99.74% of CPU samples inside aikit's encoder |
| bge-small as built, 4 cores / 16 cores / 32 threads | 9.72 / 13.52 / 14.23 | one forward at a time; 336 / 377 / 354 MiB at the peak |
| bge-small, one forward a worker, 4 / 16 / 32 workers | 11.54 / 29.69 / 33.64 | 388 / 563 / 783 MiB; every stored row and vector equal to the build as built |
| EmbeddingGemma 2, CUDA, sync as built (pairs) | 11.61 | GPU busy 96% (median of 1,051 samples); device peak 1,340 MiB |
| the same passages, one a call | 10.11 | 5,386 tokens a second; 31 to 873 tokens a passage, 533 on average |
| two a call | 11.33 | 6,033 tokens a second |
| eight a call | 12.20 | 6,500 tokens a second; 2,350 to 5,234 positions a call; 7.7% over pairs |
| 32 and more a call | refused | 14,605 positions and up: over goinfer's 8,192 |
| passages cut to 64 tokens, one a call | 60.62 | 3,714 tokens a second |
| the same, eight a call | 321.69 | 19,711 tokens a second |
| the same, 128 a call | 615.42 | 37,709 tokens a second; 7,843 positions; 2,100 MiB |

1. **The GPU is not waiting for pbrain.** It is busy 96% of the time. On passages of real length a
   larger batch gains 7.7%. What limits the build is how fast goinfer's CUDA path runs long inputs:
   6,500 tokens a second with eight real passages a call, against 37,709 with 128 short ones in a
   call of about the same size. Which kernel is responsible is not known; one a call, the short
   inputs are the slower per token, so part of it is a cost per call. That is goinfer ask 8, which
   asks for a profile first. **Nothing in this plan promises a faster GPU sync on long passages.**
2. **Batching does pay on short texts**: ten times from one a call to 128 a call. The fixture's
   passages are long because its items are 10 KB each. A brain of ordinary notes has many passages
   of a few sentences, so batching by token count is worth building, and it is what a faster kernel
   will need.
3. **The 8,192 cap is on the batch's total** (`backend/cuda/embeddinggemma2.go:307`). Device memory
   grows by about 0.13 MiB a position (measured; the allocation list gives 128 KB), so the largest
   batch goinfer admits takes about 1.1 GB. A VRAM budget above that cannot be spent until goinfer
   lifts the cap (ask 4).
4. **A CUDA vector does not depend on its batch.** Every vector was bit for bit the same at batch
   sizes 1, 2 and 8, and at 1 to 128 on the short control, and across repeats. goinfer's own tests
   assert the same (`backend/cuda/embeddinggemma2_test.go:297`, `:554`).
5. **bge-small on the CPU is all model, and one forward does not scale.** Tokenizing, storing and
   publishing took 0.25 s of 413 s of CPU, so an overlap of stages buys bge-small nothing. pbrain
   has no limit of four cores: the "4" in every published figure is the measurement's `taskset`.
   But aikit's spread of one forward over the cores stops paying early: from 4 cores to 16 the
   build as it is gains 39%, and from 16 cores to 32 threads 5%, using 5.5 and 8.9 cores' worth of
   CPU. One forward a worker gains 19% on 4 cores, 2.12 times on 16 and 2.35 times on 32 threads,
   with the same bytes in every stored row. Each worker costs about 14 MiB at the peak.

## Decision

1. **A search always opens the model on the CPU.** `Registry.Open` gets an argument that says what
   the caller wants. This is #32's decision, and it gives a `cuda` build the CPU's 286 ms for check,
   load and one query where it has 1,050 ms today.
2. **`sync` picks a device**: the one a person named, else the best GPU this binary can use on this
   machine, else the CPU. A named device that cannot be used is an error. An automatic choice that
   cannot be used falls back, in the end to the CPU, with a `cpu_fallback` notice.
3. **Batching lives inside the adapter's `Embed`.** The build asks the embedder how many texts it
   wants at once and hands it that window. EmbeddingGemma 2 on an accelerator packs the window into
   batches by token count under a memory budget.
4. **The budget comes from free VRAM** once the weights are on the device: 30% of what is free, at
   most 6 GiB, with the batch halved on an allocation failure. A person can set the amount, under
   the same 6 GiB. The budget limits how many passages share a call; one passage alone always
   runs.
5. **bge-small embeds on a pool of workers, one forward each.** The gate this plan set, more than
   10% at 16 cores with the same bytes, is passed at 2.12 times. A query, and any single text,
   keeps today's one forward spread over the cores.
6. **Overlapping the stages is designed here and not scheduled.** By the measurements it is worth
   under 2% today and at most about 7% with short notes on a fast device. It is the riskiest change
   to the build, so it waits for a measurement that shows the device waiting.
7. **What a run stores does not depend on window or batch size**, and with one device not on
   anything else.

## Rejected

- **A worker pool in `semantic` that calls `Embed` from several goroutines.** A GPU runs one forward
  at a time behind a mutex; its gain is a larger batch. It would also make every embedder, test
  doubles included, safe for concurrent use.
- **A tokenizing stage in `semantic`.** Tokenizing is the adapter's, and it is 0.03% of the CPU time.
- **A padded batch for bge-small.** aikit's `BERT` has none, and its padded batch for other models
  changes the last bits of a vector with the batch's shape.
- **Sorting passages by length before batching.** goinfer lays a batch's inputs end to end with no
  padding, so there is nothing to save, and membership would depend on the window.
- **A key in `pbrain.toml` or `personal.toml` for the device.** The first is tracked and shared. The
  second is per checkout, and a property of the machine set once per project drifts. pbrain has no
  per-user file, and one key does not justify one. The environment variable is the per-machine
  setting; the flag is the per-run one.
- **Recording the device in the fingerprint or beside each vector** (below, "What the index records").
- **Falling back to the CPU on any accelerator error in the middle of a run.** A device that fails
  a forward for a reason other than memory may also be returning wrong vectors.
- **Growing the budget back after a back-off.** One probe and a one-way halving cannot oscillate.
- **Using two GPUs at once.** One device a run. Nothing measured asks for more.

## Interfaces

```go
package embed

// OpenOptions says what the caller wants the model to run on. The zero value is the CPU.
type OpenOptions struct {
    // Device is a selector: "" or "cpu"; "auto"; a backend ("cuda"); or one device ("cuda:0", or
    // the persistent id the listing prints). See "Devices".
    Device string
    // MemoryBudget, when positive, replaces the VRAM rule: bytes of device memory for one batch's
    // working set. Ignored on the CPU.
    MemoryBudget int64
}

// Fallback says why the CPU runs although "auto" was asked.
type Fallback string

const (
    NoFallback        Fallback = ""
    FallbackNoSupport Fallback = "unsupported" // this binary has no accelerator for this model
    FallbackNoDevice  Fallback = "absent"      // no driver or no usable device on this machine
    FallbackFailed    Fallback = "failed"      // a device is there and could not be attached
    FallbackMemory    Fallback = "memory"      // too little device memory, at open or after backing off
)

// RunInfo describes what an opened embedder runs on. It can change between two Embed calls (an
// accelerator out of memory gives way to the CPU), never during one.
type RunInfo struct {
    Device   string   // "cpu", or the device's id: "cuda:0"
    Name     string   // the device's own name when the runtime gives one, else ""
    Budget   int64    // bytes a batch may take on the device; 0 on the CPU
    Fallback Fallback // set only when Device is "cpu" and the request was "auto"
    Window   int      // texts the embedder wants in one Embed call; 0 means 8
}

// DeviceInfo is one line of the listing.
type DeviceInfo struct {
    ID, Backend, Name     string // "cuda:0", "cuda", "NVIDIA RTX PRO 6000 …"; ID "cpu" for the CPU
    Persistent            string // the runtime's lasting id for this device, or ""
    TotalBytes, FreeBytes int64  // 0 when unknown
    Memory                string // "live", "estimated" or "unknown": how FreeBytes was obtained
    MemoryKind            string // "dedicated", "shared" (the host's, an integrated GPU) or "unknown"
}

type Embedder interface {
    Embed(ctx context.Context, role Role, texts []string) ([][]float32, error)
    Dim() int
    Fingerprint() string
    Info() RunInfo
    Close() error
}

type Entry struct {
    Model   Model
    Open    func(ctx context.Context, a *Artifact, o OpenOptions) (Embedder, error)
    Derive  func(source [][]byte) ([][]byte, error)
    Devices func() []DeviceInfo // nil for a model that runs on the CPU only
}

type Registry interface {
    Lookup(name string) (Model, bool)
    Names() []string
    Open(ctx context.Context, a *Artifact, o OpenOptions) (Embedder, error)
    Derive(m Model, source [][]byte) ([][]byte, error)
    // Devices lists every device some entry can use, each once, in the order of "Devices", the
    // CPU last. It opens no model.
    Devices() []DeviceInfo
}
```

`Embed` keeps its contract: one vector per text, in order, all or nothing, one caller at a time.
`ErrAccelerator` stays for a failure the adapter does not recover from.

```go
package semantic

type Options struct {
    // … the existing fields …
    Device    string // the selector as cli validated it; "" means "auto" in Build
    GPUMemory int64  // bytes; 0 means the rule
}

type Progress struct {
    Section     item.Section
    Done, Total int
    Waiting     bool
    // Run is set on the one report sent when the build opens the model, and again if its Info
    // changes; nil on every other report.
    Run *embed.RunInfo
}

type BuildResult struct {
    // … the existing fields …
    Device string // what embedded this section's passages; "" when the run embedded nothing
}

func (s *Source) Close() // no error: a search has no accelerator to fail
```

`Build` opens with `OpenOptions{Device: opt.Device or "auto", MemoryBudget: opt.GPUMemory}`. `Source`
opens with the zero value.

## Devices

**Names.** `cpu`. A GPU is `<backend>:<index>`, with the backend's name as goinfer's listing gives
it: `cuda:0`, `metal:0`. The AMD path is goinfer's direct Vulkan backend (goinfer #21; operator's
decision, 2026-11-22: ROCm is not asked for), so an AMD card is written `vulkan:0` in the examples
below; pbrain fixes no spelling of its own, and the examples follow goinfer if it names it
otherwise. The index is the device's
place in that backend's own listing, which can change with a reboot or a visibility mask such as
`CUDA_VISIBLE_DEVICES`. Each device also has the persistent id its runtime gives it, printed by the
listing; goinfer is asked to say which of the two survives what (ask 7).

**Selectors**, in `--device` and `PBRAIN_DEVICE`:

| Selector | Means | When it cannot be had |
|---|---|---|
| `auto` (the default) | the rule below | the next candidate, at last the CPU with `cpu_fallback` |
| `cpu` | the CPU | — |
| `cuda`, `vulkan`, `metal` | the best device of that backend, by the same rule | error |
| `cuda:1`, or a persistent id | that device and no other | error |

Two failures of a named selector, kept apart:

- It names nothing the listing shows: a backend this binary lacks, a backend with no device on
  this machine, an index or id that is not there. `invalid_config`, exit 1, before any model is
  opened; the message lists what the listing shows and names `pbrain devices`.
- It names a listed device, and attaching it fails. `io`, exit 1.

A named device never gives way to another GPU or to the CPU.

**How it is set.** `pbrain sync --device <selector>` for one run; `PBRAIN_DEVICE` for the machine; the
flag wins. `PBRAIN_GPU_MEMORY`, a whole number of MiB, sets a batch's working memory; there is no
flag for it. Neither affects a search. A bad value of either is `invalid_config`.

**The automatic choice.** Candidates are the devices of the backends compiled in that can run this
model with batching. Drop a device only when a *live* figure says it cannot hold the weights; an
estimate or an unknown figure drops nothing. Order the rest by:

1. memory kind: its own, then unknown, then shared with the host;
2. the free figure: live, then estimated, then unknown;
3. most free memory, compared only within one row of 2;
4. backend, in the order `cuda`, `vulkan`, `metal` (operator's decision, 2026-11-22);
5. lowest index.

Try them in that order; the first that attaches runs the sync. So a discrete card with 6 GB free
by a live figure comes before a card of unknown kind, and both before an integrated GPU that
reports half the machine's RAM as its ceiling. On this machine the order is the RTX PRO 6000
(97 GB free), the Radeon RX 9060 XT (16 GB, 10.7 GB of it in use), and the integrated Radeon is
dropped if its 512 MiB is reported live.

A model with no accelerator, which is bge-small today, runs on the CPU under `auto` and `cpu`; a
named backend or device is `invalid_config` for it, with a message that says the model runs on the
CPU only.

**Listing.** A new command, `pbrain devices`. It needs no brain, like `pbrain skill`, opens no model
and changes nothing. It lists what this binary could attach on this machine, GPUs in the order of
the automatic choice, the CPU last:

```
cuda:0    NVIDIA RTX PRO 6000 Blackwell Max-Q  97887 MiB, 97249 free
vulkan:0  AMD Radeon RX 9060 XT                16304 MiB, 5581 free
cpu       32 threads
```

It marks nothing as chosen. What a sync uses depends on the project's model, which the listing
does not know, and the default model runs on the CPU only; the line a person can rely on is the
notice the sync itself prints (below). A build with no accelerator prints the `cpu` line alone.
JSON, in the same order:
`{"devices":[{"id":"cuda:0","backend":"cuda","name":"…","persistent":"GPU-…","total_bytes":…,"free_bytes":…,"memory":"live","memory_kind":"dedicated"},{"id":"cpu","backend":"cpu","threads":32}]}`.
`sync --status` is not given a device line: it would have to open every GPU, and it is the command
that is promised to cost nothing.

**In sync's output.** On standard error, once per `pbrain sync`, when the model is first opened to
embed:

- on a GPU: `embedding_device: cuda:0 (NVIDIA RTX PRO 6000 Blackwell Max-Q), 1024 MiB a batch`;
- on the CPU after `auto`: `cpu_fallback`, with the text for its reason:

| `Fallback` | Text |
|---|---|
| `unsupported` | `embedding on the CPU, which is slow: this pbrain has no GPU support for <model>` |
| `absent` | `embedding on the CPU, which is slow: no usable GPU was found` |
| `failed` | `embedding on the CPU, which is slow: the GPU could not be used` |
| `memory` | `embedding on the CPU, which is slow: the GPU has too little free memory` |

- on the CPU because `cpu` was asked, or for a model with no accelerator: nothing.

A run that embeds nothing opens no model and prints neither. With `--json` each is
`{"notice":{"code":"…","message":"…"}}` on standard error, the form of every notice, and each
section's object on standard output gains `"device"`, left out when that section embedded nothing.
The text on standard output does not change. A search prints neither notice.

**What the index records: nothing about the device.** The fingerprint names the model and how text
is prepared, not what ran it (`components/semantic.md`, "Fingerprints"), because the CPU and CUDA
agree to a cosine of 0.99999999999854 at worst where rule 2 asks 0.999. That stays, and it is the
gate for each new backend: the AMD backend and Metal join the shared fingerprint only when their vectors pass
rule 2 against the CPU's; one that does not gets a fingerprint of its own. One index can therefore
hold vectors from several devices: after a fallback in the middle of a run, a sync with another
`--device`, or vectors taken from the per-user cache. No column or `meta` row says which. A column
per passage would be a new index format, which embeds every personal section again, and the cache's
files would need it too; a single `meta` row would be wrong for exactly the mixed index it is meant
to explain. The cost is stated: if a backend is ever found to make bad vectors, the remedy is a
release that gives it its own fingerprint, or deleting the index and the cache's vectors.

## Batching on an accelerator

`gemma2.Embed` for documents, with an accelerator attached:

1. Tokenize the window's texts (`tokenIDs`, unchanged: at most 2,048 tokens each).
2. Walk them in order. Close a batch when the next input would take it past `P` positions or 256
   inputs; run `EmbedIDsBatch`; go on. One input alone uses `EmbedIDs`.
3. `P = min(cap, budget / bytesPerPosition)`. An input longer than `P` runs alone. `cap` is the
   backend's limit on a batch's total, 8,192 on v0.5.0. `bytesPerPosition` is 136 KB for CUDA on v0.5.0 (0.13 MiB
   measured), a constant of the adapter until goinfer reports it (ask 5).

A query, and any call with one text, is one `EmbedIDs`, as today. `Info().Window` is 256 on an
accelerator and 8 on the CPU. The build replaces `batchSize` with the embedder's window and is
otherwise unchanged: it cuts until the window is full, embeds, stores.

**Reuse by digest must not depend on the window.** Today it does, in three ways (the last two
found by @architect-2):

- A twin in the same flush of 8 is embedded and counted twice.
- A passage can take a vector from the old rows of another item that this run is about to
  replace, if that item happens not to be stored yet. In a personal section: A held a passage X; A
  is edited to Y, and B, later in path order, now holds X. With a window of 1, A's old rows are
  gone before B is cut, and B embeds X. With a wide window B finds A's old X.
- A passage can take a vector from a row this run has just stored, or not, by the same accident.
  Paths A, B, Z: A and the unchanged Z both hold X, with different valid bytes `a` and `z` from two
  devices. A's other passage is edited; B is new and holds X. With a window of 1, A is stored
  with `a` and B, taking the lowest path, gets `a`. With A and B in one window B gets `z`.

The rule that closes all three is stated on sets that are fixed when the run starts, never on what
happens to be stored. `replace` is the set of paths this run will store, known at step 6 of the
build. A *base row* is a row as the database held it when the run started. For a passage of item
I with digest D, the vector comes from the first of these that has one:

1. **I's own base rows** with D, lowest `ord` first. They are there whenever I is cut, since only I's
   own store removes them.
2. **Base rows of paths outside `replace`** with D: unchanged items, and items gone from the
   section, whose rows are kept until the last publication. Lowest path first, then lowest `ord`.
   Nothing in the run touches these rows.
3. **This run's own answer for D**: what the first passage with D in path order was given,
   whether that was a hit in 1, 2 or 4 or a new embedding. The build keeps a map from every digest
   it has resolved to where its vector is: in memory while that item is unstored, then the stored
   row's path and `ord`. About 100 bytes a passage, 5 MB at full size.
4. **The cache**, for the shared section. Asked only for a digest this run has not met, so it
   answers from what it held before the run.

Rows of other paths in `replace`, old or newly stored, are never read for this: `vectorByDigest`
takes the asker's path and `replace`, joins `passage` to `item`, and returns tiers 1 and 2 in that
order. Every tier that hits counts as reused; a passage that reaches the end is embedded and
entered in the map. In the three cases above B then embeds X, embeds X and gets `z`, at any
window.

`indexDB` also writes its four `meta` rows in a fixed order (it ranges over a map today, which is why
#34's personal databases differed by 121 bytes).

## The VRAM rule

After the weights are on the device:

1. **The budget**, from that device's free memory. With v0.5.0, in a `cuda` build, the figure is
   `decoder.FreeBytesFor("cuda")`, which opens a second handle to the default device, the only one
   v0.5.0 attaches.

   | The figure is | Budget |
   |---|---|
   | live | 30% of it, at most the ceiling |
   | an estimate | 30% of it, at most 1 GiB |
   | unknown | 1 GiB |

   A live zero is a figure, not "unknown": the budget is zero, and step 3 applies. Nothing raises
   a budget: not a floor, and not the length of the next passage.
2. **The ceiling is 6 GiB**, the lower end of the owner's range. No measurement supports more: the
   gain from 64 to 128 short inputs a call is 2.5% at 2.1 GB of device memory in all, and on real
   lengths nothing over 8,192 positions can run. 6 GiB is 46,000 positions, which leaves room for a
   lifted cap and a faster kernel without letting 30% of a 97 GB card mean 29 GB. It is to be
   measured again when ask 4 lands.
3. **One input alone always runs**, whatever the budget. The budget limits how many inputs share a
   forward; it cannot make one passage smaller. A passage of 8 tokens takes about 1 MiB and one
   of 2,048 about 270 MiB, and each is tried on the device as it comes. This is the minimum of
   the owner's "clamp to [min, max]", read as one input, and the one place the budget is
   exceeded; see the question for the owner. If the device cannot give that memory, step 5
   decides.
4. `PBRAIN_GPU_MEMORY` replaces the 30% and nothing else: the budget is that value, at most the
   ceiling and at most a live free figure. Step 3 holds for it too, and its description says so:
   "the most device memory a batch of several passages may take".
5. **Back-off.** When a batch of several inputs fails because the device is out of memory, `P`
   becomes half the positions of the batch that failed, the same inputs are packed again in order
   into strictly smaller groups of whole inputs, and those are retried. Every failure tries
   strictly fewer positions, down to one input a call, so the retries end. `P` does not grow again
   in that run. When one input alone fails: with `auto`, close the accelerator, embed that input
   and the rest of the run on the CPU and print `cpu_fallback` (`memory`); with a named device, fail
   with `io`. This step needs goinfer ask 3. Until then any accelerator error in a batch fails the
   run, as today.
6. Any other accelerator error in a forward fails the run: `io`, exit 1,
   `the model failed to embed a passage`. What was published stays, and the next `pbrain sync`
   goes on from it.

A working GPU is never refused beforehand because a longest passage might not fit: it is attached,
used, and given up only when an input really fails. Small cards: a card with 400 MiB free gets
120 MiB, a few short passages a call and long ones alone. Shared cards: the probe reserves
nothing. Two syncs that probe at the same moment read the same figure and each plans 30% of it;
if what they take collides, an allocation fails and the back-off recovers.

## The CPU

`bge.Embed` with more than one text starts `min(W, len(texts))` goroutines. Each takes the next
index from a shared counter, tokenizes that text and calls `model.Embed`, and writes the vector to
its own slot, so the result is in input order whatever finishes first. The context is checked
before each text; the first error ends the call with no vectors. One text runs as today, so a
query and a catch-up keep aikit's threads inside their one forward. aikit's `BERT` is made for
this: each forward takes its own scratch from a pool, and with several in flight its kernels stay
serial. The measurement found every vector equal to the one-forward build's, bit for bit.

`W` is `runtime.GOMAXPROCS(0)`, at most 32. Both ends are measured: 16 workers on 16 cores give 29.69
passages a second, and 32 on the same cores' two threads each give 33.64, 13% more for 220 MiB
more. Nothing above 32 is measured, so nothing above it is used. The window is 128 texts on every
machine, four times the largest `W`: the end of a window, where workers run out of texts one by
one, is then a small part of it, and what a run stores and counts cannot depend on how many cores
the machine has. The measurement used 32 texts a call; the ticket measures 128.

Memory: the slice peaked at 354 MiB as built and 783 MiB with 32 workers. At full size the build
peaks at 998 MiB today (#34), so about 1.4 GB is expected with 32 workers; that figure is an
estimate until ticket 7 measures it.

EmbeddingGemma 2 on the CPU stays one forward at a time: goinfer's CPU forward spreads over every
core by itself and has no gate for concurrent forwards, so workers would oversubscribe.

pbrain sets no thread limit and adds no setting. `GOMAXPROCS`, `taskset` and a container's limit are
honoured through the Go runtime.

## Sameness

- **Rows.** Which items and passages are stored, their spans, digests and order, follow from the
  scan alone. Items are stored in path order.
- **Counts.** `PassagesEmbedded` is the number of distinct digests that no tier of the reuse rule
  answers when first met. At any window, for one starting index and one starting cache.
- **Vectors, one device.** The vector of a passage is the same bytes whatever the budget, the
  window, a back-off or a resumed run, because a CUDA vector does not depend on its batch
  (measured; tested in goinfer; ask 6 makes it a stated guarantee for every backend). pbrain gates
  it with its own test on the real model.
- **Vectors, the CPU.** bge-small runs one forward a text at any worker count, and aikit's kernels
  give the same bits spread or serial: measured at 1, 4, 16 and 32 workers, 1,223 of 1,223 vectors
  equal. It is the workers' acceptance test.
- **Across devices.** Not the same bytes, and not claimed: within rule 2, one fingerprint, a mixed
  index allowed (above). Three comparisons, and only these, may show different vector bytes: a
  run on one device against a run on another; a run that fell back to the CPU part of the way
  against one that did not, where the point of the fall depends on what else held the card's
  memory and is not reproducible; and a run that reused vectors another device had put in the
  index or the cache. Two runs on one device with no fallback are compared by bytes, as #34 was,
  never by a cosine.
- **A resumed run** ends with the same stored items and passages as an uninterrupted one, and with
  the same bytes when the index it started from was of one origin: every vector it held for a digest had
  the same bytes, as after syncs on one device. Where the starting index held two different
  vectors for one digest, this is not claimed. A checkpoint moves a path out of `replace`, so the
  resumed run may read a row the uninterrupted run passed over, and take the other of the two
  valid vectors. Window invariance within one run holds in both cases, and each has its own test.
  `PassagesEmbedded` and `PassagesReused` are not part of this claim: they report the work of one
  invocation. A run interrupted after a window's vectors reached the cache reports them as reused
  when it resumes, where the uninterrupted run reported them embedded. The two counters are
  compared only between uninterrupted runs that start from the same index and the same cache.

## Search

`Source.runModel` opens with the zero `OpenOptions`: the CPU, in every build. A catch-up embeds on the
CPU, one passage a call, as now. With no accelerator in a search, the accelerator branches in
`source.go` go (`runModel`, `catchUpSection`, `Close` with its retry), `search/run.go`'s early return for
a source error goes back to what spec 7.1 says, and `Source.Close` returns nothing (operator's
decision, 2026-11-22; it touches `components/cli.md` and the #30 text).

`builder.close` drops an accelerator's close error like any other: the index is published and the
process is ending. Today it fails the sync with `io`; #30 kept that, and the operator decided on
2026-11-22 that one rule covers every close.

## Overlap of the stages (designed, not scheduled)

The build's own goroutine stays the only user of SQLite and of the cache. One more goroutine runs
`Embed` on one window after another, fed through a channel of capacity 1, so one window is always
queued behind the one being embedded. The coordinator cuts ahead, gives each returned vector to its
passage, writes the cache, and stores items strictly in path order as their last vector arrives,
with the 20-second publication and the 10-second progress checks where they are today. A digest
map over the windows in flight replaces the per-window one. An error or a cancelled context closes
the channel and waits for the goroutine before the embedder is closed; every send, of a window, a
result or an error, also waits on the context, so neither side can block the other's exit. What
is held is bounded by count, not by windows: the coordinator stops cutting while more than four
windows' worth of passages are cut and not stored, whether they wait for the device, for a twin's
vector, or behind one long item that spans windows; it goes on collecting and storing meanwhile.
The reuse rule above does not change: its tiers are stated on `replace` and on base rows, not on
what is stored, and its map already says whether a digest's vector is in memory or in a stored
row; here a vector can also be still on its way, and the passage waits for it.

What it would buy, from the measurements: cutting and storing are 0.1 ms a passage, which is 0.1%
at 12 passages a second and 6% at 615; a publication stops the model for 160 to 320 ms every 20
seconds at full size, 1.5%. Schedule it when a measured sync shows the device idle for more than
3% of its time.

## Files

- `internal/embed/embed.go`: `OpenOptions`, `Fallback`, `RunInfo`, `DeviceInfo`, `Embedder.Info`,
  `Entry.Devices`, `Registry.Devices`, the new `Open` signatures.
- `internal/embed/bge/bge.go`: the workers, `Info`; the new `open` signature.
- `internal/embed/gemma2/gemma2.go`: selection in `open`, the packer, the budget, `Info`.
  `accel_cpu.go`, `accel_cuda.go`: `devices()`, `attach(id)`, `freeMemory()`, `bytesPerPosition`,
  `batchCap`; `requireAccelerator` goes.
- `internal/semantic/build.go`: the window from `Info`, the digest map, `Progress.Run`,
  `BuildResult.Device`; `batchSize` goes. `index.go`: the `meta` order, `vectorByDigest` with the
  asker and `replace`, and the lookup of a stored row by path and `ord`. `source.go`: the CPU open, the
  accelerator branches removed.
- `internal/search/run.go`: the source-error return.
- `internal/cli`: `--device` on `sync`, the two variables, the two notices, `device` in JSON, the
  `devices` command and its place among the commands that need no brain; `read.go`: `Close`.
- Every test double of `Embedder` and `Registry` gains `Info`, `Devices` and the argument.

## Tickets, in order, each with the test that proves it

1. **The seam, and search on the CPU.** `OpenOptions`, `RunInfo`, `Info`; `Source` opens the CPU.
   A hook registry proves that every search path, catch-up included, opens with the zero options
   in a `cuda` build and that `Build` asks for `auto`. With the GPU: rule 3 in a `cuda` build inside
   600 ms. Closes #32.
2. **Selection on goinfer v0.5.0.** Selectors `auto`, `cpu`, `cuda` and `cuda:0` (the default device,
   the only one v0.5.0 attaches); the fallback; both notices; `--device` and `PBRAIN_DEVICE`;
   `device` in JSON; `pbrain devices` listing the default CUDA device and the CPU. In CI, where no
   driver exists: the `cuda` build's sync of `embeddinggemma-2` succeeds on the CPU with
   `cpu_fallback`; `--device cuda` and `--device nosuch` are `invalid_config`, since no CUDA device is
   listed there; `--device cpu` prints no notice; a named device with `bge-small-en-v1.5` is
   `invalid_config`; the untagged build prints `unsupported`; `pbrain devices` runs
   from an empty directory. With the fake registry: the notice table, and a listed device whose
   attach fails, which is `io`. The automatic order as a pure function: a discrete card with a live
   figure, a shared card reporting a larger ceiling, and one of unknown kind come out in that
   order's rule (own, unknown, shared).
3. **The window, the packer and the VRAM rule.** Packer unit tests: order kept, no batch over `P`
   or 256 inputs, a longest input alone. The budget as a pure function of the figure, its kind, the
   override and the constants, table-tested: live, estimated, unknown, a live zero, 400 MiB free
   (120 MiB, not raised), an override of 8 MiB (kept), an override over the ceiling.
   `TestBuildWindowInvariance`: a fake embedder whose vector is a function of the text; windows of
   1, 8, 64 and 1,024 give the same database bytes and the same counts, on a first build and on
   the two incremental fixtures above, in a personal section, with an index whose stored vectors
   differ from what the fake would make: A edited away from X with B edited to X; and A and Z
   holding X as `a` and `z` with B new. `TestReuseTiers`: each tier, and the order inside tiers 1
   and 2. `TestBuildResumed`: on a one-origin index, a run interrupted after its first
   publication and resumed ends byte for byte as an uninterrupted one; on the `a` and `z` index
   the test compares the stored item and passage rows and how many there are, not their vector
   bytes. Neither compares `PassagesEmbedded` or `PassagesReused` across an interruption. Every
   comparison between widths starts each run from a copy of the same index and its own copy of
   the same cache, so that no other checkout can add an entry in between. With the GPU
   (`PBRAIN_ALLOW_GEMMA2=1`): the 59 fixture passages alone and batched, equal bytes; passages a
   second and device memory on the 128-item slice and on a slice of short notes.
4. **bge-small workers.** The measurement is done (above). `TestBgeWorkersSameBytes`, on the real
   model where it is present, as `TestBgeParity` is: 64 texts at 1, 4 and 32 workers give equal
   bytes. With the fake model: input order kept under random delays, a cancelled context, an error
   from one text, a closed model, and one text taking the single path. Measured again on the
   slice: passages a second and peak memory at 4 cores, 16 cores and 32 threads, with windows of
   32 and 128. It goes first (operator's decision, 2026-11-22), before tickets 1 to 3, so it
   brings the two small things it needs and touches neither the gemma adapter nor device
   selection: an optional interface, `embed.Windowed { Window() int }`, which the build asks by
   type assertion and uses in place of its batch of 8, and which ticket 1 folds into
   `Info().Window`; and the fixed order of the `meta` rows, so that two databases can be compared
   as files. `TestBuildWorkersSameDatabase`: the same section built at 1, 4 and 32 workers gives
   the same database bytes and the same counts.
5. **goinfer.** When a release has the asks: move the pin (read first, as every pin). Enumeration
   and explicit devices (ask 7): every device listed, `cuda:1`, `vulkan:0` and persistent ids as
   selectors, the automatic order above as a pure function, table-tested with this machine's three
   GPUs. The back-off (ask 3): a fake accelerator that fails with the out-of-memory error twice,
   then for one input alone; the positions tried fall strictly, eight inputs of 200 tokens end in
   single calls and not in a loop, the vectors and rows before the fall equal a run with no
   failure, and the notice prints once. The cap and the memory figure from goinfer (asks 4 and 5).
   The AMD backend joins the shared fingerprint only after rule 2 passes for it.
6. **Overlap**, if a measurement asks for it (above).
7. **Measurements and the spec.** @lead-dev: the sweep above and both models at full size, in
   `internal/cli/README.measure.md`. @architect: functional spec 7.1, 9.4, 10, the notice list and
   the command list; architecture 8.7, a decision for GPU-first sync, 17; `components/semantic.md`
   (interfaces, Build's steps, both adapters, tests); `components/cli.md`; the semantic skill topic
   and the README. #29 closes with it.

Tickets 1 to 4 need nothing from goinfer. 2 needs 1; 3 needs 2. Ticket 4 is built first: it is
the largest measured gain in this plan for the default model and waits on no decision.

## What goinfer has to give (issues on `pkieltyka/goinfer`, drafted by @architect-2, agreed)

1. Free and total memory of the device that is attached.
2. Whether that figure is live, an estimate or unknown.
3. An out-of-memory error that `errors.Is` finds, from `EmbedIDs`, `EmbedIDsBatch` and the weight
   upload, with the accelerator still usable after a failed forward.
4. **The batch cap per input, not on the total.**
5. The working memory a batch of given lengths needs, with how good the figure is, asked without a
   forward or an allocation; and the weights' resident size.
6. A vector that does not depend on the rest of the batch, as a stated guarantee on every backend.
7. Every device of every backend linked in, CUDA and the AMD backend in one binary: a persistent id, a name,
   total and free memory, whether the memory is its own, batching support; attaching one by id; an
   error that tells an absent device from a failed one.
8. **CUDA throughput on long inputs**: 6,500 tokens a second with eight real passages a call
   against 37,709 with 128 short ones, the device busy 96%; a profile first, then a target of ten
   times at 512 tokens an input.

4 and 8 are the two that decide how fast a GPU sync is. 4 alone gains little while 8 stands.

## Open questions

For the owner, through @operator:

- **bge-small has no GPU path.** It is the default model. Under this plan its sync prints no
  warning, since nothing was fallen back from and a person can do nothing about it. The other
  reading of the direction is a warning on every CPU sync. Recommended: no warning for bge-small.
  (aikit has a seam for a matrix-product backend, `BERT.UseBackend`; bge-small on a GPU would be its
  own measurement and is not in this plan.)
- **One passage alone may exceed the budget.** A longest passage needs about 270 MiB. Where 30%
  of free memory, or the amount a person set, is less than a passage needs, that passage still
  runs on the GPU, alone. The other course is to send such a run to the CPU, or to fail a named
  device. Recommended: run it, as the minimum of "clamp to [min, max]"; the limit then governs
  batching and never refuses a card that works.
- **Overlap of the stages (ticket 6)** waits for a measurement, against point 4 of the direction.
  Recommended: wait; it is worth under 2% today.
- **Release binaries are untagged**, so a release user of `embeddinggemma-2` would always read "this
  pbrain has no GPU support". Once ticket 2 makes a `cuda` build safe on a machine with no GPU, the
  Linux release can carry the tag: 56 MiB against 23 MiB, unstripped. Recommended: yes, as its own
  ticket after ticket 2.
- **`eg2_combined`**, open since #29: supported, or an experiment to leave out of the spec?

For @lead-dev, with ticket 3:

- A slice of short notes for the GPU figures: the fixture has none.

## Risks

- A GPU sync of long passages stays near 12 passages a second until goinfer ask 8 lands. Batching
  adds 8% there.
- With v0.5.0 the probe opens a second handle to the default device, once per sync, and "no GPU"
  and "the GPU failed to attach" cannot be told apart: both print `failed`'s text.
- The Radeon RX 9060 XT on this machine has 10.7 GB in use by other work. No run of ours uses it
  until the owner says so; tests of the AMD backend wait for that and for goinfer.
- Each section opens the model for itself, so a sync of both sections uploads the weights twice,
  about a second each. Not changed here.
- The figures above come from 128 items on four cores with other work on the machine.
