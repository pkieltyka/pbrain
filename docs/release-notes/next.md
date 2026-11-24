# Next release

- **Faster EmbeddingGemma 2 sync on ordinary CUDA.** The goinfer root, CUDA
  and Vulkan modules move to v0.6.7. On eligible CUDA batches whose longest
  prepared input reaches 128 tokens, the ordinary CUDA path uses transposed
  attention. On an RTX PRO 6000, fresh 219-passage syncs went from 18.52 to
  6.86 seconds for UD-Q6_K_XL (11.82 to 31.93 passages/s) and from 18.39 to
  6.79 seconds for Q8_0 with `semantic.cuda_precision = "fp32"` (11.91 to
  32.27 passages/s). On 59 passages, the times went from 2.99 to 2.49 seconds
  and 3.02 to 2.56 seconds, respectively. Sampled GPU peaks stayed at
  2,130 MiB on 219 passages and 2,094 MiB on 59. The Google and Q8_0 fp16
  controls stayed within 1% of their previous wall times. All compared
  vectors were bit-identical. [Benchmark matrix](../../internal/cli/README.measure.md#goinfer-v067-cuda-sync-66)

- **Rebuild CUDA fp16 indexes after upgrading from v0.4.4.** The Google
  pack and Q8_0 fp16 fingerprints include the pinned goinfer revision.
  Run `pbrain sync`, adding `--personal` for a personal section. The rebuild
  reuses the downloaded model files. bge-small and the Gemma base-profile
  indexes for CPU, ordinary CUDA and Vulkan stay valid.
