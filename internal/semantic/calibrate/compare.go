package main

import (
	"cmp"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/outline"
	"github.com/pkieltyka/pbrain/internal/query"
	"github.com/pkieltyka/pbrain/internal/render"
	"github.com/pkieltyka/pbrain/internal/search"
)

// These arms are research controls. The strict and partial arms instead run
// the product entry point, including its eligibility, ordering and fitter.
type comparison uint8

const (
	byCount comparison = iota
	byBM25
	withStemming
	withHeadingWeight
)

type rankedItem struct {
	item                 *item.Item
	body                 []byte
	sections             []outline.Section
	length               int
	tf, headings         []int
	matched, occurrences int
	snippetStart         int
	snippetLength        int
	score                float64
}

func (s session) compare(text string, mode comparison) (attempt, error) {
	start := time.Now()
	items, err := s.rankComparison(text, mode)
	if err != nil {
		return attempt{}, err
	}
	out, err := s.comparisonPage(items)
	if err != nil {
		return attempt{}, err
	}
	return decodePage(response{out: out, elapsed: time.Since(start)})
}

func (s session) rankComparison(text string, mode comparison) ([]rankedItem, error) {
	clauses, err := query.Parse(text)
	if err != nil {
		return nil, err
	}
	candidates, n, avg, df, err := s.candidates(clauses, mode)
	if err != nil {
		return nil, err
	}
	for i := range candidates {
		c := &candidates[i]
		c.score = float64(c.matched)
		if mode == byCount {
			continue
		}
		c.score = 0
		norm := 1.2 * (0.25 + 0.75*float64(c.length)/avg)
		for j, tf := range c.tf {
			if mode == withHeadingWeight {
				tf += c.headings[j]
			}
			if tf == 0 {
				continue
			}
			idf := math.Log(1 + (float64(n-df[j])+0.5)/(float64(df[j])+0.5))
			c.score += idf * float64(tf) * 2.2 / (float64(tf) + norm)
		}
	}
	slices.SortFunc(candidates, compareRanked)
	return candidates, nil
}

func compareRanked(a, b rankedItem) int {
	if order := cmp.Compare(b.score, a.score); order != 0 {
		return order
	}
	return newestItem(a.item, b.item)
}

func newestItem(a, b *item.Item) int {
	if a.Header.CreatedKnown != b.Header.CreatedKnown {
		if a.Header.CreatedKnown {
			return -1
		}
		return 1
	}
	if order := b.Header.Created.Compare(a.Header.Created); order != 0 {
		return order
	}
	if order := cmp.Compare(a.Key, b.Key); order != 0 {
		return order
	}
	return cmp.Compare(a.Path, b.Path)
}

func (s session) candidates(clauses []query.Clause, mode comparison) ([]rankedItem, int, float64, []int, error) {
	filters := search.Filters{} // The fixture's normal current view.
	df := make([]int, len(clauses))
	var matches []rankedItem
	n, words := 0, 0
	for _, it := range s.corpus.cat.Items() {
		if err := s.ctx.Err(); err != nil {
			return nil, 0, 0, nil, err
		}
		if filters.Admit(it, s.corpus.graph.Current(it)) != search.In {
			continue
		}
		body, err := s.corpus.cat.Body(it)
		if err != nil {
			return nil, 0, 0, nil, err
		}
		c := rankItem(it, body, clauses, mode)
		n++
		words += c.length
		if c.matched == 0 {
			continue
		}
		for i, tf := range c.tf {
			if tf > 0 {
				df[i]++
			}
		}
		matches = append(matches, c)
	}
	avg := 1.0
	if words > 0 {
		avg = float64(words) / float64(n)
	}
	return matches, n, avg, df, nil
}

func rankItem(it *item.Item, body []byte, clauses []query.Clause, mode comparison) rankedItem {
	c := rankedItem{item: it, body: body, tf: make([]int, len(clauses)), snippetLength: 200}
	bodyTokens := query.Tokens(string(body))
	fields := [][]query.Token{query.Tokens(it.Header.Title), bodyTokens}
	for _, tag := range it.Header.Tags {
		fields = append(fields, query.Tokens(tag))
	}
	const width = 200
	bodyChars := utf8.RuneCount(body)
	var found []search.SnippetOccurrence
	for fi, tokens := range fields {
		c.length += len(tokens)
		for i, clause := range clauses {
			var visit func(int, int)
			if fi == 1 && bodyChars > width {
				visit = func(start, end int) {
					found = append(found, search.SnippetOccurrence{Start: start, End: end, Clause: i})
				}
			}
			c.tf[i] += clauseMatches(tokens, clause, mode, visit)
		}
	}
	for _, tf := range c.tf {
		if tf > 0 {
			c.matched++
			c.occurrences += tf
		}
	}
	c.snippetStart = search.SnippetStart(bodyChars, width, len(clauses), found)
	if mode == withHeadingWeight {
		c.headings = make([]int, len(clauses))
		c.sections = outline.Parse(body)
		for _, section := range c.sections {
			tokens := query.Tokens(section.Title)
			for i, clause := range clauses {
				c.headings[i] += clauseMatches(tokens, clause, mode, nil)
			}
		}
	}
	return c
}

