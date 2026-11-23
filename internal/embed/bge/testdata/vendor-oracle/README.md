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
