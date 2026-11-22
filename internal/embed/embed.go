// Package embed is pbrain's seam to an embedding model: the table of supported models, and
// finding, downloading, deriving and verifying a model's files (semantic.md, "The model on disk").
// It imports no adapter; cmd/pbrain builds the registry from the adapters' entries.
package embed

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type Role uint8

const (
	Query Role = iota
	Document
)

// OpenOptions selects the model runtime. The zero value runs on the CPU.
type OpenOptions struct {
	Device       string // "" or "cpu" for the CPU, "auto" for automatic selection
	MemoryBudget int64  // bytes for a device batch
}

// Fallback describes why a model with an accelerator path runs on the CPU.
type Fallback string

const (
	NoFallback        Fallback = ""
	FallbackAsked     Fallback = "asked"
	FallbackNoSupport Fallback = "unsupported"
	FallbackNoDevice  Fallback = "absent"
	FallbackFailed    Fallback = "failed"
	FallbackMemory    Fallback = "memory"
)

// RunInfo describes the runtime used by an opened embedder.
type RunInfo struct {
	Device   string // "cpu" or a device ID such as "cuda:0"
	Name     string
	Budget   int64 // bytes for a device batch
	Fallback Fallback
}

type Embedder interface {
	// Embed returns one vector per text, in order, each of Dim() values and of unit length.
	// A text longer than the model takes is cut at a token boundary, never refused.
	// Errors: the context's error, or a plain error for a runtime failure.
	Embed(ctx context.Context, role Role, texts []string) ([][]float32, error)
	Dim() int
	Fingerprint() string
	Info() RunInfo
	Close() error
}

// Windowed is an optional embedder capability declaring the maximum number of
// texts per Embed call during sync. A positive window stays fixed for the
// lifetime of the embedder.
type Windowed interface {
	Window() int
}

type File struct {
	Name   string // file name inside the model folder
	URL    string // https, pinned in the release
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
	Open  func(ctx context.Context, a *Artifact, o OpenOptions) (Embedder, error)
	// Devices lists the adapter's usable GPUs without opening a model. Nil means CPU only.
	Devices func() []DeviceInfo
	// Derive is set only for a model with Source. It is pure: from the verified bytes of
	// Model.Source, in order, it returns the bytes of Model.Files, in order. It opens no file.
	Derive func(source [][]byte) ([][]byte, error)
}

type Registry interface {
	Lookup(name string) (Model, bool)
	Names() []string // sorted
	// Open builds the model from a.Files alone. It reads nothing from disk. It takes the bytes over:
	// a.Files is nil when it returns, with or without an error, and the caller keeps no other reference to them.
	Open(ctx context.Context, a *Artifact, o OpenOptions) (Embedder, error)
	Devices() []DeviceInfo // GPUs in selection order, followed by the CPU
	// Derive calls the entry's Derive; an error for a model without Source.
	Derive(m Model, source [][]byte) ([][]byte, error)
}

type registry struct {
	entries map[string]Entry
	names   []string
}

// NewRegistry builds a Registry from entries. cmd/pbrain passes the release's entries; tests pass
// their own. Two entries with one name are a mistake in the table, and panic.
func NewRegistry(entries ...Entry) Registry {
	r := &registry{entries: map[string]Entry{}}
	for _, e := range entries {
		if _, dup := r.entries[e.Model.Name]; dup {
			panic("embed: two models named " + e.Model.Name)
		}
		r.entries[e.Model.Name] = e
		r.names = append(r.names, e.Model.Name)
	}
	slices.Sort(r.names)
	return r
}

func (r *registry) Lookup(name string) (Model, bool) {
	e, ok := r.entries[name]
	return e.Model, ok
}

func (r *registry) Names() []string { return slices.Clone(r.names) }

