package cli

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/query"
	"github.com/pkieltyka/pbrain/internal/render"
	"github.com/pkieltyka/pbrain/internal/search"
	"github.com/spf13/pflag"
)

type valueKind uint8

const (
	boolean valueKind = iota
	text
	repeated
)

type option struct {
	name, value, help string
	kind              valueKind
}

var globals = []option{
	{"brain", "DIR", "use the brain at DIR", text},
	{"personal", "", "include personal items for this command", boolean},
	{"json", "", "emit compact JSON", boolean},
	{"help", "", "show help", boolean},
	{"version", "", "show the program version", boolean},
}

var pages = []option{
	{"limit", "N", "maximum number of results", text},
	{"offset", "N", "start at offset N", text},
	{"max-chars", "N", "maximum code points in the response", text},
}

var tagFilter = option{"tag", "TAG", "filter by tag (repeatable)", repeated}

var filters = []option{
	{"kind", "KIND", "include items of a kind (repeatable)", repeated},
	tagFilter,
	{"status", "STATUS", "match the recorded status", text},
	{"since", "WHEN", "include items created at or after WHEN", text},
	{"until", "WHEN", "include items created through WHEN", text},
	{"all", "", "include replaced items", boolean},
}

var colour = option{"color", "", "colour supported text output", boolean}
var bm25 = option{"bm25", "", "include results matching part of the query", boolean}

var writeOptions = []option{
	{"source", "TEXT", "name the source", text},
	{"tag", "TAG", "add a tag to the item (repeatable)", repeated},
	{"replaces", "ID", "record a replaced item (repeatable)", repeated},
	{"stale-after", "WHEN", "set the review date", text},
	{"author", "ACTOR", "set the item author", text},
	{"shared", "", "write to the shared section", boolean},
}

var getOptions = slices.Concat([]option{{"limit", "N", "maximum entries with --history or --outline", text}}, pages[1:], []option{
	colour, {"history", "", "show replacement lineage", boolean}, {"outline", "", "show body sections", boolean},
})

type command struct {
	name, operands, summary string
	options                 []option
}

var commands = []command{
	{"init", "", "create or complete a brain", []option{{"path", "DIR", "create the brain at DIR (default docs/brain when no brain is selected)", text}}},
	{"skill", "[list|TOPIC]", "print the skill or a topic", nil},
	{"search", "QUERY…", "find matching items", slices.Concat(filters, pages, []option{
		colour, {"context", "N", "include 1 to 3 nearby lines on each side", text},
		{"semantic", "", "include results found by meaning", boolean},
	})},
	{"list", "", "list items", slices.Concat(filters, pages)},
	{"get", "ID", "read an item, its history or its outline", getOptions},
	{"prime", "", "show current decisions and verified learnings", slices.Concat([]option{tagFilter}, pages[1:])},
	{"tags", "", "list tags", pages},
	{"check", "", "report findings about the brain", pages},
	{"add", "", "add a first-party item", slices.Concat([]option{
		{"title", "TEXT", "set the item title (required)", text},
		{"kind", "KIND", "set the item kind (default " + item.Note.String() + ")", text},
		{"body", "TEXT", "use TEXT as the body", text},
		{"body-file", "FILE", "read the body from FILE", text},
		{"decided", "ACTOR", "name the decision maker", text},
		{"verified", "ACTOR", "name the verifier", text},
		{"relates", "ID", "record a related item (repeatable)", repeated},
	}, writeOptions)},
	{"import", "INPUT", "copy a source or record a link", slices.Concat([]option{
		{"title", "TEXT", "override the title", text},
		{"link-only", "", "record a URL without fetching it", boolean},
		{"summary", "TEXT", "set the body of a linked source", text},
	}, writeOptions)},
	{"verify", "ID", "verify a learning or other first-party item", []option{{"by", "ACTOR", "name the verifier", text}}},
	{"delete", "ID", "preview or delete an item", []option{{"yes", "", "authorize deletion", boolean}}},
	{"audience", "[set MATCH shared|personal | unset MATCH]", "list or edit audience rules", pages},
	{"sync", "", "build or update the semantic index, or report its state", []option{{"status", "", "report index state without building", boolean}}},
}

func commandNamed(name string, features Features) command {
	if name == "index" {
		name = "sync"
	}
	for _, cmd := range commands {
		if cmd.name == name {
			if cmd.name == "search" && features.BM25 {
				cmd.options = slices.Concat(cmd.options, []option{bm25})
			}
			return cmd
		}
	}
	return command{}
}

type invocation struct {
	cmd        command
	indexAlias bool

	flags    *pflag.FlagSet
	operands []string
	json     bool
	format   render.Format
	now      time.Time
	zone     *time.Location
	filter   search.Filters
	clauses  []query.Clause
	query    string
	context  int
	page     pageOptions
	view     brain.View
}

