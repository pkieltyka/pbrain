# BM25 in v1, round 2: @architect

Written independently, 2026-11-21. Numbers are from @lead-dev-2's script (`searchexp.py`), which I re-ran unchanged: 20 reworded questions, the same questions cut to their two or three rarest words as a stand-in for an agent's query, and 5 questions the brain cannot answer. Small, one author, one document.

## Where BM25 sits

**As the order of a marked any-word tail, after the exact matches.** That is where it adds something: results for a query whose words are not all in one item. I would not use it anywhere else in v1.

| Option | Expected effect on the fixture | What an agent sees | My view |
|---|---|---|---|
| Rank inside the exact-match classes | None on what is found; a different order among items equal in class, recency band and standing. | The same results, reshuffled. | **Only as the last tie-break**, in place of "newest first" within a band, and only if the pilot asks. Not needed to answer the owner. |
| **A marked any-word tail** | Whole questions: from 0 first and 0 in the top three to 4 and 8 with the floor below (5 and 11 with no floor). Three-word queries: from 2 and 2 to 5 and 8. | Exact results first, unchanged. Then results marked `partial 2/3`, saying how many of the query's clauses each holds. | **Yes.** |
| A third signal fused into hybrid | Worse than the approved rule on our sample: 11 and 16 against 12 and 17. | Exact matches reordered; a decision in force can leave the first page. | **No.** |
| The tail beside the semantic results | Nothing on our sample: every BM25 success was also a semantic one. | One more group at the end. | **Yes, last and cheap:** it still broadens a search when the model or index is not ready. |

So with `--semantic` the order is: exact matches; then results found by meaning; then any-word results not already listed.

## What keeps "nothing found" honest

Three things together:

1. **A floor on clauses, not on the score.** A partial result must hold at least two of the query's clauses and at least half of them. A BM25 score cannot be compared between queries; a share of clauses can. On the sample this floor answered 0 of the 5 unanswerable questions (no floor: 4 of 5) and kept 4 first and 8 in the top three of the 20 (a floor of one third: 5 and 10, also 0 of 5). A floor on the share of rare-word weight did much worse (1 and 1): in a reworded question the rare words are exactly the ones missing.
2. **Words that are everywhere do not count towards the floor.** The script drops a fixed English stop list. pbrain has none and should not get one. I propose the corpus's own measure: a clause found in more than a fifth of the items in the view is ignored when the share is counted, and still scores. This is the part I am least sure of; a small brain makes "a fifth" coarse, and the fixture has to set it.
3. **The same gate as semantic search.** The tail ships only if, on the fixture's held-out unanswerable questions, at least 95% return nothing. A two-clause query gets no tail at all under rule 1: both clauses present is an exact match.

Marked, after the exact results, never mixed in: an agent can always tell an exact match from a partial one, and can check a partial one by the count it shows.

## The engine: BM25 in Go, over the scan

Not SQLite's full-text engine, even though SQLite is now in.

- Its word splitting is not ours, so a prefix, a phrase or an accented word could match in one and not the other, and the scan would have to stay as the reference anyway.
- It needs an index of every body kept current. Today a write never touches an index and a hand edit or a pull needs no step; the lexical answer is always the files as they are.
- The numbers it needs are cheap here: document frequencies for the query's clauses over at most 5,000 items, counted in the scan that already reads every body. Tokenising all of them took 54 to 58 ms at 16 workers and 92 ms at 4.
- Shared and personal stay apart by construction: the frequencies are counted over the items of the view, so a personal item cannot move a shared result.

Fixed parameters (k1 1.2, b 0.75), 64-bit arithmetic in a fixed order, ties by date, id and path.

## The rules that must hold

- **Determinism:** same files, view, query and flags give the same output; nothing is stored.
- **The decision in force** is decided among exact matches only, as today.
- **Replaced and outside-window counts** stay counts of exact matches. Partial results count in `more` and `--offset` like semantic ones; the per-clause counts appear only when the whole list is empty.
- **Filters and the view** bound the tail exactly as they bound exact matches.

## Default or behind a flag

**Behind a flag in v1: `--any`, and implied by `--semantic`.** A plain search keeps its meaning (an empty result says the words are nowhere together), its output and its 300 ms. The floor is uncalibrated, and a default-on tail would put that risk on every search. If the fixture shows the floor holding, making it the default is a one-line decision later.

## What I would tell the owner

Yes: keyword, BM25 and semantic together, in that order, each kind marked. BM25 was "only okay" as a replacement for the others; as a model-free third layer behind a floor it is cheap and fills the gap for brains with no model.
