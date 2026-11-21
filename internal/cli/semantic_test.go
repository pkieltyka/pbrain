package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/embed/embedtest"
	"github.com/pkieltyka/pbrain/internal/search"
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
	expect(t, rb, "model: test-model, on this machine\nshared: 0 of 2 items indexed\n", "index", "--status")
	status, out, errOut := rb.run("search", "alpha", "--semantic")
	if status != 0 || !strings.Contains(out, "alpha-a1a1") || errOut != "semantic_not_ready: no index for this model: run pbrain index\n" {
		t.Fatalf("before the build: %d %q %q", status, out, errOut)
	}
	if kit.Opens.Load() != 0 {
		t.Fatal("a not-ready search opened the model")
	}
	// The build.
	expect(t, rb, "shared: 2 items indexed, 0 items removed; 2 passages embedded, 0 reused\n", "index")
	var built struct {
		Model    string
		Sections []map[string]any
	}
	_, out, _ = rb.run("index", "--json")
	oneJSON(t, out, &built)
	if built.Model != "test-model" || len(built.Sections) != 1 || built.Sections[0]["passages_embedded"] != 0.0 || built.Sections[0]["items"] != 2.0 {
		t.Fatalf("index JSON: %s", out)
	}
	expect(t, rb, "model: test-model, on this machine\nshared: 2 of 2 items indexed\n", "index", "--status")
	// A search by meaning: "beta" is not in the first note, but the query vector of a text with no
	// keyword is the fixed unit vector, so ask for a word that matches nothing lexically.
	status, out, errOut = rb.run("search", "beta", "material", "zzz", "--semantic")
	if status != 0 || errOut != "" || !strings.Contains(out, "20261102-000000-beta-b1b1  note  provisional  2026-11-02  semantic\n") {
		t.Fatalf("a semantic hit: %d %q %q", status, out, errOut)
	}
	// A new item not yet indexed: the partial notice counts it.
	rb.put("notes/20261103-000000-gamma-c1c1.md", []string{"kind: note", "title: Third", "created: 2026-11-03T00:00:00Z", "author: agent:x"}, "gamma\n")
	_, _, errOut = rb.run("search", "zzz", "--semantic")
	if errOut != "semantic_partial: 1 item is not in the index: run pbrain index\n" {
		t.Fatalf("partial: %q", errOut)
	}
	_, _, errOut = rb.run("--personal", "search", "zzz", "--semantic", "--json")
	var n struct {
		Notice struct{ Code, Message string }
	}
	oneJSON(t, errOut, &n)
	if n.Notice.Code != "semantic_partial" || !strings.HasSuffix(n.Notice.Message, "run pbrain index --personal") {
		t.Fatalf("personal hint: %q", errOut)
	}
	// Status JSON, and a stale index after the model changes.
	var st map[string]any
	_, out, _ = rb.run("index", "--status", "--json")
	json.Unmarshal([]byte(out), &st)
	if st["model"] != "test-model" || st["model_present"] != true {
		t.Fatalf("status JSON: %s", out)
	}
	os.WriteFile(filepath.Join(rb.dir, "pbrain.toml"), []byte("[semantic]\nmodel = \"test-model-b\"\n"), 0o644)
	expect(t, rb, "model: test-model-b, not on this machine\nshared: 0 of 3 items indexed (built for another model)\n", "index", "--status")
	os.WriteFile(filepath.Join(rb.dir, "pbrain.toml"), []byte("[semantic]\nmodel = \"nope\"\n"), 0o644)
	expect(t, rb, "model: nope, not supported by this pbrain\nshared: 0 of 3 items indexed (built for another model)\n", "index", "--status")
	_, _, errOut = rb.run("search", "zzz", "--semantic")
	if !strings.HasPrefix(errOut, "semantic_not_ready: this pbrain does not support") {
		t.Fatalf("unsupported: %q", errOut)
	}
	status, _, errOut = rb.run("index")
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
	expect(t, rb, "model: none\nshared: 0 of 3 items indexed (built for another model)\n", "index", "--status")
	_, _, errOut = rb.run("search", "zzz", "--semantic")
	if errOut != "semantic_not_ready: semantic search is off: pbrain.toml sets model = \"none\"\n" {
		t.Fatalf("no model: %q", errOut)
	}
	status, _, errOut = rb.run("index")
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

func TestCLIIndexPersonal(t *testing.T) {
	rb, _ := semanticBrain(t)
	// A default personal folder that does not exist: zero, and nothing created.
	expect(t, rb, "shared: 2 items indexed, 0 items removed; 2 passages embedded, 0 reused\npersonal: 0 items indexed, 0 items removed; 0 passages embedded, 0 reused\n", "--personal", "index")
	if _, err := os.Stat(filepath.Join(rb.dir, "personal")); !os.IsNotExist(err) {
		t.Fatal("the personal folder was created")
	}
	rb.put("personal/notes/20261104-000000-mine-m1m1.md", []string{"kind: note", "title: Mine", "created: 2026-11-04T00:00:00Z", "author: agent:x"}, "delta mine\n")
	expect(t, rb, "shared: 2 items indexed, 0 items removed; 0 passages embedded, 0 reused\npersonal: 1 item indexed, 0 items removed; 1 passage embedded, 0 reused\n", "--personal", "index")
	expect(t, rb, "model: test-model, on this machine\nshared: 2 of 2 items indexed\npersonal: 1 of 1 item indexed\n", "--personal", "index", "--status")
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
		{&shared, search.SemanticState{Asked: true, Reason: search.UnsupportedModel}, "semantic_not_ready", "this pbrain does not support the model pbrain.toml names: pbrain index lists the supported names"},
		{&shared, search.SemanticState{Asked: true, Reason: search.CacheExposed}, "semantic_not_ready", "the cache folder lies in the brain, the personal section or a Git repository: set PBRAIN_CACHE to a folder outside them"},
		{&personal, search.SemanticState{Asked: true, Reason: search.ModelAbsent}, "semantic_not_ready", "the model is not on this machine: run pbrain index --personal, or set model_path"},
		{&shared, search.SemanticState{Asked: true, Reason: search.NoIndex}, "semantic_not_ready", "no index for this model: run pbrain index"},
		{&personal, search.SemanticState{Asked: true, Reason: search.ModelFailed}, "semantic_not_ready", "the model could not be used: run pbrain index --personal to check it"},
		{&shared, search.SemanticState{Asked: true, Ready: true, Missing: 1}, "semantic_partial", "1 item is not in the index: run pbrain index"},
		{&personal, search.SemanticState{Asked: true, Ready: true, Missing: 7}, "semantic_partial", "7 items are not in the index: run pbrain index --personal"},
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
	if status, _, errOut := rb.run("index"); status != 0 {
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
