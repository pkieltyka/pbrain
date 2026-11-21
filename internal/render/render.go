// Package render fits a document to an exact output budget in Unicode code points and prints it as
// text, coloured text or compact JSON (architecture 9, components/render.md). It imports only errs and
// the standard library: the read handlers in cli build the documents.
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/errs"
)

// Doc is one paged response: a fixed head, the units of the page from Offset, and a closing part
// built from what was shown.
type Doc struct {
	Head   []Part
	Units  []Unit
	Offset int
	Total  int
	Foot   func(st State) Part
	Hints  []Part
}

// State is what the closing part is built from. Next is set only when More > 0.
type State struct{ Shown, More, Next int }

// Unit is one row of a page.
type Unit struct {
	Main   Part
	Alts   []Part // complete replacements for Main, richest context first
	Extras []Part // related rows
}

// Part is one piece of output in both forms.
type Part struct {
	Text  string
	JSON  any
	Marks []Mark
}

// Mark is a stretch of a part's text that TextColor may decorate.
type Mark struct {
	Start, End int // half-open code-point span in Text
	Style      Style
}

// Style is a decoration of TextColor.
type Style uint8

const (
	Bold Style = iota
	Cyan
)

// Format is the form of the output.
type Format uint8

const (
	Text Format = iota
	JSON
	TextColor
)

// Object is one JSON object of a row, a hint or the closing part.
type Object map[string]any

// Envelope is the JSON root a list carries in its first head part. Rows names the key of the row
// array; FitBody's envelope has none.
type Envelope struct {
	Fields Object
	Rows   string // results, history, outline, items, tags, findings, rules
}

// BodyDoc is the document of an ordinary get: a header, related rows and a slice of the body.
type BodyDoc struct {
	Doc    Doc           // Offset and Total count code points; Units, Hints and Foot are unused
	Body   []byte        // valid UTF-8, same version as the header used by Doc
	LineAt func(int) int // one fresh item.LineCursor.At, called with nondecreasing offsets
	Extras []Part        // related rows for this one item
	Tail   string        // fixed linked-source line, without its final newline
}

const (
	escBold  = "\x1b[1m"
	escCyan  = "\x1b[36m"
	escReset = "\x1b[0m"
)

func (s Style) start() string {
	if s == Cyan {
		return escCyan
	}
	return escBold
}

func usageErr(msg string) error { return &errs.Error{Code: "usage", Message: msg, Exit: 2} }

func internalErr(format string, a ...any) error {
	return &errs.Error{Code: "io", Message: "render: " + fmt.Sprintf(format, a...), Exit: 1}
}

func budgetErr(limit, minimum int) error {
	return &errs.Error{
		Code:    "budget_too_small",
		Message: fmt.Sprintf("the limit of %d characters is too small; the smallest limit that works is %d", limit, minimum),
		Exit:    1,
	}
}

// runesHook, when set by a test, sees every string runes counts with its count, and returns the
// count to use: a test can tally the work or stand in a size no memory could hold.
var runesHook func(s string, n int) int

func runes(s string) int {
	n := utf8.RuneCountInString(s)
	if runesHook != nil {
		n = runesHook(s, n)
	}
	return n
}

// encodeJSON is the one encoder of every JSON piece: compact, HTML escaping off, no final newline.
func encodeJSON(v any) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", internalErr("encode: %v", err)
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

func checkMarks(p Part) error {
	n := runes(p.Text)
	prev := 0
	for _, m := range p.Marks {
		if m.Start < prev || m.End <= m.Start || m.End > n || (m.Style != Bold && m.Style != Cyan) {
			return internalErr("marks out of order, overlapping or outside their text")
		}
		prev = m.End
	}
	return nil
}

// decorate writes text with the first n of its marks as complete styling pairs.
func decorate(text string, marks []Mark, n int) string {
	if n == 0 {
		return text
	}
	var b strings.Builder
	cp, mi, open := 0, 0, false
	for _, r := range text {
		if mi < n {
			if open && cp == marks[mi].End {
				b.WriteString(escReset)
				open = false
				mi++
			}
			if !open && mi < n && cp == marks[mi].Start {
				b.WriteString(marks[mi].Style.start())
				open = true
			}
		}
		b.WriteRune(r)
		cp++
	}
	if open {
		b.WriteString(escReset)
	}
	return b.String()
}

