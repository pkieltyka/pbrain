# CPU measurements: embedding speed, memory and a small retrieval check

Measured 2026-11-20 by @lead-dev-2 on this machine. Everything ran in a scratch folder; nothing was installed system-wide. "Inference:" marks my reading. Every number is a single run on one machine: treat them as orders of magnitude, not benchmarks.

## Setup

- **Hardware.** Intel Core Ultra 7 255H laptop CPU (16 logical CPUs: performance, efficient and low-power cores), 62 GB RAM, Linux x86_64, AVX2 (no AVX-512).
- **Runtimes.**
  - llama.cpp built from source at commit `a83d68b` (2026-11-20), CPU only, `-DGGML_NATIVE=OFF`, with its `llama-embedding` tool.
  - ONNX Runtime 1.32.0 through its Python package (onnxruntime + HF tokenizers 0.23.3) in a scratch venv. That is the same C++ runtime a Go program would load through cgo, so its speed stands in for Go + ORT.
  - knights-analytics/hugot v0.8.6, pure-Go backend (GoMLX), built with Go 1.27.3 and `GOEXPERIMENT=simd`, `CGO_ENABLED` left at its default but no cgo backend tag. The test program was 38.9 MB.
- **Models** (files from Hugging Face, sizes in the model-comparison doc):
  - EmbeddingGemma 300m (v1): `ggml-org/embeddinggemma-300M-GGUF` Q8_0; `onnx-community/embeddinggemma-300m-ONNX` int8 (`model_quantized`) and q4.
  - EmbeddingGemma 2 (text part): `ggml-org/embeddinggemma-2-GGUF` Q8_0.
  - bge-small-en-v1.5: `Xenova/bge-small-en-v1.5` ONNX int8 and fp32.
  - granite-embedding-small-english-r2: `onnx-community/granite-embedding-small-english-r2-ONNX` int8.
- **Text.** 5,000 random windows of 150 to 450 words (mean 300) cut from the Markdown in `specs/` of this repository and in `../factory`: engineering prose with code spans, close to what pbrain will hold. Documents got the model's document prompt (`title: none | text: ` for EmbeddingGemma), queries its query prompt.

## Speed and memory

Throughput is in tokens per second of the model's own tokenizer; "chunk" is one window of the corpus.

| Model, runtime, threads | Chunk tokens (mean) | ms per chunk | Tokens/s | Query (short, warm) | Load | Peak RSS |
|---|---|---|---|---|---|---|
| EmbeddingGemma v1 Q8_0, llama.cpp, 4 | 335 (200 words) | ~310 | ~1,090 | ~30 ms compute | ~0.65 s | 0.71 GB |
| same, 120-word chunks, 4 | 210 | ~190 | ~1,090 | | | 0.71 GB |
| same, 300-word chunks, 12 | ~520 | ~295 | ~1,770 | | | 1.08 GB |
| same, 300-word chunks, 6 | ~520 | ~340 | ~1,530 | | | 1.08 GB |
| EmbeddingGemma v1 Q4_0, llama.cpp, 4 | ~520 | ~375 | ~1,390 | | | 1.09 GB |
| EmbeddingGemma v1 int8, ORT, 4 | 515 | 500 | ~1,030 | 103 ms | 0.05 s | 2.73 GB |
| EmbeddingGemma v1 int8, ORT, 16 | 520 | 510 | ~1,020 | 215 ms | | 2.73 GB |
| EmbeddingGemma v1 q4, ORT, 4 | 520 | 447 | ~1,160 | 12 ms | 0.16 s | 0.88 GB |
| EmbeddingGemma 2 Q8_0, llama.cpp, 4 | 210 | ~260 | ~800 | ~16 ms compute | ~0.96 s | 0.91 GB |
| bge-small-en-v1.5 int8, ORT, 4 | 442 (cut at 512) | 41 | ~10,700 | 2 ms | 0.10 s | 0.74 GB |
| granite-small-english-r2 int8, ORT, 4 | 427 | 50 | ~8,500 | 3 ms | 0.06 s | 0.72 GB |
| bge-small-en-v1.5 fp32, hugot pure Go, all 16 | ~400 (300 words) | 92 | ~4,300 | 21 ms | 0.22 s | 1.93 GB |

