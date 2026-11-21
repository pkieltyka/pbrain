// Package sys: exported surface from the pbrain specs; bodies are filled in by later phases.
package sys

import (
	"context"
	"io"
	"io/fs"
	"time"
)

type Identity struct{ Device, Inode uint64 }
type FileInfo struct {
	Identity Identity
	Mode     fs.FileMode
	Size     int64
	ModTime  time.Time
}
type Entry struct {
	Name string
	Info FileInfo
}
type ResolvedPath struct {
	Path      string // absolute path after symlink resolution
	Exists    bool
	Info      FileInfo   // valid when Exists
	Ancestors []Identity // directory identities, root first, parent last
}
type RegularFile interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
	Stat() (FileInfo, error)
}

type Dir interface {
	Sub(name string, create bool) (Dir, error)
	Read(name string) ([]byte, FileInfo, error)
	CreateTemp() (name string, w io.WriteCloser, err error)
	LinkNew(temp, name string) error
	Replace(temp, name string) error
	Remove(name string) error
	Sync() error
	Close() error
}

type Files interface {
	Resolve(path string, allowMissing bool) (ResolvedPath, error)
	OpenRoot(path string, create bool) (Dir, error)
	DirInfo(dir Dir) (FileInfo, error)
	Entries(dir Dir) ([]Entry, error)
	Stat(dir Dir, name string) (FileInfo, error)
	OpenRegular(dir Dir, name string) (RegularFile, error)
	Flush(w io.WriteCloser) error
	Lock(ctx context.Context, dir Dir, name string) (io.Closer, error)
}

func Native() Files                    { panic("not implemented") }
func OpenDir(path string) (Dir, error) { panic("not implemented") }
