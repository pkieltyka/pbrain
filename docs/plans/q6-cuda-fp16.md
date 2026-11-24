# Q6 CUDA FP16 evaluation (#67)

**Decision for v0.4.5.** Keep ordinary CUDA for `embeddinggemma-2-ud-q6-k-xl`. Evaluate FP16 through a benchmark-only hook from master `38a16ea`. A production switch needs the owner's decision after the matched indexing and retrieval results. The upstream 200-input replay found 1.90–1.93× higher warm throughput. A fresh pbrain sync also pays for verification, model load, attachment and SQLite publication. The ordinary Q6 baselines are 6.86 s for 219 passages and 2.49 s for 59. [Issue and evidence](https://github.com/pkieltyka/pbrain/issues/67)

If the owner chooses **opt-in**, preserve the distinction between an omitted setting and explicit `"fp16"` when reading `internal/brain/config.go`. Resolve an omitted setting for Q6 to fp32 and advance only explicitly opted-in Q6 sections. Google/Q8 keep their current defaults. Simply supplying `CUDAFP16Fingerprint` would activate the existing global fp16 default. If the owner chooses **default when eligible**, supply the row fingerprint and use the existing default and fp32 opt-out. For either choice, prove omitted/explicit configuration, both rebuild directions, ineligible-device fallback, and isolation from a warm opposite-profile cache before enabling the row.

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

## Evidence and recommendation

1. Compare interleaved fresh syncs on the 219-passage and 59-passage fixtures in separate SQLite indexes and caches. Match binary, verified model, RTX UUID, budget and batch settings under the #66 quiet-host timing lock. Record selector and attached ID, wall time, passages/s, verification/load/attach/forward/SQLite times, batch shapes and peak device memory. Use the observed paired spread to judge an apparent win.
2. Require SQLite `quick_check` and exact item/passage metadata. For each route, require CPU same-file cosine ≥0.999 on every passage at 768 and renormalized leading 512 dimensions, and identical bytes across batch layouts on that route. Report cross-route byte equality and worst/median cosine. A close cosine still needs its own profile.
3. Run CPU queries against each index on the full #61 retrieval fixture at the fixed 0.72 cut-off. Compare expected-item top-1 and top-3 hits, held-out abstentions and changed query outcomes, for standalone semantic and combined search. The timing subsets cannot answer this check. Report denominators and investigate offsetting wins/losses even when aggregate counts match.
4. Recommend default-on only if both fixtures show a repeatable end-to-end benefit, memory use is acceptable, and correctness and retrieval hold. A reproducible win on the larger fixture with a small-sync cost favors explicit opt-in for frequent long syncs. If neither fixture benefits, or integrity, parity, same-route identity or retrieval regresses without an accepted explanation, retain ordinary CUDA. Missing measurements leave the recommendation open.

@lead-dev owns the matched results in `internal/cli/README.measure.md`. @reviewer checks the route evidence, profiles and gates before the owner decides.
