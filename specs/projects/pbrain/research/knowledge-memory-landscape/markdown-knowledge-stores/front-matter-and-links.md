# Front matter fields, typed links, and graph vs flat

Researched 2026-11-19. "Inference:" marks my reading.

## 1. Which fields the tools use

| Concern | Factory notes (today) | Basic Memory | OKF v0.3 | MADR 4 | Dendron | Foam / Obsidian | Claude Code auto memory |
|---|---|---|---|---|---|---|---|
| Kind | `kind`: observation, learning, decision (activity folder only) | `type` (free) + per-observation `[category]` | `type` (only required key, free text) | the file *is* a decision | – | `type` (Foam, styles the graph) | `type`: user, feedback, project, reference |
| Title | first heading | `title` | `title`, `description` (one line) | H1 | `title`, `desc` | `title`, `aliases` | `name`, `description` |
| Id | filename | `permalink` (written by the tool) | path without `.md` | `nnnn` in filename | `id` (random 23 chars) | filename | filename |
| Created | `created` (with time zone) | `created` (optional) | `generated.at` | `date` (last updated) | `created` (ms epoch) | – | – |
| Updated | – | `modified` (optional) | `generated.at` = "last meaningful change" | `date` | `updated` (auto) | – | `modified` (stamped on write) |
| Author | `author` | `created_by`/`updated_by` (Cloud only) | `generated.by` actor | `decision-makers`, `consulted`, `informed` | – | – | – |
| Verification | `status: verified` | – | `verified: [{by, at}]` → derived trust tier | `status: accepted` | – | – | – |
| Lifecycle | `status`: provisional, verified, `superseded-by <file>` | – (schema can add) | `status`: draft, stable, deprecated; `superseded_by` | `status`: proposed, rejected, accepted, deprecated, "superseded by ADR-0123" | – | – | – |
| Freshness | – | per-observation valid time `@effective[a,b)` | `stale_after` (absolute instant) | – | – | – | – |
| Source | `source` (task, commit, request or URL) | `source` + checksum on extracted docs | `sources: [{id, resource, title, author, usage_count, last_modified}]` + keyed footnotes | links in "More Information" | – | – | – |
| Tags | – | `tags` + inline `#tags` | `tags` | – | `tags` | `tags` | – |
| Edit guard | – | `locked: true` | – | – | – | – | – |

