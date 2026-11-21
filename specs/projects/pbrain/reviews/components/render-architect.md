# render.md: review of the output amendment (@architect)

Reviewed `components/render.md` at sha256 05b6520116b858b5da2a303229ccc3961658327614b91aad27a3c0d8fc406c11 (base 3f204821) against functional spec d1ba2637 (sections 6.1, 6.2, 7.1, 7.2, 7.3, scenarios 39 to 41) and architecture ed3682b8 (sections 4.1, 8.4, 9, 14). I read the whole file, since the amendment touches most of it.

**Verdict: changes required. No blocker: four places where the printed bytes are not yet pinned, each one sentence.**

## Findings

1. **Should-fix: what the cyan mark covers (Search step 3, Get).** "Mark the locator cyan" can be read as the whole location line. Spec 6.2 colours "each `path:line` of a result and the value of `get`'s `path:` line". Say that the mark is the path token with its `:<line>` suffix when there is one, the path token alone on a result without a snippet, and nothing of the bracket; and that a withheld path (`?`) carries no mark.
2. **Should-fix: what `get --history` marks.** The outline's head line has the id in bold; history has no head line and says nothing. Spec 6.2 colours "the id that starts a result or a `get` response". My reading as the spec's writer, to state here: a history row is a result of that response, so the id that starts each row is bold; `list` has no `--color`.
3. **Should-fix: an empty context line.** The text form is given as four spaces, the line number, a colon, a space and the text. For a line with no text, say whether the space after the colon is printed. It should not be: `    414:`, so no line ends in a space; the JSON rule (number kept, empty `text` left out) is already stated.
4. **Should-fix: one occurrence.** The template prints `<occurrences> occurrences` for every count. Every other count pbrain prints has a singular form, and the shared footer section says "singular applies only to a count of one". Print `1 occurrence`, and `0 occurrences`. The spec shows no example with one; this file is where the rule belongs.

## Checked and right

- **Shared types.** `Doc`, `State`, `Unit` with `Alts`, `Part` with `Marks`, `Mark`, `Style`, `Format`, `Fit` and `FitBody` are architecture 9's. `BodyDoc.LineAt` takes one fresh `item.LineCursor.At` as a function, so `render` still imports only `errs` (architecture 3).
- **Fitting order.** Rows first and then frozen; then each unit's alternatives, richest first; then related rows; then hints; then, for `TextColor`, complete pairs in output order while they fit. Optional parts cannot change the minimum, the rows or the continuation. Matches architecture 9 and spec 7.1.
- **`get`.** The minimum holds the header, the closing line with its real line range, a linked item's line and one character; the range for a slice is the lines of its first and last printed character, from one forward cursor; an empty slice has no range and never asks; colour comes after the slice, lines and continuation are fixed. The `A\r\nB` case and the 512,000-character line are in `TestGetBodyAndLocatorMinimum`.
- **Location line and fields.** The line, the form without a snippet (an empty body, and a title-only passage on a body that is not empty, with real sizes), `0 occurrences` on a result found by meaning, the section only on a long snippet. JSON fields and presence match spec 6.2 and 7.1.
- **Exact paths.** Bare, or one escaped token used for both text and JSON through `json.RawMessage`, or withheld for a name that is not valid UTF-8, with `?` in the location line and no `path:` line in `get`; never U+FFFD in place of a byte. Scoped to search and get. The test has the real U+FFFD folder beside the invalid one.
- **Context.** Focal lines whole, neighbours cut at 160, the block counted with its prefixes and line feeds or with its field name and escapes, 600 as the cap, drop order, the fallback to the ordinary snippet, never for a result without a snippet.
- **Scope.** `list`, `prime`, history rows and related rows keep their projections (spec 7.3). Search fits at 3,000 by default.
- **Tests.** The alignment fixture runs with LF, CRLF and a byte-order mark; `TestContextAlternativesPreservePage` has the semantic row that takes a block's room; `TestColourFitsAfterPlain` has the stored escape sequence.

## Re-check

Reviewed `components/render.md` at sha256 27d3f1ba6e4f02968891c2b314d8d3c8bc83f912b8b00ed7a711b6efe661b40c. The delta from 05b65201 is seven lines, all on the four findings.

- **Finding 1: resolved.** The cyan mark covers only the path token and its line suffix when present, or the path token alone without a snippet; it excludes the bracket and is absent on a withheld path. In `get` only the path token is cyan; the `path:` label stays plain.
- **Finding 2: resolved.** The id that starts each history row is bold.
- **Finding 3: resolved.** A context line with no text prints up to its colon, with no space after it.
- **Finding 4: resolved.** `1 occurrence`, plural at every other count, zero included.

The tests name each case: `TestSearchLocationAndDefaultBudget` (one and zero occurrences), `TestContextAlternativesPreservePage` (the blank line), `TestColourFitsAfterPlain` (each history row, the path alone, the plain bracket and placeholder).

**Verdict: approve.**
