package search

import (
	"context"
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/graph"
	"github.com/pkieltyka/pbrain/internal/item"
)

// fakeSemantic returns fixed scores and records what it was asked.
type fakeSemantic struct {
	scores   Scores
	err      error
	cancel   context.CancelFunc
	calls    int
	text     string
	eligible []*item.Item
}

func (s *fakeSemantic) Score(_ context.Context, text string, eligible []*item.Item) (Scores, error) {
	s.calls++
	s.text, s.eligible = text, slices.Clone(eligible)
	if s.cancel != nil {
		s.cancel()
	}
	return s.scores, s.err
}

func withSemantic(s Semantic) func(*Request) { return func(r *Request) { r.Semantic = s } }

func ids(items []*item.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

func TestSemanticMerge(t *testing.T) {
	f := newFixture()
	lex := f.put("lex", "redis cache", "redis is the cache")
	near := f.put("near", "sessions", "sessions expire through a time to live; more text follows here")
	tie1 := f.put("tie-b", "tie", "tied passage")
	tie2 := f.put("tie-a", "tie", "tied passage")
	older := f.put("older", "older", "older passage")
	older.Header.Created = now.Add(-48 * time.Hour)
	titleOnly := f.put("title-only", "a title", "unrelated body")
	nan := f.put("nan", "nan", "x")
	low := f.put("low", "low", "x")
	outside := f.put("outside", "outside", "x")
	repl := f.put("replaced", "old sessions", "replaced body")
	f.put("successor", "successor", "new").Header.Replaces = []string{"replaced"}
	f.put("learning", "a learning", "x").Header.Kind = item.Learning
	src := &fakeSemantic{scores: Scores{Ready: true, Cutoff: 0.5, Missing: 2, Hits: []SemanticHit{
		{Item: lex, Score: 0.99, Start: 0, End: 5},  // matched lexically: never twice
		{Item: near, Score: 0.9, Start: 9, End: 15}, // "expire"
		{Item: near, Score: 0.95, Start: 0, End: 8}, // a duplicate: the first kept
		{Item: tie1, Score: 0.7, Start: 0, End: 4},  // ties: by date, then id, then path
		{Item: tie2, Score: 0.7, Start: 0, End: 4},
		{Item: older, Score: 0.7, Start: 0, End: 5},
		{Item: titleOnly, Score: 0.6, Start: 0, End: 0}, // an empty span: no snippet
		{Item: nan, Score: float32(math.NaN()), End: 1}, // dropped
		{Item: low, Score: 0.4999999, End: 1},           // just under the cut-off: dropped
		{Item: outside, Score: 0.8, Start: 0, End: 99},  // past the body's end: dropped
		{Item: repl, Score: 0.95, Start: 0, End: 8},     // replaced, without All: not eligible
	}}}
	res := f.search(t, "redis", withSemantic(src))
	assertRows(t, res, "lex", "near", "tie-a", "tie-b", "older", "title-only")
	if res.Total != 6 || !res.Sem.Asked || !res.Sem.Ready || res.Sem.Missing != 2 || res.Alone != nil {
		t.Fatalf("result: total %d, sem %+v", res.Total, res.Sem)
	}
	// Score got the query text and the eligible set only, in catalog order.
	if src.calls != 1 || src.text != "redis" || slices.Contains(ids(src.eligible), "replaced") || len(src.eligible) != len(f.items)-1 {
		t.Fatalf("Score was asked %d times with %q and %q", src.calls, src.text, ids(src.eligible))
	}
	// The lexical row is unchanged by the source.
	plain := f.search(t, "redis")
	if !reflect.DeepEqual(plain.Hits[0], res.Hits[0]) || plain.Replaced != res.Replaced {
		t.Fatal("the semantic part changed a lexical row")
	}
	for _, h := range res.Hits[1:] {
		if !h.Semantic || h.Class != 0 {
			t.Fatalf("%s is not marked semantic", h.Item.ID)
		}
	}
	// The snippet lies inside the passage; an empty span has none.
	if s := res.Hits[1].Snippet; s == nil || s.Text != "expire" || s.Start != 9 || s.End != 15 || s.Line == 0 {
		t.Fatalf("near's snippet: %+v", s)
	}
	if res.Hits[5].Snippet != nil {
		t.Fatal("an empty span has a snippet")
	}
	// Paging across the joint at page size one visits each row once.
	for i, id := range []string{"lex", "near", "tie-a", "tie-b", "older", "title-only"} {
		page := f.search(t, "redis", withSemantic(src), func(r *Request) { r.Offset, r.Limit = i, 1 })
		assertRows(t, page, id)
	}
	// With All, the replaced item is eligible and follows the current added hits.
	all := f.search(t, "redis", withSemantic(src), func(r *Request) { r.Filters.All = true })
	assertRows(t, all, "lex", "near", "tie-a", "tie-b", "older", "title-only", "replaced")
	if !all.Hits[6].Replaced {
		t.Fatal("the replaced added hit is not marked")
	}
	// A kind filter and a window leave items out of Score's set.
	f.search(t, "redis", withSemantic(src), func(r *Request) { r.Filters.Kinds = []item.Kind{item.Learning} })
	if !reflect.DeepEqual(ids(src.eligible), []string{"learning"}) {
		t.Fatalf("--kind learning: %q", ids(src.eligible))
	}
	// No result at all: Alone only when all three groups are empty.
	empty := f.search(t, "nothingmatches", withSemantic(src))
	if empty.Alone != nil {
		t.Fatal("Alone with semantic hits")
	}
	src.scores.Hits = nil
	if empty = f.search(t, "nothingmatches", withSemantic(src)); empty.Alone == nil || empty.Total != 0 {
		t.Fatal("no Alone with every group empty")
	}
}

func TestSemanticNotReadyAndPartial(t *testing.T) {
	f := newFixture()
	f.put("lex", "redis", "redis")
	other := f.put("other", "other", "other")
	lexOnly := f.search(t, "redis")
	for _, reason := range []NotReady{NoModel, UnsupportedModel, ModelAbsent, CacheExposed, NoIndex, ModelFailed} {
		src := &fakeSemantic{scores: Scores{Reason: reason, Hits: []SemanticHit{{Item: other, Score: 1, End: 1}}}}
		res := f.search(t, "redis", withSemantic(src))
		if !reflect.DeepEqual(res.Hits, lexOnly.Hits) || res.Sem != (SemanticState{Asked: true, Reason: reason}) {
			t.Errorf("%v: %+v", reason, res.Sem)
		}
	}
	for name, src := range map[string]*fakeSemantic{
		"an error":            {err: errors.New("runtime"), scores: Scores{Ready: true, Cutoff: 0.5}},
		"a NaN cut-off":       {scores: Scores{Ready: true, Cutoff: float32(math.NaN()), Hits: []SemanticHit{{Item: other, Score: 1, End: 1}}}},
		"an infinite cut-off": {scores: Scores{Ready: true, Cutoff: float32(math.Inf(-1)), Hits: []SemanticHit{{Item: other, Score: 1, End: 1}}}},
	} {
		res := f.search(t, "redis", withSemantic(src))
		if res.Total != 1 || res.Sem != (SemanticState{Asked: true, Reason: ModelFailed}) {
			t.Errorf("%s: total %d, %+v", name, res.Total, res.Sem)
		}
	}
	// A cancelled context returns its error, also with nothing on the page.
	ctx, cancel := context.WithCancel(context.Background())
	src := &fakeSemantic{cancel: cancel, scores: Scores{Ready: true, Cutoff: 0.5}}
	if _, err := Run(ctx, f, graph.Build(f.items), Request{Clauses: clauses(t, "nothingmatches"), Text: "nothingmatches", Limit: 5, Now: now, Semantic: src}); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled Score: %v", err)
	}
	// Without the source Score is never called and Sem is zero.
	if res := f.search(t, "redis"); res.Sem != (SemanticState{}) {
		t.Fatal("Sem set without --semantic")
	}
}

