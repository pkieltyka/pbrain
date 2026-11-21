---
status: complete
---

# Component: item

Package `internal/item`. Designed against the functional spec at sha256 d1ba2637 (sections 2.3, 2.4, 2.5, 3, 4.3, 4.6, 6.2, 8.2, 8.3) and architecture sha256 ed3682b8 (sections 4.1, 4.4, 5, 14).

## 1. Purpose and scope

`item` is the model of one stored item and everything that can be decided from one file's bytes alone:

- the types of architecture 4.1 (`Item`, `Header`, `Version`, `Layout`, `Section`, `Kind`, `Status`, `Origin`) and the effective standing;
- the layout of one file (spec 6.2): its lines and code points, the file line on which its body starts, and the file line of any body offset;
- the fold used for every comparison of ids and words (architecture 4.4);
- ids: making a new one, its slug, the date-time an id carries;
- front matter: finding the block, parsing it under pbrain's rules, the problems `check` reports about one file, title and date fallbacks;
- emitting a new item file, and validating the values a command is about to write;
- the `verify` line edit.

Out of scope:

- **Reading files and deciding what is a candidate file.** Size, UTF-8, links and ignore rules belong to `store`. `item.Parse` assumes the bytes are already valid UTF-8 and at most 4 MiB.
- **Anything that needs more than one item.** Links, currentness, duplicates and ambiguity are `graph` and `store`. The secret scan is `secretscan`, and `[tags]` declarations are `brain` and `check`.
- **Time-dependent output.** `review due` is computed here from an instant the caller passes, but the instant is the CLI's.
- **Which fields a command may set,** and the flag rules of `add` and `import`. Those are `cli` and `writes`. `item` validates the values' form only.
- **Bodies.** pbrain treats them as opaque. Beyond locating where the body starts, `item` reads a body only for the title fallback and to count line feeds and code points for the layout.

`item` does no I/O. It imports `errs`, `outline` (a pure leaf package with no pbrain imports), `go.yaml.in/yaml/v3` and the standard library.

## 2. Public interface

### 2.1 Types (architecture 4.1)

`Item`, `Version`, `Header` and `Layout` are exactly as in architecture 4.1, including `Item.Layout` and the doc comment that defines an item:

```go
// Item is one Markdown file of the brain: one piece of knowledge with its header, cited by its id.
```

It can be a decision, a learning, an observation, a note or a source; a `source` item stands for material from outside, as a copy or as a pointer, with or without a summary. The enumerations get these values and spellings:

```go
type Section uint8
const (
    Shared Section = iota
    Personal
)
func (s Section) String() string // "shared", "personal"

type Kind uint8
const (
    Decision Kind = iota
    Learning
    Observation
    Note
    Source
)
func (k Kind) String() string            // "decision", "learning", "observation", "note", "source"
func ParseKind(s string) (Kind, bool)    // exact lower-case spelling; ok=false for anything else

type Status uint8
const (
    Provisional Status = iota
    Verified
)
func (s Status) String() string          // "provisional", "verified"

type Origin uint8
const (
    OriginNone Origin = iota
    OriginFile
    OriginLink
    OriginURL
)
func (o Origin) String() string          // "", "file", "link", "url"
func ParseOrigin(s string) (Origin, bool)
```

`Decision` is the zero `Kind`, so a zero `Header` reads as a decision. `Parse` always sets `Kind`, and a test checks that no path leaves it at its zero value by accident (`TestParseKindAlwaysSet`).

### 2.2 Problems and parse outcomes

```go
// Problem is one finding about one file's front matter, for check.
// It never carries a field's value: check decides what a message may show.
type Problem struct {
    Code   string // "bad_field" | "missing_field" | "unknown_value" | "decision_without_decider"
    Field  string // the front matter key: "created", "kind", ...
    Line   int    // 1-based line in the file; 0 when the field is absent
    Column int    // 1-based; 0 when the field is absent
}

// ErrNoFrontMatter: the file does not start with a front matter block. Not an item (check: not_an_item).
var ErrNoFrontMatter = errors.New("item: no front matter")

// FrontMatterError: the block exists but fails pbrain's rules. The file is skipped (check: bad_front_matter).
type FrontMatterError struct {
    Reason FMReason
    Line   int // 1-based file line, 0 if unknown
    Column int
}
type FMReason uint8 // Unclosed, Syntax, NotMapping, DuplicateKey
func (e *FrontMatterError) Error() string // "front matter: <reason> at line N"; never quotes file content
```

These are plain Go errors, not `*errs.Error`. They never reach the command line as a failure: `store` turns them into `Skipped` entries and `check` into findings.

### 2.3 Functions

