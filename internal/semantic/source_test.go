package semantic

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/embed/embedtest"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/search"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// built is a shared section with an index and a model in the cache.
func built(t *testing.T) (*env, *section) {
	t.Helper()
	e := newEnv(t)
	s := newSection(t, item.Shared)
	s.put("notes/a.md", "Alpha", "alpha text\n")
	s.put("notes/b.md", "Beta", "beta text\n\n## Part\n\nalpha and beta\n")
	s.put("notes/c.md", "Gamma", "gamma text\n")
	e.mustBuild(s)
	return e, s
}

func (e *env) source(ctx context.Context, opt Options, text string, secs ...*section) (*Source, []*item.Item) {
	e.t.Helper()
	var dirs []SectionDir
	for _, s := range secs {
		dirs = append(dirs, s.sectionDir())
	}
	src := Start(ctx, opt, text, dirs)
	var ins []SectionInput
	var all []*item.Item
	for _, s := range secs {
		in, _ := s.scan(ctx)
		ins = append(ins, in)
		all = append(all, in.Items...)
	}
	src.Attach(ins)
	return src, all
}

func byID(items []*item.Item, id string) *item.Item {
	for _, it := range items {
		if it.ID == id {
			return it
		}
	}
	return nil
}

func TestSourceScores(t *testing.T) {
	ctx := context.Background()
	e, s := built(t)
	opens := e.kit.Opens.Load()
	src, items := e.source(ctx, e.opts(), "alpha", s)
	defer src.Close()
	res, err := src.Score(ctx, "alpha", items)
	if err != nil || !res.Ready || res.Missing != 0 || res.Cutoff != embedtest.Cutoff {
		t.Fatalf("%+v %v", res, err)
	}
	// One hit per item that has a passage within the cut-off, in the caller's order; the best
	// passage of b is its alpha part.
	var got []string
	for _, h := range res.Hits {
		got = append(got, h.Item.ID)
		if h.Score < 0 || h.Score > 1 || h.End < h.Start {
			t.Fatalf("hit %+v", h)
		}
	}
	if strings.Join(got, ",") != "notes/a.md,notes/b.md" {
		t.Fatalf("hits %q", got)
	}
	b := res.Hits[1]
	body := []rune(s.notes["notes/b.md"].body)
	if !strings.Contains(string(body[b.Start:b.End]), "alpha and beta") {
		t.Fatalf("b's span %d..%d: %q", b.Start, b.End, string(body[b.Start:b.End]))
	}
	// Only eligible items are scored; the query is embedded once.
	res, _ = src.Score(ctx, "alpha", []*item.Item{byID(items, "notes/b.md")})
	if len(res.Hits) != 1 || res.Hits[0].Item.ID != "notes/b.md" {
		t.Fatalf("eligible only: %+v", res.Hits)
	}
	if e.kit.Opens.Load() != opens+1 || e.kit.Calls.Load() != 2 {
		t.Fatalf("opens %d, calls %d", e.kit.Opens.Load()-opens, e.kit.Calls.Load())
	}
	// Of two qualifying passages the closest is kept: for "alpha beta", b's second part.
	src2, items3 := e.source(ctx, e.opts(), "alpha beta", s)
	defer src2.Close()
	res, _ = src2.Score(ctx, "alpha beta", items3)
	for _, h := range res.Hits {
		if h.Item.ID == "notes/b.md" && !strings.Contains(string(body[h.Start:h.End]), "alpha and beta") {
			t.Fatalf("b's closest passage: %q", string(body[h.Start:h.End]))
		}
	}
	// Identical across runs.
	again, items2 := e.source(ctx, e.opts(), "alpha", s)
	defer again.Close()
	r2, _ := again.Score(ctx, "alpha", items2)
	r1, _ := src.Score(ctx, "alpha", items)
	key := func(r search.Scores) string {
		var b strings.Builder
		for _, h := range r.Hits {
			fmt.Fprintf(&b, "%s %v %d %d;", h.Item.ID, h.Score, h.Start, h.End)
		}
		return b.String()
	}
	if key(r1) == "" || key(r1) != key(r2) {
		t.Fatalf("two runs differ: %s / %s", key(r1), key(r2))
	}
}

