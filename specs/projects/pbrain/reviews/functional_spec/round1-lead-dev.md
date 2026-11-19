# pbrain functional spec review · Round 1

Reviewed the 575-line draft with SHA-256 `b10a92eac100cfd0c8aee88950b545070a86d389743c468c64f3574209e4ded5` against the approved overview, the owner's Q&A and the research. I also checked the Git ignore behavior in a temporary repository. The main direction is sound: append-mostly Markdown, one brain per project, bounded local reads, and factory policy at the consumer boundary. The six blocking findings concern contracts that can expose personal data, misstate a decision, lose a write, fetch a non-public resource, or break the agent's size budget. [Approved overview](../../project_overview.md) · [Owner decisions](../../qa_round1.md) · [Research synthesis](../../research/knowledge-memory-landscape/research_summary.md)

## Blocking (6)

### B1 · Define shared enumeration and root containment before any read or write

**Sections** §§3.1, 6.1–6.2, 7.3, 8.1, 10, 11, 16.

**Problem.** The recursive reader accepts every Markdown file under the brain except dot folders. Default `personal/` is inside that tree and has no dot prefix. A straightforward implementation parses and indexes private notes as shared. The draft also allows direct `get` paths, configurable roots and `--dir`, with no policy for path traversal, symlinks or an external personal root that overlaps the shared tree or lies in another Git worktree. §6 requires an exposure check and §16 permits operation with Git absent. A temporary Git fixture confirmed that a directory ignore probe still passes after a personal child has been force-staged, so checking only the root's ignore status cannot satisfy the guarantee.

**Fix.** Classify roots and exclude the personal subtree and other ignored paths before enumerating shared candidates. Define non-overlapping canonical section roots, except for the designated personal subtree, and resolve path lookups only against already eligible items. Constrain all reads and writes to the selected section, including `--dir`, and specify treatment of symlinks and non-regular files. Require both effective ignore and tracked-file checks for protected paths in the actual containing worktree. Define how a managed external root inside another repository is checked, or restrict linked roots to plain folders outside worktrees. Require Git when its guarantees apply, and fail closed if a worktree's exposure state cannot be verified. Add tests for unique personal text and IDs, symlink aliases, escaped writes, a tracked child under an ignored root, a nested worktree and missing Git. [Git ignore semantics](https://git-scm.com/docs/gitignore) · [Git check-ignore](https://git-scm.com/docs/git-check-ignore) · [Go rooted file APIs](https://go.dev/blog/osroot)

### B2 · Keep personal links and diagnostics out of the shared interface

**Sections** §§3.3, 5.3, 6.2, 7.3, 9.1, 11, 15.4.

**Problem.** The guarantee covers every command without `--personal`, but default `check` reads the whole brain and prints paths and messages. It can disclose a personal path or identifier while correctly detecting an error. Shared `add` constrains two fields, while body citations, hand-edited fields and a stale `replaced-by` stamp can carry a personal id through `get` or the replacement resolver. Suggestions, errors and derived cache entries need the same scope. Moving a previously committed item to the personal section by hand leaves evidence and incoming links in shared Git history.

