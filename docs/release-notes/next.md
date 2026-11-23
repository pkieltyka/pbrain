# Next release

- Updated the goinfer root, CUDA and Vulkan modules to v0.6.5. The CUDA
  archives no longer read the eight decoder settings at startup.
  [goinfer #70](https://github.com/pkieltyka/goinfer/issues/70)
- Gemma indexes written with CUDA fp16 have a new fingerprint for the
  updated goinfer revision. Run `pbrain sync` to rebuild them and include
  `--personal` if you use a personal section. The rebuild reuses the
  downloaded model files.
