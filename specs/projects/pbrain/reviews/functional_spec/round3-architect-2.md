# Functional spec review, round 3

Reviewed draft `3b526d45d8a3f1fb339741134d2940b5af462cecc7833ef98d3ca089bb0a30f7`. The append-only import, single in-place edit and exposure guard across repositories are reflected in the command contracts and fixtures.

## Round-two dispositions

| Finding | Disposition |
|---|---|
| 1. Shared writes reading personal files | **Partly resolved.** §5.2 and scenarios 6–7 protect the configured personal section, resolved symlinks and caches. A reserved local path can still be copied when another personal section is configured (finding 1 below). |
| 2. External personal originals in Git | **Resolved.** §5.3 and scenario 8 apply both the ignore and untracked checks in the repository that contains each protected path. |
| 3. Publishing before caching the original | **Resolved.** §§8.1, 8.4 and scenario 15 put digest-verified raw storage before item publication. Only the derived index can fail afterward. |
| 4. Read and write eligibility | **Resolved.** §§2.2, 8.1 and scenario 9 prune ignore-matched folders even with tracked descendants and reject a shared destination readers would skip. |
| 5. Factory/qmd adoption | **Resolved.** §§5.1, 5.3–5.4 and 13 allow an ignored in-project personal folder and direct external readers to explicitly shared folders. |
| 6. Legacy edges and advancing forks | **Partly resolved.** §§4.3–4.4 and scenario 12 define modern precedence and preserve fork warnings through an advancing branch. Scenario 11 still gives a conflicting result for a mixed cycle (finding 3). |
| 7. Empty pages and budgets | **Resolved.** §6.1 and scenario 17 cover the mandatory empty envelope, minimum-limit error, past-end offsets and continuing with the same flags at a fixed time. |
| 8. Ranking | **Partly resolved.** §§2.4, 7.1 and scenarios 2–3 apply clause matching, the owner’s decision exception and exact age bands. The unknown-date fixture still overstates what the sort guarantees (finding 4). |
| 9. Cooperating writers and scan consistency | **Resolved.** §§8.1 and 9.3 scope serialization to the destination section, define scan-time reads and acknowledge the hand-edit race and lost acknowledgement. |
| 10. Multiple candidates for URL promotion | **Resolved.** §8.4 and scenario 19 require a new item. Each same-URL duplicate must be named in `--replaces`; no existing source is silently selected for editing. |
| 11. URL release gate | **Resolved.** §8.4 and scenario 20 agree on direct connections, checking every resolved address, proxy behavior and the decoded original used for the digest. |
| 12. CRLF edits | **Resolved.** §8.3 and scenario 16 explicitly accept and preserve the file’s newline style. |

## Blocking

1. **A shared input can still publish reserved personal files (§§5.2–5.3, 8.2, 8.4, 10).** Configure an external personal folder and leave a raw `confidential.txt` under the default `<brain>/personal/`. The raw file has no front matter, so the “also holds items” configuration check passes. The exposure guard correctly requires the default folder to stay ignored. Yet a shared `import` of that file passes the input rule: it is outside the configured personal section and every `.pbrain/`. The result is a shared transcript and a shared original. Independently, `add --body-file <brain>/personal.yml` can publish the ignored marker’s contents, including the external location. Apply the same reserved-path set to explicit shared file inputs as to the exposure guard, after path resolution. Test both cases and require the ordinary missing-file response with no shared item or cached original.

## Should-fix

2. **The job-queue order is correct; the related-item output only covers one side of the owner’s decision (§§3, 4.5, 5.2, 7.1–7.2, 12; scenario 2).** Both titles match every clause of `job queue`. Moving the best decision in force to the front gives Postgres, then Redis, with both entries retained when only the annotation must be dropped. The reverse lookup correctly shows the Redis doubt under Postgres. But a search for `Redis` alone returns the doubt without its explicitly linked decision, even with ample space. The owner asked to show the result’s linked item as well. Render explicit outgoing targets alongside the drafted incoming newer items, deduplicate, and reserve space for primary entries first. Include effective **standing** and the **personal** marker on each annotation in both formats. The current nested schema cannot distinguish a decision with a decider from a legacy decision of provisional standing, or mark a personal doubt attached to a shared decision in the personal view. Extend scenario 2 to the Redis-only query and a personal doubt. Teach the skill that `relates` can also mean supporting evidence; the link alone does not establish an open disagreement.