func pairCost(m Mark) int { return runes(m.Style.start()) + runes(escReset) }

// page is one candidate output of Fit: the units shown, the part chosen for each, and how many
// extras and hints are in.
type page struct {
	d      *Doc
	f      Format
	shown  int
	chosen []Part // per shown unit: Main or one of its Alts
	extras []int  // per shown unit: how many of its extras
	hints  int
	env    Envelope
	// JSON caches, by shown unit.
	rowEnc []string
	// Size caches (render.md "Whole rows"): each unit's cost in the page's format, the running sum
	// over the shown units, and the head's fixed cost. Fit keeps them current as it admits a unit,
	// swaps in an alternative or adds an extra, so a trial costs no recount of the admitted rows.
	cost      []int
	sum       int
	headCost  int
	headLines int
	overflow  bool // a count overflowed: size fails
}

// unitCost is unit i's share of the page size: its encoded row in JSON; in text, its chosen line
// and its admitted extras, each with its line feed.
func (p *page) unitCost(i int) int {
	if p.f == JSON {
		return runes(p.rowEnc[i])
	}
	n := 0
	p.grow(&n, runes(p.chosen[i].Text))
	p.grow(&n, 1)
	for j := range p.extras[i] {
		p.grow(&n, runes(p.d.Units[i].Extras[j].Text))
		p.grow(&n, 1)
	}
	return n
}

// addChecked is a + b, and false when the sum does not fit in an int.
func addChecked(a, b int) (int, bool) {
	c := a + b
	if (b > 0 && c < a) || (b < 0 && c > a) {
		return 0, false
	}
	return c, true
}

// grow adds d to *n; an overflow marks the page, whose size then fails (a size past MaxInt cannot
// be counted, so it is an internal error rather than a wrapped, negative size that would fit).
func (p *page) grow(n *int, d int) {
	v, ok := addChecked(*n, d)
	if !ok {
		p.overflow = true
		return
	}
	*n = v
}

// resync measures the head and every unit and recomputes the shown sum from the page's fields.
func (p *page) resync() {
	p.headCost, p.headLines, p.overflow = 0, 0, false
	for _, h := range p.d.Head {
		if h.Text != "" {
			p.grow(&p.headCost, runes(h.Text)+1)
			p.headLines++
		}
	}
	p.cost = make([]int, len(p.d.Units))
	p.sum = 0
	for i := range p.d.Units {
		p.cost[i] = p.unitCost(i)
		if i < p.shown {
			p.grow(&p.sum, p.cost[i])
		}
	}
}

// setShown shows the first k units, keeping the sum current.
func (p *page) setShown(k int) {
	for p.shown < k {
		p.grow(&p.sum, p.cost[p.shown])
		p.shown++
	}
	for p.shown > k {
		p.shown--
		p.grow(&p.sum, -p.cost[p.shown])
	}
}

// recost measures unit i again after its chosen part or its extras changed.
func (p *page) recost(i int) {
	old := p.cost[i]
	p.cost[i] = p.unitCost(i)
	if i < p.shown {
		p.grow(&p.sum, p.cost[i]-old)
	}
}

func (p *page) state() State {
	st := State{Shown: p.shown}
	if p.d.Offset < p.d.Total {
		st.More = max(0, p.d.Total-p.d.Offset-p.shown)
	}
	if st.More > 0 {
		st.Next = p.d.Offset + p.shown
	}
	return st
}

func (p *page) foot() Part {
	if p.d.Foot == nil {
		return Part{}
	}
	return p.d.Foot(p.state())
}

func asObject(v any) (Object, bool) {
	switch o := v.(type) {
	case Object:
		return o, true
	case map[string]any:
		return Object(o), true
	case nil:
		return nil, true
	}
	return nil, false
}

