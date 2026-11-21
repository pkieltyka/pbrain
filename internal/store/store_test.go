package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/audience"
	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/gitx"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type world struct {
	t    *testing.T
	root string
	git  gitx.Runner
	gitN *countingRunner
}

// countingRunner counts git calls and records the paths each check-ignore was asked about.
type countingRunner struct {
	gitx.Runner
	mu    sync.Mutex
	asked []string
}

func (c *countingRunner) Run(ctx context.Context, dir string, args []string, stdin []byte) (gitx.Result, error) {
	if slices.Contains(args, "check-ignore") {
		c.mu.Lock()
		for p := range strings.SplitSeq(string(stdin), "\x00") {
			if p != "" {
				c.asked = append(c.asked, p)
			}
		}
		c.mu.Unlock()
	}
	return c.Runner.Run(ctx, dir, args, stdin)
}

func newWorld(t *testing.T) *world {
	t.Helper()
	g, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	must(t, err)
	home := filepath.Join(root, "home")
	must(t, os.MkdirAll(home, 0o700))
	r := gitx.NewRunner(g, []string{"HOME=" + home, "XDG_CONFIG_HOME=" + home, "GIT_CONFIG_NOSYSTEM=1", "PATH=" + os.Getenv("PATH")})
	w := &world{t: t, root: root, gitN: &countingRunner{Runner: r}}
	w.git = w.gitN
	must(t, os.MkdirAll(w.path("p"), 0o700))
	w.gitIn("init", "-q")
	w.write("p/docs/brain/pbrain.toml", "")
	w.write("p/docs/brain/.gitignore", "personal/\npersonal.toml\n.pbrain/\n")
	return w
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (w *world) path(rel string) string      { return filepath.Join(w.root, filepath.FromSlash(rel)) }
func (w *world) brainPath(rel string) string { return w.path("p/docs/brain/" + rel) }

func (w *world) write(rel, content string) {
	w.t.Helper()
	must(w.t, os.MkdirAll(filepath.Dir(w.path(rel)), 0o700))
	must(w.t, os.WriteFile(w.path(rel), []byte(content), 0o600))
}

func (w *world) gitIn(args ...string) {
	w.t.Helper()
	res, err := w.git.Run(context.Background(), w.path("p"), args, nil)
	if err != nil || res.Status != 0 {
		w.t.Fatalf("git %v: %v %d", args, err, res.Status)
	}
}

func note(title string) string {
	return "---\nkind: note\ntitle: " + title + "\ncreated: 2026-11-01T00:00:00Z\nauthor: human:x\n---\n" + title + " body\n"
}

func (w *world) open() *brain.Brain {
	w.t.Helper()
	b, err := brain.Open(context.Background(), brain.Env{Dir: w.path("p"), Getenv: func(string) string { return "" }, Git: w.git, Files: sys.Native()}, brain.OpenOptions{})
	must(w.t, err)
	w.t.Cleanup(func() { b.Close() })
	return b
}

func code(err error) string {
	var e *errs.Error
	if errors.As(err, &e) {
		return e.Code
	}
	if err == nil {
		return ""
	}
	return "other"
}

func ids(c *Catalog) []string {
	var out []string
	for _, it := range c.Items() {
		out = append(out, it.ID)
	}
	return out
}

// Ignored, private and dot folders are pruned before they are listed: Git is asked about a
// folder, never about anything inside an ignored one.
func TestSharedScanPrunesBeforeList(t *testing.T) {
	w := newWorld(t)
	w.write("p/docs/brain/notes/a.md", note("A"))
	w.write("p/docs/brain/drafts/x.md", note("X"))
	w.write("p/docs/brain/drafts/deep/y.md", note("Y"))
	w.write("p/docs/brain/.hidden/h.md", note("H"))
	w.write("p/docs/brain/personal/notes/p.md", note("P"))
	w.write("p/docs/brain/notes/ignored.md", note("I"))
	w.write("p/.gitignore", "docs/brain/drafts/\ndocs/brain/notes/ignored.md\n")
	w.gitIn("add", "-f", "docs/brain/drafts/x.md") // tracked inside an ignored folder: still skipped
	must(t, os.Symlink(w.path("p/docs/brain/notes"), w.path("p/docs/brain/linked")))
	b := w.open()
	w.gitN.asked = nil
	c, err := Scan(context.Background(), b, brain.SharedView, KeepAll)
	must(t, err)
	if got := ids(c); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("shared items %v", got)
	}
	for _, p := range w.gitN.asked {
		if strings.Contains(p, "drafts/") && p != "./docs/brain/drafts/" || strings.Contains(p, "personal") || strings.Contains(p, ".hidden") {
			t.Errorf("git was asked about %q", p)
		}
	}
	c, err = Scan(context.Background(), b, brain.PersonalView, KeepAll)
	must(t, err)
	if got := ids(c); !slices.Equal(got, []string{"a", "p"}) {
		t.Fatalf("personal view %v", got)
	}
	if c.Lookup("P")[0].Section != item.Personal {
		t.Fatal("Lookup does not fold")
	}
}

