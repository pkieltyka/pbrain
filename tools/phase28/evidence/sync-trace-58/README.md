# #58 CUDA sync traces on pax

Ryzen 9 7950X, 124.9 GiB RAM, Linux 7.3.4, RTX PRO 6000 Blackwell Max-Q,
NVIDIA driver 595.112.07, Go 1.27.3, CGO disabled, and goinfer v0.6.3.
The corpus binary used master `84e8133` plus the Gemma trace code committed
as `d7197cd`. The only later runtime change in that commit was a BGE-only
span name. The slice test uses the committed Gemma trace code.

Each five-minute run started with an empty index and vector cache, a
prestaged local model, four logical CPUs (0–3), and a 6 GiB batch budget on
CUDA. Both used the #43 fixture of 5,000 items, 50 MB and 48,511 passages.
The input brain is retained at
`/var/tmp/pbrain-phase28-bge-34.rb61v3/corpus/docs/brain`. The traces and
the timed copies are retained at `/var/tmp/pbrain-sync-trace.NSKyVs`.
The captured exit 124 is the bounded SIGINT and includes cleanup time.

## Span totals

Cells give milliseconds and call counts. Timings are inclusive. SQLite
checkpoint serialization is within publish, and decode and attach are
within model open. Verification is timed separately.

| Span | Google corpus | Q8 corpus | Google slice | Q8 slice |
|---|---:|---:|---:|---:|
| Sync wall | 301,061.64 (1) | 300,710.24 (1) | — | — |
| Slice wall, including model open | — | — | 15,518.68 (1) | 15,709.14 (1) |
| Slice embed | — | — | 14,699.87 (1) | 14,688.99 (1) |
| Brain open | 10.95 (1) | 10.19 (1) | — | — |
| Brain scan and read | 174.37 (2) | 174.45 (2) | — | — |
| Chunking | 31.49 (428) | 30.77 (428) | — | — |
| Model verify | 217.58 (1) | 238.56 (1) | 203.79 (1) | 220.68 (1) |
| Model open | 643.68 (1) | 808.77 (1) | 614.84 (1) | 799.32 (1) |
| Model decode | 80.76 (1) | 249.74 (1) | 65.77 (1) | 249.35 (1) |
| Accelerator attach | 156.96 (1) | 148.85 (1) | 139.92 (1) | 141.83 (1) |
| Token preparation | 678.16 (28) | 715.35 (28) | 41.25 (1) | 43.05 (1) |
| Packing | 4.83 (270) | 4.86 (269) | 0.26 (13) | 0.28 (13) |
| Forwards | 298,660.22 (272) | 298,104.36 (271) | 14,657.82 (13) | 14,645.10 (13) |
| Vector cache lookup | 20.94 (4,156) | 20.75 (4,156) | — | — |
| Vector cache write | 170.90 (3,899) | 157.93 (3,899) | — | — |
| Index open | 0.02 (1) | 0.02 (1) | — | — |
| SQLite lookup | 54.25 (4,156) | 53.74 (4,156) | — | — |
| SQLite writes | 57.50 (403) | 56.38 (403) | — | — |
| SQLite commit | 2.80 (403) | 2.78 (403) | — | — |
| SQLite checkpoint | 17.57 (7) | 16.81 (7) | — | — |
| Publish | 170.58 (7) | 169.68 (7) | — | — |

| Measurement | Google corpus | Q8 corpus | Google slice | Q8 slice |
|---|---:|---:|---:|---:|
| Model opens / accelerator attaches | 1 / 1 | 1 / 1 | 1 / 1 | 1 / 1 |
| Forward share of sync or slice-embed wall | 99.20% | 99.13% | 99.71% | 99.70% |
| Forwarded passages | 4,042 | 4,025 | 200 | 200 |
| Forwarded positions | 2,025,713 | 2,017,684 | 100,066 | 100,066 |
| Positions/s inside forwards | 6,783 | 6,768 | 6,827 | 6,833 |
| GPU mean utilization inside forwards | 98.83 | 98.49 | 96.86 | 96.58 |
| GPU median utilization inside forwards | 100 | 100 | 100 | 100 |
| In-forward GPU samples (100 ms) | 2,982 | 2,977 | 146 | 146 |
| Peak sampled device memory (MiB) | 2,136 | 2,136 | 2,130 | 2,130 |
| Peak host RSS (MiB) | 2,207.0 | 1,929.4 | — | — |

Both shared-section builders opened the model, decoded it and attached
CUDA once. The trace counts include inputs forwarded in the final incomplete
window. Both retained checkpoints passed SQLite `quick_check` and held
349 shared items / 3,393 passages.
The totals put over 99% of sync wall inside goinfer calls. The sampled GPU
metric does not separate CUDA kernels, transfers and host gaps.

## Fixed slice and direct replay

The slice runs used 32 logical CPUs, the same 200 passages / 100,066 prepared
positions and the same 13 forwards at a 6 GiB budget. The `slice_embed` span
includes preparation and packing. The direct replay calls only goinfer's
`EmbedIDsBatch` after loading weights and attaching CUDA.

| Model | Slice embed positions/s | Direct replay positions/s | Direct replay passages/s |
|---|---:|---:|---:|
| Google | 6,807.3 | 6,786.8 | 13.565 |
| Q8 | 6,812.3 | 6,796.5 | 13.584 |

The prepared-ID TSV is identical for both models. Its SHA-256 is
`97c426da8c155b04bf0b34e270dcfa5e8cf18bfc1343e926f63e638fea6bcd22`. The
saved `passages-200.json` has SHA-256
`05fc47ce8a266dc50c0c26bd86de2fa31bfa8732f5b7ea64f3550d94ddfdfa75`.

Extract `capture.tar.gz` to see the raw traces, GPU samples, preflights,
exact slice and TSV, goinfer-only timings, and the derived report data.
Use `python3 tools/sync-trace/summarize.py <capture-directory>/<arm>` for
each of `google`, `q8`, `slice-google` and `slice-q8`. The
[trace harness](../../../sync-trace/README.md) has the replay commands.