func TestMergedListThreeGroups(t *testing.T) {
	f := newFixture()
	f.put("exact", "queue retry timeout", "")
	f.put("partial", "queue retry", "")
	both := f.put("both", "queue retry", "")
	meaning := f.put("meaning", "unrelated words", "unrelated")
	filler(f, 17)
	src := &fakeSemantic{scores: Scores{Ready: true, Cutoff: 0.5, Hits: []SemanticHit{
		{Item: both, Score: 0.6, End: 0}, {Item: meaning, Score: 0.9, End: 0},
	}}}
	strict := f.search(t, "queue retry timeout")
	res := f.search(t, "queue retry timeout", withBM25, withSemantic(src))
	// Exact, then found by meaning, then partial; an item both semantic and partial is semantic.
	assertRows(t, res, "exact", "meaning", "both", "partial")
	if res.Total != 4 || !reflect.DeepEqual(res.Hits[0], strict.Hits[0]) || res.Replaced != strict.Replaced || res.Outside != strict.Outside {
		t.Fatalf("the merged list: %+v", res)
	}
	if !res.Hits[2].Semantic || res.Hits[2].Partial || !res.Hits[3].Partial {
		t.Fatal("markers of the merged rows")
	}
	// The model not ready: exact and partial, with Sem set.
	src.scores.Ready, src.scores.Reason = false, NoIndex
	res = f.search(t, "queue retry timeout", withBM25, withSemantic(src))
	assertRows(t, res, "exact", "both", "partial")
	if res.Sem.Reason != NoIndex {
		t.Fatal(res.Sem)
	}
	// Paging at page size one visits every row of the whole list once.
	src.scores.Ready = true
	for i, id := range []string{"exact", "meaning", "both", "partial"} {
		page := f.search(t, "queue retry timeout", withBM25, withSemantic(src), func(r *Request) { r.Offset, r.Limit = i, 1 })
		assertRows(t, page, id)
	}
}