3. **Ignoring a legacy stamp cannot undo a valid modern edge (§§4.3–4.4; scenario 11).** Let B replace A, then give B an old stamp naming A as its successor. Ignoring the cycle-closing stamp leaves B current and A replaced. The fixture says both stay visible, which would be wrong in default search. Likewise, §4.3 says the stamped note stays current even when its stamp was ignored because a valid modern successor already replaced it. Derive currentness from the surviving edges and assert the exact default search and history results. For legacy `<file>` values, specify removal of the directory and optional `.md` suffix before comparing to an id; a filename’s basename still includes its extension.

4. **Scenario 3 promises an absolute last place for an unknown date (§7.1; scenario 3).** A verified learning of unknown date and a two-year-old provisional note in the same match class both reach the oldest band. Standing puts the verified learning first. Keep the owner’s ordered keys and narrow the fixture: unknown sorts after a dated item when class, band and standing tie. Both cases should display `unknown` for the undated item.

5. **Verification can turn a readable item into invalid YAML (§§2.4, 8.3).** A hand-edited provisional file may already contain the optional `verified_by` key. The specified edit adds a verifier line; adding a second key makes subsequent reads skip the item. Set the existing field when it has a supported representation, insert it only when absent, and return `unsupported_edit` without changing either field otherwise. Test verification of an existing provisional file with a prior verifier and require exactly one verifier key afterward.

**Verdict — changes required.** One blocking privacy gap and four should-fix findings. The primary ranking order and append-only import decision can proceed unchanged.

## Final check

**Blocker resolved** in draft `77fe60da`. §§5.1–5.3 and scenario 6 protect both the default `personal/` folder and `personal.yml` when another personal folder is configured. Shared file inputs resolve symlinks before the check and return the ordinary missing-file response without storing an original. The exposure guard rejects a `.git` entry at the root of a protected folder or at any depth below it. Scenarios 6–9 agree with the guard. No new blocker found in this scope.

- **2. Resolved.** §§4.5, 7.1 and 12 and scenario 2 cover both link directions, effective standing, personal markers and supporting evidence.
- **3. Resolved.** §4.3 and scenario 11 preserve the valid modern edge when a legacy stamp would close a cycle.
- **4. Resolved.** §7.1 and scenario 3 place an unknown date after a known one when match class, band and standing tie.
- **5. Resolved.** §8.3 and scenario 16 update an existing verifier in place and leave exactly one `verified_by` key.

**Verdict — approve.**

## Gated-sources delta check

Reviewed draft `d9dfc8c9` against the two adopted gated-source decisions. **Keep the broad gate on externally supplied URL sources.** With no rule or flag, a public GitHub issue and a private issue on the same host can look identical to `add`. Requiring `--shared` in the public case is an acceptable cost. The proposed narrower gate would silently share the first authenticated capture from an unfamiliar host. The browser permission remains correctly owned by the browser tool.

1. **Blocking — a Slack workspace rule can publish a DM (§§8.5, 12, scenario 25).** The draft permits `audience set acme.slack.com shared`. A later `add --source <DM permalink>` without flags then writes the DM-derived text to the repository. The skill says private content goes to personal, but the command’s remembered answer says shared. Treat Slack workspace hosts as mixed-audience hosts: refuse workspace-wide shared rules and generic prefixes such as `/archives`, including rules loaded from hand-edited configuration. Allow a shared scope for one identified public channel after the person authorizes it. The Slack reader must use the conversation visibility supplied by the MCP on each capture; explicitly pass `--personal` for DMs and private channels even when a rule already exists. If visibility cannot be established, do not apply a remembered shared answer automatically. Test a public channel, a DM and a private channel on one host, including a previously public channel now reported private.

2. **Blocking — an implicit personal destination has two incompatible write contracts (§§5.2, 8.1–8.5, scenarios 7 and 25).** §5.2 chooses the shared lookup and write view solely from the absence of `--personal`; §8.5 can route precisely that invocation to personal. For example, a repeated import can check duplicates in shared, cache the original in shared, or print an existing personal id under the ordinary shared diagnostic rules. Define one destination decision before validation, acquisition, duplicate lookup and caching. All writes and derived data use that section. State the narrow exception for a rule-routed personal write: it can acknowledge the newly written item as personal; keep pre-existing personal ids and paths out of implicit duplicate/link diagnostics unless the caller explicitly requests personal access. Shared reads keep their existing boundary. Extend scenario 25 with a personal file import, a duplicate retry, both output streams and a check of the shared originals cache.

