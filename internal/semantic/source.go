package semantic

import (
	"cmp"
	"context"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/search"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// Source is the search.Semantic implementation for one command (semantic.md, "The source").
type Source struct {
	opt    Options
	text   string
	reason search.NotReady // the first cheap check that failed; zero when all passed
	m      embed.Model
	dirs   []SectionDir

	cancel   context.CancelFunc
	model    chan struct{} // closed when the model task ends
	indexes  chan struct{} // closed when every index is read
	query    []float32     // the query vector, when the model task succeeded
	modelErr error         // explicit accelerator error from runModel, read only after <-s.model
	emb      embed.Embedder
	opened   []openedIndex // one per section of dirs, in order
	secs     []SectionInput

	closeMu sync.Mutex
	closed  bool
}

type openedIndex struct {
	section item.Section
	dir     sys.Dir // index/, nil when the section has none
	x       *indexDB
	state   IndexState
}

// Start needs no scan and never fails. It runs the cheap checks and, when they pass, begins
// in the background to verify the model, open it and embed text as a query.
func Start(ctx context.Context, opt Options, text string, dirs []SectionDir) *Source {
	s := &Source{opt: opt, text: text, dirs: dirs, model: make(chan struct{}), indexes: make(chan struct{})}
	modelDir, reason := s.check()
	if reason != 0 {
		s.reason = reason
		close(s.model)
		close(s.indexes)
		return s
	}
	task, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	go s.runModel(task, modelDir)
	go s.runIndexes(task)
	return s
}

// check is the table of cheap checks, in order. It returns the folder that holds the model.
func (s *Source) check() (sys.Dir, search.NotReady) {
	opt := s.opt
	if opt.ModelName == "" {
		return nil, search.NoModel
	}
	if opt.Registry == nil {
		return nil, search.UnsupportedModel
	}
	m, ok := opt.Registry.Lookup(opt.ModelName)
	if !ok {
		return nil, search.UnsupportedModel
	}
	s.m = m
	if opt.CacheErr != nil {
		return nil, search.CacheExposed
	}
	modelDir := opt.ModelDir
	if modelDir == nil {
		if opt.Cache == nil {
			return nil, search.ModelAbsent
		}
		d, err := opt.Cache.dir("models/"+embed.FolderName(m), false)
		switch {
		case errors.Is(err, ErrCacheExposed):
			return nil, search.CacheExposed
		case err != nil:
			return nil, search.ModelAbsent
		}
		modelDir = d
	}
	if _, err := embed.Find(opt.Files, m, modelDir, nil); err != nil {
		return nil, search.ModelAbsent
	}
	fp := IndexFingerprint(m)
	for _, d := range s.dirs {
		idx, err := indexDir(d.Dir, false)
		if err != nil {
			continue
		}
		man, state := readManifest(opt.Files, idx)
		idx.Close()
		if state == IndexOK && man.Fingerprint == fp && man.Dim == m.Dim {
			return modelDir, 0
		}
	}
	return nil, search.NoIndex
}

// runModel verifies the model's files, opens them and embeds the query.
func (s *Source) runModel(ctx context.Context, dir sys.Dir) {
	defer close(s.model)
	a, err := embed.Verify(s.opt.Files, dir, s.m)
	if err != nil || ctx.Err() != nil {
		return
	}
	emb, err := s.opt.Registry.Open(ctx, a)
	if err != nil {
		if ctx.Err() == nil && errors.Is(err, embed.ErrAccelerator) {
			s.modelErr = wrapAccelIO("the model cannot be opened", err)
		}
		return
	}
	s.emb = emb
	vs, err := emb.Embed(ctx, embed.Query, []string{s.text})
	if err != nil {
		if ctx.Err() == nil && errors.Is(err, embed.ErrAccelerator) {
			s.modelErr = wrapAccelIO("the model failed to embed a query", err)
		}
		return
	}
	if len(vs) != 1 || !validVector(vs[0], s.m.Dim) {
		return
	}
	s.query = vs[0]
}

// runIndexes reads each section's index, without the full row check of a build.
func (s *Source) runIndexes(ctx context.Context) {
	defer close(s.indexes)
	fp := IndexFingerprint(s.m)
	for _, d := range s.dirs {
		o := openedIndex{section: d.Section, state: IndexNone}
		if idx, err := indexDir(d.Dir, false); err == nil {
			o.dir = idx
			o.x, o.state = openIndex(ctx, s.opt.Files, idx, fp, s.m.Dim, false)
		}
		s.opened = append(s.opened, o)
	}
}

// Attach gives the source the scanned items. Call it once, after the scan, before Score.
func (s *Source) Attach(secs []SectionInput) { s.secs = secs }

// Catch-up bounds (semantic.md, "Catch-up in a search").
const (
	catchUpItems    = 20              // most uncovered items a search takes on
	catchUpPassages = 20              // most passages it embeds
	catchUpBudget   = 2 * time.Second // no embedding call starts after this
)

// catchUp is one section's part of a catch-up: its index and the items it would replace.
type catchUp struct {
	o       *openedIndex
	si      *sectionIndex
	items   []catchUpItem
	works   []*work
	missing [][]int // per work, the passages still to embed
}

type catchUpItem struct {
	rel, title string
	body       []byte
	check      Check
}

// CatchUp embeds and saves the few items that the usable indexes do not cover, within the bounds
// above, so that Score finds them (semantic.md, "Catch-up in a search"). It takes each index lock
// by one attempt only, and a fault leaves the items uncovered: it returns only the context's
// error. Call it once, after Attach and before Score.
func (s *Source) CatchUp(ctx context.Context) error {
	if s.reason != 0 {
		return nil
	}
	if err := wait(ctx, s.indexes); err != nil {
		return err
	}
	// Step 2: the gap, shared section first, each in path order.
	secs := slices.Clone(s.secs)
	slices.SortStableFunc(secs, func(a, b SectionInput) int { return cmp.Compare(a.Section, b.Section) })
	var parts []*catchUp
	total := 0
	for _, in := range secs {
		o := s.usable(in.Section)
		if o == nil {
			continue
		}
		part := &catchUp{o: o}
		for _, it := range in.Items {
			if _, ok := covers(o.x, in, it); ok {
				continue
			}
			rel, ok := relPath(in.Root, it.Path)
			if !ok {
				continue
			}
			body, err := in.Body(it)
			if err != nil {
				continue
			}
			part.items = append(part.items, catchUpItem{rel, it.Header.Title, body, ContentCheck(it.Header.Title, body)})
		}
		if total += len(part.items); total > catchUpItems {
			return nil
		}
		if len(part.items) > 0 {
			slices.SortFunc(part.items, func(a, b catchUpItem) int { return strings.Compare(a.rel, b.rel) })
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	// Step 3: the plan, with no lock and no write.
	cache := s.opt.Cache != nil && s.opt.Cache.exists()
	embeds := 0
	for _, part := range parts {
		part.si = &sectionIndex{opt: s.opt, m: s.m, sec: part.o.section, x: part.o.x, cache: cache && part.o.section == item.Shared}
		for _, c := range part.items {
			w := newWork(c.rel, c.title, c.body, c.check)
			missing, err := part.si.cut(ctx, w, c.title, c.body)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				return nil
			}
			part.works = append(part.works, w)
			part.missing = append(part.missing, missing)
			embeds += len(missing)
		}
	}
	if embeds > catchUpPassages {
		return nil
	}
	// Step 4: the model.
	if err := wait(ctx, s.model); err != nil {
		return err
	}
	if s.modelErr != nil {
		return s.modelErr
	}
	if s.query == nil {
		return nil
	}
	// Step 5: each section under its lock.
	now := s.opt.Now
	if now == nil {
		now = time.Now
	}
	var began time.Time
	for _, part := range parts {
		lock, err := s.opt.Files.TryLock(part.o.dir, indexLockName)
		if err != nil {
			continue
		}
		if began.IsZero() {
			began = now()
		}
		exposed, err := s.catchUpSection(ctx, part, func() bool { return now().Sub(began) >= catchUpBudget })
		lock.Close()
		if err != nil {
			return err
		}
		if exposed {
			return nil
		}
	}
	return nil
}

// catchUpSection is step 5 for one section, under its lock: it replaces each item whose passages
// all get a vector before late reports the time is up, then publishes. exposed reports a cache
// found to hold a Git repository, which ends the catch-up; err is only the context's error.
func (s *Source) catchUpSection(ctx context.Context, part *catchUp, late func() bool) (exposed bool, err error) {
	o := part.o
	if m, state := readManifest(s.opt.Files, o.dir); state != IndexOK || m != o.x.from {
		return false, nil // a build has published since this search read the index
	}
	replaced := 0
	defer func() {
		if replaced > 0 && err == nil {
			if m, perr := o.x.publish(ctx, s.opt.Files, o.dir, s.opt.Rand); perr == nil {
				o.x.from = m
			}
		}
	}()
items:
	for k, w := range part.works {
		for _, i := range part.missing[k] {
			if late() {
				break items
			}
			vs, eerr := part.si.embed(ctx, s.emb, []string{w.ps[i].Text})
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			if eerr != nil {
				if errors.Is(eerr, embed.ErrAccelerator) {
					return false, eerr
				}
				break items
			}
			if kerr := part.si.keep(w, i, vs[0]); kerr != nil {
				if kerr == errCacheGit {
					return true, nil
				}
				break items
			}
		}
		if serr := part.si.store(ctx, w); serr != nil {
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			break
		}
		replaced++
	}
	return false, nil
}

// usable is the opened index of a section that can be read, or nil.
func (s *Source) usable(sec item.Section) *openedIndex {
	for i := range s.opened {
		if o := &s.opened[i]; o.section == sec && o.state == IndexOK {
			return o
		}
	}
	return nil
}

// Score joins the background task and compares. text must be the text given to Start.
// It returns only the context's error.
func (s *Source) Score(ctx context.Context, text string, eligible []*item.Item) (search.Scores, error) {
	if text != s.text {
		panic("semantic: Score with a text other than Start's")
	}
	if s.reason != 0 {
		return search.Scores{Reason: s.reason}, nil
	}
	if err := wait(ctx, s.indexes); err != nil {
		return search.Scores{}, err
	}
	type usable struct {
		in      SectionInput
		x       *indexDB
		covered map[*item.Item]storedItem
	}
	missing := 0
	var sections []usable
	for _, in := range s.secs {
		var x *indexDB
		for _, o := range s.opened {
			if o.section == in.Section && o.state == IndexOK {
				x = o.x
			}
		}
		if x == nil {
			missing += len(in.Items)
			continue
		}
		u := usable{in: in, x: x, covered: map[*item.Item]storedItem{}}
		for _, it := range in.Items {
			if st, ok := covers(x, in, it); ok {
				u.covered[it] = st
			} else {
				missing++
			}
		}
		sections = append(sections, u)
	}
	if len(sections) == 0 {
		return search.Scores{Reason: search.NoIndex}, nil
	}
	if err := wait(ctx, s.model); err != nil {
		return search.Scores{}, err
	}
	if s.modelErr != nil {
		return search.Scores{}, s.modelErr
	}
	if s.query == nil {
		return search.Scores{Reason: search.ModelFailed}, nil
	}
	want := make(map[*item.Item]bool, len(eligible))
	for _, it := range eligible {
		want[it] = true
	}
	best := map[*item.Item]hitRow{}
	left := 0
	for _, u := range sections {
		byID := map[int64]*item.Item{}
		var ids []int64
		for it, st := range u.covered {
			if want[it] {
				byID[st.id] = it
				ids = append(ids, st.id)
			}
		}
		rows, err := u.x.scoreItems(ctx, ids, s.query, s.m.Cutoff)
		if ctx.Err() != nil {
			return search.Scores{}, ctx.Err()
		}
		if err != nil {
			// The section is not usable for this search: its items count as missing, once.
			missing += len(u.covered)
			continue
		}
		left++
		for _, row := range rows {
			it := byID[row.ItemID]
			if it == nil || !spanInside(u.in, it, row.Start, row.Stop) {
				continue
			}
			if cur, ok := best[it]; !ok || row.Distance < cur.Distance || row.Distance == cur.Distance && row.Ord < cur.Ord {
				best[it] = row
			}
		}
	}
	if left == 0 {
		return search.Scores{Reason: search.NoIndex}, nil
	}
	out := search.Scores{Ready: true, Missing: missing, Cutoff: s.m.Cutoff}
	for _, it := range eligible {
		if row, ok := best[it]; ok {
			out.Hits = append(out.Hits, search.SemanticHit{Item: it, Score: float32(1 - row.Distance), Start: row.Start, End: row.Stop})
		}
	}
	return out, nil
}

func wait(ctx context.Context, done chan struct{}) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// covers reports whether the index holds the item's present text: the same path and the same
// ContentCheck of its title and body as scanned. It never computes ContentDigest.
func covers(x *indexDB, in SectionInput, it *item.Item) (storedItem, bool) {
	rel, ok := relPath(in.Root, it.Path)
	if !ok {
		return storedItem{}, false
	}
	st, ok := x.items[rel]
	if !ok {
		return storedItem{}, false
	}
	body, err := in.Body(it)
	if err != nil || ContentCheck(it.Header.Title, body) != st.check {
		return storedItem{}, false
	}
	return st, true
}

// spanInside reports whether [start, stop) lies inside the item's body as scanned.
func spanInside(in SectionInput, it *item.Item, start, stop int) bool {
	n := it.Layout.BodyChars
	if it.Layout.BodyLine == 0 {
		body, err := in.Body(it)
		if err != nil {
			return false
		}
		n = utf8.RuneCount(body)
	}
	return start >= 0 && stop >= start && stop <= n
}

// Close cancels the background task, waits for it, closes all opened indexes once, and closes an
// opened embedder. If the embedder's Close fails (for example on a transient accelerator drain
// error), Close returns that error and retains the embedder handle so a subsequent Close retries it;
// once the embedder closes cleanly, further Close calls are no-ops. It is safe to call when Attach
// or Score were never reached.
func (s *Source) Close() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if !s.closed {
		s.closed = true
		if s.cancel != nil {
			s.cancel()
		}
		<-s.model
		<-s.indexes
		for i := range s.opened {
			if s.opened[i].x != nil {
				s.opened[i].x.Close()
				s.opened[i].x = nil
			}
			if s.opened[i].dir != nil {
				s.opened[i].dir.Close()
				s.opened[i].dir = nil
			}
		}
	}
	if s.emb != nil {
		if err := s.emb.Close(); err != nil {
			return wrapAccelIO("the model cannot be closed", err)
		}
		s.emb = nil
	}
	return nil
}

// Status loads no model and writes nothing. It returns only file-system errors (io).
func Status(ctx context.Context, opt Options, secs []SectionInput) (StatusResult, error) {
	res := StatusResult{Model: opt.ModelName}
	var m embed.Model
	if opt.ModelName != "" && opt.Registry != nil {
		m, res.Supported = opt.Registry.Lookup(opt.ModelName)
	}
	if res.Supported && opt.CacheErr == nil {
		res.ModelPresent = modelPresent(opt, m)
	}
	for _, in := range secs {
		st := SectionStatus{Section: in.Section, Items: len(in.Items), State: IndexNone}
		idx, err := indexDir(in.Dir, false)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return res, buildIOErr("the index folder cannot be read")
		default:
			st.State, st.Indexed = sectionState(ctx, opt, m, res.Supported, idx, in)
			idx.Close()
		}
		res.Sections = append(res.Sections, st)
	}
	return res, nil
}

func sectionState(ctx context.Context, opt Options, m embed.Model, supported bool, idx sys.Dir, in SectionInput) (IndexState, int) {
	if !supported {
		if _, state := readManifest(opt.Files, idx); state == IndexNone {
			return IndexNone, 0
		}
		return IndexStale, 0
	}
	x, state := openIndex(ctx, opt.Files, idx, IndexFingerprint(m), m.Dim, true)
	if state != IndexOK {
		return state, 0
	}
	defer x.Close()
	n := 0
	for _, it := range in.Items {
		if _, ok := covers(x, in, it); ok {
			n++
		}
	}
	return IndexOK, n
}

// modelPresent is Find and Verify together, at model_path or in the cache.
func modelPresent(opt Options, m embed.Model) bool {
	dir := opt.ModelDir
	if dir == nil {
		if opt.Cache == nil {
			return false
		}
		d, err := opt.Cache.dir("models/"+embed.FolderName(m), false)
		if err != nil {
			return false
		}
		dir = d
	}
	if _, err := embed.Find(opt.Files, m, dir, nil); err != nil {
		return false
	}
	_, err := embed.Verify(opt.Files, dir, m)
	return err == nil
}
