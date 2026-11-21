package semantic

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/embed/embedtest"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// loadSection reads every .md file under root, outside .pbrain/, as a note titled by its name. A
// parent and a child process load the same notes, so their content digests agree.
func loadSection(t *testing.T, kind item.Section, root string) *section {
	t.Helper()
	d, err := sys.OpenDir(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	s := &section{t: t, kind: kind, root: root, dir: d, notes: map[string]note{}, changed: map[string]bool{}}
	filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() && e.Name() == ".pbrain" {
			return filepath.SkipDir
		}
		if !e.IsDir() && strings.HasSuffix(p, ".md") {
			b, _ := os.ReadFile(p)
			rel, _ := filepath.Rel(root, p)
			s.notes[filepath.ToSlash(rel)] = note{strings.TrimSuffix(e.Name(), ".md"), string(b)}
		}
		return nil
	})
	return s
}

func writeNote(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// envAt is newEnv over a given cache folder, with no temporary folder of its own.
func envAt(t *testing.T, cachePath string) *env {
	t.Helper()
	k := embedtest.New(nil)
	c, err := OpenCache(context.Background(), CacheOptions{Path: cachePath, Files: sys.Native()}, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return &env{t: t, kit: k, dl: k.Downloader(), cache: c, path: cachePath, model: embedtest.ModelA,
		clock: &clock{t: time.Unix(1_800_000_000, 0), step: 30 * time.Second}, reg: k.Registry}
}

// hookEmbed wraps a registry so that every Embed call runs before first.
type hookEmbed struct {
	embed.Registry
	before func(texts []string)
}

type hookEmbedder struct {
	embed.Embedder
	before func([]string)
}

func (h hookEmbed) Open(ctx context.Context, a *embed.Artifact) (embed.Embedder, error) {
	e, err := h.Registry.Open(ctx, a)
	if err != nil {
		return nil, err
	}
	return hookEmbedder{e, h.before}, nil
}

func (h hookEmbedder) Embed(ctx context.Context, role embed.Role, texts []string) ([][]float32, error) {
	h.before(texts)
	return h.Embedder.Embed(ctx, role, texts)
}

// TestBuildChild is the child process of the tests below; it runs only when they start it.
func TestBuildChild(t *testing.T) {
	mode := os.Getenv("PBRAIN_SEMANTIC_CHILD")
	if mode == "" {
		t.Skip("run by TestIndexTouchesNoFile and TestBuildKilled")
	}
	root, cachePath, kind := os.Getenv("PBRAIN_ROOT"), os.Getenv("PBRAIN_CACHE_DIR"), item.Shared
	if mode == "kill" {
		kind = item.Personal
	}
	e := envAt(t, cachePath)
	s := loadSection(t, kind, root)
	index := filepath.Join(root, ".pbrain", "index")
	var calls atomic.Int32
	e.reg = hookEmbed{e.kit.Registry, func([]string) {
		n := calls.Add(1)
		switch mode {
		case "nofile":
			// Between any two steps, index/ holds lock, manifest and at most one database.
			es, _ := os.ReadDir(index)
			dbs := 0
			for _, x := range es {
				switch {
				case x.Name() == "lock" || x.Name() == "manifest":
				case dbName.MatchString(x.Name()):
					dbs++
				default:
					fmt.Fprintf(os.Stderr, "unexpected file in index/: %s\n", x.Name())
					os.Exit(3)
				}
			}
			if dbs > 1 {
				fmt.Fprintln(os.Stderr, "two databases in index/")
				os.Exit(3)
			}
		case "kill":
			if n == 2 { // inside the long item: tell the parent and wait for the kill
				os.WriteFile(os.Getenv("PBRAIN_READY"), []byte("ready"), 0o644)
				select {}
			}
		}
	}}
	if _, err := e.build(s); err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(2)
	}
}

func child(t *testing.T, mode, root, cachePath, cwd string, extra ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestBuildChild$", "-test.count=1")
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "PBRAIN_SEMANTIC_CHILD="+mode, "PBRAIN_ROOT="+root, "PBRAIN_CACHE_DIR="+cachePath)
	cmd.Env = append(cmd.Env, extra...)
	return cmd
}

