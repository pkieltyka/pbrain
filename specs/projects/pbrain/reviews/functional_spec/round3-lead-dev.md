# pbrain functional spec review · Round 3

Independent review of draft `3b526d45d8a3f1fb339741134d2940b5af462cecc7833ef98d3ca089bb0a30f7`. The append-only import, sole in-place verification edit, external exposure guard and placement of acceptance scenarios all follow the operator's decisions. [Draft](../../functional_spec.md) · [Owner decisions](../../qa_round1.md) · [Round 2 review](./round2-lead-dev.md)

## Round 2 disposition

| Finding | Status | Evidence in round 3 |
| --- | --- | --- |
| B1 · External personal worktree | **Resolved** | §§5.1, 5.3, 15.8 and 17 apply ignore and tracked-state checks in the project or an external containing worktree. A sub-worktree below a protected root is a separate case introduced by this contract. See R3-B1. |
| B2 · Shared projections and explicit inputs | **Resolved** | §§5.2, 5.4, 7.2, 9.1 and 15.6 require redacted unresolved links and refuse personal paths and aliases as shared file inputs. The new related output has a further projection rule to specify. See R3-S2. |
| B3 · Legacy lifecycle | **Partly resolved** | §§4.2–4.4, 7.2 and 15.11 put canonical links first, add stamp eligibility and share one graph with history. Discarding a stamp can still be read as changing the canonical result. See R3-S3. |
| B4 · Import transition | **Resolved** | §§4.2, 8.1, 8.4 and 15.19 use one new item, with duplicate exceptions only for the named predecessors. The merge fixture requires replacing all duplicate current links. |
| B5 · Public connection | **Resolved** | §§8.4 and 15.20 reject any non-public resolver answer before dial, reuse the checked address, verify the TLS host and require a direct request with proxy settings present. |
| S1 · Visible writes and adoption | **Resolved** | §§2.2, 5.1, 8.1, 9.2, 13 and 15.9/21 cover eligible shared destinations, a guarded in-worktree pointer and resumable init with an explicit config conflict outcome. |
| S2 · Budget modes and continuation | **Resolved** | §§6.1–6.3, 7.2–7.4 and 15.17 define mode units, empty/past-end handling, full-report exit status and continuation with all flags. Related extras need their own projection rule. See R3-S2. |
| S3 · Match class and time bands | **Resolved** | §§7.1, 11 and 15.3 define whole-clause ranking, UTC age intervals and unknown or future dates. The newly introduced decision promotion needs a clear archival boundary. See R3-S1. |
| S4 · Acknowledgement and concurrency | **Resolved** | §§8.1, 8.3, 9.3 and 15.14–15 distinguish visible commit from lost acknowledgement, limit serialization to pbrain writers and describe a scan instead of a global snapshot. |
| S5 · YAML editing and validation | **Partly resolved** | §§2.4, 8.3 and 15.16 separate the writer's format from legacy reads, preserve CRLF and reject invalid lists as a whole. The sole remaining in-place edit can still create an invalid duplicate field. See R3-S4. |
| S6 · Identity and tie-breaking | **Partly resolved** | §§2.3, 7.1, 7.3, 9.1 and 15.13 specify case-insensitive identity and path tie-breaks for search/list. Prime and findings still permit distinct records to tie at a paging boundary. See R3-S5. |
| S7 · Originals and inert output | **Resolved** | §§6.2, 8.2, 8.4 and 15.15 define bytes after both decoding steps, store originals before publication, disclose retention and scope the scan to shared item fields. Text/JSON rendering does not act on the source. |

## Ranking and related items

§7.1 and scenario 2 implement the owner's Postgres/Redis choice when both titles match the query. The old current decision moves to the front, the recent provisional note keeps its own ranked result, and the note appears beside the decision only with the explicit `relates` link and room in the rendered budget. §4.5 does not infer relationships or change standing, and §12 instructs agents to add the link when recording doubt. Scenario 3 also exercises the normal match-class and age-band order. The gaps are how archived results affect promotion and what personal-view related rows must carry. [Owner decision](../../qa_round1.md) · [Related items](../../functional_spec.md#45-related-items) · [Acceptance scenarios](../../functional_spec.md#15-acceptance-scenarios)

