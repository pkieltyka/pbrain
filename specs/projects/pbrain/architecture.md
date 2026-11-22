---
status: complete
---

# Architecture: pbrain

This document designs the system that the functional spec describes (`functional_spec.md`, as amended on 2026-11-20 for semantic search, `get --outline` and seven smaller points). Section numbers written "spec 7.1" refer to that file.

The owner answered the technical questions on 2026-11-20; section 16 records the answers as decisions. The owner's decisions of 2026-11-21 on the models, their runtimes and the vector store are in section 8.7; the design for BM25 is section 8.8; the owner's decisions of the same day on what a search result prints (file lines, sizes, occurrences, context, colour) are in sections 4.1, 8.4, 9 and 16.

This file fixes the system's shape, the data model, the interfaces between packages, the cross-cutting algorithms and the test strategy. The component designs under `components/` (section 18) carry function-level detail and test lists.

## 1. Decisions at a glance

| Area | Decision | Why |
|---|---|---|
| Language | Go 1.27, one module, `CGO_ENABLED=0` | One static binary, cross-compiled for Linux and macOS on amd64 and arm64 with no toolchain per target. |
| Process model | One short process per command. No daemon, no background work. | Nothing to start, stop or leave stale; every answer comes from the files as they are. |
| Storage | The Markdown files. `.pbrain/` holds a lock file and the originals of imports. Every file pbrain owns is reached through an open folder handle, never through a link (section 10.3). | Spec rule 1. |
| Search engine | A parallel scan of the files on every search. No index in the lexical path. | A first measurement supports it (section 8.6); the built command is measured before the question is closed. An index would add a second source of answers. |
| Semantic search | Opt-in. An `Embedder` seam with two supported models, one SQLite database of vectors per section scored with sqlite-vec, a per-user vector cache. `pbrain sync` builds the index; `search --semantic` reads it and first catches up a small gap itself (section 8.7). | Lexical answers never depend on it, a search without `--semantic` only reads, and nothing personal leaves its section. |
| Front matter | Read through a YAML library's node tree under pbrain's own rules. Written by pbrain's own emitter. `verify` edits lines, never re-serialises. | The spec promises byte-preserving edits and a restricted, predictable subset. |
| Git | The `git` binary, read-only, a few short calls per command. | The personal boundary depends on Git's real ignore rules; a re-implementation would drift. |
| Concurrency between writers | An advisory file lock per section, held for the check-and-write step. | Spec 8.1 "take turns", also across checkouts sharing a personal folder. |
| Output | Every command builds a document of units; one renderer fits it to the budget in text or JSON. | One place enforces the limit for every command and mode. |
| Dependencies | Seven third-party modules and one for tests, all pure Go, plus the embedding runtime, also without cgo (section 15). | Small surface, no cgo. |

## 2. System shape

Every command runs the same pipeline:

```
args, env, stdin
   │
   ▼
cli        parse flags, pick the command, decide the view
   │
   ▼
brain      find the brain, read pbrain.toml and personal.toml, resolve sections, run the exposure guard
   │
   ▼
store      scan the sections in the view into a catalog of items (whole files read and validated)
   │
   ▼
graph      links: valid replaces, accepted stamps, current/replaced, successors, relates
   │
   ▼
command    search | get | list | prime | tags | check | index | add | verify | delete | import | audience | init | skill
   │
   ▼
render     fit the result to the budget, print text or JSON; errors and notices to standard error
```

A write (`add`, `verify`, `delete`, `import`, `audience set`) follows the fixed order of section 10.1: decide the section, open any input, take that section's lock, then scan, check and write inside it.

## 3. Packages

All code is under `internal/` except `cmd/pbrain/main.go`, which builds the process's `Env` (the real output streams, the Git runner, the native files and the release's model registry: sections 13 and 8.7) and calls `cli.Run`. Dependencies point downwards in this list; no package imports one above it.

