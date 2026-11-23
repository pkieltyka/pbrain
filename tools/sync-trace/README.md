# Sync timing

Set `PBRAIN_TRACE` to a writable file path before running `pbrain sync`.
Each invocation appends JSON Lines. An event's `run` is the run's Unix
timestamp in nanoseconds. Its `start_ms` and `ms` use a monotonic clock.
Section and window numbers identify the builder and flush where applicable.
A forward records the device, input count, total prepared positions and
longest input. Retried forwards are separate events, and failed calls have
`"failed": true`.

The final `summary` records per-span counts and total durations, plus
model-open and accelerator-attach call counts. `share` is a fraction of
sync wall time. Times are inclusive, so `model_open` overlaps
`model_decode` and `device_attach`, and `publish` overlaps
`sqlite_checkpoint`. BGE forwards may also overlap on worker threads.
Only sync writes the trace. A traced sync handles SIGINT and SIGTERM through
normal context cancellation, which lets it write the summary after cleanup.
A failed trace append leaves the sync outcome intact and can leave an
incomplete capture.

## Five-minute corpus prefix

`sync.sh` takes a CUDA pbrain binary, an independently staged copy of the
corpus with the model and local model path already set, and a fresh output
directory. It bounds each sync to five minutes plus at most 15 seconds for
cleanup. Use an empty index and cache for each model. This script is for the
pax CPU affinity and NVIDIA device layout.

```sh
CGO_ENABLED=0 go build -tags cuda -o /tmp/pbrain-cuda ./cmd/pbrain
tools/sync-trace/sync.sh /tmp/pbrain-cuda /path/to/staged/corpus /tmp/capture
python3 tools/sync-trace/summarize.py /tmp/capture
```

## Fixed CUDA slice

`slice.sh` builds an isolated test binary, verifies a local model, and runs
the first 200 saved passages with pbrain's default CUDA budget on pax.
`slice_embed` includes token preparation, packing and forwards. The wrapper
also writes the exact prepared IDs as a TSV, one input per line with its
one-based forward group and space-separated IDs. It prepares that export
after the timing.

```sh
tools/sync-trace/slice.sh google /path/to/passages-200.json /path/to/gemma-pack /tmp/google-slice
tools/sync-trace/slice.sh q8 /path/to/passages-200.json /path/to/q8-model-directory /tmp/q8-slice
```

The CUDA slice captures sample NVIDIA GPU utilization at 100 ms. The summary
selects samples that fall within recorded forward intervals. The GPU counter
is sampled utilization, so it cannot distinguish kernels, copies or gaps
within a sample.

## Replay with goinfer alone

The replay uses the saved TSV grouping and calls only goinfer and the Go
standard library. It times each `EmbedIDsBatch` call after attaching the
device. Upstream can copy `testdata/goinfer-slice.go` into a scratch module with
the same goinfer root and CUDA module pins. Build with the regular CUDA tag.

```sh
CGO_ENABLED=0 go build -tags cuda -o /tmp/goinfer-slice ./tools/sync-trace/testdata/goinfer-slice.go
gpu=$(nvidia-smi -i 0 --query-gpu=uuid --format=csv,noheader)
export GOMAXPROCS=32 CUDA_VISIBLE_DEVICES="$gpu" PBRAIN_ALLOW_GEMMA2=1
# On pax, expose the driver library to dynamically loaded CUDA.
export LD_LIBRARY_PATH="/run/opengl-driver/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
timeout -k 10s 120s taskset -c 0-31 /tmp/goinfer-slice -device "cuda:$gpu" -format pack -model /path/to/gemma-pack -ids /tmp/google-slice/prepared-ids.tsv
timeout -k 10s 120s taskset -c 0-31 /tmp/goinfer-slice -device "cuda:$gpu" -format gguf -model /path/to/embeddinggemma-2-Q8_0.gguf -ids /tmp/q8-slice/prepared-ids.tsv
```