func TestWholeFileValidationAndStableCatalog(t *testing.T) {
	w := newWorld(t)
	w.write("p/docs/brain/notes/good.md", note("Good"))
	w.write("p/docs/brain/notes/bad-yaml.md", "---\nkind: [\n---\nbody\n")
	w.write("p/docs/brain/notes/dup.md", "---\nkind: note\nkind: note\n---\n")
	w.write("p/docs/brain/notes/plain.md", "no front matter\n")
	w.write("p/docs/brain/notes/bad-utf8.md", note("U")+"\xff\n")
	w.write("p/docs/brain/notes/big.md", note("B")+strings.Repeat("x", MaxItemBytes))
	w.write("p/docs/brain/decisions/good.md", note("Same id")) // duplicate folded id
	b := w.open()
	var first []string
	for range 5 {
		c, err := Scan(context.Background(), b, brain.SharedView, KeepNone)
		must(t, err)
		got := ids(c)
		if first == nil {
			first = got
		} else if !slices.Equal(got, first) {
			t.Fatal("the order changed between scans")
		}
		if len(c.Lookup("GOOD")) != 2 {
			t.Fatal("duplicate ids are not both kept")
		}
		reasons := map[string]string{}
		for _, s := range c.Skipped() {
			reasons[filepath.Base(s.Path)] = s.Code + "/" + s.Reason
		}
		want := map[string]string{
			"bad-yaml.md": "bad_front_matter/syntax", "dup.md": "bad_front_matter/duplicate_key",
			"plain.md": "not_an_item/", "bad-utf8.md": "bad_front_matter/invalid_utf8", "big.md": "bad_front_matter/too_large",
		}
		for k, v := range want {
			if reasons[k] != v {
				t.Fatalf("%s: %q, want %q (%v)", k, reasons[k], v, reasons)
			}
		}
	}
	if !slices.Equal(first, []string{"good", "good"}) {
		t.Fatalf("items %v", first)
	}
}

func TestSkippedDiagnosticsFromScan(t *testing.T) {
	w := newWorld(t)
	w.write("p/docs/brain/notes/dup.md", "---\nkind: note\ntitle: secret-value\ntitle: x\n---\n")
	b := w.open()
	c, err := Scan(context.Background(), b, brain.SharedView, KeepNone)
	must(t, err)
	s := c.Skipped()[0]
	if s.Line == 0 || strings.Contains(fmt.Sprint(s), "secret-value") {
		t.Fatalf("skipped entry %+v", s)
	}
}

func TestBodySnapshotAndReread(t *testing.T) {
	w := newWorld(t)
	w.write("p/docs/brain/notes/a.md", note("A"))
	b := w.open()
	all, err := Scan(context.Background(), b, brain.SharedView, KeepAll)
	must(t, err)
	none, err := Scan(context.Background(), b, brain.SharedView, KeepNone)
	must(t, err)
	it := none.Items()[0]
	body, err := none.Body(it)
	if err != nil || string(body) != "A body\n" {
		t.Fatalf("KeepNone body %q %v", body, err)
	}
	if it.Layout.BodyChars != 7 || it.Layout.BodyLine == 0 {
		t.Fatalf("layout %+v", it.Layout)
	}
	// A change after the scan: KeepAll keeps the scanned bytes, KeepNone reports ErrChanged.
	time.Sleep(10 * time.Millisecond)
	w.write("p/docs/brain/notes/a.md", note("A changed"))
	if body, err := all.Body(all.Items()[0]); err != nil || string(body) != "A body\n" {
		t.Fatalf("KeepAll body after a change %q %v", body, err)
	}
	if _, err := none.Body(it); !errors.Is(err, ErrChanged) {
		t.Fatalf("KeepNone after a change: %v", err)
	}
	must(t, os.Remove(w.path("p/docs/brain/notes/a.md")))
	if _, err := none.Body(it); !errors.Is(err, ErrChanged) {
		t.Fatalf("KeepNone after removal: %v", err)
	}
}

