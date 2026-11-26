# Opt-in Metal batches for Q8_0 (#72)

Based on master `0fa255a` and on goinfer v0.6.9 (`a1333cf2b78d`) read from the module cache: `backend/metal/embeddinggemma2_batch.go`, `embeddinggemma2_ffn.go`, the diff of `embeddinggemma2.go`, `embeddinggemma2/batch_memory.go`, `docs/use-from-go.md` and goinfer PR #99. The pin bump (step 1 of #72, @lead-dev) lands first; this plan starts from its head.

## Decision

1. **The opt-in is an environment variable, `PBRAIN_METAL_BATCH=1`, read by `pbrain sync`.** It takes effect only when the device selector names Metal: `--device metal`, `--device metal:0` or the same in `PBRAIN_DEVICE`. Under `auto`, `cpu`, another device or a model with no Metal path it changes nothing. Unset or empty is off; any value other than `1` is `invalid_config`. Without it, `metal` and `metal:0` stay singleton, and nothing about the default changes.
2. **Attach with `Encoder.UseAccelerator("metal-eg2-batch")` and validate the attachment itself.** Pre-load discovery still lists the singleton row (`TextBatching: false`) and is not changed. A batch attachment must report the system-default Metal device with `TextBatching: true` and the profile `metal-eg2-combined-v1`. There is no fallback: a failed batch attach is the named-device attach error and ends the sync before publication.
3. **The batch route writes the existing combined fingerprint**, `profileFingerprint(q8Fingerprint, "metal-eg2-combined-v1", goinferRevision)`. Turning the variable on or off reuses the same index generation and the same vector cache, with no rebuild. This rests on goinfer's record that batch vectors equal singleton vectors bit for bit, which the M4 gate in step 5 must confirm before anything else is measured. If it fails, stop and report to goinfer; pbrain does not invent a second profile.
4. **Grouping uses the existing packer** (`embedDocuments`, `batchSize`, `embedBatch`) with Metal limits of 4 documents and 2,748 aggregate prepared positions, the existing memory rule and the existing split-on-OOM retry. Output order is input order.
5. **The default is not changed by this work.** The 5,000-note rerun gives the number; the recommendation and the rule for a default are in "After qualification".

## Rejected

- **A device selector such as `metal-batch`.** `--device` takes what `pbrain devices` lists; a route is not a device, the selector pattern admits no hyphen, and `ResolveWriter` and `Listing.Match` would each have to learn the new name. An environment variable is also the easier thing to retire if batching becomes the default.
- **A `pbrain.toml` key.** The file is tracked and shared by every checkout. Batching exists only on a base Apple M4 and does not change vector identity, so it is a property of the machine, like `PBRAIN_DEVICE`.
- **Applying it under `auto`.** `chooseRuntime` treats a failed attach under `auto` as "try the next candidate", which on a Mac is the CPU at about 1 passage/s. An experimental route should not be chosen, or silently abandoned, by `auto`.
- **A fingerprint of its own.** goinfer reports the same profile for the same bytes; a second identity would rebuild indexes to store identical vectors.
- **A window of 4.** It forces a group boundary every four passages (four 900-position passages become 3 + 1, and the 1 cannot join the next window). A window of 256 lets the packer fill groups greedily across the queue, and on the #69 fixture it should reproduce goinfer's 55 forwards.
- **Sorting by length to fill groups.** It reorders work for a gain nobody has measured. Not unless the 5,000 run asks for it.
- **Leaving the caps to goinfer's refusals alone.** `batchSize` would already stop at `ErrUnsupportedBatchShape`, but pbrain states the limits it qualified, as it does for CUDA and Vulkan.

## What goinfer v0.6.9 does (verified in the code)

- `metal-eg2-batch` attaches only on a device named exactly `Apple M4` with architecture `applegpu_g16g`, the 24-layer geometry and the combined attention profile. Anything else, including an M4 Pro or Max, returns `ErrUnsupportedSelector` ("Metal batching requires the base Apple M4 combined text graph"). It never resolves to the stock profile.
- A multi-document call takes 2 to 4 documents and at most 2,748 rows in total; outside that, `BatchMemory` and `ForwardBatch` return `ErrUnsupportedBatchShape`. A single document keeps the model's own limit and runs through the same planned path.
- `BatchMemory` on the batch attachment returns `BatchMemoryApproximate`: the page-rounded bytes of the buffers a call would add to what is already retained, so it is near zero once scratch has grown. An allocation refusal is `ErrBatchOutOfMemory` and leaves the attachment usable for a smaller call.
- The attachment embeds `eg2Accel`, so it also implements `DeviceMemoryReporter` (an estimate for shared memory) and `AttachedProfileReporter`.
- v0.6.9 also changes the **singleton** route on the base M4: the gate and up projections use a new staged kernel for inputs of 31 to 2,042 rows, under the same profile name. goinfer records the words as identical and the gain as about 2.55%. So a v0.6.8 to v0.6.9 difference is not all batching; the three arms in step 5 keep the two apart.

