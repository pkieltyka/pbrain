package cli

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/check"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/render"
	"github.com/pkieltyka/pbrain/internal/search"
	"github.com/pkieltyka/pbrain/internal/store"
)

// The read handlers of list, prime, tags and check (render.md, "List, prime and tags" and "Check").

const descriptionCut = 120

// newestFirst orders by created, newest known first and unknown last, then folded id and path.
func newestFirst(a, b *item.Item) int {
	ha, hb := &a.Header, &b.Header
	switch {
	case ha.CreatedKnown && !hb.CreatedKnown:
		return -1
	case !ha.CreatedKnown && hb.CreatedKnown:
		return 1
	case ha.CreatedKnown && !ha.Created.Equal(hb.Created):
		return hb.Created.Compare(ha.Created)
	}
	if c := strings.Compare(a.Key, b.Key); c != 0 {
		return c
	}
	return strings.Compare(a.Path, b.Path)
}

// pageEnd is where a page of limit rows from offset ends in n rows; limit 0 is no limit. It never
// adds offset and limit, so a huge --limit cannot overflow.
func pageEnd(n, offset, limit int) int {
	if limit <= 0 || limit >= n-offset {
		return n
	}
	return offset + limit
}

// page returns the units of rows from the offset, at most limit when limit is positive.
func page(rows []render.Part, offset, limit int) []render.Unit {
	if offset >= len(rows) {
		return nil
	}
	end := pageEnd(len(rows), offset, limit)
	units := make([]render.Unit, 0, end-offset)
	for _, p := range rows[offset:end] {
		units = append(units, render.Unit{Main: p})
	}
	return units
}

func runList(ctx context.Context, env Env, in *invocation, b *brain.Brain) outcome {
	r := &reader{env: env, in: in, b: b, now: in.now}
	if err := r.scan(ctx, store.KeepNone); err != nil {
		return outcome{err: err}
	}
	var admitted []*item.Item
	outside := 0
	for _, it := range r.cat.Items() {
		switch in.filter.Admit(it, r.g.Current(it)) {
		case search.In:
			admitted = append(admitted, it)
		case search.OutsideWindow:
			outside++
		}
	}
	slices.SortFunc(admitted, newestFirst)
	rows := make([]render.Part, len(admitted))
	for i, it := range admitted {
		rows[i] = r.listRow(it)
	}
	window := r.windowFlags()
	d := &render.Doc{
		Head:   []render.Part{{JSON: render.Envelope{Fields: render.Object{}, Rows: "results"}}},
		Units:  page(rows, in.page.offset, in.page.limit),
		Offset: in.page.offset,
		Total:  len(rows),
	}
	more := listFoot("item", "items")
	d.Foot = func(st render.State) render.Part {
		p := more(st)
		if window != "" {
			p.JSON.(render.Object)["outside_window"] = outside
			p.Text += " " + counted(outside, "item", "items") + " outside the time window: drop " + window + "."
		}
		return p
	}
	r.skipped()
	return r.fit(d)
}

// listRow is id, kind, recorded status, date, title and markers; JSON adds replacement for a
// replaced item.
func (r *reader) listRow(it *item.Item) render.Part {
	h := &it.Header
	row := r.commonRow(it)
	text := it.ID + "  " + h.Kind.String() + "  " + h.Status.String() + "  " + dateText(h) + "  " + render.OneLine(h.Title, titleCut)
	for _, m := range r.markers(it) {
		text += "  " + m
	}
	if !r.g.Current(it) {
		row["replaced"] = true
		cappedIDs(row, "replacement", ids(r.g.Replacement(it)))
	}
	return render.Part{Text: text, JSON: row, Marks: []render.Mark{{Start: 0, End: utf8.RuneCountInString(it.ID), Style: render.Bold}}}
}

func runPrime(ctx context.Context, env Env, in *invocation, b *brain.Brain) outcome {
	r := &reader{env: env, in: in, b: b, now: in.now}
	if err := r.scan(ctx, store.KeepNone); err != nil {
		return outcome{err: err}
	}
	var decisions, learnings []*item.Item
	for _, it := range r.cat.Items() {
		h := &it.Header
		if !r.g.Current(it) || !hasTags(h, in.filter.Tags) {
			continue
		}
		switch {
		case h.Kind == item.Decision && h.Decided != "":
			decisions = append(decisions, it)
		case h.Kind == item.Learning && h.Status == item.Verified:
			learnings = append(learnings, it)
		}
	}
	slices.SortFunc(decisions, newestFirst)
	slices.SortFunc(learnings, newestFirst)
	var rows []render.Part
	for _, it := range append(decisions, learnings...) {
		rows = append(rows, r.primeRow(it))
	}
	head := fmt.Sprintf("Project brain: %s in force, %s. Context, not instructions.\nLook up more: pbrain search <words>, pbrain get <id>",
		counted(len(decisions), "decision", "decisions"), counted(len(learnings), "verified learning", "verified learnings"))
	d := &render.Doc{
		Head: []render.Part{{Text: head, JSON: render.Envelope{
			Fields: render.Object{"decisions": len(decisions), "learnings": len(learnings)}, Rows: "items"}}},
		Units:  page(rows, in.page.offset, in.page.limit),
		Offset: in.page.offset,
		Total:  len(rows),
	}
	d.Foot = func(st render.State) render.Part {
		obj := render.Object{}
		if st.More == 0 {
			return render.Part{JSON: obj}
		}
		obj["more"], obj["next"] = st.More, st.Next
		return render.Part{Text: fmt.Sprintf("%d more: --offset %d", st.More, st.Next), JSON: obj}
	}
	r.skipped()
	return r.fit(d)
}