```go
// Fold is the one comparison fold (architecture 4.4): unicode.ToLower rune by rune,
// no normalisation. ASCII-only input takes a byte loop with no allocation when already lower case.
func Fold(s string) string

// SplitFrontMatter locates the block. It returns the YAML text between the delimiters,
// the file line on which that text starts, and the byte offset at which the body starts.
// ok is false when the file has no opening delimiter. An opening delimiter without a
// closing one returns *FrontMatterError{Reason: Unclosed}.
// importer uses it to strip a leading block from imported text with exactly the rule readers use.
func SplitFrontMatter(data []byte) (yaml []byte, yamlLine int, bodyAt int, ok bool, err error)

// Parse reads one file's front matter under pbrain's rules (section 3.2).
// id is the file stem: it feeds the created and title fallbacks.
// err is ErrNoFrontMatter or *FrontMatterError; with err == nil, h is complete.
func Parse(id string, data []byte) (h Header, bodyAt int64, problems []Problem, err error)

// FirstHeading returns the title of the body's first heading, ATX or setext, outside fenced code
// (the first outline.Parse section with Level > 0 and a non-empty title), or "".
// importer uses it for spec 8.4's title inference (first heading, then page title, then file name or URL).
func FirstHeading(body []byte) string
// HeadingTitle is the item title fallback of spec 2.4: FirstHeading(body), else the first
// non-blank line that is not a fence delimiter, trimmed; "" if neither. Items only.
func HeadingTitle(body []byte) string

// Methods on Header (in addition to Standing and Linked of architecture 4.1).
func (h *Header) Standing() int
func (h *Header) Linked() bool
// ReviewDue: a stale_after is present and now is at or past it, or the value did not parse.
func (h *Header) ReviewDue(now time.Time) bool
// LegacyStamp returns the stem a "superseded-by <file>" status names:
// the text after the prefix, trimmed, with folders and a final ".md" (folded) removed.
// ok is false when the status is not a stamp or the stem is empty.
func (h *Header) LegacyStamp() (stem string, ok bool)

// IDs
func Slug(title string) string
// NewID draws ids until used(Fold(id)) is false, at most 16 times.
// rnd supplies the random characters (cli.Env.Rand). now is converted to UTC.
// Errors: *errs.Error{Code: "io"} when rnd fails or 16 draws are all used.
func NewID(now time.Time, title string, rnd io.Reader, used func(key string) bool) (string, error)
// IDTime reads the leading "YYYYMMDD-HHMMSS" of an id as a UTC instant.
func IDTime(id string) (time.Time, bool)

// Writing
type Draft struct {
    Kind       Kind
    Title      string
    Created    time.Time // written in UTC, whole seconds
    Author     string
    Source     string
    Status     Status
    Verified   string
    Decided    string
    Replaces   []string // ids as they will be written
    Relates    []string
    Tags       []string // already normalised with NormTag
    StaleAfter string   // already normalised with ParseStaleAfter, or ""
    Origin     Origin
    Fetched    time.Time // zero: not written
    SHA256     string
}
// Validate checks the form of every value a command writes (section 4.1).
// Errors: *errs.Error{Code: "invalid"}, naming the field and the limit, never the value.
func (d *Draft) Validate() error
// Emit returns the complete file: the header in pbrain's form, then the body (section 4.2).
// It calls Validate first and returns its error.
func Emit(d *Draft, body []byte) ([]byte, error)

// NormTag folds a tag and checks its form: one or more of a-z, 0-9 and "-".
func NormTag(s string) (string, bool)
// ParseStaleAfter accepts YYYY-MM-DD (the start of that day, UTC) or RFC 3339.
// norm is what Emit writes: the date as given, or the instant in UTC with a Z.
func ParseStaleAfter(s string) (t time.Time, norm string, ok bool)

// Layout (architecture 4.1, spec 6.2; section 3.7)
// Measure counts one file in one pass. data is the whole file, already checked to be UTF-8, and
// bodyAt is what Parse returned for the same bytes. Lines are counted nowhere else.
func Measure(data []byte, bodyAt int64) Layout
// LineOf returns the file line that holds the body's code point at offset cp: Layout.BodyLine plus
// the line feeds in body before it. For cp == BodyChars it is where a next character would stand.
// body is the stored body of the bytes Layout was measured on. It reads the body up to cp, so it is
// for one place; 0 when the item was never measured.
func (it *Item) LineOf(body []byte, cp int) int
// Lines answers the same question for rising offsets in one forward pass over the body.
func (it *Item) Lines(body []byte) *LineCursor
// LineCursor walks one body forward. Only Lines makes one.
type LineCursor struct { /* unexported: body, byte position, code points and line reached, bytes stepped */ }
// At returns the file line of offset cp. cp is never lower than in the call before; a lower cp
// restarts the walk from the start of the body, correct but a second pass. cp below 0 is 0, and cp
// past the end is the end.
func (c *LineCursor) At(cp int) int

// Editing (architecture 5, spec 8.3)
// EditVerify returns data with status set to verified and verified set to actor,
// changing no other byte (section 5).
// Errors: *errs.Error{Code: "unsupported_edit"} for any shape it does not edit;
// *errs.Error{Code: "invalid"} for an invalid actor.
func EditVerify(data []byte, actor string) ([]byte, error)
```

The fixed signatures of architecture 4.1 are unchanged. Everything else above is an addition inside `item`.

## 3. Reading

### 3.1 The block (`SplitFrontMatter`)