// TestIndexTouchesNoFile: a build in a process whose working folder is empty and read-only and
// whose temporary folder does not exist succeeds, and index/ holds only lock, manifest and one
// database, also between its steps.
func TestIndexTouchesNoFile(t *testing.T) {
	base := t.TempDir()
	root, cachePath, cwd := filepath.Join(base, "section"), filepath.Join(base, "cache"), filepath.Join(base, "cwd")
	for i := range 12 {
		writeNote(t, root, fmt.Sprintf("notes/n%02d.md", i), strings.Repeat("alpha beta ", 10*(i+1))+"\n")
	}
	os.MkdirAll(cwd, 0o555)
	os.Chmod(cwd, 0o555)
	t.Cleanup(func() { os.Chmod(cwd, 0o755) })
	missingTmp := filepath.Join(base, "no-such-tmp")
	out, err := child(t, "nofile", root, cachePath, cwd, "TMPDIR="+missingTmp, "TMP="+missingTmp, "TEMP="+missingTmp).CombinedOutput()
	if err != nil {
		t.Fatalf("the child build: %v\n%s", err, out)
	}
	names, _ := os.ReadDir(filepath.Join(root, ".pbrain", "index"))
	var got []string
	for _, n := range names {
		if !dbName.MatchString(n.Name()) {
			got = append(got, n.Name())
		}
	}
	if len(names) != 3 || !slices.Equal(got, []string{"lock", "manifest"}) {
		t.Fatalf("index/ holds %v", names)
	}
	if es, _ := os.ReadDir(cwd); len(es) != 0 {
		t.Fatalf("the working folder holds %v", es)
	}
	if _, err := os.Stat(missingTmp); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the temporary folder was created")
	}
	if pb, _ := os.ReadDir(filepath.Join(root, ".pbrain")); len(pb) != 1 {
		t.Fatalf(".pbrain/ holds %v", pb)
	}
}

// TestBuildKilled: a process killed in the middle of one long personal item leaves that item out
// of the index and keeps what was published; the next run embeds the item again from its start.
func TestBuildKilled(t *testing.T) {
	base := t.TempDir()
	root, cachePath := filepath.Join(base, "personal"), filepath.Join(base, "cache")
	writeNote(t, root, "a.md", "alpha one\n")
	writeNote(t, root, "b.md", "beta two\n")
	e := envAt(t, cachePath)
	if r := e.mustBuild(loadSection(t, item.Personal, root)); r.Items != 2 {
		t.Fatalf("the first build: %+v", r)
	}
	var long strings.Builder
	for i := range 30 {
		fmt.Fprintf(&long, "%s paragraph %d gamma\n\n", strings.Repeat("word ", 250), i)
	}
	writeNote(t, root, "z-long.md", long.String())
	ready := filepath.Join(base, "ready")
	cmd := child(t, "kill", root, cachePath, base, "PBRAIN_READY="+ready)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cmd.Process.Kill()
			t.Fatal("the child never reached the long item")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cmd.Process.Signal(syscall.SIGKILL)
	cmd.Wait()

	s := loadSection(t, item.Personal, root)
	x, state := s.open(e.m())
	if state != IndexOK || len(x.items) != 2 {
		t.Fatalf("after the kill: %v, %d items", state, len(x.items))
	}
	if _, ok := x.items["z-long.md"]; ok {
		t.Fatal("the killed item is half in the index")
	}
	x.Close()
	want := len(Passages("z-long", []byte(long.String())))
	texts := e.kit.Texts.Load()
	r := e.mustBuild(s)
	if r.Items != 3 || r.PassagesEmbedded != want || r.PassagesReused != 0 || int(e.kit.Texts.Load()-texts) != want {
		t.Fatalf("the next run: %+v, want %d passages embedded", r, want)
	}
}

// TestBuildStaggered: A scans, an item is added and B builds and indexes it, then A gets the lock;
// A scans again under the lock, keeps B's entry and reports nothing removed.
func TestBuildStaggered(t *testing.T) {
	e := newEnv(t)
	s := newSection(t, item.Shared)
	basic(s)
	var bResult BuildResult
	first := true
	scanA := func(ctx context.Context) (SectionInput, error) {
		in, err := s.scan(ctx)
		if first {
			first = false
			s.put("notes/x.md", "X", "delta added\n")
			var berr error
			if bResult, berr = e.build(s); berr != nil {
				t.Fatalf("B: %v", berr)
			}
		}
		return in, err
	}
	rA, err := Build(context.Background(), e.opts(), s.sectionDir(), scanA, e.dl, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bResult.Items != 4 || bResult.PassagesEmbedded != 4 {
		t.Fatalf("B: %+v", bResult)
	}
	if rA.Items != 4 || rA.ItemsRemoved != 0 || rA.PassagesEmbedded != 0 {
		t.Fatalf("A after B: %+v", rA)
	}
	x, state := s.open(e.m())
	if state != IndexOK {
		t.Fatal(state)
	}
	defer x.Close()
	if _, ok := x.items["notes/x.md"]; !ok {
		t.Fatal("A dropped B's entry")
	}
}
