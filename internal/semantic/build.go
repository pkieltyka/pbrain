package semantic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

const (
	batchSize     = 8
	publishEvery  = 20 * time.Second
	progressEvery = 10 * time.Second
	indexLockName = "lock"
)

// waitNotice is how long a build waits for the index lock before it reports Waiting; a variable
// for the tests.
var waitNotice = 2 * time.Second

func configErr(msg string) error  { return &errs.Error{Code: "invalid_config", Message: msg, Exit: 1} }
func buildIOErr(msg string) error { return &errs.Error{Code: "io", Message: msg, Exit: 1} }
func fetchFailedErr() error {
	return &errs.Error{Code: "fetch_failed", Message: "the model downloaded is not the pinned one", Exit: 1}
}

// ctxOr returns the context's error when it has ended, else err.
func ctxOr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// Build brings one section's index up to date. It calls scan after it holds the index lock and
// works from that scan alone. An item whose body cannot be read as scanned is left for the next run.
// Errors, all *errs.Error: no_model; invalid_config (unknown model, a model_path folder that is
// incomplete or fails Verify, an exposed cache, or neither a cache folder nor a model_path);
// fetch_failed from the download; io for the file system, the lock and a failed embedding.
// A cancelled context ends it with the context's error.
func Build(ctx context.Context, opt Options, sec SectionDir, scan ScanFunc, dl embed.Downloader,
	notify func(embed.Model, int64), progress func(Progress)) (BuildResult, error) {
	res := BuildResult{Section: sec.Section}
	if opt.ModelName == "" {
		return res, &errs.Error{Code: "no_model", Message: "semantic search is off: pbrain.toml sets model = \"none\"", Exit: 1}
	}
	m, ok := opt.Registry.Lookup(opt.ModelName)
	if !ok {
		return res, configErr(fmt.Sprintf("the model %q is not supported; supported models: %s",
			opt.ModelName, strings.Join(opt.Registry.Names(), ", ")))
	}
	if opt.CacheErr != nil {
		return res, configErr("the cache folder is exposed to Git or lies in the brain; set PBRAIN_CACHE to another folder")
	}
	if opt.Cache == nil && opt.ModelDir == nil {
		return res, configErr("there is no cache folder and no model_path; set PBRAIN_CACHE or model_path")
	}
	if notify == nil {
		notify = func(embed.Model, int64) {}
	}
	if progress == nil {
		progress = func(Progress) {}
	}
	now := opt.Now
	if now == nil {
		now = time.Now
	}

	idx, err := indexDir(sec.Dir, false)
	if errors.Is(err, fs.ErrNotExist) {
		in, serr := scan(ctx)
		if serr != nil {
			return res, serr
		}
		if len(in.Items) == 0 {
			return res, nil
		}
		idx, err = indexDir(sec.Dir, true)
	}
	if err != nil {
		return res, buildIOErr("the index folder cannot be opened")
	}
	defer idx.Close()

	lock, err := lockIndex(ctx, opt.Files, idx, sec.Section, progress)
	if err != nil {
		return res, err
	}
	defer lock.Close()
	in, err := scan(ctx)
	if err != nil {
		return res, err
	}
	b := &builder{ctx: ctx, opt: opt, m: m, sec: sec, in: in, idx: idx, now: now, progress: progress, res: res}
	if len(in.Items) > 0 {
		if b.art, err = ensureModel(ctx, opt, m, dl, notify); err != nil {
			return res, err
		}
	}
	defer b.close()
	return b.run()
}

// indexDir opens <section>/.pbrain/index/, creating both folders when create is set. A section
// folder that does not exist (Dir nil) has none.
func indexDir(dir sys.Dir, create bool) (sys.Dir, error) {
	if dir == nil {
		return nil, fs.ErrNotExist
	}
	pb, err := dir.Sub(".pbrain", create)
	if err != nil {
		return nil, err
	}
	defer pb.Close()
	return pb.Sub("index", create)
}

