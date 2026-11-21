---
status: complete
---

# Component: Semantic search (`embed` and `semantic`)

## Purpose and scope

These two packages give `search --semantic` its second source of results and give `pbrain index` its work. `embed` owns the `Embedder` seam, the table of supported models, and finding, downloading and verifying a model. `semantic` cuts items into passages, keeps one vector index per section and one vector cache per user, builds the index incrementally, reports its state, and scores eligible items for a search. [Architecture §§7.4, 8.5 and 8.7](../architecture.md) · [Functional spec §§7.1, 9.3, 9.4 and scenarios 33 to 37](../functional_spec.md)

Out of scope:

- The cut-off test, the order of added results, snippets and notices: `search` and `cli` (architecture §8.5).
- Headings and sections: `outline` (architecture §7.4).
- The scan, section handles, locks and file primitives: `store`, `brain`, `sys`.
- Models other than the two the owner chose on 2026-11-21. The adapter for bge-small-en-v1.5 on aikit's encoder is designed below, from its check (`research/semantic-search-and-index/embedding-models-and-go-runtime/goinfer-bge-small-check.md`). hugot's pure-Go backend, on which the model was first accepted, is the fallback: its adapter design is kept in `bge-small-hugot-adapter-design.md` in that folder, beside its spike. The adapter for EmbeddingGemma 2 on our goinfer is designed as far as today's evidence reaches, with its open parts named.
- BM25 and the partial-match tier (`search.md`): they need no model, no index and nothing from these packages.

## Interfaces

```go
package embed

type Role uint8

const (
    Query Role = iota
    Document
)

type Embedder interface {
    // Embed returns one vector per text, in order, each of Dim() values and of unit length.
    // A text longer than the model takes is cut at a token boundary, never refused.
    // Errors: the context's error, or a plain error for a runtime failure.
    Embed(ctx context.Context, role Role, texts []string) ([][]float32, error)
    Dim() int
    Fingerprint() string
    Close() error
}

type File struct {
    Name   string   // file name inside the model folder
    URL    string   // https, pinned in the release
    SHA256 [32]byte
    Size   int64
}

type Model struct {
    Name        string
    Runtime     string
    Files       []File // what the adapter is given, each pinned by digest
    Source      []File // the published files a pack is derived from; empty when Files are downloaded as they are
    Dim         int
    Cutoff      float32
    Fingerprint string // lower-case hex SHA-256; see "Fingerprints"
}

// Artifact is a model read into private memory, once, and found to be the pinned one by
// hashing those very bytes. Only Verify makes one.
type Artifact struct {
    Model Model
    Files [][]byte // each file's bytes, in the order of Model.Files
}

// Entry joins one model row with the functions of its adapter.
type Entry struct {
    Model Model
    Open  func(ctx context.Context, a *Artifact) (Embedder, error)
    // Derive is set only for a model with Source. It is pure: from the verified bytes of
    // Model.Source, in order, it returns the bytes of Model.Files, in order. It opens no file.
    Derive func(source [][]byte) ([][]byte, error)
}

// NewRegistry builds a Registry from entries. cmd/pbrain passes the release's entries; tests pass their own.
func NewRegistry(entries ...Entry) Registry

type Registry interface {
    Lookup(name string) (Model, bool)
    Names() []string // sorted
    // Open builds the model from a.Files alone. It reads nothing from disk. It takes the bytes over:
    // a.Files is nil when it returns, with or without an error, and the caller keeps no other reference to them.
    Open(ctx context.Context, a *Artifact) (Embedder, error)
    // Derive calls the entry's Derive; an error for a model without Source.
    Derive(m Model, source [][]byte) ([][]byte, error)
}

// FolderName is m.Name, a hyphen, and the first 16 hex characters of the SHA-256 over each
// file's name and digest in table order: the model's folder under <cache>/models/.
func FolderName(m Model) string

var ErrAbsent = errors.New("model files absent")
var ErrWrongModel = errors.New("not the named model")

// Find returns the folder that holds every file of m, by name and size. It is a cheap first
// test, never a licence to open: only an Artifact from Verify can be opened.
// With modelDir set (personal.toml model_path) only that folder is tried.
// Errors: ErrAbsent when a file is missing; ErrWrongModel when one has the wrong size.
func Find(files sys.Files, m Model, modelDir, cacheModels sys.Dir) (sys.Dir, error)

// Verify reads every file of m in dir whole into memory, at most File.Size bytes plus one,
// hashes the bytes it read and returns them as an Artifact. Files are read and hashed
// concurrently, one worker per processor.
// Errors: ErrAbsent, ErrWrongModel (a size or digest that is not the pinned one), or a file-system error.
func Verify(files sys.Files, dir sys.Dir, m Model) (*Artifact, error)

// Download fetches the files of m that cacheModels/<FolderName(m)>/ lacks, and gives a file its name
// only when its size and SHA-256 are the pinned ones. For a model with Source it fetches the source
// files into cacheModels/<FolderName(m)>.source/ instead, for Pack. It returns that folder.
// Errors: the context's error; *errs.Error fetch_failed (transfer, size or digest), io (the destination).
func Download(ctx context.Context, files sys.Files, dl Downloader, m Model, cacheModels sys.Dir) (sys.Dir, error)

// Pack turns downloaded source files into the model's pack. It reads every file of m.Source
// in sourceDir into memory and checks size and SHA-256; calls reg.Derive; checks each result
// against the size and SHA-256 pinned in m.Files; and only then writes the files into packDir,
// each through a temporary file, a flush and LinkNew, followed by a sync of the folder.
// Nothing is written when any check fails. embed imports no adapter: the layout is the entry's.
// Errors: ErrAbsent, ErrWrongModel (source or result not the pinned ones), or a file-system error.
func Pack(files sys.Files, reg Registry, m Model, sourceDir, packDir sys.Dir) error

// Downloader is satisfied by *fetch.Client (components/importer.md).
type Downloader interface {
    Download(ctx context.Context, rawURL string, dst io.Writer, maxBytes int64, idle time.Duration) (written int64, err error)
}
```

```go
package semantic

const ChunkerVersion = "1"

type Passage struct {
    Start, End int      // code points in the stored body
    Text       string   // what is embedded
    Digest     [32]byte // SHA-256 of Text
}

// Passages cuts one item. It is pure and never fails.
func Passages(title string, body []byte) []Passage

// ContentDigest is the SHA-256 of the title's length in bytes as 8 little-endian bytes, the
// title's bytes and the body's bytes: everything an item's passages are cut from. The length
// frames the title, so no title and body can be mistaken for another pair.
func ContentDigest(title string, body []byte) [32]byte

// IndexFingerprint is the lower-case hex SHA-256 of "pbrain-index\n" + ChunkerVersion + "\n" + m.Fingerprint.
func IndexFingerprint(m embed.Model) string

type SectionDir struct {
    Section item.Section
    Dir     sys.Dir // from Brain.OpenSection
    Root    string  // the section's absolute folder; item paths are made relative to it
}

// SectionInput is one section as one scan saw it.
type SectionInput struct {
    SectionDir
    Items []*item.Item                      // the valid items of this section
    Body  func(*item.Item) ([]byte, error) // the stored body as scanned: store.Catalog.Body
}

// ScanFunc scans one section afresh. cli implements it with store.Scan, bodies kept.
type ScanFunc func(ctx context.Context) (SectionInput, error)

type Options struct {
    Files     sys.Files
    Registry  embed.Registry
    ModelName string   // brain's Config.Semantic.Model: the name pbrain.toml gives, the default when it gives none, "" when semantic search is off
    ModelDir  sys.Dir  // personal.toml model_path, opened by cli; nil when unset
    Cache     *Cache   // nil when no cache folder is known
    CacheErr  error    // ErrCacheExposed when the folder failed its test
    Rand      io.Reader
    Now       func() time.Time
}

type CacheOptions struct {
    Path      string   // absolute: PBRAIN_CACHE, else the user's cache folder plus "pbrain"
    Forbidden []string // resolved roots it may not lie in: the brain, the personal section
    Git       gitx.Runner
    Files     sys.Files
}

var ErrCacheExposed = errors.New("cache folder is exposed")

// OpenCache tests the folder and opens it. With create false a missing folder gives a Cache
// that holds nothing. Every folder a Cache later opens is tested again for a nested repository.
// Errors: ErrCacheExposed; git_required; a file-system error.
func OpenCache(ctx context.Context, opt CacheOptions, create bool) (*Cache, error)

// Start needs no scan and never fails. It runs the cheap checks and, when they pass, begins
// in the background to verify the model, open it and embed text as a query.
func Start(ctx context.Context, opt Options, text string, dirs []SectionDir) *Source

// Attach gives the source the scanned items. Call it once, after the scan, before Score.
func (s *Source) Attach(secs []SectionInput)

// Score joins the background task and compares. text must be the text given to Start.
// It returns only the context's error.
func (s *Source) Score(ctx context.Context, text string, eligible []*item.Item) (search.Scores, error)

// Close cancels the background task, waits for it and closes an opened embedder exactly once.
// It is safe to call when Attach or Score were never reached.
func (s *Source) Close() error

type BuildResult struct {
    Section          item.Section
    Items            int // items the index holds after the run
    ItemsRemoved     int // items gone from the section whose entries this run dropped
    PassagesEmbedded int
    PassagesReused   int // taken from the index's own entries or from the cache
}

type Progress struct {
    Section     item.Section
    Done, Total int  // items to do
    Waiting     bool // waiting for another build's lock
}

// Build brings one section's index up to date. It calls scan after it holds the index lock and
// works from that scan alone. An item whose body cannot be read as scanned is left for the next run.
// Errors, all *errs.Error: no_model; invalid_config (unknown model, a model_path folder that is
// incomplete or fails Verify, an exposed cache, or neither a cache folder nor a model_path);
// fetch_failed from the download; io for the file system, the lock and a failed embedding.
func Build(ctx context.Context, opt Options, sec SectionDir, scan ScanFunc, dl embed.Downloader,
    notify func(embed.Model, int64), progress func(Progress)) (BuildResult, error)

type IndexState uint8 // IndexOK, IndexNone, IndexStale, IndexUnreadable

type SectionStatus struct {
    Section        item.Section
    Items, Indexed int
    State          IndexState
}

type StatusResult struct {
    Model        string // "" when semantic search is off
    Supported    bool
    ModelPresent bool
    Sections     []SectionStatus
}

// Status loads no model and writes nothing. It returns only file-system errors (io).
func Status(ctx context.Context, opt Options, secs []SectionInput) (StatusResult, error)
```