## The issue against the code

- **"Use the singleton path for long inputs."** pbrain cuts every input at 2,048 prepared tokens, under the 2,748 cap, so no input is too long for a group by itself. A "long input" is simply one that cannot be joined by its neighbour, and the packer already sends a group of one through `EmbedIDs`.
- **"Use `BatchMemory` to check fit."** The packer already does, but a Metal attachment has no budget today (`RunInfo.Budget` is 0), which would reject every group. The plan gives the batch route the existing budget rule.
- **The estimate is growth, not a working set.** "Fit" compares the bytes a call adds with the budget. The retained scratch is bounded by the 2,748-row cap, about 324 MB by goinfer's record.
- **"On the base M4" is a hard limit of the selector**, not only where it was measured. Any later default has to handle the refusal on every other Mac.
- **55 forwards** is goinfer's count for its own replay order. pbrain's count depends on passage order; record it, do not gate on it.
- Nothing else in the issue disagrees with the code.

## Interfaces and files

**`internal/cli/devices.go`, `syncRuntimeOptions`.** After `PBRAIN_GPU_MEMORY`: read `PBRAIN_METAL_BATCH`. Empty: nothing. `1`: `opt.MetalBatch = true`. Anything else: `&errs.Error{Code: "invalid_config", Message: "PBRAIN_METAL_BATCH must be 1 when set", Exit: 1}`. The form is checked whatever the device and model are; the value is passed through unconditionally.

**`internal/embed/embed.go`, `OpenOptions`.** Add `MetalBatch bool // opt-in: a named Metal device attaches with the batch selector`. The struct stays comparable.

**`internal/semantic/semantic.go`, `Options`.** Add `MetalBatch bool` after `GPUMemory`. `internal/cli/semantic.go` sets it beside `opt.Device, opt.GPUMemory`; `builder.openEmbedder` copies it into `embed.OpenOptions`. It is not part of the writer profile, the manifest or any fingerprint.

**`internal/embed/gemma2/metal.go`.**

```go
const (
	metalBatchSelector  = "metal-eg2-batch"
	metalBatchInputs    = 4    // goinfer v0.6.9, base Apple M4
	metalBatchPositions = 2748 // aggregate prepared positions of a multi-document call
)

// metalDefault is the identity shared by the inventory row and both attachments.
func metalDefault(d embeddinggemma2.DeviceInfo) bool {
	return d.Backend == "metal" && d.ID == "" && d.Alias == ""
}

func systemDefaultMetal(d embeddinggemma2.DeviceInfo) bool { return metalDefault(d) && !d.TextBatching } // inventory, unchanged

func attachMetal(model *embeddinggemma2.Encoder, batch bool, override int64) (profile string, budget int64, err error)
func attachedMetalProfile(acc embeddinggemma2.Accelerator, batch bool) (string, error)
```

