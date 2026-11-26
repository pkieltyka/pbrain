package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/cli/schematest"
)

func TestSchemaGolden(t *testing.T) {
	old := version
	version = "v0.0.0-test"
	t.Cleanup(func() { version = old })
	for _, format := range schemaFormats() {
		t.Run(format, func(t *testing.T) {
			env := Env{Files: noFiles{}}
			args := []string{"schema", "--format=" + format}
			code, pretty, stderr := invoke(t, env, Release, args...)
			if code != 0 || stderr != "" || !strings.HasPrefix(pretty, "{\n  ") || !strings.HasSuffix(pretty, "\n") {
				t.Fatalf("%s: exit %d, stderr %s", format, code, stderr)
			}
			var compact bytes.Buffer
			if err := json.Compact(&compact, []byte(pretty)); err != nil {
				t.Fatal(err)
			}
			code, oneline, stderr := invoke(t, env, Release, append(args, "--json")...)
			if code != 0 || stderr != "" || oneline != compact.String()+"\n" {
				t.Fatalf("compact %s output differs from pretty JSON: %d %s", format, code, stderr)
			}
			assertSchemaGolden(t, format+".golden", []byte(pretty))
		})
	}
}

func TestSchemaSelection(t *testing.T) {
	env := Env{Files: noFiles{}}
	args := []string{"--brain=/no/such/brain", "--personal", "--json", "schema",
		"get", "search", "get", "--inputs-only"}
	for _, features := range []Features{{}, Release} {
		code, data, stderr := invoke(t, env, features, args...)
		var doc schemaDocument
		oneJSON(t, data, &doc)
		if code != 0 || stderr != "" || doc.Schema != catalogVersion || doc.Pbrain != binaryVersion() ||
			doc.Invocation.OperandGaps || len(doc.Diagnostic) == 0 || len(doc.Errors) == 0 || len(doc.Notices) == 0 {
			t.Fatalf("selected schema: %d %s %s", code, data, stderr)
		}
		if len(doc.Tools) != 2 || doc.Tools[0].Command != "search" || doc.Tools[1].Command != "get" {
			t.Fatalf("expected one descriptor per selected command in catalog order: %s", data)
		}
		for _, tool := range doc.Tools {
			if len(tool.Input) == 0 || len(tool.Output) != 0 {
				t.Fatalf("inputs-only contract for %s: %s", tool.Command, data)
			}
		}
		var input jsonSchema
		if err := json.Unmarshal(doc.Tools[0].Input, &input); err != nil {
			t.Fatal(err)
		}
		if (input.Properties["bm25"] != nil) != features.BM25 {
			t.Fatalf("bm25 property for features %+v: %s", features, doc.Tools[0].Input)
		}
	}
	code, data, stderr := invoke(t, env, Release, append(args, "--format=mcp")...)
	var mcp struct{ Tools []MCPTool }
	oneJSON(t, data, &mcp)
	if code != 0 || stderr != "" || len(mcp.Tools) != 2 || len(mcp.Tools[0].OutputSchema) != 0 || len(mcp.Tools[1].OutputSchema) != 0 {
		t.Fatalf("MCP inputs-only selection: %d %s %s", code, data, stderr)
	}
	for _, tc := range []struct {
		args []string
		msg  string
	}{
		{[]string{"nonesuch"}, "unknown or uncallable command; run pbrain schema"},
		{[]string{"skill"}, "unknown or uncallable command; run pbrain schema"},
		{[]string{"schema"}, "unknown or uncallable command; run pbrain schema"},
		{[]string{"--format=xml"}, "--format requires pbrain or mcp"},
		{[]string{"--format="}, "--format requires pbrain or mcp"},
	} {
		code, data, stderr := invoke(t, env, Release, append([]string{"schema"}, tc.args...)...)
		if code != 2 || data != "" || stderr != "usage: "+tc.msg+"\n" {
			t.Errorf("%v = %d %q %q", tc.args, code, data, stderr)
		}
	}
	code, data, stderr = invoke(t, env, Release, "schema", "--help")
	if code != 0 || stderr != "" || strings.Contains(data, "--brain") || strings.Contains(data, "--personal") ||
		!strings.Contains(data, "--inputs-only") {
		t.Fatalf("schema help: %d %q %q", code, data, stderr)
	}
}

