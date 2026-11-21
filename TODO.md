TODO
====

## Ready

## In progress

- [ ] pbrain phase 28: measurement before a release (@lead-dev)
    * owner: @lead-dev
    * range/status: Linux Gemma full-size index active at 0.895 passages/s, ~15 h projected, 1.89 GiB peak so far; BGE full-size run queued after Gemma; BGE is now the default
    * next: finish Gemma then BGE Linux runs; update README tables/opening to remove Mac timing claims; then final read-only pass

- [ ] pbrain spec: revise decision 27 for BGE default and measured cost (@architect)
    * owner: @architect
    * range/status: assigned on master `20d8626`; Mac-owed spec delta is integrated
    * next: update functional spec 9.4/10/16, architecture 8.7/16/17, semantic/brain/skill specs and README with Gemma opt-in, both models' figures, and goinfer v0.4.1

- [ ] pbrain BGE-default release wiring (@architect-2)
    * owner: @architect-2
    * range/status: assigned after the revised-default spec lands; implementation on master
    * next: set `brain.DefaultModel` to BGE and update default-reading tests, CLI status and notice expectations

- [ ] pbrain goinfer v0.4.1 dependency verification (@architect-2)
    * owner: @architect-2
    * range/status: blocked; signed v0.4.1 points to `e33aa3893`, but proxy currently returns a cached not-found; pbrain remains on v0.2.0
    * next: after cache expiry, verify proxy Origin.Hash and fork module path; then bump and run Gemma pinned-file tests

- [ ] pbrain executable scenarios 21–30 and 35 (@lead-dev)
    * owner: @lead-dev
    * range/status: worktree `peter/lead-dev` at `93480a6`, which includes master `35966e7`; scenarios 21–29 are drafted
    * next: keep scenarios 21–30 and 35; coordinate helper ownership with @lead-dev-2; `-p 2`, CPUs 4–15, embedtest only

- [ ] pbrain executable scenarios 31–34 and 36–41 (@lead-dev-2)
    * owner: @lead-dev-2
    * range/status: based on master `35966e7`; starting assigned scenario set
    * next: coordinate shared helper ownership with @lead-dev; `-p 2`, CPUs 4–15, embedtest only

- [ ] pbrain final functional-scenario audit (@reviewer)
    * owner: @reviewer
    * range/status: scenarios 1–20 and output range `0bd5a12..965ed58` approved; semantic alignment, scenarios and PTY checks pass
    * next: review both submitted scenario sets: 21–30 and 35; 31–34 and 36–41

## Backlog

- [ ] Scope repeatable `--tag` help text per command (filter vs assign)

- [ ] Show search defaults (`--limit 5`, `--max-chars 3000`) in command help

- [ ] Hide unused global `--brain` and `--personal` options from `pbrain skill --help`

- [ ] macOS (arm64) runtime measurement and smoke: rule 3 for both models, search `--semantic` at 5,000 items, release-binary install from tag

## Decisions

- @operator (2026-11-21): revise decision 27: `bge-small-en-v1.5` is default; `embeddinggemma-2` remains opt-in and `none` turns semantic search off. The measured Gemma cost on the token-heavy 5,000-item/50 MB fixture informs the change; finish the same-size BGE run before release.
- @operator (2026-11-21): macOS arm64 runtime measurements and tagged-install smoke are owed in a separate backlog item and do not block the first release.
- @operator (2026-11-21): exclude item stems with invalid UTF-8 and report them through skipped-file/check diagnostics, preventing distinct raw names from folding to one displayed ID.
- @operator (2026-11-21): use `ExactValue` for control/quote/backslash IDs and paths (spaces remain bare); reject invalid-UTF-8 lookup IDs; use `bad_file_name`; map invalid-name findings to the nearest printable ancestor or `?`; omit invalid-UTF-8 JSON paths.
- @operator (2026-11-21): count only error-severity skipped files in `skipped_files`; informational `not_an_item` files do not trigger the notice.
- @operator (2026-11-21): goinfer v0.4.0 is unusable because the proxy serves upstream content; use signed fork tag v0.4.1 only after the proxy verifies `e33aa3893` and the `github.com/pkieltyka/goinfer` module path.

## Done

- [x] pbrain final skill/README audit (@lead-dev-2) `0bd5a12`

- [x] pbrain testscript harness and scenarios 1–20 (@architect-2) `502db3c`

- [x] pbrain release output fixes and scenario coverage (@lead-dev-2) `73af597`
    * Reviewer and architect approved `0bd5a12..965ed58`; semantic alignment, all 20 scripts, and PTY checks passed.

- [x] pbrain CLI/render finding re-check (@architect-2) `029ff48..f65bf43`

- [x] pbrain quoted-ID usage guidance in README and skill (@architect) `0bd5a12`

- [x] pbrain architecture/semantic documentation catch-up (@architect) `789b506`

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
