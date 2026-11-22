package semantic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

func axisVector(dim, axis int) []float32 {
	v := make([]float32, dim)
	v[axis] = 1
	return v
}

const reuseTitle = "Alpha"

type reuseNote struct {
	path, body string
	axes       []int
}

func putReuseNotes(s *section, notes []reuseNote) {
	for _, n := range notes {
		s.put(n.path, reuseTitle, n.body)
	}
}

func seedReuseIndex(t *testing.T, e *env, kind item.Section, notes []reuseNote) *section {
	t.Helper()
	if len(notes) == 0 {
		return nil
	}
	s := newSection(t, kind)
	m, ctx := e.m(), t.Context()
	x, err := newIndex(ctx, IndexFingerprint(m), m.Name, m.Dim)
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	for _, n := range notes {
		body := []byte(n.body)
		cut := Passages(reuseTitle, body)
		if len(cut) != len(n.axes) {
			t.Fatalf("seed %s: %d passages, %d vector axes", n.path, len(cut), len(n.axes))
		}
		passages := make([]storedPassage, len(cut))
		for i, p := range cut {
			passages[i] = storedPassage{Ord: i, Start: p.Start, Stop: p.End, Digest: p.Digest, Vector: axisVector(m.Dim, n.axes[i])}
		}
		if err := x.putItem(ctx, n.path, ContentDigest(reuseTitle, body), ContentCheck(reuseTitle, body), passages); err != nil {
			t.Fatal(err)
		}
	}
	dir, err := indexDir(s.dir, true)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	if _, err := x.publish(ctx, sys.Native(), dir, e.opts().Rand); err != nil {
		t.Fatal(err)
	}
	return s
}

func copyReuseIndex(t *testing.T, from, to *section) {
	t.Helper()
	if from == nil {
		return
	}
	if err := os.MkdirAll(to.index(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(to.index(), os.DirFS(from.index())); err != nil {
		t.Fatal(err)
	}
}

func indexBytes(t *testing.T, s *section) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(s.index(), s.manifestDB()))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

const (
	reuseX  = "# Alpha\n\nThe common passage.\n"
	oldTail = "\n# Tail\n\nThe old tail.\n"
	newTail = "\n# Tail\n\nThe edited tail.\n"
)

func TestBuildWindowInvariance(t *testing.T) {
	for _, test := range []struct {
		name           string
		kind           item.Section
		seed, notes    []reuseNote
		embed, reused  int
		wantBFirstAxis int // negative means no seeded vector is expected
	}{
		{name: "first build", kind: item.Shared,
			notes: []reuseNote{{path: "a.md", body: "This was retained."}, {path: "b.md", body: "This was recorded."}},
			embed: 2, wantBFirstAxis: -1},
		{name: "twins", kind: item.Personal,
			notes: []reuseNote{{path: "a.md", body: reuseX}, {path: "b.md", body: reuseX}},
			embed: 1, reused: 1, wantBFirstAxis: -1},
		{name: "replaced path no longer holds X", kind: item.Personal,
			seed:  []reuseNote{{"a.md", reuseX, []int{0}}},
			notes: []reuseNote{{path: "a.md", body: "A now says something else."}, {path: "b.md", body: reuseX}},
			embed: 2, wantBFirstAxis: -1},
		{name: "unchanged Z beats A's new row", kind: item.Personal,
			seed:  []reuseNote{{"a.md", reuseX + oldTail, []int{0, 2}}, {"z.md", reuseX, []int{1}}},
			notes: []reuseNote{{path: "a.md", body: reuseX + newTail}, {path: "b.md", body: reuseX}, {path: "z.md", body: reuseX}},
			embed: 1, reused: 2, wantBFirstAxis: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := seedReuseIndex(t, newEnv(t), test.kind, test.seed)
			var want []byte
			for _, width := range []int{1, 8, 64, 1024} {
				e := newEnv(t)
				e.reg = &windowTestRegistry{Registry: e.reg, width: width}
				s := newSection(t, test.kind)
				putReuseNotes(s, test.notes)
				copyReuseIndex(t, base, s)
				result := e.mustBuild(s)
				if result.Items != len(test.notes) || result.ItemsRemoved != 0 ||
					result.PassagesEmbedded != test.embed || result.PassagesReused != test.reused {
					t.Fatalf("window %d: result = %+v", width, result)
				}
				got := indexBytes(t, s)
				if width == 1 {
					want = got
				} else if !bytes.Equal(want, got) {
					t.Fatalf("window %d: database differs from window 1", width)
				}
				if test.wantBFirstAxis >= 0 {
					x, state := s.open(e.m())
					if state != IndexOK {
						t.Fatalf("window %d: index state %v", width, state)
					}
					v, err := x.vectorAt(t.Context(), "b.md", 0)
					x.Close()
					if err != nil || !slices.Equal(v, axisVector(e.m().Dim, test.wantBFirstAxis)) {
						t.Fatalf("window %d: B's vector = %v, %v", width, v, err)
					}
				}
			}
		})
	}
}

