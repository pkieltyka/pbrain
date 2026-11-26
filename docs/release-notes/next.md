# Next release

- **EmbeddingGemma 2 uses goinfer v0.6.9.** The new revision changes the index fingerprints for Google and Q8_0 CUDA fp16 and Q8_0 Metal. Run `pbrain sync` to rebuild those indexes. Base-profile indexes and cached model files keep their identities. ([#72](https://github.com/pkieltyka/pbrain/issues/72))

- **macOS: two commands that first use a section at the same moment no longer fail.** When a section's lock file or a model cache's lock file did not exist yet and two pbrain commands started together, one could fail at once with `io`. The lock file is now created exclusively and the open retried. A lock that cannot be taken for a reason other than waiting now says so instead of naming another command. ([#1](https://github.com/pkieltyka/pbrain/issues/1))
