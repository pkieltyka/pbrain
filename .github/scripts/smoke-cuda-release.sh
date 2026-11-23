#!/usr/bin/env bash
# Release smoke test for a Linux GPU binary. GoReleaser runs it after each GPU build; it acts
# only on a binary built for this host, with PBRAIN_VERIFY_RELEASE_CUDA=1 (the release: device
# listing and a one-note EmbeddingGemma 2 sync, which needs the model) or
# PBRAIN_VERIFY_RELEASE_CUDA=devices (CI: the device listing alone, no model). It needs no GPU
# and starts none: CUDA is masked, and a hosted runner has no discrete AMD card.
#
# It runs twice. "native" takes the machine as it is. "no-vulkan" puts an unloadable
# libvulkan.so.1 first on the loader's path, so the Vulkan loader cannot be opened whatever the
# machine has installed. Each time the binary must start, list the CPU alone as one JSON
# document on standard output, and sync on the CPU saying that no usable GPU was found: a
# missing Vulkan is "absent", never "failed".
set -euo pipefail
# GoReleaser shows a failed hook's standard output alone, so diagnostics go there too.
exec 2>&1
set -E
trap 'echo "smoke: failed at line $LINENO: $BASH_COMMAND"; for f in devices.json devices.stderr sync.json sync.stderr; do [[ -f $f ]] && { echo "== $f"; cat "$f"; }; done' ERR

mode=${PBRAIN_VERIFY_RELEASE_CUDA:-}
case "$mode" in
    1|devices) ;;
    *) exit 0 ;;
esac

binary=${1:?GPU release binary required}
target=${2:?build target required}
host=$(go env GOHOSTOS)_$(go env GOHOSTARCH)
case "$target" in
    "$host"|"${host}"_*) ;;
    *) exit 0 ;;
esac

binary=$(realpath "$binary")
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
export CUDA_VISIBLE_DEVICES= PBRAIN_DEVICE=auto PBRAIN_CACHE="$scratch/cache"
export PBRAIN_ALLOW_GEMMA2=1
unset PBRAIN_GPU_MEMORY

go version -m "$binary" | awk '$1 == "build" && $2 ~ /^-tags=/ { print "build tags: " substr($2, 7) }'

smoke() {
    local label=$1
    mkdir "$scratch/$label"
    cd "$scratch/$label"

    # A GPU driver may write lines of its own on standard error (functional spec 6.3), so that
    # stream is shown and not compared. Standard output must be the one JSON document.
    if ! "$binary" devices --json > devices.json 2> devices.stderr; then
        cat devices.stderr >&2
        echo "$label: pbrain devices failed" >&2
        return 1
    fi
    jq -se 'length == 1 and ([.[0].devices[].id] == ["cpu"])' devices.json > /dev/null
    if [[ $mode == devices ]]; then
        echo "$label:"
        cat devices.json devices.stderr
        return 0
    fi

    "$binary" init > /dev/null
    local brain=docs/brain
    printf '[semantic]\nmodel = "embeddinggemma-2"\n' > "$brain/pbrain.toml"
    if [[ -n ${PBRAIN_TEST_GEMMA2_PACK:-} ]]; then
        printf 'model_path = %s\n' "$(jq -n --arg path "$PBRAIN_TEST_GEMMA2_PACK" '$path')" > "$brain/personal.toml"
    fi
    mkdir -p "$brain/notes"
    # The text differs between the two runs, so the second one has a passage to embed and
    # opens the model again instead of taking the vector from the cache.
    cat > "$brain/notes/release-smoke.md" <<NOTE
---
kind: note
title: Release smoke test ($label)
created: 2026-11-22T00:00:00Z
author: agent:ci
---
The release can sync when no GPU is available ($label).
NOTE

    if ! "$binary" sync --json > sync.json 2> sync.stderr; then
        cat sync.stderr >&2
        echo "$label: pbrain sync failed" >&2
        return 1
    fi
    jq -se 'length == 1 and (.[0].model == "embeddinggemma-2") and
        (.[0].sections[0] | .device == "cpu" and .passages_embedded == 1)' sync.json > /dev/null
    # Lines that are not JSON objects are a driver's and are skipped.
    jq -Rn '[inputs | fromjson? | objects] | any(.notice.code == "cpu_fallback" and
        .notice.message == "embedding on the CPU, which is slow: no usable GPU was found")' sync.stderr | grep -qx true
    echo "$label:"
    cat devices.json sync.json devices.stderr sync.stderr
}

smoke native

stub="$scratch/no-vulkan-lib"
mkdir "$stub"
: > "$stub/libvulkan.so.1"
LD_LIBRARY_PATH="$stub${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}" smoke no-vulkan
