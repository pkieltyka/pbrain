# Search methods, round 1: @lead-dev-2

Independent view, written before reading the others. Data: our 20 reworded questions over the 50 sections of the functional spec (cpu-measurements.md), rescored on 2026-11-21 with bge-small on hugot (the accepted interim runtime), plus 5 unanswerable questions I added. These are small and one-author: they can show a direction, not settle a margin.

## Evidence

| Method | Right section first | In top 3 | Empty |
|---|---|---|---|
| All words (v1 rule), whole question | 0 | 0 | 19 |
| All words, stop words dropped | 0 | 0 | 19 |
| All words, stop words dropped, light stemming | 1 | 1 | 18 |
| BM25, any word | 5 | 11 | 0 |
| BM25 + stemming | 6 | 9 | 0 |
| BM25 + headings weighted ×3 | 6 | 11 | 0 |
| BM25 + both | 7 | 9 | 0 |
| bge-small cosine alone | 13 | 17 | 0 |
| **Approved rule: all words first, then semantic-only** (no cut-off) | 12 | 17 | 0 |
| RRF of BM25 and bge (k = 60) | 11 | 16 | 0 |
| RRF of stemmed BM25 and bge | 11 | 15 | 0 |
| BM25's top 3 first, then semantic | 5 | 11 | 0 |

**Abstention.** On the 5 unanswerable questions (a cake recipe, Jupiter's moons, a 1998 final, Alpine trails, guitar tuning):
- BM25 returned results for 4 of the 5;
- the all-words rule returned results for none;
- bge's best cosine was 0.46 to 0.53 on them, against 0.62 to 0.77 on the 20 answerable questions, so a calibrated cut-off has room to work.

The script is in my scratch folder (`searchexp.py`).

## Recommendations

**1. BM25 or any-word ranking: not in v1. Later only as an explicit opt-in, never automatic.**
- **Gain.** It finds something where the all-words rule finds nothing (5 and 11 against 0 and 0). But semantic search finds much more (13 and 17), and BM25 adds nothing on top of it.
- **Cost: abstention.** BM25 answered 4 of 5 questions the brain cannot answer, which is the failure the owner's "empty result must stay possible" rule exists to prevent.
- **As a fallback when the all-words search is empty?** No. The approved per-clause counts already turn an empty result into a retry with words that exist, at a cost of a few characters. A fallback would answer exactly the questions that should get nothing.
- **As a third signal in hybrid?** No: see fusion.
- **Later:** if the fixture shows that agents without a model often miss with two or three well-chosen words, a marked `--any` mode (ranked any-word, per-clause counts kept) is cheap. It needs no index: scoring 5,000 bodies is part of the scan that already happens.

**2. Fusion (RRF): no. Keep "lexical first, then semantic-only".**
- RRF scored below semantic alone (11/16 against 13/17) and below the approved rule (12/17).
- It also reorders lexical results, which breaks three approved properties: byte-identical lexical output with and without `--semantic`, the decision-in-force promotion, and a citation an agent can verify by the words it searched.
- The approved rule's one loss (12 against 13) is a lexical hit on the wrong section placed first. That is a verifiable exact match, and the price is fair.

**3. Query expansion and reranking: A5 holds.**
- Expansion needs a generative model (qmd ships a 1.7B one), which spec rule 5 excludes.
- A cross-encoder reranker (qmd's 0.6B, about 2 GB of models in all) is not generative, but our failure is recall and abstention, not the order within a page: semantic alone already put 17 of 20 in the top 3.
- qmd itself publishes no measurement that its reranker pays on project memory.
- So: expansion never; reranker later, and only if the fixture shows first-page misses with the right item on page two.

**4. Stemming, typos, proximity, field weights: none in v1.**
- **Stemming** rescued 1 of 19 empty all-words searches and gave mixed BM25 results (6/9 against 5/11). The spec's "no stemming" stays; the fixture can reopen it.
- **Typo tolerance** works against exact ids, error strings and flags, the words agents are told to search with. Never as a default.
- **Proximity:** phrases already exist, and I found no evidence for more.
- **Field weights:** match classes (title, then tags, then body) are already the strongest field weighting there is. Heading weight moved BM25 by one question (6 against 5), and on the semantic side the heading path is already in every passage's prefix.

**5. Chunking: keep ours.**
- qmd uses 900 tokens with 15% overlap. bge-small takes 512 tokens, so 900 would be cut.
- Chroma's study found 200 to 400 tokens with no overlap at or near the top (long-document-approaches.md).
- Our sections of about 1,400 code points (about 350 to 400 tokens), with no overlap, sit in that range.
- **Contextual prefixes:** we already have the model-free part. Title and heading path go in front of every passage. Anthropic's LLM-written context is out under rule 5.

**6. Graph, recency, outline: as approved.**
- Recency already orders lexical results. It should not order semantic ones, where the score is the evidence.
- Links stay pointers. Following them would spend the budget on items nobody asked for, and I found no controlled evidence that it helps agents (references-and-links.md).
- Outline-first is in v1 (`get --outline`), and passages name their section.

**7. More from qmd.**
- **Take:**
  - content-hash reuse (we have it, in the per-user cache);
  - its labelled-query bench, which is our fixture;
  - its "strong lexical signal" idea, reduced to output. When a search without `--semantic` finds nothing and a usable index exists, the closing line could say `try --semantic` (about 20 characters, only on empty results). That is proposed amendment A below.
- **Don't take:**
  - FTS5 as the lexical engine: its tokenizer is not our fold, it adds a second source of answers, and the scan is fast enough;
  - content-derived ids;
  - indexing what Git ignores, which qmd's smoke test did;
  - several results per long document;
  - remote embeddings;
  - scores shown as if they were probabilities.

## Under the owner's wording ("goinfer and sqlite-vec")

My answer does not change.
- **goinfer** is the EmbeddingGemma 2 base only if the fixture favours that model (the joint runtime note). Whichever runtime is used, the search method questions above are the same.
- **sqlite-vec** is feasible without cgo (modernc bundles it). But flat files scored faster in our test (39 ms against 57 to 60), need no dependency, and filter by eligibility in Go exactly.
- If SQLite came anyway, FTS5 BM25 would be free to add, and point 1 would still say no, for abstention.

## Proposed amendments for the owner

- **A (v1, small).** On an empty lexical result, when the index for the named model is ready, the closing line adds `try --semantic`. This is output only; ranking does not change. Affects spec 7.1, and scenario 31 gets one more case.
- **B (later, evidence-gated).** `--any`: ranked any-word search, marked as such, keeping per-clause counts. Adopt only if the fixture shows lexical-only agents missing often.
- **C (fixture).** Add two-word, agent-style queries beside whole-sentence ones; questions the brain cannot answer (to measure abstention for every method, BM25 included); and questions answered by one section of a long source. Score every row of the table above on it before any of 1 to 4 is reopened.
