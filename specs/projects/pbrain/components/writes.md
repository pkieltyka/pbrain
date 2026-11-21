---
status: complete
---

# Component: Item write commands and secret notices

## Purpose and scope

The write handlers orchestrate `add`, `verify` and `delete`. Imports reuse the new-item transaction after the importer prepares content. The handlers decide the view, validate writes under the section lock, report commit outcomes, and keep private material out of diagnostics. `secretscan` supplies the same warning rules to writes and `check`. CLI owns flag parsing and final serialization. Store owns guarded I/O and mutation commit points. Model work belongs to `pbrain sync`. [Architecture §§7.3, 10 and 13](../architecture.md) · [Functional spec §§5.2, 6.3 and 8](../functional_spec.md)

## Interfaces

The external entry is `cli.Run(ctx context.Context, env Env) (exit int)`. Add, verify and delete are handlers inside `cli`, with these integration types:

```go
package cli

type itemWrite struct {
    Author, StaleAfter string
    Tags, Replaces, Relates []string
    Explicit audience.Destination
}
type addRequest struct {
    itemWrite
    Kind item.Kind
    Title, Source, Decided, Verified string
    Body, BodyFile *string // nil means absent
}
type verifyRequest struct { ID, By string }
type deleteRequest struct { ID string; Yes bool }
type writeContext struct {
    Brain   *brain.Brain
    View    brain.View
    Now     time.Time
    Rand    io.Reader
    Stdin   io.Reader
    IsTTY   bool
    Notice  func(errs.Notice)
}
type writeResult struct {
    ID, Path string
    Section item.Section
}
type deleteResult struct {
    Item writeResult
    Deleted bool
    CurrentAgain, Dangling []string
    Original string // empty for noncopied items, otherwise removed, kept or left
}

func runAdd(ctx context.Context, wc writeContext, req addRequest) (writeResult, error)
func runVerify(ctx context.Context, wc writeContext, req verifyRequest) (writeResult, error)
func runDelete(ctx context.Context, wc writeContext, req deleteRequest) (deleteResult, error)
func publishNew(ctx context.Context, wc writeContext, input newItemInput) (writeResult, error)
```

`newItemInput` contains the pre-acquisition `audience.Request`, its selected `audience.Decision`, the narrow `store.WritePlan`, validated `item.Draft`, UTF-8 body bytes and, for a copied import, `*{Digest [32]byte; Bytes []byte}` as a private struct. A nonnil original can have an empty byte slice. The request and decision are stored so `publishNew` can recompute the same plan under the lock. `writeContext.IsTTY` is resolved at the CLI edge. The production stdin adapter supplies terminal status and in-process scripted input can be treated as non-terminal.

Handlers return `*errs.Error` for modeled failures. These include `usage` for bad argument combinations, `invalid` for bad values or link constraints, `not_found` for a missing item or refused input, `ambiguous_id`, `duplicate`, `personal_unavailable`, `invalid_config`, `conflict`, `too_large`, `unsupported_edit`, and filesystem or Git failures. Delete without confirmation returns a populated preview and `confirm_required` with exit 1. Other pre-commit errors return a zero result. Post-commit failure goes through `Notice` with code `after_write`, and still returns a success result.

```go
package secretscan

type Pattern string // pem_private_key, aws_access_key, github_token, slack_token
type Field struct { Name string; Value string }
type Match struct { Field string; Line int; Pattern Pattern }

func Scan(fields []Field) []Match
func Sensitive(value string) bool
```

`Scan` has no I/O or errors. It returns metadata without the matched text. `Line` is the one-based line within the logical field, with each entry of a list represented on its own line. Results follow supplied field order, then line and pattern. Multiple instances of one pattern on the same field line produce one match.

## Internal design

### View, preparation and transaction