- An optional UTF-8 byte-order mark at offset 0 is skipped. Offsets stay those of the file. `Measure` counts it as one code point of the file and no line (section 3.7); it is never part of the body.
- **Opening delimiter.** The first line must be `---` followed only by spaces or tabs, then LF, CRLF or the end of the file. Otherwise the result is `ok = false`, and `Parse` returns `ErrNoFrontMatter`.
- **Closing delimiter.** The first later line of the same form. With none, the result is `Unclosed`, at the opening line. A file that opens with a Markdown thematic break `---` and never closes it is therefore `bad_front_matter`, not `not_an_item`. That is the convention every front matter reader shares.
- **The YAML text** is the bytes between the two delimiter lines, and `yamlLine` is 2 (3 is impossible: a BOM adds no line).
- **`bodyAt`** is the offset just after the closing line's terminator. If the next line is empty (`\n` or `\r\n`), `bodyAt` moves past it too: `Emit` writes exactly that one blank line before a non-empty body (section 4.2), so a body round-trips byte for byte. A closing delimiter at the end of the file gives `bodyAt = len(data)`.

### 3.2 `Parse`

1. Run `SplitFrontMatter`.
2. Unmarshal the YAML text into a `yaml.Node`.
   - A library error is `Syntax`, at the line the library reports, offset by `yamlLine - 1`, or 0 if none.
   - An empty or comment-only text (a zero `Node`, or a document node with no content) is an empty mapping.
   - A document whose root is not a block or flow mapping is `NotMapping`.
3. Walk the root mapping's key nodes. A key that is not a scalar is ignored. Two scalar keys with the same exact text are `DuplicateKey`, at the second key. Keys are compared byte for byte, so `kind` and `Kind` are different keys, and `Kind` is unknown and ignored.
4. For each known key, read the value node by its expected shape:
   - **One-line field** (`kind`, `title`, `created`, `author`, `source`, `status`, `verified`, `decided`, `stale_after`, `origin`, `fetched`, `sha256`). A `ScalarNode` gives its `Value` exactly as the library decoded it. YAML typing is ignored: a tag such as `!!int` or `!!bool` changes nothing, so `verified: true` is the actor `true`.
     - A literal or folded block scalar (`|`, `>`), an alias, a sequence or a mapping is the wrong type: `bad_field`, and the field is missing.
     - A plain or quoted scalar that the source continues over several lines is accepted, folded by YAML into one line. Rendering flattens any line breaks a quoted escape produced (spec 6.2).
     - An empty value (`Value == ""` with no quotes, i.e. `key:`) is absent, not wrong. A quoted `""` is also absent. `~` and `null` are text, by the same no-typing rule.
   - **List field** (`replaces`, `relates`, `tags`). A `SequenceNode`, flow or block, whose entries are all scalars with non-empty values gives the list in order.
     - An empty sequence `[]` is an empty list.
     - Any non-scalar or empty entry makes the whole list missing, with one `bad_field` at the bad entry (spec 2.4).
     - A scalar where a list is expected is the wrong type: `bad_field`, missing. A value written `key:` is absent.
     - A tag entry is passed through `NormTag`; one that fails makes the whole `tags` list missing with `bad_field`.
5. Apply the field rules (section 3.3), then fill `Title`, which may need the body.

`bodyAt` is returned even when fields are bad. Only the `FrontMatterError` cases and `ErrNoFrontMatter` stop `Parse`. Unknown keys are never inspected, whatever their shape.

### 3.3 Field rules

| Field | Read | Problems |
|---|---|---|
| `kind` | `ParseKind`. Missing or unknown: `Note`. `KindRaw` keeps the text. | Unknown text: `unknown_value`. |
| `title` | As written. Missing: `HeadingTitle(body)`, else the id. | none (a fallback is not a problem) |
| `created` | `time.Parse(time.RFC3339, v)`, which also takes fractional seconds and requires a zone. Missing or unparseable: `IDTime(id)`, else unknown (`CreatedKnown = false`). | Missing: `missing_field`. Unparseable: `bad_field`. |
| `author` | As written. | Missing: `missing_field`. |
| `source` | As written. | Missing on a `source` item: `missing_field`. |
| `status` | `provisional` or `verified`. Text beginning `superseded-by ` is kept in `StatusRaw` and reads as `Provisional`. Anything else reads as `Provisional`. | Unknown text (not a stamp): `unknown_value`. |
| `verified`, `decided` | As written. | A `decision` without `decided`: `decision_without_decider`. |
| `replaces`, `relates` | List of ids as written. Duplicates are kept here; `graph` dedupes by target. | |
| `tags` | List after `NormTag`. | |
| `stale_after` | `ParseStaleAfter`. Present and unparseable: `StaleSet = true`, `StaleBad = true` (counts as due, spec 4.6). | Unparseable: `bad_field`. |
| `origin` | `ParseOrigin`, `source` items only; on other kinds it is read but has no effect. | Unknown text: `bad_field`. |
| `fetched` | RFC 3339. | Unparseable: `bad_field`. |
| `sha256` | As written. Its form is checked where it is used (`delete`, architecture 10.6). | |

