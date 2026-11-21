# Review: components/render.md (@lead-dev-2)

Reviewed: `components/render.md` at sha256 866e36f925532bf18feb445cc9913da908d77e2b647c3f47673c88e4709f1330, against functional spec 2b9cedaf (6.1, 6.2, 7.1 to 7.5, 9.1, scenarios 2, 17, 28, 30 to 33, 36), architecture e2ce1217 (7.4, 8.4, 8.5, 9), and the seam in my `search.md` 627a0feb (`Hit`, `Snippet`, `Result`). Standards: `design_review_standards.md`.

**Verdict: changes required.**
- The fitter is right: whole rows, mandatory and optional passes, the per-state footer, minima, the get body adapter and the hybrid minimum rule of scenario 33.
- So are the seam use (`Result` passed through, `Hit.Section`, `Semantic`, notices left to cli), the outline mapping, list's reuse of `Filters.Admit`, and check's counting and exit rule.
- One issue would make scenario 31 fail (point 1). The rest are text forms the doc describes in prose but does not pin, which the coding agent would otherwise invent.

## Blocking

1. **The "…" must not eat the stretch.** "Account for added ellipses within the 200-code-point snippet limit" means trimming the stretch to make room for one or two "…". But the stretch is chosen to end exactly at the last matched clause when the match is late. Architecture 8.4's case is a sole match at 1000 to 1005, which gives position 805, with the match in the last five code points. Trimming two code points cuts the match out of the snippet, so scenario 31's later passage would no longer show `rollback`.
   - **Fix:** print the stretch whole, at most 200 code points from `search` (`Snippet.Text`), and add "…" outside it: at most 202 code points.
   - **Why that fits the spec:** 6.2's "snippet 200" is the stretch length of 7.1 ("the stretch of snippet length"), and architecture 8.4 says the stretch "is printed as it is … and '…' where the body continues". The JSON `snippet` value has the same form.
   - If you read 6.2 as a hard 200 including the ellipses, the alternative is that `search` uses a stretch of 198. Then tell me and @architect, because 8.4's example would change. I recommend the first.
   - **Test:** add the 1000-to-1005 case to `TestSearchFooterAndSnippet`: the printed snippet ends with the matched word and then "…".

## Moderate

2. **What `get`'s not-found suggestions are.** "The first three distinct visible IDs with the same folded prefix" doesn't say which prefix. Spec 7.2 says "ids in the command's view that start with the same text". Define it, for example:
   - candidates are the visible ids whose folded form starts with the folded argument;
   - if there are none, the visible ids that share the argument's longest leading run of 15 code points (the `YYYYMMDD-HHMMSS` part of a pbrain id);
   - order by folded id, then path; at most 3; each one passed through the secret filter.
   
   Whatever you choose, give a test where a mistyped full id (one changed random suffix) still finds its neighbour, and a personal id is never suggested in the shared view.
3. **Pin the exact text forms.** Each of these is described in prose, but the punctuation and labels are left open:
   - **Search, second line.** The spec's sample is `<title> (decided by <actor>; source: <source>)`. Give the forms with no source, with no actor, and with neither (`<title>` alone?). Also say that `by <author>` is used for non-decisions and for a decision with no decider.
   - **The `get` header.** Give each line's label and order: `title:`; `author: …  verified: …  decided: …` on one line, as in the spec's sample; `source:`; `replaces:`; `replacement:`; `current:`; `relates:`; `tags:`; `stale_after:`; `origin:`; `fetched:`; `unresolved:`; `path:`. Also say where `review due`, `personal` and `linked` sit on the second line.
   - **A `--history` line.** For example `<id>  <kind>  <status>  <date>  <title> (decided by <actor>; replaces: a, b)`.
   - **The footers of `list`, `outline`, `tags` and `check`.**
     - The spec shows `3 sections. 48 more: --offset 3` for outline (with no final period after the continuation, unlike `search`).
     - It shows `3 tags.` for tags.
     - The outside-window clause for `list`: does it say "matches" or "items"?
     - The check footer, e.g. `2 findings, 1 error. 5 more: --offset 2`.
   - **The `Alone` hints.** Give the exact punctuation for k entries: ` Alone: a 1, b 2.`, the period after the last entry shown. Since `Fit` re-measures the whole output for each k, say that the footer function renders the line for the current k.
   
   A table of literal templates, one row per line kind, would close all of these and lets the golden tests be written from the doc.

## Mild

