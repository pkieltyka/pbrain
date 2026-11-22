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

### Full-size CUDA sync and CPU search comparison

2026-11-22. Candidate `b79b9aad8529a5593b1bd9cb33a5b06ccec2f8c8`
used Go 1.27.3, goinfer v0.5.0, `CGO_ENABLED=0`, `GOAMD64=v1` and
`-trimpath`. The two executables were the untagged CPU build and the
plain `-tags cuda` build. The host was a Ryzen 9 7950X with 124.9 GiB of
RAM visible to Linux, kernel 7.3.4, btrfs and an RTX PRO 6000 Blackwell
Max-Q Workstation Edition (97,887 MiB, driver 595.112.07). The measured
processes used `GOMAXPROCS=4` on CPUs 8–11, four physical cores on one L3.
The local text pack came from `f9972ac22e696a105c482202646979e33e4b5a1e`.
The runs recorded `/proc/loadavg` and `nvidia-smi` before timing each
command or benchmark process.

#### Matched CPU/CUDA sample and parity

Both fresh `pbrain sync --json` runs on the same 32 notes embedded all
59 passages from an empty index and cache. Three labeled `search
--semantic --json` queries returned their expected semantic hits in
both arms. One-minute load at the start of these commands was 11.21–14.54.

| Small fixture | CPU | CUDA |
| --- | ---: | ---: |
| Fresh sync wall time | 19.024 s | 2.551 s |
| Fresh sync peak host RSS | 2,085.7 MiB | 1,875.7 MiB |
| Median of three semantic searches | 520.9 ms | 1,081.4 ms |

The published SQLite indexes passed `PRAGMA quick_check` and had
the same index fingerprint, `e3c9a68ccccdf02cf42843d7d19fac984a5c6f922630f12217cbab68343509e9`.
All 59 pairs matched on item path, passage ordinal, span and digest.
Comparing the CPU and CUDA 768-dimensional stored vectors in float64
gave **minimum cosine 0.99999999999854**, at ordinal 0 of
`decisions/standing-rule.md`, and **median 0.99999999999955**. The vectors
differed bytewise. The untagged, model-present `TestGemmaParity` passed
with minimum cosine 0.99999999999862 across 22 raw and 22 prepared
reference texts checked at 512 and 768 dimensions.

#### First query and full indexing

CUDA rule 3 (`BenchmarkGemmaFirstQuery`, `-tags cuda -benchtime=1x
-count=1`) measured digest verification, model open, one query and close.
The five fresh processes used a primed pack. They took 1,049.7, 927.7,
1,272.3, 957.3 and 1,101.4 ms. The median was **1,049.7 ms**, and every
sample exceeded the 600 ms target. One-minute start load was 26.55–28.51.