The words are exact: `kind: Decision` is unknown and reads as `note`, with a warning. pbrain writes lower case and factory's notes use it, so a fold here would only hide hand-typing mistakes from `check`.

Problems are returned in file order: by line, then column, with absent fields (line 0) first, in the table's field order.

### 3.4 `FirstHeading` and `HeadingTitle`

Headings are those of `outline.Parse` (architecture 7.4; @architect's `outline` package, a pure leaf). Using it means the title fallback, `get --outline` and the section named with a snippet agree on one definition, fences and setext headings included.

- `FirstHeading(body)` is the `Title` of the first section of `outline.Parse(body)` with `Level > 0` and a non-empty title.
- `FirstHeading` stops there: no heading anywhere in the body gives `""`.
- `HeadingTitle` then falls back to the first non-blank line, trimmed, that is not a fence delimiter. `importer` never uses that fallback: in spec 8.4 a page title or file name comes before a prose line.
- The result is not cut. Rendering cuts titles at 120 characters (spec 6.2).

### 3.5 Effective standing and the other derived values

`Standing` follows architecture 4.1 to the letter:

- a `source` item is 1, whatever its status;
- a `decision` with `Decided` set is 3, whatever its status;
- a `decision` without `Decided` is 1, even when it says `verified`;
- any other kind is 2 when `Status == Verified`, and 1 otherwise.

The derived values:
- `Linked()`: `Kind == Source && Origin == OriginLink`.
- `ReviewDue(now)`: `StaleSet && (StaleBad || !now.Before(StaleAfter))`. "From that instant on" (spec 4.6) includes the instant.
- `LegacyStamp()`: requires `StatusRaw` to start with `superseded-by `. The rest is trimmed of spaces. Everything up to the last `/` (or `\\`) is removed. A suffix that folds to `.md` is removed. An empty stem is `ok = false`.

### 3.6 The fold

`Fold` maps each rune through `unicode.ToLower`. For a string with no byte at or above 0x80 it lower-cases bytes `A`–`Z` only. It returns the input unchanged, with no allocation, when nothing changes. Invalid UTF-8 cannot reach it from a file: `store` rejects a file whose content or whose name is not valid UTF-8 (`store.md`, "Catalog scan and rereads"). An argument that is not valid UTF-8 is folded as `[]rune` conversion would decode it, which is why `store.Lookup` refuses such an argument before folding.

Users of the fold:
- `Item.Key`, filled by `store`;
- `Catalog.Lookup`;
- `graph`'s link resolution;
- `NewID`'s `used` callback;
- `brain` and `store`'s `.pbrain` name rule;
- `query`'s tokens.

**Package order.** `query` uses this same function. @architect accepted moving `query` above `item` in architecture 3's table (2026-11-20), with imports `errs` and `item`, so there is no copy. `TestQueryUsesItemFold` (in `search.md`) checks that the two agree.

### 3.7 Layout and lines (`Measure`, `LineOf`, `LineCursor`)

Spec 6.2 names a place in an item in two units: a **file line** of the stored file, from 1, front matter and its `---` lines included, and a **body offset** in code points from 0. `Layout` joins them. The rules below are the only ones; `render` and `search` ask `item` and count nothing themselves.

**`Measure(data, bodyAt)`** walks `data` once and counts, at each byte:
- a line feed (`0x0A`), in the whole file and before `bodyAt`;
- a code point, which is any byte that is not a UTF-8 continuation byte (`b & 0xC0 != 0x80`; `data` is valid UTF-8, `store` checked it), in the whole file and from `bodyAt` on.

From those four counts:
- `FileLines` is the line feeds, plus 1 when the file does not end in a line feed: a last line without one is a line, and a final line feed starts no new one. A carriage return is an ordinary character, so a CRLF ends one line.
- `FileChars` is the code points of the whole file. A byte-order mark before the opening `---` is one code point and no line.
- `BodyChars` is the code points from `bodyAt` to the end: the stored body as `get` pages it, carriage returns included.
- `BodyLine` is 1 plus the line feeds before `bodyAt` when the body is not empty. `bodyAt` always follows a line terminator (section 3.1), so that is the line the body's first character stands on. For an empty body (`bodyAt == len(data)`) it is `FileLines + 1`, the line a body would start on, whether or not the closing delimiter has a line feed.
- **Precondition.** `data` is valid UTF-8 and `bodyAt` is the offset a successful `Parse` (err == nil) returned for the same bytes. That offset follows a line terminator or equals `len(data)`, so it never splits a rune. Any other offset is outside the contract: it could fall inside a multibyte rune, and the counts would then be meaningless.

`store`'s scan calls `Measure` on the bytes it validated, with the `bodyAt` `Parse` returned, and keeps the result in `Item.Layout` (architecture 4.1). `Parse` and `SplitFrontMatter` keep their signatures and count no lines, so `add`, `verify` and `importer` are untouched.

**`LineOf` and `LineCursor`.** The file line of body offset `cp` is `BodyLine` plus the line feeds among the body's first `cp` code points. For `cp == BodyChars` that is where a next character would stand, so `LineOf(body, BodyChars)` of a body ending in a line feed is one past its last line. Both read only the body passed, `data[bodyAt:]` of the measured bytes, and decode it rune by rune from where they stand: an ASCII byte is one step, and a longer rune is `utf8.DecodeRune`'s size.
- `LineOf` makes a cursor and asks it once. It is for one place, such as `search`'s one hit.
- A `LineCursor` keeps its byte position, code points and line between calls, so offsets asked in rising order cost one pass over the body together. `get` seeds one at the start of a slice and moves it as the fitter grows the slice; the outline walks its sections in order. A body of one 512,000-character line is read once, which the cursor's unexported count of bytes stepped shows in the test.
- An offset lower than the last one restarts at the body's start. The result is still right, but it costs a second pass, which the same count shows; no caller in the design asks that way.
- On an item never measured (`BodyLine == 0`) both return 0, so a missing `Measure` shows as line 0 in a test rather than as a plausible wrong line.

## 4. Writing

### 4.1 `Draft.Validate`

Limits are in code points (spec 2.4, 8.2). Each failure is `invalid`, with a message of the form `<field>: <rule>`, for example `title: at most 120 characters` or `tags: at most 10 entries`. The value itself is never quoted, since it may be a secret or a personal id.

| Field | Rule |
|---|---|
| all text values | Valid UTF-8. No line break (LF, CR, U+0085, U+2028, U+2029) and no control character (`unicode.IsControl`), tab included. Other characters that YAML does not allow literally (section 4.2: U+FEFF, U+FFFE, U+FFFF) are accepted and written escaped. |
| `Title` | Non-empty after trimming spaces; at most 120. |
| `Author` | Non-empty; at most 200. |
| `Source` | At most 2,000. Required (non-empty) when `Kind == Source`. |
| `Verified`, `Decided` | At most 200; `Decided` only on a decision. |
| `Replaces`, `Relates` | At most 10 entries each; each non-empty, at most 200. |
| `Tags` | At most 10 entries; each passes `NormTag` unchanged. |
| `StaleAfter` | "" or the `norm` output of `ParseStaleAfter`. |
| `Status` | `Verified` is refused on a `source` item (spec 3). |
| `Origin`, `Fetched`, `SHA256` | Only on a `source` item. `SHA256` is 64 lower-case hexadecimal characters when set. `OriginLink` refuses `Fetched` and `SHA256`. |
| `Created` | Not zero. |

What `Validate` does not do, because it needs other items or the command: a decision requires `--decided`, ids must exist, tags may be undeclared, and the author fallbacks. Those are `cli` and `writes`.

### 4.2 `Emit`

The header is `---\n`, then one line per present field in this fixed order, then `---\n`:

`kind, title, created, author, source, status, verified, decided, replaces, relates, tags, stale_after, origin, fetched, sha256`

- **Kept out:** empty strings, empty lists, `OriginNone` and a zero `Fetched`. `kind`, `created` and `status` are always written.
- **Dates:** `created` and `fetched` are `t.UTC().Format("2006-01-02T15:04:05Z")`; `stale_after` is the normalised string. All three are plain.
- **Text values** go through the scalar rule below; lists are `[a, b]`, each entry through the flow variant.
- **Body:** if non-empty, one `\n` (the blank line), then the body bytes unchanged. An empty body adds nothing. Bodies are never re-encoded, and their newlines are not converted.

**Plain or quoted.** A value is written plain only if all of these hold, and double-quoted otherwise:

0. Every rune is in YAML's printable set (`[#x20-#x7E] | #xA0-#xD7FF | #xE000-#xFFFD | #x10000-#x10FFFF`) and is not U+FEFF.
1. It is not empty, and does not start or end with a space or tab.
2. Its first character is none of `- ? : , [ ] { } # & * ! | > ' " % @` and the backtick.
3. It contains no `: ` and no ` #`, and does not end with `:`.
4. In a flow list, it also contains none of `, [ ] { }`.
5. It does not match a form YAML 1.1 or 1.2 resolves to another type, compared case-insensitively:
   - null: `~`, `null`;
   - booleans: `y`, `n`, `yes`, `no`, `true`, `false`, `on`, `off`;
   - integers: `[-+]?[0-9][0-9_]*(:[0-5]?[0-9])*`, `[-+]?0b[01_]+`, `[-+]?0x[0-9a-f_]+`, `[-+]?0o[0-7]+`;
   - floats: `[-+]?([0-9][0-9_]*)?\.[0-9_]*([e][-+]?[0-9]+)?`, `[-+]?[0-9][0-9_]*e[-+]?[0-9]+`, `[-+]?\.inf`, `\.nan`;
   - timestamps: a value beginning `[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}`;
   - `=` and `<<`.

An id such as `20261001-101500-cache-uses-memcached-x7b2` matches none of these and is written plain, as in spec 2.4's sample.

Double quoting escapes `\\` and `"`, and writes every rune outside YAML's printable set, and U+FEFF, as `\uXXXX`. A literal U+FFFE or U+FFFF would make go.yaml.in/yaml/v3 v3.0.6 refuse the whole header, so a written item would vanish from the next read (found by @lead-dev's review). `Validate` has already refused every control character, so in practice this covers U+FEFF, U+FFFE and U+FFFF.

