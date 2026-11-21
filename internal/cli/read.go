package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/graph"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/outline"
	"github.com/pkieltyka/pbrain/internal/render"
	"github.com/pkieltyka/pbrain/internal/search"
	"github.com/pkieltyka/pbrain/internal/secretscan"
	"github.com/pkieltyka/pbrain/internal/store"
)

// The read handlers of search and get (render.md, "Read commands"). They build a document in
// memory and fit it before anything reaches stdout.

const (
	titleCut       = 120
	actorCut       = 80
	rowSourceCut   = 80
	getSourceCut   = 200
	sectionCut     = 80
	aloneCut       = 40
	neighbourCut   = 160
	contextCap     = 600
	idListCap      = 10
	relatedCap     = 3
	snippetLength  = 200
	defaultGetBody = 6000
	maxHints       = 8
)

// reader holds what one read command works from: one brain, one scan and its graph, one instant.
type reader struct {
	env Env
	in  *invocation
	b   *brain.Brain
	cat *store.Catalog
	g   *graph.Graph
	now time.Time
}

func (r *reader) scan(ctx context.Context, keep store.BodyPolicy) error {
	cat, err := store.Scan(ctx, r.b, r.in.view, keep)
	if err != nil {
		return err
	}
	r.cat, r.g = cat, graph.Build(cat.Items())
	return nil
}

