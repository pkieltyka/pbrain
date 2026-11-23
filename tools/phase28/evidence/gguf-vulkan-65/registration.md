# pbrain #65 device acceptance registration

Device acceptance for the reviewed pbrain candidate that admits Q8_0 and
UD-Q6_K_XL on Vulkan. The runner is prepared in scratch and must be reviewed
before it runs. No device work, model load or build is part of this file.

## Frozen inputs

- pbrain candidate: the exact reviewed commit recorded in `manifest.json`; the
  runner refuses a dirty worktree or a commit mismatch.
- goinfer root and Vulkan v0.6.6, `c0a5159bbff1c2fe5c8624e1399aa7438c820677`.
- Q8_0: `embeddinggemma-2-Q8_0.gguf`, 309,855,520 B,
  `6f1bd4ac6c5df7444f9cca7ca36cafe6cfa34cd6f49fefb1e0b4be8143aed8bc`.
- UD-Q6_K_XL: `embeddinggemma-2-UD-Q6_K_XL.gguf`, 248,812,032 B,
  `dc84f042bc3ffbe6122b3b38b2fb299d66f5f37b276c9d458db85d4a4f1e6ba3`.
- Workload: the 32 frozen notes in `fixture-paths.txt`, digest-pinned by
  `fixture.SHA256` (list SHA-256 `77bab592…`), producing 59 passages at the
  768-dimension setting.
- Go: `GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH=amd64`, `-tags vulkan`.

## Device and host

AMD Ryzen 9 7950X, 124.9 GiB RAM; Radeon RX 9060 XT, Mesa RADV 26.3.1, PCI
`0000:0c:00.0`, vendor/device `1002:7590`, persistent selector
`vulkan:000000000c0000000000000000000000`, alias `vulkan:0`.

## Scoped load cap

The owner approved 4.0 in place of 1.0 for this descriptive sync on 2026-11-23,
because unrelated workstation load at 1.2–2.5 kept the quiet window closed.
Every other guard is unchanged. Attempts refused under the earlier 1.0 cap stay
refused and are not reused; this changes only new attempts of this scope. CUDA
#30 is unaffected.

## Queue and lock

Run only after the #76 disposition releases the device. The whole device stage
holds the repository's exclusive timing lock:

```
python3 <goinfer>/scripts/timing_lock.py run --exclusive --label pbrain-65-device -- \
  python3 tools/phase28/evidence/gguf-vulkan-65/run-65.py --stage all --out <out> \
  --commit <reviewed> --goinfer-repo <goinfer> \
  --q8-file <q8 gguf file> --q6-file <q6 gguf file> \
  --desktop-clients <pid-map> --vulkan-loader-dir <loader> --vulkan-layer-dir <layers>
```

The runner refuses to start unless the holder's mode is `exclusive` and its
token is in `GOINFER_TIMING_LOCK`. It waits up to 600 seconds for a quiet
window: load1 at or below 4.0, at least 8 GiB host available, at least 2 GiB
Radeon free, and no foreign pbrain or Radeon client. Desktop processes listed in
the pid map are not foreign work.

## Cells

Four fresh cells, two per quant: CPU and the named Vulkan selector. Each cell
gets a new brain, `HOME`, cache, trace and hard-linked verified model file:

| Cell | Model | Device |
| --- | --- | --- |
| q8-cpu | `embeddinggemma-2-q8-0` | `cpu` |
| q8-vulkan | `embeddinggemma-2-q8-0` | `vulkan:000000000c0000000000000000000000` |
| q6-cpu | `embeddinggemma-2-ud-q6-k-xl` | `cpu` |
| q6-vulkan | `embeddinggemma-2-ud-q6-k-xl` | `vulkan:000000000c0000000000000000000000` |

Each runs one `pbrain sync --device <selector> --json` over the 32 notes with
`GOMAXPROCS=4` and tracing on. The timer is the complete process, from start to
published index; it includes model verification, load and attach, scanning,
token preparation, cache writes, SQLite commits and manifest publication.

The layout stage runs the prebuilt `gemma2-vulkan.test` with
`TestGemmaVulkanLayouts`: 59 passages, singleton against packed, 17-input split,
reversed and 1 MiB-budget layouts, plus CPU/Vulkan cosine at 768 and the
normalized 512 prefix, per quant.

## Guards

- Launch: load1 at most 4.0; at least 8 GiB host available; at least 2 GiB
  Radeon free; no foreign pbrain or Radeon client.
- Runtime: 4 GiB attributed/device-delta cap, 512 MiB in-flight child swap cap,
  zero new host pageout, no amdgpu timeout/reset, no Vulkan validation error,
  and the validation layer must load.
- Cell deadline: 90 seconds for a sync, 600 seconds for the layout test.
- A void cell stops without an accepted result; the operator reruns the whole
  evidence directory. No partial cell enters a verdict.

## Acceptance

For each quant, CPU and Vulkan must each have a fresh index with:

- 32 items, 59 newly embedded passages, 0 reused, one shared section;
- `PRAGMA quick_check` = `ok`;
- item paths and content digests, and passage identity, ordinal, start, stop and
  digest, identical between the CPU and Vulkan cell;
- finite, unit 768-dimensional vectors;
- minimum CPU/Vulkan cosine at least 0.999, both at 768 and the normalized 512
  prefix;
- the exact persistent UUID and alias reported;
- zero child swap, zero new host pageout, no reset or validation error;
- end-to-end wall, peak host RSS and peak Radeon attribution recorded.

Same-device layout receipts must show zero differing vectors for every layout,
and the layout receipt's own CPU/Vulkan minima must be at least 0.999.

There is no new speed acceptance gate. Report the actual end-to-end wall,
passages/s and peaks, even below 2× the CPU cell. The descriptive #39 encoder
observations are context, not an indexing gate.

## Evidence and verification

`run-65.py` writes `manifest.json`, `build.json`, `preflight.json`,
`device.json`, `inventory.*`, `fixture.SHA256`, `cells/<cell>/` with the brain
and SQLite database, `layout/` with per-quant receipts, and `results.json`.
`verify.py` re-opens the databases read-only, re-derives the model, index and
folder identities, and checks every value above. It writes `verification.json`
and `verification.log`; `evidence.SHA256` covers the final tree.

## Not in scope

No default-model change, no selector or release-row change, no tag, no
publication, no CUDA or llama.cpp comparison, and no claim from a single
descriptive run.
