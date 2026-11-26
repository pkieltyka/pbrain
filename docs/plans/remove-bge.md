# Remove bge-small-en-v1.5 and make Q8_0 the default (#77)

Owner's decision, 2026-11-26: remove `bge-small-en-v1.5` completely. The new default model is `embeddinggemma-2-q8-0` (operator's choice). The Google pack and UD-Q6_K_XL stay as opt-in rows and `"none"` stays. The change is breaking and ships as v0.5.0.

Based on master `707838a`. Everything under "What the code does today" was read from that commit. Nothing was run on a device.

## Decision

1. **Delete `internal/embed/bge` whole**: the adapter, the copied BERT encoder with its kernels, its licence files, the oracle and its test data. Nothing else in the module imports the BERT code.
2. **Move the release runtime audit first.** `TestRuntimeEnvAndOutput` and its tables live in `internal/embed/bge` but audit every package the release binary links, goinfer and its backends included. It moves to `internal/buildcheck` and loses only its BERT parts.
3. **`brain.DefaultModel = "embeddinggemma-2-q8-0"`.** No other code changes for the default: no code outside `internal/embed/gemma2` branches on a model name.
4. **A project that names the removed model keeps every other command working.** `pbrain.toml` is tracked and shared, so one checkout on v0.5.0 must not lose `search`, `get` or `add` because the file still names the old model. The existing unsupported-model path already does this. One message changes: `pbrain sync` says the model was removed, names the new default and says how to switch.
5. **A project with no `model` key and an index built by bge-small needs no migration code.** The index fingerprint no longer matches, so search reports `semantic_not_ready` with its lexical results and the next `pbrain sync` downloads Q8_0, builds a new index and removes the old one. Existing tests prove it.
6. **The per-user cache is left alone.** pbrain never prunes it. The release notes name the two folders a user may delete.
7. **The frozen evaluation fixture is not touched.** Two of its notes are about bge (`decisions/bge-aikit.md`, `decisions/bge-hugot-old.md`). They are content, their paths are in committed evidence, and `batch_fixture_test.go` names one of them.
8. **The archives keep the aikit notice and gain goinfer's.** goinfer is MIT, and its notice has to travel with copies of the software, which a binary that links it is. Its `internal/modelcore` is code copied from aikit and is linked into every pbrain build, with a `LICENSE` and `NOTICE` that are byte for byte the two files the archives carry today. So `licenses/aikit` stays, sourced from a committed folder, and `licenses/goinfer` is added with goinfer's `LICENSE`, `NOTICE` and `THIRD_PARTY_LICENSES.md`, which holds the texts for what goinfer copies or links (ggml, `x/text`, `x/sys`, gocudrv, purego). A test holds the copies to the pinned module.
9. **History stays history.** Measured bge figures in `internal/cli/README.measure.md`, the evidence folders, old release notes, old plans, research and reviews are not edited. Text that describes the present is.

## Rejected

- **Reject `bge-small-en-v1.5` when `pbrain.toml` is decoded.** That fails every command, lexical ones included, for a checkout that upgraded before the tracked file was edited.
- **A notice in place of the `sync` error.** `sync` cannot build anything for a model it does not have, and building with another model than the file names would break "every checkout builds its index with the same one".
- **Treat the old name as an alias of the new default.** Same objection, and the status line would name a model that is not in use.
- **A table of retired models.** One name is retired. A constant is enough.
- **Deleting the user's cached bge files.** The cache is shared by every project and every pbrain version on the machine; an older pbrain may still use them.
- **Copying the notices from the module cache at release time.** A hook would put unreviewed files into the archives. Committed copies with a test that compares them to the pinned module are reviewed once and cannot go stale.
- **Editing goinfer's `THIRD_PARTY_LICENSES.md` down to what pbrain links.** It also covers goinfer's own release binaries and a WebGPU backend pbrain does not build. A verbatim copy is checkable; an edited one is not.
- **The Google pack as default.** It loads faster on the CPU (rule 3: 361 ms against 587 ms on pax), but it has no Metal route, downloads 1.52 GB and needs 2.1 GB on disk during its first build.

