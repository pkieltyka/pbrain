# EmbeddingGemma 2 GGUF evaluation evidence

The evaluator starts from pbrain `50f1b160c2dbb62f930a1c045ce13868d748cfc5`
and applies the scratch patch in `harness.tar.gz`. It uses the released
goinfer v0.6.1 root and CUDA modules and pbrain's verified artifact loader.
Both GGUFs come from Unsloth revision
`031f0d4b35536f69ab3509d4893c923264fcf253`. File digests, executable digests,
build information, affinity, load, and NVIDIA inventory are in the captures.

The small corpus uses [the #22 fixture paths](../gemma-pax-22/fixture-paths.txt).
`slice-paths.txt` selects 128 items from the 50 MB phase-28 corpus. The capture
records both corpus digests. Retrieval uses the checked-in calibration
fixture, with questions SHA-256
`af4054bbb01300f9f3097b3b414f4a4a5f05d246fa62b2cd48ea906358b5c041`.
Fitting uses its calibration negatives and freezes the resulting cutoff
before the held-out queries run.

| Archive | Contents |
| --- | --- |
| `load-and-quality.tar.gz` | Per-process timing, stage output, NVIDIA samples, passage comparisons, and fit and held-out retrieval output. |
| `same-file-rule2.tar.gz` | Matching prepared IDs, CPU and CUDA vectors, independent llama.cpp vectors, reference-binary provenance, and all cosine comparisons. |
| `harness.tar.gz` | Scratch adapter patch, measurement scripts, Go test overlay, and reference probe source. |

Use `tar -tzf` to list an archive and `tar -xOf` to read an individual
capture. For a repeat, use a disposable checkout at the candidate commit
above and apply `gguf-evaluator.patch`. Point `compile-eval.sh`,
`evaluate.py`, `shortlist.sh`, and `rule2.sh` at that checkout, the local
pinned model files, and the phase-28 corpora. The recorded command lines
show the Go tags, CPU affinity and reference probe flags.
