// Command calibrate measures retrieval on the frozen fixture. It does not
// tune on held-out questions or enable a release feature.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/embed/gemma2"
)

const strict = "strict"
const partial = "partial"
const hybrid = "hybrid"
const combined = "bm25-and-semantic"

// allowGemma2 is the team switch (embedtest.AllowGemma2) that lets this tool load EmbeddingGemma 2.
const allowGemma2 = "PBRAIN_ALLOW_GEMMA2"

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, out, diag io.Writer) int {
	models := embed.NewRegistry(append([]embed.Entry{gemma2.Entry()}, gemma2.GGUFEntries()...)...)
	return runWithRegistry(ctx, args, out, diag, models)
}

func runWithRegistry(ctx context.Context, args []string, out, diag io.Writer, models embed.Registry) int {
	flags := flag.NewFlagSet("calibrate", flag.ContinueOnError)
	flags.SetOutput(diag)
	stage := flags.String("stage", "check", "calibration to choose a rule, or check to test a frozen rule")
	model := flags.String("model", "", "calibration model name")
	modelPath := flags.String("model-path", "", "directory containing the model's pinned files")
	flags.Usage = func() {
		fmt.Fprintln(diag, "Usage: calibrate [-stage calibration|check] [-model NAME -model-path DIR] <fixture-directory>")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	half := heldOut
	switch *stage {
	case calibration:
		half = calibration
	case "check":
	default:
		flags.Usage()
		return 2
	}
	if flags.NArg() != 1 || (*model == "") != (*modelPath == "") {
		flags.Usage()
		return 2
	}
	if row, ok := models.Lookup(*model); ok && row.Runtime == "goinfer" && os.Getenv(allowGemma2) != "1" {
		fmt.Fprintf(diag, "calibrate: EmbeddingGemma 2 workloads are off for the team: %s=1 turns them on (owner only)\n", allowGemma2)
		return 2
	}
	fix, err := loadFixture(ctx, flags.Arg(0))
	if err != nil {
		fmt.Fprintln(diag, err)
		return 2
	}
	defer fix.close()
	if *model != "" {
		if err := fix.configureModel(models, *model, *modelPath); err != nil {
			fmt.Fprintln(diag, err)
			return 2
		}
	}
	report, err := evaluate(ctx, fix, half)
	if err != nil {
		fmt.Fprintln(diag, err)
		return 2
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintln(diag, err)
		return 2
	}
	if !report.passes() {
		return 1
	}
	return 0
}

type mode struct {
	name string
	run  func(session, string) (attempt, error)
}

func experiment(name string, rank comparison) mode {
	return mode{name, func(s session, text string) (attempt, error) { return s.compare(text, rank) }}
}

var modelFree = []mode{
	{strict, func(s session, text string) (attempt, error) { return s.search(text, false) }},
	{"guided-retry", session.retry},
	experiment("any-count", byCount),
	experiment("any-bm25", byBM25),
	experiment("bm25-stemming", withStemming),
	experiment("bm25-heading-weight", withHeadingWeight),
	{partial, func(s session, text string) (attempt, error) { return s.search(text, true) }},
}

var modelArms = []mode{
	{hybrid, func(s session, text string) (attempt, error) { return s.searchMode(text, false, true) }},
	{combined, func(s session, text string) (attempt, error) { return s.searchMode(text, true, true) }},
	{"rank-fusion", session.fuse},
}

type outcome struct {
	found, empty bool
}

type distribution struct {
	Median      float64 `json:"median_code_points"`
	Worst       int     `json:"worst_code_points"`
	MedianToken float64 `json:"median_estimated_tokens"`
	WorstToken  float64 `json:"worst_estimated_tokens"`
}

type evidenceReport struct {
	Questions       int          `json:"questions"`
	Retrieved       int          `json:"retrieved"`
	Ranks           map[int]int  `json:"ranks"`
	OnPage          int          `json:"on_page"`
	AfterRead       int          `json:"after_read"`
	Reached         float64      `json:"fraction_reaching_evidence"`
	NotRetrieved    int          `json:"miss_not_retrieved"`
	WrongFirst      int          `json:"miss_not_first"`
	EvidenceOmitted int          `json:"miss_evidence_not_visible"`
	UpperBound      int          `json:"oracle_upper_bound"`
	ToEvidence      distribution `json:"to_evidence_json"`
	Delivered       distribution `json:"delivered_json"`
	to, delivered   []int
}

type armReport struct {
	Name            string         `json:"arm"`
	Answers         int            `json:"answerable"`
	Found           int            `json:"expected_id_on_first_page"`
	Negatives       int            `json:"unanswerable"`
	Empty           int            `json:"correctly_empty"`
	Characters      int            `json:"search_code_points"`
	EstimatedTokens float64        `json:"search_estimated_tokens"`
	Calls           int            `json:"search_calls"`
	Milliseconds    float64        `json:"search_ms"`
	Evidence        evidenceReport `json:"long_document_evidence"`
	results         map[string]outcome
	transcript      map[string][]response
}

type report struct {
	NegativeHalf       string       `json:"negative_half"`
	ReleaseCheck       bool         `json:"held_out_release_check"`
	SharedOutputStable bool         `json:"shared_output_stable_with_personal"`
	PersonalReads      int          `json:"personal_controls_read"`
	PersonalSearches   int          `json:"personal_controls_found_by_search"`
	Arms               []armReport  `json:"arms"`
	Partial            gate         `json:"partial_gate"`
	Model              *modelReport `json:"model,omitempty"`
}

func (r report) passes() bool {
	return r.Partial.Pass && (r.Model == nil || r.Model.Semantic.Pass && r.Model.Combined.Pass)
}

type gate struct {
	Pass         bool          `json:"pass"`
	Abstention   bool          `json:"abstention_pass"`
	Gain         bool          `json:"gain_pass"`
	Empty        int           `json:"empty"`
	Negatives    int           `json:"unanswerable"`
	StrictFound  int           `json:"strict_found"`
	PartialFound int           `json:"partial_found"`
	Lost         []string      `json:"lost_expected_ids_on_questions,omitempty"`
	Failures     []gateFailure `json:"abstention_failures,omitempty"`
}

type gateFailure struct {
	ID         string `json:"id"`
	Regression bool   `json:"regression"`
	Lexical    bool   `json:"strict_also_nonempty"`
}

func evaluate(ctx context.Context, f *fixture, half string) (report, error) {
	c, err := scan(ctx, f.dir, brain.SharedView)
	if err != nil {
		return report{}, err
	}
	s := session{ctx: ctx, dir: f.dir, corpus: c, model: f.model}
	var qs []question
	for _, q := range f.questions {
		if q.Kind == answerable || q.Half == half {
			qs = append(qs, q)
		}
	}
	result := report{NegativeHalf: half, ReleaseCheck: half == heldOut}
	modes := slices.Clone(modelFree)
	if s.model != nil {
		if err := s.buildIndex(false); err != nil {
			return report{}, err
		}
		cutoff, err := s.calibrateCutoff(f.questions)
		if err != nil {
			return report{}, err
		}
		s.model = s.model.withCutoff(cutoff)
		result.Model = &modelReport{Name: s.model.name, Cutoff: cutoff}
		modes = append(modes, modelArms...)
	}
	for _, m := range modes {
		arm, err := runArm(s, qs, m)
		if err != nil {
			return report{}, err
		}
		result.Arms = append(result.Arms, arm)
	}
	baseline := result.arm(strict)
	result.Partial = partialGate(qs, baseline, result.arm(partial))
	if result.Model != nil {
		result.Model.Semantic = modelGate(qs, baseline, result.arm(hybrid))
		result.Model.Combined = checkAbstention(qs, baseline, result.arm(combined))
		result.Model.BM25WithModel = result.Model.Semantic.Pass && result.Partial.Pass && result.Model.Combined.Pass
	}

	personal, err := f.installPersonal(ctx)
	if err != nil {
		return report{}, err
	}
	s.corpus, err = scan(ctx, f.dir, brain.SharedView)
	if err != nil {
		return report{}, err
	}
	if s.model != nil {
		if err := s.buildIndex(true); err != nil {
			return report{}, err
		}
	}
	for i, m := range modes {
		replay, err := runArm(s, qs, m)
		if err != nil {
			return report{}, err
		}
		if err := compareTranscripts(qs, result.Arms[i], replay); err != nil {
			return report{}, err
		}
	}
	result.SharedOutputStable = true
	for _, control := range personal {
		page, err := s.searchJSON(control.query, "--personal")
		if err != nil {
			return report{}, err
		}
		found := false
		for _, hit := range page.page.Results {
			if hit.ID == control.id && hit.Personal {
				found = true
				break
			}
		}
		if !found {
			return report{}, fmt.Errorf("personal search did not return control %s", control.id)
		}
		result.PersonalSearches++
		got, err := s.get(control.id, nil, true)
		if err != nil {
			return report{}, err
		}
		var obj struct {
			ID       string `json:"id"`
			Personal bool   `json:"personal"`
		}
		if err := json.Unmarshal(got.out, &obj); err != nil || obj.ID != control.id || !obj.Personal {
			return report{}, fmt.Errorf("personal control %s did not return its own item", control.id)
		}
		result.PersonalReads++
	}
	return result, nil
}

func (r report) arm(name string) armReport {
	for _, arm := range r.Arms {
		if arm.Name == name {
			return arm
		}
	}
	panic("calibration arm missing: " + name)
}

func runArm(s session, qs []question, m mode) (armReport, error) {
	r := armReport{
		Name: m.name, results: make(map[string]outcome, len(qs)), transcript: make(map[string][]response, len(qs)),
		Evidence: evidenceReport{Ranks: map[int]int{}},
	}
	for _, q := range qs {
		a, err := m.run(s, q.Text)
		if err != nil {
			return r, fmt.Errorf("%s %s: %w", m.name, q.ID, err)
		}
		got := outcome{empty: len(a.page.Results) == 0}
		for _, hit := range a.page.Results {
			got.found = got.found || slices.Contains(q.Expect, hit.ID)
		}
		r.results[q.ID] = got
		if q.Kind == answerable {
			r.Answers++
			if got.found {
				r.Found++
			}
		} else {
			r.Negatives++
			if got.empty {
				r.Empty++
			}
		}
		r.Characters += responseChars(a.calls)
		r.Calls += len(a.calls)
		for _, call := range a.calls {
			r.Milliseconds += float64(call.elapsed) / float64(time.Millisecond)
		}
		r.transcript[q.ID] = a.calls
		if q.Style == longSource {
			evidence, err := s.evidence(q, a)
			if err != nil {
				return r, fmt.Errorf("%s %s: %w", m.name, q.ID, err)
			}
			r.Evidence.record(evidence)
			r.transcript[q.ID] = append(a.calls, evidence.followUps...)
		}
	}
	r.EstimatedTokens = float64(r.Characters) / 4
	r.Evidence.finish()
	return r, nil
}

func compareTranscripts(qs []question, original, replay armReport) error {
	for _, q := range qs {
		before, after := original.transcript[q.ID], replay.transcript[q.ID]
		equal := slices.EqualFunc(before, after, func(a, b response) bool {
			return bytes.Equal(a.out, b.out) && bytes.Equal(a.diagnostic, b.diagnostic)
		})
		if !equal {
			return fmt.Errorf("personal disclosure: %s %s shared output changed", original.Name, q.ID)
		}
	}
	return nil
}

func (r *evidenceReport) record(got evidenceRead) {
	r.Questions++
	if got.rank != 0 {
		r.Retrieved++
		r.Ranks[got.rank]++
	}
	if got.upperBound {
		r.UpperBound++
	}
	switch {
	case got.onPage:
		r.OnPage++
	case got.afterRead:
		r.AfterRead++
	case got.rank == 0:
		r.NotRetrieved++
	case got.rank > 1:
		r.WrongFirst++
	default:
		r.EvidenceOmitted++
	}
	if got.onPage || got.afterRead {
		r.to = append(r.to, got.toEvidence)
		r.delivered = append(r.delivered, got.delivered)
	}
}

func (r *evidenceReport) finish() {
	if r.Questions > 0 {
		r.Reached = float64(r.OnPage+r.AfterRead) / float64(r.Questions)
	}
	r.ToEvidence = sizes(r.to)
	r.Delivered = sizes(r.delivered)
}

func sizes(ns []int) distribution {
	if len(ns) == 0 {
		return distribution{}
	}
	slices.Sort(ns)
	median := float64(ns[(len(ns)-1)/2]+ns[len(ns)/2]) / 2
	worst := ns[len(ns)-1]
	return distribution{median, worst, median / 4, float64(worst) / 4}
}
