# Vulkan qualification and throughput on goinfer v0.6.2

Issue #49 ran on pax with an AMD Ryzen 9 7950X, Radeon RX 9060 XT
(Mesa RADV 26.3.1), NVIDIA RTX PRO 6000 Blackwell Max-Q and Linux 7.3.4.
The sync and qualification binaries used pbrain
`d5129b3b880ed0f361f07f8876700ebd60ab0ddd`, Go 1.27.3 and the
`cuda,vulkan` tags. The scratch slice binary used the same product source
with per-forward timestamps. All three goinfer v0.6.2 module tags point
to `d9454f07441a`.

The saved 55/7,712-position and 4/1,559-position partitions passed at
minimum cosine 0.999999999998413 against CPU. The layout gate had zero
bit differences across all 59 vectors, including reversed and
singleton-only layouts. Both AMD gates stayed under the 4 GiB sampled
total memory cap, with no loss or ring reset. The CUDA/CPU gate
passed at minimum cosine 0.999999999998539.

## Full-corpus windows

The 5,000-item, 48,511-passage corpus matched #43's fixture. The cold
Vulkan sync ran with four allowed CPUs and a 1 GiB requested workspace.
The table measures successive cache-write completion intervals and
includes time between forward calls. The first interval includes
startup. Tokenizer replay matched all 1,565 cache digests to the
production passage order and reconstructed the builder's 12 windows.
The position counts include the document prompt and special tokens.

| Window | Inputs | Total positions | Median / p90 positions | Inputs above 512 | Seconds | Passages/s |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 256 | 128,362 | 524.5 / 753 | 138 | 268.630 | 0.953 |
| 2 | 4 | 2,021 | 515 / 630 | 2 | 4.196 | 0.953 |
| 3 | 256 | 128,438 | 549 / 800 | 141 | 268.794 | 0.952 |
| 4 | 11 | 6,045 | 570 / 686 | 7 | 12.800 | 0.859 |
| 5 | 256 | 126,014 | 528 / 769 | 135 | 262.622 | 0.975 |
| 6 | 9 | 3,573 | 473 / 587 | 2 | 7.348 | 1.225 |
| 7 | 256 | 132,636 | 552.5 / 802 | 152 | 275.879 | 0.928 |
| 8 | 1 | 264 | 264 / 264 | 0 | 0.578 | 1.730 |
| 9 | 256 | 126,784 | 527.5 / 788 | 136 | 264.724 | 0.967 |
| 10 | 2 | 837 | 418.5 / 524 | 1 | 1.809 | 1.106 |
| 11 | 256 | 129,563 | 543 / 795 | 144 | 270.475 | 0.946 |
| 12 | 2 | 696 | 348 / 448 | 0 | 1.443 | 1.386 |

The first window's longest input was 879 positions. The other full
windows had medians of 527.5–552.5 and similar or higher p90s. The run
received SIGINT after 1,800.27 s. The shared checkpoint had 133 items
and 1,312 passages and passed SQLite `quick_check`. Sampled AMD memory
peaked at 1.20 GiB and host RSS at 1,576.6 MiB. The journal shows an
amdgpu VM-memory-stats warning at process cleanup, with no device loss
or ring reset.

The operator waived full-sync wall time and corpus DB-byte comparison
because that run would take hours and the qualification gates already
establish same-device byte identity and CPU cosine agreement. The
30-minute run and matched slice are the throughput evidence.

## Matched 200-passage slice

Every device embedded the same first 200 passages, 100,066 positions,
in production order. The slice's median was 524.5 and its p90 was 771
positions. The verified model was open before timing. Each process
used the host's default GOMAXPROCS of 32. Preflight CPU occupancy was
under 5% and NVIDIA had no other compute process. GPU runs requested
the same 1 GiB batch workspace. The AMD sample peaked at 1.26 GiB and
the kernel recorded the same cleanup warning on exit, with no loss or
reset.

| Device | Seconds | Passages/s | Forward calls | Median ms/input per call | Range ms/input per call |
| --- | ---: | ---: | ---: | ---: | ---: |
| Vulkan · RX 9060 XT | 49.243 | 4.061 | 13 | 237.0 | 221.1–314.6 |
| CUDA · RTX PRO 6000 | 14.854 | 13.464 | 14 | 72.3 | 63.9–97.3 |
| CPU · Ryzen 9 7950X | 114.571 | 1.746 | 200 | 594.3 | 64.9–1,376.4 |

Vulkan ran 2.33 times as fast as CPU. Its forward calls took 49.200 of
49.243 s. Across those 13 calls, time per input rose with the mean
positions per input (458–622 positions, Pearson r = 0.984). These are
adapter-call timings that include all backend work in each call. A
kernel trace would be needed to separately time attention and dispatch.
The four-CPU full-corpus sample used a different worker setting and
included the sync pipeline.

`capture.tar.gz` contains the raw runs, GPU samples, kernel logs, model
and build provenance, cached shapes, per-forward traces and replay
scripts. `summary.json` gives the position distributions and statistics.
