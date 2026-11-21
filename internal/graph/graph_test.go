package graph

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/item"
)

func note(id string, replaces ...string) *item.Item {
	return &item.Item{
		ID: id, Key: item.Fold(id), Path: "/shared/" + id + ".md",
		Header: item.Header{Kind: item.Note, Replaces: replaces},
	}
}

func personal(id string, replaces ...string) *item.Item {
	it := note(id, replaces...)
	it.Section = item.Personal
	it.Path = "/personal/" + id + ".md"
	return it
}

func dated(it *item.Item, day int) *item.Item {
	it.Header.CreatedKnown = true
	it.Header.Created = time.Date(2026, 11, day, 0, 0, 0, 0, time.UTC)
	return it
}

func assertItems(t *testing.T, got []*item.Item, want ...*item.Item) {
	t.Helper()
	if !slices.Equal(got, want) {
		paths := func(items []*item.Item) []string {
			var out []string
			for _, it := range items {
				out = append(out, it.Path)
			}
			return out
		}
		t.Errorf("got %v, want %v", paths(got), paths(want))
	}
}

func assertFinding(t *testing.T, g *Graph, code string, holder *item.Item, field Field, target string, why Reason) {
	t.Helper()
	for _, f := range g.Findings() {
		if f.Code == code && f.Item == holder && f.Field == field && f.Target == target && f.Reason == why {
			return
		}
	}
	t.Errorf("%s: missing %s for field %d and target %q (reason %d), got %+v", holder.Path, code, field, target, why, g.Findings())
}

func TestChainCurrentAndSuccessors(t *testing.T) {
	a, b, c := note("A"), note("B", "a"), note("C", "b")
	g := Build([]*item.Item{c, b, a})
	if g.Current(a) || g.Current(b) || !g.Current(c) {
		t.Error("only the end of the replacement chain is current")
	}
	assertItems(t, g.Replacement(a), b)
	assertItems(t, g.CurrentSuccessors(a), c)
	assertItems(t, g.CurrentSuccessors(b), c)
	assertItems(t, g.CurrentSuccessors(c))
	assertItems(t, g.History(c), a, b, c)
	assertItems(t, g.Replaces(c), b)

	c.Header.Replaces = []string{"A"}
	g = Build([]*item.Item{c, a, b})
	assertItems(t, g.CurrentSuccessors(a), b, c)
	d := note("D", "B", "C")
	g = Build([]*item.Item{d, a, c, b})
	for _, replaced := range []*item.Item{a, b, c} {
		assertItems(t, g.CurrentSuccessors(replaced), d)
	}
}

func TestValidityRules(t *testing.T) {
	for _, tc := range []struct {
		name        string
		old, newer  *item.Item
		reason      Reason
		replacement bool
	}{
		{"standing", note("A"), note("B", "A"), Standing, false},
		{"decision kind", note("A"), note("B", "A"), DecisionKind, false},
		{"source kind", note("A"), note("B", "A"), SourceKind, false},
		{"section", note("A"), personal("B", "A"), OtherSection, false},
		{"valid", note("A"), note("B", "a", "A"), 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.reason {
			case Standing:
				tc.old.Header.Status = item.Verified
			case DecisionKind:
				tc.old.Header.Kind = item.Decision
			case SourceKind:
				tc.newer.Header.Kind = item.Source
			}
			g := Build([]*item.Item{tc.old, tc.newer})
			if g.Current(tc.old) == tc.replacement {
				t.Error("currentness disagrees with link validity")
			}
			if tc.replacement {
				assertItems(t, g.Replacement(tc.old), tc.newer)
			} else {
				assertFinding(t, g, "replace_invalid", tc.newer, FieldReplaces, "A", tc.reason)
			}
		})
	}
	a := note("Absent", "missing")
	g := Build([]*item.Item{a})
	assertFinding(t, g, "dangling_link", a, FieldReplaces, "", 0)
	if len(g.Findings()) != 1 {
		t.Fatal("missing target produced more than its dangling warning")
	}
}

