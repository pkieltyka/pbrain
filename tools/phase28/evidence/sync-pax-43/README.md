# Full-size pbrain sync on pax

The measured source is `004e7e3b16661358b306a4f7701ba412cd62558b`.
The host and both executable build records are in `host.log`. The original
fixture is the retained 50 MB corpus from #34. The runner clones its Markdown
files, verifies the same canonical path/content hash, primes the pinned pack
and creates a fresh cache for each case. Set the source and pack paths in
`measure.py` to the corresponding local phase-28 artifacts. The script expects
an untagged binary named `pbrain-cpu` and a `-tags cuda` binary named
`pbrain-cuda` beside it.

```sh
git worktree add --detach "$work/source" 004e7e3b16661358b306a4f7701ba412cd62558b
cd "$work/source"
CGO_ENABLED=0 go build -trimpath -ldflags '-s -w' -o "$work/pbrain-cpu" ./cmd/pbrain
CGO_ENABLED=0 go build -trimpath -ldflags '-s -w' -tags cuda -o "$work/pbrain-cuda" ./cmd/pbrain
python3 "$work/measure.py"
python3 "$work/finish.py"
```

The original comparison interleaved the two BGE widths. The repeat runner
preserves outlying runs and fills in any missing clean samples. It considers
three consecutive 10-second intervals with at least four other logical CPUs
busy sufficient to exclude a BGE run from the timing comparison. Every trial
remains part of the database equivalence check.

`events.jsonl` records affinity and commands. `preflight.jsonl` records
short-window host occupancy, load averages and the idle NVIDIA compute queue.
`summary.jsonl` contains the wall time, maximum RSS and exact passage counts.
`load-audit.jsonl` and `matched-summary.jsonl` identify the included trials.
The BGE database comparisons include file digests, checks of each ordered row
including vector blobs, and SQLite integrity checks. The Gemma case includes
250 ms device samples in `gemma-cuda/gpu.csv.gz`.
