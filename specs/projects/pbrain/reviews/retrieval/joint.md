# Search, context and tags: the group's answer

For the owner. Written by @architect from the four round-1 papers, the operator's notes and the round-2 discussion; @architect-2, @lead-dev and @lead-dev-2 sign off at the end. Status: final, with three sign-offs.

## Recommendation

Keep search as the spec has it: word matching, strict, small results, no model. All four of us reached that independently. For engineering work the words that matter are exact (component names, error strings, flags). The systems the owner named work the same way at the start of a session: Claude Code loads a short index and reads topic files on demand, Letta pins a few files and uses file names and descriptions as signposts, and Hermes freezes a small snapshot. Several others have optional priming or none. ([Retrieval and priming research](../../research/knowledge-memory-landscape/agent-memory-systems/retrieval-priming-budgets-and-eval.md))

On qmd: it has two paths. Its plain search is model-free word matching (BM25), the same family as ours. Its semantic path needs about 2 GB of local models, or a remote service that receives the queries and the text. pbrain keeps its own search so that ids, standing, replacement, the personal boundary and the budget hold; qmd stays an optional outside reader, and the first candidate if the evaluation calls for semantic search. ([qmd research](../../research/knowledge-memory-landscape/qmd/research_summary.md))

What decides whether an agent gets the right thing in few words is mostly what was written and how, and whether a result points at the right passage. So we recommend three small spec changes, none needing a model, and one piece of work that matters more than any of them: a set of real task queries that measures whether pbrain helps, and tells us when word matching stops being enough.

One point still splits the group, two to two: whether the owner's tag vocabulary ships in v1 or waits for evidence. It is decision 1 at the end.

## How search works

An agent takes two or three durable words from its task and runs `pbrain search job queue`. Every word must appear in an item's title, tags or body. Title matches come first, then title-or-tag matches, then the rest; inside each, newer age bands, then standing. If the best match class among current results holds a decision in force, that decision goes first. The agent gets up to five results in 2,000 characters, each with id, kind, standing, date, title, who and a snippet. It runs `get` on the one that matters and cites the id.

It fails in three predictable ways:

1. **Wording.** The task says "background worker" and the decision says "job queue". Search correctly returns nothing, and the agent cannot tell which word failed.
2. **Long documents.** A 40-page transcript contains most project words once, so it matches many queries, and its snippet shows the first matching line, which may be boilerplate.
3. **Fresh noise.** Age band comes before standing, so last week's provisional notes sit above a verified learning from two months ago. That is the owner's decided order. We do not propose changing it, only measuring it.

Agreed by all four:

- **Point at the right passage.** The snippet becomes the short stretch of the body holding the most distinct query clauses, earliest first on a tie. A quoted phrase or a prefix counts as one clause, as written. For a hit whose body is longer than `get`'s default limit, the result also shows the passage's position, in text as well as JSON, so `get <id> --offset N` opens there, and the body's size in compact form (`48k`). Short items carry neither label.
- **Say which word failed, only when nothing matched.** When a search has no result at all, the closing line counts the items matching each clause alone: `0 results. Alone: postgres 12, queue 7, retry 0.` The counts use the same view, lifecycle and filters as the search, so they reveal nothing the caller could not see, and the line is capped. A short page that has results gets no such line: it may only be the budget. The agent knows the synonyms; pbrain tells it which words exist. No near-miss is ever returned.
- **Not now: partial matches or stemming.** The operator's notes suggest ranking items that match some of the words. None of us recommends it for v1: "never the nearest wrong item" is what lets an agent trust an empty result. The evaluation decides both.

## Context discipline

The per-call limits are sound. What is not yet measured is the whole lookup: a search, a retry and two reads are each within limits and still cost the sum.

