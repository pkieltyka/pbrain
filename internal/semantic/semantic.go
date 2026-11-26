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
	Files          sys.Files
	Registry       embed.Registry
	ModelName      string // pbrain.toml semantic.model; "" when off
	Device         string // sync selector, empty for auto
	CUDAPrecision  string
	GPUMemory      int64 // batch budget override in bytes
	MetalSingleton bool
	ModelDir       sys.Dir // personal.toml model_path, opened by cli; nil when unset
	Cache          *Cache  // nil when no cache folder is known
	CacheErr       error   // ErrCacheExposed when the folder failed its test
	Rand           io.Reader
	Now            func() time.Time
	writer         func() embed.WriteProfile
}

type CacheOptions struct {
	Path      string   // absolute: PBRAIN_CACHE, else the user's cache folder plus "pbrain"
	Forbidden []string // resolved roots it may not lie in: the brain, the personal section
	Git       gitx.Runner
	Files     sys.Files
}

var ErrCacheExposed = errors.New("cache folder is exposed")

type BuildResult struct {
	Section          item.Section
	Items            int // items the index holds after the run
	ItemsRemoved     int // items gone from the section whose entries this run dropped
	PassagesEmbedded int
	PassagesReused   int    // taken from the index's own entries or from the cache
	Device           string // last device used, empty when nothing was embedded
}

type Progress struct {
	Section           item.Section
	Done, Total       int            // items to do
	Waiting           bool           // waiting for the index lock, held by a build or a search's catch-up
	Run               *embed.RunInfo // set at open and when the runtime changes
	PrecisionFallback string
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
