// Package embed: exported surface from the pbrain specs; bodies are filled in by later phases.
package embed

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/pkieltyka/pbrain/internal/sys"
)

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
	Open  func(ctx context.Context, a *Artifact) (Embedder, error)
	// Derive is set only for a model with Source. It is pure: from the verified bytes of
	// Model.Source, in order, it returns the bytes of Model.Files, in order. It opens no file.
	Derive func(source [][]byte) ([][]byte, error)
}

// NewRegistry builds a Registry from entries. cmd/pbrain passes the release's entries; tests pass their own.
func NewRegistry(entries ...Entry) Registry { panic("not implemented") }

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
func FolderName(m Model) string { panic("not implemented") }

var ErrAbsent = errors.New("model files absent")
var ErrWrongModel = errors.New("not the named model")

// Find returns the folder that holds every file of m, by name and size. It is a cheap first
// test, never a licence to open: only an Artifact from Verify can be opened.
// With modelDir set (personal.toml model_path) only that folder is tried.
// Errors: ErrAbsent when a file is missing; ErrWrongModel when one has the wrong size.
func Find(files sys.Files, m Model, modelDir, cacheModels sys.Dir) (sys.Dir, error) {
	panic("not implemented")
}

// Verify reads every file of m in dir whole into memory, at most File.Size bytes plus one,
// hashes the bytes it read and returns them as an Artifact. Files are read and hashed
// concurrently, one worker per processor.
// Errors: ErrAbsent, ErrWrongModel (a size or digest that is not the pinned one), or a file-system error.
func Verify(files sys.Files, dir sys.Dir, m Model) (*Artifact, error) { panic("not implemented") }

// Download fetches the files of m that cacheModels/<FolderName(m)>/ lacks, and gives a file its name
// only when its size and SHA-256 are the pinned ones.
// Errors: *errs.Error fetch_failed (transfer, size or digest), io (the destination).
func Download(ctx context.Context, files sys.Files, dl Downloader, m Model, cacheModels sys.Dir) (sys.Dir, error) {
	panic("not implemented")
}

// Pack turns downloaded source files into the model's pack. It reads every file of m.Source
// in sourceDir into memory and checks size and SHA-256; calls reg.Derive; checks each result
// against the size and SHA-256 pinned in m.Files; and only then writes the files into packDir,
// each through a temporary file, a flush and LinkNew, followed by a sync of the folder.
// Nothing is written when any check fails. embed imports no adapter: the layout is the entry's.
// Errors: ErrAbsent, ErrWrongModel (source or result not the pinned ones), or a file-system error.
func Pack(files sys.Files, reg Registry, m Model, sourceDir, packDir sys.Dir) error {
	panic("not implemented")
}

// Downloader is satisfied by *fetch.Client (components/importer.md).
type Downloader interface {
	Download(ctx context.Context, rawURL string, dst io.Writer, maxBytes int64, idle time.Duration) (written int64, err error)
}