Notes on the table:
- "Load" for llama.cpp is the time from start to the first batch in its log; a one-query run of `llama-embedding` took 0.76 s wall in total for v1 and 1.13 s for v2. A CLI that loads the model per command pays that on every semantic search.
- The ORT peak RSS includes the Python interpreter and tokenizer (inference: a few hundred MB of the figure). The int8 EmbeddingGemma ONNX graph is slow and large in ORT; its q4 graph is the better ORT choice.
- More threads help llama.cpp only modestly on this hybrid CPU (4 → 12 threads: 1.6×), and ORT not at all for EmbeddingGemma int8.
- **hugot's pure-Go backend could not run EmbeddingGemma.** v0.8.6 failed on both ONNX variants: q4 with "unsupported/unknown ONNX data type INT4", int8 with a `DequantizeLinear` shape error. It also failed on bge-small int8 with over-long input (no truncation in my harness). bge-small fp32 ran. I did not try the 1.2 GB fp32 EmbeddingGemma graph.
- llama.cpp aborted bge-small on any input over its 512-token limit, and my batched runs of it were invalid, so bge-small numbers are from ORT and hugot only.
- ONNX Runtime 1.32.0 printed `telemetry_1ds.cc … Failed to persist telemetry device ID; using an in-memory identifier` on every start, on Linux. Inference: this ORT build carries a telemetry component; pbrain's spec forbids telemetry, so ORT's telemetry behaviour must be checked and switched off before ORT could be used. I did not check what, if anything, it sends.

### What that means for 5,000 items

Inference from the rates above, for 5,000 items of about 300 words (about 2.6 million EmbeddingGemma tokens; about 2.2 million BERT tokens with truncation at 512):

| Model and runtime | First full index | Re-embedding one changed item |
|---|---|---|
| EmbeddingGemma v1, llama.cpp, 4 threads | ~40 min | ~0.3 s + model load |
| EmbeddingGemma v1, llama.cpp, 12 threads | ~25 min | |
| EmbeddingGemma 2, llama.cpp, 4 threads | ~55 min | |
| bge-small or granite-small, ORT, 4 threads | ~3.5 to 4 min | ~0.05 s |
| bge-small fp32, pure Go, 16 threads | ~8 min | ~0.1 s |

The plan's 500-word chunk takes about 0.5 s with EmbeddingGemma v1 at 4 threads (824 tokens measured in ORT: 0.5 to 0.63 s) and about 40 to 50 ms with bge-small or granite-small (cut at 512 tokens for bge).

## A small retrieval check on pbrain's own text

Purpose: a sanity check of whether semantic search finds what pbrain's lexical rule misses, on our kind of text. Not a benchmark.

- **Documents:** the 50 sections and subsections of `functional_spec.md` (status complete, 2026-11-20), each a document, cut to the model's input limit (512 tokens for bge-small, 2,048 for EmbeddingGemma v1, 8,192 for v2 and granite).
- **Queries:** 20 questions I wrote in other words than the spec uses ("how long can results be before they get cut off" for 6.1 Budget; "two people superseded the same decision in different ways" for 4.4), each with one to three acceptable sections.
- **Methods:** pbrain's v1 rule (every query word must occur; checked with and without stop words), BM25 over all words (OR), and cosine similarity of each model's embeddings.

| Method | Right section first | Right section in top 3 | Empty results |
|---|---|---|---|
| pbrain v1 rule, all words | 0 | 0 | 19 of 20 |
| BM25, any word | 5 | 11 | 0 |
| bge-small-en-v1.5 int8 (ORT) | 13 | 17 | 0 |
| granite-small-english-r2 int8 (ORT) | 11 | 15 | 0 |
| EmbeddingGemma v1 q4 (ORT) | 15 | 18 | 0 |
| EmbeddingGemma v1 Q8_0 (llama.cpp) | 15 | 19 | 0 |
| EmbeddingGemma 2 Q8_0 (llama.cpp) | 10 | 12 | 0 |

Caveats, all material:
- One author wrote the queries knowing the spec; 20 queries cannot separate models that differ by two hits. The gap between lexical and semantic is large enough to stand; the order among the models is not.
- The v1 rule's result is a worst case: the queries are whole sentences, and an agent would pick two or three words and retry. It still shows the failure the owner worries about: a paraphrase finds nothing.
- Whole sections are long; bge-small saw only their first 512 tokens. A chunked index would treat it better.
- EmbeddingGemma 2's weaker result agrees with Google's own English retrieval numbers (MTEB English v2 retrieval 51.7 for v2 against 55.7 for v1; model-comparison doc).

## Reproduce

The scripts (`bench_ort.py`, `eval.py`, `sc.py`, the hugot `main.go`) and the corpus builder were in this session's scratch folder and are not committed. The commands are plain: `llama-embedding -m <gguf> -f <prompts> --embd-separator '<#sep#>' -t <threads> -c <n> -b <n> -ub <n>`, an ORT session with `intra_op_num_threads`, and hugot's `NewGoSession` with a feature-extraction pipeline. A proper harness belongs in the evaluation fixture the retrieval review asked for.
