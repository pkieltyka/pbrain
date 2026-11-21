package search

import (
	"cmp"
	"context"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/graph"
	"github.com/pkieltyka/pbrain/internal/item"
)

type Verdict uint8

const (
	Out Verdict = iota
	In
	OutsideWindow
)

type selection uint8

const (
	excluded selection = iota
	currentIn
	replacedIn
	currentOutside
)

func (f *Filters) selectItem(it *item.Item, current bool) selection {
	h := &it.Header
	if len(f.Kinds) > 0 && !slices.Contains(f.Kinds, h.Kind) || f.Status != nil && h.Status != *f.Status {
		return excluded
	}
	for _, tag := range f.Tags {
		if !slices.Contains(h.Tags, tag) {
			return excluded
		}
	}
	if f.Window.Set() && (!h.CreatedKnown || !f.Window.Contains(h.Created)) {
		if current {
			return currentOutside
		}
		return excluded
	}
	if current {
		return currentIn
	}
	return replacedIn
}

// Admit applies lifecycle and filters. OutsideWindow describes a current item excluded only by time.
func (f *Filters) Admit(it *item.Item, current bool) Verdict {
	switch f.selectItem(it, current) {
	case currentIn:
		return In
	case replacedIn:
		if f.All {
			return In
		}
	case currentOutside:
		return OutsideWindow
	}
	return Out
}

type match struct {
	item     *item.Item
	selected selection
	eligible bool
	class    int // zero unless all clauses matched
	mask     []fields
	tf       []int
	length   int
	err      error
}

type rankedHit struct {
	source *match
	hit    Hit
	score  float64 // partial ordering only
}

// Run evaluates exact matching and optional lexical broadening over the caller's view.
func Run(ctx context.Context, c Corpus, g *graph.Graph, req Request) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p := plan(req.Clauses)
	records := collect(c.Items(), g, &req.Filters)
	if err := matchAll(ctx, c, &p, records, req.BM25); err != nil {
		return nil, err
	}

	r := &Result{Offset: req.Offset}
	ranked := rankExact(r, records, req.Now)
	if req.BM25 {
		ranked = append(ranked, rankPartial(records, req.Clauses)...)
	}
	r.Total = len(ranked)
	if r.Total == 0 {
		r.Alone = alone(records, req)
	}
	if req.Offset >= r.Total {
		return r, nil
	}

	offset := max(req.Offset, 0)
	last := offset + min(req.Limit, r.Total-offset)
	m := p.matcher()
	for _, row := range ranked[offset:last] {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		h, err := pageHit(c, &m, row, req)
		if err != nil {
			return nil, err
		}
		r.Hits = append(r.Hits, h)
	}
	return r, nil
}

func collect(items []*item.Item, g *graph.Graph, f *Filters) []match {
	out := make([]match, 0, len(items))
	for _, it := range items {
		selected := f.selectItem(it, g.Current(it))
		if selected != excluded {
			out = append(out, match{
				item: it, selected: selected,
				eligible: selected == currentIn || selected == replacedIn && f.All,
			})
		}
	}
	return out
}

func matchAll(ctx context.Context, c Corpus, p *matchPlan, records []match, bm25 bool) error {
	var workers sync.WaitGroup
	var next atomic.Int64
	for range min(runtime.GOMAXPROCS(0), len(records)) {
		workers.Go(func() {
			m := p.matcher()
			for ctx.Err() == nil {
				i := int(next.Add(1)) - 1
				if i >= len(records) {
					return
				}
				records[i].err = m.record(c, &records[i], bm25)
			}
		})
	}
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	for i := range records {
		if records[i].err != nil {
			return records[i].err
		}
	}
	return nil
}

func rankExact(r *Result, records []match, now time.Time) []rankedHit {
	var current, replaced []rankedHit
	for i := range records {
		rec := &records[i]
		if rec.class == 0 {
			continue
		}
		isReplaced := rec.selected == replacedIn
		if isReplaced {
			r.Replaced++
		}
		if rec.selected == currentOutside {
			r.Outside++
		}
		if !rec.eligible {
			continue
		}
		row := rankedHit{source: rec, hit: Hit{Item: rec.item, Class: rec.class, Replaced: isReplaced}}
		if isReplaced {
			replaced = append(replaced, row)
		} else {
			current = append(current, row)
		}
	}
	order := func(a, b rankedHit) int {
		ai, bi := a.hit.Item, b.hit.Item
		ah, bh := &ai.Header, &bi.Header
		return cmp.Or(cmp.Compare(a.hit.Class, b.hit.Class),
			cmp.Compare(recency(now, ah), recency(now, bh)), cmp.Compare(bh.Standing(), ah.Standing()),
			newest(ai, bi))
	}
	slices.SortStableFunc(current, order)
	slices.SortStableFunc(replaced, order)
	promoteDecision(current)
	return append(current, replaced...)
}

func recency(now time.Time, h *item.Header) int {
	if !h.CreatedKnown {
		return 3
	}
	switch age := now.Sub(h.Created); {
	case age < 30*24*time.Hour:
		return 0
	case age < 180*24*time.Hour:
		return 1
	case age < 365*24*time.Hour:
		return 2
	default:
		return 3
	}
}

func newest(a, b *item.Item) int {
	if a.Header.CreatedKnown != b.Header.CreatedKnown {
		if a.Header.CreatedKnown {
			return -1
		}
		return 1
	}
	if a.Header.CreatedKnown {
		if order := b.Header.Created.Compare(a.Header.Created); order != 0 {
			return order
		}
	}
	return cmp.Or(cmp.Compare(a.Key, b.Key), cmp.Compare(a.Path, b.Path))
}

func promoteDecision(current []rankedHit) {
	if len(current) == 0 {
		return
	}
	best := current[0].hit.Class
	for i, row := range current {
		h := &row.hit.Item.Header
		if row.hit.Class != best {
			break
		}
		if h.Kind == item.Decision && h.Decided != "" {
			copy(current[1:i+1], current[:i])
			current[0] = row
			return
		}
	}
}

func alone(records []match, req Request) []ClauseCount {
	out := make([]ClauseCount, len(req.Clauses))
	for i, clause := range req.Clauses {
		out[i].Label = clause.Label
	}
	for _, rec := range records {
		if rec.eligible {
			for i, mask := range rec.mask {
				if mask != 0 {
					out[i].Count++
				}
			}
		}
	}
	return out
}

func bodyError() error {
	return &errs.Error{Code: "io", Message: "could not read item body", Exit: 1}
}
