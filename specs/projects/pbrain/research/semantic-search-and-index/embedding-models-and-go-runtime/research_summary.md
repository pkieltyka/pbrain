# Embedding models and running them from Go

## Bottom Line

A local embedding model in a Go binary is feasible, and on pbrain's own text it fixes exactly the failure the owner worries about. On 20 paraphrased questions against the 50 sections of the functional spec, every model tested put the right section in its top 3 at least 12 times. pbrain's all-words rule returned nothing for 19 of the 20. EmbeddingGemma, the owner's first candidate, retrieved best (15 of 20 first, 19 in the top 3), but it is a heavy default for a CLI:
- about 1,100 tokens per second on 4 cores of a current laptop, so 25 to 40 minutes to index 5,000 items;
- 0.65 to 1 s to load per process and 0.7 to 1.1 GB of memory;
- 2,048-token context;
- Gemma Terms of Use obligations if pbrain ships or serves the weights.

EmbeddingGemma 2 (October 2026) fixes the licence (Apache-2.0, ungated) but is weaker on English retrieval and slower. Small MIT/Apache models (bge-small, granite-small-r2, arctic-embed-s) were close on this check (13 and 11 first, 17 and 15 in the top 3), at 8 to 10 times the speed and a tenth of the size. They also run in pure Go, which keeps pbrain one static binary.

Recommendation:
- **Local default:** a small permissive model (bge-small-en-v1.5 first, granite-small-r2 if 8k context matters) on hugot's pure-Go backend, downloaded once with a pinned SHA-256.
- **Opt-in quality model:** EmbeddingGemma v1 (or v2 for code-heavy brains) through llama.cpp loaded by purego.
- **Remote, opt-in:** OpenAI `text-embedding-3-small`.
- Choose the default on the retrieval fixture, not on this 20-query check.

## Key Findings