func TestOpenInputBoundary(t *testing.T) {
	w := newWorld(t)
	w.write("p/docs/brain/personal/secret.md", "personal")
	w.write("p/docs/brain/personal.toml", "")
	w.write("p/docs/brain/.pbrain/originals/x", "original")
	w.write("other/brain/.PBRAIN/data", "other brain's data")
	w.write("p/input.md", "hello")
	must(t, os.Symlink(w.path("p/docs/brain/personal/secret.md"), w.path("p/alias.md")))
	b := w.open()
	data, err := OpenInput(b, item.Shared, "input.md")
	if err != nil || string(data) != "hello" {
		t.Fatalf("an ordinary input: %q %v", data, err)
	}
	missing := func(p string) error { _, err := OpenInput(b, item.Shared, p); return err }
	want := missing("nope.md")
	if code(want) != "not_found" {
		t.Fatalf("a missing file: %v", want)
	}
	for _, p := range []string{
		w.path("p/docs/brain/personal/secret.md"), "alias.md", w.path("p/docs/brain/personal.toml"),
		w.path("p/docs/brain/.pbrain/originals/x"), w.path("other/brain/.PBRAIN/data"), w.path("p/docs/brain/personal"),
	} {
		if err := missing(p); code(err) != "not_found" || err.Error() != want.Error() {
			t.Errorf("%s: %v", p, err)
		}
	}
	// A personal write may read its own section's files.
	if data, err := OpenInput(b, item.Personal, "alias.md"); err != nil || string(data) != "personal" {
		t.Fatalf("a personal destination: %q %v", data, err)
	}
	w.write("p/big.md", strings.Repeat("x", MaxInputBytes+1))
	if err := missing("big.md"); code(err) != "too_large" {
		t.Fatalf("an over-large input: %v", err)
	}
}

func TestSymlinkAndFIFOReplacement(t *testing.T) {
	w := newWorld(t)
	w.write("p/docs/brain/notes/a.md", note("A"))
	w.write("outside/a.md", note("Outside"))
	b := w.open()
	c, err := Scan(context.Background(), b, brain.SharedView, KeepNone)
	must(t, err)
	it := c.Items()[0]
	// The folder swapped for a link to another folder holding the same name.
	must(t, os.Rename(w.path("p/docs/brain/notes"), w.path("p/docs/brain/notes-old")))
	must(t, os.Symlink(w.path("outside"), w.path("p/docs/brain/notes")))
	if _, err := c.Body(it); !errors.Is(err, ErrChanged) {
		t.Fatalf("a swapped folder: %v", err)
	}
	must(t, os.Remove(w.path("p/docs/brain/notes")))
	must(t, os.Rename(w.path("p/docs/brain/notes-old"), w.path("p/docs/brain/notes")))
	// A FIFO in place of a note never stalls a scan.
	must(t, os.Remove(w.path("p/docs/brain/notes/a.md")))
	must(t, mkfifo(w.path("p/docs/brain/notes/a.md")))
	done := make(chan error, 1)
	go func() { _, err := Scan(context.Background(), b, brain.SharedView, KeepAll); done <- err }()
	select {
	case err := <-done:
		must(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("a FIFO stalled the scan")
	}
}

func lockHolder(t *testing.T, w *world, b *brain.Brain) (*Tx, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	return Begin(ctx, b, item.Shared)
}

func TestProcessSectionLock(t *testing.T) {
	w := newWorld(t)
	b := w.open()
	tx, err := Begin(context.Background(), b, item.Shared)
	must(t, err)
	if _, err := lockHolder(t, w, b); code(err) != "io" {
		t.Fatalf("a second writer of one section: %v", err)
	}
	// The personal section's lock is independent.
	ptx, err := Begin(context.Background(), b, item.Personal)
	must(t, err)
	must(t, ptx.Close())
	must(t, tx.Close())
	must(t, tx.Close())
	tx2, err := lockHolder(t, w, b)
	must(t, err)
	tx2.Close()
	if st, err := os.Stat(w.brainPath(".pbrain/lock")); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("lock file %v", err)
	}
}

func TestLockWaitPolicies(t *testing.T) {
	w := newWorld(t)
	b := w.open()
	tx, err := Begin(context.Background(), b, item.Shared)
	must(t, err)
	defer tx.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if _, err := Begin(ctx, b, item.Shared); code(err) != "io" || time.Since(start) > time.Second {
		t.Fatalf("a cancelled wait: %v", err)
	}
}

