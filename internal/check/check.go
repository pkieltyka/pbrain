// Package check collects the findings of pbrain check (spec 9.1, render.md "Check"). It performs
// no I/O: the caller passes one scan's items, skipped files and graph.
package check

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/pkieltyka/pbrain/internal/graph"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/secretscan"
)

type Skipped struct {
	Section            item.Section
	Path, Code, Reason string
	Line, Column       int
}
type Options struct {
	Personal    bool
	HasTags     bool
	Tags        map[string]string
	Now         time.Time
	DisplayPath func(item.Section, string) string
	// Body gives an item's stored body as scanned, for the secret scan of shared items; an item
	// whose body cannot be read as scanned is not scanned. Nil scans the header fields only.
	Body func(*item.Item) ([]byte, error)
}
type Finding struct {
	Severity, Code, Path, Message string
	Field                         string
	Line, Column                  int
}

const (
	sevError   = "error"
	sevWarning = "warning"
	sevInfo    = "info"
)

var severity = map[string]string{
	"bad_front_matter":         sevError,
	"bad_field":                sevError,
	"missing_field":            sevError,
	"duplicate_id":             sevError,
	"replace_invalid":          sevError,
	"replace_cycle":            sevError,
	"personal_reference":       sevError,
	"dangling_link":            sevWarning,
	"replace_fork":             sevWarning,
	"legacy_stamp_ignored":     sevWarning,
	"decision_without_decider": sevWarning,
	"secret_suspected":         sevWarning,
	"unknown_value":            sevWarning,
	"undeclared_tag":           sevWarning,
	"review_due":               sevWarning,
	"not_an_item":              sevInfo,
}

// skippedMessage is the message of a skipped file whose scan gave no reason.
var skippedMessage = map[string]string{
	"bad_front_matter": "the front matter cannot be read; reads skip this file",
	"not_an_item":      "a .md file without front matter is not an item",
}

func rank(s string) int {
	switch s {
	case sevError:
		return 0
	case sevWarning:
		return 1
	}
	return 2
}

// withheld stands in for a value the secret scan flags: a diagnostic never prints it.
const withheld = "?"

func safe(v string) string {
	if secretscan.Sensitive(v) {
		return withheld
	}
	return v
}

// Collect returns every finding of the view, sorted by severity, path, code, field, line, column and
// message, and the number of errors among them.
func Collect(items []*item.Item, skipped []Skipped, g *graph.Graph, opt Options) (findings []Finding, errorCount int) {
	display := func(sec item.Section, path string) string {
		if opt.DisplayPath != nil {
			path = opt.DisplayPath(sec, path)
		}
		return safe(path)
	}
	add := func(code, path, field, msg string, line, col int) {
		findings = append(findings, Finding{Severity: severity[code], Code: code, Path: path, Field: field, Message: msg, Line: line, Column: col})
	}
	for _, s := range skipped {
		msg := s.Reason
		if msg == "" {
			msg = skippedMessage[s.Code]
		}
		add(s.Code, display(s.Section, s.Path), "", msg, s.Line, s.Column)
	}
	byKey := map[string][]*item.Item{}
	for _, it := range items {
		path := display(it.Section, it.Path)
		byKey[it.Key] = append(byKey[it.Key], it)
		for _, p := range it.Problems {
			add(p.Code, path, p.Field, problemMessage(p), p.Line, p.Column)
		}
		h := &it.Header
		if h.ReviewDue(opt.Now) {
			msg := "the review date has passed"
			if h.StaleBad {
				msg = "the review date cannot be read"
			}
			add("review_due", path, "stale_after", msg, 0, 0)
		}
		if opt.HasTags {
			for _, tag := range h.Tags {
				if _, ok := opt.Tags[tag]; ok {
					continue
				}
				if secretscan.Sensitive(tag) {
					add("undeclared_tag", path, "tags", "a tag is not declared in pbrain.toml", 0, 0)
				} else {
					add("undeclared_tag", path, "tags", "tag "+tag+" is not declared in pbrain.toml", 0, 0)
				}
			}
		}
		if it.Section == item.Shared {
			for _, m := range secretscan.Scan(fields(it, opt.Body)) {
				add("secret_suspected", path, m.Field, fmt.Sprintf("%s line %d: %s", m.Field, m.Line, m.Pattern), m.Line, 0)
			}
		}
	}
	for _, same := range byKey {
		if len(same) < 2 {
			continue
		}
		for _, it := range same {
			add("duplicate_id", display(it.Section, it.Path), "", fmt.Sprintf("%d files have the id %s", len(same), safe(it.ID)), 0, 0)
		}
	}
	for _, f := range g.Findings() {
		if f.Code == "personal_reference" && !opt.Personal {
			continue
		}
		add(f.Code, display(f.Item.Section, f.Item.Path), fieldName(f.Field), linkMessage(f), 0, 0)
	}
	slices.SortFunc(findings, func(a, b Finding) int {
		if c := rank(a.Severity) - rank(b.Severity); c != 0 {
			return c
		}
		for _, c := range []int{strings.Compare(a.Path, b.Path), strings.Compare(a.Code, b.Code), strings.Compare(a.Field, b.Field), a.Line - b.Line, a.Column - b.Column} {
			if c != 0 {
				return c
			}
		}
		return strings.Compare(a.Message, b.Message)
	})
	for _, f := range findings {
		if f.Severity == sevError {
			errorCount++
		}
	}
	return findings, errorCount
}