`Downloader` is `fetch.Client.Download` as the importer design defines it: https on every hop, the import's direct connection and address check, no content-type rule, a byte ceiling and an idle timeout in place of the 30-second total. `embed` owns the digest check and the publication.

## Internal design

### Fingerprints

A model's fingerprint is computed when the table is built, never at run time: SHA-256 over the model's name, each file's name and digest in table order, `Dim`, and the adapter's preparation version, a constant the adapter bumps when it changes a prompt, the pooling, the normalisation or the token limit. `Registry.Open` must return an embedder with the same `Fingerprint()` and `Dim()`, and `Build` and the source treat a difference as a runtime failure. The index fingerprint adds `ChunkerVersion`. A change of either starts a new index; a change of the chunker alone still finds every unchanged text in the cache, because the cache is keyed by the model fingerprint.

### Passages

`Passages` works on `outline.Parse(body)`.

1. For each section, the candidate text is its own span `[Start, End)`. For a section with a level above 0, if everything after its heading (one line for ATX, two for setext) is white space, the section gives no passage.
2. A candidate of at most 1,800 code points is one part. A longer one is cut into paragraphs: a paragraph ends after a run of blank lines, a fenced block (by the fence rule of architecture §7.4) is never cut by that rule, and each paragraph keeps its trailing blank lines, so the paragraphs tile the candidate. Parts are filled greedily: a paragraph is added to the current part unless the part is non-empty and would pass 1,400 code points, in which case the part is closed first.
3. A part still longer than 1,800 (one long paragraph) is cut repeatedly from its start: after the last line feed at or before code point 1,800 if that lies beyond code point 900; else after the last space in that range; else at 1,800.
4. A part that is all white space is dropped. Parts never overlap and are returned in body order.
5. `Start` and `End` are the part's code-point offsets in the body. `Text` is built from the trimmed part: the title, cut at 200 code points; then, for a section with a level above 0, a line with the titles of its ancestors and itself joined by ` > `, empty titles left out, keeping only its last 200 code points when longer; then a blank line and the part. The cap leaves a small model's input mostly to the text the passage is cited for. The layout is fixed so that an adapter can tell the two parts apart: everything before the first blank line is the prefix, everything after it is the passage. This layout and the sizes are what `ChunkerVersion` versions. An ancestor of a section is the nearest earlier section of each lower level, walking backwards.
6. If no passage results, there is one passage with `Start = End = 0` whose text is the title alone.

The title is `Header.Title` after its fallbacks. Tags, kind and other fields are not embedded.

### The model on disk

`<cache>/models/<FolderName(m)>/<File.Name>` holds a downloaded model. The folder's name carries the digest of the model's files, so two releases never share a folder, and a file in it is linked under its name once, read-only, and never replaced. `model_path` names a folder that holds every file of the model's `Files` under the same names; a model of several files is supplied whole, and a missing companion file makes it absent. For a model with a pack, that means the pack: a machine without network gets it by copying the model folder from the cache of a machine that ran `pbrain index`. Source files at `model_path` are not accepted and nothing is derived there, so the folder a person points at is only ever read.

**A model is opened only from verified bytes.** Both `Build` and the source call `Verify`, which reads every file once into the process's own memory through `Files.OpenRegular`, hashes those bytes against the pinned SHA-256, and returns them as an `Artifact`. `Registry.Open` takes only an `Artifact` and the adapter builds the model from its bytes, reading nothing from disk. `Open` takes those bytes over: neither `Build` nor the source keeps another reference, and the registry sets `a.Files` to nil when the entry's `Open` returns, so the rule holds for every adapter and for a test model without code of their own. After the check nothing in pbrain can write to what the model reads. The bytes hashed are the bytes consumed, so a file replaced by name, or changed in place through the same open file, after the check cannot reach the model, whether the cause is another release, another process or a person installing into `model_path`. The price is one copy of the model's files in memory: about 134 MB for bge-small. A file of the right name and size with other bytes fails `Verify`: in `Build` that is `invalid_config` for a `model_path` folder; for the cache the whole folder is treated as absent, the bad file is removed and downloaded again; in a search it is not ready, `ModelFailed`. The cost, about 120 ms to read and hash 130 MB, is paid in the background while the scan runs.

`Download` runs only inside `Build`, in its model step, and only for a model that is not at `model_path`. There is no resume: an interrupted download of a file starts that file again. Its steps:

1. `notify(m, bytes)` lets `cli` print the `model_download` notice with the total size and the host before the first request.
2. Each missing file is streamed by `Downloader.Download` into a temporary file in the model folder through a hashing writer, with `maxBytes = File.Size` and a 30-second idle timeout. A count other than `File.Size` or a wrong digest removes the temporary file and fails with `fetch_failed`.
3. The file gets its name with `LinkNew`. If the name appeared meanwhile, the existing file is kept. Either way the folder then goes through `Verify` like any other before it is used.

### The index database

```
<section>/.pbrain/index/
  lock                  taken by Build for its whole run
  manifest              JSON, replaced by rename
  <16 hex>.db           a SQLite database; the name is 8 random bytes
```

The manifest:

```json
{"format": 2, "fingerprint": "<64 hex>", "model": "<name>", "dim": 384, "db": "0f3a9c1e77b2d455.db", "size": 59768832, "sha256": "<64 hex>"}
```

The database, created by this schema (`1536` stands for four times the dimension, written as a literal when the table is created):

```sql
CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL) WITHOUT ROWID;
CREATE TABLE item (
    id      INTEGER PRIMARY KEY,
    path    TEXT NOT NULL UNIQUE,
    content BLOB NOT NULL CHECK (length(content) = 32)
);
CREATE TABLE passage (
    id      INTEGER PRIMARY KEY,
    item_id INTEGER NOT NULL REFERENCES item(id) ON DELETE CASCADE,
    ord     INTEGER NOT NULL,
    start   INTEGER NOT NULL,
    stop    INTEGER NOT NULL CHECK (stop >= start),
    digest  BLOB NOT NULL CHECK (length(digest) = 32),
    vector  BLOB NOT NULL CHECK (length(vector) = 1536),
    UNIQUE (item_id, ord)
);
CREATE INDEX passage_digest ON passage(digest);
```

`meta` holds four rows: `format` (`2`), `fingerprint`, `model` and `dim`. `item.path` is the item's path in the section with forward slashes; `item.content` is its `ContentDigest`. `passage.vector` is `dim` float32 values, little-endian; `passage.digest` is the SHA-256 of the embedded text. One path has one row, so an item is in the index once.

**SQLite is used in memory only; it is never given a path.** The package opens one connection to a private in-memory database (`file::memory:`, with `journal_mode=MEMORY`, `temp_store=MEMORY` and `foreign_keys=ON` set before the first transaction) and moves whole databases in and out of it with the driver's `Deserialize` and `Serialize`. The engine is `modernc.org/sqlite` with its bundled sqlite-vec; both are registered by import. The journal is kept in memory and not switched off, because SQLite does not define what a rollback does without one: a transaction that fails must leave the item as it was. Consequences: the handle rule of architecture §10.3 holds unchanged, since every byte on disk is read and written through `sys`; no journal, write-ahead log or shared-memory file ever exists; and a reader takes no lock.

**Reading** (`openIndex`, used by the source, the build and the status). Nothing from the manifest is used before it is checked:

1. Open `manifest` with `Files.OpenRegular` and read at most 4,097 bytes of it. Missing: `IndexNone`. Longer than 4,096 bytes, not a JSON object, or a `format` other than 2: `IndexUnreadable`. It is also `IndexUnreadable` unless `db` is exactly sixteen lower-case hexadecimal characters followed by `.db`, `sha256` is 64 lower-case hexadecimal characters, and `size` is a whole number from 1 to 2,147,483,647. Another fingerprint or dimension, with all of that in order: `IndexStale`.
2. Open the named file with `Files.OpenRegular` and ask the open file for its size. It must equal `size`, or the index is `IndexUnreadable` and nothing is allocated. Then read exactly that many bytes and compare their SHA-256 with the manifest. A missing file: read the manifest once more, since a build may have published; still missing, or any mismatch: `IndexUnreadable`.
3. `Deserialize` the bytes and drop pbrain's own copy. Read the four `meta` rows and require the manifest's values. Create the temporary table `eligible` and prepare the scoring statement below, which fails when a table or column it needs is missing or of another shape. A failure here: `IndexUnreadable`. A digest that matches says the file is the one that was published; it does not say the file is a pbrain index, which this step does.
4. Read `item` whole into a map from path to id and content digest.

5. **For the build and the status only**, a full check of what is stored, which a search does not pay for: no passage whose `vector` is not a blob of exactly four times `dim` bytes; no passage with `stop` below `start`; no passage without its item; and the scoring query run once with `eligible` holding every item and a fixed unit vector, read to its end. A failure: `IndexUnreadable`.

