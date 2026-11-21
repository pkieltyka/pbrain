---
status: complete
---

# Component: search (with query)

Packages `internal/query` and `internal/search`. Designed against the functional spec at sha256 d1ba2637 (sections 2.3, 6.1, 6.2, 7.1, 7.3, 15 scenarios 1 to 4, 17, 30, 31, 33 to 35, 38 to 40), architecture sha256 ed3682b8 (sections 3, 4.1, 4.4, 7.4, 8.1 to 8.5, 8.8, 13, 14), and `components/item.md` and `components/graph.md`.

The semantic seam is architecture 8.5, the partial tier architecture 8.8 and the outline package architecture 7.4 (sha256 ed3682b8), all used as given.

## 1. Purpose and scope

`query`:
- splits text into words;
- parses a search query into clauses;
- parses `<when>` values into a time window.

`search` runs one search over a corpus and a graph. It returns the ordered page with its counts and passages:
- eligibility (view, lifecycle and filters);
- matching and match classes;
- order, with the decision exception;
- the replaced and outside-window counts;
- the per-clause counts;
- the snippet stretch with its position and section;
- with `--semantic`, the merge of the results found only by meaning;
- with `--bm25`, the partial matches ranked by BM25 (section 4.6).

It also exports the filter logic `list` reuses.

Out of scope:
- **Rendering**: cutting, flattening, "…", brackets, the closing line, budgets, `more` and `next`. That is `render` (`render.md`).
- **Related lines.** The reading commands build them from the graph, for `search` and `get` alike (`render.md`), so `Hit` carries none. This departs from the wording of architecture 8.3 step 6 only in where the code sits.
- **Reading files** (`store`, through `Corpus`).
- **Embedding, the index and the cut-off value.** Those are the semantic component (@architect, `semantic.md`). `search` receives scored items through the seam.
- **Flag parsing.** `cli` builds the `Request`.

## 2. `query`

### 2.1 Interface

```go
package query

type Token struct {
    Word       string // folded with item.Fold
    Start, End int    // code points into the text, End exclusive
}
// Tokens returns the maximal runs of letters and digits (unicode.IsLetter || unicode.IsDigit), folded.
func Tokens(text string) []Token

// Scanner tokenises without allocating per token, for bodies: Next fills tok and returns false at the end.
// The folded word is valid until the next call.
type Scanner struct{ /* reusable buffers */ }
func (s *Scanner) Reset(b []byte)
func (s *Scanner) Next() (word []byte, start, end int, ok bool)

type ClauseKind uint8 // Word, Prefix, Phrase
type Clause struct {
    Kind  ClauseKind
    Words []string // folded; one for Word and Prefix, two or more for Phrase
    Label string   // as written, for the per-clause counts: a word, "run*", or "\"job queue\""
    Quoted bool    // written in double quotes (a one-word phrase included): required in a partial match (section 4.6)
}
// Parse: errors are *errs.Error{Code: "usage"}: "unclosed quote" or "the query has no words".
func Parse(q string) ([]Clause, error)

type Window struct {
    Since, Until time.Time
    HasSince, HasUntil bool
    UntilIncl bool // true for an instant or a span; false when Until is the midnight after a day
}
// ParseWindow parses the two flag values ("" = not given) at one instant in one zone.
// Errors: *errs.Error{Code: "usage"} naming the flag: "--since: not a date, instant, today, yesterday or span",
// or "--since is after --until".
func ParseWindow(since, until string, now time.Time, loc *time.Location) (Window, error)
func (w Window) Set() bool
func (w Window) Contains(t time.Time) bool // only for a known created; callers exclude unknown first
```

`query` imports `errs` and `item` (for `Fold`), as architecture 3 now orders them.

### 2.2 Words

- A word is a maximal run of runes for which `unicode.IsLetter` or `unicode.IsDigit` holds. Anything else separates words, so `ci-runner` is `ci` and `runner`, and `don't` is `don` and `t`.
- Each rune is folded by `unicode.ToLower`, which maps one rune to one rune, so offsets in the folded and the original text agree.
- Offsets count code points from the start of the text: the body's start for bodies, and the field's start for titles and tags.
- `Scanner` decodes with `utf8.DecodeRune`. Bodies are valid UTF-8 (the scan guarantees it). It folds into a reused buffer, so testing a body allocates nothing per word.

### 2.3 The query grammar

The query is the command's arguments joined with one space (`cli`). `Parse` reads it left to right:

1. **Spaces** (`unicode.IsSpace`) outside quotes separate terms.
2. **Phrases.** `"` starts a phrase and the next `"` ends it. With no closing quote, the result is `usage`, "unclosed quote".
   - The phrase's words are `Tokens` of its text; `*` inside a phrase is a separator.
   - Zero words contribute nothing; one word is a `Word` clause; two or more are a `Phrase`.
   - The label is the phrase as written, quotes included.
   - A quote in the middle of a term ends that term and starts a phrase, so `a"b c"` is the term `a`, then the phrase `"b c"`.
3. **Bare terms.** A term is a run of non-space characters outside quotes. If it ends in `*`, the term without its trailing `*`s is tokenised, and its last word becomes a `Prefix`. A `*` anywhere else is a separator. Every word of the term becomes a `Word` clause, except that last prefix.
   - `ci-run*` gives `Word ci` and `Prefix run`.
   - `*` alone and `--` give nothing.
   - Each clause's label is its word as written in the query, before folding; a prefix's label ends in `*`.
4. **Duplicates.** Two clauses with the same kind and words are one clause; the first one's label is kept, and `Quoted` is set if either occurrence was quoted. Counting a clause twice would print its count twice. So `"durable" durable` is one `Word` clause with `Quoted` set. `Quoted` changes nothing in exact matching or the per-clause counts; only the partial tier reads it.
5. No clause at all is `usage`, "the query has no words".

The grammar is pbrain's own; no library syntax is exposed. Operators such as `OR` and `-` are ordinary words or separators.

