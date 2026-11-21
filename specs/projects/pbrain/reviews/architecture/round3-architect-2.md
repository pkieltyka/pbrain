# Architecture amendment review

Reviewed `aed01c874c95293b8790e3b6a74bb0f8c39939b57e4225f0eaa947f05e787d36`, including the §13 release-feature entry point and the §16 owner decision on an uncapped ordinary SQLite table. Both align with the component contracts. The scan, admission inequalities, stable tier ordering and deduplication in §8.8 implement the accepted BM25 decision. The fork's base resolves to `e0d66171270987a0e0fd2f85d2c6834d4daf66a9`. The tokenizer and full verified first-query timings remain implementation gates.

1. **Blocking — §§8.7–8.8 and 14, combined abstention.** Run the release test for both flags together with each model. Merely printing this arm's measurements permits a release in which opposite sets of false positives from the two standalone modes produce a failing combined page. Enforce the threshold and regression list on that arm and add the 38/40, 38/40, 36/40 case in the [spec review](../functional_spec/bm25-amendment-architect-2.md).
2. **Blocking — §§8.5, 8.7 and 14, calibrating a model through the new SQL cutoff.** Production `Score` now returns only scores near or above the model's existing cutoff. Calibration requires the maximum nonexact score *before* a cutoff is chosen. For example, with a provisional 0.7 cutoff, maxima of 0.51 and 0.65 are both absent. Give calibration an uncensored score path, such as a calibration-only registry row with cutoff −1 for collecting cosine maxima. Exclude every exact-match item using the full eligible exact set, including exact items beyond the first page. Keep the rendered, fitted production command for testing the frozen cutoff. Test maxima below a provisional cutoff and an off-page exact hit.
3. **Should-fix — §8.7, reading and publishing a database generation.** Validate the manifest's database basename and size before opening the file or using the name in cleanup. On a later SQL error, discard that section's results as a unit and use the existing missing-section behavior. A successful metadata query alone cannot establish that the scoring query will complete. Specify the same recovery in status and build. Concrete cases are in the [component review](../components/semantic-architect-2.md).

**Verdict: changes required.**

## Re-check

Reviewed `433a6aa2445a9d267b419766a4ff08155ea8cf707e732b697a99e4fc2ba46d85`. Both blockers are resolved. Sections 8.7, 8.8 and 14 gate the combined rendered page per model. Calibration collects maxima at cutoff −1 and excludes even off-page exact matches. The pack digest and 24–26 ms tokenizer timing agree with the linked spike. The full adapter still has to meet 600 ms.

The database finding is partly resolved. Generation names, size checks, section-local failure and empty-section repair are specified. One persistent row-error case remains in the [semantic re-check](../components/semantic-architect-2.md). Two sentences also contradict the new provisioning order: §8.7 says a build takes the index lock only and §10.4 says no command holds two locks. Limit those statements to item/settings locks and say the model-provisioning lock is nested inside the index lock. Readers take neither.

**Verdict: changes required. No blocking findings remain.**

## Final re-check

Reviewed `380a85e5da8658ed13fde1b3eb5e2926f12f38e279af715c16d8bcb36b108230`. Sections 8.7 and 10.4 agree on lock acquisition. A build holds its section index lock and takes the per-model cache lock during provisioning. Status and build validate stored passages and execute the scoring query. A build repairs persistent row damage even when all item digests still match. Both remaining findings are resolved.

**Verdict: approve.**

## Aikit runtime and evidence-test review

Reviewed `064c9b4ffd75caec5f468a71a8deac59fa5747982e721603037f81bc992300f8`. The bge-small design names the five-file model, the bytes-only loader, the retained stdout/stderr routing and the per-model release gates. The 124–153 ms measurement in §8.7 agrees with the spike; the end-to-end estimate still needs a built-command measurement. The superseding owner instruction on fork-main pins applies to the next draft.

**Should-fix — §§8.4 and 14, section-alignment boundary.** A match early in a section *can* produce a snippet start in the preceding section. The draft says it always does. Suppose the section starts at code point 1000 and two distinct clauses occur at [1025,1030) and [1215,1220). The earliest 200-code-point window covering both starts at 1020, in the current section. For the cross-boundary test, specify one sole match ending at 1030, giving start 830, and a preceding section containing 830. For the multi-clause case, derive the named section from the actual returned offset. This correction holds under the current naming rule and requires no decision on whether to change that rule.

**Verdict: changes required. No blocking findings.**

## Aikit runtime re-check

Reviewed `e36c22ada330dd0039f43b002874ea63f1fff73f5148f6c7774a770d8d66c6ae`. Sections 8.4 and 14 correctly distinguish the two fixture outcomes. The sole match yields offset 830 in the preceding section; the two clauses yield offset 1020 in the current section. Section 14 makes the first follow-up independent of the answer key. Sections 8.7 and 15 specify direct module requirements and fork-main revisions after each module rename. Those two commit pins still need verification when available.

**Verdict: approve.**
