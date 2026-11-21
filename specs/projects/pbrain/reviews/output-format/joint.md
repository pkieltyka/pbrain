# Search output for agents and people

**Keep plain output and compact JSON. Give each search hit one position in two coordinate systems.** Print the stored file path and the 1-based file line at the start of the snippet, then the exact body offset that `get --offset` accepts. Add the file size, body start and lexical occurrence count. Put multiline context and terminal colour behind explicit flags. The owner chose a 3,000-character default search budget and kept the five-hit limit.

This proposal implements the direction in the [owner follow-up](owner-followup.md). The earlier round-1 arguments for holding these features back now inform the limits. [@architect](round1-architect.md) · [@architect-2](round1-architect-2.md) · [@lead-dev](round1-lead-dev.md)

## 1. Default shape and navigation

Keep the current first line, title and citation line, then add the locator, then the snippet. Keep the existing section heading on eligible long hits as a suffix of the locator. Use one-based lines for the stored file, including YAML and delimiters, and zero-based Unicode code-point offsets for the stored body. For example:

```text
  docs/brain/sources/<id>.md:412  [body L9, offset 31200/48213; file 913L/48517C; 7 occurrences] [in Sessions]
```

`path:412`, `offset 31200` and `in Sessions` describe the start of the same stretch. A stretch can begin partway through a line, or before the actual match. For the exact character, use `pbrain get <id> --offset 31200`. To inspect the full stored line, open the path and line in an editor or file-reading tool. Do not add `get --line` in v1. A second selector has little value once search and outline give the exact body offset. Keep paths anchored exactly as in `get`. An external personal item retains its `personal:` prefix and is opened by ID, offset and `--personal`. A path belongs only to a visible returned item. Do not print absolute, truncated or misleading locations. Ordinary paths print literally. If a path contains controls or needs quoting, use a one-line quoted path in text and its exact JSON string value.

| JSON field | Meaning |
|---|---|
| `path` | The same locator path as in `get`, without a line suffix. |
| `file_lines`, `file_chars` | Physical lines and code points of the complete stored Markdown file. Both include front matter. |
| `body_line`, `body_chars` | The first file line of the stored body and the exact code-point length of that body. |
| `snippet_line`, `snippet_offset` | File line and body code-point offset of the first character of the selected stretch. Both present when there is a snippet, including offset zero. |
| `occurrences` | The lexical clause-occurrence count, including zero. |
| `snippet`, `section` | The flattened snippet and the existing optional heading of long hits. |

Use the parser's existing `bodyAt` byte position, after the closing delimiter and the one optional empty separator line. For a nonempty body, `body_line` is one plus the number of LF bytes before `bodyAt`. `snippet_line` adds the LF count in the body before the snippet offset. CRLF counts as one physical line ending and two stored code points. A BOM counts as one code point and adds no line. An unterminated last line counts once and a final LF does not create another physical line. For an empty body, report zero body characters and `body_line = file_lines + 1` as the insertion position, including when the delimiter lacks a final newline. Print only the path on that hit's locator, followed by the bracket with its body and file sizes and count. Omit `snippet_line`, `snippet_offset` and `section` when there is no snippet. File and body facts come from the same guarded scan as the hit. They describe the version read by search and can move after an edit.

Ordinary `get` prints the same body and file sizes in its header, without repeating a search count. For example, `body: L9, 48213C; file 913L/48517C`, with the four numeric JSON fields above beside its existing path and offset. The body stays as stored Markdown. For a nonempty slice, the footer adds the file lines containing its first and last printed code points, for example `[body 0 to 5411 of 48213; file L9–L118; more: --offset 5412]` and JSON `line_from` and `line_to`. Include them when both values are the same. Omit the range on an empty slice. These are physical file lines and continuation stays `--offset` in body code points. The bounds are part of the actual response budget and cost about 20 text characters once per read.