func (p *page) encodeRow(i int) error {
	obj, ok := asObject(p.chosen[i].JSON)
	if !ok {
		return internalErr("row %d is not a JSON object", i)
	}
	row := Object{}
	maps.Copy(row, obj)
	if n := p.extras[i]; n > 0 {
		rel := make([]any, n)
		for j := range n {
			rel[j] = p.d.Units[i].Extras[j].JSON
		}
		if _, dup := row["related"]; dup {
			return internalErr("row %d already has related", i)
		}
		row["related"] = rel
	}
	s, err := encodeJSON(row)
	if err != nil {
		return err
	}
	p.rowEnc[i] = s
	return nil
}

// root builds the JSON root with placeholders for the row and hint arrays, so that its size can be
// combined with the cached row encodings without encoding the rows again.
func (p *page) root(foot Part) (Object, error) {
	root := Object{}
	maps.Copy(root, p.env.Fields)
	fo, ok := asObject(foot.JSON)
	if !ok {
		return nil, internalErr("the closing part is not a JSON object")
	}
	for k, v := range fo {
		if _, dup := root[k]; dup {
			return nil, internalErr("closing field %q repeats a head field", k)
		}
		root[k] = v
	}
	if p.shown > 0 {
		if p.env.Rows == "" {
			return nil, internalErr("rows without a row key")
		}
		if _, dup := root[p.env.Rows]; dup {
			return nil, internalErr("row key %q repeats a field", p.env.Rows)
		}
		root[p.env.Rows] = json.RawMessage("0")
	}
	if p.hints > 0 {
		if _, dup := root["alone"]; dup {
			return nil, internalErr("alone repeats a field")
		}
		root["alone"] = json.RawMessage("1")
	}
	return root, nil
}

func (p *page) rowsArray() string {
	return "[" + strings.Join(p.rowEnc[:p.shown], ",") + "]"
}

func (p *page) hintsArray() (string, error) {
	parts := make([]string, p.hints)
	for i := range p.hints {
		s, err := encodeJSON(p.d.Hints[i].JSON)
		if err != nil {
			return "", err
		}
		parts[i] = s
	}
	return "[" + strings.Join(parts, ",") + "]", nil
}

// size is the code-point count of the page's plain output, newline included. It is computed from
// the cached pieces; render() assembles the same pieces, and a test holds the two equal.
func (p *page) size() (int, error) {
	foot := p.foot()
	if p.f == JSON {
		root, err := p.root(foot)
		if err != nil {
			return 0, err
		}
		skel, err := encodeJSON(root)
		if err != nil {
			return 0, err
		}
		n := runes(skel) + 1
		if p.shown > 0 {
			p.grow(&n, 1+max(0, p.shown-1))
			p.grow(&n, p.sum)
		}
		if p.hints > 0 {
			h, err := p.hintsArray()
			if err != nil {
				return 0, err
			}
			p.grow(&n, -1+runes(h))
		}
		return p.checked(n)
	}
	n := p.headCost
	p.grow(&n, p.sum)
	last := foot.Text
	if last != "" {
		p.grow(&n, runes(last)+1)
	}
	if p.hints > 0 {
		if last == "" && p.headLines == 0 && p.shown == 0 {
			p.grow(&n, 1) // the hints make a line of their own
		}
		p.grow(&n, runes(" Alone: ")+runes(".")+2*(p.hints-1))
		for i := range p.hints {
			p.grow(&n, runes(p.d.Hints[i].Text))
		}
	}
	return p.checked(n)
}

// checked is the page's size, or an internal error once any count on the way overflowed.
func (p *page) checked(n int) (int, error) {
	if p.overflow {
		return 0, internalErr("the page size does not fit in an int")
	}
	return n, nil
}

// textParts lists the parts of the plain text output in output order, each with the number of its
// marks to decorate set by the caller; the closing part comes last with the hint suffix after it.
func (p *page) textParts() []Part {
	var out []Part
	for _, h := range p.d.Head {
		if h.Text != "" {
			out = append(out, h)
		}
	}
	for i := range p.shown {
		out = append(out, p.chosen[i])
		out = append(out, p.d.Units[i].Extras[:p.extras[i]]...)
	}
	return out
}

