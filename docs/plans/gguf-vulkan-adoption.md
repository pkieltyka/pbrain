# Adopt EmbeddingGemma 2 GGUF on Vulkan

The owner approved enabling both Q8_0 and UD-Q6_K_XL on Vulkan, including the Q6
manifest; no separate admission prompt is needed. The host implementation is
approved while #76 runs in hardware qualification; the device tests and
publication wait for the Vulkan prototype disposition. This document reconciles
pbrain's actual index and reuse rules with the cross-backend tolerance evidence,
and states how the existing default model, CUDA fp16 and strict named-device
behavior are preserved.
There is no speed acceptance gate: measure and report the actual sync, even below
2×. The host code and tests are implemented under the shared timing lock; no
model load, device run, publication or default change.

Start point: pbrain `master` `8a7b603` (merge #64, per-device CUDA selector
eligibility) with goinfer root, CUDA and Vulkan at v0.6.6,
`c0a5159bbff1c2fe5c8624e1399aa7438c820677`. `FACTORY.md` is this repository's
only instruction file; there is no `AGENTS.md` or `CLAUDE.md` here. Work happens
in an isolated worktree because `master` has unrelated dirty release notes.

## 1. Exact pinned files from #39

Reuse the files #39 validated; verify every digest before a run and do not
substitute a download or a regenerated conversion.

| Input | Bytes | SHA-256 |
| --- | ---: | --- |
| Q8_0 GGUF | 309,855,520 | `6f1bd4ac6c5df7444f9cca7ca36cafe6cfa34cd6f49fefb1e0b4be8143aed8bc` |
| UD-Q6_K_XL GGUF | 248,812,032 | `dc84f042bc3ffbe6122b3b38b2fb299d66f5f37b276c9d458db85d4a4f1e6ba3` |

Both come from `unsloth/embeddinggemma-2-GGUF` revision
`031f0d4b35536f69ab3509d4893c923264fcf253`; goinfer v0.6.6 pins both in its
`pull` manifest and decodes Q6_K. The frozen 32-note/59-passage fixture is
`internal/semantic/calibrate/testdata/retrieval/brain`, the one `batchFixture`
uses. The #39 note's Q8/Q6 receipts are the method and tolerance source:
797-position layouts `2eb20a42…` / `2775e8b4…`, prepared-ID results `100bf029…`
/ `5b9ec22b…`, 2,655-position comparisons `1c719935…` / `e0ef1644…`, six-cell
fit `79c315a8…`, corpus manifest `420988ad…`, and final seal `51bb42f0…`. The
#39 pack-only sync (`b4c451d8…`) is the pattern for a fresh-SQLite receipt; it
indexed the Google pack, so it is not Q8 evidence.

## 2. Q8_0's current Vulkan gate

The block is pbrain-side, not in goinfer's Vulkan backend:

- `internal/embed/gemma2/gguf.go` gives the Q8 row `Devices: listCUDADevices`,
  and `openQ8` calls `newRuntime(..., false)`.
- `internal/embed/gemma2/listing.go` has `listCUDADevices()` call
  `probeDevices(false)` and `deviceListing` admit only `cuda`; `listDevices()`
  with `probeDevices(true)` is the Vulkan-enabled path the Google pack uses.
- `internal/embed/embed.go` resolves a named selector against the row's
  `Devices` listing through `namedDevice`, so a Vulkan selector is
  `invalid_config` before a byte is decoded. `TestQ8RejectsVulkan` pins that for
  `vulkan`, `vulkan:0` and the persistent UUID.
- `internal/embed/gemma2/accel_vulkan.go` registers the backend only under
  `linux && amd64 && vulkan` tags.

Enabling Q8 on Vulkan is two flips on the row (listing and runtime flag), the
shape the Google row already uses, not a new backend. `TestGemmaVulkan` already
proves attach identity, memory reporting and batch layouts on the Google pack.

## 3. The Q6 manifest