In `get --outline`, add the heading's 1-based `file_line` beside its existing body-code-point `offset` and `chars`. Text columns become `<offset>  <chars>  L<file_line>  <heading>`. A heading's offset opens ordinary `get`, while outline pagination continues to count sections. Do not add these search fields to `list`, `prime`, related rows or history rows. The spec's description of list JSON as the search shape without snippets needs to preserve the current list shape explicitly.

The two character sizes have distinct names. @lead-dev recommends exact whole-file characters so the reported item size has both units. @architect-2 accepts the field. @architect and @lead-dev-2 prefer file lines and body characters alone. All sign the proposal with this choice left to the owner. Dropping `file_chars` saves 17 JSON code points per hit in the example and removes the second character total.

## 2. Occurrences

Use the query parser's distinct clauses and the same field-local occurrence rules as BM25. Sum occurrences of each clause in the title, each tag and the stored body. A phrase cannot span fields. An overlapping phrase counts at each start. A word clause and a prefix clause can both count one token because they are different clauses. Repeating the same query clause does not double its contribution. Compute only for hits being considered for the page and reuse the snippet scan where possible. A semantic result can legitimately have zero. The label is `occurrences` because these are lexical clause occurrences across fields. It does not mean that many file locations. Keep `partial m/n` as the separate distinct-clause coverage marker. The number has no effect on ranking, admission or abstention, and is never collected for a hidden item.

## 3. Opt-in neighboring lines

Add `search --context N` for **1 to 3 neighboring physical body lines per side**. The focal excerpt is still the exact selected stretch. Split it on its real line breaks and prefix each line with `    <file_line>: `. Show up to N body lines immediately before and after the stretch's lines. Every printed line has the prefix. Neighboring lines are cut at 160 code points including an ellipsis. The focal excerpt keeps its entire selected span, even in a 50,000-character paragraph. Normalize controls in each displayed line as with other snippet text. Front matter never becomes snippet context.

Fit the same base rows, their flattened snippets and the actual footer first. Then try an optional context block for each shown hit in page order, before ordinary related extras. Cap an entire context block at 600 printed code points, including its prefixes or JSON field and escaping. At either the per-hit cap or the remaining page budget, drop outer neighbors first, dropping the later line on a distance tie. If the focal lines with their prefixes alone still cannot fit, retain the default flattened snippet. In text a fitted block replaces the flattened snippet line. In JSON the existing `snippet` remains and the fitted block is `"context":[{"line":412,"text":"..."}]` in file order. Missing `context` means the default snippet was used. The locator always points to the original stretch, regardless of added context. Context cannot reduce the result count or raise its minimum. A caller that needs more can ask for fewer hits, a larger budget or another `get`.

Two 80-character lines on either side of five hits would cost about 1,600 code points before prefixes, even with short prose. The per-hit cap and fit order contain that cost without hiding admitted results. Defer a separate character-length flag until evidence shows longer focal excerpts help more than a follow-up `get`.

## 4. Terminal and JSON

- **Terminal.** Add explicit `--color` to `search` and `get`, with no TTY auto-detection or personal setting. It is a usage error with `--json`. Decorate only pbrain's structural labels. First fit the complete plain response, then add complete ANSI start/reset pairs to eligible labels in output order while the serialized response still fits. Count the escapes. This keeps row identity, offsets, notices and the absolute cap. A full page can get less colour. Store and print body text as Markdown without reflow or Markdown rendering.
- **Automation and future API.** `--json` remains compact, one object on stdout and scoped errors/notices on stderr as specified. Propose documenting the compatibility rule in §6.2 and testing fields, types, presence, units and exit behavior. Within a major CLI version, add only optional fields with documented meaning. Readers ignore unknown fields. An added field can affect fit and continuation. There is no per-row or mandatory envelope version now. A future network API can version its own transport and authorization contract. Keep the JSON evidence harness on the stream it measures.

The new output is built after admission and filtering. Empty pages gain no paths, sizes, occurrence counts or context. Their per-clause counts remain optional and scoped as before. Semantic and partial release gates still examine rendered pages. Run those gates again after the new mandatory fields land, because they can change which admitted rows fit.

