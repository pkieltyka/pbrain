# Semantic component review

Reviewed `60b06db788c4319b69c46429045b3d6fa6f5f1fc5918c4028d254317e9226755` against the owner amendment, architecture round 2 and the CLI, importer, store and brain contracts. The separation of section indexes, shared-only vector reuse, query/document roles and atomic manifest replacement is implementable. The following cases affect observable results.

1. **Blocking — OpenCache.** Step 3 accepts any cache outside a surrounding worktree; step 4 checks just that surrounding tree. Neither checks for a repository nested *under* the cache. An otherwise permitted cache with its vectors directory in another repository can publish project-derived data there. Apply the same no-nested-`.git` test used for protected directories to the cache hierarchy that pbrain is about to use, without following links. Enforce it before any cache creation or write. Test a cache outside Git with a nested worktree at its vectors folder, and an ignored cache inside a larger worktree with that same layout. Keep shared lexical searches independent.
2. **Blocking — Coverage and model on disk.** Apply architecture round-2 findings 1–2 to this component too. The segment needs a strong digest bound to the live snapshot used for output. An unchanged size, path and CRC is insufficient to authenticate indexed meaning. A model lookup based on name and size also fails to authenticate changed local weights before query inference. Specify how offline bundles fit the public `model_path` setting, how a validated file is bound to the runtime open, and how unusable local weights yield lexical fallback. Reflect both changes in the format, source APIs and fixtures.
3. **Should fix — Build and CLI.** CLI snapshots items before waiting for the index lock. Two builders can acquire the lock in a different order from the scans; the later lock holder can compact away new, valid entries using its older snapshot and report them removed. Capture a section catalog after acquiring its index lock (e.g. a caller-supplied scan callback returning items and body access), use it through publication, and let search compare with its own live scan. Test two processes with staggered scans and acquisitions. Treat the missing default personal section as empty on status without creating it; spell out what index build does with an empty section.
4. **Should fix — Segment reader, source and cache.** Validate every offset/count product with checked arithmetic against the actual file size before allocating. Validate code-point ranges against each live body and vector dimensions and finite/unit values on both cache and runtime inputs, including the single query vector. A failed query vector must yield lexical fallback. Bounds-check the selected passage before handing it to search. Exercise truncated files, valid-checksum invalid spans/counts and a runtime returning the wrong query dimension. Update the calibration fixture to score the actual rendered page and complete absence of hits as requested in the architecture review.
5. **Should fix — Getter and source ordering.** Replace the proposed `Getter.Stream` with the agreed `Download(ctx, rawURL, dst, maxBytes, idle)` bounded-writer method in importer. Use its `fetch_failed`/local `io` mapping for index. Reconcile CLI constructing the source *after* the scan with the parallel scan/model-load timing in architecture. Benchmark the actual sequence or start inference from guarded section handles before scanning and guarantee cancellation/closure on every exit.

**Verdict: changes required.**

## Re-check

At `39b7ce6cf71f00fb6fd0c2b8e18efc397ffe26890ca52dddcc10bcbf6078a0c5`. The cache now checks each directory it uses, and Build snapshots after taking its index lock. Strong content digests, checked segment arithmetic, query-vector validation, confined evidence, the bounded Downloader and the Start/Attach/Close sequence resolve the corresponding findings.

**One remaining part of finding 2.** Passing the same file descriptor from Verify to the adapter closes the filename-replacement race. It does not freeze the contents of that file: an offline installer updating a configured model in place can change bytes between hashing and the adapter’s second read from the rewound descriptor. That contradicts the stated guarantee that the bytes checked are the bytes used. Have the adapter consume a private snapshot of the bytes as hashed, or compute the digest over the actual materialized bytes the runtime will consume before accepting the model. Keep the read path local and update the memory measurement. A deterministic test should change the same open file in place after verification and before ingestion; the result must use the original verified bytes or give the declared mismatch outcome. This can be resolved within the Artifact/adapter contract.

**Verdict: changes required.**

## Final check

At `45215e7a23be45797e54c8e505dc337a83c0f096c829379c66ffd5f19b4bc70d`, Verify hashes the process-owned bytes passed to the runtime. The adapter consumes that snapshot without reopening a file. The regression covers both a renamed model and an in-place change. The spike must measure the added memory and verify the adapter loads from bytes. All review findings are resolved.

**Verdict: approve.**

## Consistency delta

At `b029d3e75fb0c073a0a000e3bcc8b1a0d7455ee523f83de45b822dbf9b0c179a`. Checked the up-to-date-index and missing-model case, invalid offline weights, empty sections, prefix truncation and the build/status/notice mappings against §9.4 and CLI. `TestBuildRestoresModel`, `TestAdapterContract` and `TestOutputAndNotices` cover the changes. An interrupted model file transfer restarts from zero.

