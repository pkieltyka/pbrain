---
status: complete
---

# Component: Filesystem and item storage

## Purpose and scope

`sys` implements the injectable, descriptor-relative filesystem operations on Linux and macOS. `store` scans a guarded brain, opens user-supplied files, locks a section, and publishes complete files. The caller decides which operations are allowed, validates links and renders messages. Item writes leave semantic data alone. Only the explicit index build updates the semantic index. [Architecture §§4.3, 7.1 and 10](../architecture.md) · [Functional spec §§2, 5 and 8](../functional_spec.md)

## Interfaces

```go
package sys

type Identity struct { Device, Inode uint64 }
type FileInfo struct {
    Identity Identity
    Mode     fs.FileMode
    Size     int64
    ModTime  time.Time
}
type Entry struct { Name string; Info FileInfo }
type ResolvedPath struct {
    Path      string     // absolute path after symlink resolution
    Exists    bool
    Info      FileInfo   // valid when Exists
    Ancestors []Identity // directory identities, root first, parent last
}
type RegularFile interface {
    io.Reader
    io.ReaderAt
    io.Seeker
    io.Closer
    Stat() (FileInfo, error)
}

type Dir interface {
    Sub(name string, create bool) (Dir, error)
    Read(name string) ([]byte, FileInfo, error)
    CreateTemp() (name string, w io.WriteCloser, err error)
    LinkNew(temp, name string) error
    Replace(temp, name string) error
    Remove(name string) error
    Sync() error
    Close() error
}

type Files interface {
    Resolve(path string, allowMissing bool) (ResolvedPath, error)
    OpenRoot(path string, create bool) (Dir, error)
    DirInfo(dir Dir) (FileInfo, error)
    Entries(dir Dir) ([]Entry, error)
    Stat(dir Dir, name string) (FileInfo, error)
    OpenRegular(dir Dir, name string) (RegularFile, error)
    Flush(w io.WriteCloser) error
    Lock(ctx context.Context, dir Dir, name string) (io.Closer, error)
}

func Native() Files
func OpenNamed(files Files, path string, create bool) (Dir, ResolvedPath, error)
func OpenDir(path string) (Dir, error)
```

`sys` returns filesystem errors and sentinel errors for an unsafe entry type, invalid component, changed read and exceeded size. It retains `errors.Is` support for `fs.ErrNotExist` and `fs.ErrExist`. An open with `create=false` on a missing entry gives `fs.ErrNotExist`. `Read` has a hard 5 MiB ceiling. Larger trusted files, such as a model, go through `OpenRegular` and the caller's streaming and digest checks.

```go
package store

type BodyPolicy uint8 // KeepAll, KeepNone
type Skipped struct {
    Section item.Section
    Path    string // absolute, for caller's display-path policy
    Code    string // bad_front_matter, bad_file_name or not_an_item
    Reason  string // fixed category, never file content
    Line, Column int // 1-based when available, otherwise zero
}
type Catalog struct { /* immutable after Scan */ }
type WritePlan struct {
    Dest     item.Section
    Lookup   brain.View
    Disclose brain.View
}
type Tx struct { /* owns section lock and child handles */ }
type Commit struct { Cleanup []Cleanup }
type Cleanup uint8 // TempLeft, FolderSyncFailed

var ErrChanged error
var ErrNameExists error

func Scan(ctx context.Context, b *brain.Brain, v brain.View, keep BodyPolicy) (*Catalog, error)
func (c *Catalog) Items() []*item.Item
func (c *Catalog) Lookup(id string) []*item.Item
func (c *Catalog) Skipped() []Skipped
func (c *Catalog) Body(it *item.Item) ([]byte, error)
func OpenInput(b *brain.Brain, dest item.Section, path string) ([]byte, error)

func Begin(ctx context.Context, b *brain.Brain, section item.Section) (*Tx, error)
func (tx *Tx) Close() error
func (tx *Tx) CheckDestination(ctx context.Context, kind item.Kind, id string) error
func (tx *Tx) Publish(ctx context.Context, kind item.Kind, id string, data []byte) (commit Commit, err error)
func (tx *Tx) ReadFile(it *item.Item) (data []byte, version item.Version, err error)
func (tx *Tx) ReplaceItem(it *item.Item, want item.Version, data []byte) (commit Commit, err error)
func (tx *Tx) RemoveItem(it *item.Item) (commit Commit, err error)
func (tx *Tx) PutOriginal(digest [32]byte, data []byte) error
func (tx *Tx) RemoveOriginal(digest [32]byte) (removed bool, err error)
func (tx *Tx) ReplaceSettings(data []byte) (commit Commit, err error)
```

