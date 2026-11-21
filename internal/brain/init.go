package brain

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/gitx"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// ignoreRules are the lines init makes sure the brain's .gitignore holds (spec 2.1).
var ignoreRules = []string{"personal/", "personal.toml", ".pbrain/"}

func invalid(msg string) error { return &errs.Error{Code: "invalid", Message: msg, Exit: 1} }

// Init creates the brain folder with its pbrain.toml and .gitignore where they are missing, then
// opens it, which runs the exposure guard (spec 9.2, architecture 6.5). It refuses a folder other
// than this project's existing brain, and inside a work tree any pbrain.toml Git lists elsewhere.
func Init(ctx context.Context, env Env, opt InitOptions) (InitResult, error) {
	files := env.Files
	cwd, err := files.Resolve(env.Dir, false)
	if err != nil || !cwd.Exists {
		return InitResult{}, ioError("the working directory cannot be read")
	}
	root, top := cwd.Path, ""
	if gd := gitDirOf(files, cwd.Path); gd != "" {
		if top, err = topOf(ctx, env, cwd.Path, gd); err != nil {
			return InitResult{}, err
		}
		root = top
	}

	var target, shown string
	switch {
	case opt.Path != "":
		target, shown = absFrom(root, opt.Path), opt.Path
	case selector(env, opt.OpenOptions) != "":
		shown = selector(env, opt.OpenOptions)
		target = absFrom(cwd.Path, shown)
	default:
		target, shown = filepath.Join(root, "docs", "brain"), "docs/brain"
	}
	r, err := files.Resolve(target, true)
	if err != nil {
		return InitResult{}, invalid("the brain folder cannot be a folder there")
	}
	target = r.Path
	if r.Exists && !r.Info.Mode.IsDir() {
		return InitResult{}, invalid("the brain folder cannot be a folder there: a file has that name")
	}

	// This project's brain, if any: the walk of spec 10 up to the project root, and Git's list.
	if existing := walkWithin(files, cwd.Path, root); existing != "" && existing != target {
		rel, _ := under(root, existing)
		return InitResult{}, invalid("this project already has a brain at " + filepath.ToSlash(rel))
	}
	if top != "" {
		markers, err := gitx.BrainMarkers(ctx, env.Git, top)
		if err != nil {
			return InitResult{}, err
		}
		for _, m := range markers {
			folder, err := files.Resolve(filepath.Join(top, filepath.Dir(m)), false)
			if err != nil || folder.Path != target {
				return InitResult{}, invalid("this project already has a pbrain.toml at " + filepath.ToSlash(m))
			}
		}
	}

	h, err := files.OpenRoot(target, true)
	if err != nil {
		return InitResult{}, invalid("the brain folder cannot be a folder there")
	}
	err = initFiles(files, h)
	h.Close()
	if err != nil {
		return InitResult{}, err
	}
	b, err := openAt(ctx, env, target)
	if err != nil {
		return InitResult{}, err
	}
	b.Close()

	res := InitResult{Dir: target, NeedsSelector: target != filepath.Join(root, "docs", "brain")}
	if rel, ok := under(root, target); ok {
		res.Path = filepath.ToSlash(rel)
	} else {
		res.Path = shown
	}
	return res, nil
}

// walkWithin is the brain the walk of spec 10 finds from dir without going above root, or "".
func walkWithin(files sys.Files, dir, root string) string {
	for d := dir; ; d = filepath.Dir(d) {
		if _, ok := under(root, d); !ok {
			return ""
		}
		if hasEntry(files, d, "pbrain.toml") {
			return d
		}
		if r, err := files.Resolve(filepath.Join(d, "docs", "brain"), false); err == nil && r.Exists && r.Info.Mode.IsDir() &&
			hasEntry(files, r.Path, "pbrain.toml") {
			return r.Path
		}
		if d == root || d == filepath.Dir(d) {
			return ""
		}
	}
}

// initFiles makes pbrain.toml where it is missing, and adds the missing ignore rules to .gitignore
// through one complete new file published in place, so a failure leaves the old bytes.
func initFiles(files sys.Files, dir sys.Dir) error {
	if _, err := files.Stat(dir, "pbrain.toml"); isNotExist(err) {
		if err := publish(files, dir, "pbrain.toml", nil, false); err != nil && !errors.Is(err, errNotExist) {
			if !isExist(err) {
				return ioError("pbrain.toml cannot be created")
			}
		}
	} else if err != nil {
		return ioError("pbrain.toml cannot be checked")
	}

	var old []byte
	exists := false
	info, err := files.Stat(dir, ".gitignore")
	switch {
	case isNotExist(err):
	case err != nil:
		return ioError(".gitignore cannot be checked")
	case !info.Mode.IsRegular():
		return invalid("the brain's .gitignore is a link or not a regular file")
	default:
		exists = true
		if old, _, err = dir.Read(".gitignore"); err != nil {
			return invalid("the brain's .gitignore cannot be read")
		}
	}
	have := map[string]bool{}
	for line := range strings.SplitSeq(string(old), "\n") {
		have[strings.TrimSuffix(line, "\r")] = true
	}
	next := bytes.Clone(old)
	for _, rule := range ignoreRules {
		if have[rule] {
			continue
		}
		if len(next) > 0 && next[len(next)-1] != '\n' {
			next = append(next, '\n')
		}
		next = append(next, rule+"\n"...)
	}
	if exists && bytes.Equal(next, old) {
		return nil
	}
	if err := publish(files, dir, ".gitignore", next, exists); err != nil {
		return ioError("the brain's .gitignore cannot be written")
	}
	return nil
}

// publish writes data to a temporary file in dir and gives it name: as a new name, or over an
// existing regular file. The temporary file is removed on every path.
func publish(files sys.Files, dir sys.Dir, name string, data []byte, replace bool) error {
	tmp, w, err := dir.CreateTemp()
	if err != nil {
		return err
	}
	defer dir.Remove(tmp) //nolint:errcheck // after LinkNew the name is a second link; after Replace it is gone
	if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
		w.Close()
		return err
	}
	if err := files.Flush(w); err != nil {
		return err
	}
	if replace {
		err = dir.Replace(tmp, name)
	} else {
		err = dir.LinkNew(tmp, name)
	}
	if err != nil {
		return err
	}
	return dir.Sync()
}

func isExist(err error) bool { return errors.Is(err, fs.ErrExist) }
