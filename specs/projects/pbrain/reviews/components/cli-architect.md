# cli.md, BM25 amendment delta: review by @architect

Reviewed `components/cli.md` at sha256 56954f3ecf73e6a810b078bedc2d124022abfb49a62814820080b315822bb677 (base 23e90cbd), against the change list of message 290, functional spec 079b3d19 (7.1, 12, scenario 38) and architecture d791e15e (8.5, 8.7, 8.8, 13). Scope: the delta only. Standards: `design_review_standards.md`.

**Verdict: changes required.** One finding; everything else in the delta is right.

## Finding

1. **Moderate. How the flag is switched on outside a release is not designed.** The text says "a single release-time boolean in the command-table constructor" holds `--bm25` back, that tests exercise "a held-back command table and an enabled table", and that "calibration builds enable the candidate flag". It does not say how. `cli.Run(ctx, Env)` takes no such switch and `Env` has none, so as written:
   - the scenario-38 script, which runs commands through `cli.Run`, cannot turn the flag on in a release where it is held back;
   - the calibration tool is another package (`internal/semantic/calibrate`) and calls `cli.Run`; it cannot reach an unexported constructor, and the gate that decides whether the flag ships cannot be run on the build that would ship it.
   
   A coding agent would have to invent this (a build tag, a package variable, an `Env` field), and the three choices differ in what a release binary can be made to do.
   
   **Fix, with the shared-type change approved here** (I own that call): add
   
   ```go
   // Features are the parts of the command surface a release may hold back.
   type Features struct {
       BM25 bool // the partial tier and its --bm25 flag (architecture 8.8)
   }

   // Release is what this build ships. It is a constant of the release, never read from the environment.
   var Release = Features{}

   // RunWith is Run with an explicit feature set, for tests and the calibration tool.
   func RunWith(ctx context.Context, env Env, f Features) (exit int)
   ```
   
   with `Run(ctx, env)` defined as `RunWith(ctx, env, Release)`. The command table is built from `f`. Scenario scripts and the calibration tool call `RunWith` with `BM25: true`; `TestCLISearchModes` runs both values; one test asserts that `Run` on a held-back build treats `--bm25` as an unknown flag. `Env` stays as it is: a feature gate is not something a command takes from the outside. State also that `Release.BM25` is set to true only by the change that records the passed gate, with the fixture report's hash in its message.

## Checked and right

- **The flag.** `--bm25` on `search` only; `list` rejects it; independent of `--semantic`; copied into `search.Request.BM25`; no alias. A held-back build omits it from help and treats it as an unknown flag, exit 2, also beside `--semantic`. Matches spec 7.1 and 16.
- **Model-free.** With `--bm25` alone no cache, database or model is opened: spec scenario 38 ("no index, model or cache is read or written").
- **Both flags.** One source lifecycle (Start before the scan, Attach, one deferred Close); a not-ready semantic part still returns the exact and partial rows with the notice. Matches architecture 8.8.
- **The registry.** `embed.NewRegistry(gohugot.Entry())`, with `gemma2.Entry()` only once that adapter passes; `Derive` used only inside `embed.Pack`. Unlike the flag, a model under calibration needs no switch: the tool builds its own registry and passes it in `Env.Models`.
- **Tests.** `TestCLISearchModes` covers the four combinations, the order of the three groups, model-free execution with a failing semantic dependency, the missing-index fallback, and both tables.
- **Output ownership.** Marker and JSON fields are left to `render.md`; notice texts to `semantic.md`. Nothing is defined twice.

## Not in this delta

The TZ, help and configuration clarifications in this file came from @lead-dev-2's earlier review and were approved at 59b68914; I did not review them again.

## Re-check

Reviewed `components/cli.md` at sha256 91a9150693d62784eea017aea4af35a77d86edb905307e1e0e76bf5bcc10f00a against the finding above and architecture aed01c87, section 13. The delta from 56954f3e is three places and nothing else.

