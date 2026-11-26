#!/usr/bin/env bash
set -euo pipefail

repo=$(git rev-parse --show-toplevel)
host="$(go env GOHOSTOS)_$(go env GOHOSTARCH)"
cd "${1:?release distribution directory required}"
version=$(jq -er .version metadata.json)
sha256sum --check --strict "pbrain_${version}_checksums.txt"

shopt -s nullglob
archives=(pbrain_"${version}"_*.tar.gz)
if (( ${#archives[@]} != 7 )); then
    echo "expected seven release archives, got ${#archives[@]}" >&2
    exit 1
fi

scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT

check_archive() {
    local os=$1 arch=$2 suffix=$3 tags=$4
    local archive="pbrain_${version}_${os}_${arch}${suffix}.tar.gz"
    local binary="$scratch/pbrain_${os}_${arch}${suffix}"
    tar -xOzf "$archive" pbrain > "$binary"
    local notice
    while IFS= read -r -d '' notice; do
        if ! tar -xOzf "$archive" "${notice#"$repo"/}" | cmp -s "$notice"; then
            echo "$archive: missing or mismatched ${notice#"$repo"/}" >&2
            return 1
        fi
    done < <(find "$repo/licenses" -type f -print0)
    local build_info build_tags
    build_info=$(go version -m "$binary")
    build_tags=$(awk -F= '/^[[:space:]]*build[[:space:]]+-tags=/ {print $2}' <<< "$build_info")
    if [[ "$build_tags" != "$tags" ]]; then
        echo "$archive: expected build tags '$tags', got '$build_tags'" >&2
        return 1
    fi
    grep -Fxq $'\tbuild\tGOOS='"$os" <<< "$build_info"
    grep -Fxq $'\tbuild\tGOARCH='"$arch" <<< "$build_info"
    grep -Fxq $'\tbuild\tCGO_ENABLED=0' <<< "$build_info"

    if [[ "$os" == linux && "${os}_${arch}" == "$host" ]]; then
        chmod +x "$binary"
        echo "testing $archive"
        (
            cd "$repo"
            PBRAIN_SCENARIO_BINARY="$binary" CUDA_VISIBLE_DEVICES= \
                go test -p 2 -count=1 -run '^TestReleaseBinaryOutput$' ./cmd/pbrain
        )
    fi
}

# The Linux amd64 GPU archive links both backends; goinfer has no Vulkan backend for arm64.
check_archive linux amd64 "" cuda,vulkan,eg2_combined,eg2_fp16
check_archive linux arm64 "" cuda,eg2_combined,eg2_fp16
for arch in amd64 arm64; do
    check_archive linux "$arch" _cpu ""
done
check_archive darwin amd64 "" ""
check_archive darwin arm64 _cpu ""
check_archive darwin arm64 "" metal
