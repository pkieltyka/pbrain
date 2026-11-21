package item

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func itemFile(header, body string) []byte {
	return []byte("---\n" + header + "---\n" + body)
}

const validFields = "created: 2026-11-19T18:21:09Z\nauthor: agent:lead-dev\n"

func TestSplitFrontMatter(t *testing.T) {
	for _, tc := range []struct {
		name, file, yaml, body string
	}{
		{"opening and closing at EOF", "---\n---", "", ""},
		{"indented delimiter is body", "---\n---\n ---\n", "", " ---\n"},
		{"skip one separator", "---\nkind: note\n---\n\n\nké", "kind: note\n", "\nké"},
		{"preserve nonempty separator", "---\n---\n \nké", "", " \nké"},
		{"BOM and CRLF", "\ufeff--- \t\r\nkind: note\r\n---\t \r\n\r\n\r\nké", "kind: note\r\n", "\r\nké"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(tc.file)
			yaml, line, at, ok, err := SplitFrontMatter(data)
			if err != nil || !ok || string(yaml) != tc.yaml || line != 2 || string(data[at:]) != tc.body {
				t.Fatalf("split gave %q, %d, %d, %v, %v", yaml, line, at, ok, err)
			}
		})
	}
}

func TestParseNoFrontMatter(t *testing.T) {
	for _, text := range []string{"", "body", " ---\n---", "\n---\n---", "---\r"} {
		_, _, _, ok, splitErr := SplitFrontMatter([]byte(text))
		_, _, _, err := Parse("id", []byte(text))
		if ok || splitErr != nil || !errors.Is(err, ErrNoFrontMatter) {
			t.Errorf("%q: split %v %v, parse %v", text, ok, splitErr, err)
		}
	}
}

func TestParseBadFrontMatter(t *testing.T) {
	for _, tc := range []struct {
		text   string
		reason FMReason
		line   int
	}{
		{"---", Unclosed, 1},
		{"---\nkey: bad\n", Unclosed, 1},
		{"---\n[k, x]\n---\n", NotMapping, 2},
		{"---\ntitle: a\n'title': bad\n---\n", DuplicateKey, 3},
		{"---\nkind: [bad\nauthor: secret\n---\n", Syntax, 0},
	} {
		_, _, _, err := Parse("id", []byte(tc.text))
		var e *FrontMatterError
		if !errors.As(err, &e) || e.Reason != tc.reason || e.Line <= 0 || tc.line > 0 && e.Line != tc.line {
			t.Errorf("%q: got %v, want reason %v line %v", tc.text, err, tc.reason, tc.line)
		}
		if err != nil && strings.Contains(err.Error(), "secret") {
			t.Errorf("error reveals input: %v", err)
		}
	}
}

func TestParseFieldShapes(t *testing.T) {
	data := itemFile(validFields+
		"Kind: [decision]\nkind: [decision]\nstatus: ~\nverified: !!bool true\nsource: null\n"+
		"origin: ''\nfetched:\ndecided: |\n  quoted\ntitle: plain\n  continued\n"+
		"unknown: &anchor {x: y}\nsha256: *anchor\ntags: [cache, Redis]\n"+
		"relates:\n  - first\n  - ''\nreplaces: []\n", "ignored")
	h, at, got, err := Parse("id", data)
	if err != nil {
		t.Fatal(err)
	}
	if h.Kind != Note || h.Title != "plain continued" || h.Verified != "true" || h.Source != "null" ||
		h.StatusRaw != "~" || len(h.Relates) != 0 || len(h.Replaces) != 0 || !reflect.DeepEqual(h.Tags, []string{"cache", "redis"}) ||
		string(data[at:]) != "ignored" {
		t.Fatalf("unexpected header %#v or body offset %d", h, at)
	}
	want := []Problem{
		{Code: "bad_field", Field: "kind", Line: 5, Column: 7},
		{Code: "unknown_value", Field: "status", Line: 6, Column: 9},
		{Code: "bad_field", Field: "decided", Line: 11, Column: 10},
		{Code: "bad_field", Field: "sha256", Line: 16, Column: 9},
		{Code: "bad_field", Field: "relates", Line: 20, Column: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("problems = %#v, want %#v", got, want)
	}
	for _, header := range []string{"tags: cache\n", "tags: [valid, wrong_tag]\n", "tags: [[nested]]\n", "tags: |\n  text\n"} {
		h, _, p, err := Parse("id", itemFile(validFields+header, ""))
		if err != nil || len(h.Tags) != 0 || len(p) != 1 || p[0].Code != "bad_field" {
			t.Errorf("%q: got tags %v, problems %v, err %v", header, h.Tags, p, err)
		}
	}
}

func TestParseFieldRules(t *testing.T) {
	file := itemFile("kind: source\ncreated: 2026-11-19\nauthor: ''\nstatus: Verified\n"+
		"stale_after: someday\norigin: wrong\nfetched: bad\nsha256: abc\n", "")
	h, _, problems, err := Parse("20260203-040506-example-a1b2", file)
	if err != nil {
		t.Fatal(err)
	}
	want := []Problem{
		{Code: "missing_field", Field: "author"},
		{Code: "missing_field", Field: "source"},
		{Code: "bad_field", Field: "created", Line: 3, Column: 10},
		{Code: "unknown_value", Field: "status", Line: 5, Column: 9},
		{Code: "bad_field", Field: "stale_after", Line: 6, Column: 14},
		{Code: "bad_field", Field: "origin", Line: 7, Column: 9},
		{Code: "bad_field", Field: "fetched", Line: 8, Column: 10},
	}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("problems = %#v, want %#v", problems, want)
	}
	if !h.CreatedKnown || h.Created.Format(time.RFC3339) != "2026-02-03T04:05:06Z" ||
		h.Status != Provisional || !h.StaleSet || !h.StaleBad || h.SHA256 != "abc" {
		t.Fatalf("unexpected header: %#v", h)
	}
}

