# Release history, activity, maturity

Retrieved 2026-11-26 via `gh api repos/NousResearch/hermes-agent` and `/releases`.

## Timeline

| Date | Event | Source |
|---|---|---|
| 2025-07-22/23 | Repo created; first commit "initital commit" by Teknium | [commit 21d80ca](https://github.com/NousResearch/hermes-agent/commit/21d80ca68346dfdb8d3556015a723a9217f8566f) |
| (undated) | v0.1.0, "the initial pre-public foundation" (not a GitHub release) | [v0.2.0 notes](https://github.com/NousResearch/hermes-agent/releases/tag/v2026.3.12) |
| 2026-02-25 | Public launch on nousresearch.com/releases (Product Hunt lists Feb 26) | [nousresearch.com/releases](https://nousresearch.com/releases) |
| 2026-03-12 | **v0.2.0**, first tagged release: "216 merged pull requests from 63 contributors". Messaging gateway, MCP client, 70+ skills plus Skills Hub, ACP server, worktrees, checkpoints | [v2026.3.12](https://github.com/NousResearch/hermes-agent/releases/tag/v2026.3.12) |
| 2026-03-17 to 2026-04-30 | v0.3.0 to v0.12.0, about one minor version a week | releases |
| 2026-05-07 | v0.13.0 "The Tenacity Release" | releases |
| 2026-05-28 | v0.15.0 "The Velocity Release" | releases |
| 2026-06-05 | v0.16.0 "The Surface Release" | releases |
| 2026-07-01 | v0.18.0 "The Judgment Release" | releases |
| 2026-07-20 | v0.19.0 "The Quicksilver Release" | releases |
| 2026-08-03 | v0.20.0 | releases |
| 2026-08-31 | v0.21.0 (press calls it the "Pantheon" release; that name is not in the GitHub title) | [releases](https://github.com/NousResearch/hermes-agent/releases/tag/v2026.8.31), [Digg](https://digg.com/tech/3wdiuxfn) |
| 2026-10-08 | Tag scheme moves from CalVer (`v2026.9.24`) to SemVer (`v0.21.6`) | releases |
| 2026-10-14 | v0.22.0 "The Threshold Release" | releases |
| 2026-11-05 | v0.23.0 "The Hearth Release": one gateway owns every session, browser Webapp, plugin auto-update, catalog of 1,050+ entries, "a leaner built-in skill set" | [v0.23.0](https://github.com/NousResearch/hermes-agent/releases/tag/v0.23.0) |
| 2026-11-24 | **v0.23.4**, latest stable; daily `canary.*` prereleases continue | releases |

So far there are ~56 stable releases and canaries in ~8.5 months of public life. Versions are still pre-1.0.

## Activity (snapshot 2026-11-26)

- Stars **266,364**, forks **60,436**, main language Python. Last push 2026-11-26T21:23Z, the minute I checked.
- ~**77,241** commits on `main`; ~403 contributors listed by the API, which caps how many it shows. The v0.23.0 notes say "666 contributors" in that release cycle alone.
- Issues/PRs from the search API: **20,112 open issues**, **34,530 open PRs**, **23,926 merged PRs**. PR numbers are above #148,000.
- v0.23.0 alone, quoted: "~10,080 commits · ~1,950 merged PRs · ~8,720 files changed · ~725,200 insertions · ~201,400 deletions · ~850 issues closed". That covers three weeks.

## Maturity: what the numbers suggest (inference)

- **Very popular and moving very fast.** An earlier write-up claimed "105K stars in 7 weeks" around v0.11.0 ([Hashnode](https://hashnode.com/posts/hermes-agent-v0110-deep-dive-the-open-source-ai-agent-that-hit-105k-stars-in-7-weeks/69eeafeca924ccdc3652c5d6)). Releases ship weekly, with daily canaries.
- **Churn and backlog risk.** 20k open issues and 34.5k open PRs point to a triage backlog well beyond what maintainers can review. The most likely cause is heavy agent-generated contribution (*inference, not verified*). Architecture changed fundamentally as recently as v0.23.0, which replaced per-surface agents with one gateway. The release notes themselves say that before this change "messages got written twice or went missing between surfaces".
- **Security posture is visibly reactive and maturing.** Exact dependency pins came after a PyPI worm incident (May 2026). The "Verified" plugin tier and spend limits on paid MCP tools arrived in v0.23.0.
- **Backing.** It is backed by a funded company (reported $1.5B valuation) with a commercial Portal and hosting, so it is not a hobby project likely to disappear soon.
- For pbrain-2: treat Hermes as a moving target. Cite specific versions or commits when borrowing designs; docs and thresholds change between releases.
