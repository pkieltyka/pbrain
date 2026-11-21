# Retrieval in a working engineering team

Ship the specified lexical search and judge it by one practical test. Can an engineer who missed the original discussion find the current answer, cite the evidence, and stop reading? Use real tasks to test that before choosing another engine or asking everyone to maintain a tag dictionary.

## What the agent does

At the start of a queue change, try `pbrain search "job queue" --limit 3`. The default searches shared, current items. Every query clause must occur somewhere in the title, tags or body. Phrases and prefixes are available. Title matches lead, then title/tag matches, then other matches. A decision in force in the best current match class leads that class. Recency band, standing and deterministic ties order the rest. An explicit link can show a newer doubt beside the decision. In this example the agent can cite the Postgres decision and investigate the Redis proposal if it changes the task. The default search allows five items and 2,000 characters in total. [Functional spec §§6–7](../../functional_spec.md)

A predictable miss matters. Suppose a task calls the queue a “background worker” and the decision uses neither word. The answer can exist and search can correctly return zero. Retry once with a component name, error string or alternate term. If the footer reports an old match, inspect it with `--all` and follow its current successor. An empty search is evidence about the words tried. After a useful hit, fetch the cited item or passage only if the task needs more evidence. Stop when the question is answered, and report uncertainty when it persists.

## Spend the budget on evidence

Keep session priming optional, small and once per session. Three calls within their individual limits still cost a session the sum of all three. Measure that whole path, including startup, retry and follow-up. Letta’s signposts, Claude Code’s short index and Hermes’s fixed session snapshot suggest the same useful habit. [Retrieval and priming research](../../research/knowledge-memory-landscape/agent-memory-systems/retrieval-priming-budgets-and-eval.md)

The biggest gap for documents is localizing evidence. A 40-page transcript can match “queue” on page one and “rollback” on page forty. The specified snippet centers on the first query word. A good item rank can still give an agent a useless first page. For v1, choose a short body window containing the most *distinct matching clauses*, with earliest offset breaking ties. Make its existing code-point offset available in text as well as JSON for a long body, so `get <id> --offset N` opens there directly. Leave item ranking, output limits and the original text intact. Add a fixture with scattered early mentions and a useful later paragraph.

## What writers do

Keep each source as evidence. When there is a reusable finding, write one learning with the symptom a future engineer might search, the condition where it applies, the observed outcome and a citation to its test, commit or source passage. Search before recording it. Add a decision only when someone actually decided. A transcript may contain three proposals and zero decisions. For restricted materials, the audience of a derived learning deserves its own check. A shared pointer alone gives no permission to publish what the source says.

Keep tags optional. Use a few stable component or domain names across features, incidents and decisions. Kind and standing already have fields. Start with a handful of examples in the project’s existing contributor guidance. If teams repeatedly miss items by using different names, add a small, human-maintained advisory vocabulary with descriptions in tracked `pbrain.toml`, discoverable on demand. Existing and untagged items must keep working. Avoid expanding queries from descriptions or injecting the vocabulary at startup.

## Decisions and evidence

**Functional spec now.** Define passage selection, offset and the retry/stop guidance. Ask the team which past tasks genuinely depended on prior knowledge, which needed an exact passage, and when “unknown” was the correct outcome. Collect queries in the next engineer’s words before showing them the stored titles.

**Architecture and evaluation.** Implement a Go scan as the reference. Compare any derived index against it for identical visibility, ranking, excerpts and omissions. Measure the existing 5,000-item/50 MB latency targets separately from relevance. For each real task, record acceptable current ids or “unknown”, whether a top-three hit supports the answer, original evidence reached, total characters and calls. Include a replaced name, a misleading transcript, a private-only answer and a paraphrase. Compare with the team’s current lookup at the same reading budget. [qmd research](../../research/knowledge-memory-landscape/qmd/research_summary.md)

**Later.** Repeated paraphrase misses justify testing an explicit broader retrieval mode. Repeated latency misses justify indexing. Keeping those decisions separate tells us whether we need to find different items or simply find the same items faster.
