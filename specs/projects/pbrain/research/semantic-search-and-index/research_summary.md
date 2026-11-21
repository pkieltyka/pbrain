# Research: Built-in semantic search and index for pbrain

## Bottom Line

Semantic search can be built into pbrain as one Go binary with no cgo, and on our own text it fixes the failure the owner expects from keyword search. On 20 questions worded differently from the functional spec, pbrain's all-words rule found nothing for 19, while every embedding model tested put the right section in its top three at least 12 times. The recommendation is to add it to v1 as a second, opt-in search mode, built last and released only when the evaluation fixture shows it helps and can still return nothing for an unrelated question. Lexical search stays the default and the reference. The model that fits a command-line tool is a small permissively licensed one run in pure Go, not EmbeddingGemma, which retrieved best but takes 25 to 40 minutes to index 5,000 items and up to a second to load on every command. The useful idea in PageIndex needs no model at all: an outline of a long document, read first, then one section fetched by position; that part can ship whether or not semantic search does. Nothing here was measured end to end: no run combined a model, an index and the command line, and nothing ran on macOS.

## Key Findings

- **The paraphrase failure is real and large.** All-words matching returned nothing for 19 of 20 paraphrased questions; matching any word (BM25) put the right section first 5 times; small embedding models 11 to 13 times; EmbeddingGemma v1 15 times. The questions were whole sentences written by one person who knew the spec, so this separates lexical from semantic search, not the models from each other. ([Embedding models and Go runtimes](./embedding-models-and-go-runtime/research_summary.md))
- **EmbeddingGemma is the best retriever and a heavy default.** About 1,100 tokens per second on four laptop cores, 0.65 to 1 s to load per process, 0.7 to 1.1 GB of memory, and Gemma's terms of use apply if pbrain ships the weights. EmbeddingGemma 2 is Apache-licensed but weaker on English retrieval and slower. ([Embedding models and Go runtimes](./embedding-models-and-go-runtime/research_summary.md))
- **Small models are close, fast and run in pure Go.** bge-small (MIT) and granite-small-r2 (Apache) run at 8 to 10 times the speed and a tenth of the size. hugot's pure-Go backend ran bge-small at 92 ms per chunk, 21 ms per query and 0.22 s to load, with a peak of 1.9 GB of memory while indexing; it could not load EmbeddingGemma. ONNX Runtime needs cgo and a 25 to 45 MB library per platform, and printed a telemetry notice. ([Embedding models and Go runtimes](./embedding-models-and-go-runtime/research_summary.md))
- **The index is the easy part.** A pure-Go SQLite build with a bundled vector extension compiled for all four targets. Exact search over 5,000 vectors of 768 dimensions took 8.75 ms at the 95th percentile, and 60 ms over 50,000, so approximate search is not needed. ([qmd and the Go index engine](./qmd-and-go-index-engine/research_summary.md))
- **A vector search always returns something.** It has a nearest item for any question, so "no result" needs an absolute similarity cut-off, calibrated per model on questions that should find nothing. Rank fusion orders candidates; it cannot say whether any of them is relevant. ([qmd and the Go index engine](./qmd-and-go-index-engine/research_summary.md))
- **qmd is a working reference, not a template.** It fuses keyword and vector ranks, chunks at about 900 tokens with 15% overlap, and can add a query-expansion model and a reranker. No evidence was found that the reranker pays for itself on engineering notes. ([qmd and the Go index engine](./qmd-and-go-index-engine/research_summary.md))
- **PageIndex's Markdown mode is headings, with no model.** Its retrieval is an LLM agent walking the outline, which the calling agent already does. Its 98.7% FinanceBench figure is the vendor's own, on an older system, with no independent reproduction. On our 86,000-character spec an outline is 1,874 characters, about 2% of the text. ([Long documents, PageIndex and references](./long-documents-and-pageindex/research_summary.md))
- **Model-based document methods cost a model call per chunk or section** (Contextual Retrieval, section summaries, RAPTOR, GraphRAG) and do not fit pbrain. Links should stay explicit and be shown, never followed automatically. ([Long documents, PageIndex and references](./long-documents-and-pageindex/research_summary.md))

## Implications

What follows is the summary writer's recommendation, built on the findings above.

### Recommended design

1. **Two modes.** Lexical search is unchanged and stays the default. A hybrid mode is asked for explicitly. It returns the exact lexical hits first, in today's order, then items found only by meaning, each marked as such. A semantic-only item must pass the calibrated cut-off, so an empty result stays possible. No query-expansion model and no reranker.
2. **One small local model**, bge-small or granite-small-r2, chosen on the evaluation fixture, run through hugot's pure-Go backend. It is downloaded once, with a pinned hash, into a per-user cache, never committed and never built into the binary. EmbeddingGemma and any remote model are later options.
3. **A derived index per section**, in that section's `.pbrain/`, so nothing personal enters a shared index. It holds vectors for chunks: a heading section, split near 400 tokens when larger, with one best passage per item in results. A fingerprint of the model and the chunking starts a new index when either changes.
4. **Search never waits for a build.** An explicit command builds the index and can resume. When the index is missing or stale, hybrid mode returns the lexical results and says the semantic part was not ready.
5. **Long documents, model-free:** `get <id> --outline` lists a long item's sections with positions and sizes, and a search passage names its section.

### Where this changes the group's earlier answer

