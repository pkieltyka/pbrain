package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/errs"
)

// listDoc is a search-like page: a title line with the envelope, n units from offset of total,
// each with optional alternatives and extras, a footer in the shared template, and hints.
func listDoc(total, offset, n int, alts, extras bool, hints []string) *Doc {
	d := &Doc{
		Head:   []Part{{Text: "search: queue", JSON: Envelope{Fields: Object{"query": "queue"}, Rows: "results"}}},
		Offset: offset,
		Total:  total,
	}
	for i := range n {
		id := fmt.Sprintf("2026112%d-id-é%d", i%10, offset+i)
		u := Unit{Main: Part{
			Text:  id + "  decision  verified\n  Title <" + strings.Repeat("ü", i%3) + "> & \"q\"",
			JSON:  Object{"id": id, "title": "Title <&>", "n": 0},
			Marks: []Mark{{Start: 0, End: utf8.RuneCountInString(id), Style: Bold}},
		}}
		if alts {
			u.Alts = []Part{
				{Text: u.Main.Text + "\n" + strings.Repeat("    12: context line\n", 3) + "    15: x", JSON: Object{"id": id, "context": []any{1, 2, 3}}},
				{Text: u.Main.Text + "\n    12: x", JSON: Object{"id": id, "context": []any{1}}},
			}
		}
		if extras {
			u.Extras = []Part{
				{Text: "  relates to " + id + "-a", JSON: Object{"id": id + "-a"}},
				{Text: "  newer related " + id + "-b", JSON: Object{"id": id + "-b"}},
			}
		}
		d.Units = append(d.Units, u)
	}
	d.Foot = func(st State) Part {
		text := fmt.Sprintf("%d result", st.Shown)
		if st.Shown != 1 {
			text += "s"
		}
		text += "."
		o := Object{}
		if st.More > 0 {
			text += fmt.Sprintf(" %d more: --offset %d.", st.More, st.Next)
			o["more"], o["next"] = st.More, st.Next
		}
		return Part{Text: text, JSON: o}
	}
	for i, h := range hints {
		d.Hints = append(d.Hints, Part{Text: h + " " + fmt.Sprint(i+1), JSON: Object{"clause": h, "count": i + 1}})
	}
	return d
}

func budgetMin(t *testing.T, err error) (int, bool) {
	t.Helper()
	var e *errs.Error
	if !errors.As(err, &e) || e.Code != "budget_too_small" {
		return 0, false
	}
	m := regexp.MustCompile(`smallest limit that works is (\d+)`).FindStringSubmatch(e.Message)
	if m == nil {
		t.Fatalf("budget_too_small without its minimum: %q", e.Message)
	}
	var n int
	fmt.Sscan(m[1], &n)
	return n, true
}

var ansi = regexp.MustCompile("\x1b\\[[0-9]+m")

// Every limit from 1 to past the complete output: the output never passes the limit, the reported
// minimum works and one less does not, and more units appear only as the limit grows.
func TestFitEveryLimit(t *testing.T) {
	docs := map[string]*Doc{
		"page":       listDoc(12, 4, 5, false, false, nil),
		"alts":       listDoc(12, 4, 5, true, false, nil),
		"extras":     listDoc(3, 0, 3, false, true, nil),
		"empty":      listDoc(0, 0, 0, false, false, []string{"postgres", "queue"}),
		"past-end":   listDoc(12, 13, 0, false, false, nil),
		"everything": listDoc(7, 2, 5, true, true, nil),
	}
	for name, d := range docs {
		for _, f := range []Format{Text, JSON, TextColor} {
			prevShown := -1
			minimum := 0
			for limit := 1; limit < 2500; limit++ {
				out, err := Fit(d, limit, f)
				if err != nil {
					m, ok := budgetMin(t, err)
					if !ok {
						t.Fatalf("%s %d limit %d: %v", name, f, limit, err)
					}
					if minimum == 0 {
						minimum = m
					} else if m != minimum {
						t.Fatalf("%s %d: minimum moved from %d to %d", name, f, minimum, m)
					}
					if limit >= m {
						t.Fatalf("%s %d: limit %d failed with minimum %d", name, f, limit, m)
					}
					continue
				}
				if minimum != 0 && limit < minimum {
					t.Fatalf("%s %d: limit %d below the minimum %d worked", name, f, limit, minimum)
				}
				if n := utf8.RuneCount(out); n > limit {
					t.Fatalf("%s %d limit %d: %d code points", name, f, limit, n)
				}
				if !strings.HasSuffix(string(out), "\n") || strings.HasSuffix(string(out), "\n\n") {
					t.Fatalf("%s %d: not one final newline: %q", name, f, out)
				}
				shown := len(regexp.MustCompile(`(?m)^(\x1b\[1m)?2026112\d-id-`).FindAllString(string(out), -1))
				if f == JSON {
					var v map[string]any
					if err := json.Unmarshal(out, &v); err != nil {
						t.Fatalf("%s: invalid JSON %q", name, out)
					}
					rows, _ := v["results"].([]any)
					shown = len(rows)
				}
				if shown < prevShown {
					t.Fatalf("%s %d: fewer units at a larger limit", name, f)
				}
				prevShown = shown
			}
		}
	}
}

