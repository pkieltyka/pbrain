package item

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/errs"
)

// Draft holds the values published in a new item.
type Draft struct {
	Kind       Kind
	Title      string
	Created    time.Time
	Author     string
	Source     string
	Status     Status
	Verified   string
	Decided    string
	Replaces   []string
	Relates    []string
	Tags       []string
	StaleAfter string
	Origin     Origin
	Fetched    time.Time
	SHA256     string
}

// Validate checks values that a command is about to publish.
func (d *Draft) Validate() error {
	if d.Kind.String() == "" {
		return invalid("kind", "unknown kind")
	}
	if d.Status.String() == "" {
		return invalid("status", "unknown status")
	}
	if d.Origin > OriginURL {
		return invalid("origin", "unknown origin")
	}
	for _, field := range []struct {
		name, value string
		limit       int
	}{
		{"title", d.Title, 120},
		{"author", d.Author, 200},
		{"source", d.Source, 2000},
		{"verified", d.Verified, 200},
		{"decided", d.Decided, 200},
		{"stale_after", d.StaleAfter, 200},
		{"sha256", d.SHA256, 200},
	} {
		if err := validateText(field.name, field.value, field.limit); err != nil {
			return err
		}
	}
	if strings.TrimSpace(d.Title) == "" {
		return invalid("title", "must not be empty")
	}
	if d.Author == "" {
		return invalid("author", "must not be empty")
	}
	if d.Kind == Source && d.Source == "" {
		return invalid("source", "required on a source item")
	}
	if d.Decided != "" && d.Kind != Decision {
		return invalid("decided", "only a decision has a decider")
	}
	for _, field := range []struct {
		name   string
		values []string
	}{{"replaces", d.Replaces}, {"relates", d.Relates}, {"tags", d.Tags}} {
		if len(field.values) > 10 {
			return invalid(field.name, "at most 10 entries")
		}
		for _, value := range field.values {
			if value == "" {
				return invalid(field.name, "entries must not be empty")
			}
			if err := validateText(field.name, value, 200); err != nil {
				return err
			}
			if field.name == "tags" {
				if tag, ok := NormTag(value); !ok || tag != value {
					return invalid("tags", "expected a lower-case tag")
				}
			}
		}
	}
	if d.StaleAfter != "" {
		if _, norm, ok := ParseStaleAfter(d.StaleAfter); !ok || norm != d.StaleAfter {
			return invalid("stale_after", "expected a normalised date or instant")
		}
	}
	if d.Kind == Source && d.Status == Verified {
		return invalid("status", "a source cannot be verified")
	}
	if d.Kind != Source {
		switch {
		case d.Origin != OriginNone:
			return invalid("origin", "only a source has an origin")
		case !d.Fetched.IsZero():
			return invalid("fetched", "only a source has a fetched date")
		case d.SHA256 != "":
			return invalid("sha256", "only a source has a digest")
		}
	}
	if d.SHA256 != "" && (len(d.SHA256) != 64 || strings.Trim(d.SHA256, "0123456789abcdef") != "") {
		return invalid("sha256", "expected 64 lower-case hexadecimal characters")
	}
	if d.Origin == OriginLink {
		if !d.Fetched.IsZero() {
			return invalid("fetched", "a linked source is not fetched")
		}
		if d.SHA256 != "" {
			return invalid("sha256", "a linked source has no digest")
		}
	}
	if d.Created.IsZero() || d.Created.UTC().Year() < 0 || d.Created.UTC().Year() > 9999 {
		return invalid("created", "expected a nonzero RFC 3339 instant")
	}
	if !d.Fetched.IsZero() && (d.Fetched.UTC().Year() < 0 || d.Fetched.UTC().Year() > 9999) {
		return invalid("fetched", "expected an RFC 3339 instant")
	}
	return nil
}

func validateText(field, value string, limit int) error {
	if !utf8.ValidString(value) {
		return invalid(field, "expected valid UTF-8")
	}
	if strings.ContainsFunc(value, func(r rune) bool {
		return unicode.IsControl(r) || r == '\u2028' || r == '\u2029'
	}) {
		return invalid(field, "expected one line without control characters")
	}
	if utf8.RuneCountInString(value) > limit {
		return invalid(field, "at most "+strconv.Itoa(limit)+" characters")
	}
	return nil
}

