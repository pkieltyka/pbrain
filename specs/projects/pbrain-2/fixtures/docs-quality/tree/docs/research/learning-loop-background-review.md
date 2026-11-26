# The learning loop: nudges, the background review fork, and the skill interface

Read at Hermes Agent `main` @ `d3ecbeeb` (2026-11-26; v0.23.4). Links pinned to that commit.

## Bottom line

Hermes' "learns by doing" loop is **not** a reflection step the agent does in the conversation. After a turn is delivered, Hermes may **fork a second copy of the agent in a background thread**. The fork replays the conversation snapshot with a fixed review prompt and a restricted tool whitelist, and writes straight to the memory and skill stores. Two counters decide when it fires: user turns since the last memory write, and tool-loop iterations since the last skill write. The default for both is 10.

Module docstring, [agent/background_review.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/background_review.py):

> "Background memory/skill review — fork the agent to evaluate the turn. After every turn `AIAgent.run_conversation` may spawn a daemon thread that replays the conversation snapshot in a forked :class:`AIAgent` and asks 'should any skill/memory be saved or updated?'. Writes go straight to the memory + skill stores; the main conversation and prompt cache are never touched. The fork inherits the parent's live runtime (provider, model, credentials, cached system prompt) so it hits the same prefix cache, and runs under a dispatch-side tool whitelist."

The README still markets this as "Agent-curated memory with periodic nudges" ([README.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/README.md)). In current code a "nudge" means this counter-triggered review fork.

## Triggers (the "nudges")

From [agent/review_signals.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/review_signals.py):

> "Hermes runs its background memory/skill review on two counters: user turns since the last memory review (`memory.nudge_interval`) and tool-calling iterations since the last skill review (`skills.creation_nudge_interval`)."

- **Memory trigger**: `_tick_memory_nudge` in [agent/turn_context.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/turn_context.py) runs at turn start. It increments `_turns_since_memory` and fires at `>= _memory_nudge_interval` (the counter then resets to 0). It only runs when the `memory` tool is available and a store exists. A call to the `memory` tool also resets it. On resume, the counter is rebuilt from history: `prior_user_turns % interval`.
- **Skill trigger**: `skill_review_due(agent)` = `_skill_nudge_interval > 0 and _iters_since_skill >= _skill_nudge_interval and "skill_manage" in valid_tool_names`. It counts **agent-loop iterations** (tool-calling steps), not turns, and a `skill_manage` call resets it.
- **Defaults**: `memory.nudge_interval: 10`, with the config comment "Periodic built-in memory review; 0 when an external provider auto-extracts." ([config_defaults.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/hermes_cli/config_defaults.py)). `skills.creation_nudge_interval` defaults to 10 (`_apply_agent_section` in [agent/agent_init.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/agent_init.py)).
- **Spawn site**: `finalize_turn` in [agent/turn_finalizer.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/turn_finalizer.py). The review runs "AFTER delivery so it never competes with the user's task", and only when `final_response and not interrupted and not skip_background_review and (review_memory or review_skills)`. Cron sets `skip_background_review` because "the fork costs ~30K tokens / event with no human-in-the-loop benefit." Delegated subagents (`_delegate_depth > 0`) and one-shot `chat -q` turns are also excluded (`background_review_enabled`, `load_background_review_settings`).
- **Manual trigger**: `/refine [focus]` "Run the background memory/skill self-improvement review **now** ... Optional focus text steers the review" ([slash-commands.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/website/docs/reference/slash-commands.md)). The focus text is appended to the prompt with "prioritize it over the general instructions above".

## Which prompt runs

`_PROMPT_NAME_BY_SCOPE`: (memory, skills) → `_COMBINED_REVIEW_PROMPT`; memory only → `_MEMORY_REVIEW_PROMPT`; skills only (also the default) → `_SKILL_REVIEW_PROMPT`. Per-agent overrides through class attributes are supported.

### Memory review prompt (verbatim core)

