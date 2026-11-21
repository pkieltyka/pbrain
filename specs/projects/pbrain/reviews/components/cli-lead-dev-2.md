# Review: components/cli.md (@lead-dev-2)

Reviewed: `components/cli.md` at sha256 d1065a1aebe02360488d0af377bb9a5a95c32bbcbfb2c3609399a6b6476ca372 (round 1), against:
- functional spec 2b9cedaf: 6, 7, 8.1 to 8.6, 9, 10 and 15;
- architecture ed11b876: 6.1, 9, 10.1, 13, and the semantic lifecycle of 8.7 as @architect described it in his message of 2026-11-21;
- the seams of `search.md` 8eb11c57 and `render.md` 866e36f9.

Standards: `design_review_standards.md`.

**Verdict: changes required.** Moderate points 1 to 3 only:
- The command table, global-flag handling, the two-pass dispatch, the pre-scan for JSON mode, the single instant, stdin handling, write ordering and the exit and stream rules are right.
- Two pieces that only `cli` can own are missing: the notice texts for semantic search, and the order of the semantic lifecycle calls.

## Moderate

1. **Map each semantic reason to a notice with its fix.** Spec 7.1 says `semantic_not_ready` comes "with the reason and the fix". `search` returns `Result.Sem` (`Asked`, `Ready`, `Reason`, `Missing`) and prints nothing (`search.md` 6.1). So `cli` owns the text, and it should be a table, one row per `NotReady` value, for example:

   | Reason | Message (text form) |
   |---|---|
   | `NoModel` | `semantic search is off: pbrain.toml names no model (semantic = { model = "…" })` |
   | `UnsupportedModel` | `this pbrain does not support the model pbrain.toml names; run pbrain index to see the supported names` |
   | `ModelAbsent` | `the model is not on this machine: run pbrain index (it downloads once) or set model_path` |
   | `CacheExposed` | `the model cache lies inside a Git repository: set PBRAIN_CACHE outside it` |
   | `NoIndex` | `no index for this model: run pbrain index` (with `--personal` when the view is personal) |
   | `ModelFailed` | `the model could not be used; lexical results only` |
   
   The `semantic_partial` text is `<n> items are not in the index yet: run pbrain index`. Each message names no item, path or URL. Name the `index_progress` and `model_download` forms here too, or say which component owns them.
2. **Order the semantic lifecycle calls.** @architect's fix pass on 8.7 has `semantic.Start(ctx, opt, text, dirs)` before the scan, `Attach(secs)` after it, `Score` joined by `search.Run`, and `Close` deferred on every exit, cancelling and closing the embedder once. Only `cli` sequences these calls around `brain.Open`, `store.Scan` and `search.Run`.
   - Add the ordered steps for `search --semantic`: guard, then `Start`, then scan, then `Attach`, then `Run` (which calls `Score`), then fit, then print, with `Close` deferred.
   - Add the ordered steps for `index` and `index --status`.
   - Say that a `Start` failure becomes `Sem.Ready = false` and never an error exit.
   - Test that `Close` runs once on the error paths, a usage error after `Start` included.
   
   Align the names with `semantic.md` 39b7ce6c.
3. **The notice for skipped files.** Spec 6.3 says "files skipped by a read are reported as one notice with their count, pointing to `check`", and `render.md` leaves that to `cli`.
   - Name its code (none is listed in spec 6.3; propose one, e.g. `skipped_files`, to @architect for the code list).
   - Give its message: `<n> files skipped: run pbrain check`.
   - Say which commands emit it: every read that scans, and writes after their scan. The count is cut to the view, so a skipped personal file is not counted in the shared view.

## Mild

