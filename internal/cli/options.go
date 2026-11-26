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

type valueForm uint8

const (
	free valueForm = iota
	integer
	choice
)

type option struct {
	name, value, help string
	kind              valueKind
	form              valueForm
	choices           func() []string
	required          bool
}

type operand struct {
	name, help string
	required   bool
	form       valueForm
	choices    func() []string
}

func textOption(name, value, help string) option {
	return option{name: name, value: value, help: help, kind: text}
}

func boolOption(name, help string) option {
	return option{name: name, help: help, kind: boolean}
}

func integerOption(name, help string) option {
	return option{name: name, value: "N", help: help, kind: text, form: integer}
}

func repeatOption(name, value, help string) option {
	return option{name: name, value: value, help: help, kind: repeated}
}

const personalHelp = "Include the personal section, which Git ignores. Without it personal items do not exist for this call. Set it only for the person's own request, and keep what it returns out of shared items and outside services."

var globals = []option{
	textOption("brain", "DIR", "use the brain at DIR"),
	boolOption("personal", personalHelp),
	boolOption("json", "emit compact JSON"),
	boolOption("help", "show help"),
	boolOption("version", "show the program version"),
}

var pages = []option{
	integerOption("limit", "maximum number of results"),
	integerOption("offset", "start at offset N"),
	integerOption("max-chars", "maximum code points in the response"),
}

var tagFilter = repeatOption("tag", "TAG", "filter by tag (repeatable)")

func statuses() []string             { return []string{"provisional", "verified"} }
func audienceActions() []string      { return []string{"set", "unset"} }
func audienceDestinations() []string { return []string{"shared", "personal"} }
func firstPartyKinds() []string      { return item.Kinds()[:int(item.Note)+1] }

var filters = []option{
	{name: "kind", value: "KIND", help: "include items of a kind (repeatable)", kind: repeated, form: choice, choices: item.Kinds},
	tagFilter,
	{name: "status", value: "STATUS", help: "match the recorded status", kind: text, form: choice, choices: statuses},
	textOption("since", "WHEN", "include items created at or after WHEN"),
	textOption("until", "WHEN", "include items created through WHEN"),
	boolOption("all", "include replaced items"),
}

var colour = boolOption("color", "colour supported text output")
var bm25 = boolOption("bm25", "include results matching part of the query")

var writeOptions = []option{
	textOption("source", "TEXT", "name the source"),
	repeatOption("tag", "TAG", "add a tag to the item (repeatable)"),
	repeatOption("replaces", "ID", "record a replaced item (repeatable)"),
	textOption("stale-after", "WHEN", "set the review date"),
	textOption("author", "ACTOR", "set the item author"),
	boolOption("shared", "write to the shared section"),
}

var getOptions = slices.Concat([]option{integerOption("limit", "maximum entries with --history or --outline")}, pages[1:], []option{
	colour, boolOption("history", "show replacement lineage"), boolOption("outline", "show body sections"),
})

type toolHints struct {
	ReadOnly    bool `json:"read_only"`
	Destructive bool `json:"destructive"`
	Idempotent  bool `json:"idempotent"`
	OpenWorld   bool `json:"open_world"`
	LongRunning bool `json:"long_running"`
}

type command struct {
	name, operands, summary string
	about                   string
	args                    []operand
	options                 []option
	callable                bool
	hints                   toolHints
}

var itemID = operand{name: "id", help: "the exact item ID", required: true}
var readOnly = toolHints{ReadOnly: true, Idempotent: true}

