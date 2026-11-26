# Lessons for pbrain-2

## Bottom Line

Hermes Agent (Nous Research, MIT, v0.23.4 of 2026-11-24) "learns by doing" by forking itself after every 10 user turns or 10 tool iterations. The fork replays the conversation and writes straight into personal, unreviewed stores in `~/.hermes` (`MEMORY.md`, `USER.md`, skills); the approval gate is off by default. pbrain-2 should borrow the parts that fit:

- the prompt discipline: grounded facts only, a do-not-capture list, one fact in one place, and update before create;
- the frozen, bounded snapshot injected at session start;
- skill-shape rules, recast as guidance for writing how-to docs;
- provenance of how each write came to exist;
- "never auto-delete".

It should not borrow the unattended writer. Hermes's own issues show that writer deleting a user's permission rules and recording its own instructions as the user's traits. pbrain's Git review and authored-only handoff are the safer form of the same loop. Promotion of memory should gain a third target, a doc under `./docs`, delivered through normal Git review. The agent-facing file should be a static, human-written `BRAIN.md` pointer, not a committed, agent-written `MEMORY.md`.

## Key Findings

- **The learning loop is a background fork, triggered by counters.** `memory.nudge_interval` and `skills.creation_nudge_interval` both default to 10. The fork writes "straight to the memory + skill stores" under a whitelist of tools. pbrain's equivalent is the single authored handoff prompt at Stop, already designed; a periodic fork does not fit a tool that harnesses call. [background_review.py](https://github.com/NousResearch/hermes-agent/blob/4b38ec1b664feb0f08da7e7d0971e962ced0d33b/agent/background_review.py#L1-L6) · [review_signals.py](https://github.com/NousResearch/hermes-agent/blob/4b38ec1b664feb0f08da7e7d0971e962ced0d33b/agent/review_signals.py)
- **The review prompts are the most reusable artefact.** Rules worth copying into the pbrain skill and handoff prompt: "Save only facts GROUNDED in this conversation", never infer user traits, no "Negative claims about tools", no unresolved dead ends presented as a working method, "The same lesson learned twice is ONE rule", fix in place. Refuse the "Be ACTIVE" bias, which #30220 blames for false positives. [Prompts L441-L660](https://github.com/NousResearch/hermes-agent/blob/4b38ec1b664feb0f08da7e7d0971e962ced0d33b/agent/background_review.py#L441-L660)
- **Unattended writers fail in ways that matter for a shared store.** In #105921 a skill-nudge fork deleted four standing permission rules from `MEMORY.md`; the user found it only by diffing against an automatic Git commit. In #32858 (open) the review prompt, sent with the `user` role, was saved as the user's preferences and synced to Honcho. These are arguments for pbrain's authored-only, Git-reviewed design. [#105921](https://github.com/NousResearch/hermes-agent/issues/105921) · [#32858](https://github.com/NousResearch/hermes-agent/issues/32858)
- **Hermes's MEMORY.md is personal, generated and bounded; its project file (AGENTS.md / .hermes.md) is human-written.** pbrain-2 should split the same way: a static checked-in `BRAIN.md` pointer, and a dynamic `pbrain recall --task` snapshot at SessionStart (the design caps it at three rows and 1,200 characters, frozen for the session). Do not commit a generated memory file: Git history cannot be forgotten, and the file would churn across worktrees. [Which file does what](https://hermes-agent.nousresearch.com/docs/user-guide/which-file-does-what) · [Memory docs](https://hermes-agent.nousresearch.com/docs/user-guide/features/memory)
- **Agent-written skills map to agent-proposed docs, not to pbrain managing harness skill folders.** Add a doc as a third `promote` target, alongside learning and decision. Adopt update-before-create (`pbrain docs search` first). Use read-before-write as a content-hash compare-and-swap. Hermes itself trusts project skills from Git only for roots listed in `trusted_project_dirs`, and calls its scanner "not an approval gate". [config_defaults.py](https://github.com/NousResearch/hermes-agent/blob/4b38ec1b664feb0f08da7e7d0971e962ced0d33b/hermes_cli/config_defaults.py#L1397-L1440)
- **The approval gate is off by default and leaks.** `write_approval` defaults to false and lives only in the memory and skill tools; generic file writes bypass it (#60440). pbrain's gate is at the store boundary plus Git review, so keep it there and default it on for shared writes. [write_approval.py](https://github.com/NousResearch/hermes-agent/blob/4b38ec1b664feb0f08da7e7d0971e962ced0d33b/tools/write_approval.py#L1-L13)
- **Curator lessons transfer only as reports.** Hermes marks skills stale after 14 days, archives after 30, "never auto-deletes", and keeps LLM consolidation off by default. For pbrain, memory TTLs already cover expiry. For docs, add at most a read-only hygiene report (unretrieved docs, broken cited paths) and a front-matter flag that locks a doc against agent edits. Never let usage data feed ranking. [curator.md](https://github.com/NousResearch/hermes-agent/blob/4b38ec1b664feb0f08da7e7d0971e962ced0d33b/website/docs/user-guide/features/curator.md)
- **Cheap admission hardening to borrow.** Hermes rejects memory entries with injection or exfiltration patterns or invisible Unicode. pbrain's `secretscan` covers only tokens. Add rejection of invisible and control Unicode and of the hook prompt's own text for shared stores.

## Details

- [borrow-and-refuse.md](./borrow-and-refuse.md) — the per-idea mapping (reflection nudges, agent-written skills, promotion, MEMORY.md, recall injection, curator, approval and provenance), each with fit, cost, risk and what to refuse. Read it when writing functional-spec sections.
- [hermes-mechanisms-brief.md](./hermes-mechanisms-brief.md) — a short, sourced description of the Hermes mechanisms behind the mapping, with verbatim prompt quotes and the failure issues. Source pinned to commit `4b38ec1`.

## Open Questions / Gaps

- I could not read the MarkTechPost article on Hermes's `/learn` command (HTTP 403). The skill-integrity plugin page returned 404. I did not cover how `/learn` stages skills.
- This pass did not measure whether Hermes's loop improves outcomes. That evidence belongs to the "Evidence and user reports" subtopic.
- The cost figures are qualitative judgements from the design documents, not measurements.
- Open for the spec: the doc front-matter policy (needed for a doc promote target and a lock flag), and whether top doc hits join SessionStart injection. That should be measured in the planned 24-task evaluation.

## Sources

- [NousResearch/hermes-agent at 4b38ec1](https://github.com/NousResearch/hermes-agent/tree/4b38ec1b664feb0f08da7e7d0971e962ced0d33b) — source for the review fork, prompts, whitelist, provenance, write approval and config defaults (main, 2026-11-26; latest release v0.23.4, 2026-11-24)
- [Hermes memory docs](https://hermes-agent.nousresearch.com/docs/user-guide/features/memory) — MEMORY.md/USER.md, frozen snapshot, scanning, session_search, providers (undated, 2026)
- [Hermes skills docs](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills?format=md) — skill_manage, storage, progressive disclosure, approval gate (undated, 2026)
- [Which file does what](https://hermes-agent.nousresearch.com/docs/user-guide/which-file-does-what) — split between user-scoped and project-scoped files
- [Curator docs (repo)](https://github.com/NousResearch/hermes-agent/blob/4b38ec1b664feb0f08da7e7d0971e962ced0d33b/website/docs/user-guide/features/curator.md) — stale and archive lifecycle, pin and lock, consolidation off by default
- [Issue #105921](https://github.com/NousResearch/hermes-agent/issues/105921) — unattended fork deleted memory permission rules (incident on 2026-09-08)
- [Issue #32858](https://github.com/NousResearch/hermes-agent/issues/32858) — review prompt recorded as user traits and synced to Honcho (open, 2026-05-26)
- [Issue #30220](https://github.com/NousResearch/hermes-agent/issues/30220) — misrouting and duplication between stores; "Be ACTIVE" bias
- pbrain inputs: [project_overview.md](../../../project_overview.md); [pm-position.md](../../pm-position.md); `docs/plans/session-memory.md` (peter-architect-2 worktree, #83); `specs/projects/pbrain/research/mnemosyne-comparison.md` (peter-lead-dev worktree, #82)
