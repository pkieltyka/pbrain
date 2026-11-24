# aikit v1.79.1 BGE bit oracle

Captured before the port using the BGE adapter at pbrain `e9245f0`. The
pbrain code matches the v0.4.3 release at `0f3396c`. The baseline test
binaries linked aikit v1.79.1 at
`0bbc048828e3e546fcf280659d9d9f4ef694936d`, module checksum
`h1:J8fGlbr5HlGBukmT1U/B4JhdeGWhPw+DJnZEaYamTnk=`.

`inputs.json` holds 113 ordered texts and roles. The first 59 are
`semantic.Passages` outputs from the 32 notes in the #22 fixture list,
ordered by path and passage ordinal. The next 22 come from
`../reference.json`. The remaining 32 test empty/whitespace, composed and
decomposed Unicode, CJK, punctuation, literal specials, unknown words,
WordPiece word boundaries and content cuts. The content-cut cases cover
509, 510 and 511 raw tokens for queries, plain documents and split
prefix/passage documents. `ids.json.gz` contains each input's encoded
segments before the cut and the final wrapped IDs. Both architectures
produced identical input and ID files.

The compressed `*.f32le.gz` files hold 113 consecutive vectors in that
input order, each consisting of 384 little-endian float32 bit patterns
(173,568 raw bytes total per architecture). Compare within the captured
architecture and dispatch path.

| Architecture | Runner and dispatch | Raw vector SHA-256 |
|---|---|---|
| Linux amd64 | AMD Ryzen 9 7950X, AVX2/FMA, `GOAMD64=v1` | `03a70aca614b4346f67fd153aeb2959ba07830fa62f94a19101122637fa21af4` |
| Linux arm64 | QEMU user-mode 11.1.2, `qemu-aarch64 -cpu max` with NEON, `GOARM64=v8.0` | `902c7539292ce67f06520e7b45e808c59f6736655a8769ab5bfe93f688678393` |

Both binaries used Go 1.27.3, `-trimpath`, no build tags and
`CGO_ENABLED=0`. Both ran with `GOMAXPROCS=4` on host physical cores 0–3.
The capture used `embed.Verify` on the real pinned five-file model row.
The arm64 record qualifies numerical parity under emulation.

| File | Verified SHA-256 |
|---|---|
| `model.safetensors` | `3c9f31665447c8911517620762200d2245a2518d6e7208acc78cd9db317e21ad` |
| `tokenizer.json` | `d241a60d5e8f04cc1b2b3e9ef7a4921b27bf526d9f6050ab90f9267a1f9e5c66` |
| `config.json` | `094f8e891b932f2000c92cfc663bac4c62069f5d8af5b5278c4306aef3084750` |
| `sentence_bert_config.json` | `84e39fda68ccbff05bfa723ae9c0e70e23e2ec373b76e0f8c6e71af72a693cbf` |
| `pooling.json` | `d1caf60c96f5fba2157c0c26b76d80818fad6cf0b8eb5e73ec372ff9818eba5c` |

Set `PBRAIN_TEST_BGE_MODEL` to the verified model folder and run
`go test ./internal/embed/bge -run '^TestBgeVendorOracle$' -count=1`
on the matching CPU path. The test checks every ID and vector bit on
singleton and four-worker calls. The test harness uses
`PBRAIN_CAPTURE_BGE_ORACLE` to select an output directory when run
against the pre-port code. That mode writes scratch captures, separate
from the frozen checked-in files.

## Compatibility and throughput

The #62 candidate matched both architecture oracles on all 113 inputs,
including singleton and four-worker vector bits. The native amd64 run
also passed the reference cosine, worker and verified-byte loader tests.
The emulated arm64 oracle comparison completed in 1,138 seconds. To
reproduce it, cross-compile the test and run it from the package
directory with the same model files and CPU assignment.

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GOARM64=v8.0 \
  go test -trimpath -c -o /tmp/pbrain-bge-arm64.test ./internal/embed/bge
cd internal/embed/bge
PBRAIN_TEST_BGE_MODEL=/path/to/bge-model GOMAXPROCS=4 \
  taskset -c 0-3 qemu-aarch64 -cpu max /tmp/pbrain-bge-arm64.test \
  -test.run '^TestBgeVendorOracle$' -test.v -test.timeout=45m
```

An existing aikit-built fixture retained both published generations and
database bytes on sync. The 14 search responses were identical. The
candidate embedded zero unchanged passages across the 32 shared and two
personal items. A fresh rebuild with the warmed cache reused all 59 shared
vectors and embedded the two personal passages.

On the Ryzen 9 7950X with cores 0–3 and `GOMAXPROCS=4`, the benchmark warmed
each encoder and timed embedding the same 59 documents. Loading and
verification ran outside the timer. Three alternating matched pairs each
met the 4% external CPU limit. The earlier series included a load spike
and was repeated. Rates are passages per second.

| Runtime | Three matched runs | Median | Range |
|---|---|---:|---:|
| aikit v1.79.1 | 35.75, 35.57, 35.14 | 35.57 | 35.14–35.75 |
| Internal BERT | 35.34, 35.35, 35.29 | 35.34 | 35.29–35.35 |

The median fell 0.65% within the overlapping ranges. Each of the six
release graphs audited 9 Go files in the local encoder/tokenizer and 16
in its linalg package with zero new exceptions.
