package graph

import (
	"iter"
	"slices"

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/item"
)

const (
	none = -1
	many = -2
)

func (g *Graph) resolve(holder item.Section, key string) int {
	return g.lookup(key, holder, holder == item.Shared)
}

func (g *Graph) lookup(key string, section item.Section, onlySection bool) int {
	found := none
	for _, i := range g.byKey[key] {
		if onlySection && g.items[i].Section != section {
			continue
		}
		if found != none {
			return many
		}
		found = i
	}
	return found
}

func distinct(ids []string) iter.Seq2[string, string] {
	return func(yield func(id, key string) bool) {
		seen := make(map[string]struct{}, len(ids))
		for _, id := range ids {
			key := item.Fold(id)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			if !yield(id, key) {
				return
			}
		}
	}
}

func (g *Graph) finding(code string, it *item.Item, f Field, id string, why Reason) {
	g.facts = append(g.facts, LinkFinding{Code: code, Item: it, Field: f, Target: id, Reason: why})
}

func (g *Graph) missing(it *item.Item, field Field, key string) {
	code := "dangling_link"
	if it.Section == item.Shared && g.lookup(key, item.Personal, true) != none {
		code = "personal_reference"
	}
	g.finding(code, it, field, "", 0)
}

func replaceRule(new, old *item.Header) Reason {
	switch {
	case new.Standing() < old.Standing():
		return Standing
	case old.Kind == item.Decision && new.Kind != item.Decision:
		return DecisionKind
	case (new.Kind == item.Source) != (old.Kind == item.Source):
		return SourceKind
	default:
		return 0
	}
}

type replaceEdge struct {
	target int
	id     string
}

func (g *Graph) buildReplaces() {
	candidates := make([][]replaceEdge, len(g.items))
	for i, it := range g.items {
		for id, key := range distinct(it.Header.Replaces) {
			j := g.resolve(it.Section, key)
			switch j {
			case none:
				g.missing(it, FieldReplaces, key)
				continue
			case many:
				g.finding("replace_invalid", it, FieldReplaces, id, Ambiguous)
				continue
			case i:
				g.finding("replace_cycle", it, FieldReplaces, id, InCycle)
				continue
			}
			old := g.items[j]
			why := OtherSection
			if it.Section == old.Section {
				why = replaceRule(&it.Header, &old.Header)
			}
			if why != 0 {
				g.finding("replace_invalid", it, FieldReplaces, id, why)
				continue
			}
			candidates[i] = append(candidates[i], replaceEdge{target: j, id: id})
			g.out[i] = append(g.out[i], j)
		}
	}
	components := stronglyConnected(g.out)
	for i, edges := range candidates {
		g.out[i] = g.out[i][:0]
		for _, edge := range edges {
			if components[i] == components[edge.target] {
				g.finding("replace_cycle", g.items[i], FieldReplaces, edge.id, InCycle)
				continue
			}
			g.out[i] = append(g.out[i], edge.target)
			g.in[edge.target] = append(g.in[edge.target], i)
		}
	}
}

func (g *Graph) addStamps() {
	type stamp struct {
		from, to int
		stem     string
	}
	var stamps []stamp
	for i, it := range g.items {
		stem, ok := it.Header.LegacyStamp()
		if !ok {
			continue
		}
		from := g.lookup(item.Fold(stem), it.Section, true)
		var why Reason
		switch {
		case from == none:
			why = Missing
		case from == many:
			why = Ambiguous
		case len(g.in[i]) != 0:
			why = AlreadyLinked
		}
		if why != 0 {
			g.finding("legacy_stamp_ignored", it, FieldStatus, stem, why)
		} else {
			stamps = append(stamps, stamp{from, i, stem})
		}
	}
	if len(stamps) == 0 {
		return
	}
	// Append candidate edges for the SCC pass, then keep only accepted stamps.
	lengths := make([]int, len(g.out))
	for i, edges := range g.out {
		lengths[i] = len(edges)
	}
	for _, stamp := range stamps {
		g.out[stamp.from] = append(g.out[stamp.from], stamp.to)
	}
	components := stronglyConnected(g.out)
	for i := range g.out {
		g.out[i] = g.out[i][:lengths[i]]
	}
	for _, stamp := range stamps {
		if components[stamp.from] == components[stamp.to] {
			g.finding("legacy_stamp_ignored", g.items[stamp.to], FieldStatus, stamp.stem, InCycle)
			continue
		}
		g.out[stamp.from] = append(g.out[stamp.from], stamp.to)
		g.in[stamp.to] = append(g.in[stamp.to], stamp.from)
	}
}

func (g *Graph) buildRelates() {
	for i, it := range g.items {
		for _, key := range distinct(it.Header.Relates) {
			j := g.resolve(it.Section, key)
			switch j {
			case none:
				g.missing(it, FieldRelates, key)
			case many, i:
			default:
				g.rel[i] = append(g.rel[i], j)
				if len(g.in[i]) == 0 {
					g.relIn[j] = append(g.relIn[j], i)
				}
			}
		}
	}
	for _, pointers := range g.relIn {
		slices.SortFunc(pointers, func(a, b int) int { return newest(g.items[a], g.items[b]) })
	}
}

// CheckNewReplaces validates proposed links in the destination's view.
// The caller reserves the new id against existing links so publication cannot close a cycle.
// When a target is already replaced, returns its current successors with the error.
func (g *Graph) CheckNewReplaces(newSection item.Section, newHeader *item.Header, ids []string) ([]*item.Item, error) {
	var targets []*item.Item
	for _, id := range ids {
		i := g.resolve(newSection, item.Fold(id))
		switch i {
		case none:
			return nil, writeError("not_found", "replaces target not found")
		case many:
			return nil, writeError("ambiguous_id", "replaces target is ambiguous")
		}
		old := g.items[i]
		why := OtherSection
		if old.Section == newSection {
			why = replaceRule(newHeader, &old.Header)
		}
		if why != 0 {
			return nil, &LinkError{Target: id, Reason: ruleMessage(why)}
		}
		if !g.Current(old) {
			return g.CurrentSuccessors(old), &LinkError{Target: id, Reason: "target is already replaced"}
		}
		targets = append(targets, old)
	}
	return targets, nil
}

// LinkError refuses one proposed replaces link. It carries the target as the caller gave it and the
// reason, and its message names no id: the caller writes the id at its own boundary (spec 6.2). It
// unwraps to an invalid domain error.
type LinkError struct {
	Target string // the id as given
	Reason string // why the link is refused, e.g. "target has higher standing"
}

func (e *LinkError) Error() string { return "replaces target: " + e.Reason }
func (e *LinkError) Unwrap() error { return writeError("invalid", e.Error()) }

func writeError(code, message string) *errs.Error {
	return &errs.Error{Code: code, Message: message, Exit: 1}
}

func ruleMessage(reason Reason) string {
	switch reason {
	case OtherSection:
		return "target is in another section"
	case Standing:
		return "target has higher standing"
	case DecisionKind:
		return "only a decision replaces a decision"
	case SourceKind:
		return "source items replace source items only"
	default:
		return "invalid link"
	}
}
