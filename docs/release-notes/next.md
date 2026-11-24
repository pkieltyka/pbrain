# Next release

- bge-small now ships its BERT encoder and WordPiece tokenizer inside pbrain.
  Its vectors are bit-identical, so existing indexes and cached vectors
  remain usable. ([#62](https://github.com/pkieltyka/pbrain/issues/62))

- **EmbeddingGemma 2 GGUF on AMD.** Q8_0 and the new opt-in UD-Q6_K_XL row
  can sync on Vulkan in the standard Linux amd64 archive. Select Q6 with
  `semantic.model = "embeddinggemma-2-ud-q6-k-xl"` in `pbrain.toml`. Its
  first sync fetches one verified 249 MB GGUF converted by Unsloth from
  Google's EmbeddingGemma 2. Q6 supports CPU, ordinary CUDA and Vulkan.
  Use `pbrain sync --device vulkan` to select AMD explicitly. Search runs on
  the CPU, and bge-small remains the default. The RX 9060 XT qualification
  passed CPU/Vulkan cosine and same-device batch-layout byte checks for
  both rows.

  Upgrading from v0.4.3 preserves all existing fingerprints. Selecting Q6
  requires a sync for its separate index and cache. Vulkan can reuse
  existing Q8_0 vectors from a CPU or CUDA fp32 index. Moving a CUDA fp16
  Q8_0 index to Vulkan changes precision and rebuilds the requested
  sections on sync. ([#65](https://github.com/pkieltyka/pbrain/issues/65))
