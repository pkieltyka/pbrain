package cli

import (
	"context"
	"errors"
	"io"
	"path/filepath"

	"github.com/pkieltyka/pbrain/internal/sys"
)

type writeFaults struct {
	sys.Files
	beforeLock func() error
	beforeTemp func(path string) error
	beforeLink func(path, name string) error
	remove     func(path, name string) error
	replace    func(path, name string) error
	sync       func(path string) error
}

type faultDir struct {
	sys.Dir
	path string
	f    *writeFaults
}

var injectedFailure = errors.New("synthetic filesystem failure")

func (f *writeFaults) OpenRoot(path string, create bool) (sys.Dir, error) {
	dir, err := f.Files.OpenRoot(path, create)
	if err != nil {
		return nil, err
	}
	return &faultDir{dir, path, f}, nil
}

func (d *faultDir) Sub(name string, create bool) (sys.Dir, error) {
	child, err := d.Dir.Sub(name, create)
	if err != nil {
		return nil, err
	}
	return &faultDir{child, filepath.Join(d.path, name), d.f}, nil
}

func (d *faultDir) LinkNew(temp, name string) error {
	if d.f.beforeLink != nil {
		if err := d.f.beforeLink(d.path, name); err != nil {
			return err
		}
	}
	return d.Dir.LinkNew(temp, name)
}

func (d *faultDir) CreateTemp() (string, io.WriteCloser, error) {
	if d.f.beforeTemp != nil {
		if err := d.f.beforeTemp(d.path); err != nil {
			return "", nil, err
		}
	}
	return d.Dir.CreateTemp()
}

func (d *faultDir) Remove(name string) error {
	if d.f.remove != nil {
		if err := d.f.remove(d.path, name); err != nil {
			return err
		}
	}
	return d.Dir.Remove(name)
}

func (d *faultDir) Replace(temp, name string) error {
	if d.f.replace != nil {
		if err := d.f.replace(d.path, name); err != nil {
			return err
		}
	}
	return d.Dir.Replace(temp, name)
}

func (d *faultDir) Sync() error {
	if d.f.sync != nil {
		if err := d.f.sync(d.path); err != nil {
			return err
		}
	}
	return d.Dir.Sync()
}

func unwrapped(dir sys.Dir) sys.Dir {
	if d, ok := dir.(*faultDir); ok {
		return d.Dir
	}
	return dir
}

func (f *writeFaults) DirInfo(dir sys.Dir) (sys.FileInfo, error) {
	return f.Files.DirInfo(unwrapped(dir))
}
func (f *writeFaults) Entries(dir sys.Dir) ([]sys.Entry, error) {
	return f.Files.Entries(unwrapped(dir))
}

func (f *writeFaults) Stat(dir sys.Dir, name string) (sys.FileInfo, error) {
	return f.Files.Stat(unwrapped(dir), name)
}

func (f *writeFaults) OpenRegular(dir sys.Dir, name string) (sys.RegularFile, error) {
	return f.Files.OpenRegular(unwrapped(dir), name)
}

func (f *writeFaults) Lock(ctx context.Context, dir sys.Dir, name string) (io.Closer, error) {
	if f.beforeLock != nil {
		if err := f.beforeLock(); err != nil {
			return nil, err
		}
	}
	return f.Files.Lock(ctx, unwrapped(dir), name)
}

func (f *writeFaults) TryLock(dir sys.Dir, name string) (io.Closer, error) {
	return f.Files.TryLock(unwrapped(dir), name)
}
