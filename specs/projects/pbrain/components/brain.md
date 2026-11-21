---
status: complete
---

# Component: Brain, configuration and Git

## Purpose and scope

`brain` finds one brain, decodes project and local settings, checks where private data can land, and owns the root handles for one command. `gitx` supplies read-only queries with the selected worktree's real index and ignore rules. `init` uses the same discovery and guard policy. The store handles item scans and mutations. The audience package owns URL and rule validation, and the semantic package owns model recognition and cache handling. [Architecture §§4.2, 4.5, 6 and 12](../architecture.md) · [Functional spec §§2, 5, 9.2 and 10](../functional_spec.md)

## Interfaces

The `Brain`, `SectionRoot`, `View`, `Config` and `LocalConfig` layouts are in architecture §4.2. `Config.Semantic.Model` is the resolved model name (below) and `LocalConfig.ModelPath` an optional string. A `Brain` is command-scoped and is used only by pointer. Its private state holds the environment, the Git top if any, the protected paths and opened section handles.

```go
package brain

type Env struct {
    Dir    string              // absolute command working directory
    Getenv func(string) string
    Git    gitx.Runner
    Now    func() time.Time
    Files  sys.Files
}

type OpenOptions struct { Brain string } // raw --brain value, empty when absent
type InitOptions struct {
    OpenOptions
    Path string // raw --path, empty when absent
}
type InitResult struct {
    Dir             string // canonical target, internal only
    Path            string // where the brain is: relative to the project root ("." at the root, ".." segments for a target outside it), or the supplied absolute path for an outside target given as one
    Selector        string // the value for --brain or PBRAIN_DIR that resolves from the caller's directory: the relative path from Env.Dir to the target, or the supplied absolute path
    NeedsSelector   bool
}
type ProtectedPath struct {
    Path      string // absolute, canonical when reachable
    Directory bool   // intended role, also for a missing path
}
type IgnorePath struct {
    Path      string // absolute and inside the brain's worktree, if any
    Directory bool
}

func Open(ctx context.Context, env Env, opt OpenOptions) (*Brain, error)
func Init(ctx context.Context, env Env, opt InitOptions) (InitResult, error)
func (b *Brain) Files() sys.Files
func (b *Brain) CallerDir() string
func (b *Brain) OpenSection(section item.Section) (sys.Dir, error)
func (b *Brain) EnsurePersonal(ctx context.Context) (sys.Dir, error)
func (b *Brain) Close() error
func (b *Brain) RequireView(view View) error
func (b *Brain) Protected() []ProtectedPath
func (b *Brain) SharedIgnored(ctx context.Context, paths []IgnorePath) ([]bool, error)
func (b *Brain) DisplayPath(section item.Section, path string) string
func (b *Brain) ResolveAuthor(ctx context.Context, explicit string) (string, error)
func (b *Brain) AudienceRules() ([]audience.Rule, error)
func ReadLocal(dir sys.Dir) (LocalConfig, error)
func EditAudience(dir sys.Dir, edit func([]audience.Rule) ([]audience.Rule, error)) (data []byte, changed bool, err error)
```

- `Open` returns `no_brain` when no marker exists, `invalid_config` for a malformed or conflicting configuration, `personal_exposed` or `git_required` from the guard, and `io` for other essential filesystem failures. The caller defers `Close`; it releases every cached root, once. Neither a scanner nor a writer closes a borrowed root. Child handles are owned by whoever called `Sub`.
- `OpenSection` returns the same handle for a successfully opened section throughout a command. An unreachable configured personal section yields `personal_unavailable`. A missing default personal folder under a reachable brain is an empty personal section. `OpenSection` returns `fs.ErrNotExist` for that case. `RequireView` succeeds, so read commands can return an empty personal contribution without creating anything. `EnsurePersonal` creates that default folder only for a write choosing it, checks its new handle and caches it. Subsequent calls return that handle. It never creates a configured external folder.
- `Init` returns `invalid` if a different brain already serves this project or the target cannot safely be a folder. It can also return `invalid_config`, `personal_exposed`, `git_required` and `io` for the same storage checks as `Open`. `NeedsSelector` tells the CLI to print the selector hint: `Path`, and `Selector` marked `from this directory`, as one line when the two are equal. An absolute path appears in either only when the caller supplied it for a target outside the project root; neither is ever derived as an absolute path.
- `SharedIgnored` returns a bool per input in the original order. Outside Git it returns all false. A Git failure is `git_required`. `Protected` returns a copy of the active protected path set for the input guard and shared pruning.
- `ResolveAuthor` returns `invalid` when the selected value is empty, multiline or too long, and `git_required` when Git is required but fails. `AudienceRules` returns a copy of the validated rule snapshot, or its deferred `invalid_config` error. The strict `ReadLocal` returns zero settings on missing and refuses malformed settings and invalid rules. `EditAudience` returns unpersisted bytes. Validation and callback errors return before the caller publishes anything.

