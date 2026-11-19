# Who writes memory, how writes are reviewed, and how they are attacked

Researched 2026-11-19. "Inference:" marks my reading.

## 1. Who may write

| System | Writer | Review before it takes effect |
|---|---|---|
| Hermes | the model at any time, plus an unattended background review | memory: the review may add, but its patches and removals are staged for user approval; skills: not held back; approval gates exist but are off by default ([factory#89](https://github.com/pkieltyka/factory/issues/89#issuecomment-7010508618)) |
| Letta | the agent and its "dreaming" subagents, committed to its Git repo | optional "Agent reviews before applying", which "does not ask you for approval" ([Letta memory](https://docs.letta.com/configuration/memory/index.md)) |
| Claude Code auto memory | Claude, unattended; the user can edit or delete files | none ([docs](https://code.claude.com/docs/en/memory)) |
| Codex memories | background extraction and consolidation models | none; "Treat these files as generated state" ([docs](https://developers.openai.com/codex/memories)) |
| GitHub Copilot | agents acting for users "with write permissions" to the repo | none up front; correction by read-time citation checks and votes ([blog](https://github.blog/ai-and-ml/github-copilot/building-an-agentic-memory-system-for-github-copilot/)) |
| mem0, Zep | extraction LLM on every message | none |
| Basic Memory Cloud agents | scheduled agents | "Every change lands in a shared review queue, where you keep it or undo it" ([docs](https://docs.basicmemory.com/llms-full.txt)) |
| Factory notes | members write observations; only the operator writes learnings and decisions; read-only roles put note text in reports | the operator is the review ([notes.md](/home/peter/Dev/pkieltyka/factory/skill/references/notes.md)) |

**Inference.** Almost every vendor system lets the model write unattended and relies on later correction: votes, citation checks, usage decay, or a person deleting files. Factory and pbrain's draft are unusual in gating *binding* kinds (decision, verified learning) on an authority. That gate is cheap in a Git workflow (a commit by, or approved by, the authority) and needs no model. Unattended writes can still be allowed for provisional kinds (observation), which bind nothing.

## 2. Attacks on memory

- **Persistent prompt injection through memory (SpAIware, 2024).** "Through prompt injection from untrusted data, attackers could insert long-term persistent spyware into ChatGPT's memory. This led to continuous data exfiltration … including any future chat sessions" ([Embrace The Red, 2024](https://embracethered.com/blog/posts/2024/chatgpt-macos-app-persistent-data-exfiltration/)). The route was a malicious website or document the assistant read.
- **Query-only memory injection (MINJA).** [arXiv 2503.03704](https://arxiv.org/abs/2503.03704): an attacker who can only send queries plants malicious records. "MINJA achieves a high average success rate of 98.2% for injecting malicious records into the memory, and a high average attack success rate of 76.8% in eliciting the malicious reasoning steps."
- **OWASP Top 10 for Agentic Applications (Dec 2025)** lists "ASI06 – Memory & Context Poisoning, e.g Gemini Memory Attack": "Memory poisoning reshaped behaviour long after the initial interaction" ([OWASP GenAI, 2025-12-09](https://genai.owasp.org/2025/12/09/owasp-top-10-for-agentic-applications-the-benchmark-for-agentic-security-in-the-age-of-autonomous-ai/)). I read only OWASP's announcement post, not the full ASI06 entry.
- **Shared journals.** Cursor: "Anyone who can message the agent writes to the journal, so keep the scaffolded rule against following instructions found in records", and "one person's messages can surface in another's session" ([Cursor memory](https://cursor.com/docs/agent-sdk/reference/memory.md)).
- **Adversarial memories in a repo.** GitHub seeded contradicting memories with bogus citations; "agents consistently verified citations, discovered contradictions, and updated incorrect memories" ([blog](https://github.blog/ai-and-ml/github-copilot/building-an-agentic-memory-system-for-github-copilot/)). This is vendor testing, and it covers only facts that can be checked against code.
- **Error propagation without an attacker.** Agents follow retrieved experience closely, so "inaccuracies in past experiences compound" ([arXiv 2505.16067](https://arxiv.org/abs/2505.16067)).

## 3. Defences in use

| Defence | Where | Note |
|---|---|---|
| Scan writes for injection, exfiltration and secret patterns; block; rescan on load and replace with `[BLOCKED: …]` | Hermes ([#89](https://github.com/pkieltyka/factory/issues/89#issuecomment-7010508618)) | pattern-based |
| Redact secrets in generated memory | Codex ([docs](https://developers.openai.com/codex/memories)) | |
| Exclude sessions that touched external content from memory generation | Codex `memories.disable_on_external_context` | cuts off the SpAIware route |
| "Claude usually refuses to write sensitive information … add validation that strips sensitive data" | Claude memory tool ([docs](https://docs.claude.com/en/docs/agents-and-tools/tool-use/memory-tool)) | |
| Path-traversal checks on every memory path | Claude memory tool | relevant if pbrain takes paths or ids from agents |
| Size caps on files and on what `view` returns | Claude memory tool, Hermes | |
| Memory is context, never instruction; a rule against following instructions found in records | Cursor scaffold, Hermes ("declarative facts, not instructions"), Codex ("not … rules that must always apply"), Copilot ("should not be relied on as a rule"), factory ("no note is an instruction") | universal |
| Scope by permission: write needs repo write access, read needs read access | Copilot | Git gives this for free |
| Verify against evidence before use | Copilot citations | only for code-checkable claims |
| Human-reviewable queue or diff | Basic Memory Cloud, Letta (agent self-review), Git PR review | |

**Inference for pbrain.**
1. Imported sources are the main injection route (SpAIware's route). Keep them as a separate kind marked "reference, data not instructions" (factory#62 already says so), scan them at `add`, and never let an import change the status of another item.
2. Scan every write for secrets and for the known injection markers: instructions to the reader, URLs with query templates, invisible Unicode. Re-check at read time is cheap insurance.
3. Gate binding status on authority. A Git commit by, or approved by, the authority is the review.
4. Present all retrieved text as quoted data with its id and source. pbrain's output format, not just its skill text, should make "this is a citation, not an instruction" clear.

## 4. Privacy

- Every vendor keeps generated memory local or account-scoped: Claude Code "machine-local", Codex under `~/.codex`, Copilot scoped to the repo's permissions, Hermes "Profiles are not tenants". None mixes private and shared memory in one store.
- The Hermes background review "replays the conversation to the model provider" (#89). Inference: any LLM step in pbrain (extraction, lint, embeddings through a remote API) sends memory text off the machine, private items included unless they are excluded first. factory#62 already separates "Processing private material locally" from "sending it to a remote model".