1. Validate flags and acquire a guarded `Brain`. Read `Brain.AudienceRules` for every item mutation, including one with explicit audience. For a new item, decide from the explicit flag, matching rule or normal default in `audience.Decide`. `Lookup` is the destination's view, including shared for a personal write. `Disclose` is the caller's view. For an existing item both views are the caller's, and `Dest` is the section of the uniquely found file. Require the selected lookup view to be usable; choosing personal by rule has the same availability requirement as a flag.
2. Emit `destination` before any input access for every import and for an add routed by an audience rule. Name the section, its display folder, the choosing rule when present and what reaches the repository. Import adds the acquisition-specific details from the importer design. Prepare files via `store.OpenInput`, read bounded stdin, or fetch and convert entirely outside the section lock. Linked imports do not acquire input bytes. Complete all local validation now that does not require an item catalog. Use one captured invocation instant for `created`, `fetched` and ID generation. No clock call takes place inside the transaction.
3. Under `store.Begin` on `Dest`, use `brain.ReadLocal` through the brain handle, then recompute the plan. If the destination, selecting rule or personal root changed while preparation was in progress, return `conflict` before storing bytes. Compare effective normalized configured paths by their identity. For verify or delete, rescan the flag-selected view and resolve the exact same section and path. Refuse a change in the resolved target. Verify also requires the original observed file version. Shared writes never populate the catalog with personal items.
4. Build a fresh graph from the locked catalog, then apply duplicate, link, standing and destination checks. For copied imports place and verify the original, then publish the complete item. For verify or delete use the existing-path primitive and its version check. Keep the lock through original cleanup, filesystem sync and post-commit error collection. Release it before returning to the CLI.

Every error or suggestion carrying an item reference is formatted at the CLI boundary from typed values after filtering by `Disclose`. The formatted line may mention the new item itself on success. This exception does not extend to previous items, other paths or suggestions from the lookup catalog. Shared fallback errors for private duplicates and rejected replacements name only the operation and reason. Never interpolate a preformatted graph or OS error if it could carry an undisclosed id, path or secret. The explicit personal view can print disclosed personal references with their audience marker. The early import notice describes the chosen section rather than any existing private item.

### Add and import

Add requires `--title`. It accepts decision, learning, observation or note, with note as default. Decision requires `--decided`, rejects `--verified` and starts with recorded status verified. Other first-party kinds reject `--decided` and start provisional unless an explicit `--verified` is valid. Validate author and verifier as one-line actors and apply author precedence via `Brain.ResolveAuthor`. Use `item.NormTag`, `item.ParseStaleAfter` and `item.Draft.Validate` for field constraints. The draft rejects oversized text fields and lists rather than truncating them. Default to an empty body for a terminal, otherwise read stdin until EOF, unless `--body` or `--body-file` supplied it. A body must be valid UTF-8 and no more than 512,000 code points. Cap stdin at 2,048,001 bytes before decoding, so every valid body can fit and oversized streams cannot force unbounded buffering. Empty input is valid.

The importer supplies `Prepared` text and original bytes. The write handler assigns author, status, created, tags, links and fetched to the draft; only copied imports receive a digest and fetched timestamp. A copied import is a source with origin file or URL. For these check existing current items of the destination section for a qualifying duplicate:

- A file import compares lowercase SHA-256 of its raw bytes with current origin-file items.
- A URL import or linked item compares the stored, as-requested URL string with current origin-URL and origin-link items. Keep query and fragment distinctions and never use an audience rule key to identify duplicates.
- A valid `--replaces` lifts the duplicate check only for the exact targets it names. Validate every named target first. If duplicates remain, fail with `duplicate`, listing the permitted candidates in deterministic order. A file import citing a URL never occupies that URL's duplicate key.

Use `graph.CheckNewReplaces` for existence, ambiguity, same-section replacement, standing and kind constraints, and whether the old item is current. When graph returns current successors for a refused replacement, filter and bound that list before formatting. Resolve each `--relates` id in the new item's origin view and refuse missing or ambiguous IDs. Personal items may relate to shared items.

