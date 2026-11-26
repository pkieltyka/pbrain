# What Hermes is

## Bottom Line

The "Hermes" people praise for automatic skill creation and "learning by doing" is **Hermes Agent** by **Nous Research** ([NousResearch/hermes-agent](https://github.com/NousResearch/hermes-agent)). It is an MIT-licensed, self-hosted, model-agnostic Python agent harness with a closed learning loop: it creates and improves skills and nudges itself to keep memory. It launched publicly on 2026-02-25.

It shares a brand with Nous's older Hermes LLMs (Hermes 2/3/4) but is a separate product, and it works with any provider: Anthropic, OpenAI, OpenRouter, Nous Portal, or local Ollama/vLLM/llama.cpp. Models need at least 64k context and tool calling.

You can run it as:
- a TUI/CLI
- a single gateway process serving ~25 messaging platforms
- an ACP editor agent
- a Desktop app or browser Webapp
- cron jobs

Nous Portal is the paid side, offering models, tools and optional cloud hosting.

The "Hermes" that Mnemosyne integrates with (`mnemosyne-oss/mnemosyne/integrations/hermes`) is **the same Hermes Agent**. Mnemosyne is a third-party plugin for Hermes's single memory-provider slot.

The project is huge and moves fast: 266k stars, ~77k commits, weekly releases with daily canaries, and it is still pre-1.0 (v0.23.4, 2026-11-24). It also has a large backlog: 20k open issues and 34.5k open PRs. Pin versions when borrowing ideas.

## Key Findings

- **Identity.** The README says: "The self-improving AI agent built by Nous Research. It's the only agent with a built-in learning loop — it creates skills from experience, improves them during use, nudges itself to persist knowledge, searches its own past conversations…" ([README](https://github.com/NousResearch/hermes-agent/blob/main/README.md)).
- **"Co-worker" wording is not from Nous.** The official site does not use it. A team-colleague framing appears in NVIDIA's NemoClaw-for-Hermes blueprint, where teammates correct the agent and it turns guidance into skills ([NVIDIA](https://build.nvidia.com/nvidia/nemoclaw-for-hermes-agent.md)).
- **Maker and licence.** MIT, "Copyright (c) 2025 Nous Research". The first commit was by Teknium on 2025-07-23 ([LICENSE](https://github.com/NousResearch/hermes-agent/blob/main/LICENSE)). Nous reportedly raised ~$90M at ~$1.5B in Oct 2026 (secondary sources only; [The Block](https://www.theblock.co/post/408237)).
- **How it runs.**
  - Commands: `hermes` for the TUI, `hermes gateway` for Telegram/Discord/Slack/WhatsApp/Signal/Email and more, the ACP server for VS Code/Zed/JetBrains, cron, batch, API server, and Desktop/Webapp.
  - Six terminal backends: local, Docker, SSH, Modal, Daytona, Vercel Sandbox.
  - State lives in `~/.hermes`, with profiles via `-p`.
  - It can import from OpenClaw, Claude Code, Codex and Cursor.
  - Sources: [README](https://github.com/NousResearch/hermes-agent/blob/main/README.md), [architecture](https://hermes-agent.nousresearch.com/docs/developer-guide/architecture).
- **Models.** 18+ providers across three API modes (chat completions, Codex responses, Anthropic messages), plus local Ollama/vLLM/SGLang/llama.cpp/LM Studio. The docs say: "requires at least 64,000 tokens of context" ([providers](https://hermes-agent.nousresearch.com/docs/integrations/providers)).
- **Architecture.**
  - "One AIAgent class serves CLI, gateway, ACP, batch, and API server."
  - A tool registry with ~28 toolsets.
  - SQLite with FTS5 for session persistence.
  - Plugins for tools, hooks, platforms and memory. The memory provider slot is single-select.
  - Skills follow the agentskills.io format.
  - Since v0.23.0, one gateway per machine owns every session.
  - Sources: [architecture](https://hermes-agent.nousresearch.com/docs/developer-guide/architecture), [v0.23.0](https://github.com/NousResearch/hermes-agent/releases/tag/v0.23.0).
- **Releases.** v0.2.0 on 2026-03-12 was the first tag (216 PRs, 63 contributors). Named releases: Tenacity, Velocity, Surface, Judgment, Quicksilver, Threshold, Hearth (v0.23.0, 2026-11-05). The latest is v0.23.4 (2026-11-24). Tags moved from CalVer to SemVer in Oct 2026 ([releases](https://github.com/NousResearch/hermes-agent/releases)).
- **Mnemosyne is the same host.** It registers via the `hermes_agent.memory_providers` entry point, pins NousResearch/hermes-agent commit `f4ac0bea`, is listed in Nous's marketplace ("requires Hermes >=0.21.4"), and advises turning off Hermes's built-in MEMORY.md/USER.md ([integration README](https://github.com/mnemosyne-oss/mnemosyne/blob/main/integrations/hermes/README.md), [marketplace](https://hermes-agent.nousresearch.com/docs/plugins/mnemosyne)).

## Details

- [identity-and-disambiguation.md](./identity-and-disambiguation.md) — which Hermes is meant, the other things named Hermes, where the "co-worker" phrase comes from, and the evidence that Mnemosyne targets the same host.
- [hermes-agent-overview.md](./hermes-agent-overview.md) — maker, licence, Portal pricing, install and entry points, local vs hosted, model support and requirements, high-level architecture with module names.
- [release-history-and-maturity.md](./release-history-and-maturity.md) — dated release timeline, activity numbers, and a maturity assessment.
- [mnemosyne-integration.md](./mnemosyne-integration.md) — the `integrations/hermes` package identity, versions and install; how it relates to built-in memory.

## Open Questions / Gaps

- I found no primary Nous launch post. The launch date (Feb 25 vs Feb 26, 2026) comes from nousresearch.com/releases and Product Hunt.
- Funding figures come only from press and aggregators.
- The "co-worker" phrasing was not traced to a specific user post; the Evidence subtopic owns that.
- I read the architecture page through WebFetch summaries, not raw page text. Module names match the repo root listing.
- The open issue and PR counts are huge. I did not find out how much of that is spam or agent-generated.

## Sources

- [NousResearch/hermes-agent README](https://github.com/NousResearch/hermes-agent/blob/main/README.md) — identity, features, install, commands (main, 2026-11-26)
- [GitHub API repo metadata and releases](https://github.com/NousResearch/hermes-agent/releases) — stars, licence, release dates (snapshot 2026-11-26)
- [v0.2.0 release notes](https://github.com/NousResearch/hermes-agent/releases/tag/v2026.3.12) — first tag, 2026-03-12
- [v0.23.0 release notes](https://github.com/NousResearch/hermes-agent/releases/tag/v0.23.0) — Hearth release, 2026-11-05
- [pyproject.toml](https://github.com/NousResearch/hermes-agent/blob/main/pyproject.toml) and [LICENSE](https://github.com/NousResearch/hermes-agent/blob/main/LICENSE) — authorship, Python versions, dependency policy
- [Architecture docs](https://hermes-agent.nousresearch.com/docs/developer-guide/architecture) — components (current docs)
- [Providers docs](https://hermes-agent.nousresearch.com/docs/integrations/providers) — models and requirements (current docs)
- [hermes-agent.nousresearch.com](https://hermes-agent.nousresearch.com/) — tagline, Portal pricing, local vs hosted
- [nousresearch.com/releases](https://nousresearch.com/releases) — launch date 02/25/26; Hermes LLM family history
- [mnemosyne integrations/hermes README](https://github.com/mnemosyne-oss/mnemosyne/blob/main/integrations/hermes/README.md) and [plugin.yaml](https://github.com/mnemosyne-oss/mnemosyne/blob/main/integrations/hermes/plugin.yaml) — host identity (v0.7.8 in source)
- [Hermes marketplace: mnemosyne](https://hermes-agent.nousresearch.com/docs/plugins/mnemosyne) — listing v0.7.7
- [NVIDIA NemoClaw for Hermes Agent](https://build.nvidia.com/nvidia/nemoclaw-for-hermes-agent.md) — team/"co-worker" framing (search summary only)
- [The Block, 2026-07-14](https://www.theblock.co/post/408237) — funding (secondary)
