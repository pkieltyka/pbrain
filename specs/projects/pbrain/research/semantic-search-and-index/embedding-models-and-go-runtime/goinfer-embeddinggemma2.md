# goinfer's EmbeddingGemma 2 package: an independent check

Researched 2026-11-20 by @lead-dev-2, at the owner's request. Every measurement was a single run on the machine and with the method of [cpu-measurements.md](./cpu-measurements.md): an Intel Core Ultra 7 255H, Linux x86_64 with AVX2, 4 threads unless stated, the file cache warm. The module was downloaded into a scratch folder, its source was read before anything was built or run, and nothing was installed outside the scratch folder. "Inference:" marks my reading.

## Bottom line

**Not as pbrain's runtime today, neither interim nor default. But it is the best base found for EmbeddingGemma 2 in pure Go: build on it rather than write a library from scratch.**

- **It is correct.** On 22 varied texts its vectors match sentence-transformers' EmbeddingGemma 2 to a worst cosine of 0.9999999999986, with identical token ids, at 768 and at 512 dimensions.
- **It is pure Go.** There is no cgo and no shared library, it cross-compiles for all four targets with `CGO_ENABLED=0`, and it adds about 1 MB to a binary. Its source makes no network calls, reads no environment variable and writes no file.
- **It fails the architecture's acceptance rule** of loading plus one query within 600 ms (architecture 8.7, sha256 96c56856). Load alone takes 1.3 s, and a query about 85 ms, so a `search --semantic` process would spend about 1.4 s before it ranks anything. That also breaks the 1 s hybrid target (spec 16).
- **It is slow to index.** It embeds about 440 tokens/s at 4 threads, against about 670 for llama.cpp's EmbeddingGemma 2 Q8_0 on the same text, so a first index of 5,000 items takes about 1.5 hours.
- **It is a young, single-author project with fast churn:** v0.x, about 2,200 commits in the 15 days since its latest release.

Inference, on what to do next. The load time comes from goinfer widening every bf16 tensor to float32 and copying it onto the Go heap (`model.go:135-147`), not from the model's math. Two contained changes look likely to bring load under the 600 ms rule:
- keep the weights in bf16 and widen them as they are used, which also halves resident memory;
- or memory-map a float32 copy of the text weights without copying it.

Neither was tried here. Making those two changes, in a fork or upstream, and then re-measuring against the 600 ms rule is less work than a new library, and keeps a parity test that already passes. Until the load time is fixed, the architecture's provisional choice stands: bge-small on hugot's pure-Go backend.

## 1. What it is

| | |
|---|---|
| Import path | `github.com/townsendmerino/goinfer/embeddinggemma2`, a package in the module `github.com/townsendmerino/goinfer`, not a module of its own. `go get …/embeddinggemma2@latest` resolves through the module. |
| Version | Latest release v0.24.1, 2026-11-05 (proxy.golang.org `@latest`, commit `7f19e161`). 30 or more release tags since v0.1.0 on 2026-06-04. |
| The package's history | Added 2026-10-06 (`4878652f`), the day EmbeddingGemma 2 was released. It first shipped in a release in v0.24.0 (2026-11-05). Since v0.24.1 it has had comment-only changes. |
| Author and maintainers | One person: 11,624 of 11,700 commits; the other 76 are a CI identity. The repository has `CLAUDE.md`, `AGENTS.md` and `GEMINI.md` files, so it is developed largely with coding agents. 26 stars, 6 forks, 10 open issues (GitHub API, 2026-11-20). |
| Activity | 5,452 commits since 2026-10-20, and 2,179 since v0.24.1 (15 days). Pushed minutes before this check. |
| Licence | MIT, for goinfer and for its dependency `aikit` (same author, v1.76.0, created 2026-05-31). |
| go.mod (v0.24.1) | `go 1.27.0`; requires `github.com/townsendmerino/aikit v1.76.0`, `golang.org/x/sys v0.47.0`, `golang.org/x/text v0.40.0`. |
| Packages compiled in | goinfer: `embeddinggemma2`, `tokenizer`, `multimodal`, `internal/vision`, `internal/audio`. aikit: `embed`, `linalg`, `mmap`, `internal/cursor`. Plus `x/sys/cpu` (amd64 only) and `x/sys/unix` (darwin), and `x/text`'s `transform` and `unicode/norm`. |
| cgo | None. `go list -deps` shows no cgo file in any of these packages, also with `CGO_ENABLED=1`. |
| Assembly | None in goinfer. aikit's `linalg` has 20 `.s` files and `embed` has 2: hand-written AVX2 and FMA kernels for amd64, and NEON for arm64. The 2 `embed` files serve GGUF only and are unused here. On amd64, a CPUID check falls back to pure Go without AVX2. On arm64 the NEON f32 kernels are always used, with no fallback. Under `GOEXPERIMENT=simd` there are simd twins. |

