// Package gitx runs the read-only Git queries pbrain needs (architecture 6.3, components/brain.md).
// Paths are passed literally, relative to the work tree's top; every call is bound to that tree and
// its real index whatever the caller's environment selects.
package gitx

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pkieltyka/pbrain/internal/errs"
)

// Result is the outcome of one Git process that ran: its standard output and exit status.
type Result struct {
	Stdout []byte
	Status int
}

// Runner starts Git. An error means Git did not run to an exit status (it could not start, or the
// context ended); a non-zero status is a Result.
type Runner interface {
	Run(ctx context.Context, dir string, args []string, stdin []byte) (Result, error)
}

// Path is one path below the Git top.
type Path struct {
	Rel       string // native relative path below the Git top
	Directory bool
}

// callLimit caps each Git call; a parent deadline that comes first wins.
const callLimit = 10 * time.Second

// removedEnv select another repository, index or object store, or turn on pathspec magic: the
// runner starts Git without them (architecture 6.3).
var removedEnv = []string{
	"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY",
	"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_NAMESPACE", "GIT_CEILING_DIRECTORIES",
	"GIT_LITERAL_PATHSPECS", "GIT_GLOB_PATHSPECS", "GIT_NOGLOB_PATHSPECS", "GIT_ICASE_PATHSPECS",
}

type runner struct {
	executable string
	env        []string
}

// NewRunner returns the production runner for the git executable, starting each process with
// environ less the variables of removedEnv, with optional locks and the pager off.
func NewRunner(executable string, environ []string) Runner {
	env := make([]string, 0, len(environ)+3)
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		drop := name == "GIT_OPTIONAL_LOCKS" || name == "GIT_PAGER" || name == "PAGER" || slices.Contains(removedEnv, name)
		if !drop {
			env = append(env, kv)
		}
	}
	env = append(env, "GIT_OPTIONAL_LOCKS=0", "GIT_PAGER=cat", "PAGER=cat")
	return &runner{executable: executable, env: env}
}

func (r *runner) Run(ctx context.Context, dir string, args []string, stdin []byte) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, callLimit)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.executable, append([]string{"--no-pager"}, args...)...)
	cmd.Dir = dir
	cmd.Env = r.env
	cmd.Stdin = bytes.NewReader(stdin)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr // kept from callers: it can name local paths
	err := cmd.Run()
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.Exited() {
		return Result{Stdout: out.Bytes(), Status: exit.ExitCode()}, nil
	}
	if err != nil {
		return Result{}, err
	}
	return Result{Stdout: out.Bytes(), Status: 0}, nil
}

// required is the error of every failed Git call: it names no path and no Git message.
func required(what string) error {
	return &errs.Error{Code: "git_required", Message: "git " + what + " failed; pbrain needs a working git for this work tree", Exit: 1}
}

func run(ctx context.Context, r Runner, dir, what string, args []string, stdin []byte, ok ...int) (Result, error) {
	res, err := r.Run(ctx, dir, args, stdin)
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return Result{}, required(what)
	}
	for _, s := range ok {
		if res.Status == s {
			return res, nil
		}
	}
	return Result{}, required(what)
}

// Top is the absolute top of the work tree that holds dir.
func Top(ctx context.Context, r Runner, dir string) (string, error) {
	res, err := run(ctx, r, dir, "rev-parse", []string{"rev-parse", "--show-toplevel"}, nil, 0)
	if err != nil {
		return "", err
	}
	top := strings.TrimSuffix(string(res.Stdout), "\n")
	if top == "" || strings.ContainsRune(top, '\n') || !filepath.IsAbs(top) {
		return "", required("rev-parse")
	}
	return filepath.Clean(top), nil
}

// arg is a path as Git takes it on standard input: "./" first, so that a leading colon is an
// ordinary character, and a final slash for a folder, so that directory-only rules match it.
func arg(p Path) string {
	s := "./" + filepath.ToSlash(p.Rel)
	if p.Directory && !strings.HasSuffix(s, "/") {
		s += "/"
	}
	return s
}

// Ignored reports, in input order, whether an ignore rule of the work tree matches each path,
// tracked or not (check-ignore --no-index).
func Ignored(ctx context.Context, r Runner, top string, paths []Path) ([]bool, error) {
	out := make([]bool, len(paths))
	if len(paths) == 0 {
		return out, nil
	}
	var in bytes.Buffer
	for _, p := range paths {
		in.WriteString(arg(p))
		in.WriteByte(0)
	}
	res, err := run(ctx, r, top, "check-ignore", []string{"check-ignore", "--no-index", "-z", "--stdin"}, in.Bytes(), 0, 1)
	if err != nil {
		return nil, err
	}
	matched := map[string]bool{}
	if len(res.Stdout) > 0 {
		if res.Stdout[len(res.Stdout)-1] != 0 {
			return nil, required("check-ignore")
		}
		for name := range strings.SplitSeq(string(res.Stdout[:len(res.Stdout)-1]), "\x00") {
			matched[name] = true
		}
	}
	for i, p := range paths {
		out[i] = matched[arg(p)]
	}
	return out, nil
}

// Tracked reports whether the index holds the path or, for a folder, anything below it.
func Tracked(ctx context.Context, r Runner, top string, path Path) (bool, error) {
	rel := strings.TrimSuffix(filepath.ToSlash(path.Rel), "/")
	if rel == "" {
		rel = "."
	}
	res, err := run(ctx, r, top, "ls-files", []string{"--literal-pathspecs", "ls-files", "--cached", "-z", "--", rel}, nil, 0)
	if err != nil {
		return false, err
	}
	return len(res.Stdout) > 0, nil
}

// BrainMarkers lists, relative to top, every pbrain.toml of the work tree that is tracked or
// untracked and not ignored, and present in the working tree.
func BrainMarkers(ctx context.Context, r Runner, top string) ([]string, error) {
	res, err := run(ctx, r, top, "ls-files", []string{"ls-files", "--cached", "--others", "--exclude-standard", "-z"}, nil, 0)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for rel := range strings.SplitSeq(string(res.Stdout), "\x00") {
		if rel == "" || seen[rel] || filepath.Base(rel) != "pbrain.toml" {
			continue
		}
		seen[rel] = true
		native := filepath.FromSlash(rel)
		if _, err := os.Lstat(filepath.Join(top, native)); err != nil {
			continue // deleted from the working tree
		}
		out = append(out, native)
	}
	return out, nil
}

// UserName reads user.name in dir, so the repository there answers before the global
// configuration. found is false when no value is set.
func UserName(ctx context.Context, r Runner, dir string) (name string, found bool, err error) {
	res, err := run(ctx, r, dir, "config", []string{"config", "user.name"}, nil, 0, 1)
	if err != nil {
		return "", false, err
	}
	if res.Status == 1 {
		if len(res.Stdout) > 0 {
			return "", false, required("config")
		}
		return "", false, nil
	}
	return strings.TrimSuffix(string(res.Stdout), "\n"), true, nil
}