func TestVerifiedReplacedByVerifiedStaysVerified(t *testing.T) {
	a, b, c := note("A"), note("B", "A"), note("C", "A")
	a.Header.Status, b.Header.Status = item.Verified, item.Verified
	g := Build([]*item.Item{a, b, c})
	assertItems(t, g.Replacement(a), b)
	assertFinding(t, g, "replace_invalid", c, FieldReplaces, "A", Standing)
	if a.Header.Status != item.Verified || a.Header.Standing() != 2 {
		t.Error("replacement changed the status on disk")
	}
}

func TestCyclesIgnoredHistoryEnds(t *testing.T) {
	a, b, c, d := note("A", "A"), note("B", "C"), note("C", "B"), note("D", "B")
	g := Build([]*item.Item{a, b, c, d})
	assertFinding(t, g, "replace_cycle", a, FieldReplaces, "A", InCycle)
	assertFinding(t, g, "replace_cycle", b, FieldReplaces, "C", InCycle)
	assertFinding(t, g, "replace_cycle", c, FieldReplaces, "B", InCycle)
	assertItems(t, g.History(a), a)
	assertItems(t, g.History(b), b, d)
	assertItems(t, g.History(c), c)
	if g.Current(b) || !g.Current(c) {
		t.Error("valid tail was lost with the cycle")
	}

	b.Header.Replaces, c.Header.Replaces, d.Header.Replaces = []string{"C"}, []string{"D"}, []string{"B"}
	g = Build([]*item.Item{b, c, d})
	for _, it := range []*item.Item{b, c, d} {
		assertItems(t, g.History(it), it)
		if !g.Current(it) {
			t.Errorf("%s: cycle edge survived", it.ID)
		}
	}
}

func TestForks(t *testing.T) {
	a, b, c := note("A"), note("B", "A"), note("C", "A")
	d, e := note("D", "B"), note("E", "C", "D")
	g := Build([]*item.Item{a, b, c, d})
	assertItems(t, g.CurrentSuccessors(a), c, d)
	facts := g.Findings()
	if len(facts) != 1 || facts[0].Code != "replace_fork" || facts[0].Item != a {
		t.Fatalf("findings: %+v", facts)
	}
	assertItems(t, facts[0].Successors, c, d)
	facts[0].Successors[0] = a // a caller cannot mutate the graph's slices
	assertItems(t, g.Findings()[0].Successors, c, d)
	g = Build([]*item.Item{a, b, c, d, e})
	assertItems(t, g.CurrentSuccessors(a), e)
	if len(g.Findings()) != 0 {
		t.Fatalf("resolved fork generated %v", g.Findings())
	}
}

func TestLegacyStamps(t *testing.T) {
	stamp := func(it *item.Item, target string) { it.Header.StatusRaw = "superseded-by " + target }
	t.Run("accepted and missing", func(t *testing.T) {
		a, b := note("A"), note("B")
		b.Header.Kind = item.Source // stamps never consult standing or kind
		stamp(a, "../sources/B.MD")
		stamp(b, "../sources/Z.md")
		g := Build([]*item.Item{a, b})
		assertItems(t, g.Replacement(a), b)
		assertFinding(t, g, "legacy_stamp_ignored", b, FieldStatus, "Z", Missing)
	})
	t.Run("masked by valid replace", func(t *testing.T) {
		a, b, c := note("A"), note("B", "A"), note("C")
		stamp(a, "C")
		g := Build([]*item.Item{a, b, c})
		assertItems(t, g.Replacement(a), b)
		assertFinding(t, g, "legacy_stamp_ignored", a, FieldStatus, "C", AlreadyLinked)
	})
	t.Run("joint stamp cycle", func(t *testing.T) {
		a, b, c := note("A"), note("B"), note("C")
		stamp(a, "B")
		stamp(b, "A")
		stamp(c, "C")
		g := Build([]*item.Item{a, b, c})
		for _, it := range []*item.Item{a, b, c} {
			if !g.Current(it) {
				t.Errorf("%s was replaced by a cyclic stamp", it.ID)
			}
			stem, _ := it.Header.LegacyStamp()
			assertFinding(t, g, "legacy_stamp_ignored", it, FieldStatus, stem, InCycle)
		}
	})
	t.Run("mixed cycle keeps valid edges", func(t *testing.T) {
		a, b := note("A"), note("B", "A")
		stamp(b, "A")
		g := Build([]*item.Item{a, b})
		assertItems(t, g.Replacement(a), b)
		assertItems(t, g.History(b), a, b)
		assertFinding(t, g, "legacy_stamp_ignored", b, FieldStatus, "A", InCycle)
	})
	t.Run("same section only", func(t *testing.T) {
		a, b, holder := note("target"), personal("target"), personal("holder")
		stamp(holder, "target")
		g := Build([]*item.Item{a, b, holder})
		assertItems(t, g.Replacement(holder), b)
		g = Build([]*item.Item{a, holder})
		assertFinding(t, g, "legacy_stamp_ignored", holder, FieldStatus, "target", Missing)
		b2 := personal("TARGET")
		g = Build([]*item.Item{a, b, b2, holder})
		assertFinding(t, g, "legacy_stamp_ignored", holder, FieldStatus, "target", Ambiguous)
	})
}