- `attachMetal` calls `UseAccelerator("metal")` or, with `batch`, `UseAccelerator(metalBatchSelector)`. Errors, all wrapping `embed.ErrAccelerator`: `ErrUnsupportedSelector` gives `device metal:0 cannot run selector "metal-eg2-batch": <cause>` (the wording of `attachDevice`); anything else gives `attach metal:0: <cause>`, as today.
- `attachedMetalProfile` requires `AttachedDeviceReporter` with `metalDefault(d) && d.TextBatching == batch`, else `attached device is not the Metal system default` (today's text). For `batch` the profile must be `metalCombinedProfile`; for the singleton it is stock or combined as today. Any other profile: `unrecognized Metal vector profile %q`.
- With `batch`, the budget is the existing rule applied to the attachment's own memory reading. Split `attachedBudget` in two, because its identity check cannot be used for Metal: it requires `AttachedDevice().ID == id`, and a Metal attachment has an empty ID while pbrain's `metal:0` is a synthetic alias.
  - `attachedBudget(acc, id, override)` keeps the ID equality check for CUDA and Vulkan, then returns `deviceBudget(acc, id, override)`. Its behaviour and error texts do not change.
  - `deviceBudget(acc, label, override) (int64, error)` is the memory half only: read `DeviceMemoryReporter` when the accelerator has one, and return `memoryBudget(memory, override)`. It compares no identity. `label` is used only in the error text `read memory of <label>`.
  - `attachMetal` calls `attachedMetalProfile(acc, batch)` first, which is the Metal identity check, and only then, with `batch`, `deviceBudget(acc, metalDeviceID, override)`. It never calls `attachedBudget`.
  - With goinfer's estimated reading the budget is 30% of the estimate up to 1 GiB, and `PBRAIN_GPU_MEMORY` overrides it as for other devices. The singleton keeps budget 0, so its `embedding_device` notice does not change. The batch route's notice reads `metal:0 (Apple M4), 1024 MiB a batch`, which is how a run shows which route it took.
- A rejected attachment is closed through `closeRejectedAttachment`, as today.

**`internal/embed/gemma2/gemma2.go`.**

- `encoder` gains `metalBatch bool`.
- In `newRuntime`'s attach closure: `batch := opt.MetalBatch && selector == metalDeviceID` (the registry has already resolved `metal` to `metal:0`; under `auto` the selector is `auto`, so the opt-in is ignored), then `metalProfile, budget, err = attachMetal(candidate, batch, opt.MemoryBudget)`. Set `e.metalBatch` when the attach succeeded with `batch`. `e.cpu` stays nil for Metal: no CPU fallback on either route.
- `Window()` returns 1 for `metal:0` only when `!e.metalBatch`; the batch route returns `batchInputs` (256).

**`internal/embed/gemma2/batch.go`, `batchSize`.** Same signature. When `info.Device == metalDeviceID` the input cap is `metalBatchInputs` and the position cap `metalBatchPositions` in place of `batchInputs` and `batchPositions`, in the slice bound, the position test and the final return. Everything else is as it is: `ErrUnsupportedBatchShape` after the first input ends the group; an `Approximate` estimate is compared with the budget; an `Unknown` one falls back to `bytesPerPosition` (136,000, above goinfer's recorded 117,955 a position for this path). `embedBatch`'s retry on `ErrBatchOutOfMemory` and its `forward` span (inputs, total and longest positions) need no change.

**Error cases, as a user sees them.**

| Case | Result |
| --- | --- |
| `PBRAIN_METAL_BATCH=yes` | `invalid_config: PBRAIN_METAL_BATCH must be 1 when set`, exit 1, before any scan |
| `=1`, `--device metal`, not a base M4 | `io: the model cannot be opened: … device metal:0 cannot run selector "metal-eg2-batch": …`, exit 1, nothing published |
| `=1`, `--device metal`, a build without Metal or a model other than Q8_0 | today's result for that selector; the variable adds nothing |
| `=1` with `auto`, `cpu` or no selector | today's behaviour; singleton Metal if `auto` picks Metal |
| a group's allocation is refused | the group is split and retried (existing rule); a refused single input fails the section at its last checkpoint |

## Steps

Each step is one reviewable change on the developer's branch.

**1. Adapter.** `metal.go`, `gemma2.go`, `batch.go` as above. Tests, all on Linux with fakes:
- `TestMetalListingAndProfile` (extend): `attachedMetalProfile(acc, true)` accepts the default identity with `TextBatching: true` and the combined profile, and gives the combined fingerprint; it rejects the stock profile, `TextBatching: false`, a non-default ID and a missing reporter. `attachedMetalProfile(acc, false)` still rejects `TextBatching: true`. The inventory still drops a Metal row that reports batching.
- `TestGemmaPacker` (new rows, device `metal:0`, budget 1 GiB): lengths `500×5` give groups `4, 1`; `1000, 1000, 1000, 10` give `2, 2`; `2048, 2048, 700` give `1, 2` (2,748 exactly is admitted); `700, 2048, 1` give `2, 1` (2,749 is over the cap). One row with a budget under the estimate gives singletons.
- `TestMetalBatchDispatch` (new, beside `TestMetalSingletonDispatch`, a `fakeBatchAccel` under a test name, `e.metalBatch = true`, budget 1 GiB): `Window()` is 256; six documents run as batches of at most four and the vectors come back in input order; `ErrBatchOutOfMemory` on a batch splits it and still returns every vector; `ErrUnsupportedBatchShape` from the forward is an `embed.ErrAccelerator`; the device stays `metal:0` throughout.
- `TestMetalBatchAttachError` (new): `attachMetal(tiny, true, 0)` fails with `embed.ErrAccelerator` and a message naming `metal-eg2-batch` (goinfer's "no accelerator in this binary"). Skip it when `metalCompiled` is true: with the real backend linked the call would open the GPU, which hosted CI must not do.
- `TestAttachedBudget` (extend): `deviceBudget` gives the budget for an accelerator whose attached ID is empty, with and without a memory reporter and with an override, and returns the reporter's error under the label; `attachedBudget` still rejects an attached ID that differs from the selector.
- `TestMetalSingletonDispatch` passes unchanged.

**2. Plumbing.** `embed.OpenOptions`, `semantic.Options`, `builder.openEmbedder`, `cli`. Tests:
- `TestSyncDeviceOptions` (new subtests): `PBRAIN_METAL_BATCH=1` reaches the opener as `MetalBatch: true` with the resolved device; `0`, `true` and `yes` fail with the `invalid_config` text and no open; a project on the default model syncs normally with the variable set.
- `TestMetalIndexProfiles` (extend): with a fake embedder that returns the combined fingerprint, a sync with `MetalBatch` switched on over an index built with it off embeds nothing and keeps the generation, and the reverse; the manifest carries no trace of the setting.

**3. Device test.** New `internal/embed/gemma2/gguf_metal_test.go`, build tag `darwin && arm64 && metal`, `TestGemmaMetalBatchLayouts`. Widen the tag of `batch_fixture_test.go` with `|| (darwin && arm64 && metal)` and move `layoutReceipt` there. It skips unless `PBRAIN_TEST_GEMMA2_Q8` and `PBRAIN_ALLOW_GEMMA2=1` are set (the existing `fixtureDir` gate), so hosted CI never runs it; on a Mac that is not a base M4 it fails with the attach error, which is correct.
- Texts: the 59-passage fixture, or every `.md` note under `PBRAIN_TEST_BATCH_NOTES` when that is set (same parsing, no exclusions, no fixed count), so the M4 run can use the 219-passage corpus.
- Open the CPU, the singleton route (`Device: "metal:0"`) and the batch route (the same with `MetalBatch: true`). Assert: windows 1 and 256; budgets 0 and above 0; equal fingerprints, both the row's combined Metal fingerprint; the attachments report `TextBatching` false and true.
- Reference bits are the singleton route, one text a call. On the batch route compare, by bits, at 768 values and at the normalised first 512: each text alone, all texts in one call, chunks of 3, and the reversed order restored. Every count of differing vectors must be 0.
- CPU against the singleton route: cosine at least 0.999 at both widths.
- Run the batch route's calls under a `trace.Run` and read the `forward` spans back: the one-call layout has at least one span with `inputs` above 1, none above 4, none with `inputs` above 1 and `total_positions` above 2,748, and the inputs sum to the passage count. Without this the test could pass without batching.
- With `PBRAIN_METAL_LAYOUT_DIR` set, write the receipt (passages, differing counts by layout, cosine minima, forward count and largest group).

**4. Spec text, with steps 1 to 3.** `components/semantic.md`: in "Devices", the paragraph that says the Metal row has "no batch capability" and "a window of one" gains the opt-in route (variable, named selectors only, attach validation, limits 4 and 2,748, window 256, budget rule, no fallback); in "Metal indexing profiles (#69)", one sentence that the batch route writes the combined profile and shares its index and cache; the test plan rows for the tests above. State there that the route works on a base Apple M4 only. `functional_spec.md` and `README.md` are not touched (operator's decision).

**5. M4 qualification** (@lead-dev, existing Mac rules: idle window, one heavy run at a time, one scratch directory with `GOCACHE`, `GOMODCACHE`, `TMPDIR` and `PBRAIN_CACHE` under it, clean up, keep the evidence, no settings changed). Cgo-free `-tags metal` builds, `GOMAXPROCS=4`, the pinned Q8_0 GGUF verified in every fresh sync, `PBRAIN_TRACE` on, `/usr/bin/time -l` around each sync.

Gate first, before any timing: `TestGemmaMetalBatchLayouts` on the 59-passage fixture and again with `PBRAIN_TEST_BATCH_NOTES` on the #69 corpus (22 notes, 219 passages, 110,157 positions). Any differing vector stops the work (decision 3).

Then three arms on the #69 corpus, each a fresh brain copy with an empty index and its own empty `PBRAIN_CACHE`:

| Arm | Binary | Command |
| --- | --- | --- |
| A, v0.6.8 singleton | master `0fa255a` | `pbrain sync --device metal` |
| B, v0.6.9 singleton | this work's head | `pbrain sync --device metal` |
| C, v0.6.9 batch | the same binary as B | `PBRAIN_METAL_BATCH=1 pbrain sync --device metal` |

Run A, B, C and then C, B, A. Record for each run: wall time, passages/s, the verify, decode, attach, forward and SQLite splits, the number of forwards and each one's inputs and positions, the `pack` span total, and the peak footprint and RSS. Also build one CPU index with the B binary (`--device cpu`) as the same-file reference.

Checks, with the #69 verifier (`tools/phase28/evidence/metal-69/verify-m4.py`) extended into `tools/phase28/evidence/metal-batch-72/`:
- SQLite `quick_check` passes on every index, and ordered item and passage metadata are identical across all seven.
- B and C: all 219 stored vectors equal by bits at 768 and at the normalised first 512, in both repetitions, and the two manifests carry the same fingerprint.
- A, B and C against the CPU index: minimum cosine at least 0.999 at both widths. Record how many vectors differ between A and B; goinfer's record predicts none, and it is not a gate.
- The eight frozen queries of #69 return the same ordered IDs on A, B, C and the CPU index.
- In C every forward has at most 4 inputs, every multi-input forward at most 2,748 positions, and inputs and positions sum to 219 and 110,157 in every arm.
- Reuse on the device: on C's finished index, a sync without the variable attaches once, runs no forward and keeps the generation; then the same the other way.

**6. The 5,000-note corpus, only if step 5 passes.** One fresh full sync on the #22 corpus for C and then one for B, each in its own idle window, followed on C's index by the one-note increment and the warm `search --semantic` timing under the 1 s target. One pair is enough (operator's decision); a difference under 3% is reported as not distinguishable. Then the documents: a "#72" section in `internal/cli/README.measure.md` with the tables and the evidence folder, saying the variable works on a base Apple M4 only; `docs/release-notes/next.md`. `README.md` and the functional spec do not mention the variable while the route is limited to that one device (operator's decision): it is documented in `components/semantic.md` "Devices" and in `README.measure.md` only.

## After qualification

goinfer measured 1.085× for batches over v0.6.9 singletons at the library level, and the staged kernel about 1.026× over v0.6.8. The end-to-end figure should be near those, far from the 2× that goinfer #92 named as realistic for kernel work, which v0.6.9 has not done.

The default stays singleton in #72 (operator's decision). The bar for a later default, which is #73's scope: every step 5 check passed, the 5,000-note gain is at least 5% end to end, and peak footprint rises by no more than 300 MB. That is a separate change, because a default needs what the opt-in deliberately lacks: on `ErrUnsupportedSelector` it must fall back to the singleton selector, so that every other Mac keeps working, and `auto` must follow the same rule.

## Risks and gaps

- **Nothing here was run.** Every statement about goinfer is from reading v0.6.9's source and PR #99; the bit equality the fingerprint decision rests on is goinfer's record until step 5.
- **A window of 256 on a slow device.** At about 10 passages/s a window is about 25 s of work between stores, against one passage today. A cancelled or failed sync redoes at most the window in flight.
- **Memory.** goinfer recorded about 887 MB of tracked buffers for the batch route against 659 MB for the singleton. Process footprint on the M4 is measured in step 5, not known.
- **No hosted coverage of the real route.** GitHub's macOS runners cannot run it; steps 1 and 2 cover pbrain's logic with fakes, and the route itself is covered only by step 3's test, run on the owner's M4 in step 5.
- **One machine class.** The selector refuses everything but a base M4, so no other Mac can show a problem or a gain.
- **`PBRAIN_METAL_BATCH` under `auto` is silently ignored.** The notice shows the route (a batch budget appears only on the batch route), and the protocol always names the device.
