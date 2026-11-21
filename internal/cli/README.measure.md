# Release measurements

Run on Linux and on an arm64 Mac. Keep the machine idle and record its
CPU, OS, filesystem, executable revision and the cache preparation
method. The release targets are in functional spec §16 and architecture
§§8.6–8.7. Publish the first sample separately from the warm samples.

Build from the release commit or pass the installed release executable
as `PBRAIN_MEASURE_BIN`. The test builds a cgo-disabled binary when
that variable is absent.

```sh
PBRAIN_MEASURE=1 PBRAIN_MEASURE_PROCS=4 \
  go test -p 2 -count=1 -timeout=10h -run '^TestMeasureReads$' -v ./internal/cli
```

Set `PBRAIN_TEST_BGE_MODEL` to the absolute directory containing the
five pinned bge-small files to add model cases. To measure a release that
lists EmbeddingGemma 2, run separately with `PBRAIN_TEST_GEMMA2_PACK` set
to the pinned pack directory and use `-timeout=18h`. The binary must
register the selected model. The test builds a fresh index in each of two 5,000-item,
50 MB fixtures, one shared and one including 500 personal items. It
times semantic searches on both indexes and runs the combined search
when the executable exposes BM25. It then adds one short note and times
the next index. The local checkpoint stays untouched. The index, vector
cache and generated items are temporary. Each index command has its own
timeout, configurable with `PBRAIN_MEASURE_INDEX_TIMEOUT`. The defaults
are four hours for bge-small and eight hours for EmbeddingGemma 2. The
generated corpus contains many invented words that split into several
tokens. The two first builds can take hours on this workload.

The table records latency, Git invocations and child peak resident memory.
`PBRAIN_MEASURE_RUNS` sets the number of warm samples per row. To measure
a colder first sample, set `PBRAIN_MEASURE_COLD_PREP` to your executable
for preparing the file cache. Its first argument is the scratch checkout,
and in model cases its second argument is the model directory. Describe
which pages you actually evicted. The harness labels the first sample's
cache state uncontrolled when the helper is absent.

Measure architecture §8.7 rule 3 separately on both machines. The
benchmark reads and verifies the model, opens it, embeds one query and
closes it. Each line below the header is one timed run.

```sh
GOMAXPROCS=4 go test -p 2 -run '^$' -bench '^BenchmarkBgeFirstQuery$' \
  -benchtime=1x -count=3 ./internal/embed/bge
```

For an EmbeddingGemma 2 release, use `BenchmarkGemmaFirstQuery` in
`./internal/embed/gemma2` with the pinned pack.

Build the release binary with `CGO_ENABLED=0` for Linux and Darwin on both
amd64 and arm64. Once a release tag is available, verify versioned install
in a scratch directory by setting `GOBIN` there and running
`go install github.com/pkieltyka/pbrain/cmd/pbrain@"$PBRAIN_RELEASE_TAG"`.
Run the installed binary's `--version` and record the result with the
timings.