func (p *page) render(colour bool, limit int) ([]byte, error) {
	foot := p.foot()
	if p.f == JSON {
		root, err := p.root(foot)
		if err != nil {
			return nil, err
		}
		if p.shown > 0 {
			root[p.env.Rows] = json.RawMessage(p.rowsArray())
		}
		if p.hints > 0 {
			h, err := p.hintsArray()
			if err != nil {
				return nil, err
			}
			root["alone"] = json.RawMessage(h)
		}
		s, err := encodeJSON(root)
		if err != nil {
			return nil, err
		}
		return []byte(s + "\n"), nil
	}
	parts := p.textParts()
	parts = append(parts, foot)
	used := 0
	if colour {
		size, err := p.size()
		if err != nil {
			return nil, err
		}
		used = size
	}
	var b strings.Builder
	lines := 0
	for i, part := range parts {
		isFoot := i == len(parts)-1
		text := part.Text
		if colour {
			n := 0
			for _, m := range part.Marks {
				if used+pairCost(m) > limit {
					break
				}
				used += pairCost(m)
				n++
			}
			text = decorate(text, part.Marks, n)
			if n < len(part.Marks) {
				colour = false // stop at the first pair that does not fit
			}
		}
		if isFoot {
			suffix := ""
			if p.hints > 0 {
				hs := make([]string, p.hints)
				for j := range p.hints {
					hs[j] = p.d.Hints[j].Text
				}
				suffix = " Alone: " + strings.Join(hs, ", ") + "."
			}
			if text == "" && suffix == "" {
				break
			}
			if text == "" && lines > 0 {
				// hints extend the last line already written
				s := strings.TrimSuffix(b.String(), "\n")
				b.Reset()
				b.WriteString(s)
				b.WriteString(suffix)
				b.WriteByte('\n')
				break
			}
			text += suffix
		} else if text == "" {
			continue
		}
		b.WriteString(text)
		b.WriteByte('\n')
		lines++
	}
	return []byte(b.String()), nil
}

func checkDoc(d *Doc, f Format) (Envelope, error) {
	var env Envelope
	if d.Offset < 0 || d.Total < 0 {
		return env, internalErr("negative offset or total")
	}
	if d.Offset >= d.Total && len(d.Units) > 0 {
		return env, internalErr("units past the end")
	}
	if d.Offset < d.Total && len(d.Units) > d.Total-d.Offset {
		return env, internalErr("more units than remain")
	}
	if d.Offset < d.Total && len(d.Units) == 0 {
		return env, internalErr("no units for a page that has some")
	}
	for _, part := range d.Head {
		if err := checkMarks(part); err != nil {
			return env, err
		}
	}
	for _, u := range d.Units {
		for _, part := range append(append([]Part{u.Main}, u.Alts...), u.Extras...) {
			if err := checkMarks(part); err != nil {
				return env, err
			}
		}
	}
	if f == JSON {
		if len(d.Head) == 0 {
			return env, internalErr("a JSON document needs an envelope")
		}
		switch e := d.Head[0].JSON.(type) {
		case Envelope:
			env = e
		case *Envelope:
			env = *e
		default:
			return env, internalErr("the first head part carries no envelope")
		}
	}
	return env, nil
}