`GGUFEntries()` currently returns one row, Q8; `cmd/pbrain/main.go`,
`internal/semantic/calibrate/main.go` and five test files index `[0]`.
`cmd/pbrain/main_test.go` expects `embeddinggemma-2-ud-q6-k-xl` to be
"not supported by this pbrain". goinfer already pins and decodes Q6_K, so the
missing artifact is pbrain's row, identity and release wiring, not a loader.

The owner approved the row. Add: name `embeddinggemma-2-ud-q6-k-xl`, runtime
`goinfer`, dimension 768, cutoff 0.72 (the #47 fitted value), one `verityFile`,
no `Source` and no `Derive`, and the same device policy as Q8. Its identity by
the existing recipe:

| Field | Value |
| --- | --- |
| Model fingerprint | `16d5093f31a4399589c717804e04d2ad3fd70df5bd42b36bf60424dc4599e2f4` |
| Index fingerprint | `8c964b1108dc4e42f65ea44837ce518911dc462ccb57d8fe1eba862a7e98bfa7` |
| Cache folder | `embeddinggemma-2-ud-q6-k-xl-22e1f6123853c8cb` |

Q8 keeps its existing `CUDAFP16Fingerprint`; Q6 gets none, because no fp16 CUDA
profile has been qualified for it. Q6 becomes an opt-in row, not a default, and
the 0.72 cutoff is not refitted by this lane.

## 4. Capability, identity and fingerprint

- **Capability.** `DeviceInfo.TextBatching` must be true; Vulkan sets it.
  `RunnableSelectors` is CUDA-only in goinfer v0.6.6 and returns nil for Vulkan,
  `ComputeCapability` is zero, and fp16 selection is CUDA-only. Vulkan memory is
  `live` or `estimated`; pbrain's ranking already handles both.
- **Identity.** The alias is `vulkan:<index>`, the persistent ID is
  `vulkan:<32 hex>`; `deviceListing` requires unique, backend-prefixed IDs and
  aliases, and `attachDevice` rejects an attached ID that differs from the
  requested one. `--device` accepts either form; `pbrain devices --json` prints
  `id`, `persistent`, `name`, memory and `free_bytes`.
- **Fingerprint.** The model fingerprint is file-based and backend-free;
  `IndexFingerprint` is `SHA-256("pbrain-index\n" + "1\n" + modelFingerprint)`.
  The manifest (format 3) records model, dimension and fingerprint, not a
  device.

## 5. Index and reuse against the cross-backend evidence

The actual rules, read from the tree:

- One index database per section; `manifest` format 3 names the DB, its size and
  CRC-32C, the model, the dimension and the fingerprint.
- `reuse` searches the index by the 32-byte **passage digest** first
  (`baseVectorByDigest`, preferring the asking item's own rows, then
  `vectorByDigest`, any row). The shared section then reads
  `<cache>/vectors/<vectorFingerprint>/<first 2 hex>/<digest>`.
- `vectorFingerprint` is the base model fingerprint, or the CUDA fp16
  fingerprint when an fp16 writer resolved. It is never a device.
- A read vector is accepted only if its CRC, 768-value length, finiteness and
  unit length (within 0.001) pass. There is no device or byte-identity check,
  and no reuse code change is needed for Vulkan.

Consequences, which the tests must state rather than assume:

- A Vulkan sync on an existing CPU- or CUDA-built index reuses those vectors by
  digest and embeds only missing passages; the reverse also holds. One index may
  therefore hold vectors from several qualified devices, as architecture §8.7
  already says. The index stays under one fingerprint.
- Q8 and Q6 have different model and index fingerprints, so they never share an
  index or a vector-cache namespace.
- Cross-device vectors are **not** byte-identical. The measured same-file
  CPU/Vulkan minima are 0.999999999997 (Q8 prepared-ID), 0.999999999998 (Q6
  prepared-ID), 0.9999999999997415 (Q8, 2,655 positions) and 0.9999999999998137
  (Q6, same length). CPU/CUDA Q8 was 0.999999999998416 and CPU/Vulkan on the
  Google pack 0.999999999998413. All sit far above rule 2's 0.999.
  Same-file CPU/Vulkan digests differ (Q8 `ca3593d8…` / `4b229a17…`, Q6
  `dee1d1e5…` / `0ea62a8d…`), so only **same-device layout bits** are a byte
  gate; cross-device agreement is a cosine gate.
- Reuse across backends is thus intended and inside the tolerance already
  admitted. A vector produced on one device may satisfy a passage first written
  by another; the rules and §8.7 allow it, and this lane adds no device to the
  fingerprint.

## 6. Preserving the default model, CUDA fp16 and named devices

- **Default.** `brain.DefaultModel` stays `bge-small-en-v1.5`, applied when
  `semantic.model` is omitted. The registry keeps `bge-small-en-v1.5` first and
  `embeddinggemma-2` second; Q6 is appended as an opt-in row. A CPU-only model
  has `Devices == nil`, so no Vulkan probe or fallback notice appears for the
  default.
- **CUDA fp16.** `CUDAFP16Selector`, `CUDAFP16Fingerprint`, `fp16Resolver`,
  `selectFP16` and `ResolveWriter` are untouched. fp16 still resolves only to a
  CUDA device with compute capability 8.0+ and a linked selector; an explicit
  Vulkan device with `cuda_precision = "fp16"` resolves to the base profile, as
  it does today for non-CUDA devices. Neither GGUF row gains an fp16
  fingerprint, and Vulkan gets no precision writer.
- **Strict named devices.** `namedDevice` still fails `invalid_config` for an
  unknown selector; a named device that fails to probe or attach still fails
  `ErrAccelerator`/`io` and never falls back to the CPU. Only `auto` may fall
  back, with its notice. `attachDevice` still verifies the attached persistent
  ID and memory budget. A CPU-only build has `vulkanCompiled == false`, so Q8
  and Q6 keep rejecting Vulkan exactly as today. `--device` and `PBRAIN_DEVICE`
  keep their validation and precedence, and `PBRAIN_GPU_MEMORY` keeps its
  bounds.

## 7. Fresh SQLite path

Use a fresh brain with no prior index or cache:

```
pbrain --brain <tmp>/brain sync --device vulkan:<uuid> --json
```

It writes `<brain>/.pbrain/index/manifest` and `<16 hex>.db`. Verify read-only,
as the #39 receipt does:

- `PRAGMA quick_check` returns `ok`.
- 32 `item` rows with 32-byte content digests, and 59 `passage` rows.
- Source path and content digest, and passage identity, ordinal, start, stop and
  digest, match a frozen CPU-built control database exactly.
- Every vector is finite, 768-dimensional and unit length; record the minimum
  CPU cosine and the vector SHA-256.
- The trace has `forward`, `sqlite_commit` and `publish` spans; the run has zero
  child swap, zero new host pageout and no device reset; the reported alias and
  persistent ID are the pinned Radeon.
- A void start or execution attempt is discarded and retried once in place under
  the same guards; a second void stops without a published result, and no retry
  enters a measured API call.

A CPU-built control from the same fixture, model and preparation gives the
comparison database. The manifest, DB name and size must match the row.

## 8. Test scope

**Host-only, no device.**

- Row count, Q8 and Q6 identity, file pins and the three fingerprint/folder
  constants; five `GGUFEntries()[0]` sites become name lookups.
- Both build variants: a CPU-only build still rejects Vulkan for Q8 and Q6; a
  `vulkan` build lists and resolves it.
- Release names: Q6 is known and "not on this machine", not "not supported";
  the default model and Google row are unchanged.
- Cross-backend reuse: a CPU-built index plus a Vulkan-authored vector and the
  reverse reuse by digest under one fingerprint; Q8 and Q6 never share a
  namespace; an fp16 index is not read under the base profile, and a Vulkan
  fp16 request resolves to the base profile.
- Preservation: `brain.DefaultModel`; fp16 resolver returns no Vulkan device;
  `namedDevice` still errors for unknown selectors; an attach failure does not
  fall back.

**Vulkan device tests, deferred until the prototype disposal** (`linux && amd64
&& vulkan`, gated by `PBRAIN_TEST_GEMMA2_Q8` or `PBRAIN_TEST_GEMMA2_Q6` and
`PBRAIN_TEST_VULKAN_DEVICE`, with `PBRAIN_GPU_MEMORY=1024` for the 1 GiB budget):

- A sibling of `TestGemmaQ8CUDAByteLayouts`: singleton, packed, reversed and
  17-input-split layouts are identical by bits on Vulkan.
- CPU/Vulkan cosine over the 32 notes/59 passages at 768 and normalized 512,
  every value at least 0.999; log the minimum and its passage.
- Attach identity and live memory match the pinned persistent ID; a 1 MiB budget
  run still embeds; a wrong or absent persistent ID fails loudly.
- Q6 repeats the same checks. Q6's drift against the Google pack is
  quantization, not parity; the same-file rule-2 comparison needs the pinned
  llama.cpp probe and stays an external reference step.

**End to end.** One fresh Q8 Vulkan sync on the 32-note fixture with the §7
checks, then the same for Q6; a CPU-built control per model proves metadata and
source identity. No index or vector cache is shared between models, and a
cross-backend case reuses a CPU index and adds one note on Vulkan.

## 9. Measurement, with no new speed gate

Correctness gates come first; only then one traced diagnostic sync per model.
Timing uses the repository timing lock, one heavy workload at a time, warm
files, a fresh brain/index/cache, explicit `GOMAXPROCS` and CPU affinity, and no
other GPU client. The owner approved a 4.0 load cap for this descriptive sync on
2026-11-23; every other guard is unchanged, and attempts refused under the
earlier 1.0 cap stay refused. Report wall time, committed passages/s, peak host RSS, sampled
device memory, child swap, host pageout, launch load, and the exact binary and
module hashes.

There is no pass/fail threshold and no 2× bar. Report the actual sync even when
it is below 2× the CPU run. The #39 descriptive cells observed about 2.5× on
documents for Q8/Q6, but that was encoder timing, not pbrain indexing; it is
context only. Do not take a CUDA, llama.cpp or cross-machine ratio without that
harness. Sync speed is the only claim: search opens the CPU in every build, so
rule 3 and the 600 ms gate do not change.

## 10. Minimal change list

1. `internal/embed/gemma2/gguf.go`: give Q8 `listDevices` and
   `newRuntime(..., true)` under the Vulkan tag; add the Q6 row.
2. `internal/embed/gemma2/gguf_test.go`: admission tests per build variant,
   updated rejection expectations, Q6 identity.
3. New `gguf_vulkan_test.go`: layout, cosine, attach and 1 MiB budget checks.
4. `internal/semantic/reuse_test.go` or a new test: cross-backend reuse by
   digest, namespace separation, fp16/base non-mixing.
5. `cmd/pbrain/main_test.go` and the calibrate row tests: Q6 known and absent;
   default and fp16 expectations unchanged.
6. New env-gated e2e test or driver for the §7 fresh-index checks.

No `go.mod` change while v0.6.6 stands, no tag, no default-model change, no
change to reuse, fp16 or named-device code, and no product change to goinfer
from this repository.

## 11. Risks and open decisions

- Q8 loads a 512 MiB FP32 token table plus weights; host RSS was about 1.4–1.6
  GiB and CUDA peak about 964 MiB. Vulkan must stay inside the 1 GiB workspace,
  2 GiB Radeon headroom and 8 GiB host guard.
- Vulkan exposes no `RunnableSelectors` and no fp16 profile; the Q8/Q6 Vulkan
  build must not advertise a selector it does not have.
- Reuse trusts the first valid vector for a digest. The tolerance evidence
  supports that across CPU/CUDA/Vulkan, but a future backend or model change
  outside rule 2 would need its own evidence before sharing a fingerprint.
- A device reset or validation error voids the run; no retry may enter a
  measured API call.
- #76 may change the goinfer pin or the projection path. Re-pin and repeat the
  matched control if it ships a release; otherwise stay on v0.6.6.

## 12. What this step does not do

No model load or device run. No default, selector or release-row change. No tag,
push or tracker write. The host implementation lands while #76 runs; the device
tests and publication wait for its measured result in sequence. The pbrain
issue is opened by @pm when the operator starts that phase.
