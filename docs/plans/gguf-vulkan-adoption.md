# Adopt EmbeddingGemma 2 GGUF on Vulkan

Plan for the pbrain lane after the goinfer Vulkan projection prototype. It maps
the current Q8_0 gate, the absent UD-Q6_K_XL row, the device/model identity, and
the fresh-index test path, then states the smallest test-only change. There is no
code or device work in this step, and no default changes. The reviewed goinfer
prototype result comes first; this lane reuses goinfer v0.6.6 unless that result
ships a release, in which case the pin moves and the matched control repeats.

Start point: pbrain `master` `8a7b603` (merge #64, per-device CUDA selector
eligibility) with goinfer root, CUDA and Vulkan at v0.6.6,
`c0a5159bbff1c2fe5c8624e1399aa7438c820677`. `FACTORY.md` is this repository's
only instruction file; there is no `AGENTS.md` or `CLAUDE.md` here. Work happens
in an isolated worktree because `master` has unrelated dirty release notes.

## 1. Fixed inputs

- Q8_0: `embeddinggemma-2-Q8_0.gguf`, 309,855,520 B,
  `6f1bd4ac6c5df7444f9cca7ca36cafe6cfa34cd6f49fefb1e0b4be8143aed8bc`.
- UD-Q6_K_XL: `embeddinggemma-2-UD-Q6_K_XL.gguf`, 248,812,032 B,
  `dc84f042bc3ffbe6122b3b38b2fb299d66f5f37b276c9d458db85d4a4f1e6ba3`.
- Both are from `unsloth/embeddinggemma-2-GGUF` revision
  `031f0d4b35536f69ab3509d4893c923264fcf253`; goinfer's `pull` manifest pins
  both and `internal/gguf` decodes Q6_K.
- The frozen 32-note/59-passage fixture is
  `internal/semantic/calibrate/testdata/retrieval/brain`, the one
  `batchFixture` uses.
- Reuse the #49 Vulkan method (same-device layout bits, CPU/Vulkan cosine) and
  the #51 Q8 gates (CPU/CUDA cosine, byte identity, retrieval at 0.72). The
  #39 pack-only sync is the pattern for a fresh-SQLite receipt, not Q8 evidence.

## 2. Q8_0's current Vulkan gate

The gate is in pbrain, not in goinfer's Vulkan backend:

- `internal/embed/gemma2/gguf.go` gives the Q8 row `Devices: listCUDADevices`,
  and `openQ8` calls `newRuntime(..., false)`.
- `internal/embed/gemma2/listing.go` has `listCUDADevices()` call
  `probeDevices(false)` and `deviceListing` admit only `cuda`; `listDevices()`
  with `probeDevices(true)` is the Vulkan-enabled path the Google pack uses.
- `internal/embed/embed.go` resolves a named selector against the row's
  `Devices` listing through `namedDevice`, so a Vulkan selector is
  `invalid_config` before a byte is decoded. `TestQ8RejectsVulkan` pins that for
  `vulkan`, `vulkan:0` and the persistent UUID.
- `internal/embed/gemma2/accel_vulkan.go` registers the backend only under the
  `linux && amd64 && vulkan` tags.

So enabling Q8 on Vulkan is two flips on the row (the listing and the runtime
flag), the same shape the Google row already uses, not a new backend. The attach
path itself is already proven by `TestGemmaVulkan`: `attachDevice` defaults the
selector to the backend name, so Vulkan attaches with `vulkan`, and
`AttachedDeviceReporter`/`DeviceMemoryReporter` supply identity and budget.

## 3. The missing Q6 manifest

There is no Q6 model manifest (release row) in pbrain.
`GGUFEntries()` returns one row, Q8; `cmd/pbrain/main.go`,
`internal/semantic/calibrate/main.go` and five test files index `[0]`.
`cmd/pbrain/main_test.go` expects `embeddinggemma-2-ud-q6-k-xl` to be
"not supported by this pbrain". goinfer already pins and decodes Q6_K, so the
missing artifact is pbrain's row, identity and release wiring, not a loader.