> "Review the conversation above and consider saving to memory if appropriate. ... If something stands out, save it once, in the right store, using the memory tool with the matching target. If nothing is worth saving, just say 'Nothing to save.' and stop."

It embeds `_MEMORY_ROUTING_BLOCK`, which is the heart of the user-modelling discipline:

> "USER.md (memory tool, target='user'): who the user is — persona, preferences, communication and work style, personal details they revealed, and expectations about how you should behave. MEMORY.md (memory tool, target='memory'): facts about the ENVIRONMENT you operate in ... One fact goes to ONE store, never both ..."

> "Save only facts GROUNDED in this conversation — a user message or tool result you can point to. Never infer the user's traits, skill level, role, mood or preferences from topic, tone or tools used, and never turn a one-off request into a standing preference (in either store) unless the user marked it general ('always', 'from now on', 'remember') or stated a standing like/dislike ('I hate X', 'stop doing Y'). If unsure, leave it out — a wrong fact misleads every future session."

A code comment ties the two-store rule to a regression: "Without this the reviewer wrote profile data into MEMORY.md and the same lesson into both stores until both hit their size limits (#30220)."

### Skill review prompt (interface only — skill mechanics belong to another subtopic)

It is deliberately biased toward action: "Be ACTIVE — most sessions produce at least one skill update, even if small. A pass that does nothing is a missed learning opportunity, not a neutral outcome." Its preference order is: patch a currently loaded skill → patch an existing umbrella → add a `references/`/`templates/`/`scripts/` support file → create a new class-level skill. It enforces read-before-write via `skill_view`, includes a "Do NOT capture" list (environment-dependent failures, negative claims about tools, transient errors, one-off narratives, unresolved failures), and says: "Never delete a skill here" (consolidation is left to the curator).

### The memory ↔ skill boundary (the interface)

The combined prompt states the split explicitly:

> "Memory says 'who the user is and what the current situation and state of your operations are'; skills say 'how to do this class of task for this user'. A user-preference lesson lives in exactly ONE place: the skill that governs the task when one exists, USER.md only for cross-cutting preferences no skill owns — never both."

So task-specific user corrections are routed into skills rather than USER.md. The foreground system prompt guidance says the same ("Skills come first ..."; [prompt_builder.py](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/agent/prompt_builder.py)). Memory is the residual store for cross-task facts.

## The fork: isolation, cost, safety

- **Cache parity**: on the same model, the fork reuses the parent's system prompt, reasoning settings, `tools[]` and conversation snapshot "byte-identical ... at fork birth so the review can reuse its prompt-cache prefix" (memory.md docs). Tools are advertised identically, but "Dispatch stays behind the review whitelist; advertising is not permission" (`_inherit_parent_tool_surface`).
- **Tool whitelist**: "Background review can use memory, skill-management, and read-only file tools by default" (docs). The file write tools and the terminal are denied. `auxiliary.background_review.extra_tools` can opt in named tools; the docs suggest "tools that stage a proposal for human review", e.g. `propose_shared_memory`.
- **No external provider in the fork**: built with `skip_memory=True`, because "an external memory plugin scoped to the parent's session_id would leak the harness prompt into the user's real memory namespace" (`background_review.py`; commit `4ece521b`, 2026-05-17, "isolate background review fork from external memory plugins (#27190)").
- **Bounds**: `_REVIEW_MAX_ITERATIONS = 16`. The input-token budget defaults to 75% of the review model's context window, capped at 600,000, with a 120,000 fallback (`max_input_tokens` overrides it).
- **Cheaper model routing**: `auxiliary.background_review.{provider,model}`. A different model gets a compact **digest** instead of the full transcript (`_digest_history`: the last 24 messages verbatim, older turns collapsed to `USER: …[:300]` / `ASSISTANT[tools: …]` lines). Docs claim "~3–5× in benchmarks" lower cost and that "memory capture was identical and skill capture near-identical to the main-model review". This is a vendor claim; I found no published benchmark data.
- **Disable**: `auxiliary.background_review.enabled: false` stops automatic forks, but `/refine` still works. Docs: "The review fork can burn a meaningful share of total tokens on busy hosts."
- **Local GPUs**: on the managed local llama-server, reviews are deferred until the machine is idle (`defer: auto`, `defer_max_age_s: 1800`). Queued reviews coalesce per session. A new prompt preempts a running review, which is then re-queued. The queue is in-memory, so reviews still pending at exit are lost.
- **Cancellation**: a new live turn supersedes a running review (`_interrupt_background_review`, "superseded by a new live turn"). Plugin-injected turns can wait up to 120 s for it (`_BACKGROUND_WORK_WAIT_SECONDS`).
- **Approval**: with `memory.write_approval: true` / `skills.write_approval: true`, background writes are staged. Background destructive memory edits are staged even with the gate off (see [builtin-memory-stores.md](./builtin-memory-stores.md)).
- **Telemetry and visibility**: usage is persisted in `session_model_usage` with `task='background_review'`, and an `agent.log` line is written. In chat, `display.memory_notifications: off|on|verbose` shows "💾 Memory updated" or diff previews.