// The size computed from cached pieces equals the assembled output for every page state.
func TestPageSizeMatchesOutput(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for trial := range 400 {
		total := rng.Intn(8)
		offset := rng.Intn(total + 2)
		n := 0
		if offset < total {
			n = 1 + rng.Intn(total-offset)
		}
		d := listDoc(total, offset, n, rng.Intn(2) == 0, rng.Intn(2) == 0, []string{"a", "b"}[:rng.Intn(3)])
		for _, f := range []Format{Text, JSON} {
			env, err := checkDoc(d, f)
			if err != nil {
				t.Fatal(err)
			}
			p := &page{d: d, f: f, env: env, chosen: make([]Part, n), extras: make([]int, n), rowEnc: make([]string, n)}
			p.shown = rng.Intn(n + 1)
			for i := range n {
				p.chosen[i] = d.Units[i].Main
				if k := len(d.Units[i].Alts); k > 0 && rng.Intn(2) == 0 {
					p.chosen[i] = d.Units[i].Alts[rng.Intn(k)]
				}
				p.extras[i] = rng.Intn(len(d.Units[i].Extras) + 1)
				if f == JSON {
					if err := p.encodeRow(i); err != nil {
						t.Fatal(err)
					}
				}
			}
			if f == JSON || p.shown > 0 || len(d.Hints) == 0 {
				p.hints = rng.Intn(len(d.Hints) + 1)
			}
			p.resync()
			size, err := p.size()
			if err != nil {
				t.Fatal(err)
			}
			out, err := p.render(false, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := utf8.RuneCount(out); got != size {
				t.Fatalf("trial %d format %d: size %d, output %d: %q", trial, f, size, got, out)
			}
		}
	}
}

// Architecture 9's example: 12 matches, offset 4, limit 5 and room for two give next 6 and more 6.
func TestPageLimitsAndMinima(t *testing.T) {
	d := listDoc(12, 4, 5, false, false, nil)
	two := 0
	for limit := 1; limit < 3000; limit++ {
		out, err := Fit(d, limit, Text)
		if err == nil && strings.Count(string(out), "-id-") == 2 {
			two = limit
			if !strings.HasSuffix(string(out), "2 results. 6 more: --offset 6.\n") {
				t.Fatalf("two rows: %q", out)
			}
			break
		}
	}
	if two == 0 {
		t.Fatal("no limit gave two rows")
	}
	out, err := Fit(d, two, JSON)
	if err != nil {
		t.Fatal(err)
	}
	var v struct{ More, Next int }
	_ = json.Unmarshal(out, &v)
	if v.More == 0 {
		// JSON may fit a different count at the same limit; its own state must still be consistent.
		var w map[string]any
		_ = json.Unmarshal(out, &w)
		if rows := w["results"].([]any); len(rows) != 5 {
			t.Fatalf("no more with %d rows: %s", len(rows), out)
		}
	}
	// An offset at or past the end is an empty page with no continuation.
	past := listDoc(12, 13, 0, false, false, nil)
	out, err = Fit(past, 100, Text)
	if err != nil || string(out) != "search: queue\n0 results.\n" {
		t.Fatalf("past the end: %q %v", out, err)
	}
	out, err = Fit(past, 100, JSON)
	if err != nil || string(out) != `{"query":"queue"}`+"\n" {
		t.Fatalf("past the end JSON: %q %v", out, err)
	}
	// A non-positive limit is usage; inconsistent documents are io.
	var e *errs.Error
	if _, err := Fit(d, 0, Text); !errors.As(err, &e) || e.Code != "usage" || e.Exit != 2 {
		t.Fatalf("limit 0: %v", err)
	}
	bad := listDoc(3, 0, 3, false, false, nil)
	bad.Units[1].Main.Marks = []Mark{{Start: 5, End: 2}}
	if _, err := Fit(bad, 1000, Text); !errors.As(err, &e) || e.Code != "io" {
		t.Fatalf("bad marks: %v", err)
	}
	if _, err := Fit(&Doc{Total: 1, Foot: d.Foot}, 1000, Text); !errors.As(err, &e) || e.Code != "io" {
		t.Fatalf("a page with no units: %v", err)
	}
}

func TestTextAndJSONEscapes(t *testing.T) {
	d := listDoc(1, 0, 1, false, false, nil)
	d.Units[0].Main.JSON = Object{"title": "<a> & \"b\" \u2028 é", "count": 0, "s": ""}
	out, err := Fit(d, 1000, JSON)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `"<a> & \"b\" \u2028 é"`) || !strings.Contains(s, `"count":0`) {
		t.Fatalf("escaping: %s", s)
	}
	if strings.Count(s, "\n") != 1 {
		t.Fatalf("not one line: %q", s)
	}
}

