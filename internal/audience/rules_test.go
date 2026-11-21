package audience

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/errs"
)

func wantCode(t *testing.T, err error, code string) *errs.Error {
	t.Helper()
	var e *errs.Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("want %q; got %#v", code, err)
	}
	if e.Error() != e.Message || e.Message == "" {
		t.Fatalf("error has no usable diagnostic: %#v", e)
	}
	exit := 1
	if code == "usage" {
		exit = 2
	}
	if e.Exit != exit {
		t.Fatalf("exit %d; want %d", e.Exit, exit)
	}
	return e
}

func TestMixedHostsAndLongestPrefix(t *testing.T) {
	for _, key := range []string{"docs.google.com/document/d/A", "docs.google.com/spreadsheets/d/B/sheet", "docs.google.com/presentation/d/C", "drive.google.com/file/d/D", "drive.google.com/drive/folders/E", "claude.ai/artifact/F", "claude.ai/code/artifact/G", "work.slack.com/archives/C01", "github.com/owner", "gitlab.com/group", "notgithub.com", "work.slack.com.example"} {
		if _, err := Set(nil, key, Shared); err != nil {
			t.Errorf("unexpected refusal for %s: %#v", key, err)
		}
	}
	for _, key := range []string{"docs.google.com", "docs.google.com/document", "Docs.Google.com.", "https://docs.google.com:8443/document/d", "drive.google.com/drive", "claude.ai/code/artifact", "acme.slack.com", "acme.slack.com/archives", "acme.slack.com/archives/D012", "acme.slack.com/archives/G012", "acme.slack.com/archives/c012", "github.com", "gitlab.com"} {
		_, err := Set(nil, key, Shared)
		wantCode(t, err, "invalid")
		_, err = ValidateRules([]Rule{{key, Shared}})
		wantCode(t, err, "invalid_config")
		if _, err = Set(nil, key, Personal); err != nil {
			t.Errorf("refused personal rule %s", key)
		}
	}
	rules := []Rule{{"Acme.Slack.Com.", Personal}, {"acme.slack.com/archives/C012", Shared}, {"acme.slack.com/archives/C012/private", Personal}}
	for _, tc := range []struct {
		src   string
		dest  Destination
		match string
	}{
		{"https://ACME.slack.com/archives/C012/private/thread", Personal, "acme.slack.com/archives/C012/private"},
		{"https://acme.slack.com/archives/C012/msg", Shared, "acme.slack.com/archives/C012"},
		{"https://acme.slack.com/archives/C012suffix", Personal, "acme.slack.com"},
		{"https://acme.slack.com/archives/D512", Personal, "acme.slack.com"},
	} {
		d, err := Decide(rules, Request{Method: LinkedURL, Source: tc.src})
		if err != nil || d.Dest != tc.dest || d.Matched != tc.match {
			t.Errorf("%s: %#v %#v", tc.src, d, err)
		}
	}
	_, err := Decide(rules, Request{Method: LinkedURL, Source: "https://acme.slack.com:8443/archives/C012"})
	wantCode(t, err, "audience_required")
	d, err := Decide(rules, Request{Explicit: Personal, Method: LinkedURL, Source: "https://acme.slack.com/archives/C012"})
	if err != nil || d != (Decision{Dest: Personal}) {
		t.Fatalf("explicit selection: %#v %#v", d, err)
	}
}

func TestDecisionBeforeAcquisition(t *testing.T) {
	for _, tc := range []struct {
		req  Request
		want Destination
		code string
	}{
		{Request{Source: " 	HTTPS://docs.google.com/document/d/abc\t"}, "", "audience_required"},
		{Request{Source: "https:foo"}, "", "audience_required"},
		{Request{Source: "task 42, https://docs.google.com/document/d/abc"}, Shared, ""},
		{Request{}, Shared, ""},
		{Request{Source: "https://public.example/paper"}, "", "audience_required"},
		{Request{Source: "https://docs.google.com/document/d/abc", Method: FetchedURL}, Shared, ""},
		{Request{Source: "https://public.example/paper", Method: LinkedURL}, Shared, ""},
		{Request{Source: "https://docs.google.com:8443", Method: LinkedURL}, "", "audience_required"},
		{Request{Source: "https://public.example/%2e/", Method: LinkedURL}, "", "audience_required"},
		{Request{Source: "https:broken", Explicit: Personal}, Personal, ""},
		{Request{Source: "https://github.com/owner", Explicit: Shared, Method: LinkedURL}, Shared, ""},
	} {
		d, err := Decide(nil, tc.req)
		if tc.code != "" {
			wantCode(t, err, tc.code)
			continue
		}
		if err != nil || d != (Decision{Dest: tc.want}) {
			t.Errorf("%#v: %#v %#v", tc.req, d, err)
		}
	}

	for _, src := range []string{"https://user:private-locator@example.org/path", "https://github.com/private-locator?t=private-locator"} {
		_, err := Decide(nil, Request{Source: src})
		if strings.Contains(wantCode(t, err, "audience_required").Message, "private-locator") {
			t.Fatal("error included the private locator")
		}
	}
	rules := []Rule{{"example.org", Shared}, {"https://user:hidden@example.org", Personal}}
	_, err := Decide(rules, Request{Explicit: Shared})
	e := wantCode(t, err, "invalid_config")
	if strings.Contains(e.Message, "hidden") || !strings.Contains(e.Message, "2") || !strings.Contains(e.Message, "personal.toml") {
		t.Fatalf("unsafe or unactionable diagnostic: %s", e.Message)
	}
	for _, method := range []Acquisition{Citation, FetchedURL, LinkedURL} {
		d, err := Decide([]Rule{{"public.example/team", Personal}}, Request{Source: "https://public.example/team/page", Method: method})
		if err != nil || d != (Decision{Dest: Personal, Matched: "public.example/team"}) {
			t.Errorf("%v: %#v %#v", method, d, err)
		}
	}
}

func TestSetCanonicalUnsetAbsent(t *testing.T) {
	original := []Rule{{"Z.example", Personal}, {"A.example", Personal}}
	before := slices.Clone(original)
	got, err := Set(original, "HTTPS://a.example.:443/", Shared)
	if err != nil || !slices.Equal(got, []Rule{{"a.example", Shared}, {"z.example", Personal}}) {
		t.Fatalf("set: %#v %#v", got, err)
	}
	same, err := Unset(got, "missing.example")
	if err != nil || !slices.Equal(same, got) {
		t.Fatalf("absent unset: %#v %#v", same, err)
	}
	same[0].Match = "separate"
	if got[0].Match != "a.example" {
		t.Fatal("unset aliased caller")
	}
	got, err = Unset(got, "a.example:443")
	if err != nil || !slices.Equal(got, []Rule{{"z.example", Personal}}) {
		t.Fatalf("unset existing: %#v %#v", got, err)
	}
	if !slices.Equal(original, before) {
		t.Fatal("caller table changed")
	}
	bad := []Rule{{"docs.google.com", Personal}, {"Docs.Google.com.:443", Personal}}
	_, err = Set(bad, "%", Destination("wrong"))
	wantCode(t, err, "invalid_config")
	_, err = Unset(bad, "%")
	wantCode(t, err, "invalid_config")
	_, err = Set(nil, "example.org", Destination("wrong"))
	wantCode(t, err, "usage")
	_, err = Unset(nil, "https://hidden:secret@host")
	wantCode(t, err, "invalid")
}