### 4.3 Ids

- **`Slug(title)`:**
  1. `strings.ToLower`.
  2. Every maximal run of characters outside `a-z0-9` becomes one `-`.
  3. Leading and trailing `-` are trimmed.
  4. Cut to 40 bytes (all ASCII by now) and trim a trailing `-` again.
  5. If nothing is left, use `item`.
  
  Trimming the hyphens is this design's reading of spec 2.3, which says nothing about the ends. Without it, `"Redis!"` would give `redis-` and an id with `--`.
- **`NewID`:** `now.UTC().Format("20060102-150405") + "-" + Slug(title) + "-" + four characters`.
  - Each character comes from one byte of `rnd`: bytes 252 to 255 are rejected and drawn again, and the rest map to `"abcdefghijklmnopqrstuvwxyz0123456789"[b % 36]`, which is uniform.
  - At most 16 ids are drawn. Each one taken (`used(Fold(id))`) draws again.
  - At 36^4 combinations per second and slug, 16 taken draws in a row only happen when `used` is broken. That case is `io` with the message "could not draw an unused id".
- **`IDTime`:** the first 15 bytes must be `[0-9]{8}-[0-9]{6}` and parse with `time.ParseInLocation("20060102-150405", ..., time.UTC)`, which rejects impossible dates. Factory's ids and pbrain's share this form.

