# Opt-in Metal batches for Q8_0 (#72)

Based on master `0fa255a` and on goinfer v0.6.9 (`a1333cf2b78d`) read from the module cache: `backend/metal/embeddinggemma2_batch.go`, `embeddinggemma2_ffn.go`, the diff of `embeddinggemma2.go`, `embeddinggemma2/batch_memory.go`, `docs/use-from-go.md` and goinfer PR #99. The pin bump (step 1 of #72, @lead-dev) lands first; this plan starts from its head.

**Superseded in part (2026-11-26).** The owner made batching the default and folded #73 into this issue. The [addendum](#addendum-batching-is-the-default-2026-11-26) at the end replaces decisions 1, 2 and 5, the rejection of "Applying it under `auto`", the 5,000-note pair of step 6 and "After qualification". The rest stands.

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

## Addendum: batching is the default (2026-11-26)

Owner's decision: Metal batching is the default wherever goinfer supports it. Faster is enough and the extra memory is accepted. #73 is folded into #72. The M4 is reserved for goinfer #92, so this change is built and tested with fakes only and no new device run is made. It is written against `peter/lead-dev` at `b2f4e92`, where steps 1 to 4 are implemented.

### The evidence it rests on

These are @lead-dev's M4 runs with the opt-in binary at `b2f4e92`, as reported; I did not rerun them.

- **Same vectors.** On 59 and 219 passages the batch route matched the singleton route bit for bit at 768 values and at the normalised first 512, in every layout. The 219 stored vectors of the batch and singleton indexes were identical.
- **219 passages, nine interleaved runs.** v0.6.9 singleton 9.443 passages/s, batch 10.230: 8.34% more. Median wall time 23.193 s against 21.408 s. Peak footprint 2,129.8 MB against 2,333.2 MB: 203.4 MB more.
- **5,000 notes, one batch run.** 4,864.81 s at 9.97 passages/s, against 5,288.0 s at 9.17 for the v0.6.8 singleton.

The v0.6.9 singleton run at 5,000 notes was stopped, so the pair of step 6 does not exist and the full-size comparison includes the staged kernel's gain. The bar in "After qualification" (5% on that pair, 300 MB) is replaced by the owner's decision on the figures above.

### Decision

1. **Every Metal attachment of a Q8_0 sync asks for `metal-eg2-batch` first**, whether `auto`, `metal` or `metal:0` selected the device. When goinfer accepts, the sync runs the batch route exactly as steps 1 to 3 built it: window 256, groups of at most 4 documents and 2,748 positions, the budget rule, the split on OOM.
2. **A refusal is not an error.** The adapter then attaches with `UseAccelerator("metal")` and runs the singleton route as v0.4.6 does: window 1, no budget, stock or combined profile as the device reports. Two things count as a refusal:
   - `UseAccelerator("metal-eg2-batch")` returns an error that is `embeddinggemma2.ErrUnsupportedSelector`. In goinfer v0.6.9 that is every Mac but a base Apple M4.
   - It attaches, but `attachedMetalProfile(acc, true)` rejects the attachment: no `TextBatching`, a profile other than `metal-eg2-combined-v1`, or not the system default. v0.6.9 never does this; the rule keeps pbrain from batching on something it has not qualified. The attachment is closed first.
3. **Any other failure of the batch attach is the device's attach error, as today.** No fallback. Up to the point where goinfer refuses, the batch and plain selectors run the same code (device open, pipeline build, matrix set-up), and after it the same weight upload. So an error that is not a refusal would fail the singleton attach too, and retrying would only hide it.
4. **The usual device notice applies.** When sync embeds on Metal, it emits `embedding_device`. A refusal followed by successful singleton attachment adds neither a refusal notice nor `cpu_fallback`. The budget suffix tells the routes apart: `metal:0 (Apple M4), 1024 MiB a batch` on the batch route, `metal:0 (<name>)` on the singleton route.
5. **`PBRAIN_METAL_BATCH` stays, as an opt-out.** `PBRAIN_METAL_BATCH=0` keeps every Metal sync on the singleton route. Unset or empty is the default. Any other value, `1` included, is `invalid_config`. No explicit opt-in remains, so a refusal is never an error.
6. **The fingerprint does not change.** The batch route reports `metal-eg2-combined-v1` and writes the combined fingerprint. Upgrading, falling back and `PBRAIN_METAL_BATCH=0` all reuse the same index and vector cache on a base M4. On other Macs the profile and fingerprint are what the singleton reports, as before.

