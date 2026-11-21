# Architecture round 2

Reviewed `96c56856e31fa2967886647524e8c4597a6ff65ed1fb4c2455af00d746f6db9e` against functional amendment `2b9cedaf`, owner A1–A8/B/C, the approved component interfaces, and scenarios 33–37. The separation between lexical search and the optional embedder, flat-file scoring, separate section indexes, explicit incremental builds and gated interim runtime fit those decisions. The measurements justify trying this design. The actual binary still needs the end-to-end timing and release fixture.

## Blocking

1. **§§8.7 and 17, identifying current item text.** A path, length and CRC-32C cannot establish that an index entry represents a live item. A collision retains an obsolete vector, so an unrelated item can pass the cut-off and rank for its previous meaning. Printing current text does not repair the false hit, and stale code-point spans may be out of bounds. Compare a strong digest (e.g. SHA-256) over the same scanned snapshot the query renders, with the same projection on build and lookup. Include the title and any metadata that affects passage input if using a content projection. Store it with the item entry. This can be an additional catalog fact for semantic operations without changing lexical-only scanning. Revalidate span bounds before slicing. Test with an injected equal weak version on different input and verify the changed item is excluded and counted.
2. **§8.7, model identity on query.** The registry says it opens verified files, but search accepts a model by name and size alone. A wrong file of the same size at a configured local path or a damaged cache entry could load successfully under the pinned fingerprint and score vectors in another embedding space. Describe how a query uses the very bytes authenticated by the release digests. Either verify files on open or have index build stage verified immutable artifacts and bind later opens to their verified identities, with explicit revalidation on change. Define how the single offline `model_path` supplies every file when a release entry names a bundle. Test same-size wrong weights and a missing companion file: a build fails without publishing, and hybrid searches preserve their lexical result and report not-ready. Measure the verification cost in the stated 1 s budget.

## Should fix

3. **§§8.5 and 8.7, evidence spans.** The merge discards `SemanticHit.End`, taking 200 code points from Start even if the closest passage ends earlier. A two-line section can therefore quote the following, unrelated section as evidence; a title-only hit has a zero-length body span. Respect the half-open span in both item and body bounds, then take at most 200 characters from within it. Return no body snippet for title-only evidence. Test adjacent short headings and the linked item without a summary.
4. **§§8.7 and 14, release gate.** The abstention test counts queries with no *added* result. It can pass when every supposed abstention still produces an unrelated lexical result. Measure the complete rendered page, on held-out queries expected to yield zero, alongside gain on answerable tasks. Distinguish untouched regression cases required to abstain from an aggregate 95% estimate useful for calibration. Freeze the IDs and split before tuning. The gain gate should also observe actual first-page content: a changed continuation can affect a budget-bound page.
5. **§§8.7, 10, 13 and 17, cascaded contracts.** §13 still says the semantic flag never turns a successful search into a failure. §7.1 of the amended spec permits a larger hybrid minimum. Preserve lexical failures and fallback for retrieval faults, and explicitly allow `budget_too_small` for the combined requested mode. Remove the obsolete HTML-fixture uncertainty in §17: the approved importer design already records the public article, shell and short-page measurements. The model file uses the newly agreed streaming `fetch.Client.Download` contract. Keep the 30-second import limit independent of the model transfer.

**Verdict: changes required.**

## Re-check

At `ed11b876ab594b0a2708cac5aff6a07cbe1bee0d32291474c871b8f174319007`. The strong length-framed content digest, live-span checks, rendered-page release gates, per-mode budget and direct model download resolve findings 1 and 3–5.

**One remaining part of finding 2.** Passing the same file descriptor from Verify to the adapter closes the filename-replacement race. It does not freeze the contents of that file: an offline installer updating a configured model in place can change bytes between hashing and the adapter’s second read from the rewound descriptor. That contradicts the stated guarantee that the bytes checked are the bytes used. Have the adapter consume a private snapshot of the bytes as hashed, or compute the digest over the actual materialized bytes the runtime will consume before accepting the model. Keep the read path local and update the memory measurement. A deterministic test should change the same open file in place after verification and before ingestion; the result must use the original verified bytes or give the declared mismatch outcome. This can be resolved within the Artifact/adapter contract.

**Verdict: changes required.**

## Final check

At `73c832e43d883b0aacc9de5adfecb4873bfb4fbc7fb89f795733167cf955780d`, Verify hashes the process-owned bytes passed to the runtime. The adapter consumes that snapshot without reopening a file. The regression covers both a renamed model and an in-place change. The spike must measure the added memory and verify the adapter loads from bytes. All review findings are resolved.

**Verdict: approve.**

## Consistency delta

At `d7a34fb3aef56ab3d1fcdab680a3e138e3f19a8036627457ff1fa27a75c61fda`. Reviewed §8.7 model restoration and passage truncation, §3 package dependencies, §8.3 related-line ownership and §13 disclosure and notices. The build can restore verified model bytes with an up-to-date index without opening the inference runtime. The notice list matches functional §6.3 and CLI delegates its wording to the designated owner.

**Verdict: approve.**

## Interim runtime delta

At `d9eb42ae433cd17efff3369facc8a9a33c426cbf6b23d0cfcfc47eaef74191f8`. Sections 8.7 and 14 carry the release toolchain, native arm64 Mac measurement, real-process environment and output checks, and retrieval gates. Checking the pinned GoMLX sources confirms that some settings are captured during package initialization. The early guard is acceptable with the prescribed binary tests on every release target and the pinned-patch fallback. The spike measured Linux/amd64. The release tests and the end-to-end hybrid latency and retrieval measurements remain to be run on the implementation.

**Verdict: approve.**
