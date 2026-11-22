---
status: complete
---

# Implementation Plan: pbrain

An ordered checklist. The detail is in [architecture.md](architecture.md) (section 18 is the build order this follows) and in the design named with each phase under [components/](components/). Tests are those of each design's test plan and the scenarios of [functional_spec.md](functional_spec.md) section 15.

## Phases

Each phase is one ticket and one review. It names its owner, what must be merged first ("after") and what it can run beside. The two build lanes are @lead-dev and @lead-dev-2, each in its own worktree. The **third lane** is @architect-2, after the fixture or beside it: three pure packages that touch neither lead-dev's code.

- [ ] **0. Skeleton.** @lead-dev-2. Everything waits for it. The module `github.com/pkieltyka/pbrain` on Go 1.27, the MIT licence, CI (format, vet, tests, the race job, the `CGO_ENABLED=0` builds, the check that `go.mod` has no replace or exclude directive), `errs`, and every exported type and signature of architecture 18 step 0 with empty bodies, compiling together.
- [ ] **1. `outline` and `item`.** @lead-dev. After 0; beside 2 and 3. `item.md`; architecture 4.1, 7.4.
- [ ] **2. `sys` and `gitx`.** @lead-dev-2. After 0; beside 1 and 3. `store.md`, `brain.md`; architecture 6.3, 10.3.
- [ ] **3. `audience`: the canonical form, the rules and the decision.** Third lane, else @lead-dev-2. After 0; beside 1 and 2. `audience.md`. The package is pure; `add` and linked imports need its decision.
- [ ] **4. `graph` and `secretscan`.** @lead-dev. After 1; beside 5. `graph.md`, `writes.md`.
- [ ] **5. `brain`.** @lead-dev-2. After 1, 2 and 3; beside 4 and 6. `brain.md`: discovery, configuration, views, the exposure guard.
- [ ] **6. `query` and `search`, lexical.** @lead-dev. After 4; beside 5, 7 and 8. `search.md`: matching, order, window, counts, snippet with its line, occurrences, context lines, and the partial tier behind `Features.BM25`.
- [ ] **7. `store`.** @lead-dev-2. After 5; beside 6 and 9. `store.md`: the scan, guarded input, the write primitives.
- [ ] **8. `render`.** @lead-dev-2. After 0; beside anything. `render.md`.
- [ ] **9. `cli` core.** @lead-dev. After 5; beside 7 and 8. `cli.md`: dispatch, flags, streams, exit codes, `RunWith`, `init`.
- [ ] **10. `skill`.** Third lane, else @lead-dev. The texts and their tests after 0; the command after 9. `skill.md`.
- [ ] **11. Reading commands I: `search` and `get`.** @lead-dev-2. After 6, 7, 8 and 9; beside 13. `render.md`, `cli.md`: `search`, and `get` with `--history` and `--outline`. First whole commands: their scenarios, and the measurements of architecture 8.6 on the generated 5,000-item brain.
- [ ] **12. Reading commands II: `list`, `prime`, `tags`, `check`.** @lead-dev-2. After 11; beside 13 and 14. `render.md`, `cli.md`.
- [ ] **13. Writing commands.** @lead-dev. After 3, 7 and 9; beside 11 and 12. `writes.md`: `add`, `verify`, `delete`.
- [ ] **14. The `audience` command.** @lead-dev. After 13; beside 15. `audience.md`: listing and editing the rules.
- [ ] **15. `importer`: files and linked items.** @lead-dev-2. After 13; beside 14. `importer.md`.
- [ ] **16. `importer/fetch` and URL imports.** The fetch client, with its `Download`: third lane, else @lead-dev-2, after 0. `import <url>`: @lead-dev-2, after 15. Released only when scenario 20 passes.
- [ ] **17. The evaluation fixture.** @architect-2, under way; beside everything. The fixture brain and `questions.json`: answerable questions, reworded ones, unanswerable ones in their two halves with regression marks, the question styles, and an evidence quotation for each long-source question. Frozen in one commit, with ids, before any constant or cut-off is tuned (`semantic.md`, "Calibration"; architecture 14). The owner may add questions before the freeze. The harness needs 11.
- [ ] **18. The calibration tool without a model, and the `--bm25` gates.** @lead-dev. After 11 and 17; beside 19 to 22. The model-free arms and the "reaching the evidence" reader (`semantic.md`); the partial tier's own gates (architecture 8.8). A pass lets `--bm25` ship in a release that lists no model; `Release.BM25` is set in the change that records it.
- [ ] **19. Semantic (a): the merge.** @lead-dev. After 11; beside 20 and 21. `search.md` section 6 and its rendering, against a fake source.
- [ ] **20. Semantic (b1): passages and the index database.** @lead-dev-2. After 7; beside 19 and 21. `semantic.md`: fingerprints, passages, the database and the scoring query.
- [ ] **21. `embed`: the model table and provisioning.** @lead-dev. After 2; beside 19 and 20. `semantic.md`: the registry, find, verify, download through the `Downloader` interface, and pack, with the test model and a fake downloader.
- [ ] **22. Semantic (b2): the build.** @lead-dev-2. After 20 and 21; beside 24. The incremental build, publication, the vector cache, model recovery and their failure tests.
- [ ] **23. Semantic (b3): the source and the commands.** @lead-dev-2. After 19, 22 and the fetch client of 16; beside 24. Readiness, scoring, the status (it asks `embed` whether the model is there), `pbrain index` with `--status`, `search --semantic`, their notices; scenarios 33 to 37 with the test model. Here `cli` hands the real `fetch.Client` to `index` as its downloader.
- [ ] **24. The bge-small adapter.** @lead-dev. After 21; beside 22 and 23. `embed/bge` on aikit v1.79.0, with its parity and runtime tests.
- [ ] **25. bge-small's gates.** @lead-dev with @lead-dev-2. After 18, 23 and 24. The model arms of the tool; the cut-off from the calibration half; the abstention and gain gates; the combined gate with `--bm25`; the evidence figures; time and memory of `search --semantic` and `index`. The model enters the table only when its own gates pass. If only the combined gate fails, with every tested admission rule, the model ships and `--bm25` is held back from that release (spec 16).
- [ ] **26. goinfer for EmbeddingGemma 2.** @architect, in `github.com/pkieltyka/goinfer`. Independent of every pbrain phase; beside anything. The loader from bytes, the encoder's constructor from a built model, tokenizer and prompts, and what the text-only pack needs: a topic branch, a pull request in our goinfer, then a tag after v0.1.0 (architecture 8.7; `semantic.md`, "The EmbeddingGemma 2 adapter").
- [ ] **27. The EmbeddingGemma 2 adapter and its gates.** @lead-dev. After 25 and 26. `embed/gemma2`: the pack and its derive step, the adapter, parity, rule 3; then its cut-off, its abstention and gain gates, and the combined gate with `--bm25` again for this model, with the same outcome on a combined-only failure.
- [ ] **28. Measurement before a release.** @lead-dev-2. After the phases a release ships. The built binary on Linux (the same on an arm64 Mac is owed and tracked as a backlog item of its own, since macOS is deferred: owner's decision, 2026-11-21): the speed targets of spec 16, and for each model rule 3, a first index, an index after one note, and peak memory (architecture 8.6, 8.7, 14); for `embeddinggemma-2` the full-size figures were deferred while the model was on hold (owner's decision, 2026-11-21) and were measured once it was cleared for use (2026-11-22; issue #22, `internal/cli/README.measure.md`): a first index with the `cuda` build on a Ryzen 9 7950X with an NVIDIA RTX PRO 6000, an index after one note, peak memory, and search with and without the GPU. A first index of that brain on CPUs alone was not finished and stays a projection of 15 hours. The four targets built with `CGO_ENABLED=0`; `go install` of a tagged version; results published. Repeated for each release that adds a search mode or a model.

## What can be released when

In the spec's delivery order (section 16), each after phase 28 for what it ships: reads and writes after 14; imports after 15; public fetch after 16; `--bm25` alone after 18; `--semantic` with bge-small after 25; `embeddinggemma-2` after 27. A release that lists a model has `--bm25` only if the combined gate passed for every model it lists. The gates judge rendered pages, so they are run again when what a result prints changes.

## Notes

- **Two places where this order departs from architecture 18**, which stays as it is. `outline` is built with `item`, not after `search`, because `item` takes a title's fallback from it. `secretscan` is built with `graph`, not with the writing commands, because `check` in phase 12 needs it.
- **Renamed since.** The `pbrain index` of phases 23 and 25 is `pbrain sync` (owner's decision, 2026-11-21; spec 9.4). There is no alias: `pbrain index` is an unknown command.
- **Lanes.** @lead-dev-2: 0, 2, 5, 7, 8, 11, 12, 15, 16, 20, 22, 23, 28. @lead-dev: 1, 4, 6, 9, 13, 14, 18, 19, 21, 24, 25, 27. @architect-2: 17, then the third lane: 3, 10 and the fetch client of 16. @architect: 26.
