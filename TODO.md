TODO
====

## Ready

## In progress

- [ ] pbrain phase 28: measurement before a release (@lead-dev)
    * owner: @lead-dev
    * range/status: Gemma full-size index active at 0.895 passages/s, ~15 h projected, 1.89 GiB peak so far; BGE queued after Gemma
    * next: finish Gemma then same-size BGE; first tag waits for timings and final read-only pass; skill/README audit stale after `789b506`

- [ ] pbrain release output fixes and scenario coverage (@lead-dev-2)
    * owner: @lead-dev-2
    * range/status: `0bd5a12..79218bf` on `peter/lead-dev-2`; reviewer found two P2s; fixes are uncommitted in the worktree; supplemental `TestSemanticAlignment` passed on CPUs 4–15 with `-p 2`
    * next: format graph-validation target IDs with `ExactValue`; check and propagate overflow in `unitCost` additions and test each fix; then request re-review, including the semantic alignment run

- [ ] pbrain final skill/README audit (@lead-dev-2)
    * owner: @lead-dev-2
    * range/status: prior gap fixed on master `0bd5a12`; audit target now includes branch head `79218bf`, recheck pending
    * next: report audit findings or approval to @operator and @pm; `-p 2`, no model/index runs

- [ ] pbrain testscript harness and scenarios 1–20 (@architect-2)
    * owner: @architect-2
    * range/status: committed `502db3c` on `peter/scenarios`, based on `0bd5a12`; harness and scenarios 1–20 pass reported checks
    * next: reviewer checks `0bd5a12..502db3c` after the output fixes; operator applies PTY patch `2ba91a38` after both ranges integrate

- [ ] pbrain executable scenarios 21–41 (@lead-dev)
    * owner: @lead-dev
    * range/status: based on committed harness `502db3c` on `peter/lead-dev`; scenarios 21–29 are untracked worktree files, scenario 35 and boundary coverage are planned
    * next: finish scenarios 21–41, including damaged-passage/shared-view and 39–41; `-p 2`, CPUs 4–15; embedtest indexes are allowed, no real model/full-size runs

- [ ] pbrain final functional-scenario audit (@reviewer)
    * owner: @reviewer
    * range/status: output review `0bd5a12..79218bf` requires two P2 fixes; supplemental `TestSemanticAlignment` passed per @architect-2; harness/scenario range `0bd5a12..502db3c` awaits review
    * next: re-review output fixes and run semantic alignment; then review scenario ranges 1–20 and 21–41

## Backlog

- [ ] Scope repeatable `--tag` help text per command (filter vs assign)

- [ ] Show search defaults (`--limit 5`, `--max-chars 3000`) in command help

- [ ] Hide unused global `--brain` and `--personal` options from `pbrain skill --help`

## Decisions

- @operator (2026-11-21): retain `embeddinggemma-2` as default despite the ~15 h projection for 5,000 token-heavy items/50 MB; compare same-size BGE after Gemma and hold the first release tag for full-size timings and the final read-only pass.
- @operator (2026-11-21): exclude item stems with invalid UTF-8 and report them through skipped-file/check diagnostics, preventing distinct raw names from folding to one displayed ID.
- @operator (2026-11-21): use `ExactValue` for control/quote/backslash IDs and paths (spaces remain bare); reject invalid-UTF-8 lookup IDs; use `bad_file_name`; map invalid-name findings to the nearest printable ancestor or `?`; omit invalid-UTF-8 JSON paths.
- @operator (2026-11-21): count only error-severity skipped files in `skipped_files`; informational `not_an_item` files do not trigger the notice.
- @operator (2026-11-21): do not touch goinfer tags; the imported v0.4.0 tag is under owner review, and the next fork release awaits approval.

## Done

- [x] pbrain CLI/render finding re-check (@architect-2) `029ff48..f65bf43`
    * All eight findings resolved; replay and focused tests passed on CPUs 4–15 with `-p 2`; reviewer full-range verdict remains pending.

- [x] pbrain quoted-ID usage guidance in README and skill (@architect) `0bd5a12`
    * Core explains decoding quoted IDs; README explains `--json` and shell quoting; skill remains 400 words.

- [x] pbrain architecture/semantic documentation catch-up (@architect) `789b506`
    * Records first-index costs, admission/output rules, 41 scenarios, runtime facts and current test names.

- [x] pbrain measured-cost and path-safety spec updates (@architect) `4f936a7`

- [x] pbrain pre-release skill/README audit (@lead-dev-2) `6dee71e`

- [x] pbrain release pipeline (@lead-dev-2) `7cf8ced`

- [x] Goinfer fork documentation follow-up (@architect-2) `e33aa3893`

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