func TestParseKindAlwaysSet(t *testing.T) {
	for _, tc := range []struct {
		field string
		kind  Kind
	}{
		{"", Note}, {"kind: [decision]\n", Note}, {"kind: Decision\n", Note},
		{"kind: decision\n", Decision}, {"kind: learning\n", Learning},
		{"kind: observation\n", Observation}, {"kind: source\n", Source},
	} {
		h, _, _, err := Parse("id", itemFile(validFields+tc.field, ""))
		if err != nil || h.Kind != tc.kind {
			t.Errorf("%q: kind %s, error %v, want %s", tc.field, h.Kind, err, tc.kind)
		}
	}
}

func TestTitleFallbacks(t *testing.T) {
	for _, tc := range []struct{ body, heading, title string }{
		{"pre\n# ATX\n", "ATX", "ATX"},
		{"pre\nSetext\n===\n", "Setext", "Setext"},
		{"```\n# ignored\n```\n# Visible\n", "Visible", "Visible"},
		{"```\n# ignored\n```\nLook\n-----\n", "Look", "Look"},
		{"\r\n  First line  \nLast line\n", "", "First line"},
		{"~~~\n~~~\n", "", ""},
		{"", "", ""},
	} {
		body := []byte(tc.body)
		if FirstHeading(body) != tc.heading || HeadingTitle(body) != tc.title {
			t.Errorf("%q: heading %q title %q, want %q %q", body, FirstHeading(body), HeadingTitle(body), tc.heading, tc.title)
		}
		h, _, _, err := Parse("id", itemFile(validFields, tc.body))
		want := tc.title
		if want == "" {
			want = "id"
		}
		if err != nil || h.Title != want {
			t.Errorf("%q: parsed title %q, error %v, want %q", body, h.Title, err, want)
		}
	}
}

func TestCreatedFallback(t *testing.T) {
	for _, tc := range []struct{ id, header, want string }{
		{"id", "created: 2026-11-19T17:00:00+01:00\n", "2026-11-19T16:00:00Z"},
		{"id", "created: 2026-11-19T18:21:09.123Z\n", "2026-11-19T18:21:09.123Z"},
		{"20261119-182109-old", "created: 2026-11-19\n", "2026-11-19T18:21:09Z"},
		{"20261119-182109-old", "", "2026-11-19T18:21:09Z"},
		{"unknown", "created: 2026-11-19T18:21:09\n", ""},
	} {
		h, _, _, err := Parse(tc.id, itemFile(tc.header+"author: agent:lead-dev\n", ""))
		if err != nil || h.CreatedKnown != (tc.want != "") || h.CreatedKnown && h.Created.UTC().Format(time.RFC3339Nano) != tc.want {
			t.Errorf("%s %q: created %v, known %v, error %v, want %q", tc.id, tc.header, h.Created, h.CreatedKnown, err, tc.want)
		}
	}
}

func TestFactoryNotesRead(t *testing.T) {
	data := itemFile(validFields+"status: superseded-by ../decisions/new.MD\n", "One note\n")
	original := bytes.Clone(data)
	h, _, problems, err := Parse("id", data)
	stem, stamped := h.LegacyStamp()
	if err != nil || h.Kind != Note || !stamped || stem != "new" || len(problems) != 0 || !bytes.Equal(data, original) {
		t.Fatalf("old note changed or unreadable: %#v, %v %v", h, problems, err)
	}
	h, _, problems, err = Parse("id", itemFile(validFields+"kind: decision\nstatus: verified\n", ""))
	if err != nil || h.Standing() != 1 || !reflect.DeepEqual(problems, []Problem{{Code: "decision_without_decider", Field: "decided"}}) {
		t.Fatalf("old decision: %#v %v %v", h, problems, err)
	}
	h, _, _, err = Parse("id", itemFile(validFields+"kind: learning\nstatus: verified\n", ""))
	if err != nil || h.Standing() != 2 {
		t.Fatalf("verified learning: %#v %v", h, err)
	}
}

func FuzzParse(f *testing.F) {
	f.Add("---\ntitle: 🧪\n---\nbody", "id")
	f.Add("\ufeff---\r\nkind: source\r\n---\r\n", "id")
	f.Add("---\n{title: a, title: b}\n---\n", "id")
	f.Fuzz(func(t *testing.T, text, id string) {
		if !utf8.ValidString(text) || len(text) > 4<<20 {
			t.Skip()
		}
		data := []byte(text)
		h, at, _, err := Parse(id, data)
		if err != nil {
			return
		}
		if at < 0 || at > int64(len(data)) || !utf8.Valid(data[at:]) {
			t.Fatalf("invalid body offset %d", at)
		}
		// An arbitrary unknown key cannot fill any field of a valid header.
		withUnknown := itemFile(validFields+"kind: note\nunknown: [verified, learning, source, tags]\n", string(data[at:]))
		clean := itemFile(validFields+"kind: note\n", string(data[at:]))
		got, _, _, err := Parse(id, withUnknown)
		want, _, _, cleanErr := Parse(id, clean)
		if err != nil || cleanErr != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("an unknown key changed the header: %#v != %#v", got, want)
		}
		if h.Kind > Source {
			t.Fatalf("unexpected kind %d", h.Kind)
		}
	})
}
