# Q8_0 default on pax (#77)

The Linux amd64 binaries were extracted from the verified GoReleaser
snapshot of `b77d8ad0a83327b868ef4cf47acab96ea1153f6c`. The receipts
record the archive checksums, binary hashes, build metadata and run
arguments. The untagged CPU build and the CUDA/Vulkan release build
both searched on the CPU with four Go workers pinned to CPUs 8–11.

The source was a scratch copy of the frozen #22 corpus as retained by
the #32 Q8_0 run. The runner confirmed all note paths and file hashes
before and after. It verified the local GGUF against the release hash
and left the model key unset. The RTX rebuild into the v0.6.9 CUDA fp16
profile sealed the GGUF in its managed cache. The model's verity
attribute appears in `qualification.json`. Both builds reported their
4,500 shared and 500 personal items fully indexed before search.

The run used goinfer's host timing lock and the #32 idle and NVIDIA
compute checks. Each stage needed two consecutive five-second samples
with at most 10% host busy and 4% busy on each selected CPU and its SMT
sibling. The process receipts record monotonic elapsed time and
`wait4` peak RSS. Each query and view has a priming receipt
(`-0`) and five timed receipts. CPU and GPU build order alternated by
pair. SQLite quick-check, count and hash receipts are recorded before
and after. The capture contains those receipts and the search output.

The v0.4.7 archive checksum and one-note sync confirm the retired
model's two cache paths. The real Q8_0 runtime quiet test ran separately
under the timing lock after the measured search.

Archive SHA-256

`ea382c85a0fdaee62e13742cd88a96392d36c9b215a7469504dc34ed6d0d73c7`

```sh
capture=$(mktemp -d)
tar -xf capture.tar.xz -C "$capture"
python3 -B verify-capture.py "$capture/evidence"
```

[Measured results](../../../../internal/cli/README.measure.md#q8_0-default-on-the-5000-note-linux-fixture-77)
