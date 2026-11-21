# Long documents, PageIndex and references

## Bottom Line

PageIndex's useful idea for pbrain needs no model: a table of contents per long document, built from its headings, read first, then one section fetched by range. Its "reasoning-based retrieval" is an LLM agent walking that outline with two tools, which is the job the calling agent already does. Its headline 98.7% on FinanceBench is the vendor's own measurement of an older system, with no independent reproduction.

For pbrain v1, recommended:
- an outline for long items: the headings with offsets and sizes, plus a node for text before the first heading;
- search that matches on sections and reports the best section as the item's passage, with its heading path and offset;
- section-sized index chunks with no overlap.

On our 86,000-character functional spec, that outline is 1,874 characters, about 2% of the text. Model-based approaches (Contextual Retrieval, section summaries, RAPTOR, GraphRAG) need an LLM per chunk or per section and stay out of pbrain. Late chunking is worth a trial only once semantic search exists. Links between items should stay explicit and be shown as pointers, never followed automatically. Section-precise citations, and later backlinks from ids in bodies, are cheap additions.

## Key Findings

- **PageIndex** (MIT, 40.5k stars, v0.7.4 on 2026-11-19, [repo](https://github.com/VectifyAI/PageIndex)) has three index builders:
  - PDF "Flash": the tree comes from layout with no LLM; an LLM writes summaries.
  - Classic PDF: an LLM for nearly every step.
  - **Markdown**: headings alone, zero model calls with summaries off. A maintainer confirms this in [#324](https://github.com/VectifyAI/PageIndex/issues/324).
  
  Query time is an LLM agent with `get_document_structure` ("call this first to locate relevant sections") and `get_page_content` ("tight, targeted page ranges"). Node ids are positional (`0001`) and shift when headings are added. Markdown nodes have no end offset, and text before the first heading is dropped ([#609](https://github.com/VectifyAI/PageIndex/issues/609)).
- **PageIndex's evidence:**
  - The 98.7% FinanceBench score was measured by Vectify on Mafin 2.5 (GPT-4o / DeepSeek v3, Feb 2025), with the vendor's annotators settling ambiguous questions.
  - Its newer benchmark compares 41 LLMs, all using PageIndex, and has no vector-RAG baseline.
  - It shows that an LLM can navigate outlines of financial reports, not that outlines beat embeddings in general.
- **Other approaches and their costs** ([long-document-approaches.md](./long-document-approaches.md)):
  - Heading-aware splitting and parent-child retrieval need no generative model.
  - Late chunking ([arXiv 2409.04701](https://arxiv.org/abs/2409.04701); the plan's 2409.04418 is a chemistry paper) needs a long-context embedding model.
  - Contextual Retrieval needs an LLM call per chunk. It cut top-20 failures by 49 to 67% on Anthropic's own benchmark, at $1.02 per million tokens.
  - RAPTOR and section summaries need an LLM.
  - Chroma's study found 200 to 400-token chunks with no overlap at or near the top; 800 tokens with 400 overlap scored lowest on most metrics.
- **Agents already read by range:** Claude Code's Read returns at most 25,000 tokens and pages larger files by offset, and Aider keeps a 1k-token repository map. On agentic search versus embeddings, Claude Code's team reports "mostly vibes" for dropping vector RAG. Cursor's A/B test found grep plus semantic search 12.5% more accurate than grep alone. Inference: give agents a map and a precise jump; add semantic matching beside lexical matching, not instead of it.
- **References** ([references-and-links.md](./references-and-links.md)):
  - Obsidian and Basic Memory derive backlinks and follow relations with no model; GraphRAG and HippoRAG need an LLM at index time, and GraphRAG indexing is "an expensive operation".
  - Evidence that following explicit links helps agents is indirect: +14 points on HotpotQA for a trained retriever following Wikipedia links.
  - pbrain's current rule (explicit `relates`/`replaces`, shown as pointers) fits the evidence.
- **Measured on this repository:** outlines cost 1 to 2% of a document. Our spec's full outline (51 sections) is 1,874 characters, or 654 to level 2, so it fits inside `search`'s 2,000-character default budget.

## Recommendation

**v1 (model-free; a functional-spec amendment, since 7.2 and 6.1 change):**
1. **`get <id> --outline`** prints a long item's sections: heading path, level, start offset, own size and subtree size, under the usual budget and paging. Details:
   - include setext headings and a "before the first heading" node;
   - skip both kinds of code fence;
   - merge tiny sections, keeping their titles.
   
   A long item's search result could mention that an outline exists.
2. **The search passage names its section.** This builds on the retrieval review's agreed "best passage" rule: the passage's heading path and offset are printed with it, so the agent opens `get <id> --offset N` at the right place.
3. **Index sections, not whole items** (for lexical scoring now, and for vectors if semantic search is adopted):
   - a chunk is a heading section;
   - an oversized section is split at paragraphs around 400 tokens, with no overlap;
   - a headingless document (such as a transcript) is split by size;
   - results stay one per item, with the best chunk as its passage.
   
   Chunk sizes should be tuned on the retrieval fixture. Index design is @lead-dev's subtopic and the architecture's.
4. **Section-precise citations:** `source` or body text may cite `<id>#<heading path>` or `<id>@<offset>`, and `get` accepts them. An offset into a source item stays valid, because pbrain never edits a source item after import.

**Later, on evidence:**
- late chunking, once a long-context embedding model is in use;
- backlinks from ids mentioned in bodies ("cited by 3 items"), which needs an amendment to the opaque-body rule;
- optional summaries written by an agent at import time, as an ordinary field it supplies, never generated by pbrain.

**Not for pbrain:** LLM tree search inside pbrain (the agent does it), Contextual Retrieval, RAPTOR, GraphRAG.

## Details

- [pageindex-in-depth.md](./pageindex-in-depth.md): the three builders and their model needs, node shape, the query-time agent and its tools, the results and their caveats, and the six ideas that carry over.
- [long-document-approaches.md](./long-document-approaches.md): heading sections, parent-child, chunk size, late chunking, Contextual Retrieval, RAPTOR; what Claude Code, Cursor and Aider do; outline sizes measured on our documents.
- [references-and-links.md](./references-and-links.md): pbrain's links today, how others link and traverse, the evidence for following links, and section citations and backlinks.

## Open Questions / Gaps

- **No independent check of PageIndex's results**, and no measurement of outline-first navigation against semantic chunks on pbrain-like content. The retrieval fixture should include long sources and questions answered by one section.
- **Transcripts usually have no headings,** so the outline falls back to size-based parts. Speaker-turn segmentation is unexplored.
- **Not read:**
  - the RAPTOR cost appendix;
  - HippoRAG 2's resource analysis;
  - a 2026 independent ablation of document structure in dense retrieval (arXiv 2610.10170), which was seen only in a search summary.
- **Not found:**
  - a controlled study of explicit links helping LLM agents;
  - a GraphRAG indexing cost in dollars from Microsoft;
  - current Cursor documentation on read-range limits.

## Sources

- [PageIndex](https://github.com/VectifyAI/PageIndex) at `614cb81` (2026-11-20): README, `page_index_md.py`, `page_index_classic.py`, `agent_tools.py`, `utils.py`, `flash/README.md`, `page_search.py`; issues #324, #355, #609; [intro blog](https://pageindex.ai/blog/pageindex-intro) (2025-09-19); [Mafin 2.5](https://github.com/VectifyAI/Mafin2.5-FinanceBench) and its blog (2025-02-19); [financial QA benchmark](https://github.com/VectifyAI/pageindex-benchmarks/tree/main/financial-qa); [MCP docs](https://pageindex.ai/docs/mcp); [pricing](https://pageindex.ai/pricing).
- [Contextual Retrieval](https://www.anthropic.com/news/contextual-retrieval) (2024-09-19); [late chunking arXiv 2409.04701](https://arxiv.org/abs/2409.04701) and [Jina blog](https://jina.ai/news/late-chunking-in-long-context-embedding-models/); [RAPTOR arXiv 2401.18059](https://arxiv.org/abs/2401.18059); [Chroma chunking report](https://www.trychroma.com/research/evaluating-chunking) (2024-07-03); LangChain [MarkdownHeaderTextSplitter](https://github.com/langchain-ai/langchain/blob/master/libs/text-splitters/langchain_text_splitters/markdown.py) and [ParentDocumentRetriever](https://github.com/langchain-ai/langchain/blob/master/libs/langchain/langchain_classic/retrievers/parent_document_retriever.py); LlamaIndex [AutoMergingRetriever](https://developers.llamaindex.ai/python/examples/retrievers/auto_merging_retriever/).
- [Claude Code tools reference](https://code.claude.com/docs/en/tools-reference) (fetched 2026-11-20); [effective context engineering](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents) (2025-09-29); [Boris Cherny on X](https://x.com/bcherny/status/2017824286489383315) (2026-02-01); [Latent Space](https://www.latent.space/p/claude-code) (2025-05-07); [Cursor semantic search](https://cursor.com/blog/semsearch) (2025-11-06); [Aider repo map](https://aider.chat/docs/repomap.html).
- [Obsidian backlinks](https://obsidian.md/help/Plugins/Backlinks); [Basic Memory](https://github.com/basicmachines-co/basic-memory); [GraphRAG](https://github.com/microsoft/graphrag) and [arXiv 2404.16130](https://arxiv.org/abs/2404.16130); [LazyGraphRAG](https://www.microsoft.com/en-us/research/blog/lazygraphrag-setting-a-new-standard-for-quality-and-cost/); [HippoRAG](https://arxiv.org/abs/2405.14831), [HippoRAG 2](https://arxiv.org/abs/2502.14802); [Asai et al. 2020](https://arxiv.org/abs/1911.10470).
- Local measurement on 2026-11-20: a heading parser over `functional_spec.md` and two research documents in this repository.
