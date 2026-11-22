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
lists EmbeddingGemma 2, run separately with `PBRAIN_ALLOW_GEMMA2=1` (the
owner-only team switch) and `PBRAIN_TEST_GEMMA2_PACK` set to the pinned
pack directory and use `-timeout=18h`. The binary must
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

### Ryzen 9 7950X rerun and final-pin warm check

2026-11-21. The `pax` host has an AMD Ryzen 9 7950X (16 physical cores,
32 logical CPUs), 124.9 GiB of RAM visible to Linux, kernel 7.3.4 and
btrfs. Both stages used CPUs 0–3 (one thread on each of four physical
cores) and `GOMAXPROCS=4`. The separate checkpoint sampler ran on CPU 4.
Both binaries used Go 1.27.3, `CGO_ENABLED=0` and `-trimpath`.

The full run used the same `9f53910` baseline binary as the Intel run,
with the pinned model already on disk and an empty index and vector cache.
The fixture again had 4,500 shared and 500 personal items totaling
50,000,000 bytes. The first build embedded 43,642 shared and 4,869 personal
passages, with zero reuses. The update embedded one personal passage.

| Command | Wall time | Passages/s | Peak RSS | Git calls |
| --- | ---: | ---: | ---: | ---: |
| First index, both sections | 4,705.8 s (1 h 18 m 26 s) | 10.31 | 1,314.9 MiB | 15 |
| Index after one new personal note | 0.678 s | — | 471.6 MiB | 11 |

The warm check used candidate `cbaaa0ebe7978c0f8096ec46f38237b2aae4b1ed`
with aikit v1.79.1 and goinfer v0.4.1. It read a copy of the retained
baseline index with the incremental note removed from the copied fixture.
Each row had ten warm samples after an untimed priming read. The first
sample's cache state was uncontrolled, and each sample made nine Git calls.

| Search and view | First (ms) | Warm median (ms) | Warm range (ms) | Max warm RSS (MiB) |
| --- | ---: | ---: | ---: | ---: |
| Semantic, shared, exact hit | 297.5 | 297.7 | 292.7–302.9 | 506.5 |
| Semantic, shared, paraphrase | 294.4 | 292.2 | 287.2–295.9 | 504.0 |
| Semantic + BM25, shared, paraphrase | 291.4 | 291.1 | 284.0–296.6 | 504.2 |
| Semantic, personal, exact hit | 325.3 | 325.7 | 317.6–331.0 | 549.7 |
| Semantic, personal, paraphrase | 315.4 | 320.6 | 316.3–329.7 | 547.3 |
| Semantic + BM25, personal, paraphrase | 316.7 | 321.8 | 316.1–324.9 | 547.6 |

The first build took 22.5% less time on the Ryzen host with the same
baseline binary, and its peak RSS was within 3% of the Intel run. The
one-note update was 92 ms quicker in this pair of single samples. Warm
search medians were broadly similar, 2–4% lower than the Intel results
and within 3.5 ms of the Ryzen baseline medians on the same rows. Both
hosts peaked at about 550 MiB in warm search and stayed under the 1-second
search target.

[Full run log](../../tools/phase28/evidence/bge-pax-full.log),
[warm check log](../../tools/phase28/evidence/bge-pax-warm.log),
[checkpoint samples](../../tools/phase28/evidence/bge-pax-index-samples.jsonl),
[host details](../../tools/phase28/evidence/bge-pax-host.log).

### Warm BGE search after CRC-32C and bounded catch-up

2026-11-22. Candidate `87fd48ba537583f507e8e025115baa57c4c6d76c` used
Go 1.27.3, `CGO_ENABLED=0`, `-trimpath`, and `GOMAXPROCS=4` on CPUs 0–3
of the same Ryzen 9 7950X (124.9 GiB visible RAM, Linux 7.3.4, btrfs).
`pbrain --personal sync --json` rebuilt the retained format-2 indexes in
a scratch copy. It reused all 43,642 shared passages and embedded 4,870
personal passages, including the one-note update. The kit's `warm` step
restored the 5,000-item, 50 MB fixture for the search measurements.

| Warm semantic search | `cbaaa0e` median (ms) | Candidate median (ms) | Candidate range (ms) |
| --- | ---: | ---: | ---: |
| Shared, exact hit | 297.7 | 374.0 | 302.1–588.2 |
| Shared, paraphrase | 292.2 | 397.9 | 292.2–689.4 |
| Shared, paraphrase + BM25 | 291.1 | 364.4 | 285.2–559.0 |
| Personal, exact hit | 325.7 | 401.3 | 331.6–680.0 |
| Personal, paraphrase | 320.6 | 423.2 | 320.5–717.2 |
| Personal, paraphrase + BM25 | 321.8 | 379.9 | 326.3–700.6 |