**Fix.** Define an audience for all output and derived state, including errors, prefix suggestions, link resolution, history, counts and `check`. Default `check` should publish shared findings and generic exposure errors. Reserve detailed personal paths/content for explicit `check --personal` and use a generic shared diagnostic for blocked cross-section references. Validate supported citation and link fields on write, and omit or quarantine malformed cross-section structured references on read without printing the target. State the limit for prose deliberately copied by humans and the history implications of moving a previously shared item. Add a matrix that searches for the same personal marker in both streams, JSON and text, shared caches, direct lookup and diagnostic paths. [Owner privacy decisions](../../qa_round1.md) · [Git history and clones](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/removing-sensitive-data-from-a-repository#about-sensitive-data-exposure)

### B3 · Make replacement links authoritative, preserve prior standing and define malformed graphs

**Sections** §§3.3, 4, 5.1–5.3, 7.3, 9.1, 12.

**Problem.** Owner answer 2 gives authority to the new item's link. The draft instead hides an old item when its stamp names any existing item, and `check --fix` can manufacture the purportedly authoritative link from that weaker evidence. A stale stamp can therefore hide an item after the new edge is removed or rejected. Stamping a verified old learning also destroys its recorded base status, so later validation cannot reliably evaluate the standing rule from disk. Self-links, multi-item cycles and history across forks or many-to-one replacements have no defined outcome. A cycle can hide every item in it or make chain traversal fail to terminate.

**Fix.** Separate recorded verification from derived lifecycle. Validate incoming replacement links against same-section endpoints and base standing. Ignore invalid edges, including the edges in a cycle, report them, and keep otherwise eligible nodes visible. Limit fallback to a named legacy compatibility path for existing factory stamps, with an explicit precedence and conservative behavior for missing or invalid targets. Any auto-repair regenerates mirrors from valid links. Define immediate successors, all current heads, and a deterministic history order. A fork gets all heads and no invented winner. Add fixtures for a missing or contrary stamp, a provisional edge against a formerly verified record, a cycle and a merged fork later resolved by one replacement. [Owner answer 2](../../qa_round1.md) · [Supersession research](../../research/knowledge-memory-landscape/markdown-knowledge-stores/research_summary.md)

### B4 · Give multi-file writes a commit point and define same-checkout races

**Sections** §§5.1–5.3, 8.1–8.4, 9.1–9.2, 11, 15.11.

**Problem.** `add --replaces A --replaces B` publishes a new file and edits two old ones while promising no writes on failure. An atomic rename can replace one file. It does not make the three changes a single transaction. An interrupted stamp also contradicts §5.2's refusal of a second successor in the same checkout unless validation and publication share a write boundary. Parallel fetch/verify/manual edits can overwrite newer content. A cached original and its Markdown item are also two writes.

**Fix.** For v1, make atomic publication of the new Markdown file the replacement commit point. Omit old-file stamping by default, or make it a best-effort convenience that clearly reports a committed new id and any missing mirrors. A retry must not create a second successor. Coordinate pbrain writers per physical section. Revalidate current state under the write lock and compare the source version before committing an in-place edit acquired earlier. Stage cached originals before publishing the reference if they are required, and permit cleanup of orphaned local objects. Specify that manual writers are outside the locking protocol. Exercise failure between publishing the item and stamping the first predecessor, concurrent attempts to replace one item, two verifies, stale fetch versus a hand edit, and two Git branches resolving a fork. [Owner answer 2](../../qa_round1.md) · [Git merge research](../../research/knowledge-memory-landscape/markdown-knowledge-stores/ids-layout-and-git-merges.md)

### B5 · Specify the public-only HTTP boundary through the actual connection

**Sections** §§1, 8.3–8.4, 11, 16.

**Problem.** A DNS check detached from the connection cannot enforce public-only fetching. The spec does not state how to handle mixed DNS answers, alternative IP representations, environment proxies, redirect URL validation or ancillary requests by a converter. Go's default HTTP transport can inherit a proxy. A 5 MiB limit also needs a byte domain, since compressed input and converted output expand independently. These apply equally to fetching a previously bare or hand-edited URL.

**Fix.** Define a direct public HTTP(S) fetch profile, with a deliberate proxy policy, no ambient auth, validation of every hop, allowed addresses enforced at dial time, and certificate checks for the URL host. Specify rejection of non-public/special-purpose destinations and unsupported redirect schemes, and prevent network access during conversion. Apply independent total time, wire/decoded input and normalized output limits. Retain a public-URL phase in v1, gated on tests for redirect escape, mixed answers, address changes, proxy environment, compression, limits and no published item on failure. [Owner answer 8](../../qa_round1.md) · [Go HTTP transport](https://pkg.go.dev/net/http#Transport) · [Ingestion research](../../research/knowledge-memory-landscape/source-ingestion/provenance-and-first-importer.md)

### B6 · Define a bounded, usable response for every output shape

**Sections** §§7.1–7.5, 9.1, 11, 15.9, 16.

**Problem.** Whole-result packing applies in §7.1, while `get` must both return one item and clip the body. A legacy path or metadata arrays alone can exceed the minimum 200-character budget, leaving no room for a page of body. History has no count, paging or omission contract. `search` and `prime` tell the agent to get more but offer no general continuation under their result caps. `check` limits count only, and its required stdout JSON on findings disagrees with the rule that failure leaves stdout empty. JSON escaping and UTF-8 offsets are not covered by a field-by-field truncation rule.

**Fix.** Define the cap over the final serialized text or JSON, including newlines, framing and omission notices. Specify units as UTF-8 bytes or Unicode code points and fix the same body-offset unit for text and JSON. Reserve a minimal valid envelope, limit or omit optional metadata with an explicit indicator, and ensure a body continuation advances or returns a typed error when the chosen cap cannot hold it. Define reproducible continuation for retained list/history surfaces, plus a way to narrow `prime` and recover an exact item. Bound diagnostics, aggregate skipped-file warnings, and give expected `check` findings their own exit-1 stdout exception. Test escaping, Unicode, huge links/metadata, no-fit results, large history and minimum-budget pages. [Beads budget evidence](../../research/knowledge-memory-landscape/beads-and-gas-town/session-priming.md) · [Go JSON encoding](https://pkg.go.dev/encoding/json#Marshal)

## Should fix (9)

### S1 · Make factory adoption a deliberate compatibility contract

**Sections** §§3.3, 4, 5.3, 13.

**Problem.** The draft says existing notes are adopted unchanged, but existing factory decision notes were never required to have `decided_by`. Those notes become provisional and disappear from prime. A legacy stamp stores a file reference and can be meaningful before any new-format link exists. Factory also restricts which folders and private operations its members may use, while the generic recursive store and skill are broader.

**Fix.** Document the changed standing explicitly, keep existing bytes on init, and give the operator a pre-adoption report and manual review path for decisions with unknown authority. Never infer the decider from the author. Define legacy stamp path resolution inside the allowed scope, and ship an unchanged fixture containing a decision, a verified learning and a path-form supersession. Make the factory recipe preserve its pre-existing folder and personal-read policies. [Factory notes reference](https://github.com/pkieltyka/factory/blob/master/skill/references/notes.md) · [Owner answers 1–3](../../qa_round1.md)

### S2 · Treat duplicate IDs as ambiguity

**Sections** §§3.2, 7.3, 8.1, 9.1, 11.

**Problem.** File names can collide across directories or worktrees, including with older arbitrary stems. Choosing the lexically first path can silently retarget citations, authority links and mutators. A move changes the winner. The four-character suffix reduces cross-branch risk but cannot make §11's “never conflict” guarantee true.

**Fix.** Keep the owner-approved ID form. Check uniqueness across the applicable section, including custom subdirectories and filesystem case behavior, before admitting writes. Exclude ambiguous shared ids from graph resolution and reject ambiguous exact lookup and mutation, with safe diagnostics. Define namespace handling when a personal id collides with a shared one so that enabling personal mode cannot change a shared edge. Add forced-collision and merged-worktree fixtures. [Owner answer 3](../../qa_round1.md) · [ID and Git merge evidence](../../research/knowledge-memory-landscape/markdown-knowledge-stores/ids-layout-and-git-merges.md)

### S3 · Specify the editable YAML subset and handling of invalid values

**Sections** §§3.3, 5.4, 8.1–8.4, 9.1, 11, 16.

**Problem.** The draft promises arbitrary hand-edited YAML, semantic validation, and byte preservation for unrelated lines. Values can be multi-line, collection mappings can be flow style, aliases can change meaning, and duplicate keys can be read differently by parsers. The write/read behavior for unknown statuses, malformed dates or lists, missing mandatory source data, huge legacy files and malformed UTF-8 is unspecified. The footnote syntax also conflates ordinary Markdown footnotes and pbrain citations.

**Fix.** Define a small schema for pbrain-managed top-level fields and supported front-matter delimiters. State what is read, skipped, treated conservatively or loadable but ineligible for automatic editing. Refuse unsupported edits with a helpful error, preserve unrelated bytes and maintain fixtures for comments, CRLF, multiline values, duplicate keys and merge conflict markers. Define a narrow body citation syntax or defer special footnote parsing and use front-matter links plus normal Markdown references. Keep hand-written file size and metadata limits explicit. This is feasible in Go for a documented subset. The draft's guarantee across general YAML needs a lossless editor or an explicit refusal path. [Front-matter research](../../research/knowledge-memory-landscape/markdown-knowledge-stores/front-matter-and-links.md)

### S4 · Separate a source's citation from its refresh identity

**Sections** §§3.3, 8.3–8.4, 11, 14.

**Problem.** §14 imports a local Docs export with a URL as provenance. §8.4 then qualifies it for refresh merely because `source` is a URL. A fetch could overwrite the transcript with a Docs landing page. Duplicate checking also treats the same filename from different folders as the same source, and treats different URLs with byte-identical content as equivalent even though the citations differ. A bare link's optional body is overwritten on fetch, and the sample's `--body` is missing from the import flag inventory.

**Fix.** Distinguish display citation from acquisition method and refresh locator. Only explicitly fetched or fetchable web links get the latter. Record the requested and final safe URL, conversion method/version and the digest's byte definition for reproducibility. Deduplicate by an explicit stable locator within the section, using the digest for content identity/cache reuse. Decide how to keep a link annotation when attaching content, or omit annotation at first. Specify unchanged, unavailable and changed outcomes and how to cite the version that a decision relied on. [Source provenance research](../../research/knowledge-memory-landscape/source-ingestion/provenance-and-first-importer.md)

### S5 · Extend source trust and content handling through publishing

**Sections** §§1, 3.3–3.4, 4, 7.2–7.5, 8.1–8.4, 9.1, 12.

**Problem.** Stripping local Markdown front matter leaves remote Markdown treatment unclear. HTML processing says little about active URLs or raw HTML in Markdown, while plain-text output can carry control characters or look like tool output. A secret scan on only title/body misses provenance URLs and values written by refresh. An originals cache can retain content normalization removed. A short, valid reference can have fewer than 200 characters, and a JS login shell can have more. The current cut-off does not establish success.

**Fix.** Use one inert-content contract for all acquired bodies and every refresh path. Generate trusted metadata separately, preserve an explicit external-source label in presented output, and constrain active markup and controls at the appropriate conversion or rendering boundary. Define what can be cached and when raw retention is skipped, including source permissions and prohibited credential material. Use substantive extracted content and representative fixtures to choose the supported HTML behavior. If a secret check stays in v1, apply it to all newly persisted shared fields on every public write and report only location/category. Test local and remote front matter pretending to set authority, short valid text, an empty/static shell, redirection metadata, and secrets in a URL or header field. [Owner answers 1, 8 and 9](../../qa_round1.md) · [Converter fixture evidence](../../research/knowledge-memory-landscape/source-ingestion/converters-and-fidelity.md) · [Source security and retention](../../research/knowledge-memory-landscape/source-ingestion/provenance-and-first-importer.md)

### S6 · Pin lexical relevance and filtering enough for contract tests

**Sections** §§5.3, 7.2–7.4, 11, 15.

**Problem.** “Whole word”, case folding, phrase/prefix syntax and “lexical relevance” leave different implementations free to return different answers for e.g. `ci-runner`, `client.go`, Unicode case and unmatched quotes. The spec leaves source-only matches without a snippet rule and does not say whether `--status verified --all` uses the original or derived replaced status. `list` says newest first while §5.3 puts replaced items last. A general search sorted by absolute standing can bury a precise learning below loosely matched decisions.

**Fix.** State token boundaries, query parse errors, empty-query behavior, phrase/prefix handling and deterministic field weighting with representative fixtures. Filter audience and lifecycle before counting or limiting. Define status filters in terms of stored verification, with replacement as a separate inclusion switch. State whether a query's terms can span fields, what snippet appears for a metadata-only match, and how list orders replaced entries. For general search, use relevance first with standing to break equivalent relevance, or prescribe a small authority boost with a fixture proving where it wins. Keep decisions first in prime. [Agent retrieval research](../../research/knowledge-memory-landscape/agent-memory-systems/retrieval-priming-budgets-and-eval.md)

### S7 · Distinguish disposable indexes from original evidence and qualify determinism

**Sections** §§1, 5.4, 7.5, 8.3, 9.2, 15.10, 16.

**Problem.** “Deleting `.pbrain/` loses nothing” is false when it holds cached originals of local files that may be gone or URLs that may change. Rebuild equivalence can cover search results. It cannot guarantee reproduction of the original. “The same files” also omits the clock used by `stale_after` and the timezone/section/config inputs. Requiring a fresh persistent index for every command plus hot reads under 300 ms introduces freshness and write-lock complexity without benchmark evidence.

**Fix.** Document raw snapshots and derived indexes as different local data with different deletion consequences. Rebuild the index alone and compare answers with fixed time and config. Define freshness for ordinary Git checkout, same-size edits, deletions, replaced files, malformed or version-mismatched caches and simultaneous commands. Let architecture choose a scan or local cache after measuring cold CLI calls, warm reads and rebuilds on the stated synthetic data. Include complete command overhead and a reproducible test platform. [Research scale gaps](../../research/knowledge-memory-landscape/research_summary.md) · [Source cache research](../../research/knowledge-memory-landscape/source-ingestion/provenance-and-first-importer.md)

### S8 · Keep workflow authority in the consumer and scope the prime payload

**Sections** §§4, 5.2, 7.5, 12–13.

**Problem.** The general-purpose skill tells agents which position to follow, although owner answer 1 assigns workflow rules to the consumer. It says a verified learning holds until a newer decision replaces it, while §5.2 allows an equally ranked verified item to replace that learning. Factory's own policy is more specific. Prime also implies that local snapshots with identical item files always render identically despite differing review dates, tag filters and time.

**Fix.** The generic skill explains provenance, standing, current status and exact retrieval. It leaves binding force and authority checks to the consumer, and tells the caller to surface competing heads or evidence. Specify a frozen snapshot per session at the integration point, with deterministic output for the same scope and clock. Include compact attribution/source when it helps the agent choose a reference, and link omitted entries via the same filters. Give factory its specific behavior in the recipe or notes reference. [Owner answers 1 and 7](../../qa_round1.md) · [Priming and frozen snapshots](../../research/knowledge-memory-landscape/agent-memory-systems/retrieval-priming-budgets-and-eval.md)

### S9 · Reduce the initial command and metadata surface

**Sections** §§3.3–3.4, 7–10, 12, 17–18.

**Problem.** The product-level surface combines a persistent index and reindex command, CLI and history pagination, `list`, `fetch`, `check --fix`, custom write directories, two-way lifecycle edits and a fixed secret scanner. The owner asked for a small general-purpose tool. The complexity comes chiefly from duplicate entry points and mutation semantics. Five simple kind labels are cheap to keep if they remain mostly descriptive.

**Fix.** Ship the small common set around `init`, `add`, `verify`, `import`, `search`, `get`, `prime`, `check` and `skill`, keeping the owner-approved v1 imports. Drop a separate `reindex` in favor of automatic invalidation and rebuild. If browse is required in v1, expose one bounded queryless listing path so `prime` has a compatible continuation. Put any basic refresh under `import` with an explicit existing item id, or defer refresh if bare and fetched import cover the owner decision. Defer auto-repair, custom write paths and body citation parsing unless an acceptance case actually depends on them. Keep the five kinds and use lifecycle behavior only where required. [Owner decisions](../../qa_round1.md) · [Research synthesis](../../research/knowledge-memory-landscape/research_summary.md)

## Nit (1)

### N1 · Keep the output examples internally consistent

**Sections** §§7.2–7.3, 7.5, 15.

**Problem.** The search example shows `status: verified` alongside `replaced_by`, although the prose promises derived status `replaced`. The prime example's omission command includes provisional learnings and loses the active tag filter. Its replacement history example is a field sketch presented as JSON.

**Fix.** Use one valid current JSON sample and one valid replaced/history JSON sample from the acceptance fixture. Ensure the prime follow-up reproduces the intended eligible set, or describe it as a broader lookup. [Functional spec](../../functional_spec.md#72-pbrain-search-query)

## Positions on the open questions

| Question | Position and reason |
| --- | --- |
| 1 · Stamp spelling | Keep `replaces` in CLI and new-item front matter. If a status stamp must be written for factory interoperability, retain `superseded-by <file>` for that compatibility mode. Prefer no automatic stamp in the common path, or a mirror field that preserves the old verification state. One authoritative edge avoids drift and merge conflicts. |
| 2 · Standing for replacement | Keep decision-to-decision and the ban on provisional replacing verified. Validate against original recorded standing and reject cycles or invalid endpoints. Consider forbidding source items from replacing first-party knowledge even if independently verified, to preserve the owner's reference-only boundary. |
| 3 · Standing before relevance | Use relevance first for general search and standing to break equal relevance, with decision-first prime. Test the owner's contradictory-note fixture plus a precise learning versus several barely matching decisions. |
| 4 · Shared imports | Keep the owner's shared default. Make destination visible in the success contract and disclose it before publication in interactive use. Support explicit `--shared` and `--personal` for scripts, with the default still shared. |
| 5 · Secret scan | Leave a blocking scanner to the consumer for the first cut, or ship a tiny warning-only check. The example prefixes are broad, the override creates another policy path, and a short list cannot validate arbitrary imported content or source permissions. A retained check needs common coverage of all write paths. |
| 6 · Budgets | Keep five results and 2,000/6,000/2,000 as initial targets. Fix the rendered-output and pagination contract first, then validate with Unicode, escaped JSON and oversized metadata. Measure tokens on real agent prompts separately. |
| 7 · `verified_by` | Keep one optional scalar when an explicit verify action occurs. It distinguishes recording and confirmation in an answer without requiring Git access. Avoid adding timestamps or a per-actor audit trail. Owner answer 1 supports this as a lightweight choice, and does not specifically mandate the OKF field. |
| 8 · Actor form | Keep `human:` and `agent:` as display conventions with legacy bare text accepted. Enforce a single printable line and a sane length bound. Actor strings are provenance claims, and the consumer reviews authority. |
| 9 · Prime contents | Keep current decisions and verified learnings only, with source/actor where space permits and a useful omission cue. Use exact on-demand lookup for bodies and avoid a configurable kinds policy in v1. |

## Positions on the architect's pushback

- **A · Import audience.** Choose the drafted shared default and improve the destination cue. Requiring an explicit audience or defaulting imports to personal would change the approved behavior. Default disclosure, explicit flags, and a personal path keep the owner's choice visible. A transcript import should be an authorized human action because the resulting Markdown follows repo readership and history. [Owner answer 9](../../qa_round1.md)
- **B · Fetched URLs.** Agree with phasing. Deliver local files and bare links first, then a public static fetch phase within v1. Set a narrow, testable HTML fidelity target and put fetch behind the connection, redirect and resource-limit fixtures in B5 before release. Manual Docs export already covers the transcript path. [Owner answer 8](../../qa_round1.md) · [Ingestion research](../../research/knowledge-memory-landscape/source-ingestion/research_summary.md)
- **C · Twelve commands.** Agree on cutting standalone `reindex` and `fetch`. A separate `list` should survive only if agent runs need deterministic queryless browsing and the same contract cannot be covered by one query entry point. Preserve the owner-required `prime`, bare and fetched URL imports, and exact lookup. [Owner answers 7–10](../../qa_round1.md)

## Verdict

**Changes required.** Six blocking, nine should-fix and one nit. Resolve the personal boundary, authoritative lifecycle, publication and concurrency semantics, public-fetch contract and bounded response behavior before architecture. The existing acceptance fixture is a good baseline once it also exercises those failures.