4. **Where "long" comes from.** Use `Snippet.Long`, which `search` sets from `Request.LongBody`, the configured `get` limit (`pbrain.toml` `get.max_chars`, else 6,000). Don't recompute it in render, and have the search handler fill `LongBody` from configuration. The doc says "when the body is long" without naming the source.
5. **JSON key order.** Sorted map keys give `author` before `id`. That is valid and deterministic, but an agent skimming a cut transcript sees the id first in the spec's samples. Consider an ordered row type (a slice of key and value pairs) following the spec's field order. It costs nothing in measurement.
6. **Hints with a past-the-end offset.** You print no hints when `Total == 0` and `--offset > 0`. That is a defensible reading of "an offset past the end of them"; state it as a decision and test it. `search` fills `Alone` whenever `Total == 0`, so the choice is render's.
7. **Related lines on semantic hits.** Say that semantic hits get related lines like any result. Scenario 33 does not forbid it, and the rule is the same.

## Checked and fine

- **`State`, `More` and `Next`** come from the rows shown. An empty page has no continuation. A nonpositive budget is `usage`, and `budget_too_small` states the measured minimum.
- **`FitBody`** reserves the header, footer, link line and first code point. It then takes related lines, then the longest body prefix, cut on code points. The range line `[body a to b of n; more: --offset c]` matches the spec, and an offset past the end gives the header alone.
- **Hybrid:** the minimum comes from the output as asked, and the lexical row is unchanged at the new minimum (spec 7.1 at bca4e48a and later, scenario 33).
- **The search seam is used as I designed it:**
  - `Result.Total` and `Offset` are passed through;
  - the snippet edges come from `Start`, `End` and `BodyChars`;
  - `Hit.Section` is cut at 80;
  - the `semantic` marker comes first;
  - `replaced_matched` and `outside_window` come from `search` and are never recomputed.
- **Outline:** `Start` and `Tree - Start`, level marks, `(before the first heading)`, section-ordinal paging, an empty body with no rows, and no model or index.
- **List** reuses `Filters.Admit`, and outside-window counting matches `search`'s set table.
- **Prime and tags** follow 7.4 and 7.5, including personal-tag invariance.
- **Check:**
  - one finding per issue;
  - redaction through `secretscan`;
  - `dangling_link` names no target;
  - errors are counted before paging;
  - exit 1 with the report, also on an empty page.

## Recheck, sha256 263e69e7e4771428d74067437be6fe7720460c4b381f489bcb60a01f920b5971

**Verdict: approve.**

1. **The ellipses (the blocker).** The whole stretch of at most 200 code points is printed, and "…" is added outside it, in text and JSON, and budgeted. No character of the stretch is removed. The 1000-to-1005 case in `TestSearchFooterAndSnippet` keeps the match at the right edge.
2. **Suggestions for a missing id:** first the visible ids that begin with the whole folded argument; then, if there are none, the 15-character UTC-stamp prefix; deduplicated, sorted, filtered for secrets, at most 3. `TestMissingIDSafeSuggestions` is added.
3. **Literal templates for:**
   - the four variants of search's second line, and `decided by` versus `by`;
   - the `get` header's labels and order, and its markers;
   - the history row;
   - the search footer, including the replaced and outside-window phrases and the `Alone` punctuation;
   - the footers of list, history, outline (with no final period after the continuation, as in the spec), tags, check and prime.
   
   `TestModeTextTemplates` covers them.
4. to 7. **The mild points:**
   - `LongBody` comes from `get.max_chars`, and `Snippet.Long` decides the suffix;
   - sorted JSON keys are kept as a stated decision (acceptable);
   - there are no hints at a non-zero offset, tested;
   - semantic hits get related lines.

Consistency finding 4 is also met. `check.Collect(items, skipped, g, opt)` is a pure entry point: `Skipped` is projected by `cli`, `DisplayPath` is injected, and it returns the full sorted findings and the error count, while `cli` owns paging and the exit status. That keeps `check` importing only `item`, `graph` and `secretscan` (architecture 3).

## Review of the BM25 delta, sha256 6ee88823457c634ffe7ee0b81d7a2a01d05c1832d177dd99fd1fe647c2571251 (base 263e69e7)

**Verdict: approve.** I diffed it against 263e69e7: seven passages changed, all from @architect's list (message 290), spec 079b3d19 (7.1, scenario 38) and architecture d791e15e (9).
- **Markers.** `semantic` or `partial <matched>/<clauses>` comes first and never both, then `linked`, `review due`, `replaced`, `personal`.
- **JSON fields.** `"partial": true` with numeric `matched` and `clauses` appears on partial rows only, the numbers always printed there and nowhere else. `"semantic": true` appears only on semantic rows.
- **Minima.** The minimum is the command's as asked, for all three groups. The extended `TestHybridContinuationRaisesMinimum` covers exact plus semantic, exact plus partial, and exact plus both, in text and JSON. At the stated minimum the exact row and snippet are unchanged, `more` equals the number of added rows, `next` is 1, and paging at limit 1 visits the rows in tier order.
- **Exact-only diagnostics** are preserved: `replaced_matched` and `outside_window` are counts of exact matches, and the `alone` hints appear only when the whole merged list is empty at offset 0.
- **The seam with `search.md`** (at 5898a0f9, with the BM25 change) agrees. `Hit.Partial`, `Matched` and `Clauses` are read as given. The page comes from the merged list L, then S, then P. With the semantic tier unavailable, the exact and partial hits are fitted and the notice goes to stderr.

