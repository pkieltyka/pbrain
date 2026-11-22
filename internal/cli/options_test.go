package cli

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/query"
	"github.com/pkieltyka/pbrain/internal/render"
)

func TestCLIArgumentMatrix(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"init", []string{"--brain", "somewhere", "init", "--path=other"}, "init"},
		{"skill core", []string{"skill"}, "skill"},
		{"skill list", []string{"skill", "list"}, "skill"},
		{"skill topic", []string{"--personal", "skill", "import"}, "skill"},
		{"search", []string{"--json", "search", "java", "--kind=note", "queue"}, "search"},
		{"list", []string{"list", "--brain", "somewhere", "--tag", "Queue", "--tag=plan"}, "list"},
		{"get", []string{"get", "--offset", "010", "id"}, "get"},
		{"history", []string{"get", "id", "--history", "--limit", "200"}, "get"},
		{"outline", []string{"get", "--outline", "id", "--limit=200"}, "get"},
		{"prime", []string{"prime", "--tag", "api", "--offset", "0"}, "prime"},
		{"tags", []string{"tags", "--limit=200"}, "tags"},
		{"check", []string{"check", "--limit=201"}, "check"},
		{"add", []string{"add", "--title=example", "--body", "--json", "--tag", "a,b", "--tag=next"}, "add"},
		{"import file", []string{"import", "--shared", "--source", "citation", "file.md"}, "import"},
		{"linked empty summary", []string{"import", "https://example.test/doc", "--link-only", "--summary="}, "import"},
		{"verify", []string{"verify", "--by", "agent:tester", "id"}, "verify"},
		{"delete", []string{"delete", "--yes", "id"}, "delete"},
		{"audience list", []string{"audience", "--limit", "20"}, "audience"},
		{"audience set", []string{"audience", "set", "github.com/you", "shared", "--personal"}, "audience"},
		{"audience unset", []string{"audience", "unset", "github.com/you"}, "audience"},
		{"sync", []string{"sync", "--status"}, "sync"},
		{"literal", []string{"search", "--", "--json"}, "search"},
		{"leading delimiter", []string{"--", "search", "--json"}, "search"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, err := parseOptions(tc.args, Features{})
			if err != nil || in.cmd.name != tc.want {
				t.Fatalf("got %s, %v, want %s", in.cmd.name, err, tc.want)
			}
			switch tc.name {
			case "search":
				if !in.json || in.query != "java queue" || len(in.clauses) != 2 {
					t.Errorf("bad search parse: %+v", in)
				}
			case "get":
				if in.page.offset != 10 {
					t.Error("paging was not parsed in decimal")
				}
			case "add":
				if in.json || in.string("body") != "--json" || !slices.Equal(in.all("tag"), []string{"a,b", "next"}) {
					t.Error("flag value was split or parsed as an option")
				}
			case "list":
				if !slices.Equal(in.filter.Tags, []string{"queue", "plan"}) {
					t.Errorf("bad tags: %v", in.filter.Tags)
				}
			case "linked empty summary":
				if !in.has("summary") || in.string("summary") != "" {
					t.Error("the explicitly empty summary was lost")
				}
			case "sync":
				if !in.boolean("status") {
					t.Errorf("bad sync parse: %+v", in)
				}
			case "literal", "leading delimiter":
				if in.json || in.query != "--json" {
					t.Error("option processing continued after --")
				}
			}
		})
	}
	const unknown = "unknown command; run pbrain --help"
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"index"}, "usage: " + unknown + "\n"},
		{[]string{"index", "--status"}, "usage: " + unknown + "\n"},
		{[]string{"--json", "index"}, `{"error":{"code":"usage","message":"` + unknown + `"}}` + "\n"},
		{[]string{"index", "--status", "--json"}, `{"error":{"code":"usage","message":"` + unknown + `"}}` + "\n"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			status, stdout, stderr := invoke(t, Env{Files: noFiles{}}, Features{}, tc.args...)
			if status != 2 || stdout != "" || stderr != tc.want {
				t.Fatalf("status %d stdout %q stderr %q, want exit 2 and %q", status, stdout, stderr, tc.want)
			}
		})
	}
}

func requireCode(t *testing.T, err error, want string) {
	t.Helper()
	var actual *errs.Error
	if !errors.As(err, &actual) || actual.Code != want {
		t.Errorf("got %v, want %q", err, want)
	}
}

