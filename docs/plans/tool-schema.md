# `pbrain schema`: a tool description of the CLI for agents (#80)

Owner's request, 2026-11-26. `pbrain schema` prints, for every command an agent can call, its name, description, input JSON Schema, output JSON Schema and error codes, generated from the command table the parser already uses. The batching and MCP work (#81, `docs/plans/agent-batching.md`) takes its tools from here.

Based on master `c8c58db` and read from that commit. I ran the argv forms and JSON outputs quoted below on a scratch build. The MCP facts are from the 2026-07-28 revision of the specification, fetched today.

## Decision

1. **One catalog, in `internal/cli`, built from the `commands` table.** The table gains what a schema needs and the parser does not yet hold as data: named operands, the type of each value, and the cross-flag rules. The parser and `validate()` are changed to read those same fields, so a flag, a range or a rule exists in one place.
2. **A tool call is bound to argv, not to parsed state.** `Bind` turns `(tool, arguments)` into the argv of the equivalent command line and the existing `RunWith` executes it. Every call runs the same parser, validation and handlers as the CLI, so the two cannot behave differently.
3. **Output schemas are written by hand and held to the output by tests.** The `--json` documents are built from `render.Object` maps with conditional fields, so they cannot be reflected. Each command gets a declared schema, and the test helpers validate every `--json` document the suite produces against it, strictly.
4. **Two formats.** `pbrain schema` prints pbrain's own document; `pbrain schema --format mcp` prints the `tools` array of an MCP `tools/list` result. No OpenAI or Anthropic function shape is emitted: both are a key rename of the MCP shape, and both vendors' agents read MCP.
5. **`personal` is an explicit boolean input of every tool that has views**, false when absent, read on each call. No transport may default it on.
6. **The catalog only grows within a major version**, the rule spec 6.2 already has for JSON. A frozen copy from each release and a test hold it.
7. **The skill points at it.** A new topic, `pbrain skill schema`, and one sentence in the core.
8. **It is evaluated with real agents before it is called done**: four arms on the retrieval fixture, scored by scripts, with the description wording tuned on one half of the tasks and reported on the other.

## Rejected

- **Generating output schemas from Go types.** It would mean rewriting every reader to typed structs first, a large change to code whose output is fixed by the spec. The strict test gives the same guarantee for what the suite exercises.
- **A separate hand-written schema file.** It drifts the day a flag is added.
- **Binding to the parsed `invocation`.** A second path into the handlers would need its own validation and would diverge.
- **Property names in snake_case.** A name that differs from its flag needs a translation table. Properties are the flag names exactly, hyphens included.
- **Bare tool names** (`search`, `get`, `list`). Clients that merge tools from several servers collide on them; the MCP specification warns of exactly this.
- **OpenAI strict mode.** It needs every property required and optional ones made nullable, which is a different schema to keep compatible. A caller who wants it can derive it.
- **Cross-flag rules as `allOf`/`if`/`not` in the input schemas.** Several tool APIs refuse those keywords at the top of an input schema. The rules go into the description as sentences, and into pbrain's own format as data.
- **A budget on `pbrain schema`'s output.** It reads no brain. Its size is controlled by selecting commands and by `--inputs-only`, and it is measured in the evaluation.

## What the code has today (verified)