To check the spread, ten paired `search 'where is login state kept'
--semantic --json` trials alternated the binaries in opposite orders
on successive trials, using each binary's complete index. The earlier
binary then measured 340.3 ms shared and 444.1 ms personal, versus
359.2 and 387.5 ms for the candidate. Median paired changes were −33.2
and −29.2 ms, with differences of both signs and a spread exceeding
the median shifts. Even `cbaaa0e` ran slower in the paired repeat than
in its original run. This full-path comparison leaves the isolated CRC
gain unresolved.

| Candidate operation | Warm median (ms) | Range (ms) |
| --- | ---: | ---: |
| `sync --status --json`, shared | 563.1 | 410.3–730.9 |
| `--personal sync --status --json` | 535.4 | 481.8–597.2 |
| Shared search, one new passage, lock free | 863.2 | 663.7–1,042.0 |
| Shared search, one new passage, lock held | 377.5 | 294.5–611.7 |
| Shared search, 20 new passages, lock free | 1,242.4 | 1,069.0–1,368.0 |
| Shared search, 20 new passages, lock held | 344.1 | 290.0–462.4 |

Each case had ten timed fresh processes after a warm-up. Catch-up used
one or 20 new shared notes, one uncached passage each, and restored the
same index between pairs. Lock-free catch-up added median paired costs
of 453.3 and 883.4 ms, including embedding, publication and scoring the
updated index.
Every free-lock trial saved the entire gap and returned a new semantic
hit. The 20-passage case exceeded one second in all ten timed trials.
Every held-lock search returned `semantic_partial` before the lock was
released and left the prior index generation in place.

[Kit warm log](../../tools/phase28/evidence/bge-pax-crc-catchup-warm.log),
[paired samples and correctness checks](../../tools/phase28/evidence/bge-pax-crc-catchup-samples.jsonl),
[host details](../../tools/phase28/evidence/bge-pax-crc-catchup-host.log).

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

### SQLite sync with published goinfer v0.5.0

On November 21, 2026, pbrain commit `93ae631ea40e8b092d0aff228b162c7d8597e2cd`
ran with the published goinfer and CUDA backend modules at `v0.5.0`.
All three Linux amd64 binaries used Go 1.27.3, `GOWORK=off`, `CGO_ENABLED=0`,
`GOAMD64=v1`, and `-trimpath`. Runtime used `GOMAXPROCS=4` on a Ryzen 9
7950X with an RTX PRO 6000 Blackwell Max-Q Workstation Edition, NVIDIA driver
595.112.07, and btrfs. The CPU build used no tags. The GPU builds used
`-tags cuda` and `-tags 'cuda eg2_combined'` respectively.

The fixture was the same 32 current shared Markdown notes used in the
[goinfer application benchmark](https://github.com/pkieltyka/goinfer/blob/v0.5.0/docs/measurements/pbrain-embedding-rtx-combined-2026-11-21/README.md#pbrain-application-on-the-workstation).
The source notes are in `internal/semantic/calibrate/testdata/retrieval/brain`
at pbrain commit `b4def8fad8ea34101c369777604144fbbec2182a`.
They produced 59 passages and 9,271 prepared tokens in the reference
fixture. The model was the local text pack derived from
`google/embeddinggemma-2@f9972ac22e696a105c482202646979e33e4b5a1e`
with BF16 weights widened to FP32 for deterministic inference.

Each arm ran once in CPU, CUDA, then combined CUDA order. Each command used
a fresh process. The fresh sync started with an empty SQLite index and
vector cache. The no-change sync used the resulting index, and the
one-note sync added one short note that produced one passage. One-minute
start load ranged from 0.70 to 1.32. Filesystem cache and GPU thermal
state were uncontrolled.

| `pbrain sync --json` wall time | CPU | CUDA | Combined CUDA |
| --- | ---: | ---: | ---: |
| Fresh index, 59 new vectors | 14.61 s | 2.34 s | 2.06 s |
| No changes, zero new vectors | 243 ms | 237 ms | 240 ms |
| One new note, one new vector | 461 ms | 1.05 s | 1.04 s |

Every SQLite database passed `PRAGMA quick_check`. Both CUDA builds
stored bit-identical ordered vectors. Each build matched its pre-tag
vector digest and returned the same rankings for the three labeled
semantic queries `a003`, `a004` and `a005`.

## Release build and install

Build the release binary with `CGO_ENABLED=0` for Linux and Darwin on both
amd64 and arm64. The Darwin binaries are only built: running them is
deferred with macOS (architecture §17). Once a release tag is available,
verify versioned install in a scratch directory by setting `GOBIN` there
and running
`go install github.com/pkieltyka/pbrain/cmd/pbrain@"$PBRAIN_RELEASE_TAG"`.
Run the installed binary's `--version` and record the result with the
timings.
