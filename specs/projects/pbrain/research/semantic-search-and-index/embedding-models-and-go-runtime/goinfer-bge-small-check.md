# bge-small-en-v1.5 on aikit (goinfer's own dependency): a check

Run 2026-11-21 by @lead-dev-2, at the owner's request, to see whether pbrain can drop hugot and run both models on one runtime family. Same rig, texts and rules as [spike-bge-small-hugot.md](./spike-bge-small-hugot.md): an Intel Core Ultra 7 255H, Linux x86_64, Go 1.27.3, `GOMAXPROCS=4` unless stated, warm cache, single runs. The source of the packages involved was read before anything ran (aikit v1.78.0; its encoder, embed, linalg and mmap packages were also covered by the goinfer audit).

## Bottom line

**Switch bge-small to aikit's encoder and drop hugot,** after one small addition to aikit: a loader that reads from an `fs.FS`, about 100 lines that mirror code aikit already has. On every rule it is as good as hugot or better, except indexing speed:

| | aikit `encoder.BERT` | hugot + GoMLX (accepted interim) |
|---|---|---|
| Token ids and vectors against sentence-transformers, 22 texts | ids identical; worst 1 − cos 1.7e-13 | ids identical; worst 1 − cos 2.8e-10 (SIMD) |
| Verify + load + first query | **124 to 153 ms** | 440 to 485 ms (SIMD) |
| Load alone | 8 to 12 ms | 269 to 306 ms |
| Warm query | 16 to 19 ms | 23 to 27 ms |
| Peak memory, one search | **0.14 GB** (the 134 MB of model bytes included) | 0.60 GB |
| Indexing, 4 threads | ~2,800 tokens/s; 5,000 items ≈ 14 min | ~3,700 to 4,100 tokens/s; ≈ 10 min |
| Peak memory, indexing | 0.17 to 0.19 GB | 0.82 to 0.88 GB |
| Binary size added (stripped) | **≈ 0.9 MB** | 18 to 24 MB |
| `GOEXPERIMENT=simd` needed | no (it made no difference) | yes, or indexing takes about 95 min |
| Builds from in-memory bytes | yes, once `LoadBERTFromFS` exists (below) | yes, through the `s3:` trick |
| Environment variables read, stdout prints | none in the packages used | `GOMLX_*` at start-up (fatal on bad values); one `fmt.Printf` path |
| 20-question check | 13 first, 17 in top 3 | 13 and 17 |

It also means **one dependency for both models**: goinfer's main branch already requires aikit v1.78.0, so EmbeddingGemma 2 on the goinfer fork and bge-small on aikit share it. The two things that may not reach pbrain stay out:
- hugot's large dependency tree (GoMLX, protobuf, go-huggingface with `net/http`);
- the `envguard` and stray-output work-arounds that exist only because of GoMLX. The null-device redirect stays a good default regardless.

The cost is indexing: about 30% slower than hugot with SIMD, so roughly 14 minutes instead of 10 for a first index of 5,000 items. The per-user cache spares later work trees from repeating it.

## What aikit has, and the one gap

- aikit's `encoder` package runs BERT-family embedders and lists `BAAI/bge-small-en-v1.5` among its 12 certified models. It reads CLS pooling from `1_Pooling/config.json` and has a parity test against a sentence-transformers reference (`encoder/bge_test.go`, `TestBGE_parity`, cosine ≥ 0.9999; `docs/embedder-coverage.md`). The WordPiece tokenizer is aikit's own (`embed.Tokenizer`). The relevant API is `encoder.LoadBERT(dir)` and `BERT.Embed(ids []int32) []float32`, which pools and normalises.
- **The gap.** `LoadBERT` reads `config.json`, `sentence_bert_config.json` and `1_Pooling/config.json` with `os.ReadFile`, and maps `model.safetensors` by path. The Artifact rule forbids that. aikit already has the `fs.FS` forms of every piece (`embed.OpenSafetensorsFromFS`, `embed.LoadTokenizerFromFS`, and the unexported `poolingFromFS`), and its nomic-bert `Model` has `LoadFromFS`. Only BERT lacks one.
- **The fix, tried here.** In a scratch copy of aikit v1.78.0 I added `encoder.LoadBERTFromFS(fsys fs.FS, dir string) (*BERT, error)`: `loadBERT` with each read swapped for its `fs.FS` form, about 100 lines including a `BERT.Tokenizer()` accessor. The probe served the five verified buffers through a tiny `fs.ReadFileFS` that hands out the slices themselves, so the weights alias the checked bytes and nothing is copied.
  - `strace` shows each model file opened exactly once, by the check step's own read, and nothing else in the model folder.
  - The accessor is optional: the adapter can load its own tokenizer with the public `embed.LoadTokenizerFromFS`.