func TestSourceNotReady(t *testing.T) {
	ctx := context.Background()
	e, s := built(t)
	run := func(opt Options) search.Scores {
		t.Helper()
		src, items := e.source(ctx, opt, "alpha", s)
		defer src.Close()
		res, err := src.Score(ctx, "alpha", items)
		if err != nil || res.Ready || len(res.Hits) != 0 {
			t.Fatalf("%+v %v", res, err)
		}
		return res
	}
	opens := e.kit.Opens.Load()
	before := cacheFiles(t, e.path)
	opt := e.opts()
	opt.ModelName = ""
	if r := run(opt); r.Reason != search.NoModel {
		t.Errorf("no model: %v", r.Reason)
	}
	opt = e.opts()
	opt.ModelName = "nope"
	if r := run(opt); r.Reason != search.UnsupportedModel {
		t.Errorf("unknown: %v", r.Reason)
	}
	opt = e.opts()
	opt.CacheErr = ErrCacheExposed
	if r := run(opt); r.Reason != search.CacheExposed {
		t.Errorf("exposed: %v", r.Reason)
	}
	// A .git folder on the way to the model.
	g := filepath.Join(e.path, "models", embed.FolderName(e.m()), ".git")
	os.Mkdir(g, 0o755)
	c, _ := OpenCache(ctx, CacheOptions{Path: e.path, Files: sys.Native()}, false)
	opt = e.opts()
	opt.Cache = c
	if r := run(opt); r.Reason != search.CacheExposed {
		t.Errorf("a nested repository: %v", r.Reason)
	}
	c.Close()
	os.Remove(g)
	// The model's file of the wrong size, or no cache.
	p := filepath.Join(e.path, "models", embed.FolderName(e.m()), "keywords.txt")
	good, _ := os.ReadFile(p)
	os.WriteFile(p, good[:len(good)-1], 0o644)
	if r := run(e.opts()); r.Reason != search.ModelAbsent {
		t.Errorf("wrong size: %v", r.Reason)
	}
	os.WriteFile(p, good, 0o644)
	opt = e.opts()
	opt.Cache = nil
	if r := run(opt); r.Reason != search.ModelAbsent {
		t.Errorf("no cache: %v", r.Reason)
	}
	// An index built for another model, and none at all.
	opt = e.opts()
	opt.ModelName = embedtest.ModelB
	e.dl.Serve("", nil)
	if _, err := Build(ctx, Options{Files: sys.Native(), Registry: e.kit.Registry, ModelName: embedtest.ModelB, Cache: e.cache, Rand: opt.Rand}, newSection(t, item.Shared).sectionDir(),
		func(context.Context) (SectionInput, error) { return SectionInput{}, nil }, e.dl, nil, nil); err != nil {
		t.Fatal(err)
	}
	if r := run(opt); r.Reason != search.ModelAbsent && r.Reason != search.NoIndex {
		t.Errorf("another model: %v", r.Reason)
	}
	os.RemoveAll(filepath.Join(s.root, ".pbrain"))
	if r := run(e.opts()); r.Reason != search.NoIndex {
		t.Errorf("no index: %v", r.Reason)
	}
	// No check opened a model, wrote a file or made a request.
	if e.kit.Opens.Load() != opens || cacheFiles(t, e.path) != before {
		t.Fatalf("a not-ready source opened a model or wrote: %d opens", e.kit.Opens.Load()-opens)
	}
}

func cacheFiles(t *testing.T, root string) string {
	var names []string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, p)
		}
		return nil
	})
	return strings.Join(names, "\n")
}

// badQuery wraps the kit so that a query embeds to v.
type badQuery struct {
	embed.Registry
	v []float32
}

type badQueryEmb struct {
	embed.Embedder
	v []float32
}

func (b badQuery) Open(ctx context.Context, a *embed.Artifact) (embed.Embedder, error) {
	e, err := b.Registry.Open(ctx, a)
	return badQueryEmb{e, b.v}, err
}

