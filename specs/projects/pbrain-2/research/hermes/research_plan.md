# Research Plan: Hermes

## Goal

Understand how Hermes achieves automatic skill creation and updates and its "learns by doing" behaviour, so pbrain-2's memory (and docs) design can take what works. Embedded in `/spec new_project pbrain-2`; feeds the functional spec step.

## Run

- Model: claude-opus-5-5 (inherited) for the subtopic agents and the summary agent

## Subtopics

- [ ] What Hermes is — Identify which "Hermes" people mean when they praise automatic skill creation and an agent that "learns by doing" / "feels like working with a co-worker" (likely an agent framework, e
- [ ] Automatic skill creation and updates — How Hermes creates and updates skills on its own: what triggers creating a new skill, what a skill file contains (format, frontmatter, scripts), where skills are stored, how existing skills are patched or improved, any review/approval step, how it avoids bad or duplicate skills, versioning, and how skills are selected/loaded into context later (progressive disclosure, indexes)
- [ ] Memory and the learning loop — How Hermes captures and uses memory: session capture, persistent memory files or stores (e
- [ ] Evidence and user reports — What users and reviewers actually report: the "co-worker" / "learns by doing" experience, concrete examples, any benchmarks or evaluations of its learning, and known failure modes (skill bloat, drift, wrong or stale skills, memory pollution, privacy/secret leakage, cost)
- [ ] Lessons for pbrain-2 — Map Hermes's ideas onto pbrain-2: read specs/projects/pbrain-2/project_overview

## Focus Details

### What Hermes is

Identify which "Hermes" people mean when they praise automatic skill creation and an agent that "learns by doing" / "feels like working with a co-worker" (likely an agent framework, e.g. Nous Research's Hermes Agent, but verify; also note the "Hermes" host that Mnemosyne integrates with — mnemosyne-oss/mnemosyne integrations/hermes — and whether it is the same thing). Cover: who makes it, licence, how it is run (CLI/TUI/gateway, harness, models it supports, local vs hosted), architecture at a high level, release history, activity and maturity. Do NOT go deep on the skill mechanism, the memory loop, or user sentiment — other subtopics own those.

### Automatic skill creation and updates

How Hermes creates and updates skills on its own: what triggers creating a new skill, what a skill file contains (format, frontmatter, scripts), where skills are stored, how existing skills are patched or improved, any review/approval step, how it avoids bad or duplicate skills, versioning, and how skills are selected/loaded into context later (progressive disclosure, indexes). Cite code or docs. Compare briefly with Claude Code/Anthropic Agent Skills format if Hermes uses or mirrors it. Memory capture and the general learning loop belong to another subtopic.

### Memory and the learning loop

How Hermes captures and uses memory: session capture, persistent memory files or stores (e.g. MEMORY.md / USER.md style), periodic reflection or "nudges" to save knowledge, summarisation/consolidation, session search/recall across past conversations, user modelling, context injection limits, and how this loop interacts with skill creation (which is another subtopic — only describe the interface). Include how third-party memory providers (e.g. Mnemosyne, Honcho or others) plug in, and what lifecycle hooks exist. Cite code/docs.

### Evidence and user reports

What users and reviewers actually report: the "co-worker" / "learns by doing" experience, concrete examples, any benchmarks or evaluations of its learning, and known failure modes (skill bloat, drift, wrong or stale skills, memory pollution, privacy/secret leakage, cost). Prefer primary sources (GitHub issues/discussions, maintainer posts, substantive write-ups) over hype; label anecdote vs measurement. Do not re-describe the mechanisms — other subtopics own them.

### Lessons for pbrain-2

Map Hermes's ideas onto pbrain-2: read specs/projects/pbrain-2/project_overview.md, docs/plans/session-memory.md (in /home/peter/Dev/pkieltyka/pbrain.worktrees/peter-architect-2 if not on master) and specs/projects/pbrain/research/mnemosyne-comparison.md (in /home/peter/Dev/pkieltyka/pbrain.worktrees/peter-lead-dev). Using web sources on Hermes's design (lightly — the other subtopics go deep), identify what pbrain-2's memory and docs should borrow: e.g. agents writing/updating their own skills via pbrain, promotion of memories into skills or docs, reflection nudges, a MEMORY.md-style agent-facing file, recall injection. For each: fit, cost, risk (especially privacy between shared and personal, and reviewability in Git), and what to refuse.
