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
The standard Linux archives include CUDA support. Use the Linux archives ending in
`_cpu.tar.gz` for a smaller CPU-only binary. `go install` also gives a CPU-only binary.
The macOS binaries are CPU-only, untested and unsupported for now.

For CUDA support on Linux with `go install`, use
`go install -tags cuda github.com/pkieltyka/pbrain/cmd/pbrain@latest`.
To build with CUDA from source on Linux, run `CGO_ENABLED=0 go build -tags cuda ./cmd/pbrain`.
The default model is `bge-small-en-v1.5` in every build. The CUDA build loads
`libcuda.so.1` dynamically at runtime. Set `LD_LIBRARY_PATH` if the driver library is outside the
default loader search path. Run `pbrain devices` to see the available devices. For an
EmbeddingGemma 2 sync, the CUDA build picks the default CUDA device when it can and warns when
it runs on the CPU. Use `pbrain sync --device cuda` to require CUDA or `--device cpu` to request
the CPU. `PBRAIN_DEVICE` sets the choice when the flag is absent. The default bge-small model
always syncs on the CPU without a notice, regardless of the device selector.
For CUDA sync, the batch budget is 30% of free GPU memory after loading the model,
up to 6 GiB. Set `PBRAIN_GPU_MEMORY` in MiB to override it. A passage can run alone
when it exceeds the batch budget.

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
index; with an index, it first adds up to 20 new or changed items itself.

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
of memory.

With an NVIDIA GPU and the CUDA build described above, `embeddinggemma-2` built that index in
1 hour 10 minutes on a Ryzen 9 7950X with an RTX PRO 6000, with a peak of about 5 GB of memory.
Search uses the CPU in every build. On that brain, search with `embeddinggemma-2` took about
0.8 seconds, and `bge-small-en-v1.5` took about 0.3 seconds. To use `embeddinggemma-2`, set the
model in `docs/brain/pbrain.toml` and run `pbrain sync` again.

```toml
[semantic]
model = "embeddinggemma-2"
```

Set `model = "none"` in the same section to turn semantic search off.

| Model | First download | Local model files | Cutoff | Expected item on first page (47 answerable) | Empty page (30 unanswerable) |
| --- | ---: | ---: | ---: | ---: | ---: |
| `bge-small-en-v1.5` (default) | 134 MB | 134 MB | 0.65 | 40 | 29 |
| `embeddinggemma-2` | 1.52 GB | 552 MB | 0.72 | 42 | 30 |

The download notice rounds up to whole MB (135 MB for bge, 1,522 MB for Gemma).
Allow about 2.1 GB on disk for Gemma's model files during the first build, while the
download and the derived local model coexist. The index needs additional space.

On this fixture, strict search found the expected item on 16 of 47 first pages.
Both models kept all 16, and adding `--bm25` to `--semantic` gave the same counts
shown above. For six long-document questions, the reader saw the supporting passage
on the search page or after one follow-up read in five cases with Gemma and four
with bge. [Calibration results and method](internal/semantic/calibrate/README.md)

## Measured speed

| Release measurement (5,000 items, 50 MB) | Linux (Intel Core Ultra 7 255H) | Apple silicon Mac (untested, deferred) |
| --- | --- | --- |
| Lexical search / get, personal view | 135–189 ms / 134 ms | Not yet measured |
| BGE semantic search, shared view (4,500 visible items) | 302–309 ms | Not yet measured |
| BGE semantic search, personal view | 332–333 ms | Not yet measured |
| BGE semantic + BM25, shared / personal view | 298 ms / 335 ms | Not yet measured |
| BGE first index / one-note update | 1 h 41 m / 0.77 s | Not yet measured |
| EmbeddingGemma 2 first index, CPUs only | Projected 15 h (not run to the end) | Not yet measured |

Search and get figures are warm medians on four CPUs. On four cores of a Ryzen 9 7950X the first index
took 1 h 18 m, and the semantic search medians were 291–298 ms in the shared view and 321–326 ms in the
personal view. On that machine with its RTX PRO 6000, EmbeddingGemma 2's first index took 1 h 10 m with
the CUDA build. Its CPU semantic search medians were 773–820 ms on four cores.
[Method, both machines and peak memory](internal/cli/README.measure.md)
