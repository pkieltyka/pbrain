# How skills are selected and loaded into context

Baseline hermes-agent v0.23.4. Sources: [`agent/prompt_builder.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/agent/prompt_builder.py), [`tools/skills_tool.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/tools/skills_tool.py), [`agent/skills_index_delta.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/agent/skills_index_delta.py), [Skills System doc](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills).

## Three levels of progressive disclosure

From the docs:

```
Level 0: skills_list()           → [{name, description, category}, ...]   (~3k tokens)
Level 1: skill_view(name)        → Full content + metadata       (varies)
Level 2: skill_view(name, path)  → Specific reference file       (varies)
```

This is the same pattern as Anthropic's Agent Skills (metadata ~100 tokens per skill always loaded; SKILL.md "Under 5k tokens" when triggered; resources "None until accessed", per the [Anthropic overview](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview)). The difference is that Hermes loads files through **dedicated tools** (`skill_view`) rather than bash `cat`. That is what lets it count use, enforce read-before-write, render template variables and check readiness (missing env vars, credential files, deps).

## Level 0: the index in the system prompt

`build_skills_system_prompt` renders a compact index grouped by category. Each row is `- name: description`, with the description clipped to `skills.index_desc_limit` (default **60** chars; "wider routes better, costs tokens"). The surrounding instruction is strongly pro-loading:

> "## Skills
> Before replying, scan the skills below. If a skill matches or is even partially relevant to your task, you MUST load it with skill_view(name) and follow its instructions. Err on the side of loading — it is always better to have context you don't need than to miss critical steps, pitfalls, or established workflows. … Skills also encode the user's preferred approach, conventions, and quality standards for tasks like code review, planning, and testing — load them even for tasks you already know how to do, because the skill defines how it should be done here.
> …
> <available_skills>
> …
> </available_skills>
>
> Only proceed without loading a skill if genuinely none are relevant to the task."

Index construction details:

- **Selection is done by the model**, from name plus a 60-char description. There is no embedding retrieval or ranking. The curator prompt says as much: "An agent searching skills matches on descriptions, not on exact names".
- **Filtering** before rendering: disabled skills (globally or per platform), `platforms:` mismatch, `requires_*`/`fallback_for_*` toolset conditions, required apps, shadowed duplicates, and quarantined project skills.
- **Coding posture demotion**: categories outside the current coding context collapse to a names-only line, but "NEVER drop entries — agent-created skills are the model's project memory and it won't rediscover them via skills_list."
- **Caching**: the index is cached in-process and in a disk snapshot, keyed on a manifest of the skill files, so another process's changes are noticed (#92313).
- **Prompt-cache stability**: the system prompt stays byte-stable for the life of a conversation. Skills created, installed or archived mid-conversation (including by the agent itself or the curator) reach the model as a per-turn note instead: "[System: The skills available in this conversation changed after its system prompt was built. … Now available (load with skill_view(name) when relevant …)". The system prompt picks up the change at the next compaction ([`agent/skills_index_delta.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/agent/skills_index_delta.py)). So a newly self-written skill is usable in the same session.

## Level 1: `skill_view(name)`

Returns JSON with `content` (the rendered SKILL.md, with `${HERMES_SKILL_DIR}` etc. substituted), `description`, `tags`, `related_skills`, `linked_files` (a dict of files grouped by `references`/`templates`/`assets`/`scripts`), a `usage_hint` ("To view linked files, call skill_view(name, file_path) …"), readiness/setup status, and any `deps_note`. Side effects: `bump_view` and `bump_use` (the curator's activity clock), and the read mark for the background review's read-before-write guard. A repeat view whose result is byte-identical returns a short stub in the same session (cleared on compaction), except inside the review fork.

The linter's `oversized-body` rule explains the cost model: "skill_view loads all of it and it stays in context for every later call of the session". Hence the push toward a lean SKILL.md plus `references/`.

## Level 2: `skill_view(name, file_path)`

Loads one support file. The `/learn` knowledge-base layout depends on this: SKILL.md carries "an index of every reference file with a one-line "load this when ..." description", and "reference files cost nothing until a question actually needs them". The agentskills.io spec gives the same guidance ("Keep file references one level deep from `SKILL.md`").

## User-driven loading

- Every skill is a slash command: `/excalidraw …`. Up to 5 leading `/skill` tokens can be stacked in one message.
- **Skill bundles**: `~/.hermes/skill-bundles/<slug>.yaml` groups skills under one command plus an optional `instruction`. Bundles "generate a fresh user message at invocation time … no system prompt mutation".
- `hermes -s <skill>` preload, and `skills:` lists on cron jobs.
- Skill `config` values are appended to the loaded message ("[Skill config (from ~/.hermes/config.yaml): …]").

## After compaction

When context is compressed, loaded skill content can be pruned and replaced by `[SKILL_PRUNED:…]` markers. A reload rule rides in the compaction summary ("the only place markers exist", per the comment in `prompt_builder.py` referencing `agent/context_compressor_tail.py`). Claude Code takes a similar approach: re-attached skills keep "the first 5,000 tokens of each" with "a combined budget of 25,000 tokens" ([Claude Code skills docs](https://code.claude.com/docs/en/skills)).