// lockIndex takes the index lock, reporting Waiting once after two seconds; only the context
// ends the wait.
func lockIndex(ctx context.Context, files sys.Files, idx sys.Dir, section item.Section, progress func(Progress)) (io.Closer, error) {
	first, cancel := context.WithTimeout(ctx, waitNotice)
	lock, err := files.Lock(first, idx, indexLockName)
	cancel()
	if err == nil {
		return lock, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		return nil, buildIOErr("the index lock cannot be taken")
	}
	progress(Progress{Section: section, Waiting: true})
	lock, err = files.Lock(ctx, idx, indexLockName)
	if err != nil {
		return nil, ctxOr(ctx, buildIOErr("the index lock cannot be taken"))
	}
	return lock, nil
}

// builder is one run of Build from step 5 on, under the index lock.
type builder struct {
	ctx      context.Context
	opt      Options
	m        embed.Model
	sec      SectionDir
	in       SectionInput
	idx      sys.Dir
	now      func() time.Time
	progress func(Progress)
	res      BuildResult

	art *embed.Artifact
	emb embed.Embedder
	x   *indexDB
	si  *sectionIndex // x, shared with a catch-up's code

	changed  bool
	lastPub  time.Time
	lastProg time.Time
	done     int
	total    int
}

func (b *builder) close() {
	if b.emb != nil {
		b.emb.Close()
	}
	if b.x != nil {
		b.x.Close()
	}
	b.art = nil
}

// work is one item to do and the vectors of its passages as they are found.
type work struct {
	path    string
	content [32]byte // ContentDigest, written beside the check
	check   Check
	ps      []Passage
	vecs    [][]float32
	reused  int
}

type queued struct {
	w *work
	i int
}

func (b *builder) run() (BuildResult, error) {
	ctx, files := b.ctx, b.opt.Files
	fp := IndexFingerprint(b.m)
	x, state := openIndex(ctx, files, b.idx, fp, b.m.Dim, true)
	if state != IndexOK {
		var err error
		if x, err = newIndex(ctx, fp, b.m.Name, b.m.Dim); err != nil {
			return b.res, ctxOr(ctx, buildIOErr("the index cannot be built"))
		}
		b.changed = true // an index that is missing, unreadable or stale is replaced even when empty
	}
	b.x = x
	b.removeLeftovers()

	// Step 6: what to do and what is dead.
	seen := map[string]bool{}
	var todo []*work
	bodies := map[*work]struct {
		title string
		body  []byte
	}{}
	for _, it := range b.in.Items {
		rel, ok := relPath(b.sec.Root, it.Path)
		if !ok {
			continue
		}
		seen[rel] = true
		body, err := b.in.Body(it)
		if err != nil {
			continue // changed since the scan: left for the next run
		}
		check := ContentCheck(it.Header.Title, body)
		if st, ok := x.items[rel]; ok && st.check == check {
			continue
		}
		w := newWork(rel, it.Header.Title, body, check)
		todo = append(todo, w)
		bodies[w] = struct {
			title string
			body  []byte
		}{it.Header.Title, body}
	}
	slices.SortFunc(todo, func(a, c *work) int { return strings.Compare(a.path, c.path) })
	var dead []string
	for p := range x.items {
		if !seen[p] {
			dead = append(dead, p)
		}
	}
	slices.Sort(dead)
	b.total = len(todo)
	b.lastPub, b.lastProg = b.now(), b.now()
	b.si = &sectionIndex{opt: b.opt, m: b.m, sec: b.sec.Section, x: x, cache: b.sec.Section == item.Shared && b.opt.Cache != nil}

	// Steps 7 to 10.
	var pending []*work
	var queue []queued
	for _, w := range todo {
		src := bodies[w]
		delete(bodies, w)
		missing, err := b.si.cut(ctx, w, src.title, src.body)
		if err != nil {
			return b.res, err
		}
		for _, i := range missing {
			queue = append(queue, queued{w, i})
		}
		pending = append(pending, w)
		if len(queue) >= batchSize {
			if err := b.flush(queue, pending); err != nil {
				return b.res, err
			}
			queue, pending = nil, nil
		}
	}
	if err := b.flush(queue, pending); err != nil {
		return b.res, err
	}
	b.art = nil // no batch left: the runtime is never opened when nothing was queued
	if len(dead) > 0 {
		if err := x.removeItems(ctx, dead); err != nil {
			return b.res, ctxOr(ctx, buildIOErr("the index cannot be updated"))
		}
		b.changed = true
	}
	if b.changed {
		if err := b.publish(); err != nil {
			return b.res, err
		}
	}
	b.res.Items = len(x.items)
	b.res.ItemsRemoved = len(dead)
	return b.res, nil
}

