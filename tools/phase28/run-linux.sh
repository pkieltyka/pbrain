#!/usr/bin/env bash
set -euo pipefail
export GOWORK=off

here=$(cd -- "$(dirname -- "$0")" && pwd -P)
repo=$(git -C "$here" rev-parse --show-toplevel)
cpus=${PHASE28_CPUS:-0-3}
baseline=9f53910a980200ff562a5c5f3657f4e2b1db7539
harness=67cf6464e9c1848a65b7e0228140f31a5e3ea495

usage() {
    cat >&2 <<'EOF'
Usage:
  run-linux.sh init WORKDIR [CANDIDATE]
  run-linux.sh provision WORKDIR bge|gemma
  run-linux.sh pack WORKDIR GEMMA_SOURCE GEMMA_PACK
  run-linux.sh full|resume|warm WORKDIR bge|gemma MODEL_DIR
EOF
    exit 2
}

[[ $# -ge 2 ]] || usage
action=$1
mkdir -p -- "$2"
work=$(cd -- "$2" && pwd -P)
shift 2

archive() {
    mkdir -- "$2"
    git -C "$repo" archive "$1" | tar -x -C "$2"
}

build() {
    env CGO_ENABLED=0 GOMAXPROCS=2 taskset -c "$cpus" go -C "$1" build -p 2 -trimpath -o "$2" ./cmd/pbrain
    go version -m "$2"
    sha256sum "$2"
}

case "$action" in
    init)
        [[ $# -le 1 ]] || usage
        candidate=$(git -C "$repo" rev-parse --verify --end-of-options "${1:-HEAD}^{commit}")
        archive "$baseline" "$work/baseline-src"
        archive "$harness" "$work/harness-src"
        archive "$candidate" "$work/candidate-src"
        printf '%s\n' "$candidate" > "$work/candidate-revision"
        python3 - "$work/harness-src/internal/cli/measure_test.go" <<'PY'
from pathlib import Path
import sys
file = Path(sys.argv[1])
source = file.read_text()
old = 'filepath.EvalSymlinks(t.TempDir())'
new = 'filepath.EvalSymlinks(os.Getenv("PBRAIN_PHASE28_RETAIN"))'
assert source.count(old) == 1
file.write_text(source.replace(old, new))
PY
        cp -- "$here/testdata/measure_resume_test.go" "$work/harness-src/internal/cli/"
        cp -- "$here/evidence/gemma-corpus-counts.json" "$work/gemma-corpus-counts.json"
        build "$work/baseline-src" "$work/pbrain-baseline" | tee "$work/baseline-build.log"
        build "$work/candidate-src" "$work/pbrain-candidate" | tee "$work/candidate-build.log"
        ;;
    provision)
        [[ $# -eq 1 ]] || usage
        case "$1" in
            bge) name=bge-small-en-v1.5 ;;
            gemma) name=embeddinggemma-2 ;;
            *) usage ;;
        esac
        provision=$(mktemp -d -- "$work/provision-$1.XXXXXX")
        cd -- "$provision"
        unset PBRAIN_DIR
        export PBRAIN_CACHE="$work/download-cache" GOMAXPROCS=4
        "$work/pbrain-baseline" init
        printf 'semantic.model = "%s"\n' "$name" > docs/brain/pbrain.toml
        "$work/pbrain-baseline" add --title 'Provision benchmark model' --body 'One item for model setup.' --author agent:readbench
        taskset -c "$cpus" "$work/pbrain-baseline" index
        shopt -s nullglob
        # embed.FolderName ends in a 16-character digest. Gemma keeps a .source sibling.
        dirs=("$PBRAIN_CACHE/models/$name"-????????????????/)
        if [[ ${#dirs[@]} -ne 1 ]]; then
            printf 'Expected one %s model directory in %s/models, found %s\n' "$name" "$PBRAIN_CACHE" "${#dirs[@]}" >&2
            exit 1
        fi
        printf '%s\n' "${dirs[0]%/}" | tee "$work/$1-model-path"
        ;;
    pack)
        [[ $# -eq 2 && -d $1 ]] || usage
        source_dir=$(cd -- "$1" && pwd -P)
        mkdir -p -- "$2"
        dest_dir=$(cd -- "$2" && pwd -P)
        mkdir -p "$work/baseline-src/cmd/phase28-pack"
        cp -- "$here/testdata/pack_gemma.go" "$work/baseline-src/cmd/phase28-pack/main.go"
        env CGO_ENABLED=0 GOMAXPROCS=4 taskset -c "$cpus" go -C "$work/baseline-src" run -p 2 ./cmd/phase28-pack "$source_dir" "$dest_dir"
        ;;
    full|resume|warm)
        [[ $# -eq 2 && -d $2 ]] || usage
        flavor=$1
        model=$(cd -- "$2" && pwd -P)
        export CGO_ENABLED=0 GOMAXPROCS=4 PBRAIN_MEASURE=1 PBRAIN_MEASURE_PROCS=4 PBRAIN_MEASURE_RUNS=10
        export PBRAIN_MEASURE_BIN="$work/pbrain-baseline" PBRAIN_PHASE28_RETAIN="$work/$flavor-corpus"
        unset PBRAIN_TEST_BGE_MODEL PBRAIN_TEST_GEMMA2_PACK PBRAIN_MEASURE_COLD_PREP
        case "$flavor" in
            bge) export PBRAIN_TEST_BGE_MODEL="$model" PBRAIN_MEASURE_INDEX_TIMEOUT=8h ;;
            gemma) export PBRAIN_TEST_GEMMA2_PACK="$model" PBRAIN_MEASURE_INDEX_TIMEOUT=30h ;;
            *) usage ;;
        esac
        case "$action" in
            full)
                mkdir -- "$PBRAIN_PHASE28_RETAIN"
                expr='^TestMeasureReads$/^personal$'
                ;;
            resume)
                [[ $flavor == gemma ]] || usage
                export PBRAIN_PHASE28_PAUSE="$work/gemma-paused.json" PBRAIN_PHASE28_COUNTS="$work/gemma-corpus-counts.json" PBRAIN_PHASE28_RESULT="$work/gemma-resumed-build.json"
                [[ -f $PBRAIN_PHASE28_PAUSE ]]
                expr='^TestMeasureResumeGemma$'
                ;;
            warm)
                copy="$work/candidate-$flavor-corpus"
                mkdir -- "$copy"
                cp -a --reflink=auto -- "$PBRAIN_PHASE28_RETAIN/personal" "$PBRAIN_PHASE28_RETAIN/git" "$copy/"
                export PBRAIN_PHASE28_RETAIN="$copy" PBRAIN_MEASURE_BIN="$work/pbrain-candidate"
                python3 - "$copy/personal/docs/brain" <<'PY'
from pathlib import Path
import sys
brain = Path(sys.argv[1])
added = list(brain.glob('personal/notes/*-one-incremental-measurement-*.md'))
assert len(added) == 1, added
added[0].unlink()  # Only in this scratch copy.
items = [file for file in brain.rglob('*.md') if '.pbrain' not in file.parts]
assert len(items) == 5000 and sum(file.stat().st_size for file in items) == 50_000_000
PY
                expr='^TestMeasureRetainedReleaseReads$'
                ;;
        esac
        printf 'Starting %s %s at %s\n' "$action" "$flavor" "$(date -u -Iseconds)"
        status=0
        taskset -c "$cpus" go -C "$work/harness-src" test -p 2 -count=1 -timeout=32h -run "$expr" -v ./internal/cli 2>&1 | tee "$work/$flavor-$action.log" || status=$?
        printf '%s\n' "$status" > "$work/$flavor-$action.exit"
        exit "$status"
        ;;
    *) usage ;;
esac
