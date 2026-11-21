package render

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/errs"
)

type bodyCase struct {
	head   []string
	fields Object
	extras []string
	body   string
	offset int
	tail   string
	base   int    // the file line of the body's first character
	lfsp   *[]int // line feeds before each code point, computed once
}

// lineAt is the file line of code point cp: base plus the line feeds before it.
func (c bodyCase) lineAt(cp int) int {
	if c.lfsp == nil {
		panic("bodyCase without lfsp")
	}
	if *c.lfsp == nil {
		lfs := make([]int, 1, len(c.body)+1)
		for _, r := range c.body {
			n := lfs[len(lfs)-1]
			if r == '\n' {
				n++
			}
			lfs = append(lfs, n)
		}
		*c.lfsp = lfs
	}
	return c.base + (*c.lfsp)[min(cp, len(*c.lfsp)-1)]
}

func (c bodyCase) doc(spy *[]int) *BodyDoc {
	d := &BodyDoc{Body: []byte(c.body), Tail: c.tail}
	d.Doc.Offset, d.Doc.Total = c.offset, utf8.RuneCountInString(c.body)
	for i, h := range c.head {
		p := Part{Text: h}
		if i == 0 {
			p.JSON = Envelope{Fields: c.fields}
			p.Marks = []Mark{{Start: 0, End: min(3, utf8.RuneCountInString(h)), Style: Bold}}
		}
		d.Doc.Head = append(d.Doc.Head, p)
	}
	for _, e := range c.extras {
		d.Extras = append(d.Extras, Part{Text: e, JSON: Object{"id": e}})
	}
	d.LineAt = func(cp int) int {
		*spy = append(*spy, cp)
		return c.lineAt(cp)
	}
	return d
}

// expected is the output for the slice [offset, end) with k related lines, built straight from the
// rules of render.md, independently of FitBody's sizing.
func (c bodyCase) expected(f Format, end, k int) string {
	rs := []rune(c.body)
	total := len(rs)
	nonEmpty := end > c.offset
	slice := ""
	if nonEmpty {
		slice = string(rs[c.offset:end])
	}
	if f == JSON {
		root := map[string]any{}
		maps.Copy(root, c.fields)
		if k > 0 {
			rel := []any{}
			for _, e := range c.extras[:k] {
				rel = append(rel, map[string]any{"id": e})
			}
			root["related"] = rel
		}
		if nonEmpty {
			root["body"] = slice
			root["line_from"] = c.lineAt(c.offset)
			root["line_to"] = c.lineAt(end - 1)
			if total-end > 0 {
				root["more"], root["next"] = total-end, end
			}
		}
		var b bytes.Buffer
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(root)
		return b.String()
	}
	var b strings.Builder
	for _, h := range c.head {
		b.WriteString(h + "\n")
	}
	for _, e := range c.extras[:k] {
		b.WriteString(e + "\n")
	}
	if nonEmpty {
		b.WriteString("\n" + slice)
		if !strings.HasSuffix(slice, "\n") {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "[body %d to %d of %d; file L%d–L%d", c.offset, end-1, total, c.lineAt(c.offset), c.lineAt(end-1))
		if total-end > 0 {
			fmt.Fprintf(&b, "; more: --offset %d", end)
		}
		b.WriteString("]\n")
	}
	if c.tail != "" {
		b.WriteString(c.tail + "\n")
	}
	return b.String()
}

// oracle chooses as render.md does: the reserve is the fixed part and one character; related lines
// while they fit with it; then the longest slice that fits.
func (c bodyCase) oracle(f Format, limit int) (string, int) {
	total := utf8.RuneCountInString(c.body)
	first := c.offset
	if c.offset < total {
		first = c.offset + 1
	}
	need := utf8.RuneCountInString(c.expected(f, first, 0))
	if need > limit {
		return "", need
	}
	k := 0
	for k < len(c.extras) && utf8.RuneCountInString(c.expected(f, first, k+1)) <= limit {
		k++
	}
	best := first
	for end := first; end <= total && c.offset < total; end++ {
		if utf8.RuneCountInString(c.expected(f, end, k)) <= limit {
			best = end
		}
	}
	return c.expected(f, best, k), 0
}

var bodyRunes = []string{"a", "é", "\n", "\r", "\r\n", "\"", "\\", "\u2028", "\t", "\b", "\x01", "😀", " ", "<", "&"}

func TestFitBodyAgainstOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for trial := range 300 {
		var body strings.Builder
		for range rng.Intn(12) {
			body.WriteString(bodyRunes[rng.Intn(len(bodyRunes))])
		}
		total := utf8.RuneCountInString(body.String())
		c := withLines(bodyCase{
			head:   []string{"20261121-id", "decision  verified", "body: L9, " + fmt.Sprint(total) + "C"},
			fields: Object{"id": "20261121-id", "offset": 0, "body_chars": total},
			extras: []string{"relates to a", "newer related bb", "relates to ccc"}[:rng.Intn(4)],
			body:   body.String(),
			offset: rng.Intn(total + 2),
			base:   1 + rng.Intn(120),
		})
		if rng.Intn(3) == 0 {
			c.tail = "linked: content not copied; read it at the source with your own access: https://example.com/x"
		}
		for _, f := range []Format{Text, JSON} {
			full := utf8.RuneCountInString(c.expected(f, max(c.offset, total), len(c.extras))) + 3
			for limit := 1; limit <= full; limit++ {
				var spy []int
				out, err := FitBody(c.doc(&spy), limit, f)
				want, need := c.oracle(f, limit)
				if need > 0 {
					var e *errs.Error
					if !errors.As(err, &e) || e.Code != "budget_too_small" || !strings.Contains(e.Message, fmt.Sprintf("works is %d", need)) {
						t.Fatalf("trial %d f %d limit %d: want budget_too_small %d, got %v", trial, f, limit, need, err)
					}
					continue
				}
				if err != nil {
					t.Fatalf("trial %d f %d limit %d: %v", trial, f, limit, err)
				}
				if string(out) != want {
					t.Fatalf("trial %d f %d limit %d body %q offset %d:\ngot  %q\nwant %q", trial, f, limit, c.body, c.offset, out, want)
				}
				for i := 1; i < len(spy); i++ {
					if spy[i] < spy[i-1] {
						t.Fatalf("LineAt went backwards: %v", spy)
					}
				}
				if c.offset >= total && len(spy) > 0 {
					t.Fatalf("LineAt called for an empty slice")
				}
			}
		}
	}
}