- **EmbeddingGemma v1** ([card](https://gemma.ai/models/embeddinggemma/model-card-v1/), 2025-09-04): 308M parameters, of which about 200M are a 262k-token vocabulary; 768 dimensions (Matryoshka to 128); 2,048 tokens; prompts `task: search result | query: ` and `title: none | text: `. MTEB(eng, v2) retrieval 55.7. Q4/Q8 cost under 0.4 points, but truncating to 256 dimensions costs 4.3 retrieval points. Files are 278 to 334 MB in GGUF, 197 to 309 MB in ONNX. qmd 2.9.0 uses its Q8_0 GGUF from an ungated mirror.
- **Licence:** the [Gemma Terms of Use](https://gemma.ai/resources/terms/) (2026-04-01) cover v1 "regardless of the source that you obtained it from". Redistribution requires:
  - writing Google's use restrictions into pbrain's own terms;
  - shipping the terms;
  - marking modified files;
  - a NOTICE file.
  
  The `google/` repository is gated and refuses anonymous downloads (HTTP 401). Whether a first-run download by pbrain counts as distribution is a legal question.
- **EmbeddingGemma 2** ([card](https://huggingface.co/google/embeddinggemma-2), launched 2026-10-06): Apache-2.0, not gated, 8,192 tokens, the same prompts, much better at code (MTEB Code 78.68 against 68.76), but English retrieval 51.7 against 55.7. On our text it was the weakest model tested (10 of 20 first) and ran about 800 tokens/s.
- **Measured on an Intel Core Ultra 7 255H, 4 threads** ([cpu-measurements.md](./cpu-measurements.md)):

  | Model and runtime | Time per chunk | Tokens/s |
  |---|---|---|
  | EmbeddingGemma v1 Q8_0, llama.cpp | ~0.3 s per 335-token chunk, ~0.5 s per 500 words | ~1,090 (1,770 at 12 threads) |
  | EmbeddingGemma v1 q4, ONNX Runtime | | ~1,160 |
  | bge-small int8, ONNX Runtime | 41 ms per chunk | ~10,700 |
  | granite-small-r2 int8, ONNX Runtime | 50 ms per chunk | ~8,500 |
  | bge-small fp32, hugot pure Go (16 threads) | 92 ms per chunk | |

  A short query embeds in 2 to 30 ms once the model is loaded.
- **Go runtimes** ([go-runtime-options.md](./go-runtime-options.md)):
  - **hugot's pure-Go backend** (Apache-2.0, v0.8.6, pure-Go tokenizer since 2026-10) runs BERT-class ONNX models with no cgo. It failed on both EmbeddingGemma ONNX graphs in v0.8.6 (int4 and int8 unsupported).
  - **ONNX Runtime** needs cgo and a 25 to 45 MB library per platform, and printed a telemetry message on Linux.
  - **llama.cpp** runs EmbeddingGemma v1 and v2. It loads without cgo through purego (yzma), using about 6 to 8 MB of libraries per platform. It is locked to an exact library version, and yzma has no Intel Mac build.
- **Shipping the model:** `go:embed` allows files up to about 2 GB, but would grow the binary by 35 to 335 MB for every user. Downloading on first use with a pinned hash, as qmd, gortex and Ollama do, is the norm. Air-gapped machines need a configured path instead.
- **Remote options** ([model-comparison.md](./model-comparison.md)):
  - OpenAI text-embedding-3-small: $0.02 per 1M tokens, API data not used for training by default, 30-day abuse logs, zero retention available.
  - Gemini embedding: $0.15 to $0.20. The free tier lets Google use and review content, so paid projects only.
  - Voyage and Cohere: training on your data unless you opt out.
  
  A 5,000-item brain costs about $0.05 to index remotely. Any remote model breaks the spec's "Local" rule unless the person opts in, and personal items must never be sent without their own choice.

## Details

- [cpu-measurements.md](./cpu-measurements.md): set-up, the speed and memory table, the 5,000-item estimates, and the 20-query retrieval check with its caveats.
- [model-comparison.md](./model-comparison.md): EmbeddingGemma v1 and v2 in full, twelve small alternatives with licences and files, the Gemma terms clause by clause, what qmd uses, and remote providers with prices and data terms.
- [go-runtime-options.md](./go-runtime-options.md): pure Go, ONNX Runtime, llama.cpp through purego, and a helper process; model packaging; tokenisers; a recommendation per default-model choice.

## Open Questions / Gaps

- **The model choice needs the retrieval fixture.** My check is 20 queries written by one person who knows the spec; it separates semantic from lexical search, not the models from each other.
- **Not measured:**
  - macOS and arm64;
  - EmbeddingGemma inside a Go program (llama.cpp's own tool and ORT from Python stood in);
  - Qwen3-Embedding-0.6B, arctic-embed-s and harrier-270m.
- **The 300 ms read target:** a model load per command (0.2 s small, 0.65 to 1 s EmbeddingGemma) does not fit it. Whether pbrain keeps a resident process, embeds queries with a small model, or falls back to lexical search until the index is warm is for the index subtopic and the architecture.
- **Legal:** whether downloading Gemma v1 weights on first use makes pbrain a distributor under the Gemma terms.
- **ONNX Runtime telemetry on Linux:** observed, not investigated.
- **Prices not found:** Cohere's embedding prices; OpenAI's batch price for embeddings.

## Sources

- EmbeddingGemma: [model card v1](https://gemma.ai/models/embeddinggemma/model-card-v1/), [paper arXiv 2509.20354](https://arxiv.org/html/2509.20354), [launch blog](https://developers.googleblog.com/en/introducing-embeddinggemma/), [HF repo](https://huggingface.co/google/embeddinggemma-300m) (API checked 2026-11-20); [EmbeddingGemma 2 card](https://huggingface.co/google/embeddinggemma-2), [blog 2026-10-06](https://blog.google/innovation-and-ai/technology/developers-tools/embeddinggemma-2/), [arXiv 2610.37364](https://arxiv.org/abs/2610.37364).
- [Gemma Terms of Use](https://gemma.ai/resources/terms/) (last modified 2026-04-01), [Prohibited Use Policy](https://gemma.ai/resources/prohibited_use_policy).
- Alternatives: model cards linked in model-comparison.md; Qwen3 Embedding paper arXiv 2506.05176; jina-code paper arXiv 2508.21290.
- qmd v2.9.0 `src/llm.ts`, `src/store.ts` ([release 2026-11-18](https://github.com/tobi/qmd/releases/tag/v2.9.0)).
- Remote: [OpenAI your data](https://platform.openai.com/docs/guides/your-data), [Gemini API terms](https://ai.google.dev/gemini-api/terms) and [pricing](https://ai.google.dev/gemini-api/docs/pricing), [Voyage pricing](https://docs.voyageai.com/docs/pricing) and [FAQ](https://docs.voyageai.com/docs/faq), [Cohere data commitments](https://cohere.com/enterprise-data-commitments); all fetched 2026-11-20.
- Runtimes: [hugot](https://github.com/knights-analytics/hugot) v0.8.6, [gomlx](https://github.com/gomlx/gomlx), [yalue/onnxruntime_go](https://github.com/yalue/onnxruntime_go), [yzma](https://github.com/hybridgroup/yzma) v1.33.0, [llama.cpp](https://github.com/ggml-org/llama.cpp) PRs #15798, #16367, #30054 and release b12590, [anyproto/any](https://github.com/anyproto/any) pin comment, [eliben/go-sentencepiece](https://github.com/eliben/go-sentencepiece), [gomlx/go-huggingface](https://github.com/gomlx/go-huggingface).
- Local runs on 2026-11-20: llama.cpp `a83d68b`, ONNX Runtime 1.32.0, hugot v0.8.6 with Go 1.27.3.