func TestFactoryNotesGraph(t *testing.T) {
	parse := func(id, kind, status string) *item.Item {
		t.Helper()
		data := []byte("---\nkind: " + kind + "\nstatus: " + status + "\ncreated: 2026-11-19T00:00:00Z\nauthor: agent:test\n---\n")
		h, _, _, err := item.Parse(id, data)
		if err != nil {
			t.Fatal(err)
		}
		it := note(id)
		it.Header = h
		return it
	}
	old := parse("legacy", "note", "superseded-by ../learnings/new.md")
	newer := parse("new", "learning", "verified")
	decision := parse("decision", "decision", "verified")
	g := Build([]*item.Item{old, newer, decision})
	assertItems(t, g.CurrentSuccessors(old), newer)
	if decision.Header.Standing() != 1 || !g.Current(newer) || g.Current(old) {
		t.Fatal("legacy items were not preserved")
	}
}

func TestDuplicateIdsInvalidateLinks(t *testing.T) {
	for _, id := range []string{"Duplicate", "duplicate"} {
		a, b, holder := note("Duplicate"), note(id), note("Holder", "DUPLICATE")
		b.Path = "/shared/other/" + id + ".md"
		holder.Header.Relates = []string{"duplicate"}
		g := Build([]*item.Item{a, b, holder})
		assertFinding(t, g, "replace_invalid", holder, FieldReplaces, "DUPLICATE", Ambiguous)
		assertItems(t, g.RelatesTo(holder))
		if !g.Current(a) || !g.Current(b) || len(g.Findings()) != 1 {
			t.Errorf("duplicate targets unexpectedly resolved: %v", g.Findings())
		}
	}
}

func TestSectionsAndViews(t *testing.T) {
	s, p := note("target"), personal("target")
	sh, ph := note("shared-holder", "target"), personal("private-holder", "target", "only-shared")
	onlyShared, onlyPrivate := note("only-shared"), personal("only-private")
	sh.Header.Relates = []string{"target", "only-private"}
	ph.Header.Relates = []string{"only-shared", "target"}
	g := Build([]*item.Item{s, sh, onlyShared})
	assertFinding(t, g, "dangling_link", sh, FieldRelates, "", 0)
	assertItems(t, g.Replacement(s), sh)

	g = Build([]*item.Item{s, p, sh, ph, onlyShared, onlyPrivate})
	assertItems(t, g.Replacement(s), sh)
	assertItems(t, g.RelatesTo(sh), s)
	assertItems(t, g.RelatesTo(ph), onlyShared)
	assertFinding(t, g, "personal_reference", sh, FieldRelates, "", 0)
	assertFinding(t, g, "replace_invalid", ph, FieldReplaces, "target", Ambiguous)
	assertFinding(t, g, "replace_invalid", ph, FieldReplaces, "only-shared", OtherSection)
}

func TestShownAndUnresolved(t *testing.T) {
	h := note("holder", "PRIVATE", "missing", "Missing", "holder")
	p := personal("private")
	h.Header.Relates = []string{"private", "MISSING"}
	g := Build([]*item.Item{h})
	if got := g.Unresolved(h); got != 2 {
		t.Errorf("shared unresolved = %d, want 2", got)
	}
	if got := g.Shown(h, FieldReplaces); !slices.Equal(got, []string{"holder"}) {
		t.Errorf("shared shown = %v", got)
	}
	g = Build([]*item.Item{h, p})
	if got := g.Unresolved(h); got != 1 {
		t.Errorf("personal unresolved = %d, want 1", got)
	}
	if got := g.Shown(h, FieldReplaces); !slices.Equal(got, []string{"PRIVATE", "holder"}) {
		t.Errorf("personal shown = %v", got)
	}
}

