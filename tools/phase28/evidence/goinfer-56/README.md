# goinfer v0.6.3 qualification

Measured on pax with the Google EmbeddingGemma 2 and Q8_0 files already
available locally. The pin-only control is `2e599bd`, the uncapped
experiment is `81fa599` and the capped candidate is `b09762d`.
`ee27dbd` carries the v0.3.1 Vulkan absence parser fix. That fix affects
failed discovery. The hardware captures use the successful discovery
path on the earlier measured candidate.

`capture.tar.gz` contains provenance, raw vectors and workspace
estimates, 100 ms NVIDIA and 25 ms Radeon device samples, real-layout
tests, CPU timings, manifests and SQLite files from the storage checks.
Its scripts keep the model and fixture paths used on pax.
`summary.json` is generated from the capture. To check it after extraction,
run these commands from the extracted directory.

```bash
python3 harness/summarize-56.py .
python3 harness/verify-capture.py .
```

The two CUDA slice series each replayed the same 200 passages with
three alternating default-budget pairs and a 1 GiB pin-only control.
The prior release's slice inputs are in
[`vulkan-49-v062/capture.tar.gz`](../vulkan-49-v062/), member
`./slice/passages.json`. The first uncapped invocation asked the pin-only
binary for a UUID selector it did not support and did no embedding.
The completed series used `cuda:0` for that control on the same
UUID-masked device. The operator waived the paired full CUDA syncs
after choosing to keep the 8,192 aggregate-position cap.

The real storage runs used the same initial content and cache state per
layout. The retry build injected the certified marker before a CUDA
forward and wrote the event to a scratch trace. The final app normally
routes library output away from its CLI streams.

The Radeon runs selected the RX 9060 XT by persistent ID with a 1 GiB
workspace and a 4 GiB total-process cap. The timed partial sync ended
early at the operator's direction. The retained checkpoint contains
2,357 passages. The CPU check sampled 500 of the first 2,098 passages
in path and ordinal order, without replacement, with Python's
`random.Random(56)`. The manifest-selected database, sample indices and
both devices' vector bytes are archived.

[Measurement summary](../../../../internal/cli/README.measure.md#goinfer-v063-device-and-batch-qualification-56)
