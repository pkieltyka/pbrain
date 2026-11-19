# pbrain functional spec review · Round 2

Independent review of draft `75bb412bca10a97d06dc18b3114805075b4978ab1bdde83cb672f0d1c498d670`. The revision settles authoritative new-item links, removal of replacement stamps and automatic repair, the warning-only secret scan, output measurement after serialization, and delivery of public URL fetching as the final v1 phase. I checked both Git path cases in temporary repositories. The remaining blockers are concentrated in the section boundary, compatibility edges and URL import. [Draft](../../functional_spec.md) · [Owner decisions](../../qa_round1.md) · [Round 1 review](./round1-lead-dev.md)

## Round 1 disposition

| Finding | Status | Evidence in round 2 |
| --- | --- | --- |
| B1 · Shared enumeration and containment | **Partly resolved** | §§2.2, 5.1–5.3, 7.2, 15.8–9 and 16 exclude ignored paths, personal roots and symlinks from shared discovery, use exact IDs, and require Git for the worktree guard. An external Git worktree remains exempt, and writes can target paths excluded by shared discovery. See R2-B1 and R2-S1. |
| B2 · Personal links and diagnostics | **Partly resolved** | §§5.2–5.4, 6.3, 9.1 and 15.6–7 define scope for both streams, acknowledge Git history, and reserve personal findings for opt-in. Generic exposure reporting is sound. The boundary still needs an explicit projection rule for persisted invalid links and for commands that read explicit file inputs. See R2-B2. |
| B3 · Authoritative replacement and malformed graphs | **Partly resolved** | §§2.4 and 4.1–4.3 preserve recorded status, remove automatic repair, reject cycles, and define forks. §§4.4 and 7.2 leave priority, cycle handling and history ambiguous when the legacy stamp and new links coexist. See R2-B3. |
| B4 · Publication and write races | **Partly resolved** | §§8.1 and 15.14–15 provide a one-item commit, writer serialization and orphan-cache recovery. Cache failure, acknowledgement loss and the precise scope of the conflict guarantee still need a contract. See R2-S4. |
| B5 · Public-only fetch | **Partly resolved** | §§8.4, 15.20 and 16 specify direct HTTP(S), redirects, TLS, distinct size limits and no ancillary fetches. The guard must operate before dial, and fixture expectations have to match mixed DNS and ignored proxy semantics. See R2-B5. |
| B6 · Bounded usable responses | **Partly resolved** | §§6.1–6.3, 7 and 15.17 count rendered code points, require progress, add continuation and give findings their stdout exception. History flags, body envelope and continuation scope still disagree. See R2-S2. |
| S1 · Factory adoption | **Partly resolved** | §§3, 4.4, 13 and 15.21 document missing deciders, operator review and unchanged fixture bytes. §13 offers an in-worktree personal-folder pointer disallowed by §5.1. Legacy stamp behavior also needs the graph contract in R2-B3. See R2-S1. |
| S2 · Duplicate IDs | **Partly resolved** | §§2.3, 5.2 and 15.13 protect exact reads and writes from ambiguous IDs and isolate shared edge lookup. A fully specified total order and case rule for duplicate records are still needed. See R2-S6. |
| S3 · Editable YAML and invalid values | **Partly resolved** | §§2.4, 9.1 and 15.16 define a readable subset, conservative skipping, opaque bodies and refusal of unsupported edits. CRLF and field validation still disagree between requirements and fixtures. See R2-S5. |
| S4 · Citation and refresh identity | **Partly resolved** | §8.4 separates `origin` from citation and drops general refresh, which protects local exports with URL citations. I accept deferring final URL and converter-version fields. The replacement and fill-in dispatch still conflicts, and hash byte identity needs precision. See R2-B4 and R2-S7. |
| S5 · Imported content, scanning and retention | **Partly resolved** | §§6.2–6.3, 8.2, 8.4 and 15.4 apply common body normalization and implement the owner-approved warning. I accept omitting a source-permission engine. State who authorizes the extra retained copy and define the actual cache bytes and scan boundaries. See R2-S7. |
| S6 · Lexical ordering and status filters | **Partly resolved** | §§7.1, 7.3 and 15.1 define token boundaries, scope, recorded status and the approved ranking shape. Phrase/prefix match classes and band boundaries can still change results. See R2-S3. |
| S7 · Index versus evidence and determinism | **Partly resolved** | §§8.4, 9.3, 11, 15.5 and 16 separate originals from indexes, fix time in acceptance and leave the backend to measurement. “Files on disk when the command starts” overpromises a snapshot with parallel manual writers. See R2-S4. |
| S8 · Consumer authority and prime | **Resolved** | §§7.4 and 12–13 freeze the injected slice for one session, surface forks and leave binding policy with the consumer. The remaining pagination defect belongs to B6. |
| S9 · Surface area | **Resolved** | §§8–9 and 17 remove standalone `fetch`, `reindex`, auto-repair and custom folders. Retaining one bounded queryless inventory has a clear agent use. R2-B4 offers a simpler import mutation path. |
| N1 · Output examples | **Resolved** | §§7.1–7.4 use recorded status, a concrete history object and a prime offset. The remaining executable pagination contract is in R2-S2. |

