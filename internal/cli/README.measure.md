# Release measurements

The first release measurements ran on Linux. Issue #69 adds Apple Silicon
Q8 indexing and CPU controls. Architecture §17 tracks the remaining
macOS measurements. Keep the machine idle and
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
amd64 and arm64. Darwin arm64 Metal is qualified for Q8 indexing and CPU
search on Apple M4. The broader Darwin test and workload coverage remains
deferred (architecture §17). [Completed M4 qualification](#apple-silicon-metal-q8-indexing-69)

Once a release tag is available,
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

## Q8_0 release qualification on goinfer v0.6.2 (#51)

2026-11-22 on pax at `694b726564c1ad139ece70bcdd60b433d3a18053`, with
root goinfer and CUDA v0.6.2. Runs used four physical Ryzen 9 7950X
cores (CPUs 0–3), 124.9 GiB RAM, Linux 7.3.4 and Go 1.27.3. CUDA
runs used the NVIDIA RTX PRO 6000 Blackwell Max-Q. The tests and smoke
commands used the pinned 309,855,520-byte Unsloth Q8_0 file and the
release row `embeddinggemma-2-q8-0`.

| Gate | Result |
| --- | --- |
| CPU rule 3, five fresh processes with a primed model file | 586.6 ms median, 561.5–594.0 ms, 5/5 below 600 ms, 1,034.5 MiB peak RSS |
| CPU minimum cosine against the same-file llama.cpp reference | 0.9998506099 |
| CUDA minimum cosine against the same-file llama.cpp reference | 0.9998506078 |
| Direct CPU/CUDA parity, 59 passages at 768 dimensions | 0.999999999998416 minimum cosine |
| CUDA byte identity | All 59 vectors matched singleton outputs in packed, 17-input split and reversed layouts |
| Held-out retrieval at the frozen 0.72 cutoff | Both CPU- and CUDA-built indexes gave 39/47 first, 42/47 top three and first page, and 30/30 empty pages on unanswerable queries with and without BM25 |

Rule 3 includes file verification, model load, one query and close. The
five samples were 586.6, 586.8, 594.0, 561.5 and 576.3 ms. The slowest
sample was 6.0 ms below the target on this host. Same-file parity used
matching prepared tokens for 22 reference texts and 59 passages, with all
162 comparisons per device meeting cosine 0.999 across 768 dimensions
and the normalized 512-prefix. The standalone retrieval gate found the
expected note on 42/47 first pages against 16/47 for strict search.

The release CLI indexed and found the same one-note fixture with a local
offline `model_path`, with `--device auto` selecting `cuda:0`, and after
a fresh-cache download. The download emitted the 310 MB model notice and
installed one file with the expected SHA-256. Q8_0 uses CPU for search
and supports CPU and CUDA for sync.

[Commands and raw gate evidence](../../tools/phase28/evidence/gguf-51/)

## goinfer v0.6.3 device and batch qualification (#56)

2026-11-22 on pax with a Ryzen 9 7950X, 124.9 GiB RAM, Linux 7.3.4 and
Go 1.27.3. The root, CUDA and Vulkan modules all resolve to goinfer
`050fdffba776`. The candidate was `b09762d`, the pin-only control
`2e599bd`, and the exploratory uncapped build `81fa599`.

The RTX PRO 6000 Blackwell Max-Q matched slice used 200 saved passages
with 100,066 positions. The timer covered preparation and embedding
with a verified Google pack already open. Each default-budget series
ran three alternating pairs with `GOMAXPROCS=32`, a 6 GiB batch budget,
and 100 ms device samples.

| CUDA slice | Passages/s, median (range) | Forwards | Largest positions | Sampled device peak |
| --- | ---: | ---: | ---: | ---: |
| Initial pin-only control | 13.512 (13.509–13.588) | 13 | 8,106 | 2,130 MiB |
| Uncapped trial | 13.769 (13.762–13.770) | 3 | 47,436 | 7,188 MiB |
| Pin-only control for the capped rerun | 13.505 (13.503–13.509) | 13 | 8,106 | 2,130 MiB |
| Capped candidate | 13.503 (13.499–13.505) | 13 | 8,106 | 2,130 MiB |

The uncapped trial gained 1.9% and used 5,058 MiB more device memory.
The operator retained the 8,192-position aggregate cap for both GPUs
and waived the paired full CUDA syncs. `BatchMemory` can shorten a
capped group. The capped slice reported up to 1,098,090,784 additional
workspace bytes and 547,139,584 resident weight bytes. Device samples
also include context and driver memory. Peak host RSS was 1,873 MiB
for the capped candidate and 1,869 MiB for its matched control. The
1 GiB control reproduced #49 at 13.48 passages/s versus 13.46.
All slice vector bytes matched across both series and both overrides.

On CUDA, both the Google and the Q8_0 packs gave all 59 singleton
vectors byte for byte in the default 7,712 + 1,559 layout. A forced
retry-safe failure on the first group replaced it with groups of 3,669,
3,711 and 332 positions, followed by the original 1,559 on the same device.
Every vector remained identical. The direct CPU/CUDA minimum cosine
on 59 passages was 0.999999999998540 for Google and 0.999999999998416
for Q8_0. Both satisfied the same-file reference gate on CPU and CUDA.

From independently copied cold and seeded initial states, real CUDA
syncs of the reuse fixture kept the same counts, cache paths and
payloads, ordered rows, and database bytes for both sections across
the 1 MiB, default and forced-retry layouts. The fixture exercised
duplicate passages, competing base rows, a shared-cache-only hit and
a personal-only passage whose digest stayed out of the shared cache.

Q8_0 CPU rule 3 on v0.6.3 used four physical cores and five fresh
processes with warm files. Verification, load, one query and close
took 597.1, 587.8, 592.8, 599.0 and 583.9 ms. The median was 592.8 ms
and all five were below 600 ms. Peak host RSS was 1,032.1 MiB.

On the Radeon RX 9060 XT with Mesa 26.3.1, all 59 singleton vectors
matched the 1 GiB, reversed and 1 MiB Vulkan layouts by bits. The
CPU/Radeon minimum cosine was 0.99999999999841. At `GOMAXPROCS=32`,
the 200-passage slice measured 4.054 passages/s on Radeon and
1.880 on CPU. All 200 matched at cosine 0.999 or higher, with a
minimum of 0.999999999995588 and zero cross-device bit matches.
The Radeon slice peaked at 1,282 MiB of sampled attributable VRAM.

The four-core Radeon partial sync used a 1 GiB workspace and ran for
656.5 seconds before the operator's SIGINT. It left 241 items and
2,357 passages in a checkpoint that passed SQLite's `quick_check`.
Peak sampled VRAM attributable to the run was 1,308 MiB, below the
4 GiB total-process cap. The AMD driver logged a nonzero VM-memory
counter on shutdown in the slice and partial sync. The guard observed
no device loss or ring reset.
The CPU verification sampled 500 of the first 2,098 checkpointed
passages in path and ordinal order without replacement, with seed 56.
All 500 passed cosine 0.999, with a minimum of 0.999999999963812 and
zero cross-device bit matches.

After the v0.3.1 parser fix was merged into the branch, a dual-tag
binary with CUDA masked and a Vulkan loader with no available ICD
listed only the CPU. A one-note sync used CPU with the
`cpu_fallback` reason “no usable GPU was found”.

[Commands and raw gate evidence](../../tools/phase28/evidence/goinfer-56/)


## Managed model verification on Linux (#59)

On pax (Ryzen 9 7950X, Linux 7.3.5, btrfs), source `4e949dc` and
baseline `d8384b1` used the same Go 1.27.3 untagged build with cgo disabled.
Each model used a two-note ready brain built by normal sync. The candidate
sealed fresh cache inodes while reusing that index. The original cache stayed
unsealed for the baseline and candidate fallback arms. Model data was warm
in the filesystem cache. Before each model series, two five-second host
samples were at most 4% busy and listed no NVIDIA compute process.

Rule 3 sums model verification, CPU open, one query and close on the
production `search --semantic` path. Each cell lists five fresh processes in
run order, pinned to CPUs 0–3 with `GOMAXPROCS=4`. The raw control times the
existing adapter benchmark with ordinary `Verify`.

| Model | Arm | Rule 3 samples (ms) | Median (ms) | Range (ms) | Verify median (ms) | CLI wall median (ms) |
|---|---|---|---:|---:|---:|---:|
| bge-small | Baseline | 119.1, 111.0, 112.7, 111.9, 113.5 | 112.7 | 111.0–119.1 | 93.1 | 130.1 |
| bge-small | Sealed | 63.7, 65.0, 61.3, 61.1, 61.2 | 61.3 | 61.1–65.0 | 41.9 | 79.3 |
| bge-small | Unsealed fallback | 112.0, 112.4, 111.9, 111.6, 111.1 | 111.9 | 111.1–112.4 | 92.7 | 129.6 |
| bge-small | Raw Verify benchmark | 115.9, 111.1, 111.5, 114.4, 112.4 | 112.4 | 111.1–115.9 | — | — |
| Google pack | Baseline | 346.2, 342.9, 350.5, 341.7, 342.2 | 342.9 | 341.7–350.5 | 196.0 | 434.1 |
| Google pack | Sealed | 241.7, 239.3, 241.8, 241.3, 239.8 | 241.3 | 239.3–241.8 | 92.7 | 331.9 |
| Google pack | Unsealed fallback | 343.2, 340.1, 340.0, 348.4, 346.3 | 343.2 | 340.0–348.4 | 194.4 | 435.0 |
| Google pack | Raw Verify benchmark | 344.9, 343.3, 346.7, 347.6, 342.5 | 344.9 | 342.5–347.6 | — | — |
| Q8_0 | Baseline | 566.3, 552.6, 539.8, 552.5, 551.9 | 552.5 | 539.8–566.3 | 213.5 | 634.1 |
| Q8_0 | Sealed | 437.8, 437.2, 459.7, 434.2, 424.9 | 437.2 | 424.9–459.7 | 93.6 | 520.3 |
| Q8_0 | Unsealed fallback | 560.2, 548.0, 565.3, 553.7, 543.7 | 553.7 | 543.7–565.3 | 210.3 | 637.7 |
| Q8_0 | Raw Verify benchmark | 592.3, 558.1, 559.9, 553.6, 557.2 | 558.1 | 553.6–592.3 | — | — |

Q8_0 saved 115.4 ms versus the matched baseline and finished under 600 ms in
all five sealed runs. Its verification stage still took 93.6 ms for the
authenticated read and private copy. The sealed runs recorded one kernel
digest hit per file on every invocation (5 for bge-small, 4 for the Google
pack, 1 for Q8_0). Neither strict arm recorded hits. The query-vector hashes,
ranked output and notices matched byte-for-byte across all three production
arms for each model. The ordinary path still checks full SHA-256 for
unsealed caches and external `model_path` files.

[Raw records and measurement driver](../../tools/phase28/evidence/model-verity-59/)

## CUDA document precision (#61)

The paired syncs ran on pax with a Ryzen 9 7950X and an RTX PRO 6000
Blackwell Max-Q (compute capability 12.0, driver 595.112.07). They
used Go 1.27.3, goinfer v0.6.4 at `ab2721995e02`, the Linux amd64
GPU build tags, CPUs 0–3 and `GOMAXPROCS=4`. Each sample started
with two five-second measurements at or below 4% host CPU busy and
no NVIDIA compute process. Only one workload ran at a time. Each
invocation had 115 seconds before SIGINT, with a five-second grace period.

Real `pbrain sync` built cold indexes and vector caches from the same
22 shared notes in the #43 corpus (fixture SHA-256
`bbf4f8235b28a52979fe0121e1c8ed58e8a45904095649bfeea8d81bba04c117`).
Both precisions embedded 219 passages and 110,157 prepared positions
in 14 forwards per run. There were two pairs per row with the precision
order reversed in the second pair. Each sync opened the model and attached
once. All used the default 6 GiB workspace budget.

