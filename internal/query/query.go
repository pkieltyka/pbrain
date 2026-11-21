// Package query tokenizes search text and parses clauses and time filters.
package query

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/item"
)

type Token struct {
	Word       string // folded with item.Fold
	Start, End int    // code points into the text, End exclusive
}

// Tokens returns the maximal runs of letters and digits, folded.
func Tokens(text string) []Token {
	var tokens []Token
	for _, token := range words(text) {
		tokens = append(tokens, token.Token)
	}
	return tokens
}

type writtenWord struct {
	Token
	byteStart, byteEnd int
}

// words maps original labels through the canonical fold.
func words(text string) []writtenWord {
	var scanner Scanner
	scanner.Reset([]byte(text))
	var out []writtenWord
	for {
		start, end, ok := scanner.nextRun()
		if !ok {
			return out
		}
		raw := text[scanner.runStart:scanner.at]
		out = append(out, writtenWord{Token{Word: item.Fold(raw), Start: start, End: end}, scanner.runStart, scanner.at})
	}
}

// Scanner walks a body with reusable storage. Next's bytes are valid until the next call.
type Scanner struct {
	data     []byte
	folded   []byte
	at       int
	cp       int
	runStart int
}

func (s *Scanner) Reset(b []byte) {
	s.data, s.at, s.cp, s.runStart = b, 0, 0, 0
}

func (s *Scanner) Next() (word []byte, start, end int, ok bool) {
	start, end, ok = s.nextRun()
	if !ok {
		return nil, 0, 0, false
	}
	raw := s.data[s.runStart:s.at]
	// Match item.Fold's rune-by-rune lowercase rule without allocating per word.
	for i := 0; i < len(raw); {
		r, n := rune(raw[i]), 1
		if r >= utf8.RuneSelf {
			r, n = utf8.DecodeRune(raw[i:])
		}
		if (r >= 'A' && r <= 'Z') || r >= utf8.RuneSelf && unicode.ToLower(r) != r {
			s.folded = append(s.folded[:0], raw[:i]...)
			for i < len(raw) {
				r, n := utf8.DecodeRune(raw[i:])
				s.folded = utf8.AppendRune(s.folded, unicode.ToLower(r))
				i += n
			}
			return s.folded, start, end, true
		}
		i += n
	}
	return raw, start, end, true
}

func (s *Scanner) nextRun() (start, end int, ok bool) {
	for s.at < len(s.data) {
		word, n := wordRune(s.data[s.at:])
		if word {
			break
		}
		s.at += n
		s.cp++
	}
	s.runStart, start = s.at, s.cp
	for s.at < len(s.data) {
		word, n := wordRune(s.data[s.at:])
		if !word {
			break
		}
		s.at += n
		s.cp++
	}
	return start, s.cp, s.cp > start
}

func wordRune(data []byte) (word bool, length int) {
	b := data[0]
	if b < utf8.RuneSelf {
		letter := b | 0x20
		return letter >= 'a' && letter <= 'z' || b >= '0' && b <= '9', 1
	}
	r, n := utf8.DecodeRune(data)
	return unicode.IsLetter(r) || unicode.IsDigit(r), n
}

type ClauseKind uint8

const (
	Word ClauseKind = iota
	Prefix
	Phrase
)

type Clause struct {
	Kind   ClauseKind
	Words  []string // folded; one for Word and Prefix, two or more for Phrase
	Label  string   // as written, for the per-clause counts
	Quoted bool     // quoted clauses are required in a partial match
}

// Parse reads exact search clauses in first-occurrence order.
func Parse(q string) ([]Clause, error) {
	type clauseKey struct {
		kind  ClauseKind
		words string
	}
	var clauses []Clause
	known := make(map[clauseKey]int)
	add := func(c Clause) {
		k := clauseKey{c.Kind, strings.Join(c.Words, "\x00")}
		if i, ok := known[k]; ok {
			clauses[i].Quoted = clauses[i].Quoted || c.Quoted
		} else {
			known[k] = len(clauses)
			clauses = append(clauses, c)
		}
	}
	for q != "" {
		r, n := utf8.DecodeRuneInString(q)
		switch {
		case unicode.IsSpace(r):
			q = q[n:]
		case r == '"':
			end := strings.IndexByte(q[1:], '"')
			if end < 0 {
				return nil, usage("unclosed quote")
			}
			end += 2
			phrase := words(q[1 : end-1])
			if len(phrase) > 0 {
				c := Clause{Kind: Word, Words: make([]string, len(phrase)), Label: q[:end], Quoted: true}
				if len(phrase) > 1 {
					c.Kind = Phrase
				}
				for i, word := range phrase {
					c.Words[i] = word.Word
				}
				add(c)
			}
			q = q[end:]
		default:
			end := strings.IndexFunc(q, func(r rune) bool { return r == '"' || unicode.IsSpace(r) })
			if end < 0 {
				end = len(q)
			}
			term := q[:end]
			base := strings.TrimRight(term, "*")
			tokens := words(base)
			for i, tok := range tokens {
				c := Clause{Kind: Word, Words: []string{tok.Word}, Label: term[tok.byteStart:tok.byteEnd]}
				if len(base) < len(term) && i+1 == len(tokens) {
					c.Kind = Prefix
					c.Label = term[tok.byteStart:]
				}
				add(c)
			}
			q = q[end:]
		}
	}
	if len(clauses) == 0 {
		return nil, usage("the query has no words")
	}
	return clauses, nil
}

func usage(message string) error {
	return &errs.Error{Code: "usage", Message: message, Exit: 2}
}
