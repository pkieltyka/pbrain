# Release archives with both GPU backends

**Decision.** From the release after v0.2.0, the standard Linux amd64 archive is built with
`-tags cuda,vulkan`, the tag set issue #49 qualified. The standard Linux arm64 archive stays
`-tags cuda`. The `_cpu` archives, the macOS archives and their names do not change, so a
release still has six archives. Owner's decision, 2026-11-22 (architecture section 16,
decision 10); this plan is the packaging that carries it out, against master `4e5645d`.

The change is one patch, `release-dual-tags.patch`, over six files. It changes no product
code: the only Go file is a test.

## Why arm64 keeps CUDA alone

goinfer's Vulkan backend at v0.6.2 supports Linux amd64 only. Its `unsupported.go` is built
for every other target and refuses every call, and pbrain's own import of the backend is
constrained to `linux && amd64 && vulkan`. So on arm64 the `vulkan` tag links nothing.
Measured: the Linux arm64 binary built with `cuda` and the one built with `cuda vulkan` are
both 44,630,279 bytes and differ only in the recorded tag string, and the second lists no
Vulkan module in `go version -m`. Building arm64 with both tags would only make its build
information claim a backend it does not have. Issue #49 qualified nothing on arm64, and the
arm64 CUDA archive is still cross-built and vetted and has run on no device.

## What the patch changes

| File | Change |
|---|---|
| `.goreleaser.yaml` | The build `pbrain-cuda` becomes two: `pbrain-cuda-vulkan` for Linux amd64 with tags `cuda`, `vulkan`, and `pbrain-cuda` for Linux arm64 with `cuda`. One archive, `pbrain-gpu`, takes both builds, so the file names stay `pbrain_<version>_linux_<arch>.tar.gz`. Both builds run the smoke script. |
| `internal/buildcheck/buildcheck_test.go` | `TestReleaseConfig` expects those three builds, each with its targets and tags. |
| `.github/scripts/verify-release-archives.sh` | Expects `-tags=cuda,vulkan` in the Linux amd64 GPU archive's build information and `-tags=cuda` in the arm64 one. It still runs `TestReleaseBinaryOutput` on each native Linux archive. |
| `.github/scripts/smoke-cuda-release.sh` | Runs twice: as the machine is, and with a Vulkan loader that cannot be opened. See below. A new mode, `PBRAIN_VERIFY_RELEASE_CUDA=devices`, runs the device listing alone, with no model. |
| `.github/workflows/ci.yml` | Builds and vets `-tags vulkan` and `-tags 'cuda vulkan'`, cross-builds and vets Linux arm64 with both tags, runs the `embed` and `cli` test packages under each of the two tag sets with CUDA masked, and runs the smoke script in `devices` mode on the snapshot binary. The existing CUDA lines are kept. |
| `.github/workflows/release.yml` | The comment only. The release still sets `PBRAIN_VERIFY_RELEASE_CUDA=1`. |

The script keeps its file name and its variable name, so nothing else that refers to them
changes. The names say CUDA and now cover both backends; renaming them is a separate, purely
cosmetic change.

## The smoke test

The script needs no GPU and starts none: CUDA is masked with `CUDA_VISIBLE_DEVICES=`, and a
hosted runner has no discrete AMD card.

1. **native**: the machine as it is.
2. **no-vulkan**: an empty file named `libvulkan.so.1` is put first on `LD_LIBRARY_PATH`.
   The loader finds it first and refuses it ("file too short") without looking further, so
   the Vulkan loader cannot be opened whatever the machine has installed.

In each run:

- `pbrain devices --json` must exit 0 and its standard output must be exactly one JSON
  document whose devices are the CPU alone. Standard error is shown and not compared, because
  a GPU driver may write there (functional spec 6.3).
- In release mode, a one-note `embeddinggemma-2` sync must report `device: cpu`, one passage
  embedded, and the `cpu_fallback` notice with the text for `absent`, "no usable GPU was
  found". A Vulkan that cannot be opened is `absent`; `failed` has another text and fails the
  script. Notices are read by skipping any line of standard error that is not a JSON object.

The second run uses a different note text, so it has a passage to embed and opens the model
again instead of taking the vector from the cache. Both runs share one model download.

## What was run

In a scratch clone of master `4e5645d` with the patch, Go 1.27.3, `CGO_ENABLED=0`, with CUDA
masked and no Vulkan loader on the path, so no GPU was touched:

- `go build` and `go vet ./...` with `-tags vulkan` and `-tags 'cuda vulkan'`, and both for
  Linux arm64 with both tags: pass.
- `go test -tags vulkan` and `go test -tags 'cuda vulkan'` over `./internal/embed/...` and
  `./internal/cli`: pass. `TestGemmaVulkan` skips without a card.
- `go test ./internal/buildcheck`: pass. `gofmt -l .`: clean. `shellcheck` on the two scripts:
  only the two warnings master already has.
- The smoke script on a binary built with the release flags and both tags: `devices` mode
  and release mode both pass, in both runs. Release mode used a local copy of the model pack.
  It does nothing with the variable unset or for a target that is not the host's.
- `git apply --check` against the main checkout: clean.

Stripped Linux amd64 binaries at this commit: 15,786,144 bytes with no tag, 46,358,791 with
`cuda`, 46,567,687 with both. Vulkan adds 208,896 bytes, and 77,368 to the gzip of the
binary (16,070,235 to 16,147,603).

## Not verified

- **GoReleaser itself.** It is not installed here. The YAML parses and `TestReleaseConfig`
  passes, but `goreleaser check` and a snapshot build have not run; CI's snapshot step is the
  first real run, and `verify-release-archives.sh` checks its six archives.
- **A hosted runner's Vulkan.** Whether `ubuntu-latest` has a Vulkan loader, and with what
  driver, is not known. The native run is written to pass either way on a machine with no
  discrete AMD card: no loader, a loader with no usable driver, and a loader that lists no
  such card all leave the CPU alone and are `absent`. If the runner's Vulkan fails some other
  way, the native run fails in release mode with the `failed` text, which is the check
  working; the fix would then be decided from that output.
- **The no-vulkan run where a loader is installed.** Here no loader was on the path, so both
  runs failed to open it. That the empty file is taken first and stops the search was shown
  with another library, not with a real `libvulkan.so.1` behind it.
- **Release mode on a runner** downloads the 1.5 GB model once, as it did for v0.2.0.

## For the owner

1. **The next version's number.** The release workflow reads `docs/release-notes/<tag>.md`.
   The notes for this change are in `docs/release-notes/next.md`, to be renamed to the tag.
2. **Nothing here tags or publishes.** The release workflow runs only on a pushed `v*` tag
   and makes a draft.

## Found on the way, not part of this change

`TestRuntimeEnvAndOutput` lists the linked packages with no build tag, and matches the
module paths of aikit and the goinfer root only. So it reads no file of the CUDA or Vulkan
backend modules, in any build, although `components/semantic.md` says it covers each target
"with the tags that target's default archive is built with". The Vulkan module at v0.6.2 has
no environment read and no print in its non-test files. The CUDA module, released since
v0.2.0, reads environment variables in files of the package the binary links, among them
`CUDA_MPS_PIPE_DIRECTORY` and several `GOINFER_CUDA_*` names. Whether to extend the audit
with a list of allowed places, or to change what the spec promises, is a decision for a
ticket of its own.
