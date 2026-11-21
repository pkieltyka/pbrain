TODO
====

## Ready

- [ ] pbrain phase 4: `graph` and `secretscan` (@lead-dev)
    * description: **4. `graph` and `secretscan`.** @lead-dev. After 1; beside 5. `graph.md`, `writes.md`.
    * criteria: run the test plan in components/graph.md and writes.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev
    * after: phase 1
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 5: `brain` (@lead-dev-2)
    * description: **5. `brain`.** @lead-dev-2. After 1, 2 and 3; beside 4 and 6. `brain.md`: discovery, configuration, views, the exposure guard.
    * criteria: run the test plan in components/brain.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev-2
    * after: phases 1, 2 and 3
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 6: `query` and `search`, lexical (@lead-dev)
    * description: **6. `query` and `search`, lexical.** @lead-dev. After 4; beside 5, 7 and 8. `search.md`: matching, order, window, counts, snippet with its line, occurrences, context lines, and the partial tier behind `Features.BM25`.
    * criteria: run the test plan in components/search.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev
    * after: phase 4
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 7: `store` (@lead-dev-2)
    * description: **7. `store`.** @lead-dev-2. After 5; beside 6 and 9. `store.md`: the scan, guarded input, the write primitives.
    * criteria: run the test plan in components/store.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev-2
    * after: phase 5
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 9: `cli` core (@lead-dev)
    * description: **9. `cli` core.** @lead-dev. After 5; beside 7 and 8. `cli.md`: dispatch, flags, streams, exit codes, `RunWith`, `init`.
    * criteria: run the test plan in components/cli.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev
    * after: phase 5
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 11: Reading commands I: `search` and `get` (@lead-dev-2)
    * description: **11. Reading commands I: `search` and `get`.** @lead-dev-2. After 6, 7, 8 and 9; beside 13. `render.md`, `cli.md`: `search`, and `get` with `--history` and `--outline`. First whole commands: their scenarios, and the measurements of architecture 8.6 on the generated 5,000-item brain.
    * criteria: run the test plan in components/render.md and cli.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev-2
    * after: phases 6, 7, 8 and 9
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 12: Reading commands II: `list`, `prime`, `tags`, `check` (@lead-dev-2)
    * description: **12. Reading commands II: `list`, `prime`, `tags`, `check`.** @lead-dev-2. After 11; beside 13 and 14. `render.md`, `cli.md`.
    * criteria: run the test plan in components/render.md and cli.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev-2
    * after: phase 11
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 13: Writing commands (@lead-dev)
    * description: **13. Writing commands.** @lead-dev. After 3, 7 and 9; beside 11 and 12. `writes.md`: `add`, `verify`, `delete`.
    * criteria: run the test plan in components/writes.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev
    * after: phases 3, 7 and 9
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 14: The `audience` command (@lead-dev)
    * description: **14. The `audience` command.** @lead-dev. After 13; beside 15. `audience.md`: listing and editing the rules.
    * criteria: run the test plan in components/audience.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev
    * after: phase 13
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 15: `importer`: files and linked items (@lead-dev-2)
    * description: **15. `importer`: files and linked items.** @lead-dev-2. After 13; beside 14. `importer.md`.
    * criteria: run the test plan in components/importer.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev-2
    * after: phase 13
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 18: The calibration tool without a model, and the `--bm25` gates (@lead-dev)
    * description: **18. The calibration tool without a model, and the `--bm25` gates.** @lead-dev. After 11 and 17; beside 19 to 22. The model-free arms and the "reaching the evidence" reader (`semantic.md`); the partial tier's own gates (architecture 8.8). A pass lets `--bm25` ship in a release that lists no model; `Release.BM25` is set in the change that records it.
    * criteria: run the test plan in components/semantic.md “Calibration” and architecture.md §8.8 and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range; keep Release.BM25 off until the gate result is recorded
    * owner lane: @lead-dev
    * after: phases 11 and 17
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 19: Semantic (a): the merge (@lead-dev)
    * description: **19. Semantic (a): the merge.** @lead-dev. After 11; beside 20 and 21. `search.md` section 6 and its rendering, against a fake source.
    * criteria: run the test plan in components/search.md §6 and render.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev
    * after: phase 11
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 20: Semantic (b1): passages and the index database (@lead-dev-2)
    * description: **20. Semantic (b1): passages and the index database.** @lead-dev-2. After 7; beside 19 and 21. `semantic.md`: fingerprints, passages, the database and the scoring query.
    * criteria: run the test plan in components/semantic.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev-2
    * after: phase 7
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 21: `embed`: the model table and provisioning (@lead-dev)
    * description: **21. `embed`: the model table and provisioning.** @lead-dev. After 2; beside 19 and 20. `semantic.md`: the registry, find, verify, download through the `Downloader` interface, and pack, with the test model and a fake downloader.
    * criteria: run the test plan in components/semantic.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev
    * after: phase 2
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 22: Semantic (b2): the build (@lead-dev-2)
    * description: **22. Semantic (b2): the build.** @lead-dev-2. After 20 and 21; beside 24. The incremental build, publication, the vector cache, model recovery and their failure tests.
    * criteria: run the test plan in components/semantic.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev-2
    * after: phases 20 and 21
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 23: Semantic (b3): the source and the commands (@lead-dev-2)
    * description: **23. Semantic (b3): the source and the commands.** @lead-dev-2. After 19, 22 and the fetch client of 16; beside 24. Readiness, scoring, the status (it asks `embed` whether the model is there), `pbrain index` with `--status`, `search --semantic`, their notices; scenarios 33 to 37 with the test model. Here `cli` hands the real `fetch.Client` to `index` as its downloader.
    * criteria: run the test plan in components/semantic.md and cli.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev-2
    * after: phases 19, 22 and fetch client from phase 16
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 24: The bge-small adapter (@lead-dev)
    * description: **24. The bge-small adapter.** @lead-dev. After 21; beside 22 and 23. `embed/bge` on aikit v1.79.0, with its parity and runtime tests.
    * criteria: run the test plan in components/semantic.md “The bge-small adapter” and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev
    * after: phase 21
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 25: bge-small's gates (@lead-dev and @lead-dev-2)
    * description: **25. bge-small's gates.** @lead-dev with @lead-dev-2. After 18, 23 and 24. The model arms of the tool; the cut-off from the calibration half; the abstention and gain gates; the combined gate with `--bm25`; the evidence figures; time and memory of `search --semantic` and `index`. The model enters the table only when its own gates pass. If only the combined gate fails, with every tested admission rule, the model ships and `--bm25` is held back from that release (spec 16).
    * criteria: run the test plan in components/semantic.md “Calibration” and architecture.md §§8.7, 8.8, 14 and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range; run the combined gate for every listed model; on a combined-only failure hold back --bm25 while a qualifying model may ship
    * owner lane: @lead-dev and @lead-dev-2
    * after: phases 18, 23 and 24
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 27: The EmbeddingGemma 2 adapter and its gates (@lead-dev)
    * description: **27. The EmbeddingGemma 2 adapter and its gates.** @lead-dev. After 25 and 26. `embed/gemma2`: the pack and its derive step, the adapter, parity, rule 3; then its cut-off, its abstention and gain gates, and the combined gate with `--bm25` again for this model, with the same outcome on a combined-only failure.
    * criteria: run the test plan in components/semantic.md and architecture.md §§8.7, 14 and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range; run the combined gate for every listed model; on a combined-only failure hold back --bm25 while a qualifying model may ship
    * owner lane: @lead-dev
    * after: phases 25 and 26
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

