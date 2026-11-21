package cli

import (
	"crypto/rand"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/semantic"
)

// A --limit of the largest int pages without overflow; the exit follows the real error count.
func TestCheckHugeLimit(t *testing.T) {
	rb := newReadBrain(t)
	os.MkdirAll(filepath.Join(rb.dir, "notes"), 0o755)
	os.WriteFile(filepath.Join(rb.dir, "notes", "a.md"), []byte("no front matter\n"), 0o644)
	os.WriteFile(filepath.Join(rb.dir, "notes", "b.md"), []byte("no front matter\n"), 0o644)
	max := strconv.Itoa(math.MaxInt)
	status, out, errOut := rb.run("check", "--offset", "1", "--limit", max)
	if status != 0 || errOut != "" || out != "info not_an_item notes/b.md  a .md file without front matter is not an item\n1 finding, 0 errors.\n" {
		t.Fatalf("check --limit MaxInt: %d %q %q", status, out, errOut)
	}
}

// The context block is sized as it prints: with HTML escaping off, "<" is one code point.
func TestContextSizedWithoutHTMLEscapes(t *testing.T) {
	rb := newReadBrain(t)
	rb.put("notes/20261101-000000-angles-a3a3.md", []string{"kind: note", "title: Angles"}, "token "+strings.Repeat("<", 119)+"\n")
	var res struct {
		Results []struct {
			Context []map[string]any `json:"context"`
		} `json:"results"`
	}
	status, out, _ := rb.run("search", "token", "--context", "1", "--json")
	oneJSON(t, out, &res)
	if status != 0 || len(res.Results) != 1 || len(res.Results[0].Context) == 0 {
		t.Fatalf("a context of angle brackets was left out: %s", out)
	}
	if strings.Contains(out, "\\u003c") {
		t.Fatal("the output escapes HTML")
	}
}

// get prints the skipped_files notice of its final scan on every path: a miss, an ambiguous id,
// --history and --outline, once each.
func TestGetSkippedNoticeOnEveryPath(t *testing.T) {
	rb := newReadBrain(t)
	rb.put("notes/20261101-000000-dup-d1d1.md", []string{"kind: note", "title: One"}, "one\n")
	rb.put("other/20261101-000000-dup-d1d1.md", []string{"kind: note", "title: Two"}, "two\n")
	os.WriteFile(filepath.Join(rb.dir, "notes", "broken.md"), []byte("---\nkind: [\n---\n"), 0o644)
	const notice = "skipped_files: 1 file skipped: run pbrain check\n"
	for _, args := range [][]string{
		{"get", "missing-id"},
		{"get", "missing-id", "--history"},
		{"get", "missing-id", "--outline"},
		{"get", "20261101-000000-dup-d1d1"},
	} {
		status, out, errOut := rb.run(args...)
		if status != 1 || out != "" || !strings.HasPrefix(errOut, notice) || strings.Count(errOut, "skipped_files") != 1 {
			t.Errorf("%v: %d %q", args, status, errOut)
		}
	}
	// On success too, once.
	rb.put("notes/20261101-000000-solo-s1s1.md", []string{"kind: note", "title: Solo"}, "solo\n")
	for _, args := range [][]string{{"get", "20261101-000000-solo-s1s1"}, {"get", "20261101-000000-solo-s1s1", "--history"}, {"get", "20261101-000000-solo-s1s1", "--outline"}} {
		_, _, errOut := rb.run(args...)
		if errOut != notice {
			t.Errorf("%v: %q", args, errOut)
		}
	}
}

// A count of one is singular: the write path's skipped-files notice and index progress.
func TestSingularNotices(t *testing.T) {
	rb := newReadBrain(t)
	os.MkdirAll(filepath.Join(rb.dir, "notes"), 0o755)
	os.WriteFile(filepath.Join(rb.dir, "notes", "broken.md"), []byte("---\nkind: [\n---\n"), 0o644)
	rb.env.Rand = rand.Reader
	status, _, errOut := rb.run("add", "--title", "T", "--body", "b", "--author", "agent:x", "--json")
	var lines []map[string]map[string]string
	for _, l := range strings.Split(strings.TrimSpace(errOut), "\n") {
		var m map[string]map[string]string
		json.Unmarshal([]byte(l), &m)
		lines = append(lines, m)
	}
	found := false
	for _, m := range lines {
		if m["notice"]["code"] == "skipped_files" {
			found = m["notice"]["message"] == "1 file skipped: run pbrain check"
		}
	}
	if status != 0 || !found {
		t.Fatalf("write's skipped notice: %d %q", status, errOut)
	}
	for p, want := range map[semantic.Progress]string{
		{Section: item.Shared, Done: 0, Total: 1}:       "shared: 0 of 1 item",
		{Section: item.Personal, Done: 3, Total: 7}:     "personal: 3 of 7 items",
		{Section: item.Shared, Waiting: true, Total: 1}: "shared: waiting for another pbrain sync",
	} {
		if got := progressMessage(p); got != want {
			t.Errorf("%+v: %q, want %q", p, got, want)
		}
	}
}
