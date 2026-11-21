# BM25 in v1, round 2: @lead-dev-2

Written independently, before reading the other round-2 papers. The owner wants keyword, BM25 and semantic search together, and the operator asked how, not whether. New data: the same 20 questions and 5 unanswerable ones, plus shorter, agent-style queries made by keeping the 2 or 3 rarest words of each question. The agent-style queries are a proxy built by one author: a direction, not a measurement.

## What the data says about placement and guards

| Ranking | 20 whole questions (first, top 3) | 3-word queries | 2-word queries | Unanswerable answered |
|---|---|---|---|---|
| All words (today) | 0, 0 | 2, 2 | 3, 3 | 0 of 5 |
| BM25 any word, no guard | 5, 11 | 5, 9 | 4, 10 | 4 of 5 |
| BM25 tail, guard G25 (below) | 2, 5 | 3, 6 | 1, 2 | **0 of 5** at every length |
| BM25 tail, guard G50 | 4, 7 | 4, 7 | 1, 2 | 2 of 5 (whole questions) |
| bge-small (semantic) | 13, 17 | | | best score 0.46 to 0.53 against 0.62 to 0.77 |
| RRF of BM25 and bge | 11, 16 | | | |

**The guard.** A clause is *informative* when at most a quarter (G25) or half (G50) of the eligible items contain it. pbrain has no stop-word list and should not get one, so this is how "the", "how" and "have" stop counting without one. An item may enter the BM25 tail only if it matches at least two informative clauses, and at least half of the query's informative clauses. G25 kept every unanswerable question empty, also as 2- and 3-word queries. G50 let two whole-sentence ones through. Short queries barely change: with two informative words, the guard asks for both, which is today's all-words rule.

## Recommendations

**1. Placement: exact matches unchanged, then a marked "partial" tail ranked by BM25.**
- **Exact matches** (every clause) keep today's order: class, recency band, standing, newest, id, path, with the decision-in-force rule.
- BM25 does not reorder exact matches. It would change the owner's decided order, and on our data the exact list is empty when it matters (19 of 20).
- **The tail** holds items that match some clauses but not all and pass the guard. Its order: BM25 descending, then newest, id, path.
- **Marking.** Each tail result is marked `partial 3/4` on its first line, in JSON `"partial": true, "matched": 3, "clauses": 4`. "4" counts every clause the query has; "3" is how many this item matched. The agent sees how weak the match is before spending a `get`.
- **With `--semantic`:** exact, then semantic-only (with its calibrated cut-off), then partial. On our data the model found everything BM25 found and more (17 against 11 in the top 3, 13 against 5 first). Partial hits placed ahead would spend the 5-result page on weaker evidence.
  - Consequence: the partial tail moves when the flag is given.
  - The invariant becomes "the exact list is identical with and without `--semantic`", which still guarantees the verifiable part.
  - An item found both ways is listed once, as semantic.
- **Not a third fused signal.** RRF scored below the approved rule (11 and 16 against 12 and 17), would reorder exact matches, and cannot abstain.

**2. Abstention: guard G25, nothing else.**
- A BM25 score floor does not work: BM25 scores are not comparable across queries.
- A marked tail alone answered 4 of 5 unanswerable questions; marking explains a weak match but does not stop it.
- G25 is a data-driven stop rule plus a share-of-clauses rule. Both are counts over the eligible items, so they are deterministic and need no model.
- The two constants (a quarter; two clauses and half) are starting values for the fixture to tune, and they are versioned with the ranking.
- **The per-clause counts** stay. They print when there is no exact match on the first page, beside any partial results, still as optional parts that fit the budget. An agent that sees `partial 2/4` and `Alone: queue 7, rollback 0` can retry with better words.

**3. Engine: BM25 computed in Go over the scan. Not FTS5.**
- The lexical path reads the files, with no index (spec 9.3: "a lexical answer reflects the files as scanned"). FTS5 would make plain search depend on an index that can be stale, missing or built by another version. It would need per-section separation so shared scores never see personal items. Its `unicode61` tokenizer folds and splits differently from pbrain's (diacritics are removed by default), so its answers would differ from the scan the rest of search uses. SQLite holding vectors for `--semantic` does not change any of this.
- **The Go cost is small.**
  - Document frequency comes free: an item either matched every clause (so it contains all of them) or was tokenised to its end.
  - Body length in tokens, for BM25's length normalisation, needs every eligible body tokenised to its end. Architecture 8.6 measured that at 92 ms with 4 workers for 57 MB, or 54 to 58 ms with 16, inside the 300 ms target.
  - It is only needed when the exact list is short of a page and the query has at least three clauses; otherwise the early stop stays.
  - The code is about a hundred lines in `search`.
- **The corpus** for df, N and the average length is the eligible set of the command's view: the shared view never counts a personal item. With the default parameters (k1 = 1.2, b = 0.75) it is about the same scoring qmd gets from FTS5.

**4. Determinism and the protected rules.**
- **Same output for the same files, query and flags:**
  - scores are summed in float64 in the query's clause order;
  - ties are broken by the existing keys;
  - the constants are fixed in the release.
- **The decision-in-force rule** applies to the exact list only. A partial result is never promoted.
- **The replaced and outside-window counts** keep their meaning: exact matches only.
- **Lifecycle, filters and the time window** apply to the tail exactly as to exact matches, before any scoring. With `--all`, replaced partials follow current ones.
- **Paging** counts exact, partial and semantic results as one list, as the semantic merge already does.

**5. Default: on in plain search.**
- With G25, short queries are untouched: one or two informative words can produce no tail beyond the exact list.
- Long queries that used to return nothing now return marked partial matches, and the 5 unanswerable questions still return nothing.
- That is the owner's "keyword + BM25" without a flag to learn. A flag would also be one more thing the skill must teach in 400 words.
- **If the group prefers opt-in,** the flag should be `--partial`. I'd accept that as a split for the owner.

## What the fixture should check

- **Arms:** the exact list alone, the exact list plus the partial tail under G25 and G50, and the full hybrid order. Each on the same rendered page.
- **Expected effects:**
  - more first-page hits for 3-plus-word and sentence queries without a model;
  - no change for 1- and 2-word queries;
  - no change in abstention;
  - no gain over semantic when a model is on.
- **Stop rule:** if G25 answers any unanswerable question in the fixture, the constants tighten before release.

## Proposed changes, after the joint answer

Spec 7.1 (query semantics, "No fallback", order, the per-clause counts, the markers, the JSON fields), 6.2 (the new markers), 12 (one skill line on partial results), 15 (a scenario for the tail and its abstention; scenario 31's count rule), and architecture 8.2 to 8.5 with `search.md` (`Hit.Partial`, `Matched`, `Clauses`, the guard, and BM25 over the scan). I own `search.md` and would make that change.
