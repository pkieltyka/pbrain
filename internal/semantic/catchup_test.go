package semantic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed/embedtest"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/search"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// lockFiles counts TryLock calls and can make them fail.
type lockFiles struct {
	sys.Files
	tries atomic.Int32
	fail  error
}

func (f *lockFiles) TryLock(d sys.Dir, name string) (io.Closer, error) {
	f.tries.Add(1)
	if f.fail != nil {
		return nil, f.fail
	}
	return f.Files.TryLock(d, name)
}

// search runs one search as cli does: Start, Attach, CatchUp, Score, Close.
func (e *env) search(ctx context.Context, opt Options, text string, secs ...*section) (search.Scores, error) {
	e.t.Helper()
	src, items := e.source(ctx, opt, text, secs...)
	defer src.Close()
	if err := src.CatchUp(ctx); err != nil {
		return search.Scores{}, err
	}
	return src.Score(ctx, text, items)
}

func hit(res search.Scores, id string) bool {
	for _, h := range res.Hits {
		if h.Item.ID == id {
			return true
		}
	}
	return false
}

// files lists every file below root, so a test can see that nothing was written.
func files(t *testing.T, root string) string { return cacheFiles(t, root) }

// sections makes a body of n headed sections, each one passage.
func sections(n int, word string) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "# Part %d\n\n%s part %d\n\n", i, word, i)
	}
	return b.String()
}

func TestCatchUp(t *testing.T) {
	ctx := context.Background()
	e, s := built(t)

	// One added note: one ContentDigest, one passage embedded, published, and found.
	s.put("notes/d.md", "Delta", "alpha delta new\n")
	db, digests, calls := s.manifestDB(), contentCalls.digest.Load(), e.kit.Calls.Load()
	res, err := e.search(ctx, e.opts(), "alpha", s)
	if err != nil || !res.Ready || res.Missing != 0 || !hit(res, "notes/d.md") {
		t.Fatalf("one added note: %+v %v", res, err)
	}
	if contentCalls.digest.Load()-digests != 1 || e.kit.Calls.Load()-calls != 2 || s.manifestDB() == db {
		t.Fatalf("%d digests, %d calls, published %v", contentCalls.digest.Load()-digests, e.kit.Calls.Load()-calls, s.manifestDB() != db)
	}
	if r := e.mustBuild(s); r.PassagesEmbedded != 0 || r.PassagesReused != 0 {
		t.Fatalf("the build after the catch-up: %+v", r)
	}

	// An edited item: replaced, and never scored for its old text.
	s.put("notes/a.md", "Plain", "gamma text now\n")
	res, _ = e.search(ctx, e.opts(), "alpha", s)
	if res.Missing != 0 || hit(res, "notes/a.md") {
		t.Fatalf("an edited item, old text: %+v", res)
	}
	if res, _ = e.search(ctx, e.opts(), "gamma", s); !hit(res, "notes/a.md") {
		t.Fatalf("an edited item, new text: %+v", res)
	}

	// A renamed item: nothing embedded, the entry under the new path, the old row left.
	body := s.notes["notes/c.md"]
	s.del("notes/c.md")
	s.put("notes/moved.md", body.title, body.body)
	calls = e.kit.Calls.Load()
	if res, _ = e.search(ctx, e.opts(), "gamma", s); res.Missing != 0 || e.kit.Calls.Load()-calls != 1 {
		t.Fatalf("a renamed item: %+v, %d calls", res, e.kit.Calls.Load()-calls)
	}
	x, _ := s.open(e.m())
	_, moved := x.items["notes/moved.md"]
	_, old := x.items["notes/c.md"]
	x.Close()
	if !moved || !old {
		t.Fatalf("after a rename: new %v, old %v", moved, old)
	}

	// Front matter only, or nothing uncovered: no lock, no write, and the model is not waited for.
	s.put("notes/b.md", s.notes["notes/b.md"].title, s.notes["notes/b.md"].body)
	lf := &lockFiles{Files: sys.Native()}
	opt := e.opts()
	opt.Files = lf
	block := make(chan struct{})
	var entered atomic.Bool
	opt.Registry = blockingOpen{e.kit.Registry, block, &entered}
	before := files(t, s.root) + files(t, e.path)
	src, _ := e.source(ctx, opt, "alpha", s)
	done := make(chan error, 1)
	go func() { done <- src.CatchUp(ctx) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("CatchUp waited for the model with nothing to catch up")
	}
	close(block)
	src.Close()
	if lf.tries.Load() != 0 || files(t, s.root)+files(t, e.path) != before {
		t.Fatalf("nothing uncovered: %d locks, files changed %v", lf.tries.Load(), files(t, s.root)+files(t, e.path) != before)
	}

	// Shared: the new vector is in the cache, and a second work tree's search reuses it.
	other := newSection(t, item.Shared)
	for rel, n := range s.notes {
		other.put(rel, n.title, n.body)
	}
	e.mustBuild(other)
	vectors := filepath.Join(e.path, "vectors")
	n := strings.Count(files(t, vectors), "\n")
	s.put("notes/e.md", "Echo", "beta echo\n")
	e.search(ctx, e.opts(), "beta", s)
	if got := strings.Count(files(t, vectors), "\n"); got != n+1 {
		t.Fatalf("cache files %d, want %d", got, n+1)
	}
	other.put("notes/e.md", "Echo", "beta echo\n")
	opens, texts := e.kit.Opens.Load(), e.kit.Texts.Load()
	if res, _ = e.search(ctx, e.opts(), "beta", other); !hit(res, "notes/e.md") || e.kit.Opens.Load()-opens != 1 || e.kit.Texts.Load()-texts != 1 {
		t.Fatalf("a second work tree: %+v, %d opens, %d texts", res, e.kit.Opens.Load()-opens, e.kit.Texts.Load()-texts)
	}

	// Personal: in the personal index, and no cache file appears.
	p := newSection(t, item.Personal)
	p.put("me.md", "Mine", "delta private\n")
	e.mustBuild(p)
	p.put("you.md", "Yours", "delta other private\n")
	before = files(t, e.path)
	if res, _ = e.search(ctx, e.opts(), "delta", s, p); res.Missing != 0 || !hit(res, "you.md") || files(t, e.path) != before {
		t.Fatalf("personal: %+v, cache changed %v", res, files(t, e.path) != before)
	}
	x, _ = p.open(e.m())
	_, ok := x.items["you.md"]
	x.Close()
	if !ok {
		t.Fatal("the personal item is not in the personal index")
	}
}

