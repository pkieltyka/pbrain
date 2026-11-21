package item

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

func testDraft() Draft {
	return Draft{
		Kind: Decision, Title: "Cache layer uses Redis",
		Created: time.Date(2026, 11, 19, 18, 21, 9, 0, time.UTC),
		Author:  "agent:lead-dev", Source: "task 42, commit 3f2a9c1",
		Status: Verified, Decided: "human:pkieltyka", StaleAfter: "2027-05-01",
		Replaces: []string{"20261001-101500-cache-uses-memcached-x7b2"},
		Relates:  []string{"20261110-140233-session-store-benchmark-q1w8"},
		Tags:     []string{"cache", "sessions"},
	}
}

func TestValidateLimits(t *testing.T) {
	for _, tc := range []struct {
		field string
		max   int
		set   func(*Draft, string)
	}{
		{"title", 120, func(d *Draft, s string) { d.Title = s }},
		{"author", 200, func(d *Draft, s string) { d.Author = s }},
		{"source", 2000, func(d *Draft, s string) { d.Source = s }},
		{"verified", 200, func(d *Draft, s string) { d.Verified = s }},
		{"decided", 200, func(d *Draft, s string) { d.Decided = s }},
		{"replaces", 200, func(d *Draft, s string) { d.Replaces = []string{s} }},
		{"relates", 200, func(d *Draft, s string) { d.Relates = []string{s} }},
	} {
		t.Run(tc.field, func(t *testing.T) {
			d := testDraft()
			tc.set(&d, strings.Repeat("é", tc.max))
			if err := d.Validate(); err != nil {
				t.Fatalf("at limit: %v", err)
			}
			over := strings.Repeat("é", tc.max+1)
			tc.set(&d, over)
			e := requireCode(t, d.Validate(), "invalid")
			if !strings.HasPrefix(e.Error(), tc.field+": ") || strings.Contains(e.Error(), over) {
				t.Fatalf("wrong diagnostic: %v", e)
			}
		})
	}
	for _, tc := range []struct {
		field string
		set   func(*Draft, []string)
	}{
		{"replaces", func(d *Draft, s []string) { d.Replaces = s }},
		{"relates", func(d *Draft, s []string) { d.Relates = s }},
		{"tags", func(d *Draft, s []string) { d.Tags = s }},
	} {
		d := testDraft()
		list := strings.Split(strings.Repeat("redis,", 10)+"cache", ",")
		tc.set(&d, list[:10])
		if err := d.Validate(); err != nil {
			t.Errorf("%s: %v", tc.field, err)
		}
		tc.set(&d, list)
		if e := requireCode(t, d.Validate(), "invalid"); !strings.HasPrefix(e.Message, tc.field+": ") {
			t.Errorf("%s: %v", tc.field, e)
		}
	}
	for _, value := range []string{"bad\nsecret", "bad\rsecret", "bad\tsecret", "bad\x00secret", "bad\u0085secret", "bad\u2028secret", "bad\u2029secret", "invalid\xff"} {
		d := testDraft()
		d.Source = value
		e := requireCode(t, d.Validate(), "invalid")
		if strings.Contains(e.Message, value) {
			t.Errorf("validation quoted a value: %q", e.Message)
		}
	}
	for _, tc := range []struct {
		field string
		edit  func(*Draft)
	}{
		{"title", func(d *Draft) { d.Title = " " }},
		{"author", func(d *Draft) { d.Author = "" }},
		{"source", func(d *Draft) { d.Kind, d.Source = Source, "" }},
		{"decided", func(d *Draft) { d.Kind = Note }},
		{"relates", func(d *Draft) { d.Relates = []string{""} }},
		{"tags", func(d *Draft) { d.Tags = []string{"Redis"} }},
		{"tags", func(d *Draft) { d.Tags = []string{"wrong_tag"} }},
		{"stale_after", func(d *Draft) { d.StaleAfter = "tomorrow" }},
		{"stale_after", func(d *Draft) { d.StaleAfter = "2026-11-19T00:00:00+00:00" }},
		{"origin", func(d *Draft) { d.Origin = OriginFile }},
		{"fetched", func(d *Draft) { d.Fetched = d.Created }},
		{"sha256", func(d *Draft) { d.SHA256 = strings.Repeat("b", 64) }},
		{"created", func(d *Draft) { d.Created = time.Time{} }},
	} {
		d := testDraft()
		tc.edit(&d)
		if e := requireCode(t, d.Validate(), "invalid"); !strings.HasPrefix(e.Message, tc.field+": ") {
			t.Errorf("want %s diagnostic, got %v", tc.field, e)
		}
	}
	source := Draft{Kind: Source, Title: "A source", Author: "me", Source: "https://example.test/article", Created: testDraft().Created,
		Origin: OriginFile, Fetched: testDraft().Created, SHA256: strings.Repeat("b", 64)}
	if err := source.Validate(); err != nil {
		t.Fatalf("valid copied source: %v", err)
	}
	bad := source
	bad.SHA256 = strings.ToUpper(bad.SHA256)
	requireCode(t, bad.Validate(), "invalid")
	bad = source
	bad.Origin = OriginLink
	requireCode(t, bad.Validate(), "invalid")
	bad.Fetched = time.Time{}
	requireCode(t, bad.Validate(), "invalid")
	bad.SHA256 = ""
	if err := bad.Validate(); err != nil {
		t.Fatalf("valid linked source: %v", err)
	}
	bad.Status = Verified
	requireCode(t, bad.Validate(), "invalid")
}

