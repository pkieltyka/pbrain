// Package check: exported surface from the pbrain specs; bodies are filled in by later phases.
package check

import (
	"time"

	"github.com/pkieltyka/pbrain/internal/graph"
	"github.com/pkieltyka/pbrain/internal/item"
)

type Skipped struct {
	Section            item.Section
	Path, Code, Reason string
	Line, Column       int
}
type Options struct {
	Personal    bool
	HasTags     bool
	Tags        map[string]string
	Now         time.Time
	DisplayPath func(item.Section, string) string
}
type Finding struct {
	Severity, Code, Path, Message string
	Field                         string
	Line, Column                  int
}

func Collect(items []*item.Item, skipped []Skipped, g *graph.Graph, opt Options) (findings []Finding, errorCount int) {
	panic("not implemented")
}
