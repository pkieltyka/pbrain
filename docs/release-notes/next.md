# Next release

## Dependencies

- Updated the goinfer root, CUDA and Vulkan modules to v0.6.4. [goinfer v0.6.4 release notes](https://github.com/pkieltyka/goinfer/releases/tag/v0.6.4)

## Faster model verification for search

On supported Linux filesystems, semantic search skips rehashing a managed model
that `pbrain sync` sealed with fs-verity. The median for checking, loading, and
running the first query fell from 552.5 to 437.2 ms for Q8_0, from 342.9 to
241.3 ms for the Google pack, and from 112.7 to 61.3 ms for bge-small.
`model_path` and unsupported filesystems still use the full SHA-256 check.

## Faster CUDA indexing

The Linux amd64 and arm64 GPU archives use the fast fp16 CUDA path by default
for Google EmbeddingGemma 2 and Q8_0 on NVIDIA GPUs with compute capability 8.0
or newer. On a bounded RTX PRO 6000 sync, the Google pack ran 4.77× faster
and Q8_0 ran 4.45× faster. Older eligible GPUs use fp32 CUDA and sync reports
the reason. Search stays on the CPU.

On upgrade, run `pbrain sync` to rebuild your Gemma index. Include `--personal`
if you index a personal section. The rebuild reuses the downloaded model
files. To retain the existing CUDA vectors, set
`semantic.cuda_precision = "fp32"` before upgrading.
