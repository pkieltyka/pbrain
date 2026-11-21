package check

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/graph"
	"github.com/pkieltyka/pbrain/internal/item"
)

const token = "gh" + "p_" + "abcdefghijklmnopqrstuvwxyz0123456789"

func mk(id string, sec item.Section, kind item.Kind, mod func(h *item.Header)) *item.Item {
	it := &item.Item{ID: id, Key: item.Fold(id), Section: sec, Path: "/b/" + id + ".md"}
	it.Header = item.Header{Kind: kind, Title: id, Author: "agent:a", Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), CreatedKnown: true}
	if kind == item.Decision {
		it.Header.Decided = "human:x"
	}
	if mod != nil {
		mod(&it.Header)
	}
	return it
}

func collect(items []*item.Item, skipped []Skipped, opt Options) ([]Finding, int) {
	if opt.DisplayPath == nil {
		opt.DisplayPath = func(sec item.Section, p string) string {
			if sec == item.Personal {
				return "personal:" + strings.TrimPrefix(p, "/b/")
			}
			return strings.TrimPrefix(p, "/b/")
		}
	}
	return Collect(items, skipped, graph.Build(items), opt)
}

func find(fs []Finding, code string) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Code == code {
			out = append(out, f)
		}
	}
	return out
}

func TestCheckGlobalStatusAndRedaction(t *testing.T) {
	old := mk("old", item.Shared, item.Decision, nil)
	a := mk("a", item.Shared, item.Decision, func(h *item.Header) { h.Replaces = []string{"old"} })
	b := mk("b", item.Shared, item.Decision, func(h *item.Header) { h.Replaces = []string{"old"} })
	dangling := mk("dang", item.Shared, item.Note, func(h *item.Header) { h.Relates = []string{"missing-" + token} })
	toPersonal := mk("topers", item.Shared, item.Note, func(h *item.Header) { h.Relates = []string{"mine"} })
	mine := mk("mine", item.Personal, item.Note, func(h *item.Header) { h.Title = "has " + token })
	dup1 := mk("dup", item.Shared, item.Note, nil)
	dup2 := mk("DUP", item.Shared, item.Note, nil)
	dup2.Path = "/b/other/DUP.md"
	tagged := mk("tagged", item.Shared, item.Note, func(h *item.Header) { h.Tags = []string{"ok", "bad-" + strings.ToLower(token)} })
	secret := mk("secret", item.Shared, item.Note, func(h *item.Header) { h.Source = "see " + token })
	items := []*item.Item{old, a, b, dangling, toPersonal, mine, dup1, dup2, tagged, secret}
	skipped := []Skipped{{Section: item.Shared, Path: "/b/broken.md", Code: "bad_front_matter", Reason: "the front matter does not parse", Line: 3, Column: 2}}
	opt := Options{Personal: true, HasTags: true, Tags: map[string]string{"ok": ""}, Now: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}
	fs, errCount := collect(items, skipped, opt)

	n := 0
	for _, f := range fs {
		if f.Severity == "error" {
			n++
		}
		if strings.Contains(f.Message+f.Path, token) || strings.Contains(f.Message, strings.ToLower(token)) {
			t.Errorf("a finding prints a secret: %+v", f)
		}
	}
	if errCount != n || errCount == 0 {
		t.Fatalf("error count %d, errors listed %d", errCount, n)
	}
	if got := find(fs, "duplicate_id"); len(got) != 2 || got[0].Path == got[1].Path {
		t.Fatalf("duplicate_id: %+v", got)
	}
	if got := find(fs, "replace_fork"); len(got) != 1 || got[0].Path != "old.md" || !strings.Contains(got[0].Message, "a, b") {
		t.Fatalf("replace_fork: %+v", got)
	}
	if got := find(fs, "dangling_link"); len(got) != 1 || got[0].Field != "relates" || strings.Contains(got[0].Message, "missing") {
		t.Fatalf("dangling_link names its target: %+v", got)
	}
	if got := find(fs, "personal_reference"); len(got) != 1 || got[0].Path != "topers.md" {
		t.Fatalf("personal_reference: %+v", got)
	}
	if got := find(fs, "undeclared_tag"); len(got) != 1 || got[0].Message != "a tag is not declared in pbrain.toml" {
		t.Fatalf("undeclared_tag: %+v", got)
	}
	// The secret scan covers shared items only.
	got := find(fs, "secret_suspected")
	var msgs []string
	for _, f := range got {
		msgs = append(msgs, f.Path+" "+f.Message)
	}
	if strings.Join(msgs, "|") != "dang.md relates line 1: github_token|secret.md source line 1: github_token|tagged.md tags line 2: github_token" {
		t.Fatalf("secret_suspected (shared items only): %q", msgs)
	}
	if got := find(fs, "bad_front_matter"); len(got) != 1 || got[0].Line != 3 || got[0].Column != 2 || got[0].Path != "broken.md" {
		t.Fatalf("bad_front_matter: %+v", got)
	}
	// Order: severity, then path, code, field, line, column and message.
	for i := 1; i < len(fs); i++ {
		if rank(fs[i-1].Severity) > rank(fs[i].Severity) || rank(fs[i-1].Severity) == rank(fs[i].Severity) && fs[i-1].Path > fs[i].Path {
			t.Fatalf("out of order at %d: %+v then %+v", i, fs[i-1], fs[i])
		}
	}

	// Without --personal: no personal_reference.
	opt.Personal = false
	fs, _ = collect([]*item.Item{old, a, toPersonal, mine}, nil, opt)
	if len(find(fs, "personal_reference")) != 0 {
		t.Fatal("personal_reference without --personal")
	}
	// A path the scanner flags is withheld.
	leaky := mk("leak", item.Shared, item.Note, nil)
	leaky.Path = "/b/" + token + ".md"
	leaky.Problems = []item.Problem{{Code: "missing_field", Field: "author"}}
	fs, _ = collect([]*item.Item{leaky}, []Skipped{{Path: "/b/x-" + token + ".md", Code: "not_an_item"}}, Options{Now: opt.Now})
	if len(fs) != 2 {
		t.Fatalf("findings for the leaky paths: %+v", fs)
	}
	for _, f := range fs {
		if f.Path != withheld {
			t.Fatalf("a sensitive path printed: %+v", f)
		}
	}
}