func (in *invocation) has(name string) bool { return in.flags.Changed(name) }
func (in *invocation) string(name string) string {
	v, _ := in.flags.GetString(name)
	return v
}
func (in *invocation) boolean(name string) bool {
	v, _ := in.flags.GetBool(name)
	return v
}
func (in *invocation) all(name string) []string {
	v, _ := in.flags.GetStringArray(name)
	return v
}

type pageOptions struct{ limit, offset, maxChars int }

func optionNamed(options []option, name string) (option, bool) {
	for _, o := range options {
		if o.name == name {
			return o, true
		}
	}
	return option{}, false
}

func splitOption(arg string) (name, value string, equals, ok bool) {
	tail, ok := strings.CutPrefix(arg, "--")
	if !ok || tail == "" {
		return "", "", false, false
	}
	name, value, equals = strings.Cut(tail, "=")
	return name, value, equals, true
}

// commandAt finds the first command operand. Local options cannot precede it.
// Recognized value arities let diagnostics still find later global flags after bad input.
func commandAt(args []string) (at int, valid bool) {
	valid = true
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				return i + 1, valid
			}
			return -1, valid
		}
		name, _, equals, long := splitOption(arg)
		if !long {
			if len(arg) > 1 && arg[0] == '-' {
				valid = false
				continue
			}
			return i, valid
		}
		o, ok := optionNamed(globals, name)
		if !ok {
			valid = false
			for _, cmd := range commands {
				o, ok = optionNamed(cmd.options, name)
				if ok {
					break
				}
			}
		}
		if ok && o.kind != boolean && !equals {
			i++
		}
	}
	return -1, valid
}

func jsonRequested(args []string, local []option) bool {
	mode := false
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			break
		}
		name, value, equals, ok := splitOption(args[i])
		if !ok {
			continue
		}
		if name == "json" {
			if !equals {
				mode = true
			} else if enabled, err := strconv.ParseBool(value); err == nil {
				mode = enabled
			}
			continue
		}
		o, known := optionNamed(globals, name)
		if !known {
			o, known = optionNamed(local, name)
		}
		if known && o.kind != boolean && !equals {
			i++
		}
	}
	return mode
}

