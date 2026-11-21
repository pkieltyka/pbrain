# Phase 28 Linux measurement handover

This kit preserves the measurement harness as a repeatable runner. Use a Linux checkout with Go 1.27.3, Git, Bash, Python 3.11 or later, `taskset`, and enough local disk for both unpacked models and two fixture copies. Use four available cores, preferably equivalent cores on a hybrid CPU. Keep the two model jobs serial.

## Prepare exact binaries and model files

From the repository root, set `work` to a new absolute directory outside any Git work tree. Set `PHASE28_CPUS` to four CPUs available on the machine. `init` defaults the candidate to the checkout's `HEAD`. Pass the approved v0.1.1 candidate commit explicitly for the final check once it lands.

```sh
work=$(mktemp -d)
export PHASE28_CPUS=0-3
tools/phase28/run-linux.sh init "$work"
```

`init` builds `pbrain-baseline` from `9f53910a980200ff562a5c5f3657f4e2b1db7539` for both full-size builds, and `pbrain-candidate` from the candidate commit for the final warm check. Each uses `CGO_ENABLED=0`, `GOWORK=off`, `-trimpath`, and `-p 2`. The original baseline SHA-256 was `f05e084ee8bc73eaf337d061ca3b5977b4e21257873741633ab3082cf6dd92e4` with Go 1.27.3 on Linux amd64. Its historical pins are aikit v1.79.0 and goinfer v0.2.0. The candidate should report the current release pins, aikit v1.79.1 and goinfer v0.4.1. Build logs record the toolchain, module pins, and binary SHA-256. The scratch test harness comes from `67cf646` with the retained-directory setup and the two extra test entry points from `testdata/`.

Use existing pinned model directories if available. Otherwise prepare them once, outside the timed runs. `provision` creates a disposable one-item brain and calls baseline `pbrain index` to download and verify the selected model. It records the resulting directory in `$work/bge-model-path` or `$work/gemma-model-path`. Each invocation creates a fresh brain and reuses `$work/download-cache`, so the same command can be retried after a failed download.

```sh
tools/phase28/run-linux.sh provision "$work" bge
tools/phase28/run-linux.sh provision "$work" gemma
bge=$(cat "$work/bge-model-path")
gemma=$(cat "$work/gemma-model-path")
```

The sources and expected local files are pinned by size and SHA-256 in the baseline `internal/embed/bge/model.go` and `internal/embed/gemma2/model.go`.

| Model | Source revision on Hugging Face | Expected flat local directory |
| --- | --- | --- |
| `bge-small-en-v1.5` | `BAAI/bge-small-en-v1.5` at `5c38ec7c405ec4b44b94cc5a9bb96e735b38267a` | `model.safetensors`, `tokenizer.json`, `config.json`, `sentence_bert_config.json`, `pooling.json`. The last is `1_Pooling/config.json` in the source repository. |
| `embeddinggemma-2` | `google/embeddinggemma-2` at `f9972ac22e696a105c482202646979e33e4b5a1e` | `tokens.bf16`, `layers.safetensors`, `tokenizer.bin`, `config.json` from the derived text-only pack. |

For local Gemma pack preparation, obtain `model.safetensors`, `tokenizer.json`, `config.json`, and `config_sentence_transformers.json` from that exact Hugging Face revision. Use any required artifact access outside the repository. Put the four unmodified source files in one directory, then use the baseline adapter's checked pack operation. Give raw sources and derived outputs different directories.

```sh
tools/phase28/run-linux.sh pack "$work" /absolute/gemma-source /absolute/gemma-pack
gemma=/absolute/gemma-pack
```

On the old machine the two model directories were `/tmp/pbrain-phase24.IV8b37/model` and `/tmp/pbrain-gemma2-QrgwC0u7/derived`. Model files, generated corpora and index databases are machine-local artifacts. The branch carries the generator, commands, and evidence needed to recreate them.

## Recreate the retained fixtures and measurements

The original BGE run exited 0. The committed BGE report is in [README.measure.md](../../internal/cli/README.measure.md), and `evidence/bge-full.log` preserves the detailed row data. A new BGE build supplies the retained BGE index for the final-pins warm check when the old index is unavailable. It can also give a second host's full-size data. Record host, CPU affinity, filesystem, and Go version with the new results.

```sh
tools/phase28/run-linux.sh full "$work" bge "$bge" &&
  tools/phase28/run-linux.sh full "$work" gemma "$gemma" &&
  tools/phase28/run-linux.sh warm "$work" bge "$bge" &&
  tools/phase28/run-linux.sh warm "$work" gemma "$gemma"
```

`full` uses the committed deterministic `genBrain` fixture from `internal/cli/measure_test.go`. A single checkout gets 4,500 shared and 500 personal items totaling exactly 50,000,000 bytes. The first build runs with the pinned local model and an empty vector cache. It produces 43,642 shared and 4,869 personal passages. The same index is used for shared and personal warm reads with ten samples per row, including the strict/semantic control. The final step adds one personal note and times the incremental build. `warm` uses only a scratch copy of the retained checkout, removes just that test note in the copy to restore the exact fixture size, and checks that the new binary can still read both indexed sections. Search runs use the original timing harness's Git-counting shim and report child peak RSS and Git counts. The first sample's file cache is uncontrolled. Each command writes its log and status beside the fixtures.

## Resume and provenance

The live old-machine Gemma process resumed under `pbrain-phase28-gemma-resume.service` at 17:45:50 UTC on 2026-11-21. The first portion had run for 5,873.416 s and peaked at 2,068,220 KiB. The initial checkpoint had 545 shared items and 5,251 published passages, with 5,272 cached vectors. The saved pause record, resume start, sampled checkpoint progress, and separate rule-3 measurements are in `evidence/`.

A true continuation requires the original retained `gemma-corpus` directory, including its `personal` Git checkout, `.pbrain/index` folders, `git` shim, and sibling `personal-model-cache`, plus its matching pause record. Without those files the new host starts a fresh `full gemma` as shown above. Report that fresh duration independently of the partial run on the old host. Avoid adding old-host active time to a new full build.

If moving the original fixture and cache by another route, place their root at `$work/gemma-corpus`. Copy the matching pause record to `$work/gemma-paused.json`, and update its `brain` and `cache` fields to their corresponding absolute paths in the moved tree. Adjust `docs/brain/personal.toml` in the scratch fixture so `model_path` is the new pinned pack directory. Restart with the original baseline binary.

```sh
tools/phase28/run-linux.sh resume "$work" gemma "$gemma"
```

The resume harness checks the recorded checkpoint coverage before starting. If the index advanced again after the recorded pause, first take a fresh pause snapshot for the actual retained state and record the cumulative active time to that point. Published passages are excluded from the resumed work count. The report sums both active CLI build times, excludes the intervening pause, and uses the highest child RSS of the two runs. It also records embedded passages and cache reuses separately.

On a new retained run, `python3 tools/phase28/index-state.py sample "$work" gemma PID` prints periodic checkpoint and memory readings for the specified `pbrain-baseline --personal index --json` child. To repeat the deliberate interruption, use `python3 tools/phase28/index-state.py pause "$work" gemma PID` after a shared checkpoint exists and before any personal checkpoint. The `full` command will exit unsuccessful because its index child received SIGTERM. Keep the generated fixture and cache, then run `resume` as above. This helper verifies the child command and working directory before signalling it.