### 2.4 `<when>` and the window

`ParseWindow` reads each given value with one instant `now` and one zone `loc` (architecture 13). Days are local days, from one local midnight to the next.

| Value | `--since` (inclusive) | `--until` |
|---|---|---|
| `YYYY-MM-DD` (calendar fields read with `time.Parse("2006-01-02", v)` in UTC, then `dayStart` in `loc`) | that day's local midnight | the next day's local midnight, exclusive |
| `today` | today's local midnight | tomorrow's midnight, exclusive |
| `yesterday` | yesterday's midnight | today's midnight, exclusive |
| RFC 3339 (`time.Parse(time.RFC3339, v)`) | that instant | that instant, inclusive |
| `<N>h`, `<N>d`, `<N>w` | `now` minus N hours, N×24 hours or N×168 hours | that instant, inclusive |

- **The start of a local day** (`dayStart(y, m, d, loc)`) is the earliest instant whose civil date in `loc` is on or after `y-m-d`. It is not taken from `time.Date(y, m, d, 0, 0, 0, 0, loc)` alone, because where a zone skips midnight Go may resolve that call to the previous day. In `America/Sao_Paulo` on 2018-11-04 it gives Nov 3 at 23:00 (found by @lead-dev's review). A binary search over the instants is not right either: the civil date is not monotone across a midnight rollback. In `America/Goose_Bay` on 1988-10-30, midnight came first at 02:00Z, the clock fell back into Oct 29 at 02:01Z, and midnight came again at 04:00Z. A bisection can return the second midnight, so the date's first hour would be lost (also @lead-dev's review).

  The routine walks the zone's constant-offset segments with `time.Time.ZoneBounds`. Inside one segment the local clock is the instant plus a fixed offset, so the local date only rises.
  1. Let `T = time.Date(y, m, d, 0, 0, 0, 0, time.UTC)`, the target's civil midnight read as UTC. Start at `t = T.Add(-26 * time.Hour).In(loc)`; every zone offset lies within ±26 hours, so no earlier instant can qualify.
  2. For the segment holding `t`, with `start, end := t.ZoneBounds()` and offset `o` (from `t.Zone()`), the first instant of the segment whose local date is the target or later is `c = T - o`, raised to `start` and to `t` when it is earlier than either.
  3. If `end` is zero (the last segment) or `c < end`, `c` is the answer, after a check that its civil date in `loc` is not before the target.
  4. Otherwise continue from `t = end`. At most 64 segments are walked, and a ±26-hour window holds far fewer in any real zone.
  
  The answer is the earliest suitable instant:
  - a skipped midnight: the day starts at the first instant that exists, e.g. 01:00;
  - a repeated midnight: the earlier one;
  - a skipped day: `Pacific/Apia` had no 2011-12-30, so that day starts where 2011-12-31 starts and is empty.

  Checked with Go 1.27.3 on 2026-11-21:

  | Zone and date | `time.Date` at midnight | `dayStart` |
  |---|---|---|
  | Goose_Bay 1988-10-30 | 02:00Z | 00:00 ADDT = 02:00Z (the first midnight) |
  | Sao_Paulo 2018-11-04 | Nov 3 23:00 -03 | Nov 4 01:00 -02 |
  | Havana 2026-03-08 | Mar 7 23:00 CST | Mar 8 01:00 CDT |
  | Havana 2026-11-01 | 00:00 CDT | 00:00 CDT (the earlier of two midnights) |
  | Apia 2011-12-30 | Dec 29 00:00 -10 | Dec 31 00:00 +14 |
  | London 2026-03-29, Kolkata 2026-11-20 | 00:00 local | unchanged |
- **The next day's start** is `dayStart` of the civil date after `y-m-d`. The civil date is computed in UTC with `time.Date(y, m, d+1, 0, 0, 0, 0, time.UTC)`, which normalises the calendar date only. `today` is the civil date of `now.In(loc)`; `yesterday` is the civil date before it.
- **Spans:**
  - N is one to six ASCII digits; units are lower-case `h`, `d` or `w`;
  - `0d` is `now`;
  - `3x`, `-3d`, `3D` and `3 d` are `usage`;
  - the bound is `time.Unix(now.Unix() - N*u, int64(now.Nanosecond()))`, with `u` = 3,600, 86,400 or 604,800 seconds.
  
  The product is at most 999,999 × 604,800 ≈ 6.05 × 10^11 seconds, well inside int64, and no `time.Duration` is formed. Checked: `999999w` back from 2026-11-20 is the year -17139, whereas the `Duration` form gives 2151-11-03, in the future. A `Duration` overflows at about 292 years, and @lead-dev showed `999999 * (168*time.Hour)` comes out negative, which would turn the filter round. The 24-hour day and 168-hour week are kept.
- **Anything else is `usage`,** with the flag named and the value not echoed. Examples: `soon`, `2026-13-01` and a date with a time but no zone.
- **An inverted window.** Let the window's last included instant be `E = Until` when `UntilIncl`, else `Until - 1ns`. If `Since > E`, the result is `usage`, "--since is after --until".
  - `--since 2026-11-19 --until 2026-11-19` is that one day.
  - `--since 2026-11-20 --until 2026-11-19` is refused.
  - Both bounds on a skipped day (Apia 2011-12-30) are refused the same way, because that day holds no instant.

`Contains(t)` is `(!HasSince || !t.Before(Since)) && (!HasUntil || t.Before(Until) || (UntilIncl && t.Equal(Until)))`. An item with unknown `created` is outside every window that is set (spec 7.1), and callers test `CreatedKnown` first.

## 3. `search`: interface

```go
package search

// Corpus and Run are fixed (architecture 8.5).
type Corpus interface {
    Items() []*item.Item
    Body(*item.Item) ([]byte, error)
}
func Run(ctx context.Context, c Corpus, g *graph.Graph, req Request) (*Result, error)

type Filters struct {
    Kinds  []item.Kind  // any of; empty = every kind
    Tags   []string     // all of; normalised with item.NormTag
    Status *item.Status // the recorded status; nil = either
    Window query.Window
    All    bool         // replaced items too
}
type Verdict uint8 // Out, In, OutsideWindow
// Admit applies lifecycle and filters to one item. OutsideWindow means only the window excluded
// a current item, which feeds the outside-window count. list uses it as search does.
func (f *Filters) Admit(it *item.Item, current bool) Verdict

type Request struct {
    Clauses  []query.Clause
    Filters  Filters
    Limit    int       // 1..50, already checked by cli
    Offset   int       // >= 0
    Now      time.Time // the command's instant
    Snippet  int       // stretch length in code points: 200
    LongBody int       // get's default limit (pbrain.toml get.max_chars, else 6,000)
    Text     string    // the query as written, arguments joined by one space (architecture 8.5)
    Semantic Semantic  // nil without --semantic (section 6)
    BM25     bool      // --bm25: add the partial tier (section 4.6; architecture 8.8)
    Context  int       // --context: 1..3 lines each side, 0 without it (section 4.7; architecture 8.4)
}

type Result struct {        // architecture 8.5's fields, unchanged
    Total    int            // exact, found by meaning and partial together
    Offset   int
    Hits     []Hit
    Replaced int
    Outside  int
    Alone    []ClauseCount  // only when Total is 0
    Sem      SemanticState  // architecture 8.5; zero without --semantic
}

type Hit struct {
    Item     *item.Item
    Class    int  // 1, 2 or 3; 0 for a semantic or partial hit
    Replaced bool
    Semantic bool     // found only by meaning (architecture 8.5)
    Partial  bool     // a partial match (section 4.6); never together with Semantic
    Matched  int      // a partial match: how many of the query's distinct clauses it matches
    Clauses  int      // a partial match: the query's number of distinct clauses
    Section  string   // the heading named with the snippet: set only when Snippet.Long (section 4.5)
    Snippet  *Snippet // nil for an empty body, or an added hit with an empty span
    Occurrences int   // the query's distinct clauses in title, tags and body, by tf's rules (section 4.8)
}
type Snippet struct {
    Text      string // the stretch as stored: line breaks kept; render flattens and adds "…"
    Start     int    // code points into the body; the get --offset value
    End       int    // exclusive
    BodyChars int    // the body's length in code points
    Long      bool   // BodyChars > LongBody: Hit.Section is set
    Line      int    // the file line that holds Start (item.LineOf; section 4.7)
    Context   []ContextLine // with Request.Context > 0, in file order; nil otherwise (section 4.7)
}

// ContextLine is one line of a snippet's context (architecture 8.4).
type ContextLine struct {
    Line   int    // its file line
    Text   string // without its line feed and without a carriage return before it;
                  // for a line of the stretch, only the part inside the stretch
    Focal  bool   // a line of the stretch
    Before bool   // focal: its line starts before the part
    After  bool   // focal: its line goes on after the part
}
type ClauseCount struct {
    Label string // as written; render cuts it at 40
    Count int
}
```

Errors from `Run`:
- `ctx.Err()` when cancelled;
- `*errs.Error{Code: "io"}` when `Corpus.Body` fails, though with `KeepAll` it cannot;
- nothing else. An empty result is not an error.

## 4. `search`: the lexical run

### 4.1 Eligibility (`Admit`)

For each item of `c.Items()` (the command's view, in catalog order), with `current = g.Current(it)`:

1. **Lifecycle.** If the item is not current and `!All`, the item is outside the page but may still feed the replaced count (see the verdicts below).
2. **Kind.** `Kinds` is non-empty and `Header.Kind` is not in it: `Out`.
3. **Tags.** Every tag in `Tags` must be in `Header.Tags`; otherwise `Out`.
4. **Status.** `Status` is set and differs from `Header.Status`: `Out`. This is the recorded status; a legacy stamp reads as provisional.
5. **Window.** `Window.Set()`, and `!CreatedKnown` or `!Window.Contains(Created)`: the window alone excludes it.

`Run` needs three sets from one pass over the items: **eligible** (would be on the page), **replaced-in-window** (for the replaced count) and **outside** (for the outside-window count). Each item that passes steps 2 to 4 is classified:

| current | in window | `All` | set |
|---|---|---|---|
| yes | yes | any | eligible |
| yes | no | any | outside |
| no | yes | yes | eligible, and replaced-in-window |
| no | yes | no | replaced-in-window only |
| no | no | any | none |

`Admit` returns `In` for eligible, `OutsideWindow` for outside, and `Out` otherwise. `Run` computes replaced-in-window from the same steps. A replaced item outside the window is counted nowhere: "the replaced count covers the window only" (spec 7.1).

### 4.2 Matching

For each item in the union of the three sets, `matchItem` computes `perClause []Fields` (`InTitle | InTags | InBody`):

- **Title.** The tokens of `Header.Title` (after its fallbacks).
- **Tags.** The tokens of each tag separately, since a phrase cannot span two tags. `redis-cache` matches the phrase `"redis cache"`; the tags `[redis, cache]` do not.
- **Body**, only when title and tags leave a clause unmatched. Tokens come from a `Scanner` over `Corpus.Body(it)`, stopping as soon as every clause has matched somewhere.
- A **Word** clause matches a token equal to its word. A **Prefix** clause matches a token that starts with its word. A **Phrase** clause matches k consecutive tokens of one field, equal to its k words in order. Each field keeps a ring of its last k folded tokens, compared at each new token.
  - **Ring slots own their bytes.** `Scanner.Next` returns a scratch slice that the next call overwrites, so the ring never stores that slice. Each worker owns K reusable `[]byte` slots (K is the longest phrase's length) and copies each token into the next slot (`slot = append(slot[:0], word...)`), so no allocation happens once the slots have grown. A ring of borrowed slices would make every retained token look like the latest one.
  - With no phrase clause, no ring is kept.
- **Lookup structures**, built once per request: a map from word to the indices of the Word clauses, the list of prefixes, and the list of phrases. Each token costs one map lookup, plus a prefix test per Prefix clause and a ring compare per Phrase clause.

`ok` is true when every clause has a non-zero `Fields`. The class is 1 when every clause has `InTitle`, 2 when every clause has `InTitle | InTags`, and 3 otherwise. Title and tags are always tested in full, so the class is exact. The body is never needed to decide class 1 or 2.

**Parallel matching.** Bodies are tested on `runtime.GOMAXPROCS(0)` workers. Each worker takes item indices from a shared counter and writes its result into that index's slot, so worker timing never changes a result. Each worker owns its `Scanner` and rings. `ctx` is checked between items.

### 4.3 Order and the decision exception

**The sort key** of a matching eligible item, compared in this order:

1. class, ascending;
2. recency band, ascending. With `age = Now.UTC() - Created.UTC()`: unknown `created` is band 3; a future `created` is band 0; `age < 30×24h` is 0, `< 180×24h` is 1, `< 365×24h` is 2, and anything else 3;
3. `Header.Standing()`, descending;
4. `Created`, descending, with unknown last;
5. `Key`, ascending (byte order of the folded id);
6. `Path`, ascending.

**The steps:**

1. Sort the current matches by this key: the list C.
2. **The decision exception.** If C is empty, for example a page of replaced matches only with `All`, there is no exception. Otherwise let `best` be C[0]'s class. Find the first item in C whose class is `best` and that is a **decision in force**: current, `Kind == Decision` and `Decided` set. If it exists and is not already first, move it to the front; everything else keeps its relative order.
3. With `All`, sort the replaced matches by the same key (list R) and append them. They never take part in the exception.
4. `Total = len(C) + len(R)`. The page is the ordered list from `Offset`, at most `Limit` items. An `Offset` at or past `Total` gives an empty page; `Total` is unchanged.

### 4.4 Counts

- `Replaced`: matches in replaced-in-window, whatever `All` is. It is the same number on every page.
- `Outside`: matches in outside, undated items included. Zero when no window is set.
- `Alone`: only when the whole result list is empty (no exact match, no semantic hit and no partial match, section 6). For each clause in query order, the number of eligible items whose `perClause` for that clause is non-zero. With no full match, no body test stopped early, so every mask is complete. Replaced items without `All`, and outside items, are not eligible and do not count: "the counts use the search's view, lifecycle and filters". The list has every clause; `render` shows at most eight, while they fit.

### 4.5 The snippet stretch

Computed only for the items on the page, with `L = req.Snippet` (200) and `n` the body's length in code points. An empty body gives `Snippet == nil`.

1. **Occurrences.** Scan the body once with a `Scanner` and record every occurrence of every clause as `(start, end, clause)` in code points:
   - a word or prefix occurrence is its token;
   - a phrase occurrence runs from its first token's start to its last token's end.
   
   An occurrence longer than L can never lie wholly inside a stretch and is dropped.
2. **No occurrence** (the item matched in its title or tags only, or only through over-long phrases): the stretch starts at 0.
3. **Otherwise, the best coverage.** An occurrence o lies wholly in the stretch starting at s exactly when `o.end - L <= s <= o.start`. Coverage can only rise at `s = max(0, o.end - L)`, so the candidates are those values, sorted and deduplicated, with 0 added.
   - Sweep the candidates in ascending order. Keep the occurrences sorted by end, adding each one once `o.end <= s + L`, and the same occurrences sorted by start, removing each one once `o.start < s`.
   - Track, per clause, the number of occurrences inside, and the number of distinct clauses with a non-zero count.
   - The first candidate reaching the maximum distinct count is the stretch start `s`. The whole sweep is O(m log m) for m occurrences.
   
   For a body of n ≤ L there is one stretch, `s = 0`. Candidates are also capped at `max(0, n - L)`; no occurrence needs more, since `o.end <= n`.
4. **The snippet:** `Start = s`, `End = min(n, s + L)`, and `Text` the body's code points in `[Start, End)`. `BodyChars = n`. `Long = n > req.LongBody`.
5. **The section.** When `Long`, `secs := outline.Parse(body)` and `i := outline.At(secs, Start)` (architecture 7.4). `Hit.Section = secs[i].Title` when `i >= 0`, `secs[i].Level > 0` and the title is not empty; otherwise it is `""`. The preamble and a body with no heading therefore name none. `render` cuts it at 80. Only `Long` page items are parsed.

This reproduces architecture 8.4's example: a sole match at code points 1000 to 1005 with L = 200 gives the candidate 805, so `Start = 805`. A stretch may begin inside a word.


### 4.6 Partial matches, with `BM25` (architecture 8.8)

**When it runs.** Only when `req.BM25` is set. Without it nothing in this section runs: matching stops early (section 4.2), no frequency is counted, and the output is today's.

**Reading every body.** With `BM25`, the early stop of section 4.2 is off. Every eligible body is tokenised to its end, on the same workers, and for each item the matcher records three things:
- `perClause` masks, as today;
- `tf[c]`: the occurrences of each clause in the title, each tag and the body. A word occurrence is an equal token, a prefix occurrence a token that starts with it, and a phrase occurrence a start position where its words follow one another inside one field (overlaps count: `"a a"` occurs twice in `a a a`);
- `len`: the number of words in the title, all tags and the body.

Items in the outside and replaced-in-window sets that are not eligible are tokenised only as far as today, since their counts never enter the corpus.

**The corpus** is the eligible set of section 4.1 (view, lifecycle, filters, window); N is its size.
- `df[c]`: the number of eligible items whose mask for c is non-zero.
- `avg`: the mean of `len` over the eligible items, or 1 when N = 0 or every length is 0.

So a shared search never counts a personal item, and `--kind` or a window changes the statistics with the set it ranks.

**Clauses.** The query's distinct clauses in first-occurrence order (section 2.3 already merges duplicates); n is their number. A phrase or a prefix is one clause. For an item, m is the number of clauses with a non-zero mask.

**Admission.** An eligible item is a partial match when all of these hold:
1. it is not an exact match (some clause has a zero mask);
2. it matches every `Phrase` clause, every `Prefix` clause and every `Quoted` clause. A word written in quotes, which section 2.3 turns into a `Word` clause, stays mandatory: `"durable" queue retry` never admits an item without `durable`, whereas `durable queue retry` may;
3. it matches at least `partialMinInformative` (5) informative clauses, where clause c is informative when `partialDFDivisor × df[c] <= N` (4 × df ≤ N; a clause with df 0 is informative);
4. it matches at least half of the informative clauses: `partialShareDivisor × matchedInformative >= informative` (2 × matched ≥ total).

The three named constants (`partialMinInformative = 5`, `partialDFDivisor = 4`, `partialShareDivisor = 2`) sit together in `search`, with a comment that they are set on the calibration half of the fixture; the minimum started at 2 and calibration raised it to 5 (phase 18). A query of five clauses or fewer can have no partial match, and neither can any query when N < 4: a matched clause then has df ≥ 1, so 4 × df > N.

**Score**, in `float64`, summed over the clauses in query order:

```
idf(c) = ln(1 + (N − df[c] + 0.5) / (df[c] + 0.5))
s     += idf(c) × tf[c] × 2.2 / (tf[c] + 1.2 × (0.25 + 0.75 × len / avg))
```

A clause with `tf = 0` adds nothing. The score is computed only for admitted items and is never printed.

**Order of the partial list P.**
1. Score descending, compared as computed. An exact tie, which happens for identical counts, goes on to the next key.
2. Then `Created` descending, unknown last.
3. Then `Key`, then `Path`.
4. Current items first; with `All`, replaced partial matches follow in the same order.

Each partial hit has `Class 0, Partial true, Matched m, Clauses n`. As for every group, a snippet stretch is chosen only for the hits on the requested page, by the rule of section 4.5, over the clauses the item does contain.

**Statistics and candidates are separate.** N, df and avg always come from the whole eligible set, whatever other groups hold. Removing the items already in L or S (section 6.1, step 5) only removes output candidates: it never recomputes a statistic. An item found by meaning therefore cannot change a partial row's admission or score, only whether that item is listed again.

### 4.7 The snippet's line and context lines

Both are computed for the page's hits that have a snippet, after `Start` and `End` are fixed, so they never change which items are listed or where a stretch begins.

**The line.** `Snippet.Line = it.LineOf(body, Start)` (`item.LineOf`, `components/item.md` 3.7): the file line that holds the stretch's first character. A stretch at offset 0 is on `Layout.BodyLine`. Every snippet has it, short body or long; only `Hit.Section` still waits for `Long`.

**Context lines**, when `req.Context` is 1 to 3 (`cli` refuses any other value). A line of the body is the run of characters up to a line feed, or up to the end of the body; a final line feed starts no new line, and nothing before the body is ever a line. For a stretch `[Start, End)` with `End > Start`:
1. **Focal lines** run from the line that holds `Start` to the line that holds `End - 1`, so a stretch that ends on a line feed adds no empty line. Each focal line's `Text` is the part of that line inside the stretch, without the line feed and without a carriage return just before it; a carriage return anywhere else is a character of the line and stays, for `render` to print as a space. `Before` is set when the line starts before `Start` (the first focal line only). `After` is set when the line has a character other than its terminator (the line feed, or a carriage return just before it) at `End` or later (the last focal line only), since `End` is exclusive: over `abc`, the stretch `[0, 2)` has `Text` `ab` and `After` set.
2. **Neighbouring lines** are up to `req.Context` whole lines before the first focal line and after the last, fewer at the start or end of the body. Their `Text` is the whole line without its line feed and without a carriage return just before it; other carriage returns stay.
3. **Line numbers.** The first focal line's `Line` is `Snippet.Line`. The others count up and down from it, one per line feed, so no second walk of the body is needed.
4. `search` cuts and drops nothing and maps no control characters. The 160-character cut, the 600-character cap and the drop order are `render`'s (architecture 9). `Snippet.Context` lists the lines in file order and is nil when `req.Context` is 0.

All of it comes from one walk outwards from the stretch over bytes the scan holds: back at most `req.Context + 1` line feeds before `Start`, forward at most as many after `End`. Neighbouring lines are copied whole, so a 512,000-character neighbour costs one copy of it per hit; that is the rare case the fitter cuts. A hit without a snippet has no line and no context.

### 4.8 Occurrences

`Hit.Occurrences` is computed for each hit on the page, exact, found by meaning or partial, and for no other item. It is the sum, over the query's distinct clauses (section 2.3 merges duplicates, so a clause the query repeats counts once), of that clause's occurrences in the title, in each tag and in the body. The rules are those of `tf` in section 4.6: a word is an equal token, a prefix a token that starts with it, a phrase a start position where its words follow one another inside one field, overlaps counted. A phrase never spans the title and a tag, two tags, or a tag and the body.
- With `req.BM25`, section 4.6's pass has already counted `tf` for every eligible item, the off-page ones included, because the statistics need them. `Occurrences` is set from that `sum(tf[c])` on the page's hits only, with no second count.
- Without it, the matching pass of section 4.2 is unchanged, early stop included, and counts nothing. The page's count is separate work: a lexical or partial page hit reuses the body occurrences of section 4.5 step 1, counted before the over-long ones are dropped, plus a scan of its title and tags. A hit found by meaning, which section 4.5 does not scan, gets the same scan of title, tags and body. A meaning-only hit can have 0.
- `Occurrences` orders, admits and suppresses nothing, and `Alone` is unaffected.

## 5. Speed

- **No extra reads.** `Run` works on the bytes the scan already holds (`KeepAll`).
- **Matching.** Body work is one tokenising pass per candidate, stopping early on a full match, so a common word over 5,000 items stops early on most bodies. Architecture 8.6's throwaway measurement (54 to 58 ms on 16 workers for 57 MB, tokenising and two words) is the upper bound for this step.
- **The other steps.** Sorting and passages are O(k log k) over the matches and the page. Outlines are parsed only for `Long` page items. The snippet's line, context lines and occurrence counts are computed for the page only, at most the limit of items: `LineOf` reads a body up to its stretch once, the context walk stays within a few lines of it, and occurrences reuse the snippet's body scan (sections 4.7, 4.8).
- **With `--bm25`.** Every eligible body is tokenised to its end (section 4.6). Architecture 8.6's scratch program measured a full tokenising pass at 92 ms with 4 workers and 54 to 58 ms with 16, for 57 MB and two words. That is an estimate for one workload, not a bound: phrases, prefixes and the frequency counts add work, and the built command is what gets measured. Admission and scores are O(N × n). The built command with the flag is measured against the same 300 ms target (spec 16).

## 6. The merged list: exact, found by meaning, partial

The seam types are architecture 8.5's (sha256 ed3682b8), used exactly as given: `Request.Text`, `Request.Semantic`, `Semantic`, `Scores`, `SemanticHit`, `NotReady`, `Result.Sem` (`SemanticState`), `Hit.Semantic` and `Hit.Section`. `search` does not import `semantic`; `cli` passes the implementation in `Request.Semantic`.

### 6.1 Steps

1. Run the lexical search of section 4 in full, unchanged. The ordered list is L: current matches with the decision exception, then replaced matches with `All`. With `req.BM25`, the same pass also collects the counts of section 4.6.
2. If `req.Semantic == nil`, `Result.Sem` is zero and S is empty: go to step 5. Otherwise set `Sem.Asked = true`.
3. **Call `Score` once,** with `req.Text` and the **eligible** set of section 4.1, in catalog order. Matched or not, an item there passed the view, lifecycle, filters and window in the same code as the lexical part. Only ineligible items are left out:
   - outside items are never compared;
   - a replaced item in the window is compared only with `All`, when it is eligible, which is the overlap of the table in 4.1;
   - without `All` a replaced item is never compared.
   - **`ctx` cancelled:** `Run` returns `ctx.Err()`.
   - **Any other error:** `Sem.Ready = false`, `Sem.Reason = ModelFailed`, and S is empty. A search never fails for want of its semantic part (spec 7.1); with `BM25` the partial matches still follow.
   - **`Scores.Ready == false`:** `Sem.Ready = false`, `Sem.Reason = Scores.Reason`, and S is empty.
   - **A `Cutoff` that is NaN or infinite:** the scorer's metadata is invalid. The result is `Sem.Ready = false` with `Reason = ModelFailed`, and S is empty, so a faulty scorer can never get past the threshold.
   - **Otherwise:** `Sem.Ready = true` and `Sem.Missing = Scores.Missing`.
4. **The added results.** `Scores.Hits` holds one hit per eligible covered item whose closest passage is at or just under the cut-off: the source asks its query for scores of at least `Cutoff` minus a small margin (architecture 8.7), and items below that are simply absent. `search` still applies `Score >= Cutoff` exactly. Hits are dropped if any of these holds:
   - the score is not finite (NaN or ±Inf) or below `Cutoff`;
   - the item is in L (a set of L's item pointers);
   - the item was not in the eligible set;
   - the item appears a second time (the first kept);
   - the span lies outside the body as scanned: `Start < 0`, `End < Start`, or `End > BodyChars`.
   
   The rest are ordered:
   - score descending;
   - then `Created` descending, unknown last;
   - then `Key`, then `Path`;
   - current items first, then, with `All`, replaced items in the same order.
   
   This is the list S; each hit has `Class 0, Semantic true`.
5. **The partial matches.** With `req.BM25`, build P by section 4.6. Its statistics come from the whole eligible set; its candidates are the admitted items that are in neither L nor S. An item appears once, in the earliest group that has it.
6. **The page.** The result list is L, then S, then P. `Total = len(L) + len(S) + len(P)`, and the page is cut from that list with `Offset` and `Limit` (spec 7.1: one list for `more`, `next` and `--offset`). The decision exception has already been applied to L alone, and `Replaced` and `Outside` describe L only. `Alone` is computed only when `Total == 0`, that is, when all three groups are empty.
7. **Snippets of added hits on the page.**
   - The snippet comes from inside the passage only: `Start = SemanticHit.Start`, `End = min(SemanticHit.End, Start + L)`, and `Text` is those code points. A short section never quotes the next one.
   - An empty span (`Start == End`: an item that matched by its title alone) has no snippet.
   - `Long` and `Hit.Section` follow section 4.5; `Line`, and `Context` when asked, follow section 4.7.
   - Every hit on the page, added or not, gets `Occurrences` by section 4.8.

`cli` turns `Result.Sem` into notices:
- `Asked` and not `Ready`: `semantic_not_ready`, with the reason and its fix;
- `Ready` with `Missing > 0`: `semantic_partial`, with the count.

`search` prints nothing.

### 6.2 Sameness and order

L is computed before `Score` is called and before P is built, and neither touches it, so each exact result's row and snippet are identical with and without `--semantic` and `--bm25`, and so are the rows and snippets of the semantic results with and without `--bm25`. The related lines under any row are optional extras that the fitter may drop when the page is full, so they can differ between budgets, and between flags at the same budget, since a longer merged list's continuation can displace one. The fitted page around those rows can differ: with added results, the closing line carries `more` and `next`, and the minimum budget can rise, so a limit that held the lexical page can fail with `budget_too_small` (spec 7.1, scenario 33). That is `render`'s, from `Total`. Scores are compared as returned, and every tie is broken by the fixed keys of step 4. BM25 scores are summed in a fixed clause order over counts that depend only on the eligible set, and ties go to fixed keys. The order is therefore fixed for fixed files, index, model, query and flags (spec 7.1 "Sameness"). `--bm25` reads and writes no index, model or cache.

## 7. Dependencies

- **`query` uses** `item` (`Fold`) and `errs`.
- **`search` uses:**
  - `query`;
  - `item` (`Standing`, `Fold`, header fields, and `LineOf` with `Item.Layout` for a snippet's line);
  - `graph` (`Current`);
  - `outline` (`Parse`, `At`) for `Hit.Section`;
  - `errs`.
- **`cli` uses `search`** to build `Request` (with `Context` from `--context`), call `Run`, and print notices; and `Filters.Admit` and `query.ParseWindow` for `list`. `render` receives `Result` through the document builders of `render.md`.
- **The semantic component** implements `Semantic`; `search` does not import it.

## 8. Test plan

| Test | What it proves | Spec |
|---|---|---|
| `TestTokens` | Letters and digits, `ci-runner`, `don't`, CJK, digits inside words, offsets in code points with multibyte text, the fold (`İ`, `Σ`). | 7.1, 2.3 |
| `TestQueryUsesItemFold` | `query` tokens equal `item.Fold` of the same runs for a table of Unicode samples. | 2.3 |
| `TestScannerNoAlloc` | `testing.AllocsPerRun` of zero per token over a body. | 16 speed |
| `TestParseQuery` | Words, prefix (`cach*`, `ci-run*`), phrases, a one-word phrase (a `Word` with `Quoted`), `*` inside, quote in mid-term, duplicates merged (`Quoted` kept if either was quoted), labels as written, `""`, unclosed quote, only punctuation gives `usage`. | 7.1 |
| `FuzzParseQuery` | Never panics; every clause has at least one word; labels are substrings of the input. | architecture 14 |
| `TestParseWindow` | Every row of section 2.4; `today` one second after and before local midnight; `--until` day at 23:59 local; `TZ` changes the boundary; `soon`, `3x`, a start after the end; `--since d --until d` is one day. DST: a normal transition day in `Europe/London` includes its own items; `America/Sao_Paulo` 2018-11-04 (skipped midnight) starts at 01:00 local and excludes an item at 23:30 on Nov 3; `America/Goose_Bay` 1988-10-30 (midnight, rollback, midnight again) starts at the first midnight, 02:00Z, and includes an item at 02:00:30Z, in the first minute; `America/Havana` 2026-11-01 starts at the earlier of its two midnights; `Pacific/Apia` 2011-12-30 (skipped day) is empty, and both bounds on it are refused. Spans: `999999w` and `999999d` give bounds in the past, never the future; `1h` is exactly 3,600 s back. | 7.1, scenario 30 |
| `TestAdmitAndCountsSets` | The table of section 4.1 for every combination; undated items with and without a window; `--status` on a stamped note. | 7.1, scenario 30 |
| `TestPhraseRingOwnsTokens` | A `Scanner` whose scratch buffer is deliberately reused: the phrase `"élan vital"` matches only where the two words are adjacent, never "élan … vital" or "vital élan", with multibyte words of equal length. | 7.1 |
| `TestMatchFieldsAndClass` | Every clause in the title gives 1; title or tags gives 2; body gives 3; a phrase out of order in the title is not class 1; a phrase across two tags does not match; a prefix. | 7.1, scenario 3 |
| `TestOrderBandsAndStanding` | 29 days provisional above 31 days verified; standing within a band; a title match above a newer body match; unknown date last on a tie; the future in band 0. | 7.1, scenario 3 |
| `TestDecisionException` | With `All` and only replaced matches, there is no exception and nothing moves. A year-old decision and a week-old note match "job queue" equally: the decision is first. A current non-decision in a better class leads, with nothing moved. A better-matching replaced item under `All` changes nothing above it. | 7.1, scenarios 1, 2 |
| `TestLifecycleAndReplacedCount` | Scenario 1's A, B, C: only C; `Replaced` counts A and B; `All` appends them in order. | 4.4, scenario 1 |
| `TestWindowCounts` | `--since 7d` leaves out the year-old decision and counts it; the count is there with no result; `--until` only; undated counted. | scenario 30 |
| `TestPaging` | 12 matches, offset 4, limit 5 gives 5 hits and `Total` 12; offset 13 gives an empty page with `Total` 12; the counts are unchanged across pages; tied items keep the order at page size one. | 6.1, scenario 17 |
| `TestAloneCounts` | Zero results give per-clause counts in query order within view and filters; a word only in a personal item counts 0 in the shared view; a page with results, or an offset past the end, has `Alone` nil; a semantic hit suppresses `Alone`. | 7.1, scenarios 31, 33 |
| `TestSnippetStretch` | The sole match at 1000 to 1005 gives 805. A `queue` on line 1 and `queue rollback` later gives the later stretch. A phrase counts as one clause. No body occurrence gives 0. A multibyte character before the hit shifts by code points, not bytes. A body shorter than L gives 0. A hit at the start gives 0. A phrase longer than L is ignored. | 7.1, architecture 8.4, scenario 31 |
| `TestSnippetSection` | A `Long` body names the nearest heading at or before `Start`; the preamble and a headingless body give none; a short body never names one. The boundary case of architecture 8.4: in a `Long` body, a sole match ending at `S + 10`, where `S` is the start of a section, gives `Start = S + 10 - 200`, 190 code points before that heading (section 4.5 step 3). When the section before starts at or before `S - 190`, `Start` lies inside it and that earlier section is the one named. | 7.1, scenario 31 |
| `TestSnippetLine` | Every snippet, short body or long, has `Line`: the file line, in the stored file split at line feeds, that holds the character at `Start`; checked on one item stored with LF, with CRLF and with a byte-order mark before the front matter, on a match deep inside one long line, and on a title-only match (offset 0, the body's first line). `Hit.Section` is still set only when `Long`. An empty body and a meaning-only hit with an empty span have no snippet. | 6.2, 7.1, scenario 39 |
| `TestContextLines` | `Context` 0 gives nil. With 1, 2 and 3: the focal lines run from the line of `Start` to the line of `End - 1`; a stretch ending on a line feed, with LF and with CRLF, adds no empty focal line, and its next neighbour is the next real line; a stretch ending at a line feed that ends the body has no line after it; a carriage return just before a line feed never appears in `Text`, while one inside a line stays (`a\rb\n` beside `a\r\n`); `Before` and `After` mark a part that does not start or end its line, with `End` exclusive (`[0, 2)` over `abc` has `After` set, and a stretch ending just before a `\r\n` has it clear); neighbours are whole lines, fewer at the body's start and end, and never the front matter; line numbers equal `LineOf` at each line's first character. A meaning-only hit's lines come from its passage-clipped stretch. Nothing is cut: a 512,000-character neighbour comes back whole. | 7.1, scenario 40, architecture 8.4 |
| `TestOccurrences` | Against counts made by hand: a word twice in the title and three times in the body gives 5; a phrase that overlaps itself counts at each start; a query that repeats a clause counts it once; a phrase across two tags or the title and a tag counts nothing; a meaning-only hit with no clause anywhere gives 0. With `BM25` on the same items, each page hit's `Occurrences` equals the sum of the `tf` that 4.6 scores with, and no page-level count runs (a counter on the page count stays at 0); off-page items have `tf` but no hit carries it. Without `BM25`, the page count runs once per page hit and for no other item (a counter on that separate count alone, not on the matching pass). | 7.1, scenario 39, architecture 8.4 |
| `TestParallelDeterminism` | The same corpus with 1 and 16 workers, and shuffled worker timing, gives identical results. | 6.1 "order is fixed" |
| `TestSemanticMerge` | With a fake `Semantic`: lexical hits first and unchanged byte for byte; hits that matched lexically never added twice; a NaN or infinite score, a score below `Cutoff`, a span outside the body and a duplicate hit are dropped; a NaN `Cutoff` gives `ModelFailed` with the lexical list; with `All`, a replaced item in the window that matched only by meaning is added after the current added hits; an added snippet never passes the passage's `End`, and an empty span gives no snippet; `Score` receives only the eligible set, which excludes replaced items without `All`, items outside the window and kinds `--kind` excludes; score order with ties by date, id and path; paging across the joint at page size one visits each once; `Alone` only when all three groups are empty; `Scores.Hits` just under `Cutoff` are dropped by `Score >= Cutoff`. | 7.1, scenario 33 |
| `TestPartialAdmission` | Scenario 38's fixture (at least 20 items): `queue retry timeout backoff worker lease` with no item holding all six admits the items with five of them and never one with only four. A phrase or prefix clause an item lacks keeps it out. A word held by more than a quarter of the eligible items does not count towards the five informative clauses or their half. A query of five clauses or fewer, and any query with N < 4, admits nothing. A question sharing only common words gives no partial match and an empty list with `Alone`. The three constants are the only thresholds (a source scan finds no other literal). | 7.1, scenario 38 |
| `TestPartialScoreAndOrder` | Scores equal a hand-computed BM25 (the formula of 4.6, tf over title, each tag and body, a phrase within one field, overlapping phrase occurrences counted) to the last bit on a small fixture. Higher score first; ties by date, id, path; with `All`, current before replaced; `Matched` and `Clauses` set; `Class` 0. | 7.1, scenario 38 |
| `TestPartialQuotedWord` | Paired queries on one fixture: `durable queue retry` admits an item holding `queue retry` without `durable`; `"durable" queue retry` does not. `"durable" durable` parses to one clause with `Quoted` set. Exact matching, the order and the per-clause counts are the same for the quoted and bare forms. | 7.1, scenario 38 |
| `TestPartialStatisticsIgnoreOtherGroups` | At N = 4, with `df(queue) = df(retry) = 1` and a disjoint item found by meaning, the partial row is admitted and scored the same with and without that semantic hit; only the merged order and the deduplication differ. | 7.1, scenario 38 |
| `TestPartialCorpusIsTheView` | df, N and avg come from the eligible set only: the shared result is identical before and after a personal item holding the same words is added; `--kind` and a window change the statistics with the set. | 5.2, scenario 38 |
| `TestMergedListThreeGroups` | Exact, then found by meaning, then partial, with no item twice (an item both semantic and partial is listed as semantic); exact rows identical with and without `--bm25` and `--semantic`; with the model not ready, exact and partial with `Sem` set; `Replaced` and `Outside` unchanged by the flags; the decision exception on L only; paging at a page size of one visits every row of the whole list once. | 7.1, scenarios 33, 38 |
| `TestBM25OffReadsAsToday` | Without `BM25`, the matching pass's body tokenising stops at the first full match (a counting `Corpus` sees the same bytes read as before), and that pass counts no frequency; the page's occurrence count of section 4.8 is separate and is checked by `TestOccurrences`. With it, every eligible body is read to its end. Neither reads an index, model or cache. | 16, scenario 38 |
| `TestSemanticNotReadyAndPartial` | Each `NotReady` reason returns the lexical results with `Sem` set (`Asked`, not `Ready`, the reason); an error from `Score` gives `ModelFailed`, and a cancelled context returns its error; `Missing` is passed through; `Score` is never called without `--semantic`, and `Sem` is then zero. | 7.1, scenarios 34, 35 |
| `BenchmarkRun5000` | The synthetic brain of architecture 8.6: common and rare words, a phrase, a prefix, `--all`, a window, each with and without `BM25`. Published with the whole-process timings. | 16 speed |
