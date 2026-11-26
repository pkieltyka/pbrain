# Session capture, session search, and context-injection budgets

Read at Hermes Agent `main` @ `d3ecbeeb` (2026-11-26; v0.23.4). Links pinned to that commit.

## Session capture

Every CLI and messaging session is persisted to a per-profile SQLite DB (`~/.hermes/state.db`) with FTS5 full-text indexes. Docs: "All CLI and messaging sessions are stored in SQLite (`~/.hermes/state.db`) with FTS5 full-text search" ([memory.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/website/docs/user-guide/features/memory.md)). The SQLite session store shipped with the memory system on 2026-02-19 (commit `440c244c`).

Session identity: on gateways the session key is deterministic per source, e.g. `agent:main:telegram:dm:<chat_id>`, `agent:main:<platform>:group:<chat_id>:<user_id>`. `group_sessions_per_user: true` is the default ([sessions.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/website/docs/user-guide/sessions.md)). Sessions form **lineages**: compression, `/branch` and `/resume` create child session ids, and search dedupes "by lineage".

Capture is the raw transcript. There is no automatic per-session summary or extraction into the built-in stores. Distillation happens only through the background review (memory/skills) or through an external provider's `sync_turn`/`on_session_end`.

## `session_search` tool

Module docstring ([tools/session_search_tool.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/tools/session_search_tool.py)):

> "Session Search Tool - long-term conversation recall over the SQLite session DB. Single-shape tool; the mode is inferred from the args: DISCOVERY (`query`; FTS5 deduped by lineage, adaptive detail hydrates only the top result), SCROLL (`session_id` + `around_message_id`; ±window around the anchor), READ (`session_id` alone; whole session or head/tail), BROWSE (no args). No LLM calls — every shape returns actual DB messages."

Details (docs + code):

