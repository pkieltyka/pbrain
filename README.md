# pbrain

pbrain keeps project knowledge in Git as Markdown. People and agents can record decisions,
learnings and sources, search for a few relevant items, and cite them by ID. By default,
search returns up to five results in 3,000 characters. Shared items live in the repository.
A Git-ignored personal section is available with `--personal`.

## Install

With Go 1.27 or later, install from a pbrain checkout.

```sh
go install ./cmd/pbrain
```

While the repository is private, tagged installs and release downloads require repository access.
Once a release is tagged, install it with
`go install github.com/pkieltyka/pbrain/cmd/pbrain@latest` or download the
Linux or macOS binary for your architecture (amd64 or arm64). [GitHub Releases](https://github.com/pkieltyka/pbrain/releases)
The standard Linux archives include GPU support. After v0.2.0, the amd64 archive has CUDA for
NVIDIA GPUs and Vulkan for discrete AMD GPUs, and the arm64 archive has CUDA alone, because the
Vulkan backend supports Linux amd64 only. The v0.2.0 GPU archives have CUDA alone. A GPU binary
starts without either driver: with no NVIDIA driver and no Vulkan loader, `pbrain devices` lists
the CPU and sync runs on the CPU. Use an archive ending in
`_cpu.tar.gz` for a CPU-only binary. Plain `go install` also gives a CPU-only binary.
On Apple Silicon, `darwin_arm64` includes Metal and `darwin_arm64_cpu` is the
CPU companion. The Darwin amd64 archive remains CPU-only.

For ordinary CUDA support on Linux with `go install`, use
`go install -tags cuda github.com/pkieltyka/pbrain/cmd/pbrain@latest`.
Starting with v0.4.0, the Linux GPU archives also link the fast CUDA kernel.
To build that variant from source, run
`CGO_ENABLED=0 go build -tags 'cuda eg2_combined eg2_fp16' ./cmd/pbrain`.
On Linux amd64, add `vulkan` to those tags to match the standard archive
and sync the Google EmbeddingGemma 2 pack on discrete AMD GPUs. The CUDA build loads
`libcuda.so.1` dynamically at runtime. Set `LD_LIBRARY_PATH` if the driver library is outside the
default loader search path. Vulkan uses the system Vulkan loader and an AMD driver.
Build the Apple Silicon variant with `CGO_ENABLED=0 go build -tags metal ./cmd/pbrain`.
Run `pbrain devices` to see the available devices in automatic selection order.
The order uses available memory information and gives live figures priority over
estimates for devices of the same memory kind.
Use `pbrain sync --device vulkan` to select AMD, or `--device vulkan:<uuid>` to pin
one card by the persistent ID from `pbrain devices --json`. Use `--device cuda`
to require CUDA or `--device cpu` to request the CPU. `PBRAIN_DEVICE` sets the choice
when the flag is absent. EmbeddingGemma 2 sync warns when it runs on the CPU.
With `embeddinggemma-2-q8-0`, use `--device metal` or `--device metal:0` to
select the system-default Metal device. Auto selection can also choose Metal for
this row. A switch between Metal profiles or between Metal and another backend
rebuilds the affected Q8 sections. CPU search can read either Metal profile.
The default `bge-small-en-v1.5` model always syncs on the CPU without a notice,
regardless of the device selector.

For CUDA sync, the batch budget is 30% of live free memory after loading the model,
up to 6 GiB. An estimated reading, such as Vulkan headroom, uses 30% up to 1 GiB.
Unknown headroom gives a 1 GiB budget. Set `PBRAIN_GPU_MEMORY` in MiB to
override the batch budget. Leave headroom for model weights and other GPU
users, including the display if it shares the card. A passage can run alone
when it exceeds the batch budget. Sync splits a batch on a retry-safe out-of-memory
error. With automatic selection, a retry-safe failure on one passage moves
an ordinary CUDA or Vulkan sync to the CPU and prints a notice. An fp16 or
Metal sync stops at its last saved checkpoint. On a base Apple M4, Metal sync
batches up to four passages. Other Apple Silicon Macs run one passage at a time.
`PBRAIN_METAL_BATCH=0` turns off Metal batching.

## Quick start

From the root of your project, create a brain in `docs/brain` and record a decision.
Use your own name for the author and the person who made the decision.

```sh
pbrain init
pbrain add --kind decision --title 'Sessions live in Redis' \
  --body 'A shared store keeps sessions available across API restarts.' \
  --author human:alex --decided human:alex
pbrain search sessions Redis
```

Copy the ID from the search result into `pbrain get <id>` to read the decision.
An ID shown in double quotes is a JSON string: decode it, or take `id` from `--json`.
Quote an ID that has spaces in the shell.
Plain search reads the Markdown directly, so a new item is searchable immediately.
Use `--json` for machine-readable results. Run `pbrain skill` for the agent guide
or `pbrain skill list` for its topics.

## Semantic search

Use `pbrain search --semantic 'where is login state kept'` to find related wording.
Exact hits come first. When the model or index is unavailable, search returns
the lexical results and a notice. Search never downloads a model or builds a first
index. CPU catch-up can cover up to 20 items and 20 passages in an existing index.
Sections indexed with CUDA fp16 need a sync to cover new or changed items.

**The first index on a machine is a person's step.** Check `pbrain sync --status`,
then run `pbrain sync` when ready. It downloads the selected model if needed and
embeds items locally. A small brain takes minutes; a large one takes hours. You can
stop the build. The next `pbrain sync` resumes from the last saved checkpoint.
After writes or a pull, rerun `pbrain sync` to pick up changes.

The default model is `bge-small-en-v1.5`: a 134 MB download, quick to load.
`embeddinggemma-2` is the larger option. On our fixture it found the expected item for two
more questions of 47, and it costs far more. With it on four CPUs of an Intel Core Ultra 7 255H,
the first build of the 5,000-item, 50 MB test brain ran at 0.902 passages a second (a passage
there averaged about 1,000 characters): a projected 15 hours, with a peak of about 2 GB of memory
observed; that build was not run to the end. That generated text is token-heavy, so ordinary notes
may go faster. The default built the same index in 1 hour 41 minutes, with a peak of about 1.4 GB
of memory. That was on four CPUs. `pbrain sync` now runs the default model on every core, up to 32:
on a 16-core Ryzen 9 7950X the same index took about 20 minutes, with a peak of about 1.9 GB.

With an NVIDIA GPU and the CUDA build described above, `embeddinggemma-2` built that index in
1 hour 1 minute on a Ryzen 9 7950X with an RTX PRO 6000, with a peak of about 4.5 GB of memory
and 2.2 GB on the GPU.
Search uses the CPU in every build. On that brain, search with `embeddinggemma-2` took about
0.8 seconds, and `bge-small-en-v1.5` took about 0.3 seconds. To use `embeddinggemma-2`, set the
model in `docs/brain/pbrain.toml` and run `pbrain sync` again.

```toml
[semantic]
model = "embeddinggemma-2"
# cuda_precision = "fp32"  # keep ordinary CUDA indexing
```

Use `model = "embeddinggemma-2-q8-0"` for Unsloth's Q8_0 conversion of
Google's EmbeddingGemma 2. It syncs on CPU, CUDA, Vulkan or Apple Silicon Metal and downloads
a single GGUF file. [Pinned model card and Apache-2.0 license](https://huggingface.co/unsloth/embeddinggemma-2-GGUF/blob/031f0d4b35536f69ab3509d4893c923264fcf253/README.md)

The Google pack and Q8_0 default to fp16 indexing with a compatible NVIDIA GPU
(compute capability 8.0 or newer) and the fast kernel linked in. Older CUDA
cards use fp32 and sync reports the reason. When upgrading, run `pbrain sync`
to rebuild the index and add `--personal` if you index a personal section.
You can retain the existing CUDA vectors by setting
`semantic.cuda_precision = "fp32"` before upgrading. Either precision reuses
the downloaded model files. Search always runs on the CPU.

Set `model = "none"` in the same section to turn semantic search off.

| Model | First download | Local model files | Cutoff | Expected item on first page (47 answerable) | Empty page (30 unanswerable) |
| --- | ---: | ---: | ---: | ---: | ---: |
| `bge-small-en-v1.5` (default) | 134 MB | 134 MB | 0.65 | 40 | 29 |
| `embeddinggemma-2` | 1.52 GB | 552 MB | 0.72 | 42 | 30 |
| `embeddinggemma-2-q8-0` | 310 MB | 310 MB | 0.72 | 42 | 30 |

The download notice rounds up to whole MB (135 MB for bge, 1,522 MB for Google's pack,
310 MB for Unsloth's Q8_0 conversion). Allow about 2.1 GB on disk for Google's model files during the first build, while the
download and the derived local model coexist. The index needs additional space.

On this fixture, strict search found the expected item on 16 of 47 first pages.
Both models kept all 16, and adding `--bm25` to `--semantic` gave the same counts
shown above. For six long-document questions, the reader saw the supporting passage
on the search page or after one follow-up read in five cases with Gemma and four
with bge. [Calibration results and method](internal/semantic/calibrate/README.md)

## Measured speed

| Release measurement (5,000 items, 50 MB) | Linux (Intel Core Ultra 7 255H) | Apple M4, 24 GiB (v0.4.6) |
| --- | --- | --- |
| Lexical search / get, personal view | 135–189 ms / 134 ms | Not measured |
| BGE semantic search, shared view (4,500 visible items) | 302–309 ms | 274–277 ms |
| BGE semantic search, personal view | 332–333 ms | 295–297 ms |
| BGE semantic + BM25, shared / personal view | 298 ms / 335 ms | Not measured |
| BGE first index / one-note update | 1 h 41 m / 0.77 s | 51 m on ten cores / not measured |
| EmbeddingGemma 2 first index, CPUs only | Projected 15 h (not run to the end) | Not measured |

Search and get figures are warm medians on four CPUs. The BGE first-index figure is also from four
CPUs, before `pbrain sync` ran the default model on every core. On a Ryzen 9 7950X the first index
now takes 20 m 25 s on 32 threads and 21 m 1 s on 16 cores; it took 1 h 18 m on four cores before
that change. The semantic search medians there were 291–298 ms in the shared view and 321–326 ms in the
personal view. On that machine with its RTX PRO 6000, EmbeddingGemma 2's first index took 1 h 1 m with
the CUDA build. Its CPU semantic search medians were 773–820 ms on four cores.

The Apple M4 column is from the v0.4.6 archives. Its search figures are warm medians of five fresh
processes on four cores. With `embeddinggemma-2-q8-0` on that Mac, the first index took 1 h 28 m
with `--device metal`, an update after one note took 1.15 s, and the CPU semantic search medians were
459–484 ms.
[Method, both machines and peak memory](internal/cli/README.measure.md)
