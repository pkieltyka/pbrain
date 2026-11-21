package semantic

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"unicode"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/outline"
)

// ChunkerVersion versions the passage rules below: the sizes, the cut rules and the text layout.
const ChunkerVersion = "1"

// Passage sizes in code points (semantic.md, "Passages").
const (
	partMax      = 1800 // a part longer than this is cut
	fillTarget   = 1400 // parts are filled with paragraphs up to this
	cutFloor     = 900  // a cut point must lie beyond this
	prefixMax    = 200  // the title and the heading line are each kept to this
	headingJoint = " > "
)

// Passage is one embedded piece of an item.
type Passage struct {
	Start, End int      // code points in the stored body
	Text       string   // what is embedded
	Digest     [32]byte // SHA-256 of Text
}

// ContentDigest is the SHA-256 of the title's length in bytes as 8 little-endian bytes, the
// title's bytes and the body's bytes: everything an item's passages are cut from. The length
// frames the title, so no title and body can be mistaken for another pair.
func ContentDigest(title string, body []byte) [32]byte {
	h := sha256.New()
	var n [8]byte
	binary.LittleEndian.PutUint64(n[:], uint64(len(title)))
	h.Write(n[:])
	h.Write([]byte(title))
	h.Write(body)
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return d
}

// IndexFingerprint is the lower-case hex SHA-256 of "pbrain-index\n" + ChunkerVersion + "\n" + m.Fingerprint.
func IndexFingerprint(m embed.Model) string {
	d := sha256.Sum256([]byte("pbrain-index\n" + ChunkerVersion + "\n" + m.Fingerprint))
	return hex.EncodeToString(d[:])
}

// Passages cuts one item. It is pure and never fails.
func Passages(title string, body []byte) []Passage {
	rs := []rune(string(body))
	secs := outline.Parse(body)
	var out []Passage
	for i, s := range secs {
		start, end := s.Start, min(s.End, len(rs))
		if start >= end {
			continue
		}
		if s.Level > 0 && blankAfterHeading(rs[start:end]) {
			continue
		}
		prefix := lastRunes(title, prefixMax, false)
		if s.Level > 0 {
			if line := headingLine(secs, i); line != "" {
				prefix += "\n" + line
			}
		}
		for _, part := range cut(rs, start, end) {
			text := strings.TrimSpace(string(rs[part[0]:part[1]]))
			if text == "" {
				continue
			}
			full := prefix + "\n\n" + text
			out = append(out, Passage{Start: part[0], End: part[1], Text: full, Digest: sha256.Sum256([]byte(full))})
		}
	}
	if len(out) == 0 {
		t := lastRunes(title, prefixMax, false)
		out = append(out, Passage{Text: t, Digest: sha256.Sum256([]byte(t))})
	}
	return out
}

// lastRunes keeps the first (or, with tail, the last) n code points of s.
func lastRunes(s string, n int, tail bool) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if tail {
		return string(r[len(r)-n:])
	}
	return string(r[:n])
}

// headingLine is the titles of section i's ancestors and its own, joined, empty ones left out,
// keeping its last 200 code points. An ancestor is the nearest earlier section of each lower level.
func headingLine(secs []Section, i int) string {
	chain := []string{secs[i].Title}
	level := secs[i].Level
	for j := i - 1; j >= 0 && level > 1; j-- {
		if secs[j].Level > 0 && secs[j].Level < level {
			chain = append(chain, secs[j].Title)
			level = secs[j].Level
		}
	}
	var parts []string
	for k := len(chain) - 1; k >= 0; k-- {
		if t := strings.TrimSpace(chain[k]); t != "" {
			parts = append(parts, t)
		}
	}
	return lastRunes(strings.Join(parts, headingJoint), prefixMax, true)
}

// Section is outline's section, named here for brevity.
type Section = outline.Section

// blankAfterHeading reports a section whose text after its heading (one line for ATX, two for
// setext) is all white space.
func blankAfterHeading(rs []rune) bool {
	first := lineEnd(rs, 0)
	rest := rs[first:]
	if !strings.HasPrefix(strings.TrimLeft(string(rs[:first]), " "), "#") {
		rest = rest[lineEnd(rest, 0):] // setext: the underline is the second line
	}
	for _, r := range rest {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// lineEnd is the index just after the line feed that ends the line starting at i, or len(rs).
func lineEnd(rs []rune, i int) int {
	for ; i < len(rs); i++ {
		if rs[i] == '\n' {
			return i + 1
		}
	}
	return len(rs)
}

// cut splits the span [start, end) of rs into parts: one part when it is short enough, else
// paragraphs filled greedily, and any part still too long cut at a line feed, a space, or 1,800.
func cut(rs []rune, start, end int) [][2]int {
	if end-start <= partMax {
		return [][2]int{{start, end}}
	}
	var parts [][2]int
	cur := [2]int{start, start}
	for _, p := range paragraphs(rs, start, end) {
		if cur[1] > cur[0] && (cur[1]-cur[0])+(p[1]-p[0]) > fillTarget {
			parts = append(parts, cur)
			cur = [2]int{p[0], p[0]}
		}
		cur[1] = p[1]
	}
	if cur[1] > cur[0] {
		parts = append(parts, cur)
	}
	var out [][2]int
	for _, p := range parts {
		for p[1]-p[0] > partMax {
			at := cutPoint(rs[p[0] : p[0]+partMax])
			out = append(out, [2]int{p[0], p[0] + at})
			p[0] += at
		}
		out = append(out, p)
	}
	return out
}

// cutPoint is where a too-long part is cut, within its first 1,800 code points: after the last line
// feed beyond code point 900, else after the last space beyond it, else at 1,800.
func cutPoint(window []rune) int {
	for i := len(window) - 1; i >= cutFloor; i-- {
		if window[i] == '\n' {
			return i + 1
		}
	}
	for i := len(window) - 1; i >= cutFloor; i-- {
		if window[i] == ' ' {
			return i + 1
		}
	}
	return len(window)
}

// paragraphs tiles [start, end) of rs into paragraphs: one ends where a non-blank line follows a run
// of blank lines, outside a fenced block (architecture 7.4's fence rule). Each keeps its trailing
// blank lines.
func paragraphs(rs []rune, start, end int) [][2]int {
	var out [][2]int
	pstart := start
	sawBlank := false
	var fence rune
	fenceLen := 0
	for i := start; i < end; {
		j := min(lineEnd(rs, i), end)
		line := strings.TrimRight(string(rs[i:j]), "\r\n")
		blank := strings.TrimSpace(line) == ""
		if fenceLen == 0 && !blank && sawBlank && i > pstart {
			out = append(out, [2]int{pstart, i})
			pstart = i
		}
		if fenceLen == 0 {
			sawBlank = blank
		}
		mark, n := fenceRun(line)
		switch {
		case fenceLen > 0:
			if mark == fence && n >= fenceLen && strings.TrimSpace(strings.TrimLeft(line, " ")[n:]) == "" {
				fenceLen = 0
				sawBlank = false
			}
		case n >= 3:
			fence, fenceLen = mark, n
		}
		i = j
	}
	if pstart < end {
		out = append(out, [2]int{pstart, end})
	}
	return out
}

// fenceRun is the fence character and run length opening line, after at most three spaces.
func fenceRun(line string) (rune, int) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || trimmed == "" {
		return 0, 0
	}
	c := rune(trimmed[0])
	if c != '\x60' && c != '~' {
		return 0, 0
	}
	n := 0
	for n < len(trimmed) && rune(trimmed[n]) == c {
		n++
	}
	return c, n
}