// sectionIndex is what a build and a catch-up share to fill one section's index: its in-memory
// database, the model that fills it and the per-user cache, which only the shared section uses.
type sectionIndex struct {
	opt   Options
	m     embed.Model
	sec   item.Section
	x     *indexDB
	cache bool // read and write opt.Cache
}

// errCacheGit is the error of a cache folder found to hold a Git repository below its root.
var errCacheGit = configErr("the cache folder holds a Git repository; move it or set PBRAIN_CACHE")

// newWork is one item to cut, with both its digest and its check.
func newWork(path, title string, body []byte, check Check) *work {
	return &work{path: path, content: ContentDigest(title, body), check: check}
}

// cut cuts w's item and takes every vector found by digest. It returns the passages still to embed.
func (s *sectionIndex) cut(ctx context.Context, w *work, title string, body []byte) ([]int, error) {
	w.ps = Passages(title, body)
	w.vecs = make([][]float32, len(w.ps))
	var missing []int
	for i, p := range w.ps {
		v, ok, err := s.reuse(ctx, p.Digest)
		if err != nil {
			return nil, err
		}
		if ok {
			w.vecs[i] = v
			w.reused++
			continue
		}
		missing = append(missing, i)
	}
	return missing, nil
}

// reuse looks for a vector by passage digest: in the index, then, for the shared section, in the cache.
func (s *sectionIndex) reuse(ctx context.Context, digest [32]byte) ([]float32, bool, error) {
	v, ok, err := s.x.vectorByDigest(ctx, digest)
	if err != nil {
		return nil, false, ctxOr(ctx, buildIOErr("the index cannot be read"))
	}
	if ok {
		return v, true, nil
	}
	if !s.cache {
		return nil, false, nil
	}
	v, ok, err = s.opt.Cache.readVector(s.m.Fingerprint, digest, s.m.Dim)
	if err != nil {
		return nil, false, errCacheGit
	}
	return v, ok, nil
}

// embed embeds texts as documents and checks every vector it gets back.
func (s *sectionIndex) embed(ctx context.Context, emb embed.Embedder, texts []string) ([][]float32, error) {
	vs, err := emb.Embed(ctx, embed.Document, texts)
	if err != nil {
		return nil, ctxOr(ctx, buildIOErr("the model failed to embed a passage"))
	}
	if len(vs) != len(texts) {
		return nil, buildIOErr("the model returned the wrong number of vectors")
	}
	for _, v := range vs {
		if !validVector(v, s.m.Dim) {
			return nil, buildIOErr("the model returned a vector that is not a unit vector of its dimension")
		}
	}
	return vs, nil
}

// keep gives passage i of w its new vector and, for the shared section, writes it to the cache.
func (s *sectionIndex) keep(w *work, i int, v []float32) error {
	w.vecs[i] = v
	if !s.cache {
		return nil
	}
	if err := s.opt.Cache.writeVector(s.m.Fingerprint, w.ps[i].Digest, v); err != nil {
		if errors.Is(err, ErrCacheExposed) {
			return errCacheGit
		}
		return buildIOErr("the vector cache cannot be written")
	}
	return nil
}

// store replaces w's item in the index with all its passages, in one transaction.
func (s *sectionIndex) store(ctx context.Context, w *work) error {
	stored := make([]storedPassage, len(w.ps))
	for i, p := range w.ps {
		stored[i] = storedPassage{Ord: i, Start: p.Start, Stop: p.End, Digest: p.Digest, Vector: w.vecs[i]}
	}
	if err := s.x.putItem(ctx, w.path, w.content, w.check, stored); err != nil {
		return ctxOr(ctx, buildIOErr("the index cannot be updated"))
	}
	return nil
}

