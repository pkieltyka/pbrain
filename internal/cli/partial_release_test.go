package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestReleasedPartialSearch(t *testing.T) {
	rb := newReadBrain(t)
	rb.env.Rand = bytes.NewReader(make([]byte, 64))
	put := func(id, title string) {
		t.Helper()
		rb.put("notes/"+id+".md", []string{"kind: note", "title: " + title, "created: 2026-11-20T00:00:00Z"}, "")
	}
	put("alpha", "queue retry backoff worker lease")
	put("beta", "queue timeout backoff worker lease")
	put("four", "queue backoff worker lease")
	for i := range 17 {
		put(fmt.Sprintf("filler-%02d", i), "unrelated")
	}
	const q = "queue retry timeout backoff worker lease"
	status, out, diag := invoke(t, rb.env, Features{}, "search", "--bm25", q)
	if status != 2 || out != "" || !strings.HasPrefix(diag, "usage: ") {
		t.Fatalf("held-back build accepted --bm25: %d %q %q", status, out, diag)
	}
	var stdout, stderr bytes.Buffer
	run := func(args ...string) string {
		t.Helper()
		env := rb.env
		env.Args, env.Stdout, env.Stderr = args, &stdout, &stderr
		stdout.Reset()
		stderr.Reset()
		if status := Run(t.Context(), env); status != 0 || stderr.Len() != 0 {
			t.Fatalf("%v: status %d, stderr %q", args, status, &stderr)
		}
		return stdout.String()
	}
	if got := run("search", q); !strings.HasPrefix(got, "0 results.") {
		t.Fatalf("strict query returned a partial: %q", got)
	}
	if got := run("search", "--bm25", q); strings.Count(got, "partial 5/6") != 2 || !strings.HasPrefix(got, "alpha ") || strings.Contains(got, "four ") {
		t.Fatalf("wrong partial text: %q", got)
	}
	type result struct {
		ID               string
		Partial          bool
		Matched, Clauses int
	}
	var page struct{ Results []result }
	oneJSON(t, run("search", "--bm25", "--json", q), &page)
	want := []result{
		{ID: "alpha", Partial: true, Matched: 5, Clauses: 6},
		{ID: "beta", Partial: true, Matched: 5, Clauses: 6},
	}
	if len(page.Results) != 2 || page.Results[0] != want[0] || page.Results[1] != want[1] {
		t.Fatalf("wrong partial JSON: %+v", page.Results)
	}
	run("add", "--author", "agent:test", "--title", "queue retry backoff worker lease fresh")
	if got := run("search", "--bm25", q); !strings.Contains(got, "lease fresh") {
		t.Fatalf("next search did not find the new partial item: %q", got)
	}
}
