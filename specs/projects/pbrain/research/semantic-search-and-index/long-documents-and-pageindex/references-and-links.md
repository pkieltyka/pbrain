# References between items and documents

Researched 2026-11-20 by @lead-dev-2. "Inference:" marks my reading.

## What pbrain has today (functional spec, status complete, 2026-11-20)

- Two explicit links in front matter: `replaces` and `relates` (sections 4.1 and 4.5). `search` and `get` show an item's related items beside it, in both directions, as one-line pointers within the budget.
- Bodies are opaque: "it parses no links or footnotes in them. To cite an item, write its id in the text" (2.4). Parsing links in bodies, inferred relations and graph views are out of scope for v1 (17).

## What others do

| System | Links | Model needed | Retrieval follows links? |
|---|---|---|---|
| Obsidian | wiki links; backlinks derived ("Linked mentions are backlinks to the notes that contain an internal link to the active note"; [help](https://obsidian.md/help/Plugins/Backlinks)) | no | shown in a pane; the reader follows |
| Basic Memory | typed relations in Markdown (`- relates_to [[Note]]`) | no | `build_context` follows relations "up to depth hops (1-5)", default depth 1 and 10 related notes, by a recursive SQL query; a compact mode returns the graph without bodies ([repo](https://github.com/basicmachines-co/basic-memory), `mcp/tools/context.py`) |
| Aider | file dependency graph | no | ranks the repository map by it ([repo map](https://aider.chat/docs/repomap.html)) |
| Microsoft GraphRAG | entity graph and community summaries extracted by an LLM | yes, at index and query time | local search enters the graph through the query's entities. "GraphRAG indexing can be an expensive operation" ([README](https://github.com/microsoft/graphrag), v3.3.0, 2026-10-08); 281 minutes to index a 1M-token podcast corpus ([arXiv 2404.16130](https://arxiv.org/abs/2404.16130)). LazyGraphRAG cuts indexing to "0.1% of the costs of full GraphRAG" ([blog](https://www.microsoft.com/en-us/research/blog/lazygraphrag-setting-a-new-standard-for-quality-and-cost/)) |
| HippoRAG / HippoRAG 2 | triples extracted by an LLM, Personalized PageRank | yes at index time | "10-30 times cheaper and 6-13 times faster" than iterative retrieval with comparable quality on multi-hop QA ([arXiv 2405.14831](https://arxiv.org/abs/2405.14831)); HippoRAG 2 recall@5 59.8 against RAPTOR 48.8 and GraphRAG 49.6 ([arXiv 2502.14802](https://arxiv.org/abs/2502.14802)) |

## Does following explicit links help agents?

- The strongest primary evidence found is old and indirect: a retriever trained to follow Wikipedia hyperlinks beat "the previous best model by more than 14 points" on HotpotQA ([Asai et al., arXiv 1911.10470](https://arxiv.org/abs/1911.10470), ICLR 2020).
- PageIndex argues that in-document references such as "see Appendix G" defeat similarity search; that is a claim, not a measurement.
- I found no controlled study of whether coding or knowledge agents do better with explicit id links between notes than with search alone.

Inference: links are cheap and exact when people or agents write them, and the agent can follow a printed id in one call. The evidence does not justify pbrain expanding results along links on its own, which would spend the context budget on items nobody asked for. The spec's current behaviour (show linked ids as pointers; never infer relations) is the right default.

## Long sources and the items that cite them

The retrieval review recommended that agents extract what matters from a long document into small items of their own, citing the document. Two small, model-free additions would make that citation precise:

1. **Cite a section, not only an item.** A citation such as `<source id>#<heading path>` or `<source id>@<offset>` in `source` or in the body, and `get` accepting it to open that section. Inference: heading paths survive edits elsewhere in the document; offsets do not, but pbrain never edits a source item after import (spec 8.1; `verify` refuses source items, 8.3), so an offset into one is stable unless someone edits the file by hand.
2. **Backlinks from body mentions (later).** Ids written in bodies, collected by a derived index, would give "cited by 3 items" on a document, so an agent reading a source sees the decisions drawn from it. This is the Obsidian pattern and needs no model, but it changes the spec's rule that bodies are opaque (2.4, 17), so it is an amendment, not a detail.

Not recommended: an entity graph, community summaries or link-following retrieval in pbrain; all need a model and their costs are documented above.
