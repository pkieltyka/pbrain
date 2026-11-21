# Long documents: approaches, costs and what agents already do

Researched 2026-11-20 by @lead-dev-2. Primary sources linked; "Inference:" marks my reading. Context: most pbrain items are short, but `source` items (design docs, transcripts, imported pages) run to 20,000 to 50,000 characters and more, up to the spec's 512,000-character import limit.

## 1. The approaches

| Approach | Model at index time | Model at query time | What it improves | Fits pbrain v1? |
|---|---|---|---|---|
| Heading-aware sections (Markdown header splitting; PageIndex Markdown mode) | none | none | chunks match the author's sections and carry a heading path | **yes** |
| Parent-child, "small-to-big" | embeddings (or none, with lexical scoring) | same | match on small pieces, return the larger unit | **yes, as item-level results with a passage** |
| Fixed-size chunks with overlap | none | none | simple | as a fallback inside oversized sections |
| Late chunking | a long-context embedding model | embedding model | chunk vectors informed by the whole document | later, only with semantic search |
| Contextual Retrieval | an LLM call per chunk, plus embeddings | embeddings, BM25, optional reranker | chunks that lose context when cut | no: needs a model call per chunk |
| Section summaries | an LLM per section | none | better outlines | no; a deterministic stand-in instead |
| RAPTOR | embeddings, clustering and LLM summaries | embeddings | thematic, multi-level questions | no |

### Heading-aware sections

LangChain's `MarkdownHeaderTextSplitter` ("Splitting markdown files based on specified headers"; [source](https://github.com/langchain-ai/langchain/blob/master/libs/text-splitters/langchain_text_splitters/markdown.py)) gives each chunk metadata naming its enclosing headers, and is usually followed by a size-based splitter. PageIndex's Markdown mode builds the same tree with no model ([pageindex-in-depth.md](./pageindex-in-depth.md)).

### Parent-child

- LangChain `ParentDocumentRetriever`: "it first fetches the small chunks but then looks up the parent IDs for those chunks and returns those larger documents" (child 400, parent 2,000 characters in its example; [source](https://github.com/langchain-ai/langchain/blob/master/libs/langchain/langchain_classic/retrievers/parent_document_retriever.py)).
- LlamaIndex `AutoMergingRetriever`: a 2,048 / 512 / 128 hierarchy; leaves are retrieved and merged into their parent when more than half of a parent's children match (`simple_ratio_thresh` 0.5; [docs](https://developers.llamaindex.ai/python/examples/retrievers/auto_merging_retriever/)).
- Inference: pbrain's results are already "parents": one result per item, cited by id. Matching on sections and reporting the best section as the result's passage, with its offset and heading path, is small-to-big without a vector store.

### Chunk size

Chroma's study ([report](https://www.trychroma.com/research/evaluating-chunking), 2024-07-03): "RecursiveCharacterTextSplitter with chunk size 200 and no overlap performs well"; "reducing chunk overlap improves IoU scores"; the common 800-token chunks with 400 overlap "scored lowest on most metrics" (recall 85.4 against 88.1 to 89.5 for 200 to 400 tokens). qmd uses 900-token chunks with 15% overlap. Inference: sections first, split at paragraphs when a section exceeds about 400 tokens, no overlap; tune on the fixture.

### Late chunking

[arXiv 2409.04701](https://arxiv.org/abs/2409.04701) (the plan's 2409.04418 is a chemistry paper): embed all tokens of the long text first, then pool per chunk, "after the transformer model and just before mean pooling", with no extra training. Jina's results (BEIR nDCG@10, naive to late): NFCorpus 23.46 to 29.98, SciFact 64.20 to 66.10, Quora unchanged; "the longer the document, the more effective" ([blog](https://jina.ai/news/late-chunking-in-long-context-embedding-models/), 2024-08-22). Inference: it needs a long-context model (EmbeddingGemma 2's 8,192 tokens would do; v1's 2,048 barely) and per-token outputs (llama.cpp can return unpooled embeddings). Worth trying only after semantic search exists.

### Contextual Retrieval

Anthropic, [2024-09-19](https://www.anthropic.com/news/contextual-retrieval): an LLM writes 50 to 100 tokens of context for each chunk before embedding and BM25. Top-20 retrieval failures fell 35% (embeddings), 49% (with BM25), 67% (with a reranker), on Anthropic's own benchmark; cost "$1.02 per million document tokens" with prompt caching. The same post: below about 200,000 tokens, "you can just include the entire knowledge base in the prompt". Inference: excluded by the spec's "pbrain calls no model"; an agent could write such context itself as an item's summary, which is a write-time choice, not a pbrain feature.

### RAPTOR and summaries

RAPTOR ([arXiv 2401.18059](https://arxiv.org/abs/2401.18059)) clusters chunks and has gpt-3.5 summarise each cluster, recursively; "20% in absolute accuracy" on QuALITY with GPT-4. Every abstractive summary needs a model; the deterministic stand-ins are the first sentence or paragraph, child titles and key terms.

## 2. What agent tools already do with long files

- **Claude Code**: "By default one call returns at most 25,000 tokens … and refuses a whole file over 256 KB, so Claude reads a larger file in pages with offset and limit"; over the limit it returns "the first page of the file, with a PARTIAL view notice"; Grep is ripgrep with line numbers ([tools reference](https://code.claude.com/docs/en/tools-reference), fetched 2026-11-20).
- **Anthropic on context**: keep "lightweight identifiers", use grep and glob "to retrieve files just-in-time", "progressive disclosure", and "a hybrid strategy" for less dynamic content such as legal or finance work ([context engineering](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents), 2025-09-29).
- **Aider**: a repository map of files and key symbols, ranked over a dependency graph, in a budget that "defaults to 1k tokens" ([repo map](https://aider.chat/docs/repomap.html)). A model-free, budgeted table of contents.
- **Agentic search vs embeddings.** Claude Code dropped "RAG + a local vector db" because "agentic search generally works better" (Boris Cherny, [X, 2026-02-01](https://x.com/bcherny/status/2017824286489383315)); asked earlier for the benchmark, he said "mostly vibes" ([Latent Space, 2025-05-07](https://www.latent.space/p/claude-code)). Cursor measured the opposite on its own A/B test: grep plus semantic search gave "on average 12.5% higher accuracy" (6.5% to 23.5% by model), with gains largest on codebases of 1,000+ files: "the combination of these two leads to the best outcomes" ([blog, 2025-11-06](https://cursor.com/blog/semsearch)).

Inference: the agents pbrain serves already page through files by offset and search by words. What they lack for pbrain's long items is a cheap map (an outline) and a reliable jump from a search hit to the right section. Cursor's numbers support adding semantic search beside lexical search, not instead of it; Claude Code's experience supports keeping navigation in the agent's hands.

## 3. Numbers for pbrain's own long documents

Measured with a 30-line heading parser on this repository (2026-11-20):

| Document | Characters | Sections | Full outline | Outline to level 2 |
|---|---|---|---|---|
| `functional_spec.md` | 86,015 | 51 | 1,874 chars | 654 chars |
| `agent-memory-systems/systems-survey.md` | 17,384 | 11 | 745 chars | 745 chars |
| `qmd/engine-and-integration.md` | 12,929 | 6 | 248 chars | 248 chars |

Inference: an outline costs about 1 to 2% of a document and fits inside `search`'s default 2,000-character budget for documents up to about 90,000 characters. A meeting transcript usually has no headings; its outline would be empty, and the fallback is fixed-size parts or speaker turns.
