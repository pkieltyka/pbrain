---
status: complete
---

# Component: Bounded read output

## Purpose and scope

`render` fits a document to an exact stdout budget in Unicode code points. The CLI read handlers select and shape the results of `search`, `get`, `list`, `prime`, `tags` and `check`. `get --outline` uses the shared Markdown parser. Storage, graph evaluation, matching, argument parsing and semantic scoring remain in their existing components. [Architecture §§7.4, 8.5, 9 and 13](../architecture.md) · [Functional spec §§6–7 and 9.1](../functional_spec.md)

## Interfaces

```go
package render

import "encoding/json"

type Doc struct {
    Head   []Part
    Units  []Unit
    Offset int
    Total  int
    Foot   func(st State) Part
    Hints  []Part
}
type State struct { Shown, More, Next int }
type Unit struct {
    Main   Part
    Alts   []Part // complete replacements for Main, richest context first
    Extras []Part // related rows
}
type Part struct {
    Text  string
    JSON  any
    Marks []Mark
}
type Mark struct {
    Start, End int // half-open code-point span in Text
    Style      Style
}
type Style uint8
const (
    Bold Style = iota
    Cyan
)
type Format uint8
const (
    Text Format = iota
    JSON
    TextColor
)

type Object map[string]any
type Envelope struct {
    Fields Object
    Rows   string // results, history, outline, items, tags, findings, rules
}
type BodyDoc struct {
    Doc    Doc          // Offset and Total count code points; Units, Hints and Foot are unused
    Body   []byte       // valid UTF-8, same version as the header used by Doc
    LineAt func(int) int // one fresh item.LineCursor.At, called with nondecreasing offsets
    Extras []Part       // related rows for this one item
    Tail   string       // fixed linked-source line, without its final newline
}

func Fit(d *Doc, limit int, f Format) ([]byte, error)
func FitBody(d *BodyDoc, limit int, f Format) ([]byte, error)
func OneLine(value string, maxChars int) string
func ExactPath(path string) (display string, encoded json.RawMessage, ok bool)
```

`Doc`, `State`, `Unit`, `Part`, `Mark`, `Style`, `Format` and `Fit` follow architecture §9. `FitBody` uses the same format and budget logic. The read handler passes a fresh `it.Lines(body).At` as `LineAt` for each call on a nonempty slice. This keeps `render` independent of `item`. No cursor is needed for an empty slice.

For a list, the first head part carries one `Envelope` in `JSON`. Each main row, alternative and extra carries its own `Object`. Related extras attach under `related`. The footer carries top-level fields such as `more`, and each hint is one `alone` row. `FitBody` uses an envelope without a `Rows` key, with body and related rows attached to the root object. Other head parts are text only. Inconsistent documents and encoding failures are internal `io` errors. A caller-supplied nonpositive budget is `usage`, and a valid but insufficient one is `budget_too_small` with the exact minimum in its message. Both fitters return an empty buffer on error. `OneLine` flattens line breaks and control characters and cuts to the supplied limit, counting the closing ellipsis in it. Zero flattens without cutting. `ExactPath` implements the search and get locator projection below.

The read handlers consume `*brain.Brain`, `*store.Catalog`, `*graph.Graph`, one evaluation instant, parsed options and the caller's view. They produce a `Doc` or `BodyDoc` in memory, then call the fitter before writing any bytes. Gate all item and link inputs by the caller's view before constructing rows. `render` imports only `errs` and the standard library. The handlers assemble domain values from `brain`, `store`, `item`, `graph`, `query`, `search`, `outline`, `check` and `secretscan`. The status and build output of `index` remain in the semantic component.

## Fitting

### Whole rows

