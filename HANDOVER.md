# pbrain handover — 2026-11-21

Moving development from `thinkpax` (Linux, 16 cores, 62 GB) to a new machine. The factory team is the same, but every agent session starts fresh. Read this first, then `FACTORY.md`, the board and `specs/projects/pbrain/`.

**Tracker:** TODO.md is retired. Tasks are GitHub Issues on pkieltyka/pbrain managed with `todos`; board https://github.com/users/pkieltyka/projects/3 (Backlog, Todo, In progress, Done). Open: #1–#4 in progress (CI fixes, gemma measurement, docs), #5–#9 backlog (v0.1.1 release, help texts, Mac checks). Closed #10–#17 record the main finished work and decisions. Install `todos` from github.com/pkieltyka/todos (`make build`) and run `todos board use pkieltyka/3` in the checkout.

Only what is on GitHub carries over. This file lists everything that existed only on the old machine.

## Where things stand

- **All 29 implementation phases are done.** Phase 28 (measurement before release) has its Linux bge-small figures; EmbeddingGemma 2's full-size figures are still owed (see below).
- **All 41 spec scenarios** have executable testscript scripts in `cmd/pbrain/testdata/scenarios` (44 files).
- **Default embedding model:** `bge-small-en-v1.5`. `embeddinggemma-2` is opt-in; `model = "none"` turns semantic search off. Owner decision 27 (revised), architecture §16.
- **Dependencies:** `github.com/pkieltyka/aikit v1.79.1` and `github.com/pkieltyka/goinfer v0.4.1`. Never require goinfer `v0.4.0`: the Go proxy permanently cached a wrong upstream tag under that number.
- **Release pipeline:** `.github/workflows/release.yml` + `.goreleaser.yaml`. A pushed `v*` tag builds Linux and macOS (amd64, arm64) into a **draft** GitHub release.

## STOP-THE-LINE: CI is red — fix before anything else

GitHub Actions has failed on every master push since `9e66477` (about 12:16 UTC on 2026-11-21). The team ran tests only locally on Linux and nobody watched Actions.

- **`v0.1.0` is tagged but has no release.** The release workflow failed its test step, so nothing was published. The fixes ship as **`v0.1.1`**. (Owner may choose to delete the `v0.1.0` tag.)
- **Rule (architecture §14):** a change counts as merged only when its Actions run is green on Linux **and** macOS, and a release is tagged only from such a commit. Check with `gh run list -R pkieltyka/pbrain` after every push.

Open failures (run `44935115413`):

| | Failure | Status | Owner (old team) |
|---|---|---|---|
| A | macOS: `model_path` and tests open paths behind the `/var` → `/private/var` link with the strict `OpenRoot` | Design done (`47c8b42`, architecture §10.3, store.md "Named roots": new `sys.OpenNamed`); implementation pending | lead-dev-2 |
| B | macOS: APFS rejects non-UTF-8 names and normalises `café` — tests that need those names must skip on darwin, naming the Mac backlog | Pending | lead-dev-2 |
| C | macOS: scenario 14 loses the lock race ("another pbrain command holds this section's lock") | Cause not yet found; read the runner log | lead-dev-2 |
| D | Linux race/release: store `TestMutationFollowsScannedIdentity` (test-fixture flaw: ext4 reuses the freed inode) | Fixed on branch `peter/store-identity` (`b454a5a`, pushed), reviewed locally; merge once its Actions run passes the store test | architect-2 |

GitHub's `macos-latest` runner is a real Mac: push a branch and CI runs on it.

## Phase 28 measurements still owed

Everything measured so far is committed in `internal/cli/README.measure.md`, `internal/semantic/calibrate/README.md` and the README speed table.

- **bge-small (default), done:** 5,000 items / 50 MB on 4 Linux CPUs: first index 1 h 41 m (7.99 passages/s, peak 1,351 MiB); one-note reindex 0.77 s; `search --semantic` 302–333 ms warm (298–335 ms with `--bm25`), up to 549 MiB.
- **EmbeddingGemma 2, owed:** a full-size build was paused at 545 items / 5,251 passages after 1 h 38 m (0.90 passages/s, projected ~15 h total, peak 1.97 GiB), then resumed on the old machine. **That run dies with the old machine.** On the new machine, rerun the full-size gemma build from scratch with `PBRAIN_MEASURE=1` (see `internal/cli/README.measure.md`); a faster machine will give different absolute numbers, so record the hardware.
- **Final-pin warm check, owed:** warm `search --semantic` on the release binary built from current master (aikit v1.79.1, goinfer v0.4.1), on both models' retained indexes.
- **When both land:** the architect replaces the remaining EmbeddingGemma projections (spec §16, architecture §8.7/§17, semantic.md, the semantic skill topic, the README).

Model files are not in the repo. `pbrain index` downloads them from Hugging Face at pinned revisions (bge-small ~135 MB; EmbeddingGemma 2 source ~1.52 GB, derived into a 552 MB pack). The adapter tests take the files via `PBRAIN_TEST_BGE_MODEL` and the gemma equivalents (see `internal/embed/*/testdata/README.md`).

## Backlog (on the board)

- **Mac backlog** (no Mac host yet; GitHub's macOS runner covers functional checks only): rule 3 (load + first query < 600 ms) for both models; `search --semantic` at 5,000 items; install of the release binary from a tag; scenario 41's terminal (pty) checks; composed vs decomposed accented ids on APFS (spec 2.3 compares without normalisation).
- Help-text observations from the final pass (shared `--tag` description; no defaults shown; `skill --help` lists unused globals).

## Old-machine-only state (lost on the move)

- **Member branches** `peter/architect-2`, `peter/lead-dev`, `peter/lead-dev-2`: never pushed, but **all their reviewed work is already on master** (cherry-picked). Nothing to recover unless a member reports unpushed WIP (they were asked to push before the move).
- **Running jobs:** `pbrain-phase28-gemma-resume.service` and `pbrain-phase28-release-warm.service` (systemd user units); logs under `/tmp/pbrain-phase28-release.Oq1j8a/`.
- **Scratch** under `/tmp`: model files, calibration reports (`/tmp/pbrain-gemma2-gates-IIB5V6xp/`), review archives. The committed READMEs record the results and their SHA-256s.
- **`factory.toml`** is the owner's formation file and is git-ignored: copy it over, or start from `factory.toml.sample`.
- **Operator memory** lives in `~/.claude/projects/<project>/memory/` on the old machine (two notes: cap heavy checks at `go test -p 2`; watch GitHub CI after every push).

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

## Conventions the team settled

- Reviews name an exact commit range; the operator merges only approved ranges and verifies the push landed (`git status -sb` in sync with origin).
- No token-shaped literals in committed files (GitHub push protection rejects them): build test tokens at run time from parts, as `internal/secretscan`'s tests do.
- Tracker records live on the issues (comments with owner, range, verdict, next step); an issue closes only after confirmed integration with green Actions.
