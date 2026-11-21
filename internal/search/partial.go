package search

import (
	"cmp"
	"math"
	"slices"

	"github.com/pkieltyka/pbrain/internal/query"
)

// Frozen by the model-free calibration in internal/semantic/calibrate.
const (
	partialMinInformative = 5
	partialDFDivisor      = 4
	partialShareDivisor   = 2
)

type partialStats struct {
	n           int
	avg         float64
	informative int
	rare        []bool
	idf         []float64
}

func stats(records []match, nClauses int) partialStats {
	s := partialStats{avg: 1, rare: make([]bool, nClauses), idf: make([]float64, nClauses)}
	df := make([]int, nClauses)
	totalLength := 0
	for _, rec := range records {
		if !rec.eligible {
			continue
		}
		s.n++
		totalLength += rec.length
		for i, mask := range rec.mask {
			if mask != 0 {
				df[i]++
			}
		}
	}
	if totalLength > 0 {
		s.avg = float64(totalLength) / float64(s.n)
	}
	for i, freq := range df {
		if partialDFDivisor*freq <= s.n {
			s.rare[i] = true
			s.informative++
		}
		s.idf[i] = math.Log(1 + (float64(s.n-freq)+0.5)/(float64(freq)+0.5))
	}
	return s
}

func (s *partialStats) admit(rec *match, clauses []query.Clause) (matched int, ok bool) {
	if !rec.eligible || rec.class != 0 {
		return 0, false
	}
	informative := 0
	for i, clause := range clauses {
		if rec.mask[i] == 0 {
			if clause.Kind == query.Phrase || clause.Kind == query.Prefix || clause.Quoted {
				return 0, false
			}
		} else {
			matched++
			if s.rare[i] {
				informative++
			}
		}
	}
	return matched, informative >= partialMinInformative && partialShareDivisor*informative >= s.informative
}

func (s *partialStats) score(rec *match) float64 {
	length := float64(rec.length)
	iNorm := 1.2 * (0.25 + 0.75*length/s.avg)
	var out float64
	for i, frequency := range rec.tf {
		if frequency != 0 {
			tf := float64(frequency)
			out += s.idf[i] * tf * 2.2 / (tf + iNorm)
		}
	}
	return out
}

func rankPartial(records []match, clauses []query.Clause) []rankedHit {
	s := stats(records, len(clauses))
	if s.n < partialDFDivisor || len(clauses) <= partialMinInformative {
		return nil
	}
	var out []rankedHit
	for i := range records {
		rec := &records[i]
		matched, ok := s.admit(rec, clauses)
		if !ok {
			continue
		}
		out = append(out, rankedHit{
			source: rec,
			hit: Hit{Item: rec.item, Replaced: rec.selected == replacedIn,
				Partial: true, Matched: matched, Clauses: len(clauses)},
			score: s.score(rec),
		})
	}
	slices.SortStableFunc(out, func(a, b rankedHit) int {
		if a.hit.Replaced != b.hit.Replaced {
			if a.hit.Replaced {
				return 1
			}
			return -1
		}
		if order := cmp.Compare(b.score, a.score); order != 0 {
			return order
		}
		return newest(a.hit.Item, b.hit.Item)
	})
	return out
}