| Model | CUDA | Wall time (s) | Passages/s | Positions/s | Peak host RSS (MiB) | Sampled GPU peak (MiB) |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Google pack | fp32 | 17.39 | 12.59 | 6,334 | 1,915 | 2,130 |
| Google pack | fp16 | 3.64 | 60.12 | 30,238 | 2,030 | 1,782 |
| Q8_0 | fp32 | 17.46 | 12.54 | 6,309 | 1,483 | 2,130 |
| Q8_0 | fp16 | 3.93 | 55.76 | 28,046 | 1,548 | 1,782 |

Rates and wall times are medians. Memory is the higher peak of the two
samples, with device memory and utilization sampled every 100 ms. The Google
pack ran 4.77× faster and Q8_0 4.45× faster. The highest sampled host RSS
increased by 115 MiB for Google and 65 MiB for Q8_0, while device peaks fell
by 348 MiB for both. Across both trials the sample median of device utilization
was 100% for fp32, 79% for Google fp16 and 77% for Q8_0 fp16 (including setup
and teardown).

| Model / CUDA | Attach (ms) | Prepare (ms) | Forward (ms) | Publish (ms) |
| --- | ---: | ---: | ---: | ---: |
| Google / fp32 | 145.6 | 42.9 | 16,248.8 | 5.7 |
| Google / fp16 | 306.2 | 46.1 | 2,454.1 | 5.7 |
| Q8_0 / fp32 | 146.2 | 44.9 | 16,158.9 | 10.0 |
| Q8_0 / fp16 | 492.3 | 37.2 | 2,352.7 | 5.4 |