// flush embeds the queued passages eight at a time, then stores every pending item, in order.
func (b *builder) flush(queue []queued, pending []*work) error {
	ctx := b.ctx
	for start := 0; start < len(queue); start += batchSize {
		batch := queue[start:min(start+batchSize, len(queue))]
		if b.emb == nil {
			emb, err := b.opt.Registry.Open(ctx, b.art)
			b.art = nil
			if err != nil {
				return ctxOr(ctx, buildIOErr("the model cannot be opened"))
			}
			b.emb = emb
		}
		texts := make([]string, len(batch))
		for i, q := range batch {
			texts[i] = q.w.ps[q.i].Text
		}
		vs, err := b.si.embed(ctx, b.emb, texts)
		if err != nil {
			return err
		}
		for i, q := range batch {
			if err := b.si.keep(q.w, q.i, vs[i]); err != nil {
				return err
			}
		}
	}
	for _, w := range pending {
		if err := b.si.store(ctx, w); err != nil {
			return err
		}
		b.changed = true
		b.res.PassagesReused += w.reused
		b.res.PassagesEmbedded += len(w.ps) - w.reused
		b.done++
		if b.now().Sub(b.lastPub) >= publishEvery {
			if err := b.publish(); err != nil {
				return err
			}
		}
		if b.now().Sub(b.lastProg) >= progressEvery {
			b.lastProg = b.now()
			b.progress(Progress{Section: b.sec.Section, Done: b.done, Total: b.total})
		}
	}
	return nil
}

func (b *builder) publish() error {
	if _, err := b.x.publish(b.ctx, b.opt.Files, b.idx, b.opt.Rand); err != nil {
		return ctxOr(b.ctx, err)
	}
	b.lastPub = b.now()
	return nil
}

// removeLeftovers removes temporary files and databases of the generated name form that the
// manifest does not name. Nothing else is removed, whatever the manifest says.
func (b *builder) removeLeftovers() {
	files := b.opt.Files
	keep := ""
	if m, state := readManifest(files, b.idx); state == IndexOK {
		keep = m.DB
	}
	entries, err := files.Entries(b.idx)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Name != keep && (isTempName(e.Name) || dbName.MatchString(e.Name)) {
			b.idx.Remove(e.Name) //nolint:errcheck // a leftover that stays is removed by a later build
		}
	}
}

// isTempName matches sys's temporary files: ".pbrain-" and 24 hexadecimal digits, then ".tmp".
func isTempName(name string) bool {
	h, ok := strings.CutPrefix(name, ".pbrain-")
	if !ok {
		return false
	}
	h, ok = strings.CutSuffix(h, ".tmp")
	return ok && len(h) == 24 && strings.Trim(h, "0123456789abcdef") == ""
}

// relPath is an item's path relative to the section's folder, with forward slashes.
func relPath(root, path string) (string, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// ensureModel is Build's step 4: it makes sure the model is on disk, the pinned one, and returns it
// verified. The cache's model lock is held throughout, after the index lock.
func ensureModel(ctx context.Context, opt Options, m embed.Model, dl embed.Downloader, notify func(embed.Model, int64)) (*embed.Artifact, error) {
	files := opt.Files
	if opt.ModelDir != nil {
		dir, err := embed.Find(files, m, opt.ModelDir, nil)
		if err == nil {
			var a *embed.Artifact
			if a, err = embed.Verify(files, dir, m); err == nil {
				return a, nil
			}
		}
		if modelErr(err) {
			return nil, configErr("the model at model_path is incomplete or is not " + m.Name)
		}
		return nil, buildIOErr("the model at model_path cannot be read")
	}
	models, err := opt.Cache.models(true)
	if err != nil {
		return nil, cacheErr(err)
	}
	lock, err := files.Lock(ctx, models, embed.FolderName(m)+".lock")
	if err != nil {
		return nil, ctxOr(ctx, buildIOErr("the model lock cannot be taken"))
	}
	defer lock.Close()
	for range 2 {
		if err := obtain(ctx, opt, m, models, dl, notify); err != nil {
			return nil, err
		}
		dir, err := opt.Cache.dir("models/"+embed.FolderName(m), false)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, cacheErr(err)
		}
		if err == nil {
			a, verr := embed.Verify(files, dir, m)
			if verr == nil {
				dropSource(opt.Cache, m)
				return a, nil
			}
			if !modelErr(verr) {
				return nil, buildIOErr("the model cannot be read")
			}
		}
		if err := removeBad(files, opt.Cache, m.Files, "models/"+embed.FolderName(m)); err != nil {
			return nil, err
		}
	}
	return nil, fetchFailedErr()
}

