# Session-start priming, on-demand retrieval, budgets, citation and evaluation

Researched 2026-11-19. "Inference:" marks my reading.

## 1. Priming vs on-demand

| System | Injected at session start | Fetched on demand |
|---|---|---|
| Claude Code | `MEMORY.md` index: "first 200 lines … or the first 25KB, whichever comes first" | topic files, read with file tools ([docs](https://code.claude.com/docs/en/memory)) |
| Hermes | `MEMORY.md` + `USER.md` snapshot "frozen" for the session (~1,300 tokens at the caps) + skill index (name + ≤60-char description) | skill bodies; session search over SQLite FTS ([#89](https://github.com/pkieltyka/factory/issues/89#issuecomment-7010508618)) |
| Letta MemFS | root files (or `system/`) in full, every turn, plus the file tree | everything else, read with file tools; "directory and file names act as signposts" ([MemFS](https://docs.letta.com/concepts/memfs/index.md)) |
| Copilot | "the most recent memories for the target repository" | none yet: "Future implementations will enable additional retrieval techniques, such as a search tool and weighted prioritization" ([blog](https://github.blog/ai-and-ml/github-copilot/building-an-agentic-memory-system-for-github-copilot/)) |
| Claude memory tool | none built in; "Claude automatically checks its memory directory before starting a task" (a `view` call) | `view` with `view_range` paging ([docs](https://docs.claude.com/en/docs/agents-and-tools/tool-use/memory-tool)) |
| Codex | memories injected when `memories.use_memories` is on (no size stated) | – ([docs](https://developers.openai.com/codex/memories)) |
| REM (coding agents) | a consolidated document "two orders of magnitude smaller" than the trajectory | the fuller record only when an in-loop gate detects "struggle"; evicted after one exposure ([NeurIPS 2026 listing](https://neurips.cc/virtual/2026/165740), via search summary, not verified first-hand) |
| Generative Agents | – | "top-ranked memories that fit within the language model's context window" each step ([arXiv 2304.03442](https://arxiv.org/abs/2304.03442)) |

**Patterns.**
- **Small, frozen, index-shaped priming plus pull on demand** is the norm. Hermes freezes its snapshot to keep the prompt cache. Claude Code and Letta inject an index or tree rather than contents.
- **Descriptions are the routing key.** Hermes skills, Letta front-matter `description`, Cursor's description-gated rules and OKF's `description` all need a one-line summary per item.
- **Context cost is the stated reason** in every system: Hermes "preserving the prefix cache", ACE's worry about context collapse, factory#62's "sessions start near 40,000 tokens and cache reads are 97% of all tokens".

**Inference for pbrain.** The default should be on-demand: bounded `search` and exact `get`. Session priming is optional and opt-in through factory's recipe. If used, it should be a fixed, capped slice (one line per item: id, kind, title, decided_by) of the decisions in force relevant to the role, generated once per session so the cache holds. Nothing should be injected that the agent could fetch by id when it needs it.

## 2. Budgets and citations in results

- Hard caps seen: Claude Code 200 lines / 25 KB; Hermes 2,200 + 1,375 characters; Codex AGENTS.md 32 KiB; Claude memory tool advice to cap `view` output and page with `view_range`.
- Citations: Copilot stores `citations` (file:line) with each fact and verifies them before use. OKF uses keyed footnotes to `sources[].id` ("Labels are keyed rather than positional … because agents constantly rewrite these documents"). Basic Memory results carry `memory://` permalinks; with valid-time queries, "Results are observation-level, each carrying the assertion that matched".
- mem0 measures the "Token Consumption … of the memories retrieved" as a first-class metric next to accuracy ([arXiv 2504.19413](https://arxiv.org/html/2504.19413)).

**Inference.** pbrain's search result should be a fixed small shape: id, kind, status, title, decided_by/author, a short snippet and the source. That gives a citation by construction, and the caller can `get` by id for the body. A result budget in items *and* characters, plus an explicit "N more not shown", follows the Claude memory tool's paging advice.

## 3. Evaluation methods

| Benchmark / method | What it tests | Relevance to pbrain |
|---|---|---|
| LoCoMo ([arXiv 2402.17753](https://arxiv.org/abs/2402.17753)) | very long conversations ("300 turns and 9K tokens on avg., over up to 35 sessions"): QA, event summarisation | conversational recall; weak fit |
| LongMemEval ([arXiv 2410.10813](https://arxiv.org/abs/2410.10813), ICLR 2025) | "information extraction, multi-session reasoning, temporal reasoning, knowledge updates, and abstention"; 500 questions | **knowledge updates** and **abstention** match supersession and "say you don't know" |
| MemoryAgentBench ([arXiv 2507.05257](https://arxiv.org/abs/2507.05257)) | "accurate retrieval, test-time learning, long-range understanding, and selective forgetting"; FactConsolidation | selective forgetting = supersession; "all methods fail on the multi-hop situation (with achieving at most 28% accuracy)" |
| DMR (MemGPT) | deep memory retrieval | saturated: Zep 94.8% vs MemGPT 93.4% ([arXiv 2501.13956](https://arxiv.org/abs/2501.13956)) |
| Vendor A/B in production | Copilot: PR merge rate 90% vs 83%; review precision +3%, recall +4% | the only real-use evidence found; vendor-run |
| Adversarial seeding | Copilot: seeded contradicting memories with bad citations | a cheap test pbrain can copy |
| Token / latency cost | mem0: tokens retrieved, p95 latency | pbrain's budget criterion |

**Inference: what pbrain should evaluate.** Public benchmarks are conversation-QA shaped and score an LLM's answers, which pbrain does not produce. Factory#62's fixture already captures the core: a decision superseded by its successor, a newer provisional note that disagrees, and a private note with a unique term. It checks that the successor comes back with who decided and the source within budget, that history finds the old item, that the private term never leaks, and that a rebuilt index gives the same answers. Additions worth considering, drawn from the above:
1. **Abstention**: a query with no match returns an explicit empty result, never the nearest wrong item (LongMemEval).
2. **Multi-hop supersession**: A → B → C, where a query for A's topic returns C and history shows the chain (MemoryAgentBench's multi-hop failure).
3. **Adversarial item**: an imported source containing instructions shows up as quoted data and changes no other item's status (SpAIware, MINJA, Copilot seeding).
4. **Token budget**: results for a fixed query set stay under N tokens (mem0's metric).
5. **Usage signal**: a retrieval count increments without changing any tracked file.
