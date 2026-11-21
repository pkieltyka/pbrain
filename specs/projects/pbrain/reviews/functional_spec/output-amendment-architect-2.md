# Output amendment review

Reviewed `ad0a4f748f913ba898e99eb09cf5ea5226e0f09b07d6f340b72cfeeaf8f60d02` against [the signed proposal](../output-format/joint.md) and the owner's decisions. The five-hit search limit, 3,000-character default, whole-file characters, two coordinate systems, empty-body insertion line and scope of new fields agree. The fenced JSON example parses.

1. **Should-fix — §7.1 and scenarios 33, 38, 40.** “Never changes its own lines or fields” for an exact result cannot hold for optional context or colour when another tier joins the page. Suppose the exact row and its context fit, then a semantic row also fits in base form, consuming the room needed for context. The required fitter returns both rows and drops the first row's optional block. Promise stable identity, order and mandatory base projection for exact hits. Explicitly allow context, related lines and decoration to be refitted against the actual merged page. Test that combination with both text and JSON.
2. **Should-fix — §6.2, §8.2 and scenario 41.** “pbrain never detects a terminal” contradicts the existing stdin behavior of `add`. Limit the rule to stdout detection for colour. “Without the flag no sequence is ever printed” should likewise refer to renderer-generated sequences. An adopted valid UTF-8 body may already contain an escape sequence; `get` promises to preserve stored body content. Test a literal control sequence in a stored body separately from generated header decoration.
3. **Should-fix — §6.2, §7.1 and scenario 39.** Exact path reporting needs a rule for non-UTF-8 filesystem names. Linux permits an otherwise valid item under a directory containing an invalid UTF-8 byte. The scan validates the file's contents; Go's JSON encoder coerces invalid filename bytes to U+FFFD. That can even point the agent at a different existing path. Keep ordinary Unicode paths unchanged. For a path whose bytes are invalid UTF-8, specify a lossless, explicitly identified byte-escaped locator (or explicitly withhold filesystem navigation for that row and direct the agent to get by ID). Do not emit a lossy path as an exact address. Test it alongside a literal U+FFFD directory and the existing quoted path cases.

**Verdict: changes required.**

## Output amendment re-check

**Approve** `d1ba263730a163d1209dc9a9ab34c88b08aa732fac947c75eaeb950da748e177`. All three findings are resolved.

- Sections 7.1 and 15 preserve the required exact-hit fields. Scenario 40 gives the tight-budget case where a second hit takes the space previously used by context, in both text and JSON.
- Section 6.2 limits terminal detection to the decision about output colour. Scenario 41 preserves an escape sequence from the stored body. The stdin rule for `add` still applies.
- Search and get withhold a filesystem address with invalid UTF-8 and retain lookup by ID. Scenario 39 distinguishes that item from one under a real U+FFFD directory. Valid paths with controls have an exact escaped representation.
