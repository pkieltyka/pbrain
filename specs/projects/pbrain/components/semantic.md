---
status: complete
---

# Component: Semantic search (`embed` and `semantic`)

## Purpose and scope

These two packages give `search --semantic` its second source of results and give `pbrain sync` its work. `embed` owns the `Embedder` seam, the table of supported models, and finding, downloading and verifying a model. `semantic` cuts items into passages, keeps one vector index per section and one vector cache per user, builds the index incrementally, reports its state, and scores eligible items for a search. [Architecture §§7.4, 8.5 and 8.7](../architecture.md) · [Functional spec §§7.1, 9.3, 9.4 and scenarios 33 to 37](../functional_spec.md)

Out of scope:

- The cut-off test, the order of added results, snippets and notices: `search` and `cli` (architecture §8.5).
- Headings and sections: `outline` (architecture §7.4).
- The scan, section handles, locks and file primitives: `store`, `brain`, `sys`.
- Models other than the EmbeddingGemma 2 rows. bge-small-en-v1.5, one of the two models the owner chose on 2026-11-21 and the default until v0.5.0, was removed on 2026-11-26; its adapter, its BERT port and its hugot fallback are no longer designed here. Their record is in `research/semantic-search-and-index/embedding-models-and-go-runtime/` and in architecture §8.7.
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
    // Info says what the embedder runs on. It can change between two Embed calls, when an
    // accelerator that ran out of memory gives way to the CPU, and never during one.
    Info() RunInfo
    // Close releases the model. An error says only that the release failed: the vectors already
    // returned stand, and the callers in semantic drop it ("The source" and "Build").
    Close() error
}

// Windowed is optional: an embedder that works best with many texts in one Embed call says how
// many. The build asks for it by type assertion and uses 8 for an embedder without it. The
// number is fixed for the life of the embedder and does not depend on the machine.
type Windowed interface {
    Window() int
}

// OpenOptions says what the caller wants the model to run on. The zero value is the CPU.
type OpenOptions struct {
    // Device is a selector ("Devices"): "" or "cpu"; "auto"; a backend, "cuda" or "vulkan"; or one
    // device: "cuda:0", "vulkan:0", or the backend and the device's persistent id, "vulkan:<uuid>".
    Device string
    // MemoryBudget, when positive, replaces the 30% of the memory rule: bytes of device memory
    // that a batch of several texts may take. Ignored on the CPU.
    MemoryBudget int64
}

// Fallback says why a model that has a GPU path runs on the CPU.
type Fallback string

const (
    NoFallback        Fallback = ""
    FallbackAsked     Fallback = "asked"       // the selector was "cpu"
    FallbackNoSupport Fallback = "unsupported" // this binary has no accelerator for this model
    FallbackNoDevice  Fallback = "absent"      // no driver or no usable device on this machine
    FallbackFailed    Fallback = "failed"      // a device is there and could not be attached
    FallbackMemory    Fallback = "memory"      // the device ran out of memory, at the upload or for one text alone
)

type RunInfo struct {
    Device   string   // "cpu", or the device's id: "cuda:0", "vulkan:0"
    Name     string   // the device's own name when the runtime gives one, else ""
    Budget   int64    // bytes a batch of several texts may take on the device; 0 on the CPU
    Fallback Fallback // empty on a GPU, and for a model with no GPU path
}

// DeviceInfo is one line of the listing.
type DeviceInfo struct {
    ID, Backend, Name     string // "cuda:0", "cuda", "NVIDIA RTX PRO 6000 …"; ID "cpu" for the CPU
    Persistent            string // the selector that names this device for good: its backend, ":" and the runtime's lasting id; "" when the runtime gives none
    TotalBytes, FreeBytes int64  // 0 when unknown
    Memory                string // "live", "estimated" or "unknown": how FreeBytes was obtained
    MemoryKind            string // "dedicated", "shared" (the host's, an integrated GPU) or "unknown"
}

// Listing is what an adapter's backends say they have, asked without a model.
type Listing struct {
    Devices []DeviceInfo
    // Failed holds, by backend name, the error of a backend that could not be asked. A backend
    // that answered, with devices or with none, is not in it; nor is one this machine has no
    // runtime for ("Devices"). Its devices, if it has any, are missing from Devices.
    Failed map[string]error
}

// ErrAccelerator marks a failure of an accelerator that the adapter does not recover from.
var ErrAccelerator = errors.New("accelerator failed")