### 4.4 Tags and review dates

- `NormTag(s)`: `Fold(strings.TrimSpace(s))`, then non-empty and only `a-z`, `0-9` and `-`. `Cache` becomes `cache`. `redis_cache` and `café` are refused: `invalid` on write, `bad_field` on read.
- `ParseStaleAfter(s)`:
  - `YYYY-MM-DD` with `time.ParseInLocation("2006-01-02", s, time.UTC)` gives `norm = s`;
  - otherwise RFC 3339 gives `norm = t.UTC().Format(time.RFC3339Nano)`, which keeps fractional seconds, so `2026-11-19T00:00:00.123Z` is not due 123 ms early;
  - otherwise `ok = false`.

## 5. The `verify` edit (`EditVerify`)

The caller (`writes`) has already decided that the item may be verified: not a source, not replaced, `Status` provisional, and `actor` from `--by` or the author rule. `EditVerify` only changes bytes, and refuses any file it cannot change safely.

1. `Validate` the actor as a one-line actor value (section 4.1). Run `SplitFrontMatter` and `Parse` the original. A failure is `unsupported_edit`; the scan normally skips such files first.
2. **Newline style:** the opening delimiter line's terminator, LF or CRLF. Every inserted line uses it.
3. **Shape checks.** The root must be a block mapping: a flow-mapping document (`{status: x}`) is `unsupported_edit`. For each of `status` and `verified` that is present:
   - its key is at column 1;
   - its value is a plain, single-quoted or double-quoted `ScalarNode` on the key's line, or empty. An empty value is `key:` with nothing after the colon but spaces and perhaps a comment; v3.0.6 gives such a node the column right after the colon and no `LineComment`, so emptiness is read from the raw line, not from the node;
   - after the key's line, skip blank lines and lines whose first non-space character is `#`. A comment line ends a plain scalar in YAML, so it is not a continuation. The next remaining line within the block must start at column 1 or be the closing delimiter, so no indented continuation follows.
   
   Anything else is `unsupported_edit`: a block scalar, a list, a value on the next line, or a multi-line scalar. The error message names the field, never its value.
4. **Rewrite a present key's line.** Work on the raw line, without its terminator. `k` is the offset just after the key's colon.
   - **The value's span.** Skip spaces and tabs after `k`.
     - If the next character is `#` or the line ends, the value is empty, and the span is empty at that point.
     - A single-quoted value ends at its closing `'` (`''` is an escaped quote).
     - A double-quoted value ends at its closing `"` (a backslash escapes the next character).
     - A plain value ends just before the first space or tab followed by `#`, or at the end of the line, with trailing spaces excluded.
   - **The comment** is everything from the first `#` after the span (after spaces or tabs) to the end of the line, together with the spaces before it. It is kept byte for byte.
   - **The new line** is the bytes up to `k`, then one space, then the new value, then the comment as found (or nothing), then the original terminator. The new value is `verified` for `status`, and `Emit`'s scalar rule applied to `actor` for `verified`.
   - So `verified:` becomes `verified: <actor>`, and `verified: # keep me` becomes `verified: <actor> # keep me`.
5. **Insert an absent key.**
   - `status: verified` goes on a new line just before the closing delimiter.
   - `verified: <actor>` goes on a new line just after the `status` line, whether that line was rewritten or inserted.
   - Both use the file's newline style.