- **Where the change goes.** The code must live in aikit, since `BERT`'s fields are private. It cannot go in the goinfer fork, so I created no `pbrain/bge-small` branch there; there is nothing to put on it. The choices, as for goinfer:
  - upstream first, a small pull request mirroring `LoadFromFS`;
  - or an owner fork of aikit pinned through a `replace`.
  
  Both need the owner's word. The patched file is in my scratch folder (`aikit-patched/encoder/bert_fs.go`).

## The adapter on aikit

It is the same adapter as the hugot one, so the preparation version need not change. Only the runtime under it changes.
- **Special tokens.** aikit's tokenizer, like Hugging Face's, reads a literal `[SEP]` in text as the real token: `a [SEP] b [CLS] c` gives 102 and 101. The adapter's rewrite to `[ SEP]` gives the plain pieces, the same ids as the hugot adapter.
- **The query instruction**, the split at the first blank line, the 510-token cut that keeps the prefix to at least 255 tokens, `[CLS]`…`[SEP]` wrapping, and `BERT.Embed`'s CLS pooling and L2 normalisation.
- **The longest-prefix test**, with a 400-token Chinese and punctuation prefix cut to 255:

  | Change | Cosine |
  |---|---|
  | the passage's first word | 0.9863 |
  | a word in the cut tail | 1.0000000 |
  
  These are the same numbers as with hugot: the vectors agree to 2.5e-10 across all 30 corpus windows.
- **Batching.** There is no batch call for BERT in aikit (`EncodeBatch` exists for its nomic `Model`). Each text is one forward pass, and they are independent and deterministic.

## Safety and build

- **No cgo** in any compiled package, also with `CGO_ENABLED=1`. It builds for linux and darwin on amd64 and arm64.
- **No `net`, `net/http`, `os/exec` or `plugin`** in the compiled set.
- **Compiled packages:** aikit's `encoder`, `embed`, `linalg`, `mmap`, `sparse`, `topk`, `internal/accum` and `internal/cursor`, plus `x/sys/cpu` and `x/text`.
- **Assembly:** `linalg` has 21 files of AVX2 and NEON kernels, chosen by CPU detection with a Go fallback on amd64. `embed` has 2, which only GGUF uses.
- **Environment and output:** no environment variable is read and nothing is printed in `embed`, `encoder`, `linalg` or `mmap` outside tests (grep, 2026-11-21). `x/sys/cpu` reads `GODEBUG` as the standard library does. Neither envguard nor the stdout redirect is needed for this runtime; the redirect is harmless to keep.
- **Binary size** (stripped, a program that loads and embeds):

  | Target | Size | A program without aikit |
  |---|---|---|
  | linux/amd64 | 3.51 MB | 2.57 MB |
  | linux/arm64 | 3.28 MB | |
  | darwin/amd64 | 3.58 MB | |
  | darwin/arm64 | 3.34 MB | 2.48 MB |
  
  `GOEXPERIMENT=simd` changes the size by a few kilobytes and the indexing speed not at all (5.8 s against 5.0 s on the same 30 windows). Only linux/amd64 was run.
- **Licence:** MIT, the same author as goinfer.

## Measurements

- **Parity:** 12 documents and 10 queries; sentence-transformers 6.3.0 reference. Token ids identical for all 22, worst 1 − cos 1.7e-13. This is the reference file used for hugot.
- **Start-up, three runs:**
  - verify (read and SHA-256 five files, 134 MB): 104 to 110 ms;
  - load: 7.9 to 8.6 ms;
  - first query: 12 to 15 ms;
  - in all 124 to 132 ms, with peak RSS 140 MB.
  
  The whole process, with ten more queries, took 0.29 to 0.31 s of wall time. Loading by path instead (`LoadBERT`, mapped) gives about 12 ms of load and a first query of 21 ms.
- **Indexing:** 30 windows, 14,142 tokens, 5.02 s at 4 threads, about 2,800 tokens/s, peak 0.19 GB. At 16 threads it was 2,280 tokens/s, slower, as on this hybrid CPU before.
- **The 20-question check:** 13 first and 17 in the top 3. As expected, the vectors are the reference's.

