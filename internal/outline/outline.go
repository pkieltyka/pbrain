// Package outline: exported surface from the pbrain specs; bodies are filled in by later phases.
package outline

type Section struct {
	Level              int    // 1 to 6; 0 for text before the first heading, or a body with no heading
	Title              string // the heading text without its marks, trimmed; "" at level 0
	Start              int    // code-point offset of the heading line's first character; 0 at level 0
	End                int    // where the next heading line starts, or the body's length: the section's own text
	Tree               int    // where the section with its subsections ends: the next heading of the same or a higher level
	ByteStart, ByteEnd int    // the own-text span [Start, End) in bytes
}

func Parse(body []byte) []Section    { panic("not implemented") } // in body order; never fails; nil for a body with no non-space character
func At(secs []Section, pos int) int { panic("not implemented") } // index of the section whose [Start, End) holds pos, or -1
