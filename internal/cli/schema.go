package cli

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
)

type schemaInvocation struct {
	Argv        []string `json:"argv"`
	Globals     []string `json:"globals"`
	Flag        string   `json:"flag"`
	Operands    string   `json:"operands"`
	OperandGaps bool     `json:"operand_gaps"`
}

type schemaDocument struct {
	Schema     string            `json:"schema"`
	Pbrain     string            `json:"pbrain"`
	Invocation schemaInvocation  `json:"invocation"`
	Exit       map[string]string `json:"exit"`
	Diagnostic json.RawMessage   `json:"diagnostic"`
	Errors     map[string]string `json:"errors"`
	Notices    map[string]string `json:"notices"`
	Tools      []Tool            `json:"tools"`
}

func describeCatalog(tools []Tool) schemaDocument {
	return schemaDocument{
		Schema: catalogVersion,
		Pbrain: binaryVersion(),
		Invocation: schemaInvocation{
			Argv:     []string{"pbrain", "<globals>", "--json", "<command>", "<flags>", "--", "<operands>"},
			Globals:  []string{"brain", "personal"},
			Flag:     "--NAME=VALUE; a true boolean is --NAME; a list is one flag per value",
			Operands: "the tool's operands, in order, each as one argument",
		},
		Exit:       map[string]string{"0": "success", "1": "failed, or a document with findings", "2": "wrong usage"},
		Diagnostic: DiagnosticSchema(),
		Errors:     errorDescriptions,
		Notices:    noticeDescriptions,
		Tools:      tools,
	}
}

type MCPTool struct {
	Name         string          `json:"name"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`
	Annotations  mcpAnnotations  `json:"annotations"`
}

type mcpAnnotations struct {
	ReadOnly    bool `json:"readOnlyHint"`
	Destructive bool `json:"destructiveHint"`
	Idempotent  bool `json:"idempotentHint"`
	OpenWorld   bool `json:"openWorldHint"`
}

// MCPTools projects the supplied catalog onto the MCP tools/list descriptors.
// A transport supplies its own protocol envelope.
func MCPTools(tools []Tool) []MCPTool {
	out := make([]MCPTool, 0, len(tools))
	for _, tool := range tools {
		description := tool.Description
		for _, rule := range tool.Rules {
			sentence := strings.TrimSuffix(rule.Text, ".")
			description += " " + sentence + "."
		}
		out = append(out, MCPTool{
			Name: tool.Name, Title: tool.Title, Description: description,
			InputSchema: tool.Input, OutputSchema: tool.Output,
			Annotations: mcpAnnotations{
				ReadOnly: tool.Hints.ReadOnly, Destructive: tool.Hints.Destructive,
				Idempotent: tool.Hints.Idempotent, OpenWorld: tool.Hints.OpenWorld,
			},
		})
	}
	return out
}

func schemaFormats() []string { return []string{"pbrain", "mcp"} }

func validateSchema(in *invocation) error {
	if in.has("format") && !slices.Contains(schemaFormats(), in.string("format")) {
		return usage("--format requires pbrain or mcp")
	}
	for _, name := range in.operands {
		if !commandNamed(name, Features{}).callable {
			return usage("unknown or uncallable command; run pbrain schema")
		}
	}
	return nil
}

func runSchema(in *invocation, features Features) outcome {
	tools := Catalog(features)
	tools = slices.DeleteFunc(tools, func(tool Tool) bool {
		return len(in.operands) != 0 && !slices.Contains(in.operands, tool.Command)
	})
	if in.boolean("inputs-only") {
		for i := range tools {
			tools[i].Output = nil
		}
	}
	var doc any
	if in.string("format") == "mcp" {
		doc = struct {
			Tools []MCPTool `json:"tools"`
		}{MCPTools(tools)}
	} else {
		doc = describeCatalog(tools)
	}
	data, err := encodeLine(doc)
	if err != nil || in.json {
		return outcome{data: data, err: err}
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		return outcome{err: err}
	}
	return outcome{data: pretty.Bytes()}
}
