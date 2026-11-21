# Search methods — independent view from @architect-2

**Keep the two approved search modes. Put any-word BM25 in the evaluation fixture now. Its strongest case is a model-free retry when someone remembers a few terms imprecisely. The evidence does not yet justify an automatic fallback or a third ranking signal.**

## What BM25 added on our 20 questions

I replayed the original BM25 scoring against the saved 50 sections and compared the *same questions*, one by one, with the saved bge-small/hugot vectors. I also tried equal-weight reciprocal rank fusion (RRF), adding `1/(60 + rank)` for each list containing the section. BM25 contributed only positive-score sections. All 50 sections entered the cosine list. This is an offline comparison without a cutoff, query expansion, qmd bonuses or parameter tuning. [Original experiment](../../research/semantic-search-and-index/embedding-models-and-go-runtime/cpu-measurements.md) · [hugot vectors and method](../../research/semantic-search-and-index/embedding-models-and-go-runtime/spike-bge-small-hugot.md)

| Method | Correct first | Correct in top three |
|---|---:|---:|
| Strict all-words baseline | 0/20 | 0/20 |
| Any-word BM25 | 5/20 | 11/20 |
| bge-small cosine | 13/20 | 17/20 |
| BM25 + cosine, plain RRF | 11/20 | 16/20 |

**All 11 questions BM25 got right in its first three were already among bge's 17.** Fusion rescued one top-three answer and lost two. This says little about an engineer typing two identifiers. All questions were answerable paraphrases of one document, from one author. The test also changed eligibility (AND → OR) *and* ranking at once. Compare simple any-clause coverage against BM25 on identical candidates before attributing the gain to the formula.

## What to take

| Method | Position and trade-off |
|---|---|
| Any-word retrieval | **Trial now, possible later amendment.** Compare strict + one targeted retry, explicit model-free broadening, and the shipped hybrid. BM25 favours rare terms, dampens repetitions and accounts for document length. It needs visible-corpus term frequencies and average lengths. Computing these per scan costs CPU. Persisting them introduces freshness work. Hidden personal items must never change shared ranks, totals or pagination. Keep quoted phrases intact and use existing time/kind/tag filters. If broadening earns a command option, label partial hits and require evidence sufficient to return zero. |
| Fusion | **Keep lexical-first in v1.** It protects a matching decision in force and exact engineering terms. RRF is inexpensive and deterministic but rank one exists even for an unrelated question. It cannot establish relevance or replace the absolute semantic cutoff. The paired results above give no reason to reorder known-good hits. |
| Stemming, typos, phrases, fields | **V1:** quoted phrases, explicit prefixes, title/tag match classes, and best passage. **Later:** test bounded proximity or a heading tie-break on a long runbook with scattered terms. Try stemming or spelling suggestions only on observed misses, preserving literal flags and identifiers. English stemming has language costs. Fuzzy matching short symbols and boosting repeated headings can fill a small page with plausible errors. No evidence here for copying qmd’s numerical field weights. |
| Passages and context | **V1:** keep section-aware spans around 1,400 code points, bounded title and heading ancestry, and offsets back into the stored body. Use `get --outline` when the first long-document hit leaves the location uncertain. qmd's ~900 tokens exceed bge's 512-token input before prefixes. **Trial later:** overlap or boundary-aware splitting on cases where the claim and its qualification straddle a split. At 15% overlap, equal-size windows cost roughly 18% more embeddings in long continuous text. Compare correct *cited passages*, truncated tails and reindex cost; deduplicate to one item before fitting the page. [qmd implementation study](../../research/semantic-search-and-index/qmd-and-go-index-engine/qmd-pipeline-and-tradeoffs.md) |
| Graph and recency | **V1:** use replacement to identify the current decision, explicit related items for support or doubt, and the existing lexical recency bands. **Later:** consider another ordering only if paired current-decision and source-document tasks improve. Popularity boosts favour heavily linked items and can let private links change public order. |
| Expansion and reranking | **Leave both outside v1.** qmd’s expansion and cross-encoder reranking add model loads and inference; no measured gain here against one targeted retry, the 300 ms lexical budget, or the 1 s hybrid budget. Its most useful extra is the repeatable benchmark with switches for each method. Add diagnostics to the fixture output; agents need citations and passage positions on their normal page. [qmd pipeline and costs](../../research/semantic-search-and-index/qmd-and-go-index-engine/qmd-pipeline-and-tradeoffs.md) |

**Owner decision:** approve a benchmark comparison now. Use exact identifiers, agent-selected keywords, paraphrases, superseded decisions, and questions with no answer or only a personal answer. Compare the same rendered budget, correct citations, abstentions, calls, latency and shared-view invariance after adding personal items. If explicit model-free broadening improves that workload, propose it as a separate functional amendment. No production ranking or storage amendment is justified today. The same experiment works with the approved flat vectors/hugot or with SQLite/goinfer if the owner chooses those later. sqlite-vec scores vectors; FTS5 or a Go scorer supplies BM25.