Draw a fresh ID with `item.NewID` using every visible catalog ID, including replaced items, as occupied. Also reserve IDs in stored `replaces` links and legacy stamps within the destination section, so drawing a previously dangling ID cannot activate an incoming replacement or create a cycle. Call `CheckDestination` before any original is written. Build complete bytes with `item.Emit`; ordinary add emits the input unchanged as body. If the final link reports an occupied name, add that ID to the occupied set and try again while still locked, with a ceiling of 16 publication attempts. A final exhausted collision or random-source failure is `io`. The published payload is always the already validated draft, and each retry uses the same invocation time and body.

### Verify

Scan the caller-selected view and resolve the folded ID. Zero or multiple matches return `not_found` or `ambiguous_id`. Retain that first match's `item.Version`. After locking and refreshing, require the same section, path and version before checking standing or applying the already-verified shortcut. A new version at the same path returns `conflict` with its bytes unchanged. Reject a source or a replaced item with `invalid`. An already-verified item otherwise succeeds with the usual result and no mutation. Read the selected file by handle and verify its version again. Pass the bytes and validated `--by` actor to `item.EditVerify`. The actor defaults through the author rule. Refuse an unsupported byte layout with `unsupported_edit`. Secret warnings cover the actor about to be stored on a shared file. Call `ReplaceItem` with the first observed version so its last comparison still guards the original content. Write output names the selected file. `verify --personal` on a shared item edits under the shared lock.

### Delete

Probe once in the chosen view to resolve the exact item. `--yes` takes the item's section lock, rechecks the selection against the fresh catalog and recomputes everything. Preview uses that same calculation from the first catalog with no lock or file creation. Compute `graph.Effect` and retain the visible remaining items only. Sort each list by folded ID with path as the tie-breaker. `current again` reports surviving items that become current. `left dangling` reports links from surviving items that resolved to the target and become unresolved. An already-unresolved link produces no entry. Do not approximate either list by the graph's current reverse links.

For a copied source, validate the digest against exactly 64 lowercase hexadecimal digits and decode it to 32 bytes before any local-data access. Inspect every remaining item of the deleted item's section, including replaced items, for the same digest. Another reference means `kept`. Otherwise probe through the borrowed section root, opening `.pbrain` and `originals` with `Sub(create=false)` and inspecting the digest entry without following a link. Close the child handles. The probe reads no file bytes and never calls `Begin` in preview. A malformed digest, missing original or unsafe original is planned as `left`. A regular candidate is shown as `removed` under the preview heading, meaning it would be removed. Recompute under the confirmed lock. Commit the item unlink before any attempted original removal. On unlink failure nothing in originals is changed. If an original unlink fails, say `left` with `after_write`. If the original unlink succeeds and its directory sync fails, say `removed` and emit a sync notice. Never inspect or remove the other section's original.

Text and JSON follow spec §8.6. Both preview and success carry the same fact fields; only success says `deleted` in text and includes `"deleted":true` in JSON. Empty id lists are omitted. Cap each list at 10 ids with an exact count of the remaining visible ids. Preview prints its result on stdout, then `confirm_required` on stderr. A successful shared deletion emits `git_history` with “If the file was committed, commit the deletion. Git history still holds it.” Git access from this operation is read-only and uses the normal guard even for previews.

### Secret warnings and post-commit notices

Compile four Go regular expressions once. These are detection heuristics:

| Pattern | Shape |
|---|---|
| `pem_private_key` | `-----BEGIN (?:[A-Z0-9]+ )*PRIVATE KEY-----` |
| `aws_access_key` | `(?:AKIA\|ASIA)[A-Z0-9]{16}` |
| `github_token` | `gh[pousr]_[A-Za-z0-9]{36}` or `github_pat_[A-Za-z0-9_]{20,255}` |
| `slack_token` | `xox[baprs]-[A-Za-z0-9-]{10,255}` |

Run `FindAllStringIndex` on each LF-delimited line for these patterns without consuming adjacent delimiters. A token candidate passes only when its neighboring bytes, where present, are outside `[A-Za-z0-9_]`. Slack's boundary alphabet also includes hyphen. PEM matches need no additional boundary. Looking at neighbors in Go avoids unsupported regexp lookaround and finds two tokens separated by one punctuation byte. `Sensitive` uses this same predicate. Feed stored scalar values in emitter order and the body after them. For each list field join entries with LF so its line number is the one-based entry within that field. None of the diagnostics prints source lexemes.