// entry returns the entry whose row is m: same name, same files and same fingerprint.
func (r *registry) entry(m Model) (Entry, error) {
	e, ok := r.entries[m.Name]
	if !ok || e.Model.Fingerprint != m.Fingerprint || FolderName(e.Model) != FolderName(m) {
		return Entry{}, fmt.Errorf("embed: %q is not a model of this release", m.Name)
	}
	return e, nil
}

func (r *registry) Open(ctx context.Context, a *Artifact, o OpenOptions) (Embedder, error) {
	if a == nil {
		return nil, errors.New("embed: no artifact")
	}
	defer func() { a.Files = nil }()
	e, err := r.entry(a.Model)
	if err != nil {
		return nil, err
	}
	if e.Open == nil {
		return nil, fmt.Errorf("embed: %q has no opener", a.Model.Name)
	}
	if len(a.Files) != len(e.Model.Files) {
		return nil, fmt.Errorf("embed: %d files for %q, which has %d", len(a.Files), a.Model.Name, len(e.Model.Files))
	}
	switch {
	case e.Devices == nil, o.Device == "", o.Device == "cpu", o.Device == "auto":
	default:
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		o.Device, err = namedDevice(e, o.Device)
		if err != nil {
			return nil, err
		}
	}
	return e.Open(ctx, a, o)
}

func (r *registry) Derive(m Model, source [][]byte) ([][]byte, error) {
	e, err := r.entry(m)
	if err != nil {
		return nil, err
	}
	if len(e.Model.Source) == 0 || e.Derive == nil {
		return nil, fmt.Errorf("embed: %q is not derived", m.Name)
	}
	return e.Derive(source)
}

// FolderName is m.Name, a hyphen, and the first 16 hex characters of the SHA-256 over each
// file's name and digest in table order: the model's folder under <cache>/models/. Each name is
// preceded by its length in bytes as 8 little-endian bytes, so no two tables hash alike.
func FolderName(m Model) string {
	h := sha256.New()
	for _, f := range m.Files {
		var n [8]byte
		binary.LittleEndian.PutUint64(n[:], uint64(len(f.Name)))
		h.Write(n[:])
		h.Write([]byte(f.Name))
		h.Write(f.SHA256[:])
	}
	return m.Name + "-" + hex.EncodeToString(h.Sum(nil))[:16]
}

var ErrAbsent = errors.New("model files absent")
var ErrWrongModel = errors.New("not the named model")

// ErrAccelerator marks an explicit hardware accelerator attach, forward, or close failure so
// callers fail loud rather than silently degrading to CPU or lexical fallback.
var ErrAccelerator = errors.New("accelerator failed")

func absent(name string) error { return fmt.Errorf("%w: %s is missing", ErrAbsent, name) }
func wrong(name string) error {
	return fmt.Errorf("%w: %s is not the pinned file", ErrWrongModel, name)
}

// Find returns the folder that holds every file of m, by name and size. It is a cheap first
// test, never a licence to open: only an Artifact from Verify can be opened.
// With modelDir set (personal.toml model_path) only that folder is tried, and it is what Find
// returns: the caller still owns it. Otherwise the folder is a new handle of
// cacheModels/<FolderName(m)>, which the caller closes; cacheModels may be nil.
// Errors: ErrAbsent when a file is missing; ErrWrongModel when one has the wrong size, or is not a
// regular file; a file-system error.
func Find(files sys.Files, m Model, modelDir, cacheModels sys.Dir) (sys.Dir, error) {
	if modelDir != nil {
		if err := present(files, modelDir, m.Files); err != nil {
			return nil, err
		}
		return modelDir, nil
	}
	if cacheModels == nil {
		return nil, absent(FolderName(m))
	}
	dir, err := cacheModels.Sub(FolderName(m), false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, absent(FolderName(m))
	}
	if err != nil {
		return nil, err
	}
	if err := present(files, dir, m.Files); err != nil {
		dir.Close()
		return nil, err
	}
	return dir, nil
}