- **Search first; priming stays optional**, small and once per session.
- **Stop rule in the skill.** Retry an empty search once with other words; read one item at a time; open a long source at the passage, never from the start; stop when the question is answered; say "unknown" when it is not.
- **No `pbrain context` bundle.** The operator's notes suggest one command that returns a budgeted bundle for a task. It is search plus `get`, and a bundle invites dumps. We would add it only if the pilot shows the hand-off between the two calls is the costly part.
- **Measure the whole path**, not each call. See Evaluation.

## Tags and a vocabulary

Tags are already in the spec: free lower-case words, a `--tag` filter on `search`, `list` and `prime`, and a search field of their own. A tagged item is found by its tag word whatever its body says, which makes tags the cheap substitute for semantic search. The weakness is drift: `cache`, `caching` and `redis-cache` for one thing.

All four agree on what tags are for: a few stable names of components or areas (`auth`, `queue`, `ci`), never kind, status or time, always optional. No tag is required, the vocabulary is never injected at session start, and queries are not widened from descriptions.

The owner's idea of declaring them in `pbrain.toml` is where we split, two to two, on timing only.

- **In v1** (@architect, @lead-dev-2, and the operator's notes): an optional table, advisory only.

  ```toml
  [tags]
  cache = "Caching layers, Redis, TTLs"
  ci = "Build and test pipelines"
  ```

  `pbrain tags` prints it with descriptions and item counts: a map of the brain in a few hundred characters, read on demand. An undeclared tag is never refused; `add` and `import` print a notice and `check` warns. With no table, tags stay free as today. The case for now: the owner proposed it, a team that declares nothing pays nothing, and drift starts on day one and is costly to repair, because renaming a tag is a hand edit across files.
- **Guidance first, the table later** (@lead-dev; also @architect-2's preference): start with a handful of tag examples in the project's guide, and add the table when the pilot shows names diverging. A registry is upkeep that must earn its place, and it does not fix wording by itself: a search for "login" still misses an item tagged `auth` unless the agent reads the vocabulary and retries.
- **@architect-2 can support the table in v1** as the owner's choice, on one condition all of us accept: someone in the project keeps the map current. @architect-2 would start with descriptions only and defer the warnings and counts until they pay their way.

Aliases, and the operator's idea of mapping code paths to tags, wait for evidence either way.

## Documents and learnings

These are two tiers and both belong in pbrain. Memory is the first-party kinds: short, written on purpose, one claim each. Documents are `source` items: long, imported or linked, kept as evidence.

- **Extract, don't rely on the document.** When a document holds something reusable, write it as its own item: the symptom a future engineer would search for, when it applies, the outcome, and a citation to the document or passage. A transcript may hold three proposals and no decision; record a decision only when someone decided.
- **Derived items need their own audience check.** A shared pointer to a restricted document gives no permission to publish what it says.
- **Ranking stays as decided, for the pilot.** @architect and @lead-dev-2 proposed one line in the order: inside a match class, first-party items before `source` items. In discussion we agreed to test it first, because being first-party is not what makes a result right. "What workaround did we adopt?" should find the learning; "What event order did the vendor trace record?" should find the document. The fixture runs such paired tasks on the same files at the same reading budget. Until then, `--kind` is the explicit way to ask for memory only. We adopt the change if the pairs show it helps and tasks that need documents do not get worse.

## Write-side hygiene

Most junk enters on write, and no ranking removes it afterwards.

- **Skill lines** (all four): one claim per item; the title states the claim with the component's name ("CI needs Docker 28", not "CI notes"); search before writing; write only what would change a future task and that a future agent could verify.
- **Someone must verify and prune.** Standing only works if someone runs `verify`: otherwise every learning stays provisional and `prime` shows none. The review list needs no new command: `list --status provisional --until 90d`, then `verify`, replace or `delete`. Who does it is the consumer's rule; for factory it needs a name.
- **Not now: notices on `add`.** @architect and the operator's notes proposed a hint when current items already contain every word of a new title, and @architect a hint for an over-long first-party item. The other three prefer pilot evidence before adding heuristics, and @architect accepts waiting.

## Evaluation

The acceptance scenarios prove behaviour. Nothing yet proves usefulness. All four recommend the same thing:

- **A fixture of real tasks.** 30 to 50 requests from factory's own work, written in the words of someone who has not seen the stored titles. Each has the item ids that should come back, or "unknown" where that is the right answer. Include a replaced name, a misleading transcript, a paraphrase, an identifier spelled differently, an answer that exists only in the personal section, and the paired learning-or-document tasks above.
- **A baseline.** The same requests and the same reading allowance, once with the team's current way of looking things up and once with pbrain.
- **Measures.** Useful current ids on the first page; whether the cited answer is right, and whether "unknown" is right when given; total characters, rendered tokens and calls for the whole lookup, retries and reads included; and no personal leak, counts included. Speed is measured apart, on the synthetic 5,000-item brain.
- **Triggers agreed in advance.** Repeated paraphrase misses justify testing a broader mode: stemming, aliases, then semantic search through qmd. Repeated speed misses justify an index. Keeping the two apart tells us whether we need to find different items or the same items faster.

It is a test fixture in the repository, not a pbrain feature, so "no usage tracking" stands.

## Where each thing goes

**Functional spec, now** (small, model-free, agreed by all four):

1. The snippet as the best passage; position and body size shown on long hits.
2. Per-clause counts when a search has no result.
3. Skill lines: query words, retry once, the stop rule, claim titles, search before writing, extract from documents, the audience of derived items, opening long sources at the passage.
4. One complete usage flow: search, read a cited item, meet a newer doubt, stop.
5. If the owner decides yes: the `[tags]` table and `pbrain tags`, with or without the warnings.

**Architecture:** a plain scan in Go as the reference. An index is added only if the speed targets (5,000 items, 50 MB) need it, and must return exactly what the scan returns. Also the tokenizer, a seam for qmd later, and the evaluation fixture with its harness.

**Later, on evidence:** first-party before documents, the `add` notices, tag aliases, stemming, a term-frequency tie-break, semantic search, partial-match ranking, a `pbrain context` bundle, paths mapped to tags, usage signals, and hiding documents unless asked for.

## Decisions for the owner

1. **Tag vocabulary in v1?** The group is split two to two, on timing. @architect and @lead-dev-2 recommend yes: an optional, advisory `[tags]` table with `pbrain tags`, a notice on write and a `check` warning. @lead-dev recommends guidance first and the table on evidence. @architect-2 prefers guidance first and can support the table, descriptions only, if someone keeps it current. If yes, name who keeps factory's map.
2. **Ranking.** Recommended by all four: keep the decided order for the pilot and let the paired tasks decide whether memory should rank before documents.
3. **On an empty result: per-clause counts, or partial matches?** Recommended by all four: counts, only when nothing matched, and keep "no fallback".
4. **Who verifies learnings and prunes provisional items in factory, and how often?** Recommended: the operator, with a review of provisional items older than 90 days. Without this, standing means nothing.
5. **What seeds the evaluation?** Recommended: factory's existing notes and the first weeks of real use. We need from the owner the recurring jobs that should succeed first, and the total reading spend that is acceptable per task.
6. **How big in a year?** Recommended: design for the spec's 5,000 items; hundreds are likely at first.
7. **Approve the spec batch** (items 1 to 4 above)? Recommended: yes.

## Sign-offs

### Sign-off architect-2

Agree with lexical search, a passage jump and the real-task evaluation. Please reflect the round-2 consensus before finalising. Keep the owner’s order for the pilot and test both a learning-needed and a source-needed task. Score passages and count zero-hit diagnostics by distinct query clauses, preserving phrases and prefixes. Bound diagnostics to the active filters and view. Show body length and the jump on long hits. Keep the tag-timing split and name the duplicate notice as a proposal with dissent. The decision-in-force exception applies only in the best current match class.

Revision 2 resolves my design requests. One factual correction for the opening: the research describes several systems with optional or absent priming. Cite Hermes, Letta and Claude Code for the specific practice of keeping signposts in context and reading detail on demand.

### Sign-off @lead-dev-2

Agree, with three corrections where my position changed during the discussion:

1. **Memory before documents (decision 2).** I still prefer it, but I agreed with @architect-2 and @lead-dev that the fixture should test it first, with paired tasks on the same files (one needing the learning, one needing the primary source) at the same reading budget. Please present decision 2 as: keep the owner's order for the pilot; adopt first-party before `source` inside a match class if the paired tasks show it helps and tasks that need documents do not get worse. That is three of us; @architect's "change it now" is the dissent.
2. **Duplicate notice on `add`.** I am not undecided: I agreed it waits for pilot evidence. Move it from "spec now" item 6 to "later, on evidence", with @architect (and the operator's notes) named as wanting it in v1.
3. **Body size.** Show it, like the passage position, only for a body longer than `get`'s default limit, as @architect-2 asked, so short learnings carry no label.

The empty-result-only trigger for the per-word counts keeps my point. In decision 1, add @architect-2's condition: the table needs someone in the project to keep it current.

Revision 2 resolves all three corrections. One fix remains in the Tags section: "@lead-dev-2 has offered to keep factory's map" overstates what I offered. I offered to design and build the table, `pbrain tags` and the `check` warning, not to keep the vocabulary current. Who does that is the owner's choice in decision 1. Please drop that sentence. With that change, I agree.

### Sign-off @lead-dev

Agree with the direction. Keep my tag-table dissent and carry four discussion outcomes into the final.

1. Keep the owner's ranking for the pilot. Compare paired tasks where the first answer needs a learning and the second needs the primary source. Revisit first-party priority on that evidence.
2. Use matching clauses for passage choice and zero-result hints. A phrase or prefix is one clause. Cap hints and apply the same eligibility filters. Show size and a passage jump only on long hits.
3. Let duplicate notices wait. Search before adding and one useful claim per title are enough to pilot.
4. Describe decision promotion as applying within the strongest current match class. Measure total rendered tokens alongside characters and calls.

### Resolution by @architect (revision 2)

The body above was revised after the two sign-offs above and the round-2 messages. Changes made:

- Ranking: the decided order is kept for the pilot and paired tasks decide; @architect withdrew "change it now", so this is the recommendation of all four (@architect-2, @lead-dev-2 correction 1, @lead-dev).
- Passage and counts are scored by distinct query clauses, with phrases and prefixes as written; counts appear only when nothing matched, within the active view and filters, capped (@architect-2, @lead-dev).
- Position and body size are shown only on hits longer than `get`'s default limit (@architect-2, @lead-dev-2 correction 3).
- The `add` notices moved to "later, on evidence", named as @architect's and the operator's proposal (@lead-dev-2 correction 2, @lead-dev).
- Tags: the split is kept and stated as one of timing, with @architect-2's condition that someone keeps the map current, and @lead-dev's guidance-first position (decision 1).
- The decision-in-force exception is stated as applying only in the best match class among current results (@architect-2).
- A baseline run and separate speed measurement were added to Evaluation (@lead-dev); no `pbrain context` command (@lead-dev, @lead-dev-2).

Revision 3, after the confirmations under the sign-offs:

- The claim that every system primes with a small index is replaced by the three named examples, with a link to the research, and a paragraph on qmd's two paths is added (@architect-2, @lead-dev).
- "Up to five results"; rendered tokens are measured with characters and calls (@lead-dev).
- The sentence about who keeps factory's tag map is removed; decision 1 asks the owner (@lead-dev-2).
- The tag split is stated as two to two (@architect-2).

All three sign-offs say agree, and every change they asked for is in the body. Final.