Proposed row: name `embeddinggemma-2-ud-q6-k-xl`, runtime `goinfer`, dimension
768, cutoff 0.72 (the #47 fitted value; owner admission still decides), one
`verityFile`, no `Source` and no `Derive`, and the same device policy chosen for
Q8. Its identity by the existing recipe:

| Field | Value |
| --- | --- |
| Model fingerprint | `16d5093f31a4399589c717804e04d2ad3fd70df5bd42b36bf60424dc4599e2f4` |
| Index fingerprint | `8c964b1108dc4e42f65ea44837ce518911dc462ccb57d8fe1eba862a7e98bfa7` |
| Cache folder | `embeddinggemma-2-ud-q6-k-xl-22e1f6123853c8cb` |

Do not add a `CUDAFP16Fingerprint` to either GGUF row; the fp16 writer is a
CUDA-only profile. Q6 has no admission decision; this lane prepares the row and
its tests but does not make Q6 selectable without the owner's word.

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
  The index manifest (format 3) records model, dimension and fingerprint, not a
  device, and architecture §8.7 allows one index to hold vectors from several
  qualified devices. Adding Vulkan therefore does not invalidate a CPU or CUDA
  index; it must not create a new namespace. Same-device vector bits are still
  required; CPU/Vulkan bytes need only meet rule 2.

## 5. Fresh SQLite path

Use a fresh brain and no prior index or cache:

```
pbrain --brain <tmp>/brain sync --device vulkan:<uuid> --json
```

It writes `<brain>/.pbrain/index/manifest` and `<16 hex>.db`. Verify read-only,
as the #39 receipt does:

- `PRAGMA quick_check` returns `ok`.
- 32 `item` rows with 32-byte content digests, and 59 `passage` rows.
- Source path and content digest, and passage identity, ordinal, start, stop and
  digest, match a frozen CPU-built control database exactly.
- Every vector is finite, 768-dimensional and unit length; minimum CPU cosine is
  at least 0.99999 for the Q8 e2e fixture; record the vector SHA-256.
- The trace has `forward`, `sqlite_commit` and `publish` spans; the run has zero
  child swap and zero new host pageout; the selected alias and persistent ID are
  the pinned Radeon.

A CPU-built control from the same fixture and preparation gives the comparison
database. The manifest, DB name and size must match the model row.

## 6. Test scope

**Host-only, no device.** Row count and identity (Q8 and Q6), two gates per
build tag (CPU-only builds still reject Vulkan; a `vulkan` build lists and
resolves it), Q6 release names and "not on this machine" status, calibrate's
`PBRAIN_ALLOW_GEMMA2=1` guard, and the fingerprint/folder constants. Update the
five `GGUFEntries()[0]` sites to look up by name.

**Vulkan device tests** (`linux && amd64 && vulkan`, gated by
`PBRAIN_TEST_GEMMA2_Q8` and `PBRAIN_TEST_VULKAN_DEVICE`, with
`PBRAIN_GPU_MEMORY=1024` for the default 1 GiB budget):

- A sibling of `TestGemmaQ8CUDAByteLayouts`: singleton, packed, reversed and
  17-input-split layouts must be identical by bits on Vulkan.
- CPU/Vulkan cosine over the 32 notes/59 passages at 768 and normalized 512,
  every value at least 0.999; log the minimum and its passage.
- Attach identity and live memory match the pinned persistent ID; a 1 MiB budget
  run still embeds.
- Q6 gets the same file once its row exists, gated by `PBRAIN_TEST_GEMMA2_Q6`.
  Its drift against the Google pack is quantization, not parity; the same-file
  rule-2 check needs the external llama.cpp reference and stays an owner step.

**End to end.** A fresh brain, one `sync` on the 32-note fixture, then the
read-only SQLite checks in §5. A second, CPU-built control proves metadata and
source identity match. No index or vector cache is shared between arms.

## 7. Measurement boundary

Correctness gates come first; only then one traced diagnostic sync. Timing uses
the repository timing lock, one heavy workload at a time, warm files, a fresh
brain/index/cache, explicit `GOMAXPROCS` and CPU affinity, and no other GPU
client. Record wall time, committed passages/s, peak host RSS, sampled device
memory, child swap, host pageout, launch load and the exact binary and module
hashes. A single traced run is diagnostic; repeat only if a decision needs it.

A worthwhile gain, as a proposal for the owner: every correctness gate passes
and a paired Q8 Vulkan sync on the 32-note fixture is at least 2× the CPU sync
with no swap, pageout or device reset. The #49 Google-pack slice saw 2.33× CPU,
so the bar is reachable but is not assumed. Sync speed is the only claim: search
opens the CPU in every build, so rule 3 and the 600 ms gate do not change. No
CUDA, llama.cpp or cross-machine ratio is taken without that harness.

## 8. Minimal change list, only after lane 1

1. `internal/embed/gemma2/gguf.go`: give Q8 `listDevices` and
   `newRuntime(..., true)` under the Vulkan tag; add the Q6 row.
2. `internal/embed/gemma2/gguf_test.go`: admission test under the tag, updated
   rejection expectations, and Q6 identity.
3. New `gguf_vulkan_test.go`: layout, cosine, attach and 1 MiB budget checks.
4. `cmd/pbrain/main_test.go` and the calibrate row tests: Q6 becomes a known,
   absent model; keep Q6 unselectable absent the owner's decision.
5. New env-gated e2e test or driver for the fresh SQLite checks in §5.

No `go.mod` change while v0.6.6 stands, no tag, no default-model change, no
product change to goinfer from this repository.

## 9. Risks and open decisions

- Q8 loads a 512 MiB FP32 token table plus weights; host RSS was about 1.4–1.6
  GiB and CUDA peak about 964 MiB. Vulkan must stay inside the 1 GiB workspace,
  2 GiB Radeon headroom and 8 GiB host guard.
- Vulkan exposes no `RunnableSelectors` and no fp16 profile; a Q8 Vulkan build
  must not advertise a selector it does not have.
- CPU/Vulkan vectors may differ in the last bits under one fingerprint; do not
  require cross-device byte identity, only same-device bits and rule-2 cosine.
- Q6 admission, cutoff and any release row are the owner's decision. Q6's
  same-file rule-2 comparison needs the pinned llama.cpp probe, which this lane
  does not run.
- A device reset or validation error voids the run; no retry may enter a
  measured API call.

## 10. What this step does not do

No code, build, model load or device run. No default, selector or release-row
change. No tag, push or tracker write. The pbrain issue is opened by @pm when
the operator starts this lane, after the goinfer prototype result.
