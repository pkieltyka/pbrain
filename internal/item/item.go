// Package item: exported surface from the pbrain specs; bodies are filled in by later phases.
package item

import (
	"errors"
	"io"
	"time"
)

type Section uint8

const (
	Shared Section = iota
	Personal
)

func (s Section) String() string { panic("not implemented") } // "shared", "personal"

type Kind uint8

const (
	Decision Kind = iota
	Learning
	Observation
	Note
	Source
)

func (k Kind) String() string         { panic("not implemented") } // "decision", "learning", "observation", "note", "source"
func ParseKind(s string) (Kind, bool) { panic("not implemented") } // exact lower-case spelling; ok=false for anything else

type Status uint8

const (
	Provisional Status = iota
	Verified
)

func (s Status) String() string { panic("not implemented") } // "provisional", "verified"

type Origin uint8

const (
	OriginNone Origin = iota
	OriginFile
	OriginLink
	OriginURL
)

func (o Origin) String() string           { panic("not implemented") } // "", "file", "link", "url"
func ParseOrigin(s string) (Origin, bool) { panic("not implemented") }

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
type FMReason uint8                       // Unclosed, Syntax, NotMapping, DuplicateKey
func (e *FrontMatterError) Error() string { panic("not implemented") } // "front matter: <reason> at line N"; never quotes file content
// Fold is the one comparison fold (architecture 4.4): unicode.ToLower rune by rune,
// no normalisation. ASCII-only input takes a byte loop with no allocation when already lower case.
func Fold(s string) string { panic("not implemented") }

// SplitFrontMatter locates the block. It returns the YAML text between the delimiters,
// the file line on which that text starts, and the byte offset at which the body starts.
// ok is false when the file has no opening delimiter. An opening delimiter without a
// closing one returns *FrontMatterError{Reason: Unclosed}.
// importer uses it to strip a leading block from imported text with exactly the rule readers use.
func SplitFrontMatter(data []byte) (yaml []byte, yamlLine int, bodyAt int, ok bool, err error) {
	panic("not implemented")
}

// Parse reads one file's front matter under pbrain's rules (section 3.2).
// id is the file stem: it feeds the created and title fallbacks.
// err is ErrNoFrontMatter or *FrontMatterError; with err == nil, h is complete.
func Parse(id string, data []byte) (h Header, bodyAt int64, problems []Problem, err error) {
	panic("not implemented")
}

// FirstHeading returns the title of the body's first heading, ATX or setext, outside fenced code
// (the first outline.Parse section with Level > 0 and a non-empty title), or "".
// importer uses it for spec 8.4's title inference (first heading, then page title, then file name or URL).
func FirstHeading(body []byte) string { panic("not implemented") }

// HeadingTitle is the item title fallback of spec 2.4: FirstHeading(body), else the first
// non-blank line that is not a fence delimiter, trimmed; "" if neither. Items only.
func HeadingTitle(body []byte) string { panic("not implemented") }

// Methods on Header (in addition to Standing and Linked of architecture 4.1).
func (h *Header) Standing() int { panic("not implemented") }
func (h *Header) Linked() bool  { panic("not implemented") }

// ReviewDue: a stale_after is present and now is at or past it, or the value did not parse.
func (h *Header) ReviewDue(now time.Time) bool { panic("not implemented") }

// LegacyStamp returns the stem a "superseded-by <file>" status names:
// the text after the prefix, trimmed, with folders and a final ".md" (folded) removed.
// ok is false when the status is not a stamp or the stem is empty.
func (h *Header) LegacyStamp() (stem string, ok bool) { panic("not implemented") }

// IDs
func Slug(title string) string { panic("not implemented") }

// NewID draws ids until used(Fold(id)) is false, at most 16 times.
// rnd supplies the random characters (cli.Env.Rand). now is converted to UTC.
// Errors: *errs.Error{Code: "io"} when rnd fails or 16 draws are all used.
func NewID(now time.Time, title string, rnd io.Reader, used func(key string) bool) (string, error) {
	panic("not implemented")
}