func TestRelatedOrders(t *testing.T) {
	a := dated(note("A"), 22)
	b, c, d := dated(note("B"), 21), dated(note("C"), 18), note("D")
	e, f := dated(note("E"), 21), note("F", "B")
	a.Header.Relates = []string{"D", "C", "A", "b", "B"}
	for _, it := range []*item.Item{b, c, d, e} {
		it.Header.Relates = []string{"A"}
	}
	g := Build([]*item.Item{d, b, a, e, f, c})
	assertItems(t, g.RelatesTo(a), d, c, b)
	assertItems(t, g.NewerRelated(a), e, c, d)
}

func TestHistoryOrder(t *testing.T) {
	a, b := dated(note("A"), 22), dated(note("B", "A"), 7)
	c, d := dated(note("C", "A"), 7), note("D", "B", "C")
	unknown, last := note("0", "A"), dated(note("E", "0"), 1)
	g := Build([]*item.Item{c, a, d, b})
	assertItems(t, g.History(d), a, b, c, d)
	g = Build([]*item.Item{c, a, last, unknown, d, b})
	assertItems(t, g.History(last), a, b, c, unknown, last, d)
}

func TestEffect(t *testing.T) {
	a, b, c := note("A"), note("B", "A"), note("C", "B")
	h, stale, private := note("holder"), note("stale"), personal("private")
	h.Header.Relates, stale.Header.Relates, private.Header.Relates = []string{"C", "c"}, []string{"missing"}, []string{"C"}
	impact := Effect([]*item.Item{a, b, c, h, stale}, c)
	assertItems(t, impact.CurrentAgain, b)
	assertItems(t, impact.Dangling, h)
	impact = Effect([]*item.Item{a, b, c, h, stale, private}, c)
	assertItems(t, impact.Dangling, h, private)
	impact = Effect([]*item.Item{a, b, c, h, stale}, a)
	assertItems(t, impact.CurrentAgain)
	assertItems(t, impact.Dangling, b)
	a.Header.StatusRaw = "superseded-by C"
	impact = Effect([]*item.Item{a, b, c}, b)
	assertItems(t, impact.CurrentAgain)
	assertItems(t, impact.Dangling, c)

	duplicate := note("A")
	duplicate.Path = "/shared/other/A.md"
	impact = Effect([]*item.Item{a, duplicate, b}, a)
	assertItems(t, impact.Dangling)
}

func TestCheckNewReplaces(t *testing.T) {
	a := note("A")
	a.Header.Status = item.Verified
	duplicate, second := note("D"), note("d")
	b, c := note("B"), note("C", "B")
	decision, source := note("decision"), note("source")
	decision.Header.Kind = item.Decision
	source.Header.Kind = item.Source
	g := Build([]*item.Item{a, b, c, decision, source, duplicate, second})
	newHeader := item.Header{Kind: item.Note}
	for _, tc := range []struct {
		id      string
		section item.Section
		code    string
		part    string
		items   []*item.Item
	}{
		{"unknown", item.Shared, "not_found", "", nil},
		{"d", item.Shared, "ambiguous_id", "", nil},
		{"a", item.Shared, "invalid", "standing", nil},
		{"c", item.Personal, "invalid", "another section", nil},
		{"decision", item.Shared, "invalid", "only a decision", nil},
		{"source", item.Shared, "invalid", "source items", nil},
		{"b", item.Shared, "invalid", "already replaced", []*item.Item{c}},
	} {
		items, err := g.CheckNewReplaces(tc.section, &newHeader, []string{tc.id})
		var de *errs.Error
		if !errors.As(err, &de) || de.Code != tc.code || !strings.Contains(de.Message, tc.part) {
			t.Errorf("%s: got %v, want %s %q", tc.id, err, tc.code, tc.part)
		}
		assertItems(t, items, tc.items...)
	}
	newHeader.Status = item.Verified
	items, err := g.CheckNewReplaces(item.Shared, &newHeader, []string{"c", "A"})
	if err != nil {
		t.Fatal(err)
	}
	assertItems(t, items, c, a)

	// Write-side id reservation excludes names already mentioned by a dangling link.
	when := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	reserved := "20261121-000000-new-aaaa"
	g = Build([]*item.Item{note("holder", reserved)})
	if len(g.Findings()) != 1 || g.Findings()[0].Code != "dangling_link" {
		t.Fatal("the reserved future id is already mentioned")
	}
	got, err := item.NewID(when, "new", strings.NewReader("\x00\x00\x00\x00\x00\x00\x00\x01"), func(id string) bool {
		return id == reserved
	})
	if err != nil || got != "20261121-000000-new-aaab" {
		t.Fatalf("reservation drew %q: %v", got, err)
	}
}

