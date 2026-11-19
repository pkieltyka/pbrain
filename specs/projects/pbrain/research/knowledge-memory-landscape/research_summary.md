# Research: Knowledge and memory landscape for pbrain

## Bottom Line

The research supports pbrain's starting direction and answers the owner's open question: use flat files with a few typed links in front matter, and treat any graph as a derived index. Every Markdown-native tool surveyed works this way, and Letta already runs agent memory as Markdown with front matter in Git, with worktrees for concurrent writers. A "sense of time and weighting" needs no database: written time, explicit supersession and an optional review date cover time; authority and status rank first, and usage signals live in a derived local index. No tool surveyed stores a confidence score. Old items are kept and superseded, never rewritten by a model. pbrain should own lexical search, exact lookup, lifecycle rules, private-file exclusion and the response budget, with qmd as an optional local reader. Ingestion can start with local Markdown and text files, because Google Docs and Meet transcripts export to Markdown natively. Two things set pbrain apart from everything surveyed: it commits shared memory that many writers edit, and it records who had the authority to decide. So the design problems are merge behaviour and write rules, not search or scale.

## Key Findings

- **Flat files, derived graph.** Basic Memory, OKF, Obsidian, Foam and ADR tools all write edges in the Markdown and compute the graph. A few front-matter keys plus plain inline links answer "what replaced X" and "what cites source S". ([Markdown knowledge stores](./markdown-knowledge-stores/research_summary.md))
- **Merge behaviour is the main risk for a committed, multi-writer store.** Sequential ids collide across branches, any shared index or log file conflicts on most merges, and an indexer that rewrites front matter produces repo-wide diffs. The safe pattern is one item per file, an id that is the never-renamed filename stem, and nothing written except by explicit commands. Factory's timestamp-and-slug convention is already close to the best pattern found. ([Markdown knowledge stores](./markdown-knowledge-stores/research_summary.md))
- **Keep and supersede.** mem0 moved to add-only, Graphiti invalidates instead of deleting, and ACE measured a model rewriting its own context from 18,282 tokens to 122 with accuracy below the no-memory baseline. Beads' semantic compaction overwrites live prose, which pbrain should not copy. ([Agent memory systems](./agent-memory-systems/research_summary.md), [Beads and Gas Town](./beads-and-gas-town/research_summary.md))
- **Time and weighting fit in files plus an index.** Git and `created` give written time, supersession bounds a decision's validity, and an absolute review date means "check again". Usage counts, votes and decay are common but always derived: in tracked files, every read would become a write. ([Agent memory systems](./agent-memory-systems/research_summary.md))
- **Automatic contradiction detection can propose, not decide.** Every method scored at most 28% on multi-hop selective forgetting in MemoryAgentBench. ([Agent memory systems](./agent-memory-systems/research_summary.md))
- **Authority is rare, and memory is an attack surface.** Vendor systems let the model write unattended and correct later; only factory records who decided. Memory poisoning through untrusted content is documented (SpAIware, MINJA), so imported text is reference data that can never set status, privacy or authority. ([Agent memory systems](./agent-memory-systems/research_summary.md), [Source ingestion](./source-ingestion/research_summary.md))
- **qmd is a good optional reader, not the default engine.** Its BM25 search needs no models; semantic search needs about 2 GB of models or a remote service that receives queries and document text. It indexes Git-ignored files, its short ids are content hashes that change on edit, and its new metadata filters read only a `qmd.metadata` namespace that pbrain's fields would have to be projected into. ([qmd](./qmd/research_summary.md))
- **Beads is a tracker; borrow three ideas.** Explicit typed links, exact keyed recall, and a small startup payload that announces what it left out. Skip its Dolt backend, whose migration and server failures are documented. Stable Gas Town and stable Beads are already incompatible on the memory calls, a caution for coupling sibling CLIs. ([Beads and Gas Town](./beads-and-gas-town/research_summary.md))
- **Priming is small, frozen and index-shaped; bodies are fetched on demand.** Claude Code loads 200 lines or 25 KB, Hermes about 1,300 tokens. Beads' byte cap can be exceeded by its first entry, so a budget must cover the whole rendered payload. Agent context, not file count, is the scale limit. ([Agent memory systems](./agent-memory-systems/research_summary.md), [Beads and Gas Town](./beads-and-gas-town/research_summary.md), [Markdown knowledge stores](./markdown-knowledge-stores/research_summary.md))
- **Ingestion is adapter-specific and carries permission questions.** Converters differ in structure, every static path returned only a loading placeholder for a JavaScript page, and PDF extraction can succeed with empty text. A Drive connector adds OAuth scope and retention choices, and committing an imported copy distributes it to every reader of the repository. ([Source ingestion](./source-ingestion/research_summary.md))

## Implications

For the decisions the spec is waiting on:

