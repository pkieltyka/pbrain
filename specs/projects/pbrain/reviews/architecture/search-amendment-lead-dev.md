# Review of the BM25, SQLite and model amendment

Reviewed `aed01c874c95293b8790e3b6a74bb0f8c39939b57e4225f0eaa947f05e787d36` against `079b3d19`, `9e1ddcc3` and the owner's decisions. The ordinary SQLite table with sqlite-vec, in-memory rollback, confined immutable publication, the Embedder boundary and the gated tokenizer-first fork follow the decisions.

## Blocking

1. **Pre-filter coverage and eligibility before computing distance (§§8.5, 8.7 and 14).** The SQL evaluates the distance for every passage. The Go filter comes after evaluation, so it cannot protect the eligible answer set when an excluded row causes the query to fail. The component also promises that an uncovered or ineligible item is never scored. On the pinned modernc and sqlite-vec build, a bad vector on item 2 caused a score query to fail after returning the valid row for item 1. Appending an `AND item_id = 1` did not prevent it. An eligible rowset materialized before invoking the function did. Use an in-memory eligible-id relation joined before scoring, or an equivalent evaluation boundary. Keep uncapped distance selection and benchmark the real query. Define section-local recovery when a query or iteration still fails. Discard that section's staged scores, retain the other section, and report coverage or readiness accordingly.
2. **Keep raw calibration input separate from a released cutoff (§8.7, §14).** The source now returns only scores around or above its current cutoff, while calibration derives a cutoff from the highest scores on unanswerables. Using the published `Score` with a default or previous cutoff censors the input to calibration. Use the same scorer with an explicitly unthresholded fixture path that receives all valid eligible non-exact item maxima, and exclude the complete exact list, irrespective of page size. Specify how the calibration treats a question with no comparable non-exact item. Then use the real CLI and frozen constants for held-out pages. Add the combined per-model abstention gate identified in the [spec review](../functional_spec/bm25-amendment-lead-dev.md).
3. **Bound and validate the serialized snapshot before allocation (§8.7).** The size/hash check takes its expected size from an editable or damaged manifest. An absurd `size` can overflow `size + 1` or drive allocation before SQLite or the fallback can reject it. Require a bounded decoded manifest and one canonical generated basename, validate the stated digest and size, compare descriptor size before allocation, and use a checked parse and bounded read. Define an upper bound and graceful behavior for a build that would exceed it. The bound applies to stored bytes and does not cap results. Test an oversized manifest value and malformed database with a matching digest, alongside the existing accidental byte-change case.

## Should fix

4. **Preserve exact results and express the new source contract (§§8.2, 8.5 and 8.8).** Keep single-word quote provenance for mandatory admission and collapse duplicate word evidence. Update the `Scores.Hits` comment to describe the thresholded source and remove the statement that it returns one hit for every compared item. Say “matching the live scan” where the current phrasing “current” could be read as excluding replaced items admitted under `--all`.
5. **Budget the actual snapshot copies (§8.7).** The pinned driver's `Deserialize` allocates and copies its input. `Serialize` also copies into a Go slice. Replace “held in memory once” with the peak model, snapshot and driver buffers in the overlap, and release the Go snapshot after deserialization. Measure combined mode with both sections and the 768-dimensional model on the four-core reference hardware. The warm scratch timing and single-section estimates do not exercise that shape. [Pinned driver source](https://gitlab.com/cznic/sqlite/-/blob/v1.62.0/conn.go#L1372)
6. **Incremental reuse survives a rename (§8.7 build steps).** Deleting every dead path before looking up passage digests drops the only local copy for a renamed personal item. Defer deletion until replacement candidates have looked up reusable vectors, or stage a digest lookup from the previous snapshot. Publish a final snapshot without dead paths. Test a personal rename of unchanged text, with no embedding or per-user-cache access.
7. **Runtime swaps need fingerprint evidence (§8.7, bge on goinfer).** A preparation match and 0.999 reference cosine do not establish that new query vectors are interchangeable with every older document vector at a fixed cutoff. If the optional goinfer comparison leads to a swap, bump the adapter version and rebuild by default. Preserve the fingerprint only if the team verifies compatible outputs for same-text, cross-runtime query/document scoring and resulting cutoff decisions. This leaves both currently approved adapters intact.

Checks: compared the amendment with the owner decisions, spec and component contract. Ran a CGO-disabled Go probe using modernc v1.62.0 and its bundled extension. The in-memory connection retained MEMORY journaling, in-memory temp storage and foreign keys on after deserialization. A materialized selection kept an excluded malformed vector away from the distance function.