## Section-alignment row, sha256 fb4151b5de19e60a4f3d2b72b229f50afc609de4791db0bc42180be805a112a5 (base 6ee88823)

**Verdict: changes required.** All three are wording fixes to the one new row, `TestSearchGetOutlineSectionAlignment`; nothing else in render.md changes. I checked the row against:
- architecture 064c9b4f, section 14 "Section alignment", 7.4 and 8.4;
- search.md 0dc8b8c4, sections 4.5 and 6 step 7 (the snippet of an added hit);
- semantic.md `TestPassageAlignment`.

What is right: the real handlers run in both formats; the `get --offset` round trip compares against the snippet with line breaks flattened and the ellipses left off; the section is picked by parent and outline position, not by title; the fenced line is checked; and the boundary case uses the S + 10 end point that search.md now states.

1. **The item must be longer than `get.max_chars`, and the row must say so.** Position, size and section are printed only when `Snippet.Long` (search.md 4.5 step 4 and the `Long` field; render.md's search rows). On a body of 6,000 code points or fewer, every heading and offset assertion in the row finds nothing to check. Fix: the body exceeds the configured `get.max_chars` (or the test sets a lower one), and the test asserts that `[in <section>, at <offset> of <size>]` and the JSON `snippet_offset`, `body_chars` and `section` are present for every hit it checks.
2. **The semantic hit needs its own query, and the two hits must sit in the two sections that share a title.**
   - An item in the exact list L is never added again as a semantic hit (search.md 6). As worded ("an exact query … and use the test embedder and an index to force a semantic hit"), the semantic hit could be on the item the exact query already matched, which cannot happen. State a second query that does not match the item lexically.
   - "The second section" and "the first section" do not say whether these are the two same-titled sections. If they are not, the case architecture 14 exists for, a title that does not tell two sections apart, is never exercised. State that the exact hit is in one of the two same-titled sections and the semantic hit is in the other, so a lookup by title would name the wrong one at least once.
3. **"The offset lies in its printed span" does not identify the section when the section has subsections.** `get --outline` prints `Tree - Start`, which includes subsections, so a position in a child also lies in its parent's span.
   - Architecture 14 checks this with `outline.At`. Add `outline.At(outline.Parse(body), position)` equal to the fixture's intended index. Alternatively, assert that the position is before the next outline row's offset.
   - Keep the printed-span check: it is the user-visible half.

Nits:
- Give `get` a budget large enough to print at least the snippet's length from the offset, so the round-trip comparison cannot be cut short by fitting.
- For the boundary case, "enough room" means the preceding section starts at or before S − 190 and has a level above 0 and a title. The preamble names nothing.
- Add a cross-reference: the architecture row's "no passage starts at the fenced line" and "every passage lies inside one section" are hosted by semantic.md `TestPassageAlignment`, so the two rows together cover section 14.

## Section-alignment recheck, sha256 3f204821c80e18c86ac83bceee0d75ec3e34c5679e372210c27870d0dee35113

**Verdict: approve.** All three changes I asked for and the three nits are fixed, in the new "Section alignment fixture" section:
- **The body is longer than the configured `get.max_chars`.** Step 2 requires the `[in <section>, at <offset> of <size>]` suffix and the JSON `section`, `snippet_offset` and `body_chars` for both hits, so none of the checks can pass with nothing printed.
- **The semantic hit has its own query.** It is a query the item does not match in full, scoring above the cutoff, asserted as a marked semantic hit. The two hits are in the two repeated sections: the exact hit in the second, the semantic hit in the first. The exact hit ends at least 200 code points into its section, so its stretch starts inside that section. This agrees with search.md 4.5 step 3.
- **Section identity.** `outline.At(outline.Parse(body), position)` must equal the row's index, as well as the printed-span check.
- **Nits.** The `get` budget fits the raw snippet. The boundary case states a sole match, a titled preceding section of level above 0 starting at or before S − 190, and position S − 190, which matches search.md 0dc8b8c4. `TestPassageAlignment` is cross-referenced.

Nothing else in render.md changed.