- **The command table.** `internal/cli/options.go`: `commands []command{name, operands, summary, options}`, `option{name, value, help, kind}` with kinds boolean, text and repeated, and shared lists (`globals`, `pages`, `filters`, `writeOptions`). `helpText` prints from it. `commandNamed` adds `--bm25` when `Features.BM25` is set.
- **Types and ranges are code.** `validateRead` parses integers with `decimal(in, flag, lower, upper)` and takes the limits from `defaults(name, history, outline)`; `--kind`, `--status` and `--tag` are checked by hand; `--context` is 1 to 3.
- **Cross-flag rules are code**, about fifteen `usage(...)` checks in `validate()` and `validateAudience()`: `--history` with `--outline`, `--limit` on `get` without either, `--body` with `--body-file`, a decision needing `--decided` and refusing `--verified`, `--summary` needing `--link-only`, `--link-only` needing a URL, `--source` only for a file import, `--shared` with `--personal`, `--status` with `--device`, and the audience forms.
- **Operands are a usage string** (`"QUERY…"`, `"ID"`, `"[set MATCH shared|personal | unset MATCH]"`).
- **JSON output.** Readers build `render.Object` maps (`read.go`, `lists.go`, `semantic.go`, `devices.go`, `audience.go`); writes use small structs (`write_command.go`). An empty page prints `{}`: the row list is left out, like every empty field (spec 6.2).
- **Diagnostics.** With `--json` each line of standard error is `{"error":{"code","message"}}` or `{"notice":{…}}`. Exit 0, 1, or 2 for usage. `check` with errors and `delete` without `--yes` print a document and exit 1; `delete` also prints `confirm_required`.
- **Argv forms that work today** (run): `pbrain --json search --limit=2 --kind=decision -- sessions Redis`; the whole query as one operand holding spaces; an operand starting with `-` after `--`.
- **Commands with no brain.** `skill` and `devices` return from `execute` before `brain.Open`; `schema` joins them.
- **The skill.** `internal/skill/core.md` is exactly at its 400-word limit; topics are at most 300 words; `TestSkillResourcesAndLimits` holds both and a list of required phrases.
- **The fixture.** `internal/semantic/calibrate/testdata/retrieval`: 35 shared items, 107 questions with expected ids and styles, and a `personal-overlay` of eight notes with eight control questions that must not be answered in the shared view.

## The command

```
pbrain schema [COMMAND…] [--format pbrain|mcp] [--inputs-only] [--json]
```

- No operand: every callable command, in the order of the command table. Operands select commands; an unknown or uncallable name is a usage error.
- Text mode prints the document indented by two spaces; `--json` prints the same value compact on one line.
- `--inputs-only` leaves the output schemas out.
- It needs no brain and no Git, and ignores `--brain` and `--personal` as `skill` does. It reflects the `Features` of the binary: `bm25` is an input of `pbrain_search` only where the flag is released.

**Callable** are the commands that act on a brain or the machine: `search`, `get`, `list`, `prime`, `tags`, `check`, `add`, `verify`, `import`, `delete`, `audience`, `sync`, `devices`, `init`. Not callable: `schema`, `skill`, the transport commands of #81, and `--help` and `--version`.

**pbrain's own format** (`--format pbrain`, the default):

```json
{
  "schema": "pbrain-tools/1",
  "pbrain": "v0.6.0",
  "invocation": {
    "argv": ["pbrain", "<globals>", "--json", "<command>", "<flags>", "--", "<operands>"],
    "globals": ["brain", "personal"],
    "flag": "--NAME=VALUE; a true boolean is --NAME; a list is one flag per value",
    "operands": "the tool's operands, in order, each as one argument"
  },
  "exit": {"0": "success", "1": "failed, or a document with findings", "2": "wrong usage"},
  "diagnostic": { "…": "JSON Schema of one line of standard error" },
  "errors": {"usage": "…", "no_brain": "…"},
  "notices": {"skipped_files": "…"},
  "tools": [
    {
      "name": "pbrain_search",
      "command": "search",
      "description": "…",
      "operands": ["query"],
      "input": { "…": "JSON Schema" },
      "rules": [{"properties": ["semantic", "…"], "text": "…"}],
      "output": { "…": "JSON Schema" },
      "document_on_exit_1": false,
      "errors": ["usage", "no_brain", "budget_too_small"],
      "notices": ["skipped_files", "semantic_not_ready", "semantic_partial"],
      "hints": {"read_only": true, "destructive": false, "idempotent": true, "open_world": false, "long_running": false}
    }
  ]
}
```

