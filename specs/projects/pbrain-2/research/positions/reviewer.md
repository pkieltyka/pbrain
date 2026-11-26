@operator — updated independent #pbrain-2 position from @reviewer, using the owner-approved project overview and still not reading other positions. This supersedes my earlier `brain-v2-position-83-1.md`; that was queued before the overview fixed memory-first ordering.

## Recommendation

Build memory into pbrain first, then add the separate docs corpus. Keep three distinct record classes: ephemeral memories, Git-authored docs, and typed reviewed items. Retrieval can be composed explicitly, but results must keep their class, source, scope and standing. Do not treat arbitrary Markdown or an agent memory as an item. pbrain remains the tool agents use to learn, with reviewable knowledge still in Git.

## (a) Docs-as-Markdown and front matter

“Any Markdown under configured docs roots is searchable” is a useful discovery contract, but it cannot mean “every file is an item.” Raw documents lack guaranteed pbrain IDs, kinds, authors, statuses, audiences, replacement links and valid item front matter. Applying strict item parsing rejects useful files; synthesizing metadata invents standing and can break `check`, graph/replacement, and personal/shared guarantees. Keep item roots and their strict schema unchanged.

Index raw docs as documents with a stable source path plus heading/line offsets. Do not infer item kind, status, author, decision-maker, audience or replacement edges. Moving a file updates its path identity/index entry; it is not a `replaces` event. A docs scan has its own parse/read diagnostics; `check` still validates typed items. Promotion into an item stays explicit and reviewable.

Require no front matter for a raw doc. Parse only a versioned, namespaced, allow-listed `pbrain:` block. No front matter means ordinary Markdown. Foreign YAML is preserved and ignored by pbrain. Partial/malformed `pbrain:` data warns and leaves the file searchable as plain text with no trusted metadata. Do not execute YAML tags or rewrite documents. Valid item front matter is recognized only in configured item roots. Test missing, partial, foreign, malformed and valid headers, including fields that resemble a private audience or verified decision.

## (b) Storage and teams across worktrees

Git Markdown remains the source of truth for docs and reviewed items. SQL files under `~/.cache/pbrain/<project>/` are derived docs indexes/vectors and can be rebuilt from a particular source snapshot. Cache identity must include repo/common-dir identity, configured docs roots and the exact worktree tree/content generation; two worktrees on different commits must not read each other’s stale index. Use guarded directory operations, per-generation locks and atomic publication. Never copy originals into the cache.

Memories differ: they are intentionally expiring local records, not rebuildable from Git. If stored in the per-user cache, cache eviction means memory loss and should be stated to users; it must never remove docs or items. A same-user, same-machine factory may share enrolled memory across linked worktrees using a key from the resolved brain/repository identity and separate shared/personal views. Do not imply sharing across OS accounts, hosts or network filesystems. Use bounded retention, concurrency-safe writes and explicit enrollment. Separate external brains and clones; a moved/rebound project must not resurrect prior private data. I would not put original docs or any reviewable authority in SQLite.

## (c) Memory, capture and safety

A memory is an expiring observation, handoff or blocker, not a durable fact. Provide `store` and `recall` by task/query, then forget. Start with explicit agent-authored memory and task-start recall. Add automatic lifecycle prompting only in a fresh, team-scoped worker whose inputs and tools are mediated. Do not harvest transcripts, tool output or mixed/private conversations. Typed factory events may supplement handoffs but remain unverified, source-labelled observations; a nonce or `source=factory` is attribution/replay control, not proof. An agent must inspect CI or other primary evidence before acting on a reported result. Secret scanning is defense in depth, never the privacy boundary. Unknown/private input taints the session before delivery and disables shared capture/injection. Personal memory is a physically separate view; shared operations never open it. Zero leak is a hard gate.

Promotion is an explicit bridge for authored memories only: preview exact bytes and destination, then use normal item rules and Git review. Never promote typed check reports, auto-promote, or let ephemeral memory set policy. For the owner’s Hermes research, study automatic skill creation/updates separately: generated skills are durable executable instructions, not short-lived memory. Require a diff and ordinary review before any skill edit. No remote service or required LLM summarizer.

## (d) CLI and existing users

Add `pbrain store/recall/forget/promote` for memories and `pbrain docs search/status/sync` for docs. A file under an enabled docs root should be discovered on sync, so v1 need not copy files through `docs add`. Preserve today’s `add/search/list/get/sync` item semantics and compatibility for at least one major release; update the skill additively. Keep unqualified `search` item-only until a clearly labeled combined-search contract is accepted. Every returned result identifies item/document/memory and its source/scope/standing. Do not silently mix results or hide lower-authority observations in a document result. Existing brain folders remain valid and empty memory state does not create a setup side effect.

## (e) Agent-facing file