## Pin

- **aikit:** `github.com/townsendmerino/aikit v1.78.0`, the version goinfer main e0d66171 requires, plus `LoadBERTFromFS` from the owner fork: commit `04b54e26`, merged into the fork's main as `eb8c5ddd` (below).
- **Model files:** as in the hugot spike (`BAAI/bge-small-en-v1.5` revision `5c38ec7c`), except that aikit reads the **safetensors** weights, not the ONNX file. The files:

  | File | Size | SHA-256 |
  |---|---|---|
  | `model.safetensors` | 133,466,304 | `3c9f31665447c8911517620762200d2245a2518d6e7208acc78cd9db317e21ad` |
  | `tokenizer.json` | 711,396 | `d241a60d…5c66` |
  | `config.json` | 743 | `094f8e89…4750` |
  | `sentence_bert_config.json` | 52 | `84e39fda68ccbff05bfa723ae9c0e70e23e2ec373b76e0f8c6e71af72a693cbf` |
  | `1_Pooling/config.json` | 190 | `d1caf60c96f5fba2157c0c26b76d80818fad6cf0b8eb5e73ec372ff9818eba5c` |
  
  Changing files changes the model row and its fingerprint; that is fine, since nothing has shipped.

## Risks

- **aikit** is the same single author as goinfer, and fast-moving; pin it exactly. Its BERT forward is old and certified, and it is a small corner of the module.
- **Indexing is about 30% slower** than hugot with SIMD. Batching BERT forwards could close that gap; not tried.
- **The `LoadBERTFromFS` change** has to be accepted upstream or carried in a fork.
- **Not run** on arm64 or macOS, or with a cold cache.

## In the owner fork (2026-11-21)

The owner chose the fork. `LoadBERTFromFS` is now committed locally in `~/Dev/pkieltyka/aikit`, on branch `pbrain/load-bert-fs` from main `b719bbda`. The owner pushed it to github.com/pkieltyka/aikit (branch `pbrain/load-bert-fs`, 2026-11-21). It was then merged into the fork's main through the fork's PR #1, as `eb8c5ddd`, and offered upstream as townsendmerino/aikit#38. The `upstream` remote (`townsendmerino/aikit`) is fetch-only, with push disabled.

- **Commit:** `04b54e2673c45cfb97ffa734ccfd7dc0b70b5f77`, "encoder: LoadBERTFromFS, the BERT loader over an fs.FS".
- **What it adds** (Experimental tier: README, CHANGELOG [Unreleased], `experimentalSyms`):
  - `encoder.LoadBERTFromFS(fsys fs.FS, dir string) (*BERT, error)`. It reads `config.json`, `model.safetensors`, `sentence_bert_config.json`, `1_Pooling/config.json` and `tokenizer.json` through `fsys`, each once and nothing else.
  - `BERT.Tokenizer() *embed.Tokenizer`, the tokenizer that load parsed. The adapter uses it, so `tokenizer.json` is read once; this replaces the "the adapter can load its own tokenizer" option above.
  - `LoadBERT` and `LoadBERTFromFS` now share one body over a small file-source interface. `LoadBERT` behaves as before.
- **Aliasing:** the weights may alias the bytes `fs.ReadFile` returns for `model.safetensors`. The doc comment says so. The adapter's `fs.ReadFileFS` hands out the checked buffers, so it must not modify or reuse them while the model is in use.
- **Tests** (`encoder/bert_fs_test.go`):
  - a seeded BF16 BERT served from an `fstest.MapFS`, under a folder that does not exist on disk, matches `LoadBERT` bit for bit (Embed, Encode, config, pooling, sequence cap). A changed weight is shown to break the comparison;
  - each model file is read exactly once, both through `Open` and through `ReadFile`;
  - missing required files fail with `fs.ErrNotExist`, and missing optional ones fall back;
  - on the real bge-small files (revision `5c38ec7c`), the `fs.FS` load matches `LoadBERT` bit for bit, matches the reference to cosine ≥ 0.9999, gives the reference's token ids through `Tokenizer()`, and reads each file once.
  
  Two mutations turned the tests red, as intended: a second `config.json` read, and a dropped `sentence_bert_config.json`.
- **Checks:** `go run -C tools ./preflight` gave `VERDICT: PASS — 10/10 clean` (1,865 tests passed, 140 skipped for absent assets and opt-ins), with the bge-small files linked in for the run. The `tools` module builds and vets. `-race` and the other CI-only jobs were not run.