Verdict: changes required.

## Recheck

Reviewed `433a6aa2445a9d267b419766a4ff08155ea8cf707e732b697a99e4fc2ba46d85`. The new join excludes uncovered and ineligible rows before scoring. Section-local failure preserves successful sections. Calibration uses uncensored scores and tests every shipped flag/model pairing. Snapshot parsing has a generated-name and descriptor-size check. Quote provenance, full-corpus statistics, read-time copy accounting, rename reuse and runtime-fingerprint handling are in place. The tokenizer results match the spike and the three local commits, and the whole EmbeddingGemma 2 adapter still has to pass the timing and parity gates.

**Remaining blocker · §8.7, validation and later failure.** The design promises that status diagnoses a persistently unreadable database and a subsequent build repairs it. In the pinned driver, I serialized a fixture with matching metadata and a passage whose eight-byte TEXT value passes the current BLOB-affinity column's length check. The revised scoring statement prepared successfully and `PRAGMA quick_check` returned `ok`. With the bad item excluded, scoring produced the valid result. With it eligible, iteration produced a SQLite error. Status and the build both accept that snapshot through `openIndex`, and the build skips the covered item, so another search fails again. Keep the efficient search filter and section fallback. Give status and build a model-free validation path that detects persistent stored-row defects before trusting coverage, and specify search-local behavior for transient statement errors. Add an end-to-end status and no-edit repair test for a matching-digest database that prepares but fails when a bad row is scored. [Component recheck](../components/semantic-lead-dev.md)

Two editorial corrections fit that pass. Scope §10.4's one-lock rule to item and settings writes and name index provisioning's ordered model-lock exception. In §8.7's query summary, put the eligibility check before the distance query, as the SQL and component now do.

Verdict: changes required on durable index validation and repair.

## Final recheck

Reviewed `380a85e5da8658ed13fde1b3eb5e2926f12f38e279af715c16d8bcb36b108230`. Section 8.7 now validates persisted passage rows in status and build without a model and classifies failures as unreadable. The next build starts afresh even when item digests match. Search keeps its fast per-section fallback. Section 10.4 now spells out the index-then-model lock order. The manifest read is bounded at 4,097 bytes.

Verdict: approve.

## Aikit runtime and evidence tests

Reviewed `064c9b4ffd75caec5f468a71a8deac59fa5747982e721603037f81bc992300f8` and semantic `69a80237` against the owner decisions. The aikit loader and ownership handoff fit the verified-byte boundary. The dependency and build tables drop the old linked runtime, startup guard and experiment. The saved fallback still has its evidence and implementation design. The output-stream rule stays at the process boundary. The revised 320–580 ms estimate distinguishes overlap from no overlap and leaves native release and full-command timings as gates.

The two accepted evidence-test changes are present. Scope the §8.4 and §14 boundary example to one occurrence ending less than 200 code points after a heading, with the preceding section reaching the computed start. The current “a match in the first 200 code points” is too broad for multiple matching clauses or an occurrence ending exactly 200 code points after the heading. This wording needs no product change. [Precise fixture](../components/search-lead-dev.md#section-alignment-test)

The fork and distribution paragraphs await the operator's fork-main update and the owner's install decision. Local `go help install` confirms that versioned installation rejects a main module with these replace directives. Release binaries and checkout-local installation remain viable while the forks are pinned. The architecture should record the owner's decision in §16 once made.

Checks: read the pinned loader API and ran its seeded parity, open-once and missing-file tests with CGO disabled in a scratch checkout. All three passed. The real-checkpoint test skipped because its model files were absent from that checkout.

Verdict: approve the runtime and test-plan design. Recheck the queued fork-main and distribution wording at its final hash.

### Runtime delta recheck

Reviewed `e36c22ada330dd0039f43b002874ea63f1fff73f5148f6c7774a770d8d66c6ae`. The module layout and CI rule preserve versioned installation without replace or exclude directives. The two libraries use their own paths and the plan retests the version of aikit selected for both adapters together. Section 8.4 and the two examples in §14 describe the existing snippet-start rule correctly. The evidence plan measures what the reader receives and keeps its answer-key upper bound distinct.

The renamed aikit topic commit `e29e0b2` identifies as `github.com/pkieltyka/aikit`. Its focused encoder loader tests passed with CGO disabled. The final main pins are still pending, and no versioned install or release tests ran.

Verdict: approve the delta. Verify the selected main commits when recorded.