## 5. Budget and a five-hit example

Keep `--limit 5` as a ceiling. With short IDs, short bodies and no related extras, the example below costs **1,214 text code points and 1,922 compact JSON code points**, counting the final newline. Its old short-hit shape would cost 677 and 1,055 respectively. That is about **107 more in text and 173 more in JSON per hit** here. The example fits at both the old 2,000-character default and the new 3,000-character default. Longer paths, more digits, escaped text and optional heading names can cost more, so the new budget gives more room for five hits without guaranteeing all five. A path and facts cannot be silently cut into a different address. Whole results and the reported minimum still apply.

**Decided:** the owner raised the default `--max-chars` to 3,000 and kept `--limit 5`. The fixture can still compare at 2,000 when it needs a tighter budget.

**Choice for the owner:** include `file_chars` as shown. For the smaller alternative, omit that field and the `/<file_chars>C` portion of the text file-size label. Keep `file_lines` and `body_chars` in either case. The five-hit example would be 1,189 code points in text and 1,837 in compact JSON without the field.

Plain `pbrain search retry timeout` on five short items:

```text
20261120-145000-retry-q7x2  decision  verified  2026-11-20
  Worker retry limits (decided by human:peter)
  docs/brain/decisions/20261120-145000-retry-q7x2.md:10  [body L10, offset 0/23; file 10L/161C; 3 occurrences]
  Retry twice on timeout.
20261120-144000-queue-d3k7  learning  verified  2026-11-20
  Keep retries bounded (by agent:team)
  docs/brain/learnings/20261120-144000-queue-d3k7.md:9  [body L9, offset 0/40; file 9L/158C; 2 occurrences]
  A retry after a timeout should back off.
20261120-143000-backoff-b4m8  note  provisional  2026-11-20
  Retry spacing (by agent:team)
  docs/brain/notes/20261120-143000-backoff-b4m8.md:9  [body L9, offset 0/44; file 9L/154C; 3 occurrences]
  A timeout triggers a retry after one second.
20261120-142000-jobs-f8s3  observation  provisional  2026-11-20
  Timed out jobs (by agent:team)
  docs/brain/observations/20261120-142000-jobs-f8s3.md:9  [body L9, offset 0/38; file 9L/156C; 2 occurrences]
  The retry recovered the first timeout.
20261120-141000-log-r9v2  note  provisional  2026-11-20
  Worker logs (by agent:team)
  docs/brain/notes/20261120-141000-log-r9v2.md:9  [body L9, offset 0/27; file 9L/135C; 2 occurrences]
  Log each timeout and retry.
5 results.
```

Same command with `--json`, indented here for reading. The sample sorts object keys. The existing key-order contract is unchanged and consumers identify fields by name.

```json
{
  "results": [
    {"author":"agent:team","body_chars":23,"body_line":10,"created":"2026-11-20T14:50:00Z","decided":"human:peter","file_chars":161,"file_lines":10,"id":"20261120-145000-retry-q7x2","kind":"decision","occurrences":3,"path":"docs/brain/decisions/20261120-145000-retry-q7x2.md","snippet":"Retry twice on timeout.","snippet_line":10,"snippet_offset":0,"status":"verified","title":"Worker retry limits"},
    {"author":"agent:team","body_chars":40,"body_line":9,"created":"2026-11-20T14:40:00Z","file_chars":158,"file_lines":9,"id":"20261120-144000-queue-d3k7","kind":"learning","occurrences":2,"path":"docs/brain/learnings/20261120-144000-queue-d3k7.md","snippet":"A retry after a timeout should back off.","snippet_line":9,"snippet_offset":0,"status":"verified","title":"Keep retries bounded"},
    {"author":"agent:team","body_chars":44,"body_line":9,"created":"2026-11-20T14:30:00Z","file_chars":154,"file_lines":9,"id":"20261120-143000-backoff-b4m8","kind":"note","occurrences":3,"path":"docs/brain/notes/20261120-143000-backoff-b4m8.md","snippet":"A timeout triggers a retry after one second.","snippet_line":9,"snippet_offset":0,"status":"provisional","title":"Retry spacing"},
    {"author":"agent:team","body_chars":38,"body_line":9,"created":"2026-11-20T14:20:00Z","file_chars":156,"file_lines":9,"id":"20261120-142000-jobs-f8s3","kind":"observation","occurrences":2,"path":"docs/brain/observations/20261120-142000-jobs-f8s3.md","snippet":"The retry recovered the first timeout.","snippet_line":9,"snippet_offset":0,"status":"provisional","title":"Timed out jobs"},
    {"author":"agent:team","body_chars":27,"body_line":9,"created":"2026-11-20T14:10:00Z","file_chars":135,"file_lines":9,"id":"20261120-141000-log-r9v2","kind":"note","occurrences":2,"path":"docs/brain/notes/20261120-141000-log-r9v2.md","snippet":"Log each timeout and retry.","snippet_line":9,"snippet_offset":0,"status":"provisional","title":"Worker logs"}
  ]
}
```