func (b badQueryEmb) Embed(ctx context.Context, role embed.Role, texts []string) ([][]float32, error) {
	if role == embed.Query {
		return [][]float32{b.v}, nil
	}
	return b.Embedder.Embed(ctx, role, texts)
}

func TestSourceBadQueryVector(t *testing.T) {
	ctx := context.Background()
	e, s := built(t)
	for name, v := range map[string][]float32{
		"wrong dimension": {1, 0},
		"not a number":    {float32(math.NaN()), 0, 0, 0, 0, 0, 0, 0},
		"not unit length": {0.5, 0, 0, 0, 0, 0, 0, 0},
	} {
		opt := e.opts()
		opt.Registry = badQuery{e.kit.Registry, v}
		src, items := e.source(ctx, opt, "alpha", s)
		res, err := src.Score(ctx, "alpha", items)
		src.Close()
		if err != nil || res.Ready || res.Reason != search.ModelFailed || len(res.Hits) != 0 {
			t.Errorf("%s: %+v %v", name, res, err)
		}
	}
}

func TestSourceLifecycle(t *testing.T) {
	ctx := context.Background()
	e, s := built(t)
	// Close without Attach or Score, after Score, and twice: the embedder is closed once, if opened.
	src := Start(ctx, e.opts(), "alpha", []SectionDir{s.sectionDir()})
	src.Close()
	src.Close()
	if e.kit.Opens.Load() != e.kit.Closes.Load() {
		t.Fatalf("opens %d, closes %d", e.kit.Opens.Load(), e.kit.Closes.Load())
	}
	src, items := e.source(ctx, e.opts(), "alpha", s)
	src.Score(ctx, "alpha", items)
	src.Close()
	if e.kit.Opens.Load() != e.kit.Closes.Load() {
		t.Fatalf("after Score: opens %d, closes %d", e.kit.Opens.Load(), e.kit.Closes.Load())
	}
	// A not-ready source closes without opening anything.
	opt := e.opts()
	opt.ModelName = ""
	Start(ctx, opt, "x", nil).Close()
	// A cancelled context ends Score with its error.
	cctx, cancel := context.WithCancel(ctx)
	block := make(chan struct{})
	var entered atomic.Bool
	opt = e.opts()
	opt.Registry = blockingOpen{e.kit.Registry, block, &entered}
	src, items = e.source(cctx, opt, "alpha", s)
	cancel()
	if _, err := src.Score(cctx, "alpha", items); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled Score: %v", err)
	}
	close(block)
	src.Close()
	// Score with another text is a programming error.
	src, items = e.source(ctx, e.opts(), "alpha", s)
	defer src.Close()
	defer func() {
		if recover() == nil {
			t.Fatal("another text did not panic")
		}
	}()
	src.Score(ctx, "beta", items)
}

type blockingOpen struct {
	embed.Registry
	block   chan struct{}
	entered *atomic.Bool
}

func (b blockingOpen) Open(ctx context.Context, a *embed.Artifact) (embed.Embedder, error) {
	b.entered.Store(true)
	<-b.block
	return b.Registry.Open(ctx, a)
}

// TestCoverage: an entry covers an item only with the same path and content digest.
func TestCoverage(t *testing.T) {
	ctx := context.Background()
	e, s := built(t)
	check := func(want int) {
		t.Helper()
		src, items := e.source(ctx, e.opts(), "alpha", s)
		defer src.Close()
		res, _ := src.Score(ctx, "alpha", items)
		if !res.Ready || res.Missing != want {
			t.Fatalf("missing %d, want %d (%+v)", res.Missing, want, res)
		}
	}
	check(0)
	s.put("notes/a.md", "Alpha", "alpha text edited\n") // an edited body
	check(1)
	s.put("notes/a.md", "Alpha renamed", "alpha text\n") // an edited title
	check(1)
	s.put("notes/a.md", "Alpha", "alpha text\n")
	check(0)
	body := s.notes["notes/c.md"]
	s.del("notes/c.md")
	s.put("notes/moved.md", body.title, body.body) // a renamed file
	check(1)
	s.del("notes/moved.md") // a deleted file: nothing to cover
	check(0)
	// Same length, other text: not covered, never scored.
	s.put("notes/a.md", "Alpha", "alphX text\n")
	src, items := e.source(ctx, e.opts(), "alpha", s)
	res, _ := src.Score(ctx, "alpha", items)
	src.Close()
	for _, h := range res.Hits {
		if h.Item.ID == "notes/a.md" {
			t.Fatal("an uncovered item was scored")
		}
	}
	if res.Missing != 1 {
		t.Fatalf("missing %d", res.Missing)
	}
}

