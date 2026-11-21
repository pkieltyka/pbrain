TODO
====

## Ready

## In progress

- [ ] pbrain phase 28: measurement before a release (@lead-dev)
    * owner: @lead-dev
    * range/status: Gemma full-size index active; latest sample 1,030 passages at 0.895/s, ~15 h projected, 1.89 GiB peak so far; BGE queued after Gemma
    * next: finish Gemma then same-size BGE; final read-only pass before first release tag; skill/README audit at `6dee71e` needs rerun after `d010e0e`

- [ ] pbrain measured-cost and path-safety spec updates (@architect)
    * owner: @architect
    * range/status: on `d010e0e`; qualify 15 h/2 GB as projected/observed, say resume from saved checkpoint, and specify UTF-8 admission plus exact-path escaping in spec/designs/scenario 39
    * next: send hashes to @operator; lead-dev-2 implements dependent safety fixes after spec lands

- [ ] pbrain release output fixes and scenario coverage (@lead-dev-2)
    * owner: @lead-dev-2
    * range/status: branch `peter/lead-dev-2` at `3c3206c`; P1 ExactPath/UTF-8 waits for spec; assigned overflow, JSON context, skip notices, empty-color fit, singular counts, linear page sizing; tests 39/40/41, semantic CRLF/BOM/text, hybrid all tiers
    * next: implement independent fixes now; finish dependent fixes after spec; use `-p 2`, no model/index, request reviewer and architect-2 recheck

- [ ] pbrain final CLI/render finding re-check (@architect-2)
    * owner: @architect-2
    * range/status: final pass on master `7cf8ced` found P1/P2/P3 issues; waiting for lead-dev-2's fix range
    * next: re-check own findings after implementation and report to @operator

- [ ] pbrain final functional-scenario audit (@reviewer)
    * owner: @reviewer
    * range/status: read-only audit against functional_spec assigned on master; code unchanged by `d010e0e`
    * next: report scenarios without tests or with mismatches to @operator and @pm

## Backlog

- [ ] Scope repeatable `--tag` help text per command (filter vs assign)

- [ ] Show search defaults (`--limit 5`, `--max-chars 3000`) in command help

- [ ] Hide unused global `--brain` and `--personal` options from `pbrain skill --help`

- [ ] Repeat skill/README binary audit after first-index text update `d010e0e`

## Decisions

- @operator (2026-11-21): retain `embeddinggemma-2` as default despite the ~15 h projection for 5,000 token-heavy items/50 MB; compare same-size BGE after Gemma and hold the first release tag for full-size timings and the final read-only pass.
- @operator (2026-11-21): exclude item stems with invalid UTF-8 and report them through skipped-file/check diagnostics, preventing distinct raw names from folding to one displayed ID.

## Done

- [x] pbrain pre-release skill/README audit (@lead-dev-2) `6dee71e`
    * No mismatch against shipped skill, model guidance or quick start; rerun needed because README/topic changed in `d010e0e`.

- [x] pbrain release pipeline (@lead-dev-2) `7cf8ced`
    * Reviewer approved; GoReleaser builds four CGO-disabled targets and publishes a draft on tag push. No tag pushed; first workflow run remains untested.

- [x] Goinfer fork documentation follow-up (@architect-2) `e33aa3893`
    * PR #8 approved and merged; v0.4.0 tag waits for owner approval.

- [x] pbrain README for phase 28 (@architect-2) `676bf6e`

- [x] aikit fork documentation follow-up (@architect-2) `0bbc048`

- [x] Goinfer upstream sync (@lead-dev-2) `765dab39`

- [x] pbrain phase 16: URL imports (@architect-2) `f753d2c`

- [x] pbrain phase 15: importer files and linked items (@architect-2) `e3b694b`

- [x] pbrain phase 11 measurement harness (@lead-dev) `606bbfc`

- [x] pbrain init selector usable from caller's directory (@architect-2) `90b4644`

- [x] pbrain spec: record released cut-offs and model state (@architect) `4eff5a7`

- [x] pbrain phase 27: EmbeddingGemma 2 gates and release wiring (@architect-2) `2233197`

- [x] pbrain phase 25: BGE gates and release wiring (@lead-dev-2) `8f32814`

- [x] pbrain phase 18: calibration and `--bm25` gates (@lead-dev) `e5aa38a`

- [x] pbrain skill guidance: first index on a machine (@architect) `05f788e`

- [x] pbrain spec: EmbeddingGemma 2 default model (@architect) `7bdf4b2`

- [x] pbrain spec: runtime audit covers all release targets (@architect) `20fd0e6`

- [x] pbrain runtime audit: all release targets (@architect-2) `eb0441f`

- [x] pbrain phase 27 adapter: EmbeddingGemma 2 (@architect-2) `2ab7d56`

- [x] pbrain architecture: audience-rule host matching (@architect-2) `1c075b5`

- [x] pbrain architecture: deterministic ordering for same-line check findings (@operator) `db929d2`

- [x] pbrain architecture: anchor printed paths (@architect-2) `met`

- [x] pbrain architecture: avoid duplicate brain on bare init (@architect-2) `met`

- [x] pbrain phase 12: Reading commands II: `list`, `prime`, `tags`, `check` (@lead-dev-2) `db929d2`

- [x] pbrain phase 19: Semantic (a): the merge (@lead-dev-2) `f8b21a0`

