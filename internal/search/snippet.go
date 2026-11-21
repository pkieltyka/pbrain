package search

import (
	"bytes"
	"cmp"
	"slices"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/outline"
)

type occurrence struct {
	start, end int
	clause     int
}

func pageHit(c Corpus, m *matcher, row rankedHit, req Request) (Hit, error) {
	h := row.hit
	body, err := c.Body(h.Item)
	if err != nil {
		return Hit{}, bodyError()
	}
	if req.BM25 {
		for _, frequency := range row.source.tf {
			h.Occurrences += frequency
		}
	} else {
		count := func(_, _, _ int) bool {
			h.Occurrences++
			return false
		}
		m.scan([]byte(h.Item.Header.Title), count)
		for _, tag := range h.Item.Header.Tags {
			m.scan([]byte(tag), count)
		}
	}
	if len(body) == 0 {
		return h, nil
	}

	l := req.Snippet
	if l <= 0 {
		l = 200
	}
	n := h.Item.Layout.BodyChars
	if h.Item.Layout.BodyLine == 0 {
		n = utf8.RuneCount(body)
	}
	if h.Semantic {
		// The snippet lies inside the passage only (section 6.1 step 7); an empty span has none.
		if !req.BM25 {
			m.scan(body, func(_, _, _ int) bool {
				h.Occurrences++
				return false
			})
		}
		if row.start == row.end {
			return h, nil
		}
		return withSnippet(h, body, row.start, min(row.end, row.start+l), n, req), nil
	}
	var start int
	if n <= l {
		// The whole body already fits. There is no stretch to choose.
		if !req.BM25 {
			m.scan(body, func(_, _, _ int) bool {
				h.Occurrences++
				return false
			})
		}
	} else {
		var found []occurrence
		m.scan(body, func(clause, start, end int) bool {
			if !req.BM25 {
				h.Occurrences++
			}
			if end-start <= l {
				found = append(found, occurrence{start, end, clause})
			}
			return false
		})
		start = bestStretch(found, l, m.plan.size)
	}
	return withSnippet(h, body, start, min(n, start+l), n, req), nil
}

// withSnippet sets the snippet of [start, end) with its line, section and context lines.
func withSnippet(h Hit, body []byte, start, end, n int, req Request) Hit {
	from, to := byteSpan(body, start, end)
	h.Snippet = &Snippet{
		Text: string(body[from:to]), Start: start, End: end, BodyChars: n, Long: n > req.LongBody,
		Line: h.Item.LineOf(body, start),
	}
	if h.Snippet.Long {
		sections := outline.Parse(body)
		if i := outline.At(sections, start); i >= 0 {
			h.Section = sections[i].Title
		}
	}
	if req.Context > 0 {
		h.Snippet.Context = contextLines(body, from, to, h.Snippet.Line, req.Context)
	}
	return h
}

// bestStretch chooses the earliest start with the most distinct clauses in its span.
// Each occurrence fits in the window, so every expired occurrence has already entered it.
func bestStretch(found []occurrence, length, clauseCount int) int {
	if len(found) == 0 {
		return 0
	}
	byStart := slices.Clone(found)
	slices.SortFunc(byStart, func(a, b occurrence) int { return cmp.Compare(a.start, b.start) })
	slices.SortFunc(found, func(a, b occurrence) int { return cmp.Compare(a.end, b.end) })

	counts := make([]int, clauseCount)
	var best, score, enters, leaves, active int
	for i := 0; i <= len(found); i++ {
		at := 0
		if i > 0 {
			at = max(0, found[i-1].end-length)
			if at == 0 || i > 1 && at == found[i-2].end-length {
				continue
			}
		}
		for enters < len(found) && found[enters].end <= at+length {
			clause := found[enters].clause
			if counts[clause] == 0 {
				active++
			}
			counts[clause]++
			enters++
		}
		for leaves < len(byStart) && byStart[leaves].start < at {
			clause := byStart[leaves].clause
			counts[clause]--
			if counts[clause] == 0 {
				active--
			}
			leaves++
		}
		if active > score {
			score, best = active, at
		}
	}
	return best
}

func byteSpan(body []byte, start, end int) (from, to int) {
	for cp := 0; cp < end && to < len(body); cp++ {
		if cp == start {
			from = to
		}
		_, n := utf8.DecodeRune(body[to:])
		to += n
	}
	return from, to
}

// contextLines uses byte positions in the body. Its first focal line always
// contains from, and its last contains to-1, even when that character is LF.
func contextLines(body []byte, from, to, line, limit int) []ContextLine {
	first := bytes.LastIndexByte(body[:from], '\n') + 1
	var out []ContextLine
	for i, next := 1, first; i <= limit && next > 0; i++ {
		at := bytes.LastIndexByte(body[:next-1], '\n') + 1
		end, _ := bodyLine(body, at)
		out = append(out, ContextLine{Line: line - i, Text: string(body[at:end])})
		next = at
	}
	slices.Reverse(out)

	next := first
	for next < to {
		at := next
		end, following := bodyLine(body, at)
		left, right := max(from, at), min(to, end)
		// A stretch may begin on a line terminator and have no text on that line.
		left = min(left, right)
		out = append(out, ContextLine{
			Line: line, Text: string(body[left:right]), Focal: true,
			Before: from > at, After: to < end,
		})
		next = following
		line++
	}
	for range limit {
		if next >= len(body) {
			break
		}
		end, following := bodyLine(body, next)
		out = append(out, ContextLine{Line: line, Text: string(body[next:end])})
		line++
		next = following
	}
	return out
}

// bodyLine returns the content end and the next physical line's start.
func bodyLine(body []byte, start int) (end, next int) {
	n := bytes.IndexByte(body[start:], '\n')
	if n < 0 {
		return len(body), len(body)
	}
	end = start + n
	next = end + 1
	if end > start && body[end-1] == '\r' {
		end--
	}
	return end, next
}
