// Package brain finds one brain, reads its settings, checks where private data can land (the
// exposure guard) and owns the folder handles of one command (architecture 4.2, 6; components/brain.md).
package brain

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pkieltyka/pbrain/internal/audience"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/gitx"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// Env is what opening a brain takes from the outside; tests replace all of it.
type Env struct {
	Dir    string // absolute command working directory
	Getenv func(string) string
	Git    gitx.Runner
	Now    func() time.Time
	Files  sys.Files
}

// OpenOptions are the command's brain selector.
type OpenOptions struct{ Brain string } // raw --brain value, empty when absent

// InitOptions are init's selectors.
type InitOptions struct {
	OpenOptions
	Path string // raw --path, empty when absent
}

// InitResult is what init made, for its output.
type InitResult struct {
	Dir           string // canonical target, internal only
	Path          string // relative to the project root, or the supplied absolute external path
	Selector      string // usable from the caller's directory as --brain or PBRAIN_DIR
	NeedsSelector bool
}

// ProtectedPath is one path of spec 5.1 that shared reads, scans and file inputs never enter.
type ProtectedPath struct {
	Path      string // absolute, canonical when reachable
	Directory bool   // intended role, also for a missing path
}

// IgnorePath is a path to ask Git's ignore rules about.
type IgnorePath struct {
	Path      string // absolute and inside the brain's worktree, if any
	Directory bool
}

// View is what a command may see: shared items, or shared and personal together.
type View uint8

const (
	SharedView View = iota
	PersonalView
)

// Brain is one opened brain, for one command. It is used by pointer and closed once.
type Brain struct {
	Dir      string      // the brain folder, absolute, resolved
	Root     string      // the path anchor (architecture 4.5)
	Config   Config      // pbrain.toml
	Local    LocalConfig // personal.toml; zero value if absent
	Shared   SectionRoot
	Personal SectionRoot // Reachable is false when its folder cannot be reached

	env        Env
	gitTop     string // "" outside a work tree
	dir        sys.Dir
	personal   sys.Dir // nil until opened
	configured bool    // personal.toml sets path
	protected  []guarded
	rulesErr   error
	mu         sync.Mutex
	closed     bool
}

// SectionRoot is where one section's items and local data live.
type SectionRoot struct {
	Section   item.Section
	Dir       string // items live under it
	DataDir   string // Dir + "/.pbrain"
	Reachable bool
}

// Config is pbrain.toml.
type Config struct {
	Search, List Limits
	Get, Prime   struct{ MaxChars int }
	Tags         map[string]string // declared tags; nil when there is no [tags] table
	HasTags      bool
	Semantic     struct {
		Model         string // "" when semantic search is off; embed checks the name
		CUDAPrecision string
	}
}

// Limits are a command's default page size and output budget.
type Limits struct{ Limit, MaxChars int }

// LocalConfig is personal.toml.
type LocalConfig struct {
	Path      string
	Author    string
	Audience  []audience.Rule
	ModelPath string // a copy of the model on this machine; "" when unset
}

var errNotExist = fs.ErrNotExist

func ioError(msg string) error { return &errs.Error{Code: "io", Message: msg, Exit: 1} }

func noBrain() error {
	return &errs.Error{Code: "no_brain", Message: "no brain found: run pbrain init, or name one with --brain or PBRAIN_DIR", Exit: 1}
}

func gitRequired(msg string) error { return &errs.Error{Code: "git_required", Message: msg, Exit: 1} }

func personalUnavailable() error {
	return &errs.Error{Code: "personal_unavailable", Message: "the personal section set in personal.toml cannot be reached", Exit: 1}
}

// Open finds the brain, loads its settings and runs the exposure guard before any item is read.
func Open(ctx context.Context, env Env, opt OpenOptions) (*Brain, error) {
	dir, err := discover(env, opt)
	if err != nil {
		return nil, err
	}
	return openAt(ctx, env, dir)
}

// selector is the explicit brain folder: --brain, else a non-empty PBRAIN_DIR.
func selector(env Env, opt OpenOptions) string {
	if opt.Brain != "" {
		return opt.Brain
	}
	if env.Getenv != nil {
		return env.Getenv("PBRAIN_DIR")
	}
	return ""
}

func absFrom(base, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(base, p)
}