```go
package gitx

type Result struct {
    Stdout []byte
    Status int
}
type Runner interface {
    Run(ctx context.Context, dir string, args []string, stdin []byte) (Result, error)
}
type Path struct {
    Rel       string // native relative path below the Git top
    Directory bool
}

func NewRunner(executable string, environ []string) Runner
func Top(ctx context.Context, r Runner, dir string) (string, error)
func Ignored(ctx context.Context, r Runner, top string, paths []Path) ([]bool, error)
func Tracked(ctx context.Context, r Runner, top string, path Path) (bool, error)
func BrainMarkers(ctx context.Context, r Runner, top string) ([]string, error)
func UserName(ctx context.Context, r Runner, dir string) (name string, found bool, err error)
```

`Runner` separates transport errors from ordinary exit statuses. The production adapter passes exit statuses through, retaining stderr internally and excluding it from caller messages. The wrappers map start failure, timeout, unexpected status and malformed protocol output to `git_required`. `UserName` treats status 1 with no stdout as missing. No wrapper prints. `cli` supplies an injectable runner and resolves the executable once from the invocation environment. An unavailable executable is handled only if a command calls Git.

## Internal design

### Discovery and configuration

1. With explicit `--brain`, else a nonempty `PBRAIN_DIR`, inspect only that directory. Resolve relative selection against the caller's directory. Otherwise check each ascending directory for `pbrain.toml`, then `docs/brain/pbrain.toml`. Stop at the first marker, including an invalid marker that must produce an error. Cross a Git boundary during ordinary discovery, as allowed by spec §10.
2. Canonicalize the selected directory, then open its handle. Read `pbrain.toml` as a regular file through the handle. A symlink or special-file marker is `invalid_config`; a missing marker is `no_brain`. Determine the Git top with the nearest ancestral `.git` entry and `gitx.Top`; the brain itself anchors printed paths outside Git. Reject a malformed or inconsistent top rather than issuing a query in another worktree.
3. Check the fixed protected paths first, then read `personal.toml` from the brain handle if present. Work out the configured personal root and its data folder, and check them before any item inspection. Resolve a configured relative path against the brain. Reject a resolved path equal to or containing the brain, including directory identity aliases. A missing or inaccessible configured root remains recorded as unreachable; shared operations can proceed and a command selecting personal gets `personal_unavailable`. A broken symlink or wrong type at a fixed protected path fails the guard.
4. When an alternate personal path is configured, ensure the guarded default folder holds no items. Walk its regular `.md` candidates by handles and without dot folders or symlinks. Apply the same whole-file limits and item parser as the store, stopping at the first valid item with a fixed `invalid_config` message. Even when inactive, this folder is pruned from shared scans and blocked for shared file inputs.
5. Cache the handles and protection facts after checks succeed. If a command creating the default personal root races another creator, accept the winner only after opening the resulting real directory and applying the guard to that handle. On failed `Open`, release all handles before returning.

