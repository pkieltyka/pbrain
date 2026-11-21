# Semantic amendment review

Reviewed `bd0ffc7c4bb0cb68763cfb5eaebd01dc51e0fa46562d8defc0486d717baf6ef1` against owner A1–A8, the accepted C amendments and the operator's subsequent decisions. Local inference, lexical-first ranking, the absolute abstention threshold, per-section storage, incremental explicit build and heading offsets agree with the intended design.

1. **Blocking — §§5.1 and 9.4, per-user cache.** `PBRAIN_CACHE` can name a folder in a tracked checkout. The spec promises a cache outside every project but gives no outcome for a misplaced override. It can otherwise put vectors from several projects into files eligible for commit in just one. Before creating or writing cache contents, check the resolved location, including its closest existing parent. Refuse cache creation inside a Git worktree or the selected brain/personal roots with `invalid_config`. Cover a missing child and a path through an alias. Lexical reads continue independently. The same cache rules apply to the default location.
2. **Should fix — §§7.1, 9.3–9.4 and scenario 35.** Cache deletion, model availability and section-index readiness are independent. With a valid local model and a built index, removing only cached passage vectors should leave hybrid answers intact. A personal-view query can have a ready shared index and a missing personal one. Define readiness per selected section, use each ready section, and return a single partial notice for unavailable eligible items when at least one index is ready. Use lexical-only plus `semantic_not_ready` when no usable section remains, or query inference cannot run. Specify status and search outcomes for an unreadable/corrupt derived index; rebuild must recover without a manual source edit. Add these fixtures with a personal-only count absent in the shared view.
3. **Should fix — §§6.1, 7.1 and scenario 33.** “Byte for byte” lexical output is incompatible with a combined footer and finite budget: adding a semantic row can add a continuation or displace an optional related line. Specify stable lexical identity, rank, snippet and row fields before fitting; compute the footer and extras on the merged page. Test a boundary budget in text and JSON. If the semantic tail alone would make a previously valid first lexical row impossible to print, specify which contract wins (my recommendation is to retain the ordinary minimum-budget rule for that requested mode). Lexical errors for bad arguments, unavailable personal storage or a genuinely insufficient budget keep their exit codes when inference is unavailable.
4. **Should fix — §9.4 and scenario 37.** Build totals mix items and passages. One changed document can reuse several passages and embed one new passage. Define `items` and `removed` in items, and `embedded` and `cached` in passages; name the units in text. Demonstrate that cost for one edited document and a no-op build.

I support keeping search read-only. An indexing lock, passage encoding and cache writes on a query introduce input-dependent latency even with a time cap. An incremental explicit build after a pull or batch of adds gives predictable lookup cost and can reuse shared passage vectors.

**Verdict: changes required.**

## Re-check at 63a56b4d

1. **Resolved.** §9.4 checks the resolved cache location before creation, including default locations and missing children. Allowing a Git-ignored, untracked cache outside either section accommodates home-directory repositories. Scenario 35 exercises the refusal.
2. **Resolved.** §§7.1 and 9.3–9.4 now distinguish per-section availability, cache-only deletion and rebuild. Scenario 35 verifies disclosure in shared and personal views.
3. **Partly resolved.** Per-row fields and order are stable, but §7.1 and scenario 33 still require the exact lexical minimum to succeed in hybrid. Take one lexical result, no related rows, and a budget exactly equal to the compact JSON for that completed page. Add one semantic-only result. The required combined continuation now adds `"more":1,"next":1` (plus commas) to that unchanged JSON. It cannot fit. Use §6.1's minimum **for the requested mode**, report `budget_too_small` and the hybrid minimum when necessary; at that minimum show the lexical row and its continuation. Remove the contrary promise in both locations and test the two exact boundaries.
4. **Resolved.** §9.4 and scenario 37 specify item and passage units, cached reuse, changed-passage costs, removed entries and no-op behavior. The skill directs agents to the explicit incremental build.

**Verdict: changes required.** One should-fix remains. No privacy blocker remains.

## Final check

At `bca4e48a6e41764d530a09f8a7964109c3bb3632290b2e85d71970dfd93aa48d`, §7.1 uses the exact minimum for the requested mode. Scenario 33 verifies both minima and includes the continuation in the hybrid response. All four findings are resolved.

Approval carries to `2b9cedaf8782db33cbb969f205914f3593908d48b039828631353181fc2238cd`: §9.4 uses the existing ancestor to select the worktree and tests the planned cache directory against that worktree's ignore and tracking rules.

Approval also carries to `106ea3cd52196f267f715574b5c3ee1a25c01043bc69f11eb127c3a29bc9a2f1`. Compared with 2b9cedaf, only the id-list continuation form in §6.2, the ten notice codes in §6.3 and the passage-count assertion in scenario 37 changed. They agree with the rendering and incremental build contracts.

**Verdict: approve.**
