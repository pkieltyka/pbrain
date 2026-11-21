# Plan: `pbrain sync` and freshness by CRC-32C

Two owner decisions of 2026-11-21. Issue #24 is the rename (owner @lead-dev). Issue #25 is the
digests (owner @lead-dev-2). The spec already carries both: functional spec §§6.3, 9.4;
architecture §§8.7, 13, 16 (decisions 28 and 29), 17; `components/cli.md`;
`components/semantic.md`. This plan adds the build order, the files and the reasons.

## Decisions

**A. Rename.** `pbrain index` becomes `pbrain sync`, with `sync --status`. `index` stays for one
release as a hidden alias that prints a `command_renamed` notice.

**B. Digests** (the owner's refined decision).

- An index entry covers an item when the CRC-32C and the byte count of its title and body match.
  A search with `--semantic`, `sync --status` and the comparison inside `sync` all use that test.
- SHA-256 is computed only inside `sync`, for an item that is new or changed and for its
  passages. It stays the key of a passage in the index (`vectorByDigest`) and in the per-user
  cache.
- The whole-database check made on every index open is CRC-32C.
- SHA-256 stays for the model's files, an import's `sha256` field and the originals.
- The index format becomes 3. The vector cache does not change.

## The three questions on B

### 1. Collisions in the caches

There is none to fix. Both content-addressed reuse paths keep their SHA-256 key: the index lookup
(`build.go` `reuse`, `index.go` `vectorByDigest`, the `passage_digest` index) and the per-user
cache (`cache.go`, `vectors/<model fingerprint>/<2 hex>/<64 hex>`). The cache keeps its layout and
needs no new namespace.

The one place a CRC decides is the per-path item check. It is not a birthday problem, because an
entry is only ever compared with the item at its own path.

- A change of the byte count is always seen.
- A change within four consecutive bytes at the same count is always seen: a CRC-32 detects every
  burst of up to 32 bits.
- Moving bytes between the title's end and the body's start is always seen: only the framed title
  length changes, and it lies within four bytes.
- Any other edit that keeps the count is missed once in 2^32.

A miss leaves a stale entry: the item can be added to a search for what it used to say, and
`pbrain sync` does not repair it. Editing the item again or deleting `.pbrain/index/` does.
Architecture §17 records this as an accepted limit, and `TestCoverage` pins it.

Rejected, with reasons:

- **CRC-32C as the passage key, widened by CRC-32 IEEE and the length.** The two polynomials are
  coprime (checked), so the pair is a 64-bit CRC. That is safe against chance and open to
  a made collision, and it would have changed the cache's keys. The refined decision makes it
  unnecessary.
- **Comparing the stored passage text on a cache hit.** Exact, but the index would have to store
  every passage text for its own lookup, about 50 MB more to read and deserialize on every search.
- **Using `item.Version` (the scan's CRC-32C and size of the whole file).** It costs a search
  nothing, but `verify` and any other front-matter edit would drop the item from semantic results
  until the next `sync`. `ContentCheck` costs 3.8 ms for 50 MB and keeps the entry.

### 2. Formats, and what a v0.1.1 user sees

`indexFormat` goes from 2 to 3: `item` gains `crc` and `size`, and the manifest's `sha256`
becomes `crc32c`. No reader of format 2 is kept.

With an index built by v0.1.1, on the new release:

1. `search --semantic` returns the lexical results with
   `semantic_not_ready: no index for this model: run pbrain sync`.
2. `pbrain sync --status` prints `shared: 0 of N items indexed (unreadable)`.
3. The next `pbrain sync` builds each section again and removes the old database file. Nothing is
   downloaded.
   - **Shared section:** every vector comes from the per-user cache, so nothing is embedded and
     the inference runtime is not opened. The line reads `0 passages embedded, <all> reused`.
   - **Personal section** (`--personal`): embedded again in full, because personal vectors are
     never cached. At our full size that is 4,869 passages, about 8 minutes at the 10.31 passages
     a second measured on the Ryzen machine (an estimate).
   - A user who deleted the cache embeds the shared section again too.

The "unreadable" wording is the existing state for a manifest of another format. A new state for
"older format" was rejected: it adds a status text and a JSON field for a one-time event.

Rejected: reading the format-2 database once to take its vectors. It keeps a second manifest
reader and a second schema in the code for one upgrade, and no release with format 2 has been
published yet.

### 3. Every SHA-256 use

Measured on one core of a Ryzen 9 7950X, pinned, in a scratch program (2026-11-21): SHA-256
2.7 GB/s, CRC-32C 12 to 13 GB/s. The built command has not been timed with the change.

| Use | Where | When it runs | Cost per call | Verdict |
|---|---|---|---|---|
| Item content digest | `passages.go:36`; `source.go:262` (`covers`), `source.go:345` (status), `build.go:229` | every `search --semantic`, every `sync --status`, every `sync`, over every item with an entry | 50 MB: 18.5 ms as SHA-256, 3.8 ms as CRC-32C | **Switches** to `ContentCheck`. SHA-256 stays for new and changed items inside `sync`. |
| Whole-database check | `index.go:469` (`readDatabase`), written at `index.go:630` | every index open (search, status, build) and every publication | 80 MB: 29.5 ms as SHA-256, 6.4 ms as CRC-32C | **Switches.** It guards against damage, not against a writer of the folder, who can replace the manifest too. |
| Passage digest | `passages.go:79,84` | `sync` only, for the passages of new and changed items | 48,511 passages of 1 KB: 21 ms in all | **Stays.** It is the reuse key. |
| Model files at load | `embed.go:296` (`Verify`) | every `search --semantic` (beside the scan), every `sync` with an item, every `sync --status` | bge-small 134 MB: 50 ms; EmbeddingGemma 2 552 MB: 205 ms | **Stays.** Pinned to the published digests; "a model is opened only from verified bytes". |
| Model download and pack | `embed.go:350`, `embed.go:410` | once per download | one pass over the files | **Stays.** |
| Model fingerprint, `FolderName`, `IndexFingerprint` | `bge/model.go:36`, `gemma2/model.go:45`, `embed.go:154`, `passages.go:50` | a few hundred bytes, at start or per open | under 1 µs | **Stay.** They are names: changing one renames the model folder or the cache folder. |
| Import digest and originals | `importer.go:160`, `store/tx.go:353,368`, `item/write.go`, `check.go:231` | once per import or delete | at most 5 MiB: about 2 ms | **Stays.** It is the `sha256:` field of the item file in Git and the name of the original. |

Expected gain: about 38 ms of CPU time per warm semantic search at our full size on this machine
(15 ms for the items, 23 ms for the databases), of a 291 to 326 ms command. Part of it overlaps
the scan, so the wall-clock gain may be smaller. After the change the largest hash on the search
path is the model check: 50 ms for bge-small. Architecture §8.7 already names the fallback for
that, a record of the verified files' identity and times; it is not in this plan.

## A. The rename: interfaces and steps (#24)

- `internal/cli/options.go`: the row becomes
  `{"sync", "", "build or update the semantic index, or report its state", …}`. `commandNamed`
  resolves `index` to that row, and `parseOptions` (`options.go:243`), its one caller for the
  command word, records on the invocation that the alias was used. `commandAt` and
  `jsonRequested` do not look at command names, so they need no change.
- `internal/cli/output.go` `helpText` needs no change: root help lists the rows of `commands`,
  where `index` is not a row, and `index --help` receives the `sync` row.
- `internal/cli/run.go`: `case "sync"`. When the alias was used and the command is not help,
  emit `command_renamed` with the message
  `pbrain index is now pbrain sync: the old name will be removed in the next release`, before
  the handler runs.
- `internal/cli/semantic.go`: `indexHint` gives `pbrain sync` and `pbrain sync --personal`; the
  `UnsupportedModel` message says `pbrain sync lists the supported names`; the waiting message is
  `waiting for another pbrain sync`. The notice code `index_progress` and every JSON field stay.
- Text: `internal/skill/core.md`, `internal/skill/topics/semantic.md` (word limits unchanged: one
  word replaces one), `README.md` (5 places), `tools/phase28/README.md`, `HANDOVER.md`.
- Callers of the command: `internal/semantic/calibrate/model.go:82`,
  `internal/cli/measure_model_test.go:265,306`, the scenario scripts 33, 35 and 37,
  `cmd/pbrain/main_test.go`, `internal/cli/semantic_test.go`, `options_test.go`,
  `outputfix_test.go`, `internal/skill/skill_test.go`.
- **Do not edit**
  `internal/semantic/calibrate/testdata/retrieval/brain/decisions/index-refresh.md`. It is an
  item of the frozen evaluation fixture; changing its text changes the calibration.

Order: (1) the table, the alias, the notice and `TestCLIArgumentMatrix`'s alias cases;
(2) the messages and `TestSemanticNoticeTable`; (3) skill text, README and the other callers;
(4) one scenario script keeps a `pbrain index` line to prove the alias from the built binary.

Errors: `index --bogus` is the usage error of `sync --bogus`. After the alias is removed,
`index` is an unknown command (usage, exit 2).

## B. The digests: interfaces and steps (#25)

```go
package semantic

// Check is the cheap identity of the bytes ContentDigest hashes: their CRC-32C (Castagnoli,
// hash/crc32) and their count, 8 + len(title) + len(body).
type Check struct {
    CRC  uint32
    Size int64
}

func ContentCheck(title string, body []byte) Check   // new
func ContentDigest(title string, body []byte) [32]byte // unchanged; the build only
```

- `passages.go`: add `Check` and `ContentCheck`, over the same framed bytes as `ContentDigest`
  (the title's length as 8 little-endian bytes, the title, the body), with the package's existing
  `castagnoli` table (`cache.go:30`). `Passage.Digest` and `IndexFingerprint` do not change.
- `index.go`:
  - `indexFormat = 3`.
  - `manifest.SHA256` (`"sha256"`, 64 hex) becomes `CRC32C` (`"crc32c"`, 8 lower-case hex,
    `%08x`). `readManifest`, `readDatabase` and `publish` follow.
  - Schema: `item` gains `crc INTEGER NOT NULL CHECK (crc BETWEEN 0 AND 4294967295)` and
    `size INTEGER NOT NULL CHECK (size >= 8)`. `content` and the `passage` table stay.
  - `storedItem` holds `id` and `check Check`. `openIndex` reads `id, path, content, crc, size`
    and returns `IndexUnreadable` for a `content` that is not 32 bytes, a `crc` that is not an
    integer in range or a `size` below 8. Table checks do not bind a file written by other hands.
  - `putItem(ctx, path, content [32]byte, check Check, passages)`.
- `source.go` `covers`: compare `ContentCheck(title, body)` with the stored check. It serves the
  search and `Status`; neither calls `ContentDigest`.
- `build.go` `run`: step 6 compares by `ContentCheck`; `ContentDigest` is computed only for an
  item that goes to `todo`; `work` carries both to `putItem`.
- `cache.go`: no change.
- Errors: no new code. A format-2 index is `IndexUnreadable` through the existing path.

Order, each step with its proving test (`components/semantic.md`, "Test plan"):

1. `ContentCheck`: `TestContentDigestFraming` (the CRC part).
2. Schema, `putItem`, `openIndex`: `TestIndexRoundTrip`, `TestIndexRejects` (bad `crc` and
   `size` rows).
3. Manifest and whole-file CRC-32C: `TestManifestIsNotTrusted`, `TestIndexRejects` (one changed
   byte), `TestDamagedPassageIsRepaired`.
4. `covers` and the build: `TestCoverage` (the limit and the call counts),
   `TestBuildFirstAndNoOp` (no `ContentDigest` on a no-op run), `TestBuildIncremental`,
   `TestBuildRename`, `TestBuildFromCache` unchanged.
5. Format change: `TestIndexFormatChange`.
6. Light check, pinned (`taskset -c 4-7`, `go test -p 2`): `./internal/semantic/...` and
   `./internal/cli/...`. The warm check of `internal/cli/README.measure.md` is rerun when heavy
   runs are allowed again, and its figures replace the scratch figures above.

The two issues touch different files except `internal/cli/semantic_test.go` and the scenario
scripts, which both edit. #24 goes first; #25 rebases on it.

## Risks and gaps

- The gain is measured in a scratch program on one machine, not on the built command.
- `components/semantic.md` says the coverage check is computed in parallel; `source.go:176` runs
  it in one loop. The plan does not change that.
- The item's SHA-256 (`item.content`) is written and read by nothing in format 3.

## Open questions

| Question | Owner |
|---|---|
| Do #24 and #25 ship in v0.1.1, which is not tagged yet, or after it? If in v0.1.1, its release notes name `pbrain index` four times and must change, no format-2 index was ever published, and the alias serves nobody outside the team. | @operator, for the owner |
| Keep `item.content` although nothing reads it? The decision says "beside its existing SHA-256", so the plan keeps it. Dropping it saves 32 bytes an item and one hash per changed item. | the owner |
| The `command_renamed` notice is the architect's addition to "hidden alias". Without it the removal in the next release comes unannounced. | @operator |
| The personal section is embedded again on the format change. Accept it, or keep a one-time reader of format 2? The plan accepts it. | the owner |