The adapter replay used the saved 200 passages and 100,066 prepared
positions from #58 (prepared-ID SHA-256
`97c426da8c155b04bf0b34e270dcfa5e8cf18bfc1343e926f63e638fea6bcd22`).
All four arms used the same IDs in 13 forwards within the 8,192-position cap.
Including preparation and packing, Google moved from 6,761 to 44,108
positions/s and Q8_0 from 6,806 to 46,137 positions/s. Forward-only figures
were 6,780 to 44,930 and 6,827 to 46,937 positions/s respectively. The
end-to-end sync above also includes verification, decode, attachment and
publication.

Production sync of the frozen 32-note retrieval subset matched all 59 ordered
passage keys, digests and spans across arms. Every vector was finite and
unit-normalized. The bits changed between precisions for every passage on
both model rows.

| Model | fp16 compared with | Minimum cosine | Median cosine | Below 0.999 |
| --- | --- | ---: | ---: | ---: |
| Google pack | CUDA fp32 | 0.999999946101 | 0.999999981474 | 0/59 |
| Google pack | CPU | 0.999999945920 | 0.999999981463 | 0/59 |
| Q8_0 | CUDA fp32 | 0.999999899604 | 0.999999956185 | 0/59 |
| Q8_0 | CPU | 0.999999899598 | 0.999999956195 | 0/59 |

On each model, the 59 fp16 vectors matched singletons bit for bit across
default, reversed, 1 MiB and injected certified-OOM retry layouts on the
same encoder. The default used groups of 7,712 and 1,559 positions. The
retry replaced the first group with 3,669, 3,711 and 332 positions. Real
pbrain syncs on both rows with that forced split produced identical item
and passage rows and all 59 shared vector-cache payloads compared with
ordinary fp16. The largest group in the layout tests was 8,172 positions.
Device memory sampled across the layout tests peaked at 1,786 MiB.

CPU search then read each precision's cold-built index of the full 35-note
retrieval fixture at cutoff 0.72. For each model and both `--semantic` and
`--semantic --bm25`, the ordered result IDs matched fp32 on all 77 queries.
There were no rank or threshold crossings. Each Gemma arm found the expected
note first on 39 of 47 questions, in the top three and on the first page on
42 of 47, and returned empty pages on all 30 held-out negatives. Both
standalone and combined admission gates passed. Strict search found 16 of
47 on the first page, and fp16 kept all 16.

The default was enabled after those checks. A one-note rollout test for
each model confirmed that the default attached `cuda-eg2-fp16`, the explicit
fp32 opt-out attached plain `cuda`, and switching back restored the original
fp16 fingerprint with one cache hit and zero new forwards. The model-free
tests also cover old or unavailable capability, UUID matching, shared and
personal cache isolation, checkpoint/resume and concurrent CPU catch-up.

The Linux amd64 release build passed CPU-only startup, device listing and
one-note sync with both GPU backends masked. The Linux arm64 release build
with the same CUDA kernel tags passed cross-build, vet, the linked-selector
test and the equivalent CPU fallback smoke under QEMU with arm64 glibc
2.39. The arm64 checks were CPU-only.

[Raw gate records and commands](../../tools/phase28/evidence/cuda-fp16-61/)

## GGUF on Vulkan device acceptance (#65)

2026-11-23. The accepted device run built pbrain `d6cc3425edbc` with Go 1.27.3,
`GOWORK=off CGO_ENABLED=0`, Linux amd64 and `-tags vulkan`, against published
goinfer root and Vulkan v0.6.6 (`c0a5159bb`). The branch adds only a verifier
fix and this note after that commit. The host was a Ryzen 9 7950X desktop with
a Radeon RX 9060 XT (RADV GFX1200), Mesa RADV 26.3.1 and the pinned Vulkan
loader/validation layers. The owner-approved 4.0 load cap applied; every other
guard was unchanged.

Each quant ran a fresh CPU and Vulkan sync over the frozen 32-note retrieval
subset, giving 59 passages in one shared section. Each cell had a new brain,
cache, HOME and trace; four Go scheduler threads; and, for Vulkan, a 1 GiB
batch workspace. The named persistent selector and its alias were the pinned
RX 9060 XT. Launch loads were 3.22–3.88. There were no child swap, host pageout,
timing voids, resets or validation errors.

| Quant / device | Wall s | Passages/s | Peak host RSS MiB | Attributed peak MiB | Device delta MiB |
| --- | ---: | ---: | ---: | ---: | ---: |
| Q8_0 / CPU | 16.487 | 3.579 | 1,485.2 | — | — |
| Q8_0 / Vulkan | 6.241 | 9.454 | 1,264.4 | 1,285.0 | 1,243.7 |
| UD-Q6_K_XL / CPU | 15.958 | 3.697 | 1,807.8 | — | — |
| UD-Q6_K_XL / Vulkan | 6.180 | 9.548 | 1,173.7 | 1,285.0 | 1,251.9 |

These are descriptive, end-to-end sync observations with tracing on. They are
not a speed acceptance gate, and no cross-device or cross-machine ratio is
claimed.

Every accepted cell had 32 items, 59 newly embedded passages, zero reused,
`PRAGMA quick_check` = `ok`, and finite unit 768-dimensional vectors. Item
paths and content digests, and passage identity, ordinal, start, stop and
digest, matched between the CPU and Vulkan cells for each quant. The minimum
CPU/Vulkan cosine was 0.9999999999975044 for Q8_0 and 0.9999999999961869 for
UD-Q6_K_XL.

The layout stage used the same 59 passages and took 96.52 s. Packed, reversed,
17-input-split and 1 MiB-budget layouts each differed from the Vulkan singleton
vectors in 0 of 59 passages for both quant. Its CPU/Vulkan minima were
0.9999999999975040 / 0.9999999999974988 (Q8_0) and 0.9999999999961872 /
0.9999999999963046 (UD-Q6_K_XL) at 768 dimensions and the normalized 512-prefix.
It sampled 6,596 Radeon attribution readings, with a 1,330.4 MiB attributed
peak and 1,294.8 MiB device delta.

The accepted evidence seal is
`c0145f00b2a91752435c1e3ba2d04c8e6970f615294fe9e1951119fbd480d4ce` over 804
files; its manifest is
`3a196c34bc1b1155dcabf874a2af6cec1654715f65357e04dd7a2df669712a05` and its
verification record is
`374684c3e88f9bb484c9e053dcd4d9ccc2bc20bce15cf2eb8149c2384601d0d2`. The
`pbrain` binary is
`64a905ff91bb4f6ad8bf681d36337bbdfa27839de5d837a716e248d79bc2f396`; the
layout test binary is
`dc4d27c912c363bb4d9772f181f828f5cb48bb33d13b0878f2b8f61e09fd9ff8`. The
Q8_0 and UD-Q6_K_XL file pins are `6f1bd4ac…` (309,855,520 B) and `dc84f042…`
(248,812,032 B); the frozen fixture list is `77bab592…` and its path list
`3726e16e…`.

