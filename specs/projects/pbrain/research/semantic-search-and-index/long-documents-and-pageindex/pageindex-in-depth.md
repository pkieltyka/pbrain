# PageIndex in depth

Researched 2026-11-20 by @lead-dev-2 against github.com/VectifyAI/PageIndex at commit `614cb81` (2026-11-20), latest release v0.7.4 (2026-11-19). Code read at that commit; the Markdown path re-checked by hand. Sources were collected with a research sub-agent and spot-checked. "Inference:" marks my reading.

## What it is

- "PageIndex: Vectorless, Reasoning-based RAG … No Vector DB, No Chunking". Two steps: "1. Index: generate a tree-structure index for each document 2. Retrieve: agentically search that tree with LLM reasoning" ([README](https://github.com/VectifyAI/PageIndex)).
- The idea, in the [intro blog](https://pageindex.ai/blog/pageindex-intro) (2025-09-19): read the table of contents, select a section, extract, and repeat until the information is sufficient, "The ToC acts as an index tree that organizes content into hierarchical nodes", with "node_id … a reference key to locate the corresponding raw data."
- Vectify AI (London). MIT licence, 40.5k stars, about 30 releases between 2026-08-25 and 2026-11-19, commits daily (GitHub API, 2026-11-20). No peer-reviewed paper; the README cites its own blog.

## Index time: three builders, different model needs

| Builder | Input | Model at index time | Notes |
|---|---|---|---|
| Flash (default for local PDFs since Aug 2026) | text PDFs | none for the tree; an LLM for node summaries and an optional "expand" | "the structure comes from the PDF's own layout, without an LLM, and the index model only writes node summaries and refines the tree" (README). Vendor cost: "about $0.001 per page" |
| Classic (`page_index_classic.py`) | PDFs | an LLM for nearly every step | TOC detection per page (first 20 pages), TOC extraction and conversion to JSON, page-number detection, matching and verifying TOC items against pages, fixing (3 attempts), a no-TOC fallback that reads the whole document in ~20k-token groups, splitting nodes over 10 pages and 20k tokens, and a summary per node |
| Markdown (`page_index_md.py`) | `.md` through the CLI or `md_to_tree()` only | **none** when summaries are off (the CLI default) | headings alone; see below |

Markdown mode, as the code does it:
- Headings are `^(#{1,6})\s+(.+)$` plus whole-line bold `^\*\*(.+?)\*\*\s*$` as level 1; lines inside ```` ``` ```` fences are skipped.
- A node's text runs from its heading to the next heading of any level (`end_line = all_nodes[i + 1]['line_num'] - 1`), so a parent's own text excludes its children.
- Nodes nest by level; `node_id` is a depth-first counter, `str(node_counter).zfill(4)`: ids are positional and change when a heading is added above.
- Optional "thinning" merges small subtrees by token count; optional LLM summaries for nodes of 200+ tokens.
- A maintainer, [issue #324](https://github.com/VectifyAI/PageIndex/issues/324): "Markdown (md_to_tree) — the tree structure is built without any LLM … set if_add_node_summary: 'no' … and you get the full hierarchy with zero LLM calls."
- Known gaps: text before the first heading is dropped, setext headings are ignored, and a file with no headings gives an empty tree ([#609](https://github.com/VectifyAI/PageIndex/issues/609)); under the summary threshold a node's text is copied into its "summary", 80.5% of nodes on one reporter's corpus ([#355](https://github.com/VectifyAI/PageIndex/issues/355)); nodes carry a heading line but no end line or offset; `~~~` fences are not handled.
- The local SDK accepts PDFs only ("only PDF files are supported in local mode"); Markdown is CLI or function only.

## Node shape

README example: `{"title", "node_id": "0006", "start_index": 21, "end_index": 22, "summary", "nodes": [...]}`, with page numbers (1-based, inclusive) for PDFs. Flash adds `key_items` (titles of merged-away subsections) and `toc_source` (`detected | bookmarks | hybrid | pages | unreadable`). Markdown nodes have `line_num` instead of a page range, and `summary` (leaf) or `prefix_summary` (parent) when summaries are on.

## Query time: an LLM agent with two tools

- No vector search in the local path. The agent (`agent_tools.py`, "identical to the cloud MCP server's tools/list") has:
  - `get_document_structure`: "Extract a document's hierarchical outline (headers, sections, page references). REQUIRED for documents over 20 pages — call this first to locate relevant sections, then pass their page numbers to get_page_content()." Outlines are paged in "parts".
  - `get_page_content`: "Use tight, targeted page ranges — never the whole document at once."
- Tool responses are capped at 100,000 characters (95% budget); outlines drop node text. Citations are page-level locally (`<cite doc=… page=…/>`), line-level in the cloud.
- The MCP server is cloud-only (`https://api.pageindex.ai/mcp`); the "Page Search API" is a separate cloud product (`page_search.py`: "Cloud-only").

## Results and how far to trust them

- "98.7% accuracy on FinanceBench … vastly outperforming vector-based RAG" (README). Measured by Vectify on "Mafin 2.5", its own system on GPT-4o and DeepSeek v3, which predates the open-source repository ([Mafin2.5-FinanceBench](https://github.com/VectifyAI/Mafin2.5-FinanceBench), blog 2025-02-19). Ambiguous or invalid questions were settled by "expert human annotations". I found no independent reproduction.
- The newer [PageIndex Financial QA Benchmark](https://github.com/VectifyAI/pageindex-benchmarks/tree/main/financial-qa) compares 41 LLM configurations, all with PageIndex retrieval: 65.7% to 99.0% on 100 questions over 10-K filings, at $1.47 to $128 per 100 questions, index cost excluded. It has no vector-RAG baseline: it compares models, not retrieval methods.
- Inference: the evidence shows that an LLM navigating a good outline answers financial-report questions well. It does not show that outlines beat embeddings in general, and the domain (long, well-structured filings with page tables) differs from pbrain's mostly short items.

## What carries over to pbrain without a model

Inference, each idea traced to the code above:

1. **An outline per long item from its headings**: Markdown mode with summaries off, plus what it lacks: a node for text before the first heading, setext headings, `~~~` fences, and for each section its start offset, its own size and its subtree size. On our 86,015-character functional spec, such an outline is 51 sections and 1,874 characters (2.2% of the text); headings to level 2 alone are 654 characters (measured with a 30-line script).
2. **Structure first, text on request**: PageIndex's two tools map to "outline of item X" and "section of item X". pbrain already has the second half: `get --offset` and `--max-chars`.
3. **Stable section addresses**: not PageIndex's positional "0001" ids, which shift when a heading is added; use the heading path (`8.4 import > Fetching a URL`) or the offset printed with the outline, which the agent uses at once.
4. **Deterministic merging of tiny sections**, keeping the merged titles (`key_items`) so the outline stays short without losing signposts.
5. **A model-free stand-in for summaries**: PageIndex's own fallback is "a parent's subsection titles, a leaf's own text", cut to 600 characters. For pbrain: the heading path and the section's first sentence.
6. **Paged outlines under a character budget**, as PageIndex splits outlines over its response cap: the same rule as pbrain's budgeted output.

What needs a model and so stays out of pbrain v1: node summaries, document descriptions, tree "expand", classic PDF TOC extraction, and the query-time tree search itself, which is the calling agent's job.