1. Builders apply the item limit and offset to the ordered rows. They pass the original `Total` and requested `Offset`. If `Offset >= Total`, the page is empty. For `k` emitted rows, set `Shown=k`, `More=max(0, Total-Offset-k)`. Set `Next=Offset+k` only when `More>0`. Validate the page bounds before doing arithmetic so an offset near max int is harmless.
2. Serialize the smallest required response: zero rows for an empty page, one row for a nonempty page. Include the complete header, faithful footer for that state and final newline. If it does not fit, return that encoded size as the minimum. A synthetic zero-row state with a continuation has no independent budget requirement when a row can be emitted.
3. For a nonempty page, try the next whole main row at each step, with a fresh footer for that state. Stop at the first row that cannot fit and freeze `State` and the admitted units. In admitted unit order, try each unit's `Alts` in order against the complete response with that row substituted. Keep the first that fits or retain `Main`. Finish each unit before trying the next. Then try related extras in admitted unit order and hints in their stored order. Stop each of these two passes at its first non-fitting addition. An optional part cannot affect the minimum, add a row or change continuation.
4. In text, join nonempty head and row lines with LF, then the footer. Preserve intentional blank separators inside a part. Optional clause hints extend the last line with ` Alone: <clause> <count>, ….` Include one final LF. JSON uses one root object, encoded compactly with HTML escaping disabled and one LF. Go's sorted string-map keys keep the encoding deterministic. Count `utf8.RuneCount` over emitted bytes, including JSON escapes and the newline. Zero-valued list lengths do not require an empty array.
5. For `TextColor`, fit exactly as `Text` through step 4. After the plain response is fixed, visit the selected parts' marks in output order. Append a complete styling pair only if both sequences fit, stopping at the first pair that does not. Bold uses `ESC[1m`, cyan uses `ESC[36m`, and both reset with `ESC[0m`. Count every code point of both sequences. Marks are sorted, nonoverlapping ranges within their part's original text. Inserting an earlier escape cannot move a later range because the writer walks the original text exactly once. JSON and plain text ignore decoration. No terminal width or environment controls the fits.

Encode each trial row once and keep cumulative code-point counts. Measure a combined state from these encoded parts, their commas and delimiters, and a fresh encoding of the footer. Appending an extra re-encodes only that row. Use the same serialization rules for these parts and for the final buffer. Decorate only the actual chosen parts, so a substituted context alternative carries its own marks. A property test compares the calculated size to the complete assembled buffer for each trial.

### Raw body

`FitBody` uses the same envelope, mandatory-part and optional-extra rules. It slices the body only on code-point boundaries. A body offset beyond the end leaves the header and no continuation, even if the original body was nonempty.

1. Reserve the header, closing line or JSON envelope, and linked locator if present. Also reserve the first remaining body code point when there is one. With that character, seed `line_from = LineAt(Offset)` and reserve the same value for `line_to`. Serialize the reserved part with the actual range, `more` and `next` for this prefix. Its size is the minimum. `get` text ends with the linked line after the body and continuation, even if the body is absent. JSON puts its complete locator in `source`.
2. Add as many related lines as fit with that reserve, in graph order. Then choose the longest remaining body prefix that fits with those lines and its actual footer. In text, body bytes pass through verbatim, after a blank line that separates them from the header and related lines. For a nonempty slice the footer is `[body <offset> to <last> of <total>; file L<from>–L<to>; more: --offset <next>]` when more remains, or `[body <offset> to <last> of <total>; file L<from>–L<to>]` at the end. JSON has `line_from` and `line_to` as numbers. Both appear for a nonempty slice on even one line. Add a separator LF between body and footer only if the body did not already end in LF. An empty slice has no body block or range fields and never calls `LineAt`.
3. Scan candidate ends in ascending code-point order. For a slice `[Offset,end)`, get `line_to` at `end-1` from the forward cursor. The LF terminating a line is on that line, even after a CR. Increment the encoded body length using the JSON string escape rules or one code point per text rune. Account for the body key, quotes and separators only when its JSON value is present. Compute footer cost from the current count and decimal widths, including both line values and the missing continuation when the last rune fits. Save the winning end and line range at each fitting candidate so final serialization never seeks the cursor backward. Consider the complete-body candidate even if an earlier footer runs out of room. The cursor never rescans a prefix and the total walk is linear. Encode the chosen prefix once and assert it fits. A property test compares against exhaustive serialization for short bodies, including U+2028/U+2029, escapes and range transitions.
4. Apply `TextColor` only after the body slice, exact lines, continuation and footer are fixed. Colour styling may touch the get header's ID and path value only. It never transforms source or body bytes.

