#!/usr/bin/env bash
set -euo pipefail

if [[ $# != 4 || ( $1 != google && $1 != q8 ) ]]; then
    echo "usage: $0 google|q8 PASSAGES_JSON MODEL_DIRECTORY NEW_OUTPUT_DIRECTORY" >&2
    exit 2
fi
arm=$1
slice=$(realpath "$2")
model=$(realpath "$3")
mkdir "$4"
out=$(realpath "$4")
cd "$(dirname "$0")/../.."
gpu=$(nvidia-smi -i 0 --query-gpu=uuid --format=csv,noheader)
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
python3 - "$PWD" >"$scratch/overlay.json" <<'PY'
import json
from pathlib import Path
import sys
root = Path(sys.argv[1])
print(json.dumps({"Replace": {
    str(root / "internal/embed/gemma2/no_device_cuda_test.go"): str(root / "tools/sync-trace/testdata/slice_test.go"),
}}))
PY
CGO_ENABLED=0 go test -c -tags cuda -overlay "$scratch/overlay.json" -o "$scratch/slice.test" ./internal/embed/gemma2
export PBRAIN_ALLOW_GEMMA2=1 CUDA_VISIBLE_DEVICES="$gpu"
export PBRAIN_SLICE="$slice" PBRAIN_TEST_GEMMA2_PACK="$model" PBRAIN_SLICE_ARM="$arm" PBRAIN_SLICE_DEVICE="cuda:$gpu"
export PBRAIN_TRACE="$out/trace.jsonl" PBRAIN_SLICE_IDS="$out/prepared-ids.tsv" GOMAXPROCS=32 TZ=UTC
if [[ -d /run/opengl-driver/lib ]]; then
    export LD_LIBRARY_PATH="/run/opengl-driver/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
fi
cat /proc/loadavg >"$out/loadavg"
nvidia-smi -i "$gpu" >"$out/nvidia-before.txt"
stdbuf -oL nvidia-smi -i "$gpu" --query-gpu=timestamp,memory.used,utilization.gpu \
    --format=csv,noheader,nounits -lms 100 >"$out/gpu.csv" &
sampler=$!
trap 'kill "$sampler" 2>/dev/null || true; wait "$sampler" 2>/dev/null || true; rm -rf "$scratch"' EXIT
timeout -k 10s 120s taskset -c 0-31 "$scratch/slice.test" -test.run '^TestTraceSlice$' -test.v
sha256sum "$out/prepared-ids.tsv"