The bundled skill should be the canonical product instruction. A generated `MEMORY.md`, `KNOWLEDGE.md` or `BRAIN.md` summary would duplicate/stale memory and consume context. If users demonstrate a need for a local entry point, provide an optional human-owned `BRAIN.md` with configured docs roots and example commands only—no generated facts, policy or full memory summary. Measure token cost and whether it prevents wrong calls before adopting it.

## (f) Build first / refuse

Memory-first sequence: first settle the memory record, view, retention and `store/recall` contracts; implement manual, opt-in store/recall with no hook and test same-user worktree concurrency, replay, expiry and private noninterference. Then qualify one narrowly mediated factory lifecycle adapter and evaluate on synthetic handoffs; do not enable automatic capture by default until it passes. Keep the owner’s Hermes research as a separate prerequisite for any auto-created/updated skill behavior. Next add a read-only docs index over configured Markdown roots, with path/heading citations and no original-file writes. Then evaluate docs retrieval on long documents against existing item search. Only after those are sound consider a combined query/API.

Refuse: treating raw Markdown as typed items; rewriting user documents; putting originals or reviewed knowledge in SQL/cache; silent mixed results; transcript harvesting; shared memory from mixed/private sessions; typed-report promotion; automatic skill edits without review; mandatory remote sync or LLM; or cache-sharing claims across OS users.

## (g) Risks and evaluation

Memory risks: cache eviction loses session memory; stale/contradictory reports mislead; false claims gain standing; replay/expiry or concurrent writes corrupt the store; host hooks leak personal context. Docs risks: stale index across worktrees, symlink/path escape, malformed front matter, moves breaking citations, huge inputs and scope errors. Compatibility risks: old skills/scripts depend on current verbs and output shape. Test the risky boundaries directly: mixed/foreign front matter; symlink and directory substitution; add/edit/move/delete during indexing; cache generation isolation across divergent commits; simultaneous linked-worktree writers; personal sentinels; replay and expiry; and promotion failures.

Measure memory separately with paired factory handoffs: correct next action/task/source/abstention, duplicated work, wrong durable conclusions, irrelevant injection, ineligibility rate, round trips/tokens, p50/p95 recall and sender delay. Require zero private/secret leaks, no verified conclusions derived from memory, and no lost/duplicated acknowledged event. Measure docs with fixed long-document questions and evidence citations, retrieval recall/precision, stale-result rate after changes, latency and cache cost versus current item search. A performance or task-success gain never compensates for a leak or false standing.

Disagreement to carry into the merged proposal: same-user local sharing is appropriate for v1; cross-account/host team memory is a different trust and synchronization product, not a directory-sharing tweak.

@operator — Hermes research amendment to my independent pbrain-2 position. I read `specs/projects/pbrain-2/research/hermes/research_summary.md`; this supersedes the Hermes-related portions of my earlier draft.

The research makes me more firmly oppose an unattended learning writer. Hermes Agent’s stock loop forks a background agent every ~10 turns/tool steps to patch/create skills and small memory files, unreviewed by default. The research finds no controlled evidence that self-written skills improve outcomes generally (curated human-written skills help; self-generated skills showed little benefit in one benchmark), while documenting skill bloat/sticky wrong rules, silent rewrites, memory pollution, echoed secrets and 21–66% extra input tokens. These results are not proof pbrain’s loop cannot help, but they make “agent feels smarter” anecdotes insufficient for adoption.

Borrow the good mechanics: the prompt asks for grounded lessons, not logs; update before create; deduplicate repeated lessons; keep startup memory a small frozen snapshot; retain provenance; never auto-delete; and make docs a promotion target. Use the designed Stop handoff and bounded SessionStart recall, not a periodic background fork. Keep a static human-written `BRAIN.md` pointer if useful; do not auto-write/commit `MEMORY.md` or `SKILL.md`. Generated skills are executable shared instructions and need an explicit diff plus normal Git review.

For the memory-first build, keep manual `store`/`recall` first, then qualify one lifecycle-authored worker. Make an authored handoff promotable either to a typed reviewed item or to a raw docs draft, with exact preview and content-hash compare-and-swap; the docs path must stay visibly a draft/proposal and preserve source/provenance. A typed check report must never become a verified decision. All durable outputs go through ordinary Git review; there is no unattended skill or doc writer.

The first snapshot should be bounded and injected once at session start (the research cites Hermes’s 2,200-character MEMORY and 1,375-character USER caps; pbrain’s current proposal is 3 rows/1,200 characters). Measure extra tokens as part of the task-resumption evaluation; the reported 21–66% overhead is a risk signal, not a pbrain estimate. Add a holdout arm for any later self-authored skill proposal before enabling it, and track bloat, stale/wrong rules, secret leakage, no-op writes and review burden as failures. Hermes’s lack of controlled evidence strengthens the requirement to measure from the first pbrain worker run.

No change to my docs/front-matter, cache/worktree, compatibility, or safety position: docs remain separate raw Markdown with optional allow-listed namespaced metadata; indexes are derived/cache-local; memories are expiring and same-user shared only; personal scope is isolated. No Hermes software was run.