## Blocking (5)

### R2-B1 · Protect personal notes and originals in an external worktree

**Sections** §§5.1, 5.3, 8.4, 10, 15.8 and 18.3.

**Problem.** The external-root exception allows personal writes and raw snapshots into a second repository with no ignore or tracked-file check. §18.3 expressly permits that repository, while §8.4 promises that every original remains local and untracked. In a temporary second repository, Git listed both `personal/memo.md` and `personal/.pbrain/fixture-digest` as unignored. Normal staging would include both. The opt-in section's visibility is a separate choice from publishing its derived data or raw originals.

**Fix.** For v1, accept external personal roots in ordinary folders outside all Git worktrees. If a second repository is an intentional deployment mode, name it as a distinct trust boundary and verify both the notes and local-data protections in the actual containing worktree before opening it. Keep external data unavailable to shared reads in either case. Add fixtures for an ordinary external folder, a path under a second Git repository, and a linked worktree. [Owner answers 5 and 9](../../qa_round1.md) · [Git file enumeration](https://git-scm.com/docs/git-ls-files)

### R2-B2 · Apply the shared view to serialized links and explicit file reads

**Sections** §§2.2, 5.2–5.4, 7.2, 8.2, 8.4, 9.1 and 15.6.

**Problem.** Resolving shared edges in the shared view leaves open whether `get` prints raw `relates`/`replaces` fields and whether the dangling-link report echoes a target. A hand-edited structured link can therefore expose a personal ID even though it does not resolve. The shared-view guarantee also covers “everything the command does”, while `import <file>` and `add --body-file` can open a path under a managed personal root and publish the body and original in the shared section. §5.4's copied-prose limitation is appropriate for manually supplied content. These paths are opened by pbrain itself.

**Fix.** Render only the shared view's resolved structured targets, and report any other target using a generic diagnostic without the raw ID. Shared reads can do this without walking personal items. Require mutually exclusive audience flags on writes. Reject managed personal file paths and their aliases as inputs to shared `import` and `add --body-file`; keep errors indistinguishable from inaccessible or missing inputs where privacy requires it. Extend the privacy fixture to cover manually inserted structured links, raw and resolved lookup output, explicit file arguments, and both streams. Clarify that qmd needs an allowlisted shared corpus or equivalent exclusions for all ignored personal material, including a legacy folder with a different name. [Draft privacy contract](../../functional_spec.md#52-two-views) · [qmd exclusion evidence](../../research/knowledge-memory-landscape/qmd/research_summary.md)

### R2-B3 · Resolve legacy stamps inside the same acyclic lifecycle model

**Sections** §§4.1–4.4, 7.2, 9.1, 15.11–12.

**Problem.** §4.4 says any uniquely resolved same-section legacy stamp hides its note. If A says `superseded-by B.md` and A also has `replaces: [B]`, the stamp and new-format link form a cycle. With independent rules both can become hidden and there is no current head. In a second case, a stamp from A to B and a valid new-format C replacing A can introduce two heads even though the owner gave new-format links authority. History explicitly follows valid `replaces` links, so an old stamp can hide a note yet leave its successor outside history.

**Fix.** Resolve canonical edges first and keep their authority. Treat a legacy stamp as a compatibility edge only if its predecessor has no valid canonical successor, the target is an unambiguous same-section item, and accepting the edge preserves an acyclic graph. Specify a deterministic legacy-cycle policy and report contradictory, invalid and unresolved stamps. Use the resulting edge set for replaced status, current heads and history. Define legacy path anchoring and reject an unqualified ambiguous filename. Add mixed-format tests, including the two-node cycle and canonical successor plus disagreeing stamp. [Owner answer 2](../../qa_round1.md) · [Legacy lifecycle](../../functional_spec.md#44-factorys-older-stamp)

### R2-B4 · Make replacement import distinct from filling an existing item

**Sections** §§4.2, 8.1, 8.4, 14–15.

**Problem.** §8.4 says a matching URL both fails as a duplicate and can be acquired anew with `--replaces`. It also unconditionally fills a current bare link at that URL. Given current source X and current bare link L for U, `import U --replaces X` can create a new item, mutate L and ignore the flag, or fail. Mutating L can discard the requested lineage. The spec provides no handling for multiple current matches after a branch merge, a changed bare-link candidate while the fetch ran, or flags that would mean overwriting L's existing title, tags or provenance.

**Fix.** Prefer append-only specialization: if a current bare link should be populated, accept `import U --replaces <link-id>` and publish a new URL item, preserving the old link in history. If keeping ID on promotion is required, make it an explicit mutually exclusive action naming the exact target, and revalidate an unchanged, current, empty `link` under the section lock. Specify a short decision table for file/link/URL origin, normal import, redo with a matching replacement and ambiguous matches. Test equal and changed bytes, simultaneous promotions and an unrelated `--replaces` when a link already exists. [Owner answer 8](../../qa_round1.md) · [Import duplicate rules](../../functional_spec.md#84-pbrain-import-file-or-url)

### R2-B5 · Make the public connection rule preventive and align its release gate

**Sections** §§8.4, 15.20, 16 and 18.2.

**Problem.** Checking the “address actually connected to” could be implemented as a peer check after dial, which already opens the forbidden connection. The draft permits choosing a public IP from mixed DNS answers, while scenario 20 says the mixed case fails. It also says to ignore proxies, while the scenario expects a proxy environment variable to cause failure. These contradictions leave the release gate and the security boundary open to interpretation.

**Fix.** Reject a mixed resolver result before any connection and connect to one of the validated addresses without a second hostname lookup. Apply the same validation to literal addresses and every redirect, including URL userinfo and disallowed schemes, retaining TLS hostname verification for HTTPS. With proxy settings present, assert a public destination is still reached directly and the proxy receives no request. If direct access is unavailable, assert a normal fetch failure. Keep the fetch-phase acceptance gate and explicitly test that rejected destinations are never dialed. [Go HTTP transport](https://pkg.go.dev/net/http#Transport) · [Go address classifications](https://pkg.go.dev/net/netip#Addr.IsGlobalUnicast) · [Owner answer 8](../../qa_round1.md)

## Should fix (7)

### R2-S1 · Require new shared items to be visible and make adoption reachable

**Sections** §§2.1–2.2, 5.1, 8.1, 9.2, 10 and 13.

**Problem.** The new discovery rule drops ignored Markdown. A repository can already ignore `docs/brain/notes/` or `*.md`; `add` would still report a successful write that the user cannot find or stage normally. I reproduced the mismatch with a new note under an ignored kind directory. Also, §13 offers `personal.yml` as an adoption path for any ignored factory private folder, while §5.1 disallows pointing it anywhere within the project worktree.

**Fix.** Before publishing, apply the shared eligibility rules to the actual destination and refuse an ignored or otherwise unreadable target. Use one path policy for root resolution, enumeration, writes and init. Keep the old ignored folder excluded until it is explicitly moved, or permit an in-worktree pointer only when it passes the full exposure guard. State the valid adoption path in the fixture. Define `init` as resumable when config creation and ignore-file updates are interrupted, and state whether existing config or an explicit conflicting `--path` wins. [Git ignore behavior](https://git-scm.com/docs/gitignore) · [Draft adoption flow](../../functional_spec.md#13-factory-integration)

### R2-S2 · Specify mode-specific response and pagination contracts

**Sections** §§6.1–6.3, 7.2–7.4, 9.1, 15.17.

**Problem.** §6.1 requires `more` and `next` on JSON pagination, while the body spec only names `next`. History “pages like list” while the get flag table uses body code points and omits `--limit`. An empty body cannot provide the mandatory first code point. The prime continuation repeats tag filters but omits `--personal`, which can change the corpus and skip items. An exhausted history or report still needs a valid minimum envelope.

**Fix.** Add a small table of offset units and count limits by command mode, including `get --history`. Define the empty body, end-of-stream and invalid-offset outcomes, `more` units, and the minimum fitting envelope for empty and nonempty responses. Continuations must retain all selection flags, including section and history. Decide `check` exit status from the full eligible report, independent of the subset emitted. Add pagination tests for empty bodies, large history and filtered personal prime. [Output budget contract](../../functional_spec.md#61-budget)

### R2-S3 · Make match class and time bands executable

**Sections** §§7.1, 11, 15.1 and 18.1.

**Problem.** “Every word in the title” can promote an item whose title contains the quoted phrase's words out of order while only its body contains the contiguous phrase. The explanatory prose promises every equally matching newer item wins regardless of standing, while the explicit key and a fixture place standing before timestamps within a band. Time boundaries, future dates and items with missing or invalid `created` are unassigned.

**Fix.** Keep the owner-approved match class → recency band → standing → timestamp → ID order. Compute each class over whole query clauses, using the same prefix and phrase predicates as matching. Define bands as non-overlapping ages at a single UTC evaluation instant, choose inclusivity at day 30/180/365, assign future and unknown dates, and freeze the clock for fixtures. Pin the two items in the standing test to one band, and cover the one-day case that crosses a band boundary. [Ranking decision](../../qa_round1.md) · [Draft ranking](../../functional_spec.md#71-pbrain-search-query)

### R2-S4 · Clarify acknowledgement loss, concurrent reads and conflict guarantees

**Sections** §§8.1, 8.4, 9.3, 11, 15.14–15.

**Problem.** Publication is now a single-file commit. A process can still die or lose stdout after that commit, so absence of the success acknowledgement does not prove a retry is safe. The draft specifies that an edit fails if bytes changed after its read, then excludes hand edits from serialization. A last comparison can detect some external edits but cannot guarantee compare-and-swap with an uncoordinated writer. A traversal also cannot promise every file as of command start across external mutations.

**Fix.** Define exit 1 with a command error as a pre-publication failure and distinguish interruption or acknowledgement loss after commit. Define the verify no-op and its output explicitly. Guarantee serialized check/write for cooperating pbrain writers, check for observed external edits before publication, and document a read seeing complete committed files with stable results on unchanged input. If a consumer needs an atomic multi-file snapshot, that is a separate requirement. Add a pipe/termination test after commit and a pbrain read racing a writer. [Write contract](../../functional_spec.md#81-the-write-contract)

### R2-S5 · Reconcile supported YAML edits with fixtures and validation

**Sections** §§2.4, 8.2–8.4, 9.1, 15.16.

**Problem.** The spec writes LF and restricts edits to fields in the written form, while the fixture requires editing CRLF without changing other bytes. The 10-entry and single-line/size constraints also lack consistent behavior for a hand-edited item with valid YAML and an out-of-range value. The check table mentions wrong types and dates, but leaves overflow and invalid contents to the implementer.

**Fix.** Define editable shape separately from emitted formatting. Accept both newline styles, preserve existing newlines and unrelated bytes, and specify behavior for a newly added key in a CRLF block. State safe read and mutation behavior for too many relations or tags, invalid actors, overlong single-line fields and invalid dates. A malformed lifecycle list must never hide a target through silent partial acceptance. Test writer-generated format as well as byte-preserving CRLF edits and supported hand-edits. [Front matter and edit contract](../../functional_spec.md#24-front-matter)

### R2-S6 · Complete identity and stable tie-breaking for malformed stores

**Sections** §§2.3, 5.2, 7.1–7.4, 8.1 and 9.1.

**Problem.** New ID admission ignores case, but rules for exact lookup and duplicate detection only say “same stem.” Different choices for `Deploy` and `deploy` can yield different answers after moving a checkout between Linux and macOS. Two ambiguous items can tie on all ordering keys when both are listed. Multiple findings for one file likewise tie under the report's sort keys, so offset paging can repeat or skip entries if order changes.

**Fix.** Specify a case policy for legacy IDs and case variants, and keep the collision handling consistent in reads, writes and graph resolution. Distinguish duplicate records using a final stable section/path key in inventory and list-like output, or omit them from ordinary results and give `check` their paths. Give check stable severity, path, code and field/location tie-breaks. Add fixed-clock multi-page fixtures with duplicated and case-variant stems. [Identity and duplicate contract](../../functional_spec.md#23-ids)

### R2-S7 · Define original byte identity, retention and inert rendering

**Sections** §§6.2–6.3, 8.2, 8.4, 15.4 and 15.20.

**Problem.** “After transfer decoding” does not say whether raw HTTP originals and their hash include content encoding like gzip. §8.4 promises a cache on each acquired file/URL and promises nothing is written on failure, while §8.1 allows an orphan after interruption and says local data is updated after publication. The permission model can stay simple, but the destination notice describes shared Markdown only, leaving operators unaware that removed front matter or credentials may remain locally in the original. Imported bodies are called inert while raw Markdown links and HTML can still be active if a consumer renders them.

**Fix.** Hash and retain the exact bytes supplied to the normalizer, explicitly after transfer and content decoding for HTTP, before Markdown cleanup. State whether a cache is mandatory. If it is, publish/cache it first in its section and tolerate an unreferenced blob if item publication fails. Disclose the local copy and its retention consequence without inspecting license rules. Apply warning-only scan consistently to shared material persisted on all paths and specify whether transient raw-only content is included. Distinguish text/JSON from any consumer rendering, with no automatic external resource retrieval and no execution of imported markup or prose. Keep Markdown/HTML fidelity fixtures and error text free of credentials. [Owner answers 8–9 and Q5](../../qa_round1.md) · [HTTP content codings](https://www.rfc-editor.org/rfc/rfc9110.html#section-8.4) · [Ingestion research](../../research/knowledge-memory-landscape/source-ingestion/provenance-and-first-importer.md)

## Architect's declines

- **Final URL and converter version.** Accept deferring both fields. v1 retains the requested locator, acquisition time, origin and digest. Resolve relative links against the effective URL during conversion and state what the digest covers. Those two decisions matter even without additional metadata.
- **Source permissions for raw retention.** Accept omitting a policy engine and source-specific permission rules. The operator should know the command retains an additional local copy, what happens on deletion, and which data the shared repository will carry. Protection from staging is a pbrain path rule.
- **Git checks on external personal folders.** Accept the exception only for an external root that lies outside all Git worktrees. A second repository is an additional operating mode that the current guarantees do not cover.
- **Generic exposure report.** Accept. One sanitized `personal_exposed` error with a safe remedy, including on `check`, preserves the shared-view contract. Keep item-level paths and values out of it, even when the failure was caused by one tracked child.

## Section 18 reviewer choices

1. **Recency bands.** Keep 30/180/365 days as initial values and the owner's chosen ordering. Specify boundaries and the clock, then tune with a real query set. This gives a recent match precedence over an equally relevant old decision while preserving standing within a band.
2. **Ignore proxies.** Keep direct-only as the v1 default and behavior. It gives a testable pre-dial policy and a clear fallback to downloaded Markdown where a proxy is mandatory. Environment variables must have no effect on a valid direct request.
3. **External personal folder without checks.** Approve for an external plain folder outside Git. Reject the unchecked external-repository option for v1 because it can stage both untouched source bytes and personal notes. An intentional personal collaboration workflow can define different versioning and cache rules later.
4. **HTML floor and budgets.** Keep rendered-output defaults as initial fixture targets. Replace the 200-character floor with a meaningful-content test where possible, or explicitly present it as a conservative HTML-only heuristic with the file and link-only escape routes. Include a short substantive article and a long loading shell in the fixtures. Length alone measures neither useful content nor the need for a browser. [Converter fixture evidence](../../research/knowledge-memory-landscape/source-ingestion/converters-and-fidelity.md)

## Verdict

**Changes required.** Five blocking and seven should-fix findings. Before approval, settle external personal storage and shared projections, legacy edge authority, the explicit source import transition and the pre-dial public URL policy.