- `Scan` returns `personal_unavailable` when its view requires an unreachable configured personal section, `git_required` on a failed shared ignore check, and `io` for other essential storage failures. Invalid notes become `Skipped` entries. `check` consumes their safe reason and position from that same scan. `Body` returns `ErrChanged` when it cannot read the same item version. The read command scans again and retries once before returning `conflict`. Returned catalog items and body slices are read-only.
- `OpenInput` returns the byte sequence from a verified regular file. A missing file and a refusal by the privacy guard return identical `not_found` errors. More than 5 MiB returns `too_large`. Other I/O failures use a generic `io` error and never include an absolute or user-supplied path.
- `Begin` waits for one section's lock and returns `io` for a timeout or lock failure, or `personal_unavailable` for a configured personal root that is unreachable. The caller refreshes config/catalog while locked. `Close` releases the lock and owned child handles, even after failure. It never closes the brain's root.
- `CheckDestination` returns `invalid` for an ignored or unsafe write destination and `ErrNameExists` for an occupied final name. `Publish` repeats the destination check and treats an existing name, including a late collision, as `ErrNameExists`; the caller may draw a new ID. I/O and Git failures return their respective codes. `ReadFile`, `ReplaceItem` and `RemoveItem` accept only an item of the locked section. A changed or absent target is `conflict`; a symlink is refused before commit.
- A mutation returning nil `err` passed its commit point. Any file cleanup after that point is recorded as typed facts in `Commit.Cleanup`; the caller turns them into `after_write` notices. A non-nil `err` means no item/settings commit occurred. Original failure is a pre-commit `io` for import or a post-commit `after_write` for delete. `RemoveOriginal` reports false with an error when the file is missing or could not be removed.

## Internal design

### Native handles and locking

Resolve external selectors or input paths with the native filesystem's symlink resolution rules before opening them. For missing paths, `Resolve` resolves the longest existing prefix and appends the components that do not yet exist when `allowMissing=true`. Preserve errors other than absence. Resolve existing objects with directory/file identities as well as path spelling. `OpenRoot` accepts a canonical absolute directory anywhere on the machine. Open it from `/`, one component at a time with `openat` using `O_DIRECTORY|O_NOFOLLOW|O_CLOEXEC`. On a missing child with creation enabled use `mkdirat`, then open it with the same flags. Reject `.`, `..`, empty names, NUL and separators at every child method. The only internal exception is opening `.` on an existing descriptor to obtain a fresh directory cursor for `Entries`. Enumerate and classify with `fstatat(AT_SYMLINK_NOFOLLOW)`. `Stat` never follows the final link.

**Named roots** (architecture §10.3). `OpenNamed(files, path, create)` is for a folder named from outside pbrain: it requires an absolute path, calls `files.Resolve(filepath.Clean(path), create)`, then `files.OpenRoot(resolved.Path, create)`, and returns the handle with the `ResolvedPath`. It makes no check of its own and uses only the two `Files` methods, so a test's failing or recording `Files` sees both calls. A link in the path, at its last component or above, is followed by the first step; the second step follows none, so a component swapped for a link in between fails with `ErrUnsafe`. `OpenDir(path)` is `OpenNamed(Native(), path, false)` returning the handle alone. `OpenRoot` is unchanged and still refuses a link in any component: it is for a path the caller has already resolved, as `brain`, `init`, the personal section, the cache and `OpenInput` do because each checks the resolved path before it opens. A new caller that opens a named folder uses `OpenNamed`; tests open a temporary folder with `OpenDir` and never with `OpenRoot` on `t.TempDir()`, which is behind a link on macOS.

`OpenRegular` opens descriptor-relative with `O_NOFOLLOW|O_CLOEXEC|O_NONBLOCK`, then checks the descriptor's type before reading. The nonblocking flag prevents an entry swapped for a FIFO from stalling the caller. Reading a bounded file checks metadata before allocation, then reads at most one byte above the ceiling and checks metadata again. Reject growth beyond the ceiling and report a change if the identity, length or modification time differs during the read. The read is private to a goroutine, and directories can be used concurrently with independent cursors.

