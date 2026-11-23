# Next release

- Updated the goinfer root, CUDA and Vulkan modules to v0.6.6. CUDA fp16
  eligibility now comes from the selected device's inventory row before
  model load. A selected fp16 writer attaches by UUID and fails if the
  device or selector is unavailable. ([#64](https://github.com/pkieltyka/pbrain/issues/64))
- The new goinfer revision changes the CUDA fp16 fingerprints for both
  Gemma models. Run `pbrain sync` to rebuild those indexes, adding
  `--personal` if you use a personal section. Sync reuses the model files.
