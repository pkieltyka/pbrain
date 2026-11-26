# Hermes Agent: maker, licence, running it, architecture

Retrieved 2026-11-26. Main branch `main`, latest stable v0.23.4 (2026-11-24).

## Maker and licence

- **Maker:** Nous Research. The LICENSE says "Copyright (c) 2025 Nous Research", and pyproject has `authors = [{ name = "Nous Research" }]` ([LICENSE](https://github.com/NousResearch/hermes-agent/blob/main/LICENSE), [pyproject.toml](https://github.com/NousResearch/hermes-agent/blob/main/pyproject.toml)). The first commit, "initital commit", is by **Teknium** on 2025-07-23 ([commit 21d80ca](https://github.com/NousResearch/hermes-agent/commit/21d80ca68346dfdb8d3556015a723a9217f8566f)).
- **Licence:** MIT for the agent software. The site FAQ says it is "free and open source under the MIT license" ([site](https://hermes-agent.nousresearch.com/)).
- **Business model:** Nous Portal is the paid side: models, a "Tool Gateway" for web search, image generation, TTS and a cloud browser, and optional cloud hosting. Plans per the landing page: Free $0 (pay as you go, free models only, no hosting); Plus $20/mo (cloud host with 4 GB RAM, 25 GB disk); Super $100/mo; Ultra $200/mo ([site](https://hermes-agent.nousresearch.com/), [README](https://github.com/NousResearch/hermes-agent/blob/main/README.md)).
- **Company context (secondary sources, not verified against a primary announcement):** Several outlets report a ~$90M Series B at a ~$1.5B valuation in October 2026, tied to a forthcoming "Hermes for Businesses" product. July reports put the round at $75M ([The Block, 2026-07-14](https://www.theblock.co/post/408237), [Techzine](https://www.techzine.eu/news/applications/142886/creator-of-the-successful-hermes-agent-now-valued-at-1-5-billion/)).

## How it is run

**Install and stack.** It is a Python application. pyproject says `requires-python = ">=3.11,<3.15`, but a comment adds "we *only* support 3.14". There is a Node/pnpm workspace for the TUI and web apps. Every direct dependency is exact-pinned, a policy "tightened on 2026-05-12 in response to the Mini Shai-Hulud worm" ([pyproject.toml](https://github.com/NousResearch/hermes-agent/blob/main/pyproject.toml)). Install options:

- Linux/macOS/WSL2: `curl -fsSL https://hermes-agent.nousresearch.com/install.sh | bash`
- Native Windows: a PowerShell one-liner, or an MSIX package
- Android/Termux: a signed APT repository
- Docker image and Nix flake (`Dockerfile` and `flake.nix` are in the repo root)

([README](https://github.com/NousResearch/hermes-agent/blob/main/README.md))

**Entry points.** The README quotes these commands:

- `hermes` starts the "Interactive CLI"; the full TUI has "multiline editing, slash-command autocomplete, … interrupt-and-redirect, and streaming tool output"
- `hermes model`, `hermes tools`, `hermes config set/get`, `hermes setup` (wizard; `--portal` for Nous OAuth), `hermes update`, `hermes doctor`
- `hermes gateway` starts the messaging gateway. The README lists Telegram, Discord, Slack, WhatsApp, Signal and Email "from a single gateway process". The architecture page says ~25 platforms ship as plugins.
- **ACP server** for VS Code, Zed and JetBrains ("editor-native agent over stdio/JSON-RPC")
- Batch runner and API server; cron scheduler
- **Desktop app** (macOS/Windows). Since v0.23.0, a **Webapp** serves the Desktop workspace from your own gateway in a browser tab; it is off by default ([v0.23.0 notes](https://github.com/NousResearch/hermes-agent/releases/tag/v0.23.0))
- `hermes -w` runs a session in a git worktree; `hermes -p <name>` selects an isolated profile ([v0.2.0 notes](https://github.com/NousResearch/hermes-agent/releases/tag/v2026.3.12), [architecture](https://hermes-agent.nousresearch.com/docs/developer-guide/architecture))

**Local vs hosted.** It is self-hosted by default ("lives on your server"). It runs on a laptop, "a $5 VPS, a GPU cluster, or serverless infrastructure". Six built-in terminal backends: "local, Docker, SSH, Modal, Daytona, and Vercel Sandbox", plus plugin backends such as Singularity/Apptainer. Nous Portal offers an optional managed cloud agent ([README](https://github.com/NousResearch/hermes-agent/blob/main/README.md), [site](https://hermes-agent.nousresearch.com/)). State lives under `~/.hermes`, or `%LOCALAPPDATA%\hermes` on native Windows.

**Migration.** `hermes import-agent openclaw|claude-code|codex|cursor` imports settings, MEMORY.md/USER.md, skills and SOUL.md ([README](https://github.com/NousResearch/hermes-agent/blob/main/README.md)). This shows Hermes positioning itself as a successor to OpenClaw.

## Models

The agent is model-agnostic and is "Not tied to" Nous's own models. You switch with `hermes model` or `/model provider:model` ([README](https://github.com/NousResearch/hermes-agent/blob/main/README.md)). The [providers page](https://hermes-agent.nousresearch.com/docs/integrations/providers) lists:

- **Hosted:** Nous Portal ("300+ models"), OpenAI Codex (ChatGPT subscription), Anthropic, OpenRouter, OpenAI API, Google Gemini/Vertex, Azure AI Foundry, AWS Bedrock, NVIDIA NIM, xAI, DeepSeek, Kimi/Moonshot, z.ai/GLM, MiniMax, Qwen/DashScope, Fireworks, Hugging Face, Ollama Cloud, and more
- **Local:** Ollama, vLLM, SGLang, llama.cpp `llama-server` ("CPU and Metal inference"), LM Studio (≥0.3.6), and any OpenAI-compatible endpoint, including a LiteLLM proxy

Hard requirements, quoted: "Hermes Agent requires at least **64,000 tokens** of context for agent use with tools. Smaller windows are rejected at startup." Tool calling must be enabled on the server: llama.cpp needs `--jinja`; vLLM needs `--enable-auto-tool-choice` and `--tool-call-parser`. Example models on that page: Claude Sonnet 4.6, Nemotron 3.5 Super VL, qwen2.5-coder:32b, Llama-3.1-70B-Instruct, gpt-oss:120b.

The architecture page says provider resolution supports "three API modes (chat completions, Codex responses, Anthropic messages), 18+ providers, OAuth, and credential pools", and that Anthropic prompt-cache breakpoints are added in `prompt_caching.py`.

## Architecture (high level)

Source: [Developer guide, Architecture](https://hermes-agent.nousresearch.com/docs/developer-guide/architecture), read through WebFetch's summary, and the repo root listing.

- **One core engine.** "One AIAgent class serves CLI, gateway, ACP, batch, and API server." `run_agent.py` is a facade over `agent/conversation_loop.py` and `agent/turn_*.py`, which form "the synchronous orchestration engine". It does provider selection, prompt building, tool execution, retries/fallback, context compression and persistence.
- **Prompt.** `prompt_builder.py` and `system_prompt.py` build a tiered system prompt. The design principle is that the system prompt should stay stable mid-conversation, for caching. `context_compressor.py` summarises middle turns.
- **Tools.** `tools/registry.py` is the registry ("Each tool file self-registers at import time"). `model_tools.py` does discovery and dispatch. `toolsets.py` has ~28 toolsets; the README says "40+ tools". There is an MCP client (stdio and HTTP), and Python scripts can call tools over RPC ("zero-context-cost turns").
- **Gateway.** A long-lived process with platform adapters, allowlists/DM pairing, slash commands, hooks and the cron tick. Since v0.23.0, "each machine runs one gateway that owns every session, and every surface attaches to it as a client" ([v0.23.0](https://github.com/NousResearch/hermes-agent/releases/tag/v0.23.0)).
- **Persistence.** `persistence/` holds storage contracts with a SQLite backend; "FTS5 search", and it tracks session lineage across compressions.
- **Plugins.** Loaded from `~/.hermes/plugins/`, `.hermes/plugins/` and pip entry points. They can register tools, hooks and CLI commands. Memory-provider and context-engine plugins are "single-select — only one of each can be active at a time". Memory providers live in `plugins/memory/`, run by `agent/memory_manager.py` against the ABC in `agent/memory_provider.py`. The plugin catalog has "over 1,050 entries" as of v0.23.0.
- **Skills.** Bundled skills are in `skills/` and optional ones in `optional-skills/`. `agent/skill_commands.py` and `hermes_cli/skills_hub.py` handle them. The format follows agentskills.io. *(The mechanism belongs to the skills subtopic.)*
- **Cron.** Jobs are stored as JSON and run as fresh agent sessions with skills injected; the page calls them "First-class agent tasks (not shell tasks)".
- **Subagents and delegation.** Isolated subagents for parallel work; delegation became pluggable in v0.23.0.
- **Research tooling.** `batch_runner.py` and `trajectory_compressor.py` do "Batch trajectory generation, trajectory compression for training the next generation of tool-calling models". This matters because Nous is a model lab: the agent also produces training data.
- **Security.** Command approval, DM pairing, container isolation, and filesystem checkpoints with `/rollback` ([README docs table](https://github.com/NousResearch/hermes-agent/blob/main/README.md), [v0.2.0](https://github.com/NousResearch/hermes-agent/releases/tag/v2026.3.12)).

The repo root also has `AGENTS.md` and `SOUL.md` (persona), and top-level modules such as `hermes_state*.py`, `session_retention.py`, `session_rescue.py`, `acp_adapter/`, `tui_gateway/`, `ui-tui/`, `web/`, `evals/` and `plugin-catalog/`.
