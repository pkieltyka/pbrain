# Third-party memory providers and lifecycle hooks

Read at Hermes Agent `main` @ `d3ecbeeb` (2026-11-26; v0.23.4). Links pinned to that commit. Honcho plugin pinned at plastic-labs/honcho `894b8832` (catalog v1.2.0). Mnemosyne pinned at mnemosyne-oss/mnemosyne `7106311a` (catalog v0.7.7).

## Two plugin surfaces

1. **Memory provider plugins**: a `MemoryProvider` ABC. Exactly one external provider is active, selected by `memory.provider`. It always runs *alongside* the built-in store, never in place of it, though the built-in store can be switched off separately.
2. **General plugin hooks** (`ctx.register_hook`), **gateway event hooks** (`~/.hermes/hooks/<name>/HOOK.yaml` + `handler.py`), and **shell hooks** (Claude Code-compatible JSON over stdio; exit 2 = block). These are for any plugin. Memory-ish plugins that are not providers (nudgers, reviewers, dashboards) use them.

## MemoryProvider contract

Source: [agent/memory_provider.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/memory_provider.py), developer guide [memory-provider-plugin.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/website/docs/developer-guide/memory-provider-plugin.md).

Module docstring: "Plugins ship in `plugins/memory/<name>/`, activated via `memory.provider` (ONE external provider at a time). Lifecycle, driven by MemoryManager: initialize -> system_prompt_block / prefetch / sync_turn per turn -> tool dispatch -> shutdown, plus optional `on_*` hooks."

### Required

| Method | Contract (quoted/paraphrased from docstrings) |
|---|---|
| `name` | short id |
| `is_available()` | "Configured, credentialed and ready? ... no network" |
| `initialize(session_id, **kwargs)` | once at agent startup. kwargs include `hermes_home` and `platform`, plus optionally `agent_context` ("primary" \| "subagent" \| "cron" \| "flush" — "skip writes for non-primary contexts"), `agent_identity`, `user_id`, `user_name`, `chat_id`, `gateway_session_key`, `cwd`, `session_title`(+source), `resumed`, `message_count` |
| `get_tool_schemas()` | OpenAI function schemas the provider adds |
| `handle_tool_call(name, args, **kw)` | returns a JSON string |
| `get_config_schema()` / `save_config()` | drives `hermes memory setup`; secrets go to `.env` |

### Optional lifecycle hooks

| Hook | When | Purpose |
|---|---|---|
| `system_prompt_block()` | system prompt assembly | "STATIC system-prompt text ... Recalled context goes through prefetch(), not here." |
| `prefetch(query, session_id)` | before each turn's API call | "Formatted recall context for the upcoming turn ... Must be fast — recall in the background and return cached results" |
| `queue_prefetch(query, session_id)` | after each turn | pre-warm recall for the next turn |
| `recall_status()` | after prefetch | drives a "🧠" recall indicator in the UI |
| `sync_turn(user, assistant, session_id, messages, turn_author)` | after each completed turn | "Persist a completed turn (non-blocking)". `messages` includes tool calls and results |
| `on_turn_start(turn_number, message, **kw)` | per-turn tick | gets `model`, author fields, `previous_message`, `remaining_tokens` |
| `on_session_end(messages)` | real session boundary | "End-of-session extraction; fires only at real session boundaries, never per-turn" |
| `on_session_switch(new_id, parent_session_id, reset, rewound)` | `/resume`, `/branch`, `/new`, compression | "rebind per-session state" |
| `on_pre_compress(messages)` | before lossy compaction | "Extract insights ... fed into the summary prompt". v2 adds a fail-closed checkpoint |
| `on_memory_write(action, target, content, metadata)` | after a built-in memory tool write | "Mirror a built-in memory-tool write". A patch arrives as `replace` with the whole new entry, and `metadata["previous_content"]` holds the old one |
| `on_delegation(task, result, child_session_id)` | parent side, after a subagent finishes | "the subagent has no provider session" |
| `identity_signature()` | each inbound gateway message | identity values whose change must bust a cached agent |
| `on_profile_created` / `on_profile_renamed` | profile lifecycle | e.g. create a remote peer |
| `backup_paths()` | `hermes backup` | state outside HERMES_HOME |
| `shutdown()` | exit | flush and close |

`on_session_end` and `on_session_switch` run as **one serialised task** on the manager's single background worker. The reason: "an ad-hoc thread raced the inline switch and misattributed transcripts" (`commit_session_boundary_async`, [memory_manager.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/memory_manager.py), #16454).

### What the MemoryManager enforces

[agent/memory_manager.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/memory_manager.py):

