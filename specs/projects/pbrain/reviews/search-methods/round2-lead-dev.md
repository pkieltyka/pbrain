# BM25 in v1 · @lead-dev

**Ship `--bm25` as an opt-in lexical ranker.** It improves ordering within the existing exact-match policy, and can append a clearly marked partial-match tail when the full exact list is empty. With both flags, keep exact results first, thresholded semantic results second and qualified partial matches last. SQLite stores the vectors. Go scores the live lexical scan.

## Where it sits

| Placement | Expected effect and what the agent sees |
|---|---|
| **Rank existing exact matches** | BM25 rewards rarer terms without widening eligibility. Useful when short, exact queries return many items. It cannot recover the 19 empty strict searches in the rewording sample. Put it after match class, recency band and standing, before newest-created and ID tie-breaks. Choose the promoted decision using the existing ordering and keep it first. The agent sees the normal exact rows and snippets. |
| **Append marked partial matches** | Recovers some empty exact searches locally. Offer it only when the complete exact list for the requested view is empty, independently of pagination. In hybrid, semantic hits get the scarce page space first. Deduplicate by item and append whole rows marked `partial` with compact coverage such as `2/3 clauses`. Use the same character cap, continuation and snippet machinery. |
| **Use BM25 as a third fusion signal** | Agreement across methods might improve some near ties, but it can also move a decision or good literal hit off page one. The offline RRF probe gave 11 first and 16 top-three hits, versus 12 and 17 for the approved ordering. Keep BM25's relevance scores inside its lexical tier. [Prior joint evidence](./joint.md#what-the-numbers-say) |

## Abstention and release

BM25 ranks eligible candidates. Its raw scale varies with query length, word rarity and corpus, so avoid a universal raw-score floor. A label alone still produces a false hit. Start the fixture with at least two distinct matching clauses and at least two-thirds coverage, rounding up. Require each quoted phrase and explicit prefix to match with its existing semantics. A two-clause query therefore still needs both. The cutoff remains the semantic leg's own. If every tier declines, print the existing empty response and clause diagnostics.

I replayed the saved 20 answerable and five unanswerable questions using the same section corpus and BM25 parameters. With all query words retained, the two-thirds gate gave two first-place and three top-three successes and zero nonempty result lists on the five unanswerables. A half-coverage gate with the same two-word minimum got four and seven but returned hits on three unanswerables. These are unbudgeted, standalone rankings on a tiny, one-author sample. Use the wider fixture to settle the policy. Check real first pages under equal budgets, include hard negative questions about topics the brain does cover, and require abstention there too. Ship exact BM25 ranking in v1 even if widening cannot pass the held-out gate. [Saved corpus and test design](../../research/semantic-search-and-index/embedding-models-and-go-runtime/cpu-measurements.md#a-small-retrieval-check-on-pbrains-own-text)

## Engine and invariants

Compute BM25 in Go over the existing parser and scan. With the flag set, gather clause occurrence counts, candidate document frequencies and document lengths in one bounded pass over the eligible view. Use one fixed formula, fixed accumulation order and deterministic created, folded-ID and path ties. Use title and tag match classes as they are. Plain search follows the same 300 ms target and reads no database. Benchmark opt-in BM25 on the same 5,000 items, including the cost of reading full bodies instead of stopping early.

FTS5 is available in the SQLite build and measured quick once indexed, but its tokenizer differs from pbrain's fold. It would require a second, fresh lexical index and careful view-scoped statistics. That would change more than how scores are computed. Keep FTS5 as a possible accelerator only if the live scan later misses the target and a parity test can prove equivalent matching and ordering. [Engine probes](../../research/semantic-search-and-index/qmd-and-go-index-engine/go-index-options-and-probes.md)

Preserve lifecycle, view, kind, tags, status and time eligibility before scoring. Replaced and outside-window diagnostics still count exact lexical matches only. Keep one stable order before slicing by offset. Extra personal files cannot change shared ranking, scores, output counts or paging.
