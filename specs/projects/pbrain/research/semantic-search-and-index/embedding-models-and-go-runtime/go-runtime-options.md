# Running an embedding model from a single Go binary

Researched 2026-11-20 by @lead-dev-2. Sources are repositories (READMEs, code, changelogs, release assets via `gh`), the Go module proxy and Hugging Face; library facts were gathered with the help of a research sub-agent and the load-bearing ones re-checked by hand. Measurements are in [cpu-measurements.md](./cpu-measurements.md). "Inference:" marks my reading.

pbrain's constraints: one Go binary, Linux and macOS on amd64 and arm64, offline after set-up, CPU only, no telemetry, no service.

## 1. The four routes

| Route | cgo | What ships besides the binary | EmbeddingGemma | Speed seen here | Main risk |
|---|---|---|---|---|---|
| Pure Go (hugot "Go" backend on GoMLX) | no | the model files | **does not load** in hugot v0.8.6 | bge-small fp32: 92 ms per ~400-token chunk, 16 threads | op coverage; memory (1.9 GB peak) |
| ONNX Runtime (official or yalue Go bindings) | yes | `libonnxruntime` 25 to 45 MB per platform | yes (ORT 1.32, q4 graph) | EG v1 q4: 447 ms per 520 tokens; bge-small int8: 41 ms | cgo cross-compiling; telemetry component; library size |
| llama.cpp through purego (yzma, gollama.cpp, kelindar/search) | no | llama.cpp shared libraries, ~6 to 8 MB per platform | yes, since 2025-09 | EG v1 Q8_0: ~1,090 tokens/s at 4 threads | exact version lock between Go bindings and the library |
| Helper process (`llama-server`, as Ollama does) | no | a server binary, 12 to 23 MB per platform | yes | as llama.cpp | a second process, a localhost port, lifecycle |

### Pure Go