// TestSourceSectionFailure: a section whose index is unusable adds its items to Missing; the other
// section's hits stand. With both failing: NoIndex.
func TestSourceSectionFailure(t *testing.T) {
	ctx := context.Background()
	e, shared := built(t)
	personal := newSection(t, item.Personal)
	personal.put("p.md", "Private", "alpha private\n")
	personal.put("q.md", "Other", "delta\n")
	e.mustBuild(personal)
	// Damage the personal database's rows so that the scoring query fails there.
	x, _ := personal.open(e.m())
	x.conn.ExecContext(ctx, "PRAGMA ignore_check_constraints = ON")
	x.conn.ExecContext(ctx, "UPDATE passage SET vector = x'01'")
	d, _ := sys.OpenDir(personal.index())
	x.publish(ctx, sys.Native(), d, rand.Reader)
	d.Close()
	x.Close()
	src, items := e.source(ctx, e.opts(), "alpha", shared, personal)
	res, err := src.Score(ctx, "alpha", items)
	src.Close()
	if err != nil || !res.Ready || res.Missing != 2 || len(res.Hits) != 2 {
		t.Fatalf("one failing section: %+v %v", res, err)
	}
	for _, h := range res.Hits {
		if h.Item.Section == item.Personal {
			t.Fatal("a hit from the failing section")
		}
	}
	os.WriteFile(filepath.Join(shared.index(), "manifest"), []byte("{"), 0o644)
	src, items = e.source(ctx, e.opts(), "alpha", shared, personal)
	res, _ = src.Score(ctx, "alpha", items)
	src.Close()
	if res.Ready || res.Reason != search.NoIndex {
		t.Fatalf("both failing: %+v", res)
	}
}

func TestStatus(t *testing.T) {
	ctx := context.Background()
	e, s := built(t)
	s.put("notes/d.md", "Delta", "delta\n")
	empty := newSection(t, item.Personal)
	opens := e.kit.Opens.Load()
	in1, _ := s.scan(ctx)
	in2, _ := empty.scan(ctx)
	res, err := Status(ctx, e.opts(), []SectionInput{in1, in2})
	if err != nil || res.Model != embedtest.ModelA || !res.Supported || !res.ModelPresent || len(res.Sections) != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if st := res.Sections[0]; st.Items != 4 || st.Indexed != 3 || st.State != IndexOK {
		t.Fatalf("shared: %+v", st)
	}
	if st := res.Sections[1]; st.Items != 0 || st.State != IndexNone {
		t.Fatalf("personal: %+v", st)
	}
	if _, err := os.Stat(filepath.Join(empty.root, ".pbrain")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Status created .pbrain/")
	}
	// Another model: stale; no model named: not supported, stale for an existing index.
	opt := e.opts()
	opt.ModelName = embedtest.ModelB
	res, _ = Status(ctx, opt, []SectionInput{in1})
	if res.Sections[0].State != IndexStale || res.ModelPresent {
		t.Fatalf("another model: %+v", res)
	}
	opt.ModelName = ""
	res, _ = Status(ctx, opt, []SectionInput{in1})
	if res.Supported || res.Sections[0].State != IndexStale || res.Sections[0].Indexed != 0 {
		t.Fatalf("no model: %+v", res)
	}
	// A damaged manifest: unreadable. The model files changed: not present.
	os.WriteFile(filepath.Join(s.index(), "manifest"), []byte("{"), 0o644)
	p := filepath.Join(e.path, "models", embed.FolderName(e.m()), "keywords.txt")
	good, _ := os.ReadFile(p)
	os.WriteFile(p, []byte(strings.Repeat("x", len(good))), 0o644)
	res, _ = Status(ctx, e.opts(), []SectionInput{in1})
	if res.Sections[0].State != IndexUnreadable || res.ModelPresent {
		t.Fatalf("damaged: %+v", res)
	}
	if e.kit.Opens.Load() != opens {
		t.Fatal("Status opened a model")
	}
}