// Alternatives never change the units shown or the continuation, at any limit.
func TestAlternativesPreservePage(t *testing.T) {
	with := listDoc(7, 2, 5, true, false, nil)
	without := listDoc(7, 2, 5, false, false, nil)
	for _, f := range []Format{Text, JSON} {
		for limit := 1; limit < 2500; limit++ {
			a, errA := Fit(with, limit, f)
			b, errB := Fit(without, limit, f)
			if (errA == nil) != (errB == nil) {
				t.Fatalf("limit %d: %v vs %v", limit, errA, errB)
			}
			if errA != nil {
				continue
			}
			ids := regexp.MustCompile(`2026112\d-id-é\d`)
			if strings.Join(ids.FindAllString(string(a), -1), ",") != strings.Join(ids.FindAllString(string(b), -1), ",") && f == Text {
				t.Fatalf("limit %d: different units\n%s\n%s", limit, a, b)
			}
			foot := regexp.MustCompile(`\d+ results?\.( \d+ more: --offset \d+\.)?|"more":\d+,"next":\d+`)
			if foot.FindString(string(a)) != foot.FindString(string(b)) {
				t.Fatalf("limit %d: continuation changed\n%s\n%s", limit, a, b)
			}
		}
	}
	// With room, the richest alternative is taken.
	out, _ := Fit(with, 5000, Text)
	if strings.Count(string(out), "context line") != 15 {
		t.Fatalf("richest alternatives not taken: %s", out)
	}
}

func TestExtrasAndHintsStopAtFirstMiss(t *testing.T) {
	d := listDoc(3, 0, 3, false, true, nil)
	out, _ := Fit(d, 5000, Text)
	if strings.Count(string(out), "relates to") != 3 {
		t.Fatalf("extras: %s", out)
	}
	empty := listDoc(0, 0, 0, false, false, []string{"postgres", "queue"})
	out, err := Fit(empty, 5000, Text)
	if err != nil || !strings.HasSuffix(string(out), "0 results. Alone: postgres 1, queue 2.\n") {
		t.Fatalf("hints: %q %v", out, err)
	}
	out, _ = Fit(empty, 5000, JSON)
	if !strings.Contains(string(out), `"alone":[{"clause":"postgres","count":1},{"clause":"queue","count":2}]`) {
		t.Fatalf("JSON hints: %s", out)
	}
	// Room for one hint only.
	full, _ := Fit(empty, 5000, Text)
	out, _ = Fit(empty, utf8.RuneCount(full)-len(", queue 2"), Text)
	if !strings.HasSuffix(string(out), "0 results. Alone: postgres 1.\n") {
		t.Fatalf("one hint: %q", out)
	}
}