func clauseMatches(tokens []query.Token, clause query.Clause, mode comparison, visit func(start, end int)) (count int) {
	for i := 0; i+len(clause.Words) <= len(tokens); i++ {
		if !tokensMatch(tokens[i:i+len(clause.Words)], clause, mode) {
			continue
		}
		count++
		if visit != nil {
			visit(tokens[i].Start, tokens[i+len(clause.Words)-1].End)
		}
	}
	return count
}

func tokensMatch(tokens []query.Token, clause query.Clause, mode comparison) bool {
	if clause.Kind == query.Prefix {
		return strings.HasPrefix(tokens[0].Word, clause.Words[0])
	}
	if mode == withStemming && clause.Kind == query.Word && !clause.Quoted {
		return stem(tokens[0].Word) == stem(clause.Words[0])
	}
	for i, token := range tokens {
		if token.Word != clause.Words[i] {
			return false
		}
	}
	return true
}

// A small English inflection heuristic for the tool-only stemming control.
// Quoted terms and prefixes keep the production clause semantics.
func stem(word string) string {
	if len(word) < 4 || strings.IndexFunc(word, func(r rune) bool { return r < 'a' || r > 'z' }) != -1 {
		return word
	}
	for _, suffix := range []struct{ from, to string }{
		{"ies", "y"}, {"ing", ""}, {"ed", ""}, {"es", ""}, {"s", ""}, {"e", ""},
	} {
		if base, ok := strings.CutSuffix(word, suffix.from); ok && len(base) >= 3 {
			if suffix.from == "s" && strings.HasSuffix(base, "s") {
				break
			}
			return strings.TrimSuffix(base+suffix.to, "e")
		}
	}
	return word
}

func (s session) comparisonPage(ranked []rankedItem) ([]byte, error) {
	doc := render.Doc{
		Head:  []render.Part{{JSON: render.Envelope{Fields: render.Object{}, Rows: "results"}}},
		Total: len(ranked),
		Foot: func(st render.State) render.Part {
			foot := render.Object{}
			if st.More > 0 {
				foot["more"], foot["next"] = st.More, st.Next
			}
			return render.Part{JSON: foot}
		},
	}
	for _, candidate := range ranked[:min(5, len(ranked))] {
		doc.Units = append(doc.Units, render.Unit{Main: render.Part{JSON: s.comparisonRow(candidate)}})
	}
	return render.Fit(&doc, 3000, render.JSON)
}

// Tool-only first pages carry search row metadata and use the published
// code-point and row limits. All four controls use the same rendering.
func (s session) comparisonRow(c rankedItem) render.Object {
	it, h := c.item, c.item.Header
	l := it.Layout
	row := render.Object{
		"id": it.ID, "title": render.OneLine(h.Title, 120), "kind": h.Kind.String(), "status": h.Status.String(),
		"created": nil, "body_line": l.BodyLine, "body_chars": l.BodyChars,
		"file_lines": l.FileLines, "file_chars": l.FileChars, "occurrences": c.occurrences,
	}
	if h.CreatedKnown {
		row["created"] = h.Created.UTC().Format(time.RFC3339Nano)
	}
	for key, value := range map[string]string{"author": h.Author, "verified": h.Verified, "decided": h.Decided, "source": h.Source} {
		if value != "" {
			row[key] = render.OneLine(value, 80)
		}
	}
	if h.Linked() {
		row["linked"] = true
	}
	if h.ReviewDue(frozenTime) {
		row["review_due"] = true
	}
	if path, err := filepath.Rel(s.dir, it.Path); err == nil {
		if _, encoded, ok := render.ExactPath(path); ok {
			row["path"] = encoded
		}
	}
	body := []rune(string(c.body))
	if len(body) != 0 && c.snippetLength > 0 {
		start := c.snippetStart
		end := min(len(body), start+c.snippetLength)
		text := render.OneLine(string(body[start:end]), 0)
		if start > 0 {
			text = "…" + text
		}
		if end < len(body) {
			text += "…"
		}
		row["snippet"], row["snippet_offset"], row["snippet_line"] = text, start, it.LineOf(c.body, start)
		if len(body) > 6000 {
			sections := c.sections
			if sections == nil {
				sections = outline.Parse(c.body)
			}
			if i := outline.At(sections, start); i >= 0 && sections[i].Level > 0 && sections[i].Title != "" {
				row["section"] = render.OneLine(sections[i].Title, 80)
			}
		}
	}
	return row
}