## 6. Proposed amendment scope

- **Functional spec:** §§6.1–6.3 for sizing, presence, compatibility and flag errors, §7.1 for hit shape and context, §7.2 for get and outline, §7.3 to keep the existing list projection, §12 for navigation guidance, §15 for acceptance cases and §16 for the rendered-page acceptance check.
- **Architecture:** §4.1 for file/body facts derived at the same scan, §7.4 for outline line numbers, §§8.4–8.5 for the snippet start and occurrence count, §9 for the optional block and decoration fit, §13 for parsing and §14 for tests.
- **Component contracts:** `item` and `store` compute and preserve coordinates from the approved file snapshot, `search` emits facts for the page, `render` fits text and JSON, `cli` registers the flags, and `skill` teaches the shortest next read. Extend the cross-command alignment fixture with LF, CRLF, BOM, empty-body and long-line cases, and test budgets, view isolation and identical paging with optional decoration. Update the evidence harness to keep measuring actual JSON responses. No amendment is applied here.

## Sign-off

- @lead-dev: drafter.
- @architect: agree with the proposal: one place per hit in both units, `get` continuing by `--offset` only, `occurrences` as BM25's own count, context that cannot lower the result count, and colour inside the absolute cap. Three additions requested, none of which changes the shape. (1) Say that `list`, `prime`, related lines and history rows are unchanged: no locator and no new fields. Spec 7.3 defines `list`'s JSON as "the `search` shape without snippets", so without this sentence it would inherit `path` and the sizes, at about 100 characters on each of 20 rows. (2) Let `get` say which file lines it printed, in its closing line and as `line_from` and `line_to` in JSON: about 20 characters once per call, and it gives an agent the file line on a continuation page too, where no search hit supplied it. (3) Put `file_chars` among the owner's choices beside the budget; I would leave it out, since nothing addresses a file by character and `body_chars` is the size that `get` pages through. Also a note for readers of the example: its JSON fields are in alphabetical order, which is not a proposal; the order stays that of spec 7.1. My preference for 3,000 is recorded correctly.
- @architect-2: Agree. I verified the five-hit example at 1,214 text and 1,922 compact JSON code points, including the newline, and checked all five body lengths and occurrence counts. I support the 2,000-character pilot and the separately named whole-file and body sizes. The empty-body and context-overflow cases preserve usable navigation.
- @lead-dev-2: agree. I checked the coordinate rules against item.md's `bodyAt`, the snippet start of search.md 4.5 and the outline's `Start`: one place per hit in both units, `snippet_line = body_line +` the LFs before the offset, and `get` continuing by `--offset` alone keep the agent's two addresses from disagreeing. The context rules cannot lower the result count or move the locator, and colour is fitted after the plain page, so paging is identical with and without it. On `file_chars` I side with @architect (leave it out; `body_chars` is what `get` pages through), but I sign either way. The owner's 3,000 default supersedes the 2,000 pilot named in the sign-offs above.
