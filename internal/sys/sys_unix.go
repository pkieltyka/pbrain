//go:build linux || darwin

package sys

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const (
	dirFlags  = unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC
	fileFlags = unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
	lockRetry = 25 * time.Millisecond
)

type native struct{}

// dir is one open folder descriptor.
type dir struct {
	fd   int
	path string // for messages only; never used to reach the folder
	once sync.Once
	err  error
}

func validName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\x00") {
		return ErrInvalidName
	}
	return nil
}

// retry runs a system call again while it is interrupted.
func retry(f func() error) error {
	for {
		err := f()
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}

func pathErr(op, path string, err error) error {
	switch {
	case errors.Is(err, unix.ELOOP), errors.Is(err, unix.ENOTDIR) && op == "open-dir":
		return &fs.PathError{Op: op, Path: path, Err: ErrUnsafe}
	}
	return &fs.PathError{Op: op, Path: path, Err: err}
}

func infoOf(st *unix.Stat_t) FileInfo {
	mode := fs.FileMode(st.Mode & 0o777)
	switch st.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
		mode |= fs.ModeDir
	case unix.S_IFLNK:
		mode |= fs.ModeSymlink
	case unix.S_IFIFO:
		mode |= fs.ModeNamedPipe
	case unix.S_IFSOCK:
		mode |= fs.ModeSocket
	case unix.S_IFCHR:
		mode |= fs.ModeDevice | fs.ModeCharDevice
	case unix.S_IFBLK:
		mode |= fs.ModeDevice
	}
	return FileInfo{
		Identity: Identity{Device: uint64(st.Dev), Inode: uint64(st.Ino)},
		Mode:     mode,
		Size:     st.Size,
		ModTime:  time.Unix(int64(st.Mtim.Sec), int64(st.Mtim.Nsec)),
	}
}

func (d *dir) child(name string) string { return filepath.Join(d.path, name) }

func (d *dir) Sub(name string, create bool) (Dir, error) {
	if err := validName(name); err != nil {
		return nil, err
	}
	var fd int
	open := func() error {
		var err error
		fd, err = unix.Openat(d.fd, name, dirFlags, 0)
		return err
	}
	err := retry(open)
	if errors.Is(err, unix.ENOENT) && create {
		if err := retry(func() error { return unix.Mkdirat(d.fd, name, 0o700) }); err != nil && !errors.Is(err, unix.EEXIST) {
			return nil, pathErr("mkdir", d.child(name), err)
		}
		err = retry(open)
	}
	if err != nil {
		return nil, pathErr("open-dir", d.child(name), err)
	}
	return &dir{fd: fd, path: d.child(name)}, nil
}

func (d *dir) openRegular(name string) (*os.File, FileInfo, error) {
	if err := validName(name); err != nil {
		return nil, FileInfo{}, err
	}
	var fd int
	if err := retry(func() error {
		var err error
		fd, err = unix.Openat(d.fd, name, fileFlags, 0)
		return err
	}); err != nil {
		return nil, FileInfo{}, pathErr("open", d.child(name), err)
	}
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		unix.Close(fd)
		return nil, FileInfo{}, pathErr("stat", d.child(name), err)
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG {
		unix.Close(fd)
		return nil, FileInfo{}, &fs.PathError{Op: "open", Path: d.child(name), Err: ErrUnsafe}
	}
	// A regular file never blocks; clear the flag that kept a swapped-in FIFO from stalling the open.
	if err := unix.SetNonblock(fd, false); err != nil {
		unix.Close(fd)
		return nil, FileInfo{}, pathErr("open", d.child(name), err)
	}
	return os.NewFile(uintptr(fd), d.child(name)), infoOf(&st), nil
}

func (d *dir) Read(name string) ([]byte, FileInfo, error) {
	f, before, err := d.openRegular(name)
	if err != nil {
		return nil, FileInfo{}, err
	}
	defer f.Close()
	if before.Size > ReadCeiling {
		return nil, FileInfo{}, &fs.PathError{Op: "read", Path: d.child(name), Err: ErrTooLarge}
	}
	data, err := io.ReadAll(io.LimitReader(f, ReadCeiling+1))
	if err != nil {
		return nil, FileInfo{}, &fs.PathError{Op: "read", Path: d.child(name), Err: err}
	}
	if len(data) > ReadCeiling {
		return nil, FileInfo{}, &fs.PathError{Op: "read", Path: d.child(name), Err: ErrTooLarge}
	}
	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil {
		return nil, FileInfo{}, pathErr("stat", d.child(name), err)
	}
	after := infoOf(&st)
	if after.Identity != before.Identity || after.Size != before.Size || !after.ModTime.Equal(before.ModTime) || int64(len(data)) != after.Size {
		return nil, FileInfo{}, &fs.PathError{Op: "read", Path: d.child(name), Err: ErrChanged}
	}
	return data, after, nil
}

