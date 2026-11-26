# The "co-worker" / "learns by doing" experience: positive reports

What users say works. Almost all of this is **anecdote** (self-reported, no control); counts are flagged where given. The phrase "co-worker" itself did not turn up in any Hermes source I searched; the project's tagline is "The agent that grows with you" ([homepage](https://hermes-agent.nousresearch.com/)).

## Curated by the project

Nous maintains a [User Stories page](https://hermes-agent.nousresearch.com/docs/user-stories) linking X, Reddit, Discord and blog posts. It is selection-biased (the vendor chose them) but links originals. Relevant entries (as summarised by WebFetch; I could not open the Reddit/X originals — reddit.com is blocked to this fetcher):

- **Self-written skills noticed and useful.** @elvissun (X, Apr 2026) watched it create an `extract-social-testimonial` skill unprompted and found it useful. @techNmak (X, Apr 2026): after five rounds of a code-review workflow the agent "had internalized my preferences", including which files to check first.
- **Feedback becomes rules.** @scotty529 (X, Jul 2026): corrections become "a dated policy rule that every future session must read". @HoLiPrompt (X, Jul 2026): an orchestrator grades worker output; repeated tasks become skills that keep being updated.
- **Nightly consolidation built by users.** u/HolmeBengt (Reddit, Jun 2026): 28 cron jobs, 30+ skills; a 3 AM "Dreaming" job reads the day's conversations and writes tomorrow's context. u/salt555 (Discord): a skill-audit skill on cron that test-runs and improves skills in a sandbox.
- **Co-worker-like delegation.** u/Physical-Economist27 (Reddit, Oct 2026): solo IT for a 165-person company; ~80% of daily work runs through Hermes, ~30% solved without him, ~50% needs only a yes/no. @ThomasBekkers (X, Jul 2026): a Slack IT-support agent solves ~90% of issues. (Percentages self-reported.)
- **Users replacing the built-in memory.** @lauratom (Discord): "After 3 failed attempts (149 hours of work... for real)", a ~22k-line memory kernel "where the LLM never writes to disk" with "decay, promotion, and supersession"; u/humanth-shashani built a temporal knowledge-graph memory. These are positive about Hermes as a platform but are also evidence the stock memory was not enough for heavy users.

The same page also lists problem stories, e.g. "Hermes was not broken. The config was default." (@alphabatcher) and @jezza2463: "I tried using Kimi 2.5 just like I would a Sonnet 4.6 but it messed all sorts of things up" — the loop's quality depends on the model.

## Independent write-ups

- **Axis Intelligence review** (2026-05-28, v0.10.0, three weeks, 5 workflow types, Claude/Qwen-72B/Mistral-7B): 8.2/10; "the compounding effect is real and measurable" for repeated task types; self-created skills 4 → 11 → 17 over 21 days; downsides incl. "Skill transfer is domain-bounded", reliable local models need 32B+ ([source](https://axis-intelligence.com/?p=30521)). The "measurable" part is skill counts and less prompting, not a task metric.
- **7 days, one task** (DEV, 2026-05-16): the daily news-digest skill grew from 12 to 62 lines and gained filtering, scoring and dedup without the author touching it. The author also lists "Stale preferences persist in memory and need manual correction" and "Skills can occasionally degrade rather than improve" ([source](https://dev.to/sreejit_/i-ran-hermes-agent-on-the-same-task-for-7-days-the-skill-file-on-day-7-looked-nothing-like-day-1-2oa8)). Partly AI-drafted by the author's own admission.
- **Tracker praise from heavy users**, usually as preamble to a bug: "the most capable CLI AI agent I've used" ([#5563](https://github.com/NousResearch/hermes-agent/issues/5563), 3 weeks, 8+ h/day); "one of Hermes's biggest strengths" ([#6051](https://github.com/NousResearch/hermes-agent/issues/6051)); "One of the system's strongest differentiators" ([#25833](https://github.com/NousResearch/hermes-agent/issues/25833)).
- **Hacker News** (~mid-2026, [thread](https://hn.svelte.dev/item/48586005)): chapel — the agent "know[s] exactly how I like to do my work and that means I have to remind it less"; skeptic segmondy — "nothing new about memory (aka sessions saved), skills (aka prompt in a file)".
- **Curator test** (CodeCut, 2026-07-17): dry-run consolidation correctly proposed merging three overlapping CSV-cleaning skills into one — "consolidation can identify duplicate skill intent, not just identical names" — but consolidation is "off by default" ([source](https://codecut.ai/hermes-autonomous-skills-curator/)).
- **Udemy guide**: "The gain comes from the agent layer, not from changes to the model's weights" and the agent is only as good as the human steering it ([source](https://blog.udemy.com/hermes-agent-guide/)) — framing, not evidence.

## Pattern across positive reports

1. The good experiences are on **repeated, narrow tasks** (daily digest, code review rounds, IT tickets) where a skill gets used again soon.
2. Many of the best setups add **their own discipline** on top: graders, dated rules, nightly consolidation jobs, external memory kernels, skill audits — i.e. users supply the verification the stock loop lacks.
3. Users like that skills are **plain files they can read and edit** (also credited in the self-trust critique). Nobody reports wanting the writes to be invisible.