**Verdict: approve.**

## Interim runtime delta

At `ddf45fad57fef305abb00e2850d01215fcccc78feced6ab88e2c38e8b04e8824`. The three artifact sizes and SHA-256 values match spike §8. The read-only memory filesystem serves the verified files. The preparation version covers the query instruction, neutralization of special tokens, the 510-token cut, CLS pooling and normalization. The process-level tests distinguish an adapter writing to stdout from the command preserving its actual output when a library emits a stray message. The spike measured Linux/amd64. The release tests and the end-to-end hybrid latency and retrieval measurements remain to be run on the implementation.

**Verdict: approve.**

Approval carries to `a427520cfd64310138ef165287e6faf33ef48c18fe5804b9ef4f14bef7f535bd`. The guard now covers every repository executable linking the backend, including calibration. The release test covers individual and combined backend settings and checks real output across multiple inference batches and process exit.

**Verdict: approve.**

## SQLite, model-pack and BM25 amendment

Reviewed `9e1ddcc3e6960f985daa76e8d89823f14347b3f358096cd3eac3cc4144244ba6`. The ordinary vector table gives every qualifying passage a chance to match. A single in-memory connection with a memory journal supports rollback. The two parity tests distinguish raw goinfer parity from parity under pbrain's input preparation. The 600 ms acceptance condition still covers verification, loading and the first query.

1. **Blocking — Calibration and The wider comparison.** Collect uncensored per-question maxima before applying the release cutoff. The SQL predicate now prunes the data that `Score` formerly provided to calibration. A calibration-only model copy with cutoff −1 suffices. Exclude the complete exact-match set when collecting scores. Test scores below the candidate cutoff and an exact match off the first page. Run the held-out abstention and regression gates on the combined flags for *each model*, also using the candidate CLI feature set. Standalone 95% passes can combine to 90%; the numerically worked case is in the architecture review.
2. **Blocking — The index database, Reading and Publishing.** The manifest is the source of the filename passed to both open and unlink. The read procedure checks its format and fingerprint but does not require the filename to match the generated pattern. A malformed manifest naming `lock` could lead cleanup after a successful repair to unlink the lock file a builder still holds, allowing another builder to lock a different inode. Accept only a single lowercase 16-hex generation basename followed by `.db`; validate before use, including recovery and garbage collection. Give the manifest a small read bound, validate the declared size against the regular-file descriptor, and check it can be safely represented before allocating or adding the sentinel byte. Test a reserved basename and an impossible size; rebuilding must preserve the held lock.
3. **Should-fix — The index database, The source and Build.** Reading the metadata and item map can succeed while a scoring query subsequently fails. I reproduced this with a SQLite database containing the four expected metadata rows and an item table but missing the passage table. The current source says only cancellation can return an error and leaves the other path undesigned. Prepare/check the required schema when opening. On a query or iteration error, discard any hits already collected for that section, mark it unreadable and count the eligible items once as missing. Keep valid sections; use the existing no-usable-index notice if none survives. Check status and rebuild recovery, including an existing unreadable generation in a now-empty section, where the replacement needs publication even with no embeddings.
4. **Should-fix — Build step 4 and TestDeriveDeterministic.** Index locks belong to sections. Two projects can concurrently provision the same model in one per-user cache. One can verify its pack and remove the shared source directory while the other is still opening source files for derivation. Serialize provision/repair/cleanup per model fingerprint and recheck for the completed pack after acquiring that lock, or specify an equivalent verified-winner retry. Leave searches and vector cache reads lock-free. Test two processes building distinct sections in one cache, with one paused between opening source files. Both should finish with the same verified pack.
**Verdict: changes required.**

## SQLite and BM25 re-check

Reviewed `7f3fd74c0e2d92b37a6846c4ff45fe52191cc20aa644eb7031ea3a2b639ec9dd`. Both blockers are resolved. Calibration now collects uncensored maxima, excludes the entire exact set, and gates each model with both flags. Database generation names and sizes are checked before use. The model-provisioning finding is resolved by the per-model lock and the second lookup after acquisition.

**Remaining should-fix — Reading, Status and Build.** A missing passage table is now detected at preparation, and search discards the whole section if stepping through the query fails. A persistent invalid value can still pass preparation and the item digest check, then fail only when the distance function evaluates the row. A separate status or no-change build follows the metadata-only path and can report healthy or skip repair indefinitely. For status/build, validate the stored passage data needed for scoring (including vector type and dimension) before treating covered rows as usable. A read-only search can keep its current fast path. Test a self-consistent manifest and a live item whose stored vector is malformed, then run search, status, ordinary index, and search again in separate processes. The final search should succeed after the build repairs that section. Distinguish persistent damage from a transient read error that has cleared by the next command.