- **Layout and schema.** A shallow store: shared items, a Git-ignored private folder, ingested sources and a Git-ignored derived index. Kind is a front-matter field, not a folder. Front matter stays flat: kind, one-line title, created, author, source, status, who decided or verified, `supersedes`, optional `relates` and review date. No tracked hand-maintained index or log.
- **Query.** Bounded search returning a fixed small shape (id, kind, status, title, who, snippet, source), an exact get, and "N more not shown". Lifecycle filtering must happen before the limit, or results underfill.
- **Ingestion.** A first importer for local Markdown and text that records source locator, acquisition time and input digest. Browser rendering, a Drive connector, OCR and hosted extractors wait for a real source set.
- **Skill.** Search and get on demand by default; memory is never an instruction; session priming is optional and capped.
- **General-purpose framing.** The research was read against factory's notes convention: authority, the kinds and the field names come from factory, while most other tools let the model write without review. The spec must say which rules are pbrain's core and which a consumer configures.

Questions this leaves for the owner:

1. How much of factory's propose-and-decide authority model is core to a general-purpose pbrain?
2. Is supersession written only on the new item, or is the old item also stamped?
3. Keep factory's field names, or adopt those of Google's Open Knowledge Format, the best outside vocabulary found but about six weeks old? Add a random suffix to ids?
4. Where does the brain live by default: a folder in the project repo, or elsewhere?
5. Does a passed review date flag an item or hide it? Are usage signals in the first version, given they are per machine?
6. Is session priming in the first version?
7. Is ingestion in the first release, and does it need URLs? The overview mentions browser use; the research recommends deferring it.
8. Are original caches tracked, and who may read normalized copies of non-public sources?
9. Which runtime? qmd is Node or Bun with a roughly 934 MiB install, and converter choice depends on the stack.

## Conflicts and Uncertainty

- **Where supersession is written.** Beads stores an old-to-new edge and closes the old item; factory and MADR put it in the old item's status; the Markdown research recommends `supersedes` on the new item as authoritative, because adding a file always merges. The evidence leans to the new item, with the old item's status as a checked convenience.
- **Index files.** Claude Code, Hermes, Letta and Karpathy's wiki all rely on an index file, yet a tracked index is a merge hotspot. Those indexes are machine-local or single-writer. For pbrain the lean is to generate any index or priming slice from the derived index.
- **Git under concurrent writers.** Beads left Git-tracked JSONL for Dolt to get concurrency; Letta uses Git worktrees successfully. Beads holds a live task graph with mutable state, and pbrain holds append-mostly prose, so the lean is Git. No one has published conflict rates.
- **Staleness by age or by authority.** Hermes, Copilot and Generative Agents demote or expire by age and use; factory says freshness alone replaces nothing. The lean is that weighting only reorders eligible items and never outranks supersession or authority.
- **qmd as the query layer.** Letta's use of qmd supports the owner's "good match". The qmd study narrows it to an optional reader over a shared-only collection, and the rule that an indexer never writes to sources rules out adding `qmd.metadata` to items. qmd also moves fast: its 2.9.0 README and code disagree on filtering.

## Gaps

- **qmd in real use.** Only a Linux lexical smoke test ran. Closing it needs model-backed quality, latency and remote-provider runs on real pbrain notes, on more than one platform.
- **Search at store scale.** No `rg` or FTS5 benchmark; scale claims are inference. A synthetic store of a few thousand items would close it.
- **Multi-writer merges.** No measured conflict rate for agents writing Markdown memory on parallel branches, and no full Gas Town run or Dolt multi-writer benchmark. A worktree fixture for pbrain's own write path would close the part that matters.
- **Ingestion fixtures.** Converter tests were synthetic. Untested: an authenticated Google Workspace export, a real transcript at meeting length, browser login flows and complex PDFs. Size limits are proposals until checked against real documents.
- **Second-hand or unverified sources.** The REM paper and OWASP ASI06 were seen only through summaries. Memory benchmark numbers are vendor-reported. Hermes facts come from factory#89 and were not re-read.
- **Consumers other than factory.** The research assumed a multi-agent team; nothing tested the rules against a single agent or a person working alone.

## Subtopics

- [qmd](./qmd/research_summary.md): what qmd 2.9.0 does and how pbrain sits beside it. Optional local reader; pbrain owns ids, lifecycle, privacy and budgets.
- [Beads and Gas Town](./beads-and-gas-town/research_summary.md): the tracker's data model, storage history and session priming. Borrow typed links, keyed recall and announced omissions; skip Dolt and compaction.
- [Markdown knowledge stores](./markdown-knowledge-stores/research_summary.md): layouts, ids, front matter and links across Markdown-native tools. Recommends a layout and schema built around clean merges.
- [Agent memory systems](./agent-memory-systems/research_summary.md): how memory systems handle staleness, contradiction, writes and priming. Sorts mechanisms into files, derived index, model and server.
- [Source ingestion](./source-ingestion/research_summary.md): converters, Google Docs and Meet, provenance and caching. Start with local files and native Markdown export.