// Fit prints d within limit code points, newline included (architecture 9).
func Fit(d *Doc, limit int, f Format) ([]byte, error) {
	if limit <= 0 {
		return nil, usageErr("--max-chars must be a positive number")
	}
	if f != Text && f != JSON && f != TextColor {
		return nil, internalErr("unknown format")
	}
	env, err := checkDoc(d, f)
	if err != nil {
		return nil, err
	}
	pf := f
	if f == TextColor {
		pf = Text
	}
	p := &page{d: d, f: pf, env: env,
		chosen: make([]Part, len(d.Units)), extras: make([]int, len(d.Units)), rowEnc: make([]string, len(d.Units))}
	for i, u := range d.Units {
		p.chosen[i] = u.Main
		if pf == JSON {
			if err := p.encodeRow(i); err != nil {
				return nil, err
			}
		}
	}
	p.resync()
	fits := func() (bool, int, error) {
		n, err := p.size()
		return n <= limit, n, err
	}
	// The required output: zero units for an empty page, the first unit otherwise.
	if len(d.Units) > 0 {
		p.setShown(1)
	}
	ok, n, err := fits()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, budgetErr(limit, n)
	}
	for p.shown < len(d.Units) {
		p.setShown(p.shown + 1)
		if ok, _, err := fits(); err != nil {
			return nil, err
		} else if !ok {
			p.setShown(p.shown - 1)
			break
		}
	}
	// Alternatives, in unit order, the first that fits.
	for i := range p.shown {
		for _, alt := range d.Units[i].Alts {
			prev, prevEnc := p.chosen[i], p.rowEnc[i]
			p.chosen[i] = alt
			if pf == JSON {
				if err := p.encodeRow(i); err != nil {
					return nil, err
				}
			}
			p.recost(i)
			ok, _, err := fits()
			if err != nil {
				return nil, err
			}
			if ok {
				break
			}
			p.chosen[i], p.rowEnc[i] = prev, prevEnc
			p.recost(i)
		}
	}
	// Extras in unit order, stopping at the first that does not fit.
extras:
	for i := range p.shown {
		for p.extras[i] < len(d.Units[i].Extras) {
			p.extras[i]++
			if pf == JSON {
				if err := p.encodeRow(i); err != nil {
					return nil, err
				}
			}
			p.recost(i)
			ok, _, err := fits()
			if err != nil {
				return nil, err
			}
			if !ok {
				p.extras[i]--
				if pf == JSON {
					if err := p.encodeRow(i); err != nil {
						return nil, err
					}
				}
				p.recost(i)
				break extras
			}
		}
	}
	// Hints in order, stopping at the first that does not fit.
	for p.hints < len(d.Hints) {
		p.hints++
		ok, _, err := fits()
		if err != nil {
			return nil, err
		}
		if !ok {
			p.hints--
			break
		}
	}
	out, err := p.render(f == TextColor, limit)
	if err != nil {
		return nil, err
	}
	if got := utf8.RuneCount(out); got > limit {
		return nil, internalErr("output of %d code points passes the limit %d", got, limit)
	}
	return out, nil
}

// OneLine flattens line breaks and control characters to spaces, a CRLF to one space, and cuts the
// result to maxChars code points with "…" as the last of them. Zero flattens without cutting.
func OneLine(value string, maxChars int) string {
	var b strings.Builder
	n := 0
	prevCR := false
	for _, r := range value {
		if r == '\n' && prevCR {
			prevCR = false
			continue
		}
		prevCR = r == '\r'
		if r == '\u2028' || r == '\u2029' || unicode.IsControl(r) {
			r = ' '
		}
		b.WriteRune(r)
		n++
	}
	s := b.String()
	if maxChars <= 0 || n <= maxChars {
		return s
	}
	cut := 0
	for i := range s {
		if cut == maxChars-1 {
			return s[:i] + "…"
		}
		cut++
	}
	return s
}

// needsEscape reports a rune of Unicode category C or Z other than the ASCII space, unassigned
// code points included (category Cn).
func needsEscape(r rune) bool {
	if r == ' ' {
		return false
	}
	if unicode.In(r, unicode.C, unicode.Z) {
		return true
	}
	return !unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S)
}

