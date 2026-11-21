// Package search ranks items in one command view and selects supporting passages.
package search

import (
	"context"
	"time"

	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/query"
)

// Corpus and Run are fixed (architecture 8.5).
type Corpus interface {
	Items() []*item.Item
	Body(*item.Item) ([]byte, error)
}

type Filters struct {
	Kinds  []item.Kind  // any of; empty = every kind
	Tags   []string     // all of; normalised with item.NormTag
	Status *item.Status // the recorded status; nil = either
	Window query.Window
	All    bool // replaced items too
}
type Request struct {
	Clauses  []query.Clause
	Filters  Filters
	Limit    int       // 1..50, already checked by cli
	Offset   int       // >= 0
	Now      time.Time // the command's instant
	Snippet  int       // stretch length in code points: 200
	LongBody int       // get's default limit (pbrain.toml get.max_chars, else 6,000)
	Text     string    // the query as written, arguments joined by one space (architecture 8.5)
	Semantic Semantic  // nil without --semantic (section 6)
	BM25     bool      // --bm25: add the partial tier (section 4.6; architecture 8.8)
	Context  int       // --context: 1..3 lines each side, 0 without it (section 4.7; architecture 8.4)
}

type Result struct { // architecture 8.5's fields, unchanged
	Total    int // exact, found by meaning and partial together
	Offset   int
	Hits     []Hit
	Replaced int
	Outside  int
	Alone    []ClauseCount // only when Total is 0
	Sem      SemanticState // architecture 8.5; zero without --semantic
}

type Hit struct {
	Item        *item.Item
	Class       int // 1, 2 or 3; 0 for a semantic or partial hit
	Replaced    bool
	Semantic    bool     // found only by meaning (architecture 8.5)
	Partial     bool     // a partial match (section 4.6); never together with Semantic
	Matched     int      // a partial match: how many of the query's distinct clauses it matches
	Clauses     int      // a partial match: the query's number of distinct clauses
	Section     string   // the heading named with the snippet: set only when Snippet.Long (section 4.5)
	Snippet     *Snippet // nil for an empty body, or an added hit with an empty span
	Occurrences int      // the query's distinct clauses in title, tags and body, by tf's rules (section 4.8)
}
type Snippet struct {
	Text      string        // the stretch as stored: line breaks kept; render flattens and adds "…"
	Start     int           // code points into the body; the get --offset value
	End       int           // exclusive
	BodyChars int           // the body's length in code points
	Long      bool          // BodyChars > LongBody: Hit.Section is set
	Line      int           // the file line that holds Start (item.LineOf; section 4.7)
	Context   []ContextLine // with Request.Context > 0, in file order; nil otherwise (section 4.7)
}

// ContextLine is one line of a snippet's context (architecture 8.4).
type ContextLine struct {
	Line int    // its file line
	Text string // without its line feed and without a carriage return before it;
	// for a line of the stretch, only the part inside the stretch
	Focal  bool // a line of the stretch
	Before bool // focal: its line starts before the part
	After  bool // focal: its line goes on after the part
}
type ClauseCount struct {
	Label string // as written; render cuts it at 40
	Count int
}

type SemanticState struct {
	Asked, Ready bool
	Reason       NotReady // why, when Asked and not Ready
	Missing      int      // items of the view that were not compared
}

// Semantic is the second source of candidates. The semantic package implements it.
type Semantic interface {
	// Score compares the query text with each eligible item that a usable index covers.
	// A missing model or index is not an error: it is Scores.Ready == false.
	Score(ctx context.Context, text string, eligible []*item.Item) (Scores, error)
}

type Scores struct {
	Ready   bool
	Reason  NotReady
	Missing int
	Cutoff  float32       // the model's cut-off
	Hits    []SemanticHit // one per eligible, covered item whose closest passage is at or just under the cut-off; in no order
}

type SemanticHit struct {
	Item       *item.Item
	Score      float32
	Start, End int // code-point span of the closest passage in the stored body
}

type NotReady uint8

const (
	NoModel NotReady = iota + 1
	UnsupportedModel
	ModelAbsent
	CacheExposed
	NoIndex
	ModelFailed
)
