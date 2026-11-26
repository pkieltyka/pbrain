package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

const catalogVersion = "pbrain-tools/1"

type Tool struct {
	Name            string          `json:"name"`
	Command         string          `json:"command"`
	Title           string          `json:"-"`
	Description     string          `json:"description"`
	Operands        []string        `json:"operands"`
	Input           json.RawMessage `json:"input"`
	Output          json.RawMessage `json:"output,omitempty"`
	Rules           []ToolRule      `json:"rules,omitempty"`
	DocumentOnExit1 bool            `json:"document_on_exit_1"`
	Errors          []string        `json:"errors"`
	Notices         []string        `json:"notices"`
	Hints           toolHints       `json:"hints"`
}

type ToolRule struct {
	Properties []string `json:"properties"`
	Text       string   `json:"text"`
}

type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Result struct {
	Output  json.RawMessage
	Error   *Diagnostic
	Notices []Diagnostic
	Exit    int
}

type jsonSchema struct {
	Type                 string                 `json:"type,omitempty"`
	Title                string                 `json:"title,omitempty"`
	Description          string                 `json:"description,omitempty"`
	Properties           map[string]*jsonSchema `json:"properties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	Items                *jsonSchema            `json:"items,omitempty"`
	Enum                 []string               `json:"enum,omitempty"`
	Minimum              *int                   `json:"minimum,omitempty"`
	Maximum              *int                   `json:"maximum,omitempty"`
	Default              any                    `json:"default,omitempty"`
	AdditionalProperties *bool                  `json:"additionalProperties,omitempty"`
	AnyOf                []*jsonSchema          `json:"anyOf,omitempty"`
}

func schemaJSON(schema *jsonSchema) json.RawMessage {
	data, err := json.Marshal(schema)
	if err != nil {
		panic(err) // schemas contain only the package's fixed data
	}
	return data
}

func toolGlobal(cmd command, name string) bool {
	switch name {
	case "brain":
		return cmd.name != "devices"
	case "personal":
		return cmd.name != "devices" && cmd.name != "init"
	default:
		return false
	}
}

func toolOptions(cmd command) []option {
	options := make([]option, 0, len(cmd.options)+2)
	for _, o := range globals {
		if toolGlobal(cmd, o.name) {
			options = append(options, o)
		}
	}
	for _, o := range cmd.options {
		if o.name != "color" {
			options = append(options, o)
		}
	}
	return options
}

func optionSchema(cmd command, o option) *jsonSchema {
	value := &jsonSchema{Type: "string", Description: o.help}
	if o.kind == boolean {
		value.Type, value.Default = "boolean", false
		return value
	}
	switch o.form {
	case choice:
		value.Enum = o.choices()
	case integer:
		value.Type = "integer"
		modePage := cmd.name == "get" && (o.name == "limit" || o.name == "max-chars")
		lo, hi, def := bounds(cmd.name, o.name, modePage, false)
		value.Minimum = &lo
		if hi != 0 {
			value.Maximum = &hi
		}
		if modePage {
			value.Description += pageHelpDefault(cmd.name, o.name)
		} else if def >= lo && (hi == 0 || def <= hi) {
			value.Default = def
		}
		switch {
		case (cmd.name == "search" || cmd.name == "list") && (o.name == "limit" || o.name == "max-chars"),
			(cmd.name == "get" || cmd.name == "prime") && o.name == "max-chars":
			value.Description += ". Project configuration can change the default."
		}
	}
	if o.kind == repeated {
		item := &jsonSchema{Type: value.Type, Enum: value.Enum}
		return &jsonSchema{Type: "array", Description: o.help, Items: item}
	}
	return value
}

func inputSchema(cmd command) *jsonSchema {
	closed := false
	schema := &jsonSchema{Type: "object", Properties: map[string]*jsonSchema{}, AdditionalProperties: &closed}
	for _, o := range toolOptions(cmd) {
		schema.Properties[o.name] = optionSchema(cmd, o)
		if o.required {
			schema.Required = append(schema.Required, o.name)
		}
	}
	for _, arg := range cmd.args {
		value := &jsonSchema{Type: "string", Description: arg.help}
		if arg.form == choice {
			value.Enum = arg.choices()
		}
		schema.Properties[arg.name] = value
		if arg.required {
			schema.Required = append(schema.Required, arg.name)
		}
	}
	return schema
}

// Catalog returns the callable commands for this feature set in command order.
func Catalog(features Features) []Tool {
	var tools []Tool
	for _, entry := range commands {
		if !entry.callable {
			continue
		}
		cmd := commandNamed(entry.name, features)
		tool := Tool{
			Name: "pbrain_" + cmd.name, Command: cmd.name, Title: cmd.summary, Description: cmd.about,
			Operands: make([]string, 0, len(cmd.args)), Input: schemaJSON(inputSchema(cmd)), Hints: cmd.hints,
		}
		for _, arg := range cmd.args {
			tool.Operands = append(tool.Operands, arg.name)
		}
		if variants := outputFor(cmd.name); len(variants) != 0 {
			tool.Output = schemaJSON(publishedOutput(variants))
		}
		tool.Errors, tool.Notices = toolDiagnostics(cmd.name)
		tool.DocumentOnExit1 = documentOnExit1(cmd.name)
		for _, rule := range rules {
			if rule.applies(cmd.name) {
				tool.Rules = append(tool.Rules, ToolRule{Properties: append([]string(nil), rule.properties...), Text: rule.message})
			}
		}
		tools = append(tools, tool)
	}
	return tools
}

// Bind translates named JSON arguments into the corresponding CLI arguments.
// The CLI remains responsible for value ranges and command semantics.
func Bind(tool string, args json.RawMessage, features Features) ([]string, error) {
	name, ok := strings.CutPrefix(tool, "pbrain_")
	cmd := commandNamed(name, features)
	if !ok || !cmd.callable {
		return nil, usage(fmt.Sprintf("unknown tool %q", tool))
	}
	options := toolOptions(cmd)
	properties := make(map[string]option, len(options)+len(cmd.args))
	for _, o := range options {
		properties[o.name] = o
	}
	for _, arg := range cmd.args {
		properties[arg.name] = textOption(arg.name, "", arg.help)
	}
	values, err := bindValues(tool, args, properties)
	if err != nil {
		return nil, err
	}

	var argv, local, operands []string
	for _, o := range options {
		dest := &local
		if toolGlobal(cmd, o.name) {
			dest = &argv
		}
		for _, value := range values[o.name] {
			if o.kind == boolean {
				*dest = append(*dest, "--"+o.name)
			} else {
				*dest = append(*dest, "--"+o.name+"="+value)
			}
		}
	}
	for i, arg := range cmd.args {
		value, present := values[arg.name]
		switch {
		case arg.required && !present:
			return nil, usage(fmt.Sprintf("%s requires argument %q", tool, arg.name))
		case present && len(operands) != i:
			return nil, usage(fmt.Sprintf("%s argument %q has an earlier missing operand", tool, arg.name))
		case present:
			operands = append(operands, value[0])
		}
	}
	argv = append(argv, "--json", cmd.name)
	argv = append(argv, local...)
	argv = append(argv, "--")
	return append(argv, operands...), nil
}

func bindValues(tool string, raw json.RawMessage, properties map[string]option) (map[string][]string, error) {
	values := make(map[string][]string)
	if len(raw) == 0 {
		return values, nil // MCP permits an omitted arguments object.
	}
	badObject := func() error { return usage(tool + " arguments must be a JSON object") }
	if !utf8.Valid(raw) {
		return nil, badObject()
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return nil, badObject()
	}
	for decoder.More() {
		key, err := decoder.Token()
		name, ok := key.(string)
		if err != nil || !ok {
			return nil, badObject()
		}
		property, known := properties[name]
		if !known {
			return nil, usage(fmt.Sprintf("%s has no argument %q", tool, name))
		}
		if _, duplicate := values[name]; duplicate {
			return nil, usage(fmt.Sprintf("%s repeats argument %q", tool, name))
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, badObject()
		}
		args, valid := argumentValues(property, value)
		if !valid {
			return nil, usage(fmt.Sprintf("%s argument %q has an invalid value type", tool, name))
		}
		values[name] = args
	}
	if last, err := decoder.Token(); err != nil || last != json.Delim('}') {
		return nil, badObject()
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, badObject()
	}
	return values, nil
}

func argumentValues(o option, raw json.RawMessage) ([]string, bool) {
	if bytes.Equal(raw, []byte("null")) {
		return nil, false
	}
	switch o.kind {
	case boolean:
		switch string(raw) {
		case "true":
			return []string{""}, true
		case "false":
			return nil, true
		}
	case repeated:
		var list []json.RawMessage
		if json.Unmarshal(raw, &list) == nil {
			out := make([]string, 0, len(list))
			for _, entry := range list {
				value, ok := argumentString(entry)
				if !ok {
					return nil, false
				}
				out = append(out, value)
			}
			return out, true
		}
	default:
		if o.form == integer {
			value, ok := argumentInteger(string(raw))
			return []string{value}, ok
		}
		value, ok := argumentString(raw)
		return []string{value}, ok
	}
	return nil, false
}

func argumentString(raw json.RawMessage) (string, bool) {
	var value string
	ok := len(raw) > 0 && raw[0] == '"' && json.Unmarshal(raw, &value) == nil
	return value, ok && !strings.ContainsRune(value, 0)
}

// JSON Schema integers include numbers such as 2.0 and 2e0. Normalize
// exactly without a float or a large expansion for enormous exponents.
func argumentInteger(raw string) (string, bool) {
	sign := ""
	number, negative := strings.CutPrefix(raw, "-")
	if negative {
		sign = "-"
	}
	if len(number) == 0 || number[0] < '0' || number[0] > '9' {
		return "", false
	}
	if !strings.ContainsAny(number, ".eE") {
		return raw, true
	}
	exponent := 0
	if i := strings.IndexAny(number, "eE"); i >= 0 {
		exp := number[i+1:]
		number = number[:i]
		limit := len(raw) + 20
		n, err := strconv.Atoi(exp)
		if err != nil {
			n = limit
			if strings.HasPrefix(exp, "-") {
				n = -limit
			}
		}
		exponent = max(-limit, min(limit, n))
	}
	whole, fraction, _ := strings.Cut(number, ".")
	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		return "0", true
	}
	significant := strings.TrimRight(digits, "0")
	zeros := exponent - len(fraction) + len(digits) - len(significant)
	if zeros < 0 {
		return "", false
	}
	if len(significant)+zeros > 19 {
		return raw, true // outside any supported int range; let the CLI report it
	}
	return sign + significant + strings.Repeat("0", zeros), true
}

type callDiagnostics struct {
	result   *Result
	onNotice func(Diagnostic)
}

func (w callDiagnostics) Write(data []byte) (int, error) {
	// emitDiagnostic writes one complete JSON line at a time.
	var line struct {
		Notice *Diagnostic `json:"notice"`
		Error  *Diagnostic `json:"error"`
	}
	if json.Unmarshal(data, &line) == nil {
		switch {
		case line.Notice != nil && line.Error == nil:
			w.result.Notices = append(w.result.Notices, *line.Notice)
			if w.onNotice != nil {
				w.onNotice(*line.Notice)
			}
		case line.Error != nil && line.Notice == nil:
			w.result.Error = line.Error
		}
	}
	return len(data), nil
}

// Call runs the same command as the bound argv with isolated streams.
func Call(ctx context.Context, env Env, tool string, args json.RawMessage, features Features, onNotice func(Diagnostic)) Result {
	var result Result
	diagnostics := callDiagnostics{&result, onNotice}
	argv, err := Bind(tool, args, features)
	if err != nil {
		result.Exit = emitError(diagnostics, true, err)
		return result
	}
	var stdout bytes.Buffer
	env.Args, env.Stdin, env.Stdout, env.Stderr = argv, nil, &stdout, diagnostics
	result.Exit = RunWith(ctx, env, features)
	result.Output = bytes.TrimSpace(stdout.Bytes())
	return result
}