- [ ] pbrain phase 28: Measurement before a release (@lead-dev-2)
    * description: **28. Measurement before a release.** @lead-dev-2. After the phases a release ships. The built binary on an arm64 Mac and on Linux: the speed targets of spec 16, and for each model rule 3, a first index, an index after one note, and peak memory (architecture 8.6, 8.7, 14). The four targets built with `CGO_ENABLED=0`; `go install` of a tagged version; results published. Repeated for each release that adds a search mode or a model.
    * criteria: run the test plan in architecture.md §§8.6, 8.7, 14 and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range; measure the built binary on an arm64 Mac before each release
    * owner lane: @lead-dev-2
    * after: the phases included in each release
    * next: @operator starts this phase after its listed prerequisites merge and work is ready; owner runs checks and requests @reviewer approval before merge.

## In progress

- [ ] pbrain phase 1: outline and item (@lead-dev)
    * description: **1. outline and item.** @lead-dev. After 0; beside 2 and 3. item.md; architecture 4.1, 7.4.
    * criteria: run the test plan in components/item.md; architecture.md §§4.1, 7.4 and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev
    * after: phase 0
    * work: branch peter/lead-dev, range 46ef8005f642b0295877b29e38d9550c7e409235..21db1167158d3067f1b251f757793994433d994f; commits 1c88613 (outline) and 21db116 (item). Includes master 46ef800; branch reported clean. Covers architecture §§4.1/7.4, components/item.md tests and scenarios 16, 21 and 39 at package level. Flow-scalar writer quotes ? in list entries after a pinned-scanner regression.
    * verdict: implementation complete; review requested, reviewer verdict pending
    * next: @reviewer reviews phase 1 after phase 8; then @operator performs integration review.

- [ ] pbrain phase 2: sys and gitx (@lead-dev-2)
    * description: **2. sys and gitx.** @lead-dev-2. After 0; beside 1 and 3. store.md, brain.md; architecture 6.3, 10.3.
    * criteria: run the test plan in components/store.md and brain.md; architecture.md §§6.3, 10.3 and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev-2
    * after: phase 0
    * work: branch peter/lead-dev-2, approved range dae47b2..713936a (13073fc plus lock fix 713936a), with master 46ef800 merged. Reviewer verified owner-only mode on pre-existing lock files; TestLockTightensPermissiveFile covers 0666, FIFO and new lock. Targeted sys tests passed plain/race/CGO-disabled, vet passed native and Darwin/arm64, and diff check passed. macOS runtime not exercised.
    * verdict: approved by @reviewer; ready for integration
    * next: @operator integrates phase 2 together with phase 8 once phase 8 is approved.

- [ ] pbrain phase 8: render (@lead-dev-2)
    * description: **8. render.** @lead-dev-2. After 0; beside anything. render.md.
    * criteria: run the test plan in components/render.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @lead-dev-2
    * after: phase 0
    * work: branch peter/lead-dev-2, range 86bab2d..7ce83fd, one commit; render.md Interfaces/Fitting and architecture §9. Resubmitted for review after the restart.
    * verdict: review requested; reviewer verdict pending
    * next: @reviewer reviews 86bab2d..7ce83fd next; after approval @operator integrates it with phase 2.

- [ ] pbrain phase 10: `skill` (@architect-2)
    * description: **10. `skill`.** Third lane, else @lead-dev. The texts and their tests after 0; the command after 9. `skill.md`.
    * criteria: run the test plan in components/skill.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @architect-2
    * after: phase 0 for texts/tests; phase 9 for command
    * work: skill texts/package commit `46ef800` merged to master and pushed; @reviewer approved range `6ec6bac..46ef800`, including byte-for-byte resource text, 400-word core and package behavior. CLI command integration remains after phase 9.
    * verdict: part 1 approved and merged; phase remains open for the command part after phase 9
    * next: after phase 9 merges, @architect-2 wires the skill command, tests text/JSON dispatch, no-brain release binary and parser integration, then requests review.

- [ ] pbrain phase 16: importer/fetch and URL imports (@architect-2 (fetch client); @lead-dev-2 (URL imports))
    * description: **16. importer/fetch and URL imports.** The fetch client, with its Download: third lane, else @lead-dev-2, after 0. import <url>: @lead-dev-2, after 15. Released only when scenario 20 passes.
    * criteria: run the test plan in components/importer.md and architecture.md §8.7 (Downloader) and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range; confirm the URL-import release gate, scenario 20
    * owner lane: @architect-2 (fetch client); @lead-dev-2 (URL imports)
    * after: phase 0 for fetch client; phase 15 for URL imports
    * work: fetch-client patch SHA-256 a2cebcaf7735905984d41075aa4ccbae1bdbf99a1ed23d808b045771450420d9 applied on peter/architect-2 at fcd6d01; review range 46ef800..fcd6d01, one commit. URL imports remain assigned to @lead-dev-2 after phase 15. Fetch tests cover redirects, per-hop DNS/TLS/Host checks, proxy independence, decoded/raw limits, content types, deadlines, cancellation and partial sink failure; no public service contacted.
    * verdict: fetch-client part in review; URL imports not started
    * next: @reviewer reviews the fetch-client range after phases 2, 8 and 1; @lead-dev-2 starts URL imports after phase 15; phase 23 requires the fetch client merged.

## Backlog

- [ ] aikit fork documentation follow-up (@lead-dev-2)
    * description: update active README, architecture and release instructions to the owner fork's current aikit v1.79.0 and GPU v0.36.x pin series; preserve dated history
    * criteria: address @reviewer post-merge P2 findings; retain LICENSE/NOTICE attribution and historical records; run fork checks and request @reviewer review before @operator merges on green CI
    * work: local branch `pkieltyka/docs-pins`, commit `0f8fd988`; not pushed. @reviewer found stale active references to aikit v1.75.0 / gpu v0.35.x in README.md, docs/architecture.md and RELEASING.md.
    * next: rebase or verify against current main, open the fork PR, request @reviewer review, and wait for @operator to merge on green CI

