# Architecture review, round 1 · @lead-dev

Reviewed architecture `c5c5342b`, then rechecked the findings against fix-pass draft `d2ad3bf6cf645a7ed19d35771ca52ab694242417a3064e40d9fe61315e73058d` and complete functional spec `4622a82f`. Findings below cover contracts beyond the first review. [Architecture](../../architecture.md) · [Functional spec](../../functional_spec.md) · [First review](./round1-architect-2.md)

## Blocking

### LD-B1 · Separate a write's lookup view from its disclosure view

**Sections** Architecture §§2–4.3, 10.1, 12–13, functional spec §5.2 and scenario 26.

A rule can send a write to personal without `--personal`. The write needs the personal catalog to find duplicates and check links. The generic error rule in §13 excludes items outside the command's view, which leaves room to use that internal catalog as the disclosure boundary. A `duplicate` error or replacement suggestion could then name an existing personal item. The spec allows the acknowledgement to name the newly created item and bars names of other personal items in this case.

**Fix.** Have the write command pass a destination section, lookup view and disclosure view explicitly. Derive the disclosure view from the caller's flag, with the new-item acknowledgement as an explicit exception. Apply that boundary before formatting IDs, paths or suggestions in errors and notices. Exercise a rule-selected personal duplicate and a failed replacement whose candidates are personal. Check both streams in text and JSON. An explicit personal call should still receive the diagnostic its view permits.

### LD-B2 · Run the Git guard against the resolved worktree and its actual index

**Sections** Architecture §§6.2–6.4, 10, functional spec §§2.2 and 5.3.

The runner specifies `git -C` without defining its inherited environment. Repository and index selectors can override the worktree that pbrain meant to inspect. In a scratch Git repo with an ignored personal folder and a force-added note, `check-ignore --no-index` matched the folder. The default `ls-files` named the note, but the same query with an alternate empty `GIT_INDEX_FILE` returned nothing. Taken together the two queries could approve a tracked personal directory.

**Fix.** Define an environment policy in `gitx.Runner` that binds discovery, ignore and tracked-state checks to the resolved target worktree. Clear inherited repository/index-selection overrides, or refuse their use for the guard. Keep the normal ignore configuration for that worktree. Specify which target repository supplies the last-resort `user.name` when the caller selects a different brain. Add a real-Git fixture with an alternate index, and verify that an ignored, force-added file still yields `personal_exposed` before item access.

### LD-B3 · Give delete an explicit before-and-after graph contract

**Sections** Architecture §§4.3, 7.2, 10.6, functional spec §§4.2–4.4 and 8.6.

“Compute the facts from the graph” leaves the implementation to infer what happens after removal. The existing graph methods return current edges and display-oriented backlinks. Consider A stamped as replaced by C and a valid canonical edge from B replacing A. The canonical edge masks A's stamp. Deleting B activates the stamp, so A stays replaced. Counting edges in the existing graph would incorrectly say A becomes current. `NewerRelated` also omits older or replaced items whose links can be left dangling.

**Fix.** Design a pure graph operation that compares a catalog with and without the selected item, using the same validation and originating-view resolution as reads. Report current-again only for affected items that become current, and dangling only for remaining links that resolved to the target before removal and become unresolved. Compute preview and confirmed facts through the same operation. Cover the formerly masked stamp and an older backlink in addition to scenario 29.

## Should fix

### LD-S1 · Keep input waits outside the section's critical section

**Sections** Architecture §§2, 10.1–10.5 and 11.

§10.1 places file reads before the lock, but leaves stdin, URL acquisition and HTML conversion unplaced. A pipe waiting for EOF or an import waiting for a response could hold the only writer lock while another agent times out. This also affects teams sharing one external personal section across worktrees.

**Fix.** Make the shared write protocol explicit. Resolve the destination and guard the input before acquisition, then prepare bounded input and conversion outside the lock. Under the lock refresh mutable state, repeat the checks that depend on it, and publish. If the audience or destination changed during preparation, fail before storing anything under the earlier notice. Hold the lock through original reference changes and publication. Test that a stalled preflight in one process lets another complete a write to the same section.

### LD-S2 · Give the fitter the full result count and the requested offset

**Sections** Architecture §§8.3, 8.5, 9 and 14, functional spec §§6.1 and 7.1.

The engine hands the renderer an already offset and limited page. The fitter can shorten that page again. It needs the count before either limit to report what remains, and the starting offset to produce a continuation. “Plus the counts” currently leaves that payload implicit.

**Fix.** Make the page contract carry total eligible matches, requested offset, ordered candidates up to the item limit, and separate outside-window and replaced diagnostic counts. Let one layer compute `more` and `next` from the number actually rendered. Test 12 eligible hits at offset 4 and limit 5 with a character budget that fits only two. The first response should continue at 6 with six remaining, with diagnostic counts unchanged on later pages.

### LD-S3 · Make CLI and time parsing seams explicit enough to test

**Sections** Architecture §§2–3, 4.2, 8.1–8.3 and 14, functional spec §§6.3 and 7.1.