func TestCatchUpBounds(t *testing.T) {
	ctx := context.Background()

	// 21 uncovered items over both sections of a personal view: nothing at all.
	e, s := built(t)
	p := newSection(t, item.Personal)
	p.put("me.md", "Mine", "delta private\n")
	e.mustBuild(p)
	for i := range 11 {
		s.put("new/s"+strconv.Itoa(i)+".md", "S", "alpha "+strconv.Itoa(i)+"\n")
	}
	for i := range 10 {
		p.put("new/p"+strconv.Itoa(i)+".md", "P", "delta "+strconv.Itoa(i)+"\n")
	}
	lf := &lockFiles{Files: sys.Native()}
	opt := e.opts()
	opt.Files = lf
	digests, texts, db := contentCalls.digest.Load(), e.kit.Texts.Load(), s.manifestDB()
	res, err := e.search(ctx, opt, "alpha", s, p)
	if err != nil || res.Missing != 21 || contentCalls.digest.Load() != digests || e.kit.Texts.Load()-texts != 1 || lf.tries.Load() != 0 || s.manifestDB() != db {
		t.Fatalf("21 items: %+v %v, %d digests, %d texts, %d locks", res, err, contentCalls.digest.Load()-digests, e.kit.Texts.Load()-texts, lf.tries.Load())
	}

	// 20 items of one passage: all caught up.
	e, s = built(t)
	for i := range 20 {
		s.put("new/n"+strconv.Itoa(i)+".md", "N", "alpha "+strconv.Itoa(i)+"\n")
	}
	if res, _ = e.search(ctx, e.opts(), "alpha", s); res.Missing != 0 {
		t.Fatalf("20 items: %+v", res)
	}

	// 3 items needing 21 passages: nothing written.
	e, s = built(t)
	for i, n := range []int{7, 7, 7} {
		s.put("new/l"+strconv.Itoa(i)+".md", "L"+strconv.Itoa(i), sections(n, "alpha"))
	}
	if got := len(Passages("L0", []byte(sections(7, "alpha")))); got != 7 {
		t.Fatalf("%d passages", got)
	}
	db = s.manifestDB()
	if res, _ = e.search(ctx, e.opts(), "alpha", s); res.Missing != 3 || s.manifestDB() != db {
		t.Fatalf("21 passages: %+v", res)
	}

	// 5 items of 5 passages, 5 of them in the cache from another work tree: 20 to embed, caught up.
	e, s = built(t)
	other := newSection(t, item.Shared)
	other.put("same.md", "Same", sections(5, "beta"))
	e.mustBuild(other)
	s.put("new/same.md", "Same", sections(5, "beta"))
	for i := range 4 {
		s.put("new/f"+strconv.Itoa(i)+".md", "F"+strconv.Itoa(i), sections(5, "alpha"))
	}
	texts = e.kit.Texts.Load()
	if res, _ = e.search(ctx, e.opts(), "alpha", s); res.Missing != 0 || e.kit.Texts.Load()-texts != 21 {
		t.Fatalf("20 after reuse: %+v, %d texts", res, e.kit.Texts.Load()-texts)
	}

	// The clock passes catchUpBudget after the first of three items: one published and scored.
	e, s = built(t)
	for i := range 3 {
		s.put("new/t"+strconv.Itoa(i)+".md", "T", "alpha "+strconv.Itoa(i)+"\n")
	}
	e.clock.step = time.Second // the lock's reading, then one reading before each call
	calls := e.kit.Calls.Load()
	res, _ = e.search(ctx, e.opts(), "alpha", s)
	if res.Missing != 2 || !hit(res, "new/t0.md") || e.kit.Calls.Load()-calls != 2 {
		t.Fatalf("the time bound: %+v, %d calls", res, e.kit.Calls.Load()-calls)
	}
	x, _ := s.open(e.m())
	_, ok := x.items["new/t0.md"]
	_, late := x.items["new/t1.md"]
	x.Close()
	if !ok || late {
		t.Fatalf("published: t0 %v, t1 %v", ok, late)
	}
}