var commands = []command{
	{
		name: "init", summary: "create or complete a brain", callable: true,
		about:   "Create or complete a brain. Use path to choose a destination when there is no selected brain.",
		options: []option{textOption("path", "DIR", "create the brain at DIR (default docs/brain when no brain is selected)")},
	},
	{name: "skill", operands: "[list|TOPIC]", summary: "print the skill or a topic"},
	{
		name: "search", operands: "QUERY…", summary: "find matching items", callable: true,
		about: "Find matching items with IDs and cited snippets. Read a result with get. Semantic search may update the index and vector cache for recent changes.",
		args:  []operand{{name: "query", help: "the whole query, including any quoted phrases", required: true}},
		options: slices.Concat(filters, pages, []option{
			colour, integerOption("context", "include 1 to 3 nearby lines on each side"),
			boolOption("semantic", "include results found by meaning"),
		}),
	},
	{
		name: "list", summary: "list items", callable: true, hints: readOnly,
		about:   "List items without a search query. Narrow the list with kind, tag, status or a time window.",
		options: slices.Concat(filters, pages),
	},
	{
		name: "get", operands: "ID", summary: "read an item, its history or its outline", callable: true, hints: readOnly,
		about: "Read an item by ID. Use outline for section offsets and history for replacement lineage. Keep the same options when following next.",
		args:  []operand{itemID}, options: getOptions,
	},
	{
		name: "prime", summary: "show current decisions and verified learnings", callable: true, hints: readOnly,
		about:   "Read the current decisions and verified learnings before work. Use tag to stay on topic.",
		options: slices.Concat([]option{tagFilter}, pages[1:]),
	},
	{
		name: "tags", summary: "list tags", callable: true, hints: readOnly,
		about:   "List tags and their visible item counts. Prefer an existing tag when adding an item.",
		options: pages,
	},
	{
		name: "check", summary: "report findings about the brain", callable: true, hints: readOnly,
		about:   "Report problems with the brain without changing files. A report with errors is still a result.",
		options: pages,
	},
	{
		name: "add", summary: "add a first-party item", callable: true,
		about: "Add a decision, learning, observation or note. Supply a title and use body or body-file for its text. Name replaced items with replaces.",
		options: slices.Concat([]option{
			{name: "title", value: "TEXT", help: "set the item title (required)", kind: text, required: true},
			{name: "kind", value: "KIND", help: "set the item kind (default " + item.Note.String() + ")", kind: text, form: choice, choices: firstPartyKinds},
			textOption("body", "TEXT", "use TEXT as the body"),
			textOption("body-file", "FILE", "read the body from FILE"),
			textOption("decided", "ACTOR", "name the decision maker"),
			textOption("verified", "ACTOR", "name the verifier"),
			repeatOption("relates", "ID", "record a related item (repeatable)"),
		}, writeOptions),
	},
	{
		name: "import", operands: "INPUT", summary: "copy a source or record a link", callable: true, hints: toolHints{OpenWorld: true},
		about: "Import outside material from a file or URL. Use link-only to record a URL without fetching it. Audience rules select a destination.",
		args:  []operand{{name: "input", help: "the file path or HTTP(S) URL", required: true}},
		options: slices.Concat([]option{
			textOption("title", "TEXT", "override the title"),
			boolOption("link-only", "record a URL without fetching it"),
			textOption("summary", "TEXT", "set the body of a linked source"),
		}, writeOptions),
	},
	{
		name: "verify", operands: "ID", summary: "verify a learning or other first-party item", callable: true,
		about: "Mark a first-party item as verified. Supply the verifying actor with by when needed.",
		args:  []operand{itemID}, options: []option{textOption("by", "ACTOR", "name the verifier")},
	},
	{
		name: "delete", operands: "ID", summary: "preview or delete an item", callable: true, hints: toolHints{Destructive: true},
		about: "Preview removal and its effects before deleting. Set yes only after the person authorizes deletion.",
		args:  []operand{itemID}, options: []option{boolOption("yes", "authorize deletion")},
	},
	{
		name: "audience", operands: "[set MATCH shared|personal | unset MATCH]", summary: "list or edit audience rules", callable: true,
		about: "List audience rules with no action. Set a rule to route future writes, or unset an existing rule.",
		args: []operand{
			{name: "action", help: "leave absent to list rules", form: choice, choices: audienceActions},
			{name: "match", help: "the match key for set or unset"},
			{name: "to", help: "the destination for set", form: choice, choices: audienceDestinations},
		},
		options: pages, hints: toolHints{Destructive: true, Idempotent: true},
	},
	{
		name: "sync", summary: "build or update the semantic index, or report its state", callable: true,
		about: "Report semantic index state with status. A build may download model files and take hours on CPU. Start a first build only when requested.",
		options: []option{
			boolOption("status", "report index state without building"),
			textOption("device", "SELECTOR", "run embedding on SELECTOR (default PBRAIN_DEVICE or auto)"),
		},
		hints: toolHints{Idempotent: true, OpenWorld: true, LongRunning: true},
	},
	{
		name: "devices", summary: "list devices available for embedding", callable: true, hints: readOnly,
		about: "List the CPU and available accelerators for embedding. Use a device selector when building a semantic index.",
	},
}

