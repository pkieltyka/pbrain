// Package schematest checks CLI JSON contracts from the unit and process test suites.
package schematest

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type cacheKey struct {
	schema string
	strict bool
}

var compiled sync.Map

func Compile(schema []byte, strict bool) (*jsonschema.Schema, error) {
	key := cacheKey{string(schema), strict}
	if stored, ok := compiled.Load(key); ok {
		return stored.(*jsonschema.Schema), nil
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
	if err != nil {
		return nil, err
	}
	if strict {
		closeObjects(doc.(map[string]any))
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	const name = "urn:pbrain:test-schema"
	if err := compiler.AddResource(name, doc); err != nil {
		return nil, err
	}
	s, err := compiler.Compile(name)
	if err == nil {
		compiled.Store(key, s)
	}
	return s, err
}

// The strict schema is the variant selected by the command line.
func closeObjects(node map[string]any) {
	if node["type"] == "object" {
		node["additionalProperties"] = false
	}
	if properties, ok := node["properties"].(map[string]any); ok {
		for _, property := range properties {
			closeObjects(property.(map[string]any))
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		closeObjects(items)
	}
}

func Validate(schema []byte, strict bool, document []byte) error {
	compiled, err := Compile(schema, strict)
	if err != nil {
		return err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(document))
	if err != nil {
		return err
	}
	return compiled.Validate(doc)
}

func Check(variant, diagnostic []byte, errorCodes, noticeCodes []string, documentOnExit1 bool, exit int, stdout, stderr string) error {
	if stdout != "" {
		if exit != 0 && !(exit == 1 && documentOnExit1) {
			return fmt.Errorf("unexpected document on exit %d", exit)
		}
		if !strings.HasSuffix(stdout, "\n") || strings.Count(stdout, "\n") != 1 {
			return fmt.Errorf("stdout must be one JSON line")
		}
		if err := Validate(variant, true, []byte(stdout)); err != nil {
			return fmt.Errorf("stdout: %w", err)
		}
	}
	for line := range strings.SplitSeq(stderr, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue // native diagnostics can share stderr
		}
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(line))
		if err != nil {
			continue // native diagnostics are not guaranteed to be JSON
		}
		shape, err := Compile(diagnostic, false)
		if err != nil {
			return err
		}
		if err := shape.Validate(doc); err != nil {
			return fmt.Errorf("stderr: %w", err)
		}
		for kind, value := range doc.(map[string]any) {
			code := value.(map[string]any)["code"].(string)
			allowed := noticeCodes
			if kind == "error" {
				allowed = errorCodes
			}
			if !slices.Contains(allowed, code) {
				return fmt.Errorf("undeclared %s code %q", kind, code)
			}
		}
	}
	return nil
}