// discover returns the canonical brain folder: the selector's, or the first marker found walking up
// from the working directory, at each level <dir>/pbrain.toml then <dir>/docs/brain/pbrain.toml.
func discover(env Env, opt OpenOptions) (string, error) {
	files := env.Files
	if sel := selector(env, opt); sel != "" {
		if !filepath.IsAbs(sel) {
			// Resolve the directory before joining: ".." is relative to the physical cwd.
			cwd, err := files.Resolve(env.Dir, false)
			if err != nil || !cwd.Exists {
				return "", noBrain()
			}
			sel = filepath.Join(cwd.Path, sel)
		}
		r, err := files.Resolve(sel, false)
		if err != nil || !r.Exists || !r.Info.Mode.IsDir() {
			return "", noBrain()
		}
		if !hasEntry(files, r.Path, "pbrain.toml") {
			return "", noBrain()
		}
		return r.Path, nil
	}
	start, err := files.Resolve(env.Dir, false)
	if err != nil || !start.Exists {
		return "", noBrain()
	}
	for d := start.Path; ; d = filepath.Dir(d) {
		if hasEntry(files, d, "pbrain.toml") {
			return d, nil
		}
		if r, err := files.Resolve(filepath.Join(d, "docs", "brain"), false); err == nil && r.Exists && r.Info.Mode.IsDir() &&
			hasEntry(files, r.Path, "pbrain.toml") {
			return r.Path, nil
		}
		if d == filepath.Dir(d) {
			return "", noBrain()
		}
	}
}

// hasEntry reports an entry of any type named name in the canonical folder dir.
func hasEntry(files sys.Files, dir, name string) bool {
	h, err := files.OpenRoot(dir, false)
	if err != nil {
		return false
	}
	defer h.Close()
	_, err = files.Stat(h, name)
	return err == nil
}

// gitDirOf is the nearest folder at or above dir holding a .git entry, or "".
func gitDirOf(files sys.Files, dir string) string {
	for d := dir; ; d = filepath.Dir(d) {
		if hasEntry(files, d, ".git") {
			return d
		}
		if d == filepath.Dir(d) {
			return ""
		}
	}
}

// topOf asks Git for the top of the work tree holding dir, which has a .git entry at want, and
// refuses an answer that is another folder.
func topOf(ctx context.Context, env Env, dir, want string) (string, error) {
	if env.Git == nil {
		return "", gitRequired("git is needed for this work tree but cannot be run")
	}
	top, err := gitx.Top(ctx, env.Git, dir)
	if err != nil {
		return "", err
	}
	r, err := env.Files.Resolve(top, false)
	if err != nil || r.Path != want {
		return "", gitRequired("git reported a work tree other than the one holding this folder")
	}
	return r.Path, nil
}

func openAt(ctx context.Context, env Env, dir string) (_ *Brain, err error) {
	files := env.Files
	b := &Brain{env: env, Dir: dir}
	defer func() {
		if err != nil {
			b.Close()
		}
	}()
	if b.dir, err = files.OpenRoot(dir, false); err != nil {
		return nil, noBrain()
	}
	data, _, rerr := b.dir.Read("pbrain.toml")
	switch {
	case errors.Is(rerr, errNotExist):
		return nil, noBrain()
	case errors.Is(rerr, sys.ErrUnsafe):
		return nil, invalidConfig("pbrain.toml is not a regular file")
	case errors.Is(rerr, sys.ErrTooLarge):
		return nil, invalidConfig("pbrain.toml is too large")
	case rerr != nil:
		return nil, ioError("pbrain.toml cannot be read")
	}
	if b.Config, err = decodeProject(data); err != nil {
		return nil, err
	}
	b.Root = dir
	if g := gitDirOf(files, dir); g != "" {
		if b.gitTop, err = topOf(ctx, env, dir, g); err != nil {
			return nil, err
		}
		b.Root = b.gitTop
	}
	b.Shared = SectionRoot{Section: item.Shared, Dir: dir, DataDir: filepath.Join(dir, ".pbrain"), Reachable: true}

	// The fixed protected paths are checked before personal.toml is read.
	fixed := []guarded{
		{ProtectedPath{filepath.Join(dir, "personal"), true}, "the default personal folder"},
		{ProtectedPath{filepath.Join(dir, "personal.toml"), false}, "personal.toml"},
		{ProtectedPath{filepath.Join(dir, ".pbrain"), true}, "the shared .pbrain folder"},
	}
	if err := b.guard(ctx, fixed); err != nil {
		return nil, err
	}
	l, err := readLocalFile(b.dir)
	if err != nil {
		return nil, err
	}
	b.Local, b.rulesErr = l.config, l.rulesErr

	var personal []guarded
	if b.Local.Path != "" {
		b.configured = true
		if err := b.placePersonal(); err != nil {
			return nil, err
		}
		if b.Personal.Reachable {
			personal = []guarded{
				{ProtectedPath{b.Personal.Dir, true}, "the personal section"},
				{ProtectedPath{b.Personal.DataDir, true}, "the personal .pbrain folder"},
			}
		}
	} else {
		p := filepath.Join(dir, "personal")
		b.Personal = SectionRoot{Section: item.Personal, Dir: p, DataDir: filepath.Join(p, ".pbrain"), Reachable: true}
		personal = []guarded{{ProtectedPath{b.Personal.DataDir, true}, "the personal .pbrain folder"}}
	}
	if err := b.guard(ctx, personal); err != nil {
		return nil, err
	}
	b.protected = append(fixed, personal...)
	if b.configured {
		if err := b.dormantDefault(); err != nil {
			return nil, err
		}
	}
	return b, nil
}

