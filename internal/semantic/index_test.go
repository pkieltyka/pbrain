package semantic

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	mrand "math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/sys"
)

const testDim = 8

// recFiles records every name the code under test opens.
type recFiles struct {
	sys.Files
	opened []string
}

func (f *recFiles) OpenRegular(dir sys.Dir, name string) (sys.RegularFile, error) {
	f.opened = append(f.opened, name)
	return f.Files.OpenRegular(dir, name)
}

type indexFolder struct {
	path string
	dir  sys.Dir
}

func newFolder(t *testing.T) indexFolder {
	t.Helper()
	p := t.TempDir()
	d, err := sys.OpenDir(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return indexFolder{p, d}
}

func (f indexFolder) names(t *testing.T) []string {
	t.Helper()
	es, err := os.ReadDir(f.path)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

func (f indexFolder) write(t *testing.T, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.path, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f indexFolder) read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(f.path, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func unitVector(r *mrand.Rand) []float32 {
	v := make([]float32, testDim)
	var n float64
	for i := range v {
		v[i] = float32(r.NormFloat64())
		n += float64(v[i]) * float64(v[i])
	}
	for i := range v {
		v[i] /= float32(math.Sqrt(n))
	}
	return v
}

func cosineDistance(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	return 1 - dot/(math.Sqrt(na)*math.Sqrt(nb))
}

// fill builds an index with items items of per passages each and returns what it stored.
func fill(t *testing.T, x *indexDB, r *mrand.Rand, items, per int) map[string][]storedPassage {
	t.Helper()
	want := map[string][]storedPassage{}
	for i := range items {
		path := "notes/n" + strconv.Itoa(i) + ".md"
		var ps []storedPassage
		for o := range per {
			p := storedPassage{Ord: o, Start: o * 10, Stop: o*10 + 9, Vector: unitVector(r)}
			p.Digest = sha256.Sum256([]byte(path + strconv.Itoa(o)))
			ps = append(ps, p)
		}
		if err := x.putItem(context.Background(), path, sha256.Sum256([]byte(path)), ps); err != nil {
			t.Fatal(err)
		}
		want[path] = ps
	}
	return want
}

func published(t *testing.T, f indexFolder, items, per int) (manifest, map[string][]storedPassage) {
	t.Helper()
	ctx := context.Background()
	x, err := newIndex(ctx, "fp-a", "test-model", testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	want := fill(t, x, mrand.New(mrand.NewPCG(1, 2)), items, per)
	m, err := x.publish(ctx, sys.Native(), f.dir, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return m, want
}

func TestIndexRoundTrip(t *testing.T) {
	ctx := context.Background()
	f := newFolder(t)
	m, want := published(t, f, 5, 3)
	if names := f.names(t); len(names) != 2 || !slices.Contains(names, "manifest") || !slices.Contains(names, m.DB) {
		t.Fatalf("index/ holds %q", names)
	}
	data := f.read(t, m.DB)
	sum := sha256.Sum256(data)
	if m.Size != int64(len(data)) || m.SHA256 != hex.EncodeToString(sum[:]) || m.Format != indexFormat || m.Dim != testDim {
		t.Fatalf("manifest %+v for %d bytes", m, len(data))
	}
	var onDisk manifest
	if err := json.Unmarshal(f.read(t, "manifest"), &onDisk); err != nil || onDisk != m {
		t.Fatalf("manifest file %+v, %v", onDisk, err)
	}
	for _, full := range []bool{false, true} {
		x, state := openIndex(ctx, sys.Native(), f.dir, "fp-a", testDim, full)
		if state != IndexOK {
			t.Fatalf("full=%v: %v", full, state)
		}
		if len(x.items) != len(want) || x.model != "test-model" {
			t.Fatalf("%d items, model %q", len(x.items), x.model)
		}
		for path, ps := range want {
			it, ok := x.items[path]
			if !ok || it.content != sha256.Sum256([]byte(path)) {
				t.Fatalf("item %s: %+v", path, it)
			}
			got, err := x.passagesOf(ctx, it.id)
			if err != nil || len(got) != len(ps) {
				t.Fatalf("%s: %d passages, %v", path, len(got), err)
			}
			for i := range ps {
				if got[i].Ord != ps[i].Ord || got[i].Start != ps[i].Start || got[i].Stop != ps[i].Stop ||
					got[i].Digest != ps[i].Digest || !slices.Equal(got[i].Vector, ps[i].Vector) {
					t.Fatalf("%s passage %d: %+v, want %+v", path, i, got[i], ps[i])
				}
			}
		}
		x.Close()
	}

	// A second publication links a new name, removes the old file, and a name taken is skipped.
	x, _ := openIndex(ctx, sys.Native(), f.dir, "fp-a", testDim, true)
	defer x.Close()
	if err := x.removeItem(ctx, "notes/n0.md"); err != nil {
		t.Fatal(err)
	}
	taken, _ := hex.DecodeString(strings.TrimSuffix(m.DB, ".db"))
	next := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	m2, err := x.publish(ctx, sys.Native(), f.dir, bytes.NewReader(append(taken, next...)))
	if err != nil {
		t.Fatal(err)
	}
	if m2.DB != hex.EncodeToString(next)+".db" {
		t.Fatalf("the second database is %q", m2.DB)
	}
	if names := f.names(t); len(names) != 2 || !slices.Contains(names, m2.DB) {
		t.Fatalf("index/ holds %q", names)
	}
	y, state := openIndex(ctx, sys.Native(), f.dir, "fp-a", testDim, true)
	if state != IndexOK || len(y.items) != 4 {
		t.Fatalf("%v with %d items", state, len(y.items))
	}
	if _, ok := y.items["notes/n0.md"]; ok {
		t.Fatal("the removed item is still there")
	}
	y.Close()
}

func manifestWith(t *testing.T, m manifest, change map[string]any) []byte {
	t.Helper()
	var fields map[string]any
	b, _ := json.Marshal(m)
	json.Unmarshal(b, &fields)
	for k, v := range change {
		fields[k] = v
	}
	out, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestIndexRejects(t *testing.T) {
	ctx := context.Background()
	f := newFolder(t)
	m, _ := published(t, f, 3, 2)
	good, goodManifest := f.read(t, m.DB), f.read(t, "manifest")
	restore := func() {
		f.write(t, m.DB, good)
		f.write(t, "manifest", goodManifest)
	}
	state := func() IndexState {
		x, s := openIndex(ctx, sys.Native(), f.dir, "fp-a", testDim, true)
		if x != nil {
			x.Close()
		}
		return s
	}
	if s := state(); s != IndexOK {
		t.Fatalf("the published index: %v", s)
	}
	changed := slices.Clone(good)
	changed[len(changed)/2] ^= 1
	longer := append(slices.Clone(good), 0)
	// A database whose own header is broken but whose digest the manifest names.
	broken := slices.Clone(good)
	copy(broken, "not a database!!")
	brokenSum := sha256.Sum256(broken)

	cases := []struct {
		name     string
		db       []byte
		manifest []byte
	}{
		{"one changed byte", changed, nil},
		{"truncated", good[:len(good)-1], nil},
		{"longer", longer, nil},
		{"not JSON", nil, []byte("format = 2")},
		{"JSON null", nil, []byte("null")},
		{"over 4096 bytes", nil, append(slices.Clone(goodManifest[:len(goodManifest)-1]), []byte(strings.Repeat(" ", 4096)+"}")...)},
		{"format 1", nil, manifestWith(t, m, map[string]any{"format": 1})},
		{"format as text", nil, manifestWith(t, m, map[string]any{"format": "2"})},
		{"no format", nil, bytes.Replace(goodManifest, []byte(`"format":2,`), nil, 1)},
		{"broken header", broken, manifestWith(t, m, map[string]any{"sha256": hex.EncodeToString(brokenSum[:])})},
	}
	for _, c := range cases {
		restore()
		if c.db != nil {
			f.write(t, m.DB, c.db)
		}
		if c.manifest != nil {
			f.write(t, "manifest", c.manifest)
		}
		if s := state(); s != IndexUnreadable {
			t.Errorf("%s: %v", c.name, s)
		}
	}

	// Rows that contradict the manifest, and a database without the passage table, each published
	// with a manifest that names their true digest.
	for _, c := range []struct {
		name string
		sql  string
	}{
		{"meta fingerprint", "UPDATE meta SET value = 'fp-other' WHERE key = 'fingerprint'"},
		{"meta model", "UPDATE meta SET value = 'other-model' WHERE key = 'model'"},
		{"meta dim", "UPDATE meta SET value = '9' WHERE key = 'dim'"},
		{"meta format", "UPDATE meta SET value = '1' WHERE key = 'format'"},
		{"no meta row", "DELETE FROM meta WHERE key = 'model'"},
		{"no passage table", "DROP TABLE passage"},
		{"short content digest", "PRAGMA ignore_check_constraints = ON; UPDATE item SET content = x'00' WHERE id = 1"},
		{"wrong-length vector", "PRAGMA ignore_check_constraints = ON; UPDATE passage SET vector = x'0000' WHERE id = 1"},
		{"stop below start", "PRAGMA ignore_check_constraints = ON; UPDATE passage SET stop = start - 1 WHERE id = 1"},
		{"passage without its item", "PRAGMA foreign_keys = OFF; DELETE FROM item WHERE id = 1"},
	} {
		x, err := newIndex(ctx, "fp-a", "test-model", testDim)
		if err != nil {
			t.Fatal(err)
		}
		fill(t, x, mrand.New(mrand.NewPCG(3, 4)), 2, 2)
		for _, s := range strings.Split(c.sql, "; ") {
			if _, err := x.conn.ExecContext(ctx, s); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
		}
		if _, err := x.publish(ctx, sys.Native(), f.dir, rand.Reader); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		x.Close()
		if s := state(); s != IndexUnreadable {
			t.Errorf("%s: %v", c.name, s)
		}
	}

	// Another fingerprint or dimension is stale; no manifest is none.
	restore()
	if _, s := openIndex(ctx, sys.Native(), f.dir, "fp-b", testDim, false); s != IndexStale {
		t.Errorf("another fingerprint: %v", s)
	}
	if _, s := openIndex(ctx, sys.Native(), f.dir, "fp-a", 16, false); s != IndexStale {
		t.Errorf("another dimension: %v", s)
	}
	os.Remove(filepath.Join(f.path, "manifest"))
	if _, s := openIndex(ctx, sys.Native(), f.dir, "fp-a", testDim, false); s != IndexNone {
		t.Errorf("no manifest: %v", s)
	}

	// A vector of the wrong length cannot be inserted, through putItem or past it.
	x, err := newIndex(ctx, "fp-a", "test-model", testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	bad := []storedPassage{{Ord: 0, Start: 0, Stop: 1, Vector: make([]float32, testDim+1)}}
	if err := x.putItem(ctx, "a.md", [32]byte{}, bad); err == nil {
		t.Fatal("putItem stored a vector of the wrong length")
	}
	if _, ok := x.items["a.md"]; ok {
		t.Fatal("a failed putItem left the item recorded")
	}
	var n int
	x.conn.QueryRowContext(ctx, "SELECT count(*) FROM item").Scan(&n)
	if n != 0 {
		t.Fatalf("a failed putItem left %d item rows", n)
	}
	x.conn.ExecContext(ctx, "INSERT INTO item (id, path, content) VALUES (1, 'b.md', zeroblob(32))")
	if _, err := x.conn.ExecContext(ctx, "INSERT INTO passage (item_id, ord, start, stop, digest, vector) VALUES (1, 0, 0, 1, zeroblob(32), zeroblob(36))"); err == nil {
		t.Fatal("the schema took a vector of the wrong length")
	}
}

// The read side of TestManifestIsNotTrusted: nothing is opened under a name the manifest gives
// before it is checked, nothing is allocated for a size the file does not have, and a publication
// over such a manifest removes nothing.
func TestManifestIsNotTrusted(t *testing.T) {
	ctx := context.Background()
	f := newFolder(t)
	m, _ := published(t, f, 2, 2)
	f.write(t, "lock", []byte("held"))
	upper := strings.ToUpper(strings.TrimSuffix(m.DB, ".db")) + ".db"
	f.write(t, upper, f.read(t, m.DB))
	raw := func(field, value string) []byte {
		b := manifestWith(t, m, map[string]any{field: "PLACEHOLDER"})
		return bytes.Replace(b, []byte(`"PLACEHOLDER"`), []byte(value), 1)
	}
	cases := []struct {
		name     string
		manifest []byte
	}{
		{"db lock", manifestWith(t, m, map[string]any{"db": "lock"})},
		{"db manifest", manifestWith(t, m, map[string]any{"db": "manifest"})},
		{"db with a separator", manifestWith(t, m, map[string]any{"db": "../" + m.DB})},
		{"db nested", manifestWith(t, m, map[string]any{"db": "x/" + m.DB})},
		{"db upper-case", manifestWith(t, m, map[string]any{"db": upper})},
		{"db too short", manifestWith(t, m, map[string]any{"db": m.DB[1:]})},
		{"db too long", manifestWith(t, m, map[string]any{"db": "0" + m.DB})},
		{"size 0", raw("size", "0")},
		{"size negative", raw("size", "-1")},
		{"size 2^31", raw("size", "2147483648")},
		{"size 2^63", raw("size", "9223372036854775808")},
		{"size fractional", raw("size", "1.5")},
		{"size as text", raw("size", `"1024"`)},
		{"size with an exponent", raw("size", "1e3")},
		{"dim as text", raw("dim", `"8"`)},
		{"db as a number", raw("db", "1234")},
		{"sha256 short", manifestWith(t, m, map[string]any{"sha256": m.SHA256[2:]})},
		{"sha256 upper-case", manifestWith(t, m, map[string]any{"sha256": strings.ToUpper(m.SHA256)})},
	}
	before := f.names(t)
	for _, c := range cases {
		f.write(t, "manifest", c.manifest)
		rf := &recFiles{Files: sys.Native()}
		if _, s := openIndex(ctx, rf, f.dir, "fp-a", testDim, false); s != IndexUnreadable {
			t.Errorf("%s: %v", c.name, s)
		}
		if !slices.Equal(rf.opened, []string{"manifest"}) {
			t.Errorf("%s: opened %q", c.name, rf.opened)
		}
	}

	// A size the file does not have: the database is opened, but nothing of that size is allocated.
	for _, size := range []int64{m.Size + 1, m.Size - 1, maxDatabaseSize} {
		f.write(t, "manifest", raw("size", strconv.FormatInt(size, 10)))
		var a, b runtime.MemStats
		runtime.ReadMemStats(&a)
		_, s := openIndex(ctx, sys.Native(), f.dir, "fp-a", testDim, false)
		runtime.ReadMemStats(&b)
		if s != IndexUnreadable {
			t.Errorf("size %d: %v", size, s)
		}
		if grew := b.TotalAlloc - a.TotalAlloc; grew > 1<<20 {
			t.Errorf("size %d: %d bytes allocated", size, grew)
		}
	}

	// A publication over an untrusted manifest removes nothing but its own temporary files.
	f.write(t, "manifest", manifestWith(t, m, map[string]any{"db": "lock"}))
	x, err := newIndex(ctx, "fp-a", "test-model", testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	m2, err := x.publish(ctx, sys.Native(), f.dir, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	after := f.names(t)
	for _, n := range before {
		if !slices.Contains(after, n) {
			t.Errorf("%s was removed", n)
		}
	}
	if len(after) != len(before)+1 || !slices.Contains(after, m2.DB) {
		t.Errorf("before %q, after %q", before, after)
	}
}

func TestIndexTooLarge(t *testing.T) {
	ctx := context.Background()
	f := newFolder(t)
	m, _ := published(t, f, 1, 1)
	defer func(old int64) { maxDatabaseSize = old }(maxDatabaseSize)
	maxDatabaseSize = m.Size
	x, err := newIndex(ctx, "fp-a", "test-model", testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	fill(t, x, mrand.New(mrand.NewPCG(5, 6)), 200, 4)
	_, err = x.publish(ctx, sys.Native(), f.dir, rand.Reader)
	var e *errs.Error
	if !errors.As(err, &e) || e.Code != "too_large" {
		t.Fatalf("publishing past the limit: %v", err)
	}
	if names := f.names(t); len(names) != 2 || !slices.Contains(names, m.DB) {
		t.Fatalf("index/ holds %q", names)
	}
	y, s := openIndex(ctx, sys.Native(), f.dir, "fp-a", testDim, true)
	if s != IndexOK || len(y.items) != 1 {
		t.Fatalf("the old index: %v", s)
	}
	y.Close()
}

func TestScoringQuery(t *testing.T) {
	ctx := context.Background()
	x, err := newIndex(ctx, "fp-a", "test-model", testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	r := mrand.New(mrand.NewPCG(7, 8))
	want := fill(t, x, r, 60, 100) // 6,000 passages
	q := unitVector(r)
	byID := map[int64]string{}
	for path, it := range x.items {
		byID[it.id] = path
	}
	var eligible []int64
	listed := map[string]bool{}
	for path, it := range x.items {
		if it.id%6 != 0 {
			eligible = append(eligible, it.id)
			listed[path] = true
		}
	}
	// A malformed vector in an item that is not listed is never read.
	var outside int64
	for path, it := range x.items {
		if !listed[path] {
			outside = it.id
			break
		}
	}
	if _, err := x.conn.ExecContext(ctx, "PRAGMA ignore_check_constraints = ON"); err != nil {
		t.Fatal(err)
	}
	if _, err := x.conn.ExecContext(ctx, "UPDATE passage SET vector = x'01' WHERE item_id = ?", outside); err != nil {
		t.Fatal(err)
	}
	x.conn.ExecContext(ctx, "PRAGMA ignore_check_constraints = OFF")

	check := func(cutoff float32) []hitRow {
		t.Helper()
		hits, err := x.scoreItems(ctx, eligible, q, cutoff)
		if err != nil {
			t.Fatalf("cut-off %v: %v", cutoff, err)
		}
		bound := 1 - float64(cutoff) + cutoffMargin
		got := map[[2]int]hitRow{}
		for _, h := range hits {
			path, ok := byID[h.ItemID]
			if !ok || !listed[path] {
				t.Fatalf("an item not listed was scored: %d", h.ItemID)
			}
			p := want[path][h.Ord]
			if d := cosineDistance(p.Vector, q); math.Abs(d-h.Distance) > 1e-5 || h.Start != p.Start || h.Stop != p.Stop {
				t.Fatalf("%s/%d: distance %v, Go %v", path, h.Ord, h.Distance, d)
			}
			got[[2]int{int(h.ItemID), h.Ord}] = h
		}
		for path, ps := range want {
			if !listed[path] {
				continue
			}
			for _, p := range ps {
				d := cosineDistance(p.Vector, q)
				_, ok := got[[2]int{int(x.items[path].id), p.Ord}]
				if math.Abs(d-bound) > 1e-5 && ok != (d <= bound) {
					t.Fatalf("%s/%d at distance %v, bound %v: returned %v", path, p.Ord, d, bound, ok)
				}
			}
		}
		return hits
	}
	if hits := check(-1); len(hits) != 50*100 {
		t.Fatalf("every listed passage: %d", len(hits))
	}
	// A cut-off between the 4,500th and 4,501st nearest listed passages.
	var ds []float64
	for path, ps := range want {
		for _, p := range ps {
			if listed[path] {
				ds = append(ds, cosineDistance(p.Vector, q))
			}
		}
	}
	slices.Sort(ds)
	if hits := check(float32(1 - (ds[4499]+ds[4500])/2)); len(hits) <= 4096 || len(hits) >= 5000 {
		t.Fatalf("%d passages within the bound", len(hits))
	}
	check(0.6)

	// The margin: a passage just past 1 - cutoff is still returned, for search's exact comparison
	// to drop.
	all, _ := x.scoreItems(ctx, eligible, q, -1)
	d := all[0].Distance
	cutoff := float32(1 - d + 5e-7)
	if 1-float64(cutoff) >= d {
		t.Fatalf("cut-off %v does not put %v past 1 - cutoff", cutoff, d)
	}
	hits, err := x.scoreItems(ctx, eligible, q, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(hits, func(h hitRow) bool { return h.ItemID == all[0].ItemID && h.Ord == all[0].Ord }) {
		t.Fatal("the passage in the margin was not returned")
	}

	// The malformed item makes the query fail once it is listed: the check above was not vacuous.
	if _, err := x.scoreItems(ctx, append(slices.Clone(eligible), outside), q, -1); err == nil {
		t.Fatal("a malformed vector was scored without an error")
	}
	if _, err := x.scoreItems(ctx, eligible, q[:testDim-1], 0); err == nil {
		t.Fatal("a query of the wrong length was run")
	}
}