Sources: [factory notes.md](/home/peter/Dev/pkieltyka/factory/skill/references/notes.md); [Basic Memory Knowledge Format](https://docs.basicmemory.com/concepts/knowledge-format); [OKF v0.3 SPEC](https://raw.githubusercontent.com/open-knowledge-format/open-knowledge-format/main/SPEC.md); [MADR template](https://raw.githubusercontent.com/adr/madr/main/template/adr-template.md); [Dendron Frontmatter](https://wiki.dendron.so/notes/ffec2853-c0e0-4165-a368-339db12c8e4b); [Foam note properties](https://raw.githubusercontent.com/foambubble/foam/main/docs/user/features/note-properties.md); [Obsidian Properties](https://help.obsidian.md/properties); [Claude Code memory](https://code.claude.com/docs/en/memory).

**Observations:**
- Every tool has a free-text `type`/`kind`. Only factory (three kinds), Claude Code (four) and MADR (decisions only) fix a vocabulary. OKF says "consumers MUST tolerate unknown types gracefully". Inference: pbrain should fix a small vocabulary, since its supersession and authority rules depend on kind, and should report unknown kinds rather than silently accept them.
- **Who decided** appears only in MADR (`decision-makers`) and in OKF's `verified.by`, which separates *wrote* from *confirmed*: "who *wrote* a concept need not be who *confirmed* it". That maps directly onto pbrain's "agents propose; whoever has the authority decides; the item records who" (factory#62).
- **Supersession** is written three ways: a status value that names the replacement (MADR, factory), separate fields (OKF `status: deprecated` + `superseded_by`), or both files updated by the tool (adr-tools). None of the tools surveyed records `supersedes` on the new item as the only source. adr-tools writes both directions. Basic Memory's sample ADR schema has `supersedes?` on the new item.
- **Time** comes in three senses: written (`created`, `generated.at`), confirmed (`verified[].at`), and valid (Basic Memory's `@effective[a,b)`, OKF's `stale_after`). OKF deferred validity windows and a `refuted` event to a later version.
- **Weighting / confidence**: no surveyed Markdown tool stores a numeric confidence or score. OKF explicitly refuses: "It does not store a credibility score: a score is subjective, unportable across consumers, and goes stale. Credibility is *inferred* from the signals". Inference: this supports pbrain putting authority and status in files and computing any weighting (retrieval counts, recency) in the derived index.

## 2. YAML pitfalls

- Obsidian does not support nested properties or Markdown inside properties, and links in properties must be quoted ([Properties](https://help.obsidian.md/properties)). Inference: keep front matter flat with scalar or list values, so it renders in Obsidian and GitHub and greps line by line.
- OKF v0.3 §13.2: YAML 1.2 core schema; quote values that look like numbers or booleans; "no re-serialization of unchanged values". Timestamps are RFC 3339 with an offset, or date-only meaning the start of that day in UTC.
- Basic Memory's re-serialisation bug ([#2125](https://github.com/basicmachines-co/basic-memory/issues/2125)) shows the risk of round-tripping YAML through a dumper.

## 3. Typed links: front matter or inline

| Option | Used by | For | Against |
|---|---|---|---|
| Typed keys in front matter (`supersedes: [id]`, `source: url`) | factory (`status: superseded-by`), OKF path fields (`superseded_by`, `sources[].resource`), MADR (status text) | machine-readable without parsing prose; one line to grep; easy for an index to turn into edges (OKF §6.4: "A path field … that resolves to a concept in the bundle" is a graph edge) | not shown as links in GitHub or Obsidian unless quoted wikilinks |
| Typed list items in the body (`- depends_on [[X]]`) | Basic Memory | readable, many relation types, renders as a list | needs a custom parser; free vocabulary; prose in front of a link is ambiguous (Basic Memory had to add quoting rules and a `#bm:links_to` escape in v0.21–0.23) |
| Untyped inline links | OKF, Foam, Obsidian, Karpathy | normal Markdown; renders everywhere | edge type is only in prose: "The specific kind … is conveyed by the surrounding prose, not by the link itself" (OKF §6.1) |

OKF deferred typed relationships: "Use links, path fields (§6.4), and extension keys (§4.1) until the community agrees on one form" (§12.2).

**Inference.** For pbrain, a handful of typed keys in front matter carries the semantics the tool enforces: `supersedes`, `source`, `relates`. Inline Markdown links in the body are allowed and indexed as untyped "mentions". Body relation syntax (Basic Memory style) adds parsing and ambiguity for no gain while the typed vocabulary is this small.

## 4. Graph vs flat files

- Basic Memory has a "real knowledge graph", but its edges come from text patterns and it is stored as a derived SQLite index. `bm_context` follows edges to build context ([Observations and relations](https://docs.basicmemory.com/concepts/observations-and-relations)).
- OKF's graph is "two kinds of directed edges", links and path fields, computed by the consumer (§6.4). No graph is stored.
- Karpathy's wiki uses a hand-kept `index.md` and plain links, and says this works to "~hundreds of pages" without embeddings ([gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)).
- factory#62's recommendation: "No graph database. A few typed links in front matter (`supersedes`, `source`, `relates`)".

**Inference.** Every tool surveyed stores edges in the Markdown and treats any graph as derived. A "graph" for pbrain is therefore just the index's edge table built from front-matter keys and body links. That is enough for the queries pbrain needs: "what replaced X", "history of X" and "what cites source S". Multi-hop traversal (Basic Memory's `bm_context`) is a later, optional reader feature that needs no change to the format.

## 5. Scaling limits

- **Agent context, not the files, is the binding limit.** Claude Code loads only "The first 200 lines of `MEMORY.md`, or the first 25KB" at start-up ([memory docs](https://code.claude.com/docs/en/memory)). Codex stops concatenating AGENTS.md at `project_doc_max_bytes` "(32 KiB by default)" ([Codex AGENTS.md guide](https://developers.openai.com/codex/guides/agents-md)). Anything listing the whole store hits these caps long before the filesystem or Git struggles.
- **Hand-read indexes** work to "~hundreds of pages" (Karpathy, anecdotal).
- **Editors**: Obsidian users report 23,000+ notes working, with slowdowns tied to plugins ([forum](https://forum.obsidian.md/t/performance-on-large-vaults/114864)). Anecdotal.
- **Indexing**: Basic Memory's open performance issues are about search ranking and process start-up ([#1746](https://github.com/basicmachines-co/basic-memory/issues/1746), [#1731](https://github.com/basicmachines-co/basic-memory/issues/1731)), not file counts.
- I found no benchmark of plain `rg` over a Markdown store of a given size. Inference, unmeasured: a project brain of hundreds to low thousands of small files is well within what `rg` or SQLite FTS5 handle interactively. The real limit is how much an agent can afford to read, which is why bounded results and get-by-id matter more than the storage engine.
