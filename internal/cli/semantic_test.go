package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/embed/embedtest"
	"github.com/pkieltyka/pbrain/internal/search"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// semanticBrain is a brain whose pbrain.toml names the test model, with the model's files at
// model_path and a cache folder of its own: no command reaches a network.
func semanticBrain(t *testing.T) (*readBrain, *embedtest.Kit) {
	t.Helper()
	rb := newReadBrain(t)
	kit := embedtest.New(nil)
	m := kit.Models[embedtest.ModelA]
	mp := filepath.Join(t.TempDir(), "model")
	os.MkdirAll(mp, 0o755)
	for _, f := range m.Files {
		os.WriteFile(filepath.Join(mp, f.Name), kit.Bytes[f.URL], 0o644)
	}
	os.WriteFile(filepath.Join(rb.dir, "pbrain.toml"), []byte("[semantic]\nmodel = \"test-model\"\n"), 0o644)
	os.WriteFile(filepath.Join(rb.dir, "personal.toml"), []byte("model_path = \""+mp+"\"\n"), 0o644)
	cache := filepath.Join(t.TempDir(), "cache")
	rb.env.Models = kit.Registry
	rb.env.Getenv = func(k string) string {
		if k == "PBRAIN_CACHE" {
			return cache
		}
		return ""
	}
	rb.put("notes/20261101-000000-alpha-a1a1.md", []string{"kind: note", "title: First note", "created: 2026-11-01T00:00:00Z", "author: agent:x"},
		"This note is about alpha topics.\n")
	rb.put("notes/20261102-000000-beta-b1b1.md", []string{"kind: note", "title: Second note", "created: 2026-11-02T00:00:00Z", "author: agent:x"},
		"Mostly beta material.\n")
	return rb, kit
}

