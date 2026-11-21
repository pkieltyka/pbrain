package search

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/graph"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/query"
)

var now = time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)

type fixture struct {
	items []*item.Item
	body  map[*item.Item][]byte
	reads map[*item.Item]int
	mu    sync.Mutex
	fail  *item.Item
}

func newFixture() *fixture {
	return &fixture{body: make(map[*item.Item][]byte)}
}

func (f *fixture) put(id, title, body string) *item.Item {
	it := &item.Item{ID: id, Key: item.Fold(id), Path: "/brain/" + id + ".md",
		Header: item.Header{Kind: item.Note, Title: title, CreatedKnown: true, Created: now}}
	f.items = append(f.items, it)
	f.body[it] = []byte(body)
	addLayout(it, body)
	return it
}

func addLayout(it *item.Item, body string) {
	const header = "---\n---\n\n"
	it.Layout = item.Measure([]byte(header+body), int64(len(header)))
}

func (f *fixture) Items() []*item.Item { return f.items }
func (f *fixture) Body(it *item.Item) ([]byte, error) {
	if it == f.fail {
		return nil, errors.New("private file name")
	}
	if f.reads != nil {
		f.mu.Lock()
		f.reads[it]++
		f.mu.Unlock()
	}
	return f.body[it], nil
}

func clauses(t testing.TB, text string) []query.Clause {
	t.Helper()
	cs, err := query.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	return cs
}

