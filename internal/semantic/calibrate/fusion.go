package main

import (
	"cmp"
	"maps"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/search"
)

// fuse is a research arm. It combines positive BM25 ranks with semantic
// ranks that reach the same cutoff as the real model arms.
func (s session) fuse(text string) (attempt, error) {
	start := time.Now()
	bm25, err := s.rankComparison(text, byBM25)
	if err != nil {
		return attempt{}, err
	}
	filter := search.Filters{}
	var eligible []*item.Item
	for _, it := range s.corpus.cat.Items() {
		if filter.Admit(it, s.corpus.graph.Current(it)) == search.In {
			eligible = append(eligible, it)
		}
	}
	hits, err := s.modelScores(text, eligible)
	if err != nil {
		return attempt{}, err
	}
	row, _ := s.model.registry.Lookup(s.model.name)
	ranked, err := fuseRanks(s, bm25, hits, row.Cutoff)
	if err != nil {
		return attempt{}, err
	}
	out, err := s.comparisonPage(ranked)
	if err != nil {
		return attempt{}, err
	}
	return decodePage(response{out: out, elapsed: time.Since(start)})
}

func fuseRanks(s session, lexical []rankedItem, semantic []search.SemanticHit, cutoff float32) ([]rankedItem, error) {
	const k = 60
	byItem := make(map[*item.Item]rankedItem, len(lexical)+len(semantic))
	for i, hit := range lexical {
		hit.score = 1 / float64(k+i+1)
		byItem[hit.item] = hit
	}
	semantic = slices.DeleteFunc(semantic, func(h search.SemanticHit) bool { return h.Score < cutoff })
	slices.SortFunc(semantic, func(a, b search.SemanticHit) int {
		if order := cmp.Compare(b.Score, a.Score); order != 0 {
			return order
		}
		return newestItem(a.Item, b.Item)
	})
	for i, hit := range semantic {
		ranked, alsoLexical := byItem[hit.Item]
		if !alsoLexical {
			body, err := s.corpus.cat.Body(hit.Item)
			if err != nil {
				return nil, err
			}
			ranked = rankedItem{item: hit.Item, body: body}
			if hit.Start >= 0 && hit.End <= utf8.RuneCount(body) && hit.End > hit.Start {
				ranked.snippetStart = hit.Start
				ranked.snippetLength = min(200, hit.End-hit.Start)
			}
		}
		ranked.score += 1 / float64(k+i+1)
		byItem[hit.Item] = ranked
	}
	ranked := slices.Collect(maps.Values(byItem))
	slices.SortFunc(ranked, compareRanked)
	return ranked, nil
}