func TestReuseTiers(t *testing.T) {
	e := newEnv(t)
	m, ctx := e.m(), t.Context()
	x, err := newIndex(ctx, IndexFingerprint(m), m.Name, m.Dim)
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	digest := Passages("Alpha", []byte("X"))[0].Digest
	put := func(path string, axes ...int) {
		t.Helper()
		var passages []storedPassage
		for i, axis := range axes {
			passages = append(passages, storedPassage{Ord: i, Start: i, Stop: i + 1, Digest: digest, Vector: axisVector(m.Dim, axis)})
		}
		body := []byte("Test body")
		if err := x.putItem(ctx, path, ContentDigest(reuseTitle, body), ContentCheck(reuseTitle, body), passages); err != nil {
			t.Fatal(err)
		}
	}
	put("a.md", 0, 1)
	put("b.md", 2, 3)
	put("z.md", 4)
	if err := e.cache.writeVector(m.Fingerprint, digest, axisVector(m.Dim, 6)); err != nil {
		t.Fatal(err)
	}
	b := &builder{ctx: ctx, x: x, si: &sectionIndex{opt: e.opts(), m: m, x: x, cache: true},
		replace: map[string]bool{"b.md": true, "new.md": true},
		vectors: map[[32]byte]vectorRef{digest: {pending: &work{vecs: [][]float32{axisVector(m.Dim, 5)}}}}}
	assert := func(path string, wantAxis int) {
		t.Helper()
		v, found, err := b.reuse(digest, queued{w: &work{path: path}})
		if err != nil || !found || !slices.Equal(v, axisVector(m.Dim, wantAxis)) {
			t.Fatalf("%s reuse = %v, %t, %v, want axis %d", path, v, found, err, wantAxis)
		}
	}
	assert("b.md", 2)   // own row beats an earlier path, and lowest ord wins
	assert("new.md", 0) // lowest unchanged path and ord beat the run and the cache
	b.replace["a.md"] = true
	assert("new.md", 4) // earlier replaced rows cannot answer
	put("a.md", 7)
	assert("new.md", 4) // replacement rows cannot answer either
	b.replace["z.md"] = true
	assert("new.md", 5) // the first answer this run beats the cache
	delete(b.vectors, digest)
	assert("new.md", 6) // the cache answers the first request
}

func TestBuildResumed(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprintf("mixed=%t", mixed), func(t *testing.T) {
			e := newEnv(t)
			e.clock.step = 30 * time.Second
			e.reg = &windowTestRegistry{Registry: e.reg, width: 1}
			notes := []reuseNote{
				{path: "a.md", body: reuseX + newTail},
				{path: "b.md", body: reuseX},
				{path: "z.md", body: reuseX},
			}
			axisZ := 0
			if mixed {
				axisZ = 1
			}
			base := seedReuseIndex(t, e, item.Personal, []reuseNote{
				{"a.md", reuseX + oldTail, []int{0, 2}},
				{"z.md", reuseX, []int{axisZ}},
			})
			resumed, whole := newSection(t, item.Personal), newSection(t, item.Personal)
			for _, s := range []*section{resumed, whole} {
				putReuseNotes(s, notes)
				copyReuseIndex(t, base, s)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			_, err := Build(ctx, e.opts(), resumed.sectionDir(), resumed.scan, e.dl, nil, func(p Progress) {
				if p.Run == nil && p.Done == 1 {
					cancel() // the test clock forces publication before this progress report
				}
			})
			if !errors.Is(err, context.Canceled) || resumed.manifestDB() == base.manifestDB() {
				t.Fatalf("interrupted build = %v, manifest still at base: %t", err, resumed.manifestDB() == base.manifestDB())
			}
			e.mustBuild(resumed)
			e.mustBuild(whole)
			if !mixed {
				if !bytes.Equal(indexBytes(t, resumed), indexBytes(t, whole)) {
					t.Fatal("resumed index differs from uninterrupted index")
				}
				return
			}
			x, a := resumed.open(e.m())
			y, b := whole.open(e.m())
			if a != IndexOK || b != IndexOK {
				t.Fatalf("index states = %v, %v", a, b)
			}
			defer x.Close()
			defer y.Close()
			if !reflect.DeepEqual(x.items, y.items) {
				t.Fatalf("item rows differ: %v vs %v", x.items, y.items)
			}
			for path, row := range x.items {
				left, err := x.passagesOf(t.Context(), row.id)
				if err != nil {
					t.Fatal(err)
				}
				right, err := y.passagesOf(t.Context(), y.items[path].id)
				if err != nil {
					t.Fatal(err)
				}
				for i := range left {
					left[i].Vector = nil
				}
				for i := range right {
					right[i].Vector = nil
				}
				if !reflect.DeepEqual(left, right) {
					t.Fatalf("%s passage rows differ: %+v vs %+v", path, left, right)
				}
			}
		})
	}
}