- **Single external provider**: "builtin always accepted, only ONE external allowed" (`add_provider`). Rationale: "tool-schema bloat, conflicting backends".
- **Egress secret scrub**: every fan-out to a provider ("turn sync, recall queries, session-end and pre-compress transcripts, memory-tool mirrors, delegation results, provider tool args") passes through `redact_for_egress`. This holds "even when `security.redact_secrets` is off" (#115104). Docs caveat: "Credentials with no recognisable shape ... are not detected."
- **Timeouts and circuit breaker**: 8 s prefetch timeout. A call stuck for more than 30 s counts as stuck. After 4 stuck calls admission stops, with a 60 s probe interval and at most 16 calls in flight per (home, provider).
- **Profile isolation on multiplexed gateways**: one fresh instance per agent. The manager refuses an instance another profile already bound (`memory_provider_instance_shared`) and refuses callers whose profile it cannot resolve. Background threads must use `spawn_context_thread` because "a plain thread starts with an empty context: under multiplexed profiles it would silently write into the *default* profile's store".
- **Forks are provider-less**: subagents, the curator, the batch runner and the background review run without a provider, so harness prompts don't pollute the user's memory. The one exception is when `checkpoint_required` is armed: then the fork loads the provider with `agent_context="subagent"`.

### How recall is injected

`prefetch_all` merges provider outputs. `build_memory_context_block` ([agent/memory_context.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/memory_context.py)) wraps them:

```
<memory-context>
[System note: The following is recalled memory context, NOT new user input. Treat as authoritative reference data — this is the agent's persistent memory and should inform all responses.]
...
</memory-context>
```

The block goes into the user turn (the "api_content sidecar"), not the system prompt. A `StreamingContextScrubber` strips any `<memory-context>` spans from model output, so recalled memory never leaks to the UI. Repeated bullets inside one block are deduped. Size limits are in [session-search-and-recall.md](./session-search-and-recall.md).

*Discrepancy:* the Honcho page says its two layers are "injected into the system prompt" ([honcho.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/website/docs/user-guide/features/honcho.md)), but the provider contract routes dynamic recall through `prefetch()` into the user message. I did not read the Honcho plugin source to settle this. Possibly only the static part uses `system_prompt_block()`.

### Discovery and packaging

Discovery order (earlier wins): bundled `plugins/memory/<name>/` (closed to new providers) → user `$HERMES_HOME/plugins/<name>/` → project `./.hermes/plugins/<name>/` (opt-in via `HERMES_ENABLE_PROJECT_PLUGINS=1`) → the `hermes_agent.memory_providers` pip entry point. The ordering is deliberate: "a directory dropped into a working tree can never shadow a shipped provider". A provider can also ship read-only skills (`ctx.register_skill` → `<provider>:<skill>`) and a `cli.py` for `hermes <provider> ...` subcommands.

Bundled today: RetainDB and ByteRover, which "leave core on October 15, 2026" (unmaintained). Catalog providers: Honcho, Hindsight, Supermemory, Mem0, OpenViking, Holographic, Memori, Mnemosyne, and many community entries in [plugin-catalog/](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/plugin-catalog) (agentmemory, qdrant-memory, obsidian-memory, memobase, and others). The provider comparison table in [memory-providers.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/website/docs/user-guide/features/memory-providers.md) lists storage, cost and tool count per provider.

## Honcho (Plastic Labs): user modelling as a service

Sources: [honcho.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/website/docs/user-guide/features/honcho.md), [plugin-catalog/honcho.yaml](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/plugin-catalog/honcho.yaml), plugin source at [plastic-labs/honcho/hermes-plugin-honcho](https://github.com/plastic-labs/honcho/tree/main/hermes-plugin-honcho).

- It "maintains a running model of who the user is — their preferences, communication style, goals, and patterns — by reasoning about conversations after they happen". This is the "deepening model of who you are" from the README.
- **Peers**: user and AI are both peers. Each Hermes profile gets its own AI peer ("Each peer sees only its own observations and conclusions"). Observation toggles `observeMe`/`observeOthers` come with presets `directional` (default, mutual) and `unified`.
- **Two-layer context**: (1) base context, made of session summary, user representation, user peer card, AI self-representation and AI identity card, refreshed every `contextCadence` turns (default 1). (2) A "dialectic supplement", LLM-synthesised reasoning about the user's current needs, every `dialecticCadence` turns (default 2), with `dialecticDepth` 1–3 passes (pass 1 is a self-audit, pass 2 a reconciliation). The dialectic uses cold-start vs warm prompts, and a prewarm fires at session start. A `contextTokens` budget truncates the result.
- **Tools (5)**: `honcho_profile`, `honcho_search`, `honcho_context`, `honcho_reasoning`, `honcho_conclude` (create/delete "conclusions").
- **Hooks declared**: `on_session_end`. Requires `HONCHO_API_KEY`.
- **Egress disclosure** (catalog): "conversation turns, derived conclusions and peer/session ids are sent to the Honcho API (api.honcho.dev by default, or the self-hosted base_url you configure)". Requests also carry the Hermes version, OS platform and model id headers.
- History: Honcho shipped in-tree first and moved to the catalog (same config and tools).

## Mnemosyne: local-first provider

Sources: [plugin-catalog/mnemosyne.yaml](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/plugin-catalog/mnemosyne.yaml), [mnemosyne integrations/hermes/README.md](https://github.com/mnemosyne-oss/mnemosyne/blob/7106311a95683cbb908b1f4f0f9f4df034aad50b/integrations/hermes/README.md), [integrations/hermes-catalog/README.md](https://github.com/mnemosyne-oss/mnemosyne/blob/7106311a95683cbb908b1f4f0f9f4df034aad50b/integrations/hermes-catalog/README.md).

- Catalog description: "Local-first memory layer for Hermes Agent — SQLite with vector + FTS5 hybrid recall, episodic consolidation, temporal knowledge graph. Zero cloud." The embedder is BAAI/bge-small-en-v1.5, downloaded on first use. `requires_hermes: ">=0.21.4"`. It lists **42 tools** in the catalog entry (its README says 40) and **no** generic hooks: it plugs in purely through the MemoryProvider lifecycle.
- Packaging: the catalog directory is a thin wrapper (`kind: exclusive`) that re-exports `register` from the PyPI package `mnemosyne-hermes` (source in `integrations/hermes/`). A legacy "wrapper mode" install also lives at `$HERMES_HOME/plugins/mnemosyne`; the provider guide's "HERMES_HOME survival contract" exists for such sidecar wrappers.
- **Capture**: "After Hermes completes a turn, the Mnemosyne provider stores the full interaction (user message, assistant response, tool calls, and available execution context) in a local SQLite database with vector embeddings." Memories are tagged with importance, scope (session/global), veracity (stated/inferred/tool/imported) and optional expiry. `sync_turn` drops machine-generated envelopes (cron preambles, compaction summaries, and so on) with a "best-effort text filter".
- **Recall**: explicit `mnemosyne_recall` (vector + FTS5 + importance, with tunable weights) plus a conservative automatic prefetch. The prefetch has a "lexical gate — a memory must share multiple distinctive non-generic terms with the query and meet a minimum query-coverage fraction — before silent injection", and it injects a `## Mnemosyne Context` block.
- **Consolidate**: `mnemosyne_sleep` "compresses old working memories into episodic summaries". It runs both as auto-sleep and on session end.
- **Lifecycle**: it rebinds on `/new`, `/resume`, `/branch` and compression, and skips contexts `subagent`, `cron`, `flush`, `background` and `skill_loop`. It is fail-soft: database or disk errors are logged and suppressed.
- It recommends turning built-in memory **off** (`memory_enabled: false`, `user_profile_enabled: false`). It warns *not* to use `hermes tools disable memory`, because that toolset gates provider tools too.
- There is a different, unrelated `birkin-mnemosyne` catalog entry: a local Markdown provider with create-only notes and non-deleting archive.

## General plugin hooks relevant to memory

From [hooks.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/website/docs/user-guide/features/hooks.md) (valid names come from `hermes_cli.plugins.VALID_HOOKS`):

| Hook | Kind | Memory relevance |
|---|---|---|
| `pre_llm_call` | directive | once per turn. Returned `{"context": ...}` is appended to the **user message**, which is how a non-provider plugin can inject recall. It can also `block` a turn, and a blocked prompt is stored as a placeholder that memory providers, session_search and the review all see instead of the original |
| `post_llm_call` | observer | once per successful turn. Gets the full history plus `review_signals` (the self-learning cadence) and a `side_question_token` for `ctx.fork_session` |
| `on_session_start` / `on_session_open` | observer | session init |
| `on_session_end` | observer | "Despite its name this hook fires after **every turn**". Compare the provider `on_session_end`, which is a real boundary |
| `on_session_finalize` / `on_session_reset` / `on_session_delete` | observer | real teardown (`/new`, quit); flush per-session state |
| `post_compaction` | observer | after context compression |
| `post_curator_run` | observer | after a curator pass over skills |
| `on_skill_lifecycle` | observer | "after an authoritative skill-usage state change" (name, provenance, use count) |
| `subagent_start` / `subagent_stop` | observer | delegation |
| `transform_tool_result`, `transform_llm_output` | transform | could redact before anything is stored |
| `register_system_prompt_section(..., position="after_memory")` | API | bounded, cache-safe always-on guidance (4,000 chars each, 8,000 total) |

Gateway event hooks (`HOOK.yaml`): `gateway:startup`, `session:start`, `session:end`, `session:reset`, `session:compress`, `agent:start`, `agent:step`, `agent:end`, `reaction:*`, `command:*`.

Example non-provider catalog plugins built on these: `turn-completion-nudge` (`post_llm_call`, nudges the agent to continue when it stops early, capped at 2 consecutive nudges) and `memory-review` (desktop UI for staged memory writes). Both are in [plugin-catalog/](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/plugin-catalog).

## Observations (inference, labelled)

- *Inference:* The provider API is shaped around four moments: **before a turn** (prefetch), **after a turn** (sync, queue_prefetch), **at a boundary** (session end/switch, pre-compress), and **when the agent writes** (on_memory_write mirror). That is a clean, reusable decomposition for any memory backend.
- *Inference:* Much of the contract's complexity (contextvars threads, per-instance binding, serialised boundary tasks, egress scrubbing) comes from the multi-profile gateway and from third-party code running in-process. A single-user local tool can skip most of it, but should keep the secret scrub and the "no provider in forks" rule.