func TestCheckBodyScan(t *testing.T) {
	it := mk("n", item.Shared, item.Note, nil)
	mine := mk("p", item.Personal, item.Note, nil)
	body := func(x *item.Item) ([]byte, error) {
		if x == it {
			return []byte("line one\n" + token + "\n"), nil
		}
		return []byte(token), nil
	}
	fs, _ := collect([]*item.Item{it, mine}, nil, Options{Body: body})
	if got := find(fs, "secret_suspected"); len(got) != 1 || got[0].Message != "body line 2: github_token" {
		t.Fatalf("body scan: %+v", got)
	}
	// A body that cannot be read as scanned is not scanned, and not reported.
	fs, _ = collect([]*item.Item{it}, nil, Options{Body: func(*item.Item) ([]byte, error) { return nil, errors.New("changed") }})
	if len(find(fs, "secret_suspected")) != 0 {
		t.Fatal("an unreadable body was scanned")
	}
}

func TestCheckProblemsAndDue(t *testing.T) {
	it := mk("p", item.Shared, item.Decision, func(h *item.Header) {
		h.Decided = ""
		h.StaleSet, h.StaleBad = true, true
	})
	it.Problems = []item.Problem{{Code: "bad_field", Field: "created", Line: 4, Column: 10}, {Code: "decision_without_decider", Field: "decided"}}
	fs, errCount := collect([]*item.Item{it}, []Skipped{{Path: "/b/x.md", Code: "not_an_item"}}, Options{})
	want := []string{
		"error bad_field p.md created 4:10 created has the wrong type or cannot be read",
		"warning decision_without_decider p.md decided 0:0 a decision without decided has provisional standing",
		"warning review_due p.md stale_after 0:0 the review date cannot be read",
		"info not_an_item x.md  0:0 a .md file without front matter is not an item",
	}
	var got []string
	for _, f := range fs {
		got = append(got, f.Severity+" "+f.Code+" "+f.Path+" "+f.Field+" "+itoa(f.Line)+":"+itoa(f.Column)+" "+f.Message)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") || errCount != 1 {
		t.Fatalf("got %d errors:\n%s", errCount, strings.Join(got, "\n"))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// The scan covers the raw kind and status as stored, as a shared write does.
func TestCheckScansRawKindAndStatus(t *testing.T) {
	status := mk("st", item.Shared, item.Note, func(h *item.Header) { h.StatusRaw = token })
	kind := mk("kd", item.Shared, item.Note, func(h *item.Header) { h.KindRaw = token })
	fs, _ := collect([]*item.Item{status, kind}, nil, Options{})
	var got []string
	for _, f := range find(fs, "secret_suspected") {
		got = append(got, f.Path+" "+f.Message)
	}
	if strings.Join(got, "|") != "kd.md kind line 1: github_token|st.md status line 1: github_token" {
		t.Fatalf("raw kind and status: %q", got)
	}
}

// Every stored field of a parsed item reaches the projection: with a secret in each header field,
// every field but the body is reported, by the same names a write uses.
func TestStoredProjectionIsComplete(t *testing.T) {
	h := item.Header{KindRaw: token, Title: token, Author: token, Source: token, StatusRaw: token, Verified: token,
		Decided: token, Replaces: []string{token}, Relates: []string{token}, Tags: []string{token}, SHA256: token}
	s := StoredOf(&h)
	v := reflect.ValueOf(s)
	for i := range v.NumField() {
		name := v.Type().Field(i).Name
		switch name {
		case "Body", "Created", "StaleAfter", "Fetched", "Origin": // the body is the caller's; the rest are parsed values
			continue
		}
		f := v.Field(i)
		if f.Kind() == reflect.String && f.String() == "" || f.Kind() == reflect.Slice && f.Len() == 0 {
			t.Errorf("StoredOf leaves %s empty", name)
		}
	}
	var names []string
	for _, f := range s.Fields() {
		names = append(names, f.Name)
	}
	want := "kind title created author source status verified decided replaces relates tags stale_after origin fetched sha256 body"
	if strings.Join(names, " ") != want {
		t.Fatalf("fields %q", names)
	}
}

// A bad_file_name skip is an error that names the nearest folder whose path can be written, or "?";
// every Finding.Path is valid UTF-8; ids in messages go through Options.Value.
func TestCheckBadFileName(t *testing.T) {
	skipped := []Skipped{
		{Section: item.Shared, Path: "/b/notes/x\xff.md", Code: "bad_file_name", Reason: "invalid_utf8"},
		{Section: item.Shared, Path: "/b/a\xfe/deeper/y\xff.md", Code: "bad_file_name", Reason: "invalid_utf8"},
		{Section: item.Shared, Path: "/b/\xfez.md", Code: "bad_file_name", Reason: "invalid_utf8"},
		{Section: item.Shared, Path: "/b/odd\xfe/plain.md", Code: "not_an_item"},
	}
	one := mk("a\nb", item.Shared, item.Note, nil)
	two := mk("A\nB", item.Shared, item.Note, nil)
	two.Path = "/b/other/A\nB.md"
	quote := func(s string) string { return "<" + s + ">" }
	fs, errorCount := collect([]*item.Item{one, two}, skipped, Options{Value: quote})
	bad := find(fs, "bad_file_name")
	var paths []string
	for _, f := range bad {
		if f.Severity != "error" || f.Message != "the file name is not valid UTF-8, so the file has no id; reads skip this file" {
			t.Fatalf("%+v", f)
		}
		paths = append(paths, f.Path)
	}
	if want := []string{"?", "?", "notes/"}; !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths %q", paths)
	}
	if p := find(fs, "not_an_item"); len(p) != 1 || p[0].Path != "?" {
		t.Fatalf("not_an_item under an odd folder: %+v", p)
	}
	for _, f := range fs {
		if !utf8.ValidString(f.Path) {
			t.Fatalf("invalid path %q", f.Path)
		}
	}
	dups := find(fs, "duplicate_id")
	if len(dups) != 2 || !strings.Contains(dups[0].Message, "<a\nb>") && !strings.Contains(dups[0].Message, "<A\nB>") {
		t.Fatalf("duplicate ids without Value: %+v", dups)
	}
	if errorCount != 5 {
		t.Fatalf("%d errors", errorCount)
	}
}
