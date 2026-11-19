# Memory types, staleness, contradiction and weighting

Researched 2026-11-19. Cross-cutting analysis of the systems in [systems-survey.md](./systems-survey.md). "Inference:" marks my reading.

## 1. Memory types in use

| System | Types it distinguishes |
|---|---|
| Claude Code auto memory | `user`, `feedback`, `project`, `reference` ([docs](https://code.claude.com/docs/en/memory)) |
| Hermes | environment facts (`MEMORY.md`), user profile (`USER.md`), procedural (skills), episodic (session history) ([factory#89](https://github.com/pkieltyka/factory/issues/89#issuecomment-7010508618)) |
| MemGPT / Letta | in-context working memory vs out-of-context recall (messages) and archival (text) ([arXiv 2310.08560](https://arxiv.org/abs/2310.08560)); now pinned-in-prompt files vs on-demand files ([MemFS](https://docs.letta.com/concepts/memfs/index.md)) |
| Generative Agents | observations, reflections (higher-level inferences), plans ([arXiv 2304.03442](https://arxiv.org/abs/2304.03442)) |
| Basic Memory | free observation categories: `[decision]`, `[fact]`, `[preference]`, `[risk]`… ([docs](https://docs.basicmemory.com/concepts/observations-and-relations)) |
| ACE | strategies, domain concepts, failure modes as bullets ([arXiv 2510.04618](https://arxiv.org/abs/2510.04618)) |
| Copilot | repository facts (with code citations) vs user-level preferences ([docs](https://docs.github.com/en/copilot/concepts/agents/copilot-memory)) |
| Factory notes | observation (provisional), learning (verified, operator only), decision (operator only, who decided) ([notes.md](/home/peter/Dev/pkieltyka/factory/skill/references/notes.md)) |

**Inference.** Two axes recur. *Who it is about*: the user/person vs the project/world. *How it binds*: a fact or observation (checkable, may go stale), a procedure or learning (reusable how-to), a decision (binding until replaced, not checkable against code), and episodic history (raw, rarely worth keeping; see CTIM-Rover below). Only factory gives decisions explicit authority. pbrain's spec already names decision, learning and observation. "Preference" can stay out of scope, since factory treats the user's preferences as brief material, not learnings (#89).

## 2. How contradiction and staleness are detected

| Approach | Who | When | Cost | Fits Markdown in Git? |
|---|---|---|---|---|
| LLM compares each new fact with similar stored ones and overwrites or deletes | mem0 paper (ADD/UPDATE/DELETE/NOOP) | write | 1+ LLM call per fact + embeddings | No: rewrites history, needs a vector index |
| LLM detects contradiction and *invalidates* the old edge (valid-time end set) | Zep/Graphiti, Mem0^g | write | LLM per write + graph DB | Partly: the invalidation could be written as a field, but detection needs an LLM |
| Store everything, resolve at read time with ranking | mem0 v3 ("ADD only … handles this at retrieval time") | read | ranking signals | Yes: append-only files plus derived index |
| Explicit supersession link written by whoever has authority | ADRs (Nygard, MADR, adr-tools), factory, OKF `superseded_by` | write, by a person or an authorised agent | none | Yes, natively |
| Verify a memory's citations against current code before use; re-store if wrong | GitHub Copilot | read | a few file reads | Yes, for code-checkable facts only |
| Usage decay: unused → stale → archived | Hermes skills (14/30 days), Copilot (expiry without reinforcement; downvotes), Claude memory tool guidance | background | counters | Counters in the derived index; the archive decision stays a human one |
| Absolute review date: stale means "check again" | OKF `stale_after` | read | none | Yes |
| Expiration hides but keeps | mem0 `expiration_date` ("fetching it by ID still returns it") | read | none | Yes |
| Periodic LLM lint or "contradiction check" writing a report | Karpathy LLM-wiki lint, Basic Memory Cloud agents, Letta `/doctor` | background | LLM pass | Yes, as a report or proposals only |

Sources: [mem0 paper](https://arxiv.org/html/2504.19413), [mem0 v3 migration](https://docs.mem0.ai/migration/platform-v2-to-v3.md), [Zep paper §2.2.3](https://arxiv.org/html/2501.13956), [Graphiti README](https://raw.githubusercontent.com/getzep/graphiti/main/README.md), [Copilot blog](https://github.blog/ai-and-ml/github-copilot/building-an-agentic-memory-system-for-github-copilot/), [Copilot docs](https://docs.github.com/en/copilot/concepts/agents/copilot-memory), [OKF v0.3](https://raw.githubusercontent.com/open-knowledge-format/open-knowledge-format/main/SPEC.md), [mem0 expiration](https://docs.mem0.ai/platform/features/memory-expiration.md), [Claude memory tool](https://docs.claude.com/en/docs/agents-and-tools/tool-use/memory-tool), [Karpathy gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f), [Letta memory](https://docs.letta.com/configuration/memory/index.md).

**The trend is from destructive updates to keep-and-supersede.** mem0 went from UPDATE/DELETE to "ADD only: nothing is overwritten or deleted". Graphiti: "invalidated — not deleted". Mem0^g marks relations "invalid rather than physically removing them". mem0 expiry "hides … does not delete". RD-Forget ([arXiv 2609.10263](https://arxiv.org/abs/2609.10263)): "A superseded fact can mislead a current-state answer and still be essential for a historical query", so it "separates what an agent stores from what it uses". This is exactly factory#62's "Superseded items are hidden by default, with history on request".

**Automatic contradiction resolution is weak.** MemoryAgentBench ([arXiv 2507.05257](https://arxiv.org/abs/2507.05257)): "all methods fail on the multi-hop situation (with achieving at most 28% accuracy)" on selective forgetting, and "Selective Forgetting still poses a significant challenge to all memory mechanisms" even with reasoning models. LongMemEval ([arXiv 2410.10813](https://arxiv.org/abs/2410.10813)) lists "knowledge updates" and "abstention" as separate abilities, and commercial assistants show "a 30% accuracy drop". Inference: an LLM that decides at write time what a new note contradicts would be unreliable. Explicit supersession by an authority, plus a lint that *proposes* conflicts, is more trustworthy and needs no model at run time.

**Newest-wins is the common default, and factory rejects it.** Graphiti "consistently prioritizes new information". Copilot refreshes a memory's timestamp when it re-validates it. Generative Agents rank by recency. Factory: "freshness alone does not replace" a decision (factory#62). Inference: pbrain's ordering is authority and explicit supersession first, then status (verified over provisional), then recency only as a tie-break among items of equal standing.

## 3. How validity and time are represented

- **Bi-temporal** (Zep): valid time [t_valid, t_invalid) and transaction time [t′created, t′expired) on each fact ([arXiv 2501.13956](https://arxiv.org/html/2501.13956)).
- **Valid-time qualifiers per fact** (Basic Memory v0.24): `@effective[2026-06-10,2026-07-27)`. Undated facts are excluded from valid-time queries ([docs](https://docs.basicmemory.com/concepts/metadata-search)).
- **Review date** (OKF `stale_after`), **expiry date** (mem0 `expiration_date`).
- **Written time only** (Claude Code `modified`, factory `created`).

Inference for a Markdown store: transaction time is free from Git (commit times) and from `created`. Valid time matters most for decisions. Supersession gives an implicit end ("in force from this item's acceptance until its successor's"), so a separate `valid_until` is rarely needed. An optional review date (`stale_after`, meaning "check again") covers time-limited facts. Avoid relative TTLs: OKF chose an absolute instant so that "the staleness decision is a plain comparison". Fail safe on bad dates (OKF: "MUST NOT treat a `stale_after` value that it cannot parse as fresh"). mem0 fails the other way ("Malformed dates fail open"). For a review flag, failing toward "check again" is the safer choice.

## 4. Weighting and usage signals

- **Generative Agents**: score = recency (0.995 decay per hour since last retrieval) + importance (LLM 1–10 at write) + relevance, equally weighted ([arXiv 2304.03442](https://arxiv.org/abs/2304.03442)).
- **ACE**: per-bullet helpful/harmful counters, updated deterministically ([arXiv 2510.04618](https://arxiv.org/abs/2510.04618)).
- **Copilot**: automatic up and down votes on validation. "Upvoted memories are more likely to be sent"; downvoted ones past a threshold "are no longer used and will be deleted after a period of disuse"; vote counts "intentionally not displayed" ([docs](https://docs.github.com/en/copilot/concepts/agents/copilot-memory)).
- **Hermes**: skill usage counters drive stale and archive ([#89](https://github.com/pkieltyka/factory/issues/89#issuecomment-7010508618)).
- **OKF**: refuses stored scores; records objective signals (`usage_count`, `last_modified`) and lets consumers infer ([SPEC](https://raw.githubusercontent.com/open-knowledge-format/open-knowledge-format/main/SPEC.md)).
- **Experience-following** ([arXiv 2505.16067](https://arxiv.org/abs/2505.16067)): agents copy retrieved past executions closely, so a wrong memory propagates. "Future task evaluations can serve as free quality labels for stored memory."

**Inference for pbrain.** Weights that change on every read (counts, votes, last-retrieved) must not go in the tracked files: every read would become a write and a merge conflict. They belong in the derived index, or in a local usage log that is not tracked. Two consequences: the counts are per machine unless shared some other way, and rebuilding the index loses them unless the log is kept. Weighting should only reorder items that are already eligible. It must never surface a superseded item in place of its successor or demote a decision below an observation.

## 5. Which mechanisms need what

| Mechanism | Markdown in Git is enough | Needs a derived local index (SQLite etc.) | Needs an LLM or embeddings at run time | Needs a server or graph DB |
|---|---|---|---|---|
| Explicit supersession and history | ✓ (front-matter link) | for fast reverse lookup | – | – |
| Authority (who decided, who verified) | ✓ | – | – | – |
| Hide superseded by default, get by id | ✓ (computed at read) | ✓ (fast) | – | – |
| Review date / expiry that hides but keeps | ✓ | – | – | – |
| Valid-time intervals per fact | ✓ (fields) | for interval queries | – | – |
| Citation verification at read (Copilot) | ✓ (source = path:line or commit) | – | the reading agent does it | – |
| Usage counts, decay, votes | ✗ (merge churn) | ✓ (or an untracked log) | – | for shared counts across machines |
| Lexical search with a result budget | rg (small) | ✓ FTS5 | – | – |
| Semantic or hybrid search | – | ✓ (vectors) | embeddings (local or remote) | – |
| Automatic contradiction detection | – | – | ✓ (unreliable per benchmarks) | Zep needs a graph DB |
| Automatic link suggestion (A-MEM) | – | ✓ | embeddings or LLM | – |
| Bi-temporal transaction time | ✓ via Git history | – | – | – |
| Concurrent multi-agent writers | ✓ with one file per item and worktrees (Letta does this) | – | – | Letta shared repos are cloud-hosted |
