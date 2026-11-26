# Which "Hermes"?

Retrieved 2026-11-26.

## Answer

The "Hermes" praised for automatic skill creation and "learns by doing" is **Hermes Agent** by **Nous Research** (repo [NousResearch/hermes-agent](https://github.com/NousResearch/hermes-agent)). It is a different product from Nous's older **Hermes** LLM family (Hermes 2/3/4), with which it shares a brand.

Evidence that the skill-learning claims belong to this project:

- README, in Nous's words: "The self-improving AI agent built by Nous Research. It's the only agent with a built-in learning loop — it creates skills from experience, improves them during use, nudges itself to persist knowledge, searches its own past conversations, and builds a deepening model of who you are across sessions." ([README](https://github.com/NousResearch/hermes-agent/blob/main/README.md))
- `pyproject.toml` description: "The self-improving AI agent — creates skills from experience, improves them during use, and runs anywhere" ([pyproject.toml](https://github.com/NousResearch/hermes-agent/blob/main/pyproject.toml))
- Landing page: "It learns your projects, auto-generates skills, and never forgets how it solved a problem." Tagline "The Agent That Grows With You". ([hermes-agent.nousresearch.com](https://hermes-agent.nousresearch.com/))
- Nous releases page, dated 02/25/26: "An autonomous agent that lives on your server, remembers what it learns, and gets more capable the longer it runs." ([nousresearch.com/releases](https://nousresearch.com/releases))

### The "co-worker" phrase

I found no official Nous page that uses "co-worker" wording; the landing page does not ([hermes-agent.nousresearch.com](https://hermes-agent.nousresearch.com/)). The closest primary-ish source is NVIDIA's **NemoClaw for Hermes Agent** blueprint. Per the search summary it describes Hermes joining shared team environments (PR review, incident triage, launch review), where teammates correct it and it turns repeated guidance into reusable skills ([build.nvidia.com](https://build.nvidia.com/nvidia/nemoclaw-for-hermes-agent.md)). I did not read that page in full. *Inference:* the "co-worker" framing comes from users and partners, not from Nous's own marketing. The Evidence subtopic owns user sentiment.

## Other things called "Hermes" (not the subject)

| Name | What it is | Relation |
|---|---|---|
| Hermes 2 / 3 / 4, DeepHermes, Hermes-4.3-Seed-36B | Nous Research's fine-tuned open-weight LLMs (2023 to Dec 2025) | Same company and brand. Hermes Agent is model-agnostic and does not need them. [nousresearch.com/releases](https://nousresearch.com/releases) |
| Hermes Chat (10/29/24) | Nous chat interface | Earlier Nous product, not the agent ([releases](https://nousresearch.com/releases)) |
| Hermes Desktop | Desktop app (macOS 12+, Windows) for Hermes Agent | Part of Hermes Agent. The README links "Hermes Desktop". Community desktop wrappers also exist, e.g. `fathah/hermes-desktop`, which shows up on tessl.io |
| Hermes JS engine (Meta / React Native) | JavaScript engine | Unrelated, and does not appear in agent discussions *(from general knowledge, not fetched)* |
| Many forks/mirrors (UfukNode/hermes-agent, codeberg/gitee mirrors, "diet-hermes") | Copies of Nous's repo | Show up in search results. Not the canonical project ([search](https://github.com/UfukNode/hermes-agent)) |

## The Mnemosyne "Hermes" host is the same thing

`mnemosyne-oss/mnemosyne` ships `integrations/hermes/` (files: `README.md`, `plugin.yaml`, `pyproject.toml`, `src`, `tests`, `PLATFORM_SUPPORT.md`). Its README is titled "Mnemosyne for Hermes Agent" and says: "Local-first memory provider for Hermes Agent". It ties to Nous's project in four ways:

- It registers through "Hermes' memory provider discovery system (`hermes_agent.memory_providers`)" and is activated with `hermes config set memory.provider mnemosyne` ([integration README](https://github.com/mnemosyne-oss/mnemosyne/blob/main/integrations/hermes/README.md)).
- It pins a "supported reference" of "Hermes `f4ac0bea1ea50bba62886fb0b7e308e4edee8fd3`". That commit exists in NousResearch/hermes-agent (a merge of PR #141508, 2026-10-19) ([commit](https://github.com/NousResearch/hermes-agent/commit/f4ac0bea1ea50bba62886fb0b7e308e4edee8fd3)).
- It cites Hermes internals: "the memory toolset gates both built-in AND provider injection at `agent_init.py:1163-1172`".
- Nous's own plugin marketplace lists it at [hermes-agent.nousresearch.com/docs/plugins/mnemosyne](https://hermes-agent.nousresearch.com/docs/plugins/mnemosyne). That listing shows publisher `mnemosyne-oss` (not Nous), v0.7.7, "requires Hermes >=0.21.4", install `hermes plugins install mnemosyne`, and says it is "a memory provider, loaded only when `memory.provider: mnemosyne` is set".

So Mnemosyne is a **third-party memory-provider plugin for Nous's Hermes Agent**, not a separate "Hermes". More detail is in [mnemosyne-integration.md](./mnemosyne-integration.md).
