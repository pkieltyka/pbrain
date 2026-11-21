# Component review: semantic search

Reviewed `45215e7a23be45797e54c8e505dc337a83c0f096c829379c66ffd5f19b4bc70d` against architecture `73c832e4`, functional spec `2b9cedaf` and the storage, importer and search contracts. The verified artifact uses the bytes actually passed to inference. The length-framed title/body digest and range checks keep old text out. Search stays read-only, and publication is an atomic manifest switch under a section-local index lock.

1. **Blocking · The model on disk and Build, recover a missing model even if vectors are current.** Download and Verify currently run only before the first passage embedding. Scenario: build a complete index using the downloaded model, delete only the per-user cache, and run `pbrain index` without an item change. Every entry still covers its item, so there is no embedding batch and the model is never restored. Every hybrid query keeps reporting `semantic_not_ready`, contrary to spec §§9.3–9.4. Make artifact availability and verification a distinct preflight for explicit builds with indexed items. Restore a missing or bad cache model and reject a wrong configured local bundle even when a build reuses every vector. Only initialize the inference runtime when an embedding batch needs it. Add a scenario 35 fixture for delete cache → no-op index restores model → hybrid query works, and a wrong local model with a current index. The same invariant belongs beside the architecture's lazy-load step.
2. **Should fix · Passages and adapter contract, state what evidence a truncated model input can support.** The title and heading caps limit common prefixes, but 400 code points can still exhaust a small model's token budget for dense or non-ASCII text. Have the spike test that a distinct body token survives the worst permitted prefix, or have model-specific preparation reserve space for at least some body text. Document the fallback for a passage truncated by the tokenizer so the ranked span is not described as wholly scored if only a prefix was embedded.

**Verdict — Changes required.** The model identity fixes work. Close the explicit rebuild's cache recovery before approving the component.

## Re-check

Reviewed `b029d3e75fb0c073a0a000e3bcc8b1a0d7455ee523f83de45b822dbf9b0c179a`. Build checks or restores artifacts on a nonempty section even when the index needs no new passages. It opens the inference runtime only for a queued batch. The new fixture restores a deleted cache with a current index and checks a wrong configured bundle. Preparation reserves space for actual passage tokens, and the adapter spike and test measure that at the dense-prefix boundary. The output and notice mapping fits the CLI delegation and the ten approved codes.

**Verdict — Approve.**

## BGE adapter delta

Reviewed `ddf45fad57fef305abb00e2850d01215fcccc78feced6ab88e2c38e8b04e8824` against the accepted spike `ab5fdde3` and architecture `d9eb42ae`. The pinned releases, three model bytes digests and sizes, memory shim, prompt, 510-token cut, special-token rule, padding path, CLS pooling and L2 normalization agree with the spike. The fingerprint accounts for preparation choices. Release builds enable SIMD and require the outstanding arm64 Mac measurement. The real-model reference and retrieval gates remain in place. [Accepted spike](../../research/semantic-search-and-index/embedding-models-and-go-runtime/spike-bge-small-hugot.md)

The startup guard is viable with the pinned graph. The concrete GoMLX import panicked before `main` when I passed an invalid `GOMLX_GO_AVX2_ASM` in a scratch process. With the proposed guard under the pbrain module path, a scratch binary reached `main` with the hostile variables inherited, with and without SIMD. It kept explicit command output on its original pipes and discarded the direct standard-stream writes. I also checked that the selected Go toolchain's `os` package imports `syscall` on all four targets. The dependency order matters, so the real-binary regression belongs in release verification. [Go program initialization](https://go.dev/ref/spec#Program_initialization)

Two nonblocking notes for the tests. Apply the same startup guard to the standalone calibrator and any real-adapter helper binary, so inherited settings cannot distort the release fixture. Exercise profiling for enough calls to print if accidentally enabled, or assert from the child that the variable is gone. With `GOMLX_OP_PROFILE=1`, one query does not trigger the table. Include the fatal-read variables `GOMLX_GO_AVX2_ASM` and `GOMLX_TENSOR_SUMMARY_SAMPLE_SIZE` in hostile startup coverage.

**Verdict — Approve.** The scratch checks verify startup and stream isolation on Linux amd64, while native arm64 Mac timing and the full retrieval fixture remain release gates.

## SQLite, second model and wider calibration

Reviewed `9e1ddcc3e6960f985daa76e8d89823f14347b3f358096cd3eac3cc4144244ba6` against architecture `aed01c87` and spec `079b3d19`. The per-section database and link/rename publication fit the store interface. Memory journaling and the explicit foreign-key setting close the rollback concern. The EmbeddingGemma 2 partial implementation remains behind its tokenizer, artifact identity, parity, timing and retrieval checks.

