package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// faultFiles is the native seam with chosen failures in the folder operations of a write.
type faultFiles struct {
	sys.Files
	failLink, failRemove, failSync, failReplace bool
}

type faultDir struct {
	sys.Dir
	f *faultFiles
}

var errInjected = errors.New("injected")

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
func (f *faultFiles) TryLock(d sys.Dir, name string) (io.Closer, error) {
	return f.Files.TryLock(unwrap(d), name)
}
func (d faultDir) Sub(name string, create bool) (sys.Dir, error) {
	return d.f.wrap(d.Dir.Sub(name, create))
}
func (d faultDir) LinkNew(temp, name string) error {
	if d.f.failLink {
		return errInjected
	}
	return d.Dir.LinkNew(temp, name)
}
func (d faultDir) Replace(temp, name string) error {
	if d.f.failReplace {
		return errInjected
	}
	return d.Dir.Replace(temp, name)
}
func (d faultDir) Remove(name string) error {
	if d.f.failRemove {
		return errInjected
	}
	return d.Dir.Remove(name)
}
func (d faultDir) Sync() error {
	if d.f.failSync {
		return errInjected
	}
	return d.Dir.Sync()
}

func (w *world) openWith(files sys.Files) *brain.Brain {
	w.t.Helper()
	b, err := brain.Open(context.Background(), brain.Env{Dir: w.path("p"), Getenv: func(string) string { return "" }, Git: w.git, Files: files}, brain.OpenOptions{})
	must(w.t, err)
	w.t.Cleanup(func() { b.Close() })
	return b
}

// A failure before the link leaves no item and no temporary file; failures after it are reported
// as cleanup with the item committed.
func TestCommitPointFailures(t *testing.T) {
	w := newWorld(t)
	ff := &faultFiles{Files: sys.Native()}
	b := w.openWith(ff)
	tx, err := Begin(context.Background(), b, item.Shared)
	must(t, err)
	defer tx.Close()
	ctx := context.Background()
	must(t, os.MkdirAll(w.brainPath("notes"), 0o700))

	ff.failLink = true
	if _, err := tx.Publish(ctx, item.Note, "a", []byte(note("A"))); code(err) != "io" {
		t.Fatalf("a failed link: %v", err)
	}
	if entries, _ := os.ReadDir(w.brainPath("notes")); len(entries) != 0 {
		t.Fatalf("a failed publish left %v", entries)
	}
	ff.failLink = false

	ff.failRemove, ff.failSync = true, true
	c, err := tx.Publish(ctx, item.Note, "b", []byte(note("B")))
	if err != nil {
		t.Fatalf("cleanup failures after the commit point were an error: %v", err)
	}
	if !slices.Equal(c.Cleanup, []Cleanup{TempLeft, FolderSyncFailed}) {
		t.Fatalf("cleanup %v", c.Cleanup)
	}
	if _, err := os.Stat(w.brainPath("notes/b.md")); err != nil {
		t.Fatal("the committed item is not visible")
	}
	ff.failRemove, ff.failSync = false, false

	// An original that cannot be stored leaves no original; the caller then publishes no item.
	ff.failLink = true
	data := []byte("orig")
	if err := tx.PutOriginal(sha256.Sum256(data), data); code(err) != "io" {
		t.Fatalf("a failed original: %v", err)
	}
	ff.failLink = false
}

// A huge sparse file at the digest's name is refused by its size, never hashed.
func TestOriginalReadIsBounded(t *testing.T) {
	w := newWorld(t)
	b := w.open()
	tx, err := Begin(context.Background(), b, item.Shared)
	must(t, err)
	defer tx.Close()
	data := []byte("small original")
	digest := sha256.Sum256(data)
	must(t, os.MkdirAll(w.brainPath(".pbrain/originals"), 0o700))
	f, err := os.Create(w.brainPath(fmt.Sprintf(".pbrain/originals/%x", digest)))
	must(t, err)
	must(t, f.Truncate(64<<30))
	f.Close()
	start := time.Now()
	if err := tx.PutOriginal(digest, data); code(err) != "io" {
		t.Fatalf("a sparse original: %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("the sparse file was read")
	}
}

// A failed rename of personal.toml leaves no staged copy of the new settings beside the brain.
func TestReplaceSettingsFailureLeavesNoTemp(t *testing.T) {
	w := newWorld(t)
	w.write("p/docs/brain/personal.toml", "author = \"human:old\"\n")
	ff := &faultFiles{Files: sys.Native()}
	b := w.openWith(ff)
	tx, err := Begin(context.Background(), b, item.Shared)
	must(t, err)
	defer tx.Close()
	ff.failReplace = true
	if _, err := tx.ReplaceSettings([]byte("author = \"human:new\"\n")); code(err) != "io" {
		t.Fatalf("a failed rename: %v", err)
	}
	entries, _ := os.ReadDir(w.brainPath(""))
	for _, e := range entries {
		if e.Name() != "pbrain.toml" && e.Name() != ".gitignore" && e.Name() != "personal.toml" && e.Name() != ".pbrain" {
			t.Fatalf("left beside the brain: %s", e.Name())
		}
	}
	if data, _ := os.ReadFile(w.brainPath("personal.toml")); string(data) != "author = \"human:old\"\n" {
		t.Fatalf("the settings changed: %q", data)
	}
}

// A sync failure of the new originals folder's parent, or of the originals folder after the link,
// is io before the referencing item can be published.
func TestOriginalSyncFailures(t *testing.T) {
	w := newWorld(t)
	ff := &faultFiles{Files: sys.Native()}
	b := w.openWith(ff)
	tx, err := Begin(context.Background(), b, item.Shared)
	must(t, err)
	defer tx.Close()
	data := []byte("orig")
	digest := sha256.Sum256(data)
	ff.failSync = true
	if err := tx.PutOriginal(digest, data); code(err) != "io" {
		t.Fatalf("the parent sync after creating originals: %v", err)
	}
	ff.failSync = false
	must(t, os.MkdirAll(w.brainPath(".pbrain/originals"), 0o700))
	ff.failSync = true
	if err := tx.PutOriginal(sha256.Sum256([]byte("other")), []byte("other")); code(err) != "io" {
		t.Fatalf("the originals sync after the link: %v", err)
	}
	ff.failSync = false
	must(t, tx.PutOriginal(digest, data))
}
