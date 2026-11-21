# qmd in depth and the Go index engine

## Bottom Line

Prototype a single-binary hybrid path using modernc SQLite with its bundled sqlite-vec, with pbrain's plain Go lexical scan as the reference and offline fallback. At 50,000 synthetic float32 vectors of dimension 768, SQLite exact cosine search took about 60 ms at p95 with warm file pages, so approximate search is premature. Borrow qmd's structure-aware chunking, model and input fingerprints, and rank fusion, while keeping pbrain's identity, authority, visibility and output budget. Preserve the lexical default until an owner-approved amendment and a real-task pilot justify exposing an opt-in hybrid mode with an explicit abstention gate. Model startup, initial indexing and relevance on real project tasks are still separate gates. [Driver probe](./go-index-options-and-probes.md) · [Proposed hybrid and lifecycle design](./hybrid-ranking-and-index-lifecycle.md) · [Functional spec §§5–7 and 17](../../../functional_spec.md)

## Key Findings

- **qmd 2.9.0 is a working reference.** Document-level SQLite FTS5 BM25 and exact sqlite-vec chunk KNN feed weighted RRF. Defaults use 900 tokens per chunk, 15% overlap and Markdown breakpoints, with a tokenizer check for local embeddings. Hybrid queries can use an expansion model, and optional cross-encoder reranking scores one passage per candidate file. The tagged code and README disagree on score transforms. I did not find proof that the reranker pays for itself on engineering-project memory. [qmd pipeline and evidence](./qmd-pipeline-and-tradeoffs.md)
- **One no-cgo index binary is feasible.** Modernc v1.62.0 bundles FTS5 and a translated sqlite-vec v0.1.9. The smoke probe ran on Linux amd64 and cross-built for Linux arm64 and both macOS architectures. The three compared drivers all reported SQLite 3.54.1. The published sqlite-vec wrapper for ncruces did not build with ncruces v0.37.0. Its current FTS5 needs explicit registration and its supported vector extension is the distinct approximate vec1. Mattn's cgo path works with FTS5 enabled and C headers. [Driver matrix and compile results](./go-index-options-and-probes.md#sqlite-driver-comparison)
- **Exact search is fast enough to test first.** At 5,000 × 768 the modernc vec0 query took 8.75 ms at p95, at 50,000 × 768 it took 59.76 ms. Go brute-force with an already resident matrix was faster in the same fixture. Reloading the large matrix from SQLite cost about 195 ms before any lookup. No numbers include embeddings, full CLI startup, Git checks or pbrain's result assembly. [Measured fixture and limits](./go-index-options-and-probes.md#scratch-measurements)
- **Do not let limits or scores substitute for the contract.** Filter eligible items before selecting windows, aggregate each long document's passages to one item and calibrate a semantic-only cutoff on expected-empty tasks. RRF is a candidate-ranking tool. It neither proves relevance nor decides authority. Keep decision promotion within the best eligible lexical class and label semantic-only evidence. A default FTS5 tokenizer may miss pbrain terms under its different Unicode rules, so test indexed output against the scan. [Ranking proposal](./hybrid-ranking-and-index-lifecycle.md#retrieval-order-and-abstention) · [FTS5 caveat](./go-index-options-and-probes.md#fts5-fits-as-an-accelerator)
- **Treat all indexes as derived.** Store shared and personal indexes separately under their protected `.pbrain/` paths. Revalidate the filesystem on every refresh, version the embedding space, batch writes and permit a lexical scan when a generation is stale or corrupt. Gate any remote embeddings on explicit scope and separate authorization for personal data. [Lifecycle design](./hybrid-ranking-and-index-lifecycle.md#storage-refresh-and-recovery)

## Details

- [qmd pipeline and trade-offs](./qmd-pipeline-and-tradeoffs.md) covers chunking, FTS5, vector grouping, RRF weights, expansion, reranking and model cost.
- [Go index options and probes](./go-index-options-and-probes.md) compares three SQLite drivers, other Go indexes and exact lookup measurements for 5,000 to 50,000 chunks.
- [Hybrid ranking and index lifecycle](./hybrid-ranking-and-index-lifecycle.md) models abstention, passage offsets, artifact fingerprints, concurrent refresh, privacy and the evaluation gate.

## Open Questions / Gaps

- Scratch timings use one Linux amd64 host, synthetic text and vectors, and warm file pages. macOS builds were compiled only. Real cold-start latency, model inference, update contention, and actual item/chunk ratios need measurements.
- No semantic model was downloaded or run here. This pass does not resolve embedding model quality, runtime packaging or whether 256 dimensions preserve useful recall. The embedding/runtime subtopic owns those choices.
- I could not find a controlled qmd quality result isolating the reranker on the intended engineering workload. The project pilot should compare strict, semantic additions and any reranker at a fixed context budget before changing defaults.
- Calibration needs judged real misses, including out-of-scope and access-denied cases. A model-specific threshold and whether semantic hits ever move ahead of exact lexical results are product choices for the amendment.

## Sources

- [qmd 2.9.0, released 2026-11-18](https://github.com/tobi/qmd/releases/tag/v2.9.0) and its [tagged store](https://github.com/tobi/qmd/blob/v2.9.0/src/store.ts) describe the reference pipeline.
- [SQLite FTS5](https://www.sqlite.org/fts5.html) and [SQLite WAL](https://sqlite.org/wal.html) define lexical ranking and concurrent-storage behaviour.
- [Modernc SQLite v1.62.0](https://pkg.go.dev/modernc.org/sqlite@v1.62.0) and its [vec package](https://pkg.go.dev/modernc.org/sqlite/vec) document the no-cgo integration verified in the local probe.
- [sqlite-vec v0.1.9](https://github.com/asg017/sqlite-vec/releases/tag/v0.1.9) and [SQLite vec1](https://sqlite.org/vec1.html) distinguish the stable exact engine and the separate ANN extension.
- [Reciprocal Rank Fusion, SIGIR 2009](https://cormack.uwaterloo.ca/cormack/cormacksigir09-rrf.pdf) supplies the rank-fusion baseline.
