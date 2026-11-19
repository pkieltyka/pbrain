# qmd

## Bottom Line

Use qmd 2.9.0 as an optional local reader over a deliberately scoped shared Markdown collection. Its BM25 path needs zero model downloads, and the new namespaced metadata filters could support an adapter later. pbrain should own its default lexical interface, stable lookup, authority and supersession rules, private-file exclusion before indexing, and total response budget. Remote embeddings and reranking are explicit opt-ins that send queries and document text to the configured services. [qmd 2.9.0 release](https://github.com/tobi/qmd/releases/tag/v2.9.0) · [pbrain requirements and fixture](https://github.com/pkieltyka/factory/issues/62)

## Key Findings

- **Markdown stays canonical.** A collection is a directory with a glob and exclusions. qmd builds derived SQLite content, FTS5 and optional vector indexes. An isolated three-file CLI test confirmed qmd indexed a Git-ignored matching file and skipped a symlink to a private file. Configure the corpus itself as shared-only. [Collections](https://github.com/tobi/qmd/blob/v2.9.0/README.md#configuring-indexyml) · [Scan implementation](https://github.com/tobi/qmd/blob/v2.9.0/src/cli/qmd.ts#L2216-L2229)
- **The three search commands have different costs.** `search` is model-free BM25, free-text `vsearch` uses semantic embeddings and local expansion, and ordinary `query` adds lexical fusion and reranking. Typed queries can skip expansion, and reranking can be turned off. The default model set is roughly 2 GB, separate from Node/Bun and native package dependencies. [Search flow](https://github.com/tobi/qmd/blob/v2.9.0/src/store.ts#L7288-L7444) · [Model sizes](https://github.com/tobi/qmd/blob/v2.9.0/README.md#gguf-models-via-node-llama-cpp)
- **Metadata predicates landed in 2.9.0.** They operate on the opt-in YAML `qmd.metadata` namespace across CLI, SDK, MCP and HTTP. Existing pbrain fields would need projection for predicate pushdown. The release and tagged code show scoped BM25 sees every matching candidate before limiting, which supersedes an older overfetch description in the tagged README. [Release](https://github.com/tobi/qmd/releases/tag/v2.9.0) · [Scoped FTS code](https://github.com/tobi/qmd/blob/v2.9.0/src/store.ts#L5310-L5380)
- **The agent interface offers snippets and line windows.** JSON search emits a path, short docid, score and snippet, with optional metadata and context. The six-character id is a content hash prefix and collision lookup takes the first active match. A pbrain adapter would use its own id, current note state and bounded citation payload. [JSON formatter](https://github.com/tobi/qmd/blob/v2.9.0/src/cli/qmd.ts#L2836-L2863) · [Id lookup](https://github.com/tobi/qmd/blob/v2.9.0/src/store.ts#L4493-L4521)

## Details

- [Engine and integration](./engine-and-integration.md) — versioned implementation evidence, CLI and MCP, privacy boundaries, install measurements, and adapter trade-offs.

## Open Questions / Gaps

- The published package was smoke-tested for lexical search and lookup on Linux. Semantic model performance, cross-platform installation, remote provider interoperability and relevance on real pbrain notes were outside that fixture. Upstream benchmarks and issue timings use different data, versions and hardware. [Benchmark interface](https://github.com/tobi/qmd/blob/v2.9.0/README.md#benchmarking) · [Model-load report](https://github.com/tobi/qmd/issues/977)

## Sources

- [qmd 2.9.0](https://github.com/tobi/qmd/releases/tag/v2.9.0) — November 18, 2026 UTC release, especially filtering, remote models and indexing behavior.
- [Tagged README](https://github.com/tobi/qmd/blob/v2.9.0/README.md) — collection config, models, CLI and MCP at 2.9.0.
- [Tagged implementation](https://github.com/tobi/qmd/blob/v2.9.0/src/store.ts) — scoped retrieval and content-hash lookup at 2.9.0.