// placePersonal works out the configured personal section: a relative path starts at the brain;
// the section may not be the brain or hold it. A folder that cannot be reached is recorded so.
func (b *Brain) placePersonal() error {
	files := b.env.Files
	r, err := files.Resolve(absFrom(b.Dir, b.Local.Path), true)
	if err != nil {
		b.Personal = SectionRoot{Section: item.Personal, Dir: absFrom(b.Dir, b.Local.Path)}
		b.Personal.DataDir = filepath.Join(b.Personal.Dir, ".pbrain")
		return nil
	}
	p := r.Path
	if p == b.Dir || strings.HasPrefix(b.Dir+string(filepath.Separator), p+string(filepath.Separator)) {
		return invalidConfig("personal.toml: path may not be the brain folder or hold it")
	}
	if r.Exists {
		self, err := files.Resolve(b.Dir, false)
		if err == nil && (r.Info.Identity == self.Info.Identity || slices.Contains(self.Ancestors, r.Info.Identity)) {
			return invalidConfig("personal.toml: path may not be the brain folder or hold it")
		}
	}
	b.Personal = SectionRoot{Section: item.Personal, Dir: p, DataDir: filepath.Join(p, ".pbrain")}
	if !r.Exists || !r.Info.Mode.IsDir() {
		return nil
	}
	h, err := files.OpenRoot(p, false)
	if err != nil {
		return nil
	}
	b.personal = h
	b.Personal.Reachable = true
	return nil
}

// dormantDefault refuses a brain whose default personal folder still holds an item when
// personal.toml sets another path (spec 10).
func (b *Brain) dormantDefault() error {
	if filepath.Join(b.Dir, "personal") == b.Personal.Dir {
		return nil
	}
	h, err := b.dir.Sub("personal", false)
	if err != nil {
		return nil // no default folder, or not a folder: the guard has checked its type
	}
	defer h.Close()
	found, err := holdsItem(b.env.Files, h, 0)
	if err != nil {
		return ioError("the default personal folder cannot be read")
	}
	if found {
		return invalidConfig("personal/ holds items while personal.toml sets another path; move them there")
	}
	return nil
}

// maxItemBytes is the store's whole-file limit (spec 2.4).
const maxItemBytes = 4 << 20

func holdsItem(files sys.Files, dir sys.Dir, depth int) (bool, error) {
	if depth > 64 {
		return false, nil
	}
	entries, err := files.Entries(dir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name, ".") {
			continue
		}
		switch {
		case e.Info.Mode.IsDir():
			sub, err := dir.Sub(e.Name, false)
			if err != nil {
				continue
			}
			found, err := holdsItem(files, sub, depth+1)
			sub.Close()
			if err != nil || found {
				return found, err
			}
		case e.Info.Mode.IsRegular() && strings.HasSuffix(e.Name, ".md") && e.Info.Size <= maxItemBytes:
			data, _, err := dir.Read(e.Name)
			if err != nil || len(data) > maxItemBytes || !validUTF8(data) {
				continue
			}
			if _, _, _, err := item.Parse(strings.TrimSuffix(e.Name, ".md"), data); err == nil {
				return true, nil
			}
		}
	}
	return false, nil
}

// Files is the file seam the brain was opened with.
func (b *Brain) Files() sys.Files { return b.env.Files }

// CallerDir is the command's working directory.
func (b *Brain) CallerDir() string { return b.env.Dir }

// OpenSection returns the section's folder handle, the same one on every call of a command. The
// brain still owns it. A default personal folder that does not exist yet is fs.ErrNotExist; a
// configured one that cannot be reached is personal_unavailable.
func (b *Brain) OpenSection(section item.Section) (sys.Dir, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, ioError("the brain is closed")
	}
	if section == item.Shared {
		return b.dir, nil
	}
	if b.personal != nil {
		return b.personal, nil
	}
	if b.configured {
		return nil, personalUnavailable()
	}
	h, err := b.dir.Sub("personal", false)
	if err != nil {
		if errors.Is(err, errNotExist) {
			return nil, err
		}
		return nil, ioError("the personal section cannot be opened")
	}
	b.personal = h
	return h, nil
}