func invalid(field, rule string) error {
	return &errs.Error{Code: "invalid", Message: field + ": " + rule, Exit: 1}
}

// Emit writes a complete item, retaining the stored body bytes as given.
func Emit(d *Draft, body []byte) ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	out := append(make([]byte, 0, len(body)+256), "---\n"...)
	add := func(key, value string) {
		if value != "" {
			out = append(out, key...)
			out = append(out, ": "...)
			out = append(out, value...)
			out = append(out, '\n')
		}
	}
	addList := func(key string, values []string) {
		if len(values) == 0 {
			return
		}
		out = append(out, key...)
		out = append(out, ": ["...)
		for i, value := range values {
			if i > 0 {
				out = append(out, ", "...)
			}
			out = appendScalar(out, value, true)
		}
		out = append(out, "]\n"...)
	}
	add("kind", d.Kind.String())
	add("title", scalarText(d.Title))
	add("created", d.Created.UTC().Format(time.RFC3339))
	add("author", scalarText(d.Author))
	add("source", scalarText(d.Source))
	add("status", d.Status.String())
	add("verified", scalarText(d.Verified))
	add("decided", scalarText(d.Decided))
	addList("replaces", d.Replaces)
	addList("relates", d.Relates)
	addList("tags", d.Tags)
	add("stale_after", d.StaleAfter)
	add("origin", d.Origin.String())
	if !d.Fetched.IsZero() {
		add("fetched", d.Fetched.UTC().Format(time.RFC3339))
	}
	add("sha256", scalarText(d.SHA256))
	out = append(out, "---\n"...)
	if len(body) != 0 {
		out = append(out, '\n')
		out = append(out, body...)
	}
	return out, nil
}

func scalarText(s string) string {
	if s == "" || plainScalar(s, false) {
		return s
	}
	return string(quoteScalar(nil, s))
}

func appendScalar(out []byte, s string, flow bool) []byte {
	if plainScalar(s, flow) {
		return append(out, s...)
	}
	return quoteScalar(out, s)
}

var yamlTyped = regexp.MustCompile(`(?i)^(?:[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])*|[-+]?0b[01_]+|[-+]?0x[0-9a-f_]+|[-+]?0o[0-7]+|[-+]?(?:[0-9][0-9_]*)?\.[0-9_]*(?:e[-+]?[0-9]+)?|[-+]?[0-9][0-9_]*e[-+]?[0-9]+|[-+]?\.inf|\.nan)$`)
var yamlDateStart = regexp.MustCompile(`^[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}`)

func plainScalar(s string, flow bool) bool {
	// The pinned YAML scanner treats '?' as an indicator inside a flow scalar.
	if flow && strings.ContainsAny(s, ",?[]{}") {
		return false
	}
	if s == "" || s != strings.Trim(s, " \t") ||
		strings.ContainsRune("-?:,[]{}#&*!|>'\"%@`", rune(s[0])) ||
		strings.Contains(s, ": ") || strings.HasSuffix(s, ":") || strings.Contains(s, " #") ||
		strings.ContainsFunc(s, func(r rune) bool { return !yamlPrintable(r) }) {
		return false
	}
	switch strings.ToLower(s) {
	case "~", "null", "y", "n", "yes", "no", "true", "false", "on", "off", "=", "<<":
		return false
	}
	return !yamlTyped.MatchString(s) && !yamlDateStart.MatchString(s)
}

func yamlPrintable(r rune) bool {
	return r != '\ufeff' && (r >= 0x20 && r <= 0x7e || r >= 0xa0 && r <= 0xd7ff ||
		r >= 0xe000 && r <= 0xfffd || r >= 0x10000 && r <= 0x10ffff)
}

func quoteScalar(out []byte, s string) []byte {
	const hex = "0123456789ABCDEF"
	out = append(out, '"')
	for _, r := range s {
		switch {
		case !yamlPrintable(r):
			out = append(out, '\\', 'u', hex[(r>>12)&15], hex[(r>>8)&15], hex[(r>>4)&15], hex[r&15])
		case r == '\\' || r == '"':
			out = append(out, '\\', byte(r))
		default:
			out = utf8.AppendRune(out, r)
		}
	}
	return append(out, '"')
}
