//go:build linux || darwin

package sys

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// tempRoot is a test folder with its links resolved, opened as a handle.
func tempRoot(t *testing.T) (string, Dir) {
	t.Helper()
	p, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenDir(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return p, d
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestNamesRefused(t *testing.T) {
	_, d := tempRoot(t)
	for _, name := range []string{"", ".", "..", "a/b", "a\x00b", "../x"} {
		if _, err := d.Sub(name, true); !errors.Is(err, ErrInvalidName) {
			t.Errorf("Sub(%q): %v", name, err)
		}
		if _, _, err := d.Read(name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("Read(%q): %v", name, err)
		}
		if err := d.Remove(name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("Remove(%q): %v", name, err)
		}
		if err := d.LinkNew("t", name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("LinkNew(%q): %v", name, err)
		}
		if _, err := Native().Stat(d, name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("Stat(%q): %v", name, err)
		}
	}
}

// No method follows a link, at the last component or on the way to a root.
func TestLinksAreRefused(t *testing.T) {
	p, d := tempRoot(t)
	must(t, os.Mkdir(filepath.Join(p, "real"), 0o700))
	must(t, os.WriteFile(filepath.Join(p, "f.md"), []byte("x"), 0o600))
	must(t, os.Symlink(filepath.Join(p, "real"), filepath.Join(p, "dlink")))
	must(t, os.Symlink(filepath.Join(p, "f.md"), filepath.Join(p, "flink.md")))
	if _, err := d.Sub("dlink", false); !errors.Is(err, ErrUnsafe) {
		t.Errorf("Sub on a link: %v", err)
	}
	if _, _, err := d.Read("flink.md"); !errors.Is(err, ErrUnsafe) {
		t.Errorf("Read on a link: %v", err)
	}
	if _, err := Native().OpenRegular(d, "flink.md"); !errors.Is(err, ErrUnsafe) {
		t.Errorf("OpenRegular on a link: %v", err)
	}
	if _, _, err := d.Read("real"); !errors.Is(err, ErrUnsafe) {
		t.Errorf("Read on a folder: %v", err)
	}
	if err := d.Remove("flink.md"); !errors.Is(err, ErrUnsafe) {
		t.Errorf("Remove on a link: %v", err)
	}
	if err := d.Remove("real"); !errors.Is(err, ErrUnsafe) {
		t.Errorf("Remove on a folder: %v", err)
	}
	info, err := Native().Stat(d, "flink.md")
	if err != nil || info.Mode&fs.ModeSymlink == 0 {
		t.Errorf("Stat followed the link: %v %v", info.Mode, err)
	}
	if _, err := Native().OpenRoot(filepath.Join(p, "dlink"), false); !errors.Is(err, ErrUnsafe) {
		t.Errorf("OpenRoot through a link: %v", err)
	}
	if _, err := Native().Lock(context.Background(), d, "flink.md"); err == nil {
		t.Error("Lock on a link")
	}
	if _, err := d.Sub("missing", false); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Sub on a missing folder: %v", err)
	}
}

// A handle keeps acting in the folder that was opened when that folder is swapped for a link.
func TestHandleSurvivesSwapForLink(t *testing.T) {
	p, d := tempRoot(t)
	a, err := d.Sub("a", true)
	must(t, err)
	defer a.Close()
	must(t, os.WriteFile(filepath.Join(p, "a", "x.md"), []byte("original"), 0o600))
	must(t, os.Mkdir(filepath.Join(p, "elsewhere"), 0o700))
	must(t, os.WriteFile(filepath.Join(p, "elsewhere", "x.md"), []byte("elsewhere"), 0o600))
	must(t, os.Rename(filepath.Join(p, "a"), filepath.Join(p, "a-moved")))
	must(t, os.Symlink(filepath.Join(p, "elsewhere"), filepath.Join(p, "a")))
	data, _, err := a.Read("x.md")
	if err != nil || string(data) != "original" {
		t.Fatalf("read through the old handle: %q %v", data, err)
	}
	name, w, err := a.CreateTemp()
	must(t, err)
	must(t, Native().Flush(w))
	must(t, a.LinkNew(name, "new.md"))
	if _, err := os.Stat(filepath.Join(p, "a-moved", "new.md")); err != nil {
		t.Fatal("the write did not land in the folder that was opened")
	}
	if _, err := os.Stat(filepath.Join(p, "elsewhere", "new.md")); err == nil {
		t.Fatal("the write followed the link")
	}
}

func TestFIFONeverStalls(t *testing.T) {
	p, d := tempRoot(t)
	must(t, unix.Mkfifo(filepath.Join(p, "pipe.md"), 0o600))
	done := make(chan error, 2)
	go func() { _, _, err := d.Read("pipe.md"); done <- err }()
	go func() { _, err := Native().OpenRegular(d, "pipe.md"); done <- err }()
	for range 2 {
		select {
		case err := <-done:
			if !errors.Is(err, ErrUnsafe) {
				t.Fatalf("a FIFO: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("opening a FIFO stalled")
		}
	}
}

func TestReadCeiling(t *testing.T) {
	p, d := tempRoot(t)
	f, err := os.Create(filepath.Join(p, "big.md"))
	must(t, err)
	must(t, f.Truncate(ReadCeiling+1))
	f.Close()
	if _, _, err := d.Read("big.md"); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("over the ceiling: %v", err)
	}
	must(t, os.Truncate(filepath.Join(p, "big.md"), ReadCeiling))
	data, info, err := d.Read("big.md")
	if err != nil || len(data) != ReadCeiling || info.Size != ReadCeiling {
		t.Fatalf("at the ceiling: %d %v", len(data), err)
	}
}

func TestCreateTempLinkNewReplace(t *testing.T) {
	p, d := tempRoot(t)
	publish := func(content string) (string, error) {
		name, w, err := d.CreateTemp()
		if err != nil {
			return "", err
		}
		if _, err := io.WriteString(w, content); err != nil {
			return "", err
		}
		if err := Native().Flush(w); err != nil {
			return "", err
		}
		return name, nil
	}
	tmp, err := publish("first")
	must(t, err)
	if !strings.HasPrefix(tmp, ".") || strings.HasSuffix(tmp, ".md") {
		t.Fatalf("temporary name %q", tmp)
	}
	info, err := Native().Stat(d, tmp)
	if err != nil || info.Mode.Perm() != 0o600 {
		t.Fatalf("temporary file mode %v %v", info.Mode, err)
	}
	must(t, d.LinkNew(tmp, "item.md"))
	must(t, d.Remove(tmp))
	tmp2, err := publish("second")
	must(t, err)
	if err := d.LinkNew(tmp2, "item.md"); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("LinkNew over an existing name: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(p, "item.md")); string(data) != "first" {
		t.Fatalf("the existing item changed: %q", data)
	}
	must(t, d.Replace(tmp2, "item.md"))
	if data, _ := os.ReadFile(filepath.Join(p, "item.md")); string(data) != "second" {
		t.Fatalf("Replace: %q", data)
	}
	must(t, d.Sync())
}

// The exclusive rename that stands in for a hard link refuses an existing target too.
func TestRenameNoReplace(t *testing.T) {
	p, d := tempRoot(t)
	fd := d.(*dir).fd
	must(t, os.WriteFile(filepath.Join(p, "a"), []byte("a"), 0o600))
	must(t, os.WriteFile(filepath.Join(p, "b"), []byte("b"), 0o600))
	if err := renameNoReplace(fd, "a", "b"); !errors.Is(err, unix.EEXIST) {
		t.Fatalf("over an existing target: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(p, "b")); string(data) != "b" {
		t.Fatal("the target changed")
	}
	must(t, renameNoReplace(fd, "a", "c"))
	if _, err := os.Stat(filepath.Join(p, "a")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the source stayed")
	}
}

func TestOpenRootCreatesOwnerOnly(t *testing.T) {
	p, _ := tempRoot(t)
	target := filepath.Join(p, "x", "y", "z")
	if _, err := Native().OpenRoot(target, false); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing without create: %v", err)
	}
	d, err := Native().OpenRoot(target, true)
	must(t, err)
	defer d.Close()
	for _, q := range []string{"x", "x/y", "x/y/z"} {
		st, err := os.Stat(filepath.Join(p, q))
		if err != nil || st.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s: %v %v", q, st.Mode(), err)
		}
	}
	if _, err := Native().OpenRoot("relative/path", false); err == nil {
		t.Fatal("a relative root was opened")
	}
}

func TestEntriesAndInfo(t *testing.T) {
	p, d := tempRoot(t)
	must(t, os.WriteFile(filepath.Join(p, "b.md"), []byte("bb"), 0o600))
	must(t, os.Mkdir(filepath.Join(p, "a"), 0o700))
	must(t, os.Symlink("b.md", filepath.Join(p, "c")))
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			es, err := Native().Entries(d)
			if err != nil || len(es) != 3 || es[0].Name != "a" || !es[0].Info.Mode.IsDir() ||
				es[1].Info.Size != 2 || es[2].Info.Mode&fs.ModeSymlink == 0 {
				t.Errorf("entries: %+v %v", es, err)
			}
		}()
	}
	wg.Wait()
	info, err := Native().DirInfo(d)
	if err != nil || !info.Mode.IsDir() || info.Identity.Inode != inode(t, p) {
		t.Fatalf("DirInfo: %+v %v", info, err)
	}
}