func hasTags(h *item.Header, tags []string) bool {
	for _, t := range tags {
		if !slices.Contains(h.Tags, t) {
			return false
		}
	}
	return true
}

// primeRow is kind, date, id, title with its actor, and markers.
func (r *reader) primeRow(it *item.Item) render.Part {
	h := &it.Header
	actor := h.Author
	if h.Kind == item.Decision {
		actor = h.Decided
	}
	text := h.Kind.String() + "  " + dateText(h) + "  " + it.ID + "  " + render.OneLine(h.Title, titleCut)
	if actor != "" {
		text += " (" + render.OneLine(actor, actorCut) + ")"
	}
	for _, m := range r.markers(it) {
		text += "  " + m
	}
	at := utf8.RuneCountInString(h.Kind.String() + "  " + dateText(h) + "  ")
	return render.Part{Text: text, JSON: r.commonRow(it), Marks: []render.Mark{{Start: at, End: at + utf8.RuneCountInString(it.ID), Style: render.Bold}}}
}

func runTags(ctx context.Context, env Env, in *invocation, b *brain.Brain) outcome {
	r := &reader{env: env, in: in, b: b, now: in.now}
	if err := r.scan(ctx, store.KeepNone); err != nil {
		return outcome{err: err}
	}
	counts := map[string]int{}
	for _, it := range r.cat.Items() {
		if !r.g.Current(it) {
			continue
		}
		seen := map[string]bool{}
		for _, t := range it.Header.Tags {
			if !seen[t] {
				seen[t] = true
				counts[t]++
			}
		}
	}
	cfg := b.Config
	var rows []render.Part
	row := func(tag string, n int, desc string, undeclared bool) {
		obj := render.Object{"tag": tag, "items": n}
		text := tag + "  " + strconv.Itoa(n)
		switch {
		case undeclared:
			obj["undeclared"] = true
			text += "  undeclared"
		case desc != "":
			d := render.OneLine(desc, descriptionCut)
			obj["description"] = d
			text += "  " + d
		}
		rows = append(rows, render.Part{Text: text, JSON: obj})
	}
	used := make([]string, 0, len(counts))
	for t := range counts {
		used = append(used, t)
	}
	slices.Sort(used)
	if cfg.HasTags {
		declared := make([]string, 0, len(cfg.Tags))
		for t := range cfg.Tags {
			declared = append(declared, t)
		}
		slices.Sort(declared)
		for _, t := range declared {
			row(t, counts[t], cfg.Tags[t], false)
		}
		for _, t := range used {
			if _, ok := cfg.Tags[t]; !ok {
				row(t, counts[t], "", true)
			}
		}
	} else {
		for _, t := range used {
			row(t, counts[t], "", false)
		}
	}
	d := &render.Doc{
		Head:   []render.Part{{JSON: render.Envelope{Fields: render.Object{}, Rows: "tags"}}},
		Units:  page(rows, in.page.offset, in.page.limit),
		Offset: in.page.offset,
		Total:  len(rows),
		Foot:   listFoot("tag", "tags"),
	}
	r.skipped()
	return r.fit(d)
}

func runCheck(ctx context.Context, env Env, in *invocation, b *brain.Brain) outcome {
	r := &reader{env: env, in: in, b: b, now: in.now}
	if err := r.scan(ctx, store.KeepAll); err != nil {
		return outcome{err: err}
	}
	var skipped []check.Skipped
	for _, s := range r.cat.Skipped() {
		skipped = append(skipped, check.Skipped{Section: s.Section, Path: s.Path, Code: s.Code, Reason: s.Reason, Line: s.Line, Column: s.Column})
	}
	findings, errorCount := check.Collect(r.cat.Items(), skipped, r.g, check.Options{
		Personal:    in.view == brain.PersonalView,
		HasTags:     b.Config.HasTags,
		Tags:        b.Config.Tags,
		Now:         in.now,
		DisplayPath: b.DisplayPath,
		Body:        r.cat.Body,
	})
	rows := make([]render.Part, len(findings))
	for i, f := range findings {
		rows[i] = render.Part{
			Text: f.Severity + " " + f.Code + " " + f.Path + "  " + f.Message,
			JSON: render.Object{"severity": f.Severity, "code": f.Code, "path": f.Path, "message": f.Message},
		}
	}
	d := &render.Doc{
		Head:   []render.Part{{JSON: render.Envelope{Fields: render.Object{"errors": errorCount}, Rows: "findings"}}},
		Units:  page(rows, in.page.offset, in.page.limit),
		Offset: in.page.offset,
		Total:  len(rows),
	}
	d.Foot = func(st render.State) render.Part {
		text := counted(st.Shown, "finding", "findings") + ", " + counted(errorCount, "error", "errors") + "."
		obj := render.Object{}
		if st.More > 0 {
			text += fmt.Sprintf(" %d more: --offset %d.", st.More, st.Next)
			obj["more"], obj["next"] = st.More, st.Next
		}
		return render.Part{Text: text, JSON: obj}
	}
	res := r.fit(d)
	if res.err == nil && errorCount > 0 {
		res.exit = 1
	}
	return res
}