// present checks every file of list by name, type and size. A file of the wrong size wins over a
// missing one: the folder then holds something that is not the model.
func present(files sys.Files, dir sys.Dir, list []File) error {
	var missing error
	for _, f := range list {
		info, err := files.Stat(dir, f.Name)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if missing == nil {
				missing = absent(f.Name)
			}
		case err != nil:
			return err
		case !info.Mode.IsRegular() || info.Size != f.Size:
			return wrong(f.Name)
		}
	}
	return missing
}

// Verify reads every file of m in dir whole into memory, at most File.Size bytes plus one,
// hashes the bytes it read and returns them as an Artifact. Files are read and hashed
// concurrently, one worker per processor.
// Errors: ErrAbsent, ErrWrongModel (a size or digest that is not the pinned one), or a file-system error.
func Verify(files sys.Files, dir sys.Dir, m Model) (*Artifact, error) {
	data, err := readPinned(files, dir, m.Files)
	if err != nil {
		return nil, err
	}
	return &Artifact{Model: m, Files: data}, nil
}

// readPinned reads each file of list from dir and checks its size and digest, with at most one
// worker per processor. The first error in table order is returned.
func readPinned(files sys.Files, dir sys.Dir, list []File) ([][]byte, error) {
	out := make([][]byte, len(list))
	errl := make([]error, len(list))
	next := make(chan int)
	var wg sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), len(list)) {
		wg.Go(func() {
			for i := range next {
				out[i], errl[i] = readOne(files, dir, list[i])
			}
		})
	}
	for i := range list {
		next <- i
	}
	close(next)
	wg.Wait()
	for _, err := range errl {
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// readOne reads exactly f.Size bytes of f.Name, after the open file's own size agrees, refuses a
// byte more, and compares the SHA-256 of the bytes it holds.
func readOne(files sys.Files, dir sys.Dir, f File) ([]byte, error) {
	if f.Size < 0 {
		return nil, wrong(f.Name)
	}
	r, err := files.OpenRegular(dir, f.Name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, absent(f.Name)
	}
	if errors.Is(err, sys.ErrUnsafe) {
		return nil, wrong(f.Name)
	}
	if err != nil {
		return nil, err
	}
	defer r.Close()
	info, err := r.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size != f.Size {
		return nil, wrong(f.Name)
	}
	data := make([]byte, f.Size+1)
	n, err := io.ReadFull(r, data)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if int64(n) != f.Size {
		return nil, wrong(f.Name)
	}
	data = data[:n:n]
	if sha256.Sum256(data) != f.SHA256 {
		return nil, wrong(f.Name)
	}
	return data, nil
}

// downloadIdle is the idle timeout of one model download (semantic.md, "The model on disk").
const downloadIdle = 30 * time.Second

// downloadSet is where Download puts m's files and which: the model folder and Files, or, for a
// model derived from Source, the folder <FolderName(m)>.source and the source files.
func downloadSet(m Model) (string, []File) {
	if len(m.Source) > 0 {
		return FolderName(m) + ".source", m.Source
	}
	return FolderName(m), m.Files
}

// Download fetches the files of m that cacheModels/<FolderName(m)>/ lacks, and gives a file its name
// only when its size and SHA-256 are the pinned ones. For a model with Source it fetches the
// source files into cacheModels/<FolderName(m)>.source/ instead, for Pack. It returns that folder;
// the caller closes it. A name that appeared meanwhile keeps its file; Verify judges it.
// Errors: the context's error; *errs.Error fetch_failed (transfer, size or digest), io (the destination).
func Download(ctx context.Context, files sys.Files, dl Downloader, m Model, cacheModels sys.Dir) (sys.Dir, error) {
	folder, list := downloadSet(m)
	dir, err := cacheModels.Sub(folder, true)
	if err != nil {
		return nil, ioErr()
	}
	for _, f := range list {
		if err := fetchOne(ctx, files, dl, dir, f); err != nil {
			dir.Close()
			return nil, err
		}
	}
	if err := dir.Sync(); err != nil {
		dir.Close()
		return nil, ioErr()
	}
	return dir, nil
}

func fetchOne(ctx context.Context, files sys.Files, dl Downloader, dir sys.Dir, f File) error {
	if _, err := files.Stat(dir, f.Name); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return ioErr()
	}
	tmp, w, err := dir.CreateTemp()
	if err != nil {
		return ioErr()
	}
	// After LinkNew the file keeps its other name; before it, nothing is left behind.
	defer dir.Remove(tmp) //nolint:errcheck // a leftover temporary file is removed by a later build
	h := sha256.New()
	n, err := dl.Download(ctx, f.URL, hashWriter{w, h}, f.Size, downloadIdle)
	if err != nil {
		w.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var e *errs.Error
		if errors.As(err, &e) && (e.Code == "io" || e.Code == "fetch_failed") {
			return e
		}
		return fetchFailed()
	}
	if n != f.Size || [32]byte(h.Sum(nil)) != f.SHA256 {
		w.Close()
		return fetchFailed()
	}
	if err := files.Flush(w); err != nil {
		return ioErr()
	}
	if err := dir.LinkNew(tmp, f.Name); err != nil && !errors.Is(err, fs.ErrExist) {
		return ioErr()
	}
	return nil
}

