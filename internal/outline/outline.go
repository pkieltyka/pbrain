// Package outline finds Markdown headings and their spans in the stored body.
package outline

import (
	"bytes"
	"sort"
	"unicode/utf8"
)

type Section struct {
	Level              int    // 1 to 6; 0 for non-blank text before the first heading
	Title              string // heading text without its marks
	Start, End         int    // own-text span in code points
	Tree               int    // end including subsections, in code points
	ByteStart, ByteEnd int    // own-text span in bytes
}

// Parse returns the sections in body order. A whitespace-only body has no sections.
func Parse(body []byte) []Section {
	var sections []Section
	var prev []byte
	var prevByte, prevRune int
	var fence byte
	var fenceLen int
	cp := 0
	for start := 0; start < len(body); {
		n := bytes.IndexByte(body[start:], '\n')
		end := len(body)
		if n >= 0 {
			end = start + n + 1
		}
		raw := body[start:end]
		line := raw
		if n >= 0 {
			line = bytes.TrimSuffix(raw[:len(raw)-1], []byte{'\r'})
		}

		trimmed := indent(line)
		mark, count := fenceRun(trimmed)
		level, title := 0, []byte(nil)
		headingByte, headingRune := start, cp
		switch {
		case fenceLen != 0:
			if mark == fence && count >= fenceLen && len(bytes.Trim(trimmed[count:], " ")) == 0 {
				fenceLen = 0
			}
			prev = nil
		case count >= 3:
			fence, fenceLen = mark, count
			prev = nil
		default:
			level, title = atx(trimmed)
			if level == 0 && prev != nil {
				if level = underline(trimmed); level != 0 {
					title = bytes.TrimSpace(prev)
					headingByte, headingRune = prevByte, prevRune
				}
			}
			prev = nil
			if level == 0 && setextText(trimmed) {
				prev, prevByte, prevRune = trimmed, start, cp
			}
		}
		if level != 0 {
			if len(sections) == 0 && len(bytes.TrimSpace(body[:headingByte])) != 0 {
				sections = append(sections, Section{})
			}
			sections = append(sections, Section{
				Level: level, Title: string(title), Start: headingRune, ByteStart: headingByte,
			})
		}
		cp += utf8.RuneCount(raw)
		start = end
	}

	if len(sections) == 0 {
		if len(bytes.TrimSpace(body)) == 0 {
			return nil
		}
		return []Section{{End: cp, Tree: cp, ByteEnd: len(body)}}
	}
	var ancestors []int
	for i := range sections {
		s := &sections[i]
		s.End, s.ByteEnd, s.Tree = cp, len(body), cp
		if i+1 < len(sections) {
			s.End, s.ByteEnd = sections[i+1].Start, sections[i+1].ByteStart
		}
		for len(ancestors) != 0 {
			j := ancestors[len(ancestors)-1]
			if sections[j].Level < s.Level {
				break
			}
			sections[j].Tree = s.Start
			ancestors = ancestors[:len(ancestors)-1]
		}
		if s.Level == 0 {
			s.Tree = s.End
		} else {
			ancestors = append(ancestors, i)
		}
	}
	return sections
}

// At returns the index whose own-text span contains pos, or -1.
func At(secs []Section, pos int) int {
	i := sort.Search(len(secs), func(i int) bool { return secs[i].End > pos })
	if i == len(secs) || pos < secs[i].Start {
		return -1
	}
	return i
}

func indent(line []byte) []byte {
	for n := 0; n < 3 && len(line) > 0 && line[0] == ' '; n++ {
		line = line[1:]
	}
	return line
}

func fenceRun(line []byte) (byte, int) {
	if len(line) == 0 || line[0] != '`' && line[0] != '~' {
		return 0, 0
	}
	mark := line[0]
	n := 1
	for n < len(line) && line[n] == mark {
		n++
	}
	return mark, n
}

func atx(line []byte) (int, []byte) {
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n < len(line) && line[n] != ' ' && line[n] != '\t' {
		return 0, nil
	}
	title := bytes.TrimRight(line[n:], " \t")
	beforeHashes := bytes.TrimRight(title, "#")
	if len(beforeHashes) < len(title) && len(beforeHashes) != 0 && beforeHashes[len(beforeHashes)-1] == ' ' {
		title = beforeHashes
	}
	return n, bytes.TrimSpace(title)
}

func underline(line []byte) int {
	line = bytes.TrimRight(line, " ")
	if len(line) == 0 {
		return 0
	}
	switch line[0] {
	case '=':
		if len(bytes.Trim(line, "=")) == 0 {
			return 1
		}
	case '-':
		if len(line) >= 2 && len(bytes.Trim(line, "-")) == 0 {
			return 2
		}
	}
	return 0
}

// line has already lost up to three indentation spaces.
func setextText(line []byte) bool {
	if len(bytes.TrimSpace(line)) == 0 {
		return false
	}
	switch line[0] {
	case ' ', '\t', '-', '*', '+', '>', '|':
		return false
	}
	n := 0
	for n < len(line) && line[n] >= '0' && line[n] <= '9' {
		n++
	}
	return n == 0 || n == len(line) || line[n] != '.' && line[n] != ')'
}