Both fitters compute the minimum for the command as asked. For search, one exact result followed by a semantic or partial result can need a larger budget than the exact result alone. The merged list adds a continuation to the footer in both text and JSON. If that minimum exceeds the budget, return `budget_too_small` with the new value. At that value the exact row and snippet are unchanged, and the footer points to the next row on the merged list. All three groups share the same budget.

## Read commands

### Common rows and fields

Use a single builder for the item projection in search, list, history and prime. Text renders search/list/prime dates as UTC `YYYY-MM-DD` or `unknown`; JSON and get use the full UTC RFC 3339 instant, or `unknown`. Recorded status is always from the item, even for a replaced item; standing is separately formatted only where requested. Cut titles to 120, actors to 80 and ordinary source locators to 80 in result rows or 200 in get. Flatten one-line values before measuring. Do not transform body bytes. JSON keeps deterministic sorted keys so composition uses the standard encoder. Consumers identify fields by key.

| Shape | Field and presence rules |
|---|---|
| Common JSON row | `id`, `kind`, `status`, `title`, `created` with required fallback values. Nonempty `author`, `source`, `decided` and actor `verified`. Flags `review_due`, `linked`, `personal` only when true. |
| Search hit | Common row plus exact `path` when representable, numeric `body_line`, `body_chars`, `file_lines`, `file_chars` and `occurrences` always. A snippet adds `snippet`, `snippet_offset` including zero, and `snippet_line`. Include `section` for a named heading on a long snippet, cut at 80. A fitted alternative adds `context`. For a semantic hit add `"semantic": true`. For a partial hit add `"partial": true` and numeric `matched` and `clauses`. Omit these three partial fields on exact and semantic rows. |
| Replacement state | With `--all`, `replaced:true` for replaced rows. Search carries their `replacement` array from direct replacements. List keeps one text line per item and carries `replacement` in JSON for replaced items. History carries `replaces` from effective links. Get carries `replacement` and `current`; all are arrays of ids, also for one. |
| Related | `link` (`to` or `from`), `id`, `kind`, `standing`, `created`, `title`, followed by applicable true `personal`, `replaced`, `review_due` and `linked` markers. |
| Get header | Common item fields plus exact `path` when representable, visible `replaces` and `relates`, `replacement`, `current`, `unresolved` when positive, `tags`, `stale_after`, `origin`, `fetched` when present. `body_line`, `body_chars`, `file_lines`, `file_chars` and requested `offset` are always numeric. Include `body`, `line_from` and `line_to` when a nonempty slice is emitted. A linked item's `source` is complete. |
| Common envelopes | Omit empty lists, empty strings and false flags. `more` and `next` appear together only when `More>0`. `errors`, `decisions`, `learnings`, outline totals and tag counts include zero where the mode uses them. |

ID lists keep the first ten and express the tail in text as `(+N more)`, or the matching `<field>_more` count in JSON when positive. This cap applies to ids inside a field; paged row counts remain independent. Related rows come from `Graph.RelatesTo` then `Graph.NewerRelated`, deduplicating items in that order to a maximum of three before fit. Related lines print `relates to` or `newer related`, followed by id, kind, effective standing, date, title and markers. All search hits can receive them. These rows never affect `Total`. Build reference strings from the graph's view-filtered facts.

Only search adds a locator or snippet data to the common item row. List, prime, history and related rows retain their current text and JSON projections. Get and outline add only their own fields below.

### Exact paths