func TestEmitForm(t *testing.T) {
	d := testDraft()
	body := []byte("\nA stored body.\r\n最後")
	out, err := Emit(&d, body)
	if err != nil {
		t.Fatal(err)
	}
	want := "---\nkind: decision\ntitle: Cache layer uses Redis\ncreated: 2026-11-19T18:21:09Z\n" +
		"author: agent:lead-dev\nsource: task 42, commit 3f2a9c1\nstatus: verified\n" +
		"decided: human:pkieltyka\nreplaces: [20261001-101500-cache-uses-memcached-x7b2]\n" +
		"relates: [20261110-140233-session-store-benchmark-q1w8]\ntags: [cache, sessions]\nstale_after: 2027-05-01\n---\n\n" + string(body)
	if string(out) != want {
		t.Fatalf("emitted %q, want %q", out, want)
	}

	d.Title, d.Author, d.Source, d.Verified = "redis: safe # text", "+1.7e3", "null", "on"
	d.Replaces = []string{"a,b", "a]b", "x[0]", "1:59", "yes", "x\\y", "key: value"}
	d.Relates = []string{"no", "2026-11-19", "0?"}
	out, err = Emit(&d, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{
		"title: \"redis: safe # text\"\n", "author: \"+1.7e3\"\n", "source: \"null\"\n",
		"verified: \"on\"\n", "replaces: [\"a,b\", \"a]b\", \"x[0]\", \"1:59\", \"yes\", x\\y, \"key: value\"]",
		"relates: [\"no\", \"2026-11-19\", \"0?\"]\n",
	} {
		if !bytes.Contains(out, []byte(part)) {
			t.Errorf("missing %q from %q", part, out)
		}
	}
	if !bytes.HasSuffix(out, []byte("---\n")) {
		t.Fatal("an empty body acquired a separator")
	}
	d.Title = "round-trip \ufffe\uffff\ufeff"
	out, err = Emit(&d, nil)
	if err != nil || !bytes.Contains(out, []byte(`round-trip \uFFFE\uFFFF\uFEFF`)) {
		t.Fatalf("non-printing YAML code points: %q, %v", out, err)
	}
	h, _, _, err := Parse("id", out)
	if err != nil || h.Title != d.Title || !reflect.DeepEqual(h.Relates, d.Relates) {
		t.Fatalf("round-trip: %q, %v", h.Title, err)
	}
}

func TestStaleAfterPrecision(t *testing.T) {
	value := "2026-11-19T01:00:00.123+01:00"
	when, norm, ok := ParseStaleAfter(value)
	if !ok || norm != "2026-11-19T00:00:00.123Z" {
		t.Fatalf("got %q, %v", norm, ok)
	}
	h := Header{StaleSet: true, StaleAfter: when}
	if h.ReviewDue(when.Add(-time.Millisecond)) || !h.ReviewDue(when) {
		t.Fatal("fractional review boundary is wrong")
	}
}

func FuzzEmitParseRoundTrip(f *testing.F) {
	f.Add("Élan", "human:Alice", "task: 42", "\r\n正文\n", "0x22")
	f.Add("round-trip \ufffe\uffff\ufeff", "yes", ".5e-2", "", "a,b")
	f.Add("redis", "  first author", "010", "\n\n---\n", "x [y]")
	f.Fuzz(func(t *testing.T, title, author, source, body, link string) {
		if !utf8.ValidString(body) || len(body) > 512000 {
			t.Skip()
		}
		d := testDraft()
		d.Title, d.Author, d.Source, d.Relates = title, author, source, []string{link}
		if err := d.Validate(); err != nil {
			t.Skip()
		}
		out, err := Emit(&d, []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		h, at, problems, err := Parse("id", out)
		if err != nil || len(problems) != 0 || h.Title != title || h.Author != author || h.Source != source ||
			h.Kind != d.Kind || h.Status != d.Status || h.Decided != d.Decided || !h.Created.Equal(d.Created) ||
			!reflect.DeepEqual(h.Replaces, d.Replaces) || !reflect.DeepEqual(h.Relates, d.Relates) ||
			!reflect.DeepEqual(h.Tags, d.Tags) || string(out[at:]) != body {
			t.Fatalf("round-trip changed input: %q gives %#v, %v, %v, at %d", out, h, problems, err, at)
		}
		block, _, _, _, _ := SplitFrontMatter(out)
		var decoded map[string]any
		if err := yaml.Unmarshal(block, &decoded); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"kind", "title", "author", "source", "status", "decided"} {
			if v, exists := decoded[key]; exists {
				if _, ok := v.(string); !ok {
					t.Errorf("%s resolved as %T (%#v)", key, v, v)
				}
			}
		}
		for _, key := range []string{"replaces", "relates", "tags"} {
			for _, v := range decoded[key].([]any) {
				if _, ok := v.(string); !ok {
					t.Errorf("%s entry resolved as %T (%#v)", key, v, v)
				}
			}
		}
	})
}