All new directories and temporary files are owner-only by default. Existing permissions are left in place on directories, and existing private files are never made more permissive. A temporary name is unpredictable, starts with a dot and has no `.md` extension. Open it exclusively with `O_CREAT|O_EXCL|O_NOFOLLOW` in the final folder. `Flush` calls file sync before close. `LinkNew` uses `linkat` without following links. Where hard links are unavailable, fall back to `renameat2(RENAME_NOREPLACE)` on Linux and `renameatx_np(RENAME_EXCL)` on macOS. An existing target remains an error. If both primitives fail, leave the target unchanged. `Replace` is the same-directory rename used only for a checked existing item or settings. `Remove` unlinks a regular entry relative to its handle. The cleanup after `LinkNew` accepts a missing temporary name because the exclusive-rename path already consumed it. [Linux rename contract](https://www.man7.org/linux/man-pages/man2/renameat2.2.html) · [Apple rename contract](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/man/man2/rename.2)

`Files.Lock` opens the named regular owner-only lock without following links and takes `flock(LOCK_EX|LOCK_NB)`. It retries would-block at 25 ms intervals until its supplied context ends. Retry interrupted syscalls within the same context. It has no timeout of its own, so an explicit index build can wait longer. Closing the holder releases the descriptor and lock, including when a process dies. The lock file stays in place.

`Begin` borrows the opened root and opens or creates its `.pbrain/` child. A write to the missing default personal section first uses `EnsurePersonal`. `Begin` calls `Files.Lock` for `.pbrain/lock` with a context capped at the earlier of the caller deadline and 30 seconds, mapping failure to `io`. Hold the descriptor through cleanup. Settings use the same lock in the shared section. A command takes only one section lock. Readers and previews never call `Begin`. Native advisory locking coordinates pbrain processes; concurrent hand edits can still land in the final compare-and-mutate gap.

### Catalog scan and rereads

1. Scan shared first. In personal view also borrow the personal root or treat an absent default personal folder as empty. The guard has already run. A shared scan prunes dot directories, all protected paths and aliases, symlinks, and special files before traversal. A personal scan applies the same structural filters, treats its own section root as permitted, and skips Git ignore entirely.
2. Visit one breadth level at a time. Collect each eligible child directory and each regular file ending in `.md`. Submit the shared candidates in one ordered `brain.SharedIgnored` batch after pruning. Never enumerate a child whose batch result is ignored, including a tracked file below that child. Keep only path components and checked identities in the pending set. When re-opening a chain from the borrowed root, use `Sub(create=false)` and compare every reopened directory with the identity observed during enumeration. Skip vanished or swapped routes. This keeps descriptors bounded by workers and depth, even in a broad tree.
3. Use at most `runtime.GOMAXPROCS(0)` workers to open candidate files. Compare the opened file's identity to the listed one and verify type. Use a bounded 4 MiB read that checks growth. On a concurrent rename, missing/replaced entry or inconsistent read, skip that candidate for this scan. Permissions or other unexpected read failures fail with generic `io`. Whole-file overflow and invalid UTF-8 produce `bad_front_matter` with safe reasons `too_large` and `invalid_utf8`. A parser-rejected header supplies the safe reason (`unclosed`, `syntax`, `not_mapping` or `duplicate_key`) and any line and column from `item.FrontMatterError`. A missing front-matter block (`item.ErrNoFrontMatter`) produces `not_an_item` with no position. **The name is admitted next.** The stem is the file name without its final `.md`. If the block is present and the stem is not valid UTF-8 (`utf8.ValidString`), the file produces `bad_file_name` with reason `invalid_utf8` and no position, whatever the parser said about the header, and never becomes an item: `Fold` would decode its bad bytes to U+FFFD and give it the key of another file. The order is fixed: whole-file overflow, invalid UTF-8 in the content, no front matter, the name, then the parser's own errors. So a file without front matter stays `not_an_item` whatever its name, and every catalog ID is valid UTF-8. A valid stem under a folder whose name is not valid UTF-8 is an item as before. No check diagnostic rereads the file or quotes raw parser text. Recoverable field problems stay on the parsed item.
4. Populate `item.Version` from the observed size, modification time and CRC-32C of the entire read. Save `bodyAt` from `item.Parse` as the body byte offset. After a successful parse, assign `Item.Layout = item.Measure(data, bodyAt)` using the same validated whole-file bytes, for both `KeepAll` and `KeepNone`. Keep body bytes only with `KeepAll`. Sort items and skipped entries by section, then relative path in byte order. Build a map of the folded ID to all matching items. `Lookup` folds its argument with the item rule and keeps duplicate IDs. For an argument that is not valid UTF-8 it returns nothing, before folding: the fold would turn its bad bytes into U+FFFD and find the item whose stem really holds that character.
5. A `KeepNone` `Body` request opens the saved route again through its guarded handles and verifies the file identity and complete `Version` before slicing at its saved body offset. Missing, newly invalid, symlinked, replaced or changed bytes return `ErrChanged`; an unrelated I/O error maps to `io`. `KeepAll` returns the original slice, so a concurrent `verify` cannot mix a header with another body's bytes.

### Guarded caller input

Resolve a relative input against `Brain.CallerDir`. Run the shared-destination refusal before opening or reading file content. Compare the canonical path to every protected path by path components and by the identities of existing protected directories or files. Reject any ancestor component whose architecture fold equals `.pbrain`, independently of the filesystem's case policy and the selected brain. A refused path, a missing file and an alias with no usable target all yield `not_found` with the same fixed text.

Open the resolved parent with its canonical components through `OpenRoot(create=false)`. Check the parent's observed identity and the input descriptor's identity against the resolved observation before reading. If something was swapped to a link, protected route or different file, return the same `not_found` without reading the content. Cap at 5 MiB with a size check and a bounded read, then verify that the observation still matches. Higher-level checks validate UTF-8 and character counts. stdin goes directly to its command's bounded parser.

### Write primitives and originals

- To validate a new item destination, derive the kind folder from its enum and the final name from the caller's generated ID. Reject an unsafe folder, a nonregular occupying entry and a protected destination. For shared, ask `SharedIgnored` about the folder with a trailing slash and the final path even if absent. Run this before storing an original. `Publish` performs the final check under the same lock, opens or creates the kind folder, writes and flushes the complete temp file, then calls `LinkNew` once. The successful link or exclusive rename commits. Remove the temp if still present and sync the folder after commitment. Creation of any destination directory is synced in its parent.
- `ReadFile` resolves the item's relative route from the locked section through no-follow handles, bounds its read and requires the item snapshot's `Version`. `ReplaceItem` stages the edited bytes next to the existing file, re-reads and compares the expected `Version` immediately before rename, then renames and syncs. `RemoveItem` rechecks the target version, then unlinks it and syncs the folder. No command follows a swapped path to a different object. A compare and unlink or rename remain separate operations.
- **Known limit: a recreated file can be the same file.** A file's identity is its device and inode number, and its `Version` is its size, its modification time and the CRC-32C of its bytes. A file that is unlinked and created again can be given the inode number that was just freed, which ext4 does readily. If it also holds the same bytes and carries the same modification time, nothing pbrain observes tells it from the file that was scanned, and `ReplaceItem` and `RemoveItem` act on it without `conflict`. This is accepted and is not a change to make: the file acted on holds exactly the bytes that were checked, so the result is the one the command decided on, and a stronger identity would need a file-system feature pbrain does not require. It does bind the tests: a fixture for the conflict path must make the replacement certain to differ, by staging the new file beside the old one and renaming it over it, so that both inodes exist at once and cannot be equal. Removing a file and writing it again is not such a fixture, since the outcome then depends on the file system's allocator; `TestMutationFollowsScannedIdentity` failed on that under the race detector on Linux.
- Settings publication requires the shared lock. Use a brain-relative temporary file and a complete TOML payload. Replace a checked regular `personal.toml` atomically; for missing settings use `LinkNew`, with a simultaneous external creation reported as `conflict`. The caller reads and encodes the new settings while holding this lock.
- `PutOriginal` receives an already computed digest and verifies the input bytes against it. The input ceiling is 5 MiB. Open `.pbrain/originals` under the locked section with no-follow handles. The filename is lower-case hexadecimal encoding of the 32-byte digest. For an existing file, verify regular type and the exact expected byte count by descriptor before reading. Hash at most the expected length plus one sentinel byte, require exact EOF and unchanged metadata, then compare the digest. Reject a size or digest mismatch with `io` before item publication. For a missing original, publish by the same temp/flush/link protocol and use the same bounded verification on the published bytes. A failure can leave an unused original in local data; never publish the referencing item on failure.
- `RemoveOriginal` accepts a parsed 32-byte digest, so no caller string can become a path component. It requires an existing regular file under `.pbrain/originals` of the locked section. The caller checks whether any other item in that section still references it, while locked, before calling. A failed unlink returns `removed=false`. If unlink succeeds and directory sync fails, return `removed=true` and the sync error so the caller can report the actual state and emit `after_write`.

## Dependencies

`sys` depends on the standard library and approved `x/sys/unix`. `store` uses `sys`, `brain`, `item` and `errs`. `brain` uses `sys` and has no dependency on store. The catalog implements `search.Corpus` without importing search. CLI uses the catalog and the storage write primitives. The importer prepares data before CLI passes it to these primitives. Semantic uses `OpenSection` and its `Sub` handles for the index and `OpenRoot` for the per-user cache.

## Test plan

| Test | Assertions and spec scenarios |
|---|---|
| `TestSharedScanPrunesBeforeList` | Instrument directory listing to prove ignored, private and dot folders are pruned before listing. Repeat with a tracked ignored note and linked worktree. Spec 6–9. |
| `TestWholeFileValidationAndStableCatalog` | Malformed YAML, invalid UTF-8 only in body, over 4 MiB, no front matter, duplicate folded IDs, and scrambled worker completion. Spec 5, 13, 17 and 21. |
| `TestNameAdmission` | Two files with valid front matter whose stems are the bytes `78 ff` and `78 ef bf bd`: the catalog holds one item, the second, and `Lookup` of its ID returns it alone; the first is a `Skipped` entry with `bad_file_name` and `invalid_utf8`; `Lookup` of the bytes `78 ff` returns nothing. A file with the first name and no front matter is `not_an_item`; one with that name and a duplicate key is `bad_file_name`. A valid stem under a folder with an invalid name is an item. A stem holding a line feed is an item with that ID. Spec 2.2, 2.3 and 39. |
| `TestSkippedDiagnosticsFromScan` | A duplicate key supplies its safe parser position and a large file supplies its size category on the same check page. No check-time reread or raw parser content. Spec §9.1 and 17. |
| `TestBodySnapshotAndReread` | A verify during search cannot mix versions. A get with changed bytes retries once; a second change gives conflict. Spec 5 and 16. |
| `TestOpenNamedRoots` | A folder reached through a linked ancestor, and one whose own name is a link, open with `OpenNamed`; the returned path holds no link, and `OpenRoot` on the path as given is `ErrUnsafe`. A link at a component below the opened root is `ErrUnsafe` from `Sub` and from every read. A component of the resolved path swapped for a link after `Resolve` makes the open fail. With `create`, the missing folders below the deepest existing one are made. A relative path is refused. The test makes its own links, so it proves the same on Linux, and it passes unchanged on macOS, where the temporary folder is already behind `/var`. Architecture §10.3. |
| `TestOpenInputBoundary` | Direct, symlink and case aliases to protected paths, other brains' `.pbrain/` and external personal originals all yield the same error as a missing file. No reads before a refusal, and no original is created. Spec 6, 9 and 26. |
| `TestSymlinkAndFIFOReplacement` | Swap a parent folder or final entry after inspection. Reads do not escape to a new target and a FIFO cannot stall. Apply the same swap to shared kind and originals folders. Spec 6 and 9. |
| `TestProcessSectionLock` | Two processes share a section and serialize replacement. A stalled input does not own the lock. Shared and personal locks are independent. Killing a holder releases the lock. Spec 14, 24 and 29. |
| `TestLockWaitPolicies` | A controllable cancellation/deadline proves native Files.Lock follows only the supplied context, while Begin supplies a 30-second cap and returns io on timeout. Semantic build can wait with a longer parent context. Spec 14, 29 and 35. |
| `TestExclusivePublish` | Use two real processes on each native target. One publishes, the other cannot replace its file. Exercise the hard-link path and the exclusive-rename fallback. Spec 14–15. |
| `TestCommitPointFailures` | Inject failures at each syscall before and after link, rename and unlink. Committed files stay visible and final cleanup failures are distinct. Original write failure leaves no item. Spec 15–16 and 29. |
| `TestDigestScopedOriginal` | Original corruption prevents import, duplicate digest is reused, and a malformed digest never becomes a path. Personal deletion cannot remove the shared original with an equal digest and vice versa. Spec 15, 18 and 29. |
| `TestOriginalReadIsBounded` | A large sparse object at the requested digest never gets hashed. A counting reader sees at most the expected size plus one byte if an original grows during verification. Neither case commits the item. Spec 15 and 18. |
| `TestAbsentIndexAndCacheRoot` | Reads open missing index parents without creation. An explicit index build can open a real cache root outside the brain with the same no-follow contract. Item writes leave both paths alone. Semantic amendment, 35. |