3. **Should-fix — validate the actual rule scope and preserve exceptions (§§8.5, 10).** Keep `audience set`, `unset` and the bare listing command. Define one canonical representation for host and whole path segments for setting, loading, unsetting and matching rules. Validate hand-edited rules identically. Enumerate the supported mixed-host path forms: a provider-wide family prefix or a path whose document identity lives only in the ignored query cannot authorize one document. Two equal-length segment prefixes matching one canonical URL must be identical, so removing the tie fallback is sound **after** duplicate canonical scopes are rejected on load. An explicitly authorized `set` may update that one scope. Serialize atomic rule-file edits and preserve `path`, `author` and other rules so a concurrent set cannot drop a narrower personal exception. Treat listing as explicit access to private configuration and apply the existing read budget to it.

4. **Should-fix — a bare gated URL can itself be private (§§8.4–8.5, 12).** `--link-only` still stores a full locator and title. A DM permalink or access-bearing artifact URL can therefore reach Git without any audience choice. For a bare link obtained through an authenticated reader, apply the same saved audience or ask for an explicit one before writing; pass `--personal` for private Slack conversations. At minimum require an audience on recognized mixed-audience hosts in link-only mode, where no fetch establishes public visibility. Cover a no-rule DM link in the fixture. The shared fallback for a page pbrain actually fetched remains intact.

**Verdict — changes required.** Two blocking and two should-fix findings.

### Re-check

Draft `b88f5244`. Both blockers are resolved.

- **Slack sharing (§§8.5, 12; scenarios 24–25).** Workspace-wide and generic `/archives` shared rules are rejected on entry and on load. A shared rule is scoped to a specific C channel. D/G links cannot match it. For private C channels the agent must use `--personal`; when visibility is unknown it must use `--personal` or ask.
- **Write destination (§§5.2, 8.5; scenario 26).** A gated write with no audience fails before acquisition. Once selected, one destination governs lookup, locking, originals and publication. A rule-routed personal import leaves the shared cache untouched, and a duplicate retry withholds the existing personal id.

**Verdict — approve.** No new blocker found within these two paths.

## Owner batch 1 check

Compared `a125b385` with `e3e0dc82c6a62b0419bee894623ec474406f7999447bd2052a8e4f2a154535c3`. The draft grew from 10,044 to 10,276 words (+232). The retired-name scan returned zero hits, the section references resolve, and the document status remains `draft`. Parsed the YAML item example, three TOML examples, one fenced JSON example and six inline JSON examples. The actor fields and add flags, OKF mapping, per-section caches and protected configuration paths match the requested batch. The plural values of the output-only `replacement` field preserve both direct successors after a fork.

**Blocking — discovery can prevent initialization of a different project (§§9.2, 10).** Suppose `outer/docs/brain/pbrain.toml` exists and `outer/child/` is a separate Git checkout with no brain. From the child, plain `pbrain init` targets `child/docs/brain` under §9.2. The §10 walk finds the outer project’s marker, so §9.2’s new “refuses any other folder” rule rejects the child’s first initialization. Scope that refusal to an existing brain in the project being initialized. An ancestor marker above the child worktree root must not veto creation. Verify the child gets its own config and subsequent commands there find it first, while the parent’s files remain untouched. The owner’s walk order for ordinary commands can stay as written.

**Verdict — changes required.** One initialization blocker.

## Owner batch 1 re-check

Checked §§9.2 and 10 and scenario 27 in `fe68106472f6439decc21d666a292723e4982d960c9d1d5fa7f035d0f0bb2e4c` against `e3e0dc82`.

- **Nested project blocker resolved.** The existing-brain check stops at the initializing project’s root. Scenario 27 creates the child checkout’s brain, finds it first on subsequent reads, and preserves the outer brain. Ordinary read discovery still follows the owner’s upward walk.
- **Item 4 confirmed.** The `init` target precedence is `--path`, then `--brain`, then `PBRAIN_DIR`, then `docs/brain`. Scenario 27 covers the overrides. §§9.2 and 10 distinguish relative `--path` at the project root from relative `--brain` or `PBRAIN_DIR` at the current directory.

**Verdict — approve.** No blocker in the two assigned checks.

## Owner batch 2 check

Compared `e296c95f` to `7ad42067639e2cb388c337c8770e13be912882841fff5a002dac75b864809186` (10,678 → 11,367 words, +689). The copied/linked model, audience rules, summary validation, search marker, warning-only secret scan and deferred cache commands follow the owner’s decisions. Scenario 28 covers all six requested cases. The import skill uses the owner’s “pricing thread for the Q4 launch, Nov 2026” example. The document remains draft; section references resolve and the YAML, TOML and JSON samples parse.

