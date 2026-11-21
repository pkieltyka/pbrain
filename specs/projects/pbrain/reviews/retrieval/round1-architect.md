# Retrieval, context and tags: round 1, @architect

Written without reading the other round-1 files or the operator's notes. Based on the functional spec at sha256 5372ff08 and the knowledge-memory-landscape research.

## Recommendation

Keep search lexical, strict and small, as the spec has it. What decides whether an agent gets the right thing in few words is mostly what was written, and how, not a cleverer ranker. So I would spend v1 effort on five small additions: a declared tag vocabulary, help on an empty result, two write-time notices, a rule that keeps documents from crowding out memory, and a measured evaluation that tells us when lexical search stops being enough. Semantic search stays out of v1.

The research supports this. Every system surveyed primes with a small index and pulls bodies on demand. Unfiltered memory made coding agents worse (CTIM-Rover), while small curated items helped. qmd's model-free search is the same family as ours, and its semantic path costs about 2 GB of models or sends text off the machine, which breaks the spec's "local" rule.

## How search works today, and where it will hurt

The spec's search matches every query word in the title, tags or body, with no stemming and no fallback. It orders by where the words matched (title, then title or tags, then anywhere), then by age band, then by standing, and puts a decision in force first. It returns five results in 2,000 characters, and `get` fetches one body.

That is a sound contract for context. Three weaknesses will show in practice:

1. **Word mismatch.** "caching" does not find "cache", and "auth" does not find "authentication". A four-word query often returns nothing. The spec's answer is "try fewer words", but the agent cannot tell which word failed.
2. **Long documents match everything.** A 40-page transcript contains most common project words once. In the "anywhere" class it ranks level with a note that is about the topic, and a recent transcript ranks above an older learning.
3. **Fresh noise outranks older knowledge.** Age band comes before standing, so last week's provisional observations sit above a verified learning from two months ago in the same match class. That is the owner's decided order; it needs watching, not changing yet.

## Recommendations by topic

**Empty results.** When nothing matches, the closing line says how many current items match each word alone, for example `0 results. cache: 12, eviction: 0.` It costs one line and tells the agent which word to drop or widen with a prefix (`evict*`). No near-miss results are returned, so the "no fallback" rule holds.

**Tags and a vocabulary.** Tags exist in the spec: free lower-case words, a `--tag` filter, and a search field. What is missing is agreement on which tags exist, so agents will invent `auth`, `authn` and `login` for one thing. I recommend the owner's idea, kept small:

- An optional `[tags]` table in `pbrain.toml`: each tag with a one-line description. It names areas of the project (`auth`, `billing`, `ci`), never kinds or statuses, and stays short, around 30 at most.
- `pbrain tags` prints the vocabulary with descriptions and item counts. That is a map of the brain an agent can read in a few hundred characters, and it is what makes `prime --tag` and `search --tag` usable.
- An undeclared tag is not refused: `add` prints a notice naming the declared tags, and `check` warns. That fits "record, don't enforce". With no `[tags]` table, tags stay free as today.

Aliases (a tag that also matches its synonyms in a query) would attack word mismatch directly, but I would wait for the evaluation.

**Documents and memory.** These are two tiers and should behave differently. Memory is the first-party kinds: short, written on purpose, one claim each. Documents are `source` items: long, imported, reference. Three rules:

- Within a match class, first-party items rank before `source` items. This is one line in the order and needs the owner's decision.
- The skill tells an agent that imports a document to record the few things that matter from it as items of its own, each naming the document in `relates`. Memory items become the index into documents.
- Long documents are read by part, not whole: the result's snippet position already lets `get --offset` start there.

**Write-side hygiene.** Three cheap things:

- The skill asks for one claim per item and a title that states it with the component's name ("Cache layer uses Redis", not "Cache notes"). A good title answers the question from the result line, with no `get`.
- `add` prints a notice when current items already have every word of the new title, naming up to three, so the agent replaces or relates instead of duplicating.
- `add` prints a notice when a first-party body is long, say over 2,000 characters: it is probably a document.

Upkeep needs no new command: `list --status provisional --until 90d` is the review list, and `delete` or `verify` acts on it. Who reviews, and how often, is the consumer's rule.

**Priming as the brain grows.** `prime` lists decisions newest first inside 2,000 characters, so at a few hundred decisions the old, fundamental ones fall off. Scoping by tag is the fix, which is another reason for a vocabulary.

**Evaluation.** The acceptance scenarios test behaviour, not usefulness. I recommend a golden set built from the first weeks of real factory use: 30 to 50 task-shaped queries, each with the items that should come back. Measure hits in the first five results, results judged irrelevant, empty results, and characters returned. Agree in advance what result triggers stemming, a term-frequency tie-break or semantic search. It is a test fixture, not a pbrain feature, so "no usage tracking" stands.

## Questions that matter in practice

1. When does an agent search, and with what words: the task's title, or terms it picks?
2. Who prunes? Without a reviewer of provisional items, noise wins.
3. Is a decision's title enough to act on, or will agents `get` every result?
4. How many items do we expect in a year: hundreds or tens of thousands? It decides scan versus index.
5. Do agents need semantic recall, or do good titles and tags cover it? Only the evaluation can say.

## Where each thing belongs

- **Functional spec, now:** the tag vocabulary, `pbrain tags` and the `check` warning; per-word counts on an empty result; the two `add` notices; first-party before `source`; the skill additions.
- **Architecture:** scanning versus an index and which one, the tokenizer, how an index is kept fresh, the speed targets, and the evaluation harness with its golden set.
- **Later:** tag aliases, stemming, a term-frequency tie-break, semantic search through qmd, usage signals, a one-line summary on copied documents, and strict tag enforcement.
