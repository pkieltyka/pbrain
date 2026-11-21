# Embedding models for pbrain: local candidates, licences and remote options

Researched 2026-11-20 by @lead-dev-2. Sources are model cards, Hugging Face API records, papers and vendor pages, fetched on that date; scores are the vendors' own unless stated. "Inference:" marks my reading. Speed and memory measured on this machine are in [cpu-measurements.md](./cpu-measurements.md).

A note on score names: "MTEB(eng, v2) Retrieval" (10 tasks) and the older "MTEB Retrieval" (15 BEIR tasks) are different suites and are not comparable with each other.

## 1. EmbeddingGemma 300m (v1), the owner's first candidate

| Fact | Value | Source |
|---|---|---|
| Released | 2025-09-04 | [launch blog](https://developers.googleblog.com/en/introducing-embeddinggemma/) |
| Size | 308M parameters: "roughly 100M model parameters and 200M embedding parameters" (blog) | blog; [HF repo](https://huggingface.co/google/embeddinggemma-300m) |
| Architecture | Gemma 3 turned into a bidirectional encoder, mean pooling, two dense layers to 768 | [model card](https://gemma.ai/models/embeddinggemma/model-card-v1/) |
| Dimensions | 768, truncatable by Matryoshka to 512, 256 or 128, then re-normalised | model card |
| Context | 2,048 tokens | model card; `max_position_embeddings: 2048` |
| Tokenizer | Gemma 3 tokenizer, vocabulary 262,144; ships `tokenizer.model` (SentencePiece) and `tokenizer.json` | [ONNX config](https://huggingface.co/onnx-community/embeddinggemma-300m-ONNX/raw/main/config.json) |
| Prompts | query `task: search result \| query: `; document `title: {title or "none"} \| text: ` | model card, "Prompt Instructions" |
| Licence | Gemma Terms of Use (section 4 below); HF repo gated ("manual") | HF API, 2026-11-20 |

Scores (model card; retrieval sub-scores from the [paper](https://arxiv.org/html/2509.20354), tables 5 to 7):

| Dimensions | MTEB(Multilingual, v2) mean | MTEB(eng, v2) mean | MTEB(Code) mean | MTEB(eng, v2) Retrieval |
|---|---|---|---|---|
| 768 | 61.15 | 69.67 | 68.76 | 55.7 |
| 512 | 60.71 | 69.18 | 68.48 | 54.0 |
| 256 | 59.68 | 68.37 | 66.74 | 51.4 |
| 128 | 58.23 | 66.66 | 62.96 | 46.0 |

- **Quantisation costs little**: Q8_0 69.49 and Q4_0 69.31 on MTEB(eng, v2) against 69.67 in full precision (model card, quantisation-aware checkpoints).
- **Truncation costs more on retrieval than the means suggest**: English retrieval falls 4.3 points at 256 dimensions and 9.7 at 128 (paper). Inference: keep 768, or 512 at most, for search.
- **Files** (HF API): GGUF Q8_0 333.6 MB (`ggml-org/embeddinggemma-300M-GGUF`, ungated, what qmd downloads), Q4_0 277.9 MB (unsloth); ONNX int8 309 MB and q4 197 MB of weights (`onnx-community`). The large vocabulary keeps even the q4 forms near 200 to 280 MB.
- The ONNX card warns: "EmbeddingGemma activations do not support `fp16`".

## 2. EmbeddingGemma 2: newer, Apache-2.0, weaker on English retrieval

- `google/embeddinggemma-2`: HF repo created 2026-09-14, licence `apache-2.0`, **not gated** (HF API, checked 2026-11-20); launch blog [2026-10-06](https://blog.google/innovation-and-ai/technology/developers-tools/embeddinggemma-2/); report [arXiv 2610.37364](https://arxiv.org/abs/2610.37364). The card's licence links to the Gemma 4 licence page, which serves Apache 2.0. The Gemma Terms of Use appendix lists "EmbeddingGemma" and not EmbeddingGemma 2 (fetched 2026-11-20).
- Multimodal: 740M parameters in total; the text part is 270M ("130M transformer + 140M embedder"). 768 dimensions (Matryoshka 512/256/128), **8,192-token context**, same prompts as v1 ([card](https://huggingface.co/google/embeddinggemma-2), `config_sentence_transformers.json`).
- Card: "Do not use `float16` … the model returns NaN or silently degraded embeddings rather than raising an error".
- Scores: MTEB(eng, v2) mean 68.46, MTEB(Code) **78.68** (v1 68.76), but MTEB(eng, v2) **Retrieval 51.7** against v1's 55.7, and multilingual retrieval 60.8 against 62.5 (card and paper tables).
- Text-only files: GGUF Q8_0 309.9 MB (`ggml-org/embeddinggemma-2-GGUF`), UD-Q4_K_XL 175.7 MB (unsloth); ONNX q4 174 MB.
- On our own text it found the right spec section first 10 times in 20, against 15 for v1, and ran about 25% slower with a 1 s load ([cpu-measurements.md](./cpu-measurements.md)).

Inference: v2 solves the licence and gating problem and is better for code; for English engineering prose, v1 retrieves better. qmd 2.9.0 still uses v1.

## 3. Small alternatives

| Model | Params | Dims | Context | Licence | Reported retrieval | Files |
|---|---|---|---|---|---|---|
| [bge-small-en-v1.5](https://huggingface.co/BAAI/bge-small-en-v1.5) | 33M | 384 | 512 | MIT | MTEB Retrieval(15) 51.68 | GGUF Q8_0 37 MB; ONNX int8 34 MB, fp32 133 MB |
| [bge-base-en-v1.5](https://huggingface.co/BAAI/bge-base-en-v1.5) | 109M | 768 | 512 | MIT | Retrieval(15) 53.25; eng v2 Retrieval 54.8 | GGUF Q8_0 118 MB |
| [gte-small](https://huggingface.co/thenlper/gte-small) | 33M | 384 | 512 | MIT | Retrieval(15) 49.46 | GGUF Q8_0 37 MB |
| [granite-embedding-small-english-r2](https://huggingface.co/ibm-granite/granite-embedding-small-english-r2) | 47M | 384 | 8,192 | Apache-2.0 | MTEB-v2 Retrieval 53.9; CoIR 53.8 | ONNX int8 52 MB |
| [snowflake-arctic-embed-s](https://huggingface.co/Snowflake/snowflake-arctic-embed-s) | 33M | 384 | 512 | Apache-2.0 | MTEB Retrieval 51.98 | ONNX int8 34 MB |
| [arctic-embed-m-v1.5](https://huggingface.co/Snowflake/snowflake-arctic-embed-m-v1.5) | 109M | 768 (256) | 512 | Apache-2.0 | MTEB Retrieval 55.14 | GGUF Q8_0 118 MB |
| [nomic-embed-text-v1.5](https://huggingface.co/nomic-ai/nomic-embed-text-v1.5) | 137M | 768 (to 64) | 8,192 | Apache-2.0 | MTEB mean 62.28 | GGUF Q8_0 146 MB |
| [gte-modernbert-base](https://huggingface.co/Alibaba-NLP/gte-modernbert-base) | 149M | 768 | 8,192 | Apache-2.0 | BEIR 55.33; CoIR 79.31 | ONNX int8 150 MB |
| [Qwen3-Embedding-0.6B](https://huggingface.co/Qwen/Qwen3-Embedding-0.6B) | 0.6B | 1024 (MRL) | 32K | Apache-2.0 | eng v2 Retrieval 61.83; MTEB(Code) 75.41 | GGUF Q8_0 639 MB |
| [harrier-oss-v1-270m](https://huggingface.co/microsoft/harrier-oss-v1-270m) (2026-03) | 270M | 640 | 32,768 | MIT | multilingual Retrieval 66.4 (EG2 paper table) | files not checked |
| [all-MiniLM-L6-v2](https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2) | 22.7M | 384 | 256 word pieces | Apache-2.0 | Retrieval(15) 41.95 | ONNX int8 23 MB |
| [mxbai-embed-xsmall-v1](https://huggingface.co/mixedbread-ai/mxbai-embed-xsmall-v1) | 24M | 384 | 4,096 | Apache-2.0 | not on card | GGUF Q8_0 31 MB |

- **Not usable commercially:** jina-embeddings-v3, jina-code-embeddings and jina v5 are CC-BY-NC-4.0 on HF ("For commercial usage inquiries, feel free to contact us"), however strong (jina-embeddings-v5-text-nano: eng v2 Retrieval 58.8). LFM2.5-Embedding and Nemotron embedding models use non-OSI licences.
- **Prefixes differ by model** (bge: `Represent this sentence for searching relevant passages: ` on queries; nomic: `search_query: `/`search_document: `; Qwen3: an `Instruct:` line on queries). The index must record which model and prompts made its vectors.
- On our text: bge-small found the right section first 13 times in 20 (17 in the top 3), granite-small 11 (15); EmbeddingGemma v1 15 (19). With 20 queries the small models are not clearly worse; they are 8 to 10 times faster and a tenth of the size.

Inference: for English engineering text the permissively licensed small models (bge-small, arctic-embed-s, granite-small-r2) are the practical default; Qwen3-Embedding-0.6B is the strongest permissive option if size and speed allow, at about twice EmbeddingGemma's size.

## 4. The Gemma Terms of Use for v1 (redistribution)

[Gemma Terms of Use](https://gemma.ai/resources/terms/), "Last modified: April 1, 2026"; the appendix lists EmbeddingGemma.
- 1.1: "'Gemma' means the set of machine learning language models, trained model weights and parameters identified in the Appendix, regardless of the source that you obtained it from." So a GGUF copy from `ggml-org` is still Gemma. Inference: a quantised file is a "Model Derivative".
- 3.1: redistribution is allowed if you (1) "include the use restrictions referenced in Section 3.2 as an enforceable provision in any agreement … governing the use and/or distribution" and notify users of them; (2) "provide all third party recipients … a copy of this Agreement"; (3) mark modified files; (4) ship a "Notice" text file: "Gemma is provided under and subject to the Gemma Terms of Use found at gemma.ai/resources/terms".
- 3.2: use must follow the [Prohibited Use Policy](https://gemma.ai/resources/prohibited_use_policy); "Google reserves the right to restrict (remotely or otherwise) usage".
- Acceptance is by use: "By using, reproducing, modifying, distributing … you agree to be bound by this Agreement."

Inference: shipping v1 weights inside pbrain, or serving them from pbrain's own release, makes pbrain a distributor with conditions (1) to (4), including writing Google's use restrictions into pbrain's terms. Having pbrain download v1 from a third-party mirror on first use avoids pbrain shipping the weights, but each user still accepts the Gemma terms by using them. Whether pbrain's downloader "distributes" is a legal question I could not settle from primary sources. The `google/` repository itself refuses anonymous downloads (HTTP 401 `GatedRepo`, checked 2026-11-20); HF says access "can only be done from your browser", so a CLI could only use mirrors.

EmbeddingGemma 2 (Apache-2.0) and every model in section 3 except the jina and LFM ones carry none of this.

## 5. What qmd uses

qmd v2.9.0 (released 2026-11-18, MIT), `src/llm.ts`: embedding `hf:ggml-org/embeddinggemma-300M-GGUF/embeddinggemma-300M-Q8_0.gguf` (v1, from the ungated mirror); reranker `Qwen3-Reranker-0.6B` Q8_0 (639 MB); query expansion a fine-tuned Qwen3 1.7B at Q4_K_M (1.28 GB). Prompts are Google's (`task: search result | query: …`, `title: … | text: …`). Chunks are "900 tokens per chunk with 15% overlap". A source comment measures about 150 MB per 2,048-token embedding context for EmbeddingGemma. Models are "auto-downloaded on first use" into `~/.cache/qmd/models/`. Details of qmd's engine are @lead-dev's subtopic.

## 6. Remote embedding options (opt-in)

| Provider and model | Price per 1M tokens | Dims | Max input | Data use (quoted terms) |
|---|---|---|---|---|
| OpenAI `text-embedding-3-small` / `-large` | $0.02 / $0.13 | 1536 / 3072, shortenable | 8,192 | "data sent to the OpenAI API is not used to train or improve OpenAI models (unless you explicitly opt in)"; `/v1/embeddings`: abuse-monitoring logs up to 30 days; Zero Data Retention eligible ([your data](https://platform.openai.com/docs/guides/your-data)) |
| Google `gemini-embedding-001` / `gemini-embedding-2` | $0.15 / $0.20 (batch half) | 128 to 3072 | 2,048 / 8,192 | Free tier: Google "uses the content you submit … to provide, improve, and develop" products, "human reviewers may read" it, "Do not submit sensitive, confidential, or personal information". Paid (billing-enabled Cloud project): prompts not used to improve products; logged for a limited time for abuse ([terms](https://ai.google.dev/gemini-api/terms), effective 2026-03-23) |
| Voyage `voyage-4-lite` / `voyage-4` / `voyage-code-3` | $0.02 / $0.06 / $0.18 | 256 to 2048 | 32,000 | Training use unless an admin with a payment method opts out ([FAQ](https://docs.voyageai.com/docs/faq)); first 200M tokens free |
| Cohere `embed-v4.0` / v5 | not published on the pages fetched | 256 to 2048 | 128k | Opt-out of training in dashboard; logs deleted after 30 days ([data commitments](https://cohere.com/enterprise-data-commitments)) |

- Quality: on MTEB(Code), voyage-code-3 79.84 and gemini-embedding-001 76.48 (jina-code paper table); OpenAI's own MTEB average for 3-small is 62.3 (older suite).
- Inference: **OpenAI `text-embedding-3-small`** is the sensible remote default: cheapest, no training use by default, 30-day abuse logs with zero retention available, an OpenAI-compatible API that most local servers also speak (so one client covers both). Gemini only on a paid project, never its free tier. Voyage and Cohere need an explicit opt-out first. A 5,000-item brain is about 2.5 million tokens: about $0.05 to index with 3-small.
- A remote model sends item text off the machine, against the spec's rule 5 ("Local") unless the person opts in; personal items must never go to it without the person's own choice.
