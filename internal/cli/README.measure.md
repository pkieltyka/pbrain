# Release measurements

Measure on Linux before the first release. macOS is deferred: it is not
tested or supported for now, and Apple silicon Mac measurements are
tracked separately in architecture §17. Keep the machine idle and
record its CPU, OS, filesystem, executable revision and cache preparation
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
register the selected model. To measure both views after one index build,
select `-run '^TestMeasureReads$/^personal$'`. It builds one 5,000-item,
50 MB fixture with 4,500 shared and 500 personal items. The timed shared
and personal searches use that same index. The separate `shared` subtest
measures a checkout with all 5,000 items shared.

The fixture's short session-storage note must be absent from the strict
page and present with its ID and `semantic: true` in the timed paraphrase
pages, including the combined search when the executable exposes BM25.
The test then adds one short note and times the next index. The local
checkpoint stays untouched. The index, vector
cache and generated items are temporary. Each index command has its own
timeout, configurable with `PBRAIN_MEASURE_INDEX_TIMEOUT`. The defaults
are four hours for bge-small and eight hours for EmbeddingGemma 2. The
generated corpus contains many invented words that split into several
tokens. A first build can take hours on this workload.

The table records latency, Git invocations and child peak resident memory.
`PBRAIN_MEASURE_RUNS` sets the number of warm samples per row. To measure
a colder first sample, set `PBRAIN_MEASURE_COLD_PREP` to your executable
for preparing the file cache. Its first argument is the scratch checkout,
and in model cases its second argument is the model directory. Describe
which pages you actually evicted. The harness labels the first sample's
cache state uncontrolled when the helper is absent.

Measure architecture §8.7 rule 3 separately from the full CLI timings.
The benchmark reads and verifies the model, opens it, embeds one query
and closes it. Each line below the header is one timed run.

```sh
GOMAXPROCS=4 go test -p 2 -run '^$' -bench '^BenchmarkBgeFirstQuery$' \
  -benchtime=1x -count=3 ./internal/embed/bge
```

For an EmbeddingGemma 2 release, use `BenchmarkGemmaFirstQuery` in
`./internal/embed/gemma2` with the pinned pack.

## bge-small-en-v1.5, Linux

2026-11-21. Whole commands used the `9f53910` baseline, built with Go
1.27.3, `CGO_ENABLED=0` and `-trimpath`. Linux 6.18.63 on an Intel Core
Ultra 7 255H, btrfs, CPUs 0–3 and `GOMAXPROCS=4`. The generated fixture had
5,000 items and 50,000,000 bytes, including 500 personal items. All commands
used the local pinned model files. The first build started with an empty
vector cache and includes process start. Peak RSS is the maximum resident
set reported for the `pbrain` child, separate from the Go test process.

| Command | Wall time | Peak RSS | Git calls |
| --- | ---: | ---: | ---: |
| First index, both sections | 6,072.9 s (1 h 41 m 13 s) | 1,350.9 MiB | 15 |
| Index after one new personal note | 0.770 s | 469.1 MiB | 11 |

The first build embedded 43,642 shared passages and 4,869 personal
passages, with zero reuses in both sections. The total was 48,511 passages,
or 7.99 passages/s across the whole build. The update embedded one personal
passage and reused zero.

Each search row has ten warm samples after an untimed priming read.
The first sample's cache state was uncontrolled. Every sample had nine
Git calls. The paraphrase and combined rows required the expected ID marked
`semantic: true`. The lexical control omitted that ID.

| Search and view | First (ms) | Warm median (ms) | Warm range (ms) | Max warm RSS (MiB) |
| --- | ---: | ---: | ---: | ---: |
| Semantic, shared, exact hit | 310.1 | 309.2 | 296.5–323.5 | 506.4 |
| Semantic, shared, paraphrase | 279.0 | 302.3 | 284.9–321.4 | 503.9 |
| Semantic + BM25, shared, paraphrase | 282.7 | 298.4 | 287.0–319.0 | 504.4 |
| Semantic, personal, exact hit | 313.3 | 333.2 | 322.1–357.2 | 549.4 |
| Semantic, personal, paraphrase | 344.6 | 331.8 | 309.8–361.3 | 546.8 |
| Semantic + BM25, personal, paraphrase | 350.2 | 335.1 | 311.8–353.2 | 547.2 |

Shared searches saw 4,500 items. Personal searches saw all 5,000. Across
the personal lexical cases, warm search medians were 135.4–189.3 ms and
the `--bm25` medians were 151.9–196.2 ms. The `get` median was 134.3 ms.

In the separate Linux rule-3 run from `4eff5a7`, verifying files, opening
the model, embedding one query and closing took 89–123 ms over five warm
samples, with a median of 93 ms.

## EmbeddingGemma 2, Linux

2026-11-21. Binary built from `4eff5a7` (release wiring `2233197`) with
Go 1.27.3, `CGO_ENABLED=0` and `-trimpath`. Linux 6.18.63 on an Intel Core
Ultra 7 255H, btrfs, pinned to CPUs 0–3 with `GOMAXPROCS=4`. The pinned
model pack was already on disk and in the page cache after setup.

| Timed operation | Samples | Median | Range |
| --- | ---: | ---: | ---: |
| Verify pinned files, open model, embed one query, close | 5 | 281 ms | 253–366 ms |
| Built binary, complete one-item semantic search | 10 | 502 ms | 478–553 ms |

Both measurements used `where is login state kept`. The first row uses
`BenchmarkGemmaFirstQuery` from the binary's source revision with
`-benchtime=1x -count=5`. The second row times fresh release processes running
`search --semantic --json` after one untimed warm-up. The scratch brain
used embeddinggemma-2, the default at that revision, and a local indexed note about session storage.
Strict search returns zero results. Every timed process returns one
semantic hit with empty stderr. Maximum peak RSS is 1,131,164 KiB (1.08 GiB).
The complete command includes verification, load and inference, and is
an upper bound for rule 3 on this fixture. Every warm Linux sample fits
the 600 ms rule. Cold-cache and 5,000-item results are separate; arm64 Mac
results are deferred (architecture §17).

## Release build and install

Build the release binary with `CGO_ENABLED=0` for Linux and Darwin on both
amd64 and arm64. The Darwin binaries are only built: running them is
deferred with macOS (architecture §17). Once a release tag is available,
verify versioned install in a scratch directory by setting `GOBIN` there
and running
`go install github.com/pkieltyka/pbrain/cmd/pbrain@"$PBRAIN_RELEASE_TAG"`.
Run the installed binary's `--version` and record the result with the
timings.