- [knights-analytics/hugot](https://github.com/knights-analytics/hugot) (Apache-2.0, v0.8.6 released 2026-11-11): "The native Go backend is always available, as it does not require any cgo dependencies. It is provided by the GoMLX project. … enabling GOEXPERIMENT=simd gives this backend a significant performance boost." Also: "The Go backend is designed for simpler workloads … and for smaller models such as all-MiniLM-L6-v2." ORT and XLA backends sit behind build tags, so a default build is cgo-free. Since v0.8.1 (2026-10-02) its tokenizer is pure Go ("Removed the Rust tokenizer dependency"), using gomlx/go-huggingface's `hftokenizer`, which parses EmbeddingGemma's `tokenizer.json` format.
- Measured: a 38.9 MB test binary embedded bge-small fp32 at 92 ms per chunk with all 16 CPUs (about 2.2× slower than ORT int8 on 4), 21 ms per short query, 0.22 s load, 1.93 GB peak memory with batches of 8. The int8 bge graph and both EmbeddingGemma graphs failed (`unsupported/unknown ONNX data type INT4`; a `DequantizeLinear` shape error).
- EmbeddingGemma 2 text-only loading was fixed in hugot's development branch on 2026-11-17 (issue #158) and is not released.
- The GoMLX author's spreadsheet (MiniLM, 2026-11-03) reports the pure-Go SIMD backend at 19.2K tokens/s against ONNX Runtime's 12.9K on an AVX2 desktop. That ORT figure was run through GoMLX's own ONNX bridge, and I found no independent comparison. My measurement on a laptop was slower than ORT.
- In production: zzet/gortex (about 2.1k stars) and sammcj/mcp-devtools use hugot's Go backend with MiniLM, downloading the model on first use.
- rcarmo/memento (MIT) has a hand-written pure-Go EmbeddingGemma 2 engine that reads GGUF, off by default. Its own benchmark on an SSE2-only Celeron: 2.1 s per query against 0.16 s for GTE-small. Inference: feasible, but a bespoke inference engine is not something pbrain should own.

### ONNX Runtime

- Two Go bindings, both cgo plus `dlopen` of a shared library: [yalue/onnxruntime_go](https://github.com/yalue/onnxruntime_go) (MIT; "you'll need a version of Go with cgo support") and the new official `github.com/microsoft/onnxruntime/go` (on the module proxy, 2026-11-17), which hugot moved to in v0.7.9.
- Library sizes (ORT 1.31): linux/arm64 25.3 MB, darwin/arm64 44.8 MB, linux/x64 29.3 MB (copies redistributed by yalue and hugot; the official release page was unreachable from this sandbox). No supported way to link ORT statically into a consumer was found.
- Cross-compiling cgo for macOS from Linux needs osxcross with an SDK extracted from Xcode (goreleaser-cross packages it); the usual alternative is native macOS CI runners.
- ORT 1.32 printed a 1DS telemetry message at start-up on Linux (cpu-measurements). Inference: must be verified and disabled before use, given pbrain's "no telemetry" rule.

### llama.cpp through purego

- llama.cpp has supported EmbeddingGemma since PR #15798 (merged 2025-09-04), its dense projection layers since #16367 (2025-10-09), and EmbeddingGemma 2 since #30054 (2026-10-06). It L2-normalises by default; the caller adds the task prompts.
- [hybridgroup/yzma](https://github.com/hybridgroup/yzma) (Apache-2.0, v1.33.0, 2026-11-13): "yzma uses the purego and ffi packages so CGo is not needed." Its platform table lists Linux amd64/arm64, **macOS arm64 only**, Windows amd64. An embeddings example sets `Embeddings = 1` and reads `GetEmbeddingsSeq`.
- The version lock is real: anyproto/any pins llama.cpp `b11600` with the comment "too old a build fails to load ('undefined symbol'), too new a one loads but reads shifted struct fields (llama_model_n_embd comes back 0 and context creation aborts)."
- Minimal CPU library sets in the official b12590 release (2026-11-20): ~8.1 MB linux/x64 with one CPU variant (27.5 MB with all 14 microarchitecture variants), ~7.3 MB linux/arm64, ~5.7 MB macOS arm64, ~6.7 MB macOS x64. A custom single-variant build is smaller.
- Inference: with purego, the libraries can be embedded with `go:embed` and written to a cache folder on first use, then opened. `CGO_ENABLED=0` cross-compiles to all four targets. macOS amd64 is not covered by yzma; gollama.cpp or kelindar/search use the same purego pattern but are smaller projects (kelindar/search ships no macOS build).
- Not suitable: go-skynet/go-llama.cpp (last commit 2024-03, predates EmbeddingGemma); Ollama's `llama/` is no longer an importable package, Ollama runs `llama-server` as a subprocess.

### Helper process

Ollama, docker-agent (through Docker Model Runner) and rcarmo/memento (a short-lived worker) run the model in another process. Inference: this isolates memory and crashes and lets one warm model serve several agents, but adds a process to manage and, for a server, a localhost port; it is not a single file.

## 2. Getting the model onto the machine

- **Embedding in the binary** works within Go's limit (`MaxSymSize = 2e9`, about 2 GB per embedded file), and go-huggingface's `hub.NewEmbed` loads a model from an embedded file system. But pbrain's binary would grow by the model: about 35 to 50 MB for a small int8 model, 130 MB for bge-small fp32 (what hugot's Go backend needs), 175 to 335 MB for EmbeddingGemma. Every user pays that even with semantic search off.
- **Download on first use** is the common pattern (qmd, gortex, mcp-devtools, Ollama): a pinned URL and SHA-256, a cache folder such as `~/.cache/pbrain/models`, offline afterwards. It needs network once, and the person must be told what is fetched and how big it is. For air-gapped machines, a path to model files given in configuration.
- EmbeddingGemma v1 can only come from third-party mirrors, since `google/` is gated, and carries the Gemma terms; EmbeddingGemma 2 and the MIT or Apache models do not (model-comparison doc).

## 3. Tokenising in Go

- With llama.cpp, the GGUF carries the vocabulary and llama.cpp tokenises: nothing to do in Go.
- With ONNX models, pure-Go tokenisers exist: gomlx/go-huggingface (`tokenizer.json`, including EmbeddingGemma's merges format; used by hugot) and eliben/go-sentencepiece (Gemma's SentencePiece `tokenizer.model`, "extensively tested vs. the SentencePiece Python bindings"). daulet/tokenizers wraps the Rust library through cgo and is no longer needed.

## 4. Recommendation

Inference, from the above and the measurements:

1. **If the default model is a small BERT-class model (bge-small, arctic-embed-s, granite-small-r2):** use hugot's pure-Go backend. It keeps one static, cgo-free binary for all four targets, its speed is adequate (about 8 minutes for a first index of 5,000 items, about 0.1 s per changed item, 21 ms per query) and it has real users. Costs: fp32 model files (about 130 MB for bge-small), high peak memory that batching must bound, and a spike to confirm the chosen model loads (bge-small fp32 did; int8 did not).
2. **If the default must be EmbeddingGemma:** use llama.cpp through purego (yzma-style), with the shared libraries embedded and unpacked on first run, and a pinned llama.cpp build in CI. Costs: about 6 to 8 MB of libraries per platform, the version lock, no yzma support for Intel Macs, about 0.65 to 1 s to load the model per process, 0.7 to 1.1 GB of memory, and 25 to 55 minutes to index 5,000 items on a laptop.
3. **Avoid ONNX Runtime** for pbrain: it needs cgo and a 25 to 45 MB library per platform, brings a telemetry component, and was not faster than llama.cpp for EmbeddingGemma here.
4. **Whatever the runtime, keep the model out of the read path's critical latency.** A per-command model load (0.2 s small, 0.65 to 1 s EmbeddingGemma) breaks the spec's 300 ms read target. Options for the architecture: embed only the query at search time with a small model; or keep a resident process; or fall back to lexical search when the index is not ready. That is @lead-dev's index subtopic and the architecture's call.

## Gaps

- No benchmark of EmbeddingGemma on any Go runtime; mine used llama.cpp's own tool and ORT from Python.
- Not run on macOS or on arm64.
- No independent comparison of GoMLX's pure-Go backend with native ORT on the same model and machine, beyond my one laptop run.
- ORT's telemetry behaviour on Linux not investigated.
- Whether onnx-community's EmbeddingGemma graphs will load in a later hugot release.
