TODO
====

## Ready

## In progress

- [ ] pbrain spec: functional spec (@architect)
    * description: gather the owner's round-1 answers, then draft the functional spec
    * criteria: record owner answers in `specs/projects/pbrain/qa_round1.md`; draft `specs/projects/pbrain/functional_spec.md`; have @architect-2 and @lead-dev review it each round with findings in `specs/projects/pbrain/reviews/functional_spec/roundN-<name>.md`; apply owner batches 1–4 across the whole spec, preserve front-matter YAML, verify all references and parse all YAML/TOML/JSON samples, and leave status as draft
        1. Remove `_by` suffixes: store `verified: <actor>` and `decided: <actor>`; use `--verified` and `--decided`; output `replacement` as a list for `replaced_by`; retain `status: provisional | verified`; verify writes status and actor; explain the `verified` to OKF `verified: [{by, at}]` conversion without other OKF changes.
        2. Replace root `.pbrain.yml` with tracked `<brain>/pbrain.toml` (same settings minus `path`); replace ignored `<brain>/personal.yml` with `personal.toml` (same path, author and audience rules in TOML). Discover by walking upward for `<dir>/pbrain.toml`, otherwise `<dir>/docs/brain/pbrain.toml`. `init` target precedence is `--path`, `--brain`, `PBRAIN_DIR`, then `docs/brain`; `init [--path <folder>]` defaults to `docs/brain`, creates the TOML and `.gitignore`, and explains `--brain`/`PBRAIN_DIR` for non-default paths. Define project root where needed, remove obsolete path-containment rules, and state that item front matter remains YAML.
        3. Brain and personal local data folders use `.pbrain/`, never tracked. Any file input inside a path resolving to a folder named `.pbrain` is refused, across brains and sections, including symbolic links, case variants and aliases; retain scenario 6 coverage for foreign and external-personal `.pbrain/` paths both directly and through symbolic links. Refuse any `.pbrain` folder as a file input, even if it is not pbrain's.
        4. No `verified_by`, `decided_by`, `--verified-by`, `--decided-by`, `replaced_by`, `.pbrain.yml`, `personal.yml`, or `--root` remains, except quoted OKF `superseded_by` or factory `superseded-by`; no `.local`, `PBRAIN-LOCAL` or `local_unmarked` remains; references resolve, all YAML/TOML/JSON examples parse, and status is draft.
        5. Owner batch 2 model (owner decided yes to A and B, 2026-11-20): shared/personal remain the only scopes; independently, content is copied or linked. A linked item leaves content at its source for the reader's own access. Shared linked titles, links, tags and summaries are repository-visible; personal linked items hide their existence. Use “linked” consistently; define near scopes/kinds (2.5/5) and reference from 8.4.
        6. V1 linked items remain kind `source`, origin `link`, with no new front-matter field. Add `pbrain import --link-only <url> --summary "<text>"`: optional one-paragraph plain-text body, maximum 300 characters; reject longer input and reject `--summary` without `--link-only`; a bare link without summary remains valid. Search and secret-scan the summary; never generate or fetch-derived-write it. Apply audience rules unchanged, including Slack D/G IDs never matching shared rules, and make notices clear that title, link and summary are shared. Linked imports store no original, but update any index like every write; originals are kept only for copied imports. `get` prints the summary and full linked locator exactly once in the linked line; JSON `source` carries the full locator. A source URL may be up to 2,000 characters; reject 2,001. Search labels linked items in text and JSON. Skill guidance: hand-write an identity-only summary, e.g. “pricing thread for the Q4 launch, Nov 2026” (what the source is, never what it says); no figures, quotes, private-party names or conclusions; DMs/private channels stay personal; readers fetch live content through their own access. Test summary-only search, get output, 301-character summary and invalid-flag rejection, 2,000/2,001-character locator boundaries, audience_required for an unruled linked Google Doc, and secret-scan warning on a token in summary.
        7. Section 17 “Likely later” sketches per-reader caching: `pbrain cache <id> <file>` saves fetched content in that section's `.pbrain/` with fetch time and digest, never in item files; `get --cached` labels “local copy, fetched <date>”; cache search requires an explicit flag, labels hits and reports linked items not cached here. Defer because content differs per reader, copies outlive revoked access, `.pbrain/` differs per checkout/worktree, and live fetch already works.
        8. Owner batch 3 (owner decision, 2026-11-20): rename `.local/` back to `.pbrain/` everywhere. Because `.local` is generic and `.pbrain` is distinctive, remove `PBRAIN-LOCAL` marker writing, `local_unmarked` warnings and deleted-marker cases. Refuse file input from any folder named `.pbrain` after path resolution, regardless of which brain it belongs to, including case variants and aliases; retain scenario 6 for another brain's and an external personal section's `.pbrain/` supplied directly or through a symbolic link. State in one clause that unrelated `.pbrain` folders are refused too.
        9. Owner batch 4 (owner decisions, 2026-11-20): add `pbrain delete <id> [--personal] [--yes]` consistently to command lists, flows, scenarios, skill and delivery order. Preview without `--yes` makes no changes and fails with a new confirmation error; show the file path, replaced items that become current, and links left dangling; `--yes` deletes and reports the same facts in text and JSON. Respect personal visibility (`--personal` required; shared view never reveals personal existence or links); preserve ordinary errors including ambiguous IDs (never delete both). Deleting copied sources also removes an unused original from that section's `.pbrain/`; indexes update; linked sources have no original. Run normal read-only Git exposure checks, but never stage, commit or rewrite history; explain that users must commit the deletion and that Git history still retains a committed file, while history rewrites for leaked secrets/private data are out of scope. Skill: replace wrong/outdated items with `--replaces`; delete duplicates, junk, secrets or private content in shared; agents delete only their own items unless asked by a person. Section 11 points dangling/moved-item guidance to delete; `check` still reports dangling links. Cover preview no-op, replacement lineage revival, original cleanup/retention, ambiguous ID refusal, and personal `not_found` without `--personal`.
        10. Add `--since <when>` and `--until <when>` to search and list consistently. Filter by `created` only, with other filters before count/limit; ranking within the window is unchanged and decision-in-force is computed within filtered results. Accept `YYYY-MM-DD`, RFC 3339, `today`, `yesterday`, and `Nh`/`Nd`/`Nw` spans from the recency evaluation instant. Local timezone for bare dates/today/yesterday is set by `TZ`; RFC 3339 is interpreted as written. `--since` includes its instant; document the chosen endpoint semantics, including that bare-date/today/yesterday `--until` includes the full day. Exclude unknown-created items when either flag is set; unparseable values and `--since` later than `--until` are usage errors. When filtered, report current matches excluded by the time window in the closing line and a matching JSON field (same for list). Cover recent-search older-match count, local-day-boundary list, inclusive bare-date `--until`, unknown dates, invalid values and inverted ranges.
    * work: batches 1 and 2 checks approved at SHA-256 `e296c95f9742f6a8f97059d88ce7a8c8e7c1ab563e9702b22e973fbb918cd0a0` and `360bd54958fcbafa5466e27e6630f4f4ae1da9abcf55e7ddc544db73e8f93352`; batch 3 owner decision applied and approved at SHA-256 `0928b6b0107d3ac676d8c4eeb9a49520d602a4968ed4c3677fb9288bc8b00a53`; rationale: `.local` is generic (roughly `~/.local`) while `.pbrain` is distinctive
        * batch 2 decisions: originals only for copied imports; linked import stores no original but updates indexes; `get` prints the full linked locator once and JSON `source` is complete; source URL limit is 2,000 characters
        * batch 3 checks reviewed `functional_spec.md` diff from `360bd549`; zero `.local`, `PBRAIN-LOCAL` or `local_unmarked` hits; all 21 section references resolve; all 11 YAML/TOML/JSON samples parse; status draft
        * @operator's owner-approved one-sentence edits say both people and agents feed the brain; `FACTORY.md` and `project_overview.md` carry the edits and will be committed with the spec after owner approval
        * batch 4 applied and approved by @architect-2 and @lead-dev at SHA-256 `5372ff086aaf29b2f5193ec17228985f097e0c90d3c5e32dab324d5b305bf6bd` (+479 words from `88a482fa`); targeted review reports show draft status, 23 references resolve, and all 12 YAML/TOML/JSON samples parse
        * batch 4 fix-pass decisions: missing links are `dangling_link` warnings only, never `replace_invalid`; for `verify` and `delete`, the flag chooses lookup view while the resolved item's own section supplies the lock and files; owner accepted reviewers' recommendations for the fix pass, including read-only exposure checks without Git mutations, safe same-section original cleanup, resolved dangling-link reporting, and outside-window counts even for empty results
    * verdict: batches 1–4 approve; functional spec remains draft, awaiting owner approval
    * next: owner approves or requests changes; after approval, commit the functional spec with the owner-approved `FACTORY.md` and `project_overview.md` edits

## Backlog

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