An index is **usable** when all four steps pass, and for the build and the status the fifth as well. An entry **covers** an item when the item's path relative to `Root` is in that map and the stored digest equals `ContentDigest(item.Header.Title, body)`, with the title and body exactly as this command scanned them. The item's `Version` is not used: a CRC and a length cannot show that a vector was made from the present text. A change to front matter that leaves the title alone, such as `verify`, keeps the entry. The digest is computed, in parallel, only for items whose path has an entry; an item whose body cannot be read as scanned is not covered.

**Scoring** is one query per section, over the items that may be scored and no others. `eligible` is a temporary table of the connection (`CREATE TEMP TABLE eligible (id INTEGER PRIMARY KEY)`), emptied and filled for each search with the ids of the items that are eligible and covered. The query vector is a blob of `dim` float32 values:

```sql
SELECT item_id, ord, start, stop, d
FROM (SELECT p.item_id, p.ord, p.start, p.stop, vec_distance_cosine(p.vector, ?1) AS d
      FROM temp.eligible e CROSS JOIN passage p ON p.item_id = e.id)
WHERE d <= ?2;
```

`CROSS JOIN` makes SQLite start from `eligible` and reach `passage` through the index of `UNIQUE (item_id, ord)`, so `vec_distance_cosine` is called only on the passages of listed items. An item that is filtered out or not covered is never scored, and a bad row belonging to one cannot fail the query; a whole-table scan failed in exactly that case when tried. `?2` is `1 - Cutoff + 0.000001`: the query returns every passage of those items at or just under the cut-off, with no limit on their number, and `search` applies the cut-off exactly. Vectors are in an ordinary table and not in a `vec0` virtual table, because `vec0`'s nearest-neighbour query refuses more than 4,096 rows and its row-by-row read was 20 to 100 times slower in our measurement (architecture §8.7).

**Publishing** (the build only): `Serialize` the in-memory database; if the result is larger than 2,147,483,647 bytes, fail with `too_large` and leave the previous index in force; write the bytes to a temporary file in `index/` and flush it; link it under a new random name of the form above; sync the folder; write a manifest that names it with its size and SHA-256 and rename that into place; sync; remove the previous database file. Only a name of the generated form is ever opened or removed, so no manifest, however damaged, can make a build remove `lock`, `manifest` or anything outside the folder. A failure before the rename leaves the old index in force.

### Build

For one section, `Build`:

1. Fails with `no_model` when `ModelName` is empty, which means semantic search is off, and with `invalid_config` when the registry does not know it (the message lists `Names()`), when `CacheErr` is set, or when there is neither a cache nor a `ModelDir`.
2. If the section has no `.pbrain/index/` folder, calls `scan` once without a lock; with no item it returns a zero result and creates nothing. An empty section therefore never gets a `.pbrain/` from this command. If the folder exists, the build goes on even with no item: an index that is unreadable or built for another fingerprint is then replaced by an empty database, so the status stops reporting it.
3. Opens `.pbrain/index/`, creating both folders, and takes `lock`. The wait has no limit of its own: after 2 seconds it reports `Progress{Waiting: true}` once, and the context ends it. Then it calls `scan`: the catalog it works from is always taken while it holds the lock, so a build that waited cannot publish from an older view of the files than the build before it, or delete that build's rows.
4. **Makes sure of the model**, whenever the scan found at least one item, whether or not anything will be embedded. For a model in the cache it first takes the model's own lock, `<cache>/models/<FolderName(m)>.lock`, with `Files.Lock` under the command's context, and holds it until the model is verified: two projects of one person share the cache and each holds only its own index lock, so without it one build could remove source files another is still reading. The order is always index lock, then model lock (architecture §10.4), and this is the only place two locks are held at once. With the lock held it looks again, since another build may have finished the work. It calls `Find`; if the model is absent it calls `Download`, unless `ModelDir` is set, in which case an absent or wrong-sized file is `invalid_config`. For a model with `Source`, the files fetched are the source files, into `<cache>/models/<FolderName(m)>.source/`, each kept only if its size and digest are the pinned ones; then `Pack` writes the pack into `<cache>/models/<FolderName(m)>/`, and the source files are removed once `Verify` has accepted the pack; the empty `.source` folder may remain. A run interrupted before that finds the source files again and does not download them twice. Deriving holds the source and the pack in memory together, about 2 GB for EmbeddingGemma 2, once. It then calls `Verify`. A failure of `Verify` is `invalid_config` for `ModelDir`; for the cache the files that failed are removed, obtained again and verified once more, and a second failure is `fetch_failed`. So a run that reuses every vector still restores a model whose cache was deleted and still refuses a wrong local copy, and after any successful run a search finds the model. A source digest that is wrong is `fetch_failed`; a pack that does not come out with its pinned digests, and any file-system failure, is `io`; in every case no unverified pack is left under the model's name. The model lock is released here. The `Artifact` is kept only until it is known whether a passage must be embedded; the inference runtime is opened only for the first batch.
5. Opens the index (`openIndex`, with its full check). If it is not usable, which includes an index whose stored passages fail that check although no item has changed, it starts a new in-memory database with the schema and the four `meta` rows for the expected fingerprint. It then removes leftovers of a crash or of an older fingerprint: temporary files, and files with a name of the generated database form other than the one in use. It removes nothing else, whatever the manifest says.
6. Computes each item's content digest, then finds the items no entry covers (to do), in path order, and the rows whose path has no item in the scan (dead); `ItemsRemoved` counts the dead paths. Dead rows are kept until step 10's last publication, so that step 7 can still take their vectors: an item that was only renamed is not embedded again.
7. For each item to do: cuts its passages from the same title and body that were hashed, and for each passage looks for a vector by digest, first in `passage` (through its index on `digest`, dead rows and the item's own old rows included), then, for the shared section, in the cache. Passages found are reused. The others are queued.
8. Embeds queued passages eight at a time, as `Document`. Before the first batch it opens the `Artifact` of step 4 with `Registry.Open`; with no batch, the runtime is never opened and the `Artifact` is dropped. A vector from the model that is not of the right length, not finite or not of unit length within 0.001 fails the run with `io`; a vector from the cache or from the database that fails the same test is treated as absent and embedded again. For the shared section each new vector is written to the cache at once.
9. When all passages of an item have vectors, replaces the item in one transaction: delete the row of that path, which removes its passages, then insert the item and its passages.
10. Publishes whenever 20 seconds have passed since the last publication, at an item boundary, and at the end if anything changed. Before the last publication the dead rows are deleted in one transaction. A publication in between may still hold them; they cover no item and are never scored. What an interruption can lose is therefore at most 20 seconds of work plus the item in progress, however long that item is; for the shared section its vectors are already in the cache. A run that changed nothing publishes nothing.
11. Reports `Progress` every 10 seconds, releases the lock and returns the counts.

An interrupted run leaves a usable index holding every published item; the next run finds the rest to do. An item whose body cannot be read as scanned (`store.ErrChanged`) is skipped and stays to do. Publication rewrites the whole database, about 160 ms at 35,000 passages of 384 dimensions by a scratch measurement, which is why it is paced by time and not by count.

### The vector cache

`<cache>/vectors/<model fingerprint>/<first two hex of the digest>/<digest hex>` holds one vector: `dim` float32 values, little-endian, then the CRC-32C of those bytes. A read accepts only a file of exactly that size with a matching checksum and finite values of unit length within 0.001; anything else is removed and counts as absent. A write goes to a temporary file in the same folder and is linked under its name without a flush; an existing name is left alone. No lock is taken. Only `Build` of the shared section reads or writes it.

`OpenCache` decides whether the folder may be used, before creating anything:

1. Resolve `Path` with `Files.Resolve(path, true)`. If it does not exist, resolve its nearest existing ancestor instead and append the missing names.
2. If the resolved path equals or lies under a `Forbidden` root: `ErrCacheExposed`.
3. Find the work tree of the resolved path (or of that ancestor) with `gitx.Top`. Outside any work tree the folder is accepted.
4. Inside one, the planned folder, passed as a directory, must be reported by `gitx.Ignored`, and `gitx.Tracked` must find nothing under it. Otherwise: `ErrCacheExposed`.
5. No repository may sit inside the cache where pbrain works. Every folder the cache opens, from its root down (`models/`, `models/<FolderName(m)>/`, `vectors/`, `vectors/<fingerprint>/` and each two-character folder under it), is tested with `Files.Stat(dir, ".git")` when it is opened and before anything is read, created or written in it; an entry of that name, of any type, is `ErrCacheExposed`. Folders are opened through handles and links are never followed, so a file pbrain writes can lie in no repository that starts at or below the cache root. A search tests only the three folders it reads; a build tests each folder as it first uses it.

`cli` computes `Path` from `Env.Getenv` alone: `PBRAIN_CACHE`, made absolute against the current directory; else `$XDG_CACHE_HOME/pbrain` when that variable is an absolute path, else `$HOME/.cache/pbrain`, on Linux; `$HOME/Library/Caches/pbrain` on macOS. With none of these set there is no cache: `Options.Cache` is nil.

### The source

`Start` runs the cheap checks in this order and remembers the first that fails as `search.NotReady`:

| Check | Reason |
|---|---|
| `ModelName` is empty: semantic search is off | `NoModel` |
| the registry does not know the name | `UnsupportedModel` |
| `CacheErr` is set, or a cache folder on the way to the model holds `.git` | `CacheExposed` |
| `embed.Find` fails: a file missing or of the wrong size | `ModelAbsent` |
| no section in `dirs` has a manifest with the expected fingerprint | `NoIndex` |

If all pass, it starts one goroutine that runs `embed.Verify`, opens the `Artifact` it returns, and embeds the text as `Query`, while a second opens each section's index. The query vector must have `Dim` finite values and unit length within 0.001. The goroutine runs under a context that `Close` cancels. `Score`, after `Attach`, then:

1. Takes each section's index, which the background task has opened with `openIndex` beside the model, and decides coverage. A section whose index is not usable contributes all its items to `Missing`; in a usable one, each item no entry covers adds one. If no section is usable: not ready, `NoIndex`.
2. Waits for the background task. A failed verification, a failed open, a failed embedding or a query vector that fails its test: not ready, `ModelFailed`, and the search keeps its lexical results. A `text` other than the one given to `Start` is a programming error and panics.
3. For each usable section, fills `eligible` with the ids of the items that are eligible and covered, and runs the scoring query. It drops every row whose span does not lie inside the body as scanned. For each item it keeps the row with the smallest distance, and on a tie the smallest `ord`. The score is `1 - d` as a float32. If filling the table, the query or the reading of its rows fails, every row already read for that section is dropped, the section is treated as not usable for this search and all its items are added to `Missing`, once; the other section's hits stand. If no section is left: not ready, `NoIndex`. The hits are returned in the order of the caller's list; items with no row are not hits.
4. Returns `Ready`, `Missing`, the model's `Cutoff` and the hits.

`Score` returns an error only when the context ends. The source never writes, never takes a lock and never downloads.

### What `cli` does

- **Which model.** `Options.ModelName` is `Config.Semantic.Model` as `brain` resolved it (`components/brain.md`): the name `pbrain.toml` gives; `brain.DefaultModel`, which is `embeddinggemma-2`, when it has no `semantic.model` key; and `""` when it says `"none"`. `semantic` and `embed` hold no default of their own, so a test registry needs no entry of that name, and a test in `cli` or `cmd/pbrain` checks that the release registry holds `brain.DefaultModel`.
- `search --semantic`: after `brain.Open` and before the scan, open the cache with `create = false` and call `Start` with the query text and the view's section folders, deferring `Close` so that a failed scan or any later error cancels the work. After the scan, which already keeps bodies for a search, `Attach` one `SectionInput` per section, put the source in `search.Request.Semantic`, and turn `Result.Sem` into one notice: `semantic_not_ready` with the reason and `pbrain index` (or `PBRAIN_CACHE` for an exposed cache), or `semantic_partial` with `Missing` when it ran and `Missing` is above zero.
- `pbrain index`: open the cache with `create = true`, then call `Build` for the shared section and, with `--personal`, for the personal one, in that order, each with a `ScanFunc` that scans with bodies kept, and print one line per result. A personal section whose folder does not exist is not created: its line reports no item. `notify` becomes the `model_download` notice and `progress` the `index_progress` notices.
- `pbrain index --status`: one scan with bodies kept, the cache with `create = false`, then `Status`. A section whose folder or index folder does not exist is reported as `IndexNone`, with its item count, and nothing is created. `ModelPresent` is the result of `Find` and `Verify` together.

### Output and notice texts

`cli` prints these; this section is their one definition. No text names an item, a path or a URL.

**`pbrain index`.** One line per `BuildResult`, in the order built, all four numbers always present, a noun singular when its number is 1:

```
shared: 5000 items indexed, 1 item removed; 12 passages embedded, 3 reused
```

JSON: `{"model": "<ModelName>", "sections": [{"section": "shared", "items": 5000, "items_removed": 1, "passages_embedded": 12, "passages_reused": 3}]}`, numbers printed also when zero.

**`pbrain index --status`.** The first line is one of `model: none` (semantic search is off), `model: <name>, on this machine`, `model: <name>, not on this machine` and `model: <name>, not supported by this pbrain`. Then one line per section of the view: `<section>: <Indexed> of <Items> items indexed`, followed by ` (built for another model)` for `IndexStale` or ` (unreadable)` for `IndexUnreadable`; `IndexNone` adds nothing. JSON: `{"model": "<name>", "model_present": true, "sections": [{"section": "shared", "items": 5000, "indexed": 4988}]}`, with `"stale": true` or `"unreadable": true` on a section; `model` is left out when semantic search is off and `model_present` when false.

**Notices.** `<x>` is `pbrain index` in the shared view and `pbrain index --personal` in the personal view.

| Code | When | Message |
|---|---|---|
| `semantic_not_ready` | `NoModel` | `semantic search is off: pbrain.toml sets model = "none"` |
| `semantic_not_ready` | `UnsupportedModel` | `this pbrain does not support the model pbrain.toml names: pbrain index lists the supported names` |
| `semantic_not_ready` | `CacheExposed` | `the cache folder lies in the brain, the personal section or a Git repository: set PBRAIN_CACHE to a folder outside them` |
| `semantic_not_ready` | `ModelAbsent` | `the model is not on this machine: run <x>, or set model_path` |
| `semantic_not_ready` | `NoIndex` | `no index for this model: run <x>` |
| `semantic_not_ready` | `ModelFailed` | `the model could not be used: run <x> to check it` |
| `semantic_partial` | `Ready` and `Missing > 0` | `<Missing> items are not in the index: run <x>` (`1 item is` for one) |
| `model_download` | from `notify`, before the first request | `downloading model <name>, <n> MB, from <host>`, with the size in units of 1,000,000 bytes rounded up and the distinct hosts of the files' addresses, comma-separated |
| `index_progress` | from `progress`, every 10 seconds | `<section>: <Done> of <Total> items`, or `<section>: waiting for another pbrain index` when `Waiting` |

### Calibration

`internal/semantic/calibrate` is a program in the repository, not part of the binary. It takes a fixture folder and a model name. The fixture is a brain and `questions.json`, frozen before any tuning: a list of `{"id": "…", "q": "…", "kind": "answerable" | "unanswerable", "expect": ["<item id>"], "half": "calibration" | "held-out", "regression": true}`, where `expect` belongs to answerable questions and `half` and `regression` to unanswerable ones. Each question also carries a `style`, one of the kinds listed under "The wider comparison", and a question answered by one section of a long source carries `evidence` ("Reaching the evidence"). The tool refuses a fixture in which an unanswerable question has no `half`, or in which there are fewer than 30 answerable questions or fewer than 30 unanswerable ones in either half. It builds the index, then sets and checks each model's cut-off:

1. **Cut-off.** For each calibration question it takes the highest score among the eligible items that are not exact matches. Two things keep that number honest. The scores are collected with no cut-off in force: the tool passes `Score` a copy of the model's row whose `Cutoff` is −1, so the scoring query, which is the production query, returns every passage. And the exact matches are the whole list, from `search.Run` called directly with a limit equal to the number of items, not the first page of a command. A question with no item left to score contributes no score, which reaches no cut-off. The cut-off is the smallest two-decimal value `c` such that at most 5% of those scores satisfy `score >= c`, the comparison a search uses, so a score exactly at the cut-off counts as reaching it.
2. **Abstention gate.** With that cut-off in a copy of the model row, it runs each held-out question through `cli.RunWith` as `search --semantic --json` with default limits and reads the page. A question passes only if the page has no result, lexical or added. At least 95% must pass, and every question with `regression` must pass. The report lists each failure with whether it already had a lexical result.
3. **Gain gate.** It runs each answerable question through `cli.RunWith` with and without `--semantic` and looks for an expected id on the first page. The count with the flag must be higher, and no question may have one without the flag and none with it.

It prints the cut-off and both gates and exits 1 when a gate fails; a model whose gates fail stays out of the table, and no other cut-off is tried by hand. The printed cut-off is copied into the model table by hand.

**The wider comparison** (owner's decision, 2026-11-21). The same tool runs every arm below on the same fixture, each through `cli.RunWith` where the product has the mode, with the partial tier switched on in its feature set whatever the release holds back, and through the tool's own scorer where it does not, always over the same items, view and filters and with the same character budget:

| Arm | What it is |
|---|---|
| Strict | `search` as it is: every clause must match. |
| Strict with one guided retry | A second strict search with the words the first one's per-clause counts showed to exist. |
| Any word, by count | Items holding any clause, ranked by how many distinct clauses they hold. Tool only. |
| Any word, by BM25 | The same candidates, ranked by BM25: only the ranking differs from the arm above. Tool only. |
| BM25 with stemming; BM25 with headings weighted | Two variants of the arm above. Tool only. |
| Partial tier | `search --bm25`: the admission rule of `search.md` and BM25 order. |
| Hybrid | `search --semantic`, once per model. |
| All three | `search --bm25 --semantic`. |
| Fusion | Reciprocal-rank fusion of the BM25 and semantic lists. Tool only, for comparison. |

Question kinds, each recorded on the question: a whole sentence worded unlike its answer; two or three words as a working agent would choose; an exact identifier, flag or error string; a question whose answer was superseded; a question answered by one section of a long source; a question the brain cannot answer, far from its subject; one that is near its subject and still unanswerable (a named thing that does not exist); and one that only a personal item answers.

For every arm the report gives, on the rendered first page: questions with an expected id, questions correctly left empty, characters, calls, and time. It also checks that no shared-view output changes when personal items are added. **The partial tier has its own gates**, of the same form as a model's: on the held-out unanswerable questions at least 95% must return an empty page with `--bm25`, every regression case must pass, and more answerable questions must show an expected id with the flag than without. **Both flags together are gated too, for each model:** the same abstention threshold and regression cases on `search --bm25 --semantic`. Two modes can pass alone and fail together: of 40 held-out questions, 38 empty pages with one and 38 with the other can be 36 with both. If the tier fails alone, or together with a model the release lists, with every tested admission rule, the tier is held back; its constants are set on the calibration half only.

**Reaching the evidence** (owner's decision, 2026-11-21, from `research/ken-review.md`; a measurement, with no change to the product). On a long source, finding the item is not the same as seeing the answer. So each question answered by one section of a long source has exactly one expected id and records `"evidence"`: a quotation of at most 120 code points copied from the supporting passage of that item. The tool refuses a fixture in which such a question has no quotation, has more than one expected id, or has a quotation that is not found exactly once in that item's body. Text is compared with every run of white space read as one space, since a snippet prints line breaks as spaces.

For every arm, on these questions, the tool plays one fixed reader, which is given the question and nothing from the answer key:

1. It runs the arm's search with the question's view flags and `--json`, and reads the page. The arm with a guided retry runs both of its searches, and the page is the second one's.
2. If the page has a result, it makes one follow-up read of the first result on the page, with the same view flags: `get <id> --offset <position> --json` with the position that result gave (`snippet_offset`), or `get <id> --json` when it gave none, at the default budget.

One stream serves everything: the `--json` responses the reader received are what it chooses from, what the tool judges and what is measured. Nothing is taken from the text form of a call, whose fit under the same budget can differ (spec 6.1): a fifth result or the end of a body can be in one form and not in the other. The tool judges what the reader received, and only here uses the expected id and the quotation:

- **Retrieved:** the expected id is on the page. Its rank is recorded.
- **Evidence on the page:** the quotation is in the snippet of the expected item's result.
- **Evidence after the read:** the first result is the expected item and the quotation is in the body that the read returned.

Per arm the report gives:

- the number of these questions, how many retrieved the item, and at which ranks;
- how many reached the evidence, on the page and after the read, and that fraction of all of them;
- for the questions that reached it, two sizes, each as the median and the worst. **To the evidence** is every response read before the one that shows the quotation, plus that one up to the end of the quotation. **Delivered** is every response whole, up to and including that one. Every call counts, both searches of a guided retry included. A size is the code points of a response as printed, its JSON escaping included, so these sizes describe the JSON form and not the text form. The end of the quotation in a printed response is found by encoding the field's value up to the end of the quotation with the encoder pbrain prints with. An estimate in tokens, a quarter of the size, is printed beside each size;
- the misses apart, as three counts that enter no size: item not retrieved; item retrieved but not first, with the quotation not in its snippet; item first, with the quotation neither in its snippet nor in the read;
- **an upper bound, labelled as one:** how many questions would reach the evidence if the one read were made on the expected item wherever it stands on the page. It uses the answer key to choose, so it is never added to the figures above.

These figures are reported and not gated: no release decision hangs on them in v1.

### The adapter contract

Whatever runtime is chosen, its adapter (a sub-package of `embed`, imported only where the release registry is built) must: build the model from the `Artifact`'s bytes alone, opening no file; hand those bytes to its runtime without keeping a second reference that could write them, and never write them itself; add the model's own query and document prompts; keep text from a body from being read as a special token of the model; cut an over-long text at the model's token limit, and for a document cut it so that the passage keeps at least half of that limit: the prefix is cut first, only as far as needed and never below half the limit, then the passage from its end; print nothing on standard output; return unit-length vectors of `Dim`; give the same vectors for the same text in any batch; hold no global state between embedders; and make no network call.

**Stray output.** No package that is linked is known to print: the aikit packages were read for it at the release our aikit was forked from, and the loader adds no print; goinfer's were read in its audit. A test of the normal path still cannot show that a failure path stays silent, and one raw line would break the output contract of spec 6.3. So the guarantee is made where it cannot be missed: `cmd/pbrain` takes the process's real standard output and standard error for `Env.Stdout` and `Env.Stderr`, then points the variables `os.Stdout` and `os.Stderr` at the null device, before any command runs. `cli` writes only through `Env`, so its output is unchanged, and anything a linked library prints through those variables is discarded, whatever path it takes. A crash of the Go runtime still reaches the real standard error, because the runtime writes to the descriptor itself. No runtime reads an environment variable either; `TestRuntimeEnvAndOutput` reads the linked sources again on every new pin.

### Our libraries

Both adapters run on libraries the project holds as its own (owner's decision, 2026-11-21). They were forked from one author's two modules, both MIT, and now diverge: each declares its own module path, so pbrain's `go.mod` requires them directly, with no replace directive, and `go install` of a version of pbrain works (architecture §15).

| | goinfer | aikit |
|---|---|---|
| Module, the path in `go.mod` and in imports | `github.com/pkieltyka/goinfer` | `github.com/pkieltyka/aikit` |
| Forked from | `github.com/townsendmerino/goinfer` at `e0d66171` | `github.com/townsendmerino/aikit` at `b719bbda`, the release v1.78.0 |
| Release pbrain pins | v0.2.0 (`a759890b`): the tokenizer pack and the loader from bytes. Our numbering started at v0.1.0 (`bdf9daf9`), the module rename | v1.79.0 (`3c0896c0`): the loader from bytes, under our module path |
| Still to come | nothing in goinfer: the pack and its derive step are the adapter's, in pbrain | nothing pbrain needs |
| Offered to the original author | `townsendmerino/goinfer#1194` | `townsendmerino/aikit#38` |
| Imported by | `embed/gemma2`: the text encoder and tokenizer packages | `embed/bge`: `encoder` and `embed`; and goinfer itself |

- **The pins** (owner's decision, 2026-11-21). pbrain requires `github.com/pkieltyka/aikit v1.79.0` and `github.com/pkieltyka/goinfer v0.2.0`; `go.sum` pins their content. A pin is always a tag on our `main`, never an untagged commit. Both tags exist: aikit v1.79.0 holds everything the bge-small adapter uses, and goinfer v0.2.0 everything the EmbeddingGemma 2 adapter uses.
- **One aikit.** goinfer requires our aikit too. Go links one copy of a module, the newer of the versions required, so pbrain's requirement of aikit is kept at or after the one in the goinfer release it pins, and `TestBgeParity` and `TestGemmaParity` run on what is linked.
- **Changes** are made on a topic branch and reach our `main` through a reviewed pull request in our repository; then pbrain's pin moves. A new pin is read first (what changed in the packages pbrain links, and in the kernels they use), then must pass every test below.
- **The original author's work** is taken only by a deliberate merge into our `main`, read and tested like any change; after the rename such a merge touches every import line. Offering our changes back is optional and is the owner's step. Parity is shown on each pinned commit, not assumed from the releases first measured, goinfer v0.24.1 and aikit v1.78.0.
- **Licence.** Both keep their MIT licence and notice files with the original copyright, and a line that says what they were forked from.

### The bge-small adapter

Package `internal/embed/bge` is the adapter, and the only package of pbrain that imports aikit. Every fact below was checked in `goinfer-bge-small-check.md` against aikit v1.78.0 with the loader added; names of aikit's functions are those of our aikit v1.79.0.

```go
package bge

// Entry is the release row for bge-small-en-v1.5 and its opener.
func Entry() embed.Entry
```

**The model row.**

| Field | Value |
|---|---|
| `Name` | `bge-small-en-v1.5` |
| `Runtime` | `aikit` |
| `Dim` | 384 |
| `Cutoff` | set by the calibration tool before the row ships; the row is not in a release until both gates pass |
| Preparation version | `bge-1`: the query instruction, CLS pooling, L2 normalisation, the 510-token cut rule and the special-token rule below |

Files, from `https://huggingface.co/BAAI/bge-small-en-v1.5/resolve/5c38ec7c405ec4b44b94cc5a9bb96e735b38267a/<path>`; `File.Name` is the local name, flat, as in every model folder:

| `File.Name` | Path in the repository | Size | SHA-256 |
|---|---|---|---|
| `model.safetensors` | `model.safetensors` | 133,466,304 | `3c9f31665447c8911517620762200d2245a2518d6e7208acc78cd9db317e21ad` |
| `tokenizer.json` | `tokenizer.json` | 711,396 | `d241a60d5e8f04cc1b2b3e9ef7a4921b27bf526d9f6050ab90f9267a1f9e5c66` |
| `config.json` | `config.json` | 743 | `094f8e891b932f2000c92cfc663bac4c62069f5d8af5b5278c4306aef3084750` |
| `sentence_bert_config.json` | `sentence_bert_config.json` | 52 | `84e39fda68ccbff05bfa723ae9c0e70e23e2ec373b76e0f8c6e71af72a693cbf` |
| `pooling.json` | `1_Pooling/config.json` | 190 | `d1caf60c96f5fba2157c0c26b76d80818fad6cf0b8eb5e73ec372ff9818eba5c` |

aikit reads the safetensors weights, not the ONNX file the hugot row named, so this row's fingerprint is new. Nothing has shipped with the other.

**What our aikit adds.** The original's `encoder.LoadBERT(dir)` reads four files with `os.ReadFile` and maps the weights by path. Commit `04b54e26`, merged as `eb8c5ddd`, adds two functions and changes no behaviour of the existing loader:

```go
package encoder // github.com/pkieltyka/aikit/encoder

// LoadBERTFromFS is LoadBERT reading every file through fsys, rooted at dir: config.json,
// model.safetensors, sentence_bert_config.json, tokenizer.json and 1_Pooling/config.json,
// each once. The weights may alias the bytes that fs.ReadFile returns for model.safetensors.
func LoadBERTFromFS(fsys fs.FS, dir string) (*BERT, error)

// Tokenizer returns the tokenizer the model loaded, or nil when it could not parse one.
func (b *BERT) Tokenizer() *embed.Tokenizer
```

The adapter also uses three functions aikit has today: `(*embed.Tokenizer).Encode(text) []int32`, which gives a text's token ids with no wrapping tokens; `(*embed.Tokenizer).SpecialID(literal) (int32, bool)`; and `(*BERT).Embed(ids []int32) []float32`, which runs the model on ids that are already wrapped, pools as the pooling file declares and normalises.

**Opening from memory.** The opener:

1. Requires that the `Artifact` holds exactly the row's five files, in the row's order, none of them empty, and fails otherwise, before anything is loaded. It takes the five slices out and sets `a.Files` to nil.
2. Puts them in a `handoff`: an unexported type that implements `fs.FS` and `fs.ReadFileFS` over a fixed table of the five paths aikit asks for (`config.json`, `model.safetensors`, `sentence_bert_config.json`, `tokenizer.json`, and `1_Pooling/config.json` for `pooling.json`). It can be built only with all five. The first `ReadFile` of a path returns the slice itself, with no copy, forgets it and counts the read. Every other request is refused with `fs.ErrNotExist` and counted as refused: a second read of a path, any other name, and any call of its own `Open` method.
3. Calls `encoder.LoadBERTFromFS(handoff, ".")`.
4. Requires that each of the five paths was read exactly once and that nothing was refused, that `Tokenizer()` is not nil, and that `SpecialID` knows `[CLS]` and `[SEP]`; otherwise it fails and the model is dropped.

Steps 1 and 4 together show that the pooling file was there and was read. That matters because aikit does not complain without it: it pools by the mean, and every other check would still pass. An emptied `handoff` alone would not show it, since one built without the pooling file also ends empty.

The handover is what keeps the checked bytes private. Go allows a caller of `ReadFile` to modify what it gets, and a copy would be the cautious answer, at the price of a second 133 MB. Instead each slice is given away once: after step 3 the weights' bytes are referenced only from inside aikit, which uses them in place and does not write them, the four small files have been parsed and released, and pbrain holds no reference through which anything could write. This is the loader's own condition seen from the other side: its contract says a caller that hands out its own buffer must not modify it while the model is in use, and the adapter neither modifies nor reuses a buffer while the model lives, because it no longer has one. That is why a search peaks at 0.14 GB. The benchmark records the peak; a rise towards twice the model's size would show a copy.

**`Embed`.** The adapter does its own cutting and wrapping, because aikit's `Encode` path cuts from the end of the whole text and, like the reference tokenizer, reads literal special-token text as the real token.

1. **Special tokens.** In each text, every literal `[PAD]`, `[UNK]`, `[CLS]`, `[SEP]` and `[MASK]` gets a space after its first character (`[SEP]` becomes `[ SEP]`). The tokenizer then gives the ordinary pieces, the same as for the text with no special meaning.
2. **A query** gets the instruction `Represent this sentence for searching relevant passages: ` in front, is tokenised with `Tokenizer().Encode`, and is cut from its end to 510 tokens.
3. **A document** is split at its first blank line into prefix and passage; with no blank line the whole text is the passage. Each part is tokenised on its own, which gives the same tokens as the whole text, since no token crosses white space. If together they pass 510 tokens, the prefix is cut from its end to `max(255, 510 - passage tokens)`, then the passage is cut from its end to what remains. The passage therefore always has at least 255 tokens of room.
4. The tokens are wrapped with the ids of `[CLS]` and `[SEP]`, and `Embed(ids)` gives the vector: the first token's state, L2-normalised. aikit has no batch call for this model, so the texts of one call are embedded one after another, each in one forward pass; a text's vector does not depend on what else the call holds. The adapter never passes more than 512 ids, so aikit's own silent cut at that length is never reached.
5. The context is checked before each text; a forward pass in progress is not interrupted.

`Close` calls the model's `Close` and drops it. aikit's kernels take their parallelism from the number of processors Go is allowed; the adapter does not change it.

**Measured in the check** (one Intel laptop, 4 threads, single runs): token ids equal to the reference on 22 texts and a worst difference from its vectors of 1.7e-13 in cosine; check, load and first query 124 to 153 ms, of which the check of the five files is 104 to 110 ms and the load 8 to 12 ms; a warm query 16 to 19 ms; a search peaks at 0.14 GB, the 134 MB of model bytes included; indexing runs at about 2,800 tokens a second and peaks under 0.2 GB, so a first index of 5,000 items takes about 14 minutes. The longest-prefix test of "What a cut passage is evidence for" passes with the same numbers as on hugot (0.9863 for a changed first word, 1.0 for a changed word in the cut tail), and across 30 windows of our corpus the two runtimes' vectors agree to 2.5e-10. About 1 MB of binary. Not measured: any arm64 or macOS run, a cold file cache, and batching.

**The fallback** is the hugot adapter of the earlier draft (`bge-small-hugot-adapter-design.md`). It is faster to index (about 10 minutes) and worse on every other count: 440 to 485 ms to a first query, 0.60 GB, 18 to 24 MB of binary, and three workarounds. It is not built.

### The EmbeddingGemma 2 adapter

Package `internal/embed/gemma2` is the only package of pbrain that imports goinfer. It is designed from `goinfer-embeddinggemma2.md` and `goinfer-architecture-review.md`; the parts that are not built yet are marked.

**The library** is the goinfer column of "Our libraries" above. It was first measured at the original's release v0.24.1.

**The model row** (`gemma2.Entry()`).

| Field | Value |
|---|---|
| `Name` | `embeddinggemma-2` |
| `Runtime` | `goinfer` |
| `Dim` | 768 |
| `Cutoff` | set by the calibration tool; the row is in no release until the rules of architecture §8.7 and both gates pass |
| Preparation version | `gemma2-1`: the prompts, pooling, cut rule and special-token rule below |

`Source`, from `https://huggingface.co/google/embeddinggemma-2/resolve/f9972ac2…/<path>` (the full revision id is pinned when the row is built):

| File | Size | SHA-256 |
|---|---|---|
| `model.safetensors` | 1,488,915,288 | `197a32965d4b1105faf060417baa899e193fb73cd401f42ec9295234d5553d79` |
| `tokenizer.json` | 32,170,510 | `4d777ef5bdc1aa36227abdfb77c3e49e7b9c892d16e1b6bda41c393504828be4` |
| `config.json` | 4,455 | `b8f1e9931b57fbc054acdb445c41765d55b0074c58d145fa82839941ad1b5bb3` |
| `config_sentence_transformers.json` | 1,565 | `031e56a498d33c349ab489a21885bcfe25b4fcba841149dc99e1e90d4a7c28f5` |

`Files`, the pack that `Derive` writes and every open verifies. Sizes of the two weight files follow from the checkpoint; the tokenizer's digest is known, and the other three are fixed when the derive procedure is written:

| `File.Name` | Holds | Size |
|---|---|---|
| `tokens.bf16` | the token-embedding table, 262,144 × 512 values, 16 bits each, as stored | 268,435,456 |
| `layers.safetensors` | the other 412 tensors of `language_model.*`, names and bytes unchanged, sorted by name, header padded to 8 bytes | about 273.6 MB |
| `tokenizer.bin` | the tokenizer's pack, from our goinfer's `tokenizer.BuildPack`; SHA-256 `45a7e7d5a1e84ad83ef469a8f6aecc3dd15f51b7c4866008e2d770142d96b709` for this source | 9,581,663 |
| `config.json` | the source `config.json` and the two prompts used, as one small JSON | under 8 KB |

**The three changes to goinfer, in the order they are done.**

1. **A fast tokenizer: done** (merged into our `main` at `2e413ee1`; `goinfer-tokenizer-spike.md`). `tokenizer.BuildPack(tokenizerJSON []byte) ([]byte, error)` writes a binary form of the tokenizer, always the same bytes for the same input, in under a second; `tokenizer.LoadPack(pack []byte) (*Tokenizer, error)` loads it in 24 to 26 ms at four threads, against 441 to 502 ms for the JSON, opening no file and keeping about 17 MB. Token ids are identical on goinfer's 48 reference texts, on 10,000 random encodings in both special-token modes, and on 3,052 texts against the Hugging Face tokenizer. It serves the Gemma tokenizer family only and refuses others. The fallbacks planned for this task are not needed.
2. **A loader from bytes: done** (goinfer v0.2.0, `a759890b`). `embeddinggemma2.NewModel(cfg Config, tokens, layers []byte) (*Model, error)` builds the model from its configuration, the token table's bytes as stored and an in-memory safetensors file of the other tensors. It keeps the table as it is and widens one row per token, widens the other tensors once, across the processors, without the second copy the loader by path makes, and does not keep the layers bytes. `embeddinggemma2.LoadConfigBytes(raw)` reads the configuration from memory. `embeddinggemma2.NewEncoder(m, tok, prompts)` builds the encoder from an already built model, a tokenizer and the prompt table, because `LoadEncoder` reads all three from a folder. Measured on the real checkpoint at four threads: weights ready in 41 ms against 693 ms for the loader by path on the same warm machine; the 48 reference texts give the reference's ids and a worst cosine of 0.999999989, and 28 of them were compared with the loader by path, bit for bit equal.
3. **The pack.** `gemma2.Entry().Derive` is the model's procedure, pure and deterministic: from the four verified source files it returns the token table's bytes (`tokens.bf16`), `layers.safetensors` as specified in the table above, `tokenizer.bin` from our goinfer's writer, and `config.json`. `embed.Pack` checks those bytes against the pinned digests before it writes anything, so a derive that goes wrong leaves no pack.

**Opening and `Embed`.**

1. `Open` builds the tokenizer from `tokenizer.bin` with `LoadPack` and the model from the two weight files, side by side, then the encoder from both and the prompts; nothing is read from disk.
2. **Special tokens.** Body text must never produce a special token. The adapter encodes text with `EncodeSegments` and special-token parsing off, which a pack-loaded tokenizer serves as the JSON-loaded one does, and adds `<bos>` and `<eos>` itself. The published `Tokenize` reads a literal `<eos>` as the real token, as the reference does; the adapter does not use it.
3. **Prompts.** A query gets `task: search result | query: ` in front, a document `title: none | text: `, as the checkpoint defines them. The item's title and section titles stay inside the text, as pbrain lays them out; whether the title should go into the prompt's title slot is a question for the fixture, and changing it changes the preparation version.
4. **The cut.** The model takes 8,192 tokens and goinfer refuses more. The adapter never sends more than 2,048 including the prompt and the two special tokens, which bounds time and memory: a 2,294-token text took 5.8 s. A document is split at its first blank line into prefix and passage and cut by the same rule as the other adapter: the prefix only as far as needed and never below half the room, then the passage from its end.
5. Texts of one call are embedded one after another; goinfer runs its matmuls across the processors by itself. The vector is the mean over every token of the projected last hidden state, L2-normalised, 768 values.

**Costs to state wherever this model is offered, the more so since it is the default (owner's decision, 2026-11-21):** a first index of about 95 minutes for 5,000 items at four threads (about 440 tokens a second), about 1 GB of memory at a search by estimate, a one-time download of 1.49 GB for a 542 MB pack, and 9 right of 20 on our reworded questions against 13 for bge-small.

**What a cut passage is evidence for.** Counting in code points cannot promise that a passage fits a model: dense or non-Latin text can take more tokens than the limit. The adapter then embeds the prefix and the beginning of the passage, and the tail is not represented in the vector. The hit still cites the whole span, and its snippet is the start of the passage, which is the part that was scored. This is a stated limit, not an error. The spike must show, for each model in the table, that with the longest permitted prefix in the densest script of its test set a word placed at the start of the passage still changes the vector.

## Dependencies

- `embed` uses `sys` and `errs`; `*fetch.Client` reaches it as a `Downloader`, passed by `cli`. `semantic` also uses `modernc.org/sqlite` and its `vec` package, in memory only. `embed/bge` imports `github.com/pkieltyka/aikit/encoder` and `github.com/pkieltyka/aikit/embed`; `embed/gemma2` imports the text encoder and tokenizer packages of `github.com/pkieltyka/goinfer`. `semantic` uses `embed`, `outline`, `item`, `search` (for the types of its seam), `gitx` (the cache test), `sys` and `errs`.
- Used by `cli` only. `search` sees `semantic.Source` through its own `Semantic` interface.
- Needs from `sys.Files`: `Resolve`, `OpenRoot`, `Entries`, `OpenRegular`, `Flush` and `Lock`; and a `Lock` that waits as long as its context allows, without the 30-second limit of the section lock.
- Needs from `brain`: `OpenSection`, with `Sub(".pbrain", create)` and `Sub("index", create)` on the handle it returns.

## Test plan

Every test uses the test registry of `internal/embed/embedtest`: a model `test-model` of 8 dimensions with cut-off 0.5, and `test-model-b` with another fingerprint. Its embedder builds a vector from a keyword table in the fixture: the normalised sum of the vectors of the keywords a text contains, or a fixed unit vector when it contains none. It counts calls and texts.

| Test | Assertions and spec scenarios |
|---|---|
| `TestPassagesSections` | One passage per section with text; a heading with nothing under it gives none; the ancestor line; a level-0 passage; an empty body gives the title passage. Spans tile and never overlap. Scenario 33. |
| `TestPassagesSplit` | A 5,000-code-point section splits at paragraph breaks near 1,400; a fenced block stays whole; one 4,000-code-point line splits at spaces, and without spaces at 1,800; multibyte text keeps code-point offsets. |
| `TestPassagesPrefixCap` | A 600-code-point title and ten nested headings leave a prefix of at most 400 code points plus separators, the innermost heading kept; a short distinct body passage under them is still in `Text`. |
| `TestPassagesStable` | The same title and body give the same digests on every run; editing one paragraph changes only the digests of parts that hold it. Scenario 37. |
| `TestIndexRoundTrip` | A database published and opened again holds the same items, spans, digests and vectors; the manifest names its size and SHA-256. |
| `TestIndexRejects` | Each is `IndexUnreadable` and reaches SQLite never or harmlessly: a database file with one changed byte, a truncated or longer file, a manifest that is not JSON or is longer than 4,096 bytes, a wrong `format`, `meta` rows that contradict the manifest, and a database whose digest matches the manifest but which lacks the `passage` table. Another fingerprint is `IndexStale`. A vector of the wrong length cannot be inserted. Scenario 35. |
| `TestManifestIsNotTrusted` | A manifest whose `db` is `lock`, `manifest`, a name with a path separator, upper-case hexadecimal or the wrong length; one whose `size` is 0, negative, 2^31, 2^63 or not a number; one whose `sha256` is malformed; one whose `size` differs from the file's own: nothing is opened under that name, nothing is allocated for that size, and nothing is removed. A build that repairs such an index still holds the same lock file afterwards: a second process cannot take it meanwhile. A database that would pass 2 GiB fails with `too_large` and the old index stays. |
| `TestIndexTouchesNoFile` | A search and a build run in a child process whose working folder is empty and read-only and whose temporary folder does not exist: both succeed, and `index/` holds only `lock`, `manifest` and one database afterwards. No journal, write-ahead log or shared-memory file appears at any moment of a build that is paused between its steps. |
| `TestScoringQuery` | Against scores computed in Go from the same vectors: the query returns the passages of the listed items within `1 - Cutoff + 0.000001`, also when more than 4,096 qualify, and `search`'s exact comparison then drops the one that lies in that margin; equal distances keep the smaller `ord`. An item not in `eligible` is never scored: with a malformed vector written to such an item, the query still succeeds and returns the others. Scenarios 33 and 34. |
| `TestDamagedPassageIsRepaired` | A published database, with a matching digest, in which one passage of an unchanged item holds a vector of the wrong length. Four separate commands: `search --semantic` returns the lexical results and the other section's hits with `semantic_partial`; `index --status` reports the section `unreadable`; `pbrain index`, with no item changed, builds the index again; the same search then returns its results with no notice. Scenario 35. |
| `TestSourceSectionFailure` | Two sections, one of which fails while its rows are read (injected), and one whose scoring statement cannot be prepared: the failing section contributes no hit, all its items are in `Missing` once, the other section's hits are returned, and the rendered command shows them with `semantic_partial`. With both failing: `NoIndex` and the lexical results. Scenario 35. |
| `TestCoverage` | An entry covers an item only with the same path and content digest; an edited body, an edited title, a renamed file and a deleted file are not covered; a front-matter-only edit still is. A body changed to other text of the same length and the same CRC-32C is not covered, is never scored and is counted in `Missing`. A row whose span passes the body's end is not used. Scenario 35. |
| `TestBuildFirstAndNoOp` | A first build embeds every passage; a second embeds none and never opens the inference runtime, though it verifies the model's files. Scenarios 35 and 37. |
| `TestBuildRestoresModel` | With a complete index and the cache deleted, a build with no item changed downloads and verifies the model, embeds nothing, opens no runtime, and the next search with the source is ready. With a complete index and wrong weights of the right size at `model_path`, the same build fails with `invalid_config` and leaves the index as it was. An empty section asks for no model. Scenario 35. |
| `TestOutputAndNotices` | Every row of the notice table, in text and JSON, in both views; singular and plural forms; the status line for each model state and each `IndexState`. No message holds an item id, a path or an address. Scenarios 35 and 37. |
| `TestBuildIncremental` | After one added note: one passage embedded. After one edited paragraph of a long source: one embedded, the rest reused from the index. After a deleted item: `ItemsRemoved` 1 and its rows gone from the published database. An item is never half present: a build stopped between two transactions holds either all of an item's old passages or all of its new ones. A replacement whose insert fails part-way (a duplicate key forced by the test) is rolled back: the old item and all its old passages are still there, and nothing is published. Scenario 37. |
| `TestBuildRename` | A personal item moved to a new path with its text unchanged: the next build embeds nothing, opens no runtime and does not touch the per-user cache, and the published database has the new path and not the old. |
| `TestModelProvisionShared` | Two processes building different sections of different projects against one empty cache, one paused after it has opened the source files: both finish with the same verified pack, the source is downloaded once, and neither fails on files the other removed. A cancelled wait for the model lock ends with the context's error. The model lock is never taken before the index lock. |
| `TestBuildFromCache` | A second work tree of the same shared files embeds nothing and never opens the inference runtime. Personal passages are embedded and no cache file appears for them. Scenarios 33 and 37. |
| `TestBuildInterrupted` | A build cancelled after its first publication leaves a usable index; the next run embeds only what was not published, and for the shared section nothing that reached the cache. A process killed in the middle of one long personal source leaves that item out of the index, and the next run embeds it again from its start. Leftover temporary files and databases the manifest does not name are removed. Scenario 35. |
| `TestBuildFingerprintChange` | With `test-model-b` named, the old database is removed, every passage is embedded again, and no old vector is scored. Scenario 37. |
| `TestBuildRepairsDamage` | A damaged database file and an unparseable manifest each give `IndexUnreadable` in `Status`, and a build restores a usable index without touching an item. Scenario 35. |
| `TestBuildConcurrent` | Two processes building one section: the second waits, reports `Waiting` and then finds nothing to do. Staggered: process A scans, an item is added, process B builds and indexes it, then A gets the lock; A scans again under the lock, keeps B's entry and reports nothing removed. A search during a build that publishes reads one whole database, the old or the new. |
| `TestBuildEmptySection` | A section with no item and no index: zero result, no `.pbrain/` created, also for a personal folder that does not exist. A section whose last item was deleted: a database with no item. A section with no item whose index is unreadable or built for another model: the build publishes an empty database and the status reports it as in order. `Status` creates nothing in either. |
| `TestBuildErrors` | Semantic search off (an empty `ModelName`): `no_model`. Unknown name: `invalid_config` listing the names. A changed body is skipped. A vector of the wrong length fails with `io` and leaves the old index. Scenario 35. |
| `TestModelIdentity` | Wrong weights of the right size, at `model_path` and in the cache, and a missing companion file of a two-file model: `Build` fails (`invalid_config`, or a new download for the cache) and publishes no database; a search returns its lexical results with not ready and never calls `Registry.Open`. A file replaced by name after `Verify` returned, and a file overwritten in place through the same open file after `Verify` returned, change nothing: the test registry hashes the bytes it is given and they are the pinned ones. Two models with one name and different files use two folders. A valid offline two-file folder builds with no request. Scenario 35. |
| `TestContentDigestFraming` | A title holding a zero byte, and a body starting with one, never collide with another title and body pair; moving bytes between the end of the title and the start of the body changes the digest. |
| `TestModelFindVerifyDownload` | Files found at `model_path` and in the cache; a wrong size or digest at `model_path` is `invalid_config`; a download with a wrong digest, a short body or a body past its size fails with `fetch_failed` and leaves no file; a second process finds the first one's file. No request is made when `model_path` is set. Scenario 35. |
| `TestCachePlacement` | Accepted: a folder outside any work tree; an ignored folder inside one, also when the home folder is a work tree. Refused: inside the brain, inside the personal section, in a work tree that does not ignore it, an ignored folder with a force-added file, a missing child of each, and a link to each. Refused for a nested repository: a cache outside Git whose `vectors` folder, fingerprint folder or one two-character folder holds `.git`, as a folder and as a file, and the same inside an ignored cache within a larger work tree; the build stops before writing into that folder. Nothing is created on refusal, and a search without `--semantic` is unaffected in every case. Scenario 35. |
| `TestCacheFiles` | A short file, a wrong checksum and a vector that is not of unit length are removed and counted absent; two writers of one vector leave one valid file. |
| `TestSourceLifecycle` | `Start` returns before the model is open and the scan can run meanwhile; `Close` without `Attach`, after a failed scan and after `Score` each cancels the task and closes the embedder once and only if it was opened; a cancelled context ends `Score` with the context's error. |
| `TestSourceBadQueryVector` | A runtime that returns a query vector of the wrong dimension, with a value that is not a number, or not of unit length: not ready with `ModelFailed`, no hit, lexical results kept. Scenario 35. |
| `TestSourceNotReady` | Each row of the check table gives its reason and opens no model; no file is written and no request made in any of them. Scenario 35. |
| `TestSourceScores` | The best passage per item, its span, ties to the earlier passage; only eligible covered items are scored; the query is embedded once; results are identical across runs. Scenarios 33 and 34. |
| `TestSourcePartial` | A usable shared index and no personal one: shared items scored, `Missing` equals the personal items; in the shared view `Missing` counts shared items only. An uncovered edited item is never scored. Scenario 35. |
| `TestStatus` | Counts per section and each `IndexState`; no model opened; correct with semantic search off. Scenarios 35 and 37. |
| `TestBgeParity` | With the model files present: for the 22 reference texts kept in `testdata` (documents and queries, code, German, Japanese, over-long texts), token ids equal the stored reference ids and each vector has cosine similarity of at least 0.999 with the stored reference vector; a call with one text and a call with eight give the same vectors. |
| `TestBgeBuildsFromBytes` | The model folder is deleted after `Verify` and before `Open`: the embedder opens and embeds, and `a.Files` is nil afterwards, also when `Open` fails. An `Artifact` with four files (the pooling file missing is one case), with six, or with an empty one fails before the loader is called. With the load step replaced by a fake, through a package variable kept for this test: a loader that skips one of the five paths (the pooling file is one case), one that reads a path twice, one that asks for another name and one that calls the `Open` method each make the opener fail. Guards the loader against a new pin of aikit. |
| `TestBgeCutRule` | A 600-token query is cut to 510 from its end. A 400-token prefix with a 300-token passage: the prefix is cut to 255 and the passage to 255. A 400-token prefix with a 50-token passage: the prefix keeps 400 and nothing is cut. Changing the passage's first word changes the vector; changing a word in its cut tail does not. |
| `TestBgeSpecialTokens` | `a [SEP] b [CLS] c` gives the ids of the plain pieces, never ids 101 or 102 inside the text; a text with and without the inserted space gives the same ids. |
| `TestRuntimeQuiet` | For each real model, in a child process whose standard output and standard error are pipes the test reads: opening the model and embedding print nothing on either. |
| `TestStrayOutputDiscarded` | In a child process built with `cmd/pbrain`'s start-up code: after it has run, `fmt.Printf` and a write to the `os.Stderr` variable put no byte on either real pipe, while writes through `Env.Stdout` and `Env.Stderr` arrive whole. This covers a library's failure-path print without having to cause the failure. |
| `TestRuntimeEnvAndOutput` | A source check on the pinned commits. For every Go file, tests excluded, of the aikit and goinfer packages that `cmd/pbrain` links, listed with `go list -deps` once for each of the four release targets, so that a file a build constraint leaves out on the test's own machine is read too: no call of `os.Getenv`, `os.LookupEnv`, `os.Environ` or `syscall.Getenv`, and no use of `os.Stdout`, `os.Stderr`, `fmt.Print`, `fmt.Printf`, `fmt.Println`, `print`, `println` or the default logger of `log`, outside a short list of allowed places kept in the test with a reason for each: one list for all targets, keyed by file and symbol. Each target must select files of both modules. It fails when a new pin adds one. The aikit packages have none today. |
| `TestReleaseBinaryOutput` | The built release binary of every target, started as a fresh process: `--version`, a lexical `search`, `search --semantic` and `index` each leave exactly the command's output on the process's real standard output pipe. The `index` case embeds several passages and the search case runs after it. The calibration tool is run the same way. |
| `TestGemmaParity` | With the model present, on the pinned commit of our goinfer, two separate comparisons. (a) goinfer's own encoding path, with no cap and special-token text read as the reference reads it, against the stored raw references of all 22 texts, the literal `<eos>` document and the 2,294-token document included: identical token ids and cosine similarity of at least 0.999, also at 512 dimensions. This is rule 2 and it is never weakened. (b) The adapter, which treats marker text as ordinary text and stops at 2,048 tokens, against references made by sentence-transformers from input prepared and trimmed in exactly the same way: the same bounds. |
| `TestGemmaTokenizerBinary` | In pbrain, on the pinned commit: `BuildPack` of the pinned `tokenizer.json` gives the pinned digest; the pack-loaded tokenizer gives the stored reference ids of the 22 texts. Our goinfer carries the wider checks (its unit tests, its fuzz target and its real-checkpoint test). Load time is recorded by the benchmark, with 100 ms as the release threshold; the spike measured 24 to 26 ms. |
| `TestGemmaBuildsFromBytes` | The pack folder is deleted after `Verify` and before `Open`: the embedder opens and embeds. Vectors equal those of the published loader on the same checkpoint. |
| `TestGemmaSpecialTokens` | A body holding `<eos>`, `<bos>` and `<start_of_turn>` yields no special id between the first and the last token; the same body without them differs only by those pieces. |
| `TestGemmaCut` | No call passes 2,048 tokens; a long prefix with a short passage keeps the passage whole; a long passage is cut from its end; a 9,000-token text is cut, not refused. |
| `TestDeriveDeterministic` | Deriving twice from the same source gives the same bytes, which match the pinned digests; a source with one changed byte, and a result with one changed byte, are refused and nothing is written; the source files are removed only after the pack verifies, and the empty folder may remain; an interrupted run does not download the source twice; source files placed at `model_path` are not accepted and nothing is written there. |
| `TestCalibrate` | On a fixture with known scores: the cut-off from the calibration half only; the abstention gate fails when a held-out page shows a lexical result, and when a regression case shows any; the gain gate fails when a tighter page drops an expected id; a fixture without recorded halves or with too few questions is refused; the report has one row per arm of the wider comparison, and the two any-word arms score the same candidates; the partial tier's gates fail on a near-miss question that returns partial rows; maxima below a provisional cut-off are still collected; an exact match beyond the first page is excluded from the maxima; with 38 of 40 empty pages for each mode alone and 36 for both, the combined gate fails and the release check with it; a long-source question with no `evidence`, with two expected ids, or with a quotation found twice in its item, is refused; with one such question whose evidence is on the page, one that needs the read, one whose item is first and never shows it and one whose item is not retrieved, the report counts each once; with the expected item fifth on the page and the quotation not in its snippet, the reader opens the first result, the question is a miss of the second kind, and only the upper bound counts it; with a long body after the quotation, the size to the evidence is smaller than the size delivered by exactly that tail; in the guided-retry arm both searches are charged; at a budget where the text form of the page would hold a fifth result that the JSON form does not, an expected item in fifth place is not retrieved, and where the JSON body of the read ends before a quotation that the text form would have shown, the evidence is not visible, and every size is that of the JSON responses; no miss enters a size. Scenario 34. |
| `TestAdapterContract` | For each real model, where its files are present: reference vectors reproduced with cosine similarity of at least 0.999; an over-long text is cut, not refused; with the longest permitted prefix, a changed word at the start of the passage changes the vector; batch size does not change a vector; fingerprint and dimension match the table. Skipped with a message otherwise. |
| `TestPassageAlignment` | On the alignment item of architecture §14 (two sections with one title under different parents, a fenced line that looks like a heading, multibyte text): every span from `Passages` lies inside one section of `outline.Parse` and none starts at the fenced line; the two sections of the same title give different passages with different heading paths in their prefixes; for a hit on each, `SemanticHit.Start` is in the intended section, told apart by its place in the outline, and the body from `Start` begins with the passage text that was embedded. |