func (d *dir) CreateTemp() (string, io.WriteCloser, error) {
	for range 16 {
		var b [12]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", nil, err
		}
		name := ".pbrain-" + hex.EncodeToString(b[:]) + ".tmp"
		var fd int
		err := retry(func() error {
			var err error
			fd, err = unix.Openat(d.fd, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
			return err
		})
		if errors.Is(err, unix.EEXIST) {
			continue
		}
		if err != nil {
			return "", nil, pathErr("create", d.child(name), err)
		}
		return name, os.NewFile(uintptr(fd), d.child(name)), nil
	}
	return "", nil, &fs.PathError{Op: "create", Path: d.path, Err: fs.ErrExist}
}

func (d *dir) LinkNew(temp, name string) error {
	if err := validName(temp); err != nil {
		return err
	}
	if err := validName(name); err != nil {
		return err
	}
	err := retry(func() error { return unix.Linkat(d.fd, temp, d.fd, name, 0) })
	switch {
	case err == nil:
		return nil
	case errors.Is(err, unix.EEXIST):
		return &fs.PathError{Op: "link", Path: d.child(name), Err: fs.ErrExist}
	case errors.Is(err, unix.EPERM), errors.Is(err, unix.ENOTSUP), errors.Is(err, unix.EOPNOTSUPP),
		errors.Is(err, unix.EMLINK), errors.Is(err, unix.EXDEV), errors.Is(err, unix.ENOSYS):
		// No hard links here: an exclusive rename gives the same guarantee.
		err = retry(func() error { return renameNoReplace(d.fd, temp, name) })
		if errors.Is(err, unix.EEXIST) {
			return &fs.PathError{Op: "rename", Path: d.child(name), Err: fs.ErrExist}
		}
		if err != nil {
			return pathErr("rename", d.child(name), err)
		}
		return nil
	}
	return pathErr("link", d.child(name), err)
}

func (d *dir) Replace(temp, name string) error {
	if err := validName(temp); err != nil {
		return err
	}
	if err := validName(name); err != nil {
		return err
	}
	if err := retry(func() error { return unix.Renameat(d.fd, temp, d.fd, name) }); err != nil {
		return pathErr("rename", d.child(name), err)
	}
	return nil
}

func (d *dir) Remove(name string) error {
	if err := validName(name); err != nil {
		return err
	}
	var st unix.Stat_t
	if err := unix.Fstatat(d.fd, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return pathErr("remove", d.child(name), err)
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG {
		return &fs.PathError{Op: "remove", Path: d.child(name), Err: ErrUnsafe}
	}
	if err := retry(func() error { return unix.Unlinkat(d.fd, name, 0) }); err != nil {
		return pathErr("remove", d.child(name), err)
	}
	return nil
}

func (d *dir) Sync() error {
	if err := retry(func() error { return unix.Fsync(d.fd) }); err != nil {
		return pathErr("sync", d.path, err)
	}
	return nil
}

func (d *dir) Close() error {
	d.once.Do(func() { d.err = unix.Close(d.fd) })
	return d.err
}

func asDir(x Dir) (*dir, error) {
	d, ok := x.(*dir)
	if !ok || d == nil {
		return nil, ErrForeign
	}
	return d, nil
}

func (native) Resolve(path string, allowMissing bool) (ResolvedPath, error) {
	if !filepath.IsAbs(path) {
		return ResolvedPath{}, &fs.PathError{Op: "resolve", Path: path, Err: errors.New("not an absolute path")}
	}
	path = filepath.Clean(path)
	existing, missing := path, []string(nil)
	var resolved string
	for {
		r, err := filepath.EvalSymlinks(existing)
		if err == nil {
			resolved = r
			break
		}
		if !errors.Is(err, fs.ErrNotExist) || !allowMissing {
			return ResolvedPath{}, err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return ResolvedPath{}, err
		}
		missing = append([]string{filepath.Base(existing)}, missing...)
		existing = parent
	}
	out := ResolvedPath{Path: filepath.Join(append([]string{resolved}, missing...)...)}
	if len(missing) == 0 {
		var st unix.Stat_t
		if err := unix.Lstat(resolved, &st); err != nil {
			return ResolvedPath{}, err
		}
		out.Exists, out.Info = true, infoOf(&st)
	}
	// Identities of the folders from the root down to the deepest existing parent.
	parent := filepath.Dir(out.Path)
	var chain []string
	for p := parent; ; p = filepath.Dir(p) {
		chain = append([]string{p}, chain...)
		if p == filepath.Dir(p) {
			break
		}
	}
	for _, p := range chain {
		var st unix.Stat_t
		if err := unix.Stat(p, &st); err != nil {
			if errors.Is(err, unix.ENOENT) {
				break
			}
			return ResolvedPath{}, err
		}
		out.Ancestors = append(out.Ancestors, infoOf(&st).Identity)
	}
	return out, nil
}

func (native) OpenRoot(path string, create bool) (Dir, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, &fs.PathError{Op: "open-dir", Path: path, Err: errors.New("not a clean absolute path")}
	}
	var fd int
	if err := retry(func() error {
		var err error
		fd, err = unix.Open("/", dirFlags, 0)
		return err
	}); err != nil {
		return nil, pathErr("open-dir", "/", err)
	}
	var cur Dir = &dir{fd: fd, path: "/"}
	for name := range strings.SplitSeq(strings.TrimPrefix(path, "/"), "/") {
		if name == "" {
			continue
		}
		next, err := cur.Sub(name, create)
		cur.Close()
		if err != nil {
			return nil, err
		}
		cur = next
	}
	return cur, nil
}

