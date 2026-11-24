# BGE runtime source

This directory contains the BERT and WordPiece paths from
[`github.com/pkieltyka/aikit` v1.79.1](https://github.com/pkieltyka/aikit/tree/0bbc048828e3e546fcf280659d9d9f4ef694936d),
commit `0bbc048828e3e546fcf280659d9d9f4ef694936d`. The source module checksum
is `h1:J8fGlbr5HlGBukmT1U/B4JhdeGWhPw+DJnZEaYamTnk=`. The original
[LICENSE](LICENSE) and [NOTICE](NOTICE) are copied verbatim.

| Local file | Upstream source |
|---|---|
| `bert.go` | `encoder/bert.go`, `encoder/pooling.go`, `encoder/validate_dims.go` |
| `forward.go` | `encoder/bert.go`, `encoder/forward.go` |
| `attention.go` | `encoder/attention.go` |
| `layernorm.go` | `encoder/layernorm.go` |
| `parallel.go` | `encoder/parallel.go`, `encoder/linalg.go` |
| `scratch.go` | `encoder/scratch.go` |
| `safetensors.go` | `embed/safetensors.go` |
| `normalize.go` | `embed/pool.go` |
| `tokenizer.go`, `tokenizer_test.go` | `embed/tokenize.go`, `embed/tokenize_test.go` |
| `bert_test.go` | Adapted gates from `encoder/bert_fs_test.go` and `encoder/bert_test.go`, with generated F32 weights |
| `internal/linalg/*.go`, `internal/linalg/*.s` | Corresponding files under upstream `linalg/`, retaining the required FP32 entry points and architecture dispatch |
| `internal/linalg/contract_test.go` | Adapted GELU and scaled-softmax gates from `linalg/exp_contract_dispatch_test.go` |

The port accepts only the five in-memory model files used by the BGE adapter.
It retains F32 weights, learned positions, type embeddings, CLS pooling, and
WordPiece normalization and memoization. The I64 position-ID buffer is
validated in the safetensors header. The forward derives positions from the
token order. Vector reductions, tile widths and CPU dispatch are preserved.
The amd64 dot and activation kernels use AVX2/FMA when the CPU and OS support
them. The arm64 kernels use NEON. Scalar fallbacks remain available.

The model files are fetched separately by pbrain. No weights are shipped in
the source tree or release archives. The module itself is absent from
pbrain's dependency graph. The `aikit` runtime label records provenance of
the existing model row.