- [ ] goinfer fork documentation follow-up (@lead-dev-2)
    * description: point active user and release instructions to github.com/pkieltyka/goinfer, remove unsupported directions, and clear the stale workspace checksum
    * criteria: address @reviewer post-merge P2 findings in README.md, RELEASING.md, demo/chat/README.md and docs/ARCHITECTURE.md; preserve valid attribution/history/changelog references; run fork checks and request @reviewer review before @operator merges on green CI
    * work: local branch `pkieltyka/fork-docs`, commit `4fccdb46`, based on the pre-merge head and requiring rebase; `go.work.sum` is gitignored/untracked, so regenerate or remove the stale townsendmerino v0.22.0 entry if present rather than commit it.
    * next: rebase on owner-fork main, open the fork PR, request @reviewer review, and wait for @operator to merge on green CI

- [ ] pbrain implementation: deterministic ordering for same-line check findings
    * description: @lead-dev notes that check findings on the same line need deterministic tie-breaking
    * next: define and apply a stable tie-break rule during implementation

- [ ] pbrain architecture: audience-rule host matching
    * description: architecture note from @lead-dev for mixed-audience hosts
    * criteria: classify hosts by canonical DNS hostname; preserve non-default ports in exact rule keys
    * next: carry this rule into architecture design

- [ ] pbrain architecture: anchor printed paths
    * description: architecture note from @operator; printed paths need an anchor
    * next: define and apply the path anchor in architecture and output design

- [ ] pbrain architecture: avoid duplicate brain on bare init
    * description: a project whose only brain is outside `docs/brain` can acquire a second brain from a bare `init`
    * next: define how initialization detects an existing external brain before choosing a default target

## Done

- [x] pbrain phase 26: goinfer for EmbeddingGemma 2 (@architect (scratch patch; operator applies/PRs))
    * description: **26. goinfer for EmbeddingGemma 2.** @architect, in github.com/pkieltyka/goinfer. Independent of every pbrain phase; beside anything. The loader from bytes, the encoder constructor from a built model, tokenizer and prompts, and what the text-only pack needs (architecture 8.7; semantic.md, The EmbeddingGemma 2 adapter).
    * criteria: run the test plan in components/semantic.md The EmbeddingGemma 2 adapter and architecture.md §8.7 and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain reviewer approval of the applied patch and PR range; test against the pinned reference; @architect delivers a patch only
    * owner lane: @architect (scratch patch; operator applies/PRs)
    * after: none; independent of pbrain phases
    * work: patch SHA-256 701dea56 applied to goinfer topic branch pkieltyka/eg2-from-bytes; PR #4 merged by @operator as a759890b and tagged v0.2.0 on the owner fork. Reviewer approved code head 0e1687075ac54d6cbcd4438e1e4715986b9af5bd after the layout audit edge fix; package/layout tests, diff check passed. Owner directed merge without waiting for fork CI.
    * verdict: complete; implementation merged and v0.2.0 tagged
    * next: none; phase 27 pins goinfer v0.2.0

- [x] pbrain phase 3: `audience`: the canonical form, the rules and the decision (@architect-2) `6ec6bac`
    * description: **3. `audience`: the canonical form, the rules and the decision.** Third lane, else @lead-dev-2. After 0; beside 1 and 2. `audience.md`. The package is pure; `add` and linked imports need its decision.
    * criteria: run the test plan in components/audience.md and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range
    * owner lane: @architect-2
    * after: phase 0
    * work: @architect-2 patch SHA-256 `6b392d0a` applied by @operator; commit `6ec6bacc92060abccc2e546756478af3f5f5db16` on `peter/architect-2`, range `921a1a4..6ec6bac`. @reviewer approved: gofmt, vet, tests, diff check and fresh 320,923-execution fuzz run passed; x/net v0.63.0 pinned. @operator fast-forwarded and pushed master `6ec6bac`.
    * verdict: approved by @reviewer; merged to master and pushed by @operator
    * next: none; pure audience rules are integrated; audience command remains phase 14

- [x] pbrain spec pins for goinfer v0.2.0 (@architect)
    * description: update the factual library pin state after phase 26 merged and tagged goinfer v0.2.0
    * criteria: update architecture.md §8.7 pins and §15 module row, §17 risk; update semantic.md Our libraries, release row and The pins; record aikit v1.79.0, goinfer v0.2.0 at a759890b and the measured 41 ms weight load, while keeping whole-adapter rule 3 pending
    * owner: @architect, assigned by @operator
    * work: factual update committed on master as 8a43469; architecture §8.7/15/17 and semantic.md Our libraries/release/pins now name aikit v1.79.0 and goinfer v0.2.0 at a759890b. The measured 41 ms weight load is recorded; whole-adapter rule 3 remains pending.
    * verdict: complete; operator committed the factual pin update
    * next: none; phase 27 builds the remaining pack and adapter, then must demonstrate the whole-adapter gate

- [x] pbrain phase 17: The evaluation fixture (@architect-2) `921a1a4`
    * description: **17. The evaluation fixture.** @architect-2, under way; beside everything. The fixture brain and `questions.json`: answerable questions, reworded ones, unanswerable ones in their two halves with regression marks, the question styles, and an evidence quotation for each long-source question. Frozen in one commit, with ids, before any constant or cut-off is tuned (`semantic.md`, "Calibration"; architecture 14). The owner may add questions before the freeze. The harness needs 11.
    * criteria: run the test plan in components/semantic.md “Calibration” and architecture.md §14 and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range; validate answer IDs, supersession, split membership and unique evidence quotations, then freeze the fixture with IDs before tuning
    * owner lane: @architect-2
    * after: none; independent
    * work: fixture commits `20d02c9` and `cea9a25` reviewed on `peter/architect-2`, with master merge `921a1a4` at HEAD; 35 shared items, 8 isolated personal controls, 107 questions (47 answerable; 30 unanswerables per half), six unique quotations <=120 code points. No model or tuning. @reviewer found no issues; `diff --check 76d16b3..HEAD` passed. @operator fast-forwarded to master and pushed `921a1a4`.
    * verdict: approved by @reviewer; merged to master and pushed by @operator
    * next: none; fixture is frozen before tuning