func (f *fixture) search(t testing.TB, text string, options ...func(*Request)) *Result {
	t.Helper()
	req := Request{Clauses: clauses(t, text), Text: text, Limit: 50, Now: now, Snippet: 200, LongBody: 6000}
	for _, apply := range options {
		apply(&req)
	}
	res, err := Run(context.Background(), f, graph.Build(f.items), req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func assertRows(t *testing.T, res *Result, ids ...string) {
	t.Helper()
	out := make([]string, 0, len(res.Hits))
	for _, h := range res.Hits {
		out = append(out, h.Item.ID)
	}
	if !slices.Equal(out, ids) {
		t.Errorf("rows %q, want %q", out, ids)
	}
}

func TestAdmitAndCountsSets(t *testing.T) {
	it := &item.Item{Header: item.Header{Kind: item.Note, Tags: []string{"ops", "cache"},
		Status: item.Provisional, StatusRaw: "superseded-by x", CreatedKnown: true, Created: now}}
	window, err := query.ParseWindow("7d", "", now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	wantStatus := item.Provisional
	for _, tc := range []struct {
		name         string
		current, all bool
		created      time.Time
		unknown      bool
		want         Verdict
		wantedSet    selection
	}{
		{"current", true, false, now, false, In, currentIn},
		{"replaced hidden", false, false, now, false, Out, replacedIn},
		{"replaced shown", false, true, now, false, In, replacedIn},
		{"current outside", true, false, now.AddDate(-1, 0, 0), false, OutsideWindow, currentOutside},
		{"replaced outside", false, true, now.AddDate(-1, 0, 0), false, Out, excluded},
		{"unknown outside", true, true, now, true, OutsideWindow, currentOutside},
		{"unknown replaced outside", false, true, now, true, Out, excluded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			it.Header.CreatedKnown, it.Header.Created = !tc.unknown, tc.created
			f := Filters{Kinds: []item.Kind{item.Note, item.Decision}, Tags: []string{"ops", "cache"},
				Status: &wantStatus, All: tc.all, Window: window}
			if got := f.selectItem(it, tc.current); got != tc.wantedSet {
				t.Errorf("selection %d, want %d", got, tc.wantedSet)
			}
			if got := f.Admit(it, tc.current); got != tc.want {
				t.Errorf("Admit %d, want %d", got, tc.want)
			}
		})
	}

	for _, f := range []Filters{
		{Kinds: []item.Kind{item.Observation}},
		{Tags: []string{"ops", "missing"}},
	} {
		if got := f.Admit(it, true); got != Out {
			t.Errorf("failed base filter admitted: %+v", f)
		}
	}
	unwindowed := Filters{Status: &wantStatus}
	if got := unwindowed.Admit(it, true); got != In {
		t.Errorf("unwindowed unknown or stamped note excluded: %d", got)
	}
	verified := item.Verified
	if got := (&Filters{Status: &verified}).Admit(it, true); got != Out {
		t.Errorf("stamp matched verified: %d", got)
	}
}

func TestPhraseRingOwnsTokens(t *testing.T) {
	f := newFixture()
	f.put("adjacent", "Example", "ÉLAN VITAL")
	f.put("out-of-order", "Example", "VITAL ÉLAN")
	f.put("gap", "Example", "ÉLAN unrelated VITAL")
	r := f.search(t, `"élan vital"`)
	assertRows(t, r, "adjacent")
}

func TestMatchFieldsAndClass(t *testing.T) {
	f := newFixture()
	f.put("a", "job queue retry", "")
	f.put("b", "retry", "").Header.Tags = []string{"job-queue"}
	f.put("c", "queue job", "job-queue retries")
	f.put("d", "retry", "").Header.Tags = []string{"job", "queue"}
	f.put("e", "retry queue job", "")
	r := f.search(t, `"job queue" retr*`)
	assertRows(t, r, "a", "b", "c")
	for i, h := range r.Hits {
		if h.Class != i+1 {
			t.Errorf("%s class %d, want %d", h.Item.ID, h.Class, i+1)
		}
	}
}

func TestOrderBandsAndStanding(t *testing.T) {
	f := newFixture()
	add := func(id string, days int, verified bool) *item.Item {
		it := f.put(id, "queue", "")
		it.Header.Created = now.AddDate(0, 0, -days)
		if verified {
			it.Header.Status = item.Verified
		}
		return it
	}
	add("p29", 29, false)
	add("v31", 31, true)
	add("v29", 29, true)
	add("v32", 32, true)
	add("future", -2, false)
	add("old", 400, false)
	add("unknown", 0, false).Header.CreatedKnown = false
	f.put("newer-body-only", "record", "queue")
	assertRows(t, f.search(t, "queue"), "v29", "future", "p29", "v31", "v32", "old", "unknown", "newer-body-only")
}

func TestDecisionException(t *testing.T) {
	f := newFixture()
	f.put("note", "job queue", "").Header.Created = now.AddDate(0, 0, -7)
	d := f.put("decision", "job queue", "")
	d.Header.Kind, d.Header.Decided = item.Decision, "human:owner"
	d.Header.Created = now.AddDate(-1, 0, 0)
	assertRows(t, f.search(t, "job queue"), "decision", "note")

	d.Header.Title = "job"
	f.body[d] = []byte("queue")
	addLayout(d, "queue")
	assertRows(t, f.search(t, "job queue"), "note", "decision")

	successor := f.put("current", "apart", "")
	successor.Header.Kind, successor.Header.Decided = item.Decision, "human:owner"
	successor.Header.Replaces = []string{"decision"}
	f.put("newer-note", "apart", "").Header.Replaces = []string{"note"}
	d.Header.Title = "job queue" // better-matching replaced decisions are never promoted
	r := f.search(t, "job queue", func(req *Request) { req.Filters.All = true })
	assertRows(t, r, "note", "decision")
	for _, h := range r.Hits {
		if !h.Replaced {
			t.Errorf("%s should be replaced", h.Item.ID)
		}
	}
}

func TestLifecycleAndReplacedCount(t *testing.T) {
	f := newFixture()
	f.put("a", "queue", "")
	f.put("b", "queue", "").Header.Replaces = []string{"a"}
	f.put("c", "queue", "").Header.Replaces = []string{"b"}
	r := f.search(t, "queue")
	assertRows(t, r, "c")
	if r.Replaced != 2 {
		t.Errorf("replaced %d, want 2", r.Replaced)
	}
	r = f.search(t, "queue", func(req *Request) { req.Filters.All = true })
	assertRows(t, r, "c", "a", "b")
	if r.Replaced != 2 || !r.Hits[1].Replaced || !r.Hits[2].Replaced {
		t.Fatalf("replaced count or marks wrong: %+v", r)
	}
}

func TestWindowCounts(t *testing.T) {
	f := newFixture()
	old := f.put("old-decision", "queue", "")
	old.Header.Kind, old.Header.Decided = item.Decision, "human:owner"
	old.Header.Created = now.AddDate(-1, 0, 0)
	f.put("unknown", "queue", "").Header.CreatedKnown = false
	f.put("old-replaced", "queue", "").Header.Created = now.AddDate(-1, 0, 0)
	f.put("irrelevant", "outside query", "").Header.Replaces = []string{"old-replaced"}
	window, err := query.ParseWindow("7d", "", now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	r := f.search(t, "queue", func(req *Request) { req.Filters.Window = window })
	assertRows(t, r)
	if r.Outside != 2 || r.Replaced != 0 || !slices.Equal(r.Alone, []ClauseCount{{Label: "queue"}}) {
		t.Fatalf("empty window counts: %+v", r)
	}
	window, err = query.ParseWindow("", "2025-11-21", now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	r = f.search(t, "queue", func(req *Request) { req.Filters.Window = window })
	assertRows(t, r, "old-decision")
	if r.Outside != 1 || r.Replaced != 1 {
		t.Fatalf("until-only counts: %+v", r)
	}
}

func TestPaging(t *testing.T) {
	f := newFixture()
	for i := range 12 {
		f.put(fmt.Sprintf("%02d", i), "queue", "")
	}
	r := f.search(t, "queue", func(req *Request) { req.Offset, req.Limit = 4, 5 })
	assertRows(t, r, "04", "05", "06", "07", "08")
	if r.Total != 12 || r.Offset != 4 {
		t.Fatalf("paging facts: %+v", r)
	}
	r = f.search(t, "queue", func(req *Request) { req.Offset, req.Limit = 13, 5 })
	assertRows(t, r)
	if r.Total != 12 || r.Offset != 13 || len(r.Alone) != 0 {
		t.Fatalf("past-end paging facts: %+v", r)
	}
	for i := range 12 {
		r = f.search(t, "queue", func(req *Request) { req.Offset, req.Limit = i, 1 })
		assertRows(t, r, fmt.Sprintf("%02d", i))
	}
}

func TestAloneCounts(t *testing.T) {
	f := newFixture()
	f.put("a", "durable", "")
	f.put("b", "retries", "durably").Header.Kind = item.Decision
	f.put("replaced", "durable", "")
	f.put("winner", "separate", "").Header.Replaces = []string{"replaced"}
	f.put("old", "durable", "").Header.Created = now.AddDate(-1, 0, 0)
	window, err := query.ParseWindow("7d", "", now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	options := func(req *Request) {
		req.Filters.Kinds, req.Filters.Window = []item.Kind{item.Note}, window
	}
	r := f.search(t, `private Durable "queue retry" retr*`, options)
	want := []ClauseCount{{"private", 0}, {"Durable", 1}, {`"queue retry"`, 0}, {"retr*", 0}}
	if r.Total != 0 || !slices.Equal(r.Alone, want) {
		t.Errorf("got %+v, want no rows and %v", r, want)
	}
	f.put("personal", "private", "").Section = item.Personal
	r = f.search(t, `private Durable "queue retry" retr*`, options)
	want[0].Count = 1
	if !slices.Equal(r.Alone, want) {
		t.Errorf("included personal view counts: %v, want %v", r.Alone, want)
	}
	if r := f.search(t, "durable"); len(r.Alone) != 0 {
		t.Fatal("diagnostics appeared with an exact result")
	}
}

func TestParallelDeterminism(t *testing.T) {
	f := newFixture()
	for i := range 70 {
		it := f.put(fmt.Sprintf("%d", i), "record", "retrying the queue is OK")
		it.Header.Created = now.AddDate(0, 0, -i)
	}
	previous := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	want := f.search(t, `"the queue" retr*`)
	runtime.GOMAXPROCS(16)
	if got := f.search(t, `"the queue" retr*`); !reflect.DeepEqual(got, want) {
		t.Fatal("changing worker count changed results")
	}
}

func TestRunErrors(t *testing.T) {
	f := newFixture()
	it := f.put("it", "record", "text")
	f.fail = it
	g := graph.Build(f.items)
	req := Request{Clauses: clauses(t, "text"), Limit: 2}
	_, err := Run(context.Background(), f, g, req)
	var e *errs.Error
	if !errors.As(err, &e) || e.Code != "io" || strings.Contains(e.Message, "private") {
		t.Fatalf("unexpected body error %v", err)
	}
	// A title match first reaches Body when building its page.
	req.Clauses = clauses(t, "record")
	_, err = Run(context.Background(), f, g, req)
	if !errors.As(err, &e) || e.Code != "io" {
		t.Fatalf("unexpected page error %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = Run(ctx, f, g, req); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v for cancelled search", err)
	}
}