func parseOptions(args []string, features Features) (invocation, error) {
	at, valid := commandAt(args)
	var in invocation
	if at >= 0 {
		in.cmd = commandNamed(args[at], features)
		in.indexAlias = args[at] == "index"
	}
	in.json = jsonRequested(args, in.cmd.options)
	if !valid {
		return in, usage("only global flags may precede the command")
	}
	if at >= 0 && in.cmd.name == "" {
		return in, usage("unknown command; run pbrain --help")
	}
	flags := pflag.NewFlagSet(in.cmd.name, pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	for _, list := range [][]option{globals, in.cmd.options} {
		for _, o := range list {
			switch o.kind {
			case boolean:
				flags.Bool(o.name, false, o.help)
			case repeated:
				flags.StringArray(o.name, nil, o.help)
			default:
				flags.String(o.name, "", o.help)
			}
		}
	}
	in.flags = flags
	argv := args
	if at >= 0 {
		argv = slices.Concat(args[:at], args[at+1:])
	}
	if err := flags.Parse(argv); err != nil {
		return in, usage("invalid option or option value; run pbrain --help")
	}
	in.json = in.boolean("json")
	in.operands = flags.Args()
	if in.json {
		in.format = render.JSON
	} else if in.boolean("color") {
		in.format = render.TextColor
	}
	if in.boolean("personal") {
		in.view = brain.PersonalView
	}
	return in, in.validate()
}

func (in *invocation) validate() error {
	if in.has("brain") && in.string("brain") == "" {
		return usage("--brain requires a nonempty path")
	}
	if in.json && in.boolean("color") {
		return usage("--color cannot be used with --json")
	}
	if in.boolean("help") || in.boolean("version") {
		return nil
	}
	n := len(in.operands)
	switch in.cmd.name {
	case "":
		return usage("a command is required; run pbrain --help")
	case "search":
		in.query = strings.Join(in.operands, " ")
		clauses, err := query.Parse(in.query)
		if err != nil {
			return err
		}
		in.clauses = clauses
	case "get":
		if n != 1 {
			return usage("get requires one id")
		}
		if in.boolean("history") && in.boolean("outline") {
			return usage("--history cannot be used with --outline")
		}
		if in.has("limit") && !in.boolean("history") && !in.boolean("outline") {
			return usage("--limit requires --history or --outline on get")
		}
	case "add":
		if n != 0 || !in.has("title") {
			return usage("add requires --title and takes no operands")
		}
		if in.has("body") && in.has("body-file") {
			return usage("--body cannot be used with --body-file")
		}
		if in.has("body-file") && in.string("body-file") == "" {
			return usage("--body-file requires a nonempty path")
		}
		if in.string("kind") == "decision" {
			if !in.has("decided") || in.has("verified") {
				return usage("a decision requires --decided and refuses --verified")
			}
		} else if in.has("decided") {
			return usage("--decided is only for a decision")
		}
	case "import":
		if n != 1 || in.operands[0] == "" {
			return usage("import requires one input")
		}
		url := isWebSource(in.operands[0])
		if url && in.has("source") {
			return usage("--source is only for a file import")
		}
		if !url && in.boolean("link-only") {
			return usage("--link-only requires a URL")
		}
		if in.has("summary") && !in.boolean("link-only") {
			return usage("--summary requires --link-only")
		}
	case "verify", "delete":
		if n != 1 {
			return usage(in.cmd.name + " requires one id")
		}
	case "skill":
		if n > 1 {
			return usage("skill accepts at most one topic")
		}
	case "audience":
		if err := in.validateAudience(); err != nil {
			return err
		}
	default:
		if n != 0 {
			return usage(in.cmd.name + " accepts no operands")
		}
		if in.has("path") && in.string("path") == "" {
			return usage("--path requires a nonempty path")
		}
	}
	if in.boolean("shared") && in.boolean("personal") {
		return usage("--shared cannot be used with --personal")
	}
	if in.has("context") {
		value, err := decimal(in, "context", 1, 3)
		if err != nil {
			return err
		}
		in.context = value
	}
	if err := in.validateRead(); err != nil {
		return err
	}
	if in.cmd.name == "add" && in.string("title") == "" {
		return &errs.Error{Code: "invalid", Message: "title: must not be empty", Exit: 1}
	}
	return nil
}

func (in *invocation) validateAudience() error {
	if len(in.operands) == 0 {
		return nil
	}
	switch in.operands[0] {
	case "set":
		if len(in.operands) != 3 || in.operands[2] != "shared" && in.operands[2] != "personal" {
			return usage("audience set requires a match and shared or personal")
		}
	case "unset":
		if len(in.operands) != 2 {
			return usage("audience unset requires one match")
		}
	default:
		return usage("unknown audience operation")
	}
	for _, o := range pages {
		if in.has(o.name) {
			return usage("paging options are only for audience listing")
		}
	}
	return nil
}

func (in *invocation) validateRead() error {
	limit, maxChars, upper := defaults(in.cmd.name, in.boolean("history"), in.boolean("outline"))
	in.page = pageOptions{limit: limit, maxChars: maxChars}
	var err error
	if in.has("offset") {
		in.page.offset, err = decimal(in, "offset", 0, 0)
		if err != nil {
			return err
		}
	}
	if in.has("limit") {
		in.page.limit, err = decimal(in, "limit", 1, upper)
		if err != nil {
			return err
		}
	}
	if in.has("max-chars") {
		in.page.maxChars, err = decimal(in, "max-chars", 1, 0)
		if err != nil {
			return err
		}
	}
	if in.cmd.name == "search" || in.cmd.name == "list" {
		for _, name := range in.all("kind") {
			kind, ok := item.ParseKind(name)
			if !ok {
				return usage("--kind requires an item kind")
			}
			in.filter.Kinds = append(in.filter.Kinds, kind)
		}
		if in.has("status") {
			switch in.string("status") {
			case "verified":
				status := item.Verified
				in.filter.Status = &status
			case "provisional":
				status := item.Provisional
				in.filter.Status = &status
			default:
				return usage("--status requires provisional or verified")
			}
		}
		in.filter.All = in.boolean("all")
	}
	if in.cmd.name == "search" || in.cmd.name == "list" || in.cmd.name == "prime" {
		for _, value := range in.all("tag") {
			tag, ok := item.NormTag(value)
			if !ok {
				return usage("--tag requires a valid tag")
			}
			in.filter.Tags = append(in.filter.Tags, tag)
		}
	}
	return nil
}

func defaults(name string, history, outline bool) (limit, maxChars, upper int) {
	switch name {
	case "search":
		return 5, 3000, 50
	case "list", "audience":
		return 20, 2000, 200
	case "get":
		switch {
		case history:
			return 20, 2000, 200
		case outline:
			return 50, 2000, 200
		default:
			return 0, 6000, 0
		}
	case "prime":
		return 0, 2000, 0
	case "tags":
		return 50, 2000, 200
	case "check":
		return 50, 4000, 0
	default:
		return 0, 0, 0
	}
}

func decimal(in *invocation, flag string, lower, upper int) (int, error) {
	n, err := strconv.Atoi(in.string(flag))
	if err != nil || n < lower || upper != 0 && n > upper {
		if upper != 0 {
			return 0, usage(fmt.Sprintf("--%s requires an integer from %d to %d", flag, lower, upper))
		}
		return 0, usage(fmt.Sprintf("--%s requires an integer of at least %d", flag, lower))
	}
	return n, nil
}

func isWebSource(raw string) bool {
	scheme, _, ok := strings.Cut(raw, ":")
	return ok && (strings.EqualFold(scheme, "http") || strings.EqualFold(scheme, "https"))
}

func usage(message string) error {
	return &errs.Error{Code: "usage", Message: message, Exit: 2}
}