// skipped sends the skipped-files notice of the final scan (cli.md, "Skipped files").
func (r *reader) skipped() {
	n := len(r.cat.Skipped())
	if n == 0 {
		return
	}
	msg := fmt.Sprintf("%d %s skipped: run pbrain check", n, plural(n, "file", "files"))
	if r.in.view == brain.PersonalView {
		msg += " --personal"
	}
	emitNotice(r.env.Stderr, r.in.json, errs.Notice{Code: "skipped_files", Message: msg})
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func counted(n int, one, many string) string { return strconv.Itoa(n) + " " + plural(n, one, many) }

func dateText(h *item.Header) string {
	if !h.CreatedKnown {
		return "unknown"
	}
	return h.Created.UTC().Format(time.DateOnly)
}

func instant(t time.Time, known bool) string {
	if !known {
		return "unknown"
	}
	return t.UTC().Format(time.RFC3339)
}

func standingWord(h *item.Header) string {
	switch h.Standing() {
	case 3:
		return "decided"
	case 2:
		return "verified"
	default:
		return "provisional"
	}
}

// commonRow is the common JSON row of search, list, history and prime.
func (r *reader) commonRow(it *item.Item) render.Object {
	h := &it.Header
	row := render.Object{
		"id":      it.ID,
		"kind":    h.Kind.String(),
		"status":  h.Status.String(),
		"title":   render.OneLine(h.Title, titleCut),
		"created": instant(h.Created, h.CreatedKnown),
	}
	for key, v := range map[string]string{"author": h.Author, "decided": h.Decided, "verified": h.Verified} {
		if v != "" {
			row[key] = render.OneLine(v, actorCut)
		}
	}
	if h.Source != "" {
		row["source"] = render.OneLine(h.Source, rowSourceCut)
	}
	if h.ReviewDue(r.now) {
		row["review_due"] = true
	}
	if h.Linked() {
		row["linked"] = true
	}
	if it.Section == item.Personal {
		row["personal"] = true
	}
	return row
}

// markers are the trailing words of a row: linked, review due, replaced and personal, in that order.
func (r *reader) markers(it *item.Item) []string {
	var out []string
	if it.Header.Linked() {
		out = append(out, "linked")
	}
	if it.Header.ReviewDue(r.now) {
		out = append(out, "review due")
	}
	if !r.g.Current(it) {
		out = append(out, "replaced")
	}
	if it.Section == item.Personal {
		out = append(out, "personal")
	}
	return out
}

// titleLine is "<title> (<actor>; source: <source>)" by presence.
func titleLine(h *item.Header) string {
	var paren []string
	switch {
	case h.Kind == item.Decision && h.Decided != "":
		paren = append(paren, "decided by "+render.OneLine(h.Decided, actorCut))
	case h.Author != "":
		paren = append(paren, "by "+render.OneLine(h.Author, actorCut))
	}
	if h.Source != "" {
		paren = append(paren, "source: "+render.OneLine(h.Source, rowSourceCut))
	}
	t := render.OneLine(h.Title, titleCut)
	if len(paren) > 0 {
		t += " (" + strings.Join(paren, "; ") + ")"
	}
	return t
}

func ids(items []*item.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

// idList keeps the first ten ids; the text ends with "(+N more)" and JSON gains <field>_more.
func idList(row render.Object, field string, list []string) string {
	shown := list[:min(len(list), idListCap)]
	row[field] = shown
	text := strings.Join(shown, ", ")
	if more := len(list) - len(shown); more > 0 {
		row[field+"_more"] = more
		text += fmt.Sprintf(" (+%d more)", more)
	}
	return text
}

// related is a hit's related rows: RelatesTo, then NewerRelated, each item once, at most three.
func (r *reader) related(it *item.Item) []render.Part {
	var out []render.Part
	seen := map[*item.Item]bool{}
	add := func(link, label string, rels []*item.Item) {
		for _, rel := range rels {
			if len(out) == relatedCap || seen[rel] {
				continue
			}
			seen[rel] = true
			h := &rel.Header
			row := render.Object{"link": link, "id": rel.ID, "kind": h.Kind.String(), "standing": standingWord(h),
				"created": instant(h.Created, h.CreatedKnown), "title": render.OneLine(h.Title, titleCut)}
			fields := []string{rel.ID, h.Kind.String(), standingWord(h), dateText(h), render.OneLine(h.Title, titleCut)}
			if rel.Section == item.Personal {
				row["personal"] = true
				fields = append(fields, "personal")
			}
			if !r.g.Current(rel) {
				row["replaced"] = true
				fields = append(fields, "replaced")
			}
			if h.ReviewDue(r.now) {
				row["review_due"] = true
				fields = append(fields, "review due")
			}
			if h.Linked() {
				row["linked"] = true
				fields = append(fields, "linked")
			}
			out = append(out, render.Part{Text: "  " + label + ": " + strings.Join(fields, ", "), JSON: row})
		}
	}
	add("to", "relates to", r.g.RelatesTo(it))
	add("from", "newer related", r.g.NewerRelated(it))
	return out
}

// pathOf is the exact path of an item for search and get, or ok false when it cannot be printed.
func (r *reader) pathOf(it *item.Item) (string, json.RawMessage, bool) {
	return render.ExactPath(r.b.DisplayPath(it.Section, it.Path))
}

func runSearch(ctx context.Context, env Env, in *invocation, b *brain.Brain, features Features) outcome {
	r := &reader{env: env, in: in, b: b, now: in.now}
	if err := r.scan(ctx, store.KeepAll); err != nil {
		return outcome{err: err}
	}
	longBody := b.Config.Get.MaxChars
	if longBody <= 0 {
		longBody = defaultGetBody
	}
	req := search.Request{
		Clauses: in.clauses, Filters: in.filter, Limit: in.page.limit, Offset: in.page.offset,
		Now: in.now, Snippet: snippetLength, LongBody: longBody, Text: in.query,
		BM25: features.BM25 && in.boolean("bm25"), Context: in.context,
	}
	res, err := search.Run(ctx, r.cat, r.g, req)
	if err != nil {
		return outcome{err: err}
	}
	r.skipped()
	data, err := render.Fit(r.searchDoc(res), in.page.maxChars, in.format)
	if err != nil {
		return outcome{err: err}
	}
	return outcome{data: data}
}

func (r *reader) searchDoc(res *search.Result) *render.Doc {
	d := &render.Doc{
		Head:   []render.Part{{JSON: render.Envelope{Fields: render.Object{}, Rows: "results"}}},
		Offset: res.Offset,
		Total:  res.Total,
	}
	for _, h := range res.Hits {
		main := r.hitPart(h, nil)
		u := render.Unit{Main: main, Extras: r.related(h.Item)}
		if h.Snippet != nil && len(h.Snippet.Context) > 0 {
			u.Alts = r.contextAlts(h)
		}
		d.Units = append(d.Units, u)
	}
	window := r.windowFlags()
	all := r.in.filter.All
	d.Foot = func(st render.State) render.Part {
		text := counted(st.Shown, "result", "results") + "."
		obj := render.Object{}
		if st.More > 0 {
			text += fmt.Sprintf(" %d more: --offset %d.", st.More, st.Next)
			obj["more"], obj["next"] = st.More, st.Next
		}
		if res.Replaced > 0 {
			obj["replaced_matched"] = res.Replaced
			if all {
				text += " " + counted(res.Replaced, "replaced item", "replaced items") + " matched."
			} else {
				text += " " + counted(res.Replaced, "replaced item", "replaced items") + " also matched: --all."
			}
		}
		if window != "" {
			obj["outside_window"] = res.Outside
			text += " " + counted(res.Outside, "match", "matches") + " outside the time window: drop " + window + "."
		}
		return render.Part{Text: text, JSON: obj}
	}
	if res.Total == 0 && res.Offset == 0 {
		for _, c := range res.Alone[:min(len(res.Alone), maxHints)] {
			label := render.OneLine(c.Label, aloneCut)
			d.Hints = append(d.Hints, render.Part{Text: label + " " + strconv.Itoa(c.Count),
				JSON: render.Object{"clause": label, "count": c.Count}})
		}
	}
	return d
}

func (r *reader) windowFlags() string {
	var flags []string
	if r.in.has("since") {
		flags = append(flags, "--since")
	}
	if r.in.has("until") {
		flags = append(flags, "--until")
	}
	return strings.Join(flags, " ")
}

// snippetText flattens the whole stretch and adds "…" where the body goes on.
func snippetText(s *search.Snippet) string {
	t := render.OneLine(s.Text, 0)
	if s.Start > 0 {
		t = "…" + t
	}
	if s.End < s.BodyChars {
		t += "…"
	}
	return t
}

// hitPart is one search result; with block set, the snippet line is replaced by that context block.
func (r *reader) hitPart(h search.Hit, block []search.ContextLine) render.Part {
	it := h.Item
	hd := &it.Header
	row := r.commonRow(it)
	lay := it.Layout

	first := it.ID + "  " + hd.Kind.String() + "  " + hd.Status.String() + "  " + dateText(hd)
	switch {
	case h.Semantic:
		first += "  semantic"
		row["semantic"] = true
	case h.Partial:
		first += fmt.Sprintf("  partial %d/%d", h.Matched, h.Clauses)
		row["partial"], row["matched"], row["clauses"] = true, h.Matched, h.Clauses
	}
	for _, m := range r.markers(it) {
		first += "  " + m
	}
	marks := []render.Mark{{Start: 0, End: utf8.RuneCountInString(it.ID), Style: render.Bold}}
	lines := []string{first, "  " + titleLine(hd)}

	display, encoded, ok := r.pathOf(it)
	if ok {
		row["path"] = encoded
	} else {
		display = "?"
	}
	row["body_line"], row["body_chars"] = lay.BodyLine, lay.BodyChars
	row["file_lines"], row["file_chars"] = lay.FileLines, lay.FileChars
	row["occurrences"] = h.Occurrences
	occ := counted(h.Occurrences, "occurrence", "occurrences")
	loc := display
	var sizes string
	if s := h.Snippet; s != nil {
		loc += ":" + strconv.Itoa(s.Line)
		sizes = fmt.Sprintf("[body L%d, offset %d/%d; file %dL/%dC; %s]", lay.BodyLine, s.Start, lay.BodyChars, lay.FileLines, lay.FileChars, occ)
		row["snippet"] = snippetText(s)
		row["snippet_offset"], row["snippet_line"] = s.Start, s.Line
	} else {
		sizes = fmt.Sprintf("[body L%d, %dC; file %dL/%dC; %s]", lay.BodyLine, lay.BodyChars, lay.FileLines, lay.FileChars, occ)
	}
	locLine := "  " + loc + "  " + sizes
	if h.Section != "" {
		sec := render.OneLine(h.Section, sectionCut)
		locLine += " [in " + sec + "]"
		row["section"] = sec
	}
	if ok {
		at := runesIn(lines) + 2
		marks = append(marks, render.Mark{Start: at, End: at + utf8.RuneCountInString(loc), Style: render.Cyan})
	}
	lines = append(lines, locLine)
	if s := h.Snippet; s != nil {
		if block == nil {
			lines = append(lines, "  "+snippetText(s))
		} else {
			var ctx []render.Object
			for _, l := range block {
				t := contextText(l)
				line := "    " + strconv.Itoa(l.Line) + ":"
				obj := render.Object{"line": l.Line}
				if t != "" {
					line += " " + t
					obj["text"] = t
				}
				lines = append(lines, line)
				ctx = append(ctx, obj)
			}
			row["context"] = ctx
		}
	}
	if h.Replaced {
		row["replaced"] = true
		repl := render.Object{}
		text := idList(repl, "replacement", ids(r.g.Replacement(it)))
		row["replacement"] = repl["replacement"]
		if m, ok := repl["replacement_more"]; ok {
			row["replacement_more"] = m
		}
		lines = append(lines, "  replacement: "+text)
	}
	return render.Part{Text: strings.Join(lines, "\n"), JSON: row, Marks: marks}
}

// runesIn counts the code points of lines joined by line feeds, with the final line feed.
func runesIn(lines []string) int {
	n := 0
	for _, l := range lines {
		n += utf8.RuneCountInString(l) + 1
	}
	return n
}

func contextText(l search.ContextLine) string {
	if !l.Focal {
		return render.OneLine(l.Text, neighbourCut)
	}
	t := render.OneLine(l.Text, 0)
	if l.Before {
		t = "…" + t
	}
	if l.After {
		t += "…"
	}
	return t
}

// contextAlts are a hit's context alternatives, richest first: each next one drops the neighbouring
// line farthest from the focal lines, the later one on a tie, down to the focal lines alone. A block
// over 600 code points in the output's format is not offered.
func (r *reader) contextAlts(h search.Hit) []render.Part {
	lines := slices.Clone(h.Snippet.Context)
	var alts []render.Part
	for {
		if r.blockSize(lines) <= contextCap {
			alts = append(alts, r.hitPart(h, lines))
		}
		drop := farthestNeighbour(lines)
		if drop < 0 {
			return alts
		}
		lines = slices.Delete(lines, drop, drop+1)
	}
}

func farthestNeighbour(lines []search.ContextLine) int {
	first, last := -1, -1
	for i, l := range lines {
		if l.Focal {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	best, dist := -1, -1
	for i, l := range lines {
		if l.Focal {
			continue
		}
		d := first - i
		if i > last {
			d = i - last
		}
		if d >= dist { // the later one on a tie
			best, dist = i, d
		}
	}
	return best
}

// blockSize is the whole addition of a block: its prefixed lines with their line feeds in text, or
// the encoded "context" field with its comma in JSON.
func (r *reader) blockSize(lines []search.ContextLine) int {
	if r.in.format == render.JSON {
		var arr []render.Object
		for _, l := range lines {
			obj := render.Object{"line": l.Line}
			if t := contextText(l); t != "" {
				obj["text"] = t
			}
			arr = append(arr, obj)
		}
		enc, _ := json.Marshal(arr)
		return utf8.RuneCount(enc) + utf8.RuneCountInString(`,"context":`)
	}
	n := 0
	for _, l := range lines {
		s := "    " + strconv.Itoa(l.Line) + ":"
		if t := contextText(l); t != "" {
			s += " " + t
		}
		n += utf8.RuneCountInString(s) + 1
	}
	return n
}

func runGet(ctx context.Context, env Env, in *invocation, b *brain.Brain) outcome {
	r := &reader{env: env, in: in, b: b, now: in.now}
	id := in.operands[0]
	switch {
	case in.boolean("history"):
		if err := r.scan(ctx, store.KeepNone); err != nil {
			return outcome{err: err}
		}
		it, err := r.resolve(id)
		if err != nil {
			return outcome{err: err}
		}
		r.skipped()
		return r.fit(r.historyDoc(it))
	case in.boolean("outline"):
		it, body, err := r.resolveWithBody(ctx, id)
		if err != nil {
			return outcome{err: err}
		}
		r.skipped()
		return r.fit(r.outlineDoc(it, body))
	default:
		it, body, err := r.resolveWithBody(ctx, id)
		if err != nil {
			return outcome{err: err}
		}
		r.skipped()
		data, err := render.FitBody(r.getDoc(it, body), in.page.maxChars, in.format)
		if err != nil {
			return outcome{err: err}
		}
		return outcome{data: data}
	}
}

func (r *reader) fit(d *render.Doc) outcome {
	data, err := render.Fit(d, r.in.page.maxChars, r.in.format)
	if err != nil {
		return outcome{err: err}
	}
	return outcome{data: data}
}

// resolveWithBody resolves id and reads its body; a file that changed since the scan is scanned and
// resolved again once, and a second change is conflict. A header and bytes are never from two scans.
func (r *reader) resolveWithBody(ctx context.Context, id string) (*item.Item, []byte, error) {
	for attempt := 0; ; attempt++ {
		if err := r.scan(ctx, store.KeepNone); err != nil {
			return nil, nil, err
		}
		it, err := r.resolve(id)
		if err != nil {
			return nil, nil, err
		}
		body, err := r.cat.Body(it)
		if err == nil {
			return it, body, nil
		}
		if !errors.Is(err, store.ErrChanged) {
			return nil, nil, err
		}
		if attempt == 1 {
			return nil, nil, &errs.Error{Code: "conflict", Message: "the item changed while it was read; run the command again", Exit: 1}
		}
	}
}

// resolve finds the one item with the folded id, or fails with not_found and safe suggestions, or
// ambiguous_id.
func (r *reader) resolve(id string) (*item.Item, error) {
	found := r.cat.Lookup(id)
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		msg := "no item has that id in this view"
		if s := r.suggestions(id); len(s) > 0 {
			msg += "; ids that start alike: " + strings.Join(s, ", ")
		}
		return nil, &errs.Error{Code: "not_found", Message: msg, Exit: 1}
	default:
		return nil, &errs.Error{Code: "ambiguous_id", Message: "two or more items have that id; check lists them", Exit: 1}
	}
}

// suggestions are up to three visible ids that start with the whole folded argument or, when there
// are none and the argument starts with a valid UTC stamp, with that stamp.
func (r *reader) suggestions(id string) []string {
	key := item.Fold(id)
	pick := func(prefix string) []string {
		seen := map[string]bool{}
		var cands []*item.Item
		for _, it := range r.cat.Items() {
			if strings.HasPrefix(it.Key, prefix) && !seen[it.Key] && !secretscan.Sensitive(it.ID) {
				seen[it.Key] = true
				cands = append(cands, it)
			}
		}
		slices.SortFunc(cands, func(a, b *item.Item) int {
			if c := strings.Compare(a.Key, b.Key); c != 0 {
				return c
			}
			return strings.Compare(a.Path, b.Path)
		})
		return ids(cands[:min(len(cands), 3)])
	}
	if s := pick(key); len(s) > 0 || key == "" {
		return s
	}
	if len(key) >= 15 {
		if _, err := time.Parse("20060102-150405", key[:15]); err == nil {
			return pick(key[:15])
		}
	}
	return nil
}

func (r *reader) historyDoc(it *item.Item) *render.Doc {
	hist := r.g.History(it)
	off := r.in.page.offset
	d := &render.Doc{
		Head:   []render.Part{{JSON: render.Envelope{Fields: render.Object{}, Rows: "history"}}},
		Offset: off,
		Total:  len(hist),
	}
	if off < len(hist) {
		for _, h := range hist[off:min(len(hist), off+r.in.page.limit)] {
			d.Units = append(d.Units, render.Unit{Main: r.historyRow(h)})
		}
	}
	d.Foot = listFoot("item", "items")
	return d
}

func (r *reader) historyRow(it *item.Item) render.Part {
	h := &it.Header
	row := r.commonRow(it)
	var paren []string
	if h.Kind == item.Decision && h.Decided != "" {
		paren = append(paren, "decided by "+render.OneLine(h.Decided, actorCut))
	}
	if reps := ids(r.g.Replaces(it)); len(reps) > 0 {
		paren = append(paren, "replaces: "+idList(row, "replaces", reps))
	}
	text := it.ID + "  " + h.Kind.String() + "  " + h.Status.String() + "  " + dateText(h) + "  " + render.OneLine(h.Title, titleCut)
	if len(paren) > 0 {
		text += " (" + strings.Join(paren, "; ") + ")"
	}
	for _, m := range r.markers(it) {
		text += "  " + m
	}
	return render.Part{Text: text, JSON: row, Marks: []render.Mark{{Start: 0, End: utf8.RuneCountInString(it.ID), Style: render.Bold}}}
}

func listFoot(one, many string) func(render.State) render.Part {
	return func(st render.State) render.Part {
		text := counted(st.Shown, one, many) + "."
		obj := render.Object{}
		if st.More > 0 {
			text += fmt.Sprintf(" %d more: --offset %d.", st.More, st.Next)
			obj["more"], obj["next"] = st.More, st.Next
		}
		return render.Part{Text: text, JSON: obj}
	}
}

func (r *reader) outlineDoc(it *item.Item, body []byte) *render.Doc {
	secs := outline.Parse(body)
	bodyChars := utf8.RuneCount(body)
	off := r.in.page.offset
	head := it.ID + "\n" + fmt.Sprintf("outline: %d sections, body %d", len(secs), bodyChars)
	d := &render.Doc{
		Head: []render.Part{{
			Text:  head,
			JSON:  render.Envelope{Fields: render.Object{"id": it.ID, "sections": len(secs), "body_chars": bodyChars}, Rows: "outline"},
			Marks: []render.Mark{{Start: 0, End: utf8.RuneCountInString(it.ID), Style: render.Bold}},
		}},
		Offset: off,
		Total:  len(secs),
	}
	if off < len(secs) {
		cur := it.Lines(body)
		for _, s := range secs[off:min(len(secs), off+r.in.page.limit)] {
			line := cur.At(s.Start)
			heading := "(before the first heading)"
			row := render.Object{"offset": s.Start, "chars": s.Tree - s.Start, "file_line": line, "level": s.Level}
			if s.Level > 0 {
				t := render.OneLine(s.Title, titleCut)
				heading = strings.Repeat("#", s.Level)
				if t != "" {
					heading += " " + t
					row["title"] = t
				}
			}
			text := fmt.Sprintf("%d  %d  L%d  %s", s.Start, s.Tree-s.Start, line, heading)
			d.Units = append(d.Units, render.Unit{Main: render.Part{Text: text, JSON: row}})
		}
	}
	d.Foot = func(st render.State) render.Part {
		text := counted(st.Shown, "section", "sections") + "."
		obj := render.Object{}
		if st.More > 0 {
			text += fmt.Sprintf(" %d more: --offset %d", st.More, st.Next)
			obj["more"], obj["next"] = st.More, st.Next
		}
		return render.Part{Text: text, JSON: obj}
	}
	return d
}

func originWord(o item.Origin) string {
	switch o {
	case item.OriginFile:
		return "file"
	case item.OriginLink:
		return "link"
	case item.OriginURL:
		return "url"
	}
	return ""
}

func (r *reader) getDoc(it *item.Item, body []byte) *render.BodyDoc {
	h := &it.Header
	g := r.g
	row := render.Object{
		"id":      it.ID,
		"kind":    h.Kind.String(),
		"status":  h.Status.String(),
		"title":   render.OneLine(h.Title, titleCut),
		"created": instant(h.Created, h.CreatedKnown),
	}
	idLen := utf8.RuneCountInString(it.ID)
	marks := []render.Mark{{Start: 0, End: idLen, Style: render.Bold}}
	status := h.Kind.String() + "  " + h.Status.String() + "  " + instant(h.Created, h.CreatedKnown)
	for _, m := range r.markers(it) {
		status += "  " + m
	}
	if h.ReviewDue(r.now) {
		row["review_due"] = true
	}
	if h.Linked() {
		row["linked"] = true
	}
	if it.Section == item.Personal {
		row["personal"] = true
	}
	if h.Kind == item.Source {
		status += "  outside material, reference only"
	}
	lines := []string{it.ID, status, "title: " + render.OneLine(h.Title, titleCut)}
	var actors []string
	for _, a := range [][2]string{{"author", h.Author}, {"verified", h.Verified}, {"decided", h.Decided}} {
		if a[1] != "" {
			v := render.OneLine(a[1], actorCut)
			actors = append(actors, a[0]+": "+v)
			row[a[0]] = v
		}
	}
	if len(actors) > 0 {
		lines = append(lines, strings.Join(actors, "  "))
	}
	label := func(name, value string) {
		if value != "" {
			lines = append(lines, name+": "+value)
		}
	}
	if h.Source != "" {
		src := render.OneLine(h.Source, getSourceCut)
		if h.Linked() {
			row["source"] = h.Source // complete, so it can be followed
		} else {
			row["source"] = src
		}
		label("source", src)
	}
	if v := g.Shown(it, graph.FieldReplaces); len(v) > 0 {
		label("replaces", idList(row, "replaces", v))
	}
	if !g.Current(it) {
		label("replacement", idList(row, "replacement", ids(g.Replacement(it))))
		label("current", idList(row, "current", ids(g.CurrentSuccessors(it))))
	}
	if v := g.Shown(it, graph.FieldRelates); len(v) > 0 {
		label("relates", idList(row, "relates", v))
	}
	if len(h.Tags) > 0 {
		row["tags"] = h.Tags
		label("tags", render.OneLine(strings.Join(h.Tags, ", "), 0))
	}
	if h.StaleSet && !h.StaleBad {
		v := h.StaleAfter.UTC().Format(time.RFC3339)
		row["stale_after"] = v
		label("stale_after", v)
	}
	if o := originWord(h.Origin); o != "" {
		row["origin"] = o
		label("origin", o)
	}
	if !h.Fetched.IsZero() {
		v := h.Fetched.UTC().Format(time.RFC3339)
		row["fetched"] = v
		label("fetched", v)
	}
	if n := g.Unresolved(it); n > 0 {
		row["unresolved"] = n
		label("unresolved", strconv.Itoa(n))
	}
	if display, encoded, ok := r.pathOf(it); ok {
		row["path"] = encoded
		at := runesIn(lines) + utf8.RuneCountInString("path: ")
		marks = append(marks, render.Mark{Start: at, End: at + utf8.RuneCountInString(display), Style: render.Cyan})
		lines = append(lines, "path: "+display)
	}
	lay := it.Layout
	lines = append(lines, fmt.Sprintf("body: L%d, %dC; file %dL/%dC", lay.BodyLine, lay.BodyChars, lay.FileLines, lay.FileChars))
	row["body_line"], row["body_chars"] = lay.BodyLine, lay.BodyChars
	row["file_lines"], row["file_chars"] = lay.FileLines, lay.FileChars
	row["offset"] = r.in.page.offset
	d := &render.BodyDoc{
		Doc: render.Doc{
			Head:   []render.Part{{Text: strings.Join(lines, "\n"), JSON: render.Envelope{Fields: row}, Marks: marks}},
			Offset: r.in.page.offset,
			Total:  utf8.RuneCount(body),
		},
		Body:   body,
		Extras: r.related(it),
	}
	cur := it.Lines(body)
	d.LineAt = cur.At
	if h.Linked() {
		d.Tail = "linked: content not copied; read it at the source with your own access: " + render.OneLine(h.Source, 0)
	}
	return d
}
