package cli

import (
	"encoding/json"
	"strings"
)

func object(properties map[string]*jsonSchema, required string) *jsonSchema {
	return &jsonSchema{Type: "object", Properties: properties, Required: strings.Fields(required)}
}

func scalarFields(texts, counts, switches string) map[string]*jsonSchema {
	fields := make(map[string]*jsonSchema)
	for _, group := range []struct{ kind, keys string }{
		{"string", texts}, {"integer", counts}, {"boolean", switches},
	} {
		for _, key := range strings.Fields(group.keys) {
			fields[key] = &jsonSchema{Type: group.kind}
		}
	}
	if personal := fields["personal"]; personal != nil {
		personal.Description = "True for an item from the personal section."
	}
	return fields
}

func arrayOf(row *jsonSchema) *jsonSchema { return &jsonSchema{Type: "array", Items: row} }

func addIDLists(fields map[string]*jsonSchema, keys string) {
	for _, key := range strings.Fields(keys) {
		fields[key] = arrayOf(&jsonSchema{Type: "string"})
		fields[key+"_more"] = &jsonSchema{Type: "integer"}
	}
}

const itemHeader = "id kind status title created"
const bodyLayout = "body_line body_chars file_lines file_chars"

func itemFields(counts, switches string) map[string]*jsonSchema {
	return scalarFields(itemHeader+" author decided verified source", counts, "review_due linked personal "+switches)
}

func relatedRows() *jsonSchema {
	fields := scalarFields("link id kind standing created title", "", "personal replaced review_due linked")
	return arrayOf(object(fields, "link id kind standing created title"))
}

func pageSchema(rowKey string, row *jsonSchema, counts, required string) *jsonSchema {
	fields := scalarFields("", "more next "+counts, "")
	fields[rowKey] = arrayOf(row)
	return object(fields, required)
}

type outputVariant struct {
	name, description string
	schema            *jsonSchema
	selected          func(invocation) bool
}

func outputFor(cmd string) []outputVariant {
	switch cmd {
	case "search":
		fields := itemFields(bodyLayout+" occurrences snippet_offset snippet_line matched clauses", "semantic partial replaced")
		for _, key := range []string{"path", "section", "snippet"} {
			fields[key] = &jsonSchema{Type: "string"}
		}
		addIDLists(fields, "replacement")
		fields["related"] = relatedRows()
		fields["context"] = arrayOf(object(scalarFields("text", "line", ""), "line"))
		result := pageSchema("results", object(fields, itemHeader+" "+bodyLayout+" occurrences"), "replaced_matched outside_window", "")
		result.Properties["alone"] = arrayOf(object(scalarFields("clause", "count", ""), "clause count"))
		return singleOutput(result)
	case "get":
		fields := itemFields(bodyLayout+" offset line_from line_to more next unresolved", "")
		for _, key := range strings.Fields("path body stale_after origin fetched") {
			fields[key] = &jsonSchema{Type: "string"}
		}
		addIDLists(fields, "replaces replacement current relates")
		fields["tags"] = arrayOf(&jsonSchema{Type: "string"})
		fields["related"] = relatedRows()
		history := itemFields("", "")
		addIDLists(history, "replaces")
		heading := object(scalarFields("title", "offset chars file_line level", ""), "offset chars file_line level")
		outline := pageSchema("outline", heading, "sections body_chars", "id sections body_chars")
		outline.Properties["id"] = &jsonSchema{Type: "string"}
		return []outputVariant{
			{"history", "With history.", pageSchema("history", object(history, itemHeader), "", ""), hasFlag("history")},
			{"outline", "With outline.", outline, hasFlag("outline")},
			{"body", "Without history or outline.", object(fields, itemHeader+" "+bodyLayout+" offset"), nil},
		}
	case "list":
		fields := itemFields("", "replaced")
		addIDLists(fields, "replacement")
		return singleOutput(pageSchema("results", object(fields, itemHeader), "outside_window", ""))
	case "prime":
		fields := itemFields("", "")
		delete(fields, "linked") // prime includes only decisions and verified learnings
		return singleOutput(pageSchema("items", object(fields, itemHeader), "decisions learnings", "decisions learnings"))
	case "tags":
		row := object(scalarFields("tag description", "items", "undeclared"), "tag items")
		return singleOutput(pageSchema("tags", row, "", ""))
	case "check":
		row := object(scalarFields("severity code path message", "", ""), "severity code path message")
		return singleOutput(pageSchema("findings", row, "errors", "errors"))
	case "add", "verify", "import":
		return singleOutput(object(scalarFields("id section path", "", ""), "id section"))
	case "delete":
		shape := func(deleted bool) *jsonSchema {
			fields := scalarFields("id section path original", "", "")
			addIDLists(fields, "current_again dangling")
			required := "id section"
			if deleted {
				fields["deleted"] = &jsonSchema{Type: "boolean"}
				required += " deleted"
			}
			return object(fields, required)
		}
		return []outputVariant{
			{"deleted", "With yes.", shape(true), hasFlag("yes")},
			{"preview", "Without yes.", shape(false), nil},
		}
	case "audience":
		rule := object(scalarFields("match to", "", ""), "match to")
		return []outputVariant{
			{"edit", "With a set or unset operand.", object(scalarFields("match to", "", ""), "match"),
				func(in invocation) bool { return len(in.operands) > 0 }},
			{"listing", "Without operands.", pageSchema("rules", rule, "", ""), nil},
		}
	case "sync":
		build := scalarFields("section device", "items items_removed passages_embedded passages_reused", "")
		status := scalarFields("section", "items indexed", "stale unreadable")
		return []outputVariant{
			{"status", "With status.", syncOutput(true, status, "section items indexed"), hasFlag("status")},
			{"build", "Without status.", syncOutput(false, build, "section items items_removed passages_embedded passages_reused"), nil},
		}
	case "devices":
		row := object(scalarFields("id backend name persistent memory_kind memory", "threads total_bytes free_bytes", ""), "id backend")
		return singleOutput(object(map[string]*jsonSchema{"devices": arrayOf(row)}, "devices"))
	case "init":
		return singleOutput(object(scalarFields("path selector hint", "", ""), ""))
	default:
		return nil
	}
}