Q8_0 keeps its existing CUDA fp16 identity. UD-Q6_K_XL is an opt-in row with no
fp16 profile; the default model, the CPU search path and the CUDA fp16 behavior
are unchanged. Raw indexes, databases and scratch binaries remain private and
are not part of the repository.

## goinfer v0.6.7 CUDA sync (#66)

On pax (Ryzen 9 7950X, RTX PRO 6000 Blackwell Max-Q, compute capability 12.0,
driver 595.112.07), we compared pbrain `d48cf1c` against `9c2b2a0`. The
builds differed in the goinfer v0.6.6/v0.6.7 pins and the required fp16
revision constant. Both used Go 1.27.3, `CGO_ENABLED=0`, `-trimpath` and the
Linux amd64 release tags `cuda,vulkan,eg2_combined,eg2_fp16`. All three
v0.6.7 modules came from the public proxy with checksum database verification
at `c26c69ed44717d86cf2218370054d5e89886b694`.

Each trial ran a complete `pbrain sync` with a fresh SQLite index and vector
cache and four physical cores (CPUs 0–3, `GOMAXPROCS=4`). The CUDA cases
used one RTX UUID and the default 6 GiB batch budget. The locally available
pinned files were verified and loaded inside the timer. Two matched pairs
per case ran in opposite version order under the exclusive timing lock.
Before each sync, two consecutive five-second samples showed at most 4%
host CPU busy and no NVIDIA compute process. Each sync had a 115-second
timeout with a five-second grace period. Tracing recorded one model open
per sync and one device attach per CUDA sync. The bge-small CPU control
ran afterward under the same timing and host checks. GPU memory is the
highest device-wide sample across both trials, sampled every 100 ms.
Wall times and rates are medians.

The larger fixture was the #61 slice of 22 notes and 219 passages
(digested path SHA-256 `bbf4f8235b28a52979fe0121e1c8ed58e8a45904095649bfeea8d81bba04c117`).
On Gemma it used 110,157 prepared positions in 14 calls. The 32-note #65/#22
fixture gave 59 passages and 9,271 positions in two Gemma calls. The longest
input in every Gemma call was at least 128 tokens. Gemma token counts and
batch shapes matched between versions.

| Passages | Model / device mode | Wall s, v0.6.6 → v0.6.7 | Passages/s, v0.6.6 → v0.6.7 | Peak GPU MiB, v0.6.6 → v0.6.7 | Exact vectors in each pair |
| ---: | --- | ---: | ---: | ---: | ---: |
| 219 | UD-Q6_K_XL / fp32 | 18.52 → 6.86 | 11.82 → 31.93 | 2,130 → 2,130 | 219/219 |
| 219 | Q8_0 / fp32 | 18.39 → 6.79 | 11.91 → 32.27 | 2,130 → 2,130 | 219/219 |
| 219 | Google pack / fp16 | 6.00 → 5.96 | 36.49 → 36.72 | 1,782 → 1,782 | 219/219 |
| 219 | Q8_0 / fp16 | 6.27 → 6.29 | 34.91 → 34.80 | 1,782 → 1,782 | 219/219 |
| 219 | bge-small / CPU | 19.31 → 19.52 | 11.34 → 11.22 | — | 219/219 |
| 59 | UD-Q6_K_XL / fp32 | 2.99 → 2.49 | 19.75 → 23.74 | 2,094 → 2,094 | 59/59 |
| 59 | Q8_0 / fp32 | 3.02 → 2.56 | 19.52 → 23.07 | 2,094 → 2,094 | 59/59 |
| 59 | Google pack / fp16 | 3.69 → 3.68 | 15.97 → 16.04 | 1,754 → 1,754 | 59/59 |
| 59 | Q8_0 / fp16 | 4.03 → 4.02 | 14.64 → 14.69 | 1,754 → 1,754 | 59/59 |
| 59 | bge-small / CPU | 1.82 → 1.81 | 32.49 → 32.54 | — | 59/59 |

