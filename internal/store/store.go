// Package store scans a brain's sections into items, reads files a caller names under the privacy
// guard, and makes every write: lock, publish, replace, remove and originals (architecture 7.1,
// 10; components/store.md). It works only on folder handles that never follow a link.
package store

import (
	"context"
	"errors"
	"hash/crc32"
	"io"
	"io/fs"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
	"weak"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// BodyPolicy says whether a scan keeps bodies in memory.
type BodyPolicy uint8

const (
	KeepAll  BodyPolicy = iota // search, check, writes
	KeepNone                   // list, prime, tags, get
)

// Skipped is a file no read may use, for the skipped-files notice and for check.
type Skipped struct {
	Section      item.Section
	Path         string // absolute, for caller's display-path policy
	Code         string // bad_front_matter or not_an_item
	Reason       string // fixed category, never file content
	Line, Column int    // 1-based when available, otherwise zero
}

// WritePlan is what a write may read and what it may name (architecture 10.1).
type WritePlan struct {
	Dest     item.Section
	Lookup   brain.View
	Disclose brain.View
}

// Commit lists the cleanup that failed after a mutation's commit point.
type Commit struct{ Cleanup []Cleanup }

// Cleanup is one failed step after a commit point; the caller reports it as after_write.
type Cleanup uint8

const (
	TempLeft         Cleanup = iota // a temporary file could not be removed
	FolderSyncFailed                // the folder could not be synced
)

var (
	// ErrChanged: the file changed since the scan read it.
	ErrChanged = errors.New("store: the file changed since it was scanned")
	// ErrNameExists: the final name is taken; the caller may draw a new id.
	ErrNameExists = errors.New("store: the name is taken")
)

// MaxItemBytes is the whole-file limit of an item (spec 2.4); MaxInputBytes that of a caller's file.
const (
	MaxItemBytes  = 4 << 20
	MaxInputBytes = 5 << 20
)

var castagnoli = crc32.MakeTable(crc32.Castagnoli)

// scanned holds, for every item a scan made, the route it was read by: each folder's identity and
// the file's. A write reaches an item only along that route, so a folder or file swapped since the
// scan, even for one holding the same bytes, is a conflict. It is keyed by a weak pointer and each
// entry is deleted when its item is collected, so it holds nothing beyond the items still in use.
var scanned sync.Map // weak.Pointer[item.Item] -> route

func remember(it *item.Item, r route) {
	key := weak.Make(it)
	scanned.Store(key, r)
	runtime.AddCleanup(it, func(k weak.Pointer[item.Item]) { scanned.Delete(k) }, key)
}

func recall(it *item.Item) (route, bool) {
	v, ok := scanned.Load(weak.Make(it))
	if !ok {
		return route{}, false
	}
	return v.(route), true
}

func ioError(msg string) error    { return &errs.Error{Code: "io", Message: msg, Exit: 1} }
func conflict(msg string) error   { return &errs.Error{Code: "conflict", Message: msg, Exit: 1} }
func invalidErr(msg string) error { return &errs.Error{Code: "invalid", Message: msg, Exit: 1} }

// route is how a scanned file is reached again from its section's root: the folder names, the
// identity each had when listed, and the file's name and identity.
type route struct {
	section item.Section
	dirs    []string
	dirIDs  []sys.Identity
	name    string
	id      sys.Identity
}

// Catalog is one scan of the view. It is immutable after Scan; its items and bodies are read-only.
type Catalog struct {
	b       *brain.Brain
	keep    BodyPolicy
	items   []*item.Item
	byKey   map[string][]*item.Item
	skipped []Skipped
	bodies  map[*item.Item][]byte
	routes  map[*item.Item]route
}

// Items is every item of the view, sorted by section, then path.
func (c *Catalog) Items() []*item.Item { return c.items }

// Lookup returns the items whose folded id equals id folded; more than one is ambiguous.
func (c *Catalog) Lookup(id string) []*item.Item { return c.byKey[item.Fold(id)] }

// Skipped is the files no read may use.
func (c *Catalog) Skipped() []Skipped { return c.skipped }

// Body is the item's stored body, from the bytes the scan read. With KeepNone the file is read
// again and must be the same version, else ErrChanged.
func (c *Catalog) Body(it *item.Item) ([]byte, error) {
	if c.keep == KeepAll {
		if b, ok := c.bodies[it]; ok {
			return b, nil
		}
		return nil, ioError("the item is not in this scan")
	}
	r, ok := c.routes[it]
	if !ok {
		return nil, ioError("the item is not in this scan")
	}
	root, err := c.b.OpenSection(r.section)
	if err != nil {
		return nil, ErrChanged
	}
	data, version, err := readRoute(c.b.Files(), root, r)
	if err != nil {
		return nil, err
	}
	if version != it.Version || int64(len(data)) < it.BodyAt {
		return nil, ErrChanged
	}
	return data[it.BodyAt:], nil
}

// openChain reaches the folder of r from root, by name, comparing each folder with the identity
// it had when listed. A missing, linked or swapped folder is ErrChanged. done releases the folder
// when openChain opened it; the root itself stays the brain's.
func openChain(files sys.Files, root sys.Dir, r route) (dir sys.Dir, done func(), err error) {
	cur, owned := root, false
	for i, name := range r.dirs {
		next, err := cur.Sub(name, false)
		if owned {
			cur.Close()
		}
		if err != nil {
			return nil, nil, ErrChanged
		}
		info, err := files.DirInfo(next)
		if err != nil || (i < len(r.dirIDs) && info.Identity != r.dirIDs[i]) {
			next.Close()
			return nil, nil, ErrChanged
		}
		cur, owned = next, true
	}
	if !owned {
		return root, func() {}, nil
	}
	return cur, func() { cur.Close() }, nil
}

// readRoute reads the file of r whole, at most MaxItemBytes, checking it is the file that was
// listed and that it did not change while read.
func readRoute(files sys.Files, root sys.Dir, r route) ([]byte, item.Version, error) {
	dir, done, err := openChain(files, root, r)
	if err != nil {
		return nil, item.Version{}, err
	}
	defer done()
	data, info, err := readBounded(files, dir, r.name, MaxItemBytes)
	if err != nil {
		if errors.Is(err, errTooLarge) {
			return nil, item.Version{}, ErrChanged
		}
		return nil, item.Version{}, err
	}
	if info.Identity != r.id {
		return nil, item.Version{}, ErrChanged
	}
	return data, versionOf(info, data), nil
}

func versionOf(info sys.FileInfo, data []byte) item.Version {
	return item.Version{Size: info.Size, ModTime: info.ModTime, Sum: crc32.Checksum(data, castagnoli)}
}

var errTooLarge = errors.New("store: over the limit")

// readBounded opens name in dir without following a link and reads it whole, refusing more than
// limit bytes. A file that is gone, replaced or changed while read is ErrChanged.
func readBounded(files sys.Files, dir sys.Dir, name string, limit int64) ([]byte, sys.FileInfo, error) {
	f, err := files.OpenRegular(dir, name)
	if err != nil {
		switch {
		case errors.Is(err, fs.ErrNotExist), errors.Is(err, sys.ErrUnsafe):
			return nil, sys.FileInfo{}, ErrChanged
		}
		return nil, sys.FileInfo{}, ioError("a file of the brain cannot be read")
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return nil, sys.FileInfo{}, ioError("a file of the brain cannot be read")
	}
	if before.Size > limit {
		return nil, before, errTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, sys.FileInfo{}, ioError("a file of the brain cannot be read")
	}
	if int64(len(data)) > limit {
		return nil, before, errTooLarge
	}
	after, err := f.Stat()
	if err != nil || after.Identity != before.Identity || after.Size != before.Size ||
		!after.ModTime.Equal(before.ModTime) || int64(len(data)) != after.Size {
		return nil, sys.FileInfo{}, ErrChanged
	}
	return data, after, nil
}

// protection is the brain's protected paths, by path and, for the ones that exist, by identity.
type protection struct {
	paths map[string]bool
	ids   map[sys.Identity]bool
}

func protectionOf(b *brain.Brain) protection {
	p := protection{paths: map[string]bool{}, ids: map[sys.Identity]bool{}}
	for _, pp := range b.Protected() {
		p.paths[pp.Path] = true
		if r, err := b.Files().Resolve(pp.Path, false); err == nil && r.Exists {
			p.ids[r.Info.Identity] = true
		}
	}
	return p
}

func (p protection) has(path string, id sys.Identity) bool { return p.paths[path] || p.ids[id] }

// candidate is a listed .md file waiting to be read.
type candidate struct {
	r    route
	path string
	size int64
}

// pending is a listed folder waiting to be listed in its turn.
type pending struct {
	dirs []string
	ids  []sys.Identity
	path string
}

// Scan reads every item of the view (architecture 7.1). The shared section is walked level by
// level, pruning dot folders, protected paths, links and special files, and asking Git about each
// level's folders and files in one batch before any folder is listed. The personal section is
// walked the same way without Git. Files are read whole on GOMAXPROCS workers.
func Scan(ctx context.Context, b *brain.Brain, v brain.View, keep BodyPolicy) (*Catalog, error) {
	if err := b.RequireView(v); err != nil {
		return nil, err
	}
	c := &Catalog{b: b, keep: keep, byKey: map[string][]*item.Item{}, bodies: map[*item.Item][]byte{}, routes: map[*item.Item]route{}}
	prot := protectionOf(b)
	var cands []candidate
	shared, err := b.OpenSection(item.Shared)
	if err != nil {
		return nil, ioError("the brain cannot be opened")
	}
	got, err := walk(ctx, b, shared, item.Shared, b.Shared.Dir, prot, true)
	if err != nil {
		return nil, err
	}
	cands = append(cands, got...)
	if v == brain.PersonalView {
		personal, err := b.OpenSection(item.Personal)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return nil, err
		default:
			got, err := walk(ctx, b, personal, item.Personal, b.Personal.Dir, prot, false)
			if err != nil {
				return nil, err
			}
			cands = append(cands, got...)
		}
	}
	if err := c.read(ctx, cands); err != nil {
		return nil, err
	}
	return c, nil
}

