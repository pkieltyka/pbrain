package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/embed/embedtest"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/search"
)

func TestChooseCutoff(t *testing.T) {
	// Two observed maxima allow zero above the cutoff, even with 18
	// questions that produced no score.
	scores := make([]*float32, 20)
	scores[0], scores[1] = new(float32(0.55)), new(float32(0.64))
	if got := chooseCutoff(scores); got != 0.65 {
		t.Fatalf("got %.2f, want 0.65 for two observed scores", got)
	}
	// A full set of twenty observed scores permits one equal or higher.
	for i := 2; i < len(scores); i++ {
		scores[i] = new(float32(0.5))
	}
	if got := chooseCutoff(scores); got != 0.56 {
		t.Fatalf("got %.2f, want 0.56; 0.55 still admits both", got)
	}
	clear(scores)
	if got := chooseCutoff(scores); got != -1 {
		t.Fatalf("got %.2f for absent maxima, want -1", got)
	}
	scores[0], scores[1] = new(float32(1)), new(float32(1))
	if got := chooseCutoff(scores); got != 1.01 {
		t.Fatalf("got %.2f for two identical vectors, want 1.01", got)
	}
}

func TestQuestionMaximumExcludesExactMatchBeyondPage(t *testing.T) {
	s := readerFixture(t, "no exact term here")
	for i := range 6 {
		path := filepath.Join(s.dir, fmt.Sprintf("exact-%d.md", i))
		if err := os.WriteFile(path, []byte(modelHeader("Source notes matchterm")+"same word"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	c, err := scan(t.Context(), s.dir, brain.SharedView)
	if err != nil {
		t.Fatal(err)
	}
	scorer := func(text string, items []*item.Item) ([]search.SemanticHit, error) {
		t.Helper()
		if text != "matchterm" || len(items) != 2 {
			t.Fatalf("expected only non-exact rows, got %q and %d items", text, len(items))
		}
		for _, it := range items {
			if it.ID != "answer" && it.ID != "wrong" {
				t.Fatalf("exact row reached the scorer: %s", it.ID)
			}
		}
		return []search.SemanticHit{{Item: items[0], Score: 0.25}, {Item: items[1], Score: 0.4}}, nil
	}
	got, err := questionMaximum(t.Context(), c, "matchterm", scorer)
	if err != nil || got == nil || *got != 0.4 {
		t.Fatalf("wrong uncensored maximum %v: %v", got, err)
	}
	got, err = questionMaximum(t.Context(), c, "source", func(string, []*item.Item) ([]search.SemanticHit, error) {
		t.Fatal("scorer called even though every item is exact")
		return nil, nil
	})
	if err != nil || got != nil {
		t.Fatalf("expected absent maximum when every item is exact: %v, %v", got, err)
	}
}

func TestCombinedGateNeedsItsOwnAbstention(t *testing.T) {
	var qs []question
	for i := range 40 {
		qs = append(qs, question{ID: fmt.Sprintf("u%02d", i), Kind: unanswered})
	}
	qs = append(qs, question{ID: "strict-correct", Kind: answerable}, question{ID: "new-correct", Kind: answerable})
	arm := func(misses ...int) armReport {
		a := armReport{Empty: 40 - len(misses), Negatives: 40, Found: 2, results: map[string]outcome{
			"strict-correct": {found: true}, "new-correct": {found: true},
		}}
		for _, q := range qs[:40] {
			a.results[q.ID] = outcome{empty: true}
		}
		for _, i := range misses {
			a.results[qs[i].ID] = outcome{}
		}
		return a
	}
	baseline := arm()
	baseline.Found, baseline.results["new-correct"] = 1, outcome{}
	p := partialGate(qs, baseline, arm(0, 1))
	m := modelGate(qs, baseline, arm(2, 3))
	mix := checkAbstention(qs, baseline, arm(0, 1, 2, 3))
	r := report{Partial: p, Model: &modelReport{Semantic: m, Combined: mix}}
	if !p.Pass || !m.Pass || mix.Pass || r.passes() {
		t.Fatalf("38/40 gates passed alone and must fail with 36/40 combined: %+v", r)
	}

	baseline.results[qs[0].ID] = outcome{empty: false}
	qs[0].Regression = true
	a := checkAbstention(qs, baseline, arm(0))
	if a.Pass || len(a.Failures) != 1 || !a.Failures[0].Lexical || !a.Failures[0].Regression {
		t.Fatalf("a strict regression must also fail the semantic gate: %+v", a)
	}
}

func TestModelCalibrationUsesRealIndexAndCLI(t *testing.T) {
	kit := embedtest.New(nil)
	modelPath := t.TempDir()
	model := kit.Models[embedtest.ModelA]
	for _, file := range model.Files {
		if err := os.WriteFile(filepath.Join(modelPath, file.Name), kit.Bytes[file.URL], 0o600); err != nil {
			t.Fatal(err)
		}
	}
	root := t.TempDir()
	for _, dir := range []string{"brain", "personal-overlay"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for path, data := range map[string]string{
		"brain/answer.md":             modelHeader("Engine notes") + "alpha details.",
		"brain/other.md":              modelHeader("Timetable") + "delta timetable.",
		"personal-overlay/private.md": modelHeader("Private notes") + "beta secret gamma.",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var qs []question
	for i := range 30 {
		qs = append(qs,
			question{ID: fmt.Sprintf("a%02d", i), Text: "alphalink", Style: "short", Kind: answerable, Expect: []string{"answer"}},
			question{ID: fmt.Sprintf("c%02d", i), Text: "betaprobe", Style: "near", Kind: unanswered, Half: calibration},
			question{ID: fmt.Sprintf("h%02d", i), Text: "gammaprobe", Style: "near", Kind: unanswered, Half: heldOut},
		)
	}
	// This held-out query scores 1.00. It must fail abstention without
	// affecting the cutoff chosen from probes scoring zero.
	qs[2].Text, qs[2].Regression = "alphaprobe", true
	writeQuestions(t, root, qs)
	var out, diag bytes.Buffer
	args := []string{"-model", embedtest.ModelA, "-model-path", modelPath, root}
	exit := runWithRegistry(t.Context(), args, &out, &diag, kit.Registry)
	if exit != 1 || diag.Len() != 0 {
		t.Fatalf("expected a gate failure report, got exit %d: %s", exit, &diag)
	}
	var r report
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Model == nil || r.Model.Cutoff != 0.01 || r.Model.Semantic.Pass ||
		!r.Model.Semantic.Gain.Pass || r.Model.Semantic.Gain.ArmFound != 30 ||
		r.Model.Semantic.Abstention.Empty != 29 || r.Model.Combined.Pass {
		t.Fatalf("wrong model outcome: %+v", r.Model)
	}
	if !r.SharedOutputStable || r.PersonalReads != 1 || r.PersonalSearches != 1 || len(r.Arms) != 10 || r.arm("rank-fusion").Found != 30 {
		t.Fatalf("incomplete rendered evaluation: stable %t, %d private reads, %d private searches, %d arms",
			r.SharedOutputStable, r.PersonalReads, r.PersonalSearches, len(r.Arms))
	}
	if kit.Opens.Load() == 0 || kit.Opens.Load() != kit.Closes.Load() {
		t.Fatal("test did not use and close the actual indexed source")
	}

	f, err := loadFixture(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer f.close()
	if err := f.configureModel(kit.Registry, embedtest.ModelA, modelPath); err != nil {
		t.Fatal(err)
	}
	s := session{ctx: t.Context(), dir: f.dir, model: f.model}
	if _, err := s.searchMode("alphalink", false, true); err == nil {
		t.Fatal("a model gate accepted a lexical-only fallback with a missing index")
	}
}

func modelHeader(title string) string {
	return "---\nkind: note\ntitle: " + title + "\ncreated: 2026-01-01T00:00:00Z\nauthor: agent:test\n---\n\n"
}

func writeQuestions(t *testing.T, root string, qs []question) {
	t.Helper()
	raw, err := json.Marshal(qs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "questions.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

// EmbeddingGemma 2 runs only behind the team switch; the tool names it in its refusal.
func TestGemma2NeedsTheTeamSwitch(t *testing.T) {
	if allowGemma2 != embedtest.AllowGemma2 {
		t.Fatalf("switch %q, tests use %q", allowGemma2, embedtest.AllowGemma2)
	}
	t.Setenv(allowGemma2, "")
	for _, name := range []string{"embeddinggemma-2", "embeddinggemma-2-q8-0", "embeddinggemma-2-ud-q6-k-xl"} {
		var out, diag bytes.Buffer
		args := []string{"-model", name, "-model-path", t.TempDir(), "testdata/retrieval"}
		if exit := run(t.Context(), args, &out, &diag); exit != 2 || out.Len() != 0 || !strings.Contains(diag.String(), "PBRAIN_ALLOW_GEMMA2=1") {
			t.Fatalf("%s: exit %d, out %q, diag %q", name, exit, out.String(), diag.String())
		}
	}
}