func TestCLISemanticCommands(t *testing.T) {
	rb, kit := semanticBrain(t)
	// Before any build: the status, and a search with its notice and lexical results.
	expect(t, rb, "model: test-model, on this machine\nshared: 0 of 2 items indexed\n", "sync", "--status")
	status, out, errOut := rb.run("search", "alpha", "--semantic")
	if status != 0 || !strings.Contains(out, "alpha-a1a1") || errOut != "semantic_not_ready: no index for this model: run pbrain sync\n" {
		t.Fatalf("before the build: %d %q %q", status, out, errOut)
	}
	if kit.Opens.Load() != 0 {
		t.Fatal("a not-ready search opened the model")
	}
	// The build.
	expect(t, rb, "shared: 2 items indexed, 0 items removed; 2 passages embedded, 0 reused\n", "sync")
	var built struct {
		Model    string
		Sections []map[string]any
	}
	_, out, _ = rb.run("sync", "--json")
	oneJSON(t, out, &built)
	if built.Model != "test-model" || len(built.Sections) != 1 || built.Sections[0]["passages_embedded"] != 0.0 || built.Sections[0]["items"] != 2.0 {
		t.Fatalf("index JSON: %s", out)
	}
	expect(t, rb, "model: test-model, on this machine\nshared: 2 of 2 items indexed\n", "sync", "--status")
	// A search by meaning: "beta" is not in the first note, but the query vector of a text with no
	// keyword is the fixed unit vector, so ask for a word that matches nothing lexically.
	status, out, errOut = rb.run("search", "beta", "material", "zzz", "--semantic")
	if status != 0 || errOut != "" || !strings.Contains(out, "20261102-000000-beta-b1b1  note  provisional  2026-11-02  semantic\n") {
		t.Fatalf("a semantic hit: %d %q %q", status, out, errOut)
	}
	// A new item while another command holds the index lock: the search does not catch it up, and
	// the partial notice counts it.
	rb.put("notes/20261103-000000-gamma-c1c1.md", []string{"kind: note", "title: Third", "created: 2026-11-03T00:00:00Z", "author: agent:x"}, "gamma\n")
	idx, err := sys.OpenDir(filepath.Join(rb.dir, ".pbrain", "index"))
	if err != nil {
		t.Fatal(err)
	}
	held, err := sys.Native().Lock(context.Background(), idx, "lock")
	if err != nil {
		t.Fatal(err)
	}
	_, _, errOut = rb.run("search", "zzz", "--semantic")
	if errOut != "semantic_partial: 1 item is not in the index: run pbrain sync\n" {
		t.Fatalf("partial: %q", errOut)
	}
	_, _, errOut = rb.run("--personal", "search", "zzz", "--semantic", "--json")
	var n struct {
		Notice struct{ Code, Message string }
	}
	oneJSON(t, errOut, &n)
	if n.Notice.Code != "semantic_partial" || !strings.HasSuffix(n.Notice.Message, "run pbrain sync --personal") {
		t.Fatalf("personal hint: %q", errOut)
	}
	held.Close()
	idx.Close()
	// With the lock free, the next search catches the item up and prints no notice.
	if _, _, errOut = rb.run("search", "zzz", "--semantic"); errOut != "" {
		t.Fatalf("after the catch-up: %q", errOut)
	}
	// Status JSON, and a stale index after the model changes.
	var st map[string]any
	_, out, _ = rb.run("sync", "--status", "--json")
	json.Unmarshal([]byte(out), &st)
	if st["model"] != "test-model" || st["model_present"] != true {
		t.Fatalf("status JSON: %s", out)
	}
	os.WriteFile(filepath.Join(rb.dir, "pbrain.toml"), []byte("[semantic]\nmodel = \"test-model-b\"\n"), 0o644)
	expect(t, rb, "model: test-model-b, not on this machine\nshared: 0 of 3 items indexed (built for another model)\n", "sync", "--status")
	os.WriteFile(filepath.Join(rb.dir, "pbrain.toml"), []byte("[semantic]\nmodel = \"nope\"\n"), 0o644)
	expect(t, rb, "model: nope, not supported by this pbrain\nshared: 0 of 3 items indexed (built for another model)\n", "sync", "--status")
	_, _, errOut = rb.run("search", "zzz", "--semantic")
	if !strings.HasPrefix(errOut, "semantic_not_ready: this pbrain does not support") {
		t.Fatalf("unsupported: %q", errOut)
	}
	status, _, errOut = rb.run("sync")
	if status != 1 || !strings.HasPrefix(errOut, "invalid_config: the model \"nope\" is not supported; supported models: test-model") {
		t.Fatalf("index with an unknown model: %d %q", status, errOut)
	}
	// Turning semantic off takes precedence even over an unavailable local model folder.
	if err := os.WriteFile(filepath.Join(rb.dir, "pbrain.toml"), []byte(`semantic.model = "none"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rb.dir, "personal.toml"), []byte(`model_path = "missing"`), 0o644); err != nil {
		t.Fatal(err)
	}
	expect(t, rb, "model: none\nshared: 0 of 3 items indexed (built for another model)\n", "sync", "--status")
	_, _, errOut = rb.run("search", "zzz", "--semantic")
	if errOut != "semantic_not_ready: semantic search is off: pbrain.toml sets model = \"none\"\n" {
		t.Fatalf("no model: %q", errOut)
	}
	status, _, errOut = rb.run("sync")
	if status != 1 || !strings.HasPrefix(errOut, "no_model: ") {
		t.Fatalf("index without a model: %d %q", status, errOut)
	}
	// A search without the flag never opens the cache or the model.
	opens := kit.Opens.Load()
	rb.run("search", "alpha")
	if kit.Opens.Load() != opens {
		t.Fatal("a lexical search opened the model")
	}
}

// lockCount counts the TryLock calls a command makes.
type lockCount struct {
	sys.Files
	tries int
}

func (f *lockCount) TryLock(d sys.Dir, name string) (io.Closer, error) {
	f.tries++
	return f.Files.TryLock(d, name)
}

// Scenario 42 through the command: a search with --semantic catches up one added note, and a
// search without it loads no model, opens no cache and takes no lock.
func TestCLISemanticCatchUp(t *testing.T) {
	rb, kit := semanticBrain(t)
	expect(t, rb, "shared: 2 items indexed, 0 items removed; 2 passages embedded, 0 reused\n", "sync")
	rb.put("notes/20261103-000000-gamma-c1c1.md", []string{"kind: note", "title: Third", "created: 2026-11-03T00:00:00Z", "author: agent:x"}, "gamma topics\n")
	// Without --semantic: no model, no cache, no lock.
	lc := &lockCount{Files: sys.Native()}
	rb.env.Files = lc
	getenv := rb.env.Getenv
	missing := filepath.Join(t.TempDir(), "no-cache")
	rb.env.Getenv = func(k string) string {
		if k == "PBRAIN_CACHE" {
			return missing
		}
		return getenv(k)
	}
	opens := kit.Opens.Load()
	if status, _, errOut := rb.run("search", "gamma"); status != 0 || errOut != "" || kit.Opens.Load() != opens || lc.tries != 0 {
		t.Fatalf("a lexical search: %d %q, %d opens, %d locks", status, errOut, kit.Opens.Load()-opens, lc.tries)
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a lexical search created the cache folder")
	}
	rb.env.Getenv = getenv
	// With --semantic: the note is found by meaning with no notice, and a sync then embeds nothing.
	status, out, errOut := rb.run("search", "gamma", "zzz", "--semantic")
	if status != 0 || errOut != "" || !strings.Contains(out, "20261103-000000-gamma-c1c1") || lc.tries != 1 {
		t.Fatalf("the catch-up: %d %q %q, %d locks", status, out, errOut, lc.tries)
	}
	expect(t, rb, "shared: 3 items indexed, 0 items removed; 0 passages embedded, 0 reused\n", "sync")
}

func TestCLIIndexPersonal(t *testing.T) {
	rb, _ := semanticBrain(t)
	// A default personal folder that does not exist: zero, and nothing created.
	expect(t, rb, "shared: 2 items indexed, 0 items removed; 2 passages embedded, 0 reused\npersonal: 0 items indexed, 0 items removed; 0 passages embedded, 0 reused\n", "--personal", "sync")
	if _, err := os.Stat(filepath.Join(rb.dir, "personal")); !os.IsNotExist(err) {
		t.Fatal("the personal folder was created")
	}
	rb.put("personal/notes/20261104-000000-mine-m1m1.md", []string{"kind: note", "title: Mine", "created: 2026-11-04T00:00:00Z", "author: agent:x"}, "delta mine\n")
	expect(t, rb, "shared: 2 items indexed, 0 items removed; 0 passages embedded, 0 reused\npersonal: 1 item indexed, 0 items removed; 1 passage embedded, 0 reused\n", "--personal", "sync")
	expect(t, rb, "model: test-model, on this machine\nshared: 2 of 2 items indexed\npersonal: 1 of 1 item indexed\n", "--personal", "sync", "--status")
}

func TestDownloadMessage(t *testing.T) {
	k := embedtest.New(nil)
	m := k.Models[embedtest.ModelPair]
	if got := downloadMessage(m, 1_000_001); got != "downloading model test-model-pair, 2 MB, from models.test" {
		t.Fatalf("%q", got)
	}
}

// Every row of the notice table, in both views.
func TestSemanticNoticeTable(t *testing.T) {
	shared, _ := parseOptions([]string{"search", "x"}, Features{})
	personal, _ := parseOptions([]string{"--personal", "search", "x"}, Features{})
	personal.view = brain.PersonalView
	cases := []struct {
		in   *invocation
		st   search.SemanticState
		code string
		msg  string
	}{
		{&shared, search.SemanticState{Asked: true, Reason: search.NoModel}, "semantic_not_ready", "semantic search is off: pbrain.toml sets model = \"none\""},
		{&shared, search.SemanticState{Asked: true, Reason: search.UnsupportedModel}, "semantic_not_ready", "this pbrain does not support the model pbrain.toml names: pbrain sync lists the supported names"},
		{&shared, search.SemanticState{Asked: true, Reason: search.CacheExposed}, "semantic_not_ready", "the cache folder lies in the brain, the personal section or a Git repository: set PBRAIN_CACHE to a folder outside them"},
		{&personal, search.SemanticState{Asked: true, Reason: search.ModelAbsent}, "semantic_not_ready", "the model is not on this machine: run pbrain sync --personal, or set model_path"},
		{&shared, search.SemanticState{Asked: true, Reason: search.NoIndex}, "semantic_not_ready", "no index for this model: run pbrain sync"},
		{&personal, search.SemanticState{Asked: true, Reason: search.ModelFailed}, "semantic_not_ready", "the model could not be used: run pbrain sync --personal to check it"},
		{&shared, search.SemanticState{Asked: true, Ready: true, Missing: 1}, "semantic_partial", "1 item is not in the index: run pbrain sync"},
		{&personal, search.SemanticState{Asked: true, Ready: true, Missing: 7}, "semantic_partial", "7 items are not in the index: run pbrain sync --personal"},
	}
	for _, c := range cases {
		n, ok := semanticNotice(c.in, c.st)
		if !ok || n.Code != c.code || n.Message != c.msg {
			t.Errorf("%+v: %+v", c.st, n)
		}
	}
	for _, st := range []search.SemanticState{{}, {Asked: true, Ready: true}} {
		if _, ok := semanticNotice(&shared, st); ok {
			t.Errorf("%+v printed a notice", st)
		}
	}
}

// TestSemanticAlignment is step 1 of render.md's alignment fixture: a query whose clauses do not all
// match lexically finds, by meaning, a passage in one of two sections with the same title; its
// locator names that section, its offset is that section's outline row, and get opens there. The
// stored file has LF line ends, CRLF, or a byte-order mark before its front matter; text and JSON
// give the same line and offset, and that line of the stored file holds the section's heading.
func TestSemanticAlignment(t *testing.T) {
	layouts := map[string]func(string) string{
		"lf":   func(s string) string { return s },
		"crlf": func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") },
		"bom":  func(s string) string { return "\ufeff" + s },
	}
	for name, layout := range layouts {
		t.Run(name, func(t *testing.T) { semanticAlignment(t, layout) })
	}
}

func semanticAlignment(t *testing.T, layout func(string) string) {
	rb, _ := semanticBrain(t)
	os.WriteFile(filepath.Join(rb.dir, "pbrain.toml"), []byte("[semantic]\nmodel = \"test-model\"\n\n[get]\nmax_chars = 100\n"), 0o644)
	body := "Préambule ünïcode, ça va.\n\n# Alpha\n\nalpha text\n\n## Setup\n\nfirst setup delta words\n\n" +
		"\x60\x60\x60\n# not a heading\n\x60\x60\x60\n\n# Beta\n\nbeta text\n\n## Setup\n\nsecond setup gamma words\n"
	const id = "20261105-000000-align-a2a2"
	rb.put("notes/"+id+".md", []string{"kind: note", "title: Align", "created: 2026-11-05T00:00:00Z", "author: agent:x"}, body)
	path := filepath.Join(rb.dir, "notes", id+".md")
	stored, _ := os.ReadFile(path)
	stored = []byte(layout(string(stored)))
	if err := os.WriteFile(path, stored, 0o644); err != nil {
		t.Fatal(err)
	}
	fileLines := strings.Split(string(stored), "\n")
	if status, _, errOut := rb.run("sync"); status != 0 {
		t.Fatal(errOut)
	}
	var ol struct {
		Outline []struct {
			Offset   int    `json:"offset"`
			FileLine int    `json:"file_line"`
			Title    string `json:"title"`
		} `json:"outline"`
	}
	_, out, _ := rb.run("get", id, "--outline", "--json")
	json.Unmarshal([]byte(out), &ol)
	var setups []int
	for i, row := range ol.Outline {
		if row.Title == "Setup" {
			setups = append(setups, i)
		}
	}
	if len(setups) != 2 {
		t.Fatalf("outline: %s", out)
	}
	for query, row := range map[string]int{"delta zzqq": setups[0], "gamma zzqq": setups[1]} {
		var page struct {
			Results []struct {
				ID            string `json:"id"`
				Semantic      bool   `json:"semantic"`
				Section       string `json:"section"`
				SnippetOffset int    `json:"snippet_offset"`
				SnippetLine   int    `json:"snippet_line"`
			} `json:"results"`
		}
		status, out, errOut := rb.run("search", query, "--semantic", "--json")
		oneJSON(t, out, &page)
		var hit *struct {
			ID            string `json:"id"`
			Semantic      bool   `json:"semantic"`
			Section       string `json:"section"`
			SnippetOffset int    `json:"snippet_offset"`
			SnippetLine   int    `json:"snippet_line"`
		}
		for i := range page.Results {
			if page.Results[i].ID == id {
				hit = &page.Results[i]
			}
		}
		if status != 0 || errOut != "" || hit == nil || !hit.Semantic || hit.Section != "Setup" {
			t.Fatalf("%q: %d %q %s", query, status, errOut, out)
		}
		want := ol.Outline[row]
		if hit.SnippetOffset != want.Offset || hit.SnippetLine != want.FileLine {
			t.Fatalf("%q: offset %d line %d, want the outline row %+v", query, hit.SnippetOffset, hit.SnippetLine, want)
		}
		if l := strings.TrimSuffix(fileLines[hit.SnippetLine-1], "\r"); l != "## Setup" {
			t.Fatalf("%q: line %d of the stored file is %q", query, hit.SnippetLine, l)
		}
		// Text: the same line in the location, the same offset, and get opens on the same line.
		_, text, _ := rb.run("search", query, "--semantic")
		loc := fmt.Sprintf("notes/%s.md:%d  [body L", id, hit.SnippetLine)
		if !strings.Contains(text, loc) || !strings.Contains(text, fmt.Sprintf(", offset %d/", hit.SnippetOffset)) {
			t.Fatalf("%q: text does not name line %d offset %d:\n%s", query, hit.SnippetLine, hit.SnippetOffset, text)
		}
		var got struct {
			Body     string `json:"body"`
			LineFrom int    `json:"line_from"`
		}
		_, out, _ = rb.run("get", id, "--offset", fmt.Sprint(hit.SnippetOffset), "--json", "--max-chars", "2000")
		oneJSON(t, out, &got)
		if !strings.HasPrefix(got.Body, "## Setup") || got.LineFrom != hit.SnippetLine {
			t.Fatalf("%q: get at %d: line %d %q", query, hit.SnippetOffset, got.LineFrom, got.Body[:min(20, len(got.Body))])
		}
		_, text, _ = rb.run("get", id, "--offset", fmt.Sprint(hit.SnippetOffset), "--max-chars", "2000")
		if !strings.Contains(text, "\n## Setup") || !strings.Contains(text, fmt.Sprintf("; file L%d–", hit.SnippetLine)) {
			t.Fatalf("%q: text get at %d:\n%s", query, hit.SnippetOffset, text)
		}
	}
}
