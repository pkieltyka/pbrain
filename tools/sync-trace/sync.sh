#!/usr/bin/env bash
set -euo pipefail

if [[ $# != 3 ]]; then
    echo "usage: $0 CUDA_BINARY STAGED_CORPUS NEW_OUTPUT_DIRECTORY" >&2
    exit 2
fi
binary=$(realpath "$1")
corpus=$(realpath "$2")
mkdir "$3"
out=$(realpath "$3")
gpu=$(nvidia-smi -i 0 --query-gpu=uuid --format=csv,noheader)
if [[ -n $(nvidia-smi -i "$gpu" --query-compute-apps=pid --format=csv,noheader) ]]; then
    echo "NVIDIA GPU already has a compute process" >&2
    exit 1
fi
export PBRAIN_ALLOW_GEMMA2=1 PBRAIN_TRACE="$out/trace.jsonl" PBRAIN_CACHE="$out/cache"
export CUDA_VISIBLE_DEVICES="$gpu" GOMAXPROCS=4 TZ=UTC
unset PBRAIN_DEVICE PBRAIN_GPU_MEMORY PBRAIN_DIR PBRAIN_BRAIN PBRAIN_PROFILE_DIR
if [[ -d /run/opengl-driver/lib ]]; then
    export LD_LIBRARY_PATH="/run/opengl-driver/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
fi
cat /proc/loadavg >"$out/loadavg"
nvidia-smi -i "$gpu" >"$out/nvidia-before.txt"
cd "$corpus"
stdbuf -oL nvidia-smi -i "$gpu" --query-gpu=timestamp,memory.used,utilization.gpu \
    --format=csv,noheader,nounits -lms 100 >"$out/gpu.csv" &
sampler=$!
trap 'kill "$sampler" 2>/dev/null || true; wait "$sampler" 2>/dev/null || true' EXIT
status=0
timeout -s INT -k 15s 300s taskset -c 0-3 "$binary" --personal sync --json --device "cuda:$gpu" \
    >"$out/stdout" 2>"$out/stderr" || status=$?
case "$status" in
    0|124) ;;
    *) cat "$out/stderr" >&2; exit "$status" ;;
esac
tail -1 "$out/trace.jsonl" | python3 -c 'import json, sys; assert json.load(sys.stdin)["span"] == "summary"'