The search and get builders pass the view-checked `Brain.DisplayPath` value to `ExactPath`. It retains the Git-root or brain-root anchor and the `personal:` prefix. It never cuts a path and never substitutes U+FFFD for invalid bytes. For invalid UTF-8 it returns `ok = false`. Use `?` in the search location instead of the path, keep any real snippet line after it, and omit the `path` key. Ordinary get omits both its path line and key.

For valid UTF-8, write a single JSON string. Escape the double quote and backslash with a backslash. Escape Unicode categories C and Z except ASCII space as `\u` plus four hexadecimal digits. Use a surrogate pair for a supplementary code point. The resulting token round-trips to the original path. Put the token into the JSON projection as `json.RawMessage` so encoding the row retains those escapes rather than escaping their backslashes again. In text display the same token when any rune needed escaping or the path contains an ordinary space. Otherwise show the path bare. In either case the file-line suffix, when present, is outside the path token. This path writer is specific to search and get. Check and other existing path output retain their current rules.

### Search

Scan with `KeepAll`, build the graph and call `search.Run` with the selected view and filters. Set `Request.Context` from the parsed flag and `Request.LongBody` from the configured `get.max_chars` (default 6,000). Fit the merged page at the configured search character budget (default 3,000), with the caller's limit (default five). Pass `Result.Offset`, `Result.Total` and the exact, semantic and partial hits in the returned order. CLI turns `Result.Sem` into a stderr notice, independently of fitting stdout. If the semantic tier is unavailable, fit the exact and partial hits that search returned.

1. Print id, kind, recorded status and date separated by two spaces. For an added hit append `semantic` or `partial <matched>/<clauses>` first. Those markers are mutually exclusive. Then append any applicable `linked`, `review due`, `replaced` and `personal` in that order. Mark the ID bold.
2. Indent the title-and-source line by two spaces. Its templates are `<title> (<actor>; source: <source>)`, `<title> (<actor>)`, `<title> (source: <source>)` and `<title>` according to presence. The actor phrase is `decided by <actor>` for a decision with a decider and `by <author>` otherwise.
3. For a snippet, print `  <path>:<line>  [body L<body_line>, offset <start>/<body_chars>; file <file_lines>L/<file_chars>C; <occurrences> occurrence[s]]`. Use `Snippet.Line` and `Snippet.Start` and the visible item's `Layout`. If the snippet is long and has a heading, append ` [in <section>]`, with the heading cut at 80. With no snippet print `  <path>  [body L<body_line>, <body_chars>C; file <file_lines>L/<file_chars>C; <occurrences> occurrence[s]]` and leave out the snippet fields. This uses the real sizes for an empty body and for a title-only semantic passage on a nonempty body. Use `Hit.Occurrences`, with `1 occurrence` and plural at every other count, including zero. The cyan mark covers only the path token and its line suffix when present, or just the path token without a snippet. It excludes the bracket and is absent when the path is withheld as `?`.
4. Indent the snippet by two spaces on its own line. Flatten the entire selected stretch of at most 200 code points and add leading or trailing `…` outside it when the span leaves body text before or after it. Use this text in JSON too. Budget for those glyphs without dropping any of the selected stretch. There is no snippet line when the span is empty.
5. A replaced hit adds its fixed `  replacement: <ids>` line before any related extras.

For `--context`, build complete alternatives for each row with `Snippet.Context`. Use each supplied `ContextLine.Line` as its physical file line. For a focal line, flatten `Text` without a cut and add ellipses according to `Before` and `After`. For a neighboring line, flatten and cut with `OneLine(Text, 160)`, ellipsis included. In text the alternative replaces the snippet with lines `    <line>: <text>` in file order and keeps its other lines and marks. If the line's display text is empty, print `    <line>:` without a space after the colon. In JSON keep the existing snippet and add a nonempty `context` array of objects with numeric `line` and string `text`, omitting an empty text value under §6.2. The number shows even on a blank line.

