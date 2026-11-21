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
)

const strict = "strict"
const partial = "partial"

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, out, diag io.Writer) int {
	flags := flag.NewFlagSet("calibrate", flag.ContinueOnError)
	flags.SetOutput(diag)
	stage := flags.String("stage", "check", "calibration to choose a rule, or check to test a frozen rule")
	flags.Usage = func() {
		fmt.Fprintln(diag, "Usage: calibrate [-stage calibration|check] <fixture-directory>")
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
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	fix, err := loadFixture(ctx, flags.Arg(0))
	if err != nil {
		fmt.Fprintln(diag, err)
		return 2
	}
	defer fix.close()
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
	if !report.Partial.Pass {
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
	NegativeHalf       string      `json:"negative_half"`
	ReleaseCheck       bool        `json:"held_out_release_check"`
	SharedOutputStable bool        `json:"shared_output_stable_with_personal"`
	PersonalReads      int         `json:"personal_controls_read"`
	Arms               []armReport `json:"arms"`
	Partial            gate        `json:"partial_gate"`
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
	s := session{ctx: ctx, dir: f.dir, corpus: c}
	var qs []question
	for _, q := range f.questions {
		if q.Kind == answerable || q.Half == half {
			qs = append(qs, q)
		}
	}
	result := report{NegativeHalf: half, ReleaseCheck: half == heldOut}
	for _, m := range modelFree {
		arm, err := runArm(s, qs, m)
		if err != nil {
			return report{}, err
		}
		result.Arms = append(result.Arms, arm)
	}
	result.Partial = partialGate(qs, result.Arms[0], result.Arms[len(result.Arms)-1])

	personal, err := f.installPersonal(ctx)
	if err != nil {
		return report{}, err
	}
	s.corpus, err = scan(ctx, f.dir, brain.SharedView)
	if err != nil {
		return report{}, err
	}
	for i, m := range modelFree {
		replay, err := runArm(s, qs, m)
		if err != nil {
			return report{}, err
		}
		if err := compareTranscripts(qs, result.Arms[i], replay); err != nil {
			return report{}, err
		}
	}
	result.SharedOutputStable = true
	for _, id := range personal {
		got, err := s.get(id, nil, true)
		if err != nil {
			return report{}, err
		}
		var obj struct {
			ID       string `json:"id"`
			Personal bool   `json:"personal"`
		}
		if err := json.Unmarshal(got.out, &obj); err != nil || obj.ID != id || !obj.Personal {
			return report{}, fmt.Errorf("personal control %s did not return its own item", id)
		}
		result.PersonalReads++
	}
	return result, nil
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

func partialGate(qs []question, baseline, broad armReport) gate {
	g := gate{Empty: broad.Empty, Negatives: broad.Negatives, StrictFound: baseline.Found, PartialFound: broad.Found}
	g.Abstention = g.Negatives > 0 && g.Empty*100 >= 95*g.Negatives
	for _, q := range qs {
		b, a := baseline.results[q.ID], broad.results[q.ID]
		if q.Kind == answerable {
			if b.found && !a.found {
				g.Lost = append(g.Lost, q.ID)
			}
		} else if !a.empty {
			g.Failures = append(g.Failures, gateFailure{q.ID, q.Regression, !b.empty})
			g.Abstention = g.Abstention && !q.Regression
		}
	}
	g.Gain = g.PartialFound > g.StrictFound && len(g.Lost) == 0
	g.Pass = g.Abstention && g.Gain
	return g
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