// A body starting at L9 with A\r\nB: a slice of A\r or A\r\n ends on L9; the page at B is L10.
func TestFitBodyCRLFLines(t *testing.T) {
	c := withLines(bodyCase{head: []string{"id"}, fields: Object{"id": "id"}, body: "A\r\nB", base: 9})
	var spy []int
	full, _ := FitBody(c.doc(&spy), 1000, Text)
	if !strings.HasSuffix(string(full), "[body 0 to 3 of 4; file L9–L10]\n") {
		t.Fatalf("whole body: %q", full)
	}
	for limit := 1; limit < 200; limit++ {
		out, err := FitBody(c.doc(&spy), limit, Text)
		if err != nil {
			continue
		}
		s := string(out)
		if strings.Contains(s, "\nA\r\n[body 0 to 1") || strings.Contains(s, "\nA\r\n[body 0 to 2") {
			if !strings.Contains(s, "file L9–L9; more") {
				t.Fatalf("limit %d: %q", limit, s)
			}
		}
	}
	c.offset = 3
	out, _ := FitBody(c.doc(&spy), 1000, Text)
	if !strings.HasSuffix(string(out), "\nB\n[body 3 to 3 of 4; file L10–L10]\n") {
		t.Fatalf("page at B: %q", out)
	}
}

// One 512,000-character line: the cursor is asked in rising order and the walk stops once even the
// whole body could not fit.
func TestFitBodyLongLineOneWalk(t *testing.T) {
	c := withLines(bodyCase{head: []string{"id"}, fields: Object{"id": "id"}, body: strings.Repeat("x", 512000), base: 9})
	var spy []int
	out, err := FitBody(c.doc(&spy), 2000, Text)
	if err != nil || utf8.RuneCount(out) > 2000 {
		t.Fatalf("%v %d", err, utf8.RuneCount(out))
	}
	if len(spy) > 2100 {
		t.Fatalf("LineAt called %d times for a 2,000-character page", len(spy))
	}
	for i := 1; i < len(spy); i++ {
		if spy[i] < spy[i-1] {
			t.Fatal("LineAt went backwards")
		}
	}
	spy = nil
	if _, err := FitBody(c.doc(&spy), 600000, JSON); err != nil {
		t.Fatal(err)
	}
	if len(spy) > 512001 {
		t.Fatalf("more than one walk: %d calls", len(spy))
	}
}

// Colour touches the header's marks only, after the plain fit: the same slice and lines.
func TestFitBodyColour(t *testing.T) {
	c := withLines(bodyCase{head: []string{"20261121-id", "path: docs/a.md"}, fields: Object{"id": "x"}, body: "hello \x1b[31mred\x1b[0m world\n", base: 9})
	for limit := 1; limit < 200; limit++ {
		var spy []int
		plain, errP := FitBody(c.doc(&spy), limit, Text)
		col, errC := FitBody(c.doc(&spy), limit, TextColor)
		if (errP == nil) != (errC == nil) {
			t.Fatalf("limit %d", limit)
		}
		if errP != nil {
			continue
		}
		if utf8.RuneCount(col) > limit {
			t.Fatalf("limit %d: colour passes the limit", limit)
		}
		stripped := strings.Replace(string(col), escBold, "", 1)
		if strings.HasPrefix(string(col), escBold) {
			stripped = strings.Replace(stripped, escReset, "", 1)
		}
		if stripped != string(plain) {
			t.Fatalf("limit %d: %q vs %q", limit, col, plain)
		}
	}
}

func withLines(c bodyCase) bodyCase {
	c.lfsp = new([]int)
	return c
}

// An empty slice (an empty body, or an offset past its end) adds no line in text, so the colour
// pass does not charge one: the first pair fits in exactly its own eight code points.
func TestFitBodyColourEmptySlice(t *testing.T) {
	for _, c := range []struct {
		body   string
		offset int
	}{{"", 0}, {"abc\n", 9}} {
		d := &BodyDoc{
			Doc:  Doc{Head: []Part{{Text: "id-1\nnote  provisional", Marks: []Mark{{Start: 0, End: 4, Style: Bold}}}}, Offset: c.offset, Total: utf8.RuneCountInString(c.body)},
			Body: []byte(c.body),
		}
		if c.offset < d.Doc.Total {
			t.Fatal("the fixture must not slice")
		}
		plain, err := FitBody(d, 1000, Text)
		if err != nil {
			t.Fatal(err)
		}
		n := utf8.RuneCount(plain)
		got, err := FitBody(d, n+8, TextColor)
		if err != nil || !strings.HasPrefix(string(got), "\x1b[1mid-1\x1b[0m") || utf8.RuneCount(got) != n+8 {
			t.Fatalf("offset %d: %q, %v", c.offset, got, err)
		}
		if got, _ := FitBody(d, n+7, TextColor); string(got) != string(plain) {
			t.Fatalf("offset %d: one short of the pair: %q", c.offset, got)
		}
	}
}
