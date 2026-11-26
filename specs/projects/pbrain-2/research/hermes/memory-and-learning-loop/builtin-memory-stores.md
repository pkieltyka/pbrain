# Built-in memory: MEMORY.md and USER.md

Read at Hermes Agent `main` @ `d3ecbeeb` (2026-11-26; latest release v0.23.4, 2026-11-24). All links are pinned to that commit.

## What it is

Two small, agent-curated Markdown files per profile, under `$HERMES_HOME/memories/` (default `~/.hermes/memories/`):

| File | Target | Purpose | Default limit |
|---|---|---|---|
| `MEMORY.md` | `memory` | "Agent's personal notes — environment facts, conventions, things learned" | 2,200 chars (~800 tokens) |
| `USER.md` | `user` | "User profile — your preferences, communication style, expectations" | 1,375 chars (~500 tokens) |

Source: [memory.md docs](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/website/docs/user-guide/features/memory.md); defaults in [config_defaults.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/hermes_cli/config_defaults.py) (`"memory_char_limit": 2200,   # ~800 tokens at 2.75 chars/token`, `"user_char_limit": 1375`). Confirmed on the published site ([hermes-agent.nousresearch.com/docs/user-guide/features/memory](https://hermes-agent.nousresearch.com/docs/user-guide/features/memory), fetched 2026-11-26).

The limits are in **characters**, not tokens, and are hard: "Memory does **not** auto-compact: when a write would exceed the limit, the `memory` tool returns an error instead of silently dropping entries. The agent then makes room itself" (docs).

The files are profile-scoped. The docs warn against two agents sharing a home: "two writers sharing one home will compound each other's entries into state neither of them (nor you) authored... if they need shared memory, use an external memory provider instead."

## On-disk format

Entries are joined by a section-sign delimiter: `ENTRY_DELIMITER = "\n§\n"` ([memory_tool_store.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/tools/memory_tool_store.py)). Entries can be multiline. Writes use `atomic_write_text` and `fcntl`/`msvcrt` file locks ([memory_tool.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/tools/memory_tool.py)).

## Injection: a frozen snapshot in the system prompt

At session start the entries are rendered into the system prompt as a block with a usage header, e.g. `MEMORY (your personal notes) [67% — 1,474/2,200 chars]` and `USER PROFILE (who the user is)` (headers from `MEMORY_BLOCK_HEADERS` in [memory_tool_store.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/tools/memory_tool_store.py)).

Docs, verbatim: "**Frozen snapshot pattern:** The system prompt injection is captured once at session start and never changes mid-session. This is intentional — it preserves the LLM's prefix cache for performance. When the agent adds/removes memory entries during a session, the changes are persisted to disk immediately but won't appear in the system prompt until the next session starts. Tool responses always show the live state."

Consequence the docs spell out ("Memory Needs Session Boundaries"): on messaging gateways a chat is one continuous session that survives restarts, so "the learning loop of *forget → recall from memory → search past sessions* almost never gets to fire". Their advice: run `/new` at natural boundaries.

## The `memory` tool

Single tool, actions `add` / `patch` / `remove`, always via an `operations` batch with one top-level `target` (`memory` or `user`). There is **no read action** ("memory content is automatically injected into the system prompt"). Schema in [memory_tool.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/tools/memory_tool.py):

> "WHEN: only for facts that apply to EVERY session regardless of task: who the user is, stable environment facts, standing conventions with no task home. Anything learned while doing a task (procedures, pitfalls, and the user's preferences and corrections for that kind of work) belongs in the task's skill via skill_manage, where it loads only when relevant; memory is injected into every turn and must stay small."

> "SKIP: trivial/obvious info, easily re-discovered facts, raw data dumps, task progress, completed-work logs, temporary TODO state (use session_search for those). Reusable procedures belong in a skill, not memory."

Mechanics (docs + code):

- **Batch atomicity, limit checked on the final result**: "a single call can remove/patch stale entries to free room AND add new ones, even when an add alone would overflow."
- **Overflow error is actionable**: returns `current_entries`, usage, and how many chars to free, telling the model to retry as one batch.
- **Substring addressing**: `patch`/`remove` take a unique `old_text` substring; ambiguous matches error; a failed match returns `closest_entries`.
- **`replace` was removed**: "It overwrote the whole matched entry, and models used it as a find-and-swap, which wiped the other facts in multi-fact entries. A `replace` call now returns an error pointing at `patch`." (docs). Note that external providers still receive patches as `replace` in `on_memory_write` (see [provider doc](./memory-providers-and-hooks.md)).
- **Exact duplicate adds are rejected** (returns success with "no duplicate added").
- **Security scan**: every add and patched entry goes through `_scan_memory_content` → `first_threat_message(content, scope="strict")`: "memory enters the system prompt, so a poisoned entry persists across sessions" ([memory_tool_store.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/tools/memory_tool_store.py)). Docs: blocks "prompt injection, credential exfiltration, SSH backdoors" and invisible Unicode.

## System prompt guidance (how the model is told to use memory)

`build_memory_guidance()` in [prompt_builder.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/prompt_builder.py) — this is the routing rule between memory and skills:

> "Skills come first: when you learn something while doing a task — a procedure, a pitfall, and the user's preferences and corrections for that kind of work — record it in the skill you used or built for the task (skill_manage), where it loads only when relevant. Memory is the narrow exception for facts that apply to EVERY session regardless of task ... Write entries as declarative facts, not instructions to yourself: 'User prefers concise responses' ✓ — 'Always respond concisely' ✗ (imperative phrasing gets re-read as a directive in later sessions and can override the user's current request). A fact stale within a week belongs in session history; procedures and workflows belong in skills."

The separate session-search guidance: "When the user references something from a past conversation or you suspect relevant cross-session context exists, use session_search to recall it before asking them to repeat themselves." (`SESSION_SEARCH_GUIDANCE`).

## Write approval gate

`memory.write_approval` (default `false`). With `true`: interactive CLI foreground writes prompt inline; everything else (messaging, scripts, the background review) is **staged** — `/memory pending`, `/memory approve <id>|all`, `/memory reject`. Docs: "This is the answer to 'the agent saved a wrong assumption about me'".

Staged `patch`/`remove` records pin the full target entry (`matched_entry`); approval refuses if the entry changed since staging (`_pin_matched_entries`, `apply_memory_pending` in [memory_tool.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/tools/memory_tool.py)). "the background review stages these even with the gate off" — i.e. background destructive edits are always staged (docs).

Limit of the gate (docs): "It is not a file-system lock: in a foreground turn the generic file tools (`write_file`, `patch`) and the terminal can still edit `MEMORY.md` / `USER.md` directly. The background review cannot by default: its tool whitelist denies those write tools."

## Enabling/disabling

- `memory.memory_enabled`, `memory.user_profile_enabled` — each store independently. Both false → tool and guidance removed from the prompt; an external provider keeps working "use this when you want a third-party memory backend *instead of* the built-in files".
- `memory` in `agent.disabled_toolsets` — "the heavier switch: it hides external provider tools too." (Mnemosyne's README warns about exactly this.)

## Pruning and inspection: `/journey`

`hermes journey` / `/journey` shows a timeline of learned skills and memory entries; `list`, `delete <node>` (skills archived, memory chunks removed), `edit <node>` (opens in `$EDITOR`). Docs section "Learning Journey". There is also an official `hermes-memory-wiki` plugin that derives "browsable subject pages and daily logs ... from local session history, plus a read-only Persistent Memory (MEMORY.md / USER.md) audit panel" ([plugin-catalog/hermes-memory-wiki.yaml](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/plugin-catalog/hermes-memory-wiki.yaml)).

## History (dates from git log on `tools/memory_tool.py`)

- 2026-02-19 `440c244c` "feat: add persistent memory system + SQLite session store" — the memory tool and session DB arrived together.
- 2026-02-19 `56ee8a5c` "remove 'read' action from memory tool".
- An April 2026 third-party write-up ([Mr. Ånand, "Inside Hermes Agent", 2026-04-02](https://mranand.substack.com/p/inside-hermes-agent-how-a-self-improving)) describes "add, replace, and remove" and a combined 3,575-char limit (= 2,200 + 1,375). The `replace` action has since been removed in favour of `patch`. Older write-ups are therefore partly stale.

## Observations (my inference, labelled)

- *Inference:* The design treats always-injected memory as an expensive, tiny, global cache (~1,300 tokens). The real knowledge volume is meant to live in skills (loaded on demand) and in the session DB (searched on demand). Most of the prompt text is about routing facts to the right store and keeping memory small.
- *Inference:* The many guardrails (no `replace`, pinned approvals, declarative-not-imperative phrasing, grounding rule in the review prompt) each read as fixes for failures seen in practice; several cite issue numbers in comments (e.g. #30220 for facts written to both stores).