func TestForeignItemZeroValues(t *testing.T) {
	a, foreign := note("A"), note("A")
	g := Build([]*item.Item{a})
	if g.Current(foreign) || g.Unresolved(foreign) != 0 || g.Shown(foreign, FieldReplaces) != nil {
		t.Fatal("foreign item returned nonzero facts")
	}
	for _, fn := range []func(*item.Item) []*item.Item{
		g.Replacement, g.CurrentSuccessors, g.History, g.RelatesTo, g.NewerRelated, g.Replaces,
	} {
		assertItems(t, fn(foreign))
	}
}

func TestDeterministicFindings(t *testing.T) {
	a, b := note("A"), note("B", "a")
	c, d := note("C", "A", "missing"), note("D", "D")
	c.Header.Relates, d.Header.Relates = []string{"A"}, []string{"A"}
	d.Header.StatusRaw = "superseded-by Z"
	items := []*item.Item{a, b, c, d}
	want := Build(items)
	rng := rand.New(rand.NewPCG(3, 5))
	for range 20 {
		rng.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
		got := Build(items)
		if !reflect.DeepEqual(got.Findings(), want.Findings()) {
			t.Fatalf("unstable findings: %v, want %v", got.Findings(), want.Findings())
		}
		assertItems(t, got.History(a), want.History(a)...)
		assertItems(t, got.NewerRelated(a), want.NewerRelated(a)...)
	}
}

func FuzzBuild(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9})
	f.Add([]byte{1, 0, 0, 1, 1, 1, 2, 2, 3, 0, 0, 5})
	f.Fuzz(func(t *testing.T, data []byte) {
		n := min(len(data)/3, 30)
		items := make([]*item.Item, n)
		for i := range n {
			id := func(b byte) string { return fmt.Sprint(int(b) % (n + 1)) }
			it := note(id(data[3*i]), id(data[3*i+1]))
			if data[3*i+2]&1 != 0 {
				it.Section = item.Personal
				it.Path = "/personal/"
			}
			it.Path += fmt.Sprint("/", i)
			it.Header.StatusRaw = "superseded-by " + id(data[3*i+2])
			it.Header.Relates = []string{id(data[3*i+2])}
			items[i] = it
		}
		g := Build(items)
		for _, it := range items {
			h := g.History(it)
			if !slices.Contains(h, it) {
				t.Fatal("history does not contain its item")
			}
			seen := make(map[*item.Item]bool)
			for _, node := range h {
				if seen[node] {
					t.Fatal("duplicate in history")
				}
				for _, target := range g.Replaces(node) {
					if !seen[target] {
						t.Fatal("history violates the edge order")
					}
				}
				seen[node] = true
			}
			succ := g.CurrentSuccessors(it)
			if g.Current(it) && len(succ) != 0 || !g.Current(it) && len(succ) == 0 {
				t.Fatal("current successors disagree with currentness")
			}
			for _, s := range succ {
				if !g.Current(s) {
					t.Fatal("successor is not current")
				}
			}
		}
	})
}

func BenchmarkBuild5000(b *testing.B) {
	items := make([]*item.Item, 5000)
	id := func(n int) string { return fmt.Sprintf("item-%04d", n) }
	for i := range items {
		it := note(id(i))
		if i%20 >= 2 {
			it.Header.Replaces = []string{id(i - 2)}
		}
		if i%20 == 0 {
			it.Header.StatusRaw = "superseded-by " + id(i+1)
		}
		if i%20 == 19 {
			it.Header.Replaces = []string{id(i - 2), id(i - 3)}
		}
		it.Header.Relates = []string{id((i + 25) % len(items))}
		items[i] = it
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = Build(items)
	}
}