// fields is the projection a shared write scans (writes.md): every stored field and the body.
func fields(it *item.Item, body func(*item.Item) ([]byte, error)) []secretscan.Field {
	h := &it.Header
	out := []secretscan.Field{
		{Name: "title", Value: h.Title},
		{Name: "author", Value: h.Author},
		{Name: "source", Value: h.Source},
		{Name: "verified", Value: h.Verified},
		{Name: "decided", Value: h.Decided},
		{Name: "replaces", Value: strings.Join(h.Replaces, "\n")},
		{Name: "relates", Value: strings.Join(h.Relates, "\n")},
		{Name: "tags", Value: strings.Join(h.Tags, "\n")},
		{Name: "sha256", Value: h.SHA256},
	}
	if body != nil {
		if b, err := body(it); err == nil {
			out = append(out, secretscan.Field{Name: "body", Value: string(b)})
		}
	}
	return out
}

func problemMessage(p item.Problem) string {
	switch p.Code {
	case "bad_field":
		return p.Field + " has the wrong type or cannot be read"
	case "missing_field":
		return p.Field + " is missing"
	case "unknown_value":
		return p.Field + " has an unknown value"
	case "decision_without_decider":
		return "a decision without decided has provisional standing"
	}
	return p.Field
}

func fieldName(f graph.Field) string {
	switch f {
	case graph.FieldReplaces:
		return "replaces"
	case graph.FieldRelates:
		return "relates"
	}
	return "status"
}

func reasonText(r graph.Reason) string {
	switch r {
	case graph.Ambiguous:
		return "the id names more than one item"
	case graph.OtherSection:
		return "the item is in the other section"
	case graph.Standing:
		return "the replacement's standing is lower"
	case graph.DecisionKind:
		return "only a decision can replace a decision"
	case graph.SourceKind:
		return "a source and another kind cannot replace each other"
	case graph.AlreadyLinked:
		return "the item is already replaced through a link"
	case graph.Missing:
		return "the item does not exist"
	case graph.InCycle:
		return "the links form a cycle"
	}
	return "it breaks a replacement rule"
}

func linkMessage(f graph.LinkFinding) string {
	field := fieldName(f.Field)
	target := safe(f.Target)
	switch f.Code {
	case "dangling_link":
		return field + " names an id that is not in this view"
	case "personal_reference":
		return field + " names a personal item"
	case "replace_cycle":
		return "replaces " + target + ": the links form a cycle and are ignored"
	case "replace_fork":
		ids := make([]string, len(f.Successors))
		for i, s := range f.Successors {
			ids[i] = safe(s.ID)
		}
		return "replaced by more than one current item: " + strings.Join(ids, ", ")
	case "legacy_stamp_ignored":
		return "superseded-by " + target + " is ignored: " + reasonText(f.Reason)
	case "replace_invalid":
		return "replaces " + target + " is ignored: " + reasonText(f.Reason)
	}
	return field
}
