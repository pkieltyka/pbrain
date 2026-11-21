package importer

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/item"
)

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var e *errs.Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("got %v, want %s", err, code)
	}
}

func TestImportAsData(t *testing.T) {
	raw := []byte("\xef\xbb\xbf--- \t\r\nkind: [invalid YAML\r\nverified: intruder\r\n---\r\n\r\n##  Work   notes\r\n\tKeep\x00 this[^1].\r\n[^1]: café\r\n")
	sum := fmt.Sprintf("%x", sha256.Sum256(raw))
	p, err := FromFile("notes.MARKDOWN", "reference", "", raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.Origin != item.OriginFile || p.Source != "reference" || p.Title != "Work notes" || p.Body != "##  Work   notes\r\n\tKeep this[^1].\r\n[^1]: café\r\n" || p.SHA256 != sum || string(p.Original) != string(raw) {
		t.Fatalf("import: %+v", p)
	}
	raw[0] = 'x'
	if p.Original[0] == 'x' {
		t.Fatal("prepared original aliases caller input")
	}
	for _, body := range []string{"", "---\nno end\n", "short text", "\t\r\n", "\u0085a\u007f"} {
		p, err := FromFile("note.txt", "", "", []byte(body))
		if err != nil || p.Source != "note.txt" || p.Title != "note.txt" || p.Body != strings.Trim(body, "\u0085\u007f") {
			t.Fatalf("%q: %+v %v", body, p, err)
		}
	}
	p, err = FromFile("x.md", "", "", []byte("# "+strings.Repeat("界", 125)))
	if err != nil || p.Title != strings.Repeat("界", 119)+"…" {
		t.Fatalf("inferred title %q: %v", p.Title, err)
	}
	for _, tc := range []struct {
		name, title, source string
		raw                 []byte
		code                string
	}{
		{"unsupported.csv", "", "", nil, "unsupported_type"}, {"bad.txt", "", "", []byte{0xff}, "unsupported_type"},
		{"bytes.txt", "", "", make([]byte, (5<<20)+1), "too_large"},
		{"chars.md", "", "", []byte(strings.Repeat("é", 512001)), "too_large"},
		{"a.md", strings.Repeat("é", 121), "", nil, "invalid"}, {"a.md", "", "one\ntwo", nil, "invalid"},
	} {
		_, err := FromFile(tc.name, tc.source, tc.title, tc.raw)
		wantCode(t, err, tc.code)
	}
}

func TestHTMLArticleAndShell(t *testing.T) {
	load := func(name string) []byte {
		t.Helper()
		b, e := os.ReadFile("testdata/" + name)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	raw := load("article.html")
	for _, source := range []string{"https://example.org/ops/start", ""} {
		p, err := FromFile("article.HTM", source, "", raw)
		if err != nil {
			t.Fatal(err)
		}
		if p.Title != "Assembly notes" || string(p.Original) != string(raw) {
			t.Errorf("HTML title %q / original mismatch", p.Title)
		}
		link := "../manual"
		if source != "" {
			link = "https://example.org/manual"
		}
		for _, s := range []string{"## Assembly notes", "[reference manual](" + link + ")", "Measure the diagonals.", "```", "measure --diagonal", "| Part", "Quantity", "| Clamp"} {
			if !strings.Contains(p.Body, s) {
				t.Errorf("missing %q from %q", s, p.Body)
			}
		}
		for _, s := range []string{"Discard this navigation", "discard-", "unused.test"} {
			if strings.Contains(p.Body, s) {
				t.Errorf("unexpected %s", s)
			}
		}
	}
	for _, name := range []string{"short.html", "shell.html"} {
		_, err := FromFile(name, "", "", load(name))
		wantCode(t, err, "no_content")
	}
	for _, n := range []int{199, 200} {
		raw := []byte("<title>Page title</title><article><p>" + strings.Repeat("界", n) + "</p></article>")
		p, err := FromFile("floor.html", "", "", raw)
		if n == 199 {
			wantCode(t, err, "no_content")
		} else if err != nil || p.Title != "Page title" || utf8.RuneCountInString(strings.TrimSpace(p.Body)) != 200 {
			t.Fatalf("at floor: %+v %v", p, err)
		}
	}
}

func TestLinkedSource(t *testing.T) {
	url := "HTTPS://Example.org/a/../x?q=val#part"
	summary := strings.Repeat("界", 300)
	p, err := Link(url, "", &summary)
	if err != nil || p.Title != url || p.Source != url || p.Origin != item.OriginLink || p.Body != summary || p.Original != nil || p.SHA256 != "" {
		t.Fatalf("linked: %+v %v", p, err)
	}
	for _, s := range []string{"", summary + "a", "new\nline", "a\u2028b", "tab\there", "\x00"} {
		_, err := Link(url, "", &s)
		wantCode(t, err, "invalid")
	}
	for _, u := range []string{"https:example.com", "ftp://example.org/a", "https://u:p@example.org", "/local", "https://:443/a", "https://example.org/has space", "https://example.org/?bad\n"} {
		_, err := Link(u, "", nil)
		wantCode(t, err, "invalid")
	}
	raw := "https://example.org/"
	raw += strings.Repeat("é", 2000-len(raw))
	p, err = Link(raw, "", nil)
	if err != nil || p.Source != raw || p.Body != "" || utf8.RuneCountInString(p.Title) != 120 || !strings.HasSuffix(p.Title, "…") {
		t.Fatalf("at cap: %v", err)
	}
	_, err = Link(raw+"z", "title", nil)
	wantCode(t, err, "invalid")
}
