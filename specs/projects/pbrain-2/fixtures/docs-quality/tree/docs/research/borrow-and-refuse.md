# What pbrain-2 should borrow from Hermes, and what to refuse

2026-11-26 · inputs for the pbrain-2 functional spec

The Hermes facts used here are sourced in [hermes-mechanisms-brief.md](./hermes-mechanisms-brief.md). The pbrain side comes from the [pbrain-2 overview](../../../project_overview.md), the approved session-memory design (`docs/plans/session-memory.md (peter-architect-2 worktree)`, #83), the PM position ([pm-position.md](../../pm-position.md)) and the Mnemosyne comparison (`specs/projects/pbrain/research/mnemosyne-comparison.md` in the peter-lead-dev worktree, #82). Fit, cost and risk are my assessment. "Cost" means engineering and runtime cost for pbrain, judged from the documents, not measured.

## The structural difference

Hermes is the harness. It owns the model loop, can fork the agent after any turn, and writes to stores in the user's home directory (`~/.hermes/memories`, `~/.hermes/skills`) that are personal and not in Git. Its approval gate is off by default. Most of its safety work came after the fact: a ledger, an archive instead of deletion, pin/lock, and gating the tools that fire on each trigger.

pbrain is a tool that harnesses call. Its durable knowledge is Markdown under Git review, shared across a team, with a strict boundary between shared and personal. The session-memory design already allows only lifecycle-triggered authorship: one host prompt at Stop, with the agent free to decline. It never harvests transcripts. Hermes's learning feels like a co-worker because three things happen without asking: something writes after every few turns, the next session starts with it already in the prompt, and wrong procedures get patched in place. pbrain can offer the second and a reviewed version of the third. It should not copy the first for shared content.

## Idea-by-idea

### 1. Periodic reflection nudge (background review fork)

- **Hermes.** Every 10 user turns or 10 tool iterations, a forked agent replays the conversation and writes memory or skills directly, with no review by default.
- **Fit: partial.** pbrain is not a harness and cannot fork the agent. The nudge maps onto the factory adapter's single Stop prompt, which is already designed (session-memory "Exact lifecycle"). A counter-based mid-session nudge could only come from the host or the skill text.
- **Cost.** The Stop prompt is low cost and already planned. A periodic fork costs a full replay of the conversation each time; Hermes budgets "75% of the review fork's resolved context window". pbrain cannot pay for it and should not.
- **Risk.** In #105921 an unattended fork deleted a user's permission rules. In #32858 the review instructions were recorded as the user's traits. On a shared channel both become team-visible errors. An unattended writer also breaks the session-memory rule that "scope precedes text generation".
- **Borrow.** The prompt discipline, not the mechanism. Put Hermes's grounding rule and do-not-capture list into the handoff prompt and the pbrain skill: store only what is grounded in a tool result or message you can point to; no negative claims about tools ("X is broken"); no environment-setup failures; no unresolved dead ends presented as a working method; no one-off narratives. Keep "If there is nothing useful to pass on, finish without storing anything" as a neutral option.
- **Refuse.** Hermes's activity bias ("Be ACTIVE … A pass that does nothing is a missed learning opportunity"), which #30220 identifies as a false-positive driver. Refuse unattended forks that hold write or forget power. Refuse any automatic forget or consolidation. Refuse sending the host instruction as `user`-role text that could be recorded as content. Add a cheap admission check that rejects a store whose text repeats the hook prompt.

### 2. Agents writing and updating their own skills through pbrain

- **Hermes.** `skill_manage create|patch|delete|write_file` into a personal skills directory, with progressive disclosure, update-before-create and enforced read-before-write.
- **Fit: low as stated; good if recast as docs.** pbrain does not own the harness's skill directory (`.claude/skills`, `.agents/skills`), which is configuration for each harness. In this repo some of those paths are deliberately write-protected. A pbrain-2 "how-to" document under `./docs` is already indexed and searchable, and it is the natural reviewed form of a learned procedure.
- **Cost.** Moderate. It needs a doc-shaped promotion target and guidance text. A new store is not needed.
- **Risk.** Skills are executable instructions; they are the highest-leverage place for prompt injection. Hermes itself loads project skills from a Git checkout only for roots in `trusted_project_dirs`, and its scanner is advisory. An agent-written procedure that lands without review would steer every later agent on the team.
- **Borrow.** (a) **The skill-shape rules as doc-writing guidance in the pbrain skill:** procedure first; a pitfall is a rule plus one clause of why; no PR numbers or dates as content; one rule per lesson; do not restate AGENTS.md or tool schemas; fix the wrong sentence in place instead of appending "UPDATE". (b) **Update before create:** before proposing a new doc, `pbrain docs search` for one that covers the class of task and propose an edit to it. (c) **Read-before-write as compare-and-swap:** if pbrain ever writes into an existing doc, require the content hash the agent last read and refuse on mismatch.
- **Refuse.** pbrain writing into any harness skill directory. Skill or doc edits that land without Git review. Deletion by an agent. Treating a passing content scan as approval (Hermes's own docs call its guard "not an approval gate").

### 3. Promotion of memories into skills or docs

- **Hermes.** There is no explicit memory-to-skill promotion. The same review pass writes either one straight from the transcript, guided by a routing rule: memory is "who the user is and what the current situation … is"; skills are "how to do this class of task".
- **Fit: high.** pbrain already designs authored-only `promote` into reviewed items, with preview and confirm, frozen bytes, refusal of typed reports and no crossing between views. pbrain-2 adds docs as a first-class context, so the obvious extension is a third target: a doc (new file or edit proposal) under a configured docs root, delivered through the same guarded writer and normal Git review.
- **Cost.** Moderate. It reuses the promote machinery. A target path inside the docs roots needs validation plus a front-matter rule (the docs front-matter policy is still open in the overview).
- **Risk.** Personal memory reaching a shared doc. That must be impossible, and it already is if promote keeps its "never route private memory to shared" rule. A second risk is a generated doc presented as reviewed; Git review is the gate, and the PR diff makes it visible.
- **Borrow.** Hermes's routing sentence as a three-way rule in the skill: a memory is task state that expires; an item is a decision or learning someone stands behind; a doc is how to do a class of work. One fact goes in one place (#30220).
- **Refuse.** Automatic promotion, LLM consolidation rewriting docs, and promotion of typed reports (already refused).

### 4. A MEMORY.md-style agent-facing file

- **Hermes.** `MEMORY.md` (2,200 chars) and `USER.md` (1,375 chars) are agent-written, user-scoped, outside the project, and injected as a frozen snapshot at session start. Project instructions are a separate, human-written `AGENTS.md` or `.hermes.md`.
- **Fit: split it in two.** Hermes's real equivalent of a checked-in file is `AGENTS.md`, which is human-written. Its `MEMORY.md` is a personal, bounded, generated snapshot. pbrain-2 should keep the two separate.
  - **Checked-in `BRAIN.md` (or a section in AGENTS.md/CLAUDE.md): human-written, static.** It explains the two contexts (memory vs docs), where docs live, and when to store, recall and promote, and it points to the skill. This matches the PM position's "tiny checked-in pointer".
  - **Dynamic snapshot: `pbrain recall --task N` at SessionStart**, bounded (the design says up to three rows and 1,200 characters), labelled untrusted, frozen for the session. This is Hermes's frozen snapshot, task-scoped and shared-safe.
- **Cost.** Low for both. The second is already designed.
- **Risk.** A generated memory file committed to Git causes merge churn across worktrees, leaks personal context into history permanently (a later forget cannot erase Git history) and goes stale. A `USER.md` analogue invites trait inference; Hermes needed an explicit grounding rule to stop "trait guesses".
- **Borrow.** Hard character caps on anything injected. A frozen snapshot at session start, with mid-session writes visible only through explicit recall (this also keeps prompt caches stable). The project-vs-user split.
- **Refuse.** An agent-written `MEMORY.md` in the repository. Any shared `USER.md`-style profile. Personal preferences, if stored at all, stay personal (`--personal` and the personal root) and are never inferred.

### 5. Recall injection

- **Hermes.** A frozen snapshot of the memory files, plus provider prefetch, plus `session_search` over every past transcript (FTS5, returning raw messages).
- **Fit: high for the snapshot, none for transcript search.** The SessionStart recall in the session-memory design is the right analogue. Docs could optionally join it: the top one or two doc hits for the task title, with path and heading citations.
- **Cost.** Low for memory, which is already designed. For docs it is modest, but it consumes context budget and should be measured in the planned 24-task evaluation before it becomes a default.
- **Risk.** Injected text is a prompt-injection channel. Hermes scans memory entries for injection, exfiltration and invisible Unicode before accepting them. pbrain's `secretscan` covers tokens, not injection.
- **Borrow.** Admission-time rejection of invisible or control Unicode and of obvious injection patterns for shared memory, on top of secretscan. Label injected rows untrusted (already designed).
- **Refuse.** `session_search`-style storage of every conversation for shared use. The approved design refuses transcript ingestion, and the PM position refuses it for v1. A personal-only transcript index is a separate, later question.

### 6. Curator: usage tracking, staleness and archiving

- **Hermes.** Deterministic marking of skills as stale after 14 days and archived after 30; it never deletes. Pin and lock. LLM consolidation is off by default. A ledger records before and after hashes.
- **Fit: partial.** Memory already expires by TTL (7 or 14 days), which is simpler and stricter. For docs, Git is the ledger and the archive, so pbrain needs neither. What carries over is a **read-only hygiene report**: docs never returned by search in N days, or docs whose cited paths no longer exist. pbrain reports and a person acts.
- **Cost.** Low for a report. Retrieval counts would be a new local, per-user record.
- **Risk.** The session-memory design rules out "ranking by past reads", so usage data must never feed ranking. Usage counts are personal telemetry and must stay in the per-user cache.
- **Borrow.** "Never auto-delete; the worst outcome is recoverable", together with pin/lock as a concept: a doc front-matter flag that marks a doc as human-owned and excludes it from agent-proposed edits.
- **Refuse.** Automatic archiving or rewriting of docs, and LLM consolidation.

### 7. Write approval, provenance and audit

- **Hermes.** `write_approval` is off by default and staged in a pending directory; generic file tools bypass it (#60440). Writes are tagged foreground or background_review and attended or unattended. Skills have a hash ledger and rollback.
- **Fit: high, mostly already ahead.** pbrain's Git review is a stronger, external gate. Its preview and confirm with frozen bytes is the staged write. Its `source`, `actor` and `session` fields carry provenance.
- **Borrow.** (a) Record **how a memory came to exist**: prompted at a lifecycle event, deliberate manual store, or typed report. Expose it in recall. Hermes's origin and attendedness split is the precedent, and its rule that "unattended review cannot delete" matches pbrain refusing forget and promote from hooks. (b) The lesson of the gate bypass: put the gate at the store boundary and in Git, not in one tool. pbrain already does this by keeping durable writes behind the guarded writer and review.
- **Refuse.** Any default-off gate for shared writes.

## Evidence that favours pbrain's Git-first stance

In #105921 the deletions were found only "by diffing `MEMORY.md` against the last automatic git commit. Without that repo the content would have been unrecoverable." Hermes users added Git after the fact to get what pbrain has by design. That supports keeping durable knowledge in reviewed Markdown and keeping session memory ephemeral and non-authoritative.

## What the "co-worker" feeling needs that pbrain can give

1. **The next session starts already knowing.** Frozen, bounded task recall at SessionStart (designed). Optionally add top doc hits (measure first).
2. **Corrections stick.** Hermes routes user corrections into the skill that governs the task. pbrain's equivalent is a skill rule: when a person corrects how a class of work is done, propose an edit to the governing doc (or a new how-to doc), through review.
3. **Low friction to record.** One prompted handoff at Stop (designed), plus a clear rule in the skill for when to `store` during a turn.

Refused across the board: unattended writers, transcript harvesting into shared memory, inferred user profiles, a committed generated memory file, approval gates that are off by default, automatic deletion or consolidation, and pbrain writing harness skill files.
