# Owner question (2026-11-21, verbatim)

> thanks :) cool.. also.. what else can we learn from qmd ..? anything else we should consider to take it as we search through docs..? we will use goinfer, and sqlite-vec all pure-go stuff.. wondering if any other kinds of search we should do like BM25 or other?

Context the operator added:
- The approved design differs from "goinfer and sqlite-vec":
  - architecture 8.7 stores vectors in flat files, not SQLite. Scoring 50k x 768 in Go took 39 ms, against 57 to 60 ms in sqlite-vec, and needs no new dependency.
  - The interim runtime is bge-small on hugot. goinfer is the base for EmbeddingGemma 2 only if the fixture favours that model (embeddinggemma2-runtime-comparison.md).
- The operator has asked the owner whether "goinfer and sqlite-vec" is a decision or an assumption. Until the owner answers, treat the approved design as standing, and say whether your answer changes under the owner's wording.
- The owner prefers design questions answered by the group jointly.
