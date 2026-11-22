package semantic

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/embed/embedtest"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/gitx"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/search"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type note struct{ title, body string }

// section is one section of notes on disk, scanned by a fake ScanFunc.
type section struct {
	t       *testing.T
	kind    item.Section
	root    string
	dir     sys.Dir
	notes   map[string]note
	changed map[string]bool // Body fails for these, as store.ErrChanged would
	scans   int
}

func newSection(t *testing.T, kind item.Section) *section {
	t.Helper()
	root := t.TempDir()
	d, err := sys.OpenDir(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return &section{t: t, kind: kind, root: root, dir: d, notes: map[string]note{}, changed: map[string]bool{}}
}

func (s *section) put(rel, title, body string) {
	s.t.Helper()
	p := filepath.Join(s.root, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		s.t.Fatal(err)
	}
	s.notes[rel] = note{title, body}
}

func (s *section) del(rel string) {
	os.Remove(filepath.Join(s.root, filepath.FromSlash(rel)))
	delete(s.notes, rel)
}

func (s *section) sectionDir() SectionDir {
	return SectionDir{Section: s.kind, Dir: s.dir, Root: s.root}
}

func (s *section) scan(context.Context) (SectionInput, error) {
	s.scans++
	in := SectionInput{SectionDir: s.sectionDir()}
	var rels []string
	for rel := range s.notes {
		rels = append(rels, rel)
	}
	slices.Sort(rels)
	byPath := map[string]string{}
	for _, rel := range rels {
		it := &item.Item{ID: rel, Section: s.kind, Path: filepath.Join(s.root, filepath.FromSlash(rel))}
		it.Header.Title = s.notes[rel].title
		byPath[it.Path] = rel
		in.Items = append(in.Items, it)
	}
	in.Body = func(it *item.Item) ([]byte, error) {
		rel := byPath[it.Path]
		if s.changed[rel] {
			return nil, errors.New("store: the file changed since it was scanned")
		}
		return []byte(s.notes[rel].body), nil
	}
	return in, nil
}

func (s *section) index() string { return filepath.Join(s.root, ".pbrain", "index") }

func (s *section) indexNames() []string {
	es, _ := os.ReadDir(s.index())
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

// open reads the published index as a search would, with the full check.
func (s *section) open(m embed.Model) (*indexDB, IndexState) {
	d, err := sys.OpenDir(s.index())
	if err != nil {
		return nil, IndexNone
	}
	defer d.Close()
	return openIndex(context.Background(), sys.Native(), d, IndexFingerprint(m), m.Dim, true)
}

func (s *section) manifestDB() string {
	d, err := sys.OpenDir(s.index())
	if err != nil {
		return ""
	}
	defer d.Close()
	m, _ := readManifest(sys.Native(), d)
	return m.DB
}

// clock advances by step on every reading.
type clock struct {
	mu   sync.Mutex
	t    time.Time
	step time.Duration
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(c.step)
	return c.t
}

// env is one test's kit, downloader and cache.
type env struct {
	t     *testing.T
	kit   *embedtest.Kit
	dl    *embedtest.Downloader
	cache *Cache
	path  string // the cache folder
	model string
	clock *clock
	reg   embed.Registry // the kit's, unless a test wraps it
}

func newEnv(t *testing.T) *env {
	t.Helper()
	k := embedtest.New(nil)
	path := filepath.Join(t.TempDir(), "cache")
	c, err := OpenCache(context.Background(), CacheOptions{Path: path, Files: sys.Native()}, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return &env{t: t, kit: k, dl: k.Downloader(), cache: c, path: path, model: embedtest.ModelA,
		clock: &clock{t: time.Unix(1_800_000_000, 0)}, reg: k.Registry}
}

// reopen opens the cache afresh, as the next command would.
func (e *env) reopen() {
	e.t.Helper()
	e.cache.Close()
	c, err := OpenCache(context.Background(), CacheOptions{Path: e.path, Files: sys.Native()}, true)
	if err != nil {
		e.t.Fatal(err)
	}
	e.cache = c
}

func (e *env) opts() Options {
	return Options{Files: sys.Native(), Registry: e.reg, ModelName: e.model, Cache: e.cache, Rand: rand.Reader, Now: e.clock.now}
}

func (e *env) m() embed.Model { return e.kit.Models[e.model] }

func (e *env) build(s *section) (BuildResult, error) {
	return e.buildWith(context.Background(), e.opts(), s)
}

func (e *env) buildWith(ctx context.Context, opt Options, s *section) (BuildResult, error) {
	return Build(ctx, opt, s.sectionDir(), s.scan, e.dl, nil, nil)
}

func (e *env) mustBuild(s *section) BuildResult {
	e.t.Helper()
	r, err := e.build(s)
	if err != nil {
		e.t.Fatalf("Build: %v", err)
	}
	return r
}

func errCode(err error) string {
	var e *errs.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// three notes of one passage each.
func basic(s *section) {
	s.put("notes/a.md", "Alpha", "alpha text\n")
	s.put("notes/b.md", "Beta", "beta text\n")
	s.put("notes/c.md", "Gamma", "gamma text\n")
}

// long is a body of n paragraphs of about 600 code points, each with its own keyword-free text.
func long(n int, mark string) string {
	var b strings.Builder
	for i := range n {
		b.WriteString(strings.Repeat("word"+string(rune('a'+i))+" ", 120))
		if i == 2 {
			b.WriteString(mark)
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

func TestBuildFirstAndNoOp(t *testing.T) {
	e := newEnv(t)
	s := newSection(t, item.Personal)
	basic(s)
	r := e.mustBuild(s)
	if r.Items != 3 || r.PassagesEmbedded != 3 || r.PassagesReused != 0 || r.ItemsRemoved != 0 || r.Section != item.Personal {
		t.Fatalf("first build: %+v", r)
	}
	if e.kit.Opens.Load() != 1 || e.kit.Closes.Load() != 1 || e.dl.Requests() != 1 {
		t.Fatalf("opens %d, closes %d, requests %d", e.kit.Opens.Load(), e.kit.Closes.Load(), e.dl.Requests())
	}
	x, state := s.open(e.m())
	if state != IndexOK || len(x.items) != 3 {
		t.Fatalf("%v", state)
	}
	x.Close()
	names := s.indexNames()
	if len(names) != 3 || !slices.Contains(names, "lock") || !slices.Contains(names, "manifest") {
		t.Fatalf("index/ holds %q", names)
	}
	db := s.manifestDB()
	digests := contentCalls.digest.Load()
	r = e.mustBuild(s)
	if r.Items != 3 || r.PassagesEmbedded != 0 || r.PassagesReused != 0 {
		t.Fatalf("second build: %+v", r)
	}
	if contentCalls.digest.Load() != digests {
		t.Fatal("a build with nothing to do computed ContentDigest")
	}
	if e.kit.Opens.Load() != 1 || s.manifestDB() != db || e.dl.Requests() != 1 {
		t.Fatal("a build with nothing to do opened the runtime, published or downloaded")
	}
}

func TestBuildIncremental(t *testing.T) {
	e := newEnv(t)
	s := newSection(t, item.Personal)
	basic(s)
	s.put("notes/long.md", "Long", long(6, ""))
	first := e.mustBuild(s)
	if first.PassagesEmbedded < 6 {
		t.Fatalf("first: %+v", first)
	}
	s.put("notes/d.md", "Delta", "delta text\n")
	if r := e.mustBuild(s); r.PassagesEmbedded != 1 || r.Items != 5 {
		t.Fatalf("one added note: %+v", r)
	}
	s.put("notes/long.md", "Long", long(6, "edited "))
	if r := e.mustBuild(s); r.PassagesEmbedded != 1 || r.PassagesReused != first.PassagesEmbedded-4 {
		t.Fatalf("one edited paragraph: %+v (first %+v)", r, first)
	}
	s.del("notes/b.md")
	if r := e.mustBuild(s); r.ItemsRemoved != 1 || r.Items != 4 || r.PassagesEmbedded != 0 {
		t.Fatalf("one deleted note: %+v", r)
	}
	x, _ := s.open(e.m())
	defer x.Close()
	if _, ok := x.items["notes/b.md"]; ok {
		t.Fatal("the deleted note is still in the published index")
	}
	var n int
	x.conn.QueryRowContext(context.Background(), "SELECT count(*) FROM passage p LEFT JOIN item i ON i.id = p.item_id WHERE i.id IS NULL").Scan(&n)
	if n != 0 {
		t.Fatalf("%d orphan passages", n)
	}
}

// A replacement whose insert fails part-way is rolled back: the old item and all its passages stay.
func TestPutItemRollsBack(t *testing.T) {
	ctx := context.Background()
	x, err := newIndex(ctx, "fp", "m", 2)
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	old := []storedPassage{{Ord: 0, Stop: 1, Vector: []float32{1, 0}}, {Ord: 1, Start: 1, Stop: 2, Vector: []float32{0, 1}}}
	if err := x.putItem(ctx, "a.md", [32]byte{1}, Check{CRC: 1, Size: 8}, old); err != nil {
		t.Fatal(err)
	}
	before := x.items["a.md"]
	// A duplicate key in the new passages fails the second insert.
	dup := []storedPassage{{Ord: 0, Stop: 1, Vector: []float32{1, 0}}, {Ord: 0, Start: 1, Stop: 2, Vector: []float32{1, 0}}}
	if err := x.putItem(ctx, "a.md", [32]byte{2}, Check{CRC: 2, Size: 8}, dup); err == nil {
		t.Fatal("a duplicate key was stored")
	}
	if x.items["a.md"] != before {
		t.Fatal("the failed replacement changed the item")
	}
	got, err := x.passagesOf(ctx, before.id)
	if err != nil || len(got) != 2 || got[1].Vector[1] != 1 {
		t.Fatalf("the old passages: %+v, %v", got, err)
	}
}

func TestBuildRename(t *testing.T) {
	e := newEnv(t)
	s := newSection(t, item.Personal)
	basic(s)
	e.mustBuild(s)
	opens := e.kit.Opens.Load()
	body := s.notes["notes/a.md"]
	s.del("notes/a.md")
	s.put("moved/a.md", body.title, body.body)
	r := e.mustBuild(s)
	if r.PassagesEmbedded != 0 || r.PassagesReused != 1 || r.ItemsRemoved != 1 || e.kit.Opens.Load() != opens {
		t.Fatalf("a rename: %+v, opens %d", r, e.kit.Opens.Load())
	}
	if _, err := os.Stat(filepath.Join(e.path, "vectors")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("a personal build touched the vector cache")
	}
	x, _ := s.open(e.m())
	defer x.Close()
	if _, ok := x.items["moved/a.md"]; !ok {
		t.Fatal("the new path is missing")
	}
	if _, ok := x.items["notes/a.md"]; ok {
		t.Fatal("the old path is still there")
	}
}

func TestBuildFromCache(t *testing.T) {
	e := newEnv(t)
	a := newSection(t, item.Shared)
	basic(a)
	e.mustBuild(a)
	if e.kit.Texts.Load() != 3 {
		t.Fatalf("%d texts", e.kit.Texts.Load())
	}
	// A second work tree of the same shared files.
	b := newSection(t, item.Shared)
	basic(b)
	r := e.mustBuild(b)
	if r.PassagesEmbedded != 0 || r.PassagesReused != 3 || e.kit.Opens.Load() != 1 {
		t.Fatalf("from the cache: %+v, opens %d", r, e.kit.Opens.Load())
	}
	// Personal passages are embedded and leave no cache file.
	vectors := filepath.Join(e.path, "vectors", e.m().Fingerprint)
	count := func() int {
		n := 0
		filepath.WalkDir(vectors, func(_ string, d fs.DirEntry, _ error) error {
			if d != nil && !d.IsDir() {
				n++
			}
			return nil
		})
		return n
	}
	if count() != 3 {
		t.Fatalf("%d cache files", count())
	}
	p := newSection(t, item.Personal)
	p.put("me.md", "Mine", "delta private\n")
	basic(p)
	r = e.mustBuild(p)
	if r.PassagesEmbedded != 4 || count() != 3 {
		t.Fatalf("personal: %+v, %d cache files", r, count())
	}
}

// failing wraps the kit's registry: its embedder runs fail after calls successful calls.
type failing struct {
	embed.Registry
	calls  int
	fail   func(n int, vs [][]float32) ([][]float32, error)
	cancel context.CancelFunc
}

type failingEmb struct {
	embed.Embedder
	r *failing
}

func (f *failing) Open(ctx context.Context, a *embed.Artifact, opt embed.OpenOptions) (embed.Embedder, error) {
	e, err := f.Registry.Open(ctx, a, opt)
	if err != nil {
		return nil, err
	}
	return failingEmb{e, f}, nil
}

func (f failingEmb) Embed(ctx context.Context, role embed.Role, texts []string) ([][]float32, error) {
	vs, err := f.Embedder.Embed(ctx, role, texts)
	f.r.calls++
	if err != nil || f.r.fail == nil {
		return vs, err
	}
	return f.r.fail(f.r.calls, vs)
}

func TestBuildInterrupted(t *testing.T) {
	e := newEnv(t)
	e.clock.step = 30 * time.Second // every item boundary publishes
	s := newSection(t, item.Shared)
	for i := range 20 {
		s.put("n/"+string(rune('a'+i))+".md", "T", strings.Repeat("alpha ", i+1)+"\n")
	}
	ctx, cancel := context.WithCancel(context.Background())
	f := &failing{Registry: e.kit.Registry}
	f.fail = func(n int, vs [][]float32) ([][]float32, error) {
		if n == 2 {
			cancel()
			return nil, context.Canceled
		}
		return vs, nil
	}
	opt := e.opts()
	opt.Registry = f
	if _, err := e.buildWith(ctx, opt, s); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled build: %v", err)
	}
	x, state := s.open(e.m())
	if state != IndexOK || len(x.items) != 8 {
		t.Fatalf("after the first publication: %v, %d items", state, len(x.items))
	}
	x.Close()
	// Leftovers of a crash go; names of other forms stay.
	for _, n := range []string{".pbrain-0123456789abcdef01234567.tmp", "0123456789abcdef.db", "notes.txt", "ABCDEF0123456789.db"} {
		os.WriteFile(filepath.Join(s.index(), n), []byte("x"), 0o644)
	}
	texts := e.kit.Texts.Load()
	r := e.mustBuild(s)
	if r.Items != 20 || r.PassagesEmbedded != 12 || e.kit.Texts.Load()-texts != 12 {
		t.Fatalf("the next run: %+v, %d texts", r, e.kit.Texts.Load()-texts)
	}
	names := s.indexNames()
	slices.Sort(names)
	if len(names) != 5 || !slices.Contains(names, "notes.txt") || !slices.Contains(names, "ABCDEF0123456789.db") || slices.Contains(names, "0123456789abcdef.db") {
		t.Fatalf("index/ holds %q", names)
	}

	// A shared build cancelled before any publication: what reached the cache is not embedded again.
	s2 := newSection(t, item.Shared)
	for i := range 20 {
		s2.put("n/"+string(rune('a'+i))+".md", "T", strings.Repeat("beta ", i+1)+"\n")
	}
	e.clock.step = 0
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	f.calls = 0
	if _, err := e.buildWith(ctx, opt, s2); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled build: %v", err)
	}
	if names := s2.indexNames(); slices.Contains(names, "manifest") {
		t.Fatal("published before the end")
	}
	texts = e.kit.Texts.Load()
	r = e.mustBuild(s2)
	if r.PassagesReused != 8 || r.PassagesEmbedded != 12 {
		t.Fatalf("after a cancelled shared build: %+v", r)
	}
}

func TestBuildFingerprintChange(t *testing.T) {
	e := newEnv(t)
	s := newSection(t, item.Personal)
	basic(s)
	e.mustBuild(s)
	old := s.manifestDB()
	e.model = embedtest.ModelB
	r := e.mustBuild(s)
	if r.PassagesEmbedded != 3 || r.PassagesReused != 0 || r.Items != 3 {
		t.Fatalf("another model: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(s.index(), old)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the old database is still there")
	}
	if _, state := s.open(e.kit.Models[embedtest.ModelA]); state != IndexStale {
		t.Fatalf("the old model's view: %v", state)
	}
	x, state := s.open(e.m())
	if state != IndexOK {
		t.Fatal(state)
	}
	x.Close()
}

func TestBuildRepairsDamage(t *testing.T) {
	e := newEnv(t)
	s := newSection(t, item.Personal)
	basic(s)
	e.mustBuild(s)
	snapshot := func() map[string]string {
		out := map[string]string{}
		for rel := range s.notes {
			b, _ := os.ReadFile(filepath.Join(s.root, rel))
			out[rel] = string(b)
		}
		return out
	}
	before := snapshot()
	damage := map[string]func(){
		"a damaged database": func() {
			p := filepath.Join(s.index(), s.manifestDB())
			b, _ := os.ReadFile(p)
			b[len(b)/2] ^= 1
			os.WriteFile(p, b, 0o644)
		},
		"an unparseable manifest": func() { os.WriteFile(filepath.Join(s.index(), "manifest"), []byte("{"), 0o644) },
		// A published database, with a matching digest, in which one passage of an unchanged
		// item holds a vector of the wrong length.
		"a damaged passage": func() {
			ctx := context.Background()
			x, _ := s.open(e.m())
			x.conn.ExecContext(ctx, "PRAGMA ignore_check_constraints = ON")
			x.conn.ExecContext(ctx, "UPDATE passage SET vector = x'00' WHERE id = 1")
			d, _ := sys.OpenDir(s.index())
			x.publish(ctx, sys.Native(), d, rand.Reader)
			d.Close()
			x.Close()
		},
	}
	for name, f := range damage {
		f()
		if _, state := s.open(e.m()); state != IndexUnreadable {
			t.Fatalf("%s: %v", name, state)
		}
		r := e.mustBuild(s)
		if r.Items != 3 {
			t.Fatalf("%s: %+v", name, r)
		}
		x, state := s.open(e.m())
		if state != IndexOK {
			t.Fatalf("%s: after the build %v", name, state)
		}
		x.Close()
		if !maps2Equal(before, snapshot()) {
			t.Fatalf("%s: an item was touched", name)
		}
	}
}

func maps2Equal(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestBuildConcurrent(t *testing.T) {
	defer func(d time.Duration) { waitNotice = d }(waitNotice)
	waitNotice = 50 * time.Millisecond
	e := newEnv(t)
	s := newSection(t, item.Personal)
	basic(s)
	release := make(chan struct{})
	entered := make(chan struct{})
	e.dl.Before = func(string) {
		close(entered)
		<-release
	}
	var wg sync.WaitGroup
	var first BuildResult
	var firstErr error
	wg.Go(func() { first, firstErr = e.build(s) })
	wait := func(c chan struct{}, what string) {
		t.Helper()
		select {
		case <-c:
		case <-time.After(20 * time.Second):
			t.Fatalf("timed out waiting for %s", what)
		}
	}
	wait(entered, "the first build's download")
	waited := make(chan struct{})
	var second BuildResult
	var secondErr error
	wg.Go(func() {
		second, secondErr = Build(context.Background(), e.opts(), s.sectionDir(), s.scan, e.dl, nil, func(p Progress) {
			if p.Waiting {
				close(waited)
			}
		})
	})
	wait(waited, "the second build to report Waiting")
	close(release)
	wg.Wait()
	if firstErr != nil || secondErr != nil {
		t.Fatal(firstErr, secondErr)
	}
	if first.PassagesEmbedded != 3 || second.PassagesEmbedded != 0 || second.Items != 3 {
		t.Fatalf("first %+v, second %+v", first, second)
	}
}

func TestBuildEmptySection(t *testing.T) {
	e := newEnv(t)
	s := newSection(t, item.Shared)
	if r, err := e.build(s); err != nil || r != (BuildResult{Section: item.Shared}) {
		t.Fatalf("empty: %+v, %v", r, err)
	}
	if _, err := os.Stat(filepath.Join(s.root, ".pbrain")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("an empty section got .pbrain/")
	}
	if e.dl.Requests() != 0 {
		t.Fatal("an empty section asked for the model")
	}
	// A personal folder that does not exist.
	missing := SectionDir{Section: item.Personal, Root: filepath.Join(t.TempDir(), "nope")}
	empty := func(context.Context) (SectionInput, error) { return SectionInput{SectionDir: missing}, nil }
	if r, err := Build(context.Background(), e.opts(), missing, empty, e.dl, nil, nil); err != nil || r.Items != 0 {
		t.Fatalf("a missing folder: %+v, %v", r, err)
	}
	// The last item deleted: a database with no item.
	s.put("a.md", "A", "alpha\n")
	e.mustBuild(s)
	s.del("a.md")
	r := e.mustBuild(s)
	if r.ItemsRemoved != 1 || r.Items != 0 {
		t.Fatalf("the last item deleted: %+v", r)
	}
	x, state := s.open(e.m())
	if state != IndexOK || len(x.items) != 0 {
		t.Fatalf("%v", state)
	}
	x.Close()
	// No item and an unreadable or stale index: an empty database replaces it, with no model.
	os.WriteFile(filepath.Join(s.index(), "manifest"), []byte("{"), 0o644)
	requests := e.dl.Requests()
	os.RemoveAll(filepath.Join(e.path, "models"))
	e.reopen()
	if r, err := e.build(s); err != nil || r.Items != 0 {
		t.Fatalf("unreadable: %+v %v", r, err)
	}
	if _, state := s.open(e.m()); state != IndexOK {
		t.Fatalf("after replacing an unreadable index: %v", state)
	}
	e.model = embedtest.ModelB
	e.mustBuild(s)
	if _, state := s.open(e.m()); state != IndexOK || e.dl.Requests() != requests {
		t.Fatalf("after replacing a stale index: %v, %d requests", state, e.dl.Requests())
	}
}

func TestBuildErrors(t *testing.T) {
	e := newEnv(t)
	s := newSection(t, item.Personal)
	basic(s)
	opt := e.opts()
	opt.ModelName = ""
	if _, err := e.buildWith(context.Background(), opt, s); errCode(err) != "no_model" {
		t.Fatalf("no model: %v", err)
	}
	opt.ModelName = "nope"
	if _, err := e.buildWith(context.Background(), opt, s); errCode(err) != "invalid_config" || !strings.Contains(err.Error(), "test-model-b") {
		t.Fatalf("unknown model: %v", err)
	}
	opt = e.opts()
	opt.CacheErr = ErrCacheExposed
	if _, err := e.buildWith(context.Background(), opt, s); errCode(err) != "invalid_config" {
		t.Fatalf("exposed cache: %v", err)
	}
	opt = e.opts()
	opt.Cache = nil
	if _, err := e.buildWith(context.Background(), opt, s); errCode(err) != "invalid_config" {
		t.Fatalf("no cache, no model_path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.root, ".pbrain")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("a refused build created .pbrain/")
	}

	// A changed body is skipped and stays to do.
	s.changed["notes/b.md"] = true
	if r := e.mustBuild(s); r.Items != 2 || r.PassagesEmbedded != 2 {
		t.Fatalf("a changed body: %+v", r)
	}
	delete(s.changed, "notes/b.md")
	if r := e.mustBuild(s); r.Items != 3 || r.PassagesEmbedded != 1 {
		t.Fatalf("the next run: %+v", r)
	}

	// A vector of the wrong length, not finite, or not of unit length fails with io and leaves the
	// old index.
	db := s.manifestDB()
	for name, bad := range map[string][]float32{
		"wrong length": {1},
		"not finite":   {float32(nanValue()), 0, 0, 0, 0, 0, 0, 0},
		"not unit":     {0.5, 0, 0, 0, 0, 0, 0, 0},
	} {
		f := &failing{Registry: e.kit.Registry, fail: func(_ int, vs [][]float32) ([][]float32, error) {
			vs[0] = bad
			return vs, nil
		}}
		opt := e.opts()
		opt.Registry = f
		s.put("notes/new.md", "New", "delta\n")
		if _, err := e.buildWith(context.Background(), opt, s); errCode(err) != "io" {
			t.Errorf("%s: %v", name, err)
		}
		if s.manifestDB() != db {
			t.Errorf("%s: a failed build published", name)
		}
	}
}

func nanValue() float64 {
	var zero float64
	return zero / zero
}

func TestBuildRestoresModel(t *testing.T) {
	e := newEnv(t)
	s := newSection(t, item.Personal)
	basic(s)
	e.mustBuild(s)
	opens := e.kit.Opens.Load()
	// The cache deleted: the model is downloaded and verified, nothing is embedded.
	os.RemoveAll(filepath.Join(e.path, "models"))
	e.reopen()
	r := e.mustBuild(s)
	if r.PassagesEmbedded != 0 || e.dl.Requests() != 2 || e.kit.Opens.Load() != opens {
		t.Fatalf("restore: %+v, %d requests", r, e.dl.Requests())
	}
	if _, err := os.Stat(filepath.Join(e.path, "models", embed.FolderName(e.m()), "keywords.txt")); err != nil {
		t.Fatal("the model was not restored")
	}
	// Wrong weights of the right size in the cache: removed, downloaded again.
	p := filepath.Join(e.path, "models", embed.FolderName(e.m()), "keywords.txt")
	good, _ := os.ReadFile(p)
	os.Remove(p)
	os.WriteFile(p, []byte(strings.Repeat("x", len(good))), 0o644)
	if _, err := e.build(s); err != nil || e.dl.Requests() != 3 {
		t.Fatalf("wrong weights in the cache: %v, %d requests", err, e.dl.Requests())
	}
	if b, _ := os.ReadFile(p); string(b) != string(good) {
		t.Fatal("the wrong weights stayed")
	}
	// A download that is never right fails with fetch_failed after one retry.
	os.Remove(p)
	e.dl.Serve(e.m().Files[0].URL, []byte(strings.Repeat("y", len(good))))
	if _, err := e.build(s); errCode(err) != "fetch_failed" {
		t.Fatalf("a wrong download: %v", err)
	}
	e.dl.Serve(e.m().Files[0].URL, good)

	// Wrong weights of the right size at model_path: invalid_config, the index as it was.
	mp := t.TempDir()
	os.WriteFile(filepath.Join(mp, "keywords.txt"), []byte(strings.Repeat("x", len(good))), 0o644)
	md, _ := sys.OpenDir(mp)
	defer md.Close()
	opt := e.opts()
	opt.ModelDir = md
	db := s.manifestDB()
	requests := e.dl.Requests()
	if _, err := e.buildWith(context.Background(), opt, s); errCode(err) != "invalid_config" {
		t.Fatalf("wrong weights at model_path: %v", err)
	}
	if s.manifestDB() != db || e.dl.Requests() != requests {
		t.Fatal("a refused model_path published or downloaded")
	}
	// The right weights there: no request.
	os.WriteFile(filepath.Join(mp, "keywords.txt"), good, 0o644)
	s.put("notes/d.md", "Delta", "delta\n")
	if r, err := e.buildWith(context.Background(), opt, s); err != nil || r.PassagesEmbedded != 1 || e.dl.Requests() != requests {
		t.Fatalf("model_path: %+v, %v, %d requests", r, err, e.dl.Requests())
	}
}

func TestModelIdentityBuild(t *testing.T) {
	e := newEnv(t)
	e.model = embedtest.ModelPair
	s := newSection(t, item.Personal)
	basic(s)
	// A two-file model at model_path with its companion missing: invalid_config, nothing published.
	mp := t.TempDir()
	pair := e.m()
	os.WriteFile(filepath.Join(mp, "keywords.txt"), e.kit.Bytes[pair.Files[0].URL], 0o644)
	md, _ := sys.OpenDir(mp)
	defer md.Close()
	opt := e.opts()
	opt.ModelDir = md
	opt.Cache = nil
	if _, err := e.buildWith(context.Background(), opt, s); errCode(err) != "invalid_config" {
		t.Fatalf("a companion missing: %v", err)
	}
	if slices.Contains(s.indexNames(), "manifest") {
		t.Fatal("published without a model")
	}
	// Whole, offline: it builds with no request.
	os.WriteFile(filepath.Join(mp, "settings.txt"), e.kit.Bytes[pair.Files[1].URL], 0o644)
	if r, err := e.buildWith(context.Background(), opt, s); err != nil || r.PassagesEmbedded != 3 || e.dl.Requests() != 0 {
		t.Fatalf("offline pair: %+v %v %d", r, err, e.dl.Requests())
	}
}

func TestModelProvisionShared(t *testing.T) {
	e := newEnv(t)
	e.model = embedtest.ModelPacked
	m := e.m()
	a, b := newSection(t, item.Shared), newSection(t, item.Personal)
	basic(a)
	basic(b)
	var mu sync.Mutex
	e.dl.Before = func(string) { time.Sleep(50 * time.Millisecond) }
	var wg sync.WaitGroup
	errsOf := make([]error, 2)
	for i, s := range []*section{a, b} {
		wg.Go(func() {
			_, err := e.build(s)
			mu.Lock()
			errsOf[i] = err
			mu.Unlock()
		})
	}
	wg.Wait()
	if errsOf[0] != nil || errsOf[1] != nil {
		t.Fatal(errsOf)
	}
	if e.dl.Requests(m.Source[0].URL) != 1 || e.dl.Requests() != 1 {
		t.Fatalf("%d requests", e.dl.Requests())
	}
	packed := filepath.Join(e.path, "models", embed.FolderName(m))
	if _, err := os.Stat(filepath.Join(packed, "keywords.txt")); err != nil {
		t.Fatal("no pack")
	}
	if _, err := os.Stat(filepath.Join(packed+".source", "keywords.src")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the source stayed after the pack verified")
	}
	// A cancelled wait for the model lock ends with the context's error.
	models, _ := sys.OpenDir(filepath.Join(e.path, "models"))
	defer models.Close()
	held, err := sys.Native().Lock(context.Background(), models, embed.FolderName(m)+".lock")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	c := newSection(t, item.Personal)
	basic(c)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := e.buildWith(ctx, e.opts(), c); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a cancelled wait for the model lock: %v", err)
	}
}

func TestVectorCache(t *testing.T) {
	e := newEnv(t)
	fp := e.m().Fingerprint
	d := [32]byte{0xab, 1}
	v := []float32{0, 1, 0, 0, 0, 0, 0, 0}
	if _, ok, err := e.cache.readVector(fp, d, 8); ok || err != nil {
		t.Fatal("an empty cache gave a vector")
	}
	if err := e.cache.writeVector(fp, d, v); err != nil {
		t.Fatal(err)
	}
	got, ok, err := e.cache.readVector(fp, d, 8)
	if !ok || err != nil || !slices.Equal(got, v) {
		t.Fatalf("round trip: %v %v %v", got, ok, err)
	}
	p := filepath.Join(e.path, "vectors", fp, "ab", hexOf(d))
	good, _ := os.ReadFile(p)
	if len(good) != 36 {
		t.Fatalf("%d bytes", len(good))
	}
	// An existing name is left alone.
	if err := e.cache.writeVector(fp, d, []float32{1, 0, 0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != string(good) {
		t.Fatal("an existing entry was replaced")
	}
	flipped := slices.Clone(good)
	flipped[0] ^= 1
	notUnit := encodeVector([]float32{0.5, 0, 0, 0, 0, 0, 0, 0})
	for name, data := range map[string][]byte{
		"short":     good[:35],
		"long":      append(slices.Clone(good), 0),
		"bad crc":   flipped,
		"not unit":  notUnit,
		"other dim": encodeVector([]float32{1, 0}),
	} {
		os.WriteFile(p, data, 0o644)
		if _, ok, _ := e.cache.readVector(fp, d, 8); ok {
			t.Errorf("%s was accepted", name)
		}
		if _, err := os.Stat(p); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s was not removed", name)
		}
	}
	if names, _ := os.ReadDir(filepath.Dir(p)); len(names) != 0 {
		t.Fatalf("temporary files stayed: %v", names)
	}
}

func hexOf(d [32]byte) string {
	const digits = "0123456789abcdef"
	var b strings.Builder
	for _, c := range d {
		b.WriteByte(digits[c>>4])
		b.WriteByte(digits[c&15])
	}
	return b.String()
}

func gitRunner(t *testing.T) (gitx.Runner, func(dir string, args ...string)) {
	t.Helper()
	g, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	home := t.TempDir()
	envv := []string{"HOME=" + home, "XDG_CONFIG_HOME=" + home, "GIT_CONFIG_NOSYSTEM=1", "PATH=" + os.Getenv("PATH")}
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(g, args...)
		cmd.Dir, cmd.Env = dir, envv
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return gitx.NewRunner(g, envv), run
}

func TestCachePlacement(t *testing.T) {
	ctx := context.Background()
	git, run := gitRunner(t)
	base, _ := filepath.EvalSymlinks(t.TempDir())
	brainDir := filepath.Join(base, "repo", "brain")
	personal := filepath.Join(base, "personal")
	os.MkdirAll(brainDir, 0o755)
	os.MkdirAll(personal, 0o755)
	repo := filepath.Join(base, "repo")
	run(repo, "init", "-q")
	os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("ignored/\nforced/\n"), 0o644)
	os.MkdirAll(filepath.Join(repo, "forced"), 0o755)
	os.WriteFile(filepath.Join(repo, "forced", "f"), []byte("x"), 0o644)
	run(repo, "add", "-f", "forced/f")
	os.Symlink(brainDir, filepath.Join(base, "link-to-brain"))
	forbidden := []string{brainDir, personal}
	try := func(p string) error {
		c, err := OpenCache(ctx, CacheOptions{Path: p, Forbidden: forbidden, Git: git, Files: sys.Native()}, false)
		if err == nil {
			c.Close()
		}
		return err
	}
	for _, p := range []string{filepath.Join(base, "outside", "cache"), filepath.Join(repo, "ignored", "cache"), filepath.Join(repo, "ignored")} {
		if err := try(p); err != nil {
			t.Errorf("accepted %s: %v", p, err)
		}
	}
	for _, p := range []string{brainDir, filepath.Join(brainDir, "missing", "child"), personal, filepath.Join(personal, "x"),
		filepath.Join(base, "link-to-brain"), filepath.Join(base, "link-to-brain", "x"), filepath.Join(repo, "notignored"),
		filepath.Join(repo, "forced"), repo} {
		if err := try(p); !errors.Is(err, ErrCacheExposed) {
			t.Errorf("refused %s: %v", p, err)
		}
	}
	// The home folder as a work tree: an ignored cache inside it is accepted.
	home := filepath.Join(base, "home")
	os.MkdirAll(home, 0o755)
	run(home, "init", "-q")
	os.WriteFile(filepath.Join(home, ".gitignore"), []byte(".cache/\n"), 0o644)
	if err := try(filepath.Join(home, ".cache", "pbrain")); err != nil {
		t.Errorf("an ignored cache in a home work tree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".cache")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("OpenCache without create made a folder")
	}

	// A nested repository inside the cache, as a folder and as a file, stops the build before it
	// writes into that folder.
	e := newEnv(t)
	s := newSection(t, item.Shared)
	basic(s)
	e.mustBuild(s)
	fp := e.m().Fingerprint
	for _, rel := range []string{"", "vectors", "vectors/" + fp, "vectors/" + fp + "/" + "ab", "models", "models/" + embed.FolderName(e.m())} {
		for _, asFile := range []bool{false, true} {
			dir := filepath.Join(e.path, filepath.FromSlash(rel))
			os.MkdirAll(dir, 0o755)
			g := filepath.Join(dir, ".git")
			if asFile {
				os.WriteFile(g, []byte("gitdir: elsewhere"), 0o644)
			} else {
				os.Mkdir(g, 0o755)
			}
			c, err := OpenCache(ctx, CacheOptions{Path: e.path, Files: sys.Native()}, false)
			if rel == "" {
				if !errors.Is(err, ErrCacheExposed) {
					t.Errorf(".git at the root: %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				fresh := newSection(t, item.Shared)
				// Text whose digest's first byte is not known in advance: the two-character case
				// is covered by a direct read below.
				fresh.put("x.md", "X", "gamma "+rel+"\n")
				opt := e.opts()
				opt.Cache = c
				_, err := e.buildWith(ctx, opt, fresh)
				if strings.HasSuffix(rel, "/ab") {
					if _, _, rerr := c.readVector(fp, [32]byte{0xab}, 8); !errors.Is(rerr, ErrCacheExposed) {
						t.Errorf(".git in %s, read: %v", rel, rerr)
					}
				} else if errCode(err) != "invalid_config" {
					t.Errorf(".git in %s (file %v): %v", rel, asFile, err)
				}
				c.Close()
			}
			os.RemoveAll(g)
		}
	}
}

func TestBuildKeepsLockAndTooLarge(t *testing.T) {
	e := newEnv(t)
	s := newSection(t, item.Personal)
	basic(s)
	e.mustBuild(s)
	lockPath := filepath.Join(s.index(), "lock")
	before, err := os.Stat(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	// An untrusted manifest that names the lock: the build repairs the index and the lock file is
	// the same file afterwards. While the build runs, another process cannot take it.
	os.WriteFile(filepath.Join(s.index(), "manifest"), []byte(`{"format":2,"db":"lock"}`), 0o644)
	idx, _ := sys.OpenDir(s.index())
	defer idx.Close()
	probe := func(p Progress) {}
	_ = probe
	e.dl.Before = nil
	var heldDuring bool
	f := &failing{Registry: e.kit.Registry, fail: func(_ int, vs [][]float32) ([][]float32, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if l, err := sys.Native().Lock(ctx, idx, "lock"); err == nil {
			l.Close()
		} else {
			heldDuring = true
		}
		return vs, nil
	}}
	opt := e.opts()
	opt.Registry = f
	if _, err := e.buildWith(context.Background(), opt, s); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(lockPath)
	if err != nil || !os.SameFile(before, after) || !heldDuring {
		t.Fatalf("the lock file: same %v, held %v", err == nil && os.SameFile(before, after), heldDuring)
	}
	// A database that would pass the limit fails with too_large and the old index stays.
	db := s.manifestDB()
	defer func(old int64) { maxDatabaseSize = old }(maxDatabaseSize)
	info, _ := os.Stat(filepath.Join(s.index(), db))
	maxDatabaseSize = info.Size()
	for i := range 300 {
		name := string(rune('a'+i%26)) + string(rune('a'+i/26%26))
		s.put("more/"+name+".md", "M "+name, strings.Repeat("delta "+name+" ", 20)+"\n")
	}
	if r, err := e.build(s); errCode(err) != "too_large" {
		after, _ := os.Stat(filepath.Join(s.index(), s.manifestDB()))
		t.Fatalf("past the limit %d: %v %+v, now %d", maxDatabaseSize, err, r, after.Size())
	}
	if s.manifestDB() != db {
		t.Fatal("the old index did not stay")
	}
}

// writeFormat2 replaces the section's index with one of format 2 as v0.1.1 published it: that
// release's schema, an item row without crc and size, and a manifest naming the file's SHA-256.
func writeFormat2(t *testing.T, s *section, m embed.Model) string {
	t.Helper()
	ctx := context.Background()
	x, err := memoryDB(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	for _, q := range []string{
		"CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL) WITHOUT ROWID",
		"CREATE TABLE item (id INTEGER PRIMARY KEY, path TEXT NOT NULL UNIQUE, content BLOB NOT NULL CHECK (length(content) = 32))",
		fmt.Sprintf("CREATE TABLE passage (id INTEGER PRIMARY KEY, item_id INTEGER NOT NULL REFERENCES item(id) ON DELETE CASCADE, ord INTEGER NOT NULL, start INTEGER NOT NULL, stop INTEGER NOT NULL CHECK (stop >= start), digest BLOB NOT NULL CHECK (length(digest) = 32), vector BLOB NOT NULL CHECK (length(vector) = %d), UNIQUE (item_id, ord))", 4*m.Dim),
		"CREATE INDEX passage_digest ON passage(digest)",
		"INSERT INTO meta (key, value) VALUES ('format', '2'), ('fingerprint', '" + IndexFingerprint(m) + "'), ('model', '" + m.Name + "'), ('dim', '" + strconv.Itoa(m.Dim) + "')",
		"INSERT INTO item (id, path, content) VALUES (1, 'notes/a.md', zeroblob(32))",
		fmt.Sprintf("INSERT INTO passage (item_id, ord, start, stop, digest, vector) VALUES (1, 0, 0, 10, zeroblob(32), zeroblob(%d))", 4*m.Dim),
	} {
		if _, err := x.conn.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	data, err := x.serialize(ctx)
	if err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(s.index())
	os.MkdirAll(s.index(), 0o700)
	const db = "0123456789abcdef.db"
	sum := sha256.Sum256(data)
	manifest := fmt.Sprintf(`{"format":2,"fingerprint":%q,"model":%q,"dim":%d,"db":%q,"size":%d,"sha256":%q}`,
		IndexFingerprint(m), m.Name, m.Dim, db, len(data), hex.EncodeToString(sum[:]))
	if err := os.WriteFile(filepath.Join(s.index(), db), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.index(), "manifest"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	return db
}

// An index of format 2 is unreadable; the next build replaces it, taking every shared vector from
// the cache and embedding the personal section again.
func TestIndexFormatChange(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	shared, personal := newSection(t, item.Shared), newSection(t, item.Personal)
	basic(shared)
	personal.put("me.md", "Mine", "delta private\n")
	e.mustBuild(shared) // fills the cache with the shared vectors
	e.mustBuild(personal)
	opens, texts := e.kit.Opens.Load(), e.kit.Texts.Load()
	for _, s := range []*section{shared, personal} {
		old := writeFormat2(t, s, e.m())
		in, _ := s.scan(ctx)
		res, err := Status(ctx, e.opts(), []SectionInput{in})
		if err != nil || res.Sections[0].State != IndexUnreadable || res.Sections[0].Indexed != 0 {
			t.Fatalf("%v status: %+v %v", s.kind, res, err)
		}
		src, items := e.source(ctx, e.opts(), "alpha", s)
		sr, _ := src.Score(ctx, "alpha", items)
		src.Close()
		if sr.Ready || sr.Reason != search.NoIndex {
			t.Fatalf("%v search: %+v", s.kind, sr)
		}
		r := e.mustBuild(s)
		x, state := s.open(e.m())
		if state != IndexOK || len(x.items) != r.Items {
			t.Fatalf("%v: %v after the build", s.kind, state)
		}
		x.Close()
		if names := s.indexNames(); slices.Contains(names, old) || len(names) != 3 {
			t.Fatalf("%v: index/ holds %q", s.kind, names)
		}
		if s == shared {
			if r.PassagesEmbedded != 0 || r.PassagesReused != 3 || e.kit.Opens.Load() != opens {
				t.Fatalf("shared: %+v, opens %d", r, e.kit.Opens.Load()-opens)
			}
		} else if r.PassagesEmbedded != 1 || e.kit.Texts.Load() != texts+1 {
			t.Fatalf("personal: %+v", r)
		}
	}
}