func hasFlag(name string) func(invocation) bool {
	return func(in invocation) bool { return in.boolean(name) }
}

func singleOutput(schema *jsonSchema) []outputVariant {
	return []outputVariant{{name: "default", schema: schema}}
}

func syncOutput(status bool, fields map[string]*jsonSchema, required string) *jsonSchema {
	root := scalarFields("model", "", "")
	root["sections"] = arrayOf(object(fields, required))
	keys := "model sections"
	if status {
		root["model_present"] = &jsonSchema{Type: "boolean"}
		keys = "sections"
	}
	return object(root, keys)
}

func publishedOutput(variants []outputVariant) *jsonSchema {
	if len(variants) == 1 {
		return variants[0].schema
	}
	schema := &jsonSchema{Type: "object"}
	for _, variant := range variants {
		variant.schema.Title = variant.name
		variant.schema.Description = variant.description
		schema.AnyOf = append(schema.AnyOf, variant.schema)
	}
	return schema
}

func selectedOutput(in invocation, variants []outputVariant) outputVariant {
	for _, variant := range variants {
		if variant.selected == nil || variant.selected(in) {
			return variant
		}
	}
	panic("output variants have no default")
}

var errorDescriptions = map[string]string{
	"usage":                "The invocation is invalid.",
	"no_brain":             "No brain could be found.",
	"invalid_config":       "A project or local setting is invalid.",
	"git_required":         "The operation requires Git.",
	"personal_exposed":     "The personal section could be exposed in Git.",
	"personal_unavailable": "The personal section cannot be reached.",
	"not_found":            "The item or input file does not exist in the call's view.",
	"ambiguous_id":         "More than one item has the requested id.",
	"invalid":              "A value, link or destination was refused.",
	"budget_too_small":     "The output limit cannot fit the required text.",
	"unsupported_edit":     "The existing item cannot be edited in place.",
	"conflict":             "A concurrent change prevented the operation.",
	"confirm_required":     "The delete preview requires a new call with yes.",
	"duplicate":            "An existing item conflicts with the proposed write.",
	"audience_required":    "The source needs an explicit shared or personal destination.",
	"unsupported_type":     "The input's format or encoding is unsupported.",
	"too_large":            "The input or produced file exceeds its size limit.",
	"no_content":           "The input has no usable content.",
	"fetch_blocked":        "The destination is outside the public address space.",
	"fetch_failed":         "The resource or model download failed.",
	"no_model":             "Semantic indexing is disabled in this brain.",
	"io":                   "The command could not complete an operation.",
}