The joint retrieval paper (`reviews/retrieval/joint.md`) recommended lexical search only in v1, with semantic search "later, on evidence". Two things have changed. The miss on paraphrases is now measured on our own text, and the cost is lower than assumed: a model about a tenth the size of the ones weighed then, no cgo and no service. So semantic search moves from "later" to "in v1, opt-in, last". What stands: lexical is the default, no near-miss is returned, the fixture decides, the ranking of lexical hits is not changed, and the tag vocabulary and best-passage snippet are unaffected.

### Changes to the functional spec

- **Rule 5 of section 1 and section 16:** pbrain calls no generative model; it may run a local embedding model. Network use gains one item, the one-time model download.
- **Section 7.1:** the hybrid mode, the marker on semantic-only results, the cut-off, the "index not ready" notice, and the section named with a passage.
- **Section 7.2:** `get --outline`.
- **A new command** to build and inspect the index, and configuration naming the model.
- **Sections 5 and 9.3:** the index's personal boundary, and that deleting an index removes semantic additions until it is rebuilt, while lexical answers stay identical.
- **Section 16:** a separate speed target for hybrid search.
- **Section 17:** semantic search leaves "under research"; rerankers, query expansion, remote embeddings and EmbeddingGemma are named as later.
- **Section 15:** scenarios for a paraphrase found only in hybrid mode, an unrelated question that returns nothing, no personal text in a shared index, and a missing index.

### Changes to the architecture

- **Section 8.7** becomes a design: chunker, embedder, vector store, index life cycle and the cut-off.
- **Question 2 (cgo):** stays "none" with a pure-Go SQLite and hugot. It reopens only if EmbeddingGemma is wanted, which needs llama.cpp libraries beside the binary.
- **Speed:** a model load of about 0.2 s per command, plus the query embedding, does not fit the 300 ms read target, and pbrain has no resident process. Hybrid search needs its own target.
- **Life cycle:** the write step that does nothing today updates the index; a corrupt or stale index falls back to the scan.
- **Size and memory:** the binary and its dependencies grow, and indexing peaked at 1.9 GB in the one measurement.

### Decisions for the owner

1. **Semantic search in v1?** Recommended: yes, as an opt-in hybrid mode delivered last and released on the fixture.
2. **Which model by default?** Recommended: a small permissive model, not EmbeddingGemma; the fixture picks between bge-small and granite-small-r2.
3. **Accept a one-time model download?** Recommended: yes, pinned by hash, started only by the explicit build command, with a local path for machines without network access.
4. **Remote embeddings?** Recommended: not in v1. They send text off the machine and would need a separate choice for personal items.
5. **How do the two result sets combine?** Recommended: lexical hits first and untouched, semantic-only items after, marked.
6. **A speed target for hybrid search?** Recommended: one second at 5,000 items; lexical stays at 300 ms.
7. **Outline for long items in v1?** Recommended: yes, with the section named in the passage; section-precise citation syntax later.
8. **Who turns it on?** Recommended: the project names the model in `pbrain.toml`, and each person builds their own index.

## Conflicts and Uncertainty

- **Chunk size and overlap.** The index study proposes 384 to 640 tokens with 10 to 15% overlap; the long-document study proposes about 400 tokens with none, citing a study where overlap scored lowest; qmd uses 900 with 15%. The evidence leans to small chunks without overlap, and bge-small's 512-token limit caps the size anyway. The fixture should settle it.
- **Default model against the owner's first candidate.** EmbeddingGemma was best on the 20-question check, but the check cannot separate models two hits apart, and its cost is measured.
- **"Default" means two things.** One study speaks of a default model, the other of keeping lexical search the default until a pilot. They agree once read as: lexical is the default mode, and the small model is the default when hybrid mode is on.
- **The vector timings used 768 dimensions**, EmbeddingGemma's size; the recommended small models have fewer, so those numbers are an upper bound.
- **Everything moves fast.** hugot's pure-Go tokenizer dates from October 2026 and EmbeddingGemma 2 from the same month.

## Gaps

- **No end-to-end measurement.** Model, index and command line were measured apart, on one Linux machine with warm files. Closing it needs a prototype timed as a whole process, also on macOS and arm64.
- **The cut-off is uncalibrated.** It needs judged questions that should find nothing, per model, in the fixture.
- **The model choice rests on 20 questions by one author.** The fixture must decide.
- **Memory while indexing** (1.9 GB) is one run with one batch size.
- **Index updates under concurrent writers** were designed, not run.
- **Each checkout builds its own index**, because `.pbrain/` is local. What that costs a team working in several work trees was not measured.
- **Legal:** whether downloading EmbeddingGemma v1 on first use makes pbrain a distributor under Gemma's terms.
- **PageIndex:** no independent test of outline-first reading against semantic chunks on content like ours; transcripts usually have no headings.
- **Not found:** Cohere's embedding prices; a controlled study of explicit links helping agents.

## Subtopics

- [Embedding models and running them from Go](./embedding-models-and-go-runtime/research_summary.md): EmbeddingGemma and twelve alternatives, remote options, four ways to run a model from Go, speed and memory measurements, and the 20-question check. Recommends a small permissive model in pure Go.
- [qmd in depth, and the index and hybrid retrieval engine in Go](./qmd-and-go-index-engine/research_summary.md): how qmd searches, three SQLite drivers, exact vector search timings, rank fusion, abstention and the index life cycle. Recommends prototyping pure-Go SQLite with the lexical scan as reference.
- [Long documents, PageIndex and references](./long-documents-and-pageindex/research_summary.md): what PageIndex does and what its results show, chunking and other long-document methods, and links between items. Recommends an outline, section-sized chunks and explicit links only.
