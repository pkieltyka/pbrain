# Q8 Metal qualification and device comparison (#69)

[Measurements and conditions](../../../../internal/cli/README.measure.md#apple-silicon-metal-q8-indexing-69) · [Approved plan](../../../../docs/plans/metal-adoption.md)

The capture contains the four public Go module receipts, both fixture
manifests, the eight frozen query strings, exact binary build information,
timing and trace records, and the SQLite indices used in the comparisons.
It includes the Mac idle checks and cleanup record, and the Radeon guard
receipts for both fixture sizes. It excludes the models, caches, executables
and fixture text. The recorded benchmark overlay only selects the stock
Metal kernel for its comparison arm. The product binary uses bare `metal`.

To recompute the accepted summaries with Python 3.10 or later and SQLite:

```sh
capture=$(mktemp -d)
tar -xJf capture.tar.xz -C "$capture"
python3 verify-m4.py "$capture/mac-base" > "$capture/m4.json"
python3 summarize-matrix.py "$capture" > "$capture/matrix.json"
cmp mac-qualification.json "$capture/m4.json"
cmp matrix.json "$capture/matrix.json"
python3 render-matrix.py "$capture/matrix.json"
```

The matrix verifier checks SQLite integrity, exact item and passage metadata,
and all vectors in each pair. It computes the same-file cosine minimum for
each profile and checks repeat byte equality. The Mac verifier also checks
both vector widths, the eight ordered query result lists and profile
migration. Linux CUDA runs use the v0.6.8 arms of the matched pin comparison.
The separate pin-comparison and migration receipts include the v0.6.7
results. Metal device allocation is unavailable. The capture records
the runtime's estimated headroom and macOS process memory measurements.