func modelErr(err error) bool {
	return errors.Is(err, embed.ErrAbsent) || errors.Is(err, embed.ErrWrongModel)
}

func cacheErr(err error) error {
	if errors.Is(err, ErrCacheExposed) {
		return configErr("the cache folder holds a Git repository; move it or set PBRAIN_CACHE")
	}
	return buildIOErr("the cache folder cannot be used")
}

// obtain downloads what the cache lacks: the files of m, or for a derived model its source, which
// Pack then turns into the pack.
func obtain(ctx context.Context, opt Options, m embed.Model, models sys.Dir, dl embed.Downloader, notify func(embed.Model, int64)) error {
	files, c := opt.Files, opt.Cache
	folder := "models/" + embed.FolderName(m)
	if dir, err := embed.Find(files, m, nil, models); err == nil {
		dir.Close()
		return nil
	} else if !modelErr(err) {
		return buildIOErr("the model folder cannot be read")
	}
	// Files of the wrong size are removed first: Download only fetches names that are missing.
	if err := removeBad(files, c, m.Files, folder); err != nil {
		return err
	}
	list, into := m.Files, folder
	if len(m.Source) > 0 {
		list, into = m.Source, folder+".source"
		if err := removeBad(files, c, list, into); err != nil {
			return err
		}
	}
	// The folders Download and Pack use are opened, and tested, through the cache first.
	target, err := c.dir(into, true)
	if err != nil {
		return cacheErr(err)
	}
	var missing int64
	for _, f := range list {
		if _, err := files.Stat(target, f.Name); errors.Is(err, fs.ErrNotExist) {
			missing += f.Size
		}
	}
	if missing > 0 {
		notify(m, missing)
	}
	got, err := embed.Download(ctx, files, dl, m, models)
	if err != nil {
		return ctxOr(ctx, err)
	}
	defer got.Close()
	if len(m.Source) == 0 {
		return nil
	}
	pack, err := c.dir(folder, true)
	if err != nil {
		return cacheErr(err)
	}
	if err := embed.Pack(files, opt.Registry, m, got, pack); err != nil {
		if !modelErr(err) {
			return buildIOErr("the model's pack cannot be written")
		}
		// A source that is not the pinned one is removed and is a failed download; otherwise
		// the derivation itself did not give the pinned pack.
		bad, rerr := badFiles(files, got, m.Source)
		if rerr != nil {
			return buildIOErr("the model's source cannot be read")
		}
		if len(bad) > 0 {
			for _, n := range bad {
				got.Remove(n) //nolint:errcheck // a file that stays is checked again by the next run
			}
			return fetchFailedErr()
		}
		return buildIOErr("the model's pack is not the pinned one")
	}
	return nil
}

// badFiles names the files of list in dir that are present but not the pinned ones.
func badFiles(files sys.Files, dir sys.Dir, list []embed.File) ([]string, error) {
	var bad []string
	for _, f := range list {
		_, err := embed.Verify(files, dir, embed.Model{Files: []embed.File{f}})
		switch {
		case err == nil, errors.Is(err, embed.ErrAbsent):
		case errors.Is(err, embed.ErrWrongModel):
			bad = append(bad, f.Name)
		default:
			return nil, err
		}
	}
	return bad, nil
}

// removeBad removes the files of list in the cache folder at rel that are not the pinned ones.
func removeBad(files sys.Files, c *Cache, list []embed.File, rel string) error {
	dir, err := c.dir(rel, false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return cacheErr(err)
	}
	bad, err := badFiles(files, dir, list)
	if err != nil {
		return buildIOErr("the model folder cannot be read")
	}
	for _, n := range bad {
		if err := dir.Remove(n); err != nil {
			return buildIOErr("a damaged model file cannot be removed")
		}
	}
	return nil
}

// dropSource removes a derived model's source files once its pack has been verified. The folder
// itself stays, empty: sys removes only regular files.
func dropSource(c *Cache, m embed.Model) {
	if len(m.Source) == 0 {
		return
	}
	dir, err := c.dir("models/"+embed.FolderName(m)+".source", false)
	if err != nil {
		return
	}
	for _, f := range m.Source {
		dir.Remove(f.Name) //nolint:errcheck // a source file that stays is only disk space
	}
}