- [x] pbrain phase 0: Skeleton (@lead-dev-2) `86bab2d`
    * description: **0. Skeleton.** @lead-dev-2. Everything waits for it. The module `github.com/pkieltyka/pbrain` on Go 1.27, the MIT licence, CI (format, vet, tests, the race job, the `CGO_ENABLED=0` builds, the check that `go.mod` has no replace or exclude directive), `errs`, and every exported type and signature of architecture 18 step 0 with empty bodies, compiling together.
    * criteria: run the test plan in architecture.md §§3–4, 7.3–7.4, 8.5, 9, 10.1, 10.3, 13 and implementation_plan.md phase 0 and the applicable scenarios in functional_spec.md §15 named by this phase; run gofmt, go vet and go test successfully; obtain @reviewer approval of the exact commit range; verify the package tree/signatures compile, the replace/exclude guard and CGO_ENABLED=0 cross-builds, and include the MIT licence
    * owner lane: @lead-dev-2
    * after: none; all other phases wait for it
    * work: @reviewer approved branch range `76d16b3..86bab2d` (skeleton commit `38a5ac2`, parser fix `86bab2d`); @operator fast-forwarded it to master and pushed `86bab2d`. gofmt clean; vet, plain/race/CGO-disabled tests, four CGO-disabled cross-builds and planted-violation guard checks passed.
    * verdict: approved by @reviewer; merged to master and pushed by @operator
    * next: phase 2 is approved and waits with phase 8 for joint integration; phase 8 is in review, then phases 1 and 16 fetch client; phase 10 command waits for phase 9

- [x] pbrain implementation plan (@architect) `76d16b3`
    * description: turn the approved architecture build order into 29 reviewable implementation phases with dependencies, owner lanes and release gates
    * criteria: obtain @architect-2 and @lead-dev approval; include the fixture, both semantic gates, the later goinfer tag, and arm64 Mac release measurement; commit and push the approved plan
    * work: final plan SHA-256 `a2d730494fd34e18e48409f45bfd00994b13eb9ba4c4083771777625b216ee7d`; both reviewers approved; committed and pushed on master as `76d16b3`
    * verdict: complete
    * next: phases 0–28 are tracked in Ready and In progress above

- [x] pbrain owner-fork module renames and runtime pins (@lead-dev-2) `aikit 0b1b868e; goinfer bdf9daf9`
    * description: rename aikit and goinfer to the owner module paths, preserve attribution, pin aikit from goinfer and publish the owner-selected tags
    * criteria: module/import paths and license attribution preserved; requested repository checks run; aikit v1.79.0 and goinfer v0.1.0 tags published to owner-fork origin only
    * work: aikit main merged at `0b1b868e`, tag v1.79.0; goinfer main merged at `bdf9daf9`, annotated v0.1.0 tag pushed to owner-fork origin. @reviewer confirmed the reviewed goinfer head tree matches the merge. Remaining P2 active-document findings are separated into the two fork documentation follow-ups in Backlog.
    * verdict: complete; owner accepted the P2 documentation findings as separate follow-ups
    * next: none

- [x] pbrain architecture, semantic and output-format amendments (@architect) `24879e6`
    * description: record owner decisions, semantic architecture, accepted amendments and fixed shared interfaces
    * criteria: preserve the Embedder seam and gates; update §3/8.7/13–17 and semantic.md for accepted BM25, the wider fixture, SQLite/sqlite-vec and both models; include schema, transaction/index-lock model, per-section file, catalog replacement, read validation and in-memory journal. Record renamed owner-fork module paths, version tags/pins and topic branch → fork-main PR → upstream workflow, and `go install @version` support without replace directives. Record the fast tokenizer pack, verified-byte encoder construction and immutable-buffer lifetime. Use aikit's encoder for bge-small; retain hugot only as documented fallback. Remove GOEXPERIMENT=simd, the s3 shim and envguard, while retaining stdout/stderr redirection to null. Add the accepted long-document evidence metrics and section-alignment tests. Record the owner's decision that a lexical snippet names the section where its selected window starts. Add the owner-approved Item definition in §4.1 and the type's doc comment with library pins. Also apply signed output-format joint `1c5a95f0`: `file_chars` included; search default `--max-chars=3000`, `--limit=5`; exact text/JSON hit shapes, coordinates/units, sizes, body-start accounting, occurrence count, `get --offset`, bounded opt-in line context and color, examples and budgets. Update the functional spec and issue the component owners' change list. Requests review from @architect-2 and @lead-dev.
    * work: fix pass applied in one delta, not staged or committed. Current functional spec SHA-256 `d1ba263730a163d1209dc9a9ab34c88b08aa732fac947c75eaeb950da748e177` (base `ad0a4f74`); architecture `ed3682b8c013d439d98fd30f6f5f74ae98084f85f332c5b4449066d2bcb45f0c` (base `50c865ad`); semantic `5f4a23ca0a1d6e703ec7af0fce4891dbd9107d1bc6d12c81585c2ab67c449294` (base `6a10dfa2`, Our libraries pin update only). All three reported hashes verified against files. Six findings addressed: paths safely escaped or withheld for invalid UTF-8; color/stdin detection and stored body bytes separated; exact required fields survive combined tiers; `FitBody` uses Format and fits inclusive file-line ranges before decoration; item/store expose Layout/Measure and coordinate mapping uses a LineCursor; context boundaries and snippet-less semantic anchors defined. Added combined-mode, path, literal-escape, line-slice and mapping cases. Also applied both library pins and the owner-approved Item definition in architecture §4.1/type comment: aikit `v1.79.0` (author reports verified tag) and goinfer `v0.1.0`; the owner-selected goinfer v0.1.0 tag is in `24879e6`. Author ran sha256, table/escape checks, path escape check and stale-word sweep. Component follow-up list: @lead-dev-2 item/search, @lead-dev store, @lead-dev render, @architect-2 cli/skill; other component docs unchanged.
    * verdict: complete; approved design package committed on master as `24879e6`
    * next: none

- [x] pbrain component: brain (@lead-dev) `24879e6`
    * description: design brain discovery/configuration, scrubbed-environment Git queries and exposure guard
    * reviewer: @architect-2; `reviews/components/brain-architect-2.md`
    * work: SHA-256 `2b0cfed0d928e069e2a1db74421f68fb1625faf359371c352fa3cf21ed42009b`; both review findings resolved; included in approved package commit `24879e6`
    * verdict: complete; approved design package committed on master as `24879e6`
    * next: none

- [x] pbrain component: store (@lead-dev) `24879e6`
    * description: design sys folder handles/locks, parallel scan, guarded inputs and write primitives/hard-link publish
    * reviewer: @architect-2; `reviews/components/store-architect-2.md`
    * criteria: preserve approved store design; update the scan to call `item.Measure(data, bodyAt)` after Parse and retain `Item.Layout` on each item; no other store behavior changes
    * work: output delta SHA-256 `396c02b72096a5cfd48e8b78f476468bf65d22fa97dea594dd989ccdec68dec6` (base `f6d47833`); scan calls `item.Measure` on the validated bytes and bodyAt from the same successful Parse before retention filtering; Layout survives both modes and KeepNone verifies later reads. Full hash confirmed by @architect-2; verdict in `reviews/components/store-architect-2.md` (`f48c135f`); @lead-dev confirms both render and store outputs are done; included in approved package commit `24879e6`.
    * verdict: complete; approved design package committed on master as `24879e6`
    * next: none