Start with the full context block. Count its whole encoded addition in JSON (including the field name, comma and escapes) or its whole prefixed block in text (including each line feed). Offer it only when it is at most 600 code points in the selected format. For each next alternative, remove the neighboring line farthest from the focal span, or the later line on a tie. Never remove a focal line. Stop with the focal lines alone. If those lines exceed the block cap, no alternative is available. If none fits the remaining response budget, retain the original row. The renderer never manufactures context for a hit with no snippet.

The footer is `<shown> result[s].`, then any `<more> more: --offset <next>.`, then the exact-match replaced count in `replaced_matched` and text. Use `<n> replaced item[s] also matched: --all.` when `--all` is absent and there are replaced matches, otherwise `<n> replaced item[s] matched.` With a time window append `<n> match[es] outside the time window: drop <flags>.` and include its exact-match `outside_window` count. Show `--since` then `--until`, separated by one space when both apply. When the full merged list is empty at offset 0, offer up to eight `alone` entries in query order, label cut at 40. Fit each optional hint by measuring the suffix for the admitted count, for example ` Alone: a 1, b 2.` One period follows the last admitted entry. No hint follows an offset greater than zero, even if `Total==0`. Shared, personal and window counts derive from search's eligibility.

### Get, history and outline

Resolve the folded exact ID with the catalog. On a miss, first try visible IDs beginning with the entire folded argument. If that gives no safe candidates and the first 15 characters are a valid pbrain UTC stamp, try that 15-character prefix instead. Deduplicate by folded ID, sort by folded ID then path, filter sensitive values and keep at most three. More than one exact match is `ambiguous_id`. Use `KeepNone` and handle a changed reread by rescanning and resolving again once. A second changed read is `conflict`. Never combine a header from one scan with bytes from another. History uses no body read.

- **Ordinary get.** Text prints the ID, then kind, recorded status, full created time and applicable `linked`, `review due`, `replaced` and `personal` in that order, separated by two spaces. A source appends `outside material, reference only` last. After the status line, print `title:`, then a line joining present `author:`, `verified:` and `decided:` with two spaces. Follow in order with `source:`, `replaces:`, `replacement:`, `current:`, `relates:`, `tags:`, `stale_after:`, `origin:`, `fetched:`, `unresolved:` and `path:`. Each present label has one space before its value. Omit empty values and an unrepresentable path. Always finish the header with `body: L<body_line>, <body_chars>C; file <file_lines>L/<file_chars>C` from `Item.Layout`. Mark the leading ID bold and, when present, just the exact path token cyan. The `path:` label and separator stay plain. `Graph.Shown` supplies displayed raw link IDs and `Graph.Unresolved` the count without hidden IDs. Build a `BodyDoc` with the reread bytes, a fresh line cursor, related extras and the fixed linked source line when applicable. An empty linked summary still prints the header and fixed source line.
- **History.** Use `Graph.History` exactly in graph order. Text is `<id>  <kind>  <status>  <date>  <title> (decided by <actor>; replaces: <ids>)` plus applicable markers in get order. Omit either absent parenthetical part and its separator, or the entire parentheses when both are absent. Mark the leading ID of each history row bold. Output `history` rows with the common JSON projection and `replaces`. Count items for paging.
- **Outline.** Pass the reread body to `outline.Parse`. For each section, use `Start` as the body code-point offset and `Tree-Start` as its size including subsections. One `it.Lines(body)` cursor visits the selected sections in rising offset order and gives the file line of each `Start`. Head text prints the ID (marked bold) and `outline: <sections> sections, body <body_chars>`. Each section line is `<offset>  <chars>  L<file_line>  <heading>`, with level-number `#` marks and title, or `(before the first heading)` at level 0. Cut the title at 120 characters. JSON root keys are `id`, `sections`, `body_chars` and `outline`, with rows `offset`, `chars`, `file_line`, `level` and `title` when nonempty. Preserve numeric zeros. Section ordinal is the paging unit. A position printed on a row is for ordinary `get --offset`. An empty body contributes no rows. `--outline --history` is a usage error. Nothing in this mode consults a model or index.

