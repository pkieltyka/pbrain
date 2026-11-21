# Component review: query and search

Reviewed `627a0febe34f8732fd03190826352e108669dc96b7335b4468000407b0d10d9e` against functional spec `2b9cedaf`, architecture `e2ce1217` and the render contract. The three-set classification table preserves removed-by-window and replaced counts. The snippet sweep implements earliest maximum distinct-clause coverage, and the semantic merge uses the fixed score, passage and readiness seam.

1. **Blocking · §2.4, midnight gaps can include the preceding local day.** Go does not promise to normalize a skipped midnight forwards. On the installed Go toolchain, `time.Date(2018, 11, 4, 0, 0, 0, 0, loc)` for `America/Sao_Paulo` produces Nov 3 at 23:00. For `Pacific/Apia` it similarly backs out of the skipped 2011-12-30 date. Resolve date boundaries as the first instant whose local civil date is at least the requested date, handling repeated and skipped midnight or a skipped day explicitly. Use the same routine for the following day's exclusive bound. Test a preceding-date item in the gap case, plus same-day inclusion on a normal DST transition.
2. **Blocking · §2.4, six digits do not bound a duration.** At one million weeks the duration is thousands of years. A scratch Go check calculated `999999 * (168*time.Hour)` as a negative interval, so subtracting it would filter from the future. Use checked arithmetic before multiplying, or subtract validated whole seconds with checked conversion. Specify the error at the representable limit and test overflows for both days and weeks. Preserve the established 24-hour semantics.
3. **Should fix · §4.2, phrase-ring token lifetime.** `Scanner.Next` returns scratch bytes reused on the next call, while phrase lookup retains the last k folded words. Copy retained words into worker-owned reusable slots, or keep stable token IDs. A ring of borrowed slices makes distinct tokens look alike. Add a test using different multibyte words and a deliberate reused scratch buffer.
4. **Should fix · §§4.1, 4.3 and 6.1, identify the overlap precisely.** Replaced-in-window overlaps eligible with `All`; step 3 currently says that whole set is never compared. Say only ineligible entries are excluded, then test a replaced semantic-only hit with `--all`. Skip the decision exception when the current lexical list is empty, including a replaced-only lexical page.
5. **Should fix · §6, reject invalid score metadata and preserve mode-relative wording.** A fake or faulty scorer with a NaN cutoff can bypass the threshold comparison. Treat a nonfinite cutoff as ModelFailed and drop nonfinite hit scores, preserving lexical results. In §6.2 distinguish unchanged lexical row units from the fitted page, whose footer and minimum can change under scenario 33. Keep the existing cancellation and non-context fallback tests.

**Verdict — Changes required.** Close both time-window errors before implementing the parser.

## Re-check

Reviewed `a5df84acebf62293d2df75c1a7c20b42584df082ab6e06a618b36093e36d1a7e`. Span subtraction no longer uses a duration. Phrase tokens have stable storage, eligibility preserves the replaced overlap, the empty-current case is explicit, and the scorer handles nonfinite cutoffs.

**Remaining blocker · §2.4.** The revised whole-range binary search still assumes local dates never go backwards as UTC advances. In `America/Goose_Bay` on 1988-10-30, 02:00Z is the first local midnight, 02:01Z rolls back to Oct 29 at 22:01, and 04:00Z is a second midnight. A scratch implementation of the documented search returned 04:00Z, omitting the first minute of the requested date. Walk constant-offset segments inside the existing bounded UTC range using `ZoneBounds`, then take the earliest candidate across those segments. On each segment local dates are monotone. Preserve the São Paulo and Apia regressions and add this repeated-date rollback.

**Verdict — Changes required.**

## Final re-check

Reviewed `c3cdbf6ca909af3879f4c22baf0a99e72a9f2b15f8ab3f2939a2c86ff7eb4078`. The constant-offset walk finds the first Goose Bay midnight at 02:00Z and preserves the skipped-midnight and skipped-day cases. I independently ran the segment search for Goose Bay, São Paulo, Apia and London.

Two editorial updates will keep the examples aligned with the stated algorithm. In the table, parse a bare date as civil calendar fields in UTC before passing them to `dayStart`, since parsing in the target location can already normalize away a skipped date. For the Goose Bay inclusion example, use 02:00:30Z, within the first minute of the target date, and say the buggy search loses its first minute.

**Verdict — Approve.**

Approval carries to `1d4ea87b7d5dbf99e7b143cadf5aa189f408e444fef8ccee76bb36d777a07e5a`. The bare-date parser uses UTC calendar fields, and the Goose Bay fixture uses 02:00:30Z.

## BM25 amendment