// IDTime reads the leading "YYYYMMDD-HHMMSS" of an id as a UTC instant.
func IDTime(id string) (time.Time, bool) { panic("not implemented") }

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
func (d *Draft) Validate() error { panic("not implemented") }

// Emit returns the complete file: the header in pbrain's form, then the body (section 4.2).
// It calls Validate first and returns its error.
func Emit(d *Draft, body []byte) ([]byte, error) { panic("not implemented") }

// NormTag folds a tag and checks its form: one or more of a-z, 0-9 and "-".
func NormTag(s string) (string, bool) { panic("not implemented") }

// ParseStaleAfter accepts YYYY-MM-DD (the start of that day, UTC) or RFC 3339.
// norm is what Emit writes: the date as given, or the instant in UTC with a Z.
func ParseStaleAfter(s string) (t time.Time, norm string, ok bool) { panic("not implemented") }

// Layout (architecture 4.1, spec 6.2; section 3.7)
// Measure counts one file in one pass. data is the whole file, already checked to be UTF-8, and
// bodyAt is what Parse returned for the same bytes. Lines are counted nowhere else.
func Measure(data []byte, bodyAt int64) Layout { panic("not implemented") }

// LineOf returns the file line that holds the body's code point at offset cp: Layout.BodyLine plus
// the line feeds in body before it. For cp == BodyChars it is where a next character would stand.
// body is the stored body of the bytes Layout was measured on. It reads the body up to cp, so it is
// for one place; 0 when the item was never measured.
func (it *Item) LineOf(body []byte, cp int) int { panic("not implemented") }

// Lines answers the same question for rising offsets in one forward pass over the body.
func (it *Item) Lines(body []byte) *LineCursor { panic("not implemented") }

// LineCursor walks one body forward. Only Lines makes one.
type LineCursor struct { /* unexported: body, byte position, code points and line reached, bytes stepped */
}

// At returns the file line of offset cp. cp is never lower than in the call before; a lower cp
// restarts the walk from the start of the body, correct but a second pass. cp below 0 is 0, and cp
// past the end is the end.
func (c *LineCursor) At(cp int) int { panic("not implemented") }

// Editing (architecture 5, spec 8.3)
// EditVerify returns data with status set to verified and verified set to actor,
// changing no other byte (section 5).
// Errors: *errs.Error{Code: "unsupported_edit"} for any shape it does not edit;
// *errs.Error{Code: "invalid"} for an invalid actor.
func EditVerify(data []byte, actor string) ([]byte, error) { panic("not implemented") }

// Item is one Markdown file of the brain: one piece of knowledge with its header, cited by its id.
type Item struct {
	ID       string // file name without ".md", as on disk
	Key      string // ID folded for comparison (section 4.4)
	Section  Section
	Path     string // absolute, symlinks resolved
	Header   Header
	Problems []Problem // what check reports about this file's front matter
	BodyAt   int64     // byte offset of the body in the bytes that were read
	Layout   Layout    // where the body lies and how large body and file are; set by the scan
	Version  Version   // identifies those bytes
}

// Version identifies one observed state of a file.
type Version struct {
	Size    int64
	ModTime time.Time
	Sum     uint32 // CRC-32C of the whole file as read
}

type Header struct {
	Kind         Kind
	KindRaw      string
	Title        string // after the fallbacks of spec 2.4
	Created      time.Time
	CreatedKnown bool
	Author       string
	Source       string
	Status       Status
	StatusRaw    string // also carries "superseded-by <file>"
	Verified     string
	Decided      string
	Replaces     []string
	Relates      []string
	Tags         []string
	StaleAfter   time.Time
	StaleSet     bool // a field is present; unparseable counts as due
	StaleBad     bool
	Origin       Origin
	Fetched      time.Time
	SHA256       string
}

// Layout says where the body lies in a file and how large both are (spec 6.2).
type Layout struct {
	BodyLine  int // the file line, from 1, on which the body starts; FileLines + 1 for an empty body
	BodyChars int // code points of the body
	FileLines int // lines of the whole file, front matter included
	FileChars int // code points of the whole file
}