// ExactPath is the path of a search location or a get header (render.md, "Exact paths"). It never
// cuts a path. ok is false for a path that is not valid UTF-8: such a path is never printed.
func ExactPath(path string) (display string, encoded json.RawMessage, ok bool) {
	if !utf8.ValidString(path) {
		return "", nil, false
	}
	var b strings.Builder
	b.WriteByte('"')
	escaped := false
	for _, r := range path {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
			escaped = true
		case needsEscape(r):
			escaped = true
			if r > 0xFFFF {
				hi, lo := utf16Pair(r)
				fmt.Fprintf(&b, "\\u%04x\\u%04x", hi, lo)
			} else {
				fmt.Fprintf(&b, "\\u%04x", r)
			}
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	tok := b.String()
	if escaped || strings.ContainsRune(path, ' ') {
		return tok, json.RawMessage(tok), true
	}
	return path, json.RawMessage(tok), true
}

// ExactValue is the text form of an id, or of any path outside the search location, get's path line
// and init (render.md "Ids and other paths"): the value bare, or, when it holds a double quote, a
// backslash or a rune ExactPath escapes, the token ExactPath builds. An ordinary space stays bare.
// ok is false for input that is not valid UTF-8; the caller prints "?".
func ExactValue(value string) (text string, ok bool) {
	tok, _, ok := ExactPath(value)
	if !ok {
		return "", false
	}
	if strings.ContainsFunc(value, func(r rune) bool { return r == '"' || r == '\\' || needsEscape(r) }) {
		return tok, true
	}
	return value, true
}

// ValueText is ExactValue's text, or "?" for a value it cannot write.
func ValueText(value string) string {
	if t, ok := ExactValue(value); ok {
		return t
	}
	return "?"
}

func utf16Pair(r rune) (hi, lo rune) {
	r -= 0x10000
	return 0xD800 + (r>>10)&0x3FF, 0xDC00 + r&0x3FF
}

// jsonRuneCost is the code points one rune of a string takes in this package's JSON encoding
// (encoding/json with HTML escaping off).
func jsonRuneCost(r rune) int {
	switch {
	case r == '"' || r == '\\' || r == '\n' || r == '\r' || r == '\t' || r == '\b' || r == '\f':
		return 2
	case r < 0x20 || r == '\u2028' || r == '\u2029':
		return 6
	}
	return 1
}

// bodyPage is one candidate output of FitBody.
type bodyPage struct {
	d        *BodyDoc
	f        Format
	env      Envelope
	extras   int
	from     int // the requested offset, in code points
	end      int // the slice is [from, end); end == from for an empty slice
	lineFrom int // file lines of from and of end-1, for a non-empty slice
	lineTo   int
	memo     map[[6]int]int
}

func (b *bodyPage) nonEmpty() bool { return b.end > b.from }
func (b *bodyPage) more() int      { return b.d.Doc.Total - b.end }

func (b *bodyPage) footer() string {
	s := fmt.Sprintf("[body %d to %d of %d; file L%d–L%d", b.from, b.end-1, b.d.Doc.Total, b.lineFrom, b.lineTo)
	if b.more() > 0 {
		s += fmt.Sprintf("; more: --offset %d", b.end)
	}
	return s + "]"
}

// root is the JSON root without the body string, whose place holds a one-character placeholder.
func (b *bodyPage) root(body json.RawMessage) (Object, error) {
	root := Object{}
	maps.Copy(root, b.env.Fields)
	set := func(k string, v any) error {
		if _, dup := root[k]; dup {
			return internalErr("body field %q repeats a head field", k)
		}
		root[k] = v
		return nil
	}
	if b.extras > 0 {
		rel := make([]any, b.extras)
		for j := range b.extras {
			rel[j] = b.d.Extras[j].JSON
		}
		if err := set("related", rel); err != nil {
			return nil, err
		}
	}
	if b.nonEmpty() {
		for _, kv := range []struct {
			k string
			v any
		}{{"body", body}, {"line_from", b.lineFrom}, {"line_to", b.lineTo}} {
			if err := set(kv.k, kv.v); err != nil {
				return nil, err
			}
		}
		if b.more() > 0 {
			if err := set("more", b.more()); err != nil {
				return nil, err
			}
			if err := set("next", b.end); err != nil {
				return nil, err
			}
		}
	}
	return root, nil
}

// fixedSize is the size of everything but the body slice's own characters: in JSON the encoded
// root with the body string's place empty, in text the lines around the slice.
func (b *bodyPage) fixedSize() (int, error) {
	// The size depends on the numbers only through their digits, so it is computed once per digit
	// pattern: the scan over a body stays linear.
	key := [6]int{b.extras, ndigits(b.end - 1), ndigits(b.more()), ndigits(b.end), ndigits(b.lineTo), ndigits(b.lineFrom)}
	if b.nonEmpty() && b.more() == 0 {
		key[2] = -1
	}
	if !b.nonEmpty() {
		key = [6]int{b.extras, -1, -1, -1, -1, -1}
	}
	if n, ok := b.memo[key]; ok {
		return n, nil
	}
	n, err := b.measureFixed()
	if err != nil {
		return 0, err
	}
	if b.memo == nil {
		b.memo = map[[6]int]int{}
	}
	b.memo[key] = n
	return n, nil
}

func ndigits(n int) int {
	if n < 0 {
		return -1
	}
	d := 1
	for n >= 10 {
		n /= 10
		d++
	}
	return d
}

func (b *bodyPage) measureFixed() (int, error) {
	if b.f == JSON {
		root, err := b.root(json.RawMessage("0"))
		if err != nil {
			return 0, err
		}
		s, err := encodeJSON(root)
		if err != nil {
			return 0, err
		}
		n := runes(s) + 1 // the final newline
		if b.nonEmpty() {
			n += -1 + 2 // the placeholder out, the string's quotes in
		}
		return n, nil
	}
	n := 0
	for _, h := range b.d.Doc.Head {
		if h.Text != "" {
			n += runes(h.Text) + 1
		}
	}
	for j := range b.extras {
		n += runes(b.d.Extras[j].Text) + 1
	}
	if b.nonEmpty() {
		n += 1 + runes(b.footer()) + 1 // the blank line, the closing line
	}
	if b.d.Tail != "" {
		n += runes(b.d.Tail) + 1
	}
	return n, nil
}

// sliceCost is the size of the body slice itself: one per character in text, its escaped form in
// JSON, and in text the line feed added after a slice that does not end in one.
func (b *bodyPage) sliceCost(chars, jsonCost int, endsLF bool) int {
	if b.f == JSON {
		return jsonCost
	}
	if !endsLF {
		return chars + 1
	}
	return chars
}

func (b *bodyPage) render(slice string, colour bool, limit int) ([]byte, error) {
	if b.f == JSON {
		enc, err := encodeJSON(slice)
		if err != nil {
			return nil, err
		}
		root, err := b.root(json.RawMessage(enc))
		if err != nil {
			return nil, err
		}
		s, err := encodeJSON(root)
		if err != nil {
			return nil, err
		}
		return []byte(s + "\n"), nil
	}
	used := 0
	if colour {
		fixed, err := b.fixedSize()
		if err != nil {
			return nil, err
		}
		used = fixed
		if b.nonEmpty() { // fixedSize already holds the whole response of an empty slice
			used += b.sliceCost(utf8.RuneCountInString(slice), 0, strings.HasSuffix(slice, "\n"))
		}
	}
	var out strings.Builder
	for _, h := range b.d.Doc.Head {
		if h.Text == "" {
			continue
		}
		text := h.Text
		if colour {
			n := 0
			for _, m := range h.Marks {
				if used+pairCost(m) > limit {
					break
				}
				used += pairCost(m)
				n++
			}
			text = decorate(text, h.Marks, n)
			if n < len(h.Marks) {
				colour = false
			}
		}
		out.WriteString(text)
		out.WriteByte('\n')
	}
	for j := range b.extras {
		out.WriteString(b.d.Extras[j].Text)
		out.WriteByte('\n')
	}
	if b.nonEmpty() {
		out.WriteByte('\n')
		out.WriteString(slice)
		if !strings.HasSuffix(slice, "\n") {
			out.WriteByte('\n')
		}
		out.WriteString(b.footer())
		out.WriteByte('\n')
	}
	if b.d.Tail != "" {
		out.WriteString(b.d.Tail)
		out.WriteByte('\n')
	}
	return []byte(out.String()), nil
}

// FitBody prints an ordinary get within limit code points (architecture 9, render.md "Raw body").
func FitBody(d *BodyDoc, limit int, f Format) ([]byte, error) {
	if limit <= 0 {
		return nil, usageErr("--max-chars must be a positive number")
	}
	if f != Text && f != JSON && f != TextColor {
		return nil, internalErr("unknown format")
	}
	if !utf8.Valid(d.Body) {
		return nil, internalErr("the body is not valid UTF-8")
	}
	total := utf8.RuneCount(d.Body)
	if d.Doc.Total != total || d.Doc.Offset < 0 {
		return nil, internalErr("offset or total does not match the body")
	}
	for _, part := range append(append([]Part{}, d.Doc.Head...), d.Extras...) {
		if err := checkMarks(part); err != nil {
			return nil, err
		}
	}
	pf := f
	if f == TextColor {
		pf = Text
	}
	b := &bodyPage{d: d, f: pf, from: d.Doc.Offset, end: d.Doc.Offset}
	if pf == JSON {
		if len(d.Doc.Head) == 0 {
			return nil, internalErr("a JSON document needs an envelope")
		}
		switch e := d.Doc.Head[0].JSON.(type) {
		case Envelope:
			b.env = e
		case *Envelope:
			b.env = *e
		default:
			return nil, internalErr("the first head part carries no envelope")
		}
		if b.env.Rows != "" {
			return nil, internalErr("a body document has no row key")
		}
	}
	// Byte position of the requested offset; past the end the slice is empty.
	start := len(d.Body)
	if d.Doc.Offset < total {
		cp := 0
		for i := range string(d.Body) {
			if cp == d.Doc.Offset {
				start = i
				break
			}
			cp++
		}
	}
	nonEmpty := d.Doc.Offset < total
	if nonEmpty {
		if d.LineAt == nil {
			return nil, internalErr("no line cursor for a non-empty slice")
		}
		b.lineFrom = d.LineAt(d.Doc.Offset)
		b.lineTo = b.lineFrom
		b.end = d.Doc.Offset + 1
	}
	firstRune, _ := utf8.DecodeRune(d.Body[min(start, len(d.Body)):])
	sizeAt := func(chars, jsonCost int, endsLF bool) (int, error) {
		fixed, err := b.fixedSize()
		if err != nil {
			return 0, err
		}
		if !b.nonEmpty() {
			return fixed, nil
		}
		return fixed + b.sliceCost(chars, jsonCost, endsLF), nil
	}
	// The required part: the header, the closing line, the linked line and the first character.
	firstCost := 0
	if nonEmpty {
		firstCost = jsonRuneCost(firstRune)
	}
	n, err := sizeAt(1, firstCost, nonEmpty && firstRune == '\n')
	if err != nil {
		return nil, err
	}
	if n > limit {
		return nil, budgetErr(limit, n)
	}
	// Related lines in graph order while they fit with that reserve.
	for b.extras < len(d.Extras) {
		b.extras++
		n, err := sizeAt(1, firstCost, nonEmpty && firstRune == '\n')
		if err != nil {
			return nil, err
		}
		if n > limit {
			b.extras--
			break
		}
	}
	// The longest slice that fits with its own closing line, scanning ends forward once.
	best, bestTo := b.end, b.lineTo
	if nonEmpty {
		chars, cost := 0, 0
		endsLF := false
		cp := d.Doc.Offset
		for _, r := range string(d.Body[start:]) {
			chars++
			cost += jsonRuneCost(r)
			endsLF = r == '\n'
			b.end = cp + 1
			b.lineTo = d.LineAt(cp)
			cp++
			n, err := sizeAt(chars, cost, endsLF)
			if err != nil {
				return nil, err
			}
			if n <= limit {
				best, bestTo = b.end, b.lineTo
				continue
			}
			// Body cost only grows and the complete body has the shortest closing line: stop once even
			// that cannot fit. A closing line's numbers only gain digits as the slice grows.
			if b.f == JSON {
				if cost+2 > limit {
					break
				}
			} else if chars > limit {
				break
			}
		}
		b.end, b.lineTo = best, bestTo
	}
	sliceEnd := len(d.Body)
	if nonEmpty {
		cp := d.Doc.Offset
		for i := range string(d.Body[start:]) {
			if cp == b.end {
				sliceEnd = start + i
				break
			}
			cp++
		}
	}
	slice := ""
	if nonEmpty {
		slice = string(d.Body[start:sliceEnd])
	}
	out, err := b.render(slice, f == TextColor, limit)
	if err != nil {
		return nil, err
	}
	if got := utf8.RuneCount(out); got > limit {
		return nil, internalErr("output of %d code points passes the limit %d", got, limit)
	}
	return out, nil
}