// Colour is fitted after the plain output: removing the added pairs gives the plain output at
// every limit, and no output passes its limit.
func TestColourFitsAfterPlain(t *testing.T) {
	d := listDoc(7, 2, 5, true, true, nil)
	sawSome, sawPartial := false, false
	for limit := 1; limit < 3000; limit++ {
		plain, errP := Fit(d, limit, Text)
		col, errC := Fit(d, limit, TextColor)
		if (errP == nil) != (errC == nil) {
			t.Fatalf("limit %d: %v vs %v", limit, errP, errC)
		}
		if errP != nil {
			continue
		}
		if utf8.RuneCount(col) > limit {
			t.Fatalf("limit %d: coloured output passes the limit", limit)
		}
		if got := ansi.ReplaceAllString(string(col), ""); got != string(plain) {
			t.Fatalf("limit %d: stripped colour differs\n%q\n%q", limit, got, plain)
		}
		pairs := strings.Count(string(col), escReset)
		if pairs > 0 {
			sawSome = true
		}
		if pairs > 0 && pairs < strings.Count(string(plain), "-id-") {
			sawPartial = true
		}
	}
	if !sawSome || !sawPartial {
		t.Fatalf("colour never added (%v) or never partial (%v)", sawSome, sawPartial)
	}
	// A stored escape sequence in a part is printed as it is, with or without colour.
	raw := listDoc(1, 0, 1, false, false, nil)
	raw.Units[0].Main.Text += " \x1b[31mred"
	a, _ := Fit(raw, 2000, Text)
	b, _ := Fit(raw, 2000, TextColor)
	if !strings.Contains(string(a), "\x1b[31mred") || !strings.Contains(string(b), "\x1b[31mred") {
		t.Fatal("a stored escape was changed")
	}
}

