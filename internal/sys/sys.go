// Package sys is pbrain's seam to the file system: open folder handles that never follow a link,
// bounded reads, exclusive publishing and advisory locks (architecture 10.3, components/store.md).
// Every operation names a single entry of an open folder; nothing is looked up by path again.
package sys

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"time"
)

// Identity names a file system object independently of its path.
type Identity struct{ Device, Inode uint64 }

// FileInfo is what pbrain reads of an entry, without following a final link.
type FileInfo struct {
	Identity Identity
	Mode     fs.FileMode
	Size     int64
	ModTime  time.Time
}

// Entry is one name of a folder with its information.
type Entry struct {
	Name string
	Info FileInfo
}

// ResolvedPath is a path after the native symlink resolution.
type ResolvedPath struct {
	Path      string // absolute path after symlink resolution
	Exists    bool
	Info      FileInfo   // valid when Exists
	Ancestors []Identity // directory identities, root first, parent last
}

// RegularFile is an open regular file.
type RegularFile interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
	Stat() (FileInfo, error)
}

// Dir is one open folder. Every method names a single entry of it and never follows a link.
type Dir interface {
	Sub(name string, create bool) (Dir, error)  // open a child folder; a link is an error
	Read(name string) ([]byte, FileInfo, error) // a regular file only, at most ReadCeiling bytes
	CreateTemp() (name string, w io.WriteCloser, err error)
	LinkNew(temp, name string) error // give temp a second name; fails with fs.ErrExist if name exists
	Replace(temp, name string) error // rename temp over name
	Remove(name string) error
	Sync() error
	Close() error
}

// Files opens folder handles and locks. Native is the real one; tests replace it to inject failures.
type Files interface {
	Resolve(path string, allowMissing bool) (ResolvedPath, error)
	OpenRoot(path string, create bool) (Dir, error)
	DirInfo(dir Dir) (FileInfo, error)
	Entries(dir Dir) ([]Entry, error)
	Stat(dir Dir, name string) (FileInfo, error)
	OpenRegular(dir Dir, name string) (RegularFile, error)
	Flush(w io.WriteCloser) error
	Lock(ctx context.Context, dir Dir, name string) (io.Closer, error)
	// TryLock opens the lock as Lock does and makes one attempt: ErrLocked when it is held.
	TryLock(dir Dir, name string) (io.Closer, error)
}

// Verity is an optional Files capability for managed model files. Sealing
// consumes the temp writer and measures the same inode on a read-only handle.
type Verity interface {
	MeasureVerity(RegularFile) ([32]byte, error)
	SealTempVerity(Dir, string, io.WriteCloser) ([32]byte, error)
}

// ReadCeiling is Dir.Read's hard limit. Larger trusted files, such as a model, go through
// OpenRegular and the caller's own streaming and digest checks.
const ReadCeiling = 5 << 20

var (
	// ErrUnsafe: the entry is a link, or not of the type the operation needs.
	ErrUnsafe = errors.New("sys: a link, or not a regular file or folder")
	// ErrLocked: TryLock found the lock held by another descriptor.
	ErrLocked = errors.New("lock is held")
	// ErrInvalidName: empty, "." or "..", or holding a separator or NUL.
	ErrInvalidName = errors.New("sys: not a single entry name")
	// ErrChanged: the file changed while it was read.
	ErrChanged = errors.New("sys: the file changed while it was read")
	// ErrTooLarge: the file is over ReadCeiling.
	ErrTooLarge = errors.New("sys: the file is too large")
	// ErrForeign: a Dir that this Files did not open.
	ErrForeign = errors.New("sys: a folder handle from another implementation")
	// ErrVerityUnavailable means the filesystem or kernel cannot use this profile.
	ErrVerityUnavailable = errors.New("sys: fs-verity is unavailable")
	// ErrVerityUnsealed means a supported filesystem reported no digest for the open file.
	ErrVerityUnsealed = errors.New("sys: file is not sealed with fs-verity")
	// ErrVerityProfile means a sealed file carries a different digest algorithm or size.
	ErrVerityProfile = errors.New("sys: unsupported fs-verity digest profile")
)

// Native returns the operating system's Files.
func Native() Files { return native{} }

// OpenNamed opens a folder named from outside pbrain, such as model_path or a test's temporary folder
// (architecture §10.3, store.md "Named roots"). The absolute path is resolved once with
// files.Resolve, following any link at its last component or above, and the resolved path is opened
// with files.OpenRoot, which follows none: links are refused only below the root, and a component
// swapped for a link between the two steps fails the open. It makes no check of its own.
func OpenNamed(files Files, path string, create bool) (Dir, ResolvedPath, error) {
	if !filepath.IsAbs(path) {
		return nil, ResolvedPath{}, &fs.PathError{Op: "open", Path: path, Err: errors.New("not an absolute path")}
	}
	resolved, err := files.Resolve(filepath.Clean(path), create)
	if err != nil {
		return nil, ResolvedPath{}, err
	}
	d, err := files.OpenRoot(resolved.Path, create)
	if err != nil {
		return nil, ResolvedPath{}, err
	}
	return d, resolved, nil
}

// OpenDir opens a named folder with the native Files: OpenNamed's handle alone.
func OpenDir(path string) (Dir, error) {
	d, _, err := OpenNamed(Native(), path, false)
	return d, err
}
