TODO
====

## Ready

## In progress

- [ ] Goinfer upstream sync (@lead-dev-2)
    * owner: @lead-dev-2
    * branch: `pkieltyka/sync-upstream`, with merge `13851908` and parity refresh `2ba9f26`; worktree is clean
    * next: finish the serialized repository checks and pbrain Gemma parity via scratch replace; report the reviewed branch head

- [ ] pbrain phase 25: BGE gates and release wiring (@lead-dev-2)
    * owner: @lead-dev-2 for BGE; @lead-dev for gate tooling
    * range/status: tool fix `6779723..9695d1b` approved; registry follow-up `50816cd..e62c2b7` approved; README `e05ff80..2976b3a` pending review; BGE 0.65 rerun passed (29/30, 40/47); Gemma rerun pending
    * next: @operator may integrate the approved tool/follow-up; @reviewer reviews the BGE record, and @architect-2 reports Gemma results before model wiring

- [ ] pbrain phase 27: EmbeddingGemma 2 gates and release wiring (@architect-2)
    * owner: @architect-2 (patch flow)
    * branch/base: master `05f788e`; adapter merged `2ab7d56`, four-target audit `eb0441f`
    * next: after Phase 25 tool fixes, use Gemma rerun's cutoff; coordinate report with @lead-dev and patch registry/default resolution/`none`/skill text/tests, then request review

- [ ] pbrain phase 28: measurement before a release (@lead-dev)
    * owner: @lead-dev
    * ranges: harness prep `e62c2b7..ce26bef`, report `ce26bef..eedbe8a` on `peter/lead-dev`
    * next: @reviewer reviews after base measurement harness; full model timings, Mac runtime and tagged install remain

- [ ] pbrain init selector usable from caller's directory (@architect-2)
    * owner: @architect-2 (patch flow)
    * range: `db929d2..9ed8766` on `peter/init-selector`
    * next: @reviewer reviews after phase 18; @operator integrates on approval

- [ ] Aikit fork documentation follow-up (@architect-2)
    * owner: @architect-2
    * PR #4: `0b1b868..1f7bd74` (`pkieltyka/docs-pins`)
    * next: @reviewer reviews when there is a gap; @operator merges after green CI

- [ ] Goinfer fork documentation follow-up (@architect-2)
    * owner: @architect-2
    * PR #5: `a759890b..93adb19f3` (`pkieltyka/fork-docs-v0.2`)
    * next: @reviewer reviews when there is a gap; @operator merges after green CI

- [ ] pbrain phase 15: importer files and linked items (@architect-2)
    * owner: @architect-2
    * range: `f3dfbcb..b4a40a5` on `peter/architect-2`
    * next: @reviewer reviews after phase 18 and the queued phase 25 reviews

- [ ] pbrain phase 16: URL imports (@architect-2)
    * owner: @architect-2
    * range: `698dfab..57c7694` on `peter/architect-2`
    * next: @reviewer reviews after phase 15; scenario 20 remains the release gate

## Backlog

## Done

- [x] pbrain phase 18: calibration and `--bm25` gates (@lead-dev) `e5aa38a`
    * Approved and merged as `ad2da67`, `48912a6`, `e5aa38a`; both stages rerun after the snippet fix, with evidence reach updated and retrieval/gate counts unchanged.

- [x] pbrain skill guidance: first index on a machine (@architect) `05f788e`
    * Five-topic guidance and the 400-word core pointer are in the approved skill design; committed to master.

- [x] pbrain spec: EmbeddingGemma 2 default model (@architect) `7bdf4b2`
    * Missing key resolves to embeddinggemma-2; exact `none` disables; empty is invalid; BGE remains selectable; downloads still begin only at `pbrain index`.

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
