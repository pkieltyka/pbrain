# Q8_0 release qualification

These captures qualify the `embeddinggemma-2-q8-0` release row in pbrain
`694b726564c1ad139ece70bcdd60b433d3a18053` with goinfer v0.6.2. The
file digest, hardware, CPU affinity, binary hashes and Go build metadata
are in the capture. [Measurement summary](../../../../internal/cli/README.measure.md#q8_0-release-qualification-on-goinfer-v062-51)

`summary.json` is generated from the archived logs with
`python3 harness/summarize-51.py evidence`. `capture.tar.gz` contains the
raw timings, independent reference probe output, prepared tokens, vector
comparisons, retrieval transcripts, and the release CLI smoke outputs.
The additional `offline-network-namespace/` capture shows the local
`model_path` smoke with only a down loopback interface and no routes.

The commands used on pax were `bash run-51.sh build`, `bash run-51.sh parity`,
`bash run-51.sh retrieval`, and `bash smoke-51.sh offline auto download`.
The harness expects the measured checkout as `eval-source`, a `model-root`
file pointing to the model directory's parent (with the verified file in
`q8/`), the retained 32-item phase 28 fixture, and the pinned llama.cpp
probe under `~/goinfer-logs/eg2-gguf-q8-q6/llama-8d60873df-cpu/`.
The archived scripts retain the fixture, device and tool paths from pax.

The test overlay dumps reference inputs and vectors. The calibration overlay
holds the production cutoff at 0.72 and selects the requested device for
the sync. Its first attempt forwarded `--device` to `sync --status`, which
rejected it. The archived harness passes it only to the indexing command.
Both successful retrieval transcripts use that correction. First-query
timings retain all five measured samples.
