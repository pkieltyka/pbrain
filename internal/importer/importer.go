// Package importer: exported surface from the pbrain specs; bodies are filled in by later phases.
package importer

import (
	"github.com/pkieltyka/pbrain/internal/importer/fetch"
	"github.com/pkieltyka/pbrain/internal/item"
)

type Prepared struct {
	Origin              item.Origin
	Title, Source, Body string
	Original            []byte // exact converter input for file/url, including a valid zero-byte input
	SHA256              string // lowercase hex for file/url; empty for linked
}

func FromFile(name, source, title string, raw []byte) (Prepared, error)    { panic("not implemented") }
func FromPage(requestedURL, title string, p *fetch.Page) (Prepared, error) { panic("not implemented") }
func Link(rawURL, title string, summary *string) (Prepared, error)         { panic("not implemented") }
