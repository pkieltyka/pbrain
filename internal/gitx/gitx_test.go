package gitx

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/errs"
)

func gitPath(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	return p
}

// repo makes a work tree with its own identity, isolated from the person's configuration.
func repo(t *testing.T) (string, Runner) {
	t.Helper()
	g := gitPath(t)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	env := []string{"HOME=" + home, "XDG_CONFIG_HOME=" + home, "GIT_CONFIG_NOSYSTEM=1", "PATH=" + os.Getenv("PATH")}
	r := NewRunner(g, env)
	run := func(args ...string) {
		t.Helper()
		res, err := r.Run(context.Background(), dir, args, nil)
		if err != nil || res.Status != 0 {
			t.Fatalf("git %v: %v %d", args, err, res.Status)
		}
	}
	run("init", "-q")
	return dir, r
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func isRequired(err error) bool {
	var e *errs.Error
	return errors.As(err, &e) && e.Code == "git_required"
}

func TestTop(t *testing.T) {
	dir, r := repo(t)
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o700); err != nil {
		t.Fatal(err)
	}
	top, err := Top(context.Background(), r, filepath.Join(dir, "a", "b"))
	if err != nil || top != dir {
		t.Fatalf("Top = %q, %v; want %q", top, err, dir)
	}
	if _, err := Top(context.Background(), r, t.TempDir()); !isRequired(err) {
		t.Fatalf("outside a work tree: %v", err)
	}
}

// Paths are literal: a space, a newline, glob characters and a leading colon are ordinary.
func TestIgnoredLiteralPaths(t *testing.T) {
	dir, r := repo(t)
	write(t, filepath.Join(dir, ".gitignore"), "personal/\n*.tmp\n\\:colon\n")
	paths := []Path{
		{Rel: "personal", Directory: true},
		{Rel: "personal/notes", Directory: true}, // missing: still matched through its parent rule
		{Rel: "docs/x.tmp"},
		{Rel: ":colon"},
		{Rel: "a b/c.md"},
		{Rel: "new\nline.md"},
		{Rel: "glob[*].md"},
		{Rel: "personal-not", Directory: true},
	}
	got, err := Ignored(context.Background(), r, dir, paths)
	if err != nil {
		t.Fatal(err)
	}
	want := []bool{true, true, true, true, false, false, false, false}
	if !slices.Equal(got, want) {
		t.Fatalf("Ignored = %v, want %v", got, want)
	}
	if got, err := Ignored(context.Background(), r, dir, nil); err != nil || len(got) != 0 {
		t.Fatalf("no paths: %v %v", got, err)
	}
	// A directory-only rule matches a missing folder only when it is passed as a folder.
	got, _ = Ignored(context.Background(), r, dir, []Path{{Rel: "personal"}})
	if got[0] {
		t.Fatal("a directory-only rule matched a file path")
	}
}

// --no-index: a rule counts whether or not the path is tracked.
func TestIgnoredCountsTrackedPaths(t *testing.T) {
	dir, r := repo(t)
	write(t, filepath.Join(dir, ".gitignore"), "secret.md\n")
	write(t, filepath.Join(dir, "secret.md"), "x")
	if res, err := r.Run(context.Background(), dir, []string{"add", "-f", "secret.md"}, nil); err != nil || res.Status != 0 {
		t.Fatal("git add -f")
	}
	got, err := Ignored(context.Background(), r, dir, []Path{{Rel: "secret.md"}})
	if err != nil || !got[0] {
		t.Fatalf("a tracked ignored path: %v %v", got, err)
	}
	tracked, err := Tracked(context.Background(), r, dir, Path{Rel: "secret.md"})
	if err != nil || !tracked {
		t.Fatalf("Tracked: %v %v", tracked, err)
	}
}

func TestTracked(t *testing.T) {
	dir, r := repo(t)
	write(t, filepath.Join(dir, "personal", "a.md"), "x")
	write(t, filepath.Join(dir, "glob[1]", "b.md"), "x")
	write(t, filepath.Join(dir, "glob1", "c.md"), "x")
	for _, p := range []string{"personal/a.md", "glob1/c.md"} {
		if res, err := r.Run(context.Background(), dir, []string{"add", "-f", p}, nil); err != nil || res.Status != 0 {
			t.Fatal("git add")
		}
	}
	for _, c := range []struct {
		p    Path
		want bool
	}{
		{Path{Rel: "personal", Directory: true}, true},
		{Path{Rel: "personal/"}, true},
		{Path{Rel: "glob[1]", Directory: true}, false}, // a literal path, not a pattern matching glob1
		{Path{Rel: "missing", Directory: true}, false},
	} {
		got, err := Tracked(context.Background(), r, dir, c.p)
		if err != nil || got != c.want {
			t.Errorf("Tracked(%+v) = %v, %v", c.p, got, err)
		}
	}
}

