# What else to learn from qmd, and other kinds of search: the group's answer

For the owner. Written by @architect from four independent round-1 papers (`round1-architect.md`, `round1-architect-2.md`, `round1-lead-dev.md`, `round1-lead-dev-2.md`) and the round-2 exchange. Status: final, with three sign-offs.

## Recommendation

**Keep search as approved: exact word matching first, then, on request, results found by meaning. Add no other kind of search to v1.** All four of us reached that independently.

**BM25 is the one method worth a serious look, and not yet.** It finds something when a reworded question finds nothing, with no model. It also answers questions the brain cannot answer, which is the failure your "an empty result must stay possible" rule exists to prevent. So we put it in the evaluation fixture now and ship it only if it earns a place there, as a mode an agent asks for, never as an automatic fallback.

**What qmd had to teach us is mostly already in the design:** passages cut along the document's structure, vectors reused when text has not changed, one result per item however long it is, filters applied before any nearest-neighbour step, and a labelled set of questions to measure with. Its query-expansion model and its reranker stay out.

The answer is the same whether the runtime is bge-small on hugot or EmbeddingGemma 2 on goinfer, and whether vectors live in flat files or in sqlite-vec.

## What the numbers say

Our 20 reworded questions over the 50 sections of the functional spec, rescored on 2026-11-21 by @lead-dev-2 and, separately, by @architect-2, with the same results where they overlap. @lead-dev-2 added 5 questions the brain cannot answer (a cake recipe, Jupiter's moons and the like).

| Method | Right section first | In the top three | Answered an unanswerable question |
|---|---|---|---|
| All words (today's rule) | 0 | 0 | 0 of 5 |
| Any word, ranked by how many of the question's words are present | 5 | 8 | 4 of 5 |
| Any word, ranked by how rare the present words are | 3 | 10 | 4 of 5 |
| Any word, ranked by BM25 | 5 | 11 | 4 of 5 |
| BM25 with stemming | 6 | 9 | |
| BM25 with headings weighted three times | 6 | 11 | |
| bge-small alone | 13 | 17 | best score 0.46 to 0.53, against 0.62 to 0.77 on the 20: a cut-off has room |
| **The approved rule**: all words first, then meaning (no cut-off applied) | 12 | 17 | |
| BM25 and bge-small fused by rank (RRF) | 11 | 16 | |

Four things follow:

- **Accepting any word is what loses abstention, whatever the ranking.** All three any-word rows answered 4 of the 5 unanswerable questions. Dropping the all-words rule explains the gain in first place; BM25's weighting is worth three more questions in the top three than a plain count, and none in first place. @lead-dev-2 and @lead-dev each ran this control and got the same figures.
- **On these 20 questions BM25 found nothing the model missed.** Every one of the 11 questions it got into its top three was already among the model's 17. That is a statement about this sample, not about BM25 in general.
- **On this sample, fusion did worse than the approved rule** (11 and 16 against 12 and 17), and worse than the model alone.
- **Stemming and heading weights moved one question each way.** Stemming rescued 1 of the 19 empty searches.

Read these with care. They are small numbers from one author's whole-sentence questions about one document, and they say little about an agent typing two identifiers. They rank all 50 sections without a cut-off and without a page: they are not what an agent would see in 2,000 characters, which is what the fixture must measure. The five unanswerable questions are an illustration, far too few to set or test a cut-off.

## Method by method

| Method | Gain | Cost | Verdict |
|---|---|---|---|
| **BM25, any-word search** | Finds something without a model: 5 first and 11 in the top three, against nothing. Cheap to compute: the scan already reads every body. | Answered 4 of 5 unanswerable questions. Its scores cannot be compared between queries, so a floor is hard to set. Weak hits could fill a 2,000-character page. | **Not in v1. Later, only if the fixture shows agents without a model missing often**, and then as an explicit mode with marked results after the exact ones. Never an automatic fallback: the per-clause counts already turn an empty result into a retry with words that exist. Not a third signal in hybrid. |
| **Fusion (RRF)** | Combines lists whose scores cannot be compared. | Scored below the approved rule here. It would reorder exact matches: lexical results would no longer be the same with and without `--semantic`, and a decision in force could be pushed off the first page by a near neighbour. Its words would still match; the agent would just not see it. Fusion also cannot abstain. | **No.** Lexical first stays. |
| **Query expansion** | Bridges project shorthand and prose. | qmd does it with a 1.7 billion-parameter generative model; rule 5 excludes one. The calling agent is a language model already. | **Generative expansion: never inside pbrain.** Your A5 holds. |
| **Reranker** | Could reorder a page. | A second model and seconds of CPU against a 1 second target. Our misses are about finding and abstaining, not order: the model already put 17 of 20 in the top three. qmd publishes no measurement that its reranker pays on project notes. | **Not in v1.** Later only if the fixture shows the right item on page two. A5 holds. |
| **Stemming** | Rescued 1 of 19. | Wrong for identifiers and other languages. | **No.** A prefix (`cach*`) already does it on request. |
| **Typo tolerance** | Small: agents rarely mistype. | Works against exact ids, flags and error strings. | **Never as matching.** |
| **Phrases and proximity** | Phrases exist. Clauses close together may beat clauses scattered through a long transcript. | Changes the decided order. | Phrases: v1. Proximity: **later**, a question for the ranking pilot. |
| **Field weights** | Our match classes (title, then title or tags, then body) are already the strictest field weighting. A heading weight moved one question. | Copied weights can bury body evidence. | **Keep the classes.** Headings as a tie-break: later, same pilot. |
| **Chunking** | qmd: about 900 tokens with 15% overlap. Ours: about 1,400 code points (350 to 400 tokens), no overlap. The one study we have put 200 to 400 tokens without overlap at or near the top. | 900 tokens would be cut by the interim model's 512-token input; overlap costs about 18% more vectors. | **Keep ours.** Try overlap only on misses caused by a cut. |
| **Contextual prefixes** | We put the item's title and section titles ahead of every passage, with no model. | A model-written context needs a generative model per passage. | Ours: v1. Model-written: never by pbrain. |
| **Graph signals** | Related lines already show what a result names and what names it. | Following links spends the budget on items nobody asked for; popularity boosts favour heavily linked items and could let private links change public order. | **Shown, never followed.** |
| **Recency** | Orders exact matches, in bands. It should not order results found by meaning, where the score is the evidence. | | **As approved.** |
| **Outline first** | In v1: `get --outline`, and a passage names its section. | | **As approved.** |

## From qmd: what we did not take, and why

- **SQLite's full-text engine as our lexical search.** It splits words by other rules than ours and would be a second source of answers; the scan is fast enough.
- **Ids derived from content**, which change when the text does.
- **Several results for one long document.**
- **Indexing whatever a pattern matches**, ignored files included: qmd did that in our earlier test.
- **Remote embedding by default**, and **scores presented as confidence**.

## If the runtime is goinfer and the store is sqlite-vec

Nothing above changes. Which library runs the model and where vectors are kept do not decide what may match or how results are ordered.

- **sqlite-vec** scores vectors; it does not provide BM25. SQLite's own full-text engine would, and our verdict on BM25 would be the same, for abstention. In our test flat files scored 50,000 vectors in 39 ms against 57 to 60 in sqlite-vec, with no new dependency.
- **goinfer with EmbeddingGemma 2** means a slower start and a first index of about 95 minutes. That makes a model-free option more valuable to measure, and expansion and reranking less affordable.

## Proposed for the owner

None of these changes how results are matched or ordered.

1. **Widen the evaluation fixture** (all four agree; it changes the test plan, not the product). Add as comparison arms: any-word matching ranked by the number of clauses matched and any-word BM25, scored on the same candidates so that the ranking alone is compared; BM25 with stemming and with heading weight, and rank fusion, beside strict search, strict search with one guided retry, and the approved hybrid. Add question kinds: two- and three-word agent-style queries, exact identifiers, superseded decisions, questions answered by one section of a long source, questions the brain cannot answer, and questions only a personal item answers. Every arm reads the same items, view and filters and gets the same character budget. Judge each on its rendered page: right ids, right abstentions, characters, calls, time, and that the shared view never changes when personal items are added.
2. **Pointing an agent at `--semantic` after an empty result: leave it to the skill in v1.** The skill already teaches that retry, and a plain search must stay independent of the model and the index, inside its 300 ms. One option remains yours to choose, now or after the fixture: when a search without `--semantic` finds nothing and `pbrain.toml` names a model, the closing line adds `try --semantic`. It would depend only on that setting, which the command has read anyway, never on whether an index exists, and it promises nothing about readiness. It costs about 17 characters on an empty page and would be a small change to spec 7.1. The group does not ask for it in v1; none of us objects if you want it.
3. **One skill line** (from @architect; a small v1 change to spec 12): when a word's ending may vary, search with a prefix, `cach*`.
4. **Questions for the ranking pilot, later:** an explicit any-word mode; proximity; headings as a tie-break; a hint beside a zero count naming a word that exists (`caching 0 (cache 12)`).

All four of us support item 3.

## Sign-off

- @architect: drafted. On proposal 2 I would have shipped the hint in v1; I accept the group's position.
- @architect-2: Agree with the recommendation. Proposal 2: oppose for v1; the skill already teaches the retry, and the fixture can measure whether agents miss it. Proposal 3: support within the existing skill word limit. Please add the measured any-clause baseline (5 first, 8 top-three, results on 4 of 5 unanswerables), and scope “BM25 adds nothing” to unique top-three successes on these 20 questions. Fusion can displace a decision from the first page; it cannot change whether its words matched.
- @lead-dev: Agree with the recommendation and fixture. Proposal 2: oppose adding the CLI hint in v1 and keep the retry in the skill. Proposal 3: support within the existing word cap. I agree that qmd-style generative expansion is outside pbrain under rule 5. Scope that statement to generative expansion so it leaves room for a separate, evidence-based decision on model-free aliases. Scope the BM25 finding to unique top-three successes on this sample. Hold corpus, scope and budget fixed across the fixture, and hold OR candidates identical specifically when comparing plain coverage with BM25.
- @lead-dev-2: **sign off** (revision 0a10beb3). Proposal 2: I accept the group position, skill guidance only in v1. The static `pbrain.toml`-only hint is acceptable to me as an owner option, but I don't ask for it; I withdrew my round-1 index-readiness version. Proposal 3: **support**: the core skill is at 358 of its 400 words, so one line fits. The evidence table, with the any-word count row, matches my rescoring.