- **Finding 1: resolved.** `Features`, `Release` and `RunWith` are declared with the signatures of architecture 13; `Run` delegates to `RunWith(ctx, env, Release)`; the command table is built from `f` and registers `--bm25` only when `f.BM25` is true; `Release.BM25` becomes true only in the change that records a passed gate; scenario scripts and the calibration tool pass `Features{BM25: true}` and leave `Release` untouched. `TestCLISearchModes` runs both settings through `RunWith` and asserts that the production `Run` on a held-back build rejects `--bm25` as an unknown flag. The calibration sentence now names `cli.RunWith`.

**Verdict: approve.**

One follow-up on my side, not on this file: `components/semantic.md`, "The wider comparison", still says the arms run "through `cli.Run`"; it should say `cli.RunWith` for the arms that use `--bm25`. I correct it with my next pass.

## Re-check after the runtime decision

Reviewed `components/cli.md` at sha256 a0b68d1506b60fde631e490d21afea92335a57857f6d2c3b3a29e71ff7ccc4cd against the owner's decision of 2026-11-21 (bge-small on aikit's encoder, hugot dropped) and architecture 064c9b4f, sections 3 and 13. Against my copy of 56954f3e the file differs in the three places approved at 91a91506 and in the process-adapter paragraph, and nowhere else.

- **`envguard` is gone**, and no hugot, GoMLX or `gohugot` reference is left in the file. This matches architecture 3, where the package is removed.
- **The output rule is kept**: `cmd/pbrain` saves the process's real streams in `Env.Stdout` and `Env.Stderr` and points `os.Stdout` and `os.Stderr` at the null device before any command runs, as architecture 13 says.
- **The registry**: `Env.Models` comes from `embed.NewRegistry`, with the bge-small entry and `gemma2.Entry()` once that adapter passes its gates, and the entries are left to `semantic.md`. Right: nothing is defined twice.

**Verdict: approve.**

Not blocking, for the next time the file moves: the adapter package is `internal/embed/bge`, so the entry can be named `bge.Entry()`, as `gemma2.Entry()` is.

## Re-check after the output amendment

Reviewed `components/cli.md` at sha256 563240ef96a1b61d3794ea068f72dd9c141b1f45fb9a405fdd18101da2afee51 against functional spec d1ba2637 (sections 6.1, 6.2, 7.1, 7.2, scenarios 39 to 41) and architecture ed3682b8 (sections 9 and 13).

- **`--context`.** Registered on `search` only and in every build; absent is 0 in `search.Request.Context`; a decimal 1 to 3 is passed unchanged; an explicit 0, a missing, malformed or out-of-range value, and the flag on any other command are `usage` before discovery. Matches spec 7.1 ("any other value is a usage error") and architecture 13.
- **`--color`.** On `search` and on every form of `get`; with `--json` it is `usage` before discovery, with the diagnostic in JSON. The format is chosen once (`render.JSON`, `render.TextColor`, `render.Text`) and passed to `FitBody` for ordinary `get` and to `Fit` for search, history and outline, as architecture 9 now has both entry points. History and outline start with the item's id, which is the one thing colour marks there, so offering the flag on them is consistent with spec 6.2.
- **No terminal test for colour.** "Colour depends only on the flag", and the paragraph keeps the one existing test, of standard input for a body-less `add`, apart from it. The test compares a pipe with a pseudo-terminal, which is the case that matters.
- **Ownership.** The budget, the line ranges, the context blocks and where colour goes are left to the renderer; the path rule is not restated here. Nothing is defined twice.
- **Default.** Search: five results, 3,000 code points.
- **Tests.** `TestCLILocationContextAndColor` covers scenarios 39 to 41 from the command line: the offset round trip with `get` and the outline's file line, every `--context` value with text and JSON across the retrieval modes, the refusals, colour on each form and its refusals, and the tight-limit comparison with and without decoration.

**Verdict: approve.**

Not blocking, carried over: the registry sentence still says "the bge-small entry backed by aikit"; it can be `bge.Entry()` when the file next moves.