**MCP format** (`--format mcp`): `{"tools": [ … ]}`, each tool with `name`, `title` (the command's summary), `description` (the same text, followed by its rules as sentences), `inputSchema`, `outputSchema` and `annotations` (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`). Names fit the specification's 1 to 128 characters of letters, digits, `_`, `-` and `.`, and also the narrower sets the vendor APIs take. Schemas are JSON Schema 2020-12, the specification's default, with no `$schema` key and no `$ref` out of the tool. The protocol envelope (`resultType`, `ttlMs`, `cacheScope`, paging) is the server's and belongs to #81.

**Input schemas** use only `type`, `properties`, `required`, `enum`, `minimum`, `maximum`, `items`, `default`, `description` and `additionalProperties: false`.

- Properties are the command's flags by name, then `brain` and `personal` where the command takes them, then its operands by name: `query` (search; one string, the whole query), `id` (get, verify, delete), `input` (import), `action`, `match` and `to` (audience; `action` is `set` or `unset`, and absent lists the rules).
- `json`, `help`, `version` and `color` are not inputs: a tool call is always `--json`.
- An integer flag is `integer` with the bounds the parser enforces; `default` is the built-in default, and the description says a project's `pbrain.toml` may change it. `kind`, `status` and `to` are enums. A repeatable flag is an array of strings. Everything else is a string whose description carries the accepted forms.
- Descriptions are the help strings, so `--help` and the schema say the same thing. A test caps a tool description at 80 words and a property description at 30.

**Output schemas** describe the one `--json` document of the command. Commands with several documents (`get`, `get --history`, `get --outline`; `sync` and `sync --status`; `audience` listing and edits; `delete` preview and result) use `oneOf`. They are published open, without `additionalProperties: false`, because spec 6.2 lets a release add fields and tells readers to ignore what they do not know. `required` holds only what spec 6.2 says is always printed. Shared row shapes are repeated inside each tool so that every schema stands alone.

## Interfaces and files

**`internal/cli/options.go`.**

```go
type valueForm uint8

const (
	free    valueForm = iota // any text; the description carries the accepted forms
	integer                  // bounds from bounds()
	choice                   // one of choices()
)

type option struct {
	name, value, help string
	kind              valueKind
	form              valueForm
	choices           func() []string // for choice
}

type operand struct {
	name, help string
	required   bool
	form       valueForm
	choices    func() []string
}

type command struct {
	name, operands, summary string // operands stays the usage text of --help
	about                   string // one to three sentences for an agent; printed by --help under the summary
	args                    []operand
	options                 []option
	callable                bool
	hints                   toolHints
}

// bounds is the range and built-in default of an integer option of a command. hi 0 means no upper bound.
func bounds(cmd, flag string, history, outline bool) (lo, hi, def int)

// rule is one cross-flag requirement of a command. validate() reports message as a usage error when broken reports true.
type rule struct {
	commands   []string // empty: every command
	properties []string // the inputs it concerns, for the schema
	message    string
	broken     func(*invocation) bool
}

var rules []rule
```

- `bounds` replaces the tuple of `defaults()` and the literals passed to `decimal`; `validateRead` and the schema both call it.
- `choices` for `kind` comes from `item`'s kind table (add `item.Kinds() []string`); for `status` it is `provisional` and `verified`; for audience `to`, `shared` and `personal`. The parser checks membership with the same function.
- Every cross-flag `usage(...)` check of `validate()` and `validateAudience()` becomes one `rule`, evaluated in the present order, with its present message. `validate()` keeps only what is not a rule between inputs: parsing the query, the audience forms, and filling `in.filter` and `in.page`.
- No message, exit code or accepted input changes in this step.

**`internal/cli/catalog.go` (new).**

```go
type Tool struct {
	Name, Command, Title, Description string
	Operands                          []string
	Input, Output                     json.RawMessage
	Rules                             []ToolRule // {Properties []string; Text string}
	DocumentOnExit1                   bool
	Errors, Notices                   []string
	Hints                             ToolHints // ReadOnly, Destructive, Idempotent, OpenWorld, LongRunning
}

type Diagnostic struct{ Code, Message string }

type Result struct {
	Output  json.RawMessage // the --json document; nil when the call printed none
	Error   *Diagnostic
	Notices []Diagnostic
	Exit    int
}

// Catalog is every callable command of this binary, in table order.
func Catalog(features Features) []Tool

// Bind returns the argv of the command line equal to this call:
// [--brain=DIR] [--personal] --json COMMAND [--flag=value]… -- [operands].
// Errors are usage errors: an unknown tool, an unknown property, a value of the wrong JSON type.
func Bind(tool string, args json.RawMessage, features Features) ([]string, error)

// Call runs one tool call through RunWith with its own output buffers and a nil Stdin.
// onNotice, when not nil, receives each notice as it is printed, already made safe for printing.
func Call(ctx context.Context, env Env, tool string, args json.RawMessage, features Features, onNotice func(Diagnostic)) Result
```

- `Bind` checks JSON types and unknown properties only. Ranges, enums and rules are left to the parser, so there is one validator.
- `Result.Notices` is every notice line of the call in order. A line of standard error that is not a JSON object (a GPU driver's, spec 6.3) is dropped.
- A call failed only when `Output` is nil. `check` with errors has `Output` and `Exit` 1; `delete` without `yes` has `Output`, `Error{confirm_required}` and `Exit` 1.
- `onNotice` is @architect-2's addition for live `model_download` and `index_progress` over a transport.

**`internal/cli/schema_output.go` (new).** One declared output schema a command, as Go values, written from the spec sections that define the documents (7.1 to 7.5, 8, 9) and checked against the handlers.

**`internal/cli/schema.go` (new).** The `schema` command: selection, the two formats, `--inputs-only`, indentation. The global tables of error and notice codes with one line each, from spec 6.3.

**`internal/cli/run.go`.** `execute` returns `runSchema(in, features)` before opening a brain. `printOptions` hides `--brain` and `--personal` for it.

**`internal/skill`.** A sixth topic, `schema`, and the core's pointer (below).

| Case | Result |
| --- | --- |
| `pbrain schema nosuch`, or `pbrain schema skill` | `usage: unknown or uncallable command; run pbrain schema`, exit 2 |
| `--format xml` | `usage: --format requires pbrain or mcp`, exit 2 |
| `Bind`: unknown tool, unknown property, wrong JSON type | a usage error naming the tool and the property |
| `Bind` succeeds and the value is out of range or breaks a rule | the parser's own usage error, as on the command line |

## Personal items

- `personal` is a boolean input of `search`, `get`, `list`, `prime`, `tags`, `check`, `add`, `verify`, `import`, `delete`, `audience` and `sync`, with `default: false`.
- Its description is the global flag's help text, rewritten once for both uses: "Include the personal section, which Git ignores. Without it personal items do not exist for this call. Set it only for the person's own request, and keep what it returns out of shared items and outside services."
- `shared` and `personal` exclude each other on `add` and `import`; that is one of the rules.
- Output rows from the personal section carry `"personal": true`, and the output schemas say so on that field.
- Nothing in the catalog lets a caller set a default for a session. #81 keeps that rule for its transports.

## Versioning

- `"schema": "pbrain-tools/1"` names the shape of the document. It changes only when that shape breaks.
- `"pbrain"` is the binary's version. The tools follow pbrain's version: within a major version no tool, input property or output field is removed, renamed or retyped, no enum value is removed, and no input becomes required.
- `internal/cli/testdata/schema/` keeps `pbrain.golden.json` and `mcp.golden.json` for the current tree and a frozen `release-vX.Y.Z.json` from each release that has the command.

## The skill

- **Topic `schema`** (at most 300 words): what `pbrain schema` and `pbrain schema <command>` print; that a call is `pbrain --json <command> --flag=value -- operands`; that inputs are the flags by name; where the output shape and the error codes are; the `personal` rule; that `--format mcp` exists for a harness that wants a tool list.
- **Core**: one sentence, "For exact flags and JSON, run pbrain schema <command>." The core is at its 400-word limit, so the same change shortens another sentence by nine words; `TestSkillResourcesAndLimits` gains `pbrain schema` in its required phrases.
- Spec 12 lists six topics and the core's new line.

## Tests that stop drift

1. **Input schema against the parser** (`TestCatalogInputs`). For every tool: its properties are exactly the command's options, minus `json`, `help`, `version` and `color`, plus its operands; every integer property's bounds equal `bounds()`; every enum equals `choices()`; `Bind` of the smallest input the schema allows parses without a usage error; `Bind` with each bound exceeded, each enum violated and each rule broken gives the parser's usage error.
2. **Every command accounted for** (`TestCatalogCoversCommands`). Each entry of `commands` is either callable with an output schema, hints and an `about`, or in the short list of uncallable ones. Adding a command without deciding fails.
3. **Outputs against their schemas.** The two test entry points, `invoke` in `internal/cli` and `scenarioCLI` in `cmd/pbrain`, validate every standard output produced with `--json` against the command's output schema made strict (`additionalProperties: false` on every object), and check each error and notice code against the tool's lists. A new field, a changed type or an undeclared code fails the test that produced it.
4. **Every declared output property is produced and pinned** (`TestCatalogOutputs`). A table of calls on a purpose-built brain, with a fixed clock, the test registry and its fake devices, covers every document variant and every property of every output schema, conditional ones included: replaced and replacing items, linked and personal rows, related lines, capped id lists with their `_more` counts, snippets with context, partial and semantic results, `alone` counts on an empty result, `outside_window`, `more` and `next`, each field of a `check` finding, a `delete` preview and result with `current_again` and `dangling`, `sync` with a device, `devices` with memory figures. The test walks each strict-validated document, records the property paths it saw, and fails if any path a schema declares was never seen. There is no list of exemptions. Each case's document is also a golden file under `testdata/schema/outputs/`, so a field that a handler stops printing changes a golden even though the published schema, being open and requiring little, would still accept the output.
5. **Golden files** (`TestSchemaGolden`): both formats byte for byte, refreshed with `-update`, so every change of the catalog is in the diff a reviewer reads.
6. **Only grows** (`TestSchemaOnlyGrows`): the current catalog against every frozen release file, by the rules of "Versioning".
7. **Valid schemas** (`TestSchemasCompile`): every emitted schema compiles as JSON Schema 2020-12.
8. **`Call` equals the CLI** (`TestCallMatchesCLI`): for a table of calls, `Call`'s `Output`, diagnostics and exit equal those of running the bound argv.

Checks 3 and 7 need a JSON Schema validator in tests. See "Decisions needed".

## Agent evaluation

**Question.** Can an agent use pbrain correctly from the schema alone, compared with the skill prose alone, and what does each cost?

**Tasks.** About 26 on the retrieval fixture, each with a scripted checker and no model as judge. Every task ends with a line `ANSWER_IDS: id1,id2` or `ANSWER_IDS: none`.

| Group | Tasks | What is checked |
| --- | --- | --- |
| Retrieval | 8 answerable questions across the fixture's styles, 2 unanswerable | expected ids cited; none cited for the unanswerable |
| Navigation | 4: continue a page with `next`, read a lineage with `history`, quote a section through `outline` and `offset`, narrow by time | ids and quoted text |
| Writes | 6: add a decision, replace an item, verify, import a file with its source, delete with preview then `yes`, add with a declared tag | the brain's state afterwards, read back with pbrain and from the files |
| Personal | 4: two control questions whose answer is only in the overlay, one question the person asks about their own notes, one private note to write | no `personal` and no overlay text for the first two; `personal` used and the item marked for the others |
| Index state | 2: a `semantic_not_ready` notice (must not run the first `sync`), a `semantic_partial` notice (should run `sync`) | the calls made |

**Arms.** Each run starts in an empty directory; the brain is reachable only through `pbrain`.

| Arm | The agent is told | Tools |
| --- | --- | --- |
| `skill` | "pbrain is installed. Run `pbrain skill` first." | shell |
| `schema` | "pbrain is installed. Run `pbrain schema` first." | shell |
| `both` | "Run `pbrain skill` first; `pbrain schema <command>` gives exact flags and JSON." This is what ships. | shell |
| `tools` | nothing about pbrain | the catalog as MCP tools, no shell |

The `tools` arm uses a small stdio server in the evaluation folder that knows nothing about pbrain's commands: it serves `pbrain schema --format mcp` as its tool list and executes a call by the `invocation` rule of the document. It tests that the document alone is enough to drive a generic executor. The shim speaks the MCP revision #81 pins for its stdio prototype, 2025-11-25, and each run records the revision its client negotiated; the tool fields this plan emits are the same in that revision and in 2026-07-28. When #81's server exists, the arm is rerun against it.

**Harnesses and models**, first pass: Claude Code (2.1.332 is installed here) with `claude-sonnet-5-5` and with `claude-haiku-5-5`; Codex CLI with the model it is configured for, its exact id taken from the run's output and recorded. pi, omp and agy are installed and can follow. Each run is headless with JSON event output, no project instructions, no memory and no other MCP server. The exact flags are confirmed against each CLI's `--help` when the runner is written, and the versions recorded.

**Measured for each run**: task passed; pbrain calls; invalid calls (exit 2); failed calls by error code; calls to `--help`, `skill` and `schema`; agent turns; input, cached and output tokens and cost as the harness reports them; wall time; and privacy violations. A `pbrain` wrapper on `PATH` logs argv, exit code and sizes; the harness's event stream gives turns and tokens.

**Size.** 26 tasks, 4 arms, 3 model configurations, 3 repetitions: 936 runs. At the published prices of the two Claude models that is on the order of 50 to 150 US dollars if billed by the token, most of it Sonnet; the Codex share is not estimated. Four runs side by side finish in a working day.

**Method.** The tasks are split in two halves by group. Descriptions may be revised twice against the first half; the second half is run once, at the end, and is the reported result. Reported per arm and model: pass rate, invalid-call rate, turns, tokens and cost per passed task, with the three repetitions' spread, and every privacy violation listed.

**What counts as done well.** On the held-out half: `schema` has no more invalid calls than `skill` and passes within two tasks of it; `both` passes at least as many as either alone; no arm has a privacy violation. A miss is reported with the transcripts, and the wording or the design is revised before the feature is called finished.

**Where it lives.** `tools/agent-eval/`: the runner and checkers (Python 3, standard library only, as `tools/phase28`), the tasks, the wrapper, the MCP shim, and `results/<date>/` with the raw logs and the summary table.

## Steps

Each is one reviewable change.

1. **The table carries types, operands and rules.** `options.go` as above, with `validate()` reading them. Proof: the whole suite passes unchanged, every usage message included; `--help` output differs only by the new `about` line and the `--personal` text.
2. **Catalog, `Bind`, `Call`.** Proof: tests 1, 2 and 8.
3. **Output schemas, the test hooks and the output cases.** Proof: tests 3, 4 and 7. Expect this step to find output fields the first draft of a schema missed, and schema properties no case can produce; both are its purpose.
4. **The `schema` command.** Formats, selection, goldens. Proof: tests 5 and 6, and scenario scripts for the text and JSON forms and the usage errors.
5. **Skill, spec and README.** The topic and core sentence; spec 9 gains the command, spec 12 the topic, spec 6.3 a pointer to the code tables; `components/cli.md` the catalog and its tests; README one paragraph.
6. **Evaluation harness and a pilot**: one model, the first half of the tasks, the four arms once. Fix the harness and the task checkers on what the pilot shows.
7. **Tune, then the full run and the report.** Up to two wording revisions on the first half; the held-out run; the summary in `tools/agent-eval/results/` and one paragraph in the README.

Steps 1 to 4 unblock #81. Steps 6 and 7 need no device, only the agent CLIs and their accounts.

## Decisions needed from @operator

1. **A test-only dependency for JSON Schema validation.** I recommend `github.com/santhosh-tekuri/jsonschema/v6` v6.0.3 (Apache-2.0, 2020-12, passes the official test suite). It is imported by test files only, so it is not linked into the binary and the release audit does not see it. The alternative is a validator of our own for the keywords we emit, which cannot check that a schema is itself valid.
2. **The evaluation's spend and accounts.** It runs the agent CLIs under the owner's logins, 936 runs in the first pass.
3. **Which models.** I named two Claude models and Codex's configured one. Fable or Opus, or other harnesses, are the owner's call.
4. **The `--personal` help text changes** for people too, since help and schema share it.

## Risks and gaps

- **A field pbrain prints but no schema declares, on a path no test runs, is not caught.** Check 3 sees only documents the suite produces. Test 4 closes the other direction: every declared property has a case and a golden.
- **Per-tool error lists are declared, not derived.** A code a tool can return but no test triggers may be missing from its list. The global table is complete by spec 6.3.
- **Rules reach an agent as sentences.** Whether models follow them is one thing the evaluation measures.
- **Step 1 touches the parser.** It is the riskiest change for people who use the CLI, and it must not alter one message; the existing suite is the guard.
- **Output schemas repeat the row shapes**, which makes the full document long. Its size in tokens is measured in the evaluation; `--inputs-only` and command selection are the levers.
- **The evaluation's numbers are for these tasks, models and versions.** Three repetitions give a spread, not a significance test.
- **The Codex model id and the headless flags of each CLI are not verified here.** Step 6 records them.
- **The MCP specification moves.** The tool fields are checked against 2026-07-28 and are common to 2025-11-25; the wire protocol differs between the two, and #81 owns the revision a server speaks.