func walk(ctx context.Context, b *brain.Brain, root sys.Dir, section item.Section, rootPath string, prot protection, git bool) ([]candidate, error) {
	files := b.Files()
	var out []candidate
	level := []pending{{path: rootPath}}
	for len(level) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		type found struct {
			isDir bool
			p     pending
			c     candidate
		}
		var batch []found
		for _, pd := range level {
			dir, done, err := openChain(files, root, route{dirs: pd.dirs, dirIDs: pd.ids})
			if err != nil {
				continue // vanished or swapped since it was listed
			}
			entries, err := files.Entries(dir)
			done()
			if err != nil {
				return nil, ioError("a folder of the brain cannot be listed")
			}
			for _, e := range entries {
				if strings.HasPrefix(e.Name, ".") {
					continue
				}
				p := filepath.Join(pd.path, e.Name)
				if prot.has(p, e.Info.Identity) {
					continue
				}
				switch {
				case e.Info.Mode.IsDir():
					batch = append(batch, found{isDir: true, p: pending{
						dirs: append(slices.Clone(pd.dirs), e.Name), ids: append(slices.Clone(pd.ids), e.Info.Identity), path: p}})
				case e.Info.Mode.IsRegular() && strings.HasSuffix(e.Name, ".md"):
					batch = append(batch, found{c: candidate{
						r:    route{section: section, dirs: pd.dirs, dirIDs: pd.ids, name: e.Name, id: e.Info.Identity},
						path: p, size: e.Info.Size}})
				}
			}
		}
		ignored := make([]bool, len(batch))
		if git && len(batch) > 0 {
			ask := make([]brain.IgnorePath, len(batch))
			for i, f := range batch {
				if f.isDir {
					ask[i] = brain.IgnorePath{Path: f.p.path, Directory: true}
				} else {
					ask[i] = brain.IgnorePath{Path: f.c.path}
				}
			}
			var err error
			if ignored, err = b.SharedIgnored(ctx, ask); err != nil {
				return nil, err
			}
		}
		level = level[:0:0]
		for i, f := range batch {
			if ignored[i] {
				continue
			}
			if f.isDir {
				level = append(level, f.p)
			} else {
				out = append(out, f.c)
			}
		}
	}
	return out, nil
}

