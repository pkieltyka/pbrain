package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	lfID    = "20261121-000000-a\nb"
	oldID   = "20261101-000000-old-o1o1"
	plainID = "20261110-000000-plain-p1p1"
)

func exactBrain(t *testing.T) *readBrain {
	rb := newReadBrain(t)
	rb.put("decisions/"+oldID+".md", []string{"kind: decision", "title: Old way", "created: 2026-11-01T00:00:00Z", "decided: human:x", "status: verified", "author: agent:x"}, "zebraword old\n")
	rb.put("notes/"+plainID+".md", []string{"kind: note", "title: Plain", "created: 2026-11-10T00:00:00Z", "author: agent:x"}, "zebraword plain\n")
	rb.put("decisions/"+lfID+".md", []string{"kind: decision", "title: New way", "created: 2026-11-21T00:00:00Z", "decided: human:x", "status: verified",
		"author: agent:x", "replaces: [" + oldID + "]", "relates: [" + plainID + "]"}, "zebraword new\n")
	return rb
}

// TestExactValueProjection (render.md, spec 6.2 and 39): an id whose file name holds a line feed is one
// token on one line wherever text prints it, JSON carries the stored id, and get with it opens the item.
func TestExactValueProjection(t *testing.T) {
	rb := exactBrain(t)
	const tok = `"20261121-000000-a\u000ab"`
	cases := []struct {
		args []string
		want []string // lines that must be printed exactly
	}{
		{[]string{"search", "zebraword"}, []string{tok + "  decision  verified  2026-11-21",
			"  newer related: " + tok + ", decision, decided, 2026-11-21, New way"}},
		{[]string{"list"}, []string{tok + "  decision  verified  2026-11-21  New way"}},
		{[]string{"prime"}, []string{"decision  2026-11-21  " + tok + "  New way (human:x)"}},
		{[]string{"get", lfID}, []string{tok}},
		{[]string{"get", lfID, "--history"}, []string{tok + "  decision  verified  2026-11-21  New way (decided by human:x; replaces: " + oldID + ")"}},
		{[]string{"get", oldID}, []string{"replacement: " + tok, "current: " + tok}},
		{[]string{"get", plainID}, []string{"  newer related: " + tok + ", decision, decided, 2026-11-21, New way"}},
		{[]string{"get", lfID, "--outline"}, []string{tok}},
		{[]string{"delete", lfID}, []string{"would delete: " + tok + "  shared  \"decisions/20261121-000000-a\\u000ab.md\""}},
	}
	for _, c := range cases {
		_, out, _ := rb.run(c.args...)
		if strings.Contains(out, lfID) {
			t.Fatalf("%q prints the raw id:\n%s", c.args, out)
		}
		lines := strings.Split(out, "\n")
		for _, w := range c.want {
			if !slices.Contains(lines, w) {
				t.Fatalf("%q: no line %q in\n%s", c.args, w, out)
			}
		}
	}
	// A miss suggests the id as a token.
	if _, _, errOut := rb.run("get", "20261121-000000-a"); !strings.Contains(errOut, "ids that start alike: "+tok+"\n") {
		t.Fatalf("suggestion: %q", errOut)
	}
	// The bold pair covers the whole token.
	if _, out, _ := rb.run("search", "zebraword", "--color"); !strings.HasPrefix(out, "\x1b[1m"+tok+"\x1b[0m  ") {
		t.Fatalf("bold token: %q", out)
	}
	// JSON keeps the stored id, and get with it opens the item.
	var page struct {
		Results []struct {
			ID      string `json:"id"`
			Related []struct {
				ID string `json:"id"`
			} `json:"related"`
		} `json:"results"`
	}
	_, out, _ := rb.run("search", "zebraword", "--json")
	oneJSON(t, out, &page)
	if len(page.Results) != 2 || page.Results[0].ID != lfID || page.Results[1].Related[0].ID != lfID {
		t.Fatalf("JSON ids: %s", out)
	}
	if code, out, _ := rb.run("get", page.Results[0].ID, "--json"); code != 0 || !strings.Contains(out, `"id":"20261121-000000-a\nb"`) {
		t.Fatalf("get by the JSON id: %d %s", code, out)
	}
	// A check finding about a file whose name holds a line feed is one line.
	rb.put("notes/bad\npath.md", []string{"kind: note", "kind: note"}, "")
	code, out, _ := rb.run("check")
	if code != 1 || !slices.Contains(strings.Split(out, "\n"), `error bad_front_matter "notes/bad\u000apath.md"  duplicate_key`) {
		t.Fatalf("check: %d\n%s", code, out)
	}
}