## Blocking (1)

### R3-B1 · Guard actual personal destinations that cross a nested Git boundary

**Sections** §§2.2, 5.1, 5.3, 8.1, 8.4, 15.8.

**Problem.** §5.3 checks a set of protected roots in their containing worktrees. An ordinary external personal root can contain a nested worktree under a kind directory such as `personal/notes/`. The root and its `.pbrain/` can both be outside Git, while the actual note destination is stageable in the nested worktree. In a temporary Git fixture, `git rev-parse --show-toplevel` failed at the external root, but Git listed the note under the nested `notes/` worktree as eligible for staging, then listed it as tracked after a normal add. A root-only implementation passes the guard and puts personal text into normal Git staging.

**Fix.** State that a managed personal subtree may not cross into a nested Git worktree, and fail closed when discovery or a write would cross one. Alternatively, apply the same ignore and tracked-file policy in the actual containing worktree of every managed personal path, including new kind destinations and raw-cache paths. Either option preserves the accepted external-folder guard. Add a fixture with an otherwise valid external root and a nested worktree at `notes/` or `sources/`, asserting no private item is published to an unprotected destination. [Exposure guard](../../functional_spec.md#53-the-exposure-guard)

## Should fix (5)

### R3-S1 · Define decision promotion over the eligible current partition

**Sections** §§7.1, 15.2–3.

**Problem.** “Best match class present” may include a replaced result with `--all`, while promotion can apply only to a decision in force. Example: a replaced item matches in its title, and a current decision and a newer current note both match only in body. Including the archive can either disable or preserve promotion among the same current hits. The owner's wording and the separate replaced-last rule point toward stable current ordering.

**Fix.** After view, query and explicit filters, partition candidates into current and replaced. Determine the strongest class and promote at most one decision within the current partition, using the normal tie-break key to choose it. Append the ordered replaced partition when requested. Extend scenario 2 with a strictly better-matching replaced item under `--all` and a strictly better-matching **current** non-decision in a separate case. The first case preserves the current ordering, and the second suppresses promotion. Put the budget between the two complete base results and those results plus the annotation in the tight-budget case. [Owner ranking decision](../../qa_round1.md)

### R3-S2 · Preserve view and identity on reverse related rows

**Sections** §§4.5, 4.6, 5.2, 6.1–6.2, 7.1–7.4 and 15.2/6.

**Problem.** §7.1 defines related rows with five fields and no `personal` marker, while §5.2 requires it for every personal item appearing in output. A personal note can link to a shared decision, so a shared parent can carry unmarked personal text in an explicitly personal query. A reverse lookup keyed only by the raw ID can also attach a backlink to the wrong item if the same stem exists in both sections or a target is invalid in its originating item's view. The new shared privacy fixture exercises outbound links only. The scope of extras across compact outputs is unclear because §4.5 says “where an item is shown” and list/prime borrow the search JSON shape.

**Fix.** Build related rows from valid, explicitly resolved edges scoped to the originating item's view, keyed by the resolved item. A related row inherits audience and review-date markers in both text and JSON, even when its parent is shared. Specify search and get as the outputs with extras, or give each other command an explicit policy. Keep optional extras out of base counts and continuations, test that they never displace a matching result or prevent a body page from advancing, and add an equal-date tie-break. Add a personal-to-shared backlink fixture: shared output and local shared data contain none of it, personal output marks the related row, and a same-ID alias cannot receive a backlink intended for a different item. [Related output](../../functional_spec.md#71-pbrain-search-query) · [View contract](../../functional_spec.md#52-two-views)

### R3-S3 · Derive status from surviving edges after a legacy stamp is rejected

**Sections** §§4.2–4.4, 7.2 and 15.11.

**Problem.** §4.3 says the note stays current whenever its stamp fails a condition. One of those conditions fails precisely when a valid canonical successor already replaces the note. In the mixed cycle fixture, when A's canonical edge replaces B and A's legacy stamp would make B replace A, discarding the stamp leaves B replaced. “Both items stay visible” can be built as both current unless visibility specifically means history or `--all`.

**Fix.** Say that a rejected stamp contributes no edge, and derive current/replaced state from the edges that survive. Keep canonical edges when removing cyclic legacy candidates. In scenario 11, assert that the stamped predecessor remains replaced by its canonical successor, and that the mixed cycle preserves the canonical edge, leaving one current item by default and both retrievable in history. State that cycle checks for legacy candidates consider the set together, so scan order cannot determine the winner. [Legacy lifecycle](../../functional_spec.md#43-factorys-older-stamp)

### R3-S4 · Define verification when the confirmation key already exists

**Sections** §§2.4, 3, 8.2–8.3, 9.1 and 15.16.

**Problem.** A hand-edited provisional file can already have a `verified_by` key. §8.3 promises to add the key's line, but doing that literally writes a duplicate key and makes the whole item unreadable under §2.4. The shared-write warning policy should also clearly reach the actor stored by this path.

**Fix.** For an editable existing key, replace its value in place. For a shape that cannot be edited without touching unrelated bytes, refuse with `unsupported_edit`. If absent, insert it once using the file's newline style. Explicitly apply warning-only secret detection to newly stored shared verification values and add a fixture with an existing key. [Verify contract](../../functional_spec.md#83-pbrain-verify-id)

### R3-S5 · Finish total ordering on paged outputs

**Sections** §§2.3, 6.1, 7.4, 9.1 and 15.17.

**Problem.** Prime ends its ordering at ID, even though two same-ID files can have identical kinds and timestamps, with different titles. Check ends at field, although two warnings in different lines of the same body tie. Both commands promise exact continuation on unchanged inputs. An implementation that takes scan or map order for ties can repeat or skip a row across process invocations.

**Fix.** Use the same stable section/path tie-break for prime as for search/list, or explicitly exclude ambiguous items from prime. For check, finish ordering by line/location and a stable occurrence key or coalesce identical findings deterministically. Cover stable ties at a one-result page size. [Continuation contract](../../functional_spec.md#61-budget)

## Verdict

**Changes required.** One blocking issue remains: the guard needs to cover a nested worktree at the actual personal write destination. The five should-fix findings address concrete output or state ambiguities. The main ranking and related-item design matches the owner's decision.

## Final check

Checked draft `77fe60da26aaafe5199a3c7ffa660ac4df48ab222b3a90346d8cef4e6dc2110b`.

- **R3-B1 · Resolved.** §§5.1–5.3 and scenarios 6–9 protect the configured and default personal roots and reject any `.git` entry before item access. The Git fixture confirmed that the rule catches a nested `.git` file and a root-level `.git` directory and permits a plain folder. I found no new blocker in this change.
- **R3-S1 · Resolved.** §7.1 and scenario 2 limit decision promotion to eligible current results and test that archives leave their order intact.
- **R3-S2 · Resolved.** §§4.5, 5.2, 7.1–7.2 and scenario 2 give related rows explicit links, audience markers, bounded output and separate ranked results. Shared links retain their originating-view rule.
- **R3-S3 · Resolved.** §§4.3–4.4 and scenario 11 preserve canonical edges, drop cyclic stamps together and derive lifecycle from surviving edges.
- **R3-S4 · Resolved.** §8.3 and scenario 16 update an existing `verified_by` key once, preserve the newline style and apply the warning scan.
- **R3-S5 · Resolved at the behavior level.** §§7.4 and 9.1 add path and line tie-breaks, and scenario 17 requires stable one-item paging on ties. Use a deterministic final tie-break or coalescing for distinct findings on the same line.

**Verdict — Approve.**

## Gated-sources delta check

Checked draft `d9dfc8c9` against the owner's gated-source and audience decisions. Keep the full HTTP(S) `--source` gate for `add` and file imports. The caller can supply `--shared` for an ordinary public citation. An unknown host with no saved rules can still be an internal or sign-in-protected source, so a known-host-only gate would silently share it. Keep the shared default for a direct fetch or bare link with no matching rule. `audience set`, `audience unset` and `audience` are a compact interface. Once the same canonical scope cannot occur twice, two distinct scopes of equal specificity cannot match one URL, so dropping the personal tie-break is sound. [Owner decisions](../../qa_round1.md#audience-rules-and-browser-permission-owner-adopted-2026-11-19) · [Audience rules](../../functional_spec.md#85-audience-rules)

### Blocking

- **G1 · Prevent shared rules from covering mixed visibility (§§8.5, 12, 15.24–25).** A rule for `acme.slack.com` would cover both public channels and DMs. The skill's personal instruction cannot stop `add` from applying that rule to a DM when no flag is supplied. Even on listed multi-tenant hosts, a check that only requires a nonempty path accepts scopes such as `docs.google.com/document` or `acme.slack.com/archives`. Specify the exact host families and accepted shared path shapes. Include Slack workspace hosts and require a channel-specific prefix such as `/archives/<channel-id>` for a user-approved shared channel. Keep host-wide personal rules. The Slack workflow must pass `--personal` when MCP identifies a DM or private channel and ask when visibility is unclear. Channel IDs alone do not establish the audience. Test broad-rule rejection, a public-channel match and a DM on the same host that cannot inherit it. [Audience rules](../../functional_spec.md#85-audience-rules)
- **G2 · Match and validate on the same canonical URL parts (§§8.5, 10, 15.24–25).** Exact hosts and whole segments leave parsing choices that can defeat the privacy rule. An implementation could allow a host-wide `docs.google.com.` rule because the built-in list checks only `docs.google.com`. Comparing escaped paths literally can also miss a narrower personal rule, leaving a broader shared rule in force. Specify one normalizer for saved rules, duplicate checking, restricted-host detection and source matching. Define case and trailing-dot handling for DNS names, the port policy, and path-escape handling. Refuse ambiguous separators and dot segments or require an explicit audience for the resulting citation. If normalization or validation fails, avoid using a broader shared rule. Apply the same validation to hand-edited local rules. Test equivalent host spellings, canonical duplicates and a path that would otherwise miss a personal exception. [Audience rules](../../functional_spec.md#85-audience-rules)

### Should fix

- **G3 · Apply the chosen section to the whole write (§§5.2, 8.1–8.5, 15.24–25).** §5.2 binds the personal view and write target to the flag. A rule-selected personal write has no flag. Make the resolved audience set the effective write view for guards, link lookup, duplicate and replacement checks, locks, originals and the index. When that section is unavailable, fail with `personal_unavailable`. Test a rule-selected personal file import with a shared duplicate and assert that there is no shared original or item.
- **G4 · Define the local settings mutation contract (§§5.3, 6.1–6.3, 8.5, 10).** Specify a canonical-key upsert for `set` and an idempotent `unset`. Validate and serialize the read-modify-write of `personal.yml` and publish the updated file atomically, preserving `path`, `author` and other settings. An overlapping edit that loses a narrower personal exception can expose the next import. Protect the file and temporary data before writing. Give the deliberate local listing a deterministic text and JSON shape and the same output bounds as other reads. Define invalid input and malformed-config outcomes.
- **G5 · Close the skill handoff cases (§§2.5, 8.4, 12, 15.22–25).** Chrome's connection prompt approves the browser session. Say what the agent does when its browser tool lacks a remembered site grant, without storing browser consent in pbrain. The artifact fragment is a provenance label and all companion files share the main URL's rule because matching ignores fragments. Apply an explicit audience per file when it differs. Keep Slack as ordinary items with permalinks and remove the remaining “Slack exports” example from the source kind.

**Verdict — Changes required.** G1 and G2 can put gated content in the shared tree without an explicit shared flag.

### Re-check

Checked draft `b88f5244`.

- **G1 · Resolved.** §8.5 defines allowed shared path shapes and rejects broad scopes, including a Slack workspace or its `/archives` root. Slack shared rules are scoped to one `C` channel, `D` and `G` links cannot inherit a shared rule, and §12 directs agents to use `--personal` for a private channel or ask when visibility is unclear. Scenarios 24 and 25 cover broad-rule rejection and a same-workspace DM for both `add` and `--link-only`.
- **G2 · Resolved.** §8.5 uses one canonical form for rule validation, upserts and source matching. Ambiguous paths require a flag. Scenario 24 covers host aliases, encoded letters, dot segments and canonical upserts. I read the mixed-audience table as matching the canonical DNS hostname regardless of port. The nondefault port remains part of the exact rule key.
- **No new blocker in the fix.** §5.2 selects the destination before item or source access. Scenario 26 checks that a rule-selected personal import leaves the shared item and original cache untouched.

**Verdict — Approve.**

## Owner batch 1 check

Reviewed only the diff from `a125b385` to `e3e0dc82c6a62b0419bee894623ec474406f7999447bd2052a8e4f2a154535c3` (+232 words). Status remains draft. The forbidden-name scan was clean, all 19 section references resolve, and the front matter YAML, three TOML blocks, one fenced JSON sample and six inline JSON samples parse. The actor fields and flags, YAML/TOML boundary, discovery precedence, init files, and OKF actor conversion match the requested migration.

### Blocking

- **OB1 · Specify the replacement output contract (§§4.4, 7.1–7.2, 15).** The owner describes `replacement` as one ID. §7.1 calls it multiple IDs and §7.2 lists the JSON key without its type. The search text format specifies a `replaced` marker and leaves the requested `replacement` output undefined. Decide the fork shape explicitly. One option is a scalar for a single direct successor, with the field omitted on a fork and all branches still visible through current successors and history. If the owner prefers an array, use the same array type for one or several direct successors. Define the bounded text representation for `search --all` and `get`, and add an ordinary replacement and fork example.
- **OB2 · Preserve the original-cache input guard (§§5.1–5.3, 8.4, 15.6).** Before the rename, §5.2 refused shared writes from file inputs under every cache directory of the old name. §5.1 now limits protection to the two sections of the selected brain. For example, with brain B selected, `add --title 'Imported note' --body-file /other/brain/.local/<digest> --shared` can copy a UTF-8 original from brain A because it lies outside B's protected roots. That original can hold material stripped during conversion. Keep the explicit-file check for cache directories from other brains, including resolved aliases, and distinguish it from the exposure scan of configured roots. Preserve the old denial on the renamed path or have the owner decide a narrower policy for the more common directory name. Add a cross-brain file-input example that asserts shared output and caches stay empty.

**Verdict — Changes required.**

## Owner batch 1 re-check

Checked draft `fe68106472f6439decc21d666a292723e4982d960c9d1d5fa7f035d0f0bb2e4c` (+245 words from `e3e0dc82`). The retired-name, reference and sample checks still pass.

- **OB1 · Resolved.** §§7.1–7.2 and scenarios 4 and 12 define ordered, comma-separated direct replacements in text and arrays in JSON for one successor or a fork. `current` remains separate.
- **OB2 · Partly resolved and still blocking.** §5.2 and scenario 6 now cover another brain's `.local/` and its default `personal/.local/`. They miss another brain's configured personal section. Example: A uses `personal.toml` with `path = "/private/notes"`; with B selected, `add --body-file /private/notes/.local/<digest> --shared` can still publish A's personal original. The operator's fix covers every brain's personal section. Require an identifiable marker in pbrain-managed `.local/` before storing originals, or conservatively refuse `.local` inputs for shared writes if the broader effect is approved. Extend scenario 6 with a foreign external personal section and a resolved alias.
- **Item 4 · Resolved.** §§9.2 and 10 put `--path` first, followed by `--brain`, `PBRAIN_DIR` and `docs/brain`. An explicit discovery path is relative to the working directory, while an init `--path` is relative to the project root. Scenario 27 covers the precedence.

**Verdict — Changes required.** The cross-brain guard still leaves configured personal caches exposed to shared file-input writes.

### External cache marker re-check

**OB2 · Resolved.** In draft `e296c95f9742f6a8f97059d88ce7a8c8e7c1ab563e9702b22e973fbb918cd0a0` (+157 words from `fe681064`, including the +31-word amendment), §§5.2 and 9.3 refuse shared file inputs from marked cache folders across brains after path resolution. Scenario 6 covers an external personal cache and its symlink. An unrelated `.local/` remains eligible if it has no marker and matches no existing protected path.

§§9.1 and 9.3 make `check` report `local_unmarked` only in the selected view, without repair. A later cache write restores the marker before storing anything. Deleting the marker on a foreign external cache leaves it undiscoverable until restored. The configured brain still protects its own cache, and the existing recognisable layouts remain guarded. The retired-name, section-reference and sample checks pass.

**Verdict — Approve.**

## Owner batch 2 check

Reviewed only `e296c95f` to `7ad42067639e2cb388c337c8770e13be912882841fff5a002dac75b864809186` (+689 words). Draft status, all 22 section references and all 11 YAML, TOML and JSON samples passed. The two audiences, summary flags and limits, search and JSON markers, audience rules, later-cache sketch, and corrected Q4 example match the owner decisions.

- **Blocking · Contradictory import disclosure (§8.4, scenario 28).** The destination notice still says every import keeps a copy of the original. The linked-item contract keeps the content at the source, and only file and URL imports write an original. Make the cache statement conditional on copied imports, keep the shared metadata disclosure on linked imports, and assert that a linked import makes no network request and stores no original.
- **Should fix · Keep the standard header on a link without a summary (§7.2, scenario 28).** The scenario says `get` prints the line alone, which can be read as removing the header. Say that the normal header and fixed source line remain when the summary is absent. The stored body is empty.

**Verdict — Changes required.** The linked-import notice needs to agree with where the content is stored.

## Owner batch 2 re-check

Checked `cb80a37389f421d1d97fa7095132839edb04d5330b7f8aa4ee8d9b15d343f3b0` (+171 words from `7ad42067`). Draft status, section references, and sample parsing pass.

- **Item 1 · Originals and notice resolved.** §§8.1 and 8.4 save and verify originals only for copied imports, and the notice tells linked users about metadata visibility without mentioning an original. Scenario 28 requires no network request, no original, and no change to `.local/`.
- **Item 1 · Remaining blocker in the write order.** §8.1 ends by updating any index, including for a linked import, while §5.1 puts an on-disk index in `.local/` and §8.4 and scenario 28 require that directory to stay unchanged. Exempt linked imports from the on-write index update and require the next read to see the item if an index exists.
- **Item 2 · Resolved.** §7.2 and scenario 28 preserve the normal `get` header and the fixed source line when the summary is absent.

**Verdict — Changes required.** The linked write order still conflicts with the unchanged `.local/` requirement when an on-disk index exists.

### Linked original and index re-check

**Resolved.** Checked `360bd54958fcbafa5466e27e6630f4f4ae1da9abcf55e7ddc544db73e8f93352` (+10 words). §8.4 says a linked import stores no original and updates an index if one exists. §8.1 is unchanged. Scenario 28 requires immediate search visibility and no original or network request. The diff is limited to those two locations.

**Verdict — Approve.**

## Owner batch 3 check

Reviewed only `360bd549` to `94c242ac32eeb972dd7bfbe595598271e5bf750652c389d9fa5b69d882b56b81` (−127 words). The draft has no `.local`, `PBRAIN-LOCAL` or `local_unmarked` remaining. All 21 section references resolve and all 11 YAML, TOML and JSON samples parse. The layout, ignore rules, store paths and scenarios use `.pbrain/`. Scenario 6 retains direct and symlink inputs to foreign and external personal caches. The name rule also covers folders unrelated to pbrain.

- **Blocking · Preserve equivalent-case aliases in the name check (§5.2, scenario 6).** The new refusal depends on the ancestor's name, while the preceding resolution clause mentions only links. On a case-insensitive volume, `.PBRAIN` may refer to the same `.pbrain` directory while symlink resolution retains the caller's spelling. A literal segment comparison can then miss the cache. Explicitly require the name check to recognise equivalent case spellings on the underlying filesystem, including after symlink resolution. Add one case-alias check alongside the direct and symlink inputs on case-insensitive filesystems.

**Verdict — Changes required.** The rename and marker removal are complete. The name-based guard needs an explicit case-alias rule.

### Owner batch 3 re-check

**Resolved.** Draft `0928b6b0107d3ac676d8c4eeb9a49520d602a4968ed4c3677fb9288bc8b00a53` (+22 words) changes only §5.2 and scenario 6. The case-insensitive folder-name check follows path resolution on every filesystem. Scenario 6 requires a shared file input under `.PBRAIN` to fail as if the file were missing.

**Verdict — Approve.**

## Owner batch 4 check

Reviewed `0928b6b0` to `88a482fa96840ac81166d202cc7c315ad7b88211017d4bcd6f4b97681c076fb8` (+980 words), including the warning amendment. Draft status, 23 section references, balanced fences and all 12 YAML, TOML and JSON samples passed. The confirmation stream and exit code, personal lookup, time-filter ranking, and outside-window count are in place.

### Blocking

- **B4-1 · Make delete's Git contract compatible with the exposure guard (§§5.3, 8.6, 10).** §8.6 says pbrain never runs Git. §5.3 requires every command opening a brain to verify ignored and tracked paths and fail when Git is required but unavailable. Keep those read-only Git checks for preview and deletion, and say that deletion does not stage, commit or rewrite history. If the no-Git sentence forbids even read-only calls, the spec needs an alternative way to enforce the guard.
- **B4-2 · Confine original cleanup and report its actual outcome (§§2.4, 8.1, 8.4, 8.6, 15.29).** Delete is the first command that removes a cache object based on a hand-editable `sha256`. Before using that value as a name, require a valid digest basename and a target confined to the selected section's `.pbrain/`. Serialize the item removal and remaining-reference check with imports in that section. Unlink the item before attempting cleanup, so an item unlink failure leaves its original intact. If cleanup fails after deletion, keep the accepted success plus notice, and report that the original remains instead of `original: removed`. Cover malformed metadata and cleanup failure in scenario 29.
- **B4-3 · Use resolved links for the delete impact and classify missing targets once (§§4.2–4.4, 5.2, 8.6, 9.1, 15.29).** The preview currently lists anything that names the id, which includes links already dangling. For example, a shared note that names a personal id is already unresolved in the shared note's view, even for a personal-view delete. Compare the resolved graph before and after deletion using each link's originating view. The owner downgraded `dangling_link`, but a missing `replaces` endpoint still breaks §4.2 and can also match the `replace_invalid` error in §9.1. Make a missing target a `dangling_link` warning only. Extend scenario 29 to cover both `replaces` and `relates` becoming dangling and to assert successful `check` when those are the only findings.

### Should fix

- **B4-4 · Define time-window behaviour at the date and output edges (§§6.1, 7.1, 7.3, 15.30).** Validate inversion against effective boundaries. With `--until 2026-11-19`, a late timestamp on November 19 is valid, while `--since 2026-11-20` is inverted. Implement a whole local day ending before the next local midnight, including on a daylight-saving transition. Say that the closing count still appears when a window excludes every current match, and name `--until` in the hint for an until-only window. Remove “the only date an item has” since `fetched` and `stale_after` exist.
- **B4-5 · Show the available flags in the core skill (§12).** The time example names only `--since`, and `pbrain delete` by itself can only preview. In the existing bullets, name `--until` and the preview-to-`--yes` flow without exceeding the 400-word cap.

**Verdict — Changes required.** Reconcile the Git guard, safe cache cleanup, and the classification and reporting of dangling links before implementation.

### Owner batch 4 re-check

Checked `5372ff086aaf29b2f5193ec17228985f097e0c90d3c5e32dab324d5b305bf6bd` (+479 words from `88a482fa`). Draft status, section references and all 12 samples pass.

- **B4-1 · Resolved.** §8.6 keeps the read-only Git exposure check for both preview and deletion. Scenario 29 covers refusal without mutation.
- **B4-2 · Resolved.** §8.6 validates and confines the digest path, removes the item first, and serializes cleanup with same-section imports. Scenario 29 covers keeping a referenced original, the correct section with `--personal`, and `original: left` when cleanup cannot proceed.
- **B4-3 · Resolved.** §8.6 limits the preview to newly dangling links in the originating view. §§4.2 and 9.1 classify a missing target only as a warning. Scenario 29 covers both link fields and successful `check` with only dangling warnings.
- **B4-4 · Resolved.** §7.1 and scenario 30 cover local-day boundaries, an inverted range, counts on empty output, and the until-only hint.
- **B4-5 · Resolved.** §12 names both time flags and the `delete` preview followed by `--yes`. Its core bullet text is 285 words.
- **Verify amendment · Resolved.** §5.2 separates the lookup view from the resolved item's section for `verify` and `delete`. Scenario 16 checks shared locking and file access for `verify --personal` on a shared item.

**Verdict — Approve.**

## Owner batch 5 check

Reviewed `5372ff08` to `b8c7d75c3bb3e494b4e48b424e5288c57d16f6490c20a678408ad31171fa8bbe` (+961 words). Draft status, 25 section references, balanced fences and all 13 YAML, TOML and JSON samples pass. The advisory vocabulary, pilot ranking, factory upkeep, and the semantic research boundary match the decisions.

### Blocking

- **B5-1 · Keep zero-match diagnostics within the output budget (§§6.1–6.2, 7.1, 15.31).** Capping the number of clauses at eight leaves each echoed clause unbounded. A valid quoted phrase can alone exceed the default output budget. Keep the ordinary closing counts and envelope as fixed output, and add at most eight complete diagnostic entries only as they fit. An empty search whose ordinary fixed part fits must still succeed. Test a long clause and a tight budget in text and JSON, including a case where an outside-window count must remain visible.
- **B5-2 · Specify visibility of discovered tag names (§§5.2, 7.5, 15.32).** Counts are scoped to the view, but the spec says undeclared tags “in use” are listed and that `--personal` adds personal items to counts. A build could discover an undeclared tag from a personal item, then print its name with a zero count in the shared view. Derive undeclared names, pagination and totals only from current items in the active view, including when no vocabulary exists. Test that a tag found only on a personal item does not appear or affect counts or pagination in shared mode.

### Should fix

- **B5-3 · Define the passage and its offset (§§6.2, 7.1–7.2, 15.31).** The agreed best stretch can cross a Markdown line break. Select it from the stored body and flatten it for one-line display. Specify the zero-based Unicode code-point offset into the stored body before flattening or adding ellipses, including an actual zero offset on long hits. Test a wrapped paragraph and a multibyte character before the hit to ensure `get --offset` opens at the same passage.
- **B5-4 · Keep the skill fallback deliberate (§§12, 14).** The new core instruction still says to use `--all` automatically after a retry, while the new complete flow stops and reports no answer. Use history only when the task calls for it. The core outline's bullets now total 421 words before the five kind examples; compress the eventual skill to the existing 400-word cap.

**Verdict — Changes required.** Bound the optional diagnostics and make the tag inventory's private-name boundary explicit.

### Owner batch 5 re-check

Checked `3461303e103b1606cdcac8d1786bb3bfd3c75e864961224411f94370033dd02b` (+326 words). Draft status, 26 section references and all 13 samples pass.

- **B5-1 · Resolved.** §7.1 reserves the ordinary footer first, cuts displayed clauses at 40 characters, and stops adding optional entries at the first that will not fit. Scenario 31 covers long clauses, the entry limit and a retained outside-window count.
- **B5-2 · Partly resolved and still blocking.** §7.5 now builds discovered names from visible current items. It also says both that every declared tag appears and that a tag used only on personal items is absent. If an existing public declaration disappears when a personal item uses it, shared readers can infer private use. Keep every declared tag visible with its shared count, including zero. Restrict the private-only omission to discovered, undeclared names. Test both a declared tag that stays at zero and an undeclared tag that remains absent, with the same shared output before and after personal use.
- **B5-3 · Resolved.** The stored-body code-point position drives `get --offset`. Scenario 31 covers wrapping, a multibyte character and offset zero.
- **B5-4 · Resolved.** §§12 and 14 use one retry and report the searches' result. The outline may be longer than the shipped text, whose cap remains enforced by scenario 23. Keep the tag naming guidance from §10 when drafting that text.

**Verdict — Changes required.** Make the shared inventory invariant under personal use of an already declared tag.

#### Declared tag visibility re-check

**B5-2 · Resolved.** Checked `41605322ceac5a7edf841528e21bf5940f0a9ca31b6c388751d08d7469bf90d1` (+43 words). §7.5 retains every public declaration at its count in the active view, including zero. Scenario 32 keeps the shared listing identical before and after personal use and excludes a private-only undeclared name from shared output and pagination.

**Verdict — Approve.**