func TestSchemasCompile(t *testing.T) {
	env := Env{Files: noFiles{}}
	code, data, stderr := invoke(t, env, Release, "schema", "--json")
	var published schemaDocument
	oneJSON(t, data, &published)
	if code != 0 || stderr != "" || len(published.Tools) == 0 {
		t.Fatalf("pbrain document: %d %q", code, stderr)
	}
	assertSchemaCompiles(t, "diagnostic", published.Diagnostic)
	for _, tool := range published.Tools {
		assertSchemaCompiles(t, tool.Name+" input", tool.Input)
		assertSchemaCompiles(t, tool.Name+" output", tool.Output)
	}
	code, data, stderr = invoke(t, env, Release, "schema", "--format=mcp", "--json")
	var mcp struct{ Tools []MCPTool }
	oneJSON(t, data, &mcp)
	if code != 0 || stderr != "" || len(mcp.Tools) != len(published.Tools) {
		t.Fatalf("MCP document: %d %q", code, stderr)
	}
	for i, tool := range mcp.Tools {
		assertSchemaCompiles(t, tool.Name+" MCP input", tool.InputSchema)
		assertSchemaCompiles(t, tool.Name+" MCP output", tool.OutputSchema)
		if p := published.Tools[i]; tool.Name != p.Name ||
			!bytes.Equal(tool.InputSchema, p.Input) || !bytes.Equal(tool.OutputSchema, p.Output) {
			t.Errorf("MCP tool %s has a different contract from %s", tool.Name, p.Name)
		}
	}
}

func TestCatalogOutputGuards(t *testing.T) {
	contract, ok := JSONOutputContract([]string{"get", "item", "--history", "--json"}, Release)
	if !ok {
		t.Fatal("get history was not selected")
	}
	for _, tc := range []struct{ out, stderr string }{
		{`{"body":"body belongs to a different variant"}` + "\n", ""},
		{`{"history":[{"id":"a","kind":"note","status":"provisional","title":"A","created":"unknown","unexpected":true}]}` + "\n", ""},
		{`{"more":"not a count"}` + "\n", ""},
		{"{}\n", `{"notice":{"code":"undeclared","message":"new notice"}}`},
		{"{}\n", `{"error":{"code":"usage","message":"x","extra":true}}`},
	} {
		if err := schematest.Check(contract.Variant, DiagnosticSchema(), contract.Errors, contract.Notices, contract.DocumentOnExit1, 0, tc.out, tc.stderr); err == nil {
			t.Errorf("contract accepted stdout %q, stderr %q", tc.out, tc.stderr)
		}
	}
}

func assertSchemaCompiles(t *testing.T, name string, data []byte) {
	t.Helper()
	var shape jsonSchema
	if err := json.Unmarshal(data, &shape); err != nil || shape.Type != "object" {
		t.Fatalf("%s does not have an object schema: %s", name, data)
	}
	if _, err := schematest.Compile(data, false); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

// Each release that ships schema freezes its pbrain-format catalog alongside the
// mutable goldens. Every frozen catalog in the current compatibility line is tested.
func TestSchemaOnlyGrows(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "schema", "release-v*.json"))
	if err != nil {
		t.Fatal(err)
	}
	current := Catalog(Release)
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			var frozen schemaDocument
			if err := json.Unmarshal(getBytes(t, path), &frozen); err != nil {
				t.Fatal(err)
			}
			if frozen.Schema != catalogVersion || len(frozen.Tools) == 0 {
				t.Fatalf("invalid release catalog %s", path)
			}
			if err := catalogGrowth(frozen.Tools, current); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func catalogGrowth(old, current []Tool) error {
	for _, from := range old {
		i := slices.IndexFunc(current, func(tool Tool) bool { return tool.Name == from.Name })
		if i < 0 {
			return fmt.Errorf("removed tool %s", from.Name)
		}
		to := current[i]
		for _, pair := range []struct {
			kind      string
			old, next json.RawMessage
		}{
			{"input", from.Input, to.Input},
			{"output", from.Output, to.Output},
		} {
			var a, b jsonSchema
			if err := json.Unmarshal(pair.old, &a); err != nil {
				return fmt.Errorf("%s frozen %s schema: %w", from.Name, pair.kind, err)
			}
			if err := json.Unmarshal(pair.next, &b); err != nil {
				return fmt.Errorf("%s current %s schema: %w", from.Name, pair.kind, err)
			}
			if err := schemaGrowth(&a, &b, from.Name+" "+pair.kind, pair.kind == "input"); err != nil {
				return err
			}
		}
	}
	return nil
}