func TestWrongUsageBeforeDiscovery(t *testing.T) {
	for _, tc := range []string{
		"", "missing", "--title name add", "add", "add --title name --body a --body-file b",
		"add --title name --personal --shared", "add --title name --body-file=", "init --path=",
		"init --brain=", "init extra", "import", "import file1 file2", "import path --link-only",
		"import https://example.test --source citation", "import https://example.test --summary=",
		"get", "get a b", "get id --outline --history", "get id --limit 4", "get id --offset=-1",
		"get id --max-chars=0", "search", "search --context 0 query", "search --context 4 query",
		"search query --context", "search query --context=0x2", "search query --limit 51", "search query --kind alias",
		"search query --status fake", "list --limit 201", "list --context 1", "list --color", "list --semantic",
		"prime --limit 10", "tags --limit 201", "check --offset 999999999999999999999999",
		"audience set foo shared --limit 1", "audience unset foo --offset 0", "audience set foo anything",
		"audience wrong", "skill one two", "verify", "delete a b", "sync extra", "search --ver query",
	} {
		t.Run(tc, func(t *testing.T) {
			_, err := parseOptions(strings.Fields(tc), Features{})
			requireCode(t, err, "usage")
		})
	}
	_, err := parseOptions([]string{"add", "--title="}, Features{})
	requireCode(t, err, "invalid")
}

func TestCLIFormatAndRetrievalFlags(t *testing.T) {
	for _, retrieval := range [][]string{nil, {"--bm25"}, {"--semantic"}, {"--semantic", "--bm25"}} {
		for _, n := range []string{"", "1", "2", "3"} {
			args := append([]string{"search", "queue"}, retrieval...)
			if n != "" {
				args = append(args, "--context", n)
			}
			in, err := parseOptions(args, Features{BM25: true})
			if err != nil || in.context != strings.Index("0123", n) {
				t.Errorf("%v: context=%d, %v", args, in.context, err)
			}
		}
	}
	for _, args := range [][]string{
		{"search", "--color", "term"}, {"get", "--color", "id"}, {"get", "--history", "--color", "id"}, {"get", "id", "--outline", "--color"},
	} {
		in, err := parseOptions(args, Features{})
		if err != nil || in.format != render.TextColor {
			t.Errorf("%v: format %d, %v", args, in.format, err)
		}
	}
	for _, args := range [][]string{
		{"search", "--json", "--color", "term"}, {"--json", "get", "--color", "id"}, {"get", "--color", "id", "--json"},
	} {
		in, err := parseOptions(args, Features{})
		requireCode(t, err, "usage")
		if !in.json {
			t.Errorf("%v failed to preserve JSON diagnostics", args)
		}
	}
	for _, args := range [][]string{
		{"search", "term", "--bm25"},
		{"search", "term", "--semantic", "--bm25", "--json"},
	} {
		in, err := parseOptions(args, Features{})
		requireCode(t, err, "usage")
		if len(args) > 3 && !in.json {
			t.Error("held-back flag prevented JSON diagnostics")
		}
	}
	_, err := parseOptions([]string{"list", "--bm25"}, Features{BM25: true})
	requireCode(t, err, "usage")
}

func TestModeSpecificPageDefaults(t *testing.T) {
	cfg := brain.Config{Search: brain.Limits{Limit: 7, MaxChars: 2345}, List: brain.Limits{Limit: 37, MaxChars: 1122}}
	cfg.Get.MaxChars, cfg.Prime.MaxChars = 6005, 1234
	for _, tc := range []struct {
		command string
		page    pageOptions
	}{
		{"search q", pageOptions{7, 0, 2345}}, {"search q --max-chars=4 --limit=2", pageOptions{2, 0, 4}},
		{"list", pageOptions{37, 0, 1122}}, {"get id", pageOptions{0, 0, 6005}},
		{"get id --history", pageOptions{20, 0, 2000}}, {"get id --outline", pageOptions{50, 0, 2000}},
		{"prime", pageOptions{0, 0, 1234}}, {"audience", pageOptions{20, 0, 2000}},
		{"check", pageOptions{50, 0, 4000}}, {"tags", pageOptions{50, 0, 2000}},
	} {
		in, err := parseOptions(strings.Fields(tc.command), Features{})
		if err != nil {
			t.Fatal(err)
		}
		in.useConfig(cfg)
		if in.page != tc.page {
			t.Errorf("%q: page %+v, want %+v", tc.command, in.page, tc.page)
		}
	}
	in, err := parseOptions([]string{"search", `"queue retry"`}, Features{})
	if err != nil || in.page != (pageOptions{5, 0, 3000}) || len(in.clauses) != 1 || in.clauses[0].Kind != query.Phrase {
		t.Errorf("search defaults or literal quote: %+v, %v", in, err)
	}
}
