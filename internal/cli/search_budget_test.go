package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// fiveHitBrain holds five notes whose whole search page is above 2,000 and below 3,000 code points
// in text and in JSON.
func fiveHitBrain(t *testing.T) *readBrain {
	rb := newReadBrain(t)
	for i, id := range []string{"a1a1", "b2b2", "c3c3", "d4d4", "e5e5"} {
		day := string(rune('1' + i))
		title := "Budget " + strings.Repeat("t", 60) + " " + id
		body := "budgetword " + strings.Repeat("filler words ", 30) + "\n"
		rb.put("notes/2026110"+day+"-000000-budget-"+id+".md",
			[]string{"kind: note", "title: " + title, "created: 2026-11-0" + day + "T00:00:00Z", "author: agent:x"}, body)
	}
	return rb
}

// Scenario 39 and render.md TestSearchLocationAndDefaultBudget: a real five-hit page fits the
// default search budget of 3,000 in both forms, and an explicit 2,000 pages it.
func TestSearchFiveHitDefaultBudget(t *testing.T) {
	rb := fiveHitBrain(t)
	for _, js := range []bool{false, true} {
		args := []string{"search", "budgetword"}
		if js {
			args = append(args, "--json")
		}
		code, out, errOut := rb.run(args...)
		if code != 0 || errOut != "" {
			t.Fatalf("json %v: exit %d, stderr %q", js, code, errOut)
		}
		if n := utf8.RuneCountInString(out); n <= 2000 || n >= 3000 {
			t.Fatalf("json %v: the fixture's page is %d code points, not between 2,000 and 3,000", js, n)
		}
		if js {
			var page struct {
				Results []json.RawMessage `json:"results"`
				More    *int              `json:"more"`
			}
			if err := json.Unmarshal([]byte(out), &page); err != nil || len(page.Results) != 5 || page.More != nil {
				t.Fatalf("default JSON page: %d results, more %v, %v", len(page.Results), page.More, err)
			}
		} else if !strings.HasSuffix(out, "\n5 results.\n") || strings.Contains(out, "more:") {
			t.Fatalf("default text page: %q", out)
		}

		code, out, _ = rb.run(append(args, "--max-chars", "2000")...)
		if code != 0 || utf8.RuneCountInString(out) > 2000 {
			t.Fatalf("json %v at 2,000: exit %d, %d code points", js, code, utf8.RuneCountInString(out))
		}
		if js {
			var page struct {
				Results []json.RawMessage `json:"results"`
				More    int               `json:"more"`
				Next    int               `json:"next"`
			}
			if err := json.Unmarshal([]byte(out), &page); err != nil || len(page.Results) == 0 || len(page.Results) >= 5 ||
				page.More != 5-len(page.Results) || page.Next != len(page.Results) {
				t.Fatalf("JSON at 2,000: %s", out)
			}
		} else if !strings.Contains(out, " more: --offset ") || strings.Contains(out, "5 results.") {
			t.Fatalf("text at 2,000: %q", out)
		}
	}
}
