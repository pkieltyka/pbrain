package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/graph"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/query"
	"github.com/pkieltyka/pbrain/internal/render"
	"github.com/pkieltyka/pbrain/internal/search"
	"github.com/pkieltyka/pbrain/internal/store"
)

func mustParse(t *testing.T, q string) []query.Clause {
	t.Helper()
	cs, err := query.Parse(q)
	if err != nil {
		t.Fatal(err)
	}
	return cs
}

// fakeSource scores one item by meaning, whatever the query.
type fakeSource struct{ id string }

func (s fakeSource) Score(_ context.Context, _ string, eligible []*item.Item) (search.Scores, error) {
	out := search.Scores{Ready: true, Cutoff: 0.5}
	for _, it := range eligible {
		if it.ID == s.id {
			out.Hits = append(out.Hits, search.SemanticHit{Item: it, Score: 0.9, Start: 0, End: min(8, it.Layout.BodyChars)})
		}
	}
	return out, nil
}

// hybridPage runs one search with a semantic source and fits it, as phase 23's cli will.
func hybridPage(t *testing.T, rb *readBrain, query string, src search.Semantic, bm25 bool, limit, offset, budget int, f render.Format) ([]byte, error) {
	t.Helper()
	ctx := context.Background()
	b, err := brain.Open(ctx, brain.Env{Dir: rb.env.Dir, Getenv: rb.env.Getenv, Now: rb.env.Now, Files: rb.env.Files}, brain.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	cat, err := store.Scan(ctx, b, brain.SharedView, store.KeepAll)
	if err != nil {
		t.Fatal(err)
	}
	in, err := parseOptions([]string{"search", query}, Features{})
	if err != nil {
		t.Fatal(err)
	}
	in.format, in.now = f, rb.env.Now()
	r := &reader{in: &in, b: b, cat: cat, g: graph.Build(cat.Items()), now: in.now}
	res, err := search.Run(ctx, cat, r.g, search.Request{Clauses: mustParse(t, query), Text: query, Limit: limit, Offset: offset,
		Now: in.now, Snippet: 200, LongBody: 6000, Semantic: src, BM25: bm25})
	if err != nil {
		t.Fatal(err)
	}
	return render.Fit(r.searchDoc(res), budget, f)
}

// TestHybridContinuationRaisesMinimum is render.md's test: an added row after one exact row raises
// the minimum budget by its continuation, and paging visits each row once in tier order.
func TestHybridContinuationRaisesMinimum(t *testing.T) {
	rb := cacheBrain(t)
	for _, f := range []render.Format{render.Text, render.JSON} {
		// The exact-only page at its own minimum budget.
		smallest := func(src search.Semantic) int {
			for n := 50; n < 5000; n++ {
				if _, err := hybridPage(t, rb, "sessions", src, false, 5, 0, n, f); err == nil {
					return n
				}
			}
			t.Fatal("nothing fits")
			return 0
		}
		minimum := smallest(nil)
		exactOnly, _ := hybridPage(t, rb, "sessions", nil, false, 5, 0, minimum, f)
		if _, err := hybridPage(t, rb, "sessions", fakeSource{costID}, false, 5, 0, minimum, f); domainError(err).Code != "budget_too_small" {
			t.Fatalf("format %d: the exact-only minimum fitted the hybrid page: %v", f, err)
		}
		// The hybrid minimum keeps the exact row unchanged, with more 1 and next 1.
		need := smallest(fakeSource{costID})
		out, _ := hybridPage(t, rb, "sessions", fakeSource{costID}, false, 5, 0, need, f)
		if f == render.Text {
			if !strings.HasPrefix(string(out), strings.TrimSuffix(string(exactOnly), "1 result. 1 replaced item also matched: --all.\n")) ||
				!strings.HasSuffix(string(out), "1 result. 1 more: --offset 1. 1 replaced item also matched: --all.\n") {
				t.Fatalf("hybrid minimum page:\n%s\nexact-only:\n%s", out, exactOnly)
			}
		} else {
			var page struct {
				Results []map[string]any `json:"results"`
				More    int              `json:"more"`
				Next    int              `json:"next"`
			}
			json.Unmarshal(out, &page)
			if len(page.Results) != 1 || page.More != 1 || page.Next != 1 || page.Results[0]["id"] != redisID {
				t.Fatalf("hybrid minimum page: %s", out)
			}
		}
	}
	// The added row, by itself: its marker and fields, and no lexical fields that do not apply.
	out, err := hybridPage(t, rb, "sessions", fakeSource{costID}, false, 1, 1, 3000, render.Text)
	if err != nil || !strings.HasPrefix(string(out), costID+"  note  provisional  2026-11-20  semantic\n") || !strings.Contains(string(out), "[body L6, 0C; file 5L/82C; 0 occurrences]") {
		t.Fatalf("the semantic row: %v\n%s", err, out)
	}
	out, _ = hybridPage(t, rb, "sessions", fakeSource{costID}, false, 1, 1, 3000, render.JSON)
	var page struct{ Results []map[string]any }
	json.Unmarshal(out, &page)
	r := page.Results[0]
	if r["semantic"] != true || r["occurrences"] != 0.0 || r["partial"] != nil || r["snippet"] != nil {
		t.Fatalf("the semantic JSON row: %s", out)
	}
}

// A partial row renders its marker and fields; an exact row on the same page has none of them.
func TestHybridPartialRow(t *testing.T) {
	rb := cacheBrain(t)
	ctx := context.Background()
	b, err := brain.Open(ctx, brain.Env{Dir: rb.env.Dir, Getenv: rb.env.Getenv, Now: rb.env.Now, Files: rb.env.Files}, brain.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	cat, err := store.Scan(ctx, b, brain.SharedView, store.KeepAll)
	if err != nil {
		t.Fatal(err)
	}
	var redis, cost *item.Item
	for _, it := range cat.Items() {
		switch it.ID {
		case redisID:
			redis = it
		case costID:
			cost = it
		}
	}
	for _, f := range []render.Format{render.Text, render.JSON} {
		in, _ := parseOptions([]string{"search", "redis"}, Features{})
		in.format, in.now = f, rb.env.Now()
		r := &reader{in: &in, b: b, cat: cat, g: graph.Build(cat.Items()), now: in.now}
		res := &search.Result{Total: 2, Hits: []search.Hit{
			{Item: redis, Class: 3, Occurrences: 1},
			{Item: cost, Partial: true, Matched: 2, Clauses: 3, Occurrences: 2},
		}}
		out, err := render.Fit(r.searchDoc(res), 3000, f)
		if err != nil {
			t.Fatal(err)
		}
		if f == render.Text {
			if !strings.Contains(string(out), costID+"  note  provisional  2026-11-20  partial 2/3\n") || strings.Contains(string(out), redisID+"  decision  verified  2026-11-19  partial") {
				t.Fatalf("partial text:\n%s", out)
			}
			continue
		}
		var page struct{ Results []map[string]any }
		json.Unmarshal(out, &page)
		if p := page.Results[1]; p["partial"] != true || p["matched"] != 2.0 || p["clauses"] != 3.0 || p["semantic"] != nil {
			t.Fatalf("partial JSON: %s", out)
		}
		if e := page.Results[0]; e["partial"] != nil || e["matched"] != nil || e["clauses"] != nil {
			t.Fatalf("exact JSON row with partial fields: %s", out)
		}
	}
}

// The hybrid minimum with a partial row, and with all three tiers (render.md
// TestHybridContinuationRaisesMinimum): in both formats the exact-only minimum fails, the minimum keeps
// the exact row alone with more equal to the added rows and next 1, and one less fails.
func TestHybridMinimumAllTiers(t *testing.T) {
	rb := cacheBrain(t)
	const extraID = "20261121-000000-extra-x9x9"
	rb.put("notes/"+extraID+".md", []string{"kind: note", "title: Session store sizing", "created: 2026-11-21T08:00:00Z"}, "sizing\n")
	ctx := context.Background()
	b, err := brain.Open(ctx, brain.Env{Dir: rb.env.Dir, Getenv: rb.env.Getenv, Now: rb.env.Now, Files: rb.env.Files}, brain.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	cat, err := store.Scan(ctx, b, brain.SharedView, store.KeepAll)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]*item.Item{}
	for _, it := range cat.Items() {
		byID[it.ID] = it
	}
	exact := search.Hit{Item: byID[redisID], Class: 3, Occurrences: 1}
	partial := search.Hit{Item: byID[costID], Partial: true, Matched: 5, Clauses: 6, Occurrences: 5}
	semantic := search.Hit{Item: byID[extraID], Semantic: true}
	for _, f := range []render.Format{render.Text, render.JSON} {
		in, _ := parseOptions([]string{"search", "redis"}, Features{})
		in.format, in.now = f, rb.env.Now()
		r := &reader{in: &in, b: b, cat: cat, g: graph.Build(cat.Items()), now: in.now}
		fit := func(hits []search.Hit, budget int) ([]byte, error) {
			return render.Fit(r.searchDoc(&search.Result{Total: len(hits), Hits: hits}), budget, f)
		}
		smallest := func(hits []search.Hit) int {
			for n := 1; n < 5000; n++ {
				if _, err := fit(hits, n); err == nil {
					return n
				}
			}
			t.Fatal("nothing fits")
			return 0
		}
		exactMin := smallest([]search.Hit{exact})
		exactOnly, _ := fit([]search.Hit{exact}, exactMin)
		for _, hits := range [][]search.Hit{{exact, partial}, {exact, semantic, partial}} {
			added := len(hits) - 1
			if _, err := fit(hits, exactMin); domainError(err).Code != "budget_too_small" {
				t.Fatalf("format %d, %d added: the exact-only minimum fitted: %v", f, added, err)
			}
			need := smallest(hits)
			if _, err := fit(hits, need-1); domainError(err).Code != "budget_too_small" {
				t.Fatalf("format %d, %d added: one under the minimum: %v", f, added, err)
			}
			out, err := fit(hits, need)
			if err != nil {
				t.Fatal(err)
			}
			if f == render.Text {
				row := strings.TrimSuffix(string(exactOnly), "1 result.\n")
				if !strings.HasPrefix(string(out), row) || string(out)[len(row):] != fmt.Sprintf("1 result. %d more: --offset 1.\n", added) {
					t.Fatalf("%d added, text minimum page:\n%s\nexact-only:\n%s", added, out, exactOnly)
				}
				continue
			}
			var page struct {
				Results []map[string]any `json:"results"`
				More    int              `json:"more"`
				Next    int              `json:"next"`
			}
			if err := json.Unmarshal(out, &page); err != nil || len(page.Results) != 1 || page.More != added || page.Next != 1 ||
				page.Results[0]["id"] != redisID || page.Results[0]["partial"] != nil || page.Results[0]["semantic"] != nil {
				t.Fatalf("%d added, JSON minimum page: %s", added, out)
			}
		}
	}
}