1. **Linked imports promise a local original they never acquire (§§8.1, 8.4, 8.5).** The destination notice still tells every importer that a copy of the original stays in local data, and the generic write order still requires storing that original and verifying its digest. A linked Google Doc has no acquired bytes or digest. Limit the original-storage step and its notice to `origin: file` and `origin: url`. For a linked item, tell the person where its metadata and summary will go; the “everyone with the repository” statement applies only to a shared destination. This also keeps the audience prompt accurate when offering the personal option.
2. **JSON lookup can return an unusable source link (§§2.4, 6.2, 7.2).** The reader accepts existing front matter of any length. Give an existing linked item a URL longer than 200 characters: `get --json` promises the link in `source`, but §6.2 truncates that field at 200. A larger output budget cannot recover the missing URL. Return the complete locator in both the linked text footer and the JSON field, counting it in the fixed response so `budget_too_small` still works. Search snippets can retain their existing caps. Verify with a long locator and an empty summary; the normal item header still identifies the result.

**Verdict — changes required.** Two linked-item contract corrections remain.

## Owner batch 2 re-check

Checked both findings against `cb80a37389f421d1d97fa7095132839edb04d5330b7f8aa4ee8d9b15d343f3b0` (+171 words from `7ad42067`).

- **Original storage resolved.** §§8.1 and 8.4 restrict storing, digest-checking and announcing an original to copied imports. Scenario 28 requires a linked import to make no network request, store no original and leave `.local/` unchanged.
- **Complete locator resolved.** §§6.2 and 7.2 reserve room for the complete link in the text footer and JSON `source`. Scenario 28 checks a 400-character link in both forms, the `budget_too_small` minimum and the §2.4 input boundary of 2,000 accepted and 2,001 rejected.

**Verdict — approve.** Both assigned findings are resolved.

## Owner batch 3 check

Reviewed the diff from `360bd549` to `94c242ac32eeb972dd7bfbe595598271e5bf750652c389d9fa5b69d882b56b81` (11,548 → 11,421 words, −127). Changes are confined to the local-data folder rename and marker removal.

§5.2 applies the name check to the resolved path for both shared `import` and `add --body-file`. Any enclosing `.pbrain` directory triggers the same refusal, including unaffiliated directories and another brain’s external personal cache. Scenario 6 retains direct and symlink cases. The layout, ignore rule, exposure guard, originals and later cache sketch use the new name throughout.

Verified zero occurrences of `.local`, `PBRAIN-LOCAL` and `local_unmarked` in the spec. The status is draft, all referenced sections resolve, and one YAML, three TOML and seven JSON samples parse.

**Verdict — approve.**

## Owner batch 4 check

Compared `0928b6b0` to `88a482fa96840ac81166d202cc7c315ad7b88211017d4bcd6f4b97681c076fb8` (11,443 → 12,423 words, +980), including the owner’s warning-severity decision. The spec remains draft. All 23 referenced sections resolve and one YAML, three TOML and eight JSON samples parse. The preview/confirmation flow, restored-decision scenario, time formats, local-day boundaries, inclusive instants and ranking within the filtered set are present.

1. **Use the resolved item’s section for deletion (§§5.2, 8.1, 8.6).** Scenario 29 allows `delete <shared-id> --personal` to show a shared item’s incoming personal links. But §5.2 says the flag chooses the write section, its lock and originals. A confirmed delete in the combined view can therefore lock the personal section while removing a shared file or check the wrong cache for its original. For delete, choose the lookup/report view with the flag, then resolve the id unambiguously and lock the actual target’s section. Under that lock, recheck the item and its same-section digest references, including replaced items. Only remove a validated original digest in that section’s cache. Test confirmed deletion of a shared source with `--personal` and the same digest in the personal cache: the personal copy must survive.
2. **A missing replacement target still qualifies for an error (§§4.2, 8.6, 9.1; scenario 29).** Delete predecessor A while B still says `replaces: [A]`. The missing target triggers the newly downgraded `dangling_link` warning, but also breaks §4.2’s “both ends exist” rule and therefore qualifies for `replace_invalid`, an error. Specify that a missing endpoint emits only `dangling_link`. Test the surviving `replaces` edge alongside the existing `relates` case: an otherwise clean brain must exit 0.
3. **Keep the exposure guard on delete (§§5.3, 8.6, 10).** “pbrain never runs Git” conflicts with the mandatory read-only Git checks before opening the brain. Skipping those checks for a destructive command would allow deletion while `personal_exposed` should stop it. Specify that delete runs the usual guards and never stages, commits or rewrites history. Use a conditional notice about history: a newly created shared item may never have been committed, and pbrain also works outside Git.
4. **Keep the time-window safeguard when the page is empty (§§6.1, 7.1, 7.3; scenario 30).** When the only matching decision is a year old, `--since 7d` must produce zero results **and** an outside-window count of one. The existing empty-result text still specifies just `0 results.`. Make the count part of the fixed footer/envelope even on a zero-hit page; direct readers to drop whichever bound or bounds were supplied. Exercise this case for both search and list.