1. **Blocking · Index database and Source.** Before allocating, validate the manifest basename, digest, parsed size and independently measured descriptor size. Define a storage-size bound and check the sentinel arithmetic. On read, verify that the database has the expected schema and reject invalid data before scoring or classifying the index as usable. A matching file hash does not make arbitrary deserialized contents a valid pbrain index. Give search and status an unreadable outcome for a recognized generation that cannot be decoded or scored, and give the build a recovery path. Cover a manifest with an impossible size and a database with a matching digest but missing passage schema.
2. **Blocking · Source.** Apply eligibility and coverage before invoking the SQL distance function. The pinned modernc build returned an error from an excluded row after yielding a valid row. A materialized eligible subset fixed the scratch reproduction. Handle query and iteration errors section by section. Do not merge a failed section's partial row stream. Recompute `Missing` with that section uncovered when the other section works, and use `NoIndex` when neither works. Compare the complete rendered command for two sections with one damaged and one usable. [Architecture details and probe](../architecture/search-amendment-lead-dev.md)
3. **Blocking · Calibration.** `Score` now applies SQL's cutoff, so the cutoff estimator cannot reliably use it as specified. Expose an unthresholded fixture path through the same query implementation, take all non-exact per-item maxima before paging, and define the no-score case. Then test the generated cutoff using the actual command. Add a held-out regression where each standalone mode passes but their union fails. Gate that combined command for each released model. Use the feature-enabled CLI for fixture tests of an unreleased partial tier.
4. **Should fix · Model on disk and Build.** Distinct index locks do not protect the shared model source folder. One project can finish installing the pack and remove verified source files while another is opening them. Make this a successful concurrent install: recheck and verify the final pinned pack on a source-cleanup race before failing or downloading again, or serialize acquisition per model. If adding a lock, keep the wait cancellable and specify its ordering relative to the index lock. Test two projects building the same model against one cache. Map a failed derivation or bad output digest and an I/O failure to their existing command errors and verify no bad pack can become usable.
5. **Should fix · Build.** Delete dead paths after renamed items can reuse their passage digests. Otherwise a rename re-embeds a personal item unchanged because no per-user cache is available. Test the same text moved to a new path in the personal section. Correct `TestScoringQuery` to expect the deliberate SQL slack and the final exact comparison.

**Verdict — Changes required.**

### Recheck

Reviewed `7f3fd74c0e2d92b37a6846c4ff45fe52191cc20aa644eb7031ea3a2b639ec9dd`. Manifest bounds and name checks, eligibility before distance, section-local fallback, uncensored calibration and combined release gates are in place. The ordered model lock closes the cache race, the error mapping is specified and rename reuse has a test. The tokenizer pin and bytes-loader boundary match the local spike.

**Remaining blocker · Reading, Build and Status.** Preparing the statement detects a missing table, but it does not establish that an indexed row can be scored. A scratch run using the exact revised join on modernc v1.62.0 produced one valid row with the bad item out of `eligible`, and failed only when I included the bad item. The serialized fixture's metadata, scoring-statement preparation and `PRAGMA quick_check` passed. A length-only check allowed a TEXT value where a vector BLOB was expected. A matching digest can accompany that image. Build step 6 then skips every covered row and publishes nothing, and the next eligible search encounters the same fault. Validate the actual stored schema and row types, lengths and invariants in build/status before they trust coverage. For durable corruption, status should report unreadable and a no-edit index run should repair it. A transient runtime or cancellation fault should keep its existing command-local handling. Extend the injected query-failure test with a persisted, matching-digest invalid vector and a successful repair. The search query can keep its prefilter and section-local fallback. [Architecture recheck](../architecture/search-amendment-lead-dev.md)

**Verdict — Changes required on durable index validation and repair.**

### Final recheck

Reviewed `8af1b16e7b983724de2d7638fe7bbd460ec2e835f6b277c3beddf256030a4aae`. Status and build now check types, lengths, spans and item references, and exhaust the scoring query with a fixed vector over all indexed items. A failed check bypasses the existing coverage on rebuild. The pinned-driver probe confirms that the new type check catches the correct-length TEXT fixture missed by both preparation and quick_check. The four-command regression tests search fallback, diagnosis, repair and a clean search. Include a correct-length non-BLOB alongside its wrong-length fixture to exercise the exact reproduction.

**Verdict — Approve.**

## Aikit runtime and evidence tests

Reviewed `69a802373c8a02ae753657b87881d9924c2833a38d3a25084e16ae9429cbb8ac`. The adapter transfers each verified buffer once, keeps no writable alias, and gets its tokenizer from the loaded BERT. The runtime consumes the five pinned files through the private FS. Its fingerprint changes with the safetensors inputs. The linked binary drops the previous runtime and keeps the explicit stream isolation. The documented fallback is outside the release imports.

Two test-plan refinements for the fix pass:

- Tie each long-source quotation to one specific expected item ID, or require exactly one expected ID for that question style. Today the schema accepts an array while the fixed reader takes “that result” and has one follow-up read. Make the read deterministic and keep the original view. This preserves separate counts for an item hit, visible evidence and misses.
- In `TestBgeBuildsFromBytes`, specify rejecting a handoff that starts without one of the five required names. The existing “empty after load” assertion catches a loader that left a supplied name unread. On its own it cannot catch a handoff built without the pooling mapping, since aikit can finish and default to mean pooling.

Checks: verified the loader contract and ran the seeded parity, open-once and missing-file Go tests against `04b54e26` with CGO disabled. The real-model test skipped in my scratch checkout for absent assets, so full-checkpoint parity evidence remains the reported fork run and spike.

**Verdict — Approve the runtime design.** Recheck the final fork-main pins and test wording when updated.

### Runtime delta recheck

Reviewed `71e8d6f6e3b04db0a708fdf066291a49acd1521a582ce78c983add17e6025b43`. The handoff rejects an incomplete set before loading and checks that all five names were read exactly once. This closes the silent mean-pooling case while keeping verified-buffer ownership. Each long-source question has one expected ID. The fixed reader opens the first result in the same view and the answer key only judges output. The test plan distinguishes unsupported evidence, rank and missing items, and charges both searches on a retry.

**Verdict — Approve the delta.** The direct module paths are consistent. The goinfer loader and model pack, final main pins and pinned-graph release tests remain outstanding.

### Evidence reader response format

Approved `6a10dfa2cae1d81e2897c251483b165aa70a6387960e6c449501e56135703f1d`. The tool judges and measures the same budget-fitted JSON stream the fixed reader gets. The new fixtures cover row and body differences across output formats and count emitted escapes. When collapsed whitespace is used to locate the quotation, map its ending back to the original field before encoding that field prefix.
