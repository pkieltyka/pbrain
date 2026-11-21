package brain

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/gitx"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// guarded is a protected path with the role its messages name. A message never names a file.
type guarded struct {
	ProtectedPath
	role string
}

func exposed(role, remedy string) error {
	return &errs.Error{Code: "personal_exposed", Message: role + " " + remedy, Exit: 1}
}

func validUTF8(b []byte) bool { return utf8.Valid(b) }

// guard is the exposure guard of spec 5.3 for paths (architecture 6.4): inside a work tree each
// must be matched by an ignore rule and hold no tracked file, and wherever it lies an existing one
// must hold no .git entry at any depth, its own top included. A path's role decides whether it is
// probed as a folder, so an absent one is checked in the right form.
func (b *Brain) guard(ctx context.Context, paths []guarded) error {
	files := b.env.Files
	type tree struct {
		top   string
		items []guarded
	}
	var trees []*tree
	byGit := map[string]*tree{}
	for _, g := range paths {
		if err := b.checkShape(g); err != nil {
			return err
		}
		start, err := nearestExisting(files, filepath.Dir(g.Path))
		if err != nil {
			return exposed(g.role, "cannot be checked; make sure it can be read")
		}
		gd := gitDirOf(files, start)
		if gd == "" {
			continue
		}
		t := byGit[gd]
		if t == nil {
			top := gd
			if gd != b.gitTop {
				var err error
				if top, err = topOf(ctx, b.env, start, gd); err != nil {
					return err
				}
			}
			t = &tree{top: top}
			byGit[gd] = t
			trees = append(trees, t)
		}
		t.items = append(t.items, g)
	}
	for _, t := range trees {
		if b.env.Git == nil {
			return gitRequired("git is needed for this work tree but cannot be run")
		}
		asked := make([]gitx.Path, len(t.items))
		for i, g := range t.items {
			rel, _ := under(t.top, g.Path)
			asked[i] = gitx.Path{Rel: rel, Directory: g.Directory}
		}
		ignored, err := gitx.Ignored(ctx, b.env.Git, t.top, asked)
		if err != nil {
			return err
		}
		for i, g := range t.items {
			if !ignored[i] {
				return exposed(g.role, "is not ignored by Git: add it to the brain's .gitignore (pbrain init writes the rules)")
			}
			tracked, err := gitx.Tracked(ctx, b.env.Git, t.top, asked[i])
			if err != nil {
				return err
			}
			if tracked {
				return exposed(g.role, "holds a file tracked by Git: remove it from the index with git rm --cached")
			}
		}
	}
	return nil
}

// checkShape checks an existing protected path's type and, for a folder, that no .git entry lies
// at or under it. The walk follows no link and reads no file.
func (b *Brain) checkShape(g guarded) error {
	files := b.env.Files
	parent := filepath.Dir(g.Path)
	ph, err := files.OpenRoot(parent, false)
	if err != nil {
		if isNotExist(err) {
			return nil // a missing parent: the path does not exist
		}
		return exposed(g.role, "cannot be checked; make sure it can be read")
	}
	defer ph.Close()
	info, err := files.Stat(ph, filepath.Base(g.Path))
	if err != nil {
		if isNotExist(err) {
			return nil
		}
		return exposed(g.role, "cannot be checked; make sure it can be read")
	}
	if !g.Directory {
		if !info.Mode.IsRegular() {
			return exposed(g.role, "is a link or not a regular file; replace it with a plain file")
		}
		return nil
	}
	if !info.Mode.IsDir() {
		return exposed(g.role, "is a link or not a folder; replace it with a plain folder")
	}
	h, err := ph.Sub(filepath.Base(g.Path), false)
	if err != nil {
		return exposed(g.role, "cannot be checked; make sure it can be read")
	}
	defer h.Close()
	found, err := holdsGit(files, h, 0)
	if err != nil {
		return exposed(g.role, "cannot be checked; make sure it can be read")
	}
	if found {
		return exposed(g.role, "holds a Git repository (.git): move the repository out of it")
	}
	return nil
}

func holdsGit(files sys.Files, dir sys.Dir, depth int) (bool, error) {
	if depth > 128 {
		return false, fs.ErrInvalid
	}
	entries, err := files.Entries(dir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if e.Name == ".git" {
			return true, nil
		}
	}
	for _, e := range entries {
		if !e.Info.Mode.IsDir() {
			continue
		}
		sub, err := dir.Sub(e.Name, false)
		if err != nil {
			return false, err
		}
		found, err := holdsGit(files, sub, depth+1)
		sub.Close()
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}

// nearestExisting is dir or its nearest existing ancestor. Only a folder that does not exist is
// climbed past; any other failure to open one is an error, so the guard fails closed.
func nearestExisting(files sys.Files, dir string) (string, error) {
	for d := dir; ; d = filepath.Dir(d) {
		h, err := files.OpenRoot(d, false)
		if err == nil {
			h.Close()
			return d, nil
		}
		if !isNotExist(err) {
			return "", err
		}
		if d == filepath.Dir(d) {
			return d, nil
		}
	}
}

func isNotExist(err error) bool { return errors.Is(err, fs.ErrNotExist) }
