# Agent memory systems

## Bottom Line

The field is moving away from destructive, LLM-decided memory updates and toward keeping everything and controlling what is used. mem0 replaced ADD/UPDATE/DELETE with "ADD only: nothing is overwritten or deleted" and resolves conflicts at retrieval time. Zep/Graphiti invalidates rather than deletes. ACE shows that letting a model rewrite its accumulated context collapses it. Automatic contradiction handling is still weak: on MemoryAgentBench every method scored at most 28% on multi-hop selective forgetting. Letta has moved memory to Markdown with front matter in a Git repository, with worktrees for concurrent writers and qmd for optional semantic search, which shows pbrain's storage choice works for agents. Nobody except factory gives memory *authority*: vendor systems let the model write unattended and correct it later with votes, citations or decay, and memory poisoning (SpAIware, MINJA 98% injection success, OWASP ASI06) is a documented threat. For a Markdown-in-Git store shared by a multi-agent team, explicit supersession, authority, review dates, hide-but-keep, citation checks and write-time scanning need no database. Usage signals, fast search and anything semantic belong in a derived local index. Automatic contradiction detection is not reliable enough to decide, only to propose.

## Key Findings

- **Keep and supersede, don't overwrite.** mem0 v3: "When information changes … both facts are preserved with temporal context", and conflicts are handled "at retrieval time instead" ([migration](https://docs.mem0.ai/migration/platform-v2-to-v3.md)). Graphiti: "old facts are invalidated — not deleted". RD-Forget: "A superseded fact can mislead a current-state answer and still be essential for a historical query" ([arXiv 2609.10263](https://arxiv.org/abs/2609.10263)). This matches factory#62's "hidden by default, history on request".
- **Rewriting by a model destroys knowledge.** ACE's "context collapse": 18,282 tokens to 122 in one step, with accuracy falling below the no-memory baseline. Its fix is itemised entries with ids, append-only deltas and deterministic non-LLM merges ([arXiv 2510.04618](https://arxiv.org/abs/2510.04618)).
- **Automatic forgetting and contradiction is unsolved.** MemoryAgentBench: "all methods fail on the multi-hop situation (with achieving at most 28% accuracy)" ([arXiv 2507.05257](https://arxiv.org/abs/2507.05257)). LongMemEval treats knowledge updates and abstention as separate hard abilities ([arXiv 2410.10813](https://arxiv.org/abs/2410.10813)). Zep's write-time LLM invalidation exists but is "newest wins".
- **Bi-temporal time is the right model, and Git gives half of it for free.** Zep stores valid time [t_valid, t_invalid) and system time [t′created, t′expired) per fact ([arXiv 2501.13956](https://arxiv.org/html/2501.13956)). In pbrain, Git and `created` give system time, and supersession bounds a decision's validity.
- **Git-backed Markdown memory exists in production.** Letta MemFS: "Every memory edit is committed"; "Memory subagents … use git worktrees so they can update memory concurrently"; root files are pinned in context and the rest is reached through `description` front matter and `MEMORY.md` signposts; semantic search is via qmd ([MemFS](https://docs.letta.com/concepts/memfs/index.md), [Context Repositories](https://www.letta.com/blog/context-repositories)).
- **Citations make memory checkable.** Copilot stores `fact` + `citations` (file:line) + `reason` and verifies citations before use. In adversarial seeding, "agents consistently verified citations, discovered contradictions, and updated incorrect memories". A/B: PR merge rate 90% vs 83% ([blog](https://github.blog/ai-and-ml/github-copilot/building-an-agentic-memory-system-for-github-copilot/)). This works for facts about code, not for decisions.
- **Usage signals are common and always derived.** Copilot votes (downvoted memories stop being used, then are deleted after disuse), Hermes skill usage (stale at 14 days, archived at 30), ACE helpful/harmful counters, and Generative Agents recency (0.995 decay per hour since last retrieval) + importance + relevance. Kept in tracked files, they would turn every read into a write.
- **Unfiltered episodic memory hurts coding agents.** CTIM-Rover's repository memory "does not outperform AutoCodeRover in any configuration" because of noise ([arXiv 2505.23422](https://arxiv.org/abs/2505.23422)). Agents copy retrieved experience, so errors propagate ([arXiv 2505.16067](https://arxiv.org/abs/2505.16067)). Curated, small, verified items help (repository memory from commits, ICLR 2026, [arXiv 2510.01003](https://arxiv.org/abs/2510.01003)).
- **Memory is an attack surface.** SpAIware planted persistent exfiltration instructions in ChatGPT memory through untrusted content ([Embrace The Red, 2024](https://embracethered.com/blog/posts/2024/chatgpt-macos-app-persistent-data-exfiltration/)). MINJA reaches 98.2% injection and 76.8% attack success through queries alone ([arXiv 2503.03704](https://arxiv.org/abs/2503.03704)). OWASP lists ASI06, Memory & Context Poisoning. Defences in use: write-time scanning and secret redaction (Hermes, Codex), excluding sessions that touched external content (Codex), "memory is not an instruction" in every vendor's guidance, and permission-scoped writes (Copilot).
- **Priming is small, frozen and index-shaped.** Claude Code loads 200 lines or 25 KB of index; Hermes a ~1,300-token frozen snapshot plus a skill index; Letta pinned files plus the tree; Copilot "the most recent memories". Bodies are fetched on demand.
- **Only factory gives memory authority.** Vendor systems let the model write unattended (Claude Code, Codex, Letta dreaming, mem0, Zep) or use approval only as an opt-in (Hermes, Basic Memory Cloud review queue). factory#89 already rejected unattended writes with pressure to save for a team.

## Conclusion: what suits a Markdown-in-Git store for a multi-agent team

**Works in files alone (no database):** explicit `supersedes` links and status; authority (`decided_by`, `verified`); hide-superseded-by-default with get-by-id and history; optional review date (`stale_after`, meaning "check again", failing toward stale); source and citations (URL, file:line, commit) that an agent can verify; one item per file so concurrent writers merge cleanly (Letta's worktree pattern); transaction time from Git. Write-time scanning for secrets and injection markers is a CLI check, not storage.

**Needs a derived, local, rebuildable index:** fast lexical search with result budgets (FTS5); reverse edges ("what superseded X"); usage counts, last-retrieved and helpful/harmful signals (index or untracked log, never tracked files; per-machine unless shared); optional semantic or hybrid search with embeddings (qmd, as Letta does).

**Needs a model, and should only propose:** contradiction or staleness lint, link suggestion (A-MEM-style), and extraction from conversations. Benchmarks show these are unreliable as deciders. Their output is a report or provisional items for the authority to accept.

**Needs a server, graph database or vendor platform, and is not recommended for pbrain v1:** Zep-style bi-temporal graphs with write-time LLM invalidation; usage counts shared across machines; cloud-hosted shared memory (Letta Cloud, Basic Memory Teams).

## Details

- [Systems survey](./systems-survey.md): per-system mechanics with quotes. Hermes (from #89), Zep/Graphiti, mem0 (paper → v3), MemGPT → Letta MemFS, A-MEM, ACE, Generative Agents, Claude Code and the memory tool, Codex, Cursor, Copilot, and coding-agent memory papers.
- [Staleness, contradiction and weighting](./staleness-contradiction-and-weighting.md): memory types, the detection approaches compared, time representations, weighting signals, and a mechanism-by-requirement table (files / index / model / server).
- [Writes, safety and review](./writes-safety-and-review.md): who writes and who reviews in each system, documented attacks (SpAIware, MINJA, OWASP ASI06), defences in use, and privacy.
- [Retrieval, priming, budgets and evaluation](./retrieval-priming-budgets-and-eval.md): session-start vs on-demand by system, budgets, citation shapes, benchmarks (LoCoMo, LongMemEval, MemoryAgentBench, DMR, Copilot A/B), and suggested additions to pbrain's acceptance fixture.

## Open Questions / Gaps

- REM (NeurIPS 2026) was read only through a search engine's summary of its listing; no arXiv full text was found. Treat its claims as unverified.
- The OWASP ASI06 entry itself was not read, only OWASP's announcement post that names it.
- Benchmark numbers (Zep DMR and LongMemEval, mem0 LoCoMo, Copilot A/B) are self-reported by the vendors. I did not look for independent replications or the published Zep-vs-mem0 benchmark disputes.
- I found no published data on Git merge-conflict rates for multi-agent memory repositories. Letta describes "git-based conflict resolution" but gives no numbers.
- How Codex sizes the memory it injects is not documented on the page read.
- Hermes was not re-read. Its facts come from factory#89 at commit `c152cfceec` and may have changed since. Only its memory-providers page was fetched fresh.
- Not covered: Honcho, Supermemory, MemoryOS, MIRIX and other memory vendors beyond a mention; reinforcement-trained memory policies; the full survey literature ("Memory in the Age of AI Agents" and similar).

## Sources

- [factory#89 Hermes study](https://github.com/pkieltyka/factory/issues/89#issuecomment-7010508618): Hermes at `c152cfceec`; architect and lead-dev, 2026
- [Hermes memory providers](https://hermes-agent.nousresearch.com/docs/user-guide/features/memory-providers): external provider plugins; current docs
- [Zep paper, arXiv 2501.13956](https://arxiv.org/abs/2501.13956) and [Graphiti README](https://raw.githubusercontent.com/getzep/graphiti/main/README.md): bi-temporal edges, invalidation; Jan 2025 / current
- [mem0 paper, arXiv 2504.19413](https://arxiv.org/abs/2504.19413): ADD/UPDATE/DELETE/NOOP, LoCoMo; Apr 2025
- [mem0 v2→v3 migration](https://docs.mem0.ai/migration/platform-v2-to-v3.md) and [memory expiration](https://docs.mem0.ai/platform/features/memory-expiration.md): ADD-only, hide-not-delete; current docs
- [MemGPT, arXiv 2310.08560](https://arxiv.org/abs/2310.08560): main and external context tiers; 2023
- [Letta MemFS](https://docs.letta.com/concepts/memfs/index.md), [Memory & dreaming](https://docs.letta.com/configuration/memory/index.md), [Shared memory](https://docs.letta.com/concepts/shared-memory/index.md), [SDK memory](https://docs.letta.com/agent-sdk/memory/index.md), [Context Repositories blog](https://www.letta.com/blog/context-repositories): Git-backed memory; Feb 2026 / current
- [A-MEM, arXiv 2502.12110](https://arxiv.org/abs/2502.12110): Zettelkasten-style links and memory evolution; Feb 2025
- [ACE, arXiv 2510.04618](https://arxiv.org/abs/2510.04618): context collapse, itemised deltas, counters; ICLR 2026
- [Generative Agents, arXiv 2304.03442](https://arxiv.org/abs/2304.03442): recency, importance and relevance retrieval; 2023
- [Claude Code memory](https://code.claude.com/docs/en/memory) and [Claude memory tool](https://docs.claude.com/en/docs/agents-and-tools/tool-use/memory-tool): auto memory, budgets, security guidance; current
- [Codex memories](https://developers.openai.com/codex/memories): local generated memories, redaction, external-context exclusion; current
- [Cursor Agent SDK memory](https://cursor.com/docs/agent-sdk/reference/memory.md): journal and injection caveats; current
- [GitHub blog: agentic memory for Copilot](https://github.blog/ai-and-ml/github-copilot/building-an-agentic-memory-system-for-github-copilot/) and [Copilot Memory docs](https://docs.github.com/en/copilot/concepts/agents/copilot-memory): citations, voting, expiry, A/B; Jan 2026 / current
- [How Memory Management Impacts LLM Agents, arXiv 2505.16067](https://arxiv.org/abs/2505.16067): experience-following, error propagation; May 2025
- [CTIM-Rover, arXiv 2505.23422](https://arxiv.org/abs/2505.23422): episodic memory noise in SWE agents; 2025
- [Repository memory, arXiv 2510.01003](https://arxiv.org/abs/2510.01003): commit-history memory for localisation; ICLR 2026
- [RD-Forget, arXiv 2609.10263](https://arxiv.org/abs/2609.10263): separating stored from used memory; Sep 2026
- [REM, NeurIPS 2026 listing](https://neurips.cc/virtual/2026/165740): lifecycle governance (via search summary only)
- [LoCoMo, arXiv 2402.17753](https://arxiv.org/abs/2402.17753), [LongMemEval, arXiv 2410.10813](https://arxiv.org/abs/2410.10813), [MemoryAgentBench, arXiv 2507.05257](https://arxiv.org/abs/2507.05257): evaluation; 2024–2025
- [MINJA, arXiv 2503.03704](https://arxiv.org/abs/2503.03704): query-only memory injection; Mar 2025
- [SpAIware, Embrace The Red](https://embracethered.com/blog/posts/2024/chatgpt-macos-app-persistent-data-exfiltration/): persistent memory injection in ChatGPT; Sep 2024
- [OWASP Top 10 for Agentic Applications announcement](https://genai.owasp.org/2025/12/09/owasp-top-10-for-agentic-applications-the-benchmark-for-agentic-security-in-the-age-of-autonomous-ai/): ASI06 Memory & Context Poisoning; Dec 2025
