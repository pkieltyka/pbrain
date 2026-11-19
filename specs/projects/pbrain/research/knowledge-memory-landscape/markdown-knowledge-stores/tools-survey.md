# Markdown-native knowledge tools: what each one actually does

Researched 2026-11-19 by @lead-dev-2. Each section gives the tool's storage layout, id scheme, front matter, links and maturity, with sources. "Inference:" marks my reading rather than what a source says.

## Basic Memory (basicmachines-co/basic-memory)

**What it is.** An MCP server and CLI (`bm`) that keeps an agent's knowledge as Markdown files and indexes them into SQLite (or Postgres) for full-text and vector search. AGPL-3.0, Python, about 4.6k stars, v1.0.0 released 2026-10-29, very active (pushed 2026-11-19) ([repo](https://github.com/basicmachines-co/basic-memory), [releases](https://github.com/basicmachines-co/basic-memory/releases)). There is a paid cloud and a Teams product; the local engine is the same ([README](https://raw.githubusercontent.com/basicmachines-co/basic-memory/main/README.md)).

**Source of truth.** "The system follows a file-first architecture where all knowledge is represented in standard Markdown files and the database serves as a secondary index" ([Technical information](https://docs.basicmemory.com/reference/technical-information)). `bm reindex --full` rebuilds it ([docs, Semantic Search](https://docs.basicmemory.com/llms-full.txt)).

**Note format** ([Knowledge Format](https://docs.basicmemory.com/concepts/knowledge-format)):

```markdown
---
title: Authentication Design
type: note
tags: [auth, security, backend]
permalink: authentication-design
created: 2026-08-01T14:30:00Z
modified: 2026-08-10T09:15:00Z
---
# Authentication Design

## Observations
- [decision] Using JWT tokens for stateless authentication #security
- [constraint] Tokens expire after 15 minutes to limit exposure window

## Relations
- implements [[API Security Requirements]]
- depends_on [[User Database Schema]]
```

- **Observations**: `- [category] content #tags (context)`; "There is no fixed list of categories". Each observation is indexed on its own, so search can return one fact from a long note ([Observations and relations](https://docs.basicmemory.com/concepts/observations-and-relations)).
- **Relations**: `- relation_type [[Target]]` in the body; an untyped inline `[[link]]` or (since v0.24) a plain Markdown link to a file in the project becomes a generic `links_to` edge. Links to notes that do not exist yet stay unresolved and reconnect when the note appears.
- **Valid time (v0.24)**: an observation can carry `@effective[2026-06-10,2026-07-27)`, `@occurred:`, `@due:` and so on; search takes `valid_at` / `valid_overlaps`. "Observations without a qualifier are excluded from any valid-time search — an undated fact makes no claim about when it holds" ([Metadata search](https://docs.basicmemory.com/concepts/metadata-search)).
- **Managed keys**: `permalink` ("Every Markdown note has one — as of v0.24 there is no setting to turn permalinks off"), `locked: true` (refuses API edits; "a guard against accidental AI edits, not access control"), `created_by` / `updated_by` (stamped by the server only in Cloud; "Local installs supply no author") ([Knowledge Format](https://docs.basicmemory.com/concepts/knowledge-format)).
- **Concurrency**: writes and edits accept `expected_checksum`; a stale one returns `REVISION_CONFLICT: '...' changed since you read it` ([MCP tools reference, in llms-full.txt](https://docs.basicmemory.com/llms-full.txt)).
- **Supersession**: there is no built-in supersession field. A user-defined schema can declare `supersedes?: ADR` and validation only warns ([Schema System example, llms-full.txt](https://docs.basicmemory.com/llms-full.txt)). Contradiction and stale-fact detection exist only as Cloud "agents" that write reports or edit notes into a review queue (same file, "Contradiction check").
- **Layout**: "Organize your files however you want — the knowledge graph is built from the content of your notes, not from where they sit on disk." Flat or foldered both work ([Projects and folders](https://docs.basicmemory.com/concepts/projects-and-folders)).

**Unflattering parts.**
- The indexer writes to source files. It adds a `permalink` to any note without one, and issue [#2125](https://github.com/basicmachines-co/basic-memory/issues/2125) (opened 2026-11-16, open) shows it re-serialises the whole front matter: "We pointed 1.0.0 at a copy of 300 notes from a real Obsidian vault: after indexing, all 300 files differed from the originals. On a vault that lives in git … that is one large diff". The same issue notes the body lost its trailing newline.
- The install pulls a FastMCP pre-release, FastEmbed and sqlite-vec. "FastEmbed and sqlite-vec are core dependencies of every install", and semantic search can no longer be turned off (v0.24) ([README](https://raw.githubusercontent.com/basicmachines-co/basic-memory/main/README.md), [Semantic search](https://docs.basicmemory.com/concepts/semantic-search)).
- Open issues on start-up cost: [#1731](https://github.com/basicmachines-co/basic-memory/issues/1731) (MCP import time exceeds client connect time-outs when several start at once) and [#2101](https://github.com/basicmachines-co/basic-memory/issues/2101) (closed; SessionStart hook compiling ~2,200 modules and exceeding its 20 s time-out).

**Relevance to pbrain.** Basic Memory shows that typed facts and typed relations can live in plain Markdown and be indexed into a rebuildable database. It also shows two risks pbrain should avoid: an index that rewrites source files, and a heavy runtime. Its observation-level indexing (one fact per bullet) is an idea worth weighing against pbrain's one-item-per-file model. Inference: for an agent on a context budget, a search hit that is a single bullet is cheaper than one that is a whole note.

## Open Knowledge Format (OKF), Google Cloud then community

**What it is.** A vendor-neutral spec for "knowledge as plain markdown files with YAML frontmatter, organized in a directory hierarchy". v0.2 lived at [GoogleCloudPlatform/open-knowledge-format](https://github.com/GoogleCloudPlatform/open-knowledge-format) (now deprecated). v0.3 is at [open-knowledge-format/open-knowledge-format](https://github.com/open-knowledge-format/open-knowledge-format) (Apache-2.0, repo created 2026-10-07, 43 stars, pushed 2026-11-18). Basic Memory exports it (`bm okf export`), and Backlog.md's companion Groma.md writes OKF 0.2 ([Backlog.md README](https://raw.githubusercontent.com/MrLesk/Backlog.md/main/README.md)). It is very young: inference, treat it as a vocabulary to borrow, not a standard to depend on.

**Key rules (v0.3 [SPEC.md](https://raw.githubusercontent.com/open-knowledge-format/open-knowledge-format/main/SPEC.md)):**
- Concept id = "The path of the concept's file within the bundle, with the `.md` suffix removed." Only `type` is required.
- A `.md` file without `type` is not a concept, "This rule lets a bundle contain files such as `README.md`, `AGENTS.md`, `CONTRIBUTING.md`, and `SKILL.md`." Dot-paths are not part of the bundle.
- Reserved `index.md` (directory listing for "progressive disclosure", may be generated) and `log.md` (date-grouped change log, newest first).
- Provenance: `sources: [{id, resource, title, author, usage_count, last_modified}]` plus per-claim footnotes keyed by `sources[].id`, "because agents constantly rewrite these documents: a positional index misattributes silently the moment the list is reordered".
- Trust: `generated: {by, at}` and `verified: [{by, at}]`, with actors written `<producer>/<version>`, `human:<id>` or `process:<id>`. Trust tier is derived: none, machine-confirmed, human-reviewed. "Trust tiers are advisory signals, not access control."
- Lifecycle: `status: draft | stable | deprecated`, and in v0.3 `superseded_by: <path>` ("Consumers that filter results to current knowledge SHOULD follow it, and give the replacement concept"). On a move or rename, "keep a short deprecated concept at the old path, with `superseded_by` set to the new path." An unknown status "is not `stable`".
- `stale_after` is an absolute instant: "Stale means that a person or a process must check the content again. It does not mean that the content is false." "Omit `stale_after` when no review date is known. Do not invent one."
- Links are plain Markdown links and untyped: "The specific kind … is conveyed by the surrounding prose". Typed relationships and validity windows are explicitly "Considered and deferred" (§12.2).
- YAML rules (v0.3 §13.2): YAML 1.2 core schema, no duplicate keys, quote values that look like numbers or booleans, "no re-serialization of unchanged values".

## Obsidian

A closed-source desktop editor over a folder of Markdown ("vault"). It is the de facto reader for these stores; Basic Memory and Karpathy's LLM-wiki pattern both use it as the viewer.
- **Properties** are YAML front matter with typed values (text, list, number, checkbox, date, date and time, tags). Defaults are `tags`, `aliases` and `cssclasses`. "Nested properties" and "Markdown in properties" are not supported, the latter "intentional … as properties are meant for small, atomic bits of information". Internal links inside properties must be quoted: `link: "[[Episode IV]]"` ([Properties](https://help.obsidian.md/properties)).
- **Links**: `[[wikilink]]` or Markdown links. Obsidian updates links on rename, but only renames done inside Obsidian ([Internal links](https://help.obsidian.md/links)). Inference: a rename by an agent with `git mv` breaks title-based wikilinks, an argument for ids that never change.
- **Scale**: users report vaults of 23,000+ notes working, with slowdowns usually blamed on plugins such as Dataview, and ~1,000 notes is "not … a large vault" ([forum, June 2026](https://forum.obsidian.md/t/performance-on-large-vaults/114864)). Anecdotal.

## Foam (foambubble/foam)

A VS Code extension, about 17.5k stars, active (vscode@0.49.0 on 2026-11-19) ([repo](https://github.com/foambubble/foam)).
- Special properties are only `title`, `type` (default `note`, used to style the graph), `tags` and `alias` ([Note properties](https://raw.githubusercontent.com/foambubble/foam/main/docs/user/features/note-properties.md)).
- Wikilinks are "identifier links" resolved by the shortest unique filename suffix. Ambiguous ones are resolved "alphabetically (deterministic), and shows a warning". Foam can generate link-reference definitions so wikilinks render in standard Markdown, and it rewrites links on rename ([Wikilinks](https://raw.githubusercontent.com/foambubble/foam/main/docs/user/features/wikilinks.md)).

## Dendron (dendronhq/dendron)

A VS Code note tool built on dot-delimited hierarchical filenames (`project1.tasks.task1.md`) instead of folders ([Hierarchies](https://wiki.dendron.so/notes/f3a41725-c5e5-4851-a6ed-5f541054d409.html)). Its required front matter is `id` (23-character random), `title`, `desc`, `updated` and `created` as Unix milliseconds ([Frontmatter](https://wiki.dendron.so/notes/ffec2853-c0e0-4165-a368-339db12c8e4b)). It went into maintenance mode in February 2023 after failing to find product–market fit ([Random Geekery, quoting the founder](https://randomgeekery.org/post/2023/02/dendron-is-officially-in-maintenance-mode/)); last push 2025-11-13. Lesson: hierarchy in the filename makes refactoring a rename of many files, and an editor-maintained `updated` field changes on every save. Inference: that is churn for Git with many writers.

## Architecture Decision Records: Nygard, MADR, adr-tools, log4brains

- **Nygard (2011)** ([post](https://www.cognitect.com/blog/2011/11/15/documenting-architecture-decisions)): one short file per decision, "numbered sequentially and monotonically. Numbers will not be reused." "If a decision is reversed, we will keep the old one around, but mark it as superseded. (It's still relevant to know that it was the decision, but is no longer the decision.)" Sections: Title, Context, Decision, Status (proposed, accepted, deprecated, superseded), Consequences.
- **MADR 4.0.0** (2024-09-17) ([template](https://raw.githubusercontent.com/adr/madr/main/template/adr-template.md)): optional front matter `status: "{proposed | rejected | accepted | deprecated | … | superseded by ADR-0123}"`, `date`, `decision-makers`, `consulted`, `informed`. Files are `nnnn-title.md` in `docs/decisions` ([README](https://raw.githubusercontent.com/adr/madr/main/README.md)). Supersession is a free-text status value pointing at the replacement.
- **adr-tools** ([README](https://raw.githubusercontent.com/npryce/adr-tools/master/README.md)): `adr new -s 9 …` "will create a new ADR file that is flagged as superceding ADR 9, and changes the status of ADR 9 to indicate that it is superceded by the new ADR". So it writes both directions. Last push 2024-04-25.
- **log4brains** (v1.1.0, 2024-12-17, low activity since) ([README](https://raw.githubusercontent.com/thomvaill/log4brains/master/README.md)): "an ADR is immutable. Only its status can change." It dropped numbers for `YYYYMMDD-slug.md` because of Git merge conflicts (see [ids-layout-and-git-merges.md](./ids-layout-and-git-merges.md)).

## zk (zk-org/zk) and nb (xwmx/nb)

- **zk** (GPL-3.0, v0.15.7 on 2026-10-15): a note's identity is its path, and the "note ID" is the unique part of the filename. Supported flavours: random (default four alphanumeric characters, "1 679 616 unique IDs") or timestamp `YYYYMMDDHHMM`. "Sequential (incremented) IDs are currently not supported by `zk`. They get ugly very quickly when deleting outdated notes" ([Note ID](https://raw.githubusercontent.com/zk-org/zk/main/docs/notes/note-id.md)).
- **nb** (AGPL-3.0, 7.25.6 on 2026-10-31): a Git-backed CLI notebook. Its numeric ids are line numbers in a tracked per-folder `.index` file: "the line number of each filename represents the id. `.index` files are included in the git repository so ids are preserved across systems". `nb index rebuild` warns "Some ids will change. Prefer `nb index reconcile`" ([README](https://raw.githubusercontent.com/xwmx/nb/master/README.md)). Inference: a tracked, positional index file is a merge hotspot when many writers append at once; pbrain should not copy it.

## Backlog.md (MrLesk/Backlog.md)

A Markdown task manager in Git (MIT, about 7.4k stars, active). Its ids are `TASK-1`-style with a prefix you can configure. Deleting "releases its ID once no other branch or worktree still holds the task", so it has to scan branches and worktrees to keep sequential ids safe ([README](https://raw.githubusercontent.com/MrLesk/Backlog.md/main/README.md)). Its read commands give "stable, versioned JSON". Inference: sequential ids across branches need tool-side coordination that pbrain can avoid with time-based ids.

## Karpathy's "LLM Wiki" pattern (gist, 2026)

An idea file, not a tool ([gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)). It has three layers: immutable raw sources, an LLM-maintained wiki of Markdown pages, and a schema file (CLAUDE.md or AGENTS.md) holding the conventions. Its operations are ingest, query and lint ("contradictions between pages, stale claims that newer sources have superseded, orphan pages"). `index.md` is a catalogue the LLM reads first, which "works surprisingly well at moderate scale (~100 sources, ~hundreds of pages) and avoids the need for embedding-based RAG infrastructure". `log.md` is append-only with greppable `## [2026-04-02] ingest | Title` headings. It names qmd for search at larger scale. Inference: it differs from pbrain on purpose. It rewrites synthesis pages in place, whereas factory's convention supersedes and never rewrites, and its single `index.md` is a shared write hotspot.