- [x] pbrain phase 11: Reading commands I: `search` and `get` (@lead-dev-2) `1d44e76`

- [x] pbrain phase 22: Semantic (b2): the build (@lead-dev-2) `1cc81c4`

- [x] pbrain phase 24: The bge-small adapter (@lead-dev) `17734d4`

- [x] pbrain phase 14: The `audience` command (@architect-2) `bc89d99`

- [x] pbrain phase 10: `skill` (@architect-2) `06b452a`

- [x] pbrain phase 13: Writing commands (@lead-dev) `96c6516`

- [x] pbrain phase 9: `cli` core (@lead-dev) `05c9317`

- [x] pbrain phase 20: Semantic (b1): passages and the index database (@lead-dev-2) `c6641ee`

- [x] pbrain phase 21: `embed`: the model table and provisioning (@lead-dev-2) `c6641ee`

- [x] pbrain phase 6: `query` and `search`, lexical (@lead-dev) `14f35cc`

- [x] pbrain phase 7: store (@lead-dev-2) `9e66477`

- [x] pbrain phase 5: brain (@lead-dev-2) `c2f3107`

- [x] pbrain phase 4: graph and secretscan (@lead-dev) `991e646`

- [x] pbrain phase 1: outline and item (@lead-dev) 31acdb7

- [x] pbrain phase 8: render (@lead-dev-2) e2a5d01

- [x] pbrain phase 2: sys and gitx (@lead-dev-2) e2a5d01

- [x] pbrain phase 26: goinfer for EmbeddingGemma 2 (@architect) `a759890b`

- [x] pbrain phase 3: `audience`: the canonical form, the rules and the decision (@architect-2) `6ec6bac`

- [x] pbrain spec pins for goinfer v0.2.0 (@architect) `24879e6`

- [x] pbrain phase 17: The evaluation fixture (@architect-2) `921a1a4`

- [x] pbrain phase 0: Skeleton (@lead-dev-2) `86bab2d`

- [x] pbrain implementation plan (@architect) `76d16b3`

- [x] pbrain owner-fork module renames and runtime pins (@lead-dev-2) `aikit 0b1b868e; goinfer bdf9daf9`

- [x] pbrain architecture, semantic and output-format amendments (@architect) `24879e6`

- [x] pbrain component: brain (@lead-dev) `24879e6`

- [x] pbrain component: store (@lead-dev) `24879e6`

- [x] pbrain component: writes (@lead-dev) `24879e6`

- [x] pbrain component: item (@lead-dev-2) `24879e6`

- [x] pbrain component: graph (@lead-dev-2) `24879e6`

- [x] pbrain component: search (@lead-dev-2) `24879e6`

- [x] pbrain component: render (@lead-dev) `24879e6`

- [x] pbrain component: semantic (@architect) `24879e6`

- [x] pbrain component: cli (@architect-2) `24879e6`

- [x] pbrain component: audience (@architect-2) `24879e6`

- [x] pbrain component: importer (@architect-2) `24879e6`

- [x] pbrain component: skill (@architect-2) `24879e6`

- [x] pbrain component consistency and owner acceptance (@architect) `24879e6`

- [x] pbrain owner question: Item versus Document naming (@architect, @architect-2, @lead-dev, @lead-dev-2) `aec2375e`

- [x] pbrain owner question: output-format hit shape (@lead-dev, @architect, @architect-2, @lead-dev-2) `1c5a95f0`

- [x] aikit `encoder.LoadBERTFromFS` implementation (@lead-dev-2) `04b54e26`

- [x] pbrain source research: ken (@architect-2) `8665fe41`

- [x] pbrain goinfer fork spike: fast EmbeddingGemma 2 tokenizer (@lead-dev-2) `e98418db`

- [x] pbrain runtime check: bge-small on goinfer/aikit (@lead-dev-2) `0d4651e3`

- [x] pbrain owner follow-up: BM25 in v1 design, round 2 (@architect, @architect-2, @lead-dev, @lead-dev-2) `90a5b60b`

- [x] pbrain owner question: search methods and lessons from qmd (@architect, @architect-2, @lead-dev, @lead-dev-2) `c114a55a`

- [x] pbrain runtime research: rembed EmbeddingGemma 2 (@architect-2) `17b7f028`

- [x] pbrain runtime comparison and recommendation (@architect, @architect-2) `e81bacd7`

- [x] pbrain runtime architecture review: goinfer EmbeddingGemma 2 (@architect) `d0bd03c8`

- [x] pbrain interim-runtime spike: bge-small on hugot pure Go (@lead-dev-2) `ab5fdde3`

- [x] pbrain runtime research: goinfer EmbeddingGemma 2 (@lead-dev-2) `af97e506`

- [x] pbrain research: semantic search and index (@lead-dev, @lead-dev-2, @architect) `complete`

- [x] pbrain spec: functional spec (@architect) `9f9f6c3`

- [x] pbrain spec: research, browser use and permission-gated sources (@lead-dev, @lead-dev-2, @architect) `1de8c0a`

- [x] factory setup: refresh to the latest factory skill (@operator) `9ea029e`

- [x] pbrain spec: project overview rewrite (@architect) `4f1988e`

- [x] pbrain spec: research, knowledge-memory-landscape (@lead-dev, @lead-dev-2, @architect) `4f1988e`
