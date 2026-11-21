package cli_test

import (
	"github.com/pkieltyka/pbrain/internal/search"
	"github.com/pkieltyka/pbrain/internal/semantic"
	"github.com/pkieltyka/pbrain/internal/store"
)

// The seams of architecture 3 and 8.5, held at compile time: the store's catalog is a search.Corpus,
// and the semantic package's source is a search.Semantic.
var (
	_ search.Corpus   = (*store.Catalog)(nil)
	_ search.Semantic = (*semantic.Source)(nil)
)