- [x] pbrain component: writes (@lead-dev) `24879e6`
    * description: design add/verify/delete, secret scan, WritePlan, commit points and `after_write`
    * reviewer: @architect-2; `reviews/components/writes-architect-2.md`
    * work: SHA-256 `b204c350773f5b8d7a52d8a01311295631c2d4599da368f431090273070f9a2b`; verification retains the initial file version, delete preview probes originals read-only, and scanner boundaries/redaction are specified; included in approved package commit `24879e6`
    * verdict: complete; approved design package committed on master as `24879e6`
    * next: none

- [x] pbrain component: item (@lead-dev-2) `24879e6`
    * description: design front matter parse/emit/edit, ids, standing and fold rule
    * reviewer: @lead-dev; `reviews/components/item-lead-dev.md`
    * criteria: preserve current parser/emit design; add `Layout` and `Measure(data, bodyAt)` counts (BodyLine, BodyChars, FileLines, FileChars), `Item.Layout`, `LineOf` and the rising-offset `LineCursor`; keep Parse signature; Measure accepts only the bodyAt from a successful Parse; specify BOM handling before first `---` and add LF/CRLF/BOM/no-final-LF/empty-body tests, plus the Item doc comment
    * work: final output delta SHA-256 `69970eba83125c3a02d4c19ac0b93e4a065f8408acd75db32ef64259ccdd33bb`; @lead-dev approved after recheck. Measure and FuzzMeasure use the same valid UTF-8 bytes and successful Parse offset. Verdict appended to `reviews/components/item-lead-dev.md`; included in approved package commit `24879e6`.
    * verdict: complete; approved design package committed on master as `24879e6`
    * next: none

- [x] pbrain component: graph (@lead-dev-2) `24879e6`
    * description: design graph, `graph.Effect`, dangling links and `current_again`
    * reviewer: @lead-dev; `reviews/components/graph-lead-dev.md`
    * work: current SHA-256 `06c2de16e4cdcf881f30077f4ddd3585203c851f0df7d55c323e6ae46390d12f`; traversal order, view-relative resolution and stamp scoping fixes approved at `935042aa`; generic lookup comment cleanup applied; included in approved package commit `24879e6`
    * verdict: complete; approved design package committed on master as `24879e6`
    * next: none

- [x] pbrain component: search (@lead-dev-2) `24879e6`
    * description: design query parsing, lexical scan/ranking, time windows, per-clause counts, hybrid merge and BM25 partial tier
    * reviewer: @lead-dev; `reviews/components/search-lead-dev.md`
    * criteria: preserve the approved BM25/semantic design; add Request.Context; add snippet Line and Context []ContextLine for every snippet, keeping Section only when Long; omit snippets for title-only semantic hits; add Hit.Occurrences from BM25 tf counting; distinguish corpus-wide BM25 TF from page-row occurrence counts and test the specified behavior
    * work: final output delta SHA-256 `1c9e64d7bef69533773e59d6413d3040d5156be20f88c8b1268ff33bdc8b8925`; @lead-dev approved after recheck. Context tests distinguish lone CR from CRLF and cover exclusive endpoints; BM25 corpus TF and page-row occurrence counts are separate; no-BM25 test isolates the fast match pass. Verdict appended to `reviews/components/search-lead-dev.md`; included in approved package commit `24879e6`.
    * verdict: complete; approved design package committed on master as `24879e6`
    * next: none

- [x] pbrain component: render (@lead-dev) `24879e6`
    * description: design `render.Doc` fitting/budgets and search/get/list/prime/tags/check output, including `get --outline` and BM25 partial rows
    * reviewer: @architect; `reviews/components/render-architect.md`
    * criteria: preserve the approved rendering design and add output-format fields/templates (location, sizes, occurrences, path encoding), get line ranges and outline file line; update Fit/FitBody to Format, add Style/marks, implement context blocks and color fitting, preserve list/prime/history rows, update 3,000-char assumptions and alignment fixture
    * work: output delta SHA-256 `27d3f1ba6e4f02968891c2b314d8d3c8bc83f912b8b00ed7a711b6efe661b40c` (base `3f204821`), verified against file. @architect reports seven-line fix of four printed-byte findings; verdict in `reviews/components/render-architect.md`. Prior handoff `05b65201` was superseded. No pending review; included in approved package commit `24879e6`.
    * verdict: complete; approved design package committed on master as `24879e6`
    * next: none

- [x] pbrain component: semantic (@architect) `24879e6`
    * description: design the semantic search/index subsystem
    * reviewer: @architect-2 and @lead-dev; `reviews/components/semantic-architect-2.md`, `reviews/components/semantic-lead-dev.md`
    * work: SHA-256 `6a10dfa2cae1d81e2897c251483b165aa70a6387960e6c449501e56135703f1d`; exactly five model-file paths and read-attempt accounting are checked before load, with the immutable zero-copy handoff retained. Evidence selection/judging/measuring uses one fitted JSON stream including escaping; `TestCalibrate` tests differing text/JSON fits. Aikit/goinfer use the renamed owner modules without replace directives; final pins await module-path PR merges. Uncommitted.
    * verdict: complete; approved design package committed on master as `24879e6`
    * next: none

- [x] pbrain component: cli (@architect-2) `24879e6`
    * description: design command table, pflag parsing, usage/JSON errors, exit codes and clock/time-zone handling
    * reviewer: @architect; `reviews/components/cli-architect.md`
    * criteria: add search-only `--context` (0 by default, explicit 1–3 only; invalid values are usage errors) and `--color` on search/get (usage error with `--json`); pass Context and TextColor through renderer; cover scenarios 39–41
    * work: output-format delta SHA-256 `563240ef96a1b61d3794ea068f72dd9c141b1f45fb9a405fdd18101da2afee51` against spec `d1ba2637` and architecture `ed3682b8`; registrations, links, fences and navigation checks reported passed; included in approved package commit `24879e6`. Base runtime change at `a0b68d15` removes envguard and retains stdout/stderr redirection.
    * verdict: complete; approved design package committed on master as `24879e6`
    * next: none

- [x] pbrain component: audience (@architect-2) `24879e6`
    * description: design canonical form, rule type/audience decision and audience command
    * reviewer: @lead-dev-2; `reviews/components/audience-lead-dev-2.md`
    * work: SHA-256 `58070db6c2b16ee1555318633eb2878f1a0c32fbe5b4670c285ebddf8142b382`; source classification, rule recovery, listing and transaction findings resolved; included in approved package commit `24879e6`
    * verdict: complete; approved design package committed on master as `24879e6`
    * next: none