func TestOneLine(t *testing.T) {
	for _, c := range []struct {
		in   string
		max  int
		want string
	}{
		{"a\r\nb\nc\rd\te\u2028f", 0, "a b c d e f"},
		{"héllo world", 5, "héll…"},
		{"héllo", 5, "héllo"},
		{"abc", 1, "…"},
		{"\x1b[31m", 0, " [31m"},
	} {
		if got := OneLine(c.in, c.max); got != c.want {
			t.Errorf("OneLine(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
	}
}

func TestExactPathProjection(t *testing.T) {
	for _, c := range []struct {
		path, display string
		quoted        bool
	}{
		{"docs/brain/decisions/a.md", "docs/brain/decisions/a.md", false},
		{"docs/my brain/a.md", "", true},
		{"docs/q\"uote/a.md", "", true},
		{"docs/back\\slash/a.md", "", true},
		{"docs/del\x7f/a.md", "", true},
		{"docs/c1\u0085/a.md", "", true},
		{"docs/ls\u2028/a.md", "", true},
		{"docs/rlo\u202e/a.md", "", true},
		{"docs/tag\U000E0001/a.md", "", true},
		{"docs/fffd\uFFFD/a.md", "docs/fffd\uFFFD/a.md", false},
		{"personal:notes/a.md", "personal:notes/a.md", false},
	} {
		display, enc, ok := ExactPath(c.path)
		if !ok {
			t.Fatalf("%q: not ok", c.path)
		}
		var back string
		if err := json.Unmarshal(enc, &back); err != nil || back != c.path {
			t.Fatalf("%q: token %s does not round-trip (%q, %v)", c.path, enc, back, err)
		}
		if c.quoted {
			if display != string(enc) || !strings.HasPrefix(display, "\"") {
				t.Errorf("%q: display %q, want the token %s", c.path, display, enc)
			}
			for _, r := range display {
				if r != ' ' && (r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) || r == 0x2028 || r == 0x202e) {
					t.Errorf("%q: raw control %U in %q", c.path, r, display)
				}
			}
		} else if display != c.display {
			t.Errorf("%q: display %q", c.path, display)
		}
		// The encoded token survives a row's encoding without a second escape.
		row, _ := encodeJSON(Object{"path": enc})
		if !strings.Contains(row, string(enc)) {
			t.Errorf("%q: re-escaped in %s", c.path, row)
		}
	}
	if _, enc, ok := ExactPath("docs/bad\xff/a.md"); ok || enc != nil {
		t.Fatal("invalid UTF-8 was printed")
	}
	if _, enc, _ := ExactPath("x\U000E0001"); string(enc) != `"x\udb40\udc01"` {
		t.Fatalf("surrogate pair: %s", enc)
	}
}

// Fitting N rows costs about N row measurements, not 1 + 2 + … + N: the size of the admitted rows is
// kept as they are admitted (render.md "Whole rows"). Bytes counted grow linearly with N.
func TestFitSizesIncrementally(t *testing.T) {
	counted := func(n int, f Format) int {
		d := &Doc{Head: []Part{{JSON: Envelope{Rows: "results"}}}, Total: n}
		for i := range n {
			d.Units = append(d.Units, Unit{Main: Part{Text: "r" + strconv.Itoa(i%10), JSON: Object{"r": i % 10}}})
		}
		d.Foot = func(State) Part { return Part{JSON: Object{}} }
		total := 0
		runesHook = func(s string, n int) int { total += len(s); return n }
		defer func() { runesHook = nil }()
		if _, err := Fit(d, 1<<30, f); err != nil {
			t.Fatal(err)
		}
		return total
	}
	for _, f := range []Format{Text, JSON} {
		small, large := counted(200, f), counted(1600, f)
		// Linear: eight times the rows, about eight times the bytes; quadratic would be about 64.
		if large > 12*small {
			t.Fatalf("format %d: %d bytes counted for 200 rows, %d for 1600", f, small, large)
		}
	}
}

// A page whose counts pass MaxInt fails with an internal error instead of wrapping to a size that
// would fit, in both formats; rolling back keeps the error until the page is measured again.
func TestPageSizeOverflowIsChecked(t *testing.T) {
	for _, f := range []Format{Text, JSON} {
		p := &page{d: &Doc{Units: make([]Unit, 2)}, f: f, cost: []int{math.MaxInt - 1, 5}}
		p.setShown(1)
		if _, err := p.size(); err != nil && f == Text {
			t.Fatalf("format %d: one unit below MaxInt: %v", f, err)
		}
		p.setShown(2)
		if n, err := p.size(); err == nil {
			t.Fatalf("format %d: size %d past MaxInt", f, n)
		} else if e := (*errs.Error)(nil); !errors.As(err, &e) || e.Code != "io" {
			t.Fatalf("format %d: %v", f, err)
		}
		p.setShown(1)
		if _, err := p.size(); err == nil {
			t.Fatalf("format %d: the overflow was forgotten", f)
		}
	}
	if _, ok := addChecked(math.MinInt, -1); ok {
		t.Fatal("negative overflow")
	}
	if n, ok := addChecked(math.MaxInt-1, 1); !ok || n != math.MaxInt {
		t.Fatal("MaxInt itself")
	}
}

// One text unit whose main line and extras together pass MaxInt marks the page, so its size fails
// instead of wrapping to a small one; JSON counts one encoded row per unit.
func TestUnitCostOverflowIsChecked(t *testing.T) {
	huge := math.MaxInt/2 + 1
	runesHook = func(s string, n int) int {
		if strings.HasPrefix(s, "HUGE") {
			return huge
		}
		return n
	}
	defer func() { runesHook = nil }()
	d := &Doc{Units: []Unit{{
		Main:   Part{Text: "HUGE main", JSON: Object{"id": "a"}},
		Extras: []Part{{Text: "HUGE extra", JSON: Object{"id": "b"}}},
	}}}
	p := &page{d: d, f: Text, shown: 1, chosen: []Part{d.Units[0].Main}, extras: []int{1}}
	p.resync()
	if n, err := p.size(); err == nil {
		t.Fatalf("size %d from a unit past MaxInt", n)
	}
	p.extras[0] = 0
	p.recost(0)
	if _, err := p.size(); err == nil {
		t.Fatal("the overflow was forgotten before the page was measured again")
	}
	p.resync()
	if n, err := p.size(); err != nil || n != huge+1 {
		t.Fatalf("the main line alone: %d %v", n, err)
	}
}
