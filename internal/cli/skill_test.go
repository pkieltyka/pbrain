package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/gitx"
	"github.com/pkieltyka/pbrain/internal/importer/fetch"
	"github.com/pkieltyka/pbrain/internal/skill"
)

func TestSkillWithoutBrain(t *testing.T) {
	// Any call through these unimplemented interfaces panics and makes Run fail.
	env := Env{
		Files: noFiles{},
		Git:   struct{ gitx.Runner }{},
		Net: fetch.Net{
			Resolver: struct{ fetch.Resolver }{},
			Dialer:   struct{ fetch.Dialer }{},
		},
	}
	for _, name := range append([]string{"", "list"}, skill.Topics()...) {
		args := []string{"--brain", "missing", "--personal", "skill"}
		want := skill.Core()
		if name != "" {
			args = append(args, name)
			if name == "list" {
				want = strings.Join(skill.Topics(), "\n") + "\n"
			} else {
				want, _ = skill.Topic(name)
			}
		}
		exit, out, diag := invoke(t, env, Features{}, args...)
		if exit != 0 || diag != "" || out != want {
			t.Fatalf("%v: exit %d, stderr %q, exact text match %t", args, exit, diag, out == want)
		}

		exit, out, diag = invoke(t, env, Features{}, append(args, "--json")...)
		if exit != 0 || diag != "" {
			t.Fatalf("%v --json: exit %d, stderr %q", args, exit, diag)
		}
		if name == "list" {
			var got map[string][]string
			oneJSON(t, out, &got)
			if len(got) != 1 || !slices.Equal(got["topics"], skill.Topics()) {
				t.Errorf("topic list: %s", out)
			}
		} else {
			var got map[string]string
			oneJSON(t, out, &got)
			if len(got) != 1 || got["text"] != want {
				t.Errorf("%v --json: wrong embedded document", args)
			}
		}
	}
}

func TestUnknownSkillTopic(t *testing.T) {
	for _, operands := range [][]string{{"unknown"}, {"../skill/core.md"}, {"topics/import.md"}, {""}, {"list", "import"}} {
		args := append([]string{"skill"}, operands...)
		exit, out, diag := invoke(t, Env{Files: noFiles{}}, Features{}, args...)
		if exit != 2 || out != "" || !strings.HasPrefix(diag, "usage: ") {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", args, exit, out, diag)
		}
		exit, out, diag = invoke(t, Env{Files: noFiles{}}, Features{}, append([]string{"--json"}, args...)...)
		if exit != 2 || out != "" || diagnosticCode(t, diag, "error") != "usage" {
			t.Errorf("--json %v: exit %d, stdout %q, stderr %q", args, exit, out, diag)
		}
	}
}

func TestImportTopicExamples(t *testing.T) {
	const url = "https://example.org/artifact"
	for _, example := range []struct {
		topic, form string
		args        []string
	}{
		{"import", "pbrain import --link-only <url>", []string{"import", "--link-only", url}},
		{"import", "--summary", []string{"import", "--link-only", url, "--summary", "pricing thread", "--personal"}},
		{"import", "add --source <message-permalink>", []string{"add", "--source", url, "--title", "Slack note", "--personal"}},
		{"claude-artifacts", "pbrain import <file> --source <artifact-url>", []string{"import", "export.md", "--source", url}},
	} {
		doc, ok := skill.Topic(example.topic)
		if !ok || !strings.Contains(doc, example.form) {
			t.Fatalf("example %q missing from %s", example.form, example.topic)
		}
		if _, err := parseOptions(example.args, Features{}); err != nil {
			t.Errorf("%s: %v: %v", example.topic, example.args, err)
		}
	}
}