type hashWriter struct {
	w io.Writer
	h hash.Hash
}

func (hw hashWriter) Write(p []byte) (int, error) {
	n, err := hw.w.Write(p)
	hw.h.Write(p[:n])
	return n, err
}

// Pack turns downloaded source files into the model's pack. It reads every file of m.Source
// in sourceDir into memory and checks size and SHA-256; calls reg.Derive; checks each result
// against the size and SHA-256 pinned in m.Files; and only then writes the files into packDir,
// each through a temporary file, a flush and LinkNew, followed by a sync of the folder.
// Nothing is written when any check fails. embed imports no adapter: the layout is the entry's.
// A name already in packDir keeps its file; Verify judges it.
// Errors: ErrAbsent, ErrWrongModel (source or result not the pinned ones), or a file-system error.
func Pack(files sys.Files, reg Registry, m Model, sourceDir, packDir sys.Dir) error {
	if len(m.Source) == 0 {
		return fmt.Errorf("embed: %q is not derived", m.Name)
	}
	source, err := readPinned(files, sourceDir, m.Source)
	if err != nil {
		return err
	}
	pack, err := reg.Derive(m, source)
	if err != nil {
		return err
	}
	if len(pack) != len(m.Files) {
		return fmt.Errorf("%w: %d files derived for %d", ErrWrongModel, len(pack), len(m.Files))
	}
	for i, f := range m.Files {
		if int64(len(pack[i])) != f.Size || sha256.Sum256(pack[i]) != f.SHA256 {
			return wrong(f.Name)
		}
	}
	for i, f := range m.Files {
		if err := writeNew(files, packDir, f.Name, pack[i]); err != nil {
			return err
		}
	}
	return packDir.Sync()
}

// writeNew gives data the name name in dir through a flushed temporary file and LinkNew.
func writeNew(files sys.Files, dir sys.Dir, name string, data []byte) error {
	tmp, w, err := dir.CreateTemp()
	if err != nil {
		return err
	}
	defer dir.Remove(tmp) //nolint:errcheck // after LinkNew the file keeps its other name
	if _, err := w.Write(data); err != nil {
		w.Close()
		return err
	}
	if err := files.Flush(w); err != nil {
		return err
	}
	if err := dir.LinkNew(tmp, name); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return nil
}

func ioErr() error {
	return &errs.Error{Code: "io", Message: "the model cannot be stored in the cache", Exit: 1}
}

func fetchFailed() error {
	return &errs.Error{Code: "fetch_failed", Message: "the model could not be downloaded", Exit: 1}
}

// Downloader is satisfied by *fetch.Client (components/importer.md).
type Downloader interface {
	Download(ctx context.Context, rawURL string, dst io.Writer, maxBytes int64, idle time.Duration) (written int64, err error)
}
