// Package render: exported surface from the pbrain specs; bodies are filled in by later phases.
package render

import "encoding/json"

type Doc struct {
	Head   []Part
	Units  []Unit
	Offset int
	Total  int
	Foot   func(st State) Part
	Hints  []Part
}
type State struct{ Shown, More, Next int }
type Unit struct {
	Main   Part
	Alts   []Part // complete replacements for Main, richest context first
	Extras []Part // related rows
}
type Part struct {
	Text  string
	JSON  any
	Marks []Mark
}
type Mark struct {
	Start, End int // half-open code-point span in Text
	Style      Style
}
type Style uint8

const (
	Bold Style = iota
	Cyan
)

type Format uint8

const (
	Text Format = iota
	JSON
	TextColor
)

type Object map[string]any
type Envelope struct {
	Fields Object
	Rows   string // results, history, outline, items, tags, findings, rules
}
type BodyDoc struct {
	Doc    Doc           // Offset and Total count code points; Units, Hints and Foot are unused
	Body   []byte        // valid UTF-8, same version as the header used by Doc
	LineAt func(int) int // one fresh item.LineCursor.At, called with nondecreasing offsets
	Extras []Part        // related rows for this one item
	Tail   string        // fixed linked-source line, without its final newline
}

func Fit(d *Doc, limit int, f Format) ([]byte, error)         { panic("not implemented") }
func FitBody(d *BodyDoc, limit int, f Format) ([]byte, error) { panic("not implemented") }
func OneLine(value string, maxChars int) string               { panic("not implemented") }
func ExactPath(path string) (display string, encoded json.RawMessage, ok bool) {
	panic("not implemented")
}