### List, prime and tags

- **List.** Use `KeepNone` and the same `search.Filters.Admit` as search for kind, tag, status, lifecycle and time. Count current items admitted by every filter except the time window for `outside_window`. Sort admitted items newest known first, unknown last, then folded ID and path. Under `--all` keep replaced items in this date order and mark them. Print one line of id, kind, recorded status, UTC date, title and markers. Use `results` in JSON. The footer gives shown count, continuation and any outside-window count, without search's lexical diagnostics.
- **Prime.** Use `KeepNone`. Use current decisions with a decider, then current verified learnings, applying all requested tags to both. Sort each group newest first, unknown last, then folded ID and path. Text always prints the two fixed spec §7.4 header lines and totals from the full groups. Rows are kind, date, id, title, actor and applicable markers. Actors are decider for decisions and author for learnings. JSON has `decisions`, `learnings` and `items`. The footer holds a continuation only when rows remain.
- **Tags.** Use `KeepNone`. Count every current item of the view once per normalized tag. From a declared table list every declared tag by name, including zero counts, then the undeclared names in use by name with a marker. With no table list names in use by name without the marker. Compare normalized names, and never count a tag on a replaced item. The root list is `tags`, with rows `tag`, `items`, `description` (cut 120) and `undeclared:true` when it applies. Text is tag, count, then description or marker. The footer is shown tags and continuation. A shared listing is byte-identical if the only change is to a tag on a personal item.

### Check

The `check` package supplies this entry point. CLI projects `store.Skipped` to `check.Skipped` without discarding any diagnostic fields. Pass the same catalog and graph of the selected view. DisplayPath is `Brain.DisplayPath`; dependency injection keeps the check package independent of brain and store.

```go
package check

type Skipped struct {
    Section item.Section
    Path, Code, Reason string
    Line, Column int
}
type Options struct {
    Personal bool
    HasTags bool
    Tags map[string]string
    Now time.Time
    DisplayPath func(item.Section, string) string
}
type Finding struct {
    Severity, Code, Path, Message string
    Field string
    Line, Column int
}
func Collect(items []*item.Item, skipped []Skipped, g *graph.Graph, opt Options) (findings []Finding, errorCount int)
```

`Collect` performs no I/O and returns the full sorted set and its full error count. Items and skipped entries come from the scoped catalog; the personal flag also controls cross-section findings. CLI owns page selection, document creation and the exit status.

Collect a `KeepAll` catalog and graph of the caller's view. `check` combines skipped file facts, parser problems, due dates, duplicate folded IDs, graph findings, shared-only secret matches and undeclared tags. Build exactly one finding per issue with severity, code, anchored path, safe description and optional sort coordinates. For each ambiguous folded ID report each affected visible file. Skipped entries carry safe reason and position from the same scan. Never reread them to explain a diagnostic. Use `secretscan.Sensitive` to redact diagnostic values, including undeclared tags and other IDs or paths that match the scanner. A dangling or private target contributes to its finding without appearing in the message. Count errors from the full findings set before taking the page.

Order by severity (error, warning, info), anchored path, code, field, line, column and message. Text is one line per finding, `<severity> <code> <path>  <message>`. JSON has `errors` even when zero, and `findings` rows with `severity`, `code`, `path` and `message`. If fitting succeeds, print the report on stdout and exit 1 when the full error count is positive, also when offset is past the end. An insufficient budget yields `budget_too_small` without a partial report. For other reads, CLI issues its single scoped `skipped_files` notice. See the CLI design for its literal text.

### Shared footer templates

In these templates singular applies only to a count of one. Append the continuation only if `More>0`.