var fmReasons = map[item.FMReason]string{
	item.Unclosed: "unclosed", item.Syntax: "syntax", item.NotMapping: "not_mapping", item.DuplicateKey: "duplicate_key",
}

// read reads the candidates on GOMAXPROCS workers and sorts the outcome by section and path.
func (c *Catalog) read(ctx context.Context, cands []candidate) error {
	files := c.b.Files()
	type result struct {
		it      *item.Item
		body    []byte
		skipped *Skipped
		err     error
	}
	results := make([]result, len(cands))
	next := make(chan int)
	var wg sync.WaitGroup
	for range max(1, min(runtime.GOMAXPROCS(0), len(cands))) {
		wg.Go(func() {
			for i := range next {
				results[i].it, results[i].body, results[i].skipped, results[i].err = c.readOne(files, cands[i])
			}
		})
	}
	for i := range cands {
		if ctx.Err() != nil {
			break
		}
		next <- i
	}
	close(next)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	for i, r := range results {
		switch {
		case r.err != nil:
			return r.err
		case r.skipped != nil:
			c.skipped = append(c.skipped, *r.skipped)
		case r.it != nil:
			c.items = append(c.items, r.it)
			c.routes[r.it] = cands[i].r
			remember(r.it, cands[i].r)
			if c.keep == KeepAll {
				c.bodies[r.it] = r.body
			}
		}
	}
	slices.SortFunc(c.items, func(a, b *item.Item) int {
		if a.Section != b.Section {
			return int(a.Section) - int(b.Section)
		}
		return strings.Compare(a.Path, b.Path)
	})
	slices.SortFunc(c.skipped, func(a, b Skipped) int {
		if a.Section != b.Section {
			return int(a.Section) - int(b.Section)
		}
		return strings.Compare(a.Path, b.Path)
	})
	for _, it := range c.items {
		c.byKey[it.Key] = append(c.byKey[it.Key], it)
	}
	return nil
}