// Variables that select another index or repository have no effect.
func TestEnvironmentIsBoundToTheTree(t *testing.T) {
	dir, _ := repo(t)
	write(t, filepath.Join(dir, "a.md"), "x")
	g := gitPath(t)
	home := t.TempDir()
	base := []string{"HOME=" + home, "XDG_CONFIG_HOME=" + home, "GIT_CONFIG_NOSYSTEM=1", "PATH=" + os.Getenv("PATH")}
	plain := NewRunner(g, base)
	if res, err := plain.Run(context.Background(), dir, []string{"add", "a.md"}, nil); err != nil || res.Status != 0 {
		t.Fatal("git add")
	}
	other := t.TempDir()
	hostile := NewRunner(g, append(base,
		"GIT_INDEX_FILE="+filepath.Join(other, "empty-index"),
		"GIT_DIR="+filepath.Join(other, ".git"),
		"GIT_WORK_TREE="+other,
		"GIT_LITERAL_PATHSPECS=0",
		"GIT_GLOB_PATHSPECS=1",
		"GIT_PAGER=less",
	))
	tracked, err := Tracked(context.Background(), hostile, dir, Path{Rel: "a.md"})
	if err != nil || !tracked {
		t.Fatalf("an inherited GIT_INDEX_FILE hid a tracked file: %v %v", tracked, err)
	}
	rr := hostile.(*runner)
	for _, kv := range rr.env {
		name, _, _ := strings.Cut(kv, "=")
		if slices.Contains(removedEnv, name) || name == "GIT_PAGER" && kv != "GIT_PAGER=cat" {
			t.Errorf("%s reached git", kv)
		}
	}
	if !slices.Contains(rr.env, "GIT_OPTIONAL_LOCKS=0") {
		t.Error("optional locks not turned off")
	}
}

func TestBrainMarkers(t *testing.T) {
	dir, r := repo(t)
	write(t, filepath.Join(dir, ".gitignore"), "ignored/\n")
	write(t, filepath.Join(dir, "docs/brain/pbrain.toml"), "")
	write(t, filepath.Join(dir, "custom/pbrain.toml"), "")
	write(t, filepath.Join(dir, "ignored/pbrain.toml"), "")
	write(t, filepath.Join(dir, "gone/pbrain.toml"), "")
	write(t, filepath.Join(dir, "docs/not-pbrain.toml"), "")
	if res, err := r.Run(context.Background(), dir, []string{"add", "docs/brain/pbrain.toml", "gone/pbrain.toml"}, nil); err != nil || res.Status != 0 {
		t.Fatal("git add")
	}
	if err := os.Remove(filepath.Join(dir, "gone/pbrain.toml")); err != nil {
		t.Fatal(err)
	}
	got, err := BrainMarkers(context.Background(), r, dir)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	want := []string{filepath.FromSlash("custom/pbrain.toml"), filepath.FromSlash("docs/brain/pbrain.toml")}
	if !slices.Equal(got, want) {
		t.Fatalf("BrainMarkers = %q, want %q", got, want)
	}
}

func TestUserName(t *testing.T) {
	dir, r := repo(t)
	if name, found, err := UserName(context.Background(), r, dir); err != nil || found || name != "" {
		t.Fatalf("unset: %q %v %v", name, found, err)
	}
	if res, err := r.Run(context.Background(), dir, []string{"config", "user.name", "Ada Lovelace"}, nil); err != nil || res.Status != 0 {
		t.Fatal("git config")
	}
	if name, found, err := UserName(context.Background(), r, dir); err != nil || !found || name != "Ada Lovelace" {
		t.Fatalf("set: %q %v %v", name, found, err)
	}
}

type fake struct {
	res Result
	err error
}

func (f fake) Run(context.Context, string, []string, []byte) (Result, error) { return f.res, f.err }

// Start failures, unexpected statuses and malformed output are git_required, naming no path.
func TestFailuresAreGitRequired(t *testing.T) {
	ctx := context.Background()
	start := fake{err: errors.New("exec: \"git\": not found in /secret/path")}
	bad := fake{res: Result{Status: 128, Stdout: []byte("fatal")}}
	unterminated := fake{res: Result{Status: 0, Stdout: []byte("./a")}}
	for name, err := range map[string]error{
		"top start":        func() error { _, err := Top(ctx, start, "/x"); return err }(),
		"top status":       func() error { _, err := Top(ctx, bad, "/x"); return err }(),
		"top relative":     func() error { _, err := Top(ctx, fake{res: Result{Stdout: []byte("rel\n")}}, "/x"); return err }(),
		"ignored status":   func() error { _, err := Ignored(ctx, bad, "/x", []Path{{Rel: "a"}}); return err }(),
		"ignored protocol": func() error { _, err := Ignored(ctx, unterminated, "/x", []Path{{Rel: "a"}}); return err }(),
		"tracked status":   func() error { _, err := Tracked(ctx, bad, "/x", Path{Rel: "a"}); return err }(),
		"markers start":    func() error { _, err := BrainMarkers(ctx, start, "/x"); return err }(),
		"user status":      func() error { _, _, err := UserName(ctx, bad, "/x"); return err }(),
		"user 1 with output": func() error {
			_, _, err := UserName(ctx, fake{res: Result{Status: 1, Stdout: []byte("x")}}, "/x")
			return err
		}(),
	} {
		if !isRequired(err) {
			t.Errorf("%s: %v", name, err)
		} else if strings.Contains(err.Error(), "/secret") || strings.Contains(err.Error(), "fatal") {
			t.Errorf("%s leaks: %v", name, err)
		}
	}
	// A cancelled context is the context's error, not git_required.
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Top(cctx, start, "/x"); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}
}
