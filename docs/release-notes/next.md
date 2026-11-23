# Next release

## Dependencies

- Updated the goinfer root, CUDA and Vulkan modules to v0.6.4. The dependency update alone keeps the existing CUDA selector. [goinfer v0.6.4 release notes](https://github.com/pkieltyka/goinfer/releases/tag/v0.6.4)

## CUDA indexing (pending #61 qualification)

The target default for Google EmbeddingGemma 2 and Q8_0 is the fast fp16 CUDA path on NVIDIA GPUs with compute capability 8.0 or newer. On upgrade, run `pbrain sync` and include `--personal` if you use a personal section. The new index reuses the downloaded model files. To retain the existing CUDA vectors, set `semantic.cuda_precision = "fp32"` before upgrading. Older NVIDIA GPUs use fp32 CUDA and sync reports the reason. Search still runs on the CPU.

The release will record the qualified default and Linux arm64 archive coverage here. If either model misses a rollout gate, fp32 stays the default and the release notes will identify the gate.