Decode with BurntSushi TOML using typed wire structures and explicit optional values so an absent key differs from zero. Apply the defaults of spec §6.1. Search and list limit overrides stay inside their flag ranges. A supplied character budget must be positive. Validate `[tags]` names against `[a-z0-9-]+` and descriptions as nonempty single-line strings. An explicitly empty table sets `HasTags`; omission leaves it false. Resolve the model name here, in `config.go`, so every reader of `Config.Semantic.Model` sees one answer (owner's decision, 2026-11-21): a missing `semantic.model` key gives `brain.DefaultModel`, the exported constant `"embeddinggemma-2"`; the value `"none"`, exactly and case-sensitively, gives `""`, which means semantic search is off; an empty string is `invalid_config`; any other string passes through unchanged and the registry judges it. Keep the local model path as a configuration string. Resolve a relative `model_path` from the brain when handing it to the semantic component. That component owns checksums, registry membership and the `PBRAIN_CACHE` selection.

Decode TOML syntax and the safety-critical path before converting the audience value. Decode `[[audience]]` with `match` and `to`, then use `audience.ValidateRules`. On open, malformed TOML and invalid path fail the command. A well-formed file with an invalid audience value or rule leaves the guarded brain readable and saves a redacted rule error in the brain. No ordinary read consults the bad rules. Every item mutation, including one with an explicit destination, must check `AudienceRules` before acquiring input or taking a lock; the audience listing uses it too. An audience edit or inside-lock item refresh uses strict `ReadLocal` through the handle and refuses a fresh invalid rule.

`EditAudience` reads the local file once under the brain's settings lock. Validate the loaded rules, give the callback a copy, then validate its returned rules. Match rules by their canonical form. If the semantic list is unchanged, return `changed=false` so a missing file stays missing and comments survive a no-op. On a change preserve every other TOML value, including `path`, `author` and `model_path`. Write a complete TOML file with stable key order via the store's atomic replacement. Comments may be lost on an actual edit. The same `ReadLocal` is used by writes for the inside-lock refresh; the caller compares the refreshed plan and effective root before continuing.

Author precedence is explicit flag, nonempty `PBRAIN_AUTHOR`, nonempty local author, then `human:` plus Git's nonempty `user.name` read in the selected brain's directory. Validate the selected value as a single nonempty line of at most 200 code points, preserving the actor spelling. Reject a supplied invalid higher-priority value rather than signing with another actor. Trim the Git record terminator; do not trim meaningful whitespace inside a valid name.

### Git subprocesses

- Copy the invocation environment. Remove `GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`, `GIT_COMMON_DIR`, `GIT_OBJECT_DIRECTORY`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`, `GIT_NAMESPACE` and `GIT_CEILING_DIRECTORIES`. Clear the four Git pathspec environment switches so the wrapper controls literal matching. Retain normal configuration and ignore sources. Set `GIT_OPTIONAL_LOCKS=0`, disable the pager and cap each call at ten seconds or the parent deadline. Use argument vectors and never a shell.
- Use `rev-parse --show-toplevel` in the nearest existing directory in the selected worktree. Only a known worktree calls this wrapper. Decode one returned absolute path by removing the record terminator. Compare it with the resolved directory containing the discovered `.git` entry. Resolve existing path aliases when comparing and refuse an inconsistent Git top.
- For ignore checks run `check-ignore --no-index -z --stdin` from the top. Prefix each worktree-relative path with `./` to pass names beginning with a colon as ordinary paths, append a slash for a directory (also for missing folders), and NUL-terminate each input. Parse the returned original names as a set, then emit bools in input order. Accept statuses 0 and 1. Do not use `--literal-pathspecs` for this command: Git rejects literal pathspec magic here.
- For tracked checks run `git --literal-pathspecs ls-files --cached -z -- <path>` from the top. Omit a folder's final slash so the query covers both the directory's children and a tracked entry at the same path. Any returned entry fails the guard, even from an unmerged index, sparse checkout or force-add. Never derive tracked state from the working copy.
- For init candidates run `ls-files --cached --others --exclude-standard -z`, then keep paths whose final component is `pbrain.toml`. Deduplicate worktree-relative paths, discard markers absent from the working tree, and use canonical folder identity when comparing with the target. Git links and nested repository contents are outside this worktree's listing.

### Exposure guard and display paths

Group normalized protected paths by the actual worktree containing their closest existing ancestor. A protected directory with its own `.git` entry is already unsafe. Test both ignore and tracked state for every protected path inside Git; absence from disk does not waive the Git check. Inside each existing protected directory, walk directory entries by handle and stop at any `.git` entry of any type. Do not follow links or read Markdown during this check. For a reachable root, failure to establish these facts is fail-closed. A Git query failure remains `git_required`; a positive exposure or an unverifiable protected directory is `personal_exposed`. The message contains only the protected path's role and a remedy. An externally configured personal root marked unreachable is not walked.

For display, a file inside the anchor uses its forward-slash relative path. An external personal file uses `personal:` plus its forward-slash relative path under that section. Use a path only after verifying section membership. Private filesystem paths and raw syscall messages stay internal.

### Init

Resolve the project root from the caller's worktree, or the current directory outside Git. Select the target by `--path`, `--brain`, `PBRAIN_DIR`, default `docs/brain`. Resolve relative `--path` and the default from the project root, and relative flag/environment brain selectors from the caller's directory. Check the normal discovery walk within the project and the Git marker inventory before creating anything. Refuse a canonical target different from an already existing brain of this project. Ignore an outer project's brain when initializing a separate nested checkout.

Report the target twice, because the two selectors start in different places: a relative `--path` starts at the project root, a relative `--brain` or `PBRAIN_DIR` at the caller's directory. `Path` is the target relative to the project root. `Selector` is the target relative to `Env.Dir`, the value that works as `--brain` or `PBRAIN_DIR` from where the caller stands; from `src/`, `init --path notes` gives `Path` `notes` and `Selector` `../notes`. A target inside the project root is always reported in these relative forms, also when it was supplied as an absolute path. A target outside it that was supplied as an absolute path is reported as supplied in both fields; one supplied as a relative path gets the two relative forms. Both use forward slashes.

Use folder handles to create the target and missing `pbrain.toml` without overwriting any file. Inspect `.gitignore` through the same target handle and refuse a symlink or special file before reading it. Compute the complete new contents by appending missing exact literal lines `personal/`, `personal.toml` and `.pbrain/`, retaining every pre-existing byte and adding a separator if needed. For changed contents, stage and flush a temporary sibling and publish it using `LinkNew` for an absent file or `Replace` for a regular existing file. A failed publication preserves the old bytes. A repeat after failure converges even if init had already created the brain marker. A read-only reopen loads settings and runs the exposure guard with the new ignore rules. An unchanged file is left in place. No item, personal directory, lock or index is created by init.

## Dependencies

`brain` depends on `gitx`, `sys`, `audience`, `item` and `errs`. Those packages do not import `brain`. `gitx` depends on the process API and `errs`. CLI builds the dependencies and consumes the guarded brain. Store uses its borrowed section handles, protection set and Git batching. Audience editing uses the same brain handle with the store's shared settings lock. Semantic uses the same section handles under `.pbrain/index/` and opens its per-user cache separately through `sys.Files`.

## Test plan

| Test | Assertions and spec scenarios |
|---|---|
| `TestDiscoveryAndInitPrecedence` | Walk from root, child and brain; explicit selector wins; custom folder, malformed config, missing brain, nested checkout, repeat init and partially populated ignore file. Spec 27. |
| `TestInitSelectorFromCallerDir` | `init --path notes` from the project root reports `Path` and `Selector` `notes`, printed once; from `src/` it reports `notes` and `../notes`, and `Open` from `src/` with `--brain ../notes`, and with `PBRAIN_DIR=../notes`, finds that brain, while `--brain notes` from there does not. An absolute `--path` outside the project is echoed unchanged in both fields; a relative one outside it gives two relative forms; an absolute one inside the project gives the relative forms. No output holds an absolute path the caller did not supply. Spec 9.2, scenario 27. |
| `TestInitFindsCustomMarkerInGit` | Tracked and ordinary untracked custom markers prevent creating another brain; an ignored or deleted marker and a separate nested checkout do not block. Owner amendment to spec 9.2. |
| `TestInitWritesIgnoreAtomically` | A symlink target is untouched. Inject rename failure over existing contents without a terminal newline, confirm marker and old bytes survive, then retry and confirm exact idempotent additions. Spec 27. |
| `TestGuardRealIndexAndRoot` | Ignored force-added note fails with empty alternate index set; repository and worktree overrides have no effect; linked worktree uses its own index and ignores. A tracked entry at the protected directory's exact path also fails. Spec 8–9. |
| `TestGuardMissingPathsAndNestedGit` | Directory-only ignore of absent paths, ignored/unignored external sections, protected `.git` directory or gitfile at root or depth, unreachable section, and required Git unavailable. No error leaks a local path. Spec 6, 8, 26 and 29. |
| `TestDefaultPersonalLifecycle` | A fresh brain supports empty personal reads without creating directories; first personal write creates a guarded root. When an external path is configured a valid dormant default item fails config, and raw default files remain protected. Spec 6 and 26. |
| `TestIgnoreAndDisplayUseLiteralPaths` | Spaces, newline, glob characters and leading colon are preserved in Git batching. Relative display and external personal display omit home paths. Spec 6 and 9, path-anchor amendment. |
| `TestLocalSettingsRoundTrip` | Preserve the other values and model path; no-op does not publish; invalid hand-written rules fail; concurrent cooperating setters retain both rules with an external or unreachable personal section. Spec 24–26. |
| `TestInvalidRuleDoesNotStopSearch` | With a valid TOML file and path but invalid audience rule, search works and an explicitly shared write, audience list and edit fail with redacted `invalid_config`. Invalid path or TOML still fails open. Spec 24. |
| `TestReadProjectSettings` | Empty config defaults, zero limits invalid, empty versus missing `[tags]`, the model name (missing key gives `DefaultModel`, `"none"` gives off, `"None"` and another name pass through, an empty string is `invalid_config`) and optional local model path. A test in `cli` or `cmd/pbrain` checks that the release registry holds `brain.DefaultModel`. Spec 27, 32 and semantic amendment. |
| `TestAuthorPrecedenceAndWorktree` | Each author source wins in order, a bad chosen value is refused, and the brain repository's user name wins over the caller's checkout. Spec 8.2, 8.3 and 21. |
| `TestBorrowedRoots` | Repeated section opens share a handle, closing the brain closes each once, absent `.pbrain/index` on `create=false` stays absent. Architecture §§4.2 and 10.3. |
