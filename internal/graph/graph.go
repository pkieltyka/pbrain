// Package graph derives replacement, history and related links from one command's items.
package graph

import (
	"cmp"
	"slices"

	"github.com/pkieltyka/pbrain/internal/item"
)

// Graph is an immutable view of one catalog. Its items must remain unchanged.
type Graph struct {
	items  []*item.Item
	nodes  map[*item.Item]int
	byKey  map[string][]int
	out    [][]int // replacer to replaced item
	in     [][]int // replaced item to replacer
	succ   [][]int // reachable current items, including self for current items
	rel    [][]int
	relIn  [][]int
	group  []int
	groups [][]int
	facts  []LinkFinding
}

// Build resolves links in the holder's view, removes invalid edges and computes currentness.
func Build(items []*item.Item) *Graph {
	n := len(items)
	g := &Graph{
		items: slices.Clone(items), nodes: make(map[*item.Item]int, n), byKey: make(map[string][]int, n),
		out: make([][]int, n), in: make([][]int, n), succ: make([][]int, n),
		rel: make([][]int, n), relIn: make([][]int, n),
	}
	for i, it := range items {
		g.nodes[it] = i
		g.byKey[it.Key] = append(g.byKey[it.Key], i)
	}
	g.buildReplaces()
	g.addStamps()
	g.deriveReplacementFacts()
	g.buildRelates()
	slices.SortStableFunc(g.facts, func(a, b LinkFinding) int {
		return cmp.Or(cmp.Compare(a.Item.Section, b.Item.Section), cmp.Compare(a.Item.Path, b.Item.Path),
			cmp.Compare(a.Field, b.Field), cmp.Compare(a.Target, b.Target))
	})
	return g
}

func (g *Graph) Current(it *item.Item) bool {
	i, ok := g.nodes[it]
	return ok && len(g.in[i]) == 0
}

func (g *Graph) Replacement(it *item.Item) []*item.Item {
	if i, ok := g.nodes[it]; ok {
		return g.references(g.in[i])
	}
	return nil
}

func (g *Graph) CurrentSuccessors(it *item.Item) []*item.Item {
	if i, ok := g.nodes[it]; ok && len(g.in[i]) != 0 {
		return g.references(g.succ[i])
	}
	return nil
}

// Replaces returns what it replaces through valid links and accepted stamps.
func (g *Graph) Replaces(it *item.Item) []*item.Item {
	if i, ok := g.nodes[it]; ok {
		return g.references(g.out[i])
	}
	return nil
}

func (g *Graph) RelatesTo(it *item.Item) []*item.Item {
	if i, ok := g.nodes[it]; ok {
		return g.references(g.rel[i])
	}
	return nil
}

func (g *Graph) NewerRelated(it *item.Item) []*item.Item {
	if i, ok := g.nodes[it]; ok {
		return g.references(g.relIn[i])
	}
	return nil
}

func (g *Graph) references(nodes []int) []*item.Item {
	if len(nodes) == 0 {
		return nil
	}
	result := make([]*item.Item, len(nodes))
	for i, node := range nodes {
		result[i] = g.items[node]
	}
	return result
}

func (g *Graph) Findings() []LinkFinding {
	out := slices.Clone(g.facts)
	for i := range out {
		out[i].Successors = slices.Clone(out[i].Successors)
	}
	return out
}

type Field uint8

const (
	FieldReplaces Field = iota
	FieldRelates
	FieldStatus
)

type LinkFinding struct {
	Code       string
	Item       *item.Item
	Field      Field
	Target     string // empty for dangling_link and personal_reference
	Reason     Reason
	Successors []*item.Item // populated for replace_fork
}

type Reason uint8

const (
	Ambiguous Reason = iota + 1
	OtherSection
	Standing
	DecisionKind
	SourceKind
	AlreadyLinked
	Missing
	InCycle
)

// Shown returns an item's written ids that exist in the command's view.
func (g *Graph) Shown(it *item.Item, f Field) []string {
	if _, ok := g.nodes[it]; !ok {
		return nil
	}
	var ids []string
	switch f {
	case FieldReplaces:
		ids = it.Header.Replaces
	case FieldRelates:
		ids = it.Header.Relates
	default:
		return nil
	}
	var out []string
	for id, key := range distinct(ids) {
		if len(g.byKey[key]) != 0 {
			out = append(out, id)
		}
	}
	return out
}

// Unresolved counts distinct ids absent from the command's view, across both link fields.
func (g *Graph) Unresolved(it *item.Item) int {
	if _, ok := g.nodes[it]; !ok {
		return 0
	}
	missing := make(map[string]struct{})
	for _, ids := range [][]string{it.Header.Replaces, it.Header.Relates} {
		for _, id := range ids {
			key := item.Fold(id)
			if len(g.byKey[key]) == 0 {
				missing[key] = struct{}{}
			}
		}
	}
	return len(missing)
}

type Impact struct {
	CurrentAgain []*item.Item
	Dangling     []*item.Item
}

// Effect compares the same view before and after removing the target.
func Effect(items []*item.Item, target *item.Item) Impact {
	remaining := make([]*item.Item, 0, len(items))
	found := false
	for _, it := range items {
		if it == target {
			found = true
		} else {
			remaining = append(remaining, it)
		}
	}
	if !found {
		return Impact{}
	}
	before, after := Build(items), Build(remaining)
	targetNode := before.nodes[target]
	var impact Impact
	for _, it := range remaining {
		if !before.Current(it) && after.Current(it) {
			impact.CurrentAgain = append(impact.CurrentAgain, it)
		}
		if newlyDangling(it, targetNode, before, after) {
			impact.Dangling = append(impact.Dangling, it)
		}
	}
	slices.SortFunc(impact.CurrentAgain, oldest)
	slices.SortFunc(impact.Dangling, oldest)
	return impact
}

func newlyDangling(it *item.Item, target int, before, after *Graph) bool {
	for _, ids := range [][]string{it.Header.Replaces, it.Header.Relates} {
		for _, id := range ids {
			key := item.Fold(id)
			if before.resolve(it.Section, key) == target && after.resolve(it.Section, key) == none {
				return true
			}
		}
	}
	return false
}
