package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/brain"
)

func TestComparisonUsesProductionSnippet(t *testing.T) {
	const input = "queue retry timeout backoff worker lease"
	const evidence = "A sealed folder holds the handoff ledger."
	for _, tc := range []struct {
		name, body string
		long       bool
	}{
		{"later dense matches", "queue " + strings.Repeat("noise ", 1300) + "\n## Recovery\nretry backoff worker lease.\n" + evidence, true},
		{"whole short body", strings.Repeat("x ", 32) + "queue retry backoff worker lease.\n" + evidence, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := readerFixture(t, tc.body)
			const filler = "---\nkind: note\ntitle: Unrelated\ncreated: 2026-01-01T00:00:00Z\nauthor: agent:test\n---\n\nOther material.\n"
			for i := range 6 {
				if err := os.WriteFile(filepath.Join(s.dir, fmt.Sprintf("filler-%d.md", i)), []byte(filler), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var err error
			s.corpus, err = scan(t.Context(), s.dir, brain.SharedView)
			if err != nil {
				t.Fatal(err)
			}
			real, err := s.search(input, true)
			if err != nil {
				t.Fatal(err)
			}
			want := onlySnippet(t, real)
			if tc.long && want.Offset <= 6000 || !tc.long && want.Offset != 0 {
				t.Fatalf("fixture did not select the intended production stretch: %+v", want)
			}
			q := question{Expect: []string{"answer"}, Evidence: evidence}
			production, err := s.evidence(q, real)
			if err != nil || tc.long && !production.afterRead || !tc.long && !production.onPage {
				t.Fatalf("production evidence: on page %t, after read %t, error %v", production.onPage, production.afterRead, err)
			}
			for _, mode := range []comparison{byCount, byBM25, withStemming, withHeadingWeight} {
				a, err := s.compare(input, mode)
				if err != nil {
					t.Fatal(err)
				}
				if got := onlySnippet(t, a); got != want {
					t.Errorf("mode %d selected %+v; production selected %+v", mode, got, want)
				}
				got, err := s.evidence(q, a)
				if err != nil || got.rank != 1 || got.onPage != production.onPage || got.afterRead != production.afterRead {
					t.Errorf("mode %d evidence: rank %d, on page %t, after read %t, error %v",
						mode, got.rank, got.onPage, got.afterRead, err)
				}
			}
		})
	}
}

type snippetFields struct {
	ID     string
	Text   string `json:"snippet"`
	Offset int    `json:"snippet_offset"`
}

func onlySnippet(t *testing.T, a attempt) snippetFields {
	t.Helper()
	var page struct{ Results []snippetFields }
	if err := json.Unmarshal(a.calls[len(a.calls)-1].out, &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Results) != 1 || page.Results[0].ID != "answer" || page.Results[0].Text == "" {
		t.Fatalf("expected one answer with a snippet: %+v", page.Results)
	}
	return page.Results[0]
}
