# Search methods, round 1: @architect

Written independently, 2026-11-21. Evidence: the 20 reworded questions over our own spec ([cpu-measurements.md](../../research/semantic-search-and-index/embedding-models-and-go-runtime/cpu-measurements.md)), the qmd reading ([qmd-pipeline-and-tradeoffs.md](../../research/semantic-search-and-index/qmd-and-go-index-engine/qmd-pipeline-and-tradeoffs.md)), the long-document study and the approved design.

## My answer in one paragraph

Keep the approved shape: exact word matching first, results found by meaning after it, each marked. The one method worth adding is **any-word matching ranked by BM25, as a second marked tail that needs no model**, because most brains will run without a model and that is where a reworded question finds nothing today. I would not ship it on the present evidence. I would make it an arm of the evaluation fixture now, because it also tells us how much of the semantic gain a model is really buying. Everything else in qmd's search path that we have not already taken should stay out.

## Item by item

| Method | Gain | Cost | When |
|---|---|---|---|
| **Any-word search ranked by BM25** | On our 20 questions: right section first for 5 and in the top three for 11, where all-words found nothing for 19. bge-small: 13 and 17. No model, no download, no index: the scan already reads every body, and tokenising all of them took 54 to 58 ms. | It always returns something (0 empty of 20), so it breaks "an empty result means the words are nowhere together" unless it has its own floor, and a BM25 score is not comparable between queries. It must never mix into the exact results. | **Later, decided on the fixture; measured in v1.** Shape if it wins: an explicit flag, results after the exact ones, marked, with a floor on how many clauses must match. |
| BM25 as the order of exact matches | None shown. | Replaces the owner's order (match class, recency, standing). | Never, unless the ranking pilot asks. |
| **Rank fusion (RRF)** | Combines lists whose scores cannot be compared. | Our exact list is ordered by authority and date, not by relevance; fusing lets a near neighbour outrank an exact match or a decision in force, and it cannot abstain. qmd itself skips its expansion when the keyword signal is strong. | Not above exact matches, ever. **Later**, as the way to order the approximate tail if it comes to have two sources (any-word and semantic). |
| **Query expansion** | qmd uses a 1.7 B model. | A generative model, against rule 5. The caller is already a language model: the per-clause counts and one retry are our expansion. | **Never** inside pbrain. |
| **Reranker** | No controlled result found for engineering notes. | A 0.6 B cross-encoder: seconds on a CPU against a 1 second target, a second download. | **Later only on fixture evidence**; I expect never. |
| **Stemming** | Would join `cache`, `caches`, `caching`. | Wrong for identifiers and for other languages; changes what "matches" means. Prefix clauses (`cach*`) already do this on request. | **v1: one skill line** teaching the prefix. Stemming itself: never in exact matching. |
| **Typo tolerance** | Small: agents rarely mistype, and a word counted 0 already shows. | Fuzzy matching returns wrong items. | Never as matching. **Later**, as a hint beside a zero count: `caching 0 (cache 12)`, a word that exists with the same start. |
| **Proximity** | Addresses a known failure: a long transcript holds every word somewhere. Clauses together in one passage are a better match than clauses scattered. | Changes the decided order. Cheap: the passage rule already finds the best stretch. | **Later**, as a question for the ranking pilot. |
| **Field weights; headings as a field** | We already rank by field as strict classes (title, then title or tags, then body). A match in a section heading could be a class between tags and body, now that headings are parsed. | Changes the decided order. | **Later**, same pilot. |
| **Chunk size and overlap** | qmd: about 900 tokens with 15% overlap. Ours: about 1,400 code points, no overlap. The one study we have put 200 to 400 tokens without overlap at or near the top, and 800 with overlap lowest; the interim model takes 512 tokens. | Larger passages point less precisely and get cut; overlap stores more vectors and returns the same text twice. | **Keep ours.** Sizes are tuned on the fixture. |
| **Contextual prefixes** | We already put the title and section titles ahead of each passage, capped. | A model-written context per passage needs a generative model per passage. | Ours: v1. Model-written: **never** by pbrain; an agent-written summary at import, later. |
| **Graph signals** | Related lines already show the items a result names and the newer ones that name it. | Following links automatically spends the budget on unasked items; counting citations needs links parsed in bodies. | Shown, never followed. "Cited by" later. |
| **Recency** | In the order already, as bands. | | Keep; the pilot measures the bands. |
| **Outline first** | In v1: `get --outline` and the section named with a passage. A further step: search inside one long item, returning its matching sections. | A new result shape. | The step: **later**. |

## From qmd: taken, and not to take

Already taken: structure-aware passages, fingerprints of model and preparation, reuse of unchanged content, one result per item whatever its length, eligibility applied before any nearest-neighbour step.

Not to take: the expansion model and hypothetical-document queries; the reranker; remote embedding by default; ids derived from content, which change when the text does; treating a score as confidence; indexing whatever a glob matches, ignored files included.

One qmd habit worth copying in spirit: it does not spend model work when keyword search is already confident. Ours is the skill's rule that `--semantic` follows an empty result, not every search.

## Does this change under "goinfer and sqlite-vec"?

No, and one point gets stronger.

- **sqlite-vec.** SQLite would bring FTS5 and its BM25 for free, which makes it tempting. But FTS5 splits words by other rules than ours, so the scan would have to stay the reference anyway, and any-word BM25 can be computed in that scan in tens of milliseconds. SQLite adds nothing here.
- **goinfer and EmbeddingGemma 2.** A first index of about 95 minutes and a slower start make a model-free tail more valuable, not less, and make expansion and reranking less affordable still.

## Proposed for the owner, if the group agrees

1. Add any-word BM25 as a third arm of the evaluation fixture (strict, strict plus any-word, strict plus semantic), and judge the semantic gain against both.
2. Add the prefix clause to the skill's retry advice.
3. Put three ranking questions to the pilot beside the existing one: proximity, headings as a field, and the zero-count hint.