4. **`TZ` details.** An empty `TZ` is UTC in Go and POSIX. A leading colon (`TZ=:Europe/London`) is accepted by Go's own lookup, but `time.LoadLocation` refuses it. Also, a `CGO_ENABLED=0` binary on a minimal system with no zoneinfo cannot load any named zone. Consider importing `time/tzdata` (about 450 KB) so `TZ` always works, and test `TZ=""` and `TZ=:Europe/London`.
5. **Configured defaults out of range.** `pbrain.toml` `search.limit = 500` is outside 1 to 50. Say whether that is `invalid_config` (my preference, from `brain`'s loader) or clamped, and which component checks it.
6. **Audience paging defaults.** "Use list's settings for audience defaults" could mean `list`'s configured `pbrain.toml` values or its built-in 20 and 2,000. `audience.md` 58070db6 says limit 20 (1 to 200) and max-chars 2,000; say that configuration does not apply.
7. **Global flags that mean nothing.** Give the rule for `--personal` on `init`, `skill`, `--help` and `--version`: ignored or `usage`. Add one test row.
8. **Help text.** Say where each command's help text comes from (the command table) and that help output is not budgeted.

## Checked and fine

- **`Env`:** `Models` is added; stdin is nil for a terminal; there is one instant with a constant closure, and two zones can run concurrently; no process-global stream or clock is read.
- **Grammar:**
  - global flags before or after the command, and local flags among operands;
  - `--`;
  - `StringArray` for repeatable flags, keeping commas;
  - an absent value is distinct from an empty one;
  - pflag with `ContinueOnError`;
  - the JSON-mode pre-scan, which skips flag values so a literal `--json` in a body cannot switch the format.
- **Usage combinations:** both destination flags; both body flags; `--summary` without `--link-only`; `--source` on a URL import; `--history` with `--outline`; `--limit` on plain `get` and on `prime`; page flags on an audience edit; inverted windows. A missing title is `usage` and an empty one `invalid`, as `item.md` validates.
- **Dispatch:** `help`, `version`, `skill` and `init` run without `brain.Open`. Everything else is guarded, and `personal_unavailable` comes before any item read.
- **Writes:** the plan comes before any input; the destination notice before acquisition; stdin is bounded and read after the decision; `OpenInput` is the only file route; a rule-routed personal write widens lookup, not disclosure.
- **Exit and streams:**
  - 0, 1 and 2;
  - `check` exits 1 with its report;
  - the delete preview prints before `confirm_required`;
  - `after_write` keeps exit 0;
  - a stdout failure after a commit tells the caller to look the item up first;
  - one JSON object per stderr line.

---

# Recheck of the importer addendum (@lead-dev-2)

Reviewed: `components/importer.md` at sha256 02229ba40dbfcda9c196edb95443910467fcc99fa18ed56fcb55593375db3278, the "Streaming a pinned model file" section and `fetch.Client.Download`. Written here because it is part of the round with @architect-2's cli.md; the importer verdict file holds the import review.

**Verdict: approve.** `Download` follows the fetch's own rules:
- HTTPS at every hop, with the address checked and pinned per connection and the certificate checked for the current host;
- no proxy, credentials or automatic decoding;
- at most five redirects and 2xx only;
- an idle timeout over every phase instead of a 30-second total;
- the declared length checked, then a stream that never writes past the ceiling, with a one-byte overflow probe;
- `fetch_failed`, or `io` for the sink, with no URL or body in messages;
- the digest checked and the file published by `embed`, never by `Download`.

The test list covers a slow but progressing download longer than the import deadline, a stall, a downgrade, a cross-host redirect, oversize with no `Content-Length`, cancellation and a failing sink.

**Mild, for `semantic.md` rather than here:** with no Range resume, an interrupted 1.49 GB download (if EmbeddingGemma 2 ships as Google's multimodal file) starts again from zero. Spec 9.4's "an interrupted run loses little" is about embedding, so this is acceptable, but `semantic.md` should say so. Hugging Face serves model files through a cross-host CDN redirect with signed query strings, so the cross-host redirect test reflects the real path.

---

## Recheck of cli.md, sha256 59b689144f558ee60b4d23edf0b1d03d43efffd102ef2f28b59884970f889d2b

**Verdict: approve.** Every point is resolved.

1. **Notice texts.** `semantic.md` is the one source for the `NotReady` messages and fixes, the `semantic_not_ready`, `semantic_partial`, `model_download` and `index_progress` texts, and the build and status output. `cli` only serialises them, as the operator decided, and a test checks each reason against that table.
2. **The semantic lifecycle:**
   - `Start` with the joined query text before the scan, then `Close` deferred straight away and run once on every later failure;
   - `Attach` after the scan, then `Run`, which joins `Score`, then fit, then one notice from `Result.Sem`;
   - for a build, a `ScanFunc` scanned after the section lock is held, tested by pausing before the scan;
   - for status, one scan, no model, nothing created;
   - the cache path from `Env.Getenv` only, and `OpenCache` with the brain and personal roots forbidden;
   - `fetch.New(env.Net)` as the downloader, and `model_path` opened as `Options.ModelDir`;
   - a lexical-only search never opens the cache.
3. **`skipped_files`.**
   - It has its code and text, with `--personal` appended in a personal view.
   - It is counted once from the final disclosed scan, cut to the view (a rule-routed personal write counts shared files only), and emitted only when non-zero.
   - It covers reads, writes and `index`, and `check` never emits it.
   - `destination` and `git_history` stay with the write handlers.
4. to 8. **The mild points:**
   - `TZ` unset gives the system zone, `TZ=""` gives UTC, and a leading colon is accepted, with `time/tzdata` embedded;
   - an out-of-range project default is `invalid_config` from the brain loader;
   - audience uses the built-in 20 and 2,000;
   - `--personal` is accepted and ignored on help, version, init and skill;
   - help is generated from the command table and is unbudgeted.

**Addendum (2026-11-21), from @architect's consistency check.** This is mild and does not block, and I missed it in the recheck above. In `TestCLISemanticAndOutline`, "assert the section lock is already held" should name the **index lock** (`<section>/.pbrain/index/lock`, taken by `semantic.Build`), not the writers' section lock from `store.Begin` (`<section>/.pbrain/lock`). A build never takes the writers' lock, so a test written against the section lock would assert the opposite of the design. Fix it at the next edit of the file.

## Recheck of cli.md, sha256 84330ddc6e233c71853fe983dda758ebf77efeeec2ef129596415da1ed5df08b

**Verdict: approve.** I diffed it against 59b68914: the one changed line is in `TestCLISemanticAndOutline`. It now asserts that a paused build holds `<section>/.pbrain/index/lock` and that an item writer can still take `<section>/.pbrain/lock`. That closes the addendum above.

## Recheck of cli.md, sha256 23e90cbdde69ea064f52b5f7130d385416962e054b7832b76303288471529540

**Verdict: approve.** I diffed it against 84330ddc. The only change is one sentence in the process-adapter paragraph: `cmd/pbrain` imports `internal/envguard`, saves the real output streams in `Env.Stdout` and `Env.Stderr`, and points `os.Stdout` and `os.Stderr` at the null device before any command runs. That matches architecture 13 and `semantic.md` a427520c.

## Output amendment delta, sha256 563240ef96a1b61d3794ea068f72dd9c141b1f45fb9a405fdd18101da2afee51

**Verdict: approve.** Checked against functional spec d1ba2637 (6.1, 6.2's colour bullet, 7.1 Context, scenarios 39 to 41, 17) and architecture ed3682b8 (9's Format, 13):
- **Search defaults** are five results and 3,000 code points, as spec 6.1 now says.
- **`--context`** is registered on search only. Absent means 0 in `Request.Context`; an explicit value must be 1 to 3; 0, out-of-range, malformed or missing values, and use on any other command, give `usage` before discovery. This matches spec 7.1 ("any other value is a usage error").
- **`--color`** is registered on search and every get form. With `--json` it is `usage` in either flag order, with empty stdout. The format is chosen once (`render.JSON`, `TextColor`, `Text`) and passed to `Fit` and `FitBody`. Stdout is never probed, and the stdin check for body-less `add` is the only terminal check, as architecture 13 notes.
- **`TestCLILocationContextAndColor`** covers 39 to 41: the get round trip at the body offset with the outline's file line, the context range and rejections, decoration at a tight limit with the base page and continuation unchanged, and pipe against pseudo-terminal output.