- [x] pbrain component: importer (@architect-2) `24879e6`
    * description: design file imports, linked items, fetch and HTML extraction
    * reviewer: @lead-dev-2; `reviews/components/importer-lead-dev-2.md`
    * work: SHA-256 `02229ba40dbfcda9c196edb95443910467fcc99fa18ed56fcb55593375db3278`; base and streaming `Download` addendum approved; interrupted downloads restart from zero; included in approved package commit `24879e6`
    * verdict: complete; approved design package committed on master as `24879e6`
    * next: none

- [x] pbrain component: skill (@architect-2) `24879e6`
    * description: design skill text layout, go:embed, word limits and topic tests
    * reviewer: @architect; `reviews/components/skill-architect.md`
    * criteria: add the navigation sentence from spec 12, distinguishing path:line for file tools from body offset for get; mention optional context and leave color to people; maintain the 400-word core cap
    * work: output-format delta SHA-256 `763d7e76f94ccbff157ff15e003a6d4e87f10208e14769b7778fb055a3ed6421`; author and reviewer report exactly 400 words in core (including Markdown bullet markers), topics 219/101/79/96; included in approved package commit `24879e6`. Base BM25 delta `fb4cd0c4` remains.
    * verdict: complete; approved design package committed on master as `24879e6`
    * next: none

- [x] pbrain component consistency and owner acceptance (@architect) `24879e6`
    * description: reconcile the twelve component designs against fixed shared types, error/notice codes and build order
    * criteria: every component signature, code and dependency agrees with architecture and functional spec; record all verdicts and hashes
    * work: current parent hashes: functional spec `d1ba2637`, architecture `ed3682b8`, semantic `5f4a23ca`; parent amendment approved by both reviewers. Current output deltas: cli `563240ef` and skill `763d7e76`, approved by @architect; store `396c02b7`, approved by @architect-2; render `27d3f1ba`, approved by @architect; item `69970eba` and search `1c9e64d7`, approved by @lead-dev. All six assigned output deltas have reviewer approvals. Prior consistency pass at older parent hashes was clean; final pass on these outputs has not run. Files remain included in approved package commit `24879e6`.
    * verdict: complete; approved design package committed on master as `24879e6`
    * next: none

- [x] pbrain owner question: Item versus Document naming (@architect, @architect-2, @lead-dev, @lead-dev-2)
    * description: recommend the term for the addressable knowledge unit
    * criteria: short independent views, joint recommendation and owner decision; no redesign
    * work: signed `reviews/naming/joint.md`, SHA-256 `aec237b2a6a8663cc470a60b79a607f8059a62f4ef91c4b8884e1321053738b3`. Owner chose to keep `Item` and approved the definition: “An item is one Markdown file of the brain: one piece of knowledge with its header, cited by its id. It can be a decision, a learning, an observation, a note or a source. A `source` item stands for material from outside, as a copy or as a pointer, with or without a summary; what is reusable in such material can also be recorded as items of the other kinds.”
    * verdict: complete; owner-approved wording is included in architecture §4.1 and the Item comment in `24879e6`
    * next: none

- [x] pbrain owner question: output-format hit shape (@lead-dev, @architect, @architect-2, @lead-dev-2)
    * description: jointly define a concrete v1 search-hit output shape for agents
    * criteria: joint specifies text/JSON fields, coordinate units and get continuation, body-start and file/body sizes, occurrence counts, optional context and color, fit costs and a worked five-hit example; owner resolves remaining choices
    * work: signed `reviews/output-format/joint.md`, SHA-256 `1c5a95f0da427411feab233c488c5d2c9ce789fd5451a5ea47dd544a04b533d2`; all three peer sign-offs recorded. Owner chose `file_chars` included, search default `--max-chars=3000`, `--limit=5`; example remains 1,214 text / 1,922 compact JSON code points. Owner assigned application as the in-progress architecture/semantic/output-format amendment above.
    * verdict: complete; owner-approved output shape is included in the spec package commit `24879e6`
    * next: none

- [x] aikit `encoder.LoadBERTFromFS` implementation (@lead-dev-2) `04b54e26`
    * description: add the filesystem-backed BERT loader required by the pbrain verified-bytes adapter
    * criteria: follow aikit contribution rules; compare with `LoadBERT` on bge-small files; verify each model file opens once; preserve immutable buffer lifetime; run repository preflight and relevant tests; do not push/PR without owner approval
    * work: commit `04b54e26` on `pbrain/load-bert-fs`; owner merged into aikit main as `eb8c5ddd` and opened upstream PR #38. Focused CGO-disabled parity/open-once/missing-file checks passed; real-checkpoint test skipped due to absent scratch model assets. Report appended to `goinfer-bge-small-check.md`, current reported SHA-256 `0d4651e3d0445a530806e7ef3625451024b06283f6b6e27783ed647b40d6d2d0`.
    * verdict: complete; code merged to owner fork main; module-path rename is tracked separately
    * next: none for this loader task; final pbrain module pin waits for the separate rename PR

- [x] pbrain source research: ken (@architect-2)
    * description: assess github.com/townsendmerino/ken for direct adoption or ideas useful to pbrain
    * criteria: identify maturity, licence, activity, dependencies and relationship to goinfer/aikit; compare storage, retrieval, memory, ergonomics and output-budget fit; recommend adopt/borrow/ignore by pbrain section; read source before running, do not edit specs
    * work: `specs/projects/pbrain/research/ken-review.md`, SHA-256 `8665fe41ad3a51f09f47876ed17c48825a7493c94097b880f69c58a46d82c99e`; reviewed ken commit `2d21e3d4`, MIT, v1.20.0, active with one human maintainer; no code run, proposals only, uncommitted.
    * verdict: owner adopted both test-plan proposals only: measure whether long-document questions retrieve the correct item and expose the supporting passage, evidence-reaching fraction and tokens to first evidence including follow-up get reads, with misses separate; test passage offsets against get text and intended outline section using repeated headings, fenced pseudo-headings and multibyte text. No product behavior change.
    * next: additions are incorporated in the current architecture and semantic design passes; no direct spec edits were made by the researcher

