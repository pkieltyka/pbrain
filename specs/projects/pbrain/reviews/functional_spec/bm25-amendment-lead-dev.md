# Review of the BM25 amendment

Reviewed `079b3d192c3edff7bacba7bfc4ddb861e7a2b4974fc177247decbde42b8ebbbe` against the owner decisions and [joint design](../search-methods/joint-bm25.md). The independent flags, tier order, marked partial rows, exact-only counts and one paging budget follow the decision.

## Blocking

1. **Test the abstention gate on the combined command (§16).** The draft gates the partial tier and each model individually. Calling both flags can fail the agreed condition that every enabled mode abstain on at least 95% of held-out questions. With 30 negatives, one false positive in each mode passes each individual gate (29/30). If they are different questions, using both flags fails (28/30). Require the rendered `--bm25 --semantic` page to pass the same abstention and regression gate with each released model before shipping that pairing. Keep independent gain checks. Hold back the partial flag if the combination cannot pass with thresholds frozen on calibration data.

## Should fix

2. **Keep a quoted word mandatory in the partial tier (§7.1, scenario 38).** The existing parser normalizes a one-word quote to a word. The owner-approved rule makes quoted clauses mandatory. Say whether `"durable" queue retry` can return a partial hit for `queue retry` that lacks `durable`. The agreed rule implies it cannot. Keep quoted provenance through normalization and add that case, with the equivalent unquoted query as a control. A duplicate bare and quoted word still counts once.
3. **Separate strict and release-conditional behavior (§7.1, §12, scenarios 23 and 38).** The “No fallback” bullet says every no-exact-match search is empty, whereas either flag can add rows. Scope it to a plain search. For a build where the partial tier is held back, name the existing `usage` behavior and make the shipped skill teach the flag only when the build offers it. Gate product acceptance for scenario 38 on the flag being shipped, while requiring the scenario to pass on the feature-enabled test build. The 400-word cap still applies.
4. **Test live changes and deterministic inputs precisely (§7.1, scenario 38).** “The output is the same after an `add`” could prohibit the live update the feature is meant to show. Add a newly eligible match and assert it is visible at once, with no model, index or cache access by BM25 alone. Specify the evaluation instant and time zone in the sameness promise because relative windows, recency and review-due markers can change even with identical files.
5. **Clarify what a failed model download retains (§9.4, scenario 35).** “Keeps nothing” conflicts with the EmbeddingGemma 2 design retaining successfully verified source files across interrupted multi-file downloads. Require discarding the failed file and never making an incomplete pack usable. Allow verified files to remain so resuming cannot require another 1.49 GB download.

Verdict: changes required.

## Recheck

Reviewed `6c458ba77a1a289cc55e97ca4340388a5d63514fc54b5217cd4a850dba3b0b02`. Section 16 gates every shipped flag/model pairing. Section 7.1 and scenario 38 preserve single-word quotes through deduplication, distinguish strict search and a held-back flag, and state the time inputs to determinism and the next-search visibility of an add. Section 12 gates the skill hint on flag availability. Section 9.4 and scenario 35 distinguish discarded downloads from verified files kept across retries, and define the offline pack. All findings are closed.

Verdict: approve.