func TestCatchUpLock(t *testing.T) {
	ctx := context.Background()
	e, s := built(t)
	s.put("notes/d.md", "Delta", "alpha delta\n")

	// The lock held by another command: no wait, nothing embedded, nothing written.
	idx, _ := sys.OpenDir(s.index())
	defer idx.Close()
	held, err := sys.Native().Lock(ctx, idx, "lock")
	if err != nil {
		t.Fatal(err)
	}
	db, texts, start := s.manifestDB(), e.kit.Texts.Load(), time.Now()
	res, err := e.search(ctx, e.opts(), "alpha", s)
	if err != nil || res.Missing != 1 || s.manifestDB() != db || e.kit.Texts.Load()-texts != 1 || time.Since(start) > time.Second {
		t.Fatalf("a held lock: %+v %v", res, err)
	}
	held.Close()

	// TryLock failing with a file-system error is treated as held.
	opt := e.opts()
	opt.Files = &lockFiles{Files: sys.Native(), fail: errors.New("injected")}
	if res, _ = e.search(ctx, opt, "alpha", s); res.Missing != 1 || s.manifestDB() != db {
		t.Fatalf("a failing TryLock: %+v", res)
	}

	// A build publishes between the search's index read and its lock: nothing is written.
	src, items := e.source(ctx, e.opts(), "alpha", s)
	<-src.indexes
	e.mustBuild(s)
	built := s.manifestDB()
	texts = e.kit.Texts.Load()
	if err := src.CatchUp(ctx); err != nil {
		t.Fatal(err)
	}
	res, _ = src.Score(ctx, "alpha", items)
	src.Close()
	if s.manifestDB() != built || e.kit.Texts.Load() != texts || res.Missing != 1 {
		t.Fatalf("a newer index: %+v", res)
	}

	// Two searches at once: each answers with or without the item, the index stays whole.
	s.put("notes/e.md", "Echo", "alpha echo\n")
	in, _ := s.scan(ctx) // the fake scan is not safe for concurrent use; both searches share one
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			src := Start(ctx, e.opts(), "alpha", []SectionDir{s.sectionDir()})
			defer src.Close()
			src.Attach([]SectionInput{in})
			if err := src.CatchUp(ctx); err != nil {
				t.Error(err)
				return
			}
			res, err := src.Score(ctx, "alpha", in.Items)
			if err != nil || !res.Ready || res.Missing > 1 {
				t.Errorf("two searches: %+v %v", res, err)
			}
		})
	}
	wg.Wait()
	if x, state := s.open(e.m()); state != IndexOK {
		t.Fatalf("after two searches: %v", state)
	} else {
		x.Close()
	}

	// A build started while a search holds the lock waits, then finds nothing to do.
	s.put("notes/f.md", "Foxtrot", "alpha foxtrot\n")
	release := make(chan struct{})
	reached := make(chan struct{})
	opt = e.opts()
	f := &failing{Registry: e.kit.Registry, fail: func(n int, vs [][]float32) ([][]float32, error) {
		if n == 2 { // the first document call, after the query
			close(reached)
			<-release
		}
		return vs, nil
	}}
	opt.Registry = f
	searched := make(chan search.Scores)
	go func() {
		res, _ := e.search(ctx, opt, "alpha", s)
		searched <- res
	}()
	<-reached
	builtCh := make(chan BuildResult)
	go func() { builtCh <- e.mustBuild(s) }()
	select {
	case <-builtCh:
		t.Fatal("the build did not wait for the search's lock")
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	if res := <-searched; res.Missing != 0 {
		t.Fatalf("the search holding the lock: %+v", res)
	}
	if r := <-builtCh; r.PassagesEmbedded != 0 || r.PassagesReused != 0 {
		t.Fatalf("the waiting build: %+v", r)
	}
}

