package skill

import (
	"io/fs"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSkillResourcesAndLimits(t *testing.T) {
	checkText := func(name, text string, limit int) {
		t.Helper()
		if !utf8.ValidString(text) || !strings.HasSuffix(text, "\n") {
			t.Errorf("%s: expected UTF-8 with a final LF", name)
		}
		if n := len(strings.Fields(text)); n == 0 || n > limit {
			t.Errorf("%s has %d words, want 1 through %d", name, n, limit)
		}
	}
	core := Core()
	checkText("core", core, 400)
	for _, guidance := range []string{"'cach*'", "six or more words", "--bm25", "5/6", "Read semantic and partial hits", "path:line", "get <id> --offset <n>", "--outline", "--context 2", "--color", "before the first, read pbrain skill semantic"} {
		if !strings.Contains(core, guidance) {
			t.Errorf("core is missing %q", guidance)
		}
	}
	semantic, _ := Topic("semantic")
	for _, guidance := range []string{"first pbrain index on a machine is a person's step", "about 1.5 GB", "model_download", "pbrain index --status"} {
		if !strings.Contains(semantic, guidance) {
			t.Errorf("semantic topic is missing %q", guidance)
		}
	}
	files, err := fs.Glob(documents, "topics/*.md")
	if err != nil {
		t.Fatal(err)
	}
	var expected []string
	for _, name := range Topics() {
		expected = append(expected, "topics/"+name+".md")
		text, ok := Topic(name)
		if !ok {
			t.Fatalf("topic %q missing", name)
		}
		checkText(name, text, 300)
	}
	slices.Sort(expected)
	if !slices.Equal(files, expected) {
		t.Errorf("embedded %v; advertised %v", files, expected)
	}
}

func TestSkillListOrder(t *testing.T) {
	want := []string{"import", "claude-artifacts", "google-docs", "browser-use", "semantic"}
	names := Topics()
	if !slices.Equal(names, want) {
		t.Fatalf("got %v; want %v", names, want)
	}
	names[0] = "changed"
	if !slices.Equal(Topics(), want) {
		t.Fatal("caller mutated the topic table")
	}
}

func TestUnknownSkillTopic(t *testing.T) {
	for _, name := range []string{"", "Import", "unknown", "../core", "topics/import.md", "/tmp/skill"} {
		if text, ok := Topic(name); ok || text != "" {
			t.Errorf("accepted unknown topic %q", name)
		}
	}
}

func TestCoreRoutesToImport(t *testing.T) {
	if !strings.Contains(Core(), "pbrain skill import") {
		t.Fatal("core lacks its import topic route")
	}
}
