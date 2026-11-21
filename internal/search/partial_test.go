package search

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/graph"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/query"
)

func withBM25(req *Request) { req.BM25 = true }

func filler(f *fixture, count int) {
	for i := range count {
		f.put(fmt.Sprintf("filler-%02d", i), "unrelated", "")
	}
}

func TestPartialAdmission(t *testing.T) {
	f := newFixture()
	f.put("with-timeout", "queue timeout", "")
	f.put("with-retry", "queue retry", "")
	f.put("queue-only", "queue", "")
	filler(f, 17)
	r := f.search(t, "queue retry timeout", withBM25)
	assertRows(t, r, "with-retry", "with-timeout")
	for _, h := range r.Hits {
		if !h.Partial || h.Matched != 2 || h.Clauses != 3 || h.Class != 0 || h.Semantic {
			t.Fatalf("unexpected partial hit %+v", h)
		}
	}
	if len(r.Alone) != 0 {
		t.Fatal("partial rows suppress empty-match diagnostics")
	}

	assertRows(t, f.search(t, "queue retry tim*", withBM25), "with-timeout")
	for _, text := range []string{`"retry timeout" queue missing`, "queue retry futile*", "queue retry", "absent", "queue absent absent2 absent3 absent4"} {
		r := f.search(t, text, withBM25)
		if text == "queue retry" {
			assertRows(t, r, "with-retry")
			if r.Hits[0].Partial {
				t.Fatal("two-clause hit is exact")
			}
		} else if r.Total != 0 || r.Alone == nil {
			t.Errorf("%q: admitted suppressed rows or lost hints: %+v", text, r)
		}
	}
	for i := range 6 {
		f.put(fmt.Sprintf("common-%d", i), "queue", "")
	}
	if got := f.search(t, "queue retry timeout", withBM25); got.Total != 0 {
		t.Fatal("a common word counted as informative")
	}

	small := newFixture()
	small.put("candidate", "queue retry", "")
	filler(small, 2)
	if got := small.search(t, "queue retry timeout", withBM25); got.Total != 0 {
		t.Fatal("N < 4 admitted a partial candidate")
	}
}

func TestPartialScoreAndOrder(t *testing.T) {
	f := newFixture()
	a := f.put("a", "retry retry", "queue queue queue")
	a.Header.Tags = []string{"queue-retry", "retry"}
	f.put("b", "queue retry", "")
	filler(f, 18)
	req := Request{Clauses: clauses(t, "queue retry timeout"), BM25: true}
	records := collect(f.items, graph.Build(f.items), &req.Filters)
	p := plan(req.Clauses)
	if err := matchAll(context.Background(), f, &p, records, true); err != nil {
		t.Fatal(err)
	}
	if got := records[0].tf; !slices.Equal(got, []int{4, 4, 0}) || records[0].length != 8 {
		t.Fatalf("tf %v, length %d", got, records[0].length)
	}
	s := stats(records, len(req.Clauses))
	const (
		N   = 20.0
		df  = 2.0
		avg = (8.0 + 2.0 + 18.0) / N
	)
	// Calculate the two equal contributions independently of the statistics pass.
	part := math.Log(1+(N-df+0.5)/(df+0.5)) * 4 * 2.2 / (4 + 1.2*(0.25+0.75*8/avg))
	if got := s.score(&records[0]); math.Float64bits(got) != math.Float64bits(part+part) {
		t.Errorf("score %.20g, want %.20g", got, part+part)
	}
	assertRows(t, f.search(t, "queue retry timeout", withBM25), "a", "b")

	replacer := f.put("replacement", "other", "")
	replacer.Header.Replaces = []string{"a"}
	assertRows(t, f.search(t, "queue retry timeout", withBM25, func(r *Request) { r.Filters.All = true }), "b", "a")

	// Same score: known creation first, then folded id and path.
	b := f.items[1]
	tied := f.put("z", b.Header.Title, "")
	tied.Header.Created = now.AddDate(0, 0, 1)
	assertRows(t, f.search(t, "queue retry timeout", withBM25), "z", "b")
}

func TestPartialQuotedWord(t *testing.T) {
	f := newFixture()
	f.put("a", "queue retry", "")
	filler(f, 8)
	assertRows(t, f.search(t, "durable queue retry", withBM25), "a")
	assertRows(t, f.search(t, `"durable" queue retry`, withBM25))
	bare := f.search(t, "durable queue retry")
	quoted := f.search(t, `"durable" queue retry`)
	if len(bare.Alone) != 3 || len(quoted.Alone) != 3 || bare.Alone[0].Count != quoted.Alone[0].Count {
		t.Fatal("quoting a word changed exact clause counts")
	}
}