- [x] pbrain goinfer fork spike: fast EmbeddingGemma 2 tokenizer (@lead-dev-2) `e98418db`
    * description: test a prebuilt tokenizer representation to make goinfer's EmbeddingGemma 2 load fast enough for the 8.7 budget
    * criteria: work in `~/Dev/pkieltyka/goinfer` on local branch `pbrain/fast-tokenizer`; follow repo instructions and read source before running; target tokenizer load in tens of milliseconds and exact IDs against reference text; report pack size and integration with bytes loader; owner later authorized push and fork PR, merged to fork main; upstream PR #1194 remains open
    * work: report `research/semantic-search-and-index/embedding-models-and-go-runtime/goinfer-tokenizer-spike.md`, SHA-256 `bcda0e77a23dc2908aa1d6ce5d36594e087af7d57de9f2b23cb03f7d86a80da0`. Owner merged tokenizer work to goinfer main as `2f80ef14`, then changelog update as `2e413ee111048fff90b9c74bdbdaf6f02299bba4`; upstream PR #1194 is open. Topic commits were `6f6806d5`, `bc477653`, `e98418db`. Pack load 24–26 ms vs 441–502 ms JSON, deterministic build 0.75–0.84 s, 9.58 MB, SHA-256 `45a7e7d5…` for tokenizer revision `f9972ac2`. Exact IDs on 48 golden texts, 10,000 random encodings, and 3,052 texts/250,897 tokens vs HF; tests/fuzz/lint passed.
    * verdict: complete; @operator retires the tokenizer-load risk for the EmbeddingGemma 2 fork path
    * next: tokenizer pack and constructor are incorporated into the approved semantic/architecture designs; no further tokenizer task remains

- [x] pbrain runtime check: bge-small on goinfer/aikit (@lead-dev-2)
    * description: determine whether aikit's encoder can replace hugot for bge-small-en-v1.5 under the §8.7 runtime and Artifact rules
    * criteria: verify parity, no-cgo/runtime behavior, in-memory loading, check+load+query time, indexing speed, memory, binary size, environment/stdout; report bottom-line recommendation
    * work: `research/semantic-search-and-index/embedding-models-and-go-runtime/goinfer-bge-small-check.md`, SHA-256 `0d4651e3d0445a530806e7ef3625451024b06283f6b6e27783ed647b40d6d2d0`; aikit v1.78.0 matched token IDs/vectors on 22 texts (worst 1−cos 1.7e-13), 13/17 on 20 questions, 124–153 ms verify/load/first query, 0.14 GB search memory, ~0.9 MB binary addition and 2,800 tok/s. The loader gap was implemented as `LoadBERTFromFS` commit `04b54e26`, merged to fork main `eb8c5ddd`; real-checkpoint focused test was skipped because scratch model assets were absent.
    * verdict: investigation complete; owner accepted the switch to aikit's encoder and dropping hugot, with hugot retained as fallback. The owner fork is `github.com/pkieltyka/aikit`.
    * next: adapter design is recorded; owner-fork module-path rename and final pin are tracked separately

- [x] pbrain owner follow-up: BM25 in v1 design, round 2 (@architect, @architect-2, @lead-dev, @lead-dev-2)
    * description: jointly design BM25 integration after the owner leaned toward keyword + BM25 + semantic
    * criteria: four independent round-two papers, joint `joint-bm25.md`, and three peer sign-offs; cover placement/order, abstention, engine, deterministic ties, decision-in-force, counts and default/flag behavior without editing product specs
    * work: final `reviews/search-methods/joint-bm25.md`, SHA-256 `90a5b60b3353c4313d9a03e77de9fff5062c070ec1d573a96820cfaacaf15462`; four notes: @architect `round2-architect.md` `d3714a1b`, @architect-2 `round2-architect-2.md` `30ce16b6`, @lead-dev `round2-lead-dev.md` `e7279aa8`, @lead-dev-2 `round2-lead-dev-2.md` `fe7b4ebc`; approved by @architect, @lead-dev and @lead-dev-2; no commit
    * verdict: owner accepted the joint proposal: opt-in `--bm25`, exact → semantic → marked partial, Go scoring over the scan, hold partial results unless the held-out gate passes; prior preferences are recorded as splits and the owner resolves the default by choosing opt-in
    * next: none; implemented in the approved spec package

- [x] pbrain owner question: search methods and lessons from qmd (@architect, @architect-2, @lead-dev, @lead-dev-2)
    * description: prepare one joint group answer to `specs/projects/pbrain/reviews/search-methods/owner-question.md`
    * criteria: each member writes an independent concise `reviews/search-methods/round1-<name>.md`; @architect drafts `joint.md`; the other three sign off or object; report final hash to @operator. Evaluate BM25/any-word ranking, RRF, query expansion/reranking, stemming, typos, phrase/proximity, field weights, chunking/context, graph, recency, outline-first and other qmd ideas with evidence/gain, costs and v1/later/never recommendations.
    * work: final `reviews/search-methods/joint.md`, SHA-256 `c114a55ad9c37cb135255120436fcc4664c827e920340c701272c068f6b5423c`; all four round-one papers are recorded above; three sign-offs verified at this hash; corrected “Four things follow” heading; uncommitted
    * verdict: round-one recommendation approved by @architect-2, @lead-dev and @lead-dev-2; its “BM25 out of v1” position was superseded by the owner's follow-up request and is answered by the separate round-two joint task below; other recommendations retain no fusion, no expansion inside pbrain, no v1 reranker/stemming/typo mode, and approved chunking, graph, recency and outline behavior
    * next: none; owner decisions were applied in the approved spec package

- [x] pbrain runtime research: rembed EmbeddingGemma 2 (@architect-2)
    * description: identify and evaluate the pure-Go rembed candidate against the goinfer rig and EmbeddingGemma 2 references
    * work: `research/semantic-search-and-index/embedding-models-and-go-runtime/rembed-embeddinggemma2.md`, SHA-256 `17b7f02875931a7c54c5efbff14878811771cff449bb27abf646b3456b79fc78`; identified fork `f401a554`; source audit, 22-text vectors/token IDs, throughput windows, retrieval fixture, startup/hash timings, clean streams/traces and four CGO-disabled builds reported; uncommitted
    * verdict: not suitable as runtime now: 21/22 parity (literal `<eos>` mismatch), 1.10–1.13 s load plus first query before verification, 1.62–1.75 GB peak, and 9/20 top-1 and 12/20 top-3; keep bge/hugot interim and goinfer as the EmbeddingGemma 2 reference/conditional base
    * next: included in the signed joint comparison

- [x] pbrain runtime comparison and recommendation (@architect, @architect-2)
    * description: compare goinfer, rembed and accepted bge-small/hugot interim runtime
    * criteria: recommend which runtime pbrain should adopt for EmbeddingGemma 2 and when, whether to change the interim choice, and whether the owner's own library should build on one candidate or is unnecessary; preserve the Embedder seam and §8.7 rules unless both authors justify a change
    * work: `research/semantic-search-and-index/embedding-models-and-go-runtime/embeddinggemma2-runtime-comparison.md`, current SHA-256 `e81bacd7d5937a06459b2d8c6133bbd6d98d9a886f577b345865644e9a288fe8`; @architect-2's approval at `18e7c367` carries to the five-word clarification; compares all three runtimes and cross-checks figures against both goinfer reports and the bge spike; uncommitted
    * verdict: signed by @architect-2; its recommendation was to keep bge-small/hugot interim and conditionally build on goinfer; the owner later superseded the “do not adopt either EG2 runtime” recommendation by deciding to support both models, pending an architecture amendment and goinfer-path choice
    * next: owner-directed follow-up tracked in the active architecture and BM25 tasks

