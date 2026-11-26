# Next release

- **Breaking: the default model is now `embeddinggemma-2-q8-0`, and `bge-small-en-v1.5` is removed.** pbrain targets brains of long documents, and EmbeddingGemma 2 suits them better than the small model it replaces. The new default is Unsloth's Q8_0 conversion of Google's EmbeddingGemma 2. It reads up to 2,048 tokens of a passage where bge-small read 512, and on our fixture it finds the expected item for 42 of 47 questions where bge-small found 40. It is the one model that can build its index on CPUs, NVIDIA (CUDA), discrete AMD (Vulkan) and Apple Silicon (Metal). `embeddinggemma-2` and `embeddinggemma-2-ud-q6-k-xl` remain models a project can name, and `model = "none"` still turns semantic search off. This release is v0.5.0. ([#77](https://github.com/pkieltyka/pbrain/issues/77))

- **Upgrading from v0.4.x.** What to do depends on `pbrain.toml`:
  - *No `model` key.* Run `pbrain sync` once, with `--personal` if you index a personal section. It downloads the pinned 310 MB model if it is not already in the cache, builds a new index, then removes the old one. Until then `search --semantic` returns its exact matches with a `semantic_not_ready` notice; it never searches the old index.
  - *`model = "bge-small-en-v1.5"`.* Delete that line to use the default, or name another model. Until then `pbrain sync` fails with `invalid_config` and says the model was removed, `search --semantic` returns its exact matches with a notice, and every other command works as before.
  - *A Gemma model is named.* Nothing changes.

- **What the new default costs.** It is a much heavier model than the one it replaces.
  - *Use a GPU build for the first index.* On the 5,000-item, 50 MB test brain (48,511 passages) the first index took 8 m 43 s on an NVIDIA RTX PRO 6000 with CUDA fp16 and 1 h 21 m on an Apple M4 with Metal. The standard Linux archives and `darwin_arm64` include GPU support. A plain `go install`, the `_cpu` archives and Darwin amd64 do not.
  - *On CPUs alone a large brain takes hours.* The model ran at 1.32 passages a second on four Ryzen 9 7950X cores and at 1.88 on 32 threads, so more cores help little. That projects to about 10 hours for the test brain; that run was not made. bge-small took 20 minutes there. Each CPU sync that embeds now prints a `cpu_fallback` notice.
  - *Search.* A search loads the model on the CPU in every build. On the test brain the warm medians of four queries were 459–484 ms on an Apple M4 and 684–729 ms on four Ryzen 9 7950X cores, with the model file sealed in the cache (fs-verity); all 40 Linux samples were under the 1 s target. Checking and loading the model and embedding one query took 553–593 ms as medians on four Ryzen cores, close to the 600 ms the design allows, and 307 ms on the M4.
  - *Memory.* A search of the full test brain held up to 1.39 GiB (1,419 MiB) on Linux. bge-small held up to 549 MiB.
  - *Catch-up.* A search still indexes a few new items itself, for up to about 2 seconds. How many it covers with this model is not measured; a 500-token passage takes about three quarters of a second on four cores.

- **`PBRAIN_DEVICE` and `--device` now apply to the default model.** They were ignored for bge-small, which had no GPU path.

- **The cache keeps the removed model's files.** pbrain never deletes from the per-user cache (`PBRAIN_CACHE`, else `~/.cache/pbrain` on Linux and `~/Library/Caches/pbrain` on macOS). To reclaim the space, delete `models/bge-small-en-v1.5-*` and `vectors/6bd9989750090942b9ff6d86f27ea8a39e255457d0c01659991f87280e181b9b` there.

- **Release archives carry goinfer's notices.** `licenses/goinfer/` holds goinfer's `LICENSE`, `NOTICE` and `THIRD_PARTY_LICENSES.md` beside `licenses/aikit/`, which stays because goinfer includes code copied from aikit.
