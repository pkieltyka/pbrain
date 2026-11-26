# Metal batching qualification (#72)

The Apple M4 ran the cgo-free binaries built with `-tags metal` from
`0fa255aa63fb6b0b4900d3c5a75934450dfc71d0` (goinfer v0.6.8) and
`b2f4e9229689c6137ee118c0f5868fb2e99313db` (goinfer v0.6.9). The
benchmark used the pinned Q8_0 file, with its SHA-256 rechecked after
transfer. Both layout tests ran on the v0.6.9 build.

`capture.tar.xz` contains the 59- and 219-passage layout receipts, the
seven initial 219-passage indexes, nine timing recheck indexes, their
traces and frozen query rankings, and read-only power and idle samples.
The native suite summary records the successful untagged and Metal runs
on `b2f4e92` for #9. The complete 5,000-note batch record includes the
sync totals, selected trace spans and slowdown guard windows. The Mac
runner checked both full SQLite generations before it wrote the receipt.
The full comparison uses the historical v0.6.8 singleton from #32. The
owner stopped the v0.6.9 singleton and skipped the full-size follow-up
checks to free the M4 for goinfer #92.

The CPU wall time and the first B wall time in the timing recheck come
from their saved macOS `time -l` results. The scratch recorder corrections
recovered completed runs from their original stdout and traces.

The archive SHA-256 is
`114df0b3032384b8d7479bfad2d6c5ce7ec90242812cc76bcd42c77fa442e67f`.

```sh
capture=$(mktemp -d)
tar -xf capture.tar.xz -C "$capture"
python3 -B verify-m4.py "$capture"
python3 -B summarize-recheck.py "$capture"
python3 -B summarize-full.py "$capture"
```

[Measured results](../../../../internal/cli/README.measure.md#opt-in-metal-batching-on-the-base-m4-72)
