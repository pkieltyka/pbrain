# BM25 and two-model amendment review

Reviewed `079b3d192c3edff7bacba7bfc4ddb861e7a2b4974fc177247decbde42b8ebbbe` against the owner-approved [BM25 decision](../search-methods/joint-bm25.md). Section 7.1 and scenario 38 preserve the two independent flags, exact ordering and counts, the semantic-before-partial tie resolution, and paging over one list.

1. **Blocking — §§15–16, release gates.** The draft gates the partial tier alone and each model alone. The accepted decision gates each enabled mode. On 40 held-out questions, BM25 can answer two incorrectly and a model can answer two *different* questions incorrectly. Both clear 95% separately. With both flags, only 36 of 40 pages are empty. Require 95% empty pages and every designated abstention regression on the rendered combined command for each shipping model as well as on each standalone mode. Add the disjoint-failures fixture. Hold the new combination's release pending correction and fresh held-out evidence. Keep model-free retrieval gains attributable to BM25 alone.
2. **Should-fix — §§9.4, 10 and scenario 35, offline model installation.** A reader following the example can point `model_path` at a complete, authentic upstream EmbeddingGemma 2 checkpoint. The component deliberately rejects that layout. Specify the supported offline input for this model: copy the verified, derived model folder from a connected machine running the matching release. The source checkpoint alone yields `invalid_config` with no fetch and no changes to the folder. Cover that pair of cases in scenario 35.

**Verdict: changes required.**

## Re-check

Reviewed `6c458ba77a1a289cc55e97ca4340388a5d63514fc54b5217cd4a850dba3b0b02`. Both findings are resolved. Section 16 gates the rendered combined command for each listed model, requires every designated regression to pass, and specifies when the release omits BM25. Sections 9.4 and 10 and scenario 35 distinguish the portable EG2 pack from the published checkpoint and give the offline failure behavior.

**Verdict: approve.**