**Verdict — changes required.**

## Owner batch 4 re-check

Reviewed `5372ff086aaf29b2f5193ec17228985f097e0c90d3c5e32dab324d5b305bf6bd` against `88a482fa` (12,423 to 12,902 words, +479). The status remains draft. All 23 referenced sections resolve. One YAML, three TOML and eight JSON samples parse.

1. **Target section and locking resolved.** §§5.2 and 8.6 use the flag for lookup and the resolved item's section for locking and file changes. Scenario 29 preserves the personal original when deleting a shared source with the same digest. Scenario 16 now covers the owner's added case in which `verify --personal` locks the shared section and updates the shared file.
2. **Missing replacement target resolved.** §§4.2 and 9.1 specify a single `dangling_link` warning. Scenario 29 deletes the predecessor and requires an otherwise clean `check` to exit 0.
3. **Git exposure guard resolved.** §8.6 requires read-only Git checks for preview and confirmed deletion, and conditions the history notice on whether the file was committed. Scenario 29 requires an exposed personal section or unavailable Git in a work tree to stop deletion.
4. **Empty time window resolved.** §§7.1 and 7.3 preserve the outside-window count on empty search and list pages and name the supplied bounds. Scenario 30 verifies both commands against a single older decision.

**Verdict — approve.**

## Owner batch 5 check

Checked `5372ff08` to `b8c7d75c3bb3e494b4e48b424e5288c57d16f6490c20a678408ad31171fa8bbe` (+961 words). The spec remains draft. All 25 referenced sections resolve. One YAML, three TOML and nine JSON samples parse. The passage jump, tag map and paging, operator upkeep, pilot ranking and semantic-research amendment are present. Three changes would prevent incorrect output.

1. **Blocking — keep a suspected secret out of the undeclared-tag notice (§§6.3, 8.2, 9.1, 10).** Supply an undeclared tag that also matches the existing Slack-token scan. A lowercase token with hyphens fits the tag alphabet. Section 10 requires naming it on stderr, while §6.3 forbids echoing a suspected secret. Give redaction precedence for this case. The shared write still succeeds, both notices can identify the field, and neither the notices nor the corresponding check messages should repeat the value. Cover text and JSON with a synthetic matching tag.
2. **Should fix — bound the size of echoed clauses (§§6.1, 7.1; scenario 31).** Limiting the footer to eight clauses still allows one long quoted phrase to exceed the entire default 2,000-character budget. This can turn a valid empty result into `budget_too_small` merely to repeat the caller's input. Cap each displayed label and trim optional diagnostic entries to the remaining rendered budget in text and JSON. Preserve the zero-result footer and applicable outside-window/replaced counts. Count each original clause in full. Test one over-budget phrase and nine short clauses.
3. **Should fix — make the retry and stopping rule consistent (§§12, 14).** The skill says to retry once with other words, “then `--all`”, even with zero replaced matches. The usage flow stops after one retry and claims the brain “holds nothing on the question”. Lexical search can miss a current answer under different terminology. Teach one targeted retry, using `--all` when the footer reports a relevant historical match or the task asks for history. After a miss say that these searches found no supporting item.

**Verdict — changes required.**

## Owner batch 5 re-check

Reviewed `3461303e103b1606cdcac8d1786bb3bfd3c75e864961224411f94370033dd02b` (+326 words from `b8c7d75c`). The spec is draft. All 26 referenced sections resolve and all 13 structured samples parse.

1. **Secret redaction resolved.** §§9.1 and 10 preserve the write and identify only the field when the secret scan flags an undeclared tag. Scenario 32 covers both notices and the check findings in text and JSON.
2. **Empty-result budget resolved.** §7.1 reserves the ordinary footer and applicable counts first, then adds up to eight hints while the rendered output fits. Each hint counts the full clause and displays at most 40 characters of it. Scenario 31 covers an over-budget phrase, nine short clauses and the outside-window count.
3. **Retry and stop rule resolved.** §§12 and 14 agree on one targeted retry and reporting which supporting items the searches found. §12 retains the 400-word cap on the shipped core skill. Scenario 23 checks the shipped text when it exists.

**Verdict — approve.**
