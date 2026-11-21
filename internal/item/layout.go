package item

import "unicode/utf8"

// Measure counts one valid UTF-8 file at the body offset returned by Parse.
func Measure(data []byte, bodyAt int64) Layout {
	layout := Layout{BodyLine: 1}
	for i, b := range data {
		beforeBody := int64(i) < bodyAt
		if b == '\n' {
			layout.FileLines++
			if beforeBody {
				layout.BodyLine++
			}
		}
		if utf8.RuneStart(b) {
			layout.FileChars++
			if !beforeBody {
				layout.BodyChars++
			}
		}
	}
	if len(data) != 0 && data[len(data)-1] != '\n' {
		layout.FileLines++
	}
	if bodyAt == int64(len(data)) {
		layout.BodyLine = layout.FileLines + 1
	}
	return layout
}

// LineOf returns the file line at a single stored-body offset in code points.
func (it *Item) LineOf(body []byte, cp int) int {
	return it.Lines(body).At(cp)
}

// Lines returns a cursor mapping ascending body offsets in one pass.
func (it *Item) Lines(body []byte) *LineCursor {
	return &LineCursor{body: body, firstLine: it.Layout.BodyLine, line: it.Layout.BodyLine}
}

// LineCursor walks a measured item's body.
type LineCursor struct {
	body         []byte
	firstLine    int
	line         int
	bytePosition int
	codePoints   int
	bytesStepped int
}

// At maps an offset to its file line. A backwards seek restarts the walk.
func (c *LineCursor) At(cp int) int {
	if c.firstLine == 0 {
		return 0
	}
	cp = max(0, cp)
	if cp < c.codePoints {
		c.bytePosition, c.codePoints, c.line = 0, 0, c.firstLine
	}
	for c.codePoints < cp && c.bytePosition < len(c.body) {
		r, n := utf8.DecodeRune(c.body[c.bytePosition:])
		c.bytePosition += n
		c.bytesStepped += n
		c.codePoints++
		if r == '\n' {
			c.line++
		}
	}
	return c.line
}