func TestResolve(t *testing.T) {
	p, _ := tempRoot(t)
	must(t, os.MkdirAll(filepath.Join(p, "real", "sub"), 0o700))
	must(t, os.Symlink(filepath.Join(p, "real"), filepath.Join(p, "alias")))
	r, err := Native().Resolve(filepath.Join(p, "alias", "sub"), false)
	if err != nil || r.Path != filepath.Join(p, "real", "sub") || !r.Exists || !r.Info.Mode.IsDir() {
		t.Fatalf("existing: %+v %v", r, err)
	}
	if last := r.Ancestors[len(r.Ancestors)-1]; last.Inode != inode(t, filepath.Join(p, "real")) {
		t.Fatalf("the last ancestor is not the parent")
	}
	if r.Ancestors[0].Inode != inode(t, "/") {
		t.Fatal("the first ancestor is not the root")
	}
	r, err = Native().Resolve(filepath.Join(p, "alias", "new", "deeper"), true)
	if err != nil || r.Path != filepath.Join(p, "real", "new", "deeper") || r.Exists {
		t.Fatalf("missing tail: %+v %v", r, err)
	}
	if _, err := Native().Resolve(filepath.Join(p, "alias", "nope"), false); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing without allowMissing: %v", err)
	}
}

// Lock follows only its context: a second holder in the same process waits until the context ends,
// and gets the lock once the first is closed.
func TestLockFollowsContext(t *testing.T) {
	_, d := tempRoot(t)
	first, err := Native().Lock(context.Background(), d, "lock")
	must(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := Native().Lock(ctx, d, "lock"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a held lock: %v", err)
	}
	if time.Since(start) < 140*time.Millisecond {
		t.Fatal("Lock gave up before its context ended")
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if _, err := Native().Lock(ctx2, d, "lock"); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled context: %v", err)
	}
	must(t, first.Close())
	second, err := Native().Lock(context.Background(), d, "lock")
	must(t, err)
	must(t, second.Close())
	must(t, second.Close()) // closing twice is harmless
}

// helper runs this test binary as another process doing one sys operation.
func helper(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	cmd.Env = append(os.Environ(), "PBRAIN_SYS_HELPER="+strings.Join(args, "\x1f"))
	return cmd
}

// TestHelperProcess is the other process of the cross-process tests; it does nothing on its own.
func TestHelperProcess(t *testing.T) {
	spec := os.Getenv("PBRAIN_SYS_HELPER")
	if spec == "" {
		return
	}
	args := strings.Split(spec, "\x1f")
	d, err := OpenDir(args[1])
	if err != nil {
		fmt.Println("error", err)
		os.Exit(2)
	}
	switch args[0] {
	case "lock":
		if _, err := Native().Lock(context.Background(), d, "lock"); err != nil {
			fmt.Println("error", err)
			os.Exit(2)
		}
		fmt.Println("locked")
		time.Sleep(time.Minute) // held until the parent kills this process
	case "publish":
		name, w, err := d.CreateTemp()
		if err == nil {
			_, err = io.WriteString(w, args[2])
		}
		if err == nil {
			err = Native().Flush(w)
		}
		if err == nil {
			err = d.LinkNew(name, "race.md")
			d.Remove(name)
		}
		switch {
		case err == nil:
			fmt.Println("ok")
		case errors.Is(err, fs.ErrExist):
			fmt.Println("exist")
		default:
			fmt.Println("error", err)
		}
	}
	os.Exit(0)
}

func TestLockAcrossProcesses(t *testing.T) {
	p, d := tempRoot(t)
	cmd := helper(t, "lock", p)
	out, err := cmd.StdoutPipe()
	must(t, err)
	must(t, cmd.Start())
	defer cmd.Process.Kill()
	line, _ := bufio.NewReader(out).ReadString('\n')
	if line != "locked\n" {
		t.Fatalf("helper: %q", line)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := Native().Lock(ctx, d, "lock"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("the other process's lock was not seen: %v", err)
	}
	must(t, cmd.Process.Kill())
	cmd.Wait()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	l, err := Native().Lock(ctx2, d, "lock")
	if err != nil {
		t.Fatalf("a killed holder kept the lock: %v", err)
	}
	l.Close()
}

// Several processes publish the same name at once: exactly one wins and its bytes stay.
func TestExclusivePublishAcrossProcesses(t *testing.T) {
	p, _ := tempRoot(t)
	const n = 6
	cmds := make([]*exec.Cmd, n)
	outs := make([]strings.Builder, n)
	for i := range n {
		cmds[i] = helper(t, "publish", p, fmt.Sprintf("content %d", i))
		cmds[i].Stdout = &outs[i]
	}
	for _, c := range cmds {
		must(t, c.Start())
	}
	wins := -1
	for i, c := range cmds {
		c.Wait()
		switch strings.TrimSpace(outs[i].String()) {
		case "ok":
			if wins >= 0 {
				t.Fatal("two processes published the same name")
			}
			wins = i
		case "exist":
		default:
			t.Fatalf("process %d: %q", i, outs[i].String())
		}
	}
	if wins < 0 {
		t.Fatal("no process published")
	}
	if data, _ := os.ReadFile(filepath.Join(p, "race.md")); string(data) != fmt.Sprintf("content %d", wins) {
		t.Fatalf("the published file is not the winner's: %q", data)
	}
	es, _ := os.ReadDir(p)
	if len(es) != 1 {
		t.Fatalf("temporary files left: %v", es)
	}
}

// inode is the inode number of path, through the same package as the code under test.
func inode(t *testing.T, path string) uint64 {
	t.Helper()
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		t.Fatal(err)
	}
	return uint64(st.Ino)
}

// A lock file that already exists with group or other permissions is made owner-only before it is
// used, and a lock file that is not a regular file is refused.
func TestLockTightensPermissiveFile(t *testing.T) {
	p, d := tempRoot(t)
	lockPath := filepath.Join(p, "lock")
	must(t, os.WriteFile(lockPath, nil, 0o600))
	must(t, os.Chmod(lockPath, 0o666))
	l, err := Native().Lock(context.Background(), d, "lock")
	must(t, err)
	defer l.Close()
	st, err := os.Stat(lockPath)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("lock file mode %v after Lock, want 0600 (%v)", st.Mode().Perm(), err)
	}
	must(t, unix.Mkfifo(filepath.Join(p, "fifo"), 0o600))
	if _, err := Native().Lock(context.Background(), d, "fifo"); !errors.Is(err, ErrUnsafe) {
		t.Fatalf("a FIFO as the lock file: %v", err)
	}
	// A new lock file is created owner-only.
	l2, err := Native().Lock(context.Background(), d, "fresh")
	must(t, err)
	defer l2.Close()
	if st, _ := os.Stat(filepath.Join(p, "fresh")); st.Mode().Perm() != 0o600 {
		t.Fatalf("new lock file mode %v", st.Mode().Perm())
	}
}