// A build paused before its scan holds index/lock; an item writer can still take .pbrain/lock.
func TestBuildHoldsOnlyTheIndexLock(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newSection(t, item.Shared)
	basic(s)
	e.mustBuild(s)
	pb, _ := sys.OpenDir(filepath.Join(s.root, ".pbrain"))
	defer pb.Close()
	idx, _ := sys.OpenDir(s.index())
	defer idx.Close()
	checked := false
	scan := func(ctx context.Context) (SectionInput, error) {
		short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		if l, err := sys.Native().Lock(short, idx, "lock"); err == nil {
			l.Close()
			t.Error("the index lock is free during the scan")
		}
		l, err := sys.Native().Lock(short, pb, "lock")
		if err != nil {
			t.Errorf("an item writer cannot lock: %v", err)
		} else {
			l.Close()
		}
		checked = true
		return s.scan(ctx)
	}
	if _, err := Build(ctx, e.opts(), s.sectionDir(), scan, e.dl, nil, nil); err != nil || !checked {
		t.Fatalf("%v, checked %v", err, checked)
	}
}

// A search during builds that publish reads one whole database, the old or the new.
func TestReadDuringPublication(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.clock.step = 30 * time.Second // every item boundary publishes
	s := newSection(t, item.Shared)
	basic(s)
	e.mustBuild(s)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 15 {
			s.put(fmt.Sprintf("more/n%02d.md", i), "More", fmt.Sprintf("delta %d\n", i))
			if _, err := e.build(s); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	reads := 0
	for {
		select {
		case <-done:
			if reads == 0 {
				t.Fatal("no read overlapped the builds")
			}
			return
		default:
		}
		d, err := sys.OpenDir(s.index())
		if err != nil {
			t.Fatal(err)
		}
		x, state := openIndex(ctx, sys.Native(), d, IndexFingerprint(e.m()), e.m().Dim, true)
		d.Close()
		if state != IndexOK {
			t.Fatalf("read %d during a publication: %v", reads, state)
		}
		x.Close()
		reads++
	}
}

// A row whose span passes the body's end is not used; an item not listed is never scored, even
// with a malformed vector.
func TestSourceRowsChecked(t *testing.T) {
	ctx := context.Background()
	e, s := built(t)
	damage := func(sql string) {
		t.Helper()
		x, _ := s.open(e.m())
		x.conn.ExecContext(ctx, "PRAGMA ignore_check_constraints = ON")
		if _, err := x.conn.ExecContext(ctx, sql); err != nil {
			t.Fatal(err)
		}
		d, _ := sys.OpenDir(s.index())
		if _, err := x.publish(ctx, sys.Native(), d, rand.Reader); err != nil {
			t.Fatal(err)
		}
		d.Close()
		x.Close()
	}
	damage("UPDATE passage SET stop = 9999 WHERE item_id = (SELECT id FROM item WHERE path = 'notes/a.md')")
	src, items := e.source(ctx, e.opts(), "alpha", s)
	res, _ := src.Score(ctx, "alpha", items)
	src.Close()
	if !res.Ready {
		t.Fatal(res)
	}
	for _, h := range res.Hits {
		if h.Item.ID == "notes/a.md" {
			t.Fatalf("a span past the body was used: %+v", h)
		}
	}
	damage("UPDATE passage SET vector = x'01' WHERE item_id = (SELECT id FROM item WHERE path = 'notes/c.md')")
	src, items = e.source(ctx, e.opts(), "alpha", s)
	defer src.Close()
	var listed []*item.Item
	for _, it := range items {
		if it.ID != "notes/c.md" {
			listed = append(listed, it)
		}
	}
	res, err := src.Score(ctx, "alpha", listed)
	if err != nil || !res.Ready || res.Missing != 0 || len(res.Hits) == 0 {
		t.Fatalf("an unlisted malformed item broke the query: %+v %v", res, err)
	}
}