Reviewed `5898a0f9c643bded6f0aacf3384c6239393a73266283ae0e8297366af847b39c` against spec `079b3d19`, architecture `aed01c87` and render `6ee88823`. The BM25 formula, tier order, deterministic scoring, snippets and exact-only counts match the joint design. Requested frequencies come from the live eligible corpus and leave the strict fast path intact.

1. **Should fix · §§2.3, 4.6.** The partial tier currently requires every `Phrase` and `Prefix`, while the grammar turns `"durable"` into a `Word`. In a corpus where the guard admits `queue retry` for `durable queue retry`, the same row would also appear for `"durable" queue retry` despite the joint requirement to keep quoted clauses mandatory. Retain a required-for-partial bit through normalization. When a quoted word duplicates a bare form, keep one clause and set the bit if either occurrence is quoted. Support this with paired quoted and bare cases, and ensure strict matching and diagnostics stay unchanged. The [spec review](../functional_spec/bm25-amendment-lead-dev.md) requests the same clarification.
2. **Should fix · §§4.6, 6.1 step 5.** “Build P ... over the eligible items that are in neither L nor S” can be read as recomputing corpus statistics on that subset. Define the subset as output candidates only. For example, at N=4 with `df(queue)=df(retry)=1` and a disjoint item admitted by semantic scoring, removing that item from the corpus moves those DFs above the quarter threshold and incorrectly removes the partial row. Add a boundary test with the same BM25 output with and without a disjoint semantic hit, modulo merged order and deduplication.
3. **Nits · §§6, 6.2 and benchmark.** Update the old architecture hash in the seam paragraph. Distinguish all three tiers' unchanged row content from optional related lines, which can be dropped by the fitter. Make it explicit that snippet stretches are selected only for paged partial rows. The 92 ms scratch token sweep is an estimate for one workload, not an upper bound on all phrase, prefix and BM25 operations.

**Verdict — Changes required.**

### Recheck

Reviewed `f2684f382a0c8b9c397a7951c27eb84c524a1a1fe27ec44a249e52dd7e059333`. Both should-fixes are closed. Quoted provenance survives word normalization and deduplication, and affects partial admission alone. The partial statistics use the full eligible corpus before removing earlier-tier candidates. The named tests cover both changes. Snippets are selected on the completed page, and the scratch timing is scoped to its workload.

One editorial correction in §6.2. Delete “never between flags at the same budget and page.” At limit 1 an added tail can cost enough footer space to displace a related line while the same exact row fits under both flag sets. Scenarios 33 and 38 keep the row and snippet fixed. The fitter decides which related lines fit with the actual footer.

**Verdict — Approve.** Approval carries to that editorial correction. The spec and architecture wording that records quoted words remains with their author.

Approval confirmed at `345c76f076f8bd46d300d90cf977a92ca881072a7bc6b06eca03085241ca4ed9`. Section 6.2 now accounts for related lines displaced by a merged continuation at the same budget.

### Section alignment test

Approved `0dc8b8c41063197e148c938da2e730cd71b29fcde0a09f710c853f6bc27796f3`. A sole occurrence ending at `S + 10` yields `Start = S - 190`, inside the preceding section when that section starts there or earlier. The test names the section at the reported start and keeps the existing rule. The reference-only citation update is consistent with the reviewed draft.

## Output amendment

Reviewed `0dfde6ef6f7dc5f4a345fc08c03f7316dcc42cc5e33f7171e67cfa55bfd2a3bc` against spec `d1ba2637`, architecture `ed3682b8` and render `27d3f1ba`. The snippets, locations and per-field clause occurrence rules align with the formatter. A title-only semantic hit keeps its occurrence count without a snippet or context.

1. **Should fix · §4.7 and `TestContextLines`.** Section 4.7 strips a CR immediately before LF. The test's assertion that carriage returns never appear in `Text` also strips the embedded CR in `a\rb\n`. Preserve that CR for render to flatten. Define `After` with the half-open endpoint: a non-terminator character at `End` is already omitted. Pin both cases, for example `a\rb\n` beside `a\r\n`, and `[0,2)` over `abc` with `After` set.
2. **Should fix · `TestOccurrences` and `TestBM25OffReadsAsToday`.** The corpus-wide TF pass with BM25 counts occurrences for off-page eligible items. Assert that only page hits receive the summed count and that BM25 adds no page-level recount. Without BM25 the early-exit matching pass stays intact, and the separate count of page hits still runs. Make the off-page spy inspect only that separate work.

**Verdict — Changes required.**

### Output amendment recheck

Reviewed `1c9e64d7bef69533773e59d6413d3040d5156be20f88c8b1268ff33bdc8b8925`. Section 4.7 and `TestContextLines` agree on lone CR, CRLF and the character at the exclusive endpoint. Section 4.8 and the two occurrence tests distinguish BM25's corpus frequencies from the page's exported count and preserve the early-exit matching pass without BM25.

**Verdict — Approve.**
