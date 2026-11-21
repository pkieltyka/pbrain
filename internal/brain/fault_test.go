package brain

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/pkieltyka/pbrain/internal/sys"
)

// faultFiles is the native file seam with publication failures switched on by name.
type faultFiles struct {
	sys.Files
	failReplace, failLink bool
}

type faultDir struct {
	sys.Dir
	f *faultFiles
}

var errInjected = errors.New("injected failure")

func (f *faultFiles) wrap(d sys.Dir, err error) (sys.Dir, error) {
	if err != nil {
		return nil, err
	}
	return faultDir{d, f}, nil
}

func unwrap(d sys.Dir) sys.Dir {
	if fd, ok := d.(faultDir); ok {
		return fd.Dir
	}
	return d
}

func (f *faultFiles) OpenRoot(p string, create bool) (sys.Dir, error) {
	return f.wrap(f.Files.OpenRoot(p, create))
}
func (f *faultFiles) DirInfo(d sys.Dir) (sys.FileInfo, error) { return f.Files.DirInfo(unwrap(d)) }
func (f *faultFiles) Entries(d sys.Dir) ([]sys.Entry, error)  { return f.Files.Entries(unwrap(d)) }
func (f *faultFiles) Stat(d sys.Dir, name string) (sys.FileInfo, error) {
	return f.Files.Stat(unwrap(d), name)
}
func (f *faultFiles) OpenRegular(d sys.Dir, name string) (sys.RegularFile, error) {
	return f.Files.OpenRegular(unwrap(d), name)
}
func (f *faultFiles) Lock(ctx context.Context, d sys.Dir, name string) (io.Closer, error) {
	return f.Files.Lock(ctx, unwrap(d), name)
}
func (d faultDir) Sub(name string, create bool) (sys.Dir, error) {
	return d.f.wrap(d.Dir.Sub(name, create))
}
func (d faultDir) Replace(temp, name string) error {
	if d.f.failReplace {
		return errInjected
	}
	return d.Dir.Replace(temp, name)
}
func (d faultDir) LinkNew(temp, name string) error {
	if d.f.failLink {
		return errInjected
	}
	return d.Dir.LinkNew(temp, name)
}

// A failed publication of .gitignore keeps the marker and the old bytes and leaves no temporary
// file; a repeat then adds exactly the missing rules.
func TestInitWritesIgnoreAtomically(t *testing.T) {
	w := newWorld(t)
	must(t, os.MkdirAll(w.path("p"), 0o700))
	w.gitIn("p", "init", "-q")
	w.write("p/docs/brain/.gitignore", "*.swp") // no final newline
	ff := &faultFiles{Files: sys.Native(), failReplace: true}
	env := w.env("p")
	env.Files = ff
	if _, err := Init(context.Background(), env, InitOptions{}); code(err) != "io" {
		t.Fatalf("a failed rename: %v", err)
	}
	if data, _ := os.ReadFile(w.path("p/docs/brain/.gitignore")); string(data) != "*.swp" {
		t.Fatalf("the old bytes changed: %q", data)
	}
	if _, err := os.Stat(w.path("p/docs/brain/pbrain.toml")); err != nil {
		t.Fatal("the marker was not created before the failure")
	}
	entries, _ := os.ReadDir(w.path("p/docs/brain"))
	if len(entries) != 2 {
		t.Fatalf("a temporary file was left: %v", entries)
	}
	ff.failReplace = false
	if _, err := Init(context.Background(), env, InitOptions{}); err != nil {
		t.Fatalf("the retry: %v", err)
	}
	if data, _ := os.ReadFile(w.path("p/docs/brain/.gitignore")); string(data) != "*.swp\npersonal/\npersonal.toml\n.pbrain/\n" {
		t.Fatalf("after the retry: %q", data)
	}
	// A failed first creation of .gitignore leaves none.
	must(t, os.MkdirAll(w.path("q"), 0o700))
	w.gitIn("q", "init", "-q")
	ff.failLink = true
	env = w.env("q")
	env.Files = ff
	if _, err := Init(context.Background(), env, InitOptions{}); err == nil {
		t.Fatal("a failed link succeeded")
	}
	if _, err := os.Stat(w.path("q/docs/brain/.gitignore")); err == nil {
		t.Fatal("a failed creation left a .gitignore")
	}
}
