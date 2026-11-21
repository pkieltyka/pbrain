# Research Plan: Built-in semantic search and index for pbrain

## Goal

The owner does not think keyword search alone will be enough, and is considering building our own qmd-like search into the pbrain binary: written in Go, a local index (SQLite, perhaps with its vector extension, or Pebble), hybrid keyword plus semantic retrieval with a simple ranking, using a small local embedding model shipped with or fetched by the binary (EmbeddingGemma is the owner's first candidate), with an optional remote embedding model, and no other service required. The owner also asked us to study qmd closely and VectifyAI's PageIndex for ideas, including how to retrieve from long documents and keep references between documents. This research decides whether that is feasible and sound for v1, and how to build it. It feeds an amendment to the functional spec (section 7.1 search, section 17 out of scope) and the architecture's search and index subsystem.

Constraints to respect: agents' context window is the key constraint (results stay small, budgeted and cited); Markdown files stay the only truth and any index is derived and rebuildable; the personal boundary must hold in the index too (nothing personal in the shared index, no personal text sent to a remote model without the person's choice); single Go binary, Linux and macOS (Windows out of scope in v1); works offline.

## Run

- Researchers (factory members): @lead-dev (gpt-6.2-sol) and @lead-dev-2 (claude-opus-5-5). @architect writes the cross-subtopic summary.
- Web tools: each researcher's own web search and fetch; `gh` for repositories; reading code is encouraged. Harmless install/version/--help runs in a scratch folder are fine; do not change system settings.

## Subtopics

- [ ] Embedding models and running them from Go (@lead-dev-2)
- [ ] qmd in depth, and the index and hybrid retrieval engine in Go (@lead-dev)
- [ ] Long documents, PageIndex and references (@lead-dev-2)

## Focus Details

### Embedding models and running them from Go (folder: embedding-models-and-go-runtime/)

EmbeddingGemma first: what it is (Google, 2025+), size and parameter count, dimensions and Matryoshka truncation, context length, quantised forms (GGUF, ONNX, int8/int4) and their quality loss, licence terms (Gemma licence: redistribution inside our binary or a download on first use?), languages, retrieval quality on MTEB retrieval and code-retrieval tasks, recommended task prompts. Compare with small alternatives: nomic-embed (text v1.5/v2), bge-small/base, gte-small, snowflake-arctic-embed, jina-embeddings (incl. code), Qwen3-Embedding-0.6B, all-MiniLM, and whatever qmd uses. Remote options for an opt-in mode: Gemini embedding, OpenAI text-embedding-3, Voyage (incl. code), Cohere: quality, price per million tokens, data-retention terms. How to run a local model from Go: pure Go inference if any is viable; ONNX Runtime through cgo (onnxruntime_go, knights-analytics/hugot); llama.cpp through cgo or purego (e.g. hybridgroup/yzma, go-skynet); a bundled helper process; consequences for a single static binary, cross-compiling for Linux and macOS (arm64, amd64), binary size, and first-run model download vs embedding the weights. Measure or find CPU latency and memory for embedding a 500-word chunk and a short query, and indexing time for ~5,000 items of a few hundred words on a laptop. Tokenisation in Go for the chosen model. Conclude: the recommended local model and runtime, the recommended remote option, and the risks.

### qmd in depth, and the index and hybrid retrieval engine in Go (folder: qmd-and-go-index-engine/)

qmd (github.com/tobi/qmd, v2.9.0+) in depth: chunking (size, overlap, Markdown-aware?), its BM25 via SQLite FTS5, vector storage (sqlite-vec?), query expansion, fusion of keyword and vector results (reciprocal rank fusion or other, weights), reranking (model, cost, whether it pays off), score normalisation, snippets, incremental indexing and change detection, what agents receive. Which parts are worth copying into pbrain and which are not (and why). The engine in Go: SQLite drivers (modernc.org/sqlite pure Go vs mattn/go-sqlite3 cgo vs ncruces/go-sqlite3 wasm), FTS5 availability and BM25 tuning in each, sqlite-vec (status, loading it from each driver, brute-force vs ANN, performance at 5,000 to 50,000 chunks), alternatives (Pebble/Badger with our own vectors and brute-force cosine, an HNSW library such as coder/hnsw, chromem-go, bleve, usearch); for our scale, is brute force fine? Hybrid ranking in practice: RRF, BM25-plus-cosine blends, how to combine with pbrain's existing order (match class, recency band, standing, decision-in-force promotion) without losing determinism, and how to keep "no near-miss" honest when vectors always return something (thresholds, abstention). Index lifecycle: derived and rebuildable, stored in .pbrain/, incremental update by path/mtime/hash, model-version changes, corruption recovery, concurrent writers (several agents on one machine, worktrees), and keeping personal items in a separate index. Conclude: the recommended storage, chunking, fusion and update design, with numbers.

### Long documents, PageIndex and references (folder: long-documents-and-pageindex/)

VectifyAI PageIndex (github.com/VectifyAI/PageIndex): what it is (reasoning-based, vectorless retrieval over a document's tree of sections), how it builds the tree, whether it needs an LLM at index or query time, its reported results (e.g. FinanceBench) and their caveats, licence and maturity. Which ideas apply to pbrain without a model call inside pbrain: a table of contents per long source item (headings, positions, sizes) that an agent can read cheaply and then get a section by offset; section-level chunks; passage jumps. Other approaches to long-document retrieval for agents (late chunking, contextual retrieval, parent-child chunks, summaries per section) and what each costs. References between documents and items (citations by id, links, a derived graph) and whether retrieval should follow them. Conclude: what pbrain should do for long sources in v1 and later.

## Output

Each subtopic folder gets research_summary.md plus any deep docs. @architect then writes research_summary.md at the top of this folder: bottom line, a recommended design for built-in semantic search (or a reasoned "not in v1"), what changes in the functional spec, and the open decisions for the owner.
