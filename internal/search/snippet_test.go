package search

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/item"
)

func TestSnippetStretch(t *testing.T) {
	for _, tc := range []struct {
		name, title, body, query string
		start                    int
	}{
		{"late sole occurrence", "example", strings.Repeat(".", 1000) + "queue", "queue", 805},
		{"most distinct clauses", "example", "queue\n" + strings.Repeat(".", 250) + "queue rollback", "queue rollback", 70},
		{"phrase counts once", "example", "queue queue\n" + strings.Repeat(".", 250) + "job queue rollback", `"job queue" rollback`, 80},
		{"title only", "queue", strings.Repeat(".", 400), "queue", 0},
		{"multibyte prefix", "example", strings.Repeat("界", 250) + " queue", "queue", 56},
		{"short body", "example", "héllo queue", "queue", 0},
		{"match at start", "example", "queue " + strings.Repeat(".", 300), "queue", 0},
		{"overlong phrase", "example", "job" + strings.Repeat(" ", 300) + "queue", `"job queue"`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			f.put("a", tc.title, tc.body)
			r := f.search(t, tc.query)
			assertRows(t, r, "a")
			if len(r.Hits) != 1 || r.Hits[0].Snippet == nil || r.Hits[0].Snippet.Start != tc.start {
				t.Fatalf("snippet start, got %#v, want %d", r.Hits, tc.start)
			}
			s := r.Hits[0].Snippet
			runes := []rune(tc.body)
			end := min(len(runes), tc.start+200)
			if s.End != end || s.Text != string(runes[tc.start:end]) {
				t.Errorf("unexpected stretch %+v", s)
			}
		})
	}
}

func TestSnippetSection(t *testing.T) {
	for _, tc := range []struct {
		name, body, section string
		longBody            int
		start               int
	}{
		{"long named", "# section\n" + strings.Repeat(".", 300) + " queue", "section", 12, 116},
		{"short unnamed", "# section\nqueue\n", "", 100, 0},
		{"headingless", strings.Repeat(".", 300) + " queue", "", 10, 106},
		{"preamble", "queue\n" + strings.Repeat(".", 300) + "\n# heading", "", 10, 0},
		{"crossing heading names start section", "# earlier\n" + strings.Repeat(".", 490) + "\n# new\nqueu",
			"earlier", 10, 311},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := "queue"
			if tc.name == "crossing heading names start section" {
				q = "queu"
			}
			f := newFixture()
			f.put("a", "example", tc.body)
			r := f.search(t, q, func(req *Request) { req.LongBody = tc.longBody })
			if len(r.Hits) != 1 || r.Hits[0].Section != tc.section || r.Hits[0].Snippet.Start != tc.start {
				t.Fatalf("got %#v, want %q at %d", r.Hits, tc.section, tc.start)
			}
		})
	}
}

func TestSnippetLine(t *testing.T) {
	for _, tc := range []struct {
		name, file string
		query      string
		start      int
		line       int
	}{
		{"LF", "---\ntitle: example\n---\n\nqueue\n", "queue", 0, 5},
		{"BOM and CRLF", "\ufeff---\r\ntitle: preview\r\n---\r\n\r\n界\r\n" + strings.Repeat(".", 220) + "queue", "queue", 28, 6},
		{"short title-only", "---\ntitle: queue\n---\n\n🪵 unrelated", "queue", 0, 5},
		{"empty body", "---\ntitle: queue\n---", "queue", -1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(tc.file)
			h, at, _, err := item.Parse("a", data)
			if err != nil {
				t.Fatal(err)
			}
			f := newFixture()
			it := f.put("a", h.Title, string(data[at:]))
			it.Layout = item.Measure(data, at)
			res := f.search(t, tc.query)
			assertRows(t, res, "a")
			snippet := res.Hits[0].Snippet
			if tc.start < 0 {
				if snippet != nil {
					t.Fatalf("empty item had snippet %+v", snippet)
				}
			} else if snippet == nil || snippet.Start != tc.start || snippet.Line != tc.line {
				t.Fatalf("got %+v, want offset %d line %d", snippet, tc.start, tc.line)
			}
		})
	}
}

