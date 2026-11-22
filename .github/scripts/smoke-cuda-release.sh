#!/usr/bin/env bash
set -euo pipefail

if [[ ${PBRAIN_VERIFY_RELEASE_CUDA:-} != 1 ]]; then
    exit 0
fi

binary=${1:?CUDA release binary required}
target=${2:?build target required}
host=$(go env GOHOSTOS)_$(go env GOHOSTARCH)
case "$target" in
    "$host"|"${host}"_*) ;;
    *) exit 0 ;;
esac

scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
cd "$scratch"
export CUDA_VISIBLE_DEVICES= PBRAIN_DEVICE=auto PBRAIN_CACHE="$scratch/cache"
export PBRAIN_ALLOW_GEMMA2=1
unset PBRAIN_GPU_MEMORY

"$binary" devices --json | jq -e '[.devices[].id] == ["cpu"]' > /dev/null
"$binary" init > /dev/null
brain=docs/brain
printf '[semantic]\nmodel = "embeddinggemma-2"\n' > "$brain/pbrain.toml"
if [[ -n ${PBRAIN_TEST_GEMMA2_PACK:-} ]]; then
    printf 'model_path = %s\n' "$(jq -n --arg path "$PBRAIN_TEST_GEMMA2_PACK" '$path')" > "$brain/personal.toml"
fi
mkdir -p "$brain/notes"
cat > "$brain/notes/release-smoke.md" <<'NOTE'
---
kind: note
title: Release smoke test
created: 2026-11-22T00:00:00Z
author: agent:ci
---
The release can sync when the CUDA device is unavailable.
NOTE

if ! "$binary" sync --json > sync.json 2> sync.stderr; then
    cat sync.stderr >&2
    exit 1
fi
jq -e '.model == "embeddinggemma-2" and
    (.sections[0] | .device == "cpu" and .passages_embedded == 1)' sync.json > /dev/null
jq -se 'any(.[]; .notice.code == "cpu_fallback" and
    .notice.message == "embedding on the CPU, which is slow: no usable GPU was found")' sync.stderr > /dev/null
cat sync.json sync.stderr
