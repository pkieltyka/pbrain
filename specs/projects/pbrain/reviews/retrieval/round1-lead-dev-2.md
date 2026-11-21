# Retrieval, context and tags: round 1, @lead-dev-2

Written independently on 2026-11-20, against the functional spec as committed in `9f9f6c3` ("iteration") and the knowledge-memory research.

## Recommendation

Keep v1 search lexical and deterministic: right for engineering work, where the words that matter (component names, error strings, flags) are exact. The risk to context is not the engine but three gaps: queries that miss on wording, long documents crowding out short learnings, and junk written in the first place. Four small, model-free additions: per-word counts when a search comes back thin, body size in every result, first-party items before outside documents of the same match, and an optional tag vocabulary in `pbrain.toml`. Semantic search and usage signals stay later, decided on an evaluation set we build now.

## 1. How search works, and where it fails in practice

Today (7.1): every word must match in the title, tags or body; no stemming, no synonyms, no fallback. Ordered by match class (title, then title or tags, then body), recency band, standing, date. Default 5 results in 2,000 characters.

Precise, but an agent's first query uses its task's wording, not the item's: "caching" misses "cache"; "postgres queue retry" misses an item without "retry". "Try fewer words" makes the agent guess which word to drop, one call at a time.

**Recommend (spec scope, v1):** when a search returns fewer results than its limit, the closing line counts each word alone: `0 results. Alone: postgres 12, queue 7, retry 0.` One line, still lexical, and it keeps "no fallback". The agent is the query expander: it knows synonyms; pbrain tells it cheaply which words exist.

## 2. Context discipline

A lookup costs up to 8,000 characters (search plus get), fine if the first item is right. Two gaps:

- **Size before reading.** A result does not say how long its body is, so an agent cannot tell a 300-character learning from a 48,000-character transcript before `get`. **Recommend:** each result shows body size (`1.2k chars`; JSON `body_chars`, already in `get`).
- **Reading inside long documents.** JSON already has `snippet_offset`. **Recommend:** text output shows it too (`at 31,200`), and the skill says: for a long source, `get --offset <snippet_offset> --max-chars 1500`, never page from the start.

## 3. Documents vs memory and learnings

The split is real. Memory (decisions, learnings, observations, notes) is short, first-party and written to be found. Documents (`source` items) are long and match almost any word. Under 7.1 a body-only match in last week's transcript outranks a verified learning from two months ago, because recency band comes before standing.

**Recommend (spec scope, v1):** within a match class, first-party items come before `source` items; recency, standing and date then apply as now. A document whose title or tags match still leads. Excluding sources by default with a count (`4 documents also matched: --kind source`) is stronger but hides documents a design question needs. The fixture should decide.

## 4. Tags and a declared vocabulary

Tags exist (front matter, `--tag`, match class 2) and matter more than they look: a tagged item is found by its tag word whatever its body says. They are the cheap substitute for semantic search, and `prime --tag` primes one area. The problem is drift (`cache`, `caching`, `redis-cache`), which makes `--tag` miss.

**Recommend (spec scope, v1), advisory only:**

```toml
[tags]
cache = "Caching layers, Redis, TTLs"
ci = "Build and test pipelines"
```

- `pbrain tags` prints the vocabulary with descriptions and counts, bounded like `list`: the brain's map, read once, like Letta's directory names or Hermes' skill index.
- `add` and `import` print an `unknown_tag` notice for an undeclared tag, and `check` reports it as a warning. Never a refusal: an empty or missing table means free tags, as today.
- Aliases and tag renames: later. A rename is a hand edit across files, which `check` makes findable.

## 5. Write-side hygiene

Most junk enters on write. Model-free levers:

- Skill: titles are claims ("CI needs Docker 28"), not topics ("CI notes"); a learning is a paragraph or two; tag from the vocabulary.
- **Question for the group:** should `add` print up to three current items whose titles share most words with the new title ("similar: <ids>"), as a duplicate warning? It is advisory, so it does not break "no inferred relations", but it is a heuristic.
- Verification is the curation signal. If nobody runs `verify`, every learning stays provisional and `prime` shows none. Who verifies is factory's rule, but the owner should know that standing only works if someone does.

## 6. Evaluation

Public benchmarks score conversational QA. **Recommend:** a fixture of factory's real notes plus synthetic sources, and 30 to 50 task-shaped queries with expected ids. Measure top-5 hits, characters returned and calls to the first hit; tune bands, source order and any later BM25 or semantic option on it. Architecture scope, built with the first reads.

## 7. Scope

- **Functional spec now:** per-word counts on thin results; body size in results; first-party before sources within a class; the `[tags]` vocabulary, `pbrain tags`, `unknown_tag`; skill lines on query strategy, long documents, titles and tags.
- **Architecture:** scan vs index (the 300 ms target at 5,000 items), the tokenizer, in-class tie-breaks (BM25 or not), the evaluation fixture.
- **Later:** stemming or aliases, semantic search through a qmd adapter, usage signals, a local opt-in query log for tuning, near-duplicate detection if the title heuristic is rejected.

## Questions that matter in practice

1. What do agents actually search for, and how often does the first query miss? Only the fixture or a query log answers this.
2. Who curates: who verifies learnings and deletes junk? Without it, standing means nothing.
3. Who owns the tag vocabulary, and does an agent ever add to it?
4. Do long documents crowd out learnings in real results?
5. At 5,000 items, is `prime` still useful, or does it need `--tag` per role?
