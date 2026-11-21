package search

import (
	"bytes"

	"github.com/pkieltyka/pbrain/internal/query"
)

type clausePattern struct {
	index int
	words []string
}

type matchPlan struct {
	size     int
	words    map[string][]int
	prefixes []clausePattern
	phrases  []clausePattern
	ringSize int
}

func plan(clauses []query.Clause) matchPlan {
	p := matchPlan{size: len(clauses), words: make(map[string][]int)}
	for i, clause := range clauses {
		switch clause.Kind {
		case query.Word:
			word := clause.Words[0]
			p.words[word] = append(p.words[word], i)
		case query.Prefix:
			p.prefixes = append(p.prefixes, clausePattern{i, clause.Words})
		case query.Phrase:
			p.phrases = append(p.phrases, clausePattern{i, clause.Words})
			p.ringSize = max(p.ringSize, len(clause.Words))
		}
	}
	return p
}

type phraseToken struct {
	word  []byte
	start int
}

type matcher struct {
	plan    *matchPlan
	scanner query.Scanner
	ring    []phraseToken
}

func (p *matchPlan) matcher() matcher {
	return matcher{plan: p, ring: make([]phraseToken, p.ringSize)}
}

// scan matches one field at a time. A visit returning true ends this field's scan.
func (m *matcher) scan(data []byte, visit func(clause, start, end int) bool) int {
	m.scanner.Reset(data)
	n := 0
	for {
		word, start, end, ok := m.scanner.Next()
		if !ok {
			return n
		}
		n++
		for _, i := range m.plan.words[string(word)] {
			if visit(i, start, end) {
				return n
			}
		}
		for _, p := range m.plan.prefixes {
			if bytes.HasPrefix(word, []byte(p.words[0])) && visit(p.index, start, end) {
				return n
			}
		}
		if len(m.ring) == 0 {
			continue
		}
		// The scanner can overwrite its folded bytes on the next token.
		slot := &m.ring[(n-1)%len(m.ring)]
		slot.word = append(slot.word[:0], word...)
		slot.start = start
		for _, p := range m.plan.phrases {
			if n >= len(p.words) && m.phraseAt(n, p.words) {
				first := m.ring[(n-len(p.words))%len(m.ring)].start
				if visit(p.index, first, end) {
					return n
				}
			}
		}
	}
}

func (m *matcher) phraseAt(end int, words []string) bool {
	first := end - len(words)
	for i, word := range words {
		if !bytes.Equal(m.ring[(first+i)%len(m.ring)].word, []byte(word)) {
			return false
		}
	}
	return true
}

type fields uint8

const (
	inTitle fields = 1 << iota
	inTags
	inBody
)

func (m *matcher) record(c Corpus, hit *match, bm25 bool) error {
	header := &hit.item.Header
	hit.mask = make([]fields, m.plan.size)
	fullCount := bm25 && hit.eligible
	if fullCount {
		hit.tf = make([]int, m.plan.size)
	}

	left := m.plan.size
	scan := func(value []byte, field fields, stop bool) {
		n := m.scan(value, func(i, _, _ int) bool {
			if hit.mask[i] == 0 {
				left--
			}
			hit.mask[i] |= field
			if fullCount {
				hit.tf[i]++
			}
			return stop && left == 0
		})
		if fullCount {
			hit.length += n
		}
	}
	scan([]byte(header.Title), inTitle, false)
	for _, tag := range header.Tags {
		scan([]byte(tag), inTags, false)
	}
	if fullCount || left > 0 {
		body, err := c.Body(hit.item)
		if err != nil {
			return bodyError()
		}
		scan(body, inBody, !fullCount)
	}
	if left == 0 {
		hit.class = matchClass(hit.mask)
	}
	return nil
}

func matchClass(mask []fields) int {
	class := 1
	for _, matches := range mask {
		if matches&(inTitle|inTags) == 0 {
			return 3
		}
		if matches&inTitle == 0 {
			class = 2
		}
	}
	return class
}
