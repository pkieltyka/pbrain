# #65 GGUF-on-Vulkan device evidence

Prepared, not run. The runner and verifier are reviewed before the device slot;
the device stage starts only after #76 releases the queue.

## Files

- `registration.md` — frozen inputs, cells, guards, acceptance and exclusions.
- `run-65.py` — build, four fresh SQLite syncs and the layout stage.
- `verify.py` — read-only verification of the complete evidence tree.
- `fixture-paths.txt` / `fixture.SHA256` — the 32-note frozen workload.
- `test-run-65.py` — host control: a new PID with an approved comm is foreign.
- `../gguf-vulkan-65` receipts are produced only by a run and are not in Git.

## Review gate

Ask @reviewer to inspect the exact reviewed commit, `run-65.py`, `verify.py`,
`registration.md`, the fixture pins and `internal/embed/gemma2/gguf_vulkan_test.go`
before any build or device work. The reviewer check covers UUID handling, the
fresh-index path, the fit guard, the CPU/Vulkan and layout checks, and truthful
scope.

## Commands

Build once, under the same exclusive lock, on the reviewed clean commit:

```
python3 <goinfer>/scripts/timing_lock.py run --exclusive --label pbrain-65-device -- \
  python3 tools/phase28/evidence/gguf-vulkan-65/run-65.py --stage build \
  --out <out> --commit <reviewed> --goinfer-repo <goinfer> \
  --vulkan-loader-dir <loader> --vulkan-layer-dir <layers>
```

Then run the device stage and verification:

```
python3 <goinfer>/scripts/timing_lock.py run --exclusive --label pbrain-65-device -- \
  python3 tools/phase28/evidence/gguf-vulkan-65/run-65.py --stage qualify --out <out> \
  --commit <reviewed> --goinfer-repo <goinfer> \
  --q8-file <q8 gguf file> --q6-file <q6 gguf file> \
  --desktop-clients <pid-map> --vulkan-loader-dir <loader> --vulkan-layer-dir <layers>

python3 tools/phase28/evidence/gguf-vulkan-65/run-65.py --stage verify --out <out>
```

`--stage all` runs build, qualify and verify in that order. The `verify` stage
needs no lock, model or device.

The foreign-client check exempts only the exact pid+comm pairs in
`--desktop-clients`; a new PID is foreign even when its comm matches a desktop
name. `python3 test-run-65.py` proves that control without a device.

## Evidence

`<out>/manifest.json` records the commit, Go build flags, exact UUID/alias,
thresholds, fixture hashes and binary hashes. `cells/<cell>/` holds the fresh
brain, SQLite index, `sync.json`, `result.json`, samples, trace and stderr.
`layout/` holds the per-quant layout receipts. `verification.json` is the
independent verdict. After a successful verification the same command writes
`evidence.SHA256`, a sorted hash list of every file except the seal itself.

The runner refuses a dirty worktree, a commit mismatch, a missing exclusive
lock, a missing validation layer, a recent amdgpu reset, foreign Radeon work, a
new host pageout, child swap over 512 MiB, or a 4 GiB attributed/device cap.
A failed cell is a void, not a retried speed sample.

## What a pass means

For each quant, a fresh CPU and a fresh Vulkan index have 32 items and 59
passages, pass `quick_check`, match each other's item and passage metadata, hold
finite unit vectors, and meet a minimum CPU/Vulkan cosine of 0.999. Same-device
layouts are identical by bits. The exact UUID is reported and the guards are
clean. Wall time, passages/s, host RSS and Radeon peaks are reported without a
new speed threshold.