func TestCatchUpNeverCreates(t *testing.T) {
	ctx := context.Background()
	type state struct {
		name   string
		set    func(e *env, s *section) Options
		reason search.NotReady
	}
	for _, c := range []state{
		{"no index", func(e *env, s *section) Options { os.RemoveAll(filepath.Join(s.root, ".pbrain")); return e.opts() }, search.NoIndex},
		{"another fingerprint", func(e *env, s *section) Options { o := e.opts(); o.ModelName = embedtest.ModelB; return o }, 0},
		{"unreadable", func(e *env, s *section) Options {
			os.WriteFile(filepath.Join(s.index(), "manifest"), []byte("{"), 0o644)
			return e.opts()
		}, search.NoIndex},
		{"format 2", func(e *env, s *section) Options { writeFormat2(t, s, e.m()); return e.opts() }, search.NoIndex},
		{"model missing", func(e *env, s *section) Options { os.RemoveAll(filepath.Join(e.path, "models")); return e.opts() }, search.ModelAbsent},
		{"cache exposed", func(e *env, s *section) Options { o := e.opts(); o.CacheErr = ErrCacheExposed; return o }, search.CacheExposed},
		{"model none", func(e *env, s *section) Options { o := e.opts(); o.ModelName = ""; return o }, search.NoModel},
	} {
		e, s := built(t)
		opt := c.set(e, s)
		s.put("notes/d.md", "Delta", "alpha delta\n")
		lf := &lockFiles{Files: sys.Native()}
		opt.Files = lf
		before := files(t, s.root) + files(t, e.path)
		res, err := e.search(ctx, opt, "alpha", s)
		if err != nil || lf.tries.Load() != 0 || files(t, s.root)+files(t, e.path) != before {
			t.Errorf("%s: %+v %v, %d locks, files changed %v", c.name, res, err, lf.tries.Load(), files(t, s.root)+files(t, e.path) != before)
		}
		if res.Ready || (c.reason != 0 && res.Reason != c.reason) {
			t.Errorf("%s: %+v", c.name, res)
		}
	}

	// model_path and no cache folder: caught up, and no cache folder appears.
	e, s := built(t)
	m := e.m()
	md := t.TempDir()
	for _, f := range m.Files {
		os.WriteFile(filepath.Join(md, f.Name), e.kit.Bytes[f.URL], 0o644)
	}
	dir, err := sys.OpenDir(md)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	missing := filepath.Join(t.TempDir(), "no-cache")
	c, err := OpenCache(ctx, CacheOptions{Path: missing, Files: sys.Native()}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	opt := e.opts()
	opt.Cache, opt.ModelDir = c, dir
	s.put("notes/d.md", "Delta", "alpha delta\n")
	if res, _ := e.search(ctx, opt, "alpha", s); res.Missing != 0 || !hit(res, "notes/d.md") {
		t.Fatalf("model_path: %+v", res)
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a catch-up created the cache folder")
	}

	// A section folder that cannot be written: the search succeeds, partial.
	if os.Geteuid() != 0 {
		e, s = built(t)
		s.put("notes/d.md", "Delta", "alpha delta\n")
		pb := filepath.Join(s.root, ".pbrain")
		filepath.Walk(pb, func(p string, _ os.FileInfo, _ error) error { return os.Chmod(p, 0o500) })
		t.Cleanup(func() { filepath.Walk(pb, func(p string, _ os.FileInfo, _ error) error { return os.Chmod(p, 0o700) }) })
		if res, err := e.search(ctx, e.opts(), "alpha", s); err != nil || !res.Ready || res.Missing != 1 {
			t.Fatalf("a folder that cannot be written: %+v %v", res, err)
		}
	}

	// The cache's vectors folder holds .git: nothing written there, the item stays uncovered.
	e, s = built(t)
	os.MkdirAll(filepath.Join(e.path, "vectors", ".git"), 0o700)
	e.reopen()
	before := files(t, e.path)
	s.put("notes/d.md", "Delta", "alpha delta\n")
	if res, _ := e.search(ctx, e.opts(), "alpha", s); res.Missing != 1 || files(t, e.path) != before {
		t.Fatalf("an exposed vectors folder: %+v", res)
	}
}

// replaceFails wraps the section folder so that renaming a file into place fails.
type replaceFiles struct{ sys.Files }
type replaceDir struct{ sys.Dir }

func unwrapRD(d sys.Dir) sys.Dir {
	if r, ok := d.(replaceDir); ok {
		return r.Dir
	}
	return d
}
func (f replaceFiles) Entries(d sys.Dir) ([]sys.Entry, error) { return f.Files.Entries(unwrapRD(d)) }
func (f replaceFiles) Stat(d sys.Dir, n string) (sys.FileInfo, error) {
	return f.Files.Stat(unwrapRD(d), n)
}
func (f replaceFiles) OpenRegular(d sys.Dir, n string) (sys.RegularFile, error) {
	return f.Files.OpenRegular(unwrapRD(d), n)
}
func (f replaceFiles) TryLock(d sys.Dir, n string) (io.Closer, error) {
	return f.Files.TryLock(unwrapRD(d), n)
}
func (d replaceDir) Sub(name string, create bool) (sys.Dir, error) {
	c, err := d.Dir.Sub(name, create)
	if err != nil {
		return nil, err
	}
	return replaceDir{c}, nil
}
func (replaceDir) Replace(string, string) error { return errors.New("injected rename failure") }

func TestCatchUpFailures(t *testing.T) {
	ctx := context.Background()
	for name, fail := range map[string]func(n int, vs [][]float32) ([][]float32, error){
		"embedder fails": func(n int, vs [][]float32) ([][]float32, error) {
			if n > 1 {
				return nil, errors.New("injected")
			}
			return vs, nil
		},
		"wrong length": func(n int, vs [][]float32) ([][]float32, error) {
			if n > 1 {
				return [][]float32{{1, 0}}, nil
			}
			return vs, nil
		},
	} {
		e, s := built(t)
		s.put("notes/d.md", "Delta", "alpha delta\n")
		opt := e.opts()
		opt.Registry = &failing{Registry: e.kit.Registry, fail: fail}
		db := s.manifestDB()
		if res, err := e.search(ctx, opt, "alpha", s); err != nil || !res.Ready || res.Missing != 1 || s.manifestDB() != db {
			t.Errorf("%s: %+v %v", name, res, err)
		}
	}

	// A publication whose rename fails: the old index stays, the item is still scored now.
	e, s := built(t)
	s.put("notes/d.md", "Delta", "alpha delta\n")
	db := s.manifestDB()
	opt := e.opts()
	opt.Files = replaceFiles{sys.Native()}
	src := Start(ctx, opt, "alpha", []SectionDir{{Section: s.kind, Dir: replaceDir{s.dir}, Root: s.root}})
	in, _ := s.scan(ctx)
	src.Attach([]SectionInput{in})
	if err := src.CatchUp(ctx); err != nil {
		t.Fatal(err)
	}
	res, _ := src.Score(ctx, "alpha", in.Items)
	src.Close()
	if res.Missing != 0 || !hit(res, "notes/d.md") || s.manifestDB() != db {
		t.Fatalf("a failed publication: %+v", res)
	}

	// A cancelled context: its error, and nothing published.
	s.put("notes/e.md", "Echo", "alpha echo\n")
	cctx, cancel := context.WithCancel(ctx)
	opt = e.opts()
	f := &failing{Registry: e.kit.Registry}
	f.fail = func(n int, vs [][]float32) ([][]float32, error) {
		if n > 1 {
			cancel()
			return nil, context.Canceled
		}
		return vs, nil
	}
	opt.Registry = f
	src, _ = e.source(cctx, opt, "alpha", s)
	err := src.CatchUp(cctx)
	src.Close()
	if !errors.Is(err, context.Canceled) || s.manifestDB() != db {
		t.Fatalf("a cancelled context: %v", err)
	}
}