| Selection | Base Apple M4 | Any other Apple Silicon Mac | With `PBRAIN_METAL_BATCH=0` |
| --- | --- | --- | --- |
| `auto` choosing Metal, `metal`, `metal:0` | batch route | singleton route, usual `embedding_device` notice when embedding | singleton route |

After the route is chosen nothing changes: a failed attach under a named selector is `io` and ends the sync before publication; under `auto` it moves to the next candidate, which on a Mac is the CPU with `cpu_fallback`. A Metal forward failure stops the section at its last checkpoint on either route.

### Rejected

- **Dropping the variable.** A fair comparison of the two routes needs one binary, which is how the 219-passage recheck was made and how goinfer #92's work will be judged. It is also the way back for a base M4 where the extra 200 MB or the 256-passage window is a problem, at no cost in rebuilds.
- **Keeping `=1` as a strict opt-in that fails on a refusal.** Nothing needs it: the device test asserts the route it opened, and a measurement reads the route from the notice and the `forward` spans. It would also bring back the hazard under `auto`, where a failed attach means the CPU.
- **Choosing the route from the inventory row's name.** That copies goinfer's hardware rule (name, architecture and the staged kernel) into pbrain, to go stale at the next pin.
- **Falling back on any batch attach error.** See decision 3.

### Changes against `b2f4e92`

**`internal/cli/devices.go`, `syncRuntimeOptions`.** `""`: nothing. `"0"`: `opt.MetalSingleton = true`. Anything else: `invalid_config`, `PBRAIN_METAL_BATCH must be 0 when set`, exit 1, before any scan and whatever the device and model are.

**`internal/embed/embed.go`.** Replace `MetalBatch` with `MetalSingleton bool // PBRAIN_METAL_BATCH=0: Metal stays on one passage a forward`. The zero value has to be the default, because every other caller builds `OpenOptions` without it.

**`internal/semantic/semantic.go`, `build.go`, `internal/cli/semantic.go`.** The same rename: `Options.MetalSingleton`, copied through `openEmbedder`. Still no part of the writer profile, the manifest or a fingerprint.

**`internal/embed/gemma2/metal.go`.**

```go
func attachMetalRoute(use func(selector string) (embeddinggemma2.Accelerator, error), batch bool, override int64) (profile string, budget int64, batched bool, err error)
```

In order:

1. Without `batch`, go to 4.
2. `use(metalBatchSelector)`. An error that is `ErrUnsupportedSelector`: go to 4. Any other error: `attach metal:0: <cause>` wrapping `embed.ErrAccelerator`, as today. The `cannot run selector` message goes away.
3. `attachedMetalProfile(acc, true)`. If it passes, `deviceBudget(acc, metalDeviceID, override)` and return `(profile, budget, true, nil)`; a budget error closes the attachment through `closeRejectedAttachment` and is returned, with no fallback. If it fails, close the attachment: a failed close returns the `attachCleanupError` that `closeRejectedAttachment` builds, which ends the open under `auto` too; a clean close goes to 4.
4. `use("metal")` and `attachedMetalProfile(acc, false)`, as today, returning `(profile, 0, false, err)`.

Both calls use the same encoder. goinfer's `UseAccelerator` leaves the encoder unchanged on an error and replaces the accelerator on success (`embeddinggemma2/accel.go`, v0.6.9).

**`internal/embed/gemma2/gemma2.go`, `newRuntime`.** `batch := !opt.MetalSingleton`, with the `selector == metalDeviceID` condition removed so that `auto` follows the same rule. Pass the candidate’s `UseAccelerator` call to `attachMetalRoute` with that flag and the budget override. Set `e.metalBatch` to the returned route. Inject the Metal attach call into the runtime helper so the runtime test can capture actual selector requests.