func (c *Catalog) readOne(files sys.Files, cand candidate) (*item.Item, []byte, *Skipped, error) {
	skip := func(code, reason string, line, col int) (*item.Item, []byte, *Skipped, error) {
		return nil, nil, &Skipped{Section: cand.r.section, Path: cand.path, Code: code, Reason: reason, Line: line, Column: col}, nil
	}
	root, err := c.b.OpenSection(cand.r.section)
	if err != nil {
		return nil, nil, nil, ioError("the brain cannot be opened")
	}
	dir, done, err := openChain(files, root, cand.r)
	if err != nil {
		return nil, nil, nil, nil // moved since it was listed: not in this scan
	}
	defer done()
	data, info, err := readBounded(files, dir, cand.r.name, MaxItemBytes)
	switch {
	case errors.Is(err, errTooLarge):
		return skip("bad_front_matter", "too_large", 0, 0)
	case errors.Is(err, ErrChanged):
		return nil, nil, nil, nil
	case err != nil:
		return nil, nil, nil, err
	}
	if info.Identity != cand.r.id {
		return nil, nil, nil, nil
	}
	if !utf8.Valid(data) {
		return skip("bad_front_matter", "invalid_utf8", 0, 0)
	}
	id := strings.TrimSuffix(cand.r.name, ".md")
	h, bodyAt, problems, err := item.Parse(id, data)
	if errors.Is(err, item.ErrNoFrontMatter) {
		return skip("not_an_item", "", 0, 0)
	}
	var fe *item.FrontMatterError
	if errors.As(err, &fe) {
		return skip("bad_front_matter", fmReasons[fe.Reason], fe.Line, fe.Column)
	}
	if err != nil {
		return skip("bad_front_matter", "syntax", 0, 0)
	}
	it := &item.Item{
		ID: id, Key: item.Fold(id), Section: cand.r.section, Path: cand.path, Header: h, Problems: problems,
		BodyAt: bodyAt, Layout: item.Measure(data, bodyAt), Version: versionOf(info, data),
	}
	return it, data[bodyAt:], nil, nil
}

// notFound is the one answer for a missing input and for one the privacy guard refuses.
func notFound() error {
	return &errs.Error{Code: "not_found", Message: "the input file does not exist", Exit: 1}
}

// OpenInput reads a file a caller names for import or --body-file, at most 5 MiB. For a shared
// write, a path that is or lies in a protected path, or in any folder named .pbrain (folded),
// gives the same not_found as a missing file, before any content is read (spec 5.2).
func OpenInput(b *brain.Brain, dest item.Section, path string) ([]byte, error) {
	files := b.Files()
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(b.CallerDir(), path)
	}
	r, err := files.Resolve(filepath.Clean(abs), false)
	if err != nil || !r.Exists || !r.Info.Mode.IsRegular() {
		return nil, notFound()
	}
	if dest == item.Shared && refused(b, r) {
		return nil, notFound()
	}
	parent, err := files.OpenRoot(filepath.Dir(r.Path), false)
	if err != nil {
		return nil, notFound()
	}
	defer parent.Close()
	if info, err := files.DirInfo(parent); err != nil || len(r.Ancestors) == 0 || info.Identity != r.Ancestors[len(r.Ancestors)-1] {
		return nil, notFound()
	}
	data, info, err := readBounded(files, parent, filepath.Base(r.Path), MaxInputBytes)
	switch {
	case errors.Is(err, errTooLarge):
		return nil, &errs.Error{Code: "too_large", Message: "the input file is over 5 MiB", Exit: 1}
	case errors.Is(err, ErrChanged):
		return nil, notFound()
	case err != nil:
		return nil, ioError("the input file cannot be read")
	}
	if info.Identity != r.Info.Identity {
		return nil, notFound()
	}
	return data, nil
}

// refused reports a resolved input inside a protected path, by path or by the identity of an
// existing protected folder or file, or below any folder whose folded name is .pbrain.
func refused(b *brain.Brain, r sys.ResolvedPath) bool {
	for _, comp := range strings.Split(filepath.ToSlash(r.Path), "/") {
		if item.Fold(comp) == ".pbrain" {
			return true
		}
	}
	files := b.Files()
	for _, p := range b.Protected() {
		if r.Path == p.Path || strings.HasPrefix(r.Path, p.Path+string(filepath.Separator)) {
			return true
		}
		pr, err := files.Resolve(p.Path, false)
		if err != nil || !pr.Exists {
			continue
		}
		if pr.Info.Identity == r.Info.Identity || slices.Contains(r.Ancestors, pr.Info.Identity) {
			return true
		}
	}
	return false
}