6. **Check the result.** `Parse` the result. Every `Header` field except `Status`, `StatusRaw` and `Verified` must equal the original's; `Status` must be `Verified` and `Verified` the actor. Any difference is `unsupported_edit` and the original is kept. This catches a shape the checks above missed, at the cost of one extra parse.

Bytes outside the one or two touched lines are copied unchanged: comments, unknown keys, key order, CRLF and the body. A file with a `verified` key and a provisional status keeps exactly one `verified` line (scenario 16). A stamp status (`superseded-by X`) is replaced like any other value; `writes` refuses `verify` on a replaced item before this runs, so this happens only when the stamp was not accepted.

## 6. Edge cases

| Case | Result |
|---|---|
| `---` alone, then EOF | `Unclosed`: `bad_front_matter`. |
| `---\n---\n` | An empty mapping: a `note` with title = id, `missing_field` for `created` and `author`. |
| A BOM before `---` | Accepted; offsets stay those of the file. `Measure`: one code point of the file, no line, not in the body. |
| `---` with trailing spaces | A delimiter. |
| CRLF throughout | Parsed; `EditVerify` keeps CRLF. `Measure`: a CRLF ends one line; the carriage return is a code point of the body. |
| No final line feed | The last line counts in `FileLines`; `LineOf(body, BodyChars)` is that last line. |
| Empty body, with or without a line feed after the closing `---` | `BodyChars` 0 and `BodyLine = FileLines + 1`; `LineOf(body, 0)` is the same. |
| A body that ends in a line feed | `LineOf(body, BodyChars)` is one past the body's last line: where a next character would stand. |
| A body that starts with `---` | Unaffected: the header has already closed. |
| `title: "line\nbreak"` | Read as written; output flattens it. |
| `tags: cache` | `bad_field`; no tags. |
| `tags: [cache, Redis]` | `[cache, redis]`. |
| `tags: [cache, "a b"]` | `bad_field`; no tags. |
| `kind: [decision]` | `bad_field`; reads as `note`. |
| `created: 2026-11-19` (no time) | Not RFC 3339: `bad_field`, then the id's date. |
| `status: superseded-by ../notes/x.md` | A stamp with stem `x`, provisional. |
| Duplicate `title` keys | `DuplicateKey`: `bad_front_matter`. |
| An anchor and an alias on a known key | The alias is the wrong type: `bad_field`. |
| A 4 MiB file with a 3 MiB header | Parsed. No separate header limit; the scan's 4 MiB file limit bounds it. |

## 7. Dependencies

- **Uses:** `errs` (`invalid`, `unsupported_edit`, `io`), `outline` (`Parse`, for the heading fallback), `go.yaml.in/yaml/v3` v3.0.6 (node API only), the standard library.
- **Used by:**
  - `store`: `Parse` and `Fold` in the scan; `Emit` and `EditVerify` in the write primitives.
  - `graph`: `Standing`, `LegacyStamp`, `Key`.
  - `search`: `Standing`, `Fold`.
  - `render`: the `String` methods, `ReviewDue`, `Linked`.
  - `check`: `Problems`, `FrontMatterError`, `ErrNoFrontMatter`.
  - `importer`: `SplitFrontMatter`, `FirstHeading`, `Draft`.
  - `writes`: `NewID`, `Draft`, `Emit`, `EditVerify`, `NormTag`, `ParseStaleAfter`.
  - `brain`: `Fold`.
  - `query`: `Fold` (package order accepted, section 3.6).
- **Speed:** `Measure` is one pass over bytes the scan already holds, with no allocation; it is on the scan's measurement list in architecture 8.6. One YAML node parse per file during the scan. At 5,000 headers of under 1 KB, parallel on the scan's workers, this is expected to be well under the scan's budget. It is measured with the whole-process benchmarks of architecture 8.6, not promised here.

## 8. Test plan

Unit tests are table-driven and use no file system.