| Mode | Base footer | Continuation suffix |
|---|---|---|
| List and history | `<shown> item[s].` | ` <more> more: --offset <next>.` |
| Outline | `<shown> section[s].` | ` <more> more: --offset <next>` |
| Tags | `<shown> tag[s].` | ` <more> more: --offset <next>.` |
| Check | `<shown> finding[s], <errors> error[s].` | ` <more> more: --offset <next>.` |
| Prime | no base footer | `<more> more: --offset <next>` as its own line |

List appends ` <n> item[s] outside the time window: drop <flags>.` after its continuation, if any, whenever a time window is active. Use the same flag order as search.

## Test plan

| Test | Assertions and spec scenarios |
|---|---|
| `TestPageLimitsAndMinima` | Both formats, budgets at and below the exact minimum, zero rows, offset past end, a single very short tag row, numeric-width transitions and continued paging. With 12 rows, offset 4 and limit 5, budget for two gives more 6 and next 6. Spec 17. |
| `TestTextAndJSONEscapes` | Multibyte and escaped characters, marker and optional field order, omitted false/empty fields, numeric zeros, one final LF, HTML escaping off. Every measurable output fits. Spec 17. |
| `TestRelatedRowsUseSpareBudget` | Two full results fit but their extras do not. Graph order and dedupe in a larger page. Personal related item stays out of shared results and counts. Spec 2 and 6. |
| `TestHybridContinuationRaisesMinimum` | In text and JSON, try one exact row followed by one semantic row, one partial row, or one of each. Using the exact-only minimum fails with `budget_too_small`. The reported minimum works, keeps the exact row and snippet unchanged, and yields `more` equal to the number of added rows and `next` equal to 1. Paging with limit 1 visits each row once in tier order. Check the first marker and JSON field presence for each row, including numeric `matched` and `clauses` on the partial row alone. Verify the exact-only counts, no `alone` hints on a nonempty merged list, and stderr notice separation with semantic unavailable. Spec 33–35 and 38. |
| `TestSearchFooterAndSnippet` | Merged and lexical totals, offset and file line including offset 0 for short and long snippets, long-only section marker, code-point wrap, two time flags and capped hints on empty first page only, including a nonzero offset when total is zero. In the 1000–1005 case the printed snippet preserves the complete match at its right edge and then the ellipsis. Spec 30, 31 and 39. |
| `TestSearchLocationAndDefaultBudget` | Text and JSON agree on the path, stored layout, snippet position and lexical occurrence total. Cover an ordinary title-only match, an empty body, a semantic title-only passage with a nonempty body, `1 occurrence`, a semantic hit with `0 occurrences` and a partial hit with separate clause coverage. Fit a five-hit page whose complete output in each form is above 2,000 and below 3,000. It fits by default, and an explicit 2,000 limit pages. Assert no locator fields in the old projections or in an empty shared response, and no shared change after adding a personal item. Spec 6.1 and 39. |
| `TestExactPathProjection` | An ordinary path is bare in text. A path with space, quote, backslash, DEL, C1, line separator or direction override is one escaped token, decodes to the unchanged path and has the line suffix outside the final quote. Cover a supplementary nonprinting code point, a valid U+FFFD path and an adjacent invalid UTF-8 path. The invalid path keeps the ID and search placeholder but adds no `path` field or get path line. Cover a visible external `personal:` path without absolute disclosure. Spec 6.2 and 39. |
| `TestContextAlternativesPreservePage` | For both formats, fit the shared main row prefix first at every budget from the minimum through a complete page. Assert the same IDs, locator, footer and continuation with context. Cover exactly 600 code points, first exclusion above 600, long neighbor cuts, equal-distance drop order, focal-only fallback, a one-line long paragraph and a dense stretch that ends on CRLF. A blank text line ends at its colon, and a forged footer inside a body appears only behind its number. A semantic tail consumes space before a context block, preserving required exact rows. Spec 40. |
| `TestColourFitsAfterPlain` | Text and TextColor use identical main and optional content, continuation and minimum. At tight budgets exercise no pair, one pair and all pairs in output order, without exceeding the cap. Removing only the added styling pairs recovers the plain body and page. Cover selected context-alternative marks, multibyte offsets, a raw escape in a stored body, an ordinary get with no path, the bold ID on each history row, and cyan around just an exact path and optional line. The bracket and withheld-path placeholder are plain. Spec 41. |
| `TestMissingIDSafeSuggestions` | Mistyped random suffix uses a valid UTC stamp prefix, literal partial ID uses its whole prefix, and private or sensitive candidates cannot appear. Spec 6 and 7.2. |
| `TestModeTextTemplates` | Literal search title variants, get header order and markers, history row, and list, prime, outline, tags and check footers, with zero, one and continued results. Spec §§7 and 9.1. |
| `TestGetBodyAndLocatorMinimum` | Body first rune, empty or past-end body, Unicode/control bytes, related priority, fixed 400-character locator and exact slice across pages. For a body starting at L9 with `A\r\nB`, a slice of `A\r` or `A\r\n` ends on L9 and the page starting at `B` gives L10. Cover digit transitions in both file lines and omitted range fields on empty slices. Compare against the exhaustive prefix oracle. A cursor spy sees nondecreasing calls and one forward walk on a single 512,000-character line. Cover the no-summary case. Spec 6, 17, 28 and 39. |
| `TestOutlineRoundTripAndPaging` | ATX, Setext, fences, preamble, headingless, empty, nested section sizes and Unicode offsets that reopen on the right heading. The printed and JSON file lines come from one cursor in rising offset order. Supports both progress pages and absent semantic resources. Spec 36 and 39. |
| `TestSearchGetOutlineSectionAlignment` | Run the real read handlers in text and JSON on the long item from architecture §14. See the fixture below. |
| `TestReadModesAndViews` | List and search filter parity, history fork order, prime grouping and actors, tags with zero, duplicate-tag suppression and shared personal-tag invariance. Spec 4, 12, 21, 30 and 32. |
| `TestCheckGlobalStatusAndRedaction` | Omitted private findings, safe parser positions, secret and dangling messages, consistent sort, and full error count controlling exit even for an empty page. Spec 6, 7, 17, 29 and 32. |