**Safety review of the compiled packages** (source read at v0.24.1; a sub-agent's file-by-file report, spot-checked by grep):

- There is no `net`, `net/http`, `os/exec`, `plugin` or `//go:generate`. No file is created, written or removed. Neither goinfer nor aikit reads an environment variable (x/sys/cpu reads `GODEBUG`, as upstream does).
- **Outside the model folder, it reads only** `/proc/self/auxv` and `/proc/cpuinfo` (CPU features). `/proc/meminfo` is reachable only from a function this path does not call.
- **The `init` functions** compute an audio mel filter bank, and turn CPU features off under opt-in build tags.
- **`unsafe`** is used for zero-copy tensor views (alignment-checked) and in the scalar fallbacks.
- **Two hazards on hostile input:**
  - a `config.json` with a negative `sliding_window` panics (`model.go:416-425`). Inference: pbrain verifies the model's digest before loading, so it would never load such a file.
  - `Tokenize` calls `tokenizer.Encode`, which "recognizes special tokens written literally in the text" and whose doc says "do NOT use this on untrusted content" (`tokenizer/sentencepiece.go:520-529`). A body containing `<eos>` gets the real end token. sentence-transformers does the same, so parity holds (both gave 22 tokens for my test text). Inference: pbrain embeds untrusted bodies, so it should neutralise special-token text before embedding, or use goinfer's `EncodeSegments` path.

The README says "Runs offline: no telemetry, no update check, no account". The network code is in other packages (`pull`, `server`), which this import path does not compile in.

## 2. The model it loads

- **Files.** It reads `config.json` (which must say `embedding_gemma2`, and only its `text_config` is used), `model.safetensors`, `tokenizer.json` and `config_sentence_transformers.json` (for the prompts) from a Hugging Face checkpoint folder. It does not read GGUF.
- **Source.** `google/embeddinggemma-2` on Hugging Face: Apache-2.0, `"gated": false`. I used revision `f9972ac2` (2026-10-28); goinfer's golden test pins an earlier one, `914f7f89`.
- **Size.** `model.safetensors` is 1,488,915,288 bytes, all bf16 (SHA-256 `197a3296…53d79`, equal to the Hugging Face LFS id). It holds the whole multimodal model: the text model has 271 M parameters, the audio tower 305 M and the vision tower 167 M.
- **Only the text tower is read.** The loader reads only `language_model.*` tensors through a read-only memory map. The vision and audio towers stay on disk unless `EnableVision` or `EnableAudio` is called.
- **The download cost.** Even so, the one-time download is the full 1.49 GB, since Google publishes no text-only file. Inference: pbrain could pin a text-only extract, which Apache-2.0 allows redistributing, but then pbrain hosts a file. llama.cpp's GGUF is text-only at 310 MB, but goinfer cannot read GGUF for this model.
- **No quantisation.** Weights are widened to float32 in memory, about 1.08 GB for the text model, and there is no int8 path in this package.
- **The whole pipeline is implemented:**
  - `<bos>`, then the named prompt, then the text, then `<eos>`. The prompts come from the checkpoint's own file: `query` is `task: search result | query: `, and `document` is `title: none | text: `.
  - the 24-layer bidirectional encoder;
  - the model's own projection to 768 (`embedding_projection`);
  - a mean over every token, the prompt included (`include_prompt: true`), then L2 normalisation.
  
  EmbeddingGemma 2 has no `2_Dense` module: its `modules.json` is Transformer, Pooling and Normalize.
- **Long inputs.** An input over 8,192 tokens is refused, never cut.
- **Matryoshka.** The library always returns 768 dimensions. A caller truncates and renormalises. Its docs keep 768 and 512 and drop 256 and 128 after their own test: recall@10 at 0.888× and 0.684× of 768's on 341 sections of their documentation.

## 3. Correctness against the reference

Reference: sentence-transformers 6.3.0 with transformers 5.23.0 and torch 2.15.0 (CPU, float32), from the same checkpoint folder, one text at a time, `prompt_name` `document` or `query`, normalised.

| Texts | Token ids | Worst 1 − cosine, 768 | Worst 1 − cosine, first 512 renormalised |
|---|---|---|---|
| 12 documents: six 300-word engineering windows (398 to 714 tokens), a Go snippet, German, Japanese, the single word "Redis", a 2,294-token text, and a text containing a literal `<eos>` | equal in count for all 12 | 1.4e-12 | 1.3e-12 |
| 10 queries (15 to 22 tokens) | equal for all 10 | below 1.4e-12 | below 1.4e-12 |

The largest difference in any vector component was 2.0e-7. The target was at least 0.999 per text; every text reached 0.9999999999986. That agrees with goinfer's own "Gate 2" claim (worst cosine 0.999999994 on 48 texts against sentence-transformers 6.1.0; `docs/tasks/task-embeddinggemma2.md:199-201`), which I could now reproduce independently.

## 4. Speed and memory

The same 30 windows of the corpus (mean 299 words, 507 EmbeddingGemma 2 tokens each, document prompt included) were run through each runtime, one process per run.

| Runtime | Threads | Load | Per chunk | Tokens/s | Short query (warm) | Peak RSS |
|---|---|---|---|---|---|---|
| goinfer v0.24.1, f32 | 4 | 1.26 to 1.39 s | 1,145 ms | ~440 | 81 to 91 ms; first 90 to 120 ms | 1.87 GB (load and queries); 2.35 GB embedding the 30 chunks; 3.0 GB with a 3,800-token section at 8 threads |
| goinfer v0.24.1, f32 | 16 | 1.26 s | | ~340 | 87 to 159 ms | 2.39 GB |
| llama.cpp `a83d68b`, EmbeddingGemma 2 Q8_0 | 4 | ~1 s | ~760 ms | ~670 | ~16 ms compute (earlier run) | 1.75 GB |
| sentence-transformers (torch, f32) | 4 | | | ~790 (on the 12 documents above; goinfer ~390 on the same) | | |
| hugot pure Go, bge-small-en-v1.5 fp32 | 4 | 0.28 s | 205 ms (cut at 512 tokens) | ~2,400 | 23 ms | 1.31 GB |

- **The whole process** for load, one query and five more queries took 1.92 to 2.02 s of wall time (three runs). Load plus the first query is about 1.4 s.
- **More threads hurt** on this hybrid CPU: 16 threads were slower than 4.
- **Long inputs cost more per token**, from attention: 2,294 tokens took 5.8 s (about 400 tokens/s), and 9 tokens took 60 ms.
- **What 5,000 items cost** (inference, at 507 tokens each, about 2.5 M tokens): about 95 minutes at 4 threads with goinfer, against about 63 with llama.cpp. Passages of about 1,400 code points (architecture 8.7) would be shorter but just as many tokens in total. The per-user cache spares later work trees from repeating this.
- **Cold start is not measured.** It would add the reading of 542 MB of bf16 text weights from disk. I did not drop the page cache, which is a system setting.

## 5. Builds

`CGO_ENABLED=0 go build -trimpath -ldflags='-s -w'` of a small program that loads the encoder and embeds a list of texts:

| Target | Result | Size | Baseline* |
|---|---|---|---|
| linux/amd64 | builds | 3.58 MB | 2.57 MB |
| linux/arm64 | builds | 3.41 MB | |
| darwin/amd64 | builds | 3.66 MB | |
| darwin/arm64 | builds | 3.45 MB | 2.48 MB |

\* The same program without goinfer (JSON, fmt, os). goinfer adds about 1.0 MB. Built with Go 1.27.3, which the module requires (`go 1.27.0`, toolchain `go1.27.3`). Only linux/amd64 was run; the arm64 and darwin binaries were built, not executed.

## 6. The 20-question paraphrase check

The same 50 sections of the functional spec and 20 reworded questions as in cpu-measurements.md. Sections ran from 1 to 3,792 tokens; none needed cutting.

| Model and runtime | Right section first | In top 3 |
|---|---|---|
| EmbeddingGemma 2, goinfer (f32, equal to the reference) | 9 | 12 |
| EmbeddingGemma 2 Q8_0, llama.cpp (earlier run) | 10 | 12 |
| EmbeddingGemma v1 Q8_0, llama.cpp (earlier run) | 15 | 19 |
| bge-small-en-v1.5 int8 (earlier run) | 13 | 17 |

goinfer ranks as EmbeddingGemma 2 does elsewhere; the difference of one first-place hit from Q8_0 is within the noise of 20 questions. The finding of the earlier check stands, and is about the model, not the runtime: on English paraphrases of our own spec, EmbeddingGemma 2 is the weakest of the models tried. The evaluation fixture (architecture 8.7, calibration) should settle this before EmbeddingGemma 2 is confirmed as the default model.

## 7. Risks

- **Maturity and bus factor.** One author, five and a half months old, v0.x, developed at a pace of hundreds of commits a day. `aikit` has the same author, 6 stars, and its own churn: goinfer HEAD already moves to aikit v1.78.
- **API stability.** No compatibility promise. pbrain would pin an exact version, vendor or fork it, and keep its own parity test.
- **Test coverage.**
  - Unit: the package's default tests run a tiny random-weight model against a float64 reference (cosine ≥ 0.9999 at every layer, the sliding mask cell for cell, attention blocking).
  - Real checkpoint: the 48-text comparison against sentence-transformers runs only under the `realckpt` build tag, with the checkpoint on disk.
  - Not checked: whether CI runs the real-checkpoint test.
- **Load time and memory.** Load is 1.3 s with 1.9 GB resident (section 4), against the 600 ms rule. This is the blocking issue.
- **arm64 is untested here.** Its NEON kernels have no pure-Go fallback, and their numbers on Apple silicon are the author's alone (M1 Pro: "1.0 s … at 449 tokens", "loaded in 2 s"; `task-embeddinggemma2.md:188-223`).
- **Surprises:**
  - The package also implements image and audio embedding.
  - Literal special-token text in an input is parsed as a special token (section 1).
  - A config missing `config_sentence_transformers.json` makes every `Encode` fail with "unknown task".
  - The one-time download is the 1.49 GB multimodal file.

## Sources

- **Code:** module source at v0.24.1 from proxy.golang.org (h1 `JGA9cJpT…`), and aikit v1.76.0. The git history is from `github.com/townsendmerino/goinfer` (HEAD `c3e93dad`, 2026-11-20). GitHub REST API for repository metadata, 2026-11-20.
- **Model:** `google/embeddinggemma-2` on Hugging Face, revision `f9972ac2` (API, 2026-11-20): `config.json`, `modules.json`, `config_sentence_transformers.json` and the `model.safetensors` header.
- **goinfer's own docs:** `docs/tasks/task-embeddinggemma2.md`, `embeddinggemma2/encoder.go`, `model.go`, `tokenizer/sentencepiece.go`, and the README.
- **Local runs, 2026-11-20:**
  - a 60-line Go probe that loads, times and dumps vectors;
  - a sentence-transformers reference script;
  - the `eval.py` questions of the earlier check;
  - llama.cpp `llama-embedding`;
  - the earlier hugot harness.
  
  All of it is in this session's scratch folder, not committed.