| Test | What it proves | Spec |
|---|---|---|
| `TestSplitFrontMatter` | Opening and closing rules, trailing spaces, BOM, CRLF, unclosed, `bodyAt` skipping one blank line only, a delimiter at EOF. | 2.4 |
| `TestParseNoFrontMatter` | A plain `.md` file gives `ErrNoFrontMatter`. | 2.2, 9.1 `not_an_item` |
| `TestParseBadFrontMatter` | Syntax error, non-mapping root, duplicate key, unclosed block: each gives its `FMReason` and a file line. | 2.4, 9.1 `bad_front_matter` |
| `TestParseFieldShapes` | Every row of section 3.2: scalar, block scalar, alias, sequence, mapping, empty, quoted empty, `~`, tags such as `!!bool`, a bad list entry, a scalar for a list. | 2.4 |
| `TestParseFieldRules` | Every row of section 3.3, with the exact `Problem` codes, fields and positions. | 2.4, 9.1 |
| `TestParseKindAlwaysSet` | No input leaves `Kind` at its zero value unless it says `decision`. | 2.5 |
| `TestTitleFallbacks` | ATX heading; setext heading after a preamble (the heading wins over the preamble line); a heading inside a fence skipped; an ATX and a setext heading after a fenced example; first line; empty body gives the id; `FirstHeading` returns "" where `HeadingTitle` falls back to the first line. | 2.4, 8.4 |
| `TestCreatedFallback` | RFC 3339 with and without fraction, date-only refused, id date used, unknown; scenario 3's "unknown" item. | 2.4, scenario 3 |
| `TestStandingTable` | The five rules of section 3.5, including a verified decision with no decider (1) and a source that says verified (1). | 3, scenario 21 |
| `TestReviewDue` | Before, at and after the instant; date-only; unparseable is due. | 4.6, scenario 5 |
| `TestLegacyStamp` | Plain stem, folders, `.MD`, trailing spaces, empty stem, not a stamp. | 4.3, scenario 11 |
| `TestFactoryNotesRead` | Factory-shaped headers: no kind gives `note`, a decision without `decided` gives standing 1 with `decision_without_decider`, a stamp, a verified learning, and no byte changed. | 13, scenario 21 |
| `TestFold` | ASCII fast path without allocation, Unicode (`Σ`, `İ`), no normalisation (composed and decomposed `é` differ). | 2.3, architecture 4.4 |
| `TestSlug` | Spec 2.3's example, punctuation runs, trimming the ends, the 40-byte cut followed by trimming, non-ASCII only gives `item`. | 2.3 |
| `TestNewIDUniqueAndUniform` | A fixed `rnd` gives a known id; a `used` that refuses the first draw gives the second; 16 refusals give `io`; bytes 252 to 255 are rejected. Two ids in one second with one slug differ. | 2.3, scenario 14 |
| `TestIDTime` | Valid, impossible date, short id, a factory-style id. | 2.4 |
| `TestValidateLimits` | Every row of section 4.1 at the limit and one over; the messages never contain the value. | 2.4, 8.2, scenario 28 (2,000 / 2,001 source) |
| `TestEmitForm` | Key order, omitted fields, LF, plain and quoted cases, the spec 2.4 sample byte for byte, the body blank line; a title holding U+FFFE, U+FFFF or U+FEFF is written escaped and reads back equal. | 2.4 |
| `TestStaleAfterPrecision` | `2026-11-19T00:00:00.123Z` is kept with its fraction; `ReviewDue` at .122 and .123. | 4.6 |
| `FuzzEmitParseRoundTrip` | For any `Draft` that passes `Validate`, and any body: `Parse(Emit(d))` gives the same values, `bodyAt` points at the body, and `yaml.Unmarshal` of the header into `map[string]any` gives a Go `string` for every text field. | architecture 5, 14 |
| `FuzzParse` | Never panics. A field is never filled from another key's node. | architecture 14 |
| `TestMeasure` | `BodyLine`, `BodyChars`, `FileLines` and `FileChars` on one header and body stored with LF, with CRLF, with a byte-order mark before `---`, with no final line feed, and with an empty body both with and without a line feed after the closing delimiter, against counts written by hand; multibyte text before and in the body. | 6.2, scenario 39, architecture 14 |
| `TestLineOf` | On the files of `TestMeasure`: `LineOf` at offset 0, inside a long line, at a character just before and just after a line feed, between a carriage return and its line feed, at `BodyChars`, and past it (clamped). Each answer equals the 1-based index of the line, in the stored file split at line feeds, that holds the byte where that code point starts; at `BodyChars` it is the line after a final line feed. An item never measured gives 0. | 6.2, architecture 14 |
| `TestLineCursorOnePass` | A body of one 512,000-character line and a body of 100,000 short lines, asked at rising offsets the way `get` and the outline ask: every answer equals `LineOf` at the same offset, and the cursor's count of bytes stepped equals the bytes up to the last offset asked, so the body is read once. A lower offset restarts and still answers right, and the count shows the second pass. | architecture 14 |
| `FuzzMeasure` | Seeds and inputs are files with a header and a body, the body often multibyte (`é`, CJK, emoji, a byte-order mark before `---`, CRLF). For each valid UTF-8 input that `Parse` accepts with err == nil, and only with the `bodyAt` it returned: `Measure` equals a naive oracle (split at line feeds, `utf8.RuneCount` of the file and of `data[bodyAt:]`), and `LineOf` at a random code-point offset of the body equals the oracle's line. Inputs `Parse` refuses are skipped, not clamped. | architecture 14 |
| `TestEditVerify` | Comments, unknown keys and CRLF kept, with only the two lines changed (diff of the bytes); an existing `verified` key left as one line; status inserted at the end; `verified` inserted after `status`; a trailing comment kept after plain, quoted (with `#` and `''` inside) and empty values; `verified:` and `verified: # keep me` in LF and CRLF; an indented comment line after `status: provisional` is not a continuation. | 8.3, scenario 16 |
| `TestEditVerifyRefuses` | A block-scalar status, a status continued on the next line, a list value, a flow-mapping document, a key not at column 1: all give `unsupported_edit`, with the input unchanged. | 8.3, scenario 16 |
| `FuzzEditVerify` | For any input that `EditVerify` accepts, the result differs from the input only in the touched lines and parses with the original's other fields. | 8.3 |