func TestExclusivePublish(t *testing.T) {
	w := newWorld(t)
	b := w.open()
	tx, err := Begin(context.Background(), b, item.Shared)
	must(t, err)
	defer tx.Close()
	ctx := context.Background()
	must(t, tx.CheckDestination(ctx, item.Note, "n1"))
	if _, err := tx.Publish(ctx, item.Note, "n1", []byte(note("N1"))); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(w.brainPath("notes/n1.md")); !bytes.Equal(data, []byte(note("N1"))) {
		t.Fatal("published bytes differ")
	}
	if err := tx.CheckDestination(ctx, item.Note, "n1"); !errors.Is(err, ErrNameExists) {
		t.Fatalf("an occupied name: %v", err)
	}
	if _, err := tx.Publish(ctx, item.Note, "n1", []byte("other")); !errors.Is(err, ErrNameExists) {
		t.Fatalf("publish over an existing name: %v", err)
	}
	if data, _ := os.ReadFile(w.brainPath("notes/n1.md")); !bytes.Equal(data, []byte(note("N1"))) {
		t.Fatal("an existing item was overwritten")
	}
	entries, _ := os.ReadDir(w.brainPath("notes"))
	if len(entries) != 1 {
		t.Fatalf("temporary files left: %v", entries)
	}
	// An ignored destination folder and a protected one are invalid.
	w.write("p/.gitignore", "docs/brain/sources/\n")
	if err := tx.CheckDestination(ctx, item.Source, "s1"); code(err) != "invalid" {
		t.Fatalf("an ignored destination: %v", err)
	}
	must(t, os.Symlink(w.path("p"), w.brainPath("learnings")))
	if err := tx.CheckDestination(ctx, item.Learning, "l1"); code(err) != "invalid" {
		t.Fatalf("a linked kind folder: %v", err)
	}
	if err := tx.CheckDestination(ctx, item.Note, "../x"); code(err) != "invalid" {
		t.Fatalf("an id with a separator: %v", err)
	}
}