Small implementation note: the specified `Dir.Read` reads up to 5 MiB. To enforce the stated 4,096-byte manifest bound during the read itself, use the existing `Files.OpenRegular` and read at most 4,097 bytes.

**Verdict: changes required. No blocking findings remain.**

## Final re-check

Reviewed `8af1b16e7b983724de2d7638fe7bbd460ec2e835f6b277c3beddf256030a4aae`. Status and build validate stored passages and consume the scoring query to completion. `TestDamagedPassageIsRepaired` specifies diagnosis and repair across four separate commands with unchanged source items. Search retains section-local fallback. The manifest reader uses `Files.OpenRegular` and reads at most 4,097 bytes to enforce the 4,096-byte limit. The remaining finding and read-limit note are resolved.

**Verdict: approve.**

## Bge-small on aikit and evaluation review

Reviewed `69a802373c8a02ae753657b87881d9924c2833a38d3a25084e16ae9429cbb8ac`. Checked the `LoadBERTFromFS`, `Tokenizer`, `Encode`, `SpecialID` and `Embed` interfaces against aikit commit `04b54e2673c45cfb97ffa734ccfd7dc0b70b5f77`; checked the five artifact pins against the spike and the archived model row. The verified-slice handoff and model lifetime agree with the loader's contract. The remaining findings concern observable test outcomes.

1. **Should-fix — Opening from memory and TestBgeBuildsFromBytes.** An empty handoff map at return proves that every supplied entry was consumed. With pooling absent from the map at entry, aikit's `poolingFromFiles` accepts the read error and selects mean pooling. The other four entries can all be consumed, the checks on the tokenizer still pass, and the proposed negative test would accept the wrong model. Validate the precise set of five named buffers on entry and track failed or repeated read attempts against the five expected paths through return. This requires only bookkeeping, with no second copy of the weights.
2. **Should-fix — Reaching the evidence.** The reader uses the fixture's expected ID to choose which hit gets its one follow-up read. If the answer is fifth, it can skip four convincing wrong hits using information the agent never had. Either label that figure explicitly as an oracle-assisted upper bound, or use a reproducible reader that cannot see the answer key. I recommend the latter: score the displayed snippets, then open the first eligible displayed result under a fixed rule, preserving view flags and the reported offset. Use expected ID/quotation only to evaluate what the reader received. Specify whether tokens to first evidence count the rendered prefix through the quotation or the entire response. The former measures position, the latter measures total context delivered; also charge any guided retry. Test a correct fifth hit, a long tail after the quotation and a retry so these figures can be reproduced.

**Verdict: changes required. No blocking findings.**

## Aikit runtime re-check

Reviewed `71e8d6f6e3b04db0a708fdf066291a49acd1521a582ce78c983add17e6025b43`. The five-file handoff finding is resolved. The opener requires the exact input set, checks one successful read of every path and fails on any refused request. The evidence reader now selects the first hit independently of the answer key, charges guided retries, and reports the oracle-assisted upper bound separately.

**Remaining should-fix — Reaching the evidence, output format.** The tool judges a JSON page but counts code points on a separately fitted text page. Under functional spec §6.1, a page that fits five text rows can fit only four JSON rows, and escaping can move a quotation beyond the end of a bounded JSON `get`. That makes the reported cost and visibility describe different readers. Choose one canonical rendering for this metric, e.g. text, and determine the hit IDs, snippets, follow-up offset, presence and charged prefix from that exact fitted output. The harness may retain structured metadata for the rows actually emitted. An independently fitted JSON run remains valid for its own metrics. Add one boundary fixture in which the two formats expose different evidence and state the expected outcome for the chosen format. No product rule or release gate changes.

**Verdict: changes required. No blockers.**

## Aikit runtime final re-check

Reviewed `6a10dfa2cae1d81e2897c251483b165aa70a6387960e6c449501e56135703f1d`. The reader selects hits, judges visible evidence and measures cost from the same printed JSON responses. Its prefix count includes JSON escaping and preceding calls. TestCalibrate exercises both a fifth hit and a quotation that fit only in text. The final finding is resolved. The two post-rename library pins remain to be verified when available.

**Verdict: approve.**

## Library pins

**Approve** `5f4a23ca0a1d6e703ec7af0fce4891dbd9107d1bc6d12c81585c2ab67c449294`. The aikit tag `v1.79.0` resolves locally to `3c0896c0`, declares the owner’s module path and contains `encoder.LoadBERTFromFS`. At review time the goinfer origin advertised no `v0.1.0` tag. The table correctly identifies that version as the planned tokenizer release. The EmbeddingGemma 2 adapter requires a later tag with the loader and pack support.