On shared add or import scan every stored field and the body, including each tag and each link. On shared verify scan the changed actor. Use the same field projection for parsed items in `check`. Warn with `secret_suspected`, naming the field, line and pattern only. Under a declared tag map emit `undeclared_tag` for an undeclared tag on either audience, with just the field name when its value is sensitive. Apply the same redaction to `check`. Other write diagnostics also use sanitized fields and typed facts. The configured behavior is warning only.

After the actual item commit point, map store cleanup facts, lock-release errors and original-cleanup errors to fixed `after_write` text without filesystem paths. Emit the successful result regardless of those failures. Import original persistence occurs before the item commit, so a failure there still returns an error. Confirmed delete's result reflects the actual state of the original. No handler loads a model or updates a semantic index.

## Dependencies

CLI write handlers use `brain`, `store`, `item`, `graph`, `audience`, `importer`, `secretscan` and `errs`. `store` owns disk mutations and does not depend on CLI or graph. The existing read-only guard runs through `brain`. `secretscan` has only standard-library dependencies. `check` also consumes its typed matches.

## Test plan

| Test | Assertions and spec scenarios |
|---|---|
| `TestWriteViewsAndDisclosure` | A rule-routed private add/import validates links in the personal view but names no other personal item on either stream, in text and JSON. Tests duplicate, already-replaced and ambiguous cases, then repeats with explicit `--personal`. Spec 6, 7, 25–26. |
| `TestAddBodyAndFlags` | Terminal default, empty redirected stdin, valid multibyte boundary, invalid UTF-8, oversized stream, protected input, author precedence, and incompatible authority/body flags. Spec 6, 14 and 21. |
| `TestAudienceChangesBeforeCommit` | Pause during input, mutate the matched rule or personal path via a cooperating action, and resume. Refuses with `conflict` and no original or item. Another independent write completes while the input is stalled. Spec 24–26 and §8.1. |
| `TestNewLinksAndCurrentDuplicates` | Wrong kind/standing, ambiguous, non-current and cross-section replacements, personal `relates` to shared, and a candidate id named by a dangling replacement. File digest and URL duplicate keys differ. Replacing one of two linked duplicates is insufficient. Spec 10–14 and 18–19. |
| `TestVerifyExistingTargets` | Skip already verified, reject source and replaced, preserve CRLF/comments/unknown keys, and yield conflict on a changed version. Verify a shared target with personal view without touching the personal section. Spec 13, 16 and 21. |
| `TestVerifyKeepsFirstVersion` | Pause after first lookup, edit the body at the same path, then allow lock acquisition. Verify returns conflict even if the newer file is already verified, and leaves it alone. Repeat with a change after the locked check. Spec §8.1 and 16. |
| `TestDeletePreviewAndScope` | At preview no lock or dir is created, correct stream split, and confirmed facts recomputed. Includes masked legacy stamps, pre-existing dangling links, personal backlinks, ambiguous id, guard failure and digest isolation. Spec 11 and 29. |
| `TestDeletePreviewMissingOriginal` | Missing originals directory and a symlink at a digest each produce `left` with no creation, traversal or mutation. Confirmed outcome is based on actual removal. Spec 29. |
| `TestCommitAndCleanupOutcomes` | Pre-commit original write/unlink/rename failures return errors. Post-commit sync and original cleanup failures keep success and report the exact original state. Spec 15, 16 and 29. |
| `TestSecretAndTagNotices` | Synthetic example of each pattern in a field and body, line numbering, linked summary, shared verify actor, punctuation delimiters, adjacent tokens, embedded substrings, and an undeclared token-shaped tag. The same predicate governs `Scan`, `Sensitive` and tag redaction. No raw value on either diagnostic stream format; warn and write. Spec 28 and 32. |