func TestContextLines(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		start, end int
		limit      int
		want       []ContextLine
	}{
		{"real LF only", "one\nnext\nthird\n", 0, 4, 1,
			[]ContextLine{{Line: 9, Text: "one", Focal: true}, {Line: 10, Text: "next"}}},
		{"CRLF no extra row", "one\r\nnext\r\n", 0, 5, 3,
			[]ContextLine{{Line: 9, Text: "one", Focal: true}, {Line: 10, Text: "next"}}},
		{"lone CR", "a\rb\na\r\n", 0, 4, 2,
			[]ContextLine{{Line: 9, Text: "a\rb", Focal: true}, {Line: 10, Text: "a"}}},
		{"exclusive end", "abc", 0, 2, 1,
			[]ContextLine{{Line: 9, Text: "ab", Focal: true, After: true}}},
		{"ends before CRLF", "a\r\n", 0, 1, 1,
			[]ContextLine{{Line: 9, Text: "a", Focal: true}}},
		{"ends at final LF", "a\r\n", 1, 3, 3,
			[]ContextLine{{Line: 9, Focal: true, Before: true}}},
		{"neighbours and trim flags", "before2\nbefore1\nabc\nnext", 17, 18, 2,
			[]ContextLine{{Line: 7, Text: "before2"}, {Line: 8, Text: "before1"},
				{Line: 9, Text: "b", Focal: true, Before: true, After: true}, {Line: 10, Text: "next"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := contextLines([]byte(tc.data), tc.start, tc.end, 9, tc.limit); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("context = %+v, want %+v", got, tc.want)
			}
		})
	}
	f := newFixture()
	f.put("a", "example", "queue\n"+strings.Repeat(".", 512000))
	if got := f.search(t, "queue").Hits[0].Snippet.Context; got != nil {
		t.Fatalf("context appeared with no flag: %+v", got)
	}
	r := f.search(t, "queue", func(req *Request) { req.Context, req.Snippet = 1, 6 })
	got := r.Hits[0].Snippet.Context
	if len(got) != 2 || !got[0].Focal || len(got[1].Text) != 512000 {
		t.Fatal("search truncated a context neighbour")
	}
}

func TestOccurrences(t *testing.T) {
	f := newFixture()
	a := f.put("a", "queue queue", "Queue queue queue")
	b := f.put("b", "queue queue", "queue")
	f.reads = make(map[*item.Item]int)
	for _, bm25 := range []bool{false, true} {
		r := f.search(t, "queue QUEUE", func(req *Request) { req.BM25, req.Limit = bm25, 1 })
		assertRows(t, r, "a")
		if r.Hits[0].Occurrences != 5 {
			t.Errorf("BM25=%v: count %d", bm25, r.Hits[0].Occurrences)
		}
		if bm25 && (f.reads[a] != 2 || f.reads[b] != 1) || !bm25 && (f.reads[a] != 1 || f.reads[b] != 0) {
			t.Errorf("BM25=%v: page body reads = %d %d", bm25, f.reads[a], f.reads[b])
		}
		clear(f.reads)
	}
	fixture2 := newFixture()
	phrase := fixture2.put("phrase", "a", "a a a")
	phrase.Header.Tags = []string{"a", "a", "a-a"}
	r := fixture2.search(t, `"a a" "A A"`)
	assertRows(t, r, "phrase")
	if r.Hits[0].Occurrences != 3 {
		t.Errorf("overlaps and field boundaries: got %d, want 3", r.Hits[0].Occurrences)
	}
	fixture3 := newFixture()
	fixture3.put("long", "title", "a"+strings.Repeat(" ", 220)+"b")
	r = fixture3.search(t, `"a b"`)
	if len(r.Hits) != 1 || r.Hits[0].Occurrences != 1 || r.Hits[0].Snippet.Start != 0 {
		t.Fatalf("overlong occurrence should be counted and ignored for coverage: %+v", r)
	}
}

func TestBestStretchAgainstExhaustive(t *testing.T) {
	found := []SnippetOccurrence{{0, 2, 0}, {2, 4, 0}, {4, 6, 1}, {7, 9, 2}, {8, 9, 0}, {10, 11, 2}}
	for length := 1; length < 14; length++ {
		best, coverage := 0, 0
		for start := 0; start <= max(0, 11-length); start++ {
			seen := map[int]bool{}
			for _, o := range found {
				if start <= o.Start && o.End <= start+length {
					seen[o.Clause] = true
				}
			}
			if len(seen) > coverage {
				best, coverage = start, len(seen)
			}
		}
		if got := SnippetStart(11, length, 3, slices.Clone(found)); got != best {
			t.Errorf("length %d: got %d, exhaustive %d", length, got, best)
		}
	}
}
