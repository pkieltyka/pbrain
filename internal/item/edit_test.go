package item

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

func TestEditVerify(t *testing.T) {
	for _, tc := range []struct {
		name, header, want, actor string
	}{
		{"plain and comment", "status: provisional  # status\nverified: old#actor # verified\n",
			"status: verified  # status\nverified: agent:test # verified\n", "agent:test"},
		{"quoted and doubled", "status: \"provisional # literal\" # status\nverified: 'it''s # inside'  # verified\n",
			"status: verified # status\nverified: \"on\"  # verified\n", "on"},
		{"quoted with escape", "status: provisional\nverified: \"a\\\"b#inside\"   # verified\n",
			"status: verified\nverified: agent:test   # verified\n", "agent:test"},
		{"empty", "status: provisional\nverified:\n", "status: verified\nverified: agent:test\n", "agent:test"},
		{"empty with comment", "status: provisional\nverified: # keep me\n", "status: verified\nverified: agent:test # keep me\n", "agent:test"},
		{"insert verified", "status: provisional\n  # keep another line\nunknown: keep\n",
			"status: verified\nverified: agent:test\n  # keep another line\nunknown: keep\n", "agent:test"},
		{"insert status", "verified: old\nunknown: keep\n", "verified: agent:test\nunknown: keep\nstatus: verified\n", "agent:test"},
		{"insert both", "unknown: keep\n", "unknown: keep\nstatus: verified\nverified: agent:test\n", "agent:test"},
		{"quoted key", "'status' : provisional\n\"verified\": old\n", "'status' : verified\n\"verified\": agent:test\n", "agent:test"},
	} {
		for _, eol := range []string{"\n", "\r\n"} {
			t.Run(tc.name+map[string]string{"\n": " LF", "\r\n": " CRLF"}[eol], func(t *testing.T) {
				body := "\n# title from body é\nstatus: this is body\n"
				file := func(fields string) []byte {
					return []byte(strings.ReplaceAll("\ufeff---\n"+validFields+fields+"---\n"+body, "\n", eol))
				}
				before, want := file(tc.header), file(tc.want)
				stored := bytes.Clone(before)
				out, err := EditVerify(before, tc.actor)
				if err != nil || !bytes.Equal(out, want) || !bytes.Equal(stored, before) {
					t.Fatalf("edit result %q, want %q, error %v", out, want, err)
				}
			})
		}
	}
}

func TestEditVerifyRefuses(t *testing.T) {
	for _, fields := range []string{
		"status: |\n  provisional\n",
		"status: >\n  provisional\n",
		"status: provisional\n  continued\n",
		"status:\n  provisional\n",
		"verified: [agent:test]\n",
		"{status: provisional}\n",
		" status: provisional\n",
		"status: &p provisional\nother: *p\n",
		"? status\n: provisional\n",
	} {
		data := itemFile(fields, "stored body")
		saved := bytes.Clone(data)
		out, err := EditVerify(data, "agent:test")
		requireCode(t, err, "unsupported_edit")
		if out != nil || !bytes.Equal(data, saved) {
			t.Errorf("refused edit changed bytes for %q", fields)
		}
	}
	_, err := EditVerify(itemFile("status: provisional\n", ""), "invalid\nactor")
	requireCode(t, err, "invalid")
	_, err = EditVerify(itemFile("status: provisional\n", ""), "")
	requireCode(t, err, "invalid")
}

func FuzzEditVerify(f *testing.F) {
	f.Add("---\nstatus: provisional # keep\nverified:\n---\n\n# body\n", "agent:me")
	f.Add("\ufeff---\r\nverified: 'old''s'\r\n---\r\n\r\n漢字", "on")
	f.Add("---\n  # keep\n---", "agent:me")
	f.Fuzz(func(t *testing.T, text, actor string) {
		if !utf8.ValidString(text) || len(text) > 4<<20 {
			t.Skip()
		}
		data := []byte(text)
		edited, err := EditVerify(data, actor)
		if err != nil {
			return
		}
		keep := func(file []byte) (other, body []byte) {
			t.Helper()
			block, line, at, ok, err := SplitFrontMatter(file)
			if err != nil || !ok {
				t.Fatalf("edit accepted unreadable front matter: %v", err)
			}
			var doc yaml.Node
			if err := yaml.Unmarshal(block, &doc); err != nil {
				t.Fatal(err)
			}
			skip := make(map[int]bool)
			if len(doc.Content) != 0 {
				keys := doc.Content[0].Content
				for i := 0; i < len(keys); i += 2 {
					if keys[i].Value == "status" || keys[i].Value == "verified" {
						skip[keys[i].Line+line-1] = true
					}
				}
			}
			for i, raw := range bytes.SplitAfter(file[:at], []byte{'\n'}) {
				if !skip[i+1] {
					other = append(other, raw...)
				}
			}
			return other, file[at:]
		}
		oldHeader, oldBody := keep(data)
		newHeader, newBody := keep(edited)
		h, _, _, err := Parse("id", edited)
		if !bytes.Equal(oldHeader, newHeader) || !bytes.Equal(oldBody, newBody) || err != nil || h.Status != Verified || h.Verified != actor {
			t.Fatalf("changed unrelated bytes: %q -> %q, %v", data, edited, err)
		}
	})
}