**Not changed.** The inventory and `pbrain devices` (the row still reports no batching), `ResolveWriter`, `Window`, `batchSize`, `embedBatch`, the caps and the budget rule.

| Case | Result |
| --- | --- |
| `PBRAIN_METAL_BATCH=1`, `yes` or `true` | `invalid_config: PBRAIN_METAL_BATCH must be 0 when set`, exit 1, before any scan |
| goinfer refuses the batch selector | singleton route, exit 0, usual `embedding_device` notice when embedding |
| the batch attach fails another way, named selector | `io: … attach metal:0: <cause>`, nothing published |
| the same under `auto` | the next candidate: the CPU with `cpu_fallback` |
| the singleton attach fails after a refusal | today's singleton error; it does not mention the batch selector |
| `PBRAIN_METAL_BATCH=0` with another device, model or build | nothing |

### Steps

**A. Code and tests, one change.** All tests run on Linux with fakes.

- `TestMetalAttachRoutes` (new, `metal_test.go`), over `attachMetalRoute` with a `use` that records its selectors and returns fake attachments:
  - accepted: one call, `metal-eg2-batch`; combined profile, a budget above 0, `batched` true;
  - refused with a wrapped `ErrUnsupportedSelector`: calls `metal-eg2-batch` then `metal`; the singleton's profile (one row stock, one combined), budget 0, `batched` false, no error;
  - the batch selector attaches without `TextBatching`, and in another row with the stock profile: that attachment is closed once, then `metal` is attached; result as the row above;
  - the same with a failing close: an `attachCleanupError`, and `metal` is never asked;
  - the batch selector fails with `ErrDeviceOutOfMemory`: `embed.ErrAccelerator` wrapping the cause, and `metal` is never asked;
  - an accepted batch attachment whose memory reading fails: the `read memory of metal:0` error, the attachment closed, `metal` never asked;
  - refused, and `metal` fails too: the singleton's `attach metal:0` error, without `metal-eg2-batch` in it;
  - `batch` false: one call, `metal`.
- `TestMetalBatchAttachError` (keep): a build without Metal answers `ErrBackendUnavailable`, which is not a refusal, so the error still names `metal-eg2-batch`. Add that calling the singleton route fails without naming it.
- `TestMetalRuntimeRoute` (new): with a fake Metal inventory and attach call, check the actual selector requests for explicit and automatic selection. Both try `metal-eg2-batch` by default and request only `metal` with `MetalSingleton`. Under `auto`, a refusal proceeds to singleton Metal. Check the resulting device, budget, window and combined fingerprint.
- `TestSyncDeviceOptions` (rework): `PBRAIN_METAL_BATCH=0` reaches the opener as `MetalSingleton: true` under `--device metal` and under `auto`; unset reaches it as false; `1`, `true` and `yes` fail with the new text and no open; a project on the default model syncs normally with `0` set.
- `TestMetalIndexProfiles` (rework the two toggle rows): a sync with `MetalSingleton` set over a combined index built without it embeds nothing and keeps the generation, and the reverse.
- `TestMetalBatchDispatch`, `TestMetalSingletonDispatch`, `TestGemmaPacker`, `TestAttachedBudget` and `TestMetalListingAndProfile` pass unchanged.
- `TestGemmaMetalBatchLayouts` (`gguf_metal_test.go`): open the singleton route with `MetalSingleton: true` and the batch route with the default options. It cannot run until the M4 is free; the macOS CI job that vets the `metal` build must still compile it.

**B. Text, with A.**