func (native) DirInfo(x Dir) (FileInfo, error) {
	d, err := asDir(x)
	if err != nil {
		return FileInfo{}, err
	}
	var st unix.Stat_t
	if err := unix.Fstat(d.fd, &st); err != nil {
		return FileInfo{}, pathErr("stat", d.path, err)
	}
	return infoOf(&st), nil
}

func (native) Entries(x Dir) ([]Entry, error) {
	d, err := asDir(x)
	if err != nil {
		return nil, err
	}
	// A fresh cursor, so that folders can be listed concurrently.
	var fd int
	if err := retry(func() error {
		var err error
		fd, err = unix.Openat(d.fd, ".", dirFlags, 0)
		return err
	}); err != nil {
		return nil, pathErr("open-dir", d.path, err)
	}
	f := os.NewFile(uintptr(fd), d.path)
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: d.path, Err: err}
	}
	sort.Strings(names)
	out := make([]Entry, 0, len(names))
	for _, name := range names {
		var st unix.Stat_t
		if err := unix.Fstatat(d.fd, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			if errors.Is(err, unix.ENOENT) {
				continue // removed since the listing
			}
			return nil, pathErr("stat", d.child(name), err)
		}
		out = append(out, Entry{Name: name, Info: infoOf(&st)})
	}
	return out, nil
}

func (native) Stat(x Dir, name string) (FileInfo, error) {
	d, err := asDir(x)
	if err != nil {
		return FileInfo{}, err
	}
	if err := validName(name); err != nil {
		return FileInfo{}, err
	}
	var st unix.Stat_t
	if err := unix.Fstatat(d.fd, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return FileInfo{}, pathErr("stat", d.child(name), err)
	}
	return infoOf(&st), nil
}

type regularFile struct{ *os.File }

func (r regularFile) Stat() (FileInfo, error) {
	var st unix.Stat_t
	if err := unix.Fstat(int(r.Fd()), &st); err != nil {
		return FileInfo{}, err
	}
	return infoOf(&st), nil
}

func (native) OpenRegular(x Dir, name string) (RegularFile, error) {
	d, err := asDir(x)
	if err != nil {
		return nil, err
	}
	f, _, err := d.openRegular(name)
	if err != nil {
		return nil, err
	}
	return regularFile{f}, nil
}

func (native) Flush(w io.WriteCloser) error {
	if f, ok := w.(*os.File); ok {
		if err := f.Sync(); err != nil {
			f.Close()
			return err
		}
	}
	return w.Close()
}

type lock struct {
	fd   int
	once sync.Once
	err  error
}

func (l *lock) Close() error {
	l.once.Do(func() { l.err = unix.Close(l.fd) })
	return l.err
}

// Lock takes an exclusive advisory lock on the named owner-only file of dir. It waits, retrying every
// 25 ms, until ctx ends; it has no timeout of its own. Closing the holder releases the lock, as does
// the end of the process. The lock file stays in place.
func (native) Lock(ctx context.Context, x Dir, name string) (io.Closer, error) {
	d, err := asDir(x)
	if err != nil {
		return nil, err
	}
	if err := validName(name); err != nil {
		return nil, err
	}
	var fd int
	if err := retry(func() error {
		var err error
		fd, err = unix.Openat(d.fd, name, unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0o600)
		return err
	}); err != nil {
		return nil, pathErr("lock", d.child(name), err)
	}
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil || st.Mode&unix.S_IFMT != unix.S_IFREG {
		unix.Close(fd)
		if err == nil {
			err = ErrUnsafe
		}
		return nil, &fs.PathError{Op: "lock", Path: d.child(name), Err: err}
	}
	for {
		err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return &lock{fd: fd}, nil
		}
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if !errors.Is(err, unix.EWOULDBLOCK) {
			unix.Close(fd)
			return nil, pathErr("lock", d.child(name), err)
		}
		t := time.NewTimer(lockRetry)
		select {
		case <-ctx.Done():
			t.Stop()
			unix.Close(fd)
			return nil, fmt.Errorf("sys: waiting for %s: %w", d.child(name), ctx.Err())
		case <-t.C:
		}
	}
}