var noticeDescriptions = map[string]string{
	"after_write":             "The write was published and a later operation failed.",
	"cuda_precision_fallback": "The selected CUDA device uses the fp32 writer.",
	"cpu_fallback":            "Sync is embedding on the CPU.",
	"embedding_device":        "Sync is embedding on the named accelerator.",
	"destination":             "The section where the write will land.",
	"git_history":             "A shared deletion leaves old versions in Git history.",
	"index_progress":          "Semantic index build progress.",
	"model_download":          "Model download progress.",
	"secret_suspected":        "Input text matches a possible secret pattern.",
	"semantic_not_ready":      "The semantic index is unavailable.",
	"semantic_partial":        "Some items lack semantic index coverage.",
	"skipped_files":           "Some broken item files were skipped.",
	"undeclared_tag":          "The item uses a tag without a declaration.",
}

// DiagnosticSchema describes each structured stderr line.
func DiagnosticSchema() json.RawMessage {
	return schemaJSON(diagnosticSchema())
}

func diagnosticSchema() *jsonSchema {
	closed := false
	line := func(key string) *jsonSchema {
		fields := scalarFields("code message", "", "")
		entry := object(fields, "code message")
		entry.AdditionalProperties = &closed
		envelope := object(map[string]*jsonSchema{key: entry}, key)
		envelope.AdditionalProperties = &closed
		return envelope
	}
	return &jsonSchema{Type: "object", AnyOf: []*jsonSchema{line("error"), line("notice")}}
}

func toolDiagnostics(cmd string) (errors, notices []string) {
	errors = []string{"usage", "io"}
	notices = []string{}
	if cmd == "devices" {
		return errors, notices
	}
	if cmd == "init" {
		return append(errors, "invalid", "invalid_config", "personal_exposed", "no_brain", "git_required"), notices
	}
	errors = append(errors, "no_brain", "invalid_config", "git_required", "personal_exposed", "personal_unavailable")
	switch cmd {
	case "search", "get", "list", "prime", "tags", "check", "audience":
		errors = append(errors, "budget_too_small")
	}
	switch cmd {
	case "search", "get", "list", "prime", "tags", "sync", "add", "verify", "import", "delete":
		notices = append(notices, "skipped_files")
	}
	switch cmd {
	case "get", "add", "verify", "delete", "import":
		errors = append(errors, "not_found", "ambiguous_id", "conflict")
	case "audience":
		errors = append(errors, "invalid", "conflict")
	}
	switch cmd {
	case "audience", "add", "verify", "import", "delete":
		notices = append(notices, "after_write")
	}
	switch cmd {
	case "search":
		notices = append(notices, "semantic_not_ready", "semantic_partial")
	case "add", "import":
		errors = append(errors, "invalid", "duplicate", "too_large", "audience_required")
		notices = append(notices, "destination", "secret_suspected", "undeclared_tag")
		if cmd == "import" {
			errors = append(errors, "unsupported_type", "no_content", "fetch_blocked", "fetch_failed")
		}
	case "verify":
		errors = append(errors, "invalid", "unsupported_edit", "too_large")
		notices = append(notices, "secret_suspected")
	case "delete":
		errors = append(errors, "invalid", "confirm_required")
		notices = append(notices, "git_history")
	case "sync":
		errors = append(errors, "no_model", "fetch_failed", "too_large")
		notices = append(notices, "embedding_device", "cpu_fallback", "cuda_precision_fallback", "index_progress", "model_download")
	}
	return errors, notices
}

// ToolOutputForArgs selects the JSON output variant for a parsed tool invocation.
// On a parser error it still returns the tool's diagnostic codes, with no output schema.
func ToolOutputForArgs(args []string, features Features) (tool Tool, output json.RawMessage, ok bool) {
	in, err := parseOptions(args, features)
	if !in.json || !in.cmd.callable || (in.flags != nil && (in.boolean("help") || in.boolean("version"))) {
		return Tool{}, nil, false
	}
	tool.Command = in.cmd.name
	tool.Errors, tool.Notices = toolDiagnostics(in.cmd.name)
	tool.DocumentOnExit1 = in.cmd.name == "check" || in.cmd.name == "delete"
	if err == nil {
		variants := outputFor(in.cmd.name)
		output = schemaJSON(selectedOutput(in, variants).schema)
		tool.Output = schemaJSON(publishedOutput(variants))
	}
	return tool, output, true
}