## Plugins can follow the same cadence

`post_llm_call` hooks receive a read-only `review_signals` dict (`tool_iterations_since_skill_review`, `user_turns_since_memory_review`, `skill_review_due`, `memory_review_due`, `background_review_enabled`, both intervals), "so a plugin that does after-turn work can follow the user's self-learning cadence instead of counting tool calls itself" ([hooks.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/website/docs/user-guide/features/hooks.md)). Plugins with the `llm.session_fork` capability can run their own review-style fork (`ctx.fork_session`).

## Consolidation over time

- **Built-in memory**: there is no automatic summarisation. Consolidation happens only when a write overflows the char limit and the model must free space in the same batch. The docs recommend consolidating above 80%.
- **Skills**: the **curator** (another subtopic) is a separate idle-triggered background fork (default every 7 days after 2 h idle). It moves unused agent-created skills `active → stale (14d) → archived (30d)`. Optional LLM consolidation (`curator.consolidate`, off by default) merges skills into umbrellas. The curator "never auto-deletes" ([curator.md](https://github.com/NousResearch/hermes-agent/blob/d3ecbeeb09523d1c76264cae0e22d84fde59102a/website/docs/user-guide/features/curator.md)). A `post_curator_run` plugin hook exists.
- **Conversation context**: the context compressor summarises old turns inside a session (see [session-search-and-recall.md](./session-search-and-recall.md)). Before that lossy rewrite, external providers get `on_pre_compress`.
- **External providers** do their own consolidation, e.g. Mnemosyne's `mnemosyne_sleep` and Honcho's server-side "conclusions" (see [memory-providers-and-hooks.md](./memory-providers-and-hooks.md)).

## History

- The review prompts and the fork were extracted from `run_agent.py` into `agent/background_review.py` on 2026-05-17 (`1f6eb173`, `d35ee7bc`). The mechanism predates that.
- 2026-05-19 `af78449a` added bundled/pinned-skill protection rules. 2026-09-09 `1571f502` "scope background review memory access to its trigger (#105921)".
- An April 2026 write-up describes the nudge as "an internal system-level prompt" the agent receives "at set intervals during a session" ([Mr. Ånand, 2026-04-02](https://mranand.substack.com/p/inside-hermes-agent-how-a-self-improving)). I could not confirm from git history whether the nudge was once injected into the live conversation rather than a fork. Current code is fork-only.

## Observations (inference, labelled)

- *Inference:* The loop's quality depends heavily on the prompt text and its guardrails, which are tuned against observed failures (hoarding `references/` files, negative tool claims hardening into refusals, trait guessing). These prompts are the most reusable artifact for pbrain-2.
- *Inference:* Running the review after delivery, in a fork, on the warm cache is what makes "always-on learning" affordable without slowing the user. Its cost is real enough that Hermes ships switches to disable or downgrade it.
