# Q6 CUDA FP16 evaluation (#67)

**Owner decision.** Keep Q6 on ordinary CUDA with no FP16 opt-in for now. Revisit when the fix for the attach overhead ships. [goinfer #82](https://github.com/pkieltyka/goinfer/issues/82)

**Recommendation to the owner.** Keep ordinary CUDA as the Q6 default. Offer `semantic.cuda_precision = "fp16"` as an explicit opt-in for users whose longer syncs benefit on eligible GPUs. FP16 reduced fresh-sync wall time by 7.6% on 219 passages and increased it by 62.5% on 59. Both routes returned the same ranked IDs on all 77 retrieval questions. The experiment used a benchmark-only hook on `38a16ea`. Production adoption requires a separate owner decision. [Issue and evidence](https://github.com/pkieltyka/pbrain/issues/67)

If approved, preserve the distinction between an omitted setting and explicit `"fp16"` in `internal/brain/config.go`. For Q6, omission and explicit `"fp32"` both select the base profile. Explicit `"fp16"` requests the accelerated profile. Registering `CUDAFP16Fingerprint` alone would activate today's global FP16 default and stale existing Q6 manifests. Preserve the Google/Q8 defaults. Before enabling Q6, test omitted and explicit configuration, both rebuild directions, ineligible-device fallback and isolation from a warm opposite-profile cache.

## Matched fresh-sync results

Two interleaved pairs per fixture used the same RTX PRO 6000 (CC 12.0), verified Q6 bytes and release-tag evaluation binary. The selector and UUID were checked for all eight attachments. Both pairs on each fixture agreed on the direction of the wall-time change.

| Passages | Ordinary CUDA | CUDA FP16 | Median peak device memory, ordinary → FP16 |
|---|---:|---:|---:|
| 219 | 6.88 s · 31.84 passages/s | 6.36 s · 34.45 passages/s | 2,130 → 1,782 MiB |
| 59 | 2.44 s · 24.22 passages/s | 3.96 s · 14.91 passages/s | 2,094 → 1,754 MiB |

On the 219-passage fixture FP16 cut forward time from 4.59 to 2.50 s, but attachment grew from 1.08 to 2.79 s. That left a 0.52 s saving on the larger sync and a 1.52 s penalty on the smaller one. Passage lengths and batch counts differed between fixtures. These results support a workload choice, with no measured crossover threshold.

Both routes passed SQLite integrity and produced identical item and passage metadata. Packed, split, reversed and 1 MiB-budget layouts reproduced each route's singleton vector bytes on both fixtures. All 278 vectors changed bytes across routes. Against the same-file CPU reference, the lowest FP16 cosines on either fixture were 0.999999123 at 768 dimensions and 0.999999173 at 512. Ordinary CUDA exceeded 0.99999999998 at both widths.

On the 35-note, 62-passage retrieval fixture, CPU queries at the fixed 0.72 cut-off returned identical ranked IDs across the two indexes for each of the 77 questions in both semantic and combined search. Each route placed the expected note first on 39/47 questions, in the top three on 42/47, and abstained on 30/30 held-out unanswerable questions.

## Two vector identities, one file

Follow Google and Q8 in `internal/embed/gemma2/gguf.go` and `model.go`. Both arms load the same verified 248,812,032-byte Q6 GGUF (SHA-256 `dc84f042bc3ffbe6122b3b38b2fb299d66f5f37b276c9d458db85d4a4f1e6ba3`), store 768 float32 values per passage and use the row's 0.72 cut-off. Preserve the model folder, tokenizer and preparation. The proposed FP16 vector key is `fp16Fingerprint(q6Fingerprint, goinferRevision)`. Hash the UTF-8 string `pbrain-vector-profile-v1\n<base vector hex>\ncuda-eg2-fp16\n<full goinfer commit>` with SHA-256, using literal newlines and no trailing newline. At v0.6.7 the commit is `c26c69ed44717d86cf2218370054d5e89886b694`. The chunker-v1 index key hashes `pbrain-index\n1\n<vector hex>` by the same convention.

| Q6 writer | Vector fingerprint (also keys shared passage reuse) | Section index fingerprint |
|---|---|---|
| Base, including ordinary CUDA | `16d5093f31a4399589c717804e04d2ad3fd70df5bd42b36bf60424dc4599e2f4` | `8c964b1108dc4e42f65ea44837ce518911dc462ccb57d8fe1eba862a7e98bfa7` |
| Proposed CUDA FP16 | `43ce747722b0e928f7dc6218872509e69b777814e2cd4bdf4cfd13b63eedf434` | `cbddba624bade0aca37c5aec7c6ab1b0efeeb962fca64ad76f8251a5b9250c77` |

Resolve through `Registry.ResolveWriter` and the row's existing `ResolveFP16`. Use the selected CUDA device's `RunnableSelectors` from goinfer discovery, the compiled selector list and a persistent UUID. CC 8.0 or newer is required. Bind `UseDevice` to that UUID and `cuda-eg2-fp16`. A missing selector or an older card resolves to the base profile before writing, with the existing selection notices. An attach or forward failure after resolving FP16 cannot authorize ordinary vectors under its key. Only a certified batch OOM permits splitting on the same encoder. UUID, ordinal and batch shape are recorded as evidence and excluded from both keys.

## Switching and rebuilding

Use the existing `internal/semantic/profile.go` manifest rules. Each shared or personal section has one effective vector fingerprint. A switch in either direction builds a fresh generation on sync. Reuse is confined to the destination profile. Personal vectors stay in the personal index. A new goinfer revision changes the FP16 key and triggers a rebuild even if its vectors happen to match. CPU search reads the committed profile without probing a GPU. CPU catch-up cannot append to an FP16 generation.

The manifest also records an FP16 *request*, including when hardware resolves it to the base profile. Today's Q6 manifests have no such marker. Enabling FP16 by default makes those manifests stale under the new request, even on a fallback host. Explicit `semantic.cuda_precision = "fp32"` preserves the base request. Switching an already marked section back to fp32 requires a sync of that section.

## Evaluation criteria

1. Compare interleaved fresh syncs on the 219-passage and 59-passage fixtures in separate SQLite indexes and caches. Match binary, verified model, RTX UUID, budget and batch settings under the #66 quiet-host timing lock. Record selector and attached ID, wall time, passages/s, verification/load/attach/forward/SQLite times, batch shapes and peak device memory. Use the observed paired spread to judge an apparent win.
2. Require SQLite `quick_check` and exact item/passage metadata. For each route, require CPU same-file cosine ≥0.999 on every passage at 768 and renormalized leading 512 dimensions, and identical bytes across batch layouts on that route. Report cross-route byte equality and worst/median cosine. A close cosine still needs its own profile.
3. Run CPU queries against each index on the full #61 retrieval fixture at the fixed 0.72 cut-off. Compare expected-item top-1 and top-3 hits, held-out abstentions and changed query outcomes, for standalone semantic and combined search. The timing subsets cannot answer this check. Report denominators and investigate offsetting wins/losses even when aggregate counts match.
4. Recommend default-on only if both fixtures show a repeatable end-to-end benefit, memory use is acceptable, and correctness and retrieval hold. A reproducible win on the larger fixture with a small-sync cost favors explicit opt-in for frequent long syncs. If neither fixture benefits, or integrity, parity, same-route identity or retrieval regresses without an accepted explanation, retain ordinary CUDA. The measured trade-off above meets the explicit opt-in criterion.

@lead-dev owns the matched results in `internal/cli/README.measure.md`. @reviewer checks the route evidence, profiles and gates before the owner decides.
