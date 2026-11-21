// Package query: exported surface from the pbrain specs; bodies are filled in by later phases.
package query

import "time"

type Token struct {
	Word       string // folded with item.Fold
	Start, End int    // code points into the text, End exclusive
}

// Tokens returns the maximal runs of letters and digits (unicode.IsLetter || unicode.IsDigit), folded.
func Tokens(text string) []Token { panic("not implemented") }

// Scanner tokenises without allocating per token, for bodies: Next fills tok and returns false at the end.
// The folded word is valid until the next call.
type Scanner struct { /* reusable buffers */
}

func (s *Scanner) Reset(b []byte)                               { panic("not implemented") }
func (s *Scanner) Next() (word []byte, start, end int, ok bool) { panic("not implemented") }

type ClauseKind uint8 // Word, Prefix, Phrase
type Clause struct {
	Kind   ClauseKind
	Words  []string // folded; one for Word and Prefix, two or more for Phrase
	Label  string   // as written, for the per-clause counts: a word, "run*", or "\"job queue\""
	Quoted bool     // written in double quotes (a one-word phrase included): required in a partial match (section 4.6)
}

// Parse: errors are *errs.Error{Code: "usage"}: "unclosed quote" or "the query has no words".
func Parse(q string) ([]Clause, error) { panic("not implemented") }

type Window struct {
	Since, Until       time.Time
	HasSince, HasUntil bool
	UntilIncl          bool // true for an instant or a span; false when Until is the midnight after a day
}

// ParseWindow parses the two flag values ("" = not given) at one instant in one zone.
// Errors: *errs.Error{Code: "usage"} naming the flag: "--since: not a date, instant, today, yesterday or span",
// or "--since is after --until".
func ParseWindow(since, until string, now time.Time, loc *time.Location) (Window, error) {
	panic("not implemented")
}
func (w Window) Set() bool                 { panic("not implemented") }
func (w Window) Contains(t time.Time) bool { panic("not implemented") } // only for a known created; callers exclude unknown first