The test plan uses in-process scripts with injected environment and time, but the CLI entry point and the per-call timezone contract are still implicit. Process-global timezone state can hide a failure in `TZ`-dependent tests that run multiple commands in one process.

**Fix.** Show a `cli.Run` entry point that accepts context, argv, stdin, stdout, stderr and explicit process inputs, returning an exit code. Resolve the effective timezone once per invocation from those inputs and use the same evaluation instant in the time window and ranker. Test two timezones in one process and run a small set on the built binary too. Include invocation from another checkout, stdin body handling and split stdout/stderr for the delete preview.

### LD-S4 · Anchor settings writes to the config file's location

**Sections** Architecture §§4.2, 6.4, 10.3–10.4 and 12, functional spec §§5.1–5.3 and 8.5.

`audience set` and `unset` mutate `<brain>/personal.toml`, but the current design takes a lock in the personal item section and opens files through that section's root. A configured personal section can be external or unreachable. It then contains neither the config file nor a reliably available lock for updating it.

**Fix.** Use the brain directory as the confined root for `personal.toml` and serialize by the canonical config location, for example using the brain's existing shared-data lock. Reload and atomically replace the file under that lock. Keep the exposure guard and establish lock order if an operation needs both settings and item locks. Cover an external personal section, an unreachable one and two setters in the same checkout.

## Notes

- **Lexical implementation (§§8.1–8.6).** Keep the Go scan as the reference. In component design, make clause parsing explicit on punctuation-plus-prefix inputs and test the lexical eligibility, counter and continuation contract together. No FTS5 tokenizer should become the public grammar accidentally. [Go index research](../../research/semantic-search-and-index/qmd-and-go-index-engine/go-index-options-and-probes.md#fts5-fits-as-an-accelerator)
- **Semantic seam only (§§8.5–8.7).** The per-section, rebuildable storage and pre-limit eligibility fit the research. Keep policy and rendering above candidate generation. A future approved mode needs to carry stable item identity, a stored-body code-point span, evidence type and index readiness through that boundary. Readiness lets the CLI distinguish a missing or stale generation from a genuine empty result. Keep engine selection and rank changes open until the owner approves the amendment. [Hybrid and lifecycle research](../../research/semantic-search-and-index/qmd-and-go-index-engine/hybrid-ranking-and-index-lifecycle.md)
- **Build sequence (§18).** Two phases make sense. The audience/disclosure contract, Git environment rules and delete graph operation need agreement before parallel component work. Native locking and no-clobber publication tests should use subprocesses on the supported hosts. In-process scenario scripts remain useful for deterministic rendering and time fixtures.

**Verdict — Changes required.** Close the disclosure, Git guard and deletion-graph contracts before component implementation.

## Round 1 fix-pass re-check

Reviewed architecture `056bf976a9fac0f24e29f5537c6d7957007aa8ca06c0cab4cfe55acea83a0eca` against functional spec `4622a82f`. Scope was the seven findings above and the semantic seam.

| Finding | Disposition | Evidence |
|---|---|---|
| LD-B1 | Resolved | §§10.1, 13 and 14 distinguish destination, lookup and disclosure. Only the new item is exempt from the disclosure view. Both streams and output modes are tested for personal duplicates and replacement suggestions. For clarity, update the `Lookup` field comment to say it follows the destination for a new item and the caller's view for an existing one, as step 2 and scenario 29 require. |
| LD-B2 | Resolved | §§6.3 and 14 clear inherited repository and index selectors, query author fallback in the selected brain, and require a real-Git regression test. In a scratch repository, the ignore rule matched and an alternate empty index omitted a force-added personal file. Removing `GIT_INDEX_FILE` returned the file from the actual index. |
| LD-B3 | Resolved | §§7.3, 10.6 and 14 share a before-and-after graph operation between preview and confirmed deletion. It uses the holder's view and has fixtures for a formerly masked stamp and an older backlink. |
| LD-S1 | Resolved | §§10.1, 10.4 and 14 put file, pipe, fetch and conversion work before locking, check the plan again under the lock, and keep publication and original cleanup locked. The stalled-input regression crosses processes. |
| LD-S2 | Partly resolved, should fix | §§8.5, 9 and 14 carry total and offset to the fitter and cover a budget-shortened page with stable diagnostic counts. Clamp the stated `More` formula to `max(0, Total - Offset - Shown)` and emit no continuation when it is zero. With 12 matches, offset 13 and zero shown, the sample formula yields −1 for the successful empty page required by spec §6.1. Cover that case in the paging fixture. |
| LD-S3 | Resolved | §§13 and 14 specify `cli.Run(ctx, Env)`, an invocation-scoped clock and zone, two-zone tests in one process, and built-binary tests for checkout selection, stdin and both delete-preview streams. |
| LD-S4 | Resolved | §§10.3–10.4, 12 and 14 anchor the settings file and lock to the brain. Setters refresh and replace the file while locked. The fixtures cover an external or unreachable personal folder and concurrent setters. |
| Semantic seam | Resolved | §§8.5–8.7 preserve shared eligibility and rendering and describe stable item identity, stored-body code-point span, evidence type and index readiness. |

**Verdict — Approve.** All three blockers are resolved. Carry the empty-page correction into the rendering design.
