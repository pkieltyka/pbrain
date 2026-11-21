// Package item reads, measures and writes an individual Markdown item.
package item

import (
	"errors"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/errs"
)

type Section uint8

const (
	Shared Section = iota
	Personal
)

func (s Section) String() string { return nameAt(sectionNames[:], uint8(s)) }

var sectionNames = [...]string{"shared", "personal"}

type Kind uint8

const (
	Decision Kind = iota
	Learning
	Observation
	Note
	Source
)

var kindNames = [...]string{"decision", "learning", "observation", "note", "source"}

func (k Kind) String() string { return nameAt(kindNames[:], uint8(k)) }

func ParseKind(s string) (Kind, bool) {
	for i, name := range kindNames {
		if s == name {
			return Kind(i), true
		}
	}
	return Note, false
}

type Status uint8

const (
	Provisional Status = iota
	Verified
)

func (s Status) String() string { return nameAt(statusNames[:], uint8(s)) }

var statusNames = [...]string{"provisional", "verified"}

type Origin uint8

const (
	OriginNone Origin = iota
	OriginFile
	OriginLink
	OriginURL
)

var originNames = [...]string{"", "file", "link", "url"}

func (o Origin) String() string { return nameAt(originNames[:], uint8(o)) }

func ParseOrigin(s string) (Origin, bool) {
	for i, name := range originNames {
		if s == name {
			return Origin(i), true
		}
	}
	return OriginNone, false
}

func nameAt(names []string, i uint8) string {
	if int(i) >= len(names) {
		return ""
	}
	return names[i]
}

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

// Standing is the item's effective rank for ranking and replacement.
func (h *Header) Standing() int {
	switch h.Kind {
	case Source:
		return 1
	case Decision:
		if h.Decided != "" {
			return 3
		}
		return 1
	default:
		if h.Status == Verified {
			return 2
		}
		return 1
	}
}

// Linked reports a source item with origin link.
func (h *Header) Linked() bool { return h.Kind == Source && h.Origin == OriginLink }

// ReviewDue reports whether the review instant has arrived or the date could not be read.
func (h *Header) ReviewDue(now time.Time) bool {
	return h.StaleSet && (h.StaleBad || !now.Before(h.StaleAfter))
}

const stampPrefix = "superseded-by "

// LegacyStamp returns the stem named by the recorded legacy status.
func (h *Header) LegacyStamp() (string, bool) {
	rest, ok := strings.CutPrefix(h.StatusRaw, stampPrefix)
	if !ok {
		return "", false
	}
	stem := strings.TrimSpace(rest)
	stem = stem[strings.LastIndexAny(stem, "/\\")+1:]
	if strings.HasSuffix(Fold(stem), ".md") {
		stem = stem[:len(stem)-len(".md")]
	}
	return stem, stem != ""
}

// Fold maps runes through unicode.ToLower, with no normalisation.
func Fold(s string) string {
	firstUpper := -1
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= utf8.RuneSelf {
			for _, r := range s {
				if r == utf8.RuneError || unicode.ToLower(r) != r {
					// Re-encode malformed arguments as rune iteration would.
					return foldRunes(s)
				}
			}
			return s
		}
		if firstUpper < 0 && b >= 'A' && b <= 'Z' {
			firstUpper = i
		}
	}
	if firstUpper < 0 {
		return s
	}
	var folded strings.Builder
	folded.Grow(len(s))
	folded.WriteString(s[:firstUpper])
	for i := firstUpper; i < len(s); i++ {
		b := s[i]
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		folded.WriteByte(b)
	}
	return folded.String()
}

func foldRunes(s string) string {
	var folded strings.Builder
	folded.Grow(len(s))
	for _, r := range s {
		folded.WriteRune(unicode.ToLower(r))
	}
	return folded.String()
}

// Slug returns the ASCII, human-readable part of a new id.
func Slug(title string) string {
	var slug strings.Builder
	separator := false
	for _, r := range strings.ToLower(title) {
		if r < 'a' || r > 'z' {
			if r < '0' || r > '9' {
				separator = slug.Len() > 0
				continue
			}
		}
		if separator {
			slug.WriteByte('-')
			separator = false
		}
		slug.WriteByte(byte(r))
		if slug.Len() >= 40 {
			break
		}
	}
	if slug.Len() == 0 {
		return "item"
	}
	s := slug.String()
	return strings.TrimSuffix(s[:min(len(s), 40)], "-")
}

const idTimeFormat = "20060102-150405"

// NewID draws up to 16 ids, discarding those whose folded id is already used.
func NewID(now time.Time, title string, rnd io.Reader, used func(key string) bool) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	prefix := now.UTC().Format(idTimeFormat) + "-" + Slug(title) + "-"
	for range 16 {
		var tail [4]byte
		for i := 0; i < len(tail); {
			var sample [1]byte
			if _, err := io.ReadFull(rnd, sample[:]); err != nil {
				return "", &errs.Error{Code: "io", Message: "could not draw a random id", Exit: 1}
			}
			if sample[0] < 252 {
				tail[i] = alphabet[int(sample[0])%len(alphabet)]
				i++
			}
		}
		id := prefix + string(tail[:])
		if !used(Fold(id)) {
			return id, nil
		}
	}
	return "", &errs.Error{Code: "io", Message: "could not draw an unused id", Exit: 1}
}

// IDTime reads the leading YYYYMMDD-HHMMSS as a UTC instant.
func IDTime(id string) (time.Time, bool) {
	if len(id) < len(idTimeFormat) {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation(idTimeFormat, id[:len(idTimeFormat)], time.UTC)
	return t, err == nil
}

// NormTag folds and validates the ASCII spelling of a tag.
func NormTag(s string) (string, bool) {
	s = Fold(strings.TrimSpace(s))
	for i := 0; i < len(s); i++ {
		b := s[i]
		if !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '-') {
			return "", false
		}
	}
	return s, s != ""
}

// ParseStaleAfter accepts a UTC day or an RFC 3339 instant.
func ParseStaleAfter(s string) (time.Time, string, bool) {
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t, s, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, t.UTC().Format(time.RFC3339Nano), true
	}
	return time.Time{}, "", false
}