func TestPartialStatisticsIgnoreOtherGroups(t *testing.T) {
	f := newFixture()
	f.put("candidate", "queue retry", "")
	f.put("other-group", "related only by meaning", "")
	filler(f, 2)
	req := Request{Clauses: clauses(t, "queue retry timeout"), BM25: true}
	records := collect(f.items, graph.Build(f.items), &req.Filters)
	p := plan(req.Clauses)
	if err := matchAll(context.Background(), f, &p, records, true); err != nil {
		t.Fatal(err)
	}
	s := stats(records, len(req.Clauses))
	if s.n != 4 || !reflect.DeepEqual(s.rare, []bool{true, true, true}) {
		t.Fatalf("stats for the full eligible corpus: %+v", s)
	}
	ranked := rankPartial(records, req.Clauses)
	if len(ranked) != 1 || ranked[0].hit.Item.ID != "candidate" {
		t.Fatalf("wrong candidate at the quarter threshold: %+v", ranked)
	}
	// Removing an earlier group's row before computing statistics would reject it.
	if restricted := rankPartial(slices.Delete(slices.Clone(records), 1, 2), req.Clauses); len(restricted) != 0 {
		t.Fatal("the fixture must distinguish the corpus from output candidates")
	}
}

func TestPartialCorpusIsTheView(t *testing.T) {
	f := newFixture()
	f.put("a", "queue retry", "")
	filler(f, 3)
	want := f.search(t, "queue retry timeout", withBM25)
	assertRows(t, want, "a")
	shared := slices.Clone(f.items)
	personal := f.put("personal", "queue retry", "")
	personal.Section, personal.Header.Kind = item.Personal, item.Observation
	assertRows(t, f.search(t, "queue retry timeout", withBM25))
	assertRows(t, f.search(t, "queue retry timeout", withBM25, func(req *Request) {
		req.Filters.Kinds = []item.Kind{item.Note}
	}), "a")
	personal.Header.Created = now.AddDate(-1, 0, 0)
	window, err := query.ParseWindow("7d", "", now, now.Location())
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, f.search(t, "queue retry timeout", withBM25, func(req *Request) {
		req.Filters.Window = window
	}), "a")
	f.items = shared
	if got := f.search(t, "queue retry timeout", withBM25); !reflect.DeepEqual(got, want) {
		t.Fatal("the shared result changed although the personal item was outside the corpus")
	}
}

func TestMergedListExactAndPartial(t *testing.T) {
	f := newFixture()
	exact := f.put("exact", "queue retry timeout", "")
	exact.Header.Replaces = []string{"old"}
	f.put("old", "queue retry timeout", "")
	f.put("partial", "queue retry", "")
	filler(f, 17)
	strict := f.search(t, "queue retry timeout")
	assertRows(t, strict, "exact")
	with := f.search(t, "queue retry timeout", withBM25)
	assertRows(t, with, "exact", "partial")
	if !reflect.DeepEqual(with.Hits[0], strict.Hits[0]) || with.Replaced != strict.Replaced || with.Outside != strict.Outside {
		t.Fatal("the partial tier changed exact rows or counts")
	}
	for i, id := range []string{"exact", "partial"} {
		r := f.search(t, "queue retry timeout", withBM25, func(req *Request) { req.Offset, req.Limit = i, 1 })
		assertRows(t, r, id)
		if r.Total != 2 {
			t.Errorf("paged total %d, want 2", r.Total)
		}
	}
}

func TestBM25OffReadsAsToday(t *testing.T) {
	f := newFixture()
	it := f.put("a", "later", "queue "+strings.Repeat("middle ", 200)+"later")
	p := plan(clauses(t, "queue later"))
	for _, bm25 := range []bool{false, true} {
		rec := match{eligible: true, item: it}
		m := p.matcher()
		if err := m.record(f, &rec, bm25); err != nil {
			t.Fatal(err)
		}
		if rec.class != 3 || (rec.mask[1]&inBody != 0) != bm25 {
			t.Fatalf("BM25=%v: matching should stop at the first exact match only without BM25: %+v", bm25, rec)
		}
		if (rec.tf != nil) != bm25 {
			t.Fatalf("BM25=%v had tf %v", bm25, rec.tf)
		}
	}
}

func BenchmarkRun5000(b *testing.B) {
	f := newFixture()
	base := strings.Repeat("the queue uses bounded retries and backoff under load\n", 200)
	text := []byte(base)
	for i := range 5000 {
		it := &item.Item{ID: fmt.Sprint(i), Key: fmt.Sprint(i), Path: fmt.Sprint(i),
			Header: item.Header{Kind: item.Note, Title: "operational notes",
				Created: now.AddDate(0, 0, -(i % 360)), CreatedKnown: true}}
		it.Layout = item.Layout{BodyLine: 4, BodyChars: len(text)}
		f.items = append(f.items, it)
		f.body[it] = text
	}
	g := graph.Build(f.Items())
	for _, bm25 := range []bool{false, true} {
		for _, q := range []struct {
			name, text string
			filters    Filters
		}{
			{"common", "queue", Filters{}},
			{"rare", "load-balancer", Filters{}},
			{"phrase", `"bounded retries"`, Filters{}},
			{"prefix", "retr*", Filters{}},
			{"all", "queue", Filters{All: true}},
			{"window", "queue", Filters{Window: query.Window{Since: now.AddDate(0, 0, -7), HasSince: true}}},
		} {
			b.Run(fmt.Sprintf("%s/bm25=%t", q.name, bm25), func(b *testing.B) {
				req := Request{Clauses: clauses(b, q.text), Limit: 5, Now: now, Snippet: 200, LongBody: 6000,
					Filters: q.filters, BM25: bm25}
				b.ReportAllocs()
				for b.Loop() {
					if _, err := Run(context.Background(), f, g, req); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
