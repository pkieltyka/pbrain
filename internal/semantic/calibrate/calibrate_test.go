package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/query"
	"github.com/pkieltyka/pbrain/internal/render"
)

func TestCalibrate(t *testing.T) {
	t.Run("frozen fixture", func(t *testing.T) {
		f, err := loadFixture(t.Context(), "testdata/retrieval")
		if err != nil {
			t.Fatal(err)
		}
		defer f.close()
		c, err := scan(t.Context(), f.dir, brain.SharedView)
		if err != nil {
			t.Fatal(err)
		}
		s := session{ctx: t.Context(), dir: f.dir, corpus: c}
		clauses, err := query.Parse(`lexical input "section sides"`)
		if err != nil {
			t.Fatal(err)
		}
		a, _, _, _, err := s.candidates(clauses, byCount)
		if err != nil || len(a) == 0 {
			t.Fatalf("count cohort: %v", err)
		}
		b, _, _, _, err := s.candidates(clauses, byBM25)
		if err != nil || !slices.EqualFunc(a, b, func(a, b rankedItem) bool { return a.item.ID == b.item.ID }) {
			t.Fatalf("count and BM25 used different cohorts: %v", err)
		}
		r, err := evaluate(t.Context(), f, calibration)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{strict, "guided-retry", "any-count", "any-bm25", "bm25-stemming", "bm25-heading-weight", partial}
		if len(r.Arms) != len(want) || !r.SharedOutputStable || r.PersonalReads != 8 || r.PersonalSearches != 8 || r.ReleaseCheck {
			t.Fatalf("wrong calibration report: %d arms, stable %t, %d private reads, %d private searches, release %t",
				len(r.Arms), r.SharedOutputStable, r.PersonalReads, r.PersonalSearches, r.ReleaseCheck)
		}
		for i, arm := range r.Arms {
			if arm.Name != want[i] || arm.Answers != 47 || arm.Negatives != 30 || arm.Evidence.Questions != 6 ||
				arm.Calls < 77 || arm.Characters <= 0 {
				t.Errorf("incomplete or misordered arm: %q, %d answers, %d negatives, %d calls",
					arm.Name, arm.Answers, arm.Negatives, arm.Calls)
			}
		}

		questions := slices.Clone(f.questions)
		questions[slices.IndexFunc(questions, func(q question) bool { return q.Kind == unanswered })].Half = ""
		if err := validateQuestions(questions, c); err == nil || !strings.Contains(err.Error(), "recorded half") {
			t.Fatalf("accepted an unlabelled negative: %v", err)
		}
		if err := validateQuestions(f.questions[:len(f.questions)-1], c); err == nil || !strings.Contains(err.Error(), "30 unanswerable") {
			t.Fatalf("accepted an undersized half: %v", err)
		}
	})

	t.Run("abstention and preservation gates", func(t *testing.T) {
		a := armReport{Found: 1, results: map[string]outcome{"same": {found: true}}}
		b := armReport{Found: 2, Empty: 38, Negatives: 40, results: map[string]outcome{
			"same": {found: true}, "extra": {found: true},
		}}
		qs := []question{{ID: "same", Kind: answerable}, {ID: "extra", Kind: answerable}, {ID: "near", Kind: unanswered}}
		a.results["near"], b.results["near"] = outcome{empty: true}, outcome{empty: false}
		if g := partialGate(qs, a, b); !g.Pass || len(g.Failures) != 1 || g.Failures[0].Lexical {
			t.Fatalf("38/40 with strict preservation should pass: %+v", g)
		}
		b.Empty = 37
		if g := partialGate(qs, a, b); g.Pass || g.Abstention || !g.Gain {
			t.Fatalf("37/40 should fail abstention: %+v", g)
		}
		b.Empty, qs[2].Regression = 38, true
		if g := partialGate(qs, a, b); g.Pass || g.Abstention {
			t.Fatalf("empty-page percentage masked a required regression: %+v", g)
		}
		qs[2].Regression = false
		b.results["same"] = outcome{}
		if g := partialGate(qs, a, b); g.Pass || g.Gain || !slices.Equal(g.Lost, []string{"same"}) {
			t.Fatalf("net gain masked a lost answer: %+v", g)
		}
	})

	t.Run("quotation validation", func(t *testing.T) {
		s := readerFixture(t, "capstan double double\n")
		q := question{Style: longSource, Kind: answerable, Expect: []string{"answer"}}
		if err := validateEvidence(q, s.corpus.cat); err == nil {
			t.Fatal("missing quotation accepted")
		}
		q.Evidence = "double"
		if err := validateEvidence(q, s.corpus.cat); err == nil {
			t.Fatal("repeated quotation accepted")
		}
		q.Evidence, q.Expect = "capstan", []string{"answer", "wrong"}
		if err := validateEvidence(q, s.corpus.cat); err == nil {
			t.Fatal("two IDs accepted for a long-source question")
		}
		q.Expect = []string{"answer"}
		if err := validateEvidence(q, s.corpus.cat); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("one rendered JSON reader", func(t *testing.T) {
		q := question{Expect: []string{"answer"}, Evidence: "café <quote>"}
		s := readerFixture(t, "prefix and café <quote>\n"+strings.Repeat("tail ", 120))
		early, err := decodePage(response{out: []byte(`{"results":[{"id":"answer","snippet":"café <quote> visible tail"}]}` + "\n")})
		if err != nil {
			t.Fatal(err)
		}
		got, err := s.evidence(q, early)
		if err != nil || !got.onPage || got.afterRead || got.rank != 1 || len(got.followUps) != 1 {
			t.Fatalf("snippet: %+v, %v", got, err)
		}
		if want := utf8.RuneCountInString(" visible tail\"}]}\n"); got.delivered-got.toEvidence != want {
			t.Fatalf("snippet size includes later JSON: got %d, want %d", got.delivered-got.toEvidence, want)
		}
		a, err := s.retry("café zzzzxylophonemissing")
		if err != nil || len(a.calls) != 2 || len(a.page.Results) != 1 {
			t.Fatalf("guided retry: %+v, %v", a.page, err)
		}
		got, err = s.evidence(q, a)
		if err != nil || !got.onPage || got.toEvidence <= responseChars(a.calls[:1]) || got.delivered != responseChars(a.calls) {
			t.Fatalf("retry was not charged: %+v, %v", got, err)
		}
		later, err := decodePage(response{out: []byte(`{"results":[{"id":"answer","snippet":"prefix","snippet_offset":5}]}` + "\n")})
		if err != nil {
			t.Fatal(err)
		}
		got, err = s.evidence(q, later)
		if err != nil || got.onPage || !got.afterRead || got.rank != 1 || got.delivered <= got.toEvidence {
			t.Fatalf("follow-up read: %+v, %v", got, err)
		}
		body := got.followUps[0].out
		end := bytes.Index(body, []byte(q.Evidence)) + len(q.Evidence)
		if excess := got.delivered - got.toEvidence; excess != utf8.RuneCount(body[end:]) {
			t.Fatalf("delivered-to-evidence delta = %d, want %d", excess, utf8.RuneCount(body[end:]))
		}

		page, err := decodePage(response{out: []byte(`{"results":[{"id":"wrong"},{"id":"two"},{"id":"three"},{"id":"four"},{"id":"answer"}]}` + "\n")})
		if err != nil {
			t.Fatal(err)
		}
		got, err = s.evidence(q, page)
		if err != nil || got.rank != 5 || !got.upperBound || got.onPage || got.afterRead || got.toEvidence != 0 || len(got.followUps) != 2 {
			t.Fatalf("the oracle's fifth-row read must not be counted: %+v, %v", got, err)
		}
		assertJSONDropsFifth(t, s, q)
	})

	t.Run("JSON escaping can hide a quotation", func(t *testing.T) {
		s := readerFixture(t, strings.Repeat("x\n", 2_300)+"pinned evidence")
		text, err := s.command("get", "answer")
		if err != nil || !bytes.Contains(text.out, []byte("pinned evidence")) {
			t.Fatalf("control text read did not show the evidence: %v", err)
		}
		page, err := decodePage(response{out: []byte(`{"results":[{"id":"answer","snippet":"x","snippet_offset":0}]}` + "\n")})
		if err != nil {
			t.Fatal(err)
		}
		q := question{Expect: []string{"answer"}, Evidence: "pinned evidence"}
		got, err := s.evidence(q, page)
		if err != nil || got.rank != 1 || got.onPage || got.afterRead || got.upperBound || got.delivered != 0 {
			t.Fatalf("the text-only quotation must not count: %+v, %v", got, err)
		}
	})
}

func assertJSONDropsFifth(t *testing.T, s session, q question) {
	t.Helper()
	doc := render.Doc{Head: []render.Part{{JSON: render.Envelope{Fields: render.Object{}, Rows: "results"}}}, Total: 5}
	doc.Foot = func(st render.State) render.Part {
		row := render.Object{}
		if st.More > 0 {
			row["more"], row["next"] = st.More, st.Next
		}
		return render.Part{Text: fmt.Sprintf("%d results.", st.Shown), JSON: row}
	}
	for _, id := range []string{"wrong", "two", "three", "four", "answer"} {
		doc.Units = append(doc.Units, render.Unit{Main: render.Part{Text: id, JSON: render.Object{"id": id, "snippet": strings.Repeat("z", 100)}}})
	}
	text, err := render.Fit(&doc, 570, render.Text)
	if err != nil || !bytes.Contains(text, []byte("answer")) {
		t.Fatalf("control text page: %s, %v", text, err)
	}
	out, err := render.Fit(&doc, 570, render.JSON)
	if err != nil {
		t.Fatal(err)
	}
	page, err := decodePage(response{out: out})
	if err != nil || len(page.page.Results) != 4 {
		t.Fatalf("JSON page: %s, %v", out, err)
	}
	got, err := s.evidence(q, page)
	if err != nil || got.rank != 0 || got.upperBound || len(got.followUps) != 1 {
		t.Fatalf("text-only fifth row counted: %+v, %v", got, err)
	}
}

func TestEvidenceEndUsesPrintedJSON(t *testing.T) {
	out := []byte(`{"before":"café <quote>","results":[{"id":"wrong","snippet":"café <quote>"},{"id":"answer","snippet":"café \t<quote> and \"escape\""}]}` + "\n")
	got, err := evidenceEnd(out, 1, "snippet", "café <quote>")
	want := utf8.RuneCount(out[:bytes.Index(out, []byte(` and \"escape\"`))])
	if err != nil || got != want {
		t.Fatalf("printed position = %d, %v, want %d", got, err, want)
	}
}

func readerFixture(t *testing.T, body string) session {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("pbrain.toml", "")
	const head = "---\nkind: note\ntitle: Source notes\ncreated: 2026-01-01T00:00:00Z\nauthor: agent:test\n---\n\n"
	write("answer.md", head+body)
	write("wrong.md", head+"Unrelated material.")
	c, err := scan(t.Context(), dir, brain.SharedView)
	if err != nil {
		t.Fatal(err)
	}
	return session{ctx: t.Context(), dir: dir, corpus: c}
}

func TestCalibrateReportSeparatesMisses(t *testing.T) {
	r := evidenceReport{Ranks: map[int]int{}}
	for _, read := range []evidenceRead{
		{rank: 1, onPage: true, upperBound: true, delivered: 99, toEvidence: 43},
		{rank: 1, afterRead: true, upperBound: true, delivered: 104, toEvidence: 78},
		{rank: 1},
		{rank: 5, upperBound: true},
		{},
	} {
		r.record(read)
	}
	r.finish()
	if r.Questions != 5 || r.Retrieved != 4 || r.Reached != 0.4 || r.NotRetrieved != 1 ||
		r.WrongFirst != 1 || r.EvidenceOmitted != 1 || r.UpperBound != 3 || r.ToEvidence.Median != 60.5 || r.Delivered.Worst != 104 {
		data, _ := json.Marshal(r)
		t.Fatalf("wrong evidence tally: %s", data)
	}
}
