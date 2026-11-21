package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/query"
	"github.com/pkieltyka/pbrain/internal/search"
	"github.com/pkieltyka/pbrain/internal/semantic"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type modelRun struct {
	name, path, cache string
	registry          embed.Registry
}

func (f *fixture) configureModel(reg embed.Registry, name, modelPath string) error {
	if _, ok := reg.Lookup(name); !ok {
		return fmt.Errorf("unknown calibration model %q", name)
	}
	abs, err := filepath.Abs(modelPath)
	if err != nil {
		return err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return err
	}
	if !utf8.ValidString(abs) {
		return errors.New("model path must be valid UTF-8")
	}
	// The scratch config always selects an existing model on disk.
	project, err := toml.Marshal(map[string]map[string]string{"semantic": {"model": name}})
	if err != nil {
		return err
	}
	personal, err := toml.Marshal(map[string]string{"model_path": abs})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(f.dir, "pbrain.toml"), project, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(f.dir, "personal.toml"), personal, 0o600); err != nil {
		return err
	}
	f.model = &modelRun{name: name, path: abs, cache: filepath.Join(f.root, "cache"), registry: reg}
	return nil
}

type cutoffRegistry struct {
	embed.Registry
	name   string
	cutoff float32
}

func (r cutoffRegistry) Lookup(name string) (embed.Model, bool) {
	row, ok := r.Registry.Lookup(name)
	if ok && name == r.name {
		row.Cutoff = r.cutoff
	}
	return row, ok
}

func (m modelRun) withCutoff(cutoff float32) *modelRun {
	m.registry = cutoffRegistry{Registry: m.registry, name: m.name, cutoff: cutoff}
	return &m
}

func (s session) buildIndex(personal bool) error {
	args := []string{"index", "--json"}
	if personal {
		args = append(args, "--personal")
	}
	if _, err := s.command(args...); err != nil {
		return err
	}
	args = append(args, "--status")
	got, err := s.command(args...)
	if err != nil {
		return err
	}
	var status struct {
		ModelPresent bool `json:"model_present"`
		Sections     []struct {
			Items, Indexed    int
			Stale, Unreadable bool
		}
	}
	if err := json.Unmarshal(got.out, &status); err != nil {
		return err
	}
	wantSections := 1
	if personal {
		wantSections++
	}
	if !status.ModelPresent || len(status.Sections) != wantSections {
		return fmt.Errorf("model or requested index sections missing: %s", got.out)
	}
	for _, section := range status.Sections {
		if section.Items == 0 || section.Items != section.Indexed || section.Stale || section.Unreadable {
			return fmt.Errorf("incomplete fixture index: %s", got.out)
		}
	}
	return nil
}

func (s session) calibrateCutoff(questions []question) (float32, error) {
	probe := s
	probe.model = s.model.withCutoff(-1)
	var maxima []*float32
	for _, q := range questions {
		if q.Kind != unanswered || q.Half != calibration {
			continue
		}
		maximum, err := questionMaximum(s.ctx, s.corpus, q.Text, probe.modelScores)
		if err != nil {
			return 0, fmt.Errorf("calibration question %s: %w", q.ID, err)
		}
		maxima = append(maxima, maximum)
	}
	return chooseCutoff(maxima), nil
}

type scoreItems func(string, []*item.Item) ([]search.SemanticHit, error)

func questionMaximum(ctx context.Context, c corpus, text string, score scoreItems) (*float32, error) {
	clauses, err := query.Parse(text)
	if err != nil {
		return nil, err
	}
	items := c.cat.Items()
	exact, err := search.Run(ctx, c.cat, c.graph, search.Request{
		Clauses: clauses, Text: text, Limit: len(items), Now: frozenTime, Snippet: 200, LongBody: 6000,
	})
	if err != nil {
		return nil, err
	}
	matched := make(map[*item.Item]bool, len(exact.Hits))
	for _, hit := range exact.Hits {
		matched[hit.Item] = true
	}
	filters := search.Filters{}
	var remaining []*item.Item
	for _, it := range items {
		if !matched[it] && filters.Admit(it, c.graph.Current(it)) == search.In {
			remaining = append(remaining, it)
		}
	}
	if len(remaining) == 0 {
		return nil, nil
	}
	hits, err := score(text, remaining)
	if err != nil {
		return nil, err
	}
	var best *float32
	for _, hit := range hits {
		if best == nil || hit.Score > *best {
			best = new(hit.Score)
		}
	}
	return best, nil
}

// Only observed maxima contribute to either side of the 5% calculation.
// Try the production comparison's inclusive float32 hundredths.
func chooseCutoff(maxima []*float32) float32 {
	scored := 0
	for _, score := range maxima {
		if score != nil {
			scored++
		}
	}
	allowed := scored / 20
	for cent := -100; cent <= 100; cent++ {
		candidate := float32(cent) / 100
		reached := 0
		for _, score := range maxima {
			if score != nil && *score >= candidate {
				reached++
			}
		}
		if reached <= allowed {
			return candidate
		}
	}
	// Identical vectors can score 1.00. If too many calibration questions
	// reach it, the next hundredth admits no hits and cannot pass gain.
	return 1.01
}

func (s session) modelScores(text string, eligible []*item.Item) ([]search.SemanticHit, error) {
	files := sys.Native()
	b, err := brain.Open(s.ctx, brain.Env{Dir: s.dir, Files: files}, brain.OpenOptions{})
	if err != nil {
		return nil, err
	}
	defer b.Close()
	shared, err := b.OpenSection(item.Shared)
	if err != nil {
		return nil, err
	}
	model, err := files.OpenRoot(s.model.path, false)
	if err != nil {
		return nil, err
	}
	defer model.Close()
	dir := semantic.SectionDir{Section: item.Shared, Dir: shared, Root: b.Shared.Dir}
	source := semantic.Start(s.ctx, semantic.Options{
		Files: files, Registry: s.model.registry, ModelName: s.model.name, ModelDir: model,
	}, text, []semantic.SectionDir{dir})
	defer source.Close()
	source.Attach([]semantic.SectionInput{{SectionDir: dir, Items: s.corpus.cat.Items(), Body: s.corpus.cat.Body}})
	scores, err := source.Score(s.ctx, text, eligible)
	if err != nil {
		return nil, err
	}
	if !scores.Ready || scores.Missing != 0 {
		return nil, fmt.Errorf("fixture model is not ready or index is incomplete (reason %d, missing %d)", scores.Reason, scores.Missing)
	}
	if scores.Cutoff == -1 && len(scores.Hits) != len(eligible) {
		return nil, fmt.Errorf("uncensored source returned %d scores for %d eligible items", len(scores.Hits), len(eligible))
	}
	for _, h := range scores.Hits {
		if math.IsNaN(float64(h.Score)) || math.IsInf(float64(h.Score), 0) {
			return nil, errors.New("model supplied a non-finite score")
		}
	}
	return scores.Hits, nil
}