- **Discovery** returns the top N sessions (default 3, max 10). The top result is fully hydrated: ±5 messages around the match plus "bookends" (the first and last 3 user+assistant messages). Lower results are compact (anchor message only) unless `detail="full"`. FTS5 syntax works (phrases, OR/NOT, prefix). `sort: newest|oldest` is a ranking bias. `after`/`before` take ISO dates or relative times like `7d`.
- **Default role filter** is `user,assistant` ("tool output is usually noise"). Pass `tool` explicitly to search tool output.
- **Ranking hygiene**: subagent, kanban and tool sessions are hidden. Cron sessions are *demoted*, not excluded, because "cron vocabulary dominates bare BM25 and starves out the user's own sessions ('recall blindness')" (#19434).
- **Bounds**: read-shape content is capped at 2,000 chars per message (`_READ_MAX_CONTENT`, after "one read returned 74K chars", #114344). 300 FTS rows are scanned before dedup.
- **Speed claims** (docs): "~20ms FTS5 query, ~1ms scroll".
- **When used**: prompt guidance only, with no automatic injection. "When the user references something from a past conversation or you suspect relevant cross-session context exists, use session_search to recall it before asking them to repeat themselves." Typical triggers listed: "we did this before", "remember when", "last time".
- **Sessions as links**: results carry an `@session:<profile>/<id>` link that the agent is told to write verbatim.

### Privacy boundary (important)

Docs, verbatim: "`session_search` reads the whole session store of the profile it runs in: every chat, from every platform and every user in that profile's allowlist. On a shared or family bot, anyone allowed to talk to it can ask the agent to find and read anyone else's conversations with it. Hermes does not separate users inside one profile ... Give each person who needs private history their own instance, under a separate OS user or container." The default profile can also read other profiles' stores via `profile=`. On a multiplexed process, only the default profile may do this.

### Version change: LLM summarisation removed

- 2026-02-24 `cc6bea8b` "enhance session search tool with parent session resolution and parallel summarization". Early versions **LLM-summarised** search hits before returning them.
- 2026-05-18 `abf1af54` "feat(session_search): single-shape tool with discovery, scroll, browse — no LLM (#27590)".
- The README still says "FTS5 session search with LLM summarization for cross-session recall" ([README.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/README.md)), and April 2026 write-ups describe summarisation ([Mr. Ånand, 2026-04-02](https://mranand.substack.com/p/inside-hermes-agent-how-a-self-improving)). Both are **stale** relative to the code. There is an `evals/session_search_schema/` harness in the repo that evaluates the tool schema across models.

### Built-in memory vs session search (docs table, abridged)

| | Persistent memory | Session search |
|---|---|---|
| Capacity | ~1,300 tokens total | All sessions |
| Cost | Token cost in every prompt | No LLM calls; on demand |
| Management | Curated by agent | Automatic — all sessions stored |
| Use | Key facts always available | "did we discuss X last week?" |

## In-session compression and its memory hooks

Inside one session, the context compressor (pluggable as a "context engine") summarises old turns when the window fills. Memory interacts with it at three points:

1. `MemoryProvider.on_pre_compress(messages)` lets an external provider "Extract insights from `messages` about to be compressed, fed into the summary prompt" ([agent/memory_provider.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/memory_provider.py)).
2. **Fail-closed checkpoint (opt-in)**: a provider with `pre_compress_checkpoint_api_version = 2` plus `compression.checkpoint_required: true` makes compression "**fail closed** before any lossy rewrite unless an active provider advertising the API completed its checkpoint". The docs add: "None of the providers bundled with Hermes advertise checkpoint API v2" ([memory-provider-plugin.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/website/docs/developer-guide/memory-provider-plugin.md)).
3. Compression rotates the session id and fires `on_session_switch`. There is also a `post_compaction` plugin hook ([hooks.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/website/docs/user-guide/features/hooks.md)).

The built-in memory block survives compression because it lives in the frozen system prompt.

## Context-injection budgets (the numbers)

| What | Where it goes | Limit | Source |
|---|---|---|---|
| MEMORY.md | system prompt (frozen at session start) | 2,200 chars | config defaults |
| USER.md | system prompt (frozen) | 1,375 chars | config defaults |
| Context files (`.hermes.md` → `AGENTS.md` → `CLAUDE.md` …, plus `SOUL.md`) | system prompt | `context_file_max_chars`; else dynamic: ~6% of the model context window (converted to chars), floor 20,000 and ceiling 500,000 chars; head/tail truncated 70/20 | [prompt_builder.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/prompt_builder.py), [context-files.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/website/docs/user-guide/features/context-files.md) |
| Plugin system-prompt sections | system prompt, "after_memory", frozen per session | 4,000 chars each; 8,000 total; 32 sections | [hooks.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/website/docs/user-guide/features/hooks.md) |
| External provider `system_prompt_block()` | system prompt | "STATIC" text only | [memory_provider.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/memory_provider.py) |
| External provider `prefetch()` recall | the **current user message**, wrapped in `<memory-context>` | full by default up to a ceiling of 10× `hooks.output_spill.max_chars` (at least 100,000 chars). Opt-in `prefetch_spill_enabled` spills anything over 10,000 chars to a file with a 500+500 char preview | memory.md, memory-provider-plugin.md |
| External prefetch latency | — | 8 s timeout (`_EXTERNAL_PREFETCH_TIMEOUT_S`), plus a stuck-call circuit breaker | [memory_manager.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/memory_manager.py) |
| `pre_llm_call` plugin context | current user message | no stated cap (spill applies) | hooks.md |
| `session_search` | tool result | 3 sessions by default; 2,000 chars per message in read shape | session_search_tool.py |

Design rule (hooks.md): plugin context goes "Always the **user message**, never the system prompt. This preserves the prompt cache". Recall is cheap per turn but is "replayed verbatim on every later request for as long as that row is in context" ([memory_context.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/memory_context.py)). That is why Hermes dedupes repeated bullets in the recall block.

Prefetch is skipped for trivial prompts: `is_trivial_prompt` matches "yes/ok/thanks/continue/lgtm…" and slash commands, "skipping recall saves a round-trip and keeps stale context from derailing one-word replies" ([memory_provider.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/memory_provider.py)).