- `components/semantic.md`, "Devices". The inventory paragraph ends at "It has no persistent UUID. Only Q8_0 can select Metal, including under auto." The opt-in paragraph becomes: "Whether `auto`, `metal` or `metal:0` selected it, a Metal attachment first calls `Encoder.UseAccelerator("metal-eg2-batch")`, which goinfer v0.6.9 accepts on a base Apple M4 only. The batch route is used when the attachment reports the system-default device, `TextBatching: true` and `metal-eg2-combined-v1`. When goinfer refuses the selector with `ErrUnsupportedSelector`, or the attachment does not report all three and has been closed, the adapter attaches with `Encoder.UseAccelerator("metal")` instead. That singleton route validates the system-default device and a stock or combined profile, and uses a window of one with `EmbedIDs`. On successful singleton fallback, sync continues and emits the usual `embedding_device` notice when it embeds, without a batch budget. A refusal adds neither a separate notice nor `cpu_fallback`. Any other attach failure is the device's attach error. `PBRAIN_METAL_BATCH=0` skips the batch selector. Empty is the default, and any other value yields `invalid_config` before scanning, whatever the device and model." The paragraph on windows, caps and budget stays.
- The same file: "The opt-in Metal batch route" becomes "The Metal batch route"; the `Options` listing shows `MetalSingleton bool // PBRAIN_METAL_BATCH=0`; the test-plan row names the tests of step A and says "falls back to the singleton route on a refusal" in place of the opt-in sentence.
- `functional_spec.md` 9.4, after "Auto can choose Metal too.": "On a base Apple M4 the sync embeds up to four passages in one pass on the GPU; other Apple Silicon Macs embed one at a time. The vectors and the index are the same either way." In the paragraph on environment variables (the one naming `PBRAIN_GPU_MEMORY`): "`PBRAIN_METAL_BATCH=0` keeps a Metal sync at one passage at a time."
- `architecture.md`: the dependency row for `backend/metal` ends "for Q8_0: up to four passages a forward on a base Apple M4, one at a time on other Macs."; "Batches" gains "On Metal a base Apple M4 packs up to 4 texts and 2,748 positions a forward, and other Macs run one text a forward."
- `README.md`: "Metal runs one passage at a time." becomes "On a base Apple M4, Metal sync batches up to four passages; other Apple Silicon Macs run one passage at a time. `PBRAIN_METAL_BATCH=0` turns the batching off."
- `docs/release-notes/next.md`, a new entry: "**Metal sync batches on a base Apple M4.** `embeddinggemma-2-q8-0` sync there now embeds up to four passages in one GPU pass. On the 219-passage fixture it ran at 10.23 passages a second against 9.44 (8% more), with about 200 MB more memory at peak, and the first index of the 5,000-item brain took 1 h 21 m against 1 h 28 m in v0.4.6. The vectors are identical bit for bit, so this needs no rebuild of its own. Other Apple Silicon Macs keep one passage at a time. `PBRAIN_METAL_BATCH=0` turns the batching off. (#72)"
- `internal/cli/README.measure.md`, the #72 section: retitle it without "Opt-in"; say that the runs used the opt-in binary at `b2f4e92` with `PBRAIN_METAL_BATCH=1` and that the released default takes the same route with nothing set; replace "The default remains singleton."; add the 5,000-note batch run with its peak memory; and say what was not run (below).

### Risks and gaps

- **The fallback has never run on a real Mac.** No Mac other than a base M4 has run v0.6.9 through pbrain. goinfer's refusal is read from its source, and pbrain's handling is tested with fakes. Until now such a Mac attached once with `metal`; it now depends on the refusal being `ErrUnsupportedSelector`.
- **The cost of a refusal is not measured.** goinfer refuses after opening the device and building its pipelines and before uploading weights, closes the device, and the singleton attach then repeats that set-up, once a section. A whole attach on the base M4 took 40 ms, 167 ms the first time; other Macs are unknown.
- **The final binary has not run on the M4.** Its batch attach makes the same calls as the measured one; only the choice before it differs. The default with nothing set, `auto`, and `PBRAIN_METAL_BATCH=0` are untested on a device, and so is the stored route flag that sets the window. Recommended before a release: `TestGemmaMetalBatchLayouts` and one 219-passage sync each for `auto`, `metal` and `=0` on the M4, a few minutes once goinfer #92 frees it. Owner: @operator to decide, @pm to track.
- **Full size.** Bit equality was shown on 59 and 219 passages, not on 5,000. There is no v0.6.9 singleton run at 5,000, so the gain there is against v0.6.8. The warm search and the one-note update on the batch-built index were not run.
- **Every base M4 now stores in windows of up to 256 passages**, about 25 s of work, where it stored after each passage.
