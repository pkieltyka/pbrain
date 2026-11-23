# CUDA fp16 qualification on pax (#61)

The candidate uses goinfer v0.6.4 at `ab2721995e027c8ce6630396c922916cb2357a1d`
and Go 1.27.3. The first sync pairs used the writer at `5ae6651`. The
parity, layout and retrieval binaries used that writer with the registry
seam in `774776c`, before changing the project default. Their projects
specified each precision explicitly. The release smoke and selector probes
used the same writer with the fp16 default and the final archive tags.

Extract `capture.tar.gz` into a temporary directory and run
`python3 tools/phase28/evidence/cuda-fp16-61/verify.py <capture-directory>`
from the checkout. It reads traces, saved vectors and search results.
No model, GPU or binary execution is needed.

The capture includes the timing, parity and ranking drivers, build metadata,
preflight and 100 ms device samples, traces, ordered row/vector exports and
all query ranks. The sampled device peak includes runtime and driver memory.
The tests ran with one workload at a time. The sync samples pinned CPUs 0–3
with `GOMAXPROCS=4`, used a 115-second SIGINT deadline with a five-second
grace period, and started after two five-second host samples at or below 4%
busy with no NVIDIA compute process.

The speed driver takes the first 22 shared note paths in sorted order from
the retained #43 corpus. The adapter replay uses the #58 200-passage slice.
The parity builds use the 32 frozen #22 paths in the repository retrieval
fixture. For ranking, each precision built a cold index over the full
35-note fixture before running the 47 answerable questions and 30 held-out
negatives at cutoff 0.72. The model files were warm and reused. The paired
and fixture builds started with fresh vector caches and databases. The
rollout test uses one note per model and retains its cache across changes.

| Local input | Path |
| --- | --- |
| #43 corpus | `/var/tmp/pbrain-phase28-bge-34.rb61v3/corpus/docs/brain` |
| #58 200-passage slice and prepared IDs | `/var/tmp/pbrain-sync-trace.NSKyVs/{passages-200.json,prepared-ids.tsv}` |
| Google pack | `/var/tmp/pbrain-phase28-gemma-22.5UFzx5/gemma-pack` |
| Q8_0 model | `/var/tmp/pbrain-sync-trace.NSKyVs/q8-model` |
| Sealed model copies for ranking | `/var/tmp/pbrain-verity-59.FXgeNi/measurement/{google,q8}/sealed-cache/models` |

To repeat locally, build the release-tagged amd64 CLI with
`CGO_ENABLED=0 go build -trimpath -tags 'cuda vulkan eg2_combined eg2_fp16' -o "$scratch/pbrain" ./cmd/pbrain`.
Run the captured `speed.py` beside it with a fresh scratch directory and a
`writer-ready` marker, then use `qualification.py` for the real layout test
and replay. Build the named test binaries with `go test -c` using the same
tags. For the replay, overlay the captured `adapter/slice_test.go` over
`internal/embed/gemma2/no_device_cuda_test.go` when building `adapter/slice.test`.
For `quality.py build`, put a
copy of the regular CLI at `pbrain-candidate` and build `pbrain-split`
with a Go overlay replacing `internal/embed/gemma2/batch.go` with the
captured `split-batch.go`. The overlay injects one certified forward OOM
before calling the real encoder. Run `quality.py rank` from the checkout
after the builds. The drivers reference the retained paths on pax. Adjust
their quiet-probe path to use the captured `quiet.py` in a new scratch directory.

`rollout.py` records the selector passed to a successful `UseDevice` with
a test-only overlay, then checks the published manifest and cache reuse.
Apply `selector-overlay.patch` to a temporary copy of the adapter and
overlay that file when building `pbrain-selector-check`. Production builds
have no selector logging from that overlay.

The amd64 smoke used the release hook with NVIDIA masked and AMD ICD access
disabled. Arm64 used QEMU user mode and an arm64 glibc from Ubuntu 24.04
(libc6 2.39-0ubuntu8.12, SHA-256 in the capture). It ran the linked-selector
test, startup, CPU-only device listing and a one-note Gemma sync. The
CUDA driver was unavailable to the emulated process. No native ARM GPU
was used.
