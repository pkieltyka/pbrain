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
The default embedding model is meant to be indexed on a GPU, so use a GPU archive or build
where the machine has one ([Semantic search](#semantic-search)).
On Apple Silicon, `darwin_arm64` includes Metal and `darwin_arm64_cpu` is the
CPU companion. The Darwin amd64 archive remains CPU-only.

For ordinary CUDA support on Linux with `go install`, use
`go install -tags cuda github.com/pkieltyka/pbrain/cmd/pbrain@latest`.
Starting with v0.4.0, the Linux GPU archives also link the fast CUDA kernel.
To build that variant from source, run
`CGO_ENABLED=0 go build -tags 'cuda eg2_combined eg2_fp16' ./cmd/pbrain`.
On Linux amd64, add `vulkan` to those tags to match the standard archive
and sync on discrete AMD GPUs. The CUDA build loads
`libcuda.so.1` dynamically at runtime. Set `LD_LIBRARY_PATH` if the driver library is outside the
default loader search path. Vulkan uses the system Vulkan loader and an AMD driver.
Build the Apple Silicon variant with `CGO_ENABLED=0 go build -tags metal ./cmd/pbrain`.
Run `pbrain devices` to see the available devices in automatic selection order.
The order uses available memory information and gives live figures priority over
estimates for devices of the same memory kind.
Use `pbrain sync --device vulkan` to select AMD, or `--device vulkan:<uuid>` to pin
one card by the persistent ID from `pbrain devices --json`. Use `--device cuda`
to require CUDA or `--device cpu` to request the CPU. `PBRAIN_DEVICE` sets the choice
when the flag is absent. Sync prints a `cpu_fallback` notice when it runs on the CPU.
With the default model, `embeddinggemma-2-q8-0`, use `--device metal` or `--device metal:0` to
select the system-default Metal device. Auto selection can also choose Metal for
this row. A switch between Metal profiles or between Metal and another backend
rebuilds the affected Q8 sections. CPU search can read either Metal profile.

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

The default model is `embeddinggemma-2-q8-0`, Unsloth's Q8_0 conversion of Google's EmbeddingGemma 2:
a 310 MB download in a single GGUF file. [Pinned model card and Apache-2.0 license](https://huggingface.co/unsloth/embeddinggemma-2-GGUF/blob/031f0d4b35536f69ab3509d4893c923264fcf253/README.md)
It reads up to 2,048 tokens of a passage, which suits long documents. It is a heavy model, and its
index is meant to be built on a GPU: it syncs on NVIDIA (CUDA), discrete AMD (Vulkan) and Apple
Silicon (Metal), and on CPUs as a fallback.

| First index of the 5,000-item, 50 MB test brain (48,511 passages) | Time | Peak memory |
| --- | --- | --- |
| NVIDIA RTX PRO 6000 with a Ryzen 9 7950X, CUDA fp16 | 8 m 43 s | 2.7 GB, and 1.9 GB on the GPU |
| Apple M4, Metal | 1 h 21 m | 3.6 GB |
| CPUs only | About 10 h, projected and not run | Not measured |

The CPU figure comes from 1.32 passages a second on four Ryzen 9 7950X cores. More cores help
little there: 32 threads reached 1.88. The test brain's generated text is token-heavy, so ordinary
notes may go faster, and a small brain still takes minutes. A plain `go install` and the `_cpu`
archives have no GPU support.

Search uses the CPU in every build and loads the model each time, which takes half a second or
more and a gigabyte of memory or more. On that brain the warm medians of four queries were 459–484 ms on an Apple M4
and 684–729 ms on four Ryzen 9 7950X cores, with up to 1.5 GB of memory.

`embeddinggemma-2` is Google's own pack of the same model: a 1.52 GB download that syncs on CPU,
CUDA or Vulkan and has no Metal path. To use it, set the model in `docs/brain/pbrain.toml` and run
`pbrain sync` again.

```toml
[semantic]
model = "embeddinggemma-2"
# cuda_precision = "fp32"  # keep ordinary CUDA indexing
```

**Upgrading from v0.4.x.** `bge-small-en-v1.5`, the default until v0.5.0, has been removed. A
project with no `model` key now uses `embeddinggemma-2-q8-0`: run `pbrain sync` once to download it,
unless it is already in the cache, and build the new index, with `--personal` if you index a personal section. Until then
`search --semantic` returns exact matches and a notice. If `pbrain.toml` names
`bge-small-en-v1.5`, delete that line or name another model; `pbrain sync` says so. The old
model's files stay in the cache folder until you delete them.

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
| `embeddinggemma-2-q8-0` (default) | 310 MB | 310 MB | 0.72 | 42 | 30 |
| `embeddinggemma-2` | 1.52 GB | 552 MB | 0.72 | 42 | 30 |

The download notice rounds up to whole MB (310 MB for Unsloth's Q8_0 conversion, 1,522 MB for
Google's pack). Allow about 2.1 GB on disk for Google's model files during the first build, while the
download and the derived local model coexist. The index needs additional space.

On this fixture, strict search found the expected item on 16 of 47 first pages.
Both models kept all 16, and adding `--bm25` to `--semantic` gave the same counts
shown above. For six long-document questions, the reader saw the supporting passage
on the search page or after one follow-up read in five cases with Google's pack. [Calibration results and method](internal/semantic/calibrate/README.md)

## Measured speed

| Release measurement (5,000 items, 50 MB) | Linux | Apple M4, 24 GiB |
| --- | --- | --- |
| Lexical search / get, personal view | 135–189 ms / 134 ms | Not measured |
| Semantic search, shared view (4,500 visible items) | 684–707 ms | 459–474 ms |
| Semantic search, personal view | 715–729 ms | 479–484 ms |
| First index on a GPU | 8 m 43 s (RTX PRO 6000, CUDA fp16) | 1 h 21 m (Metal) |
| First index on CPUs only | About 10 h, projected | Not measured |
| Update after one note | Not measured | 1.15 s (Metal) |

The semantic rows are for the default model, `embeddinggemma-2-q8-0`. Search and get figures are
warm medians on four CPUs; semantic search embeds its query on the CPU in every build. The lexical
figures are from an Intel Core Ultra 7 255H. The Linux semantic search and first index are from a
Ryzen 9 7950X with an RTX PRO 6000; its searches ran with the model file sealed in the cache
(fs-verity), and all 40 samples were under 1 s. The Apple M4 search and one-note figures are from the v0.4.6 archives, and its first
index from the v0.4.7 build that batches on Metal. Google's `embeddinggemma-2` pack, on the Ryzen:
a first index in 8 m 37 s with CUDA fp16, and CPU semantic search medians of 773–820 ms on four
cores.

Until v0.5.0 the default was the much smaller `bge-small-en-v1.5`. On the Intel machine it searched
in 302–333 ms and built the first index in 1 h 41 m on four CPUs; on the Ryzen's 32 threads the
index took 20 m 25 s. Those figures are kept in the measurement record.
[Method, both machines and peak memory](internal/cli/README.measure.md)