### Section alignment fixture

Use one item with a body longer than its configured `get.max_chars`, the same heading title under two different parents, a fenced pseudo-heading and multibyte text before the hits. Run the fixture with LF, CRLF and a BOM before the first delimiter. Pin the source line and body offset against the bytes for each variant.

1. Use a strict query with a unique body match whose end is at least 200 code points into the second of the two repeated sections. Use a second query whose clauses do not all match the item and whose test embedding scores above the cutoff for a passage in the first repeated section. Assert that the second query with `--semantic` returns a marked semantic hit.
2. For both hits, require the complete text locator with `path:line`, layout, snippet offset and `[in <section>]`. Assert exact JSON `file_lines`, `file_chars`, `body_line`, `body_chars`, `snippet_line`, `snippet_offset` and `section`. Run ordinary `get --offset <position>` with a budget that fits at least the raw snippet length after its header and footer. Its first printed file line matches `snippet_line`. Flatten that prefix and match it against the snippet text before its ellipses.
3. Identify each expected `get --outline` row by its parent and ordinal. Check the printed and JSON file line of its heading. Assert that the position is in its printed span and `outline.At(outline.Parse(body), position)` equals that row's index. The fenced pseudo-heading has no row. The [semantic component's TestPassageAlignment](semantic.md#test-plan) also checks that no passage starts at the fenced line and every passage stays inside a section.
4. Use a separate strict query with a sole match ending at `S + 10`, where `S` is a section's start. Give it a titled preceding section of level greater than zero starting at or before `S - 190`. Assert that the returned position is `S - 190`, the preceding heading is named and the same offset round-trips through `get`.

Spec §§7.1–7.2 and architecture §§7.4, 8.4 and 14.
