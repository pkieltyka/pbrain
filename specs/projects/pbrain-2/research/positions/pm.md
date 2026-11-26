# Independent PM position: pbrain as docs + memory

**Status:** draft position for the owner's build-vs-shape discussion. This is my independent view, not an approved design. I have not read the other team members' positions.

**Owner scope update (received after this independent position):** the approved pbrain-2 overview covers memory and docs, with memory first. My sequencing recommendation below differs: I would establish the docs corpus first because its Git-backed source and freshness rules are simpler to validate. For the merged proposal, memory-first is the owner's constraint; preserve docs search as a non-regression requirement. My suggested memory-first slice is manual store/recall/forget before automatic capture.

## Recommendation

Build a **two-context product**: keep pbrain's curated, reviewable items and add a distinct document corpus plus a distinct session-memory layer. Do not make arbitrary Markdown pretend to be typed items, and do not make ephemeral session memory the record for durable project knowledge. Preserve today's item commands and schemas during migration; introduce `docs` and memory verbs alongside them, then consider deprecation only after measured adoption.

The product goal should be measurable task continuity: on a later, related task, an agent should retrieve a useful prior fact or decision more often, with fewer repeated investigations and no increase in privacy leaks or unsupported claims. More stored rows alone is not evidence that agents learned.

## (a) Docs: arbitrary Markdown versus typed items

Indexing any Markdown in configured documentation roots is useful. Requiring every note to use pbrain front matter would defeat the simple “drop a file under docs and find it” workflow. But a document is not automatically a `decision`, `learning`, or `note` item. Today those kinds carry stable IDs, author/status, audience, replacement/standing relationships, and validation rules. Generic docs often have none, may use foreign YAML front matter, and may contain links that are not pbrain item references. Treating them as items would either reject ordinary files or synthesize misleading metadata and break item semantics.

Keep two result types and indexes: typed items retain required schema, IDs, `replaces`/standing rules, author, and shared/personal authorization; docs have path-and-content-derived index identity and return file path plus heading/line citation. No fake item ID, kind, audience, or replacement relation should be invented for a document.

Front-matter policy proposal:
- No front matter: index the Markdown body with title derived from the first heading or filename; preserve the whole source file.
- Partial or foreign front matter: index it as document content and do not interpret unknown keys. Recognize only a namespaced, versioned `pbrain` block with documented keys.
- A malformed or unsupported pbrain visibility/audience declaration: fail closed for that file (skip from shared results and report a diagnostic); never silently downgrade it to shared.
- A valid pbrain block may supply title/tags and explicitly classify a document as shared or personal. Personal roots and filters must be applied before opening/indexing content, using the existing guarded path model.
- Moves are remove-old-path/add-new-path during refresh. Documents do not inherit typed-item `replaces` semantics; links between them are ordinary Markdown links unless a later explicit relation is designed.

## (b) Where state lives

Original documentation stays where people edit it, normally in Git under configured folders (default `./docs`). The cache under `~/.cache/pbrain/` holds only rebuildable indexes, vectors and metadata. It is not a second source of truth.

A single cache key should not conflate worktrees. Two worktrees can share a Git common directory while containing different branches and document bytes. Key a docs index by canonical checkout/brain root, relevant configuration, and content identity; use locks per derived index. A same-user session-memory namespace can instead key by canonical repository/common-dir identity plus the project-relative brain path, so factory agents in sibling worktrees can share it intentionally. External `--brain` roots need canonical-path identity independent of the caller's checkout. Do not write session state into `.git`, and do not imply cross-user sharing from a per-user cache.

Memory records and locks belong in a separate per-user, per-project cache namespace with an explicit schema/version, bounded size, atomic snapshot/lock protocol, and crash recovery. The cache may be deleted and rebuilt without deleting docs or reviewed items. A team-shared handoff is shared among same-user agents only; broader team sharing requires a separate, explicit transport and threat model.

## (c) Memories

A memory is a compact, time-bounded observation or authored handoff for future task continuity, not a reviewed item and not proof that a check passed. It can point to an authoritative file, item, CI run, or message. Items remain the durable place for reviewed facts, decisions and learnings.

Start with explicit `store`, `recall`, and `forget`, with personal scope as the safe default and an explicit shared scope where allowed. Store provenance (author, source, session/task, time, scope, expiry); typed check reports remain explicitly unverified and can never be promoted as evidence. Recall should support a query-free “recent for this task” mode and rank authored handoffs above unverified typed reports. Use recency, task/repository matching and a small importance signal before adding embeddings. Return compact citations and uncertainty, not raw transcript dumps.