## What the code does today (verified)

- **Registries.** `cmd/pbrain/main.go` (`releaseModels`), `internal/semantic/calibrate/main.go` and `tools/model-verity-pins/testdata/main.go` each build `embed.NewRegistry(bge.Entry(), gemma2.Entry(), gemma2.GGUFEntries()...)`. The only other importer of the package is `internal/semantic/build_window_test.go`.
- **Default.** `internal/brain/config.go:20`, used by `decodeProject`. `brain` keeps any name the file gives; it validates only the empty string and `"none"`.
- **Unsupported names.** `semantic.Build` (`build.go:66`) fails with `invalid_config: the model "X" is not supported; supported models: …`. `sync --status` prints `model: X, not supported by this pbrain` (`cli/semantic.go:330`). `search --semantic` returns the lexical results with `semantic_not_ready: this pbrain does not support the model pbrain.toml names: pbrain sync lists the supported names` (`cli/semantic.go:155`). `TestCLISemanticCommands` and `TestReleaseModelSelection` cover all three.
- **Another model's index.** `sync --status` marks it `built for another model`; search does not score it; `sync` replaces it. `TestBuildFingerprintChange` (`internal/semantic/build_test.go:482`) and `TestCLISemanticCommands` (the `test-model-b` rows) cover this.
- **Scenarios need no model.** `cmd/pbrain/scenario_fixtures_test.go` builds a fake embedder named `brain.DefaultModel` through `embedtest.New`, and scenario 35 reads the name from `$PBRAIN_DEFAULT_MODEL` at its start and names it literally in one later block. Hosted CI downloads no model and sets no `PBRAIN_TEST_*` model variable.
- **The audit.** `internal/embed/bge/audit_test.go`, `audit_dependencies_test.go` and `audit_sites_test.go` (652 lines) run `go list -deps` for every GoReleaser build and review goinfer, its CUDA, Vulkan and Metal modules, purego, pflag, yaml and the importer's libraries. They require the BERT packages to be linked (`audit_test.go:114`) and hold one BERT-only test, `TestRuntimeAuditIncludesVendoredBERT`.
- **Tests only bge has.** `TestRuntimeQuiet` (a real model in a child process prints nothing) exists for bge alone, though `components/semantic.md` says "for each real model". `TestAdapterContract`'s points are already made for Gemma by `TestGemmaSpecialTokensAndCut`, `TestGemmaBuildsFromBytes` and the cancel and close cases in `accel_test.go`.
- **Workers.** One passage a core, up to 32, is bge's own code (`bge.go`, `maxWorkers`). Gemma on the CPU runs one forward at a time and spreads it over the cores.
- **Release packaging.** `.goreleaser.yaml` copies `internal/embed/bge/bert/LICENSE` and `NOTICE` into `licenses/aikit` in both archive groups, and `.github/scripts/verify-release-archives.sh` compares them. No other notice is in the archives.
- **Licences (goinfer v0.6.9, from the module cache).** The root module and the CUDA, Vulkan and Metal modules are MIT, copyright Francis Townsend-Merino. The root `NOTICE` names the copied sources (`internal/modelcore` from aikit v1.79.0, the towers and `internal/gpudev` from aikit's modules) and the linked ones (`x/text` and `x/sys`, BSD-3-Clause; gocudrv, MIT; purego, Apache-2.0). `THIRD_PARTY_LICENSES.md` holds their full texts and says they "travel with a binary distribution". `internal/modelcore/LICENSE` and `NOTICE` equal `internal/embed/bge/bert/LICENSE` and `NOTICE` byte for byte, and `go list -deps ./cmd/pbrain` shows four `goinfer/internal/modelcore` packages linked.
- **Modules.** The BERT tokenizer imports `golang.org/x/text`, which `internal/item` also uses. `go.yaml.in/yaml/v3` is used by the audit and by `buildcheck`. `go mod tidy` should change nothing.

## What makes Q8_0 a costly default

None of these makes it a wrong choice against the other Gemma rows. They are what the documents must say, and two of them are not measured.

| Cost | Figure | Source |
| --- | --- | --- |
| Download and disk | 310 MB, one file (bge: 134 MB) | README |
| Check, load and one query (rule 3, target 600 ms) | On four Ryzen 9 7950X cores, four sets of five runs: medians 562.9, 586.6, 592.8 and 552.5 ms, and one sample of the twenty over the target, at 603.6 ms. 437.2 ms with a sealed (fs-verity) cache. 306.9 ms on an Apple M4 | README.measure #47, #51, #56, #59, #32 |
| `search --semantic` at 5,000 items (target 1 s) | 459–484 ms on an Apple M4. **Not measured on Linux.** The Google pack took 773–820 ms on four Ryzen cores, and Q8_0 loads about 230 ms slower than that pack there | README.measure #32, #22 |
| Memory at a search | **Not measured for Q8_0 at full size.** 1,035 MiB for one query on an empty brain; the Google pack peaked at 1,626–1,687 MiB at 5,000 items (bge: up to 549 MiB) | README.measure #51, #22 |
| First index on CPUs alone | 1.32 passages/s on four Ryzen cores, 1.11 on four M4 cores, 1.88 on 32 threads. About 10 hours for the 48,511 passages of the 5,000-item brain: a projection, never run (bge: 20 minutes on 32 threads) | README.measure #69, #56 |
| First index on a GPU | 8 m 43 s on an RTX PRO 6000 (CUDA fp16); 1 h 21 m on an Apple M4 (Metal); 7.52 passages/s on an RX 9060 XT (Vulkan, 219 passages) | README.measure #32, #72, #69 |

What follows from them:

1. **The 1 second search target is unproven for the default on Linux**, and by the two figures above it is near 1 second on four desktop cores. Rule 3 has a few milliseconds to spare there and one sample of twenty missed it, and no four-core laptop has run it. Step 7 measures both on pax before the tag (operator's decision). It is not a Mac run.
2. **A CPU-only first index of a large brain takes hours, and more cores barely help** (1.32 to 1.88 passages/s from 4 to 32 threads). bge's one-forward-a-core workers went with it. Workers for Gemma on the CPU would be the fix; that is a new piece of work and not part of this plan. It is in the Backlog as "CPU workers for Gemma sync".
3. **The default binary of `go install` is CPU-only**, and so are Darwin amd64 (built, never run), Linux arm64 without an NVIDIA card and every `_cpu` archive. The README has to send people to the GPU archives and build tags first.
4. **Every CPU sync of the default now prints `cpu_fallback`.** That is the existing rule for a model with a GPU path, and it is the right signal.
5. **A search's catch-up covers less.** It starts no passage after 2 seconds, and a 500-token passage takes about 0.75 s on four cores. With the Google pack it saved 15 to 19 of 20 short passages inside that bound (README.measure #22); it is not measured for Q8_0. Sections built with CUDA fp16 or Metal are not caught up at all, as today.
6. **Each search process holds 1 GiB or more.** Several agents searching at once multiply it.
7. **`PBRAIN_DEVICE` now applies to the default.** It was ignored for bge, which had no GPU path.
8. **Developer runs with a real model all need the owner's switch**, `PBRAIN_ALLOW_GEMMA2=1`. Hosted CI is not affected.

In its favour, beyond the owner's reason: it reads 2,048 tokens of a passage where bge read 512; on the fixture it finds 42 of 47 against 40 and leaves 30 of 30 unanswerable pages empty against 29; and it is the one row with a route on CPU, CUDA, Vulkan and Metal.

## Interfaces, files and messages

**`internal/brain/config.go`.** `const DefaultModel = "embeddinggemma-2-q8-0"`. `brain` cannot import the adapter, so the name stays a literal; `TestReleaseModelSelection` already fails if the release registry lacks it.

**`internal/semantic/build.go`, `Build`.** When `Registry.Lookup` misses:

```go
// retiredModel was the default until v0.5.0. Its name gets a message of its own.
const (
	retiredModel   = "bge-small-en-v1.5"
	retiredDefault = "embeddinggemma-2-q8-0" // cmd/pbrain's test holds this to brain.DefaultModel
)
```

For `retiredModel` the error is `invalid_config`, exit 1, with the message

```
the model "bge-small-en-v1.5" was removed in pbrain v0.5.0: delete semantic.model from pbrain.toml to use the default, embeddinggemma-2-q8-0, or set one of: <Registry.Names(), comma separated>; then run pbrain sync
```

Any other unknown name keeps today's message. `semantic` may not import `brain`, which is why the default's name is a second literal with a test holding the two together.

**No other product code changes.** `sync --status` and the `semantic_not_ready` notice keep their texts for the removed name.

| Project state after upgrading | `search --semantic` | `pbrain sync --status` | `pbrain sync` |
| --- | --- | --- | --- |
| No `model` key, bge index, Q8_0 not downloaded | lexical results, `semantic_not_ready: the model is not on this machine: run pbrain sync, or set model_path` | `model: embeddinggemma-2-q8-0, not on this machine`, each section `built for another model` | downloads 310 MB after `model_download`, builds the new index, removes the old one |
| No `model` key, no index | the same | the same, `0 of N items indexed` | the same |
| `model = "bge-small-en-v1.5"` | lexical results, `semantic_not_ready: this pbrain does not support the model pbrain.toml names: pbrain sync lists the supported names` | `model: bge-small-en-v1.5, not supported by this pbrain` | exit 1 with the message above; no model is downloaded and no index or vector is written. The cache folder itself may be created empty, as for any unsupported name today |
| `model` names a Gemma row | unchanged | unchanged | unchanged |
| `model = "none"` | unchanged | unchanged | unchanged |

Every other command is unaffected in all five rows.

## Steps

Each step is one reviewable change and leaves `go test ./...` passing on Linux and macOS.

**1. Move the runtime audit.** `git mv` the three `audit*_test.go` files from `internal/embed/bge` to `internal/buildcheck` as `runtime_audit_test.go`, `runtime_audit_dependencies_test.go` and `runtime_audit_sites_test.go`; change the package clause; replace the `filepath.Join("..", "..", "..")` root with `buildcheck`'s `root(t)`. No rule, site or allowance changes, and the BERT assertions stay for now. Proof: `go test ./internal/buildcheck -run 'TestRuntime'` audits the same seven graphs with the same counts as before the move (compare the `audited …` log lines).

**2. Switch the default and drop the row.** One change, because the default must be in the registry at every commit.
- `cmd/pbrain/main.go`, `internal/semantic/calibrate/main.go`, `tools/model-verity-pins/testdata/main.go`: the registry is `gemma2.Entry()` and `gemma2.GGUFEntries()`.
- `brain.DefaultModel` and the `Build` message as above.
- The audit: delete `bertPackage`, `bertKernels`, the two `checked[…] == 0` conditions on them and `TestRuntimeAuditIncludesVendoredBERT`. In `scanRuntimeGraph` a package of the main module is now always skipped (today it is skipped unless it is the BERT copy). Keep the check that no aikit code is linked.
- `internal/buildcheck/buildcheck_test.go`, `allowed`: remove `"embed/bge"` from the `cmd/pbrain` and `internal/semantic/calibrate` rows.
- Tests:
  - `TestReleaseModelSelection` (`cmd/pbrain/main_test.go`): the names are the three Gemma rows; the empty configuration reports `embeddinggemma-2-q8-0, not on this machine`; `semantic.model = "bge-small-en-v1.5"` reports `not supported by this pbrain` and the unsupported notice; and a new assertion, with a cache folder of its own, that `pbrain sync` with that file exits 1 with a message that starts `invalid_config: the model "bge-small-en-v1.5" was removed in pbrain v0.5.0` and contains `brain.DefaultModel`, and that the cache then holds no `models` and no `vectors` entry and the section no index. `runIndex` opens the cache with `create` before `Build` runs, so the empty cache folder may exist; that is today's behaviour for any unsupported name and is not changed. The test's existing check that status and search leave the cache folder absent stays, on its own folder.
  - `TestCLISemanticCommands` (`internal/cli/semantic_test.go`): beside the `nope` case, the removed name gives the removal message with the test registry's names.
  - `TestModelVerityPins` (`cmd/pbrain/verity_test.go`): drop the bge row.
  - `TestReleaseBinaryOutput` (`cmd/pbrain/scenario_binary_test.go`): accept the three Gemma names; expect a device notice for every one of them (`embedding_device`, or `cpu_fallback` when the section's device is `cpu`).
  - `cmd/pbrain/testdata/scenarios/35-index-readiness-and-repair.txtar`, lines 158 to 170: this block names the fake default model literally (`testmodel bge-small-en-v1.5` and two `stdout '"model":"bge-small-en-v1\.5"'` checks), and the kit only has a model of that name while it is the default. Use `$PBRAIN_DEFAULT_MODEL` in all three, as lines 4 and 6 of the same script do, and reword its comment. (Added after approval: the first version of this plan missed these lines.)
  - `internal/brain/brain_test.go`: the two literal uses of the old name become `embeddinggemma-2`, and the row that expects `DefaultModel` from an explicit name uses `embeddinggemma-2-q8-0`.
  - `internal/semantic/build_window_test.go`: delete `TestBuildWorkersSameDatabase` (it tests bge's workers on the real model) and the import. The window test above it uses a fake and stays.
  - `internal/cli/measure_model_test.go`, `measure_test.go`: the default arm becomes Q8_0, read from `PBRAIN_TEST_GEMMA2_Q8` under `embedtest.SkipGemma2`; the Google arm stays on `PBRAIN_TEST_GEMMA2_PACK`; setting both is still an error. `PBRAIN_TEST_BGE_MODEL` is gone from the module.
- Proof: the whole suite on Linux and macOS, and `git grep -n 'embed/bge' -- '*.go'` shows only the package itself.

**3. Delete the package and its packaging.**
- `git rm -r internal/embed/bge` (60 files, once the audit and the two notice files below have moved).
- `buildcheck_test.go`, `allowed`: remove the three `internal/embed/bge…` rows.
- Notices. Before the `git rm`, `git mv` `internal/embed/bge/bert/LICENSE` and `NOTICE` to a new top-level `licenses/aikit/`. Add `licenses/goinfer/LICENSE`, `NOTICE` and `THIRD_PARTY_LICENSES.md`, copied unchanged from the pinned module. In `.goreleaser.yaml`, both archive groups take `licenses/aikit/*` and `licenses/goinfer/*` at those paths in place of the four `bert` entries. `.github/scripts/verify-release-archives.sh` compares every file under the repository's `licenses/` with the archive's copy, in place of its `LICENSE`/`NOTICE` loop.
- `TestThirdPartyNotices` (new, `internal/buildcheck`): with the module folder from `go list -m -f '{{.Dir}}' github.com/pkieltyka/goinfer`, each file of `licenses/goinfer` equals the module's file of that name, each file of `licenses/aikit` equals the one in `internal/modelcore`, and neither folder holds anything else. A pin bump that changes a notice fails here until the copy is refreshed.
- `go mod tidy`; record the diff, expected empty.
- Add `TestGemmaRuntimeQuiet` to `internal/embed/gemma2/gguf_real_test.go`, ported from bge's `TestRuntimeQuiet`: with `PBRAIN_TEST_GEMMA2_Q8` set, a child process opens the verified Q8_0 file, embeds a normal query, one holding `<eos>` and one over-long text, closes, and must have written nothing to either stream. Skipped without the file, like its neighbours.
- Proof: the suite; `goreleaser` snapshot and `verify-release-archives.sh` in CI; `TestAikitModuleRemoved` still passes; each of the seven snapshot archives lists `licenses/aikit/{LICENSE,NOTICE}` and `licenses/goinfer/{LICENSE,NOTICE,THIRD_PARTY_LICENSES.md}`; `git grep -il bge -- '*.go'` lists only `batch_fixture_test.go` (a fixture file name).

**4. Tools, text only.** `tools/phase28` measures a baseline source tree it copies, so its `bge` choice still works for a baseline up to v0.4.7: leave `run-linux.sh` and `index-state.py` as they are, and say in `tools/phase28/README.md` that the bge commands need such a baseline. `tools/model-verity-pins/README.md` and `tools/sync-trace/README.md` lose their bge lines.

**5. Specs and product text.** @architect writes this while @lead-dev does steps 1 to 4 (operator's decision), as with the #9 documents: uncommitted in the main checkout, hashes to @operator. The rule for every file: a sentence that says bge is supported, is the default or is how something works today is removed or rewritten; a measured figure stays where it is dated history.
- `functional_spec.md`. 9.4: the default's name; the selector no longer "does not apply" to the default (keep the rule for a model with no GPU path, and say no release model is one); drop "the default model on the processor … prints neither" and "*On the processor* the default model uses every core"; the status example. 10: the `pbrain.toml` comment and the list of models, with Q8_0 as the no-key default and its costs, the Google pack and Q6 as opt-in, and one line for the removed name. 16: "Platforms" and "Speed" restated for the new default, with bge's figures kept as "measured with the model removed in v0.5.0"; drop "that cost is why it is … not the default". Scenario 35 unchanged.
- `architecture.md`. Section 3: remove the three `embed/bge` rows. 8.7: three model rows, not four; the bge rows of the rule table, "bge-small on the internal BERT encoder", the hugot fallback and the aikit half of "Ours, and diverged" become one dated paragraph of history; windows, "Batches", "Sameness" and the speed budget restated without bge. Dependencies: remove the BERT port and its `x/text` note. Decision 15 gains "bge-small removed 2026-11-26". Decision 27 is revised again: the default is `embeddinggemma-2-q8-0`, with the owner's reason and the costs. Section 17: "The default model is the small one" becomes the costs table above with its two unmeasured rows; the cutoff and catch-up points lose their bge halves.
- `components/semantic.md`: "Which model" (`brain.DefaultModel`); delete "The bge-small adapter" and the #62 port paragraph; the costs paragraph that says why Gemma "is not the default"; "Dependencies"; the test plan (remove the `TestBge*` rows, move `TestRuntimeEnvAndOutput` to `buildcheck`, add `TestGemmaRuntimeQuiet`). `components/brain.md`, `cli.md` and `skill.md`: one line each.
- `internal/skill/topics/semantic.md` and its copy in `components/skill.md`: the download is 310 MB for the default; a first index on CPUs alone can take many hours and a GPU build is the intended path. Update any copy the skill's tests hold.
- `README.md`: install section (drop "The default bge-small model always syncs on the CPU without a notice"; recommend the GPU archives and tags before `go install`); "Semantic search" rewritten around the default with the costs above; the model table without the bge row and with Q8_0 marked default; "Measured speed" with Q8_0 rows in place of the BGE rows, unmeasured cells saying so.
- `internal/semantic/calibrate/README.md`: bge's results under a heading that says the model was removed in v0.5.0. `internal/cli/README.measure.md`: one sentence in the opening that bge-small was removed in v0.5.0 and its sections are history; nothing else. `HANDOVER.md`: the "Default embedding model" bullet. `implementation_plan.md`: phases 24 and 25 get "removed in v0.5.0 (2026-11-26)"; phase 28 and "What can be released when" are restated.
- `docs/release-notes/next.md` (reviewed as a release gate), in this order: the removal and the new default as a breaking change; what each kind of project sees and does (the table above, in words); the costs, with the two unmeasured figures called unmeasured; the recommendation to use a GPU build; that the cache keeps `models/bge-small-en-v1.5-*` and `vectors/6bd9989750090942b9ff6d86f27ea8a39e255457d0c01659991f87280e181b9b` until the user deletes them; that the archives now carry goinfer's notices beside aikit's. @lead-dev confirms the two folder names against a v0.4.7 cache before they are printed.

**6. Release gates** (operator's list): the approved plan, the reviewed implementation, exact-head CI green on Linux and macOS, the reviewed release notes.

**7. Before the tag: Q8_0 at full size on pax.** Required (operator's decision). @lead-dev runs it after steps 1 to 4 are reviewed, on a quiet host under the timing lock. No Mac is involved.
- **Binaries.** The two Linux amd64 release binaries of the candidate head, taken from a GoReleaser snapshot: the GPU archive's (tags `cuda,vulkan,eg2_combined,eg2_fp16`) and the `_cpu` archive's (no tags).
- **Index.** The complete Q8_0 index of the retained #22 corpus: 4,500 shared and 500 personal notes, 48,511 passages, in the profile the release writes by default on the RTX, CUDA fp16. A retained index is reused only if `pbrain --personal sync --status` with the candidate's GPU binary reports every item indexed; otherwise that binary builds it once on the RTX with `pbrain --personal sync`, about 9 minutes, outside the timings. Record each section's manifest fingerprint.
- **No timed command builds or changes the index.** Before timing, `sync --status` with each of the two binaries must report both sections complete and neither `built for another model`. If the CPU-only binary does not accept the index, stop and report; do not rebuild for it.
- **Timing, for each binary.** The four queries of #32 (shared exact, shared paraphrase, personal exact, personal paraphrase) as `pbrain search --semantic --json`, with `GOMAXPROCS=4` on CPUs 8–11: one untimed priming run, then five fresh processes a query. No search may print `semantic_not_ready` or `semantic_partial`. Record the load and GPU preflight as #32 did.
- **Recorded for each binary and query.** The five samples, the median, the range, how many were under 1 second, and the peak RSS as the maximum over the five.
- **Where it goes.** A section in `internal/cli/README.measure.md` with the evidence under `tools/phase28/evidence/`, and the figures to @architect for the cells of the README, functional spec 16 and architecture 17 that say "not measured on Linux". A median over the 1 second target is published as measured and reported to @operator before the tag.

## Risks and gaps

- **Two headline figures of the new default are unmeasured on Linux** (search time and memory at full size), and the search time may sit at its target. Step 7 measures both before the tag.
- **The 10 hour CPU figure is a projection** from a 219-passage sync.
- **The moved audit is the only guard on goinfer's environment reads and output.** Step 1 is a pure move so that a mistake shows as a changed count.
- **`TestGemmaRuntimeQuiet` runs only where the owner's switch and the model file are present.** Hosted CI never runs it, as it never ran bge's.
- **A machine with both pbrain v0.4.x and v0.5.0 on one checkout** and no `model` key rebuilds the section index each time the other version syncs. Shared vectors come back from the cache; personal ones are embedded again. Naming a Gemma row in `pbrain.toml` ends it.
- **The retired-model message holds the default's name as a literal.** `TestReleaseModelSelection` fails if it drifts from `brain.DefaultModel`.
- **The other linked modules still have no notice in the archives.** About twenty are linked besides goinfer and what it brings: modernc's SQLite and its libraries, toml, pflag, yaml, `x/net`, go-readability, html-to-markdown and their dependencies. Their BSD, MIT and Apache licences ask for the same thing. This is how every release so far has shipped, and this change does not make it worse; goinfer's file covers only goinfer's own dependencies. Closing it needs a generated listing for pbrain's release graphs, which is its own piece of work. It does not gate v0.5.0 (operator's decision) and is in the Backlog, linked to #77.
- **This reads licence texts; it is not legal advice.**
