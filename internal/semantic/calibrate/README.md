# Retrieval calibration

Run from the repository root. The tool copies the fixture to a temporary
brain and uses a fixed UTC clock and the release's default JSON budgets.
With no model selected it runs the model-free arms.

```sh
go run ./internal/semantic/calibrate -stage calibration internal/semantic/calibrate/testdata/retrieval
go run ./internal/semantic/calibrate -stage check internal/semantic/calibrate/testdata/retrieval
```

Both commands print a JSON report. Exit 1 means the partial gate failed.
The calibration stage uses only calibration negatives. Run the check on the
held-out negatives only after choosing and freezing the candidate rule.
Each stage compares the shared responses and diagnostics byte for byte
after adding personal items, then verifies each private control with
both search and get in the personal view.

The count and BM25 controls use identical candidates. The separate stemming
control uses English suffix rules on unquoted words. The heading control
adds a second count for occurrences in recognized heading titles.

## Model gates

Pass a model name and the directory of its pinned files. For bge-small
the directory uses the same flat names as `model_path` in pbrain.

```sh
go run ./internal/semantic/calibrate -model bge-small-en-v1.5 \
  -model-path /path/to/pinned/bge-small \
  internal/semantic/calibrate/testdata/retrieval
```

For EmbeddingGemma 2, pass `-model embeddinggemma-2` and the pinned
text-only pack as `-model-path`.

The tool builds the scratch index, uses uncensored production scores and
the complete exact-result list to choose the cutoff on calibration
negatives. The 5% allowance uses only questions that produced a score.
The tool then evaluates the held-out JSON pages at that cutoff. It
also reports reciprocal-rank fusion of positive BM25 ranks and semantic
ranks at the same cutoff, with equal weights and a rank constant of 60.
No model files are downloaded. The scratch indexes and cache are removed
when the run finishes.

Read `model.semantic_gate.pass` for standalone model eligibility and
`model.combined_abstention_gate.pass` for the two flags together.
`model.bm25_with_model` is true only when the standalone partial and
semantic gates and their combined gate all pass. Exit 1 means one or
more gates failed. An incomplete index or a semantic fallback stops
the evaluation as an error instead of counting a lexical-only page.

## Phase 18 gate

The frozen questions have SHA-256
`af4054bbb01300f9f3097b3b414f4a4a5f05d246fa62b2cd48ea906358b5c041`.
Each trial compiled the three constants in `internal/search/partial.go` into
the calibration tool and ran the real CLI's partial tier. The calibration
grid used minimum matches 2 and 3, document frequency divisors 4, 5, 6 and
8, and share divisors 1 and 2. A second pass tested minimums 4, 5 and 6
with frequency divisors 4, 6 and 8 and share divisor 2. None of the 25
trials used held-out negatives to choose its rule.

| Calibration candidates | Empty of 30 | Expected IDs on first page, of 47 | Outcome |
|---|---:|---:|---|
| Starting rule, 2 / 4 / 2 | 26 | 22 | Failed abstention, including regression `u029` |
| Minimum 2 or 3, share divisor 1 | 30 | 16 | No gain |
| Minimum 2 or 3, share divisor 2 | 26–27 | 22 | Failed abstention |
| Minimum 4, share divisor 2 | 26–28 | 22 | Failed abstention |
| **5 / 4 / 2** | **29** | **22** | Passed, highest gain among passing rules |
| 5 / 6 / 2 and 5 / 8 / 2 | 29 | 21 | Passed |
| Minimum 6, share divisor 2 | 30 | 20 | Passed |

Only the frozen 5 / 4 / 2 rule was checked on held-out negatives.

| Arm | Expected IDs on first page, of 47 | Empty of 30 held-out | Long-source item retrieved, of 6 | Evidence reached, of 6 |
|---|---:|---:|---:|---:|
| Strict | 16 | 30 | 0 | 0 |
| Guided retry | 18 | 25 | 1 | 1 |
| Any clause by count | 35 | 0 | 6 | 3 |
| Any clause by BM25 | 42 | 0 | 6 | 4 |
| BM25 with stemming | 41 | 0 | 6 | 6 |
| BM25 with heading weights | 42 | 0 | 6 | 4 |
| Partial tier | 22 | 30 | 4 | 4 |

The partial tier passed every held-out regression and preserved all 16
strict first-page answers. The shared replay matched byte for byte and
all eight personal controls opened in the personal view. This enables
`--bm25` for the release without a model. A release that lists bge-small
still needs the combined gate in phase 25.

The four tool-only arms now choose snippets by the product's distinct-clause
coverage rule. The comparison rows and reads changed, and the search pages
consumed fewer characters in both halves. First-page expected IDs, abstention
and long-source retrieval counts stayed the same. Evidence reached increased
from three to four for BM25 and heading weights, and from four to six for
stemming. The plain count arm still reached evidence for three.

The architecture §8.6 harness also timed the enabled flag on Linux/amd64
with four CPUs and three warm samples per case. At 5,000 items and 50 MB,
shared medians were 175–223 ms and personal-view medians were 153–213 ms.
The slowest warm sample was 267 ms. These harness queries measure the
scanning and term-frequency cost. The six-clause scenario tests the
admitted rows separately.