On the 219-passage slice, ordinary CUDA sync was 2.70× faster for Q6
and 2.71× for Q8_0. Traced forward time fell by factors of 3.55 and 3.59.
On 59 passages the corresponding wall speedups were 1.20× and 1.18×.
The fp16 wall medians changed by less than 1% on both fixtures. The
goinfer report measures inference under different shapes, while the wall
times here include verification, attachment, stores and publication.
[Goinfer inference measurements](https://github.com/pkieltyka/goinfer/blob/v0.6.7/docs/measurements/gguf-transposed-attention-2026-11-24.md)

With the v0.6.7 build, the Gemma CUDA cases ran at 31.93–36.72 passages/s
on the 219-passage fixture, 2.85–3.27× the rate of bge-small on four CPU
cores. On 59 passages, bge-small reached 32.54 passages/s and finished
ahead of the Gemma CUDA cases at 14.69–23.74 passages/s.

All 2,780 compared vector blobs matched bit for bit across the two
versions. Read-only SQLite `quick_check` and the ordered path, ordinal,
boundary and digest comparisons passed in every cell. The ordinary CUDA
indexes kept their fingerprints. Only the Google and Q8_0 CUDA fp16
fingerprints changed.

For the upgrade check, the candidate opened copies of the 59-passage
v0.6.6 indexes with their warmed vector caches. Both ordinary CUDA indexes
remained current and sync embedded zero passages. Both fp16 indexes reported
stale and sync embedded all 59 passages under their new fingerprints with
zero reuse. All four results matched their fresh v0.6.7 controls by vector
bytes.

## Q6 CUDA fp16 evaluation (#67)

The same evaluation binary from pbrain `38a16ea` ran both routes on the
RTX PRO 6000 Blackwell Max-Q (CC 12.0, driver 595.112.07). The build used
Go 1.27.3, `CGO_ENABLED=0`, `-trimpath` and the Linux amd64 release tags
`cuda,vulkan,eg2_combined,eg2_fp16`. The three goinfer modules were the
public v0.6.7 pins at `c26c69ed44717d86cf2218370054d5e89886b694`.
Both routes verified the same 248,812,032-byte Unsloth UD-Q6_K_XL GGUF
(SHA-256 `dc84f042bc3ffbe6122b3b38b2fb299d66f5f37b276c9d458db85d4a4f1e6ba3`).

A scratch Go overlay gave Q6 a revision-pinned fp16 profile and recorded
each successful attach. It checked the UUID returned by the attached
handle and read the initialized v0.6.7 pipeline flags. The ordinary
`cuda` route loaded the transposed-attention pipeline. The
`cuda-eg2-fp16` route loaded the fp16 FFN, packed attention and fusions.
Every CUDA run used the same named persistent UUID, with no fallback.
The evaluation patch targets a disposable copy of `38a16ea` and applies
with `git apply --unidiff-zero`. It contains the route receipt hook and
layout test. [Evaluation overlay](../../tools/phase28/evidence/q6-cuda-fp16-67/eval-overlay.patch)

### Matched indexing

The #66 harness setup used fresh SQLite indexes and vector caches,
CPUs 0–3, `GOMAXPROCS=4`, the default 6 GiB workspace and a named RTX
selector. Two matched pairs per fixture reversed the route order under
the timing lock. Each run started after two five-second samples at or
below 4% host CPU busy with no NVIDIA compute client. The timer covered
the full process, including model verification and loading. GPU peaks
are the highest device-wide 100 ms samples across both trials.

| Passages | CUDA mode | Wall s | Passages/s | Peak GPU MiB |
| ---: | --- | ---: | ---: | ---: |
| 219 | Ordinary | 6.88 | 31.84 | 2,130 |
| 219 | fp16 | 6.36 | 34.45 | 1,782 |
| 59 | Ordinary | 2.44 | 24.22 | 2,094 |
| 59 | fp16 | 3.96 | 14.91 | 1,754 |

Medians are from two samples per cell. On 219 passages the paired
speedups were 1.083× and 1.081×. On 59 passages the fp16 runs took 1.60×
and 1.64× as long as ordinary CUDA. Batch shapes matched between both
routes and both trials. The 219-passage case used 110,157 positions in
14 forwards with 13–17 inputs per call and longest inputs of 739–879
positions. The 59-passage case used 9,271 positions in two forwards with
55 and four inputs.

| Passages | CUDA mode | Verify ms | Load ms | Attach ms | Forward ms | SQLite ms | Publish ms |
| ---: | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 219 | Ordinary | 202.3 | 271.4 | 1,081.9 | 4,587.2 | 6.3 | 5.6 |
| 219 | fp16 | 200.1 | 259.2 | 2,785.2 | 2,501.8 | 6.4 | 5.8 |
| 59 | Ordinary | 182.9 | 260.7 | 1,084.6 | 298.0 | 2.7 | 3.6 |
| 59 | fp16 | 183.8 | 271.5 | 2,765.8 | 211.5 | 2.5 | 3.6 |

SQLite sums the lookup, write, commit and checkpoint spans. Forward time
on 219 passages fell 1.83×, while whole-sync wall time fell 7.6%. The
longer attachment accounted for most of the difference. On the shorter
fixture the attachment penalty outweighed the forward-time saving. fp16
reduced the sampled peak by 348 MiB on 219 passages and 340 MiB on 59.

### Vector and index checks

Every fresh index passed SQLite `quick_check`. Item paths, content
digests, CRCs and sizes matched, as did passage identities, ordinals,
boundaries and digests. The two trials agreed by vector bytes within
each route. The layout test compared singleton, packed, 17-input-split,
reversed and 1 MiB-budget outputs on both fixtures. All layouts agreed
by bits within each route, and the singleton vector hashes matched the
published SQLite indexes.

The CPU reference used the same verified Q6 file and goinfer pin. The
minimum cosine passed 0.999 on every passage at both 768 dimensions and
the renormalized leading 512.

| Passages | CUDA mode vs CPU | Minimum at 768 | Minimum at 512 |
| ---: | --- | ---: | ---: |
| 219 | Ordinary | 0.999999999981 | 0.999999999981 |
| 219 | fp16 | 0.999999122874 | 0.999999173004 |
| 59 | Ordinary | 0.999999999997 | 0.999999999997 |
| 59 | fp16 | 0.999999885790 | 0.999999888658 |

Ordinary and fp16 CUDA differed by bytes on all 278 distinct passages.
Their minimum and median cosines were 0.999999118237 and 0.999999926875
at 768 dimensions on 219 passages, and 0.999999886724 and 0.999999958813
on 59. At 512 dimensions the minima were 0.999999168657 and
0.999999889568, with medians of 0.999999929686 and 0.999999959839.
The largest absolute change in any stored coordinate was 0.00018084.

### Retrieval and recommendation

On the full #61 retrieval fixture (35 notes and 62 passages), actual
CPU queries used the existing 0.72 cutoff against each CUDA-built
index. The 47 answerable questions and 30 held-out unanswerable questions
produced identical ranked IDs for each of the 77 queries in both
semantic modes. The existing gain and abstention checks passed.

| Search mode | Top 1 / 47 | Top 3 / 47 | Expected on first page / 47 | Abstained / 30 |
| --- | ---: | ---: | ---: | ---: |
| Strict lexical | 15 | 16 | 16 | 30 |
| Semantic, both CUDA profiles | 39 | 42 | 42 | 30 |
| Combined, both CUDA profiles | 39 | 42 | 42 | 30 |

Recommend **explicit opt-in for Q6 fp16**, with ordinary CUDA as the
default. The larger fixture showed a repeatable gain and lower memory
use. On the short fixture a fresh sync took 62.5% longer. The retrieval
results support CPU queries against either profile at the existing
cutoff.

A future Q6 fp16 profile would use vector fingerprint
`43ce747722b0e928f7dc6218872509e69b777814e2cd4bdf4cfd13b63eedf434`
and chunker-v1 index fingerprint
`cbddba624bade0aca37c5aec7c6ab1b0efeeb962fca64ad76f8251a5b9250c77`
at this goinfer revision. Switching profiles requires sync to build
each requested section under the destination fingerprint. Cached vectors
stay within that profile, and both share the verified model file.
[Accepted shapes, route receipts and metrics](../../tools/phase28/evidence/q6-cuda-fp16-67/summary.json)
## Apple Silicon Metal Q8 indexing (#69)

The Apple M4 ran macOS 27.2.1 (26B131) with 24 GiB of unified memory. The
cgo-free Darwin arm64 build listed one Metal device as `metal:0`. Go
build information records candidate `c4faeca` and goinfer v0.6.8. Only the
qualification binary has an overlay that can force stock at the attach
call. The default arm calls the same bare `metal` selector as the release.
Both used the pinned Q8_0 GGUF, verified by pbrain in each fresh sync.

Every default Metal run selected `metal-eg2-combined-v1`. On the
join of all 219 passages, both combined repetitions and both stock
repetitions matched within their respective profile, bit for bit. Stock
and combined differed on all 219 vectors and had a minimum cosine of
0.999999999945 at 768 dimensions. Against the same-file CPU vectors, the
minimum cosines at 768 and normalized 512 dimensions were
0.999999999964 / 0.999999999965 for stock and
0.999999999993 / 0.999999999993 for combined. SQLite `quick_check`
and ordered item and passage metadata checks passed. The CPU-only binary
read both completed profiles. All eight frozen queries returned the same
ordered IDs on stock, combined and CPU indexes.

Switching a populated stock generation to the default selector rebuilt
all 219 passages. A repeated combined sync attached once with zero
forwards and kept the generation. Returning to stock reused all 219
passages from that profile's cache, with the same stored vector bytes.

### Matched device comparison

Each cell summarizes two fresh SQLite syncs from two order-reversed pairs.
The runs used the frozen 22-note/219-passage and 32-note/59-passage
fixtures and fresh vector caches. Timing includes the whole sync,
including strict verification and model load. Both hosts used
`GOMAXPROCS=4`. On Linux, CPUs 0–3 were pinned throughout the runs, which
held the timing lock and checked that the machine and RTX were idle. The
Mac ran sequentially after two samples of at least 95% CPU idle. Compare
each GPU with the CPU controls on its own host.

Times, rates and trace splits are medians. SQLite sums index read, lookup,
write, commit and checkpoint spans. Trace totals can overlap. BGE's
forward spans sum work across its CPU workers. The same-file minimum
cosine at the stored width uses the CPU index for that model on the same
host. Separate Linux CPU references cover the Google pack and Q6; BGE
stores 384 values per vector and Gemma stores 768.

RTX memory is the highest device-used sample on either run. Radeon memory
is the largest guarded share across the two runs, using process VRAM and
the increase in device-wide usage. The 120-second guard had a 1 GiB
workspace request and a 4 GiB total cap, including weights. No run
reported a device loss or reset. Metal uses unified memory. Discovery
estimated 16.8 GiB of headroom. Device allocation was unavailable through
the attached runtime, so the Metal cells omit a device peak. The maximum
process footprints at 219 passages were 1.98 GiB combined and 2.01 GiB
stock from macOS `time -l`.

The RTX rows reuse the v0.6.8 arms of the paired upgrade check. That check
used the same pbrain source apart from pins and the revision in the fp16
fingerprints. All 2,224 vector comparisons against v0.6.7 matched by bits.
Sampled peaks were identical at each size and precision. When opened by
v0.6.8, the existing Q6 and Q8 ordinary indexes embedded zero passages.
The existing Google and Q8 CUDA fp16 indexes each rebuilt 59 passages.

### Ryzen 9 7950X (Linux)

| Passages | Device and writer | Model | Wall (s) | Passages/s | Peak device (MiB) | Verify / decode / attach / forward / SQLite (ms) | Min CPU cosine |
| ---: | --- | --- | ---: | ---: | ---: | ---: | ---: |
| 219 | RTX PRO 6000, CUDA fp16 | Google pack | 6.06 | 36.13 | 1,782 | 210 / 84 / 2704 / 2388 / 7 | 0.999999817860 |
| 219 | RTX PRO 6000, CUDA fp16 | Q8_0 | 6.27 | 34.93 | 1,782 | 230 / 272 / 2878 / 2276 / 6 | 0.999999448048 |
| 219 | RTX PRO 6000, CUDA ordinary | Q8_0 | 6.83 | 32.06 | 2,130 | 227 / 283 / 1114 / 4469 / 6 | 0.999999999991 |
| 219 | RTX PRO 6000, CUDA ordinary | UD-Q6_K_XL | 6.98 | 31.38 | 2,130 | 207 / 280 / 1100 / 4611 / 7 | 0.999999999981 |
| 219 | RX 9060 XT, Vulkan | Q8_0 | 29.13 | 7.52 | 1,294 | 258 / 280 / 555 / 27383 / 7 | 0.999999999993 |
| 219 | RX 9060 XT, Vulkan | UD-Q6_K_XL | 29.38 | 7.45 | 1,289 | 207 / 285 / 557 / 27656 / 6 | 0.999999999996 |
| 219 | CPU | BGE small | 19.16 | 11.43 | — | 100 / 6 / — / 74236 / 7 | 1.000000000000 |
| 219 | CPU | Q8_0 | 165.61 | 1.32 | — | 240 / 270 / — / 164786 / 11 | 1.000000000000 |
| 59 | RTX PRO 6000, CUDA fp16 | Google pack | 3.78 | 15.60 | 1,754 | 208 / 88 / 2719 / 204 / 3 | 0.999999945920 |
| 59 | RTX PRO 6000, CUDA fp16 | Q8_0 | 4.06 | 14.52 | 1,754 | 221 / 263 / 2863 / 188 / 3 | 0.999999899598 |
| 59 | RTX PRO 6000, CUDA ordinary | Q8_0 | 2.57 | 22.92 | 2,094 | 218 / 278 / 1101 / 298 / 3 | 0.999999999998 |
| 59 | RTX PRO 6000, CUDA ordinary | UD-Q6_K_XL | 2.52 | 23.38 | 2,094 | 174 / 279 / 1099 / 291 / 3 | 0.999999999997 |
| 59 | RX 9060 XT, Vulkan | Q8_0 | 3.73 | 15.84 | 1,244 | 219 / 271 / 566 / 2157 / 4 | 0.999999999998 |
| 59 | RX 9060 XT, Vulkan | UD-Q6_K_XL | 3.70 | 15.96 | 1,244 | 183 / 285 / 554 / 2188 / 3 | 0.999999999996 |
| 59 | CPU | BGE small | 1.84 | 31.99 | — | 111 / 6 / — / 6194 / 3 | 1.000000000000 |
| 59 | CPU | Q8_0 | 14.74 | 4.00 | — | 225 / 280 / — / 14055 / 4 | 1.000000000000 |

### Apple M4 (macOS)

| Passages | Device and writer | Model | Wall (s) | Passages/s | Peak device (MiB) | Verify / decode / attach / forward / SQLite (ms) | Min CPU cosine |
| ---: | --- | --- | ---: | ---: | ---: | ---: | ---: |
| 219 | Metal default (combined) | Q8_0 | 23.76 | 9.22 | — | 125 / 129 / 62 / 23254 / 7 | 0.999999999993 |
| 219 | Metal forced stock | Q8_0 | 27.07 | 8.09 | — | 126 / 129 / 83 / 26404 / 7 | 0.999999999964 |
| 219 | CPU | BGE small | 22.75 | 9.62 | — | 53 / 5 / — / 88554 / 5 | 1.000000000000 |
| 219 | CPU | Q8_0 | 196.82 | 1.11 | — | 119 / 133 / — / 196286 / 8 | 1.000000000000 |
| 59 | Metal default (combined) | Q8_0 | 2.64 | 22.35 | — | 118 / 130 / 36 / 2266 / 3 | 0.999999999998 |
| 59 | Metal forced stock | Q8_0 | 2.96 | 19.94 | — | 119 / 130 / 34 / 2584 / 3 | 0.999999999997 |
| 59 | CPU | BGE small | 2.15 | 27.48 | — | 54 / 4 / — / 7556 / 2 | 1.000000000000 |
| 59 | CPU | Q8_0 | 16.79 | 3.51 | — | 119 / 131 / — / 16459 / 3 | 1.000000000000 |

Default Metal delivers 8.28× the M4 Q8 CPU rate on 219 passages and 6.36×
on 59. On the Linux host the 219-passage Q8 fp16 rate is 26.4× its own CPU
rate. Ordinary CUDA has the faster short-fixture Q8 sync because it spends
less time attaching. CPU BGE has the highest 59-passage rate on each host.

The Radeon slowdown per passage is mainly a difference in work per passage.
The 219-passage fixture averages 503 prepared positions per passage, compared
with 157 on the 59-passage fixture. Radeon used 14 forwards of 7,391–8,106
positions on the longer fixture and two forwards of 7,712 and 1,559 on the
shorter one. These are the same batch shapes as ordinary CUDA. The separate
4 GiB guard never intervened. At the Vulkan working-set estimate, the full
8,192-position batch limit fits in the 1 GiB workspace budget. Q8 forward
rate was about 4,300 positions/s on 59 passages and 4,020 on 219. Q6 moved
from about 4,240 to 3,980 positions/s. Forward throughput per position fell
about 6% on the longer fixture. The cause of that difference was not determined.

[Capture and verification](../../tools/phase28/evidence/metal-69/README.md)

## Rule 3 and full-size sync refresh (#32)

### Pax (Linux)

2026-11-25. Pax, Ryzen 9 7950X and RTX PRO 6000 Blackwell Max-Q,
driver 595.112.07. The release is v0.4.6 at `76aea82`, pinned to goinfer
v0.6.8. Both published Linux amd64 archives matched the release SHA-256
checksums. The suite held the host timing lock and checked for foreign
NVIDIA compute before and throughout each timed command. Google CUDA and
both Rule 3 arms used `GOMAXPROCS=4` on CPUs 8–11, matching #22's cores.
Those cores and their SMT siblings were at most 2.0% busy in the two
Rule 3 preflight samples. The entire host was 5.5–6.7% busy in those
samples.

#### Check, open and first query

Five pairs alternated one fresh process per arm. The pinned Google text
pack was primed in the filesystem cache. Each timed iteration strictly
verified its digests, loaded the model, embedded the same query as #22,
and closed the encoder. The CUDA arm includes device resolution and
attachment. The CPU benchmark is untagged. The CUDA benchmark uses the
published tag set `cuda,vulkan,eg2_combined,eg2_fp16` and a scratch-only
test override that requests and checks `cuda-eg2-fp16` on the RTX UUID.
Both benchmarks used the released source with Go 1.27.3. Normal
`pbrain search --semantic` embeds its query on the CPU.

| Arm | Five fresh samples (ms) | Median (ms) | Range (ms) | Below 600 ms |
| --- | --- | ---: | ---: | ---: |
| Untagged CPU | 354.7, 361.4, 360.8, 359.7, 371.3 | 360.8 | 354.7–371.3 | 5/5 |
| Google CUDA fp16 | 1,029.3, 824.7, 959.3, 893.5, 887.2 | 893.5 | 824.7–1,029.3 | 0/5 |

The earlier #22 CUDA median was 1,049.7 ms under higher load. The matched
fresh-process #32 rerun measured 353.2 ms for CPU (347.1–357.6) and
840.0 ms for CUDA (806.6–912.9). The earlier 286.2 ms CPU median
(282.9–353.3) came from five iterations in a single process. The v0.4.6
strict-verification CPU arm still clears 600 ms. The CUDA arm exceeds it
in every sample. These are comparisons with historical runs across
different commits, GPU kernels and start conditions.

#### Full 5,000-item sync

The retained #22 corpus contains 4,500 shared and 500 personal Markdown
notes, totaling 50,000,000 bytes. We checked all paths and content against
the old corpus before restoring its original one-note increment. Fresh
runs started with empty indexes and vector caches. The checksum-verified
release GPU binary (Go 1.27.0) ran `pbrain --personal sync --json` with
the RTX selected by persistent UUID and CUDA precision left at its
default. The two published manifests confirm the expected v0.6.8 fp16
fingerprint for each Gemma row. Both rows embedded all 48,511 passages
and passed SQLite `quick_check` in both sections.

| Sync | Wall time (s) | Passages/s | Peak host RSS (MiB) | Sampled device peak (MiB) |
| --- | ---: | ---: | ---: | ---: |
| #22 Google CUDA, goinfer v0.5.0 | 4,181.3 | 11.60 | 4,758.0 | 1,314 |
| v0.4.6 Google CUDA fp16 | 517.4 | 93.76 | 2,866.1 | 1,788 |
| v0.4.6 Q8_0 CUDA fp16 | 522.8 | 92.78 | 2,611.7 | 1,788 |
| v0.4.6 Google CUDA fp16, one new personal note | 2.00 | — | 1,927.4 | 802 |

The Google pack ran about 8.08× as fast as #22 on this fixture. Software
and host load also changed between those runs. The device columns use
different sampling methods. This refresh sampled total RTX use every
250 ms, against #22's compute-process memory sample roughly every three
seconds. The one-note run embedded exactly one new personal passage
and zero shared passages. It reused the installed model pack. Its earlier
#22 counterpart took 2.386 s.

The full Linux runs had tracing off. The 250 ms GPU samples had a median
utilization of 91% for Google (2,068 samples) and 90% for Q8_0
(2,090 samples).

The optional Linux BGE run used a whole-host idle gate for the 32-worker
baseline. All 36 preflight samples exceeded 4% busy (4.9–9.5%), so
the BGE sync did not start.

### Apple M4 (macOS)

Apple M4 arm64 with 10 logical CPUs, 10 GPU cores and 24 GiB of unified memory,
macOS 27.2.1 (26B131). The host had 339 GiB free. The last two read-only
preflight samples were 99.14% and 99.80% idle. Go 1.27.4 was already
installed. The run used one home scratch directory for the model,
corpus, Go build and module caches, temporary files, installs and
pbrain cache. The Mac's existing Go telemetry mode was `local`, with
counters in `~/Library/Application Support/go/telemetry`. That mode and
directory stayed as configured.

| Published v0.4.6 archive | SHA-256 |
| --- | --- |
| `pbrain_0.4.6_darwin_arm64.tar.gz` | `5affedee4a0806e0c6e77b4fa7b0ac29391d0081157e50e4cbad20d594864f74` |
| `pbrain_0.4.6_darwin_arm64_cpu.tar.gz` | `05f9330e28027ae2fadadb5baf7aedeb6557070194c28adcb232a5ef52e213b9` |

Both named archive checksum entries were present exactly once and passed
`shasum -a 256 -c` on the Mac. Both extracted binaries reported
`v0.4.6`. Untagged and `-tags metal` installs from `@v0.4.6` with
`CGO_ENABLED=0` also reported `v0.4.6` from their scratch `GOBIN`s.
The browser-style quarantine xattr was set on a separate scratch copy
of the Metal archive binary. That copy's `--version` did not complete
within the 20-second SSH test bound. A separate
`spctl --assess --type execute --verbose=4` check returned exit 3 and
`rejected`. The focused system logs showed no matching entry, so the
cause of the original stalled launch remains unknown.

#### CPU Rule 3

The existing `BenchmarkBgeFirstQuery`, `BenchmarkGemmaFirstQuery`
and `BenchmarkGemmaQ8FirstQuery` ran unmodified from the v0.4.6 source.
Each row used five fresh native processes, `GOMAXPROCS=4` and primed
model bytes. The measured iteration verified the pinned digest, loaded
the model, embedded the first query and closed the encoder.

| CPU model | Five fresh samples (ms) | Median (ms) | Range (ms) | Below 600 ms |
| --- | --- | ---: | ---: | ---: |
| bge-small | 99.4, 66.1, 69.8, 67.7, 66.4 | 67.7 | 66.1–99.4 | 5/5 |
| Google Gemma pack | 221.8, 200.4, 209.1, 202.8, 204.1 | 204.1 | 200.4–221.8 | 5/5 |
| Q8_0 GGUF | 306.0, 306.9, 306.3, 306.9, 315.3 | 306.9 | 306.0–315.3 | 5/5 |

The 600 ms target applies to the CPU query path in every release build.
Metal attach cost is measured in the sync trace.

#### Full 5,000-item sync

After scp, the runner checked the 5,000 note paths and file bytes on
the Mac against the original #22 fixture. The fixture digest was
`2cbd38cde77fdc6f1d67949e50efd40d261d9f0a79d6bda5a759a854218db490`.
The shared and personal sections started with empty indexes and caches.
The released Metal binary ran with four Go workers and `--device metal`.
Its preflight idle samples were 98.9% and 98.6%. The released CPU binary
ran BGE separately with ten Go workers after 99.90% and 98.35% idle samples.
Both sections passed SQLite `quick_check` for each model. The Q8_0 manifests
confirmed the default `metal-eg2-combined-v1` writer fingerprint.

| Sync | Wall time (s) | Passages/s | Peak host RSS (MiB) | Sampled device peak (MiB) |
| --- | ---: | ---: | ---: | ---: |
| Q8_0 default Metal, 48,511 passages | 5,288.0 | 9.17 | 3,305.5 | — |
| BGE small CPU, 48,511 passages | 3,064.8 | 15.83 | 1,594.3 | — |
| Q8_0 default Metal, one new personal note | 1.152 | — | 2,127.3 | — |

Metal uses unified memory. The runtime did not provide a separate
device peak. macOS `time -l` reported peak process footprints of
3,261.1 MiB for the Metal run, 1,545.9 MiB for BGE and 2,131.2 MiB for the
increment. Both full runs embedded 43,642 shared and 4,869 personal
passages. Adding the same personal note as #22 embedded one new passage
and left the shared generation intact. The full sync trace recorded
attach spans of 167.4 ms for shared and 39.8 ms for personal. The
increment attached in 39.8 ms and 38.1 ms respectively, including the
shared no-op profile check.
The Metal timings include the trace instrumentation.

| Full Metal Q8_0 span | Total (s) |
| --- | ---: |
| Model digest verification | 0.261 |
| Model decode | 0.272 |
| Device attachment | 0.207 |
| Tokenization and preparation | 8.985 |
| Forward | 5,245.937 |
| SQLite lookup, write, commit and checkpoint | 5.004 |
| Vector cache writes | 13.187 |
| Publish | 14.556 |

The 48,511 singleton forwards handled 24,475,852 prepared positions
(median 535 per passage, range 27–902) at 4,666 positions/s. Forward
spans account for 99.2% of sync wall time and include time inside the
backend on the host and device. Tokenization and preparation account
for 0.17%. macOS `time -l` recorded 163.94 s of user and 37.17 s of system
CPU time, averaging 3.8% of one logical CPU. The capture has no Metal
GPU utilization samples or kernel timings. The traced
[#69 matrix](#apple-silicon-metal-q8-indexing-69) used the same
219-passage Q8_0 fixture across devices. It measured 23.254 s of Metal
forward time in 219 singletons and 2.276 s of RTX forward time in
14 packed calls.

#### Warm semantic search

The released Metal archive ran `pbrain search --semantic --json` with
four Go workers and the same exact and paraphrase queries as #22. Each
row has one untimed priming query followed by five fresh processes.
The Q8_0 measurements read the full Metal-built index before adding
the incremental note. Each query used the CPU.

| Model | View and query | Median (ms) | Range (ms) | Below 1 s |
| --- | --- | ---: | ---: | ---: |
| Q8_0 | Shared, exact | 474.0 | 460.3–482.6 | 5/5 |
| Q8_0 | Shared, paraphrase | 458.5 | 455.3–479.5 | 5/5 |
| Q8_0 | Personal, exact | 484.4 | 481.2–501.2 | 5/5 |
| Q8_0 | Personal, paraphrase | 478.7 | 478.3–483.0 | 5/5 |
| BGE small | Shared, exact | 276.9 | 275.8–278.6 | 5/5 |
| BGE small | Shared, paraphrase | 273.9 | 273.7–275.3 | 5/5 |
| BGE small | Personal, exact | 296.5 | 296.1–298.5 | 5/5 |
| BGE small | Personal, paraphrase | 294.5 | 293.2–295.3 | 5/5 |

#### Native Go suite

Native `go test ./...` ran from v0.4.6 with Go 1.27.4, `CGO_ENABLED=0`,
four Go workers and package parallelism two. Untagged and `-tags metal`
each had 24 passing packages and two with no test files. Each hit the
same three pre-#1 lock failures:
`cmd/pbrain TestScenarios/14-concurrent-writers`,
`internal/cli TestConcurrentSettingsWriters` and
`internal/semantic TestModelProvisionShared`.

Local review receipts are under `/var/tmp/pbrain-rule3-046.dE76IE` on pax.
They include the release checksum checks, raw timings, compact Metal
forward records and the native Go logs.