Promotion is an authored workflow: preview a memory, independently write a decision/learning with a direct source citation, then use the normal guarded Git writer and human review. Typed reports must be refused from promotion. Forget is available per record; short default TTLs and size caps prevent unbounded accumulation, while no cleanup path deletes reviewable Markdown. Retention should be explicit and configurable, with a user-visible way to inspect and erase session records.

Capture should be phased. First ship explicit store/recall and a controlled factory handoff API. Then opt-in lifecycle capture for a fresh, single-task worker only, when the harness can prove that the input stream is shared-only. Never ingest whole raw transcripts, DMs, inherited history, or mixed/private sessions into shared memory. Admit short agent-authored handoffs and allow-listed typed bus facts; run secret scanning and scope checks at admission and again at write. Unknown input, a denied tool/read, or scope revocation stops shared capture for that session. Personal memory must never be returned in a shared view, regardless of ranking.

**Hermes research implications (operator-provided summary; not independently verified here):** Hermes uses an unattended background fork roughly every 10 turns/tool steps to write SKILL.md and small memory files. The reported evidence has no controlled demonstration that self-written skills improve performance; documented risks include skill bloat, sticky incorrect rules, silent rewrites, memory pollution, secret echo, and 21–66% additional tokens. Borrow its bounded session-start context, explicit capture prompts, provenance, non-destructive retention, and a reviewed promotion target. Do not copy the unattended writer or let ephemeral memory silently rewrite skill/docs files. Require an agent-authored handoff and explicit store/promotion boundaries, then measure token overhead and task outcomes against a no-memory baseline.

## (d) CLI and compatibility

Keep existing `add`, `search`, `get`, `status`, and `sync` behavior. Add `pbrain docs search` (and eventually docs add/list/get) for the corpus, plus `pbrain store`, `pbrain recall`, `pbrain forget`, and `pbrain promote` for memories. A namespace makes the object type clear; do not silently overload existing `search` to mix unrelated content. If results are combined later, label each source and make source selection explicit.

The existing skill and agent-facing schema remain compatible: old commands continue to work, while the skill teaches the new commands gradually. Existing projects without new config keep current typed-item behavior; docs indexing defaults to `./docs` only after an explicit sync/migration step or a clearly reported first-index action. Do not make a release silently crawl a large tree or expose personal files. Config migration should be additive and versioned.

## (e) Agent-facing file

A static `MEMORY.md`/`BRAIN.md` risks going stale, duplicating the skill, and consuming context on every launch. Keep the skill as the canonical explanation of when to store, recall, cite, and forget. If agents need a file in the repository, use a tiny checked-in pointer (for example `BRAIN.md`) that points to the skill and explains the two contexts; do not put generated memory summaries there by default. A future on-demand `pbrain context` can emit a bounded, current summary when the harness asks for it.

## (f) Build first / refuse first

1. Define document-vs-item identities, front-matter behavior, and cache boundaries; add fixture tests for absent/partial/foreign/malformed metadata, moves, and personal-root denial.
2. Add incremental docs indexing/search with path/heading citations and measurable freshness, preserving all old item behavior.
3. Add explicit personal `store`/`recall`/`forget`, task-scoped recall, provenance and TTL; use #80's catalog as the tool definition and #81's warm process only after the same calls work through the CLI.
4. Add authored-only promotion into existing reviewed Git items.
5. Add an opt-in factory handoff adapter after a 24-task privacy/freshness pilot; only then consider lifecycle hooks.

Refuse for v1: transcript ingestion, automatic promotion, shared memory from ordinary interactive panes, global host hooks, remote sync, memory-as-authority for CI/release gates, semantic memory ranking before deterministic recall is measured, and a single mixed docs/items/memories command that hides source or access scope.

## (g) Risks and measurement

Main risks: confusing a document with a reviewed decision; stale index after edits/moves; branch/worktree cache collisions; personal-path leaks; secrets in handoffs; false check claims; accidental long-term retention; and another agent interface that costs context without improving outcomes. Hermes adds a warning, per the supplied summary, that unattended self-authored skill updates can bloat or fossilize mistakes and add substantial tokens. Mitigate with separate schemas/results, content-based freshness, worktree-aware cache keys, fail-closed personal filters, secretscan, unverified provenance, TTL/forget tests, and one catalog shared by CLI/batch/MCP.

Measure on a fixed factory workload with repeated related tasks and clean baselines: compare current skill + typed items against docs-only, explicit memory, then opt-in handoff. Track task success and citation correctness; repeated investigation avoided; wrong/unsupported claims; invalid calls and round trips; input/output tokens; recall precision; stale/missing docs; forget/expiry behavior; and privacy/secret violations. Require zero cross-scope leaks and zero typed-report promotion in every arm. Report per-task outcomes and confidence, not only averages. Do not spend on a large model matrix until a small pilot shows a plausible improvement without safety regressions.