// EnsurePersonal is OpenSection(item.Personal) for a write that chooses the personal section: it
// creates the default folder when missing and checks it before use. It never creates a configured
// folder.
func (b *Brain) EnsurePersonal(ctx context.Context) (sys.Dir, error) {
	h, err := b.OpenSection(item.Personal)
	if err == nil || !errors.Is(err, errNotExist) {
		return h, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.personal != nil {
		return b.personal, nil
	}
	created, err := b.dir.Sub("personal", true)
	if err != nil {
		return nil, ioError("the personal section cannot be created")
	}
	g := []guarded{{ProtectedPath{b.Personal.Dir, true}, "the default personal folder"}}
	if err := b.guard(ctx, g); err != nil {
		created.Close()
		return nil, err
	}
	b.personal = created
	return created, nil
}

// Close releases every handle the brain holds, once.
func (b *Brain) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	var first error
	for _, h := range []sys.Dir{b.personal, b.dir} {
		if h != nil {
			if err := h.Close(); err != nil && first == nil {
				first = err
			}
		}
	}
	return first
}

// RequireView fails with personal_unavailable when the view needs a personal section that
// cannot be reached.
func (b *Brain) RequireView(view View) error {
	if view == PersonalView && b.configured && !b.Personal.Reachable {
		return personalUnavailable()
	}
	return nil
}

// Protected returns a copy of the protected paths.
func (b *Brain) Protected() []ProtectedPath {
	out := make([]ProtectedPath, len(b.protected))
	for i, g := range b.protected {
		out[i] = g.ProtectedPath
	}
	return out
}

// SharedIgnored reports, in input order, whether Git's ignore rules match each path of the brain's
// work tree. Outside a work tree, and for a path outside it, the answer is false.
func (b *Brain) SharedIgnored(ctx context.Context, paths []IgnorePath) ([]bool, error) {
	out := make([]bool, len(paths))
	if b.gitTop == "" || len(paths) == 0 {
		return out, nil
	}
	var asked []gitx.Path
	var at []int
	for i, p := range paths {
		rel, ok := under(b.gitTop, p.Path)
		if !ok || rel == "." {
			continue
		}
		asked = append(asked, gitx.Path{Rel: rel, Directory: p.Directory})
		at = append(at, i)
	}
	if len(asked) == 0 {
		return out, nil
	}
	if b.env.Git == nil {
		return nil, gitRequired("git is needed for this work tree but cannot be run")
	}
	got, err := gitx.Ignored(ctx, b.env.Git, b.gitTop, asked)
	if err != nil {
		return nil, err
	}
	for j, i := range at {
		out[i] = got[j]
	}
	return out, nil
}

// under is p relative to root when p lies at or under root.
func under(root, p string) (string, bool) {
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	return rel, true
}

// DisplayPath is how output names a file of the section (architecture 4.5): relative to the anchor
// with forward slashes, or personal: and its path in the personal section when it lies outside the
// anchor. A path outside its section is never printed: the answer is "".
func (b *Brain) DisplayPath(section item.Section, path string) string {
	root := b.Shared.Dir
	if section == item.Personal {
		root = b.Personal.Dir
	}
	if _, ok := under(root, path); !ok {
		return ""
	}
	if rel, ok := under(b.Root, path); ok {
		return filepath.ToSlash(rel)
	}
	if section == item.Personal {
		rel, _ := under(b.Personal.Dir, path)
		return "personal:" + filepath.ToSlash(rel)
	}
	return ""
}

// ResolveAuthor picks the author of a write: the flag, a non-empty PBRAIN_AUTHOR, personal.toml's
// author, then human: and Git's user.name in the brain's folder. The chosen value must be one
// non-empty line of at most 200 characters; a bad higher choice is refused, never skipped.
func (b *Brain) ResolveAuthor(ctx context.Context, explicit string) (string, error) {
	pick := func(v, from string) (string, error) {
		if !validActor(v) {
			return "", &errs.Error{Code: "invalid", Message: "author from " + from + ": one non-empty line of at most 200 characters", Exit: 1}
		}
		return v, nil
	}
	if explicit != "" {
		return pick(explicit, "--author")
	}
	if b.env.Getenv != nil {
		if v := b.env.Getenv("PBRAIN_AUTHOR"); v != "" {
			return pick(v, "PBRAIN_AUTHOR")
		}
	}
	if b.Local.Author != "" {
		return pick(b.Local.Author, "personal.toml")
	}
	if b.env.Git != nil {
		name, found, err := gitx.UserName(ctx, b.env.Git, b.Dir)
		if err != nil {
			return "", err
		}
		if found && name != "" {
			return pick("human:"+name, "git user.name")
		}
	}
	return "", &errs.Error{Code: "invalid", Message: "no author: pass --author, set PBRAIN_AUTHOR, or set author in personal.toml", Exit: 1}
}

// AudienceRules returns a copy of personal.toml's validated rules, or the error that made them
// unusable.
func (b *Brain) AudienceRules() ([]audience.Rule, error) {
	if b.rulesErr != nil {
		return nil, b.rulesErr
	}
	return slices.Clone(b.Local.Audience), nil
}