type File struct {
    Name   string   // file name inside the model folder
    URL    string   // https, pinned in the release
    SHA256 [32]byte
    VeritySHA256 [32]byte // optional release fs-verity file digest; zero means ordinary SHA-256 only
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

// Artifact holds private model bytes authenticated by Verify or VerifyManaged.
// Neither the registry nor the adapter reopens a file.
type Artifact struct {
    Model Model
    Files [][]byte // each file's bytes, in the order of Model.Files
}

// Entry joins one model row with the functions of its adapter.
type Entry struct {
    Model Model
    Open  func(ctx context.Context, a *Artifact, o OpenOptions) (Embedder, error)
    // Devices lists the GPUs this adapter could attach on this machine; nil for a model that
    // runs on the CPU only. It opens no model.
    Devices func() Listing
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
    // A selector that names nothing Devices lists is an *errs.Error with code invalid_config, unless
    // its backend is in the entry's Listing.Failed: then the error wraps ErrAccelerator and that
    // backend's error. For a model with no GPU path the selector is not consulted. A listed device
    // that cannot be attached wraps ErrAccelerator. A selector that names a device reaches the
    // entry's Open as that device's Persistent when it has one, else as its ID.
    Open(ctx context.Context, a *Artifact, o OpenOptions) (Embedder, error)
    // Devices lists every device some entry can use, each once, GPUs in the order of the
    // automatic choice ("Devices"), the CPU last. It opens no model. A backend whose listing
    // failed contributes nothing and is not reported here: the entries' Listing.Failed is dropped.
    Devices() []DeviceInfo
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

// VerifyManaged is for files under the checked managed cache only.
type ManagedMode uint8
const (
    ManagedStrict ManagedMode = iota // full SHA-256; also reject a known sealed-pin mismatch
    ManagedSearch                     // may authenticate the same fd with its release verity pin
)
func VerifyManaged(files sys.Files, dir sys.Dir, m Model, mode ManagedMode) (*Artifact, error)

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
// frames the title, so no title and body can be mistaken for another pair. Only the build and a
// search's catch-up call it, and only for an item they are about to cut.
func ContentDigest(title string, body []byte) [32]byte

// Check is the cheap identity of the bytes ContentDigest hashes: their CRC-32C (Castagnoli,
// hash/crc32) and their count, 8 + len(title) + len(body).
type Check struct {
    CRC  uint32
    Size int64
}

// ContentCheck is what every command computes to ask whether an entry still holds an item's text.
func ContentCheck(title string, body []byte) Check

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
    Device    string   // the selector as cli validated it, from --device or PBRAIN_DEVICE; "" means "auto" in Build. A search ignores it
    GPUMemory int64    // PBRAIN_GPU_MEMORY in bytes; 0 means the rule
    MetalSingleton bool // PBRAIN_METAL_BATCH=0 keeps Metal on one passage per forward
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

// CatchUp embeds and saves the few items that the view's usable indexes do not cover, within
// the bounds of "Catch-up in a search". Call it once, after Attach and before Score. It returns
// only the context's error: whatever else stops it leaves those items uncovered, and Score
// counts them in Missing.
func (s *Source) CatchUp(ctx context.Context) error

// Score joins the background task and compares. text must be the text given to Start.
// It returns only the context's error.
func (s *Source) Score(ctx context.Context, text string, eligible []*item.Item) (search.Scores, error)

// Close cancels the background task, waits for it, closes each opened index and closes an
// opened embedder once. An error from the embedder's Close is dropped.
// It is safe to call when Attach or Score were never reached.
func (s *Source) Close()

type BuildResult struct {
    Section          item.Section
    Items            int // items the index holds after the run
    ItemsRemoved     int // items gone from the section whose entries this run dropped
    PassagesEmbedded int
    PassagesReused   int    // taken from the index's own entries, from the cache, or from an earlier passage of this run
    Device           string // what embedded this section's passages: "cpu", "cuda:0"; "" when the run embedded nothing
}

type Progress struct {
    Section     item.Section
    Done, Total int  // items to do
    Waiting     bool // waiting for the index lock, which a build or a search's catch-up holds
    // Run is set on the one report sent when the build opens the model, and again if the
    // embedder's Info changes; nil on every other report.
    Run *embed.RunInfo
}

// Build brings one section's index up to date. It calls scan after it holds the index lock and
// works from that scan alone. An item whose body cannot be read as scanned is left for the next run.
// Errors, all *errs.Error: no_model; invalid_config (unknown model, a model_path folder that is
// incomplete or fails Verify, an exposed cache, neither a cache folder nor a model_path, or a
// device selector that names nothing this machine lists); fetch_failed from the download; io
// for the file system, the lock, a named device that cannot be attached and a failed embedding.
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

A model's fingerprint is computed when the table is built, never at run time: SHA-256 over the model's name, each file's name and digest in table order, `Dim`, and the adapter's preparation version, a constant the adapter bumps when it changes a prompt, the pooling, the normalisation or the token limit. The verified artifact and CPU query encoder use that base fingerprint. Build checks the document encoder against the resolved writer fingerprint and the row dimension before accepting a vector. A disagreement is a runtime failure. The index fingerprint adds `ChunkerVersion`. A change of either starts a new index; a change of the chunker alone still finds every unchanged text in the cache, because the cache is keyed by the model fingerprint.

The legacy fp32 vector fingerprint is shared by qualified ordinary backends. Plain CUDA gives `embeddinggemma-2` the same fingerprint as the untagged build, so the two share an index and the vector cache, and one index can hold vectors made by both. That is safe because the two agree far inside rule 2. Over the 59 passages of the fixture, the CPU vector and the CUDA vector of one passage have a cosine of at least 0.99999999999854, with a median of 0.99999999999955 (issue #22), where rule 2 asks 0.999 of an adapter against its reference. A backend whose vectors fell outside 0.999 of the CPU's would take a fingerprint of its own.

So a backend makes a model's vectors only after two things are shown for that model on that backend: its vectors pass rule 2 against the CPU's on the 59 passages, and a text's vector is the same bytes alone and in any batch ("Sameness", under "Devices"). The Google `embeddinggemma-2` pack qualifies on CUDA and on Vulkan with goinfer v0.6.2 (issue #49). On the RX 9060 XT, the saved 59-passage batch reached minimum CPU cosine 0.999999999998413. All 59 Vulkan vectors also matched their singleton bytes in the original and reversed batches and at the 1 MiB override. Each additional model row qualifies separately. The GGUF rows of EmbeddingGemma 2 are models of their own: each has its own name and file, so its own fingerprint, index and cache entries, and shares a vector with no other row ("The GGUF rows").

### CUDA indexing precision (#61)

For Google EmbeddingGemma 2 and Q8_0, the project may request `semantic.cuda_precision = "fp16"` or `"fp32"`. An omitted value takes the release default even in a CPU-only binary. The default is fp16 starting in v0.4.0. Both rows passed cosine, layout invariance and CPU-query retrieval qualification on the RTX PRO 6000. The setting selects a writer for CUDA document vectors. Queries always use the CPU. Model folder, file verification, dimensions and cut-off belong to the model row and are shared by its two precisions.

The fp32 profile keeps the existing vector fingerprint. The fp16 profile hashes the literal prefix `pbrain-vector-profile-v1`, the row’s lowercase base fingerprint, the selector `cuda-eg2-fp16` and the full qualified goinfer commit, in that order, as ASCII lines joined by LF without a final LF. Encode the SHA-256 digest as lowercase hex. Use the effective vector fingerprint for the shared vector cache and as input to the existing chunker/index fingerprint. Resolve it before opening an existing database or looking up a vector. A precision change in either direction replaces the generation under the index lock. All lookup tiers and checkpoints use one effective profile. Personal vectors remain private. The physical card and its ordinal do not enter the identity.

In a CUDA build, a nonempty sync of an eligible Gemma row requesting fp16 may perform one model-free inventory for the command, shared between sections. This applies even when every source item is unchanged. Since goinfer v0.6.6, pbrain selects fp16 when the selected CUDA row reports `cuda-eg2-fp16` in `RunnableSelectors()`. The same row supplies compute capability for the fallback explanation. `DeviceSelectors("cuda")` distinguishes a missing fp16 build from a card that does not report the selector as runnable. Older or unqualified cards use the ordinary fp32 device path. When the CUDA candidate downgrades, one `cuda_precision_fallback` notice gives the reason. Precision detection itself raises no error. Explicit device validation still applies when the command needs an embedder. Once the fp16 fingerprint is selected, attachment uses the chosen UUID and fails the write if the device is missing, the selector is unsupported or the attach otherwise fails. A certified OOM may split whole inputs on the same fp16 encoder. A singleton failure or device loss stops that section at its last checkpoint.

Format-3 manifests may include `"cuda_precision_request":"fp16"` for either actual writer under that request. Omission means fp32 request. Search and status validate the request and effective index fingerprint of each published section without probing hardware. On a default-changing upgrade, sync establishes the new policy for a previously unmarked index. Within that policy, an fp32 fallback index already built on an older card remains readable, including on a CPU-only host. CPU query vectors score either qualified profile. CPU catch-up skips fp16 sections before taking a lock or computing documents. Until sync covers a change, serve eligible existing hits and report the usual missing coverage.

The selector, revision, fallback cases, release tags and acceptance fixtures are specified in the [#61 implementation plan](../../../../docs/plans/cuda-fp16-adoption.md).

### Metal indexing profiles (#69)

The Q8_0 adapter qualifies two Metal writer profiles, `metal-eg2-stock-v1` and `metal-eg2-combined-v1`. The attached encoder reports its versioned profile. The vector fingerprint hashes the same four newline-separated fields as CUDA fp16, with the reported Metal profile in place of the CUDA selector. Use this fingerprint for the vector cache and derive the section's index fingerprint as usual. Both profiles share the pinned GGUF file. Moving between stock, combined and any base or CUDA fp16 generation rebuilds the index under the selected profile. Existing vectors can be reused only within that fingerprint.

The Metal batch route writes the combined profile and shares its index and vector cache with combined singleton syncs.

A nonempty Metal Q8_0 sync opens the verified model and attaches once per section before opening an index or looking up cached vectors. This applies even when the items are unchanged because discovery cannot determine the attached profile. The writer uses that same encoder for every passage. Explicit attachment failure ends sync before publication. Under auto, a fallback must resolve its actual profile before reading any old generation or cached vector. A Metal forward failure stops the section at its last completed checkpoint. Empty sections keep an accepted profile without attaching. The manifest omits `cuda_precision_request` for either Metal profile. Search and status can open both accepted fingerprints on the CPU without a device probe, independently of the CUDA request, and CPU catch-up skips both. [Implementation plan](../../../../docs/plans/metal-adoption.md)

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

`<cache>/models/<FolderName(m)>/<File.Name>` holds a downloaded model. The folder's name carries the digest of the model's files, so releases with different file digests use different folders. Sync publishes complete files. Sealing a verified copy or repairing a wrong file can replace the managed directory entry atomically. `model_path` names a folder that holds every file of the model's `Files` under the same names; a model of several files is supplied whole, and a missing companion file makes it absent. For a model with a pack, that means the pack: a machine without network gets it by copying the model folder from the cache of a machine that ran `pbrain sync`. Source files at `model_path` are not accepted and nothing is derived there, so the folder a person points at is only ever read.

**A model is opened only from verified bytes.** Open each regular file through its directory handle, check the opened file’s size, read exactly that many bytes into private memory and reject a trailing byte. `Registry.Open` takes the Artifact’s slices, clears them on return and gives the adapter no path to reopen. Strict verification compares the SHA-256 of the private bytes with the release table. Status and sync always perform that check on accepted files. External `model_path` uses it for every caller and is only ever read.

A search of the managed cache may authenticate the same open descriptor through Linux fs-verity. Each eligible final file has an additional release-pinned **file digest** generated from its full-SHA-verified bytes with fs-verity version 1, SHA-256, 4,096-byte blocks and empty salt on every architecture. The kernel stores the tree with the sealed inode. A matching measured digest authenticates reads from that descriptor. The resulting Artifact owns the same bytes, even if the name is later replaced. A sealed file cannot be modified in place. A missing pin, unsealed inode or unsupported platform/filesystem uses full SHA-256. A successfully measured sealed digest that differs from the pin is a hard wrong-model result. Read failures reject the Artifact. A different file of identical size or timestamps, or a record written into the cache by another process, cannot authorize other bytes. No userspace stat receipt or mmap participates.

Only sync prepares the managed cache. Under its existing model lock, it can write a private temp from the strictly verified final bytes, close its writer, enable and measure verity through a read handle retained for that inode, then rename the verified copy over its managed name. This migrates old unsealed cache entries without sealing a file that could also belong to an external model folder. An already sealed matching file requires no work. If the filesystem cannot enable the fixed profile, the original file remains available with strict verification. Removal or replacement of a corrupt sealed file uses directory operations. Search and status create no verification records. Pins for the Google pack, Q8_0 and UD-Q6_K_XL are checked against their real release files before shipping. The new pin is independent of the model and index fingerprints. [Implementation and acceptance plan](../../../../docs/plans/cache-model-verification.md)

Existing failure handling applies. A wrong model at `model_path` is `invalid_config` for sync. A corrupt managed file is removed and reprovisioned. A search that encounters a wrong model returns its lexical results with `ModelFailed`. Ordinary verification still runs in the background while the scan proceeds.

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
{"format": 3, "fingerprint": "<64 hex>", "model": "<name>", "dim": 384, "db": "0f3a9c1e77b2d455.db", "size": 59768832, "crc32c": "<8 hex>"}
```

The database, created by this schema (`1536` stands for four times the dimension, written as a literal when the table is created):

```sql
CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL) WITHOUT ROWID;
CREATE TABLE item (
    id      INTEGER PRIMARY KEY,
    path    TEXT NOT NULL UNIQUE,
    content BLOB NOT NULL CHECK (length(content) = 32),
    crc     INTEGER NOT NULL CHECK (crc BETWEEN 0 AND 4294967295),
    size    INTEGER NOT NULL CHECK (size >= 8)
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

`meta` holds four rows: `format` (`3`), `fingerprint`, `model` and `dim`. `item.path` is the item's path in the section with forward slashes; `item.content` is its `ContentDigest`, and `item.crc` and `item.size` are its `ContentCheck`. The manifest's `crc32c` is the CRC-32C of the database file's bytes as eight lower-case hexadecimal digits, most significant first. `passage.vector` is `dim` float32 values, little-endian; `passage.digest` is the SHA-256 of the embedded text. One path has one row, so an item is in the index once.

**Format 3** (owner's decision, 2026-11-21; architecture §16, decision 29) added `item.crc` and `item.size` and replaced the manifest's `sha256` by `crc32c`. No reader of an older format is kept. A manifest of format 2 is `IndexUnreadable`: a search answers `NoIndex`, the status prints `unreadable`, and the next build starts a new database and removes the old file as a leftover (step 5 of the build). Passage digests, `IndexFingerprint`, `ChunkerVersion` and the vector cache did not change, so that build takes every shared vector that is in the cache and embeds none of them; the personal section, whose vectors are never cached, is embedded again.

**SQLite is used in memory only; it is never given a path.** The package opens one connection to a private in-memory database (`file::memory:`, with `journal_mode=MEMORY`, `temp_store=MEMORY` and `foreign_keys=ON` set before the first transaction) and moves whole databases in and out of it with the driver's `Deserialize` and `Serialize`. The engine is `modernc.org/sqlite` with its bundled sqlite-vec; both are registered by import. The journal is kept in memory and not switched off, because SQLite does not define what a rollback does without one: a transaction that fails must leave the item as it was. Consequences: the handle rule of architecture §10.3 holds unchanged, since every byte on disk is read and written through `sys`; no journal, write-ahead log or shared-memory file ever exists; and a reader takes no lock.

**Reading** (`openIndex`, used by the source, the build and the status). Nothing from the manifest is used before it is checked:

1. Open `manifest` with `Files.OpenRegular` and read at most 4,097 bytes of it. Missing: `IndexNone`. Longer than 4,096 bytes, not a JSON object, or a `format` other than 3: `IndexUnreadable`. It is also `IndexUnreadable` unless `db` is exactly sixteen lower-case hexadecimal characters followed by `.db`, `crc32c` is eight lower-case hexadecimal characters, and `size` is a whole number from 1 to 2,147,483,647. Another fingerprint or dimension, with all of that in order: `IndexStale`.
2. Open the named file with `Files.OpenRegular` and ask the open file for its size. It must equal `size`, or the index is `IndexUnreadable` and nothing is allocated. Then read exactly that many bytes and compare their CRC-32C with the manifest. A missing file: read the manifest once more, since a build may have published; still missing, or any mismatch: `IndexUnreadable`.
3. `Deserialize` the bytes and drop pbrain's own copy. Read the four `meta` rows and require the manifest's values. Create the temporary table `eligible` and prepare the scoring statement below, which fails when a table or column it needs is missing or of another shape. A failure here: `IndexUnreadable`. A checksum that matches says the file was not damaged after it was published; it does not say the file is a pbrain index, which this step does.
4. Read `item` whole into a map from path to id and `Check`. A row whose `content` is not a blob of 32 bytes, whose `crc` is not a whole number from 0 to 4,294,967,295 or whose `size` is not a whole number of at least 8 makes the index `IndexUnreadable`. `content` is checked for its form and not kept.

5. **For the build and the status only**, a full check of what is stored, which a search does not pay for: no passage whose `vector` is not a blob of exactly four times `dim` bytes; no passage with `stop` below `start`; no passage without its item; and the scoring query run once with `eligible` holding every item and a fixed unit vector, read to its end. A failure: `IndexUnreadable`.

An index is **usable** when all four steps pass, and for the build and the status the fifth as well. An entry **covers** an item when the item's path relative to `Root` is in that map and the stored `crc` and `size` equal `ContentCheck(item.Header.Title, body)`, with the title and body exactly as this command scanned them. `content`, the SHA-256 of the same bytes, is written beside them when a build or a catch-up makes the entry and is not compared by anything. Until format 3 it was the comparison, and every search with `--semantic` hashed every body for it. The check is weaker than that digest by a known amount. An edit that changes the count of bytes is always seen. One that keeps the count and lies within four consecutive bytes of the title and body is always seen too, as is one that only moves bytes between the title's end and the body's start, since that changes the framed length alone. Any other edit that keeps the count is missed once in 2^32: the entry then still covers the item, and its old passages are scored until the item is edited again or the index is built afresh (architecture §17). The item's `Version` is not used, although it is a CRC-32C and a size already at hand: it covers the whole file, so a change to front matter that leaves the title alone, such as `verify`, would drop the entry, and with `ContentCheck` it keeps it. The check is computed, in parallel, only for items whose path has an entry; an item whose body cannot be read as scanned is not covered.

**Scoring** is one query per section, over the items that may be scored and no others. `eligible` is a temporary table of the connection (`CREATE TEMP TABLE eligible (id INTEGER PRIMARY KEY)`), emptied and filled for each search with the ids of the items that are eligible and covered. The query vector is a blob of `dim` float32 values:

```sql
SELECT item_id, ord, start, stop, d
FROM (SELECT p.item_id, p.ord, p.start, p.stop, vec_distance_cosine(p.vector, ?1) AS d
      FROM temp.eligible e CROSS JOIN passage p ON p.item_id = e.id)
WHERE d <= ?2;
```

`CROSS JOIN` makes SQLite start from `eligible` and reach `passage` through the index of `UNIQUE (item_id, ord)`, so `vec_distance_cosine` is called only on the passages of listed items. An item that is filtered out or not covered is never scored, and a bad row belonging to one cannot fail the query; a whole-table scan failed in exactly that case when tried. `?2` is `1 - Cutoff + 0.000001`: the query returns every passage of those items at or just under the cut-off, with no limit on their number, and `search` applies the cut-off exactly. Vectors are in an ordinary table and not in a `vec0` virtual table, because `vec0`'s nearest-neighbour query refuses more than 4,096 rows and its row-by-row read was 20 to 100 times slower in our measurement (architecture §8.7).

**Publishing** (the build, and a search's catch-up): `Serialize` the in-memory database; if the result is larger than 2,147,483,647 bytes, fail with `too_large` and leave the previous index in force; write the bytes to a temporary file in `index/` and flush it; link it under a new random name of the form above; sync the folder; write a manifest that names it with its size and CRC-32C and rename that into place; sync; remove the previous database file. Only a name of the generated form is ever opened or removed, so no manifest, however damaged, can make a build remove `lock`, `manifest` or anything outside the folder. A failure before the rename leaves the old index in force.

### Build

For one section, `Build`:

1. Fails with `no_model` when `ModelName` is empty, which means semantic search is off, and with `invalid_config` when the registry does not know it (the message lists `Names()`), when `CacheErr` is set, or when there is neither a cache nor a `ModelDir`.
2. If the section has no `.pbrain/index/` folder, calls `scan` once without a lock; with no item it returns a zero result and creates nothing. An empty section therefore never gets a `.pbrain/` from this command. If the folder exists, the build goes on even with no item: an index that is unreadable or built for another fingerprint is then replaced by an empty database, so the status stops reporting it.
3. Opens `.pbrain/index/`, creating both folders, and takes `lock`. The wait has no limit of its own: after 2 seconds it reports `Progress{Waiting: true}` once, and the context ends it. Then it calls `scan`: the catalog it works from is always taken while it holds the lock, so a build that waited cannot publish from an older view of the files than the build before it, or delete that build's rows.
4. **Makes sure of the model**, whenever the scan found at least one item, whether or not anything will be embedded. For a model in the cache it first takes the model's own lock, `<cache>/models/<FolderName(m)>.lock`, with `Files.Lock` under the command's context, and holds it until the model is verified: two projects of one person share the cache and each holds only its own index lock, so without it one build could remove source files another is still reading. The order is always index lock, then model lock (architecture §10.4), and this is the only place two locks are held at once. With the lock held it looks again, since another build may have finished the work. It calls `Find`; if the model is absent it calls `Download`, unless `ModelDir` is set, in which case an absent or wrong-sized file is `invalid_config`. For a model with `Source`, the files fetched are the source files, into `<cache>/models/<FolderName(m)>.source/`, each kept only if its size and digest are the pinned ones; then `Pack` writes the pack into `<cache>/models/<FolderName(m)>/`, and the source files are removed once `Verify` has accepted the pack; the empty `.source` folder may remain. A run interrupted before that finds the source files again and does not download them twice. Deriving holds the source and the pack in memory together, about 2 GB for EmbeddingGemma 2, once. It then performs strict verification and, for an eligible managed file, prepares its sealed copy as described in “The model on disk”. A failed identity check is `invalid_config` for `ModelDir`; for the cache the files that failed are removed, obtained again and verified once more, and a second failure is `fetch_failed`. So a run that reuses every vector still restores a model whose cache was deleted and still refuses a wrong local copy, and after any successful run a search finds the model. A source digest that is wrong is `fetch_failed`; a pack that does not come out with its pinned digests, and any file-system failure, is `io`; in every case no unverified pack is left under the model's name. The model lock is released here. The `Artifact` is kept only until it is known whether a passage must be embedded; the inference runtime is opened only for the first batch.
5. Opens the index (`openIndex`, with its full check). If it is not usable, which includes an index whose stored passages fail that check although no item has changed, it starts a new in-memory database with the schema and the four `meta` rows for the expected fingerprint. It then removes leftovers of a crash or of an older fingerprint: temporary files, and files with a name of the generated database form other than the one in use. It removes nothing else, whatever the manifest says.
6. Computes each item's `ContentCheck`, then finds the items no entry covers (to do), in path order, and the rows whose path has no item in the scan (dead); `ItemsRemoved` counts the dead paths. Dead rows are kept until step 10's last publication, so that step 7 can still take their vectors: an item that was only renamed is not embedded again.
7. For each item to do: computes its `ContentDigest`, cuts its passages from the same title and body, and for each passage looks for a vector by digest, the SHA-256 of its text, by the rule of "Reuse by digest" below. Passages found are reused. The others are queued.
8. Embeds queued passages a window at a time, as `Document`. Before the first window it opens the `Artifact` of step 4 with `Registry.Open` and `OpenOptions{Device: Options.Device, MemoryBudget: Options.GPUMemory}`, an empty `Device` standing for `auto`; with nothing queued, the runtime is never opened and the `Artifact` is dropped. The window is the embedder's `Window()` when it is `Windowed`, else 8; how the embedder runs the texts of a window, on workers or in batches on a device, is its own matter (the two adapters below). When the model is opened the build sends one `Progress` whose `Run` is the embedder's `Info()`, and another whenever `Info()` differs after a window; `BuildResult.Device` is the last `Info().Device`. An `Open` that fails with `invalid_config` ends the run with that error; one that wraps `ErrAccelerator` ends it with `io`, `the model cannot be opened`. An `Embed` that fails ends it with `io`, `the model failed to embed a passage`; what was published stays, and the next run goes on from it. A vector from the model that is not of the right length, not finite or not of unit length within 0.001 fails the run with `io`; a vector from the cache or from the database that fails the same test is treated as absent and embedded again. For the shared section each new vector is written to the cache as soon as its window returns.
9. When all passages of an item have vectors, replaces the item in one transaction: delete the row of that path, which removes its passages, then insert the item, with its digest and its check, and its passages.
10. Publishes whenever 20 seconds have passed since the last publication, at an item boundary, and at the end if anything changed. The clock is read as each item is stored, and an item is stored when the window that holds its last passage has returned from the embedder, so two publications can be 20 seconds and one window's embedding apart. Before the last publication the dead rows are deleted in one transaction. A publication in between may still hold them; they cover no item and are never scored. What an interruption can lose is therefore two things: the window being embedded, whose vectors have reached neither the database nor the cache, and the items stored since the last publication, at most 20 seconds of them. For the shared section the vectors of every window that returned are in the cache, so only the window in flight is embedded again; a personal item, whose vectors are never cached, is embedded again from its start if it was not published. A window is 8 passages for an embedder that names none, which EmbeddingGemma 2 on the processor is, and 256 for EmbeddingGemma 2 on a GPU: several seconds of work on the processor at the measured rates, and about 20 seconds on a GPU with long passages (found by @lead-dev, 2026-11-22; the earlier text promised 20 seconds and the item in progress, which held when a window was 8). A run that changed nothing publishes nothing.
11. Reports `Progress` every 10 seconds, closes the runtime if it opened one, releases the lock and returns the counts. An error from the embedder's `Close` is dropped, as in the source, whatever it ran on: the last publication is done by then, the index on disk is whole, and the run returns its counts with no error (spec 9.4; operator's decision, 2026-11-22, for an accelerator's close error too).

**Reuse by digest.** What a passage is given must not depend on the window, so the rule is stated on sets that are fixed when the run starts, never on what happens to be stored at the moment an item is cut. `replace` is the set of paths this run will store, known at step 6. A *base row* is a row as the database held it when the run started. For a passage of item I with digest D, the vector is the first of:

1. **I's own base rows** with D, lowest `ord` first. They are there whenever I is cut, since only I's own replacement removes them.
2. **Base rows of paths outside `replace`** with D: unchanged items, and items gone from the section, whose rows step 10 keeps until the last publication. Lowest path first, then lowest `ord`.
3. **This run's own answer for D**: what the first passage with D in path order was given, whether by a hit in 1, 2 or 4 or by a new embedding. The build keeps a map from every digest it has resolved to where its vector is: in memory while that item is not stored, then the stored row's path and `ord`. About 100 bytes a passage.
4. **The cache**, for the shared section. It is asked only for a digest this run has not met, so it answers from what it held before the run.

Rows of other paths in `replace`, old or newly stored, are never read for this. `vectorByDigest` takes the asker's path and `replace`, joins `passage` to `item`, and returns tiers 1 and 2 in that order. A hit in any tier counts in `PassagesReused`; a passage that reaches the end is embedded, counted in `PassagesEmbedded` and entered in the map. So a passage whose twin is queued in the same window is not embedded twice, and `PassagesEmbedded` is the number of distinct digests no tier answered, at any window, for one starting index and one starting cache. Two cases this rule decides the same way at every window, where a lookup in whatever is stored would not: in a personal section A held X, A is edited to Y and a later B now holds X, and B embeds X; A and an unchanged Z both hold X with different bytes from two devices, A's other passage is edited and a new B holds X, and B takes Z's.

The two counters report the work of one invocation. A run interrupted after a window's vectors reached the cache reports them as reused when it resumes. A resumed run ends with the same stored items and passages as an uninterrupted one, and with the same vector bytes when the index it started from held one vector for each digest; where it held two, a checkpoint moves a path out of `replace` and the resumed run may take the other.

The four `meta` rows are inserted in a fixed order, by key, so that two builds of the same section on one device give the same file.

An interrupted run leaves a usable index holding every published item; the next run finds the rest to do. An item whose body cannot be read as scanned (`store.ErrChanged`) is skipped and stays to do. Publication rewrites the whole database, about 160 ms at 35,000 passages of 384 dimensions by a scratch measurement, which is why it is paced by time and not by count.

### The vector cache

`<cache>/vectors/<model fingerprint>/<first two hex of the digest>/<digest hex>` holds one vector: `dim` float32 values, little-endian, then the CRC-32C of those bytes. A read accepts only a file of exactly that size with a matching checksum and finite values of unit length within 0.001; anything else is removed and counts as absent. A write goes to a temporary file in the same folder and is linked under its name without a flush; an existing name is left alone. No lock is taken. Only `Build` and a search's `CatchUp`, each for the shared section, read or write it.

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
| the registry does not know the name, the removed `bge-small-en-v1.5` included | `UnsupportedModel` |
| `CacheErr` is set, or a cache folder on the way to the model holds `.git` | `CacheExposed` |
| `embed.Find` fails: a file missing or of the wrong size | `ModelAbsent` |
| no section in `dirs` has a manifest with the expected fingerprint | `NoIndex` |

If all pass, it starts one goroutine that verifies through `VerifyManaged(..., ManagedSearch)` for a managed model or `Verify` for `model_path`, opens the `Artifact` it returns with the zero `OpenOptions`, and embeds the text as `Query`, while a second opens each section's index. The zero options are the CPU: a search never attaches an accelerator, in any build and whatever `Options.Device` says (owner's decision, 2026-11-22). Check, load and one query on the CPU are inside rule 3 for both models, where an accelerator's start-up alone took EmbeddingGemma 2 past it (1,050 ms against 286 ms; `internal/cli/README.measure.md`). A catch-up embeds through the same embedder, one text a call, so on the CPU too. The query vector must have `Dim` finite values and unit length within 0.001. The goroutine runs under a context that `Close` cancels. `Score`, after `Attach`, then:

1. Takes each section's index, which the background task has opened with `openIndex` beside the model and `CatchUp` may have added to, and decides coverage. A section whose index is not usable contributes all its items to `Missing`; in a usable one, each item no entry covers adds one. If no section is usable: not ready, `NoIndex`.
2. Waits for the background task. A failed verification, a failed open, a failed embedding or a query vector that fails its test: not ready, `ModelFailed`, and the search keeps its lexical results. A `text` other than the one given to `Start` is a programming error and panics.
3. For each usable section, fills `eligible` with the ids of the items that are eligible and covered, and runs the scoring query. It drops every row whose span does not lie inside the body as scanned. For each item it keeps the row with the smallest distance, and on a tie the smallest `ord`. The score is `1 - d` as a float32. If filling the table, the query or the reading of its rows fails, every row already read for that section is dropped, the section is treated as not usable for this search and all its items are added to `Missing`, once; the other section's hits stand. If no section is left: not ready, `NoIndex`. The hits are returned in the order of the caller's list; items with no row are not hits.
4. Returns `Ready`, `Missing`, the model's `Cutoff` and the hits.

`Score` returns an error only when the context ends. `Start`, `Attach`, `Score` and `Close` never write and never take a lock, and the source never downloads. `CatchUp` is the one method that writes.

**Closing the model fails nothing.** `Close` comes after the search has its scores. If the embedder's `Close` returns an error, the source drops it and releases the embedder all the same: the scores stand, and the command prints the results and exits as it would have (spec 7.1). There is nothing a caller could do with such an error, and a cleanup that fails must not take back an answer that was right. `Close` therefore returns nothing. A search has no accelerator, so no failure of one can reach it, and an error that wraps `ErrAccelerator` is treated as any other: a failed open or a failed embedding of the query is `ModelFailed`, and the search keeps its lexical results; a failed embedding in a catch-up leaves those items uncovered, as every catch-up failure does, and the items the index covers are still scored.

### Catch-up in a search

`CatchUp` does for a few items what `Build` does for any number (owner's decision, 2026-11-21; architecture §8.7 and §16, decision 30). It reuses the build's own functions for cutting, reuse by digest, replacing an item and publishing; it has no second copy of them.

```go
const (
    catchUpItems    = 20              // most uncovered items a search takes on
    catchUpPassages = 20              // most passages it embeds
    catchUpBudget   = 2 * time.Second // no embedding call starts after this
)
```

The source keeps, for each section, the manifest its index was opened from. `CatchUp`:

1. Returns at once when `Start` found the source not ready. Otherwise it waits for the task that opens the indexes.
2. **Finds the gap.** In each section whose index is usable, the uncovered items: those no entry covers, with a body that can be read as scanned. A section without a usable index contributes none: it is not caught up, and `Score` counts its items as before. With no uncovered item, or with more than `catchUpItems` over all sections, it returns.
3. **Plans**, without a lock and without writing. For each uncovered item, shared section first, in path order: `ContentCheck`, `ContentDigest`, `Passages`, and for each passage a vector by digest, first from the in-memory database, then, for the shared section, from the cache. Vectors found are kept. If the passages without a vector number more than `catchUpPassages` over all sections, it returns.
4. Waits for the model task. Without a query vector (`ModelFailed`) it returns.
5. **For each section with uncovered items**, shared first:
   1. `Files.TryLock(index, "lock")`. `ErrLocked` or any other failure: this section is skipped.
   2. Reads the manifest again. If it is not equal, field for field, to the one the index was opened from, a build has published since: the lock is released and the section skipped. So a catch-up never publishes over a newer index.
   3. For each item, in order: embeds its passages that have no vector, one passage a call, as `Document`, with the embedder that made the query vector. Before each call it compares the time since the first lock was taken with `catchUpBudget`; once that has passed it starts no call, and the items not finished stay uncovered. A vector that is not of the right length, not finite or not of unit length within 0.001 ends the catch-up of this section. For the shared section each new vector is written to the cache at once, as in a build. When every passage of the item has a vector, the item is replaced in one transaction, with its digest and its check.
   4. If any item was replaced, publishes as a build does. A failed publication is not an error.
   5. Releases the lock.
6. Returns. `Score` then decides coverage on the in-memory databases as they now are, so an item replaced in step 5 is scored whether or not its publication succeeded.

What it never does:

- It never creates `.pbrain/`, `index/` or the cache folder. The index folder exists, since a usable index was read from it. `cli` opens the cache with `create = false`; when the cache folder is missing, as it can be with `model_path`, catch-up neither reads nor writes the cache. Folders below an existing cache folder are created as a build creates them, each tested for `.git` first; `ErrCacheExposed` ends the catch-up.
- It never removes a row whose path has no item, a leftover file or a stale database: the build does (steps 5 and 10 of "Build"). `publish` removes the database file it replaces, as always.
- It never takes the model lock, never calls `Download`, `Pack` or `Verify` again, and never opens a second embedder.
- It never writes a personal vector to the cache.
- It never waits for a lock, and it never runs the full row check of step 5 of "Reading".

Why these choices:

- **The manifest is compared under the lock** because the scan and the index were read before the lock was taken. A build that published in between holds rows this search has not seen, and publishing the search's copy would drop them.
- **The scan is not taken again under the lock**, unlike a build's. Catch-up deletes nothing but the old row of an item it replaces. If a file changed after the scan, the entry made from the scanned text fails the next command's check and is replaced then.
- **One passage a call** keeps the overrun past `catchUpBudget` to one passage, also for a model that needs a second for each. The build's batches of eight would overrun by up to eight.
- **Both size bounds are decided before the lock**, so a gap that is too large costs a search the cutting and the lookups of at most 20 items and no write.
- **The time starts at the first lock**, not at `CatchUp`, so that the bound is on the embedding and not on the wait for the model.

The `index_progress` notice of a build that waits says `<section>: waiting for another pbrain command`. Before catch-up it named `pbrain sync`; the holder can now be a search, for a few seconds.

### What `cli` does

- **Which model.** `Options.ModelName` is `Config.Semantic.Model` as `brain` resolved it (`components/brain.md`): the name `pbrain.toml` gives; `brain.DefaultModel`, which is `embeddinggemma-2-q8-0`, when it has no `semantic.model` key; and `""` when it says `"none"`. `semantic` and `embed` hold no default of their own, so a test registry needs no entry of that name, and a test in `cli` or `cmd/pbrain` checks that the release registry holds `brain.DefaultModel`.
- **`model_path`.** `cli` makes a relative `model_path` absolute against the brain folder and opens it with `sys.OpenNamed(env.Files, path, false)`. The folder is one the person names, so a link in the path or above it is followed (architecture §10.3); inside the folder `Find` and `Verify` follow none, and the digests decide what the files are. A folder that cannot be opened is `invalid_config`. The calibration tool opens its model folder the same way.
- `search --semantic`: after `brain.Open` and before the scan, open the cache with `create = false` and call `Start` with the query text and the view's section folders, deferring `Close` so that a failed scan or any later error cancels the work. After the scan, which already keeps bodies for a search, `Attach` one `SectionInput` per section, call `CatchUp`, put the source in `search.Request.Semantic`, and turn `Result.Sem` into one notice: `semantic_not_ready` with the reason and `pbrain sync` (or `PBRAIN_CACHE` for an exposed cache), or `semantic_partial` with `Missing` when it ran and `Missing` is above zero.
- `pbrain sync`: open the cache with `create = true`, then call `Build` for the shared section and, with `--personal`, for the personal one, in that order, each with a `ScanFunc` that scans with bodies kept, and print one line per result. A personal section whose folder does not exist is not created: its line reports no item. `notify` becomes the `model_download` notice and `progress` the `index_progress` notices; a `Progress` whose `Run` is set becomes `embedding_device` or `cpu_fallback` instead, each printed at most once per command ("Output and notice texts"). `Options.Device` is the value of `--device`, else of `PBRAIN_DEVICE`, else empty; `Options.GPUMemory` is `PBRAIN_GPU_MEMORY`, a whole number of MiB of at least 1, in bytes. `PBRAIN_METAL_BATCH=0` sets `Options.MetalSingleton`. `cli` validates the form before scanning: a selector is `auto`, `cpu`, or a backend name of lower-case letters with an optional `:` and what follows it; other selectors, malformed GPU budgets, and nonempty `PBRAIN_METAL_BATCH` values other than `0` are `invalid_config`. Whether the selector names something is `Registry.Open`'s to say.
- `pbrain devices`: no brain is opened. Print `Registry.Devices()`, one line each.
- `pbrain sync --status`: one scan with bodies kept, the cache with `create = false`, then `Status`. A section whose folder or index folder does not exist is reported as `IndexNone`, with its item count, and nothing is created. `ModelPresent` is the result of `Find` and `Verify` together.

### Output and notice texts

`cli` prints these; this section is their one definition. No text names an item, a path or a URL.

**`pbrain sync`.** One line per `BuildResult`, in the order built, all four numbers always present, a noun singular when its number is 1:

```
shared: 5000 items indexed, 1 item removed; 12 passages embedded, 3 reused
```

JSON: `{"model": "<ModelName>", "sections": [{"section": "shared", "items": 5000, "items_removed": 1, "passages_embedded": 12, "passages_reused": 3, "device": "cuda:0"}]}`, numbers printed also when zero, and `device` left out when `BuildResult.Device` is empty. The text line does not name the device; the notice does.

**`pbrain devices`.** One line per `DeviceInfo`, GPUs first in the order of the automatic choice, then the CPU: the id, the name when there is one, and the memory: `<total> MiB, <free> free` with both figures, `<free> MiB free` with the free one alone, `<total> MiB` with the total alone, and nothing with neither; the parts are separated by two spaces or more, in columns. The CPU's line is `cpu` and `<n> threads`, the number Go is allowed. With goinfer v0.6.3 both devices can show their name, capacity and free figure. CUDA reports live free bytes. Vulkan reports estimated headroom if its driver supplies it. Nothing is marked as chosen: what a sync uses depends on the project's model, which this command does not read.

```
cuda:0  NVIDIA RTX PRO 6000 Blackwell Max-Q  97887 MiB, 97249 free
vulkan:0  AMD Radeon RX 9060 XT                16304 MiB
cpu     32 threads
```

JSON: `{"devices": [{"id": "cuda:0", "backend": "cuda", "free_bytes": 101386944512, "memory": "live", "memory_kind": "unknown"}, {"id": "vulkan:0", "backend": "vulkan", "name": "AMD Radeon RX 9060 XT", "persistent": "vulkan:<uuid>", "total_bytes": 17095983104, "memory_kind": "dedicated"}, {"id": "cpu", "backend": "cpu", "threads": 32}]}`, a field left out when empty or unknown.

**`pbrain sync --status`.** The first line is one of `model: none` (semantic search is off), `model: <name>, on this machine`, `model: <name>, not on this machine` and `model: <name>, not supported by this pbrain`. Then one line per section of the view: `<section>: <Indexed> of <Items> items indexed` (`1 item` when the section holds one), followed by ` (built for another model)` for `IndexStale` or ` (unreadable)` for `IndexUnreadable`; `IndexNone` adds nothing. JSON: `{"model": "<name>", "model_present": true, "sections": [{"section": "shared", "items": 5000, "indexed": 4988}]}`, with `"stale": true` or `"unreadable": true` on a section; `model` is left out when semantic search is off and `model_present` when false.

**Notices.** `<x>` is `pbrain sync` in the shared view and `pbrain sync --personal` in the personal view.

| Code | When | Message |
|---|---|---|
| `semantic_not_ready` | `NoModel` | `semantic search is off: pbrain.toml sets model = "none"` |
| `semantic_not_ready` | `UnsupportedModel` | `this pbrain does not support the model pbrain.toml names: pbrain sync lists the supported names` |
| `semantic_not_ready` | `CacheExposed` | `the cache folder lies in the brain, the personal section or a Git repository: set PBRAIN_CACHE to a folder outside them` |
| `semantic_not_ready` | `ModelAbsent` | `the model is not on this machine: run <x>, or set model_path` |
| `semantic_not_ready` | `NoIndex` | `no index for this model: run <x>` |
| `semantic_not_ready` | `ModelFailed` | `the model could not be used: run <x> to check it` |
| `semantic_partial` | `Ready` and `Missing > 0` | `<Missing> items are not in the index: run <x>` (`1 item is` for one) |
| `model_download` | from `notify`, before the first request | `downloading model <name>, <n> MB, from <host>`, with the size in units of 1,000,000 bytes rounded up and the distinct hosts of the files' addresses, comma-separated |
| `index_progress` | from `progress`, every 10 seconds | `<section>: <Done> of <Total> items` (`1 item` for a total of one), or `<section>: waiting for another pbrain command` when `Waiting` |
| `embedding_device` | the first `Progress.Run` of a `pbrain sync` whose `Device` is a GPU | `<Device> (<Name>), <n> MiB a batch`: the parentheses are left out when `Name` is empty, and the comma and what follows it when `Budget` is 0; `<n>` is `Budget` in MiB, rounded down |
| `cpu_fallback` | the first `Progress.Run` of a `pbrain sync` whose `Fallback` is set | `embedding on the CPU, which is slow: ` and, for `asked`, `the CPU was asked for`; `unsupported`, `this pbrain has no GPU support for <model>`; `absent`, `no usable GPU was found`; `failed`, `the GPU could not be used`; `memory`, `the GPU has too little free memory` |

Neither of the last two is printed by a search, by a sync that embeds nothing, or for a model with no GPU path, whose `Fallback` is always empty (owner's decision, 2026-11-22: the CPU is that model's normal path; since v0.5.0 only test models are such). A sync that begins on a GPU and gives it up prints both, in that order.

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

Whatever runtime is chosen, its adapter (a sub-package of `embed`, imported only where the release registry is built) must: build the model from the `Artifact`'s bytes alone, opening no file; hand those bytes to its runtime without keeping a second reference that could write them, and never write them itself; add the model's own query and document prompts; keep text from a body from being read as a special token of the model; cut an over-long text at the model's token limit, and for a document cut it so that the passage keeps at least half of that limit: the prefix is cut first, only as far as needed and never below half the limit, then the passage from its end; print nothing on standard output; return unit-length vectors of `Dim`; give the same vectors for the same text in any batch, at any worker count and at any memory budget, on one device; return them in the order of the texts; open on the CPU unless `OpenOptions` asks otherwise; hold no global state between embedders; and make no network call.

**Stray output.** A successful command cannot show that a dependency's failure path stays silent. One stray line can break the output contract in spec 6.3. `cmd/pbrain` takes the process's real standard output and standard error for `Env.Stdout` and `Env.Stderr`, then points the variables `os.Stdout` and `os.Stderr` at the null device, before any command runs. `cli` writes only through `Env`, so its output is unchanged, and anything a linked library prints through those variables is discarded, whatever path it takes. A crash of the Go runtime still reaches the real standard error, because the runtime writes to the descriptor itself.

**A GPU driver is outside that guarantee.** It is native code that a backend loads at run time, and it writes to the process's descriptors, past those two variables. Mesa's Vulkan driver for AMD cards prints `WARNING: radv is not a conformant Vulkan implementation, testing use only.` on standard error when it lists a card it is not certified for, and `pbrain devices` showed that line on the test machine (issue #49). pbrain neither redirects nor suppresses what a driver writes (operator's decision, 2026-11-22). So where a GPU backend is asked, which is `pbrain devices` and a `pbrain sync` that lists or attaches a device, standard error may hold lines that are not pbrain's, and spec 6.3 tells a program to skip a line that is not a JSON object. A search asks no driver, and its streams are pbrain's alone. Standard output held the result alone in that run; pbrain could not stop a driver from writing there either, so it is looked at in every recorded GPU run. Mesa leaves that one line out when `MESA_VK_IGNORE_CONFORMANCE_WARNING` is set to true in the environment; pbrain does not set it.

**Environment reads.** The embedding runtime contract permits no environment reads on pbrain's command paths. `TestRuntimeEnvAndOutput` checks the release build graphs and records exact source sites on reviewed pins. goinfer v0.6.5 moves the eight decoder settings to a once-only read on first decoder use. pbrain's embedding paths do not use that decoder. The audit's known startup list is empty on this pin. [goinfer #70](https://github.com/pkieltyka/goinfer/issues/70)

**Audit scope.** This is a best-effort check of selected Go source. It recognizes the listed API references, callbacks, and explicit `http.Client` and `http.Transport` literals and allocations. A transport literal may specify `Proxy: nil` for a direct connection. Reachability through arbitrary call paths, reflection, assembly or the C ABI needs separate review.

**Platform runtime exceptions (#55).** The pinned `golang.org/x/sys/cpu` initializer reads `GODEBUG` for Go's CPU-feature policy. Invalid or unsupported CPU options can produce diagnostics on real stderr before command startup. In CUDA builds the purego runtime can also write a terminal diagnostic if its thread bootstrap cannot allocate or create a thread and aborts. These writes precede or bypass command stream redirection.

The pinned modernc libc supplies the C process environment to SQLite. In Linux builds, SQLite's vector-extension initializer creates the first TLS object and copies the environment once, including for commands that never open an index. Darwin's TLS constructor does not perform that copy. pbrain neither reads, logs nor transmits the snapshot. SQLite on Linux can consult `MODERNC_SQLITE_OFD_LOCK` while initializing disk-file locking. pbrain keeps its SQLite connections private and in memory and uses its own file locks for publication. Each new connection sets `temp_store=MEMORY` and `journal_mode=MEMORY` before any index work, and reapplies both after deserialization. Consequently `SQLITE_TMPDIR` and `TMPDIR` do not select a directory for index temporary storage. The audit pins each exception to its reviewed dependency version and exact source site.

### Devices

Owner's direction, 2026-11-22: the GPU is the primary path for `pbrain sync`, the CPU a fallback that still works and says so, and a machine can have several GPUs. The plan and its measurements are `docs/plans/sync-throughput.md`. A search is not part of this: it opens the CPU ("The source").

**Names.** `cpu`. A GPU is `<backend>:<index>`, with the backend's name as the runtime's listing gives it: `cuda:0`. The index is the device's place in that backend's own listing, which can change with a reboot or a visibility mask; each device also has the persistent id its runtime gives it, when it gives one. pbrain fixes no backend name of its own. An AMD card is `vulkan:<index>`, from goinfer's Vulkan backend, and its persistent id is the card's Vulkan UUID, written `vulkan:` and 32 hexadecimal digits; the id holds until the machine's hardware or driver changes. A Linux amd64 build with the `vulkan` tag supports the qualified Google EmbeddingGemma 2 pack. CUDA persistent names include the driver UUID, for example `cuda:GPU-<uuid>`.

**Selectors**, in `OpenOptions.Device`:

| Selector | Means | When it cannot be had |
|---|---|---|
| `auto` | the automatic choice below | the next candidate, at last the CPU with a `Fallback` |
| `""`, `cpu` | the CPU | — |
| a backend, `cuda`, `vulkan` or `metal` | the first device of that backend in the automatic order | error |
| one device, `cuda:0`, `vulkan:0` or `metal:0`, or its persistent id, `vulkan:<uuid>` | that device and no other | error |

A named selector fails in one of two ways. It names nothing the listing shows, be it a backend this binary lacks, a backend with no device on this machine, or an index or id that is not there: `invalid_config`, whose message lists what the listing shows and names `pbrain devices`. Or it names a listed device and attaching it fails: an error that wraps `ErrAccelerator`, which `Build` reports as `io`. A named device never gives way to another GPU or to the CPU. A device is named by a selector equal to its `ID` or to its `Persistent`; a persistent id is always written after its backend and a colon, never bare, so the form of a selector says which backend it means.

**The listing** uses goinfer's `embeddinggemma2.Devices(ProbeOptions{IncludeMemory: true})` with no model open. Each usable batch-capable row supplies its alias as `ID`, its backend-qualified device id as `Persistent`, the name, memory kind, total bytes and free figure with its basis. CUDA reports live free bytes. Its rows also carry the goinfer compute capability and runnable selectors for writer selection. Vulkan can report estimated local-heap headroom. Rows without an unambiguous, addressable alias and persistent id are omitted. Both GGUF rows use the unified CUDA and Vulkan listing, including the reported failures for the linked backends.

A probe can return usable rows and errors together. A missing CUDA driver or `ErrNoDevice` is absence. CUDA memory-probe and context-cleanup failures stay in the listing. For a Vulkan error wrapping `ErrBackendUnavailable`, a missing loader and a `create instance` result of -9, -3, -6 or -7 are absence. The result can appear as `VkResult -9` or as a named value such as `VK_ERROR_INCOMPATIBLE_DRIVER (-9)`. Other errors stay under `Listing.Failed`, including OOM, an unexpected initialization failure, or an unrecognized result number. Separate errors in a joined probe failure are classified individually.

On Darwin arm64 with the `metal` tag, the Q8_0 inventory accepts one system-default Metal row with empty upstream ID and alias and no advertised batch capability. The command lists it as `metal:0` with its reported name and shared-memory estimates when available. Both `metal` and `metal:0` select it. It has no persistent UUID. Only Q8_0 can select Metal, including under auto.

Whether `auto`, `metal` or `metal:0` selects it, a Metal attachment first calls `Encoder.UseAccelerator("metal-eg2-batch")`, which goinfer v0.6.9 accepts on a base Apple M4 only. The batch route requires the attachment to report the system-default device, `TextBatching: true` and `metal-eg2-combined-v1`. If goinfer returns `ErrUnsupportedSelector`, or if the attachment fails those checks and closes successfully, the adapter attaches again with `Encoder.UseAccelerator("metal")`. That route validates the system-default device and a stock or combined profile, then uses a window of one with `EmbedIDs`. After successful singleton fallback, sync emits the usual `embedding_device` notice when it embeds, without a batch budget. The refusal adds neither a separate notice nor `cpu_fallback`. The batch route includes its budget in `embedding_device`. Other attachment failures follow the named-device or automatic-selection policy above. Set `PBRAIN_METAL_BATCH=0` to skip the batch attempt. Unset or empty uses the default. Any other value yields `invalid_config` before scanning, regardless of the requested device or model.

The batch route prepares windows of up to 256 passages and packs ordered groups of at most four inputs and 2,748 prepared positions. The budget is 30% of the attached device's estimated headroom, capped at 1 GiB, or 1 GiB when headroom is unknown. `PBRAIN_GPU_MEMORY` overrides it under the existing limits. The packer uses `BatchMemory` to check each proposed group. An input that cannot join its neighbor goes through `EmbedIDs`. A certified allocation failure splits the failed group and retries in order. A failed singleton ends the section at its last checkpoint.

**Attachment** passes the selected persistent id to `Encoder.UseDevice` with the linked CUDA kernel selector or `vulkan`. It checks the attached reporter's id against the request before it uses the session. An attach-time `ErrUnsupportedSelector` identifies an ineligible kernel separately from `ErrNoDevice`, with the device UUID in both errors. A moved alias can therefore refer to another card in the second inventory without redirecting the attachment. A failed candidate's accelerator is closed before another candidate is tried. A failed close ends the open, preserves the failure and retains the handle for a cleanup attempt.

**A listing that failed** is carried by `Entry.Devices`, so that an empty answer and no answer stay apart, and three callers read it:

- `Registry.Open`, for a selector that names a backend or a device. A device that matches is used, whatever else failed. With no match, a selector whose backend, the part before its colon, is in `Failed` gets an error that wraps `ErrAccelerator` and that backend's error, which `Build` reports as `io`: what the machine has is not known, so the name cannot be called wrong. Any other selector with no match is `invalid_config`, as before. The registry hands the entry's `Open` the matched device's `Persistent` when it has one and its `ID` otherwise; the adapter binds that identity on attachment and reports its `ID` in `Info`. When the writer already resolved a device for precision selection, pass the same snapshot through validation and open. An attach-time disappearance wraps `ErrAccelerator`.
- The adapter under `auto`. It tries the devices that were listed, in order, including usable rows from a partial listing. If one attaches, the failure is not reported. If none does, or none was listed, the CPU runs with `failed` when `Failed` is not empty and with `absent` when it is.
- `pbrain devices`, through `Registry.Devices`, which drops `Failed`: the command prints what could be listed and exits 0, with no notice. A person who then names the backend in a sync gets the `io` error with the driver's message. A notice of its own would be a new code in the output contract for a failure that the sync already reports.

`TestVulkanAbsentTexts` checks the two absence diagnostics in the pinned Vulkan module.

**When a selector is read.** `cli` checks its form before the scan. What it names is checked by `Registry.Open`, which the build calls lazily when it has a passage to embed, except for the early Metal profile binding on nonempty Q8_0 sections. The CUDA precision and Metal candidate checks can list devices before deciding whether a section needs new vectors. For a model with no GPU path the selector is not consulted at all, and every value gives the CPU with an empty `Fallback`, without a listing: `PBRAIN_DEVICE` is set once for a machine, and a person who names a GPU there must still be able to sync a project that uses the default model.

**The automatic choice.** Candidates are the devices of the backends compiled in that can run the model in batches. A device is dropped only when a live figure says it cannot hold the weights. The rest are tried in this order, and the first that attaches is used:

1. a free figure, live or estimated, before none: a device whose free memory is not known cannot be sized for and comes after every device whose free memory is;
2. memory kind: dedicated, then unknown, then shared;
3. the free figure: live, then estimated;
4. most free memory, compared only within one row of 3;
5. backend: `cuda`, then `vulkan`, then `metal` (operator's decision, 2026-11-22);
6. lowest index.

On pax both cards report dedicated memory with a free figure. CUDA's live figure orders it before Vulkan's estimate. The same keys apply across every device the model can use.

With none left the CPU runs, and `Fallback` says why: `unsupported` when no backend is compiled in, `absent` when none lists a device, `failed` for a failed listing or attachment, and `memory` when a live free figure is below the model's upload floor. A selector of `""` or `cpu` lists nothing and touches no GPU driver, which is every search.

**The memory budget** is what a batch of several texts may take on the device, decided once, after the weights are uploaded, from that device's free memory:

| The free figure is | Budget |
|---|---|
| live | 30% of it, at most 6 GiB |
| an estimate | 30% of it, at most 1 GiB |
| unknown | 1 GiB |

The attached accelerator's `DeviceMemoryReporter` supplies a fresh reading after weight upload. A missing reporter or successful unknown reading uses the unknown rule. A read failure declines the candidate and follows the usual attach error rules. A known zero gives a budget of zero. `OpenOptions.MemoryBudget` replaces the default budget, up to 6 GiB and at most the live free figure, when known. The reading reserves nothing. Other GPU users, including a display on the same card, can use memory between the probe and the forward.

**One text alone always runs**, whatever the budget (owner's decision, 2026-11-22). The budget limits how many texts share a forward pass; it cannot make one passage smaller, and a device that works is never refused beforehand because a longest passage might not fit.

**Back-off.** Only `ErrBatchOutOfMemory` on a forward certifies that the same encoder can retry. Cancellation and device loss take precedence. On a multi-input failure, halve the failed group's aggregate positions, keep any smaller previous limit, and reprice and run smaller groups of whole inputs in order. The new limit lasts for the section. A successful prefix is kept. After a certified singleton failure a named device returns its accelerator error. For an fp32 generation under `auto`, the adapter closes the device, embeds the failed input and the rest on its retained CPU encoder, and reports `{Device: "cpu", Fallback: "memory"}`. If closing fails, the handle stays for cleanup and the sync fails. An unmarked allocation, attach or forward error never authorizes a retry. A terminal error returns no vectors from that embed call.

**Device loss.** The Gemma adapter wraps goinfer's `ErrDeviceLost` on singleton and batch forwards. Sync reports an `io` error naming the device and closes the failed accelerator during cleanup. The next sync starts a new accelerator from the last published checkpoint.

**Device identity in storage.** The manifest and SQLite metadata identify the vector profile through the index fingerprint. No physical device ID is recorded per section or per passage. The base profile accepts vectors from its qualified CPU, CUDA and Vulkan backends for that row. The fp16 profile accepts only vectors from its qualified selector and revision. A switch rebuilds the section using the destination profile’s cache namespace.

**Sameness within a vector profile.** Two builds of the same section from the same index and cache, on one device with no fallback, give the same file at any window, worker count and budget, and are compared by bytes. Three comparisons, and only these, may show different vector bytes: one device against another; a run that fell back to the CPU part of the way against one that did not, where the point of the fall depends on what else held the card's memory; and a run that reused vectors another device had put in the index or the cache.

### Our libraries

The Gemma adapter uses the goinfer fork at `github.com/pkieltyka/goinfer`. pbrain pins its root, CUDA, Vulkan and Metal modules at v0.6.9 (`a1333cf2b78d`). CUDA links under the `cuda` tag. Vulkan links under the `vulkan` tag on Linux amd64. Metal links under the `metal` tag on Darwin arm64. The public proxy identifies all four module copies with that tagged commit and the checksum database authenticates their sums. The preceding pins and their measured gates are recorded in architecture §8.7.

The ordinary CUDA selector in v0.6.7 uses transposed attention for eligible batches with a longest prepared input of at least 128 tokens. The #66 sync comparison matched Q6 and Q8_0 ordinary CUDA output bits against v0.6.6 on two fixtures. The base model and index fingerprints are unchanged. The Google and Q8_0 CUDA fp16 profile fingerprints include the pinned goinfer revision. Updating from v0.6.7 to v0.6.8 rebuilds these indexes on sync. The matched RTX comparison kept the ordinary fingerprints and matched every tested CUDA vector bit for bit. [Qualification](../../../../internal/cli/README.measure.md#goinfer-v067-cuda-sync-66)

Updating from v0.6.8 to v0.6.9 makes the revisioned CUDA fp16 and Q8_0 Metal indexes stale until sync rebuilds them. The base profiles and verified model files retain their identities.

The module graph excludes aikit. goinfer's `internal/modelcore` is its own copy of aikit code and is linked into every build, so the release archives carry that source's MIT licence and notice under `licenses/aikit/`, and goinfer's `LICENSE`, `NOTICE` and `THIRD_PARTY_LICENSES.md` under `licenses/goinfer/`. The repository keeps those five files under `licenses/`, and `TestThirdPartyNotices` compares them with the pinned module.

### The removed bge-small adapter

Until v0.5.0 package `internal/embed/bge` held the release row `bge-small-en-v1.5` (384 dimensions, cut-off 0.65, a 512-token input) on a BERT encoder copied from aikit v1.79.1, with workers that ran one forward a core. The owner removed the model on 2026-11-26 (architecture decision 27) and the package was deleted. What remains of it:

- **The name.** `semantic.Build` answers `bge-small-en-v1.5` with `invalid_config` and a message of its own: `the model "bge-small-en-v1.5" was removed in pbrain v0.5.0: delete semantic.model from pbrain.toml to use the default, embeddinggemma-2-q8-0, or set one of: <the registry's names>; then run pbrain sync`. Every other unknown name keeps the message that lists the supported models. `semantic` may not import `brain`, so the default's name in that text is a literal; `TestReleaseModelSelection` holds it to `brain.DefaultModel`. The status and the search treat the name as any unsupported one.
- **Its indexes.** An index built with it has another fingerprint and another dimension. It is never scored: the source reports the section as not ready, the status marks it `built for another model`, and the next build replaces it, as for any change of model.
- **Its files in the cache.** Nothing removes `models/bge-small-en-v1.5-…` or its vectors under `vectors/` from the per-user cache. An older pbrain on the same machine may still use them.
- **The runtime audit.** `TestRuntimeEnvAndOutput` lived in that package and moved to `internal/buildcheck`.

### The EmbeddingGemma 2 adapter

Package `internal/embed/gemma2` is the only package of pbrain that imports goinfer. It is designed from `goinfer-embeddinggemma2.md` and `goinfer-architecture-review.md`; it is built and, since phase 27, in the release registry. Its Q8_0 row is the default model since v0.5.0 (architecture decision 27); the Google pack and UD-Q6_K_XL are rows a project opts into.

**The library** is the goinfer column of "Our libraries" above. It was first measured at the original's release v0.24.1.

**The model row** (`gemma2.Entry()`).

| Field | Value |
|---|---|
| `Name` | `embeddinggemma-2` |
| `Runtime` | `goinfer` |
| `Dim` | 768 |
| `Cutoff` | 0.72, set on the calibration half of the fixture (phase 27). On the held-out half, alone and with `--bm25`: 30 of 30 unanswerable pages empty, every regression case passed, and an expected id on the first page for 42 of 47 answerable questions against 16 for strict search, none lost. The record is `internal/semantic/calibrate/README.md`. Rule 3 of architecture §8.7 is shown on the built adapter on Linux (253 to 366 ms, `internal/cli/README.measure.md`); the Mac run is owed and tracked apart from the release (architecture §17). |
| Preparation version | `gemma2-1`: the prompts, pooling, cut rule and special-token rule below |

`Source`, from `https://huggingface.co/google/embeddinggemma-2/resolve/f9972ac22e696a105c482202646979e33e4b5a1e/<path>`:

| File | Size | SHA-256 |
|---|---|---|
| `model.safetensors` | 1,488,915,288 | `197a32965d4b1105faf060417baa899e193fb73cd401f42ec9295234d5553d79` |
| `tokenizer.json` | 32,170,510 | `4d777ef5bdc1aa36227abdfb77c3e49e7b9c892d16e1b6bda41c393504828be4` |
| `config.json` | 4,455 | `b8f1e9931b57fbc054acdb445c41765d55b0074c58d145fa82839941ad1b5bb3` |
| `config_sentence_transformers.json` | 1,565 | `031e56a498d33c349ab489a21885bcfe25b4fcba841149dc99e1e90d4a7c28f5` |

`Files`, the pack that `Derive` writes and every open verifies, 551,641,057 bytes in all. The procedure is fixed, so the same source gives the same four files, and their digests are pinned in the release:

| `File.Name` | Holds | Size | SHA-256 |
|---|---|---|---|
| `tokens.bf16` | the token-embedding table, 262,144 × 512 values, 16 bits each, as stored | 268,435,456 | `56ebbdbfd706c827f3fae1d97aa39135c7ddc0f1e6d9b12f8565f84bbfbda877` |
| `layers.safetensors` | the other 412 tensors of `language_model.*`, names and bytes unchanged, sorted by name, header padded to 8 bytes | 273,620,496 | `28a9678df941bf3eb7acca4e27ffd2fba6eef24039374d62148d705fc83b6227` |
| `tokenizer.bin` | the tokenizer's pack, from our goinfer's `tokenizer.BuildPack` | 9,581,663 | `45a7e7d5a1e84ad83ef469a8f6aecc3dd15f51b7c4866008e2d770142d96b709` |
| `config.json` | the source `config.json` and the two prompts used, as one small JSON | 3,442 | `65ef03947e006ea2a946fa323c88dd880ad6522d628723e81ddce04c2868db82` |

**The three changes to goinfer, in the order they are done.**

1. **A fast tokenizer: done** (merged into our `main` at `2e413ee1`; `goinfer-tokenizer-spike.md`). `tokenizer.BuildPack(tokenizerJSON []byte) ([]byte, error)` writes a binary form of the tokenizer, always the same bytes for the same input, in under a second; `tokenizer.LoadPack(pack []byte) (*Tokenizer, error)` loads it in 24 to 26 ms at four threads, against 441 to 502 ms for the JSON, opening no file and keeping about 17 MB. Token ids are identical on goinfer's 48 reference texts, on 10,000 random encodings in both special-token modes, and on 3,052 texts against the Hugging Face tokenizer. It serves the Gemma tokenizer family only and refuses others. The fallbacks planned for this task are not needed.
2. **A loader from bytes: done** (goinfer v0.2.0, `a759890b`). `embeddinggemma2.NewModel(cfg Config, tokens, layers []byte) (*Model, error)` builds the model from its configuration, the token table's bytes as stored and an in-memory safetensors file of the other tensors. It keeps the table as it is and widens one row per token, widens the other tensors once, across the processors, without the second copy the loader by path makes, and does not keep the layers bytes. `embeddinggemma2.LoadConfigBytes(raw)` reads the configuration from memory. `embeddinggemma2.NewEncoder(m, tok, prompts)` builds the encoder from an already built model, a tokenizer and the prompt table, because `LoadEncoder` reads all three from a folder. Measured on the real checkpoint at four threads: weights ready in 41 ms against 693 ms for the loader by path on the same warm machine; the 48 reference texts give the reference's ids and a worst cosine of 0.999999989, and 28 of them were compared with the loader by path, bit for bit equal.
3. **The pack.** `gemma2.Entry().Derive` is the model's procedure, pure and deterministic: from the four verified source files it returns the token table's bytes (`tokens.bf16`), `layers.safetensors` as specified in the table above, `tokenizer.bin` from our goinfer's writer, and `config.json`. `embed.Pack` checks those bytes against the pinned digests before it writes anything, so a derive that goes wrong leaves no pack.

**Opening and `Embed`.**

1. `Open` builds the tokenizer from `tokenizer.bin` with `LoadPack` and the model from the two weight files, side by side, then the encoder from both and the prompts; nothing is read from disk. It then follows `OpenOptions.Device` by the rules of "Devices": with the zero options nothing more happens and the encoder runs on the CPU; otherwise it attaches the selected accelerator with `Encoder.UseDevice`, uploads the weights, and reads the attached device to work out the budget. The backends this binary has are those whose goinfer module it imports: the CUDA module in a build with the `cuda` tag, the Vulkan module in a build with the `vulkan` tag on Linux amd64, and none in a build with neither, where `auto` gives the CPU with `unsupported`. The file that imports the Vulkan module is constrained to `linux && amd64 && vulkan`: goinfer's module compiles elsewhere and refuses every attach there, so on another target the tag links nothing and `vulkan` is a backend the binary lacks. Linux amd64 builds can link both tags, and the Linux amd64 GPU archive does from the release after v0.2.0; the v0.2.0 Linux GPU archives and the Linux arm64 GPU archive use `cuda` alone. The Darwin archives, the `_cpu` archives and a plain `go install` have neither CUDA nor Vulkan linked (architecture §16, decision 10). The adapter uses the model's device eligibility and pbrain's ordering and fallback rules ("Devices").
2. **Special tokens.** Body text must never produce a special token. The adapter encodes text with `EncodeSegments` and special-token parsing off, which a pack-loaded tokenizer serves as the JSON-loaded one does, and adds `<bos>` and `<eos>` itself. The published `Tokenize` reads a literal `<eos>` as the real token, as the reference does; the adapter does not use it.
3. **Prompts.** A query gets `task: search result | query: ` in front, a document `title: none | text: `, as the checkpoint defines them. The item's title and section titles stay inside the text, as pbrain lays them out; whether the title should go into the prompt's title slot is a question for the fixture, and changing it changes the preparation version.
4. **The cut.** The model takes 8,192 tokens and goinfer refuses more. The adapter never sends more than 2,048 including the prompt and the two special tokens, which bounds time and memory: a 2,294-token text took 5.8 s. A document is split at its first blank line into prefix and passage and cut by the same rule as the other adapter: the prefix only as far as needed and never below half the room, then the passage from its end.
5. **On the CPU** the texts of one call are embedded one after another with `Encoder.EmbedIDs`; goinfer runs its matrix products across the processors by itself and has no gate for forward passes side by side, so the adapter starts no workers. It is not `Windowed`: the build gives it 8 texts a call.
6. **On an accelerator**, a query and a standalone text use `EmbedIDs`. Several documents are tokenised once and packed in order. The packer tries increasing prefixes of up to 256 whole inputs and admits a valid singleton regardless of budget. It uses `Encoder.BatchMemory` on the attached encoder. A known CUDA estimate compares `AdditionalPeakBytes` against the budget, with equality allowed. Resident weights are already accounted for by the post-upload free reading. If the estimate is unknown, CUDA uses 136,000 bytes per position and Vulkan uses 107,528. Both GPU backends keep an 8,192-position aggregate ceiling. A memory estimate can further reduce the batch size. A rejected extension ends the prefix. An unsupported singleton or another estimator failure ends the embed. The adapter reprices after each forward and each certified OOM, as described in “Back-off”. On an accelerator the adapter is `Windowed` with a window of 256.
7. The vector is the mean over every token of the projected last hidden state, L2-normalised, 768 values. `Info` gives the device's id, its name when the runtime has one, and the budget: on Vulkan the id is the card's index and the name the one its listing gave, so the notice reads `vulkan:0 (AMD Radeon RX 9060 XT), 1024 MiB a batch`. `Close` closes the accelerator, if any, then drops the model.

What batching buys depends on the text: on passages of real length, 31 to 873 tokens, eight a call gave 7.7% over two, with the device busy 96% of the time, so the build's speed there is the kernel's (`pkieltyka/goinfer` issue #30); on texts of 64 tokens, 128 a call gave ten times one a call.

**The GGUF rows.** Issue #51 admitted Q8_0 on CPU and CUDA using goinfer v0.6.2. Issue #65 admitted UD-Q6_K_XL as a separate opt-in row and qualified Vulkan for both GGUFs on goinfer v0.6.6. The two rows are independent of the Google four-file pack. Both use goinfer at cutoff 0.72 and store and score all 768 normalized float32 coordinates. The normalized first 512 are an additional parity check. The role prompts, literal specials, 2,048-token cap and pooling use `gemma2-1`.

| Field | Q8_0 | UD-Q6_K_XL |
|---|---|---|
| Name | `embeddinggemma-2-q8-0` | `embeddinggemma-2-ud-q6-k-xl` |
| File | `embeddinggemma-2-Q8_0.gguf`, 309,855,520 bytes | `embeddinggemma-2-UD-Q6_K_XL.gguf`, 248,812,032 bytes |
| SHA-256 | `6f1bd4ac6c5df7444f9cca7ca36cafe6cfa34cd6f49fefb1e0b4be8143aed8bc` | `dc84f042bc3ffbe6122b3b38b2fb299d66f5f37b276c9d458db85d4a4f1e6ba3` |
| Model fingerprint | `9ebb0de1d547aee1abffec1288639ce596a06b0c32447d502782a474118cf691` | `16d5093f31a4399589c717804e04d2ad3fd70df5bd42b36bf60424dc4599e2f4` |
| Base index fingerprint at `ChunkerVersion = "1"` | `c346df62410f98848d85f7a3aa18e084fd8fab8138d97515f8045fcd3988be9d` | `8c964b1108dc4e42f65ea44837ce518911dc462ccb57d8fe1eba862a7e98bfa7` |
| Model cache folder | `embeddinggemma-2-q8-0-49b6c94be1aac068` | `embeddinggemma-2-ud-q6-k-xl-22e1f6123853c8cb` |

Both files are Unsloth conversions of Google’s `google/embeddinggemma-2`. The pinned model card declares Apache-2.0. Sync fetches each filename from `https://huggingface.co/unsloth/embeddinggemma-2-GGUF/resolve/031f0d4b35536f69ab3509d4893c923264fcf253/`. [Model card](https://huggingface.co/unsloth/embeddinggemma-2-GGUF/blob/031f0d4b35536f69ab3509d4893c923264fcf253/README.md)

- **Selection and opening.** `gemma2.GGUFEntries() []embed.Entry` returns both rows to the CLI and calibration tool. Production selects a model through `semantic.model`. The team’s real-model tests, calibration and measurements keep the existing `PBRAIN_ALLOW_GEMMA2=1` switch and require an explicit fixture directory for local tests. Production never consults this switch. Each row supplies one pinned file, empty `Source` and nil `Derive`. Sync uses pbrain’s downloader and `model_download` notice. After size and digest verification, the adapter uses `embeddinggemma2.LoadGGUFEncoderReaderAt` on the verified bytes and `Encoder.Tokenizer()` for the row’s vocabulary. The adapter releases the artifact on open and retains the existing cancellation and error paths.
- **Storage and switching models.** Each file lives under its folder in `<cache>/models/`. An offline `personal.toml` may set `model_path` to a directory containing the row’s exact file. Shared vectors live under `<cache>/vectors/<effective vector fingerprint>/<two-hex shard>/`. Personal vectors stay in their section index. A model change makes the old generation unavailable to search and catch-up. Sync builds each requested section for the selected model and reuses shared vectors only within its effective fingerprint.
- **Sync devices and precision.** Both GGUFs support CPU and NVIDIA CUDA sync and are eligible for Vulkan on Linux amd64 builds with that backend linked. Search always opens on CPU. UD-Q6_K_XL uses the base profile on all devices. Q8_0 has base, CUDA fp16 and two attached Metal profiles. Switching an existing Q8_0 base-profile index to Vulkan preserves its identity, while switching from CUDA fp16 to Vulkan rebuilds the requested sections under the base fingerprint. The #65 fixture matched CPU and Vulkan passage metadata, met the 0.999 cosine floor, and matched every vector bit across Vulkan batch layouts for both quants. [Device acceptance](../../../../internal/cli/README.measure.md#gguf-on-vulkan-device-acceptance-65)
- **Q8_0 admission record.** #47 fitted Q8_0’s cutoff independently on 30 calibration negatives. At 0.72 it matched the Google pack’s product retrieval on the frozen fixture, with 39/47 first, 42/47 top three and first page, and 30/30 held-out negatives empty. Standalone and combined gates passed for CPU- and CUDA-built indexes. The file digest, token preparation and retrieval rule are the same for #51. Carry forward that fit and use 0.72 for the release row. Rerun the frozen checks against the release wiring with the cutoff fixed; any regression is investigated before changing a threshold. On v0.6.2 the CPU and CUDA worst same-file cosines against llama.cpp were 0.9998506099 and 0.9998506078. The owner accepted the CPU rule 3 measurements and the single 603.6 ms outlier in architecture §8.7. [Original calibration](../../../../tools/phase28/evidence/gguf-47/README.md) · [v0.6.2 measurements, commit 0f64538](https://github.com/pkieltyka/pbrain/tree/0f64538/tools/phase28/evidence/gguf-47-v062)

**Historical comparison on v0.6.1 (#47).** Both quantizations then exceeded 600 ms. The following figures refer to the earlier loader.

  | | `embeddinggemma-2`, the Google pack | Q8_0 | UD-Q6_K_XL |
  |---|---|---|---|
  | Decode from verified bytes, median of three | 75.9 ms | 636.5 ms | 796.2 ms |
  | Rule 3 on the CPU, median of five: check, load, one query and close | 347.3 ms (336.9 to 354.3) | 933.8 ms (909.5 to 955.5) | 1,035.3 ms (1,022.8 to 1,048.6) |
  | Peak memory in those runs | 1,100.7 MiB | 1,400.9 MiB | 1,341.2 MiB |
  | With CUDA: first query; host peak; device peak | 1,009.4 ms; 1,804.4 MiB; 1,082 MiB | 1,263.4 ms; 1,589.2 MiB; 964 MiB | 1,425.5 ms; 1,531.7 MiB; 1,084 MiB |
  | Cosine with the Google pack's vector, 59 passages: lowest, median | | 0.999839, 0.999923 | 0.998660, 0.999423 |
  | The same over 1,223 passages on CUDA | | 0.999491, 0.999869 | 0.994587, 0.998899 |
  | Ten nearest passages shared with the Google pack; result pages shared | | 99.61%; 100.00% | 97.66%; 99.24% |
  | Rule 2, against vectors made from the same file by llama.cpp (`8d60873d`): lowest cosine on the CPU; with CUDA | | 0.99985061; 0.99985061 | 0.99910693; 0.99910695 |
  | Cut-off fitted on the calibration half | 0.72 | 0.72 | 0.72 |
  | Held-out half: expected id first, on the first page, of 47; empty pages of 30 | 39, 42; 30 | 39, 42; 30 | 39, 42; 30 |

  On the fixture a quantised row neither loses nor gains a question against the Google pack, alone or with `--bm25`, on an index built on the CPU or with CUDA, and both pass the two retrieval gates. What a row would save is the download, 310 MB or 249 MB against 1.52 GB. What it costs on v0.6.1 is 590 to 690 ms and 240 to 300 MiB at every search, the second being about the 256 MiB by which the widened token table is larger. To pass rule 3 the decode would have to fall to about 300 ms for Q8_0 and 360 ms for UD-Q6_K_XL; keeping the token table as stored and decoding the tensors across the cores are asked of goinfer in an issue filed from #47. The rule 3 figure of the Google pack here includes closing the model and is from another harness than the 286 ms of architecture §8.7. Rule 2 holds for both rows as models of their own: from the same prepared token ids, the 22 reference texts and the 59 passages, at 768 dimensions and at the normalised first 512, are each at 0.999 or better against llama.cpp on the same file, on the CPU and with CUDA, 162 comparisons a row and device. UD-Q6_K_XL passes with little room, 0.99911 at its worst passage against Q8_0's 0.99985. The cosines with the Google pack above are another matter: they measure how far the quantisation moves a vector and are not parity. At that pin rule 3 kept both rows out. That evaluation did not run: batches against single texts by bytes for these rows on CUDA; a build on CPUs; the time and memory of a first download; and anything on an AMD card. The records are in `internal/cli/README.measure.md` and `tools/phase28/evidence/gguf-47/`.

The first six 256-passage windows on the full corpus averaged about 0.95 passages/s on the RX 9060 XT with four allowed CPUs (issue #49). The matched slice and worker settings are in `internal/cli/README.measure.md`.

**Costs of the Google pack, to state wherever it is offered.** The default since v0.5.0 is the Q8_0 row of the same model, whose own costs are in architecture §17 ("The default model is the heavy one"). For the Google pack: a first index at 0.902 passages a second on four CPUs (measured on Linux in phase 28 on an Intel Core Ultra 7 255H, from the start of a run that was not finished; the estimate was about 440 tokens a second), which is about 95 minutes for 5,000 items of one passage each and a projected 15 hours for the 5,000-item, 50 MB brain of spec section 16 with its 48,511 passages, whose generated text is token-heavy; with an NVIDIA GPU and a build with the `cuda` tag, 1 hour 1 minute for that brain at 13.25 passages a second, measured on a Ryzen 9 7950X with an RTX PRO 6000 (issue #43; 1 hour 10 minutes before the build packed batches by token count); at that size, on that machine, up to 1,687 MiB of memory at a search, and while indexing with the GPU 4,318 MiB of the host's memory and 2,136 MiB of the card's (1.08 GiB at a one-item search and 1.88 GiB seen early in the unfinished run); a search at that size in 773 to 820 ms as a warm median on four cores; a one-time download of 1.52 GB for a 552 MB pack; and 9 right of 20 on our reworded questions, where the removed bge-small had 13.

**What a cut passage is evidence for.** Counting in code points cannot promise that a passage fits a model: dense or non-Latin text can take more tokens than the limit. The adapter then embeds the prefix and the beginning of the passage, and the tail is not represented in the vector. The hit still cites the whole span, and its snippet is the start of the passage, which is the part that was scored. This is a stated limit, not an error. The spike must show, for each model in the table, that with the longest permitted prefix in the densest script of its test set a word placed at the start of the passage still changes the vector.

## Dependencies

- `embed` uses `sys` and `errs`; `*fetch.Client` reaches it as a `Downloader`, passed by `cli`. `semantic` also uses `modernc.org/sqlite` and its `vec` package, in memory only. `embed/gemma2` imports the text encoder and tokenizer packages of `github.com/pkieltyka/goinfer`. `semantic` uses `embed`, `outline`, `item`, `search` (for the types of its seam), `gitx` (the cache test), `sys` and `errs`.
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
| `TestIndexRoundTrip` | A database published and opened again holds the same items, checks, spans, digests and vectors; the manifest names its size and CRC-32C. |
| `TestIndexRejects` | Each is `IndexUnreadable` and reaches SQLite never or harmlessly: a database file with one changed byte, a truncated or longer file, a manifest that is not JSON or is longer than 4,096 bytes, a wrong `format`, format 2 with its `sha256` field included, `meta` rows that contradict the manifest, an `item` row whose `crc` is text or negative or whose `size` is below 8, and a database whose checksum matches the manifest but which lacks the `passage` table. Another fingerprint is `IndexStale`. A vector of the wrong length cannot be inserted. Scenario 35. |
| `TestManifestIsNotTrusted` | A manifest whose `db` is `lock`, `manifest`, a name with a path separator, upper-case hexadecimal or the wrong length; one whose `size` is 0, negative, 2^31, 2^63 or not a number; one whose `crc32c` is malformed, or missing with a `sha256` in its place; one whose `size` differs from the file's own: nothing is opened under that name, nothing is allocated for that size, and nothing is removed. A build that repairs such an index still holds the same lock file afterwards: a second process cannot take it meanwhile. A database that would pass 2 GiB fails with `too_large` and the old index stays. |
| `TestIndexTouchesNoFile` | A search and a build run in a child process whose working folder is empty and read-only and whose temporary folder does not exist: both succeed, and `index/` holds only `lock`, `manifest` and one database afterwards. No journal, write-ahead log or shared-memory file appears at any moment of a build that is paused between its steps. |
| `TestScoringQuery` | Against scores computed in Go from the same vectors: the query returns the passages of the listed items within `1 - Cutoff + 0.000001`, also when more than 4,096 qualify, and `search`'s exact comparison then drops the one that lies in that margin; equal distances keep the smaller `ord`. An item not in `eligible` is never scored: with a malformed vector written to such an item, the query still succeeds and returns the others. Scenarios 33 and 34. |
| `TestDamagedPassageIsRepaired`, a scenario script with the test model | A published database, with a matching checksum, in which one passage of an unchanged item holds a vector of the wrong length. Four separate commands: `search --semantic` returns the lexical results and the other section's hits with `semantic_partial`; `sync --status` reports the section `unreadable`; `pbrain sync`, with no item changed, builds the index again; the same search then returns its results with no notice. Scenario 35. |
| `TestSourceSectionFailure` | Two sections, one of which fails while its rows are read (injected), and one whose scoring statement cannot be prepared: the failing section contributes no hit, all its items are in `Missing` once, the other section's hits are returned, and the rendered command shows them with `semantic_partial`. With both failing: `NoIndex` and the lexical results. Scenario 35. |
| `TestCoverage` | An entry covers an item only with the same path and content check; an edited body, an edited title, a renamed file and a deleted file are not covered; a front-matter-only edit still is. A body of another length, and one changed within four consecutive bytes at the same length, are not covered, never scored and counted in `Missing`. A body changed to other text with the same `ContentCheck`, the same length and the same CRC-32C, which the test constructs, is covered: the accepted limit of the check (architecture §17), pinned here so that a later change of the check has to meet it knowingly. A search with nothing to catch up, and `Status`, call `ContentCheck` and never `ContentDigest`: the test counts both through a package variable kept for it. A row whose span passes the body's end is not used. Scenario 35. |
| `TestBuildFirstAndNoOp` | A first build embeds every passage; a second embeds none, computes no `ContentDigest` and never opens the inference runtime, though it verifies the model's files. Scenarios 35 and 37. |
| `TestBuildRestoresModel` | With a complete index and the cache deleted, a build with no item changed downloads and verifies the model, embeds nothing, opens no runtime, and the next search with the source is ready. With a complete index and wrong weights of the right size at `model_path`, the same build fails with `invalid_config` and leaves the index as it was. An empty section asks for no model. Scenario 35. |
| `TestSemanticNoticeTable`, in `cli` | Every row of the notice table, in text and JSON, in both views; singular and plural forms; the status line for each model state and each `IndexState`. No message holds an item id, a path or an address. Scenarios 35 and 37. |
| `TestSyncDeviceNotices`, `TestCLIDevices`, in `cli` | `embedding_device` and each text of `cpu_fallback`, in text and JSON, at most once per command, and both when a run gives up its GPU; neither for a model with no GPU path, for a sync that embeds nothing or for a search; `device` in the JSON of a section that embedded and absent from one that did not. `--device` wins over `PBRAIN_DEVICE`; a malformed selector and a `PBRAIN_GPU_MEMORY` that is not a whole number of at least 1 are `invalid_config` before the scan; a well-formed selector that names no device fails a sync that has a passage to embed and not one that has none. `pbrain devices` runs from an empty directory, in text and JSON, and prints the CPU's line alone in a build with no accelerator; a device with a total and no free figure prints `<total> MiB`, and in JSON `total_bytes` with neither `free_bytes` nor `memory`. Issues #38 and #49. |
| `TestSyncDeviceOptions`, `TestMetalListingAndProfile`, `TestAttachedBudget`, `TestGemmaPacker`, `TestMetalBatchDispatch`, `TestMetalSingletonDispatch`, `TestMetalBatchAttachError`, `TestMetalAttachRoutes`, `TestMetalRuntimeRoute`, `TestMetalIndexProfiles` | The Metal opt-out reaches the adapter and other nonempty values are rejected. Inventory advertises the singleton. Both automatic and explicit Metal selection try the combined batch route by default. The opt-out requests singleton directly, and a refusal falls back to singleton Metal. Shared-memory budget calculation accepts an empty hardware ID. The packer preserves order and respects both caps and the budget, retries only certified OOM and stops on a failed singleton forward. Toggling batching preserves a combined generation. Issue #72. |
| `TestGemmaMetalBatchLayouts` | Base Apple M4 with the verified Q8_0 file. Compare singleton, packed, split and reversed batch-route embeddings bit for bit against the normal combined route at 768 and normalized 512 dimensions. Check the traced group shapes and at least 0.999 same-file CPU cosine at both widths. Issue #72. |
| `TestBuildIncremental` | After one added note: one passage embedded. After one edited paragraph of a long source: one embedded, the rest reused from the index. After a deleted item: `ItemsRemoved` 1 and its rows gone from the published database. An item is never half present: a build stopped between two transactions holds either all of an item's old passages or all of its new ones. A replacement whose insert fails part-way (a duplicate key forced by the test) is rolled back: the old item and all its old passages are still there, and nothing is published. Scenario 37. |
| `TestBuildRename` | A personal item moved to a new path with its text unchanged: the next build embeds nothing, opens no runtime and does not touch the per-user cache, and the published database has the new path and not the old. |
| `TestModelProvisionShared` | Two processes building different sections of different projects against one empty cache, one paused after it has opened the source files: both finish with the same verified pack, the source is downloaded once, and neither fails on files the other removed. A cancelled wait for the model lock ends with the context's error. The model lock is never taken before the index lock. |
| `TestBuildFromCache` | A second work tree of the same shared files embeds nothing and never opens the inference runtime. Personal passages are embedded and no cache file appears for them. Scenarios 33 and 37. |
| `TestBuildInterrupted` | A build cancelled after its first publication leaves a usable index; the next run embeds only what was not published, and for the shared section nothing that reached the cache. A process killed in the middle of one long personal source leaves that item out of the index, and the next run embeds it again from its start. Leftover temporary files and databases the manifest does not name are removed. Scenario 35. |
| CUDA writer profiles (#61) | Force policy, device and revision changes on unchanged notes. Assert cache, checkpoint and manifest isolation in both directions, CPU search on each profile, and zero CPU catch-up writes to fp16. Test per-device selector eligibility, the direct driver import boundary, typed attach errors, UUID mismatch and absent driver. Real CUDA runs qualify each row for cosine, CPU-query ranks and abstention, byte identity under reordered batches and certified splits, workspace and bounded sync throughput. See the #61 plan above. |
| `TestBuildFingerprintChange` | With `test-model-b` named, the old database is removed, every passage is embedded again, and no old vector is scored. Scenario 37. |
| `TestBuildRepairsDamage` | A damaged database file and an unparseable manifest each give `IndexUnreadable` in `Status`, and a build restores a usable index without touching an item. Scenario 35. |
| `TestBuildConcurrent` | Two processes building one section: the second waits, reports `Waiting` and then finds nothing to do. Staggered: process A scans, an item is added, process B builds and indexes it, then A gets the lock; A scans again under the lock, keeps B's entry and reports nothing removed. A search during a build that publishes reads one whole database, the old or the new. |
| `TestBuildEmptySection` | A section with no item and no index: zero result, no `.pbrain/` created, also for a personal folder that does not exist. A section whose last item was deleted: a database with no item. A section with no item whose index is unreadable or built for another model: the build publishes an empty database and the status reports it as in order. `Status` creates nothing in either. |
| `TestBuildErrors` | Semantic search off (an empty `ModelName`): `no_model`. Unknown name: `invalid_config` listing the names. A changed body is skipped. A vector of the wrong length fails with `io` and leaves the old index. Scenario 35. |
| `TestBuildUsesEmbedderWindow` | An embedder that is `Windowed` is given that many texts a call and one that is not is given 8. Issue #40. |
| `TestBuildWindowInvariance` | A fake embedder whose vector is a function of the text. Windows of 1, 8, 64 and 1,024 give the same database bytes and the same counts: on a first build; on a build with two items sharing a passage, which is embedded once; and, in a personal section, on the two incremental cases of "Reuse by digest", with an index whose stored vectors differ from what the fake would make. Every run starts from its own copy of the same index and the same cache. Issue #39. |
| `TestReuseTiers` | Each tier of "Reuse by digest" alone and each above the next: an own base row before another path's, a path outside `replace` before this run's answer, this run's answer before the cache; inside tiers 1 and 2 the lowest path and `ord`; a row of another path in `replace` is never taken, before or after that path is stored. Issue #39. |
| `TestBuildResumed` | On an index holding one vector for each digest, a run interrupted after its first publication and resumed ends byte for byte as an uninterrupted one. On an index holding two vectors for one digest, the two ends have the same item and passage rows, and the vector bytes are not compared. Neither compares `PassagesEmbedded` or `PassagesReused` across the interruption. Issue #39. |
| `TestBuildDeviceReports` | A fake embedder with a GPU `Info`: one `Progress` carries `Run` when the model opens, none after windows that leave `Info` unchanged, one more when it changes to the CPU with `memory`; `BuildResult.Device` is the last. A build that embeds nothing sends none and leaves `Device` empty. `Open` failing with `invalid_config` ends the build with it; with `ErrAccelerator`, `io`. An embedder whose `Close` fails with `ErrAccelerator` after a successful run: the counts are returned with no error. Issue #38. |
| Managed model verification (#59) | Check all release pins from real verified files. Exercise sealed hit, unsealed fallback, unsupported FS, wrong sealed digest, same-size mutation/replacement, descriptor stability after rename, sealed-cache repair and strict status/sync. Five fresh production-path Q8 rule-3 samples on CPUs 0–3 must all be under 600 ms with a median gain near the measured hash time. Compare the Google control. |
| `TestModelIdentityBuild` | Wrong weights of the right size, at `model_path` and in the cache, and a missing companion file of a two-file model: `Build` fails (`invalid_config`, or a new download for the cache) and publishes no database; a search returns its lexical results with not ready and never calls `Registry.Open`. A file replaced by name after `Verify` returned, and a file overwritten in place through the same open file after `Verify` returned, change nothing: the test registry hashes the bytes it is given and they are the pinned ones. Two models with one name and different files use two folders. A valid offline two-file folder builds with no request. Scenario 35. |
| `TestContentDigestFraming` | A title holding a zero byte, and a body starting with one, never collide with another title and body pair; moving bytes between the end of the title and the start of the body changes the digest. For `ContentCheck` the same move changes the CRC while the size stays, and one added byte changes the size. |
| `TestIndexFormatChange` | A section holding an index of format 2 as v0.1.1 published it (a manifest with `"format": 2` and `sha256`, and its database file), written by a test helper: `Status` reports `IndexUnreadable` with nothing indexed, and a search answers `NoIndex` with the lexical results. The build then publishes a format 3 index and removes the old database file. With the cache holding the shared vectors, the shared build embeds nothing and never opens the inference runtime; the personal build embeds every passage. Scenario 37. |
| `TestModelFindVerifyDownload` | Files found at `model_path` and in the cache; a wrong size or digest at `model_path` is `invalid_config`; a download with a wrong digest, a short body or a body past its size fails with `fetch_failed` and leaves no file; a second process finds the first one's file. No request is made when `model_path` is set. Scenario 35. |
| `TestCachePlacement` | Accepted: a folder outside any work tree; an ignored folder inside one, also when the home folder is a work tree. Refused: inside the brain, inside the personal section, in a work tree that does not ignore it, an ignored folder with a force-added file, a missing child of each, and a link to each. Refused for a nested repository: a cache outside Git whose `vectors` folder, fingerprint folder or one two-character folder holds `.git`, as a folder and as a file, and the same inside an ignored cache within a larger work tree; the build stops before writing into that folder. Nothing is created on refusal, and a search without `--semantic` is unaffected in every case. Scenario 35. |
| `TestVectorCache` | A short file, a wrong checksum and a vector that is not of unit length are removed and counted absent; two writers of one vector leave one valid file. |
| `TestCatchUp` | With a built index and the test model. One added note: `CatchUp` computes one `ContentDigest`, embeds one passage, publishes, and `Score` returns the note with `Missing` 0; a build run afterwards embeds nothing and reuses nothing. An edited item: replaced, and never scored for its old text. A renamed item: nothing embedded, the entry under the new path, the old row left in the database. A front-matter-only edit: no lock taken and nothing written. Nothing uncovered: no lock taken, the model task not waited for by `CatchUp`, no file touched. Shared section: the new vector is in the cache; a second work tree's search reuses it and opens the runtime for the query only. Personal section: the item is in the personal index, and no cache file appears. Scenario 42. |
| `TestCatchUpBounds` | 21 uncovered items: no `ContentDigest` computed, nothing embedded, no lock taken, nothing written, `Missing` 21. 20 items of one passage: all caught up. 3 items needing 21 passages: nothing written. 5 items of which reuse from the cache leaves 20 passages: caught up. With the injected clock passing `catchUpBudget` after the first of three items: one is published and scored, `Missing` 2, and no embedding call starts after the bound. The counts are over both sections of a personal view. Scenario 42. |
| `TestCatchUpLock` | The index lock held by another process: `CatchUp` returns without waiting, embeds nothing and writes nothing, and the search prints `semantic_partial`. A build that publishes between the search's index read and its lock (paused by the test): the manifest differs, nothing is written, and the build's database is still the one in force. Two searches at once: one catches up, the other answers without the item or with it, never with a half-written index. A build started while a search holds the lock waits and then finds nothing to do. `TryLock` failing with a file-system error is treated as held. Scenario 42. |
| `TestCatchUpNeverCreates` | No index, an index of another fingerprint, an unreadable index, format 2, a missing model, an exposed cache and `model = "none"`: after an add, a search writes no file, creates no folder and takes no lock, and its notice is the one of scenario 35. With `model_path` and no cache folder: the item is caught up and no cache folder appears. In a section folder that cannot be written: the search succeeds with `semantic_partial`. A cache folder whose `vectors` folder holds `.git`: nothing is written there and the item stays uncovered. Scenarios 35 and 42. |
| `TestCatchUpFailures` | Each leaves the search successful and the previous index in force: an embedder that fails, one that returns a vector of the wrong length, a publication whose rename fails, a cancelled context (the context's error, nothing published). After a failed publication the item is still scored in that search. Scenario 42. |
| `TestSourceLifecycle` | `Start` returns before the model is open and the scan can run meanwhile; `Close` without `Attach`, after a failed scan and after `Score` each cancels the task and closes the embedder once and only if it was opened; a cancelled context ends `Score` with the context's error. |
| `TestSearchOpensCPU` | A registry that records `OpenOptions`: `Start`, `Score` and `CatchUp`, with `Options.Device` set to a GPU, open with the zero options every time, and `Build` with an empty `Options.Device` opens with `auto`. An embedder that fails with `ErrAccelerator` at open or at the query leaves the search with its lexical results and `ModelFailed`; one that fails so in a catch-up leaves the search successful with `semantic_partial` for the uncovered items and the covered ones scored, as `TestCatchUpFailures` has it for any failure. With the real model in a build with the `cuda` tag and a GPU present: check, load and one query inside 600 ms. Issue #37. |
| `TestCLISemanticCPUClosePreservesResults`, `TestCLISyncCPUClosePreservesResults` (package `cli`) | An embedder whose `Close` returns an ordinary error, called once in each case. `search --semantic` prints the same output as with a healthy embedder, its semantic result included, writes nothing to standard error and exits 0. `sync` prints its counts and exits 0, and `sync --status` then finds every item indexed. Spec 7.1 and 9.4. |
| `TestSourceBadQueryVector` | A runtime that returns a query vector of the wrong dimension, with a value that is not a number, or not of unit length: not ready with `ModelFailed`, no hit, lexical results kept. Scenario 35. |
| `TestSourceNotReady` | Each row of the check table gives its reason and opens no model; no file is written and no request made in any of them. Scenario 35. |
| `TestSourceScores` | The best passage per item, its span, ties to the earlier passage; only eligible covered items are scored; the query is embedded once; results are identical across runs. Scenarios 33 and 34. |
| In `TestCoverage` and `TestSourceSectionFailure`; the shared-view case in the script of scenario 35 | A usable shared index and no personal one: shared items scored, `Missing` equals the personal items; an uncovered edited item is never scored. In the shared view `Missing` counts shared items only: `cli` hands the source no personal folder there, so the script proves it on the command, where the shared output is the same before and after an unindexed personal item and `--personal` then gives the one-item `semantic_partial` notice. Scenario 35. |
| `TestStatus` | Counts per section and each `IndexState`; no model opened; correct with semantic search off. Scenarios 35 and 37. |
| `TestGemmaRuntimeQuiet` | With the verified Q8_0 file present, in a child process whose standard output and standard error are pipes the test reads: opening the model and embedding a normal query, one holding `<eos>` and one over-long text print nothing on either. |
| `TestStrayOutputDiscarded`, in `cmd/pbrain` | In a child process built with `cmd/pbrain`'s start-up code: after it has run, `fmt.Printf` and a write to the `os.Stderr` variable put no byte on either real pipe, while writes through `Env.Stdout` and `Env.Stderr` arrive whole. This covers a library's failure-path print without having to cause the failure. |
| `TestRuntimeEnvAndOutput`, in `internal/buildcheck` | Source audit of `cmd/pbrain`'s actual `go list -deps` graph for every tagged and untagged GoReleaser build. It inspects selected non-test Go files from every linked non-standard-library dependency module. The Darwin arm64 Metal graph adds the nested Metal module, with pinned dormant decoder sites and purego fatal bootstrap diagnostics. It verifies that the runtime packages are present and that aikit is absent from the release graph. It flags explicit environment readers, directory and account helpers, process and proxy helpers with implicit environment access, and direct output references. It also checks function references used as callbacks, such as `os.Getenv` passed to `os.Expand`. Each reviewed dormant read, platform exception or output reference is pinned by module version, file, enclosing scope, symbol and occurrence limit per build, with the exact argument for an environment lookup. The audit fails for new matching sites or stale startup entries. [goinfer #70](https://github.com/pkieltyka/goinfer/issues/70) was fixed in v0.6.5, so the startup list is empty. A regression test checks that a stale entry still fails. |
| `TestThirdPartyNotices`, in `internal/buildcheck` | Each file of `licenses/goinfer` equals the file of that name in the pinned goinfer module, each file of `licenses/aikit` equals the one in its `internal/modelcore`, and neither folder holds anything else. |
| `TestReleaseBinaryOutput`, in `cmd/pbrain` | The built release binary of every target, started as a fresh process: `--version`, a lexical `search`, `search --semantic` and `sync` each leave exactly the command's output on the process's real standard output pipe. The `sync` case embeds several passages and the search case runs after it. The calibration tool is run the same way. It runs over both Linux variants of the native architecture, the archive built with the GPU tags and the one named `_cpu`, each taken out of its archive (issue #44). |
| `TestGemmaParity` | With the model present, on the pinned commit of our goinfer, two separate comparisons. (a) goinfer's own encoding path, with no cap and special-token text read as the reference reads it, against the stored raw references of all 22 texts, the literal `<eos>` document and the 2,294-token document included: identical token ids and cosine similarity of at least 0.999, also at 512 dimensions. This is rule 2 and it is never weakened. (b) The adapter, which treats marker text as ordinary text and stops at 2,048 tokens, against references made by sentence-transformers from input prepared and trimmed in exactly the same way: the same bounds. Each GGUF row is held to the same bounds against references made for its own file by an independent runtime, kept beside the others with the runtime's revision and command (issue #47); a row has no parity until those exist. |
| `TestGemmaTokenizerBinary` | In pbrain, on the pinned commit: `BuildPack` of the pinned `tokenizer.json` gives the pinned digest; the pack-loaded tokenizer gives the stored reference ids of the 22 texts. Our goinfer carries the wider checks (its unit tests, its fuzz target and its real-checkpoint test). Load time is recorded by the benchmark, with 100 ms as the release threshold; the spike measured 24 to 26 ms. |
| `TestGemmaBuildsFromBytes` | The pack folder is deleted after `Verify` and before `Open`: the embedder opens and embeds. Vectors equal those of the published loader on the same checkpoint. |
| `TestGemmaSpecialTokensAndCut` | A body holding `<eos>`, `<bos>` and `<start_of_turn>` yields no special id between the first and the last token; the same body without them differs only by those pieces. No call passes 2,048 tokens; a long prefix with a short passage keeps the passage whole; a long passage is cut from its end; a 9,000-token text is cut, not refused. |
| `TestDeviceSelection` | The rules of "Devices" as functions of a listing and of which attaches succeed, with no model: each selector; the automatic order on a dedicated card with a live figure, a shared card reporting a larger figure and one of unknown kind, which come out dedicated, unknown, shared; a device dropped only by a live figure under the weights; each `Fallback`; a selector naming nothing listed is `invalid_config` and a listed device that fails to attach wraps `ErrAccelerator`; a model with no GPU path opens on the CPU for every selector, a named GPU included, and makes no listing; a device is found by its `ID` and by its `Persistent`, and a bare id without its backend names nothing. In a build with the `cuda` tag on a machine with no driver, which is CI: the listing holds no GPU, `auto` gives the CPU with `absent`, and `cuda` is `invalid_config`. A device with no free figure comes after every device with one, whatever its kind: a dedicated card with no figure is tried after a card of unknown kind with a live one, and is used when that one does not attach. A backend's bare name gives its first device in that order. A device is attached by its persistent id whatever selector named it, and an attached id other than the one asked for wraps `ErrAccelerator`. A row with no id is left out of the listing. A listing that fails with `open libvulkan.so.1`, or with `create instance` and a result of -9, -3, -6 or -7, lists nothing, and `auto` ends on the CPU with `absent`; with -1, -2, a number not in that list, no readable number or any other text, the backend is in `Listing.Failed`: `auto` ends with `failed` when nothing attached and reports nothing when another backend's device did; a selector that names that backend, bare, by index or by id, wraps `ErrAccelerator` and that error and is not `invalid_config`; a selector that matches a listed device of another backend is used; and `Registry.Devices` lists the rest with the CPU and reports no failure. The entry's `Open` receives a named device's `Persistent` when it has one and its `ID` otherwise. Issues #38 and #49. |
| `TestMemoryBudget` | The budget as a function of the figure, its kind and the override: live, estimated, unknown, a live zero, 400 MiB free (120 MiB, not raised), an override of 8 MiB (kept), an override over 6 GiB and one over the free figure. Issue #39. |
| `TestGemmaPacker` and `TestGemmaPackerErrors` | Ordered prefixes respect the 8,192-position and 256-input ceilings and the workspace budget. A valid singleton is admitted. Unknown estimates use the per-position fallback. Invalid shapes and estimator errors are surfaced. |
| `TestGemmaBatchSameBytes` | With the model present, a build with the `cuda` tag and a GPU: the 59 fixture passages embedded alone, at the default budget and at an override that allows one text a batch give equal bytes. Issue #39. |
| `TestVulkanAbsentTexts` | A source check on the pinned Vulkan module, made like `TestRuntimeEnvAndOutput`: each text that the adapter reads as a machine with no Vulkan is still in the module's sources: `open libvulkan.so.1`, the operation's name `create instance`, and the format that puts the result after it, `vulkan: %s: VkResult %d`. It fails when a new pin rewords one, before a machine without Vulkan starts to report `failed`. What the adapter makes of each result number is `TestDeviceSelection`'s. |
| `TestUnifiedDeviceListing` and `TestAttachedBudget` | Preserve usable rows and actionable partial errors, exercise the optional CUDA-only listing filter, attach by persistent id, and read the attached session's memory. A mismatched id or failed reading closes the candidate. Failed close ends the open. |
| `TestGemmaOOMSplit`, `TestGemmaOOMCanceledChild` and `TestGemmaOOMTerminalAndCPUFallback` | Exercise retry order and persistent limits, cancellation, named failure and the auto CPU handoff. |
| `TestGemmaCUDAForcedSplitSameBytes` | On the CUDA-qualified Google and Q8_0 files, compare singleton outputs with the default capped layout and an injected certified failure followed by real smaller forwards on the same encoder. All 59 vector bit patterns must match. |
| `TestBuildDeviceLost` and `TestGemmaAcceleratorDispatchBatchingAndCloseRetry` | A fake singleton or batch forward reports device loss once. Sync reports `io` with the device ID, preserves the cause and closes the embedder once. |
| `TestGemmaVulkan` | With the Google pack present, a build with the `vulkan` tag and a discrete AMD card: the listing holds the card with a UUID and a total; attaching by index and by UUID gives the same attached id; the 59 fixture passages embedded on the CPU and on the card agree at a cosine of 0.999 or better, each; embedded alone, at the default budget and at an override that allows one text a batch they give equal bytes; and the notice names the card. Run by hand for the Vulkan fingerprint gate. |
| `TestGemmaVulkanLayouts` (#65) | With both verified GGUFs and a named Vulkan UUID, compare all 59 passages to CPU at 768 dimensions and the normalized 512 prefix at cosine ≥0.999. Compare the singleton, packed, reversed, 17-input-split and 1 MiB-budget Vulkan layouts bit for bit within each model. |
| `TestGemmaGGUFRows` and `TestReleaseModelSelection` (#51, #65) | Without weights, check both GGUF rows' file pins, model and index fingerprints, cache folders, dimensions and cutoffs. Q8_0 and UD-Q6_K_XL resolve with the team switch unset. Status and search with a missing model fetch nothing. The registry lists the three models and Q8_0 is the default. The removed `bge-small-en-v1.5` is reported as not supported, and `pbrain sync` answers it with the removal message. |
| `TestGGUFDevicePolicy` and `TestGGUFNamedDeviceResolution` (#65) | Both rows use the Vulkan-enabled inventory. A named Vulkan selector resolves with a matching row and is refused before decode when absent. |
| `TestGGUFInvalidArtifact` and `TestReleaseQ8OfflineRefusal` | For both GGUF rows, a wrong download digest yields `fetch_failed`, and a malformed or cancelled open releases the artifact. Wrong bytes at the Q8_0 `model_path` yield `invalid_config` on sync. |
| `TestBuildFingerprintChange` and catch-up (#51) | Use small fixtures with distinct model identities and the same vector dimension. Switching Google→Q8 and back rejects the previous DB before search or catch-up and publishes a generation with the selected fingerprint on sync. Assert zero cross-model reuse for shared and personal sections and reuse of an already warmed shared namespace only for its own model. Personal vectors never enter that cache. |
| Q8 real-model acceptance (#51) | Run with the team opt-in and local verified Q8. Check literal special-token text, query/document IDs and long-input cuts. Check the 22 reference texts and 59 passages against the independent same-file reference on CPU and CUDA at 768 and the normalized 512-prefix (each cosine ≥0.999). Prove CPU/CUDA agreement and byte-identical CUDA singleton, packed, split and reversed/restored vectors. Replay the frozen retrieval gates and repeat the matched five fresh-process CPU first-query trials, recording every sample and the 600 ms margin. |
| `TestGemmaBackOff` | A fake accelerator that reports out of memory for a batch twice and then for one text alone: the positions tried fall strictly, eight texts of 200 tokens end in single calls and not in a loop, the vectors before the fall equal a run with no failure, and `Info` becomes the CPU with `memory` under `auto` while a named device fails with `ErrAccelerator`. Any other failure of a batch is `ErrAccelerator` at once. With the release of goinfer that reports the failure as such (issue #41). |
| `TestDeriveDeterministic` | Deriving twice from the same source gives the same bytes, which match the pinned digests; a source with one changed byte, and a result with one changed byte, are refused and nothing is written; the source files are removed only after the pack verifies, and the empty folder may remain; an interrupted run does not download the source twice; source files placed at `model_path` are not accepted and nothing is written there. |
| `TestCalibrate` | On a fixture with known scores: the cut-off from the calibration half only; the abstention gate fails when a held-out page shows a lexical result, and when a regression case shows any; the gain gate fails when a tighter page drops an expected id; a fixture without recorded halves or with too few questions is refused; the report has one row per arm of the wider comparison, and the two any-word arms score the same candidates; the partial tier's gates fail on a near-miss question that returns partial rows; maxima below a provisional cut-off are still collected; an exact match beyond the first page is excluded from the maxima; with 38 of 40 empty pages for each mode alone and 36 for both, the combined gate fails and the release check with it; a long-source question with no `evidence`, with two expected ids, or with a quotation found twice in its item, is refused; with one such question whose evidence is on the page, one that needs the read, one whose item is first and never shows it and one whose item is not retrieved, the report counts each once; with the expected item fifth on the page and the quotation not in its snippet, the reader opens the first result, the question is a miss of the second kind, and only the upper bound counts it; with a long body after the quotation, the size to the evidence is smaller than the size delivered by exactly that tail; in the guided-retry arm both searches are charged; at a budget where the text form of the page would hold a fifth result that the JSON form does not, an expected item in fifth place is not retrieved, and where the JSON body of the read ends before a quotation that the text form would have shown, the evidence is not visible, and every size is that of the JSON responses; no miss enters a size. Scenario 34. |
| `TestAdapterContract` | For each real model, where its files are present: reference vectors reproduced with cosine similarity of at least 0.999; an over-long text is cut, not refused; with the longest permitted prefix, a changed word at the start of the passage changes the vector; batch size does not change a vector; fingerprint and dimension match the table. Skipped with a message otherwise. |
| `TestPassageAlignment` | On the alignment item of architecture §14 (two sections with one title under different parents, a fenced line that looks like a heading, multibyte text): every span from `Passages` lies inside one section of `outline.Parse` and none starts at the fenced line; the two sections of the same title give different passages with different heading paths in their prefixes; for a hit on each, `SemanticHit.Start` is in the intended section, told apart by its place in the outline, and the body from `Start` begins with the passage text that was embedded. |
