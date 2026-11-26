# pbrain handover — 2026-11-21

Moving development from `thinkpax` (Linux, 16 cores, 62 GB) to a new machine. The factory team is the same, but every agent session starts fresh. Read this first, then `FACTORY.md`, the board and `specs/projects/pbrain/`.

**Tracker:** TODO.md is retired. Tasks are GitHub Issues on pkieltyka/pbrain managed with `todos`; board https://github.com/users/pkieltyka/projects/3 (Backlog, Todo, In progress, Done). The board is ordered: **Todo** (in order) #1 macOS fix (CI), #18 review the phase 28 rerun kit, #3 gemma and final-pin measurements, #4 gemma docs, #5 release v0.1.1. **Backlog:** #9 Mac runtime checks, #19 index-build memory, #6–#8 help texts, #20 tidy branches. **Done:** #2 (store test fix) and #10–#17, which record the main finished work and decisions. Nothing is In progress: the old sessions ended. Install `todos` from github.com/pkieltyka/todos (`make build`) and run `todos board use pkieltyka/3` in the checkout.

Only what is on GitHub carries over. This file lists everything that existed only on the old machine.

## Where things stand

- **All 29 implementation phases are done.** Phase 28 (measurement before release) has its Linux bge-small figures; EmbeddingGemma 2's full-size figures are still owed (see below).
- **All 41 spec scenarios** have executable testscript scripts in `cmd/pbrain/testdata/scenarios` (44 files).
- **Default embedding model:** `embeddinggemma-2-q8-0` since v0.5.0. `bge-small-en-v1.5` was the default when this handover was written and was removed on 2026-11-26 (issue #77), so the bge-small statements below are history. `embeddinggemma-2` is opt-in; `model = "none"` turns semantic search off. Owner decision 27 (revised), architecture §16.
- **Dependencies:** `github.com/pkieltyka/aikit v1.79.1` and `github.com/pkieltyka/goinfer v0.4.1`. Never require goinfer `v0.4.0`: the Go proxy permanently cached a wrong upstream tag under that number.
- **Release pipeline:** `.github/workflows/release.yml` + `.goreleaser.yaml`. A pushed `v*` tag builds Linux and macOS (amd64, arm64) into a **draft** GitHub release.

## STOP-THE-LINE: CI is red — fix before anything else

> **Update 2026-11-21 (owner):** the project targets Linux for now. The macOS CI test jobs are commented out in `.github/workflows/ci.yml`, and the macOS work below (#1, #9) is on the Backlog. The integration rule is now "green on Linux" (`FACTORY.md`). The rest of this section is history.

GitHub Actions has failed on every master push since `9e66477` (about 12:16 UTC on 2026-11-21). The team ran tests only locally on Linux and nobody watched Actions.

- **`v0.1.0` is tagged but has no release.** The release workflow failed its test step, so nothing was published. The fixes ship as **`v0.1.1`**. (Owner may choose to delete the `v0.1.0` tag.)
- **Rule (architecture §14):** a change counts as merged only when its Actions run is green on Linux **and** macOS, and a release is tagged only from such a commit. Check with `gh run list -R pkieltyka/pbrain` after every push.

Open failures (run `44935115413`):

| | Failure | Status | Owner (old team) |
|---|---|---|---|
| A | macOS: `model_path` and tests open paths behind the `/var` → `/private/var` link with the strict `OpenRoot` | **Fixed** on branch `peter/lead-dev-2` (head `9f1ff90`); macOS CI run 44936780814 confirms the failures are gone | lead-dev-2 |
| B | macOS: APFS rejects non-UTF-8 names and normalises `café` | **Fixed** on `peter/lead-dev-2`: those cases skip by name, naming the Mac backlog (probe conditions `[badnames]`, `[casedistinct]`, `[nfdistinct]`) | lead-dev-2 |
| C | macOS: scenario 14's second writer fails after 0.6–1.6 s: `lock …/docs/brain/.pbrain/lock: no such file or directory`, i.e. `openat(O_RDWR\|O_CREAT\|O_NOFOLLOW)` in the open `.pbrain` dir handle returns ENOENT. pbrain never removes `.pbrain`. Suspects: the test's barrier lock wrapper (`scenarioFiles.Lock`, fds 3/4 via ExtraFiles) on darwin, or APFS `openat` behaviour under `-race`. macOS race also fails `TestConcurrentSettingsWriters` (audience_test.go:237), maybe the same cause. | **Open.** Read `9f1ff90`'s macOS logs, fix, then review once Linux and macOS are both green | lead-dev-2 |
| D | Linux race/release: store `TestMutationFollowsScannedIdentity` (test-fixture flaw: ext4 reuses the freed inode) | **Fixed and merged** (`65a3bac`); Linux CI jobs green on its branch | architect-2 |

GitHub's `macos-latest` runner is a real Mac: push a branch and CI runs on it.

## Phase 28 measurements still owed

> **Update 2026-11-21:** the bge-small final-pin warm check and a full rerun are done on the new machine (Ryzen 9 7950X, 4 cores): first index 1 h 18 m, warm `search --semantic` 291–326 ms (#3, `internal/cli/README.measure.md`).

> **Update 2026-11-21 (owner):** EmbeddingGemma 2 is on hold until the owner clears it (goinfer is being made faster for it elsewhere). Its full-size run and docs delta (#3 gemma part, #4) are on the Backlog; bge-small is the only model tested or measured. Items below that concern gemma are deferred.

Everything measured so far is committed in `internal/cli/README.measure.md`, `internal/semantic/calibrate/README.md` and the README speed table.

- **bge-small (default), done:** 5,000 items / 50 MB on 4 Linux CPUs: first index 1 h 41 m (7.99 passages/s, peak 1,351 MiB); one-note reindex 0.77 s; `search --semantic` 302–333 ms warm (298–335 ms with `--bm25`), up to 549 MiB.
- **EmbeddingGemma 2, owed:** a full-size build was paused at 545 items / 5,251 passages after 1 h 38 m (0.90 passages/s, projected ~15 h total, peak 1.97 GiB), then resumed on the old machine. **That run dies with the old machine.** On the new machine, rerun the full-size gemma build from scratch with `PBRAIN_MEASURE=1` (see `internal/cli/README.measure.md`); a faster machine will give different absolute numbers, so record the hardware.
- **Final-pin warm check, owed:** warm `search --semantic` on the release binary built from current master (aikit v1.79.1, goinfer v0.4.1), on both models' retained indexes.
- **Rerun kit:** branch `peter/lead-dev` (`f285545`), `tools/phase28/README.md`: a portable runner (`run-linux.sh init|provision|full|warm|resume`), model provisioning and gemma packing, progress sampler, and the bge and partial gemma raw evidence. Not yet reviewed or on master. Run full gemma from scratch on the new machine; the old checkpoint and cache are not in Git. Use the v0.1.1 candidate commit for the warm check.
- **When both land:** the architect replaces the remaining EmbeddingGemma projections (spec §16, architecture §8.7/§17, semantic.md, the semantic skill topic, the README).

Model files are not in the repo. `pbrain sync` downloads them from Hugging Face at pinned revisions (bge-small ~135 MB; EmbeddingGemma 2 source ~1.52 GB, derived into a 552 MB pack). The adapter tests take the files via `PBRAIN_TEST_BGE_MODEL` and the gemma equivalents (see `internal/embed/*/testdata/README.md`).

## Backlog (on the board)

- **Mac backlog** (no Mac host yet; GitHub's macOS runner covers functional checks only): rule 3 (load + first query < 600 ms) for both models; `search --semantic` at 5,000 items; install of the release binary from a tag; scenario 41's terminal (pty) checks; composed vs decomposed accented ids on APFS (spec 2.3 compares without normalisation).
- Help-text observations from the final pass (shared `--tag` description; no defaults shown; `skill --help` lists unused globals).

## Old-machine-only state (lost on the move)

- **Member branches** `peter/architect-2`, `peter/lead-dev`, `peter/lead-dev-2`: never pushed, but **all their reviewed work is already on master** (cherry-picked). Nothing to recover unless a member reports unpushed WIP (they were asked to push before the move).
- **Running jobs:** `pbrain-phase28-gemma-resume.service` and `pbrain-phase28-release-warm.service` (systemd user units); logs under `/tmp/pbrain-phase28-release.Oq1j8a/`.
- **Scratch** under `/tmp`: model files, calibration reports (`/tmp/pbrain-gemma2-gates-IIB5V6xp/`), review archives. The committed READMEs record the results and their SHA-256s.
- **`factory.toml`** is the owner's formation file and is git-ignored: copy it over, or start from `factory.toml.sample`.
- **Operator memory** lives in `~/.claude/projects/<project>/memory/` on the old machine (three notes, all folded into this file: heavy-check limits, the sandbox/signing/go.work gotchas, and watch GitHub CI after every push).

## Forks

- **goinfer** (`github.com/pkieltyka/goinfer`): main `e33aa3893` = `v0.4.1` (upstream synced to `36dbfbaf1` + fork docs). Fork tags: `v0.1.0`, `v0.2.0`, `v0.3.0` (inherited from upstream, not ours), `v0.4.0` (unusable), `v0.4.1`.
- **aikit** (`github.com/pkieltyka/aikit`): main `0bbc048` = `v1.79.1`.
- **Tag rule** (architecture §17): tag only commits on our own main, under a number neither repository has used, and check the tag on the proxy before pbrain requires it. In local clones, set `git config remote.upstream.tagOpt --no-tags` so upstream tags never land locally — that is how the bad goinfer `v0.4.0` happened.

## Setting up the new machine

1. Go 1.27.3 or later, `gh` logged in, an SSH key that can push to `pkieltyka/*` (tags are SSH-signed; set `gpg.format ssh` and `user.signingkey`).
2. Clone `pbrain`, `goinfer` and `aikit` under `~/Dev/pkieltyka/`; in each fork add `upstream` with `tagOpt --no-tags`.
3. Copy `factory.toml`, then `/factory up` from the operator pane (needs herdr). Members get fresh sessions and read `FACTORY.md`.
4. `go test -p 2 ./...` should pass locally (heavy checks: `-p 2`, race only on changed packages, one fuzz target at a time).
5. First task for the new team: the CI table above, then the owed measurements, then tag `v0.1.1`.

## Environment gotchas (from the old machine)

- **Heavy checks:** `go test -p 2`; `-race` only on changed packages plus one final full run; one fuzz target at a time with `-parallel 2` and a short `-fuzztime`; the four cross-builds in sequence. Parallel heavy runs once got every agent session OOM-killed on a 62 GB machine.
- **pbrain sync downloads** dial Hugging Face directly; inside Claude Code's sandbox they fail in ~30 ms with `fetch_failed`. Run that one step outside the sandbox.
- **Signed tags:** the global git config signs tags, so tests that run `git tag` (goinfer's cmd/gate) need `GIT_CONFIG_GLOBAL=/dev/null`.
- **Pushing from a sandbox** that blocks SSH: `GIT_CONFIG_GLOBAL=/dev/null git -c credential.helper='!gh auth git-credential' push https://github.com/pkieltyka/pbrain.git HEAD:refs/heads/<branch>`.
- **goinfer** builds its sub-modules through an untracked `go.work`: keep the workspace on for goinfer, off for pbrain. Point `GOINFER_TIMING_LOCK_FILE` and `XDG_CACHE_HOME` at scratch when home paths are read-only.

## Architect's open work and method

1. Final EmbeddingGemma figures, in one delta, once its record lands: functional spec §16; architecture §8.7 (the first-index paragraph) and §17 (default-model and EmbeddingGemma risks); semantic.md ("Costs to state wherever this model is offered"); the semantic skill topic and skill.md; the README's semantic section. If the gemma run is redone on the new machine, the bge-small figures in the same texts are from the old machine: redo them there too, or label them by machine.
2. With that delta: close architecture §8.6's measurement list against `internal/cli/README.measure.md`. The cost of counting lines on a scan stays unmeasured.
3. After the macOS fix (#1) lands: check it against architecture §10.3 and store.md "Named roots", and record in §17 which macOS cases skip.
4. Scenario 14's macOS lock failure (C): if the cause is in the lock design (store.md, `Files.Lock`, `Begin`'s 30 s wait), it needs a design answer, not a test change.

Method: the architect edits spec files in the main checkout's working tree and never commits; it reports each file's sha256, and the operator commits on a hash match. The shipped skill core must stay at 400 words or fewer, and the semantic topic must keep the four phrases its test checks. Read the whole function a sentence describes, and read the CI run, before reporting.

## Conventions the team settled

- Reviews name an exact commit range; the operator merges only approved ranges and verifies the push landed (`git status -sb` in sync with origin).
- No token-shaped literals in committed files (GitHub push protection rejects them): build test tokens at run time from parts, as `internal/secretscan`'s tests do.
- Tracker records live on the issues (comments with owner, range, verdict, next step); an issue closes only after confirmed integration with green Actions.