- [x] pbrain runtime architecture review: goinfer EmbeddingGemma 2 (@architect)
    * description: assess whether measured goinfer can meet architecture §8.7 and the hashed-bytes Artifact contract
    * work: current `research/semantic-search-and-index/embedding-models-and-go-runtime/goinfer-architecture-review.md` SHA-256 `d0bd03c86e9601659277cc562c0798fe88af6e0e0129d023c1179756ebc5a105`; source reviewed before scratch prototype; keeping BF16 resident cut weight loading from about 1 s to 97–137 ms and memory from 1.87 GB to 1.43 GB, but full Artifact read/hash/load/query remained 948–1,042 ms because tokenizer loading takes 500–550 ms; tokenizer prebuild is also needed to reach estimated 360–450 ms; recommendation separates architecture gates from later adoption; 1.49 GB is a model cost shared by runtimes; not committed
    * verdict: do not adopt goinfer now and do not change the interim runtime; if the owner chooses EmbeddingGemma 2, build on goinfer rather than start a new library, after the retrieval gates pass
    * next: incorporated in the signed joint comparison

- [x] pbrain interim-runtime spike: bge-small on hugot pure Go (@lead-dev-2)
    * description: validate bge-small-en-v1.5 against architecture §8.7 and the semantic adapter contract
    * work: `research/semantic-search-and-index/embedding-models-and-go-runtime/spike-bge-small-hugot.md`, SHA-256 `ab5fdde36658543289009ef850052cbbf5e6fef16b301a768c0c74e0bcccbc2d`; pure-Go builds, vector parity, Artifact-bytes shim, latency, memory, indexing and paraphrase checks reported; uncommitted
    * verdict: operator accepts as interim runtime under four conditions: release builds use `GOEXPERIMENT=simd` and arm64 Mac measurement; use the in-memory `s3:` shim (upstream fix preferred); adapter owns tokenization, 510-token truncation, special-token neutralization and CLS pooling; pin module/model digests and test stdout silence while handling `GOMLX_*` so they neither fail commands nor write stdout
    * next: none; adapter design and reviews are recorded above

- [x] pbrain runtime research: goinfer EmbeddingGemma 2 (@lead-dev-2)
    * description: independently assess `goinfer/embeddinggemma2` as an interim or default EmbeddingGemma 2 runtime
    * work: report `research/semantic-search-and-index/embedding-models-and-go-runtime/goinfer-embeddinggemma2.md`, SHA-256 `af97e506a666d029ee1c8a4c202e86dc5fd1606ced095eb79bf6edfe73d782a8`; pure-Go MIT implementation reproduces sentence-transformers vectors, but 1.26–1.39 s load fails the 600 ms rule; operator retains it as the best base to extend, not the current runtime
    * verdict: investigation complete; not accepted as runtime as-is
    * next: none; interim runtime spike assigned separately

- [x] pbrain research: semantic search and index (@lead-dev, @lead-dev-2, @architect)
    * description: research whether built-in semantic/hybrid retrieval belongs in pbrain v1 and how to design its index and long-document support
    * criteria: follow `specs/projects/pbrain/research/semantic-search-and-index/research_plan.md`; @lead-dev covers qmd and Go index/hybrid engine; @lead-dev-2 covers embedding models/Go runtime and long documents/PageIndex/references; @architect writes the cross-subtopic `research_summary.md` with recommendation, functional-spec implications and open owner decisions
    * work: all three subtopics and the top-level `research_summary.md` are complete; research files currently exist as untracked work under `specs/projects/pbrain/research/semantic-search-and-index/`. The owner answered all eight research decisions; the summary informs the owner-approved functional-spec amendment and architecture design.
    * verdict: research complete; findings accepted as the basis for owner decisions and design
    * next: none; implementation/design proceeds under the assigned amendment and architecture tasks

- [x] pbrain spec: functional spec (@architect) `9f9f6c3`
    * description: draft and refine `specs/projects/pbrain/functional_spec.md` through five owner-directed batches
    * criteria: record owner Q&A; apply batches 1–5; preserve item front-matter YAML; resolve section references and parse YAML/TOML/JSON examples; obtain targeted approvals from @architect-2 and @lead-dev; status complete on the owner's word
    * work: batch 5 approved at SHA-256 `41605322ceac5a7edf841528e21bf5940f0a9ca31b6c388751d08d7469bf90d1`; 26 references resolve and all 13 structured samples parse. Owner/operator set status complete; the final owner-approved spec is included in package commit `24879e6`. Iteration commit `9f9f6c3` contains prior spec work; final approved spec and reviewer updates are in package commit `24879e6`.
    * verdict: complete on the owner's word; batch 5 approved by both targeted reviewers
    * next: implementation proceeds through phases 0–28 above

- [x] pbrain spec: research, browser use and permission-gated sources (@lead-dev, @lead-dev-2, @architect) `1de8c0a`
    * description: research browser and harness-native routes to import rendered or permission-gated content into pbrain
    * criteria: follow `specs/projects/pbrain/research/browser-and-gated-sources/research_plan.md`; @lead-dev covers browser automation over CDP, including agent-browser CLI and anomaly/OpenCode Browse extension; @lead-dev-2 covers harness-native readers for Claude artifacts, Codex, agy/Google Docs, and pbrain skill topics; @architect writes the cross-subtopic `research_summary.md`
    * work: both subtopics complete with four docs each; summary by @architect complete; gated-source questions answered and audience-rule design adopted by the owner; committed in `1de8c0a`
    * verdict: complete; research feeds the functional spec
    * next: none

- [x] factory setup: refresh to the latest factory skill (@operator) `9ea029e`
    * description: refresh the factory setup guidance and portable sample to match the latest factory skill
    * criteria: update FACTORY.md Tasks, Project, and Specs sections and make `factory.toml.sample` portable
    * verdict: complete; committed in `9ea029e`
    * next: none

- [x] pbrain spec: project overview rewrite (@architect) `4f1988e`
    * description: rewrite the brain-dump intro into a clean overview
    * criteria: the owner's criteria sent to @architect; deliver `specs/projects/pbrain/project_overview.md`
    * verdict: complete; owner approved; no peer review needed for the owner's document
    * next: none

- [x] pbrain spec: research, knowledge-memory-landscape (@lead-dev, @lead-dev-2, @architect) `4f1988e`
    * description: five subtopics and the cross-subtopic `research_summary.md`
    * criteria: @lead-dev covers qmd, beads and Gas Town, and source ingestion; @lead-dev-2 covers Markdown knowledge stores and agent memory systems; @architect writes `specs/projects/pbrain/research/knowledge-memory-landscape/research_summary.md`
    * verdict: complete; feeds the functional spec
    * next: none