The untagged CPU rerun (#33) used master
`c56bb5d07f6ec72564ea55db0592dfd872f1fd31` with goinfer v0.5.0 and
Go 1.27.3. On the Ryzen 9 7950X (CPUs 0–3, `GOMAXPROCS=4`, 124.9 GiB
RAM, Linux 7.3.4, btrfs), `BenchmarkGemmaFirstQuery` with
`-benchtime=1x -count=5` produced 353.3, 286.2, 284.0, 282.9 and
287.5 ms, with a **286.2 ms median (282.9–353.3 ms)**. All five
samples fit inside the earlier Intel CPU range of 253–366 ms and
met the 600 ms target. The Ryzen CUDA run above measured
927.7–1,272.3 ms in five fresh processes, with higher host load.
The CPU rerun ran all five samples in one test process with the pinned
pack already on disk. The one-minute load average before the benchmark
was 0.80.
[CPU benchmark command and raw samples](../../tools/phase28/evidence/gemma-pax-cpu-rule3-v05.log).

The matched fresh-process rerun (#32) built the untagged CPU and
`-tags cuda` benchmarks from `dd2c6c0a6cebac09751c542e1efeaebdd9396f87`,
both with goinfer v0.5.0. Five back-to-back CPU/CUDA pairs alternated
one `BenchmarkGemmaFirstQuery` iteration per process on CPUs 0–3.
The same pinned pack was read into the filesystem cache before timing.
The one-minute load average before each sample ranged from 1.27 to
1.77, and `nvidia-smi` showed no GPU compute processes before any sample.

| Build | Median | Range |
| --- | ---: | ---: |
| Untagged CPU | 353.2 ms | 347.1–357.6 ms |
| CUDA | 840.0 ms | 806.6–912.9 ms |

CUDA added a median 482.4 ms within pairs. Every CPU sample was below
600 ms, and every CUDA sample exceeded it.
[Matched commands, preflights and samples](../../tools/phase28/evidence/gemma-pax-rule3-matched.log).

The CUDA candidate's `TestMeasureReads/personal` retained the 5,000-item,
50,000,000-byte fixture and used an empty index and vector cache. The
first `pbrain --personal sync --json` embedded 43,642 shared and 4,869
personal passages. The next sync added one personal passage. The GPU
sampler read compute-process memory roughly every three seconds, giving
a sampled lower bound on device peak memory. The one-minute load was
25.33 before the first sync.

| CUDA sync | Wall time | Passages/s | Peak host RSS | Sampled device peak |
| --- | ---: | ---: | ---: | ---: |
| First sync, both sections | 4,181.3 s (1 h 9 m 41 s) | 11.60 | 4,758.0 MiB | 1,314 MiB |
| Sync after one new personal note | 2.386 s | — | 2,421.2 MiB | — |

#### Warm search on the CUDA-built index

CUDA timed search before the one-note sync. The CPU follow-up used the
same 4,500 shared and 500 personal items and byte-identical SQLite
generations preserved from the first CUDA sync. Each row used ten
fresh warm processes after a priming read. The first sample had
uncontrolled cache state. One-minute load was 11.35–15.78 during the
CUDA search series and 10.63–13.69 during the CPU series.

| Semantic search | CPU median [range] (ms) | CPU peak MiB | CPU ≥1 s | CUDA median [range] (ms) | CUDA peak MiB | CUDA ≥1 s |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Shared, exact hit | 772.8 [696.4–1,036.4] | 1,626.5 | 1/10 | 1,452.7 [1,289.6–1,786.2] | 2,317.9 | 10/10 |
| Shared, paraphrase | 779.7 [657.3–1,052.2] | 1,627.4 | 1/10 | 1,425.9 [1,312.6–1,743.2] | 2,295.0 | 10/10 |
| Shared, paraphrase + BM25 | 802.4 [730.5–1,038.0] | 1,626.3 | 1/10 | 1,486.5 [1,256.1–1,762.1] | 2,319.0 | 10/10 |
| Personal, exact hit | 820.1 [738.8–1,026.3] | 1,657.2 | 1/10 | 1,534.1 [1,333.4–1,794.5] | 2,380.1 | 10/10 |
| Personal, paraphrase | 802.5 [767.9–998.7] | 1,647.9 | 0/10 | 1,509.7 [1,297.7–1,783.6] | 2,341.8 | 10/10 |
| Personal, paraphrase + BM25 | 809.2 [687.4–1,103.1] | 1,686.8 | 1/10 | 1,510.5 [1,327.1–1,661.1] | 2,342.2 | 10/10 |

#### Bounded catch-up on the CUDA-built index

Each trial started from the same preserved shared generation, then
added one or 20 shared notes with one uncached passage each. The timed
command was `pbrain search 'where is login state kept' --semantic --json`.
The two builds ran in alternating order. Each trial paired a held-lock
control with a free-lock search on the same notes. The table reports ten
timed trials per case after one warm-up. One-minute start load was
9.94–16.33.

| Shared search gap | CPU median [range] (ms) | CPU saved | CUDA median [range] (ms) | CUDA saved |
| --- | ---: | ---: | ---: | ---: |
| One passage, lock held | 820.4 [677.9–1,095.1] | 0 | 1,593.4 [1,377.6–1,801.2] | 0 |
| One passage, lock free | 1,738.0 [1,332.8–1,956.1] | 1 | 2,281.2 [2,040.7–2,447.8] | 1 |
| 20 passages, lock held | 790.4 [688.3–1,105.8] | 0 | 1,497.5 [1,410.0–1,852.1] | 0 |
| 20 passages, lock free | 3,592.4 [3,406.4–3,870.8] | 15–19 | 2,721.5 [2,385.0–2,964.9] | 20 |

Held-lock searches returned `semantic_partial` before release and
preserved the generation. Every free-lock trial returned a new semantic
hit. CPU searches at the full bound reported the remaining gap, with
a median of 17 passages saved. CUDA saved the entire gap in each trial.
The median paired added wall time was 917.8 ms for one passage and
2,736.7 ms for 20 on CPU, and 742.4 ms and 1,290.6 ms on CUDA. The
two-second bound applies to starting another embedding after acquiring
the lock. The measured wall time includes query setup, scoring and
publication.

[Run plan](../../docs/plans/issue-22-gemma-cuda.md),
[build and model provenance](../../tools/phase28/evidence/gemma-pax-22/host-build-model.log),
[small fixture timings](../../tools/phase28/evidence/gemma-pax-22/small-timing.jsonl),
[CPU/CUDA vector comparison](../../tools/phase28/evidence/gemma-pax-22/cpu-cuda-passage-parity.jsonl),
[untagged parity test](../../tools/phase28/evidence/gemma-pax-22/cpu-reference-parity.log),
[CUDA rule 3](../../tools/phase28/evidence/gemma-pax-22/cuda-rule3.log),
[full CUDA run](../../tools/phase28/evidence/gemma-pax-22/cuda-full.log),
[device samples](../../tools/phase28/evidence/gemma-pax-22/cuda-device-samples.jsonl),
[retained index](../../tools/phase28/evidence/gemma-pax-22/retained-index.jsonl),
[CPU warm search](../../tools/phase28/evidence/gemma-pax-22/cpu-warm.log),
[catch-up samples](../../tools/phase28/evidence/gemma-pax-22/cpu-cuda-catchup.jsonl),
[load and GPU preflight](../../tools/phase28/evidence/gemma-pax-22/preflight.jsonl).

### Full-size bge-small sync memory profile (#19)

Profiled the default `bge-small-en-v1.5` at candidate
`b67cdff542605477c2312f085af7033590f2b638` on the same Ryzen 9 7950X host
as the earlier BGE rerun. The build used four physical cores (CPUs 0–3),
124.9 GiB visible RAM, Linux 7.3.4, btrfs and Go 1.27.3. The scratch
checkout held the original 4,500 shared and 500 personal items (exactly
50,000,000 bytes), the verified local model pack, empty section indexes
and an empty external vector cache. `pbrain --personal sync --json`
embedded 43,642 shared and 4,869 personal passages in one process. The
instrumented run finished in 1 h 53 m 55 s.

The scratch build samples Linux RSS before SQLite serializes a
checkpoint, after SQLite fills its own buffer and the driver copies
it to Go, and after SQLite frees its buffer. It captures a Go heap
profile and `smaps` at every new sampled RSS maximum. No forced GC or
GC tuning was used in the full run. The profiles use Go's default
sampling and reflect the last completed GC's heap accounting.
`HeapAlloc` at each probe is the current Go allocation count in bytes,
including allocations that may become garbage before the next GC.
Linux RSS includes resident idle Go heap pages and native allocator
mappings, so the profile and RSS have different totals.

The highest sample was **1,297.8 MiB** at shared item 4,259, just after
the driver freed SQLite's temporary buffer. The adjacent sample with
both buffers live was also 1,297.8 MiB. The kernel high-water counter
reported by GNU time was 1,296.9 MiB. At the peak, the Go heap arena
accounted for 885.3 MiB resident. At that probe, Go reported 790.4 MiB
allocated and 84.0 MiB idle and unreleased. Other mappings accounted
for 412.5 MiB. The heap arena is the anonymous mapping whose extent
matches the Go heap reservation and grows with its committed pages.
This kernel omits anonymous mapping names.

| Go allocation source at the peak | Sampled in-use MiB |
| --- | ---: |
| BGE weight bytes (`embed.readOne`) | 127.3 |
| Go serialization buffer (`sqlite.(*conn).Serialize`) | 82.7 |
| Completed passage vectors (`encoder.poolOne`) | 61.1 |
| Passage text (`semantic.Passages`) | 46.6 |
| Catalog file buffers (`io.ReadAll` in `Catalog.readOne`) | 44.5 |
| Encoder float32 workspace (`encoder.ensureF32`) | 24.2 |

The profile reports 400.7 MiB sampled in use at the most recent heap
accounting. The last GC was about 42 seconds before the peak sample.
Cumulative sampled allocations at that point were 42.0 GiB, led by
25.0 GiB in BERT forward passes and 11.1 GiB in Go serialization
buffers (flat allocation sites). This turnover helps explain the
distance between live allocations and resident heap.

The checkpoint allocated an 83.9 MiB SQLite buffer and an 83.9 MiB Go
copy of the database. The adjacent samples show RSS rise by only
0.3 MiB across those allocations and stay flat after the SQLite buffer
was freed. A separate anonymous mapping had 83.9 MiB resident across
the copy and free, and reached 88.5 MiB at the final shared publication
as the database grew to that size. This is consistent with SQLite's
allocator reusing resident pages. The rest of the non-Go RSS includes
the live SQLite database, allocator pools and runtime mappings. SQLite
memory-status accounting was disabled in the pinned build, so the
profile cannot assign the remaining bytes to individual native
allocations.

**Proposed first change.** Release completed work entries once their
passages have been inserted. The build keeps them reachable through the
`todo` slice while it processes later items, and the profile attributes
about 108 MiB to passage text and vectors at the peak. Also release the
catalog reference after the pending bodies have been selected, so the
buffers can become collectible as each body's work is consumed. The
catalog's roughly 45 MiB of file buffers gives less reason to change the
scan to stream whole files on this fixture. Repeated full serialization
is a larger source of churn. Reassess the peak after releasing the work
entries before redesigning checkpoint publication.

**Separate line-counting measurement.** Alternate `item.Measure` with
the same code-point count loop without newline tracking on the 5,000
preloaded UTF-8 files. Two warm-up pairs preceded 20 measured pairs on
one thread pinned within CPUs 0–3. Both variants returned the same
file and body code-point totals. Median thread CPU time was 29.8 ms
for full layout and 22.9 ms for code points alone. The median paired
cost of line counting was **7.0 ms** over the 50 MB (3.9–9.0 ms range).
The paired elapsed-time median was 7.1 ms. These are counting-loop
costs with file I/O, parsing and scan parallelism outside the timer.

The CUDA Gemma host peak above remains an RSS measurement. A second
full CUDA build would be needed for a comparable allocator profile.

Evidence and exact commands are in
[`tools/phase28/evidence/bge-pax-19`](../../tools/phase28/evidence/bge-pax-19),
including [the peak heap profile](../../tools/phase28/evidence/bge-pax-19/peak-heap.pb.gz),
[publication samples](../../tools/phase28/evidence/bge-pax-19/memory-samples.jsonl),
and [the line-counting pairs](../../tools/phase28/evidence/bge-pax-19/line-cost.jsonl).
To reconstruct the profiler, archive the candidate into `src/` and
copy the pinned modernc.org/sqlite v1.62.0 source into `sqlite/` beside it.
Apply the decompressed `profiling.patch.gz` from their parent directory,
then use the build and run commands in `host-build-model.log` with an
empty cache, empty section index directories in a scratch copy of the
fixture, and the local model pack. The patch is for the scratch build.

### Full-size bge-small sync after releasing completed work (#34)

Compared the #19 baseline (`b67cdff542605477c2312f085af7033590f2b638`)
with candidate `3b38ee7280c07d61088b18dd38662a45a45e21df`. Both used
the same profiling patch, verified BGE pack, and 5,000-item, 50,000,000-byte
corpus on the Ryzen 9 7950X with CPUs 0–3, 124.9 GiB RAM, Linux 7.3.4,
btrfs and Go 1.27.3. The candidate used an empty vector cache and empty
indexes. The sorted Markdown content hashes matched the baseline.
Each run embedded all 43,642 shared and 4,869 personal passages.

| Peak metric | Baseline | Candidate |
| --- | ---: | ---: |
| Sampled RSS | 1,297.8 MiB | 997.8 MiB |
| Maximum RSS from GNU time | 1,296.9 MiB | 996.9 MiB |
| Go heap arena RSS at the sampled peak | 885.3 MiB | 579.1 MiB |
| Go heap profile in-use sample total | 400.7 MiB | 234.7 MiB |
| Shared items indexed at the sampled peak | 4,259 | 4,408 |

Sampled peak RSS fell by **300.0 MiB (23.1%)**. The candidate peaked
during a checkpoint with both 86.8 MiB serialization buffers live.
The Go heap arena accounted for the reduction. In the candidate peak
profile, file buffers were down from 44.5 to about 1 MiB, and passage
text and vectors had no sampled live allocations. The peak profiles
reflect different points in the GC cycle. The candidate took
1 h 24 m 46 s, with one-minute host load averaging about 11 at the
start and about 4 later in the run.

Both final SQLite databases passed `PRAGMA quick_check` on each run.
The shared database was byte-identical. The personal files differed
by 121 bytes, all on the `meta` table's page. The four metadata rows
matched, and their insertion order can vary because the build ranges
over a Go map. Ordered comparisons of every column in `item` and
`passage` matched across both sections, including all 48,511 vector
blobs.

See the [run provenance](../../tools/phase28/evidence/bge-pax-34/host-build-model.log),
[load and GPU checks](../../tools/phase28/evidence/bge-pax-34/preflight.jsonl),
[RSS samples](../../tools/phase28/evidence/bge-pax-34/memory-samples.jsonl),
[peak Go profile](../../tools/phase28/evidence/bge-pax-34/peak-heap.pb.gz),
and [database comparisons](../../tools/phase28/evidence/bge-pax-34/database-comparison.jsonl).

## Release build and install

Build the release binary with `CGO_ENABLED=0` for Linux and Darwin on both
amd64 and arm64. The Darwin binaries are only built: running them is
deferred with macOS (architecture §17). Once a release tag is available,
verify versioned install in a scratch directory by setting `GOBIN` there
and running
`go install github.com/pkieltyka/pbrain/cmd/pbrain@"$PBRAIN_RELEASE_TAG"`.
Run the installed binary's `--version` and record the result with the
timings.

## Full-size sync with BGE workers and CUDA batching on pax (#43)

2026-11-22. Candidate `004e7e3b16661358b306a4f7701ba412cd62558b`, goinfer
v0.5.0, Go 1.27.3, `CGO_ENABLED=0`, `-trimpath` and `-ldflags '-s -w'`.
Ryzen 9 7950X, 16 physical cores and 32 threads, 124.9 GiB visible RAM,
Linux 7.3.4 and btrfs. The CUDA binary ran on an NVIDIA RTX PRO 6000 Blackwell
Max-Q with driver 595.112.07 and 97,887 MiB total memory. BGE used the untagged
binary. Gemma used the `cuda` tag and explicitly selected NVIDIA `cuda:0`.

Each timed run was a complete `pbrain --personal sync --json` of the same
5,000 items and 50,000,000 Markdown bytes. The model files were primed, and
each run started with a clean index and a private empty cache. Each embedded
43,642 shared and 4,869 personal passages, with zero reuses. The process
maximum RSS is from Linux `wait4`. The NVIDIA monitor sampled every 250 ms.
Before each run, two successive 10-second samples showed at most 4% total host
CPU busy and an idle NVIDIA compute queue. The evidence also records load
averages, which still included the preceding run at some starts.

| Run | CPU affinity / GOMAXPROCS | Samples | Wall time median (range) | Passages/s median (range) | Peak host MiB median (range) |
| --- | --- | ---: | ---: | ---: | ---: |
| BGE, 16 workers | 0–15 / 16 | 3 | 21m 01s (20m 58s–21m 17s) | 38.46 (37.98–38.56) | 1,664.6 (1,576.9–1,744.1) |
| BGE, 32 workers | 0–31 / 32 | 3 | 20m 25s (20m 13s–20m 26s) | 39.60 (39.56–40.00) | 1,814.5 (1,704.5–1,909.6) |
| Gemma 2, NVIDIA CUDA | 0–3 / 4 | 1 | 1h 1m 02s | 13.25 | 4,317.5 |

The initial 16-core trial reached 37.76 passages/s. Unrelated work used at
least four logical CPUs for 6 consecutive 10-second intervals during the
run. It is retained in the raw evidence and replaced in the timing comparison
by a clean 16-core pass. The shared and personal BGE databases were
byte-identical across all runs, including that initial trial. The ordered
rows, vector blobs and SQLite `quick_check` results also matched.

At full size, 32 workers improved median BGE throughput by
3.0% over 16 workers on this host, with
9.0% higher median peak RSS. The smaller slice used in the
worker design showed a larger throughput difference. The worker cap stays at
32.

The Gemma sync selected a 6,144 MiB batch budget and used at most
2,136 MiB of sampled device memory, including
2 MiB at idle. Mean sampled device utilization was
98.1% and the median was 100%.
The earlier #22 run on the same host reached 11.60 passages/s
in 1h 10m, with 4,758 MiB peak host RSS and 1,314 MiB sampled device
memory. The current run was 14.2%
faster by passage rate, with newer batching and lower host load.

[Commands and raw measurements](../../tools/phase28/evidence/sync-pax-43/)

## EmbeddingGemma 2 GGUF on goinfer v0.6.1 (#47)

2026-11-22. The scratch evaluator used the published `LoadGGUFEncoderReaderAt`
and `Encoder.Tokenizer` APIs with verified files. The control was the pinned
Google pack in the same evaluator. The root and CUDA modules were both
v0.6.1. All runs used four physical cores of a Ryzen 9 7950X on pax, with
primed file caches. CUDA runs selected the RTX PRO 6000.

Decode is the median of three opens from verified bytes. First-query time
is the median of five fresh processes and includes verification, open,
one query and close. Peak host RSS is the maximum of those five processes.
Settled host RSS is the median after the diagnostic query and garbage
collection, with the model live. Device memory is sampled at 100 ms.

| Pack | Bytes | Decode ms, CPU / CUDA | First query ms, CPU / CUDA | Host peak/settled MiB, CPU | Host peak/settled MiB, CUDA | CUDA peak MiB |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Google | 551,641,057 | 75.9 / 77.6 | 347.3 / 1,009.4 | 1,100.7 / 1,101.0 | 1,804.4 / 1,782.3 | 1,082 |
| Q8_0 | 309,855,520 | 636.5 / 639.9 | 933.8 / 1,263.4 | 1,400.9 / 1,399.5 | 1,589.2 / 1,573.0 | 964 |
| UD-Q6_K_XL | 248,812,032 | 796.2 / 793.6 | 1,035.3 / 1,425.5 | 1,341.2 / 1,341.5 | 1,531.7 / 1,516.8 | 1,084 |

The Google pack met the 600 ms CPU rule in all five samples (336.9–354.3 ms).
Q8_0 (909.5–955.5 ms) and UD-Q6_K_XL (1,022.8–1,048.6 ms) missed it in all
five. CUDA times measure the adapter directly. Production search uses CPU.

Both GGUFs passed rule 2 against the same pinned file loaded by the
llama.cpp CPU probe at `8d60873dfb8619f2b5ed31adfbab07cccb40db5c`. The
adapter and probe used matching prepared IDs for 22 reference texts and
59 passages. Every one of the 162 comparisons per device and quant met
cosine 0.999, at 768 dimensions and the normalized 512-prefix.

| Quant | Same-file minimum cosine, CPU / CUDA | Vs Google, 59 passages min / median | Vs Google, 1,223 CUDA passages min / median | Dense top-10 overlap / product Jaccard vs Google |
| --- | ---: | ---: | ---: | ---: |
| Q8_0 | 0.999851 / 0.999851 | 0.999839 / 0.999923 | 0.999491 / 0.999869 | 99.61% / 100.00% |
| UD-Q6_K_XL | 0.999107 / 0.999107 | 0.998660 / 0.999423 | 0.994587 / 0.998899 | 97.66% / 99.24% |

The 59-passage and ranking comparisons round the same on both devices.
All three models independently fitted 0.72 on the calibration negatives
and passed both retrieval gates on CPU- and CUDA-built indexes. Each gave
39/47 first, 42/47 top three and first-page hits, and 30/30 held-out
abstentions. Product Jaccard covers the 44 nonempty unions of 77 held-out
queries; 33 pairs had two empty results. The quality and rule-2 gates
passed, and the CPU first-query gate still prevents admission of either quant.

[Commands and raw measurements](../../tools/phase28/evidence/gguf-47/)

### GGUF first query and same-file parity on goinfer v0.6.2

2026-11-22. The #47 scratch evaluator used the v0.6.2 pins from
`d5129b3b880ed0f361f07f8876700ebd60ab0ddd`, with the same verified-file
adapter, model files and query. Five fresh CPU processes per pack used
primed files and four physical Ryzen 9 7950X cores on pax. The timed
operation includes verification, loading, one query and model close.

| Pack | CPU median ms, v0.6.1 → v0.6.2 | v0.6.2 range ms | Samples below 600 ms | Peak host RSS MiB | Same-file minimum cosine, CPU / CUDA |
| --- | ---: | ---: | ---: | ---: | ---: |
| Google control | 347.3 → 356.5 | 348.4–378.1 | 5/5 | 1,110.3 | — |
| Q8_0 | 933.8 → 562.9 | 558.0–603.6 | 4/5 | 1,033.3 | 0.9998506099 / 0.9998506078 |
| UD-Q6_K_XL | 1,035.3 → 540.6 | 527.8–557.1 | 5/5 | 933.8 | 0.9991069286 / 0.9991069451 |

Q8_0 exceeded 600 ms once on this desktop. UD-Q6_K_XL stayed below
600 ms in all five samples. Both satisfied rule 2 in all 162 comparisons
per device and quant against the pinned llama.cpp CPU probe, with the same
prepared IDs for the 22 reference texts and 59 passages. The probe compared
768 dimensions and normalized 512-dimensional prefixes. The selectable
model rows are unchanged.

[Commands and raw v0.6.2 measurements](../../tools/phase28/evidence/gguf-47-v062/)

## Vulkan sync qualification on goinfer v0.6.2 (#49)

2026-11-22 on pax, using the Google EmbeddingGemma 2 pack. The RX 9060 XT
passed the saved 59-passage reproduction against CPU, with minimum cosine
0.999999999998413. All 59 Vulkan vectors were byte-identical across
singleton, default, reversed and 1 MiB layouts. The CUDA/CPU minimum
cosine was 0.999999999998539.

The first 200 passages of the full-corpus run supplied the matched slice
of 100,066 positions. Each test used the desktop's default `GOMAXPROCS=32`.
The timer covers preparation and embedding with the verified model
already open. Both GPU runs requested a 1 GiB batch workspace.

| Device | Seconds | Passages/s |
| --- | ---: | ---: |
| Vulkan · AMD Radeon RX 9060 XT | 49.24 | 4.06 |
| CUDA · NVIDIA RTX PRO 6000 Blackwell Max-Q | 14.85 | 13.46 |
| CPU · AMD Ryzen 9 7950X | 114.57 | 1.75 |

Vulkan was 2.33 times as fast as CPU on this slice. On the full corpus
with four allowed CPUs, the six complete 256-passage windows ran at
0.928–0.975 passages/s, about 0.95. That run ended at 30 minutes by
operator decision, with a readable checkpoint. See the separate
worker settings, window positions, forward breakdown and VRAM samples
in the [Vulkan capture](../../tools/phase28/evidence/vulkan-49-v062/).
