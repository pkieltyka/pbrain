# Markdown knowledge stores

## Bottom Line

The Markdown-native tools agree on the basics: one file per item, YAML front matter with a free `type`, edges written in the files, and any graph or search index derived and rebuildable (Basic Memory, OKF, Obsidian/Foam, ADRs). For a store with many agent writers on parallel branches, what matters most is merge behaviour, not features. Sequential numbers collide across branches, which made log4brains switch ADR ids to `YYYYMMDD-slug`. Shared index or log files conflict on most merges, and an indexer that rewrites front matter (Basic Memory #2125) produces repo-wide diffs. Factory's existing convention (`<UTC YYYYMMDD-HHMMSS>-<slug>.md`, `created`, `author`, `source`, `status` with superseded-by) is already close to the best pattern found. Recommended additions: a short random suffix or a duplicate-id check, a fixed `kind` vocabulary, a separate `decided_by` / `verified`, and `supersedes` on the new item. Never add an auto-updated timestamp or a tracked hand-maintained index. Google's new Open Knowledge Format (v0.3, Nov 2026) is the most useful outside vocabulary for provenance, trust and lifecycle fields, but it is weeks old and should be borrowed from rather than depended on.

## Key Findings

- **The file is the truth, the index is derived.** Basic Memory: "the database serves as a secondary index" ([technical info](https://docs.basicmemory.com/reference/technical-information)). OKF: "If you can `cat` a file, you can read OKF; if you can `git clone` a repo, you can ship it" ([SPEC v0.3](https://raw.githubusercontent.com/open-knowledge-format/open-knowledge-format/main/SPEC.md)).
- **Sequential ids break with parallel branches.** log4brains switched to `YYYYMMDD-slug` for this reason ([ADR](https://thomvaill.github.io/log4brains/adr/adr/20201016-use-the-adr-slug-as-its-unique-id/)). clan-core shipped three "ADR-3"s ([ADR 03](https://clan.lol/docs/26.05/decisions/03-adr-numbering-process)). zk refuses sequential ids ([Note ID](https://raw.githubusercontent.com/zk-org/zk/main/docs/notes/note-id.md)). Backlog.md has to scan branches and worktrees to keep `TASK-n` safe.
- **Shared files are merge hotspots.** Every new item that edits one index or log file conflicts. GitLab's changelog broke every open MR, and union merge "leave[s] the added lines … in random order" ([GitLab](https://about.gitlab.com/blog/2015/02/10/gitlab-reduced-merge-conflicts-by-90-percent-with-changelog-placeholders), [gitattributes](https://git-scm.com/docs/gitattributes)). OKF lets `index.md` be generated, and Claude Code's single `MEMORY.md` index is safe only because it is machine-local.
- **The indexer must not write to sources.** Basic Memory 1.0 writes `permalink` into front matter and re-serialises the block; "all 300 files differed from the originals" ([#2125](https://github.com/basicmachines-co/basic-memory/issues/2125)). OKF v0.3 now requires "no re-serialization of unchanged values".
- **Ids should never change.** Wikilinks only survive renames made inside Obsidian or Foam. OKF says to leave a deprecated stub with `superseded_by` when moving. So: id = filename stem, never renamed, and a change of meaning becomes a new item.
- **Supersession is common, but there is no standard form.** MADR and factory put "superseded by X" in `status`, OKF v0.3 has `status: deprecated` + `superseded_by`, adr-tools writes both files, and Basic Memory has no built-in field. "Keep the old one around, but mark it as superseded" (Nygard 2011).
- **Confirmed is separate from written.** OKF keeps `generated.by` apart from `verified[].by`, and derives a trust tier from whether a `human:` actor verified. MADR records `decision-makers`. This matches pbrain's "agents propose; the authority decides; the item records who".
- **No tool stores a confidence score.** OKF refuses to: "a score is subjective, unportable across consumers, and goes stale". It stores objective signals and the consumer infers trust. Freshness is an absolute `stale_after`, meaning "check again", not "false".
- **Typed links stay few and in front matter.** OKF deferred typed relations and treats path fields as edges. Basic Memory's body-relation syntax needed several rounds of disambiguation rules. A few front-matter keys plus untyped inline links is enough for "what replaced X" and "what cites S".
- **Context is the binding scale limit.** Claude Code loads 200 lines or 25 KB of its memory index, and Codex loads 32 KiB of AGENTS.md. Karpathy's hand-read `index.md` works to "~hundreds of pages". Nothing found shows file counts as a constraint at project-brain scale.
- **Vendors separate instructions from memory and keep memory machine-local.** CLAUDE.md, AGENTS.md and rules are instructions. Auto memory, Codex memories and the Cursor journal are recall and are not committed. Codex: "Treat memories as a helpful recall layer, not as the only source for rules that must always apply." pbrain is unusual in committing shared memory to Git, so it alone must solve multi-writer merges.

## Recommended pattern (for the spec to decide)

**Layout (shallow; a folder means something only where pbrain enforces it):**
```
<brain>/                         # e.g. a folder in the project repo, location from config
  items/                         # shared, tracked; optionally items/<kind>/ for browsing
    20261119-182109-cache-uses-redis-k3f9.md
  private/                       # git-ignored; pbrain refuses to run if tracked
  sources/                       # ingested source items; raw cache tracked or not (owner's open question)
  .index/                        # derived (SQLite/FTS), git-ignored, rebuildable
```
- Id = filename stem `<UTC YYYYMMDD-HHMMSS>-<slug>[-<4 random chars>]`. It never changes and is not repeated in front matter.
- No tracked hand-maintained `index.md` or `log.md`. Generate them on demand, or leave them uncommitted. Git history is the log.
- Folders carry no authority, kind or status (factory: "A folder, author or status gives no authority").

**Front matter (flat; scalars or lists of ids; written only by explicit commands):**
```yaml
---
kind: decision            # fixed vocabulary, e.g. decision | learning | observation | note | source
title: Cache layer uses Redis   # one line; shown in search results without the body
created: 2026-11-19T18:21:09Z
author: agent:lead-dev-2  # actor convention à la OKF: agent:<name>, human:<id>
source: https://… | task/commit ref
status: provisional       # provisional | verified | superseded   (+ accepted/rejected for decisions?)
decided_by: human:pkieltyka     # decisions only; who had the authority
verified: [{by: human:pkieltyka, at: 2026-11-20T09:00:00Z}]   # optional
supersedes: [20261001-101500-cache-uses-memcached]            # on the NEW item
relates: [<id>, …]        # optional, untyped
stale_after: 2027-05-01   # optional; omit when unknown ("Do not invent one")
tags: [cache]             # optional
---
```

**Trade-offs to settle:**
- *Where supersession is written.* `supersedes` on the new item only: append-only, and merges cleanly because only a new file is added. But a reader who greps the old file alone cannot tell it is superseded without the index. Also stamping `status: superseded` on the old file (factory, adr-tools): self-describing, but it edits an existing file, and two branches superseding the same item conflict (arguably a conflict worth surfacing). Recommendation: the new item's `supersedes` is authoritative and the old item's status line is a convenience that pbrain writes in the same commit and checks for consistency.
- *Kind by folder or by field.* Folders are cheap to browse and `rg`, but moving an item changes its path. A field is authoritative and keeps paths stable. Recommendation: the field is authoritative, and a per-kind folder is optional and set at creation.
- *Item granularity.* One item per file (factory, OKF) or many facts per file (Basic Memory observations, indexed individually). One per file keeps supersession, authority and privacy at file level, which is simpler. Basic Memory's per-fact indexing returns smaller hits. Recommendation: one item per file, kept short, so a whole item is itself a small hit.
- *Adopting OKF names* (`type`, `generated`, `verified`, `superseded_by`, `sources[]`) gives compatibility with Basic Memory and Groma.md exports. Keeping factory's names (`kind`, `author`, `source`) means existing notes are adopted unchanged, which factory#62 requires. Possible middle path: keep factory's names and offer an OKF export later.

## Details

- [Tools survey](./tools-survey.md) — per-tool format, ids, links, maturity and known problems: Basic Memory, OKF, Obsidian, Foam, Dendron, ADR tools, zk, nb, Backlog.md and Karpathy's LLM wiki. Read it before borrowing from a specific tool.
- [Ids, layout and Git merges](./ids-layout-and-git-merges.md) — evidence on id collisions, rename stability, hotspot files, indexers writing to sources, and folder layout. Read it when deciding file naming and the write path.
- [Front matter and links](./front-matter-and-links.md) — a field-by-field comparison table across seven conventions, YAML pitfalls, typed links (front matter, body or inline), graph vs flat, and scaling. Read it when writing the schema.
- [Agent memory files](./agent-memory-files.md) — how Claude Code, Codex and Cursor lay out and load memory and instruction files, their budgets, and what that implies for pbrain's layout and session-start slice.

## Open Questions / Gaps

- No measured benchmark of `rg` or FTS5 over a Markdown store of a given size. The scale claims are inference or anecdote.
- Not verified: the superdev PR on silent duplicate ADR numbers (its URL returned 404). The mechanism is sound Git behaviour, but the citation is unconfirmed.
- Not verified: whether GitHub's server-side merge honours `merge=union` (only a discussion title was seen). It doesn't matter if pbrain avoids shared files.
- OKF is about six weeks old (v0.3 repo created 2026-10-07, 43 stars). Its adoption beyond Basic Memory and Backlog.md/Groma.md is unknown.
- Basic Memory's `created`/`modified` handling when several Git clones edit the same note was not tested.
- Logseq, Org-roam, Notion and Johnny.Decimal were not covered: they are either not plain Markdown in Git or not relevant to many agent writers.

## Sources

- [Basic Memory README](https://raw.githubusercontent.com/basicmachines-co/basic-memory/main/README.md) and [docs llms-full.txt](https://docs.basicmemory.com/llms-full.txt) — format, valid time, schemas, checksums; v1.0.0 (2026-10-29), fetched 2026-11-19
- [Basic Memory issue #2125](https://github.com/basicmachines-co/basic-memory/issues/2125) — indexer rewrites front matter; opened 2026-11-16
- [OKF v0.3 SPEC](https://raw.githubusercontent.com/open-knowledge-format/open-knowledge-format/main/SPEC.md) and [v0.2 (deprecated repo)](https://github.com/GoogleCloudPlatform/open-knowledge-format) — provenance, trust, lifecycle, index and log, supersession; Nov 2026
- [Obsidian Properties](https://help.obsidian.md/properties) and [Internal links](https://help.obsidian.md/links) — property types, link behaviour; current help
- [Foam wikilinks](https://raw.githubusercontent.com/foambubble/foam/main/docs/user/features/wikilinks.md) and [note properties](https://raw.githubusercontent.com/foambubble/foam/main/docs/user/features/note-properties.md) — current main
- [Dendron hierarchies](https://wiki.dendron.so/notes/f3a41725-c5e5-4851-a6ed-5f541054d409.html), [frontmatter](https://wiki.dendron.so/notes/ffec2853-c0e0-4165-a368-339db12c8e4b), [maintenance-mode notice](https://randomgeekery.org/post/2023/02/dendron-is-officially-in-maintenance-mode/) — Feb 2023
- [Nygard, Documenting Architecture Decisions](https://www.cognitect.com/blog/2011/11/15/documenting-architecture-decisions) — original ADR, 2011
- [MADR template](https://raw.githubusercontent.com/adr/madr/main/template/adr-template.md) — MADR 4.0.0, 2024-09
- [adr-tools README](https://raw.githubusercontent.com/npryce/adr-tools/master/README.md) — supersede command; last push 2024-04
- [log4brains README](https://raw.githubusercontent.com/thomvaill/log4brains/master/README.md) and [slug-id ADR](https://thomvaill.github.io/log4brains/adr/adr/20201016-use-the-adr-slug-as-its-unique-id/) — 2020; v1.1.0 2024-12
- [clan-core ADR 03 numbering](https://clan.lol/docs/26.05/decisions/03-adr-numbering-process) — duplicate ADR numbers in practice
- [zk Note ID](https://raw.githubusercontent.com/zk-org/zk/main/docs/notes/note-id.md) — id flavours; v0.15.7
- [nb README](https://raw.githubusercontent.com/xwmx/nb/master/README.md) — tracked `.index` positional ids; 7.25.6
- [Backlog.md README](https://raw.githubusercontent.com/MrLesk/Backlog.md/main/README.md) — sequential ids across branches, JSON output
- [Karpathy, LLM Wiki gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) — index.md and log.md pattern, scale anecdote; 2026
- [GitLab changelog conflicts](https://about.gitlab.com/blog/2015/02/10/gitlab-reduced-merge-conflicts-by-90-percent-with-changelog-placeholders), [gitattributes union](https://git-scm.com/docs/gitattributes), [a3nm merge drivers](https://a3nm.net/blog/git_auto_conflicts.html) — append-only file merges
- [Claude Code memory](https://code.claude.com/docs/en/memory) — CLAUDE.md, auto memory layout and budgets; fetched 2026-11-19
- [Codex AGENTS.md guide](https://developers.openai.com/codex/guides/agents-md) and [Codex Memories](https://developers.openai.com/codex/memories) — discovery, 32 KiB cap, local generated memories
- [Cursor Rules](https://cursor.com/docs/rules.md) and [Agent SDK memory](https://cursor.com/docs/agent-sdk/reference/memory.md) — rule front matter, journal caveats
- [agents.md](https://agents.md/) — the AGENTS.md convention
- [Obsidian forum: performance on large vaults](https://forum.obsidian.md/t/performance-on-large-vaults/114864) — anecdotal scale, June 2026
- [factory notes convention](/home/peter/Dev/pkieltyka/factory/skill/references/notes.md) — pbrain's starting format (local)