func TestReplaceAndRemoveCheckTheVersion(t *testing.T) {
	w := newWorld(t)
	w.write("p/docs/brain/notes/a.md", note("A"))
	b := w.open()
	c, err := Scan(context.Background(), b, brain.SharedView, KeepAll)
	must(t, err)
	it := c.Items()[0]
	tx, err := Begin(context.Background(), b, item.Shared)
	must(t, err)
	defer tx.Close()
	data, v, err := tx.ReadFile(it)
	if err != nil || v != it.Version || !bytes.Equal(data, []byte(note("A"))) {
		t.Fatalf("ReadFile: %v", err)
	}
	if _, err := tx.ReplaceItem(it, v, []byte(note("A2"))); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(w.brainPath("notes/a.md")); string(got) != note("A2") {
		t.Fatal("not replaced")
	}
	// The scanned version is now stale: replace and remove are conflict.
	if _, err := tx.ReplaceItem(it, v, []byte("x")); code(err) != "conflict" {
		t.Fatalf("a stale replace: %v", err)
	}
	if _, err := tx.RemoveItem(it); code(err) != "conflict" {
		t.Fatalf("a stale remove: %v", err)
	}
	c, _ = Scan(context.Background(), b, brain.SharedView, KeepAll)
	if _, err := tx.RemoveItem(c.Items()[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.brainPath("notes/a.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("not removed")
	}
	// A personal item is refused under the shared lock.
	w.write("p/docs/brain/personal/notes/p.md", note("P"))
	c, _ = Scan(context.Background(), b, brain.PersonalView, KeepAll)
	for _, pit := range c.Items() {
		if pit.Section == item.Personal {
			if _, _, err := tx.ReadFile(pit); code(err) != "invalid" {
				t.Fatalf("an item of another section: %v", err)
			}
		}
	}
}

func TestDigestScopedOriginal(t *testing.T) {
	w := newWorld(t)
	b := w.open()
	tx, err := Begin(context.Background(), b, item.Shared)
	must(t, err)
	data := []byte("original bytes")
	digest := sha256.Sum256(data)
	must(t, tx.PutOriginal(digest, data))
	must(t, tx.PutOriginal(digest, data)) // reused
	if err := tx.PutOriginal(sha256.Sum256([]byte("other")), data); code(err) != "io" {
		t.Fatalf("a digest mismatch: %v", err)
	}
	name := fmt.Sprintf("%x", digest)
	must(t, os.WriteFile(w.brainPath(".pbrain/originals/"+name), []byte("corrupt bytes!"), 0o600))
	if err := tx.PutOriginal(digest, data); code(err) != "io" {
		t.Fatalf("a corrupt original: %v", err)
	}
	must(t, os.WriteFile(w.brainPath(".pbrain/originals/"+name), data, 0o600))
	tx.Close()
	// The personal section's originals are separate.
	ptx, err := Begin(context.Background(), b, item.Personal)
	must(t, err)
	if removed, err := ptx.RemoveOriginal(digest); removed || err == nil {
		t.Fatal("a personal delete removed the shared original")
	}
	ptx.Close()
	tx, _ = Begin(context.Background(), b, item.Shared)
	defer tx.Close()
	if removed, err := tx.RemoveOriginal(digest); !removed || err != nil {
		t.Fatalf("RemoveOriginal: %v %v", removed, err)
	}
	if removed, _ := tx.RemoveOriginal(digest); removed {
		t.Fatal("a missing original was removed")
	}
}

// Two cooperating setters each add a rule under the shared lock; both rules survive, also with an
// external personal section (carried from phase 5).
func TestLocalSettingsConcurrentSetters(t *testing.T) {
	w := newWorld(t)
	must(t, os.MkdirAll(w.path("ext"), 0o700))
	w.write("p/docs/brain/personal.toml", "path = \""+w.path("ext")+"\"\nauthor = \"human:p\"\n")
	var wg sync.WaitGroup
	errc := make(chan error, 8)
	for i := range 8 {
		wg.Go(func() {
			b, err := brain.Open(context.Background(), brain.Env{Dir: w.path("p"), Getenv: func(string) string { return "" }, Git: w.git, Files: sys.Native()}, brain.OpenOptions{})
			if err != nil {
				errc <- err
				return
			}
			defer b.Close()
			tx, err := Begin(context.Background(), b, item.Shared)
			if err != nil {
				errc <- err
				return
			}
			defer tx.Close()
			root, _ := b.OpenSection(item.Shared)
			data, changed, err := brain.EditAudience(root, func(r []audience.Rule) ([]audience.Rule, error) {
				return audience.Set(r, fmt.Sprintf("example.com/team%d", i), audience.Personal)
			})
			if err == nil && changed {
				_, err = tx.ReplaceSettings(data)
			}
			if err != nil {
				errc <- err
			}
		})
	}
	wg.Wait()
	close(errc)
	for err := range errc {
		t.Fatal(err)
	}
	root, err := sys.OpenDir(w.brainPath(""))
	must(t, err)
	defer root.Close()
	l, err := brain.ReadLocal(root)
	if err != nil || len(l.Audience) != 8 || l.Author != "human:p" || l.Path != w.path("ext") {
		t.Fatalf("after 8 setters: %+v %v", l, err)
	}
}

// A kind folder replaced, after the scan, by another ordinary folder holding the same bytes with
// the same modification time is not the scanned item: replace and remove are conflict and the
// replacement is untouched.
func TestMutationFollowsScannedIdentity(t *testing.T) {
	w := newWorld(t)
	w.write("p/docs/brain/notes/a.md", note("A"))
	b := w.open()
	c, err := Scan(context.Background(), b, brain.SharedView, KeepAll)
	must(t, err)
	it := c.Items()[0]
	tx, err := Begin(context.Background(), b, item.Shared)
	must(t, err)
	defer tx.Close()
	st, _ := os.Stat(w.brainPath("notes/a.md"))
	must(t, os.Rename(w.brainPath("notes"), w.brainPath("notes-old")))
	w.write("p/docs/brain/notes/a.md", note("A"))
	must(t, os.Chtimes(w.brainPath("notes/a.md"), st.ModTime(), st.ModTime()))
	if _, _, err := tx.ReadFile(it); code(err) != "conflict" {
		t.Fatalf("ReadFile through a swapped folder: %v", err)
	}
	if _, err := tx.ReplaceItem(it, it.Version, []byte(note("B"))); code(err) != "conflict" {
		t.Fatalf("ReplaceItem through a swapped folder: %v", err)
	}
	if _, err := tx.RemoveItem(it); code(err) != "conflict" {
		t.Fatalf("RemoveItem through a swapped folder: %v", err)
	}
	if data, _ := os.ReadFile(w.brainPath("notes/a.md")); string(data) != note("A") {
		t.Fatal("the replacement folder's file changed")
	}
	// The file itself replaced by one with the same bytes and time, in the same folder.
	must(t, os.RemoveAll(w.brainPath("notes")))
	must(t, os.Rename(w.brainPath("notes-old"), w.brainPath("notes")))
	must(t, os.Remove(w.brainPath("notes/a.md")))
	w.write("p/docs/brain/notes/a.md", note("A"))
	must(t, os.Chtimes(w.brainPath("notes/a.md"), st.ModTime(), st.ModTime()))
	if _, err := tx.RemoveItem(it); code(err) != "conflict" {
		t.Fatalf("RemoveItem of a replaced file: %v", err)
	}
	// An item no scan made is refused.
	if _, _, err := tx.ReadFile(&item.Item{Section: item.Shared, Path: w.brainPath("notes/a.md")}); code(err) != "invalid" {
		t.Fatalf("an item from no scan: %v", err)
	}
}

// The scanned routes live only as long as their items: repeated scans in one process do not grow
// the table without bound.
func TestScannedRoutesAreReleased(t *testing.T) {
	w := newWorld(t)
	for i := range 20 {
		w.write(fmt.Sprintf("p/docs/brain/notes/n%d.md", i), note(fmt.Sprint(i)))
	}
	b := w.open()
	count := func() int {
		n := 0
		scanned.Range(func(any, any) bool { n++; return true })
		return n
	}
	base := count()
	for range 50 {
		if _, err := Scan(context.Background(), b, brain.SharedView, KeepNone); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	for range 50 {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
		if n = count(); n <= base+20 {
			break
		}
	}
	if n > base+20 {
		t.Fatalf("%d routes kept after 50 scans of 20 items", n-base)
	}
	// A live catalog's items keep their routes.
	c, err := Scan(context.Background(), b, brain.SharedView, KeepNone)
	must(t, err)
	runtime.GC()
	for _, it := range c.Items() {
		if _, ok := recall(it); !ok {
			t.Fatal("a live item lost its route")
		}
	}
}

// TestNameAdmission is store.md's test: a file with front matter whose stem is not valid UTF-8 is a
// bad_file_name skip, never an item, so it cannot share a key with a stem that really holds U+FFFD.
func TestNameAdmission(t *testing.T) {
	w := newWorld(t)
	w.write("p/docs/brain/notes/x\xff.md", note("Bad name"))
	w.write("p/docs/brain/notes/x\uFFFD.md", note("Real replacement character"))
	w.write("p/docs/brain/plain/x\xff.md", "no front matter\n")
	w.write("p/docs/brain/dup/x\xff.md", "---\nkind: note\nkind: note\n---\n")
	w.write("p/docs/brain/odd\xfe/good-stem.md", note("Under an odd folder"))
	w.write("p/docs/brain/notes/a\nb.md", note("Line feed"))
	c, err := Scan(context.Background(), w.open(), brain.SharedView, KeepAll)
	must(t, err)
	got := ids(c)
	slices.Sort(got)
	if want := []string{"a\nb", "good-stem", "x\uFFFD"}; !slices.Equal(got, want) {
		t.Fatalf("items %q, want %q", got, want)
	}
	if found := c.Lookup("x\uFFFD"); len(found) != 1 || found[0].ID != "x\uFFFD" {
		t.Fatalf("lookup of the real U+FFFD stem: %v", found)
	}
	if found := c.Lookup("x\xff"); len(found) != 0 {
		t.Fatalf("lookup of invalid bytes found %v", found)
	}
	if found := c.Lookup("A\nB"); len(found) != 1 {
		t.Fatal("the stem with a line feed is not an item")
	}
	reasons := map[string]string{}
	for _, s := range c.Skipped() {
		rel, _ := filepath.Rel(w.brainPath(""), s.Path)
		reasons[filepath.ToSlash(rel)] = s.Code + "/" + s.Reason
		if s.Line != 0 || s.Column != 0 {
			t.Fatalf("%q has a position", rel)
		}
	}
	want := map[string]string{
		"notes/x\xff.md": "bad_file_name/invalid_utf8",
		"plain/x\xff.md": "not_an_item/",
		"dup/x\xff.md":   "bad_file_name/invalid_utf8",
	}
	if !maps.Equal(reasons, want) {
		t.Fatalf("skipped %q, want %q", reasons, want)
	}
}
