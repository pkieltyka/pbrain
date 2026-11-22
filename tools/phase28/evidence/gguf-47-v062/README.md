# GGUF re-measure on goinfer v0.6.2

On 2026-11-22, the [#47 scratch evaluator](../gguf-47/) ran with pbrain
`d5129b3b880ed0f361f07f8876700ebd60ab0ddd`'s goinfer v0.6.2 pins. The
evaluator inherited the #47 source overlay and both verified Unsloth
files. The CPU runs used four physical cores on a Ryzen 9 7950X and the
CUDA runs used an NVIDIA RTX PRO 6000. Host Linux was 7.3.4 and the toolchain
was Go 1.27.3. The source archive contains the Go test overlay and module
checksums alongside the capture.

`summary.json` holds the five fresh-process CPU samples per model. The
benchmark times verification, constructor, one query and close at
`-test.benchtime=1x`. File caches were primed and captured preflight CPU
occupancy was below 4%. The process wall time and peak RSS are logged
separately. No measured sample was dropped.

`rule2-summary.jsonl` holds the minimum and the count below 0.999 on each
device. `capture.tar.gz` holds the raw samples, vectors, prepared token
IDs, independent llama.cpp CPU probe output, device preflights, model
digests, and the scripts. The probe binary at llama.cpp revision
`8d60873dfb8619f2b5ed31adfbab07cccb40db5c` passed its pinned checksum
before use. Each CPU and CUDA output set contains 22 reference texts and
59 document passages. See the [measurement summary](../../../../internal/cli/README.measure.md#gguf-first-query-and-same-file-parity-on-goinfer-v062).