| Package | Responsibility | Imports (of pbrain's own) |
|---|---|---|
| `cli` | Command table, flag parsing, global flags, exit codes, the error and notice streams. One file per command; each command is glue between the packages below. | all |
| `check` | Builds the findings of `pbrain check` from items, skipped files and a graph. | `graph`, `item`, `secretscan` |
| `semantic` | Passages, the per-section vector index, the per-user vector cache and its placement test, the incremental build, its status, and the scores a search asks for. | `search`, `embed`, `outline`, `item`, `gitx`, `sys`, `errs` |
| `embed` | The `Embedder` seam, the table of supported models, and finding, downloading and verifying a model. It downloads through an interface that `importer/fetch` satisfies. One sub-package per model, holding its adapter, imported only where the registry is built. | `sys`, `errs` |
| `embed/embedtest` | The test registry of the semantic test plan: small models with fixed vectors, their pinned bytes and a fake downloader. Imported by tests only. | `embed`, `errs` |
| `importer` | File and link imports, body cleaning, HTML to Markdown. Sub-package `importer/fetch` is the public-fetch client. | `item`, `errs` |
| `search` | Matching, match classes, ranking, the time window, per-clause counts, passage choice, and the merge of semantic results. Declares the `Corpus` it reads and the `Semantic` source it may ask. | `graph`, `item`, `query`, `outline`, `errs` |
| `render` | The budgeted document model and its text and JSON printers. | `errs` |
| `store` | Scanning sections into items; guarded input files; the write primitives: lock, publish, edit, remove, originals. | `brain`, `item`, `sys`, `errs` |
| `brain` | Discovery, configuration, sections and views, the exposure guard, the path anchor. | `audience`, `item`, `gitx`, `sys`, `errs` |
| `graph` | Link validity, cycles, currentness, successors, history order, related items, and the effect of removing an item. | `item`, `errs` |
| `audience` | The canonical URL form, the mixed-audience host table, the rule type and the audience decision. | `errs` |
| `query` | The tokenizer, query clauses, `<when>` parsing. It uses `item.Fold`, so it sits above `item`. | `item`, `errs` |
| `item` | The item model, front matter parsing and emitting, ids and slugs, kinds, actors, standing, the fold. It takes a body's first heading from `outline`, so the title fallback agrees with every other use of headings. | `outline`, `errs` |
| `outline` | The headings and sections of a body (section 7.4). | none |
| `gitx` | The read-only Git queries. | `errs` |
| `secretscan` | The four patterns. | none |
| `skill` | The embedded skill text. | none |
| `errs` | Error codes, notices, exit codes. | none |
| `sys` | The seams to the outside: file locking and link-free file operations on open folder handles. Replaceable in tests. | none |

Two packages stand outside the list. `semantic/calibrate` is the calibration tool of section 8.7, a command of its own that imports `cli` and the packages below it. `buildcheck` holds the build's own checks as tests (no replace or exclude directive in `go.mod`, the import rule of this table, the release configuration) and imports nothing of pbrain's.

`search` does not import `store`: it declares the small `Corpus` interface it needs, and the store's catalog satisfies it. Nor does it import `semantic`: it declares the `Semantic` interface, `semantic` implements it, and `cli` joins the two. Before two developers build in parallel, one change adds every exported type and signature of sections 4, 7.3, 7.4, 8.5, 8.7, 9, 10.1, 10.3 and 13 with empty bodies and proves they compile together. Three contracts are agreed in that change because several packages lean on them: what a write may read and what it may name (section 10.1), Git's environment (section 6.3) and the effect of a delete (section 7.3).

## 4. Data model

### 4.1 Items

An item is one Markdown file of the brain: one piece of knowledge with its header, cited by its id. It can be a decision, a learning, an observation, a note or a source. A `source` item stands for material from outside, as a copy or as a pointer, with or without a summary; what is reusable in such material can also be recorded as items of the other kinds.

```go
package item

type Section uint8 // Shared, Personal
type Kind uint8    // Decision, Learning, Observation, Note, Source
type Status uint8  // Provisional, Verified
type Origin uint8  // OriginNone, OriginFile, OriginLink, OriginURL

// Item is one Markdown file of the brain: one piece of knowledge with its header, cited by its id.
type Item struct {
    ID       string   // file name without ".md", as on disk
    Key      string   // ID folded for comparison (section 4.4)
    Section  Section
    Path     string   // absolute, symlinks resolved
    Header   Header
    Problems []Problem // what check reports about this file's front matter
    BodyAt   int64     // byte offset of the body in the bytes that were read
    Layout   Layout    // where the body lies and how large body and file are; set by the scan
    Version  Version   // identifies those bytes
}

// Version identifies one observed state of a file.
type Version struct {
    Size    int64
    ModTime time.Time
    Sum     uint32 // CRC-32C of the whole file as read
}

type Header struct {
    Kind         Kind
    KindRaw      string
    Title        string    // after the fallbacks of spec 2.4
    Created      time.Time
    CreatedKnown bool
    Author       string
    Source       string
    Status       Status
    StatusRaw    string    // also carries "superseded-by <file>"
    Verified     string
    Decided      string
    Replaces     []string
    Relates      []string
    Tags         []string
    StaleAfter   time.Time
    StaleSet     bool      // a field is present; unparseable counts as due
    StaleBad     bool
    Origin       Origin
    Fetched      time.Time
    SHA256       string
}

// Standing is the effective standing, derived once here and used by ranking,
// link validity, rendering and prime. The recorded fields stay as written for
// check and for --status.
//   source                         -> 1, whatever its status says
//   decision with Decided set      -> 3, whatever its status says
//   decision without Decided       -> 1, also when its status says verified
//   any other kind, Status verified -> 2
//   otherwise                      -> 1
func (h *Header) Standing() int
// Linked reports a source item with origin link.
func (h *Header) Linked() bool

// Layout says where the body lies in a file and how large both are (spec 6.2).
type Layout struct {
    BodyLine  int // the file line, from 1, on which the body starts; FileLines + 1 for an empty body
    BodyChars int // code points of the body
    FileLines int // lines of the whole file, front matter included
    FileChars int // code points of the whole file
}

// Measure counts one file in one pass. data is the whole file, already checked to be UTF-8, and
// bodyAt is what Parse returned for the same bytes. Lines are counted nowhere else.
func Measure(data []byte, bodyAt int64) Layout

// LineOf returns the file line that holds the body's code point at offset cp: Layout.BodyLine plus
// the line feeds in body before it. For cp == BodyChars it is where a next character would stand,
// not the line of a character that exists. It reads the body up to cp, so it is for one place.
func (it *Item) LineOf(body []byte, cp int) int

// Lines answers the same question for rising offsets in one forward pass over the body.
func (it *Item) Lines(body []byte) *LineCursor

// At returns the file line of offset cp. cp is never lower than in the call before.
func (c *LineCursor) At(cp int) int
```

The whole file is read and validated when it is scanned (section 7.1), so a header and its body always come from the same bytes. `Parse` keeps its signature (`components/item.md`): it returns the header and `bodyAt`. The scan in `store` then calls `item.Measure` on the same bytes with that `bodyAt` and keeps the result in `Item.Layout`, so the counts describe the version the command read and need no second read; the other callers of `Parse` and `SplitFrontMatter` (`add`, `verify`, `importer`) are untouched. In `Measure`, a line ends at a line feed, a last line without one still counts, a byte-order mark is one code point and no line, and for an empty body `BodyLine` is `FileLines + 1` whether or not the closing delimiter has a line feed. From a body offset to a file line there is one rule and two ways to ask. `search` asks once per hit, with `LineOf`. `get` and the outline ask for many rising offsets of one body, with a `LineCursor`: `get` seeds it at the start of the slice and moves it as the fitter grows the slice, and the outline walks its sections in order, so a body that is one line of 512,000 characters is still read once. Whether the catalog keeps the bodies in memory afterwards is the command's choice (section 4.3).

### 4.2 Brain, sections and views

```go
package brain

type View uint8 // SharedView, PersonalView (shared and personal together)

type Brain struct {
    Dir      string     // the brain folder, absolute, resolved
    Root     string     // the path anchor (section 4.5)
    Config   Config     // pbrain.toml
    Local    LocalConfig // personal.toml; zero value if absent
    Shared   SectionRoot
    Personal SectionRoot // Reachable is false when its folder cannot be reached
}

type SectionRoot struct {
    Section   item.Section
    Dir       string // items live under it
    DataDir   string // Dir + "/.pbrain"
    Reachable bool
}

type Config struct {
    Search, List Limits         // Limit, MaxChars
    Get, Prime   struct{ MaxChars int }
    Tags         map[string]string // declared tags; nil when there is no [tags] table
    HasTags      bool
    Semantic     struct{ Model string } // the resolved name: DefaultModel when pbrain.toml has no semantic.model, "" when it says "none" (semantic search off); checked by embed, not here
}

type LocalConfig struct {
    Path      string
    Author    string
    Audience  []audience.Rule
    ModelPath string // a copy of the model on this machine; "" when unset
}

// Open finds the brain, loads configuration and runs the exposure guard.
func Open(ctx context.Context, env Env, opt OpenOptions) (*Brain, error)
```

`Env` carries the current directory, the environment lookup, the Git runner, the clock and the file seam (`sys.Files`), so tests replace all five.

**A brain is command-scoped** (decided 2026-11-20, on a proposal from the store design). Besides the fields above, a `Brain` holds unexported state for one command: the file seam and one open folder handle per section, opened once and reused by the guard, the scan, the writes and the index. It is always used as `*Brain` and never copied. `cli` closes it when the command ends.

```go
func (b *Brain) Files() sys.Files
func (b *Brain) OpenSection(s item.Section) (sys.Dir, error) // the same handle on every call; personal_unavailable when unreachable
func (b *Brain) Close() error
```

`store.Scan` and `store.OpenInput` keep their signatures: they reach the handles through the brain.

### 4.3 The catalog and the graph

```go
package store

type BodyPolicy uint8 // KeepAll: search, check, writes. KeepNone: list, prime, tags, get.

type Catalog struct{ /* unexported */ }

func Scan(ctx context.Context, b *brain.Brain, v brain.View, keep BodyPolicy) (*Catalog, error)

func (c *Catalog) Items() []*item.Item          // every item in the view, sorted by section, then path
func (c *Catalog) Lookup(id string) []*item.Item // by folded id; more than one means ambiguous
func (c *Catalog) Skipped() []Skipped            // files no read may use, for the notice and for check
func (c *Catalog) Body(it *item.Item) ([]byte, error)
```

With `KeepAll`, `Body` returns the bytes read by the scan. With `KeepNone` it reads the file again and compares its `Version`; if the file changed since the scan it returns `ErrChanged`, and the command scans again and retries once, failing with `conflict` if the file changes again. No file is kept open between the scan and a later read.

```go
package graph

type Graph struct{ /* built once per command */ }

func Build(items []*item.Item) *Graph

func (g *Graph) Current(it *item.Item) bool
func (g *Graph) Replacement(it *item.Item) []*item.Item       // direct, oldest first, then id
func (g *Graph) CurrentSuccessors(it *item.Item) []*item.Item
func (g *Graph) History(it *item.Item) []*item.Item           // the connected lineage, in spec 7.2's order
func (g *Graph) RelatesTo(it *item.Item) []*item.Item         // resolved outgoing relates
func (g *Graph) NewerRelated(it *item.Item) []*item.Item      // current items naming it, newest first
func (g *Graph) Unresolved(it *item.Item) int
func (g *Graph) Findings() []LinkFinding                      // invalid, cycle, fork, stamp ignored, dangling
```

`NewerRelated` applies no date test. An item can only name one that already exists, so "newer" in spec 7.1 describes the link; a date set by hand must not hide the pointer.

### 4.4 Comparing ids and words

One fold is used everywhere text is compared: Unicode lower case, rune by rune (`unicode.ToLower`), with no normalisation. It serves id comparison (spec 2.3), word matching (spec 7.1) and the `.pbrain` folder-name rule (spec 5.2). A composed and a decomposed accent therefore do not match each other; that limit is recorded in section 17.

### 4.5 The path anchor

Printed paths (in `get`, write results, `delete` and `check`) are never absolute:

- The anchor is the top of the Git work tree that holds the brain, or the brain folder itself outside a work tree.
- A file under the anchor prints as its path relative to the anchor, with forward slashes: `docs/brain/decisions/<id>.md`.
- A personal item outside the anchor prints as `personal:` followed by its path relative to the personal section: `personal:notes/<id>.md`.

This keeps home-directory names out of an agent's context and makes output the same on every machine.

### 4.6 What `.pbrain/` holds

```
<section>/.pbrain/
  lock                    the writers' lock for this section (empty file); in the brain it also guards personal.toml
  originals/<sha256>      one file per original, named by its digest
  index/lock              held by a running pbrain sync for this section
  index/manifest          the fingerprint, and the database file in use with its size and CRC-32C
  index/<name>.db         a SQLite database: items, passages and their vectors (section 8.7)
```

The lexical path has no index. `index/` belongs to semantic search alone: only `pbrain sync` and the bounded catch-up of `search --semantic` write it (section 8.7), a write of an item never touches it (spec 8.1), and deleting it changes no lexical answer.

## 5. Front matter

**Reading.** `item.Parse` is given the whole file's bytes, which the scan has already checked for size and UTF-8 (section 7.1). It finds the block between the `---` lines and parses it with the YAML library's node tree, not into structs. pbrain then applies its own rules to the tree:

- The document must be a mapping. A duplicate top-level key is an error (the library keeps both nodes; pbrain detects them).
- For a known key, a scalar node gives its text exactly as written; a sequence of scalars, flow or block, gives a list. Any other shape makes that field "wrong type", treated as missing and reported by `check` as `bad_field`. YAML's own typing is ignored: `verified: true` is the actor "true".
- Unknown keys are ignored, whatever their shape.
- `status` beginning `superseded-by ` is kept raw for the legacy stamp (spec 4.3) and reads as provisional.

A file that fails any of the first checks is a `Skipped` entry with reason `bad_front_matter`.

**Writing.** `item.Emit` writes pbrain's own form: `---`, one `key: value` per line in a fixed key order, lists as `[a, b]`, LF. A scalar is written plain when that is unambiguous YAML and double-quoted otherwise; the rule is conservative (quote on a leading or trailing space, a leading indicator character, `: `, ` #`, or a value YAML would read as another type). A property test parses every emitted header back and requires the same values.

**Editing.** `item.EditVerify` is a line editor over the raw bytes. It finds the top-level `status` and `verified` lines from the node positions, requires each to be absent or a single-line scalar, replaces the value in place or inserts one line after `status` (or at the end of the block) in the file's newline style, and returns the new bytes. Any other shape returns `unsupported_edit`. No other byte changes.

## 6. Opening the brain

### 6.1 Commands that need no brain

`init`, `skill`, `--help` and `--version` are dispatched before anything below. They never call `brain.Open`, so `pbrain skill` works in an empty directory and `init` can run where no brain exists yet.

### 6.2 Discovery

As spec 10: with `--brain` or `PBRAIN_DIR` (flag first) the named folder must hold `pbrain.toml`; otherwise walk up from the current directory, at each level trying `<dir>/pbrain.toml`, then `<dir>/docs/brain/pbrain.toml`. The brain folder is then resolved through symbolic links once, and every later path is built from the resolved form.

### 6.3 Git queries

`gitx.Runner` runs the `git` binary with a ten-second limit, no pager, no optional locks and `-z` output. Paths are passed as literal paths relative to the work tree's top, never as patterns, with one fixed exception: the marker query of `init` below.

**Environment.** Every call is bound to the work tree that the resolved path lies in, and to that tree's real index. The runner starts Git with the variables that select another repository or index removed: `GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`, `GIT_COMMON_DIR`, `GIT_OBJECT_DIRECTORY`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`, `GIT_NAMESPACE` and `GIT_CEILING_DIRECTORIES`. Otherwise an inherited `GIT_INDEX_FILE` could hide a force-added personal file from the guard. Configuration is left alone, so the tree's own and the person's global ignore rules apply as they do for Git itself. A work tree is detected without Git, by a `.git` entry in the folder or a parent. Inside one, a failure to start Git, or an exit status that reports a fatal error, is `git_required`. Outside one, no call is made.

| Call | Used for |
|---|---|
| `git -C <dir> rev-parse --show-toplevel` | The anchor, and `init`'s project root. |
| `git -C <top> check-ignore --no-index -z --stdin` | Which paths an ignore rule matches. `--no-index` makes a rule count whether or not the path is tracked, as spec 2.2 requires. Exit status 0 means at least one path matched, 1 means none did, and anything else is a failure. |
| `git -C <top> ls-files -z -- <paths>` | The guard's "holds no tracked file". |
| `git -C <top> ls-files -z --cached --others --exclude-standard -- ':(glob)**/pbrain.toml'` | `init`: every `pbrain.toml` of the project, tracked, or untracked and not ignored (section 6.5). |
| `git -C <brain> config user.name` | The last author default, only when needed. It is read in the brain's folder, so the brain's repository answers first and the person's global configuration second, whatever directory the command was run from. |

The guard and the scan make separate `check-ignore` calls, because they ask different questions about different paths. A folder is always passed with a trailing slash, also when it does not exist yet: Git's directory-only rules such as `personal/` match the name only in that form. A personal folder inside another work tree gets its own calls with that tree's top. How many processes a command starts is measured (section 8.6), not promised.

### 6.4 The exposure guard

`brain.Guard` runs in `Open`, before any item is read. The protected paths are spec 5.1's: the personal section, the default `<brain>/personal/`, `personal.toml` and each section's `.pbrain/`. Each is known to be a folder or a file by its role, not by looking at the disk, so an absent one is probed in the right form. For each:

1. If it lies in a work tree: `check-ignore` must report it, also when it does not exist yet, and `ls-files` under it must be empty.
2. If it exists: a walk under it must find no entry named `.git`, at any depth, its own top included. The walk does not follow links and stops at the first hit.

Failure is `personal_exposed` with the kind of path and the fix, never an item name. An unreachable personal folder is not checked and sets `Reachable = false`; a command that needs it fails with `personal_unavailable`.

The guard establishes that personal storage is safe where it lies. It does not decide whether a given file may be copied into a shared item; that is the input-file rule of section 10.2.

### 6.5 `init`

The target is `--path`, else `--brain`, else `PBRAIN_DIR`, else `docs/brain`; a relative `--path` starts at the project root. `init` creates the folder and an empty `pbrain.toml` only where missing. For `.gitignore` it adds each of the three rules that is not already a line of the file and leaves every other line as it was; it creates the file only if there is none. It then runs the guard, which on a fresh checkout meets protected folders that do not exist yet and passes on the rules alone.

**A second brain by accident** is refused (spec 9.2, accepted 2026-11-20). The spec's walk cannot see a brain outside `docs/brain`, so inside a work tree `init` also asks Git for every `pbrain.toml` of the project, before it creates anything. If one lies in a folder other than the target, `init` fails with `invalid` and names that file by its path from the top of the work tree. A file of that name that is not a brain blocks `init` in the same way; that cost was accepted with the amendment. A nested repository is not listed by its parent, so a separate checkout inside a project still gets its own brain. Outside a work tree only the walk applies.

## 7. Scanning and links

### 7.1 The scan

`store.Scan` walks each section in the view. The two sections are discovered differently.

**The shared section.**

- Never entered: a folder whose name starts with a dot, every protected path (so the personal section and `.pbrain/` are pruned before their contents are listed), any symbolic link, and anything that is not a regular file or a folder.
- The walk goes level by level. The folders and `.md` files of one level are passed to one `check-ignore` call before any of those folders is listed. An ignored folder is dropped without ever being listed; an ignored file is dropped. A brain is two or three levels deep, so this is two or three calls.

**The personal section**, when the view includes it and it is reachable.

- Git's ignore rules are not applied: the section is ignored as a whole by design, and that must not hide its items.
- The same rules otherwise: no dot-folders, no symbolic links, regular files only.

**Every candidate file** is then opened without following a link, confirmed to be a regular file, and read whole, in parallel, on as many workers as there are CPUs:

- Over 4 MiB, not valid UTF-8 from first byte to last, or front matter that fails the rules of section 5: the file is a `Skipped` entry and contributes to no read. This is decided on the whole file, so a good header over a broken body never reaches a result or the graph.
- No front matter at all: recorded for `check`'s `not_an_item`, not an item.
- Front matter, and a file name that is not valid UTF-8: a `Skipped` entry for `check`'s `bad_file_name`, not an item (spec 2.2). The id is the name, and folding an invalid name would give it the key of another file; for the same reason a lookup with an argument that is not valid UTF-8 finds nothing.
- Otherwise it becomes an item, with the `Version` of the bytes that were read.

Results are sorted by section and path, so worker timing never changes an answer.

### 7.2 Links

`graph.Build` works in the order spec 4.2 and 4.3 give:

1. Resolve each `replaces` id to exactly one item of the same section. A missing target is a dangling link (a warning, never `replace_invalid`). An ambiguous or cross-section target is invalid.
2. Drop links that break the standing and kind rules.
3. Find cycles among the remaining links with a strongly-connected-components pass; every link inside a component of more than one item, and every self-link, is ignored.
4. Add legacy stamps as candidate links: the named file's stem must resolve to one item of the section, no valid `replaces` link may already point at the stamped item, and a stamp that would close a cycle is dropped. Stamps are judged together: the cycle test runs on valid links plus all candidate stamps, and only stamps are removed.
5. Derive: an item is replaced when a link points at it; current successors by following links forward to current items; history as the connected component, ordered so an item follows everything it replaces, ties oldest first, then id.

`relates` is resolved in the view of the item that holds the link: a shared item's links resolve among shared items only, also in the personal view (spec 5.2). The reverse map gives `NewerRelated`.

### 7.3 What removing an item changes

`delete` does not read its facts off the graph of today, because removing an item can change which links count. If B validly replaces A, and A also carries an older stamp naming C, B's link masks the stamp; deleting B wakes the stamp, and A stays replaced. So the effect is computed by comparison:

```go
package graph

type Impact struct {
    CurrentAgain []*item.Item // replaced before, current after
    Dangling     []*item.Item // hold a replaces or relates link that resolved to the target before and resolves to nothing after
}

// Effect builds the graph with and without target, by the same rules, and compares them.
func Effect(items []*item.Item, target *item.Item) Impact
```

`Dangling` covers every remaining item, old or new, current or replaced, and each link is resolved in the view of the item that holds it, as for reads. A link that resolved to nothing before is not in it. The preview and the confirmed delete both call `Effect`; what they may name is then cut to the disclosure view (section 10.1).

### 7.4 Headings and sections

Three places need the sections of a body: `get --outline` (spec 7.2), the section named with a snippet (spec 7.1) and the passages of the semantic index (section 8.7). One pure package serves all three, so a position printed by one is a position the others agree on.

```go
package outline

type Section struct {
    Level int    // 1 to 6; 0 for text before the first heading, or a body with no heading
    Title string // the heading text without its marks, trimmed; "" at level 0
    Start int    // code-point offset of the heading line's first character; 0 at level 0
    End   int    // where the next heading line starts, or the body's length: the section's own text
    Tree  int    // where the section with its subsections ends: the next heading of the same or a higher level
    ByteStart, ByteEnd int // the own-text span [Start, End) in bytes
}

func Parse(body []byte) []Section    // in body order; never fails; nil for a body with no non-space character
func At(secs []Section, pos int) int // index of the section whose [Start, End) holds pos, or -1
```

The rules are a fixed subset of Markdown, chosen so that no parser library is needed and a heading inside code never counts:

- A line ends at a line feed; a carriage return before it is ignored when a line is tested.
- **Fences.** A line that, after at most three spaces, starts with three or more backticks, or three or more tildes, opens a fence. It closes at a line that, after at most three spaces, is at least as many of the same character and then only spaces. An unclosed fence runs to the end. Nothing inside a fence is a heading.
- **ATX headings.** At most three spaces, one to six `#`, then a space, a tab or the end of the line. The title drops a closing run of `#` that follows a space.
- **Setext headings.** A non-blank text line directly followed by a line that, after at most three spaces, is only `=` (one or more: level 1) or only `-` (two or more: level 2), then spaces. The text line must not itself be a heading or a fence line, be indented four or more, or start with `-`, `*`, `+`, `>`, `|` or a digit followed by `.` or `)`. The section starts at the text line.
- A level-0 section exists only when its text has a non-space character.

`get --outline` prints, per section, `Start`, `Tree - Start`, the file line of `Start` (from one `item.LineCursor` over the sections in order) and the level marks with `Title`. A snippet names the title of the section that `At` finds for its position, when that section has a level above 0 and a title. One test holds the three uses to that agreement on a body with repeated titles, a fenced line that looks like a heading and multibyte text (section 14, "Section alignment").

## 8. Search

### 8.1 Words and queries

`query.Tokens` yields maximal runs of letters and digits (`unicode.IsLetter`, `unicode.IsDigit`), folded, each with its start and end as code-point offsets into the text. `query.Parse` turns a query into clauses:

```go
type Clause struct {
    Kind  ClauseKind // Word, Prefix, Phrase
    Words []string   // one for Word and Prefix; two or more for Phrase
    Label string     // as the caller wrote it, for the per-clause counts
    Quoted bool      // written in double quotes, a one-word phrase included: must match in the partial tier (section 8.8)
}
```

A bare term that holds separators, such as `ci-runner`, becomes one word clause per token. A term ending in `*` is a prefix: `ci-run*` is the word `ci` and the prefix `run`. A `*` anywhere else is a separator. This grammar is pbrain's own; no search library's query syntax is exposed. A double-quoted string is a phrase; a one-word phrase is a word. An unclosed quote, or no word at all, is a usage error.

### 8.2 Matching

For an item, each clause is tested against three token streams: the title, the tags (each tag tokenised, so `redis-cache` is `redis` and `cache`) and the body. A word matches an equal token, a prefix a token that starts with it, a phrase its words as consecutive tokens of one field.

```go
type Fields uint8 // InTitle | InTags | InBody, per clause

func matchItem(q []query.Clause, title, tags string, body []byte) (perClause []Fields, ok bool)
```

`ok` is true when every clause matched somewhere. The match class follows from `perClause`: 1 when every clause is in the title, 2 when every clause is in the title or the tags, 3 otherwise.

### 8.3 One search, step by step

`search` asks the scan to keep bodies, so every step below works on the bytes of one read.

1. **Eligible items.** From the catalog: the view, then lifecycle (current only, unless `--all`), then `--kind`, `--tag`, `--status` and the time window. Items the window alone removed are kept aside for the outside-window count.
2. **Match.** Title and tags are tested from the header. The body is tested only if they do not already satisfy every clause; bodies are tested in parallel, and one test stops as soon as every clause has matched.
3. **Counts.** The replaced count (replaced items that match, inside the window) and the outside-window count (current items that match, outside it or undated) come from the same pass, run over those two side sets.
4. **Order.** Sort current matches by (class, recency band, standing descending, created descending with unknown last, id, path). Then the one exception: if the first result's class holds a decision in force, the first such decision moves to the top. Replaced matches, with `--all`, are sorted the same way and appended. With `--semantic`, the results found only by meaning are then appended to that list (section 8.5), and with `--bm25` the partial matches after those (section 8.8); nothing before them moves.
5. **Page.** Take the candidates from `--offset` up to `--limit`, and keep the total number of ordered matches. The renderer may show fewer; it alone works out what is left (section 9).
6. **Passages.** For every item on the page that has a body, choose the snippet (section 8.4), whether the item matched in its body or only in its title or tags. For a body longer than `get`'s limit, name the section the snippet starts in (section 7.4). Related lines are not part of the search: the read handler takes them from the graph for the hits on the page, as it does for `get`.
7. **Nothing matched.** If no item is eligible to be returned, lexically or by meaning, the per-clause counts are taken from the `perClause` masks of step 2. A search with no match has necessarily tested every eligible body to its end, so the masks are complete.

One instant and one time zone are taken once per command (section 13) and used for the recency band and the time window alike. Standing is `Header.Standing` of section 4.1 everywhere.

### 8.4 Choosing the passage

A stretch is any run of the body of the snippet length (200 code points); a body shorter than that is one stretch. For a page item with a non-empty body, the stretch chosen is the one that holds the most distinct clauses, each as a whole occurrence, and among those the one that begins earliest. With no occurrence at all, which happens when the item matched in its title or tags only, it is the stretch at the start of the body.

One pass collects every occurrence of every clause as (start, end, clause), in code points, sorted by start. The best coverage is found with a sliding window over the occurrences. The earliest stretch reaching it begins 200 code points before the end of the earliest-ending window that has that coverage, or at 0 if that is negative: a sole match at code points 1000 to 1005 gives position 805. A stretch may therefore begin inside a word; it is not moved to a word boundary. The component design carries the exact loop and its cases.

The stretch's start is its position: a code-point offset into the stored body, the value `get --offset` takes. It is printed as it is, with line breaks as spaces and "…" where the body continues on either side. Every snippet carries that position and the file line that holds it (`item.LineOf`); the section is named only when the body is longer than `get`'s configured limit. Because the position is the stretch's start, it can lie in the section before the one that holds the match: a sole occurrence that ends fewer than 200 code points after its section starts gives a position before that section, and the section named with the snippet is always the one that holds the position. With several clauses the position is wherever the best stretch starts, which can be inside the same section. The alignment test of section 14 pins one case of each.

**Occurrences.** For each hit of the page, `search` counts the occurrences of the query's distinct clauses in the title, in each tag and in the body, by the rules BM25's `tf` uses (section 8.8): a word is an equal token, a prefix a token that starts with it, a phrase a place where its words follow one another inside one field, overlaps counted. The sum is `Hit.Occurrences`. It is computed after the page is chosen, for at most as many items as the limit, in the pass that already collects the body's occurrences for the snippet. It orders and admits nothing, and a hit found only by meaning can have 0.

**Context lines.** With `Request.Context` above 0, a snippet also carries its lines in `Snippet.Context`, in file order; without the flag it is nil.

```go
// ContextLine is one line of a snippet's context.
type ContextLine struct {
    Line   int    // its file line
    Text   string // without its line feed and without a carriage return before it;
                  // for a line of the stretch, only the part inside the stretch
    Focal  bool   // a line of the stretch
    Before bool   // focal: its line starts before the part
    After  bool   // focal: its line goes on after the part
}
```

For a stretch `[Start, End)` that is not empty, the focal lines run from the line that holds `Start` to the line that holds `End - 1`, so a stretch that ends on a line feed adds no empty line. The others are up to `Context` whole body lines before the first focal line and after the last; nothing before the body is ever a line. `search` cuts and drops nothing: the fitter does (section 9). A hit without a snippet, which is an empty body or a hit found by meaning whose closest passage is its title alone, has no offset, no line and no context.

### 8.5 The entry point and the seam

```go
package search

// Corpus is what search reads. The store's catalog satisfies it.
type Corpus interface {
    Items() []*item.Item
    Body(*item.Item) ([]byte, error)
}

// Run is the search of spec 7.1. It is one concrete function.
func Run(ctx context.Context, c Corpus, g *graph.Graph, req Request) (*Result, error)
```

`Request` holds the parsed clauses, the filters, the window, the limit and offset, the instant of evaluation, the time zone and the snippet settings. For `--semantic` it also holds `Text`, the query as written with its arguments joined by one space, and `Semantic`, the source to ask; `Semantic` is nil without the flag. `BM25` is true with `--bm25` (section 8.8). `Context` is the number of lines asked for with `--context`, 0 without it (section 8.4).

```go
type Result struct {
    Total    int   // ordered results before offset and limit: exact, found by meaning and partial together
    Offset   int   // as requested
    Hits     []Hit // from Offset, at most the limit; each with class, snippet (its offset, its file line and, with
                   // Context, its lines), section, Occurrences, Semantic bool,
                   // and for a partial match Partial bool with Matched and Clauses
    Replaced int   // replaced items that matched lexically, inside the window
    Outside  int   // current lexical matches the window left out
    Alone    []ClauseCount // only when Total is 0
    Sem      SemanticState // what the semantic part did; zero without --semantic
}

type SemanticState struct {
    Asked, Ready bool
    Reason       NotReady // why, when Asked and not Ready
    Missing      int      // items of the view that were not compared
}

// Semantic is the second source of candidates. The semantic package implements it.
type Semantic interface {
    // Score compares the query text with each eligible item that a usable index covers.
    // A missing model or index is not an error: it is Scores.Ready == false.
    Score(ctx context.Context, text string, eligible []*item.Item) (Scores, error)
}

type Scores struct {
    Ready   bool
    Reason  NotReady
    Missing int
    Cutoff  float32       // the model's cut-off
    Hits    []SemanticHit // one per eligible, covered item whose closest passage is at or just under the cut-off; in no order
}

type SemanticHit struct {
    Item       *item.Item
    Score      float32
    Start, End int // code-point span of the closest passage in the stored body
}

type NotReady uint8 // NoModel, UnsupportedModel, ModelAbsent, CacheExposed, NoIndex, ModelFailed
```

`search` never computes `more` or `next`: the renderer does, from what it actually printed. The CLI calls nothing else in `search`.

**The merge.** The lexical search runs exactly as in section 8.3 and gives the list L. If `Request.Semantic` is set, `Run` calls `Score` once with the eligible set of step 1, so the view, lifecycle, filters and window bound both parts through the same code. The added results are the hits whose score reaches `Cutoff` and whose item is not in L; a score that is not a number is dropped. They are ordered by score descending, then `created` descending with unknown last, then id, then path; with `--all` the added current items come before the added replaced ones. The result list is L followed by the added results, and `Total`, the offset and the limit apply to that list. The decision exception looks at L only. `Replaced` and `Outside` stay lexical. A hit whose span does not lie inside the body as scanned is dropped. An added hit's snippet is taken from inside its passage only: at most 200 code points from `Start`, never past `End`, so a short section never quotes the next one as its evidence. Its position is `Start`. A passage with an empty span, which is an item matched by its title alone, has no snippet.

The policy sits in `search`, above candidate generation: `semantic` returns scores and spans and decides nothing about what is shown. `cli` turns `Result.Sem` into a `semantic_not_ready` or `semantic_partial` notice.

`Corpus` remains the seam for a faster source of the same items: any such source must give `Run` the same items and bodies, so lexical results stay identical, byte for byte.

### 8.6 Why a scan first, and what is still to measure

A throwaway Go program was measured on 2026-11-20 against a synthetic brain of 5,000 files and 57 MB, on a 16-core Linux machine with the files in the page cache.

| Work | 16 workers | 4 workers | 1 worker |
|---|---|---|---|
| Read every file whole, check UTF-8, checksum it | 30 to 40 ms | 47 ms | 91 ms |
| The same, plus tokenise every body and test two words | 54 to 58 ms | 92 ms | 358 ms |

This is enough to try a scan first. It is not enough to say the target is met: the program parsed no YAML, ran no Git, built no graph, rendered nothing, and its start-up was not counted. The spec's targets are a read under 300 ms and a first read under 10 seconds, at 5,000 items and 50 MB, process start included.

So the question closes on the built command, not on this table:

- Measure `search`, `list`, `get` and `prime` as whole processes, on Linux and on macOS, with a cold and a warm file cache, on the synthetic brain as a plain checkout and with a personal section. The macOS runs are owed: macOS is deferred, and they are tracked apart from the release (section 17).
- Report the first read after a cold start separately from the reads that follow, with the number of Git processes each command started.
- Include what the scan now does for every file besides checking it: counting its lines and its code points, and those of its body (section 4.1). The throwaway program above did not count them.
- Publish the timings with the release. If a warm read takes more than 300 ms on a four-core machine, an index is reconsidered, with the scan kept as the reference its results are compared against.

**Where this list stands** (`internal/cli/README.measure.md`). Published: `search` and `get` as whole processes on Linux, on two machines, on the synthetic brain with a personal section. Their warm medians, 135 to 189 ms and 134 ms on the first machine, are inside 300 ms, so the scan stays. Not published: `list` and `prime`, which the harness can time; a cold file cache, since the first sample's cache state was left uncontrolled; the checkout with every item shared; and macOS. The counting of the third point is inside every published figure, and was then timed by itself (issue #19, the memory-profile section of that file). `item.Measure`, the one pass that counts a file's lines and code points, ran over the 5,000 files of the brain, 50 MB already in memory, on one thread of the second machine: 29.8 ms of processor time as a median. The same loop without the line count took 22.9 ms, so counting lines costs 7.0 ms over the whole brain (3.9 to 9.0 ms over 20 pairs), and the scan spreads it over its workers. Reading and parsing the files are outside that timer.

### 8.7 Semantic search

`search --semantic` adds results found by meaning after the lexical ones (spec 7.1). It is a second source of results, not a faster lexical search. Three rules bound the design: a lexical answer never depends on it; a search writes only to catch up a small gap in an index that already exists, and otherwise reads (decision 30); and everything it stores is derived from the files and can be deleted. The function-level design is in `components/semantic.md`.

```
pbrain sync         items ─► passages ─► vectors, from the index, the cache or the Embedder ─► <section>/.pbrain/index/
search --semantic   query text ─► Embedder ─► query vector ─┐
                    index ─► vectors of eligible, unchanged items ─┴─► closest passage per item ─► search: cut-off, merge
```

**The Embedder seam.**

```go
package embed

type Role uint8 // Query, Document

// An Embedder turns texts into vectors in one embedding space.
type Embedder interface {
    // Embed returns one vector per text, in order, each of Dim() values and of unit length.
    // A text longer than the model takes is cut by the embedder at a token boundary, not refused.
    Embed(ctx context.Context, role Role, texts []string) ([][]float32, error)
    Dim() int
    Fingerprint() string // equal to the Fingerprint of the Model it was opened for
    Close() error
}

// Model is one row of the table of supported models, fixed in a release.
type Model struct {
    Name        string  // as written in pbrain.toml
    Runtime     string  // which adapter opens it
    Files       []File  // Name, URL, SHA256, Size: what the adapter is given, each pinned by digest
    Source      []File  // the published files a pack is derived from, when Files are not downloaded as they are
    Dim         int
    Cutoff      float32 // calibrated on the fixture (below)
    Fingerprint string  // SHA-256 over the files' digests, the adapter's preparation version and Dim
}

// Artifact is a model read into private memory, once, and found to be the pinned one by
// hashing those very bytes. Only embed.Verify makes one.
type Artifact struct {
    Model Model
    Files [][]byte // each file's bytes, in the order of Model.Files
}

type Registry interface {
    Lookup(name string) (Model, bool)
    Names() []string
    Open(ctx context.Context, a *Artifact) (Embedder, error) // builds the model from a.Files alone and takes them over: a.Files is nil afterwards
    Derive(m Model, source [][]byte) ([][]byte, error)       // for a model with Source: the pack's bytes from the source's
}
```

pbrain passes plain text and a role; the adapter adds the model's own query and document prompts, pools and normalises. Those choices are the adapter's preparation version, so changing one changes the fingerprint. The fingerprint is known from the table without loading a model, which lets `search` and `sync --status` judge an index at no cost. The registry is a field of `cli.Env` (section 13): the release wires the built-in table and its adapters, and tests wire a fixed test model. Each supported model is one entry behind `Registry.Open`; nothing above the seam knows which runtime runs it. What a model runs on is asked at that seam and nowhere else: `Open` takes the caller's `OpenOptions`, a search passes the zero value, which is the CPU, and `pbrain sync` passes the device a person named or `auto` ("The device", below; decision 31).

**Two models are supported, and the project chooses** with `semantic.model` in `pbrain.toml` (owner's decision, 2026-11-21): `bge-small-en-v1.5` on aikit's encoder, and `embeddinggemma-2` on goinfer, which is built on aikit's kernels; both are now the project's own libraries, `github.com/pkieltyka/aikit` and `github.com/pkieltyka/goinfer`. With no `semantic.model` key the project's model is the default, `bge-small-en-v1.5`, and `"none"` turns semantic search off (decision 27 as revised, in section 16). `brain` resolves that once, when it reads the configuration (`brain.DefaultModel`), so `cli`, `semantic` and `embed` see one name and hold no default of their own; a test checks that the release registry holds the default. Each is a row of the model table with its own adapter, fingerprint, dimension and cut-off, and each index is built for one fingerprint, so changing the model means building again. The same four rules decide whether a row may be in a release. A runtime is accepted when it:

1. builds with `CGO_ENABLED=0` for all four targets and ships no shared library;
2. reproduces its model's reference vectors (cosine similarity of at least 0.999 per text);
3. checks its files against their digests, loads the model and embeds one query, all within 600 ms on a four-core laptop, so the 1 second target can hold;
4. passes the two release gates on the fixture (below).

| Model and runtime | Rule 1 | Rule 2 | Check, load and one query | First index, 5,000 items of about 500 tokens | State |
|---|---|---|---|---|---|
| `bge-small-en-v1.5` on the encoder of our aikit | yes: built for the four targets, run on linux/amd64 | yes: token ids equal, worst difference 1.7e-13 on 22 texts | 89 to 123 ms, median 93, on the built adapter at 4 threads on Linux with warm files (phase 28, five runs); 124 to 153 ms in the first check | about 14 minutes | Released at cut-off 0.65 (phase 25) and the default model (decision 27 as revised, in section 16). 384 dimensions; 512-token input. Memory on the 5,000-item, 50 MB brain (phase 28): up to 549 MiB at a search and 1,351 MiB while indexing; the first check, on a few texts, had 0.14 GB and under 0.2 GB. 134 MB of files; about 1 MB more binary. |
| `bge-small-en-v1.5` on hugot's pure-Go backend | yes | yes: token ids equal, vectors within 2.8e-10 on 22 texts | 440 to 485 ms at 4 threads | about 10 minutes (about 95 without `GOEXPERIMENT=simd`) | The fallback, not linked: accepted on its spike, then replaced. 0.60 GB at a search and under 0.9 GB while indexing; 18 to 24 MB more binary; three workarounds (below). |
| `embeddinggemma-2` on goinfer as published (v0.24.1) | yes | yes: worst difference 1.4e-12 on 22 texts | 1.34 to 1.48 s, with nothing checked | about 95 minutes | Not usable as published: fails rule 3 and loads from a folder path, so the bytes checked would not be the bytes loaded. |
| `embeddinggemma-2` on our goinfer | yes | must stay as published: the changes alter no value | 253 to 366 ms, median 281, on the built adapter at 4 threads on Linux with warm files (phase 28, five runs, on an Intel Core Ultra 7 255H); the whole one-item command takes 478 to 553 ms. On goinfer v0.5.0, on four cores of a Ryzen 9 7950X: 283 to 353 ms, median 286 (five runs). In a build with the `cuda` tag: 928 to 1,272 ms, median 1,050, on a Ryzen 9 7950X with an NVIDIA RTX PRO 6000 (five runs), which is over rule 3's 600 ms and is why a search opens the CPU in every build (decision 31; section 17) | about 95 minutes: unchanged | Released at cut-off 0.72 (phase 27) as the model a project opts into: it was the default until its first index was measured (decision 27 as revised, in section 16). Its gates passed on the fixture. Rule 3 is shown on Linux; the Mac run is owed and tracked apart from the release (section 17). 1.08 GiB at a one-item search, measured, and 1.88 GiB seen while indexing. On the 5,000-item, 50 MB brain, on the Ryzen machine: up to 1,687 MiB at a search on four cores, and 4,758 MiB while indexing in a build with the `cuda` tag. 768 dimensions. |

**The first-index column is for short items.** Its figures are estimates for 5,000 items of about 500 tokens each, 2.4 to 2.5 million tokens in all: about one passage an item. Time follows the number of passages, not of items. The brain that spec section 16 measures, 5,000 items in 50 MB, cuts into 48,511 passages, 9.7 an item, so its first index takes about ten times the column's figure. Phase 28 measured `embeddinggemma-2` on that brain on Linux on four CPUs: 0.902 passages a second, which is 92 minutes for 5,000 passages, as estimated, and about 15 hours for the whole brain, about 3.3 MB of items an hour, with a peak of 1.88 GiB of memory seen while indexing. The rate is from 772 passages early in the run, sampled over 856 seconds while it was still going. That run was not finished, so the 15 hours are a projection from it, on four CPUs of an Intel Core Ultra 7 255H; no first index of that brain has been finished with this model on CPUs alone. The generated text is token-heavy, with many invented words that split into several tokens, so ordinary notes may go faster, and a smaller brain finishes far sooner: a brain of 500 passages takes about 10 minutes. `bge-small-en-v1.5`, the default model, was then measured on the same brain and the same four CPUs (`internal/cli/README.measure.md`): 1 hour 41 minutes for the 48,511 passages, 7.99 passages a second, with a peak of 1,351 MiB; an index after one new note took 0.77 seconds. That is about nine times EmbeddingGemma's rate, where the two estimates had said six. With a GPU the larger model keeps pace with the small one. On a Ryzen 9 7950X with an NVIDIA RTX PRO 6000, using four cores, a build with the `cuda` tag finished `embeddinggemma-2`'s first index of that brain in 1 hour 10 minutes, 11.60 passages a second, where bge-small on four cores of the same machine took 1 hour 18 minutes at 10.31. It peaked at 4,758 MiB of host memory and at least 1,314 MiB on the device, sampled every three seconds; a sync after one new note took 2.39 seconds. On a brain of 32 notes and 59 short passages the same machine took 19.0 seconds for the first sync on four cores alone and 2.6 seconds with the GPU. The machine was busy with other work in all of these runs, at a one-minute load of 10 to 28.

**Two quantised files of EmbeddingGemma 2 are rows of their own, once each passes these rules** (owner's direction, 2026-11-22; issue #47). goinfer v0.6.1 opens Unsloth's Q8_0 and UD-Q6_K_XL files of the text model, 310 MB and 249 MB against a 1.52 GB download for a 552 MB pack. They would be the rows `embeddinggemma-2-q8-0` and `embeddinggemma-2-ud-q6-k-xl`, each with its own fingerprint, index, cache entries and cut-off, beside `embeddinggemma-2` and with the default unchanged (`components/semantic.md`, "The GGUF rows"). A quantised model is another model for rules 2 and 3 and for the gates: its reference is made from its own file, its cut-off is fitted for it, and a row that fails one is reported and not shipped. Nothing is assumed from the file's size: the loader widens every tensor to 32 bits, the token table included, 512 MiB where the pack holds 256 MiB, and decodes them one after another, so the memory of a search and rule 3 are measured before a row is offered. No row has a figure in the table above yet.

Not chosen: llama.cpp through purego (shared libraries unpacked at run time, 0.96 s to load), and rembed, whose EmbeddingGemma 2 code exists only in an untagged fork and fails parity on one text. The comparison is in `research/semantic-search-and-index/embedding-models-and-go-runtime/embeddinggemma2-runtime-comparison.md`.

**bge-small on aikit's encoder** (owner's decision, 2026-11-21, on `goinfer-bge-small-check.md`: one Intel laptop, single runs). aikit is the kernel module that goinfer is built on, by the same author, so both models run on one runtime family and one module serves both. It replaces hugot, which had been accepted on its spike. Its conditions are part of the design (`components/semantic.md`, "The bge-small adapter"):

- **One addition to aikit.** aikit's loader for this kind of model reads a folder by path, which the rule that a model is built from checked bytes forbids. Our aikit adds `encoder.LoadBERTFromFS`, the same load through a file system the caller gives, and an accessor for the tokenizer it loaded (merged into its `main` as `eb8c5ddd`); aikit already loads its other models that way. The adapter hands the five checked buffers over through it once, the weights are used where they lie, and no file is opened after the check.
- **The adapter does the model's own preparation**, as it did on hugot: tokenising without the wrapping tokens, the cut to 510 tokens that leaves the passage at least 255, and making literal special-token text harmless, which aikit's tokenizer would otherwise read as the real token. aikit pools on the first token, as the model's own pooling file declares, and normalises. The preparation version is unchanged: across 30 windows of our corpus the vectors from the two runtimes agree to 2.5e-10.
- **Nothing to guard.** The aikit packages that are linked read no environment variable and print nothing (checked at the release it was forked from; the loader adds none), so the `envguard` package of the earlier draft is gone. `cmd/pbrain` still points the process's output variables at the null device (section 13): that guarantee does not rest on what a library does today, and a test reads the linked sources again on every new pin (section 14).
- **No toolchain experiment.** `GOEXPERIMENT=simd` is not set; it changed nothing here. aikit's kernels are Go assembly for AVX2 and NEON with a Go fallback, and no cgo. Only linux/amd64 has been run with a model. The arm64 builds are built and their sources audited for every release; measuring them on a Mac is owed and tracked apart from the release (section 17).
- **The cost is indexing speed.** aikit has no batch call for this model, so each passage is one forward pass: about 2,800 tokens a second at 4 threads against about 4,000 on hugot, so 14 minutes against 10 for a first index of 5,000 short items (the table's column). Batching was not tried.
- **Modules and model files are pinned** by commit and digest (below, section 15 and the component design). The model row now names the safetensors weights and four small files, so its fingerprint is not the one the hugot row had. Nothing has shipped, so no index is lost.

**The fallback for bge-small is hugot's pure-Go backend.** It passed the same rules on its spike (`spike-bge-small-hugot.md`), and its adapter design, as both reviewers approved it, is kept beside the spike in `bge-small-hugot-adapter-design.md`. It is not linked. Going back would cost what the table shows and bring back three workarounds: a load through an internal `s3:` code path of hugot, the removal of the backend's `GOMLX_…` variables before any package starts, and release builds with `GOEXPERIMENT=simd`. It would be taken only if our aikit could no longer be kept. It would change one adapter, one row of the model table and the rows of section 15, and nothing above the seam.

**EmbeddingGemma 2 on our goinfer** (owner's choice among three options, 2026-11-21: fork goinfer and hold it to the rules; not a looser budget for this model). goinfer is correct and pure Go, reads no environment variable and prints nothing; what it lacks is a fast start and a load from checked bytes. It gets three changes, measured or estimated in `goinfer-architecture-review.md`:

1. **A tokenizer that loads in tens of milliseconds: done.** The 30 MB `tokenizer.json` took 441 to 502 ms to parse. Our goinfer now builds a binary form of it once (`tokenizer.BuildPack`, 9,581,663 bytes, the same bytes every time) and loads that in 24 to 26 ms (`tokenizer.LoadPack`), from bytes, opening no file. Token ids are identical on goinfer's 48 reference texts, on 10,000 random encodings in both special-token modes, and on 3,052 texts against the Hugging Face tokenizer itself. It is merged into our goinfer's `main` (`2e413ee1`), reported in `goinfer-tokenizer-spike.md`. This was the unproven part and the fallbacks written for it are not needed.
2. **A loader that builds the model from bytes**, keeping the token-embedding table in its stored 16-bit form and widening one row per token, and widening the other tensors once, in parallel, with no second copy; and a constructor for the encoder from an already built model, tokenizer and prompt table, since the published one reads a folder. **Done**, in goinfer v0.2.0 (`a759890b`): `embeddinggemma2.NewModel` and `NewEncoder`. Measured on the real checkpoint at 4 threads: the weights are ready in 41 ms, against 693 ms for the loader by path on the same warm machine; the 48 reference texts give the reference's ids and a worst cosine of 0.999999989, and the vectors equal the published loader's bit for bit on the 28 texts compared.
3. **A text-only model pack.** The published file is 1.49 GB, of which the text part is 542 MB; reading and hashing 1.49 GB took 1.2 to 1.6 s. `pbrain sync` downloads the pinned published files once (the row's `Source`), checks their digests, and writes the pack (the row's `Files`: the token table, the other weights, the prebuilt tokenizer, the configuration) by a fixed procedure, so the pack's digests are known in advance and pinned in the release too. Every later open checks the pack, as for any model. `embed.Verify` reads and hashes a model's files concurrently: 542 MB took about 140 ms with four readers.

**Our libraries, their pins, and how they are built and checked.**

- **Ours, and diverged** (owner's decision, 2026-11-21). aikit and goinfer are one author's two modules (`github.com/townsendmerino/…`, MIT). The project now holds its own copy of each and takes it where pbrain needs: `github.com/pkieltyka/aikit`, forked at the release v1.78.0 (`b719bbda`), and `github.com/pkieltyka/goinfer`, forked at `e0d66171`. Each declares its own module path, so pbrain requires both directly and its `go.mod` needs no replace directive. Their licence and notice files keep the original copyright, with a line that names the origin. The owner plans to remove what is not used and to optimise inside them later; nothing is removed now beyond what the rename needs.
- **The pins.** `go.mod` requires `github.com/pkieltyka/aikit v1.79.1` and `github.com/pkieltyka/goinfer v0.6.1` (owner's decisions, 2026-11-21 and 2026-11-22; the move from v0.5.0 is issue #48 and is told at the end of this point), and `go.sum` pins their content. A pin is always a tag on our `main`. aikit v1.79.0 is the tag on the merge that renames the module (`3c0896c0`); it holds the loader from bytes. v1.79.1 (`0bbc048`) releases the fork's later work, which is all outside what pbrain links: the nested modules' requirements, the `gpupins` tool and the documents. No Go or assembly file of a package pbrain links differs between the two tags, so the adapters run the same code; the number was unused in both repositories, and the proxy's copy was checked against the commit, file for file and by its `h1` checksum, before the pin moved. goinfer started its own numbering at v0.1.0, the tag on the merge of its rename (`bdf9daf9`), which holds the tokenizer pack. v0.2.0 (`a759890b`) adds the loader from bytes and the encoder's constructor, which is what the EmbeddingGemma 2 adapter needs. v0.4.1 (`e33aa3893`) is our `main` after a deliberate merge of the original's `main` (`36dbfbaf1`) and the fork's own documents; pbrain pinned it next, and the adapter passed its pinned-file tests on it unchanged. v0.5.0 (`2b1379446`) is our `main` 33 commits later, all made in the fork, with no merge of the original. In the packages pbrain links it adds two calls to the text encoder, `EmbedIDs` and `EmbedIDsBatch`, which embed token ids the caller has prepared. The adapter uses both, so pbrain no longer builds on an earlier tag. The same release holds the CUDA backend, a module of its own nested in goinfer's repository: `go.mod` requires `github.com/pkieltyka/goinfer/backend/cuda v0.5.0` (`e7482f8ad`, the next commit, which changes only that module's `go.mod` and `go.sum`). Only a build with the `cuda` tag links that module, and with it the two outside modules it brings, `github.com/eitamring/gocudrv v0.3.2` (MIT) and `github.com/ebitengine/purego v0.10.1` (Apache 2.0). A build without the tag links none of the three. That is what `go install` gives unless a tag is passed, what the macOS archives hold, and what the Linux archives named `_cpu` hold; the Linux archives without that suffix are built with the tag, and the amd64 one with the `vulkan` tag as well (decisions 10 and 31). **v0.4.0 must never be required.** A tag of the original with that name reached our repository by mistake, and the module proxy served it once: the proxy keeps that content for good (`806ef38b4`, whose `go.mod` declares the original's module path), so the number is spent and v0.4.1 is the same commit under a fresh one. Before the pin moved, the proxy's v0.4.1 was checked against the commit, file for file and by its `h1` checksum. For v0.5.0 and the CUDA module the check was made after the pin moved: the copies that `go.sum` pins equal the tagged commits file for file. (A module's copy leaves out the modules nested under it, and the CUDA module's copy gains the repository's licence file; Go packs them so.) The adapter's parity test passed on v0.5.0 after the pin moved, with the model present (pbrain `b79b9aa`): a minimum cosine of 0.99999999999862 over 22 raw and 22 prepared texts, at 768 and 512 dimensions. goinfer v0.4.1, v0.5.0 and the CUDA module each require aikit v1.79.0. goinfer requires our aikit itself; Go links one copy of a module, the newer of the versions required, so pbrain's requirement of aikit is kept at or after the one in the goinfer release it pins, and the parity tests run on what is linked. pbrain imports goinfer's text encoder and tokenizer packages and aikit's `encoder` and `embed`, and nothing else of either; a build with the `cuda` tag imports the CUDA backend module as well. **v0.6.1** (`e2324268a`; owner's direction, 2026-11-22; issue #48) is the pin after v0.5.0. Between the two, in what pbrain links, goinfer gained a loader for a GGUF file of the text model (v0.6.0) and, in v0.6.1, that loader from a reader, `LoadGGUFEncoderReaderAt`, which keeps neither the reader nor its bytes, and `Encoder.Tokenizer`; v0.6.0 also brought a second backend module, `github.com/pkieltyka/goinfer/backend/vulkan`, for AMD cards on Linux amd64. `go.mod` requires both backend modules at v0.6.1. Their tags are one commit, `e4b58e96f`, the child of the root's, and it changes only the two modules' `go.mod` and `go.sum`, to require the released root; the owner accepted two commits under one version for this pin, and `pkieltyka/goinfer` issue #41 asks for one. The CUDA module's code is that of v0.5.0: only a test file and the module's own two files differ. The Vulkan module is linked only by a build with the `vulkan` tag on Linux amd64, and brings no outside module but purego, at the version CUDA already brings. goinfer published v0.6.0 and v0.6.1 with its own device and parity runs deferred, so the pin moves on pbrain's evidence alone: `TestGemmaParity` with the model present, and all 59 fixture passages agreeing between the CPU and CUDA at 0.999 or better, both on v0.6.1.
- **Changes** are made on a topic branch and reach our `main` through a reviewed pull request in our repository; then pbrain moves its pin. Each new pin is re-read and must pass the tests below. Offering a change to the original author is optional and is the owner's step: two are open, `townsendmerino/aikit#38` for the loader and `townsendmerino/goinfer#1194` for the tokenizer pack. The original author's later work, which comes at hundreds of commits a day with no compatibility promise, is taken only by a deliberate merge into our `main`, read and tested like any change; after the rename such a merge touches every import line. What was first measured are the releases goinfer v0.24.1 and aikit v1.78.0, so parity is shown again on each pinned commit.
- **The checks** that admit it to the table, all in the repository: token ids and vectors against stored references from sentence-transformers on the 22 texts (rule 2); the timing of check, load and one query on a four-core machine, on Linux, and on a Mac once one is available (rule 3; section 17); the adapter contract of `components/semantic.md`, where a literal `<eos>` in a body must not become the real token and an over-long input is cut before the call, since goinfer refuses one; and the fixture's two gates (rule 4). For the loader our aikit carries its own tests (the same vectors as the loader by path on the bge-small files, and every file read once), and pbrain's adapter tests run against the pinned commit under the same four rules.

Two costs come with the model whatever the runtime, and are stated to whoever chooses it: a first index at about 0.9 passages a second on four CPUs, which is about 95 minutes for 5,000 short items and a projected 15 hours for the 50 MB brain of spec section 16 (above), unless the machine has an NVIDIA GPU and a build that uses it, which finished that brain in 1 hour 10 minutes; and a result of 9 right of 20 on our reworded questions against 13 for bge-small. The fixture, not this plan, shows which model serves a project better.

**Passages.** An item is embedded as passages, cut by one fixed rule so that the same text always gives the same passages:

- A passage is the own text of one section of the body (section 7.4), heading line included. A section with nothing under its heading gives none.
- A section longer than 1,800 code points is split at paragraph breaks into parts of about 1,400, a fenced block counting as one paragraph. A single paragraph longer than 1,800 is split at a line break, else at a space, else at 1,800. Parts do not overlap.
- An item with no passage, such as a linked item without a summary, gets one passage of its title alone.
- What is embedded is the item's title, the titles of the section and the sections above it, and the passage text. The title is cut at 200 code points and the line of section titles keeps its last 200, so the model always sees the text it is scored for. What is recorded is the passage's span in the stored body, in code points: the value a snippet and `get --offset` use.

Sizes are counted in code points, not model tokens, so cutting needs no model and no tokenizer: a build whose vectors all come from the cache loads nothing. 1,400 code points is about 400 tokens of engineering prose. A dense passage can exceed a small model's input. The embedder then cuts it, keeping at least half of its input for the passage text, and the vector speaks for the passage's beginning, which is also what the snippet shows; the fixture decides whether the sizes stay. The sizes and the layout of the embedded text are the chunker's version, part of the index fingerprint.

**The index: one SQLite database per section, with sqlite-vec** (owner's decision, 2026-11-21, replacing the flat files of the earlier draft). The engine is `modernc.org/sqlite`, a pure-Go build of SQLite 3.54 that bundles sqlite-vec 0.1.9, so there is still no cgo. The database is `.pbrain/index/<name>.db`; a small manifest beside it names the file in use.

```sql
CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL) WITHOUT ROWID;
    -- format, fingerprint, model, dim
CREATE TABLE item (
    id      INTEGER PRIMARY KEY,
    path    TEXT NOT NULL UNIQUE,                        -- in the section, forward slashes
    content BLOB NOT NULL CHECK (length(content) = 32),  -- SHA-256 of what the passages were cut from
    crc     INTEGER NOT NULL CHECK (crc BETWEEN 0 AND 4294967295), -- CRC-32C of the same bytes
    size    INTEGER NOT NULL CHECK (size >= 8)           -- and their count: what a search compares
);
CREATE TABLE passage (
    id      INTEGER PRIMARY KEY,
    item_id INTEGER NOT NULL REFERENCES item(id) ON DELETE CASCADE,
    ord     INTEGER NOT NULL,                            -- order in the item
    start   INTEGER NOT NULL,                            -- code-point span in the stored body
    stop    INTEGER NOT NULL CHECK (stop >= start),
    digest  BLOB NOT NULL CHECK (length(digest) = 32),   -- SHA-256 of the embedded text
    vector  BLOB NOT NULL CHECK (length(vector) = 1536), -- dim float32 values; the literal is 4 x dim
    UNIQUE (item_id, ord)
);
CREATE INDEX passage_digest ON passage(digest);
```

**Scoring is one query with sqlite-vec's distance function**, the cut-off applied inside it, over the items that may be scored and no others:

```sql
-- eligible: a temporary table of item ids, filled for this search
SELECT item_id, ord, start, stop, d
FROM (SELECT p.item_id, p.ord, p.start, p.stop, vec_distance_cosine(p.vector, ?1) AS d
      FROM temp.eligible e CROSS JOIN passage p ON p.item_id = e.id)
WHERE d <= ?2;   -- ?1 the query vector, ?2 = 1 - cut-off, loosened by 0.000001
```

`semantic` first puts into `eligible` the ids of the items that are eligible for this search and whose text in the index matches the scan. The join starts from that table (`CROSS JOIN` fixes the order in SQLite), so the distance function is never called on another item's vector: an item that is filtered out or out of date is not scored at all, and a bad row belonging to one cannot fail the query. The query returns every passage at or above the cut-off, with no limit on their number; `semantic` keeps the closest passage of each item, and `search` applies the cut-off exactly. Vectors are kept in an ordinary table, not in sqlite-vec's `vec0` virtual table, because of what pbrain asks: every item above a cut-off, each by its best passage. `vec0` answers "the nearest k" and refuses a k above 4,096; read row by row for every distance it is far slower.

| Way of scoring, measured in scratch on 2026-11-21, 4 threads, a shared machine | 35,000 × 384 | 50,000 × 768 |
|---|---|---|
| **Ordinary table, `vec_distance_cosine`, joined from the eligible ids, cut-off in the query (chosen)** | 39 to 43 ms with every item eligible; 2 to 4 ms with a tenth | 68 to 74 ms; 8 ms with a tenth |
| The same over the whole table, with no eligible set: fails if any row is bad | 23 to 40 ms | 52 to 61 ms |
| The same, every row returned and grouped in Go | 53 to 58 ms | 96 to 99 ms |
| `vec0`, nearest 4,096 joined to their passages | 60 ms | 96 ms |
| `vec0`, nearest 20 (the earlier research measured 57 to 60 ms at 50,000 × 768) | 20 to 36 ms | 46 to 82 ms |
| `vec0`, every row's distance | 400 to 450 ms | about 6 s |
| Flat files scored in Go (the earlier draft; research measurement, one core) | not measured | 39 ms |

**A database file is written once and then only read.** SQLite never opens a file itself:

- **A search** reads the manifest, reads the database file whole through the section's folder handle, checks its size and CRC-32C against the manifest, and hands the bytes to SQLite as an in-memory database (`Deserialize`). The manifest is trusted for nothing before it is checked: it must be small (at most 4 KB, and no more than that is read), the database name must have the generated form, sixteen lower-case hexadecimal characters and `.db`, the checksum must be 8 hexadecimal characters, and the size must lie between one byte and 2 GiB and equal the size the open file itself reports, all before anything is allocated. A build whose database would pass 2 GiB fails with `too_large` and leaves the previous index in force; that bound is on stored bytes, far above the design size, and caps no result. Only names of that form are ever opened or removed, so no manifest, however damaged, can point a build at its own lock file. No path is given to SQLite, so the handle rule of section 10.3 holds unchanged, and nothing is created: no journal, no write-ahead log, no shared-memory file. Reading the index, a search still writes nothing and takes no lock; only its catch-up, below, does.
- **The build** works on the same kind of in-memory database. Each item is replaced in one transaction: its old rows are deleted and its new ones inserted, so a half-indexed item never exists. To publish, it serialises the database, writes it to a temporary file through the folder handle, flushes it, links it under a new random name, and replaces the manifest by a rename; then it removes the previous file. A reader that finds its file gone reads the manifest once more.
- **No write-ahead log, and no journal on disk.** These in-memory databases keep their rollback journal in memory (`journal_mode=MEMORY`), so a transaction that fails is rolled back properly and still no file is created; temporary storage is in memory too, and foreign keys are on, which the cascade from an item to its passages needs. A write-ahead log would have a search create and write a shared-memory file, needs sidecar files that SQLite opens by name, and does not work on network file systems; a database opened in place would need SQLite's own file locks and a reader could meet a half-finished commit after a crash. Whole-file replacement gives the same guarantees as before with none of that, at the cost of rewriting the file at each publication.
- **The locks.** `index/lock` still serialises builds of one section, held for the whole run. Readers and item writers never wait for it. Two transactions never meet: only the build writes, and only in its own memory.
- **Validation on read.** The whole-file checksum rejects a file damaged on disk before SQLite sees it. It is CRC-32C since format 3 (owner's decision, 2026-11-21) and was SHA-256 before. The check runs on every search, and it was never a defence against someone who can write the folder, who can replace the manifest too; so the stronger hash bought nothing here. Then the meta rows must name format 3, the expected fingerprint and dimension, and the scoring statement is prepared, which fails if a table or column it needs is missing. Each returned span is checked against the body as scanned before it is used. The table's checks reject a vector of the wrong length when it is written.
- **A failure later on.** If the scoring query or the reading of its rows fails all the same, that section's results are dropped as a whole, the section counts as unreadable for this search, and its items are counted as left out; other sections stand. A search does not look for such damage in advance. The status and the build do: when they open an index they also check every stored passage (each vector a blob of the right length, each span in order, each passage belonging to an item) and run the scoring query once over all items. An index that fails is reported as unreadable by the status and built afresh by the next `pbrain sync`, even when no item has changed, and also in a section that has no item left, where the build publishes an empty database. So a damaged vector cannot fail every search for good while the status says all is well.

| Cost at the index's full size, measured in scratch | 35,000 × 384, 57 to 68 MB | 50,000 × 768, 154 to 195 MB |
|---|---|---|
| Read the file | 27 to 44 ms | 68 to 94 ms |
| The whole-file check, as SHA-256 (formats up to 2) | 17 to 23 ms | 41 to 57 ms |
| Hand to SQLite and read the item table | 30 to 37 ms | 75 to 99 ms |
| The scoring query | 39 to 43 ms | 68 to 74 ms |
| Serialise, write and flush, per publication | about 160 ms | about 320 ms |

Since format 3 the whole-file check is CRC-32C. One core of a Ryzen 9 7950X checks 80 MB in 6 ms that way, against 30 ms for SHA-256, in a scratch run of 2026-11-21; the built command has not been timed with it. Against the flat files, a search pays about 50 to 60 ms more at our size, and the binary grows by about 7 MB. What the database gives in return is a schema that tools can inspect, transactions while building, and a distance function that is not ours to maintain.

**Change detection.** Every command already scans the files. An index entry is used only when a live item of that section has the entry's path and the same content check: the CRC-32C and the byte count of the title's length, the title and the body exactly as this command scanned them, the same projection the build cut its passages from (owner's decision, 2026-11-21). Anything else in the index is ignored. So a deleted or edited item drops out of the semantic part by itself, without any write step. A change to front matter alone, such as `verify`, leaves the entry valid, because no passage depends on it; that is why the `Version` of section 4.1, a CRC-32C and a size of the whole file, is not used for this. The entry also stores the SHA-256 of that projection, as before. The comparison never computes it, in a search, in the status or in a build. It is computed only for an item that is about to be cut, new or changed: by the build, and by a search's catch-up for the few items it takes on (below). The same step computes the SHA-256 of each passage's text, which stays the key for reusing a vector from the database or the cache. Until format 3 the comparison itself was by SHA-256 and cost every semantic search a hash of every body, 18.5 ms for 50 MB on one core in the scratch run above; the CRC-32C takes 3.8 ms. The price is a known limit: an edit that keeps the byte count and is not confined to four consecutive bytes passes unseen once in 2^32, and a vector made from the old text then scores for the item until it is edited again (section 17).

**The build** (`pbrain sync`) is the writer that can do everything; a search's catch-up, after it, is the other. Per section, under `index/lock`:

1. Read the manifest and the database it names. A missing, unreadable or other-fingerprint index starts as a new empty database, and files the manifest does not name are removed.
2. Compare with the scan, by path and content check: items without a matching row are to do; rows whose path has no item are dead, and are deleted only at the end, after step 4 has looked for reusable vectors, so that an item that was only renamed is not embedded again.
3. Make sure of the model, whenever the section has an item and whether or not anything will be embedded: find it, download it if it is absent and no `model_path` is set (and derive its pack, for a model that has one), and check its bytes against the pinned digests. A run that reuses every vector therefore still restores a model whose cache was deleted, and still refuses a wrong local copy, so after any successful run a search finds the model.
4. For each item to do, hash its title and body for the entry, cut its passages and look for each passage's vector by its text digest, the SHA-256 of the embedded text: first in the database, then, for the shared section, in the per-user cache. Only what is found nowhere is embedded, a window at a time. The lookup is ordered so that its answer cannot depend on the window: the item's own rows, then rows of items this run does not replace, then what an earlier passage of this run was given, then the cache; the old rows of another item that this run replaces are never read (`components/semantic.md`, "Reuse by digest"). The inference runtime is opened when the first such passage appears, so a run with nothing to embed opens none.
5. Publish every 20 seconds, at an item boundary, and at the end. An interrupted run loses what was not yet published: the window of passages being embedded, 128 for bge-small and 256 on a GPU, and up to 20 seconds of items stored after the last publication. For the shared section it loses only that window's embedding, since each new vector is put in the cache when its window returns; a long personal source that is interrupted is embedded again from its start.

One new short note costs a scan, a model load, one passage and one publication: about 0.6 seconds with bge-small at the index's full size, by adding up parts measured apart (start-up and the scan about 0.2 s, the index read about 0.1 s, the model checked and loaded about 0.12 s, the passage under 0.05 s, the publication about 0.16 s). A build takes its section's index lock and, while it makes sure of the model, that model's lock in the cache, in that order (section 10.4). It takes no item lock, so readers and writers of items never wait for it.

**The device** (owner's direction, 2026-11-22; decision 31; `components/semantic.md`, "Devices"; the plan and its figures are `docs/plans/sync-throughput.md`). For a model that has a GPU path, the GPU is the primary path of a build and the CPU a fallback that still works and says so.

- **Who uses what.** A search opens the model on the CPU, always. Check, load and one query take EmbeddingGemma 2 286 ms on the CPU and 1,050 ms with CUDA's start-up, so a GPU can only cost a search time, and rule 3 holds in every build. A build asks for a device: `--device`, else `PBRAIN_DEVICE`, else `auto`. These are per-machine settings, so neither configuration file holds them.
- **Naming.** `cpu`, or `<backend>:<index>` with the backend's name as goinfer lists it. A named device is used or the command fails; it never gives way to another. `auto` tries the GPUs this binary can use, a card with memory of its own before one of unknown kind before one that shares the host's, a live free figure before an estimate, then the most free memory, and at last the CPU with a `cpu_fallback` notice. `pbrain devices` lists them without a brain or a model. An AMD card is the exception while goinfer cannot say which cards it has without loading a model (`pkieltyka/goinfer` issue #29; operator's decision, 2026-11-22): its backend is named alone, `vulkan`, with no index; `--device vulkan` attaches the machine's one discrete AMD card or fails with goinfer's reason; `auto` tries it last, after every listed GPU, and only when none of them attached; and `pbrain devices` prints a line that says the card is detected when a sync attaches it.
- **Batches.** The build gives an embedder as many texts a call as it asks for, 8 if it does not ask. bge-small asks for 128 and runs them on up to 32 workers, one forward pass each: one forward spread over the cores gains 39% from 4 cores to 16, where 16 workers give 2.12 times that. EmbeddingGemma 2 on a GPU asks for 256 and packs them into forward passes by token count, under a budget of 30% of the device's free memory and at most 6 GiB; one text alone always runs. When the device runs out of memory the batch is halved and tried again, and when one text alone does not fit, an `auto` build finishes on the CPU.
- **What it does not promise.** A faster GPU build of long passages. With the device busy 96% of the time, eight passages a call gave 7.7% over two; the limit is how fast goinfer's CUDA path runs a long input, 6,500 tokens a second at 533 tokens a passage against 37,709 with 128 short texts a call. That, and goinfer's cap of 8,192 positions a batch, which keeps any budget over about 1.1 GB from being spent, are asked of goinfer (`pkieltyka/goinfer` issues #23 to #30).
- **Sameness.** What is stored and counted follows from the scan, at any window, worker count or budget. On one device the vectors are the same bytes too, because neither runtime lets a text's vector depend on its batch: aikit, and goinfer on the CPU and on CUDA. The Vulkan backend has to show that, and rule 2 against the CPU, on an AMD card before it makes a model's vectors (`components/semantic.md`, "Fingerprints"). Across devices they differ in their last bits, inside rule 2, under one fingerprint; the index records no device, and one index may hold vectors from several.
- **Not built.** Overlapping the cut, the model and the store in one build (issue #42): tokenizing, storing and publishing were 0.25 s of 413 s of CPU, and it waits for a measurement that shows the device idle more than 3% of the time. Two GPUs at once. A GPU path for bge-small.

**Catch-up in a search** (owner's decision, 2026-11-21; decision 30). A search with `--semantic` already has, by the time it compares, everything a small build needs: the scan with its bodies, each section's index in memory, and the model open for the query. So before it scores it does a build's steps 2, 4 and 5 for the few items no entry covers, within fixed bounds, and scores from the result.

- **Where.** Only in a section whose index is usable, and only when the model answered the query. No index, another fingerprint, an unreadable file or a missing model stay with `pbrain sync`, with the notices they have today.
- **How much.** At most 20 uncovered items in the view, and at most 20 passages to embed after the database and, for the shared section, the cache have been asked for each passage's vector. Over either bound, nothing is written. No embedding call starts later than 2 seconds after the first lock is taken; items finished by then are saved, the others stay uncovered.
- **The lock.** `index/lock`, the build's lock, by one attempt that never waits (`Files.TryLock`). Held by a build or by another search: no catch-up for that section. With the lock held, the manifest is read again and must be the one the index was opened from; if a build has published in between, the search does not write, because its copy is the older one. The item lock is not taken, as in a build.
- **What is written.** The same things a build writes, by the same code: each item replaced in one transaction, one publication per section at the end, and for the shared section each new vector into the cache as it is made. Rows of items that are gone are left for the build to remove. Nothing is created that a build or a download would have had to create first: not `.pbrain/`, not `index/`, not the cache folder.
- **What it answers.** The scoring runs on the in-memory database with the new rows, whether or not the publication succeeded. Every failure in between, the lock busy, a file system that refuses the write, a bad vector, is silent: the items it could not add are counted in `Missing`, as before.
- **What it costs.** For one short note: one passage to embed and one publication, about 0.1 and 0.2 seconds by the figures above, so about 0.3 seconds on top of the search. For the full bound: about 2 seconds of embedding and one publication. Neither has been measured on the built command. A search that catches up is outside the 1 second target.
- **What the old rule guarded, and what stands in its place.** A search that only read could run anywhere, and two of them could never disagree. Now a search with the flag can write in `index/` and the cache, so two searches can differ when one of them could not get the lock or ran out of time; the notice says which. A search without the flag is untouched, and it stays the proof that lexical answers need nothing local.

**The per-user cache: adopted, for shared passages only.** `<cache>/vectors/<fingerprint>/` holds one small file per passage vector, named by the SHA-256 of the embedded text. A file is created by link under its final name, so concurrent builds need no lock and a crash leaves nothing half-written. The cache stays plain files and is not a SQLite database: several builds, in several projects, write it at the same moment, and a database would bring back the file locking and sidecar files that the index avoids. The cache makes a new work tree's or a fresh clone's index cost a file read per passage instead of minutes of embedding, which matters because `.pbrain/` is per checkout and factory works in many work trees. Personal vectors are never written there: a new work tree has no personal items to save time on, and a personal folder shared between checkouts brings its own `.pbrain/index/`. That keeps spec 5.1's rule without a new protected place. The cache also holds the downloaded model, under `<cache>/models/<name>-<digest>/`, where the digest covers the names and digests of the model's files, so two releases never share a folder and a file there is never replaced once it has its name. `model_path` names a folder with the same file names, all of them: a missing companion file makes the model absent. The download goes through `fetch.Client.Download` (`components/importer.md`): https on every hop, the import's address check, the pinned size as a ceiling and an idle timeout in place of an import's 30-second total. `embed` hashes what arrived and gives the file its name only when the digest is the pinned one. Before anything is created in it, the cache folder is tested as the exposure guard tests a protected path (section 6.4): the work tree is found from its nearest existing parent, the planned folder itself is asked of Git, and it may lie in neither the brain nor the personal section. Two projects of one person share this cache, and each build holds only its own section's lock. So downloading, repackaging, repairing and cleaning up a model are done under a lock file of that model in the cache, taken after the section's index lock and never before it, with a wait the caller can cancel; a build that gets the lock looks again for a finished model before doing anything. A search takes no model lock and reads only finished model files. The cache is never pruned in v1; deleting it is always safe.

**The query.** `semantic.Source` implements `search.Semantic`. It answers "not ready" from cheap checks, in this order: semantic search turned off, a name this release does not know, the cache exposed, a file of the model absent or of the wrong size, no usable index in the view. Otherwise it starts work at once, in the background: it reads every file of the model once into its own memory, hashes those bytes against the pinned digest, hands the same bytes to the adapter, and embeds the query. The bytes that were checked are the bytes that are loaded: nothing on disk is read again, so neither a file swapped by name nor one changed in place after the check can reach the model. `cli` owns that task. It starts the source with the query text before the scan, so the two overlap; hands it the scanned items afterwards; and closes it when the command ends, on success or on any error, which cancels the work and closes an opened embedder exactly once. Between the scan and the search, `cli` calls the source's `CatchUp`, which does the bounded catch-up described above. `Score` only joins the task and compares. A model is never opened, by a search or by a build, from bytes that were not checked in the same process, so a wrong or damaged file of the right size cannot put a query into another embedding space under a valid fingerprint; it makes the semantic part not ready, and the lexical results stand. The same background task reads, checks and opens each section's database, which needs no scan. `Score` then decides which items the index covers, runs the scoring query once per section, drops the rows of items that are not eligible or not covered, and keeps for each item its closest passage; the score is one minus the distance, the cosine similarity. Sections are judged one by one: a missing or unreadable index of one section only adds its items to `Missing`.

**The cut-off and the release gates.** A vector search always has a nearest item, so abstention needs an absolute cut-off per model. It is set and checked by a tool in the repository, on the evaluation fixture (section 14). The fixture is frozen before any tuning: every question has an id, a kind (answerable with its expected item ids, or unanswerable), and, for the unanswerable ones, a fixed half, calibration or held-out. It needs at least 30 answerable questions worded unlike their answers and at least 30 unanswerable ones in each half.

1. **The cut-off.** On the calibration half, take for each question the highest score among items that do not match lexically. The scores are collected with no cut-off in force: the tool uses a copy of the model's row whose cut-off is −1, so the scoring query returns every passage and nothing below a provisional value is missing. An item counts as matching lexically if it is anywhere in the exact result list, taken whole, not only on its first page. A question with no item left to score contributes no score, which reaches no cut-off. The cut-off is the lowest two-decimal value that at most 5% of those scores reach, where a score equal to the cut-off reaches it, as in a search. This is an estimate, used only to choose the number.
2. **Abstention gate.** Each held-out question is run as the real command, `search --semantic` with default limits, and its whole page is judged: it passes only with no result at all, lexical or added. At least 95% must pass, and every question the fixture marks as a regression case must pass. The report also says how many failures already had a lexical result.
3. **Gain gate.** Each answerable question is run as the real command with and without the flag. More questions must show an expected id on their first page with the flag than without, and none that showed one without the flag may lose it with it, which a tighter page could cause.

4. **The combined gate.** Where a release also has the partial tier, the abstention gate is run again for each model with both flags together, with the same threshold and the same regression cases. Two modes can each pass alone and fail together: on 40 held-out questions, 38 empty pages for one and 38 for the other can leave only 36 empty with both.

A model enters the table only with its gates passed at that cut-off; if they do not pass, the model stays out and no other cut-off is tried by hand. Its cut-off is a constant of that release. There is no setting for it.

**The speed budget** for one search with `--semantic` at 5,000 items, warm, with bge-small:

| Step | Budget | Basis |
|---|---|---|
| Process start, configuration, Git, the guard | 50 ms | not yet measured on the built command |
| Scan and lexical search | 150 ms | section 8.6, extrapolated |
| Beside the scan: read and hash the model's files, load it, embed the query | 155 ms | measured in the check: 124 to 153 ms at 4 threads, with nothing else running |
| Beside the scan: read, check and open the index database | 110 ms | measured in scratch at 35,000 × 384: 74 to 104 ms |
| Checksum the scanned titles and bodies | 30 ms | budgeted for SHA-256 of 50 MB across the scan's workers, an estimate; with the CRC-32C of format 3, 3.8 ms on one core in scratch |
| The scoring query | 45 ms | measured in scratch: 39 to 43 ms with every item eligible |
| Merge, snippets, render | 40 ms | an estimate |

Three things run side by side after 50 ms of start-up: the scan with its hashing (150 and 30 ms), the model (155 ms) and the index (110 ms). The scan is now the longest of them, so the path is 50 ms, 180 ms, then 85 ms for the query, the merge and the rendering: about 320 ms against a target of 1 second, if the three do not slow each other. On a four-core machine they will, because the scan's workers and the model's kernels each use every core; with no overlap at all the rows add up to 580 ms. The built command decides, and it has not been measured. Memory at the peak: the model's bytes and its runtime, and for each section the index twice for a moment, because SQLite copies the bytes it is given; pbrain releases its own copy as soon as SQLite has taken it. A search of both sections with the larger model on a four-core machine is the case to measure; the figures above are one section at a time. For `embeddinggemma-2` the model's row must come to 600 ms or less by rule 3, with a larger index (768 dimensions: about 60 ms for the query and about 200 ms to read and open at the stress size). If hashing the model on every search proves too dear, the fallback is a record, written by `pbrain sync`, of the verified files' identity and times, rechecked by a search; it is not designed in. The built command is measured as section 8.6 describes, with and without the flag, for each model. A search that catches up is measured too, after one added note and at the full bound of 20 passages.

**Not designed in:** approximate search, quantised vectors, a resident process, query expansion, reranking and remote embeddings (spec 17); SQLite's full-text engine, which the lexical search does not use; a database opened in place. The index is read whole into memory on each search; if that proves too dear at larger sizes, the next step is a read-only file layer over the folder handle, which the SQLite build offers.

### 8.8 Partial matches, ranked by BM25

`search --bm25` adds, after the exact matches and after any results found by meaning, the items that hold a substantial part of the query without holding all of it, each marked `partial m/n` (owner's decision of 2026-11-21, on the group's `reviews/search-methods/joint-bm25.md`). It needs no model and no index: it is computed in `search`, in Go, over the same scan, with the same parser and fold. A write, a hand edit or a pull is found at once. The two flags are independent; with both, a model that is not ready still leaves the exact and partial results and the usual notice.

**The corpus.** Everything is counted over the eligible items of step 1 of section 8.3: the view, lifecycle, filters and window. Call their number N. A shared search therefore never depends on a personal item or its words.

**Clauses.** Each distinct clause of the query counts once, in the order it first appears; a phrase or a prefix is one clause. A word written both bare and in quotes is one clause and keeps `Quoted`. n is their number, and for an item m is how many of them it matches, in its title, tags or body, by the rules of section 8.2.

**Admission.** A partial match is an eligible item that is not an exact match and passes all of:

1. it matches every quoted clause (a phrase, or a single quoted word) and every prefix clause of the query;
2. a clause is *informative* when at most a quarter of the eligible items match it (4 × df ≤ N), which includes a clause that nothing matches; the item matches at least five informative clauses;
3. it matches at least half of all the informative clauses.

So a query of five clauses or fewer has no partial match, and neither has any query in a brain of fewer than four eligible items. The three numbers are constants in the code, set on the calibration half of the fixture: five, a quarter and a half. The minimum started at two and was raised to five in calibration (phase 18). With two, the rule left 26 of 30 calibration pages empty and failed the required regression case u029. Of the 25 variants tried on the calibration half, the one chosen left 29 of 30 calibration pages empty, passed every calibration regression case and raised the questions with an expected id on the first page from 16 to 22 of 47, losing none. Only that rule, frozen, was then scored on the held-out half: 30 of 30 pages empty, and the same rise from 16 to 22. This is a floor on evidence, not on the score: a BM25 score cannot be compared between queries.

**Order.** Partial matches are ordered by BM25 over the same corpus: for each clause, `ln(1 + (N − df + 0.5) / (df + 0.5))` times `tf × 2.2 / (tf + 1.2 × (0.25 + 0.75 × len / avg))`, summed in query order in 64-bit floats. `tf` is the clause's occurrences in the item's title, tags and body, a phrase counted within one field; `len` is the item's number of words in those fields and `avg` its mean over the eligible items. Ties go to the newer `created`, unknown last, then id, then path. With `--all`, current partial matches come before replaced ones.

**The list.** The result list is the exact matches, then the results found by meaning, then the partial matches; an item appears once, in the earliest group that has it. Leaving an item out of the partial group because an earlier group has it removes a candidate only: N, the document frequencies and the mean length are always those of all the eligible items. `Total`, the offset and the limit apply to the whole list. The decision exception, `Replaced` and `Outside` look at exact matches only. The per-clause counts appear only when the whole list is empty. A partial match's snippet is chosen by the rule of section 8.4.

**Cost.** Exact matching stops reading a body as soon as every clause has matched. For `--bm25` every eligible body is read to its end and its words and clause occurrences counted. A scratch program that tokenised every body of the 5,000-file brain took 54 to 58 ms with 16 workers and 92 ms with 4 (section 8.6). A search without the flag does none of this. The built command with the flag is measured against the same 300 ms target.

**Why not SQLite's full-text engine**, now that SQLite is a dependency: it splits words by other rules than pbrain's, so phrases, prefixes and accented words could match in one and not in the other; it would need an index of every body kept current, where today no write touches an index and a hand edit needs no step; and its statistics could not be restricted to the eligible items of a view.

**Release.** The partial tier has its own gates on the fixture, of the same form as a model's (`components/semantic.md`, "The wider comparison"), and must also pass together with each model a release lists (section 8.7, the combined gate). The starting rule returned nothing for five far-off questions and five partial rows for one question about a thing the brain does not hold, so it is not expected to pass unchanged. If no admission rule passes, the release ships without the flag, not with a looser rule.

## 9. Output and budgets

Every reading command builds a `render.Doc` and hands it to one fitter.

```go
package render

type Doc struct {
    Head   []Part            // always printed
    Units  []Unit            // candidates from Offset, in order
    Offset int               // the requested offset
    Total  int               // units before offset and limit
    Foot   func(st State) Part // the closing line or envelope tail, from what was shown
    Hints  []Part            // optional parts of the closing line, in order (the per-clause counts)
}

// State is what the closing part is built from.
type State struct {
    Shown int // units printed
    More  int // max(0, Total - Offset - Shown)
    Next  int // Offset + Shown; printed only when More > 0
}

type Unit struct {
    Main   Part   // never cut in half
    Alts   []Part // fuller forms of Main, largest first: the context blocks of search --context; nil otherwise
    Extras []Part // related lines: added only after every Main that fits
}

type Part struct {
    Text  string // the text form, without the final newline
    JSON  any    // the value marshalled into the JSON form
    Marks []Mark // stretches of Text that --color may decorate, in order; nil for most parts
}

type Mark struct {
    Start, End int // a code-point span of Text
    Style      Style
}

type Style uint8  // Bold (ESC[1m), Cyan (ESC[36m); the reset after either is ESC[0m
type Format uint8 // Text, JSON, TextColor

func Fit(d *Doc, limit int, f Format) (out []byte, err error)
// FitBody, the render design's entry point for get, takes the same Format in place of its bool.
```

**Fitting.** The size of an output is its count of code points as finally printed, newline included. `Fit` adds units one at a time: after adding unit *k* it builds the complete output for *k* units, with the closing part computed for that state, and measures it. It stops at the first *k* that does not fit and keeps *k - 1*. Then, for each kept unit in order, it tries the unit's `Alts`, largest first, and puts the first one with which the complete output still fits in the place of `Main`; a unit none of whose alternatives fits keeps its `Main`. It then adds extras in unit order, and hints in order, each only while the complete output still fits, stopping at the first that does not. Last, for `TextColor`, it adds decoration: for each `Mark` of the parts it printed, in output order, the start sequence of the style before the stretch and the reset after it, each pair only while the complete output still fits, stopping at the first that does not. Alternatives and decoration therefore never change which units are shown, `More`, `Next` or the minimum, and no output passes the limit. The minimum is always a real output, never a made-up one. When the page has no unit to show, the output for zero units must fit. When it has at least one, the first unit with the closing part that goes with it must fit, and the zero-unit output is never measured: a one-row page can be shorter than an empty page that announces more. If the required output does not fit, the error is `budget_too_small` and carries its measured size, the smallest limit that works. `More` and `Next` come from the number of units actually printed, so they are right whether the limit or the budget cut the page: with 12 matches, offset 4, limit 5 and room for two, the answer is `next` 6 and `more` 6. An offset at or past the end is a successful empty page: with 12 matches and offset 13, nothing is shown, `More` is 0 and no continuation is printed. Counts that describe the whole search, such as replaced and outside-window matches, are the same on every page. `get --outline` is an ordinary document whose units are sections, one line each. `get` uses the same fitter with the header as `Head`. It reserves room in this order: the header, the closing line, a linked item's complete link line, and one character of a non-empty body; then related lines while they fit; then the body, cut to the remaining room at a code-point boundary. The file lines in the closing line are part of what is measured: for a printed slice `[a, b)` that is not empty they are the lines of `a` and of `b - 1`, taken from one `LineCursor` that moves with the slice, and an empty slice has none. With `TextColor` the slice, its lines, the continuation and the minimum are those of the plain form, and the header's marks are added afterwards while they fit. If the reserved part does not fit, the error is `budget_too_small`.

**JSON** is printed compact, on one line, with HTML escaping off. The size is the count of code points in the encoded bytes, newline included, so an escape is counted as it is printed. Presence is decided field by field, not by a blanket rule on zero values:

- A false flag, an empty string and an empty list are left out.
- A number that is itself the answer is always printed, zero included: `snippet_offset`, `snippet_line`, `body_chars`, `body_line`, `file_lines`, `file_chars`, `file_line`, `line_from`, `line_to`, `occurrences`, each `line` of `context`, `offset`, `chars`, `level`, each `count` in `alone`, a tag's `items`, the totals `errors`, `decisions`, `learnings` and `sections`, and every count `sync` prints.
- `more` and `next` are printed only when something was left out; `"more": 0` is never printed (spec 6.2).
- A partial match carries `"partial": true` with `matched` and `clauses`, and in text the marker `partial 5/6`; no other row carries them.
- A search result carries `path`, `body_line`, `body_chars`, `file_lines`, `file_chars` and `occurrences`, and with a snippet `snippet_line` and `snippet_offset`. Rows of `list`, `prime` and history, and related lines, carry none of them (spec 7.3).
- A path in a search result or in `get` is printed bare, or as one JSON string with every control and non-printing character as a `\u` escape, or not at all when it is not valid UTF-8 (spec 6.2). `render` writes that string itself, for text and for JSON: the JSON encoder leaves DEL and the C1 controls raw and turns an invalid byte into U+FFFD, which could name another folder.

**Context blocks** are built here from the lines `search` supplies (section 8.4). A line prints as four spaces, its file line, a colon and its text, flattened like any value outside a body; a part of the stretch gets "…" where its line goes on, and a neighbouring line is cut at 160 code points. The alternatives of a unit are the block with every line, then with the outermost neighbouring line removed, the later one when two are equally far, and so on down to the stretch's own lines. A block over 600 code points is not offered: in text that is its lines with their prefixes and line feeds, in JSON the `context` field as encoded. In text an alternative is the unit with the block in place of its snippet line; in JSON it is the unit with `context` added and `snippet` kept. **Colour** marks are set on the id that starts a result or a `get` response (bold) and on a result's `path:line` and the value of `get`'s `path:` line (cyan), and nowhere else. Marks never overlap, belong to the part that was finally chosen (an alternative carries its own), and never lie inside a body, a snippet or a context line: what pbrain adds and what a stored body holds stay apart, and a stored escape sequence is printed as it is.

The smallest limit that works belongs to the command as asked. With `--semantic` or `--bm25` the closing part can carry `more` and `next` where the exact search alone would not, so the fitter measures the minimum on the merged list and may report a larger one.

The render component design lists every field of every command with its rule.

**Text** values outside a body pass through one function that flattens line breaks and control characters to spaces and applies the cuts of spec 6.2. An id or a path does not: it is an address, so text prints it whole through `render.ExactValue`, bare, or as the JSON string of an exact path when it holds a non-printing character, a double quote or a backslash, or as `?` when it is not valid UTF-8 (spec 6.2, "One id or path, one line").

## 10. Writing

### 10.1 The order of a write

A write carries three separate facts, decided once and passed explicitly to everything it calls:

```go
package store

type WritePlan struct {
    Dest     item.Section // where the item is written, or where the existing item is
    Lookup   brain.View   // what the write may read: for a new item it follows Dest; for an existing item it is the caller's view
    Disclose brain.View   // what its output may name: follows the caller's flag, never a rule
}
```

A rule can send a write to the personal section without `--personal`. Its `Lookup` is then the personal view, because duplicates and links are checked there, while its `Disclose` stays the shared view. The one exception is the new item itself, which the result line names. Every id, path and suggestion in an error or notice is cut to `Disclose` before it is formatted (section 13), so such a write reports a duplicate without naming the personal item it collides with. With `--personal` both views are the personal one and the caller gets the full diagnostic.

Every write then follows one order, and nothing is read or acquired out of turn:

1. Parse the flags.
2. Make the plan. For a new item: the flag, else an audience rule, else shared, or `audience_required` (section 12). For `verify` and `delete`: scan the view the flag chose, resolve the id to exactly one item, and take the section that item is in. `import` prints its destination notice here.
3. Prepare the input, outside any lock: open a file through the guarded opener of section 10.2, read standard input to its end, fetch a URL, convert HTML, clean and measure the body. This is where a command may wait on a pipe or a server.
4. Take the destination section's lock (section 10.4).
5. Refresh inside the lock: scan again, read `personal.toml` again and make the plan again. If the plan differs from the one of step 2, the command fails with `conflict` and stores nothing, since its notice named another destination. Then run every check against the fresh scan: links, duplicates, destination, and for `verify` and `delete` that the id still resolves to the same item.
6. For a copied import, store the original (section 10.7).
7. Publish: the commit point (section 10.5).
8. Clean up, still inside the lock, then release it.

Only steps 5 to 8 hold the lock, and none of them waits on anything outside the file system, so a stalled pipe or server in one process never blocks another writer.

A preview (`delete` without `--yes`) stops after step 2's scan: it takes no lock and creates no folder or file, also on a fresh clone with no `.pbrain/`.

### 10.2 Input files

`add --body-file` and `import <file>` read their input through one function and nowhere else.

```go
// OpenInput returns the bytes of a caller-named file for a write into dest.
// A refusal is indistinguishable from a missing file.
func OpenInput(b *brain.Brain, dest item.Section, path string) ([]byte, error)
```

- The path is resolved through every symbolic link first.
- For a shared destination the resolved path is refused if it is, or lies under, a protected path, or if any folder on it is named `.pbrain` under the fold of section 4.4.
- The file is then opened without following a final link, and the opened file is compared with the resolved one, so a swap between the check and the open is caught.
- A refusal returns the same error, with the same message, as a file that does not exist: `not_found` (spec 5.2). Nothing is stored and no original is written.

Text on standard input does not pass through it; the spec leaves that to the caller.

### 10.3 Files pbrain owns

pbrain never creates, replaces or removes a file by a path. It works on open folder handles:

```go
package sys

// Dir is one open folder. Every method names a single entry of it and never follows a link.
type Dir interface {
    Sub(name string, create bool) (Dir, error) // open a child folder; a link is an error
    Read(name string) ([]byte, FileInfo, error) // a regular file only
    CreateTemp() (name string, w io.WriteCloser, err error)
    LinkNew(temp, name string) error  // give temp a second name; fails if name exists
    Replace(temp, name string) error  // rename temp over name
    Remove(name string) error
    Sync() error
    Close() error
}

// OpenNamed opens a folder that is named from outside pbrain: Resolve once, then OpenRoot on the
// resolved path. It returns the resolved path with the handle.
func OpenNamed(files Files, path string, create bool) (Dir, ResolvedPath, error)
func OpenDir(path string) (Dir, error) // OpenNamed on the native files, the handle alone: for tests and tools
```

- The section's folder is opened once per command. Every folder below it (the kind folder, a nested folder holding an adopted item, `.pbrain`, `.pbrain/originals`) is reached by `Sub`, one component at a time, each opened relative to its parent's handle with link-following off.
- The operation on the file itself is made relative to the handle of its folder, again with link-following off.
- So the folder an operation acts in is the folder that was checked, by identity: swapping a folder for a link between a check and a use changes nothing, because nothing is looked up by path again. A link anywhere on the way fails the write with `invalid` before anything is written.

On Linux and macOS this is the `openat` family of calls from `golang.org/x/sys/unix`. The standard library's `os.Root` is not enough here: it stops a path leaving its root but follows links inside it, and the default personal section lies inside the shared section's folder.

**A root that a person names is resolved once, and links are refused only below it.** A path that comes from outside pbrain may reach its folder through links. On macOS `/var` and `/tmp` are links, so every temporary folder is behind one, and home folders and volumes are often laid out the same way. Refusing those would refuse ordinary machines and would protect nothing, since whoever names a folder may name any folder. So each such path is resolved once with `Files.Resolve`, which follows every link in it as the operating system would, and the folder is then opened by its resolved path with `Files.OpenRoot`, which walks down from `/` and follows none. Everything below that handle keeps the rule above. The roots are: the brain, whether found from the working directory or given by `--brain` or `PBRAIN_DIR`; the target of `init`; the personal section that `personal.toml` names; the per-user cache; `model_path`; and the folder of an input file. The two steps together give three guarantees. A link above a root, or the root's own name being a link, is followed. A link at any component below a root is an error. And a component of the resolved path that is swapped for a link between the two steps makes the open fail, because the open follows nothing. Every check that depends on where a root really lies (the exposure guard, the cache's placement test, the rule that the personal section may not hold the brain) is made on the resolved path, never on its spelling. `sys.OpenNamed` is the two steps for a caller with no check to make in between. `OpenRoot` itself stays strict: handed a path with a link in it, it fails, so a caller that forgets to resolve is found on the first machine whose paths hold links and never follows a link unseen. That is what happened with `model_path` and with tests that opened a temporary folder directly: both passed on Linux and failed on macOS (section 14).

`personal.toml` belongs to the brain, not to the personal section: it is read and replaced through the brain folder's handle, wherever the personal section lies and whether or not it can be reached.

### 10.4 The locks

Item writes use two kinds of lock, both exclusive `flock`s on a file, each waited for up to 30 seconds before failing with `io`:

- **A section's lock**, `<section>/.pbrain/lock`, for writes to that section's items and originals. It is held from step 4 to the end of step 8, so reference checks on originals and their removal happen inside it.
- **The settings lock** for `personal.toml`. It is the shared section's lock file, `<brain>/.pbrain/lock`, because the settings file lives in the brain and that folder is always reachable, even when the personal section is elsewhere or missing. `audience set` and `unset` take it, read `personal.toml` again inside it, apply their one change to that fresh copy and replace the file, so two commands at once keep both rules.

No item write holds two locks. A write that reads the rules does not need the settings lock: the file is replaced by a rename, so a reader sees the old file or the new one whole, and step 5 reads it again. Readers and previews take no item lock and wait for no lock, and only a write or `pbrain sync` creates `.pbrain/`.

`pbrain sync` uses two locks of its own, neither of them an item lock, so it never makes a reader or an item writer wait (section 8.7): the section's index lock, `<section>/.pbrain/index/lock`, held for the whole build and waited for as long as the command's context allows; and, only while it makes sure of the model, that model's lock in the per-user cache. It is the one command that holds two locks at once, always in that order: index lock, then model lock. A search with `--semantic` that catches up takes the index lock too, by one attempt that never waits, and holds it for a few seconds at most; it takes no model lock and no item lock.

### 10.5 Publishing, and the commit point

For a new item:

1. Write the complete file to a temporary name in the destination folder, a dot-file that does not end in `.md`, and flush it to disk.
2. Give it its final name with a hard link, which fails if the name exists. Where the file system has no hard links, an exclusive rename is used instead.
3. Remove the temporary name and flush the folder.

Step 2 is the **commit point**: from then on the item exists and every reader sees it whole. Each operation has one:

| Operation | Commit point |
|---|---|
| `add`, `import` | The hard link (or exclusive rename) that gives the item its name. |
| `verify` | The rename of the new bytes over the item file. |
| `delete` | The removal of the item file. |
| `audience set`, `unset` | The rename of the new `personal.toml` over the old. |

**Before the commit point** any failure exits 1 with its error: nothing was written that a reader can see, and the command can be repeated. A stored original with no item may remain.

**After it** the command has succeeded and exits 0 with its normal result, whatever happens next. A failure in what follows (removing the temporary name, flushing the folder, removing an original) is a notice with code `after_write`, saying what was left behind. It is never reported as "nothing was written".

### 10.6 `verify` and `delete`

`verify`: inside the lock, read the file, compute the edit (section 5), write the new bytes to a temporary file, check that the item file's `Version` is still the one that was read, and rename the temporary file over it. A difference is `conflict`.

`delete`: inside the lock, compute the facts with `graph.Effect` on the fresh scan (section 7.3), the same call the preview makes. Remove the item file: the commit point. Then, for a copied source, remove `originals/<sha256>` only if the digest is 64 lower-case hexadecimal characters, the name is a regular file directly in that section's `originals` folder, and no other item of the section, replaced ones included, has the digest. The output says `removed`, `kept` or `left`; a failed removal is `left` with a notice.

### 10.7 Originals

`store.PutOriginal` writes `originals/<sha256>` by the same temporary-file-and-link step, then reads it back and compares the digest. An existing file of that name is accepted only if its digest matches. Originals are only ever named by a digest pbrain computed or validated, and only inside the lock, so an import and a `delete` of the same original cannot interleave.

## 11. Importing

### 11.1 Bodies

File and fetched bodies go through one pipeline: check the type, check UTF-8, convert HTML to Markdown, strip a leading front matter block, remove control characters other than line breaks and tabs, measure, apply the 512,000-character limit. HTML conversion is: parse (`golang.org/x/net/html`), extract the main content (`go-readability`), convert to Markdown with relative links resolved against the base URL (`html-to-markdown`). The 200-character floor is measured on the extracted text.

Two limits are known and stated, not solved, in v1. A page that is an application shell can clear the floor with the wrong text; whoever captures such a page must scope it to the document first (spec 12, `browser-use`). A multi-file artifact is imported file by file, each with the artifact link and a `#<path>` label; pbrain does not know what a tool saved for it.

### 11.2 The public fetch

`fetch.Client` is built from two seams, `Resolver` and `Dialer`, so tests supply both.

```go
type Resolver interface { LookupNetIP(ctx context.Context, host string) ([]netip.Addr, error) }
type Dialer   interface { DialContext(ctx context.Context, network string, addr netip.AddrPort) (net.Conn, error) }

func (c *Client) Get(ctx context.Context, rawURL string) (*Page, error) // Page: final URL, media type, original bytes
```

pbrain drives the request loop itself; the HTTP library never follows a redirect and never reuses a connection. For the first request and for each of at most five redirects, in this order:

1. Parse the URL. Refuse anything but `http` or `https`, and any user information.
2. Resolve the host once. Every address returned must pass `public(addr)`; a literal address is tested the same way. Otherwise the result is `fetch_blocked`, and no connection is opened for this hop.
3. Send this one request on a new connection to one of the checked addresses, dialled by number. The request is bound to that address: its dial function connects to it and to nothing else. TLS verifies the certificate for the URL's host name.
4. A redirect response returns to step 1 with its target, also when the target is the same host.

Because each request has its own connection and its own check, a second request can never ride on a connection that was approved for another host or an earlier answer.

- The transport has no proxy, no cookie jar and no credentials, and keep-alive is off. pbrain asks for and decodes compression itself.
- `public` rejects loopback, private, link-local, unique-local, shared (100.64.0.0/10), multicast, unspecified, documentation and benchmark ranges, 240.0.0.0/4, IPv4-mapped and translated IPv6 forms of any of these, and anything else in the special-purpose registries. It is a table in one file with a test per range.
- One context enforces 30 seconds in total. The received body is read through a 5 MiB limit; decoding a compressed body is read through a second 5 MiB limit.
- Every failure other than a blocked address is `fetch_failed`, `unsupported_type`, `too_large` or `no_content`, and writes nothing.

## 12. Audience rules

**The canonical form** (`audience.Canon`) is one function used for rules and for sources:

1. Parse with `net/url` after adding a scheme to a rule that has none. Refuse user information.
2. Host: lower case; an internationalised name to its ASCII form (`idna.Lookup`); a trailing dot removed. Port: dropped when it is the scheme's default (443 for a rule without a scheme), kept otherwise.
3. Path, worked on in its escaped form: refuse a backslash and an encoded slash or backslash; split on `/`; in each segment decode the percent-escapes of unreserved characters only; then, on the decoded segments, refuse an empty one (other than a trailing one), `.` and `..`. So `%2E%2E` is refused exactly as `..` is.
4. Drop the scheme, the query and the fragment. The key is `host[:port]/seg/seg`.

A rule with no canonical form is refused. A source with no canonical form matches no rule: only a flag can give it an audience (step 2 of the decision below).

**Two hosts, two uses** (a backlog item). The rule key keeps a non-default port, so a rule for `host:8443/x` matches only sources on that port. The mixed-audience table is looked up by the canonical DNS host name alone, without the port, so a port never lets a shared rule slip past the table.

**The table** is data in one file: exact hosts for Google Docs, Drive, `claude.ai`, GitHub and GitLab; any host ending in `.slack.com` for Slack. Each entry lists the path shapes a shared rule must start with, as segment patterns.

**The decision** for a write has one answer for every case, in this order:

1. An explicit flag decides.
2. A source that is an `http` or `https` URL with no canonical form: `audience_required`. It can match no rule, and it never falls through to the shared default, linked items on ordinary hosts included.
3. The longest matching rule decides.
4. A gated write (spec 8.5: a URL source pbrain did not fetch itself, or a linked item on a mixed-audience host): `audience_required`.
5. Otherwise shared.

A source that is not a URL, such as "task 42", is untouched by steps 2 to 4. The decision is step 2 of every write (section 10.1), before anything is read or acquired, and is made again inside the lock. Whether a URL can be fetched at all is a separate check in `fetch`.

`personal.toml` is rewritten whole by `audience set` and `unset`, from a copy read inside the settings lock (section 10.4), through a temporary file and a rename in the brain folder (section 10.3). Known keys and every rule are kept. Comments in the file are not; that is recorded in section 17.

## 13. The command line, errors and exit codes

```go
package cli

// Env is everything a command takes from the outside. Tests build their own.
type Env struct {
    Args   []string
    Stdin  io.Reader
    Stdout io.Writer
    Stderr io.Writer
    Getenv func(string) string
    Dir    string           // the current directory
    Now    func() time.Time
    Rand   io.Reader
    Git    gitx.Runner
    Net    fetch.Net        // the resolver and the dialer
    Files  sys.Files        // opens folder handles and locks; replaced to inject failures
    Models embed.Registry   // the supported embedding models and their adapters; tests wire a fixed test model
}

func Run(ctx context.Context, env Env) (exit int) // RunWith(ctx, env, Release)

// Features are the parts of the command surface a release may hold back.
type Features struct {
    BM25 bool // the partial tier and its --bm25 flag (section 8.8)
}

// Release is what this build ships: a constant of the release, never read from the environment.
var Release = Features{BM25: true}

// RunWith is Run with an explicit feature set, for tests and the calibration tool.
func RunWith(ctx context.Context, env Env, f Features) (exit int)
```

A held-back feature does not exist for `Run`: its flag is not registered, so using it is a usage error. Scenario scripts and the calibration tool call `RunWith` to exercise it, which is how the gate that decides whether it ships can be run on the build that would ship it; `Release.BM25` becomes true only in the change that records the passed gate. It is true now: the partial tier passed its gate alone and its combined gate beside each of the two models (`internal/semantic/calibrate/README.md`).

`--context` on `search` and `--color` on `search` and `get` are ordinary flags, present in every build. `cli` refuses a `--context` outside 1 to 3 and `--color` together with `--json` as usage errors, passes the first as `search.Request.Context` and the second as `render.TextColor`. Nothing else turns colour on: nothing looks at whether standard output is a terminal, and there is no setting and no environment variable for it. `add` still looks at standard input, to tell a pipe from a terminal (spec 8.2); that is another question.

`Run` reads nothing from the process itself: no global clock, no `os.Getenv`, no `time.Local`. At its start it takes one instant from `Now` and one time zone from `TZ` as `Getenv` gives it (the system zone when unset), and passes both down; the recency band, the time window and review dates all use that pair. Two commands with different zones can therefore run in one test process.

```go
package errs

type Error struct {
    Code    string // one of spec 6.3's codes, no_model included
    Message string
    Exit    int    // 1, or 2 for usage
}

func (e *Error) Error() string // so *Error is an ordinary Go error; cli finds it with errors.As

type Notice struct{ Code, Message string }
```

- Every package returns `*errs.Error` for an expected failure, built where the failure is understood. Any other error reaching `cli` is wrapped as `io`.
- A write reports an error only for a failure before its commit point (section 10.5). After it, failures are `after_write` notices and the exit code is 0.
- `cli` is the only place that prints, and the binary makes that true of linked libraries too: `cmd/pbrain` hands the process's real standard output and standard error to `Env`, then points the `os.Stdout` and `os.Stderr` variables at the null device, so a library that prints through them prints nowhere. No linked package is known to print (section 8.7); the rule is there so that a later pin of a runtime cannot break the output contract. On an error it prints nothing on standard output (the two exceptions of spec 6.3 print their document first), then the error on standard error, as a line of text or as one JSON object with `--json`.
- Notices are printed on standard error as they arise, so `import`'s destination notice appears before anything is acquired.
- No message is built from a value the secret scan flags or from an item outside what the command may name. For a read that is its view; for a write it is the plan's `Disclose` view (section 10.1), which can be narrower than what the write read. Ids, paths and suggestions enter a message only through one function in `cli` that takes that view and drops what lies outside it. Packages return typed values, never formatted references, and call sites do not format them freely, which keeps the rule checkable. `errs` imports nothing and knows no view.
- A fault in the semantic part, its catch-up included, never fails a search. Whatever keeps it from running, a missing or wrong model, a missing or damaged index, an exposed cache, is a `semantic_not_ready` or `semantic_partial` notice, and the lexical results are returned. Every error the search would have without the flag keeps its code and exit status. One error can be new with the flag: `budget_too_small`, because the minimum belongs to the command as asked and the merged page can need a continuation (section 9). `pbrain sync` is the command that fails: `no_model`, `invalid_config` for an unknown model, a wrong `model_path`, an exposed cache or a device selector that names nothing on this machine, `fetch_failed` for the download, and `io` for a named GPU that cannot be started or a GPU that fails in the middle of a run for a reason other than memory. A search never meets a GPU, so none of that reaches it.
- The notice codes are the ten of spec 6.3: `after_write`, `destination`, `git_history`, `index_progress`, `model_download`, `secret_suspected`, `semantic_not_ready`, `semantic_partial`, `skipped_files` and `undeclared_tag`. Each text has one owner: `components/semantic.md` for the four of semantic search and the index, `components/cli.md` for `skipped_files`, and the write and import designs for the rest.
- There is no log file and no debug output in v1. A panic is a bug: `cli.Run` recovers, prints `io` with a fixed message and exits 1.

`check`'s findings are sorted by one total order (a backlog item): severity, path, code, field, line, column, message.

## 14. Testing

| Kind | What it covers |
|---|---|
| Unit tests, table-driven | Every package. The pure ones (`item`, `query`, `graph`, `search`, `render`, `audience`) carry most of the logic and need no file system. |
| Scenario scripts | The 41 acceptance scenarios of spec 15, run with `testscript` from `cmd/pbrain/testdata/scenarios`: every scenario has at least one script, and a long one may be split into several. A script runs commands against a fixture brain in a Git work tree and states the expected standard output, standard error and exit code. Three commands are registered with `testscript`'s `Main`, so each call is the test binary started again as a child process with the caller's environment, a script's time zone included. Scripts use two of them: `pbrain` is `cmd/pbrain`'s own start-up, the code a release runs, and `pbrain-test` is the same start-up with what a script needs fixed: a clock the script sets, the `embedtest` registry, which the semantic scenarios use, a scripted network that serves only what the script declares and fails the script on any other request, and `cli.RunWith`, so that a script can run with the partial tier held back. The third, `pbrain-stray`, is called by no script: it is that start-up around a command that prints through the process's own output variables and through `Env`'s writers, for `TestStrayOutputDiscarded` (the "Release binary" row). No script reaches the real network or loads a model. Scenarios 14 and 15 need processes that overlap or die: two replacements of one item at once (one succeeds, one fails), two adds in the same second (two ids), and a process killed right after publication, followed by a read. Scenario 41's terminal cases, the same command on a pipe and on a pseudo-terminal, run on Linux only: on other systems the test is skipped with a message that names the Mac backlog item, where their macOS run is tracked (section 17). |
| The built binary | A small set runs the real executable as a subprocess: a command run from another checkout's directory, a body on standard input, the delete preview's split between the two streams, and every test of locking and of publishing without overwriting, which must cross real processes. They run on Linux; their macOS run is deferred (section 17). |
| Fuzz tests | Front matter parsing (never panics, never mis-attributes a field), query parsing, `audience.Canon`, `render.Fit` (the output never exceeds the limit, and the reported minimum works), `outline.Parse` (sections are ordered, never overlap and cover the body), and also `item.EditVerify`, `item.Measure` and `graph.Build`. |
| Property tests | Emit then parse returns the same header. Paging with `next` visits every unit once. |
| Concurrency | Several test processes replacing one item at once: one succeeds (scenario 14). Readers during writes never see a partial file. A `verify` racing a `get` and a `search` yields a header, snippet and body from one version. Two `audience set` commands at once keep both rules. An import racing the `delete` of the same original leaves a consistent result. A write stalled on its input in one process lets another write to the same section finish. |
| Delete effect | `graph.Effect` on the cases of scenario 29, on a stamp that a deleted link was masking (the stamped item stays replaced), and on an older item whose link is left dangling. |
| Paging | 12 matches, offset 4, limit 5, room for two results: `next` 6, `more` 6, and the replaced and outside-window counts unchanged on the next page. Offset 13: an empty page that succeeds, with no `more` and no `next`. |
| Audience decision | A linked item on an ordinary host whose URL has an invalid path: `audience_required` without a flag, written with one. |
| Settings | `audience set` with the personal section outside the brain, with it unreachable, and from two processes in one checkout. |
| Failure injection | The file operations of `sys` are replaceable in tests. A failure is injected on each side of every commit point: before it the command exits 1 and nothing is visible; after it the command exits 0 with an `after_write` notice. |
| Boundary | A personal item is found with `--personal` and absent from shared results, totals and errors. A rule-routed personal write that hits a duplicate, and one whose replacement candidates are personal, name no personal item on either stream in text or JSON, while the same call with `--personal` gets the full diagnostic. A folder swapped for a link between a check and its use changes nothing. An ignored shared folder is never listed. Scenario 6 runs through both `add --body-file` and `import`, with no original and no partial item after a refusal. A good header over an invalid body is skipped and cannot replace another item. A preview on a fresh clone creates nothing. |
| Fetch | A fake resolver and dialer: every forbidden address case of scenario 20, with a counter proving no forbidden dial happened, and a redirect to the same host proving the check runs on every hop. |
| Canonical form | One table of URL spellings run through the rule path and the source path, which must agree, encoded dot segments included. |
| Passage | A sole match at code points 1000 to 1005 of a long body: the position is 805, the earliest stretch that holds it, not one centred on it or moved to a word boundary, and `get --offset 805` opens on the same text. |
| Git | Real `git` in temporary repositories: ignored folders, force-added files, nested repositories, linked work trees (scenarios 8 and 9); an ignored, force-added personal file still stops every command when `GIT_INDEX_FILE` points at an empty index; a clean `init` with no protected folder on disk; a second `init` over a `.gitignore` that holds one of the three rules and other lines; `pbrain skill` with no brain. Skipped with a clear message when `git` is absent. |
| Outline | One table of bodies for `outline.Parse`: both heading styles, fences of both kinds, an unclosed fence, a list item above a rule, text before the first heading, CRLF, multibyte text. The same positions are checked through `get --outline`, `get --offset` and a snippet's section (scenario 36). |
| Section alignment | One long item built for it (a test borrowed from ken, `research/ken-review.md`): two sections with the same title under different parents, a fenced block holding a line that looks like a heading, and multibyte text before the match. For a lexical result (section 8.4) and for a result found by meaning (section 8.7), in text and JSON: `get --offset` at the returned position opens on the text the snippet shows; `outline.At` puts that position in the section the fixture names by its place in the outline, since the title does not tell the two apart; the heading printed with the snippet is that section's; the position lies within the position and size that `get --outline` prints for it; and the fenced line starts no section and no passage. Every passage of the item lies inside one section. Two boundary cases are pinned as well, in a section that starts at code point 1000. A sole occurrence ending at 1030 gives position 830; the section before holds 830 and is the one named. Two clauses at 1025 to 1030 and 1215 to 1220 give position 1020, in the section itself, which is the one named. The same body is also stored with CRLF line ends and with a byte-order mark before the front matter: every printed `path:line` is the line of the stored file that holds the snippet's first character, and `get --offset` at the printed offset opens on that character (scenario 39). |
| Location, context and colour | `item` counts on files with LF, CRLF, a byte-order mark, no final line feed and an empty body with and without a final line feed: `BodyLine`, `BodyChars`, `FileLines`, `FileChars`, and `LineOf` at offset 0, in the middle of a long line and at the end. `Occurrences` against counts made by hand, a phrase that overlaps itself and a repeated clause included, equal to the `tf` sum BM25 uses on the same item. Rendered pages: the location line and its JSON fields on a long hit, a short hit, a title-only hit, an empty body and a result found by meaning; `list`, `prime` and history rows unchanged byte for byte. With `--context`: the same units, `More`, `Next` and minimum as without it at every limit from the minimum up, blocks dropped from the outside in, a block over 600 code points not used, a body line that imitates a result line shown only behind its number. With `--color`: the output with the sequences removed equals the plain output at every limit, no output passes its limit, and no terminal, variable or setting produces a sequence without the flag (scenarios 39 to 41). The release gates of section 8.7 and 8.8 are run on pages with the location line. Edge cases, each in text and JSON: a hit found by meaning through its title alone (path, real body size, no snippet fields); a stretch that ends exactly on a line feed, with CRLF and at the end of the file (no empty focal line, the next real line as neighbour); `get` slices that end on a line feed, between a carriage return and its line feed, and at or past the end (the lines of the first and last printed character, none for an empty slice); coloured `get` at tight limits with the same slice, lines, continuation and minimum as plain; a body of one 512,000-character line read once, shown by a counter on the cursor; a stored body with an escape sequence printed byte for byte with and without `--color`; paths with a space, an escape character, a C1 control and a direction override, escaped in both forms; a folder whose name is not valid UTF-8 beside one with U+FFFD in that place, the first never printed and the second printed as itself; an exact result with its context block joined by a semantic result that takes the block's room. |
| Semantic search | Scenarios 33 to 35 and 37 run with a test model in `Env.Models` whose vectors come from a table in the fixture, so scores are exact: order and ties of added results, the cut-off on both sides, eligibility under every filter, per-section readiness, the two notices, a build interrupted between publications, reuse from the index and from the cache, an exposed cache, a damaged database file, a changed fingerprint. One test proves that SQLite opens no file: a search and a build run where every path but the folder handles is unusable, and the index folder holds only the lock, the manifest and one database afterwards. A counter on the test model proves that `search` embeds one text, that a second build embeds none and that `add` embeds nothing. No personal vector in the shared index or the cache is checked by reading the files. |
| Release binary | The built binary of every target, started as a fresh process: `--version`, a lexical search, a search with `--semantic` and `sync` leave only the command's output on the process's real standard output. A second child process shows that a print through the `os.Stdout` or `os.Stderr` variable reaches neither real stream after start-up, while `Env`'s writers do. A source check on the pinned commits of our two libraries finds no read of the environment and no print in the packages the binary links on any of the four release targets, outside one short list of allowed places kept in the test with their reasons; it fails when a new pin adds one. |
| Partial matches | Admission on small brains built for it: a phrase, a prefix or a single quoted word that does not match excludes the item, and the same word unquoted does not; a clause in more than a quarter of the items does not count; five informative clauses and half of them are both required; queries of five clauses or fewer and brains of fewer than four items give none. BM25 against values computed by hand, ties included. Exact results and their order are the same with and without the flag; an item found by meaning is not repeated as partial, and its removal changes no statistic (a brain of four items at the quarter boundary gives the same partial rows with and without it); totals and paging cover the whole list; the shared output is identical before and after personal items are added. |
| Index database | The scoring query against a reference computed in Go on the same vectors: the same passages at or above the cut-off, also when more than 4,096 qualify; an item outside the eligible set is never scored, and a bad vector on one does not fail the query; a manifest naming `lock`, a name with a path, or an impossible size is refused before anything is opened, removed or allocated, and a rebuild keeps the lock it holds; a database whose digest matches but which lacks a table is unreadable; with two sections and one failing during its query, the other's results stand; ties to the earlier passage; an item replaced in one transaction is never half present; two builds publish in turn and the later keeps the earlier one's rows; a search's catch-up within its bounds and outside them, with the lock busy and with a manifest that changed under it (`components/semantic.md`). The index reader is held to its rule by these table tests and has no fuzz target: a manifest or a database file with a changed byte is refused before SQLite sees it. Timings of read, check, open, query and publication at 35,000 × 384 and 50,000 × 768. |
| Model runtime | For each model in the table: the adapter reproduces reference vectors stored in the repository (cosine similarity of at least 0.999), cuts an over-long text instead of failing, and reports the table's fingerprint and dimension. Run where the model files are present, skipped with a clear message otherwise. |
| Benchmarks | The measurement plan of section 8.6: whole-process timings on the synthetic 5,000-item, 50 MB brain, built by a generator in the repository. Results are published, not gated, because machines differ. The same runs time `search --semantic` against its 1 second target, a first `pbrain sync`, and an index after one added note. |
| Evaluation | The fixture of `reviews/retrieval/joint.md`: 30 to 50 real task requests with expected ids or "unknown", the paired learning-or-document tasks, and a harness that reports useful ids on the first page, right abstentions, total characters, estimated tokens and calls. A personal leak, counts included, fails the run. The baseline run with the team's current lookup is a written procedure, not code. For semantic search the fixture also holds at least 30 questions the brain cannot answer in each half and 30 answerable ones worded unlike their answers; a calibration tool in the repository sets each model's cut-off and reports the two release gates of section 8.7. The same tool runs the wider comparison the owner asked for (`components/semantic.md`): strict search, strict with one guided retry, any-word matching by count and by BM25 on the same candidates, BM25 with stemming and with heading weight, the partial tier, hybrid search per model, both flags together, and rank fusion; over whole-sentence questions, two- and three-word queries, exact identifiers, superseded answers, answers in one section of a long source, far and near unanswerable questions, and questions only a personal item answers. Every arm is judged on its rendered first page at the same budget. The abstention gate is enforced, not only reported, on `--bm25` alone, on `--semantic` alone per model and on both flags per model; a test builds the case of 38, 38 and 36 empty pages of 40 and requires the release check to fail. For the questions answered by one section of a long source, finding the item is not counted as seeing the answer (`research/ken-review.md`): the fixture quotes the supporting text, and for every arm a fixed reader that is given nothing from the answer key reads the first page and then opens the first result at the position it gives. The tool records whether the expected item is on the page and whether the quoted text reached the reader, on the page or in that one read. It reports the fraction that reach the evidence; what was read up to the quotation and what was delivered in all, with every call charged; the misses of each kind apart, in no average; and, labelled as an upper bound, what a reader told which item to open would have reached. Reported, not gated (`components/semantic.md`, "Reaching the evidence"). |

Every scenario of spec 15 is an executable test; that is the coverage target. CI runs on Linux amd64 and has two kinds of job: the suite under the race detector, which needs cgo and so runs natively with it on, and the suite against the release configuration, `CGO_ENABLED=0`, which also cross-builds the binary for the four targets. A local run is not that: after every push to master its CI run is looked at, a change counts as merged only when the run is green, and a release is tagged only from such a commit. macOS is deferred (owner's decision, 2026-11-21: pbrain targets Linux for now): the same two suites ran on macOS arm64 until that day and were turned off while failing, and section 17 says what is owed before they return. The first tag, v0.1.0, was cut on 2026-11-21 while CI had been failing unseen since that morning; its release was not published, and the next release is v0.1.1.

## 15. Dependencies and build

Third-party modules, all pure Go, with the versions current on 2026-11-20:

| Module | Version | Used for |
|---|---|---|
| `github.com/spf13/pflag` | v1.1.0 | Flag parsing with flags after positional arguments. |
| `go.yaml.in/yaml/v3` | v3.0.6 | Reading front matter as a node tree. |
| `github.com/BurntSushi/toml` | v1.6.0 | `pbrain.toml` and `personal.toml`. |
| `golang.org/x/net` | v0.63.0 | `html` for parsing, `idna` for host names. |
| `golang.org/x/sys` | v0.50.0 | `flock`, the `openat` family of calls and exclusive rename. |
| `codeberg.org/readeck/go-readability/v2` | v2.1.3 | Main-content extraction from HTML. |
| `github.com/JohannesKaufmann/html-to-markdown/v2` | v2.5.2 | HTML to Markdown. |
| `github.com/rogpeppe/go-internal` | v1.16.0 | `testscript`, tests only. |
| `modernc.org/sqlite` | v1.62.0 | SQLite 3.54.1 translated to Go, with sqlite-vec 0.1.9 bundled: the index database and its distance function (section 8.7). It brings `modernc.org/libc` v1.80.1 and a few small modules. About 7 MB of binary. |
| `github.com/pkieltyka/aikit` | v1.79.1 (`0bbc048`): the original's v1.78.0 plus the loader from bytes, under our module path; the same linked code as v1.79.0 | The runtime for `bge-small-en-v1.5` (its `encoder` and `embed` packages) and the kernels under goinfer (section 8.7). About 1 MB of binary. |
| `github.com/pkieltyka/goinfer` | v0.6.1 (`e2324268a`): our `main` with the original merged up to its `36dbfbaf1`, the tokenizer pack, the loader from bytes, the calls that embed prepared token ids and the loader of a GGUF text checkpoint from a reader, under our module path. Never v0.4.0, which the module proxy holds with the original's content (section 8.7) | The runtime for `embeddinggemma-2`: its text encoder and tokenizer packages only (section 8.7). About 1 MB of binary together with its kernels. Added with part (e) of step 12. |
| `github.com/pkieltyka/goinfer/backend/cuda` | v0.6.1 (`e4b58e96f`): goinfer's CUDA backend, a module nested in its repository, with the code of v0.5.0. It brings `github.com/eitamring/gocudrv` v0.3.2 and `github.com/ebitengine/purego` v0.10.1 | Linked only in a build with the `cuda` tag, which lets `pbrain sync` embed `embeddinggemma-2` on an NVIDIA GPU (section 8.7); a search in that build still runs on the CPU. A build without the tag links none of it. Since issue #44 the Linux release archives are built with the tag, and an archive without it, named `_cpu`, is published beside each (decision 10). The tag about triples the binary: stripped, as released, Linux amd64 is 44.2 MiB with it and 15.0 MiB without, and its archive 15.2 MiB against 6.4 MiB; arm64 is 42.6 MiB against 14.1 MiB, and 14.5 MiB against 5.8 MiB (GoReleaser snapshot at `304b808`, on goinfer v0.5.0 and before the amd64 archive took the `vulkan` tag; the release sizes on v0.6.1 with both tags are owed by issue #48 and the Vulkan issue under #41; the row below has a scratch figure). Not stripped, amd64 was 53 MiB against 22 MiB. The tagged binary needs no NVIDIA library to start: the driver's library is looked for when a sync first wants a GPU, and where it is missing `auto` falls back to the CPU. That was checked on the extracted amd64 archive with the driver hidden: a sync of `embeddinggemma-2` printed `cpu_fallback` and indexed, a sync of the default model printed nothing even with `PBRAIN_DEVICE=cuda:99`, and `pbrain devices` listed the CPU alone. |
| `github.com/pkieltyka/goinfer/backend/vulkan` | v0.6.1 (`e4b58e96f`): goinfer's Vulkan backend for AMD cards, a module nested in its repository, with its compute shaders compiled and embedded. It brings `github.com/ebitengine/purego` v0.10.1, which the CUDA module brings too | Linked only in a build with the `vulkan` tag on Linux amd64, which lets `pbrain sync` embed EmbeddingGemma 2 on a discrete AMD card (section 8.7, "The device"); goinfer supports no other target, so the Linux arm64 archive is built without it. It needs no Vulkan library to start: `libvulkan.so.1` is opened when a sync first tries the card, and where it is missing `auto` falls back to the CPU. The Linux amd64 release archive is built with this tag and `cuda` together (owner's decision, 2026-11-22; decision 10). The two backends link side by side, and this one is small: in scratch builds of the command at `b4843f5` on goinfer v0.6.1, stripped, Linux amd64 was 46,334,215 bytes with `cuda` and 46,485,767 with both tags, 151,552 more, and 15,773,856 with neither; the three GPU builds need the same three system libraries to start. Those are executables before the selector code, not release archives. |

Everything else is the standard library: `net/http`, `net/netip`, `encoding/json`, `crypto/sha256`, `crypto/rand`, `hash/crc32`, `embed`, `os/exec`. The vector cache needs no module.

The binary is built with `CGO_ENABLED=0 go build ./cmd/pbrain`. No toolchain experiment and no build tag is set. The two runtime modules carry Go assembly and no cgo, and add between 1 and 2 MB to the binary; the packages linked for bge-small import nothing for network or subprocess use (`goinfer-bge-small-check.md`), and the adapters' tests hold both runtimes to no network call and no file opened. The version printed by `--version` comes from the module's build information, overridden by a linker flag in release builds. CI runs `gofmt`, `go vet` and the tests of section 14.

**Installing with `go install`.** `go install github.com/pkieltyka/pbrain/cmd/pbrain@<version>` works only for a module whose `go.mod` holds no replace or exclude directive. pbrain's holds none: both libraries are required under their own module paths (section 8.7), which keeps the first route of decision 10 in section 16. A CI check fails the build if either directive appears in `go.mod`. What it installs is the build without the `cuda` tag, which embeds on the CPU only; `go install -tags cuda github.com/pkieltyka/pbrain/cmd/pbrain@<version>` installs the one that can use an NVIDIA GPU, checked against a commit of the repository (`304b808`); `-tags cuda,vulkan` on Linux amd64 installs what the release archive holds, with the AMD backend too.

## 16. Decisions the owner confirmed

The owner answered on 2026-11-20. Questions 1 to 13 were this document's technical questions, each answered "yes" to the recommendation. The rest are the semantic-search decisions taken on the research summary (`research/semantic-search-and-index/research_summary.md`).

1. **Go version and module path.** Go 1.27; module `github.com/pkieltyka/pbrain`.
2. **cgo.** None: `CGO_ENABLED=0`. Semantic search does not reopen it.
3. **Command-line parsing.** `spf13/pflag` with pbrain's own command table. No command framework: pbrain needs exact control of usage errors, JSON errors and exit codes, and pflag gives flags after positional arguments, which the standard library does not.
4. **YAML and TOML.** `go.yaml.in/yaml/v3` (the maintained continuation; `gopkg.in/yaml.v3` last released in 2022) for reading only; `BurntSushi/toml` for configuration.
5. **HTML.** `golang.org/x/net/html`, `codeberg.org/readeck/go-readability/v2` and `JohannesKaufmann/html-to-markdown/v2`. The importer design records their run on the article, script-shell and short-page fixtures (`components/importer.md`).
6. **Git.** Shell out to the `git` binary, read-only. No Git library.
7. **Search engine.** A scan first, with no index, for the lexical path; confirmed or reopened by the measurements of section 8.6.
8. **JSON form.** Compact, one line, with the presence rules of section 9; `"more": 0` is never printed. Now spec 6.2.
9. **Path anchor.** Section 4.5. Now spec 6.2.
10. **Distribution.** `go install`, plus release binaries for Linux and macOS on amd64 and arm64 built by GoReleaser. Since issue #44 (owner's decision, 2026-11-22; decision 31) each Linux target has two archives: `pbrain_<version>_linux_<arch>.tar.gz`, built with GPU support, which can embed on a GPU and falls back to the CPU without one, and `pbrain_<version>_linux_<arch>_cpu.tar.gz`, built without it. GPU support is both backends on amd64, the tags `cuda` and `vulkan`, so that a person with an NVIDIA or an AMD card gets it from the default download (owner's decision, 2026-11-22), and `cuda` alone on arm64, where goinfer has no Vulkan backend. The macOS archives keep their names and are built without a tag, and so is a plain `go install`; `go install -tags cuda`, or `-tags cuda,vulkan` on Linux amd64, of the same path builds the GPU variant. The Linux arm64 archive with the tag is cross-built and vetted and has not run on a device. Homebrew and Nix packaging later. Since 2026-11-21 macOS is deferred (section 17): its binaries are still built with each release, untested and unsupported.
11. **The skill.** Markdown files in the repository, embedded with `go:embed`; a test enforces the word limits and the topic list.
12. **Tests.** The standard `testing` package, `testscript` for the scenarios, native fuzzing. No assertion library.
13. **Licence.** MIT, as factory.
14. **Semantic search is in v1**, as an opt-in mode, built last, released only when the fixture shows a gain over lexical search and correct abstention (section 8.7).
15. **The models** (as decided on 2026-11-21). Two are supported and the project chooses in `pbrain.toml`: `bge-small-en-v1.5` on aikit's encoder, with one added loader, and `embeddinggemma-2` (Apache-2.0) on goinfer, with three changes of which one is done. Both run on the project's own libraries (decision 24) and are held to the rules of section 8.7. hugot's pure-Go backend, on which bge-small was first accepted, is the documented fallback and is not linked. EmbeddingGemma v1 is not used. pbrain defines the `Embedder` seam; the fixture shows which model serves a project better.
16. **The download.** Once, pinned by SHA-256, into a per-user cache, started only by `pbrain sync`; a local path serves machines without network.
17. **No remote embeddings** in v1.
18. **Combining.** Lexical results first and untouched; then results found only by meaning, marked, above a calibrated absolute cut-off. No query expansion, no reranker.
19. **Speed.** 1 second for a search with `--semantic` at 5,000 items; 300 ms stays for lexical reads. A search never waits for a build. A search that catches up (decision 30) is outside the 1 second.
20. **`get --outline`** is in v1, and a snippet names its section.
21. **Who turns it on.** The project names the model in `pbrain.toml`; each person builds their own index, per section. Since decision 27 a project that names none has the default model, so semantic search is available unless the project turns it off.
22. **The vector store** (2026-11-21). SQLite with sqlite-vec, in the pure-Go build, replacing the flat files of the earlier draft. Vectors are kept in an ordinary table and scored with sqlite-vec's `vec_distance_cosine`, not in its `vec0` virtual table, and there is no cap on the number of results found by meaning (the owner's decision on the measurements of section 8.7).
23. **Search methods** (2026-11-21). The evaluation fixture is widened with any-word and fusion arms and more kinds of question, and the skill teaches the prefix clause. A hint on empty results is not added. BM25 enters v1 behind an opt-in flag, `--bm25`, as the group's `reviews/search-methods/joint-bm25.md` sets out: exact matches, then results found by meaning, then marked partial matches, scored in Go over the scan; the partial tier is held back if it fails its gates (section 8.8).
24. **The libraries** (2026-11-21). aikit and goinfer are forked to `github.com/pkieltyka/aikit` and `github.com/pkieltyka/goinfer`, and the forks diverge from their origin: each takes its own module path, so pbrain requires them directly, with no replace directive, and `go install` at a version works (decision 10). The team builds what pbrain needs in them, on a topic branch and through a reviewed pull request into our `main`; the loader from bytes in aikit, and the tokenizer pack and the loader from bytes in goinfer, are merged. Offering a change to the original author is optional and is the owner's step. The owner plans to remove unused parts and to optimise later. pbrain pins tagged releases: aikit v1.79.1 (v1.79.0 until the fork's later work was released) and goinfer v0.5.0 (v0.2.0 until the fork took in the original's later work, then v0.4.1 until the fork added the calls that embed prepared token ids and its CUDA backend; v0.4.0 is unusable, section 8.7). `go.mod` also requires goinfer's CUDA backend module at v0.5.0, which only a build with the `cuda` tag links. On 2026-11-22 the pin moved to goinfer v0.6.1 for its GGUF loader and its Vulkan backend, with both backend modules at v0.6.1 (issue #48; section 8.7). goinfer's own numbering started at v0.1.0.
25. **The section named with a lexical snippet** (2026-11-21) stays as spec 7.1 has it: the section that holds the snippet's position, which for a match near the top of a section can be the section before it (section 8.4). The "reaching the evidence" figures of section 14 will show whether that costs anything.
26. **What a search result prints** (2026-11-21, on `reviews/output-format/joint.md`). Plain text stays the default and the agent's contract, with stored Markdown for bodies and compact JSON on request; JSON only gains fields. Every result says where its snippet starts in two units, the file line (`path:line`, front matter included) and the body offset that `get --offset` takes, with the line on which the body starts, the size of the body and of the whole file in lines and characters, and a count of the query's occurrences. `get` and `get --outline` give the same file lines. Lines of context (`--context`) and colour (`--color`) are opt-in flags; colour is never chosen by detecting a terminal. There is no `get --line`. The default limit of `search` is 3,000 characters for 5 results.
27. **The default model** (2026-11-21, revised the same day). When `pbrain.toml` names no model, pbrain uses `bge-small-en-v1.5`. A project can name `embeddinggemma-2`, the larger model, instead, and `model = "none"` turns semantic search off. Nothing is downloaded until `pbrain sync` is run. As first taken, this decision made `embeddinggemma-2` the default, on a stated first index of about 95 minutes for 5,000 items. Phase 28 then measured it: 0.902 passages a second on four CPUs, a projected 15 hours for the 5,000-item, 50 MB brain of spec 16 (section 8.7). On that figure the owner made the smaller model the default and kept `embeddinggemma-2` in the release for a project that names it. Nothing had shipped, so no project's index changes model.
28. **`pbrain sync`** (2026-11-21). The command that builds the index and brings it up to date is `pbrain sync`, with `sync --status`; it was `pbrain index`. There is no alias: `pbrain index` is an unknown command. A hidden alias with a `command_renamed` notice was specified and built the same day, and the owner removed it before any release carried it. The index, its folder, its lock and the `index_progress` notice keep their names.
29. **Freshness by CRC-32C** (2026-11-21). Whether an index entry still holds an item's text is decided by the CRC-32C and the byte count of its title and body, on every command that asks: a search with `--semantic`, `sync --status` and the comparison in `sync`. SHA-256 is computed only for an item that is new or changed and for its passages, inside `sync` and, since decision 30, in a search's catch-up, and stays the key of the passages in the index and in the per-user cache, so no cache can hand out another text's vector. The whole-database check made on every open is CRC-32C too. SHA-256 stays wherever a hash is made once: the model's files, an import's `sha256` field and the originals. The index format becomes 3 (section 8.7).
30. **A search catches up a small gap** (2026-11-21). With an index present and the model on the machine, `search --semantic` embeds and saves up to 20 uncovered items needing up to 20 passages, within about 2 seconds, before it answers. It takes the section's index lock only if it is free. Larger gaps, a first index and a model download stay with `pbrain sync`. This relaxes the rule that a search only reads, for a search with the flag (section 8.7, "Catch-up in a search").
31. **GPU first for the build, never for a search** (2026-11-22). For a model that has a GPU path, `pbrain sync` embeds on a GPU when this binary and this machine have one, chosen by `--device`, `PBRAIN_DEVICE` or automatically, and falls back to the CPU with a `cpu_fallback` notice; that notice is for such a model only, since the CPU is the default model's normal path. The batch is sized from free GPU memory, 30% and at most 6 GiB, with one text alone always allowed. A search opens the model on the CPU in every build. `pbrain devices` lists what can be used. The index and the fingerprint record no device. The Linux release archives are built with the `cuda` tag since the fallback went in, with a CPU-only archive beside each (issue #44; decision 10), and the amd64 one with goinfer's Vulkan backend for AMD cards as well: until goinfer can list such a card, it is named `vulkan` with no index, tried last under `auto`, and shown by `pbrain devices` as detected when a sync attaches it. Overlapping the stages of a build is deferred until a measurement asks for it (section 8.7, "The device").

Decided by the architect within those answers, and agreed by the operator on 2026-11-20:

- A write of an item never touches the index or loads a model; `pbrain sync` is incremental and was the only writer until decision 30 (section 8.7).
- A search does not catch up on new items. It would make a read write files and take a lock, make two identical searches differ, and spend an unknown part of the speed target. Superseded for a small gap by decision 30; it still holds for a large one.
- A partly current index is used for what it covers, with a notice that counts the rest.
- The per-user vector cache is adopted, for shared passages only.
- Inside the owner's decision for SQLite: the database file is written once and read whole from verified bytes, with SQLite never opening a file itself, and the per-user cache stays plain files (section 8.7).

## 17. Risks and gaps

**Amendments accepted on 2026-11-20**, all now in the functional spec and in this design: `init` refuses when Git lists another `pbrain.toml` (section 6.5); compact JSON and the presence rules (section 9); the path anchor (section 4.5); the fold (section 4.4); the `after_write` notice and `not_found` for a missing input file (sections 10.2, 10.5 and 13); `conflict` for a write whose audience rules change while it prepares (section 10.1).

**Verification gaps:**

- Speed is not established: section 8.6 lists what was measured and what must be.
- `flock` is advisory and unreliable on some network file systems. A brain on such a mount can lose the "take turns" guarantee. Not solved in v1.
- The hard-link publish step needs a file system with hard links or exclusive rename. Both paths need a test on macOS, which is deferred (below).
- The folder-handle operations of section 10.3 use calls that differ slightly between Linux and macOS, the exclusive rename most of all. They need their own tests on both; the macOS ones are not run while macOS is deferred (below).
- **Semantic search first ran end to end on the 35-item fixture only**, with bge-small on Linux: 171 to 208 ms for a search, 131 to 164 ms for an index with nothing to do, 9.0 s for the first index with the model's download, and 151 MB at a search. The 1 second target at 5,000 items, the build time and the memory at that size, and every such figure for embeddinggemma-2, were budgets and estimates then. Phase 28 has since measured them for bge-small (the default-model point below); and, on a second machine with a GPU, for embeddinggemma-2 (the EmbeddingGemma 2 point below); that model's first index on CPUs alone stays a projection. Two rows of the speed budget (hashing the bodies, and merging and rendering) are estimates too.
- **The bge-small runtime was measured under Linux on amd64 only, on two processor models.** On an Intel Core Ultra 7 255H, in two workloads: single runs on a few texts in the first check, and the full-size runs of phase 28 (section 8.7). Whether the two ran on one physical machine is not recorded. Phase 28 then repeated the full-size runs on four cores of an AMD Ryzen 9 7950X (`internal/cli/README.measure.md`). No arm64 or macOS build has been run, nor a cold file cache, and its arm64 kernels are separate assembly that no run of ours has exercised. Its loader exists only in our aikit. Its first index is about 40% longer than the fallback's.
- **EmbeddingGemma 2 is released; rule 3 is shown for it on Linux on CPUs, which is what a search uses.** It is the model a project opts into, at cut-off 0.72, with its gates passed on the fixture (`internal/semantic/calibrate/README.md`). Phase 28 timed the built adapter on Linux at four threads with warm files (`internal/cli/README.measure.md`): check, load and one query in 253 to 366 ms, the whole one-item command in 478 to 553 ms, and a peak of 1.08 GiB at a search. It was on hold from 2026-11-21, while the goinfer fork was made faster for it, and the owner cleared it for use again on 2026-11-22, with goinfer v0.5.0. A team run of it sets `PBRAIN_ALLOW_GEMMA2=1`, and it stays off by default. The figures above were taken on an Intel Core Ultra 7 255H before v0.5.0. On v0.5.0 the same benchmark was repeated without the GPU on the second machine (issue #33; `internal/cli/README.measure.md`): check, load and one query took 283 to 353 ms, median 286, over five runs on four cores, inside rule 3's 600 ms and inside the Intel range. The whole command on a 32-note brain took 521 ms there, as a median of three. Its full-size figures come from that second machine alone, a Ryzen 9 7950X with an NVIDIA RTX PRO 6000, using four cores while it was busy with other work, at a one-minute load of 10 to 28 (`internal/cli/README.measure.md`). A build with the `cuda` tag finished the first index in 1 hour 10 minutes with a peak of 4,758 MiB of host memory. At 5,000 items `search --semantic` took 773 to 820 ms as a warm median in the untagged build, with up to 1,687 MiB, and 5 of 60 samples took a second or more. In the `cuda` build it took 1,426 to 1,534 ms with up to 2,380 MiB, and every sample missed the 1 second target. The `cuda` build also fails rule 3: check, load and one query took 928 to 1,272 ms, median 1,050, against 600 ms, and against the 286 ms of the untagged build on the same machine. The two were not taken alike: the `cuda` runs were five fresh processes at a one-minute load of 26.6 to 28.5, and the untagged runs five in one test process at a load of 0.8, so the gap is not all the GPU's start-up; a matched pair in fresh processes at low load is in `internal/cli/README.measure.md`. What follows is decided (decision 31): a search opens the model on the CPU in every build, so these two `cuda` figures for a search describe a build as it was before that change, and the GPU is for `pbrain sync` alone. Still not measured: a first index of that brain finished on CPUs alone, which stays a projection of 15 hours from the Intel run; a cold file cache; a second GPU or driver; and the Linux arm64 build with the `cuda` tag on any device, which is released built and not run. If a figure fails its target on the machines a project uses, the model's place in the table has to be looked at again.
- **macOS is deferred: pbrain targets Linux for now** (owner's decision, 2026-11-21, which widens the same day's earlier decision that the first release would not wait for Mac measurements). CI tests on Linux only, and a change is merged on a green Linux run (section 14). The darwin binaries, amd64 and arm64, are still cross-built in CI and built with each release, and their linked sources audited, but nothing runs them: they are untested and unsupported, and the arm64 kernels of both runtimes have never run a real model in our hands. The macOS CI jobs, which ran the suite without model files, were turned off that day while failing. What they showed is recorded with the Mac backlog (issue #1). Two kinds of failure are fixed on master since (issue #21): folders opened behind macOS's `/var` link, `model_path` and the tests' temporary folders, now go through `sys.OpenNamed` (section 10.3); and the name-admission cases that APFS cannot hold, names that are not UTF-8 or that differ only by case or by composed and decomposed accents, are skipped where the file system lacks the trait. One fault in pbrain itself is not fixed. On macOS, creating a missing lock file with a plain `openat(O_CREAT)` can fail with "no such file" when several commands create it at once, so the first simultaneous use of a section, or of a model in the cache, can fail one of them with `io`; a second try succeeds. Its fix waits unreviewed on branch `peter/lead-dev-2`. Owed before macOS is supported, and tracked apart from any release (issues #1 and #9): that fix reviewed and the suite green on a Mac, the macOS CI jobs back, rule 3 for both models on an arm64 Mac, `search --semantic` at 5,000 items there, installing the release binary from a tag and running it, and scenario 41's terminal cases, which need a pseudo-terminal opened in macOS's own way and so run on Linux only (operator's call, 2026-11-21: code for a terminal we cannot run would be unverified). A Mac user may meet any of these faults, a start slower than rule 3 allows, or a fault in those kernels, before we do. If the Mac figures fail rule 3, the model's place in the table is looked at again.
- **The index database was measured in scratch only**, on a shared machine: reading, checking, opening and querying it, and publishing it, at two sizes, one section at a time. Both sections together with the 768-dimension model on four cores have since been measured inside the built command only (the EmbeddingGemma 2 point above). Each publication rewrites the whole file, about 160 ms at our size and about 320 ms at the stress size. It also holds the database three times over for a moment: the live database, the buffer SQLite serialises it into, and the copy of that buffer the driver makes for Go. Near the end of the shared section of the full-size bge-small build the two copies were 83.9 MiB each (issue #19; `internal/cli/README.measure.md`). The resident size rose by only 0.3 MiB across them and did not fall when SQLite freed its buffer, which fits SQLite's allocator keeping those pages and using them again at each publication; the Go copy is made anew every time, 11 GiB in all by that build's peak. That reuse does not make the copies free: in the next profiled build (issue #34) the highest sample fell in a publication, with both copies live at 86.8 MiB each. SQLite's own memory counters are off in the pinned build, so its share is read from the process's mappings and not from SQLite. The search holds the whole database in memory. Whether SQLite touches any file when used this way is to be proved by a test, not assumed.
- **The partial tier is narrow, and may not ship beside a model.** Its starting rule, two informative clauses, left too many calibration pages with results and failed a required regression case; calibration raised the minimum to five, so only a query of six clauses or more can have a partial match, and the tier may ship in a release that lists no model. Beside a model it ships only if the combined gate passes for that model (section 8.7). The built command with `--bm25` was timed at 5,000 items in phase 28: warm medians of 152 to 196 ms in the personal view, inside the 300 ms target (`internal/cli/README.measure.md`).
- **The default model is the small one, and its full-size figures come from two Linux machines.** A project that names no model gets `bge-small-en-v1.5` (decision 27 as revised): a download of 134 MB, and on the 5,000-item, 50 MB brain of spec section 16 (phase 28, four CPUs of an Intel Core Ultra 7 255H): a first index of 1 hour 41 minutes at 7.99 passages a second, a peak of 1,351 MiB of memory while indexing and up to 549 MiB at a search, and `search --semantic` in 302 to 333 ms as a warm median (298 to 335 ms with `--bm25` added). On four cores of an AMD Ryzen 9 7950X the same build took 1 hour 18 minutes at 10.31 passages a second, with a peak of 1,315 MiB while indexing and up to 550 MiB at a search, and `search --semantic` took 292 to 326 ms as a warm median (291 to 322 ms with `--bm25` added). The memory is several times what the first small check showed (0.14 GB and under 0.2 GB), and it grows as the build goes through the brain. A profiled build on the Ryzen machine, of the code as it was before the change below, shows where it went (issue #19; `internal/cli/README.measure.md`, the memory-profile section). The process held 227 MiB once the model was open, before the first item, and its highest sample was 1,298 MiB, at shared item 4,259 of 4,500, just after a publication. Of that, 885 MiB was Go heap and 413 MiB was everything else: the in-memory SQLite database, SQLite's allocator and the runtime's own mappings, which the profile cannot divide further. Go's heap held about twice what was live. Its profile, as of the last collection 42 seconds before the peak, counts 401 MiB in use: 127 MiB of model weights, 83 MiB for the Go copy of the serialised database, 61 MiB of finished passage vectors, 47 MiB of passage text, 44 MiB of the file buffers the scan read, and 24 MiB of the encoder's working space. At the peak Go counted 790 MiB allocated, which includes whatever had become garbage since that collection, and another 84 MiB idle and not returned. The turnover goes some way to explain the distance: by then the build had allocated 42 GiB in all, 25 GiB of it in the model's forward passes and 11 GiB in copies of the serialised database. So the earlier reading from the design, that the bodies a build keeps and the serialised database account for the memory, held only in part: the kept file buffers were 44 MiB, and what the model and each publication allocate and drop weighed more. The profile pointed to a first saving, since made and measured (issue #34; the section after it in that file): that build kept every finished item's passage text and vectors reachable while it worked on later items, about 108 MiB at the peak by the profile, and kept the scan's file buffers too. The build now lets go of an item's work once its rows are stored, and of the scan once it holds the bodies it still has to cut. Profiled the same way on the same machine, cores and brain, its highest sample is 998 MiB where it was 1,298 MiB: 300 MiB less, or 23%. All of that is Go heap, 579 MiB resident where it was 885 MiB, which is more than the 153 MiB of text, vectors and file buffers the first profile counted as live. The new peak falls during a publication, at shared item 4,408, with both copies of the database live at 86.8 MiB each. The heap profile there counts 235 MiB in use: the 127 MiB of weights, 86 MiB for the Go copy of the database and 13 MiB of working space, with the file buffers at about 1 MiB and no passage text or vectors sampled. The two profiles were taken at different points of the collector's cycle, so their in-use totals do not subtract into a saving; the resident sizes do. The index is unchanged: the shared database is the same file byte for byte, and every row of `item` and `passage` matches in both sections, all 48,511 vectors included. The personal file differs by 121 bytes on the page of its four `meta` rows, which hold the same values and whose order of insertion is not fixed. The 1,351 MiB and 1,315 MiB above were measured before this change and have not been repeated, so 998 MiB, from the instrumented build, is so far the only peak of the build as it now is. The two profiled runs took 1 hour 54 minutes and 1 hour 25 minutes with their instrumentation and are not timings. The EmbeddingGemma 2 build with the GPU has no such profile; its 4,758 MiB is a resident size alone. On the fixture it is the weaker model by a little: an expected id on the first page for 40 of 47 questions against 42, 29 of 30 unanswerable pages empty against 30, and the evidence reached for four of six long-source questions against five. A project that names `embeddinggemma-2` takes on that model's costs: a download of 1.52 GB once; at the size of that brain, up to 1,687 MiB of memory at a search, which takes 773 to 820 ms as a warm median where bge-small takes 292 to 326 ms on the same machine; and a first index at 0.902 passages a second on four CPUs (section 8.7), a projected 15 hours for that brain from the Intel run, or 1 hour 10 minutes and 4,758 MiB of memory on a machine with an NVIDIA GPU and a build that uses it (the EmbeddingGemma 2 point above). Neither model costs anything before `pbrain sync` is run, but whoever runs it on a notice's advice, a person or an agent, starts all of it. A run that is stopped resumes from the last saved checkpoint at the next `pbrain sync`: it loses the window of passages being embedded, at most 128 with this model and 256 on a GPU, and at most 20 seconds of finished work, and the shared vectors of every finished window stay in the cache. Changing a project's model later means every person builds their index again.
- **Two libraries to maintain.** aikit and goinfer are ours now (section 8.7). pbrain pins aikit v1.79.1 and goinfer v0.6.1, both tagged, and goinfer's CUDA and Vulkan backend modules at v0.6.1, which bring two outside modules into `go.mod` (section 8.7). goinfer's last two releases were published with their own device and parity runs deferred, so each pin rests on pbrain's parity tests alone. Their original author moves at hundreds of commits a day; none of that reaches pbrain unless it is merged into our `main` on purpose, and after the rename such a merge touches every import line. A fix made there is ours to notice. Version numbers are a second hazard: the original's tags use the same names as ours could, and one that is fetched and pushed to our repository becomes a version of our module that the proxy keeps for good with the wrong content. That is how goinfer v0.4.0 was lost (section 8.7). A tag is made only on our `main`, under a number neither repository has used, and is checked on the proxy before pbrain requires it.
- **The AMD path and the GGUF rows are specified ahead of their measurements** (2026-11-22). Nothing has run on an AMD card through pbrain. Rule 2 against the CPU, batches against single texts by bytes, the 108 KB a position that the batch size is worked out from, the speed of a build and the size of the release archive with both tags are all owed, and the Vulkan path is merged for a model row only when the first two pass for it. goinfer v0.6.1 attaches a machine's single discrete AMD card and refuses when there are two, lists nothing, reports no memory, and tells its failures by text alone, which the adapter reads (section 8.7). So a machine with two such cards can use neither, a batch is sized from 1 GiB and not from what is free, and `pbrain devices` shows the `vulkan` line on every machine that runs the default Linux amd64 download, card or no card. `pkieltyka/goinfer` issues #23 to #30 were all open at v0.6.1. The GGUF rows have no cut-off, no rule 3 figure and no memory figure until issue #47 reports them. One rule is the team's and not the product's: the AMD card of the machine these runs are made on also drives its display, so a run on it may hold at most 4 GiB of device memory in all, weights included; `PBRAIN_GPU_MEMORY` is set to keep it there, the driver's `mem_info_vram_used` is sampled during the run, and the run is stopped if its share passes 4 GiB (owner, 2026-11-22). pbrain has no such limit of its own.
- **Every search result is about 110 characters longer** than in the drafts the measurements and estimates so far assumed, and about 170 in JSON. The default page grew from 2,000 to 3,000 characters to keep five results, so each search hands an agent up to half as much again. The release gates and the "reaching the evidence" figures are taken on these pages. Counting lines and characters adds work to every scan. Phase 28 timed the built commands at 5,000 items, lexical search at 135 to 189 ms and `get` at 134 ms as warm medians, which shows the whole command inside the 300 ms target. The counting was then timed by itself (issue #19): 29.8 ms of one thread's processor time over the 50 MB for lines and code points together, of which the line count is 7.0 ms (section 8.6).
- **Both cut-offs rest on one small fixture**: 35 shared items, 47 answerable questions, and 60 unanswerable ones in two halves of 30. On the held-out half, alone and with `--bm25`, bge-small at 0.65 leaves 29 of 30 unanswerable pages empty and gives an expected id on the first page for 40 of 47 answerable questions, and embeddinggemma-2 at 0.72 leaves 30 of 30 empty and gives 42 of 47, against 16 of 47 for strict search, with none lost and every regression case passed. A brain of thousands of real items may behave differently, and a cut-off is a constant of the release.
- **Passage sizes** are starting values counted in code points; dense text can exceed a small model's input and lose its tail.
- **The cache is never pruned**, and it grows with every project and model a person uses.
- **A search with `--semantic` can now write** (decision 30). Its catch-up is measured on one machine, four cores of a Ryzen 9 7950X, for gaps of one short passage an item (`internal/cli/README.measure.md`). With bge-small one new passage added 0.45 s to a search and 20 added 0.88 s, and the whole gap was saved each time. With EmbeddingGemma 2 one added 0.92 s and 20 added 2.7 s, and the search saved 15 to 19 of the 20; a build with the `cuda` tag added 0.74 s and 1.3 s and saved all 20. Long passages were not measured. For them the earlier estimate stands: at about one passage a second, EmbeddingGemma 2 fits two or three in the 2 seconds, so a gap of long items is saved only in part. Catch-up embeds one passage a call, so that the 2 seconds can be kept with a slow model; whether that is slower for bge-small than the build's batches of eight is not known. Publication serialises the index, which makes two more copies of it in memory for a moment (the index-database point above), so the peak of such a search is higher than the 550 MiB measured; by how much is not measured. Where the lock can be taken and the write then fails, as on a full disk, each search embeds the same personal items again, up to 2 seconds each time. There is no setting that turns catch-up off; a brain that cannot be written is the only place it never runs. `flock` on a network file system is as unreliable for it as for a build (above).
- **Freshness rests on a 32-bit checksum** (decision 29). An entry is taken to hold an item's present text when the CRC-32C and the byte count of its title and body agree. A change of the count is always seen, and so is a change within four consecutive bytes, such as a corrected word of the same length. Any other edit that keeps the count is missed with a chance of 1 in 2^32. The item then keeps its old passages: a search can add it for what it used to say, with a snippet cut at the old spans, and `pbrain sync` does not repair it, since it compares the same way. Editing the item again or deleting the section's `index/` folder does. Lexical results, `get` and every write are unaffected. A CRC is also no defence against a deliberate collision: whoever edits an item can shape the edit so that its entry stays, and the harm is the same stale entry, for that item only. The stored SHA-256 of the entry is not read by anything in format 3.
- **A change of index format embeds the personal section again.** An index of an older format is built afresh. The shared section comes back from the cache without embedding; personal vectors are never cached, so they are made again: at our full size 4,869 passages, about 8 minutes at the 10.31 passages a second measured on the Ryzen machine, an estimate. Format 3 is the first such change after a tagged release.

**Known limits accepted:**

- No Unicode normalisation in matching (section 4.4).
- `audience set` drops comments from `personal.toml`.
- The core skill's outline in spec 12 is longer than its 400-word limit; the shipped text must be tighter, and scenario 23 enforces it.
- Each checkout builds its own index; the cache removes most of that cost for shared items, none of it for a model change.
- Scores can differ in their last digits between processor types, so the order of two nearly equal added results is fixed on one machine, not across machines (spec 7.1).

## 18. Component designs and build order

One design per component under `components/`, each reviewed by one teammate:

| Design | Covers | Author | Reviewer |
|---|---|---|---|
| `item.md` | Front matter parse, emit and edit; ids; standing; the fold. | @lead-dev-2 | @lead-dev |
| `graph.md` | The graph, `graph.Effect`, dangling links and items that become current again. | @lead-dev-2 | @lead-dev |
| `search.md` | Query parsing, the scan and ranking, time windows, per-clause counts, the merge on the semantic seam. | @lead-dev-2 | @lead-dev |
| `render.md` | The fitter and budgets, the reading commands, `get --outline` over `outline`, the semantic marker and section. | @lead-dev | @lead-dev-2 |
| `brain.md` | Discovery and configuration, `gitx`, the exposure guard. | @lead-dev | @architect-2 |
| `store.md` | `sys` handles and locks, the scan, guarded input files, the write primitives. | @lead-dev | @architect-2 |
| `writes.md` | `add`, `verify`, `delete`; `secretscan`; the write plan; commit points. | @lead-dev | @architect-2 |
| `cli.md` | The command table, flags, errors, exit codes, `cli.Run`. | @architect-2 | @lead-dev-2 |
| `audience.md` | The canonical form, the rule type, the decision, the `audience` command. | @architect-2 | @lead-dev-2 |
| `importer.md` | File imports, linked items, the fetch, the HTML libraries and their fixture check. | @architect-2 | @lead-dev-2 |
| `skill.md` | The skill text, `go:embed`, the word-limit and topic tests. | @architect-2 | @lead-dev-2 |
| `semantic.md` | `embed` and `semantic`: models, passages, the index files, the build, the cache, the query, calibration. | @architect | @architect-2, @lead-dev |

The code is built in this order, which follows the spec's delivery order:

0. The shared types and signatures of sections 4, 7.3, 7.4, 8.5, 8.7, 9, 10.1, 10.3 and 13, compiled with empty bodies, with the three contracts named in section 3 agreed.
1. `item` and front matter (parse, emit, edit, ids, standing).
2. `graph`, which needs only `item`.
3. `audience` (the canonical form and the rule type), then `brain`, `gitx` and the exposure guard.
4. `sys` (folder handles, locks) and `store`: the scan, guarded input files, then the write primitives.
5. `query` and `search`, lexical only: `Request.Semantic` is nil. The partial tier of section 8.8 is built here too, behind its flag, and released only when its gates pass.
6. **Outline.** `outline`, which needs nothing.
7. `render`, then the reading commands in `cli`: `search`, `get` with `--outline`, `list`, `prime`, `tags`, `check`, and the section named with a snippet.
8. The writing commands: `add`, `verify`, `delete`, with `secretscan`.
9. The audience decision and the `audience` command.
10. `importer` for files and linked items; `skill`.
11. `importer/fetch`, released only when scenario 20 passes.
12. **Semantic**, last, in five parts that can each be tested alone: (a) the merge in `search` and its rendering, against a fake `Semantic`; (b) `semantic` with the test model: passages, the index database, the build, the cache, the status, the source; (c) `embed`: the model table, finding and downloading a model, and the bge-small adapter on aikit's encoder, against our aikit at its pinned commit; (d) the fixture's questions, the calibration tool and the measurements; (e) EmbeddingGemma 2, in our goinfer and then here: the fast tokenizer and the loader from bytes with the encoder's constructor (both done, goinfer v0.2.0), the text-only pack and its derive step, and the adapter. A model is released only when the rules and both gates of section 8.7 pass for it; bge-small does not wait for (e).

After step 0, steps 1 to 3 touch different packages, as do 5, 6 and 7, so two developers can work in parallel: `graph`, `query`, `search`, `outline` and `render` need only `item` or nothing, and can be built and tested without a file system. Step 12 (a) and (b) need no model and can start as soon as steps 5 and 7 are done.
