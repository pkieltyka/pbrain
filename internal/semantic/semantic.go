// Package semantic: exported surface from the pbrain specs; bodies are filled in by later phases.
package semantic

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/gitx"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/search"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type SectionDir struct {
	Section item.Section
	Dir     sys.Dir // from Brain.OpenSection
	Root    string  // the section's absolute folder; item paths are made relative to it
}

// SectionInput is one section as one scan saw it.
type SectionInput struct {
	SectionDir
	Items []*item.Item                     // the valid items of this section
	Body  func(*item.Item) ([]byte, error) // the stored body as scanned: store.Catalog.Body
}

// ScanFunc scans one section afresh. cli implements it with store.Scan, bodies kept.
type ScanFunc func(ctx context.Context) (SectionInput, error)

type Options struct {
	Files     sys.Files
	Registry  embed.Registry
	ModelName string  // pbrain.toml semantic.model; "" when off
	ModelDir  sys.Dir // personal.toml model_path, opened by cli; nil when unset
	Cache     *Cache  // nil when no cache folder is known
	CacheErr  error   // ErrCacheExposed when the folder failed its test
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

// Start needs no scan and never fails. It runs the cheap checks and, when they pass, begins
// in the background to verify the model, open it and embed text as a query.
func Start(ctx context.Context, opt Options, text string, dirs []SectionDir) *Source {
	panic("not implemented")
}

// Attach gives the source the scanned items. Call it once, after the scan, before Score.
func (s *Source) Attach(secs []SectionInput) { panic("not implemented") }

// Score joins the background task and compares. text must be the text given to Start.
// It returns only the context's error.
func (s *Source) Score(ctx context.Context, text string, eligible []*item.Item) (search.Scores, error) {
	panic("not implemented")
}

// Close cancels the background task, waits for it and closes an opened embedder exactly once.
// It is safe to call when Attach or Score were never reached.
func (s *Source) Close() error { panic("not implemented") }

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

type SectionStatus struct {
	Section        item.Section
	Items, Indexed int
	State          IndexState
}

type StatusResult struct {
	Model        string // "" when none is named
	Supported    bool
	ModelPresent bool
	Sections     []SectionStatus
}

// Status loads no model and writes nothing. It returns only file-system errors (io).
func Status(ctx context.Context, opt Options, secs []SectionInput) (StatusResult, error) {
	panic("not implemented")
}

// Source is the search.Semantic implementation for one command (semantic.md).
type Source struct { /* unexported */
}
