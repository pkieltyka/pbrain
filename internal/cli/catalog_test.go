package cli

import (
	"encoding/json"
	"fmt"
	"maps"
	"math/rand"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
)

func TestCatalogCoversCommands(t *testing.T) {
	var got []string
	for _, tool := range Catalog(Release) {
		if tool.Name != "pbrain_"+tool.Command || tool.Description == "" || len(tool.Output) == 0 || len(strings.Fields(tool.Description)) > 80 {
			t.Fatalf("invalid tool description or name: %+v", tool)
		}
		for _, code := range tool.Errors {
			if errorDescriptions[code] == "" {
				t.Errorf("%s references unknown error %q", tool.Command, code)
			}
		}
		for _, code := range tool.Notices {
			if noticeDescriptions[code] == "" {
				t.Errorf("%s references unknown notice %q", tool.Command, code)
			}
		}
		got = append(got, tool.Command)
	}
	var want []string
	for _, cmd := range commands {
		switch cmd.name {
		case "skill", "schema":
			if cmd.callable {
				t.Fatalf("%s cannot be called as a tool", cmd.name)
			}
		default:
			if !cmd.callable {
				t.Fatalf("%s has no catalog decision", cmd.name)
			}
			want = append(want, cmd.name)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("catalog = %v, want %v", got, want)
	}
}

func TestCatalogInputs(t *testing.T) {
	for _, features := range []Features{{}, Release} {
		for _, tool := range Catalog(features) {
			t.Run(fmt.Sprintf("%s/bm25=%t", tool.Command, features.BM25), func(t *testing.T) {
				cmd := commandNamed(tool.Command, features)
				var schema jsonSchema
				if err := json.Unmarshal(tool.Input, &schema); err != nil {
					t.Fatal(err)
				}
				want := map[string]bool{}
				if cmd.name != "devices" {
					want["brain"] = true
					if cmd.name != "init" {
						want["personal"] = true
					}
				}
				arguments := map[string]any{}
				var required []string
				for _, o := range cmd.options {
					if o.name == "color" {
						continue
					}
					want[o.name] = true
					if o.required {
						required = append(required, o.name)
						arguments[o.name] = "title"
					}
				}
				for _, arg := range cmd.args {
					want[arg.name] = true
					if arg.required {
						required = append(required, arg.name)
						arguments[arg.name] = "sample"
					}
				}
				if got := slices.Sorted(maps.Keys(schema.Properties)); !slices.Equal(got, slices.Sorted(maps.Keys(want))) {
					t.Fatalf("properties = %v, want %v", got, want)
				}
				if !slices.Equal(schema.Required, required) {
					t.Fatalf("required = %v, want %v", schema.Required, required)
				}
				if schema.Type != "object" || schema.AdditionalProperties == nil || *schema.AdditionalProperties {
					t.Fatalf("input schema must be a closed object: %s", tool.Input)
				}
				bindParsed(t, tool.Name, arguments, features, "")
				for _, arg := range cmd.args {
					property := schema.Properties[arg.name]
					if property.Type != "string" || len(strings.Fields(property.Description)) > 30 {
						t.Fatalf("%s: invalid operand schema", arg.name)
					}
					if arg.form == choice && !slices.Equal(property.Enum, arg.choices()) {
						t.Fatalf("%s: operand has wrong choices", arg.name)
					}
				}
				for _, o := range toolOptions(cmd) {
					property := schema.Properties[o.name]
					// The approved shared help text is the sole length exception.
					if words := len(strings.Fields(property.Description)); words > 30 && property.Description != personalHelp {
						t.Errorf("%s has %d words in its description", o.name, words)
					}
					switch o.form {
					case choice:
						if o.kind == repeated {
							property = property.Items
						}
						if !slices.Equal(property.Enum, o.choices()) {
							t.Errorf("%s choices = %v, want %v", o.name, property.Enum, o.choices())
						}
					case integer:
						testNumericProperty(t, cmd, o, property, arguments, features)
					}
				}
			})
		}
	}
}

func testNumericProperty(t *testing.T, cmd command, o option, prop *jsonSchema, minimal map[string]any, features Features) {
	t.Helper()
	modes := []string{""}
	if cmd.name == "get" {
		modes = append(modes, "history", "outline")
	}
	for _, mode := range modes {
		if o.name == "limit" && cmd.name == "get" && mode == "" {
			continue // this flag requires a paginated get
		}
		lo, hi, def := bounds(cmd.name, o.name, mode == "history", mode == "outline")
		if prop.Type != "integer" || prop.Minimum == nil || *prop.Minimum != lo ||
			hi == 0 && prop.Maximum != nil || hi != 0 && (prop.Maximum == nil || *prop.Maximum != hi) {
			t.Fatalf("%s %s %s: schema bounds %+v, want %d..%d", cmd.name, mode, o.name, prop, lo, hi)
		}
		if cmd.name == "get" && (o.name == "limit" || o.name == "max-chars") {
			if prop.Default != nil || !strings.Contains(prop.Description, fmt.Sprintf("%d", def)) {
				t.Fatalf("get %s lacks the %s default %d in its description: %+v", o.name, mode, def, prop)
			}
		} else {
			var want any
			if def >= lo && (hi == 0 || def <= hi) {
				want = float64(def)
			}
			if prop.Default != want {
				t.Fatalf("%s %s default = %v, want %v", cmd.name, o.name, prop.Default, want)
			}
		}
		args := maps.Clone(minimal)
		if mode != "" {
			args[mode] = true
		}
		message := fmt.Sprintf("--%s requires an integer of at least %d", o.name, lo)
		if hi != 0 {
			message = fmt.Sprintf("--%s requires an integer from %d to %d", o.name, lo, hi)
			args[o.name] = hi
			bindParsed(t, "pbrain_"+cmd.name, args, features, "")
			args[o.name] = hi + 1
			bindParsed(t, "pbrain_"+cmd.name, args, features, message)
		}
		args[o.name] = lo
		bindParsed(t, "pbrain_"+cmd.name, args, features, "")
		args[o.name] = lo - 1
		bindParsed(t, "pbrain_"+cmd.name, args, features, message)
	}
}

func bindParsed(t *testing.T, tool string, args any, features Features, message string) {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	argv, err := Bind(tool, raw, features)
	if err != nil {
		t.Fatal(err)
	}
	_, err = parseOptions(argv, features)
	env := Env{Models: embed.NewRegistry()}
	exit, _, stderr := invoke(t, env, features, argv...)
	if message == "" {
		if err != nil || exit == 2 {
			t.Fatalf("%v: parse %v, CLI exit %d: %s", argv, err, exit, stderr)
		}
		return
	}
	want, _ := encodeLine(map[string]Diagnostic{"error": {Code: "usage", Message: message}})
	if err == nil || domainError(err).Code != "usage" || domainError(err).Message != message ||
		exit != 2 || stderr != string(want) {
		t.Fatalf("%v: parse %v, exit %d, stderr %s, want usage %q", argv, err, exit, stderr, message)
	}
}

func TestCatalogRules(t *testing.T) {
	for _, tc := range []struct{ tool, arguments, message string }{
		{"get", `{"id":"id","history":true,"outline":true}`, "--history cannot be used with --outline"},
		{"get", `{"id":"id","limit":1}`, "--limit requires --history or --outline on get"},
		{"add", `{"title":"name","body":"x","body-file":"x"}`, "--body cannot be used with --body-file"},
		{"add", `{"title":"name","body-file":""}`, "--body-file requires a nonempty path"},
		{"add", `{"title":"name","kind":"decision"}`, "a decision requires --decided and refuses --verified"},
		{"add", `{"title":"name","decided":"person"}`, "--decided is only for a decision"},
		{"import", `{"input":"https://example.test/","source":"x"}`, "--source is only for a file import"},
		{"import", `{"input":"path","link-only":true}`, "--link-only requires a URL"},
		{"import", `{"input":"path","summary":"x"}`, "--summary requires --link-only"},
		{"audience", `{"action":"set","match":"x","to":"shared","limit":1}`, "paging options are only for audience listing"},
		{"add", `{"title":"name","shared":true,"personal":true}`, "--shared cannot be used with --personal"},
		{"sync", `{"status":true,"device":"cpu"}`, "--device cannot be used with --status"},
		{"search", `{"query":"text","kind":["invalid"]}`, "--kind requires an item kind"},
		{"search", `{"query":"text","status":"invalid"}`, "--status requires provisional or verified"},
		{"list", `{"kind":["invalid"]}`, "--kind requires an item kind"},
		{"list", `{"status":"invalid"}`, "--status requires provisional or verified"},
		{"import", `{"input":"path","shared":true,"personal":true}`, "--shared cannot be used with --personal"},
		{"audience", `{"action":"set","match":"x","to":"invalid"}`, "audience set requires a match and shared or personal"},
		{"audience", `{"action":"invalid"}`, "unknown audience operation"},
		{"add", `{}`, "add requires --title and takes no operands"},
	} {
		bindParsed(t, "pbrain_"+tc.tool, json.RawMessage(tc.arguments), Release, tc.message)
	}
}

func TestBindStrictArguments(t *testing.T) {
	argv, err := Bind("pbrain_search", json.RawMessage(`{"brain":"a b","query":"--quoted words","kind":["decision","source"],"limit":2e0,"tag":["a,b","--flag"],"personal":false}`), Release)
	want := []string{"--brain=a b", "--json", "search", "--kind=decision", "--kind=source", "--tag=a,b", "--tag=--flag", "--limit=2", "--", "--quoted words"}
	if err != nil || !slices.Equal(argv, want) {
		t.Fatalf("Bind = %v, %v, want %v", argv, err, want)
	}
	for _, tc := range []struct{ tool, raw, message string }{
		{"search", `{}`, `requires argument "query"`},
		{"search", `{"query":"x","query":"y"}`, `repeats argument "query"`},
		{"search", `{"query":"x","q":"y"}`, `no argument "q"`},
		{"search", `{"query":"x","personal":null}`, `argument "personal"`},
		{"search", `{"query":"x","limit":1.1}`, `argument "limit"`},
		{"search", `{"query":"x","limit":"2"}`, `argument "limit"`},
		{"search", `{"query":"x","kind":[null]}`, `argument "kind"`},
		{"search", `{"query":"x","kind":"note"}`, `argument "kind"`},
		{"search", `{"query":"x\u0000"}`, `argument "query"`},
		{"search", `{"Query":"x"}`, `no argument "Query"`},
		{"search", `{"query":"x"} {}`, "arguments must be a JSON object"},
		{"search", `null`, "arguments must be a JSON object"},
		{"search", `[]`, "arguments must be a JSON object"},
		{"audience", `{"action":"unset","to":"personal"}`, `argument "to" has an earlier missing operand`},
		{"audience", `{"match":"unset","to":"personal"}`, `argument "match" has an earlier missing operand`},
		{"skill", `{}`, "unknown tool"},
	} {
		got, err := Bind("pbrain_"+tc.tool, json.RawMessage(tc.raw), Release)
		if err == nil || domainError(err).Code != "usage" || !strings.Contains(err.Error(), tc.message) || got != nil {
			t.Errorf("%s %s = %v, %v, want %s", tc.tool, tc.raw, got, err, tc.message)
		}
	}
}

func TestCallMatchesCLI(t *testing.T) {
	rb, _ := semanticBrain(t)
	rb.put("notes/broken.md", []string{"kind: invalid", "title: Broken"}, "")
	id := "20261101-000000-alpha-a1a1"
	for _, tc := range []struct{ tool, args string }{
		{"search", `{"query":"alpha","semantic":true}`},
		{"search", `{"query":"alpha","limit":51}`},
		{"get", fmt.Sprintf(`{"id":%q,"outline":true}`, id)},
		{"get", `{"id":"missing"}`},
		{"check", `{}`},
		{"delete", fmt.Sprintf(`{"id":%q}`, id)},
	} {
		tool := "pbrain_" + tc.tool
		args := json.RawMessage(tc.args)
		argv, err := Bind(tool, args, Release)
		if err != nil {
			t.Fatal(err)
		}
		exit, out, stderr := invoke(t, rb.env, Release, argv...)
		var live []Diagnostic
		got := Call(t.Context(), rb.env, tool, args, Release, func(d Diagnostic) { live = append(live, d) })
		var diagnostics strings.Builder
		for _, notice := range got.Notices {
			line, _ := encodeLine(map[string]Diagnostic{"notice": notice})
			diagnostics.Write(line)
		}
		if got.Error != nil {
			line, _ := encodeLine(map[string]Diagnostic{"error": *got.Error})
			diagnostics.Write(line)
		}
		if got.Exit != exit || string(got.Output) != strings.TrimSpace(out) ||
			diagnostics.String() != stderr || !slices.Equal(live, got.Notices) {
			t.Fatalf("%s: %+v live %+v; CLI = %d %q %q", tc.args, got, live, exit, out, stderr)
		}
	}
}

func TestCallClearsStdin(t *testing.T) {
	env := writeEnv(t)
	env.Rand = rand.New(rand.NewSource(80))
	env.Stdin = readerFunc(func([]byte) (int, error) {
		t.Fatal("a tool read the launch standard input")
		return 0, nil
	})
	result := Call(t.Context(), env, "pbrain_add", json.RawMessage(`{"title":"Through the catalog"}`), Release, nil)
	if result.Exit != 0 || len(result.Output) == 0 || result.Error != nil {
		t.Fatalf("add through Call: %+v", result)
	}
}

func TestCatalogAddKindsAndAudience(t *testing.T) {
	rb, _ := semanticBrain(t)
	rb.env.Rand = rand.New(rand.NewSource(80))
	for _, k := range append(firstPartyKinds(), "source") {
		args := map[string]string{"title": "catalog fixture", "kind": k, "author": "agent:example"}
		if k == "decision" {
			args["decided"] = "person"
		}
		raw, _ := json.Marshal(args)
		got := Call(t.Context(), rb.env, "pbrain_add", raw, Release, nil)
		if k == "source" {
			if got.Exit != 1 || got.Error == nil || got.Error.Code != "invalid" || got.Error.Message != "kind: add accepts decision, learning, observation or note" {
				t.Fatalf("source add: %+v", got)
			}
		} else if got.Exit != 0 {
			t.Fatalf("%s add: %+v", k, got.Error)
		}
	}
	audience := func(raw string) Result {
		return Call(t.Context(), rb.env, "pbrain_audience", json.RawMessage(raw), Release, nil)
	}
	got := audience(`{"action":"set","match":"personal","to":"personal"}`)
	if got.Exit != 0 {
		t.Fatalf("set: %+v", got.Error)
	}
	before := audience(`{}`)
	if before.Exit != 0 || !strings.Contains(string(before.Output), "personal") {
		t.Fatalf("list: %+v", before)
	}
	for _, raw := range []string{`{"action":"unset","to":"personal"}`, `{"match":"unset","to":"personal"}`} {
		bad := audience(raw)
		if bad.Exit != 2 || bad.Error == nil || bad.Error.Code != "usage" {
			t.Fatalf("%s: %+v", raw, bad)
		}
	}
	if after := audience(`{}`); string(after.Output) != string(before.Output) {
		t.Fatalf("invalid call edited rules: %s vs %s", after.Output, before.Output)
	}
	if got := audience(`{"action":"unset","match":"personal"}`); got.Exit != 0 {
		t.Fatalf("unset: %+v", got.Error)
	}
}
