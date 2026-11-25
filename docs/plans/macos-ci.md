# macOS CI and runtime checks (#1, #9)

Based on master `76aea82`. Darwin arm64 is a release target since v0.4.6, yet no macOS job has run since 2026-11-21 (`a9ca52e`). This plan turns the macOS jobs back on, lands the one fix of #1 that master lacks, and says what #9's runtime checks need.

## Decision

1. Port fix C of #1 (the lock-file create race) to master by hand. A and B are already there.
2. Add two hosted macOS arm64 jobs to `ci.yml`, release and race, as on Linux, plus a job that runs the two Darwin arm64 archives GoReleaser built. No job does Metal GPU work.
3. Find the rest of the failures by running: 81 test files changed since the last macOS run (8,131 added lines), so the list cannot be read off the code.
4. For #9: port scenario 41's terminal checks to Darwin, test the accent rule on APFS in scenario 13, and take rule 3, the 5,000-item search and the install check from @lead-dev's M4 benchmark run.
5. When the macOS jobs are green on master, the integration rule becomes "green on Linux and macOS".

## Where #1 stands

| Part | State on master | Where |
| --- | --- | --- |
| A, named roots through links | Merged (`34b025c`, #21): `sys.OpenNamed`, `TestOpenNamedRoots` | `internal/sys/sys.go:115` |
| B, file-system traits | Merged (`8f86b1d`): `[badnames]`, `[casedistinct]`, `[nfdistinct]` | `cmd/pbrain/scenario_bytes_test.go:118` |
| C, lock-file create race | **Not merged.** `openLockFile` still opens with bare `O_CREAT` | `internal/sys/sys_unix.go:515` |
| Truthful lock message (`3a1e60f`) | Not merged. `Begin` calls every lock failure "another pbrain command holds this section's lock" | `internal/store/tx.go:66` |

C's cause, confirmed on the hosted runner: on macOS, concurrent `openat(O_CREAT)` calls for one missing file can return ENOENT. Run `44940680449` (`4e578c7`, the test without the fix) failed `TestLockFileCreateRace` at round 40. Run `44941162660` (`b479e00`, with the fix) was green on all nine jobs, including macOS release and race. That run's image was `macos-26-arm64` (macOS 26.7.2), which is what `macos-latest` resolved to.

Checked here, on Linux: `go vet ./...` passes for darwin/arm64 untagged, darwin/arm64 with `-tags metal`, and darwin/amd64, so the tests compile. A text search found no Linux-only path or call (`/proc`, `/sys`, `/dev/pts`, pidfd) in tests outside `*_linux_test.go`. Neither is a run.

No Go suite has run natively on the M4 (@lead-dev, 2026-11-25). #69 used binaries cross-built on Linux from `c4faeca` with the release flags and copied over; no GoReleaser archive and no binary built on the Mac has run there. Go 1.27.4 is installed on the M4. So the first macOS run of today's suite is step 3's, and step 6's install row is the first run of a shipped archive.

## Rejected

- **Merge or rebase `peter/lead-dev-2`.** #1 says to leave the branch as it is, and its `sys_unix.go` patch no longer applies: master moved the open into `openLockFile`, shared by `Lock` and `TryLock`.
- **One macOS job, release only.** `TestConcurrentSettingsWriters` failed on macOS under race alone. The lock faults are timing faults, and the race job took about six minutes on a public repository.
- **Metal work on the hosted runner.** It is a VM; what its Metal listing returns is not ours to rely on. Native Metal stays on the M4.
- **Skip by `GOOS`.** A skip names a probed trait of the machine, as B does. A test that fails on macOS for another reason gets a cause and a fix.
- **Normalise ids to NFC.** Spec 2.3 and architecture 4.4 say ids are compared without normalisation. Step 5 tests that rule on APFS; changing it is a product decision nobody has asked for.

## Steps

### 1. Fix C in `sys`

- `internal/sys/sys_unix.go`: add the constant `lockOpenTries = 50` and the function `openLock(d *dir, name string) (int, error)` exactly as in `git show b479e00:internal/sys/sys_unix.go`. In `openLockFile` replace the `retry(... O_CREAT ...)` block with `fd, err := openLock(d, name)`; keep the `pathErr("lock", ...)` wrap and the `ownerOnly` check. `Lock`, `TryLock`, the flock loop, `O_NOFOLLOW` and mode 0600 do not change. `Sub` does not change: `TestSubCreateRace` passed on macOS without a change to it.
- `internal/sys/sys_unix_test.go`: add `TestLockFileCreateRace`, `TestLockInRemovedFolder` and `TestSubCreateRace` from `b479e00`, unchanged. Their helpers (`tempRoot`, `must`, `asDir`, `ownerOnly`) exist on master.
- `internal/store/tx.go` and `cmd/pbrain/scenario_process_test.go`: apply `3a1e60f` (`git apply --check` accepts it on master). `Begin` then says "another pbrain command holds this section's lock; try again" only when its wait ran out, and "the section's lock cannot be taken" otherwise. The code stays `io`. This changes a message text; store.md fixes only the code.
- Proof: the three tests on Linux and macOS; scenario 14, `TestConcurrentSettingsWriters` and `TestModelProvisionShared` pass on macOS unmodified.

### 2. The macOS jobs

In `.github/workflows/ci.yml`, replace the header comment about macOS being off, and add:

```yaml
  macos:
    name: macOS arm64 (${{ matrix.mode }})
    runs-on: macos-latest
    timeout-minutes: 30
    strategy:
      fail-fast: false
      matrix:
        mode: [release, race]
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
      - name: the runner is darwin/arm64
        run: test "$(go env GOHOSTOS)/$(go env GOHOSTARCH)" = darwin/arm64
      - name: the suite in the release configuration
        if: matrix.mode == 'release'
        env:
          CGO_ENABLED: "0"
        run: go test ./...
      - name: build, vet and test the Metal build (fake devices; no GPU work)
        if: matrix.mode == 'release'
        env:
          CGO_ENABLED: "0"
        run: |
          go build -tags metal -o /dev/null ./cmd/pbrain
          go vet -tags metal ./...
          go test -tags metal ./internal/embed/... ./internal/cli
      - name: the suite under the race detector
        if: matrix.mode == 'race'
        env:
          CGO_ENABLED: "1"
        run: go test -race ./...
```

In the `check` job, after "verify release archives and checksums", upload `dist/pbrain_*_darwin_arm64.tar.gz` and `dist/pbrain_*_darwin_arm64_cpu.tar.gz` as the artifact `darwin-arm64-archives` (`if-no-files-found: error`, `retention-days: 1`). Then:

```yaml
  macos-archives:
    name: macOS arm64 release archives
    needs: check
    runs-on: macos-latest
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
      - name: the runner is darwin/arm64
        run: test "$(go env GOHOSTOS)/$(go env GOHOSTARCH)" = darwin/arm64
      - uses: actions/download-artifact@<current major>
        with:
          name: darwin-arm64-archives
          path: dist
      - name: run each archive's binary
        run: |
          for suffix in "" _cpu; do
            dir="$RUNNER_TEMP/pbrain$suffix"
            mkdir "$dir"
            tar -xzf dist/pbrain_*_darwin_arm64$suffix.tar.gz -C "$dir"
            PBRAIN_SCENARIO_BINARY="$dir/pbrain" go test -count=1 -run '^TestReleaseBinaryOutput$' ./cmd/pbrain
          done
          # Listing only: exit 0 and some output. Which rows a hosted VM lists is not checked.
          out=$("$RUNNER_TEMP/pbrain/pbrain" devices --json)
          test -n "$out"
          echo "$out"
```

The checksums are not checked again on macOS: `check` verified them on Linux, and `verify-release-archives.sh` needs `sha256sum`. `release.yml` does not change: a tag is cut only from a commit whose CI run is green, and that run now includes these jobs, on archives that differ from the release's in the version string alone. The Darwin amd64 archive stays build-only.

Rules for these jobs:

- A test under `-tags metal` must use a fake listing. One that reads the host's Metal listing is a test defect: give it a fake.
- No retry wrapper and no weakened assertion. Before merge, the macOS jobs pass three times in a row on the branch head (`gh run rerun`); C failed one round in 40.

### 3. Fix what the first run shows

Push steps 1 and 2 on a branch and read the macOS jobs. For each failure, write one line on #1: test, cause, fix. A product fault on macOS is fixed in the product. A Linux assumption in a test is ported, or skipped by a probed trait with the reason logged. A design question (locking, publishing, the handle calls of architecture 10.3) stops and goes to @architect and @operator before any change. #1 closes when all macOS and Linux jobs are green, no test was skipped or weakened to get there, and @reviewer approves the range.

### 4. Scenario 41's terminal checks on Darwin (#9)

- Split `cmd/pbrain/pty_linux_test.go`: everything except `openPTYFiles` moves to `pty_unix_test.go` (`//go:build linux || darwin`), unchanged. `pty_linux_test.go` keeps `openPTYFiles`. `pty_other_test.go` becomes `//go:build !linux && !darwin` and its reason drops the Mac backlog.
- New `pty_darwin_test.go` with `openPTYFiles`: open `/dev/ptmx` with `O_RDWR|O_NOCTTY|O_CLOEXEC`; `unix.IoctlSetInt(fd, unix.TIOCPTYGRANT, 0)`; `unix.IoctlSetInt(fd, unix.TIOCPTYUNLK, 0)`; read the terminal's name with `unix.Syscall(unix.SYS_IOCTL, fd, unix.TIOCPTYGNAME, ptr)` into a 128-byte buffer (the size the request encodes; x/sys v0.50.0 exports no helper for it); open that name `O_RDWR|O_NOCTTY`; clear `OPOST` with `unix.IoctlGetTermios(fd, unix.TIOCGETA)` and `unix.IoctlSetTermios(fd, unix.TIOCSETA, tio)`. Same error wrapping as on Linux.
- `41-colour.txtar`: drop "(on macOS: the Mac backlog)" from the comment. `[pty]` now holds on both systems.
- Proof: `TestColourPipeAndTerminal` and scenario 41's `[pty]` lines run, and do not skip, in both macOS jobs. They compare terminal bytes with pipe bytes exactly, so lost or translated output fails.

### 5. Accents on APFS (#9)

APFS keeps a name's bytes as given and treats composed and decomposed spellings as one name. pbrain compares ids without normalisation (spec 2.3), and `pbrain add` makes ASCII slugs only, so the case arises with hand-named files alone. Expected by the spec, not yet run on APFS: the stored spelling is found, the other spelling is `not_found`, and the item is listed once.

- `13-duplicate-ids.txtar`: keep the `[!nfdistinct] fsnote` line without "macOS tracked in the Mac backlog", and add under `[!nfdistinct]`: `pbrain get` of the fixture's stored spelling succeeds; `pbrain get` of the other spelling with `--json` exits 1 with `not_found` on stderr; `pbrain list --json` shows the id once.
- The same wording change in `39-names.txtar`, `39-location.txtar`, `store_test.go:712` and `exactvalue_test.go:173`: the reason stays, the backlog reference goes. The `[badnames]` and `[casedistinct]` skips stay: APFS cannot hold those fixtures.
- If the run shows another result, report it to @operator as a bug against spec 2.3; do not change the expectation.

### 6. Native checks on the M4 (#9), run by @lead-dev

The owner asked for the pbrain benchmarks on the M4 (`agent@clawdia2100.local`) on 2026-11-25, and @operator assigned them to @lead-dev with these checks added. One run covers both; nothing here is measured twice. Limits as in #69: an idle window, one heavy run at a time, one scratch directory (`$S` below), cleaned up afterwards with the evidence kept, `PBRAIN_ALLOW_GEMMA2=1` for the Gemma rows. It uses the published v0.4.6 `darwin_arm64` and `darwin_arm64_cpu` archives where they apply and records their hashes, with chip, macOS version, Go version, file system and load.

**The scratch directory.** Every artifact of the run stays under one fresh directory in the home folder, except the Go telemetry counters described below, an exception @operator accepted on 2026-11-25. Every Go and pbrain command of this step runs in a shell with these set:

```sh
export S=$(mktemp -d "$HOME/pbrain-m4.XXXXXX")
mkdir -p "$S"/{install,src,corpus,models,evidence,tmp,gocache,gomodcache,pbrain-cache}
export GOCACHE="$S/gocache" GOMODCACHE="$S/gomodcache" TMPDIR="$S/tmp" PBRAIN_CACHE="$S/pbrain-cache"
export GOFLAGS=-modcacherw   # the module cache stays writable, so the clean-up is a plain rm -rf
go env GOCACHE GOMODCACHE GOTELEMETRY GOTELEMETRYDIR   # recorded with the results
```

The repository is cloned into `$S/src` for the benchmarks and the suite, the corpus goes into `$S/corpus`, model files for the benchmarks into `$S/models`, and traces and logs into `$S/evidence`. `PBRAIN_CACHE` keeps the model and vector cache out of `~/Library/Caches/pbrain`. Two things follow from this:

- **Go's telemetry counters are the one write this cannot move.** `GOTELEMETRYDIR` is not settable (`go help telemetry`); in the default `local` mode each go command adds counter data there, outside `$S`. Turning telemetry off would itself change a setting on the owner's Mac. Record the mode and the directory, and leave both as found.
- **With `TMPDIR` under the home folder, the tests' temporary folders are not behind a link**, as macOS's default `/var/folders` is. The hosted jobs keep the default, so that case is covered there and not on the M4.

**Install from the tag.** Pass: each version equals the tag, the Metal archive lists `metal:0`, and the quarantine result is recorded, whatever it is.

```sh
set -euo pipefail   # run this block as a script: a failed check stops it
cd "$S/install"
gh release download v0.4.6 -R pkieltyka/pbrain \
  -p 'pbrain_0.4.6_darwin_arm64.tar.gz' -p 'pbrain_0.4.6_darwin_arm64_cpu.tar.gz' -p 'pbrain_0.4.6_checksums.txt'
# Only the two archives downloaded; the checksum file lists all seven archives.
grep -E ' pbrain_0\.4\.6_darwin_arm64(_cpu)?\.tar\.gz$' pbrain_0.4.6_checksums.txt > selected.sha256
# Exactly the two expected names, each once, before anything is verified.
test "$(awk '{print $2}' selected.sha256 | LC_ALL=C sort | tr '\n' ' ')" = 'pbrain_0.4.6_darwin_arm64.tar.gz pbrain_0.4.6_darwin_arm64_cpu.tar.gz '
shasum -a 256 -c selected.sha256
mkdir metal cpu quarantine
tar -xzf pbrain_0.4.6_darwin_arm64.tar.gz -C metal
tar -xzf pbrain_0.4.6_darwin_arm64_cpu.tar.gz -C cpu
metal/pbrain --version && metal/pbrain devices && cpu/pbrain --version
# What a browser download carries: flags 0083, the time in hex, the agent, an id.
cp metal/pbrain quarantine/pbrain
xattr -w com.apple.quarantine "0083;$(printf %x "$(date +%s)");Safari;$(uuidgen)" quarantine/pbrain
quarantine/pbrain --version || echo "quarantined copy: exit $?"   # recorded, not a failure of the script
CGO_ENABLED=0 GOBIN="$S/install/gobin-cpu" go install github.com/pkieltyka/pbrain/cmd/pbrain@v0.4.6
CGO_ENABLED=0 GOBIN="$S/install/gobin-metal" go install -tags metal github.com/pkieltyka/pbrain/cmd/pbrain@v0.4.6
gobin-cpu/pbrain --version && gobin-metal/pbrain --version && gobin-metal/pbrain devices
```

Nothing goes onto the system `PATH`. The quarantine attribute is set on that one scratch copy and on nothing else.

**Rule 3.** In `$S/src`, after one priming run, five fresh processes each of `GOMAXPROCS=4 go test -run '^$' -bench '^<name>$' -benchtime=1x -count=1 <package>` for `BenchmarkBgeFirstQuery` (`./internal/embed/bge`, `PBRAIN_TEST_BGE_MODEL`), `BenchmarkGemmaFirstQuery` (`./internal/embed/gemma2`, `PBRAIN_TEST_GEMMA2_PACK`) and `BenchmarkGemmaQ8FirstQuery` (`./internal/embed/gemma2`, `PBRAIN_TEST_GEMMA2_Q8`). All three open the model on the CPU, which is what a search does in every build. Pass: median under 600 ms. Every sample is published; a miss goes to the owner.

**Metal attach cost.** This is not rule 3 and has no target. No benchmark opens the model on Metal, and none is added for this run (@operator, 2026-11-25). The figure is the `device_attach` span of the Metal Q8 syncs' traces (`PBRAIN_TRACE`, `tools/sync-trace/README.md`), the split #69 recorded, reported with `model_open` beside it and labelled attach cost. A gated Metal attach-plus-query number would be a separate issue.

**`search --semantic` at 5,000 items.** On the 5,000-item corpus of #22, copied into `$S/corpus`: a fresh full sync with Metal Q8 (default profile) with `PBRAIN_TRACE` set to a file in `$S/evidence`, then the one-note increment; a bge-small CPU sync if the Mac stays idle. After each sync, warm `pbrain search --semantic` with the `darwin_arm64` archive binary, timed as the Linux refresh (#32) does. Pass: warm median under 1 s (functional spec §16). Sync time, rate and peak memory are recorded. The bge-small row is the default model's; if it does not run now, it stays owed and #9 stays open for it.

**The suite, natively.** In `$S/src`: `CGO_ENABLED=0 go test ./...` and `CGO_ENABLED=0 go test -tags metal ./internal/embed/... ./internal/cli`. Required to close #9, at the commit that closes #1: both pass there, with the terminal checks run and not skipped. An attempt during this run, on master as it is, is early signal only: the lock-create race may show in scenario 14, `TestConcurrentSettingsWriters` or `TestModelProvisionShared`, and the terminal checks skip. Any other failure is new, goes to @architect with test name, package and text, and joins step 3's list.

Results are grouped by machine in the #32 refresh write-up in `internal/cli/README.measure.md` and fill the Mac column of the README's speed table. #9 closes when the install, rule 3 and 5,000-item results are recorded for bge-small and Q8_0, the native suite has passed at the commit that closes #1, and steps 4 and 5 are on master.

Not in this plan: a rule 3 benchmark for UD-Q6_K_XL (none exists), the Google pack's 5,000-item index on the Mac's CPUs, cold-file-cache figures, and Darwin amd64.

### 7. Text that changes with it

@architect writes these when @operator asks; @operator owns `FACTORY.md`.

- With step 1: store.md "Native handles and locking" gains the create rule (an existing lock file is opened; a missing one is created only with `O_CREAT|O_EXCL`; at most 50 tries 1 ms apart when the create loses the race or either call returns ENOENT) and rows for the three tests. `docs/release-notes/next.md` gets the fix; it ends the known issue of the v0.1.1 notes.
- When the macOS jobs are green on master: `FACTORY.md` ("green on Linux" and "macOS is out of scope"), architecture §14 (the CI paragraph) and §17 (the two macOS test bullets and "macOS coverage", which still describes the lock fault), functional spec §16 "Platforms".
- After step 6: the README speed table, the opening paragraph and "Release build and install" of `README.measure.md`, architecture §17's owed measurements.

## Risks and gaps

- **Unknown failures.** Only C is known. Step 3 may find more work than this plan sizes; anything that touches the lock or publish design comes back for a decision.
- **Hosted runner noise.** Timing-sensitive tests may fail there and not on the M4. The three-green rule is the guard; a test that fails one run in several gets a cause, as C did.
- **Pseudo-terminal output on macOS.** Whether Darwin can drop output still queued when the child's side closes is not verified. The byte-exact comparison would show it as a failure. If it does, stop and send @architect the log.
- **`unix.Syscall(unix.SYS_IOCTL, ...)` on Darwin** is not compiled or run here. Step 4's proof is the run.
- **`pbrain devices` in a VM.** The spec says it exits 0 whatever the listing does. If the Metal archive crashes or hangs on the hosted runner, that is a product finding for @operator, not a reason to drop the step.
- **Unsigned binaries.** The archives are not notarised. Step 6's quarantine row measures what a browser download meets. Signing needs an Apple developer identity and is the owner's call.

## Questions

| Question | Owner |
| --- | --- |
| The current major of `actions/upload-artifact` and `actions/download-artifact` | implementer, at step 2 |