func commandNamed(name string, features Features) command {
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
	cmd      command
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

type rule struct {
	commands   []string // empty applies to every command
	properties []string
	message    string
	broken     func(*invocation) bool
}

var jsonAndColor = rule{nil, []string{"json", "color"}, "--color cannot be used with --json",
	func(in *invocation) bool { return in.json && in.boolean("color") }}

var rules = []rule{
	{[]string{"get"}, []string{"history", "outline"}, "--history cannot be used with --outline",
		func(in *invocation) bool { return in.boolean("history") && in.boolean("outline") }},
	{[]string{"get"}, []string{"limit", "history", "outline"}, "--limit requires --history or --outline on get",
		func(in *invocation) bool { return in.has("limit") && !in.boolean("history") && !in.boolean("outline") }},
	{[]string{"add"}, []string{"body", "body-file"}, "--body cannot be used with --body-file",
		func(in *invocation) bool { return in.has("body") && in.has("body-file") }},
	{[]string{"add"}, []string{"body-file"}, "--body-file requires a nonempty path",
		func(in *invocation) bool { return in.has("body-file") && in.string("body-file") == "" }},
	{[]string{"add"}, []string{"kind", "decided", "verified"}, "a decision requires --decided and refuses --verified",
		func(in *invocation) bool {
			return in.string("kind") == "decision" && (!in.has("decided") || in.has("verified"))
		}},
	{[]string{"add"}, []string{"kind", "decided"}, "--decided is only for a decision",
		func(in *invocation) bool { return in.string("kind") != "decision" && in.has("decided") }},
	{[]string{"import"}, []string{"input", "source"}, "--source is only for a file import",
		func(in *invocation) bool { return isWebSource(in.operands[0]) && in.has("source") }},
	{[]string{"import"}, []string{"input", "link-only"}, "--link-only requires a URL",
		func(in *invocation) bool { return !isWebSource(in.operands[0]) && in.boolean("link-only") }},
	{[]string{"import"}, []string{"summary", "link-only"}, "--summary requires --link-only",
		func(in *invocation) bool { return in.has("summary") && !in.boolean("link-only") }},
	{[]string{"audience"}, []string{"action", "limit", "offset", "max-chars"}, "paging options are only for audience listing",
		func(in *invocation) bool {
			return len(in.operands) != 0 && slices.ContainsFunc(pages, func(o option) bool { return in.has(o.name) })
		}},
	{[]string{"add", "import"}, []string{"shared", "personal"}, "--shared cannot be used with --personal",
		func(in *invocation) bool { return in.boolean("shared") && in.boolean("personal") }},
	{[]string{"sync"}, []string{"status", "device"}, "--device cannot be used with --status",
		func(in *invocation) bool { return in.boolean("status") && in.has("device") }},
}

func (r rule) applies(command string) bool {
	return len(r.commands) == 0 || slices.Contains(r.commands, command)
}

func (in *invocation) validate() error {
	if in.has("brain") && in.string("brain") == "" {
		return usage("--brain requires a nonempty path")
	}
	if jsonAndColor.broken(in) {
		return usage(jsonAndColor.message)
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
		if n != len(in.cmd.args) {
			return usage("get requires one id")
		}
	case "add":
		if n != len(in.cmd.args) || slices.ContainsFunc(in.cmd.options, func(o option) bool { return o.required && !in.has(o.name) }) {
			return usage("add requires --title and takes no operands")
		}
	case "import":
		if n != len(in.cmd.args) || in.operands[0] == "" {
			return usage("import requires one input")
		}
	case "verify", "delete":
		if n != len(in.cmd.args) {
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
	for _, rule := range rules {
		if rule.applies(in.cmd.name) && rule.broken(in) {
			return usage(rule.message)
		}
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
		if len(in.operands) != len(in.cmd.args) || !slices.Contains(in.cmd.args[2].choices(), in.operands[2]) {
			return usage("audience set requires a match and shared or personal")
		}
	case "unset":
		if len(in.operands) != len(in.cmd.args)-1 {
			return usage("audience unset requires one match")
		}
	default:
		return usage("unknown audience operation")
	}
	return nil
}

func (in *invocation) validateRead() error {
	var err error
	in.context, err = in.intOption("context")
	if err != nil {
		return err
	}
	in.page.offset, err = in.intOption("offset")
	if err != nil {
		return err
	}
	in.page.limit, err = in.intOption("limit")
	if err != nil {
		return err
	}
	in.page.maxChars, err = in.intOption("max-chars")
	if err != nil {
		return err
	}
	if in.cmd.name == "search" || in.cmd.name == "list" {
		option, _ := optionNamed(in.cmd.options, "kind")
		kinds := option.choices()
		for _, name := range in.all("kind") {
			if !slices.Contains(kinds, name) {
				return usage("--kind requires an item kind")
			}
			kind, _ := item.ParseKind(name)
			in.filter.Kinds = append(in.filter.Kinds, kind)
		}
		if in.has("status") {
			option, _ := optionNamed(in.cmd.options, "status")
			value := in.string("status")
			if !slices.Contains(option.choices(), value) {
				return usage("--status requires provisional or verified")
			}
			status := item.Provisional
			if value == item.Verified.String() {
				status = item.Verified
			}
			in.filter.Status = &status
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

func (in *invocation) intOption(name string) (int, error) {
	lo, hi, def := bounds(in.cmd.name, name, in.boolean("history"), in.boolean("outline"))
	if !in.has(name) {
		return def, nil
	}
	return decimal(in, name, lo, hi)
}

// bounds returns the limits and built-in default for an integer option.
// A zero upper bound means the parser imposes no explicit upper limit.
func bounds(command, flag string, history, outline bool) (lo, hi, def int) {
	switch flag {
	case "offset":
		return 0, 0, 0
	case "context":
		return 1, 3, 0
	case "limit":
		switch command {
		case "search":
			return 1, 50, 5
		case "list", "audience":
			return 1, 200, 20
		case "get":
			if history {
				return 1, 200, 20
			}
			if outline {
				return 1, 200, 50
			}
		case "tags":
			return 1, 200, 50
		case "check":
			return 1, 0, 50
		}
		return 1, 0, 0
	case "max-chars":
		switch command {
		case "search":
			return 1, 0, 3000
		case "get":
			if !history && !outline {
				return 1, 0, 6000
			}
			return 1, 0, 2000
		case "list", "audience", "prime", "tags":
			return 1, 0, 2000
		case "check":
			return 1, 0, 4000
		}
	}
	return 0, 0, 0
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