// Spec 39 and 6.3: of two stems 78 ff and 78 ef bf bd, only the second is an item; one notice counts the
// first; get with the first's bytes finds nothing; check reports it as bad_file_name naming its folder.
// A .md file without front matter raises no notice.
func TestNameAdmissionCommands(t *testing.T) {
	rb := newReadBrain(t)
	rb.put("README.md", nil, "")
	os.WriteFile(filepath.Join(rb.dir, "README.md"), []byte("# A brain\n\nquokkaword\n"), 0o644)
	rb.put("notes/plain.md", []string{"kind: note", "title: Plain quokkaword", "author: agent:x"}, "quokkaword\n")
	if _, _, errOut := rb.run("search", "quokkaword"); errOut != "" {
		t.Fatalf("a file without front matter raised %q", errOut)
	}
	if !badNamesKept(t) {
		t.Skip(badNamesSkip)
	}
	rb.put("notes/x\xff.md", []string{"kind: note", "title: Bad name", "author: agent:x"}, "wombatword\n")
	rb.put("notes/x\uFFFD.md", []string{"kind: note", "title: Real name", "author: agent:x"}, "wombatword\n")
	for _, js := range []bool{false, true} {
		args := []string{"search", "wombatword"}
		if js {
			args = append(args, "--json")
		}
		code, out, errOut := rb.run(args...)
		want := "skipped_files: 1 file skipped: run pbrain check\n"
		if js {
			want = `{"notice":{"code":"skipped_files","message":"1 file skipped: run pbrain check"}}` + "\n"
		}
		if code != 0 || errOut != want {
			t.Fatalf("json %v: %d %q", js, code, errOut)
		}
		if js {
			var page struct {
				Results []struct {
					ID   string `json:"id"`
					Path string `json:"path"`
				} `json:"results"`
			}
			oneJSON(t, out, &page)
			if len(page.Results) != 1 || page.Results[0].ID != "x\uFFFD" || page.Results[0].Path != "notes/x\uFFFD.md" {
				t.Fatalf("results: %s", out)
			}
		} else if !strings.HasPrefix(out, "x\uFFFD  note") || !strings.Contains(out, "  notes/x\uFFFD.md:") || !strings.HasSuffix(out, "\n1 result.\n") {
			t.Fatalf("text: %s", out)
		}
	}
	if code, out, _ := rb.run("get", "x\uFFFD"); code != 0 || !strings.HasPrefix(out, "x\uFFFD\n") {
		t.Fatalf("get the real U+FFFD id: %d %s", code, out)
	}
	if code, _, errOut := rb.run("get", "x\xff"); code != 1 ||
		errOut != "skipped_files: 1 file skipped: run pbrain check\nnot_found: no item has that id in this view\n" {
		t.Fatalf("get the bad bytes: %d %q", code, errOut)
	}
	code, out, errOut := rb.run("check")
	if code != 1 || errOut != "" || !slices.Contains(strings.Split(out, "\n"),
		"error bad_file_name notes/  the file name is not valid UTF-8, so the file has no id; reads skip this file") {
		t.Fatalf("check: %d %q\n%s", code, errOut, out)
	}
}

// A refused replaces link names its target as one token (spec 6.2): graph hands the target over and
// cli writes it.
func TestRefusedReplacesTargetIsOneToken(t *testing.T) {
	rb := newReadBrain(t)
	const target = "20261121-120000-a\"b"
	rb.put("decisions/"+target+".md", []string{"kind: decision", "title: Quoted", "created: 2026-11-21T12:00:00Z",
		"author: human:x", "decided: human:x", "status: verified"}, "body\n")
	add := []string{"add", "--kind", "note", "--title", "New note", "--body", "new body", "--author", "agent:test", "--replaces", target}
	code, out, errOut := rb.run(add...)
	if code != 1 || out != "" || errOut != `invalid: replaces "20261121-120000-a\"b": target has higher standing`+"\n" {
		t.Fatalf("text: %d %q %q", code, out, errOut)
	}
	code, _, errOut = rb.run(append(add, "--json")...)
	var diag struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	oneJSON(t, errOut, &diag)
	if code != 1 || diag.Error.Code != "invalid" || diag.Error.Message != `replaces "20261121-120000-a\"b": target has higher standing` {
		t.Fatalf("JSON: %d %q", code, errOut)
	}
}

// badNamesSkip is why a case that needs a name that is not valid UTF-8 is skipped where the file
// system refuses one (APFS); the macOS side is in the Mac backlog.
const badNamesSkip = "the file system refuses names that are not valid UTF-8; macOS tracked in the Mac backlog"

// badNamesKept reports whether the file system under the test's temporary folder keeps such a name.
func badNamesKept(t *testing.T) bool {
	return os.WriteFile(filepath.Join(t.TempDir(), "x\xff"), nil, 0o600) == nil
}
