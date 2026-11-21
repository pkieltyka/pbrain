package cli

import (
	"context"
	"encoding/json"
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