func schemaGrowth(old, current *jsonSchema, path string, input bool) error {
	if current == nil {
		return fmt.Errorf("removed %s", path)
	}
	if old.Type != current.Type {
		return fmt.Errorf("changed type of %s from %s to %s", path, old.Type, current.Type)
	}
	if !input && len(old.AnyOf) == 0 && len(current.AnyOf) != 0 {
		// A command that previously produced a single variant can add one
		// without losing the old document.
		for _, branch := range current.AnyOf {
			if schemaGrowth(old, branch, path+"."+branch.Title, false) == nil {
				return nil
			}
		}
		return fmt.Errorf("no variant preserves the previous document at %s", path)
	}
	if len(current.Enum) != 0 {
		if len(old.Enum) == 0 {
			return fmt.Errorf("added a restrictive enum to %s", path)
		}
		for _, value := range old.Enum {
			if !slices.Contains(current.Enum, value) {
				return fmt.Errorf("removed enum value %q from %s", value, path)
			}
		}
	}
	if input {
		for _, name := range current.Required {
			if !slices.Contains(old.Required, name) {
				return fmt.Errorf("new required input %s.%s", path, name)
			}
		}
	}
	for name, previous := range old.Properties {
		if err := schemaGrowth(previous, current.Properties[name], path+"."+name, input); err != nil {
			return err
		}
	}
	if old.Items != nil {
		if err := schemaGrowth(old.Items, current.Items, path+"[]", input); err != nil {
			return err
		}
	}
	for _, previous := range old.AnyOf {
		i := slices.IndexFunc(current.AnyOf, func(branch *jsonSchema) bool { return branch.Title == previous.Title })
		if i < 0 {
			return fmt.Errorf("removed variant %s.%s", path, previous.Title)
		}
		if err := schemaGrowth(previous, current.AnyOf[i], path+"."+previous.Title, input); err != nil {
			return err
		}
	}
	return nil
}

func TestCatalogGrowthChecks(t *testing.T) {
	old := []Tool{{
		Name:   "pbrain_example",
		Input:  json.RawMessage(`{"type":"object","properties":{"choice":{"type":"array","items":{"type":"string","enum":["a","b"]}}}}`),
		Output: json.RawMessage(`{"type":"object","anyOf":[{"title":"base","type":"object","properties":{"id":{"type":"string"}}}]}`),
	}}
	grows := old[0]
	grows.Input = json.RawMessage(`{"type":"object","properties":{"choice":{"type":"array","items":{"type":"string","enum":["b","a","c"]}},"other":{"type":"boolean"}}}`)
	grows.Output = json.RawMessage(`{"type":"object","anyOf":[{"title":"new","type":"object"},{"title":"base","type":"object","properties":{"id":{"type":"string"},"next":{"type":"integer"}}}]}`)
	if err := catalogGrowth(old, []Tool{grows, {Name: "pbrain_new"}}); err != nil {
		t.Fatalf("additive catalog rejected: %v", err)
	}
	single := old[0]
	single.Output = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}}}`)
	if err := catalogGrowth([]Tool{single}, []Tool{grows}); err != nil {
		t.Fatalf("preserved document in new union rejected: %v", err)
	}
	for _, tc := range []struct {
		name string
		add  string
		from string
		to   string
	}{
		{"removed tool", "name", "pbrain_example", "pbrain_different"},
		{"removed property", "input", "choice", "renamed"},
		{"removed enum", "input", `"b","a","c"`, `"a","c"`},
		{"required input", "input", `"properties":`, `"required":["other"],"properties":`},
		{"retyped nested input", "input", `"type":"string","enum"`, `"type":"integer","enum"`},
		{"removed variant", "output", `"title":"base"`, `"title":"gone"`},
		{"retyped output", "output", `"id":{"type":"string"}`, `"id":{"type":"integer"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := grows
			switch tc.add {
			case "name":
				changed.Name = tc.to
			case "input":
				changed.Input = bytes.Replace(changed.Input, []byte(tc.from), []byte(tc.to), 1)
			case "output":
				changed.Output = bytes.Replace(changed.Output, []byte(tc.from), []byte(tc.to), 1)
			}
			if err := catalogGrowth(old, []Tool{changed}); err == nil {
				t.Fatal("breaking change accepted")
			}
		})
	}
}
