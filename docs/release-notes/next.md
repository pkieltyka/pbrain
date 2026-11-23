# Next release

## Dependencies

- Updated the goinfer root, CUDA and Vulkan modules to v0.6.4. The dependency update alone keeps the existing CUDA selector. [goinfer v0.6.4 release notes](https://github.com/pkieltyka/goinfer/releases/tag/v0.6.4)

## Faster model verification for search

On supported Linux filesystems, semantic search skips rehashing a managed model
that `pbrain sync` sealed with fs-verity. The median for checking, loading, and
running the first query fell from 552.5 to 437.2 ms for Q8_0, from 342.9 to
241.3 ms for the Google pack, and from 112.7 to 61.3 ms for bge-small.
`model_path` and unsupported filesystems still use the full SHA-256 check.

## CUDA indexing (pending #61 qualification)

The target default for Google EmbeddingGemma 2 and Q8_0 is the fast fp16 CUDA path on NVIDIA GPUs with compute capability 8.0 or newer. On upgrade, run `pbrain sync` and include `--personal` if you use a personal section. The new index reuses the downloaded model files. To retain the existing CUDA vectors, set `semantic.cuda_precision = "fp32"` before upgrading. Older NVIDIA GPUs use fp32 CUDA and sync reports the reason. Search still runs on the CPU.

The release will record the qualified default and Linux arm64 archive coverage here. If either model misses a rollout gate, fp32 stays the default and the release notes will identify the gate.
