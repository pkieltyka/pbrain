package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// lockWait caps how long a write waits for its section's lock.
const lockWait = 30 * time.Second

// Tx is one write's hold on one section: its lock and the handles it opened. A command takes one.
type Tx struct {
	b       *brain.Brain
	files   sys.Files
	section item.Section
	root    sys.Dir // borrowed from the brain
	rootDir string
	data    sys.Dir // the section's .pbrain, owned
	lock    io.Closer
	closed  bool
}

// Begin takes the section's lock, waiting at most 30 seconds or the caller's deadline. A write to
// a missing default personal section creates it first; a configured one that cannot be reached is
// personal_unavailable. The caller refreshes its settings and catalog while it holds the lock.
func Begin(ctx context.Context, b *brain.Brain, section item.Section) (*Tx, error) {
	var root sys.Dir
	var err error
	if section == item.Personal {
		root, err = b.EnsurePersonal(ctx)
	} else {
		root, err = b.OpenSection(item.Shared)
	}
	if err != nil {
		var e *errs.Error
		if errors.As(err, &e) {
			return nil, err
		}
		return nil, ioError("the section cannot be opened for writing")
	}
	data, created, err := sub(b.Files(), root, ".pbrain", true)
	if err != nil {
		return nil, ioError("the section's .pbrain folder cannot be opened")
	}
	if created {
		root.Sync() //nolint:errcheck // a lost folder entry is recreated by the next write
	}
	lctx, cancel := context.WithTimeout(ctx, lockWait)
	defer cancel()
	l, err := b.Files().Lock(lctx, data, "lock")
	if err != nil {
		data.Close()
		return nil, ioError("another pbrain command holds this section's lock; try again")
	}
	tx := &Tx{b: b, files: b.Files(), section: section, root: root, data: data, lock: l, rootDir: b.Shared.Dir}
	if section == item.Personal {
		tx.rootDir = b.Personal.Dir
	}
	return tx, nil
}

// sub opens or creates the child folder name, reporting whether it created it.
func sub(files sys.Files, dir sys.Dir, name string, create bool) (sys.Dir, bool, error) {
	_, err := files.Stat(dir, name)
	missing := errors.Is(err, fs.ErrNotExist)
	h, err := dir.Sub(name, create)
	return h, missing && err == nil, err
}

// Close releases the lock and the handles the transaction owns. It never closes the brain's root.
func (tx *Tx) Close() error {
	if tx.closed {
		return nil
	}
	tx.closed = true
	err := tx.lock.Close()
	if derr := tx.data.Close(); err == nil {
		err = derr
	}
	return err
}

// kindFolder is the folder a new item of kind is written to (spec 2.1).
func kindFolder(k item.Kind) string { return k.String() + "s" }

func (tx *Tx) destination(ctx context.Context, kind item.Kind, id string) (folder, name string, err error) {
	folder, name = kindFolder(kind), id+".md"
	if strings.ContainsAny(id, "/\\\x00") || id == "" || strings.HasPrefix(id, ".") {
		return "", "", invalidErr("not a valid item id")
	}
	folderPath := filepath.Join(tx.rootDir, folder)
	finalPath := filepath.Join(folderPath, name)
	for _, p := range tx.b.Protected() {
		if folderPath == p.Path || strings.HasPrefix(folderPath, p.Path+string(filepath.Separator)) {
			return "", "", invalidErr("the destination folder is protected")
		}
	}
	info, err := tx.files.Stat(tx.root, folder)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return "", "", ioError("the destination folder cannot be checked")
	case !info.Mode.IsDir():
		return "", "", invalidErr("the destination folder is a link or not a folder")
	}
	if tx.section == item.Shared {
		ignored, err := tx.b.SharedIgnored(ctx, []brain.IgnorePath{{Path: folderPath, Directory: true}, {Path: finalPath}})
		if err != nil {
			return "", "", err
		}
		if ignored[0] || ignored[1] {
			return "", "", invalidErr("the destination is ignored by Git, so the item would never be shared")
		}
	}
	if err == nil {
		dir, err := tx.root.Sub(folder, false)
		if err != nil {
			return "", "", invalidErr("the destination folder is a link or not a folder")
		}
		defer dir.Close()
		if _, err := tx.files.Stat(dir, name); err == nil {
			return "", "", ErrNameExists
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", "", ioError("the destination cannot be checked")
		}
	}
	return folder, name, nil
}

// CheckDestination checks a new item's destination before anything is stored: the kind folder must
// be a plain folder that is not protected and, in the shared section, not ignored by Git; the name
// must be free (ErrNameExists).
func (tx *Tx) CheckDestination(ctx context.Context, kind item.Kind, id string) error {
	_, _, err := tx.destination(ctx, kind, id)
	return err
}

// stage writes data to a new temporary file in dir and flushes it.
func (tx *Tx) stage(dir sys.Dir, data []byte) (string, error) {
	tmp, w, err := dir.CreateTemp()
	if err != nil {
		return "", ioError("a temporary file cannot be created")
	}
	if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
		w.Close()
		dir.Remove(tmp) //nolint:errcheck
		return "", ioError("a temporary file cannot be written")
	}
	if err := tx.files.Flush(w); err != nil {
		dir.Remove(tmp) //nolint:errcheck
		return "", ioError("a temporary file cannot be written")
	}
	return tmp, nil
}

// settle removes a temporary file still present and syncs the folder after a commit, recording
// what failed.
func settle(dir sys.Dir, tmp string) Commit {
	var c Commit
	if tmp != "" {
		if err := dir.Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
			c.Cleanup = append(c.Cleanup, TempLeft)
		}
	}
	if err := dir.Sync(); err != nil {
		c.Cleanup = append(c.Cleanup, FolderSyncFailed)
	}
	return c
}

// Publish writes a new item: the destination is checked again under the lock, the complete file is
// staged and flushed in the kind folder, then given its final name exclusively. That link is the
// commit point; a name taken by then is ErrNameExists.
func (tx *Tx) Publish(ctx context.Context, kind item.Kind, id string, data []byte) (Commit, error) {
	folder, name, err := tx.destination(ctx, kind, id)
	if err != nil {
		return Commit{}, err
	}
	dir, created, err := sub(tx.files, tx.root, folder, true)
	if err != nil {
		return Commit{}, invalidErr("the destination folder is a link or not a folder")
	}
	defer dir.Close()
	if created {
		if err := tx.root.Sync(); err != nil {
			return Commit{}, ioError("the new folder cannot be synced")
		}
	}
	tmp, err := tx.stage(dir, data)
	if err != nil {
		return Commit{}, err
	}
	if err := dir.LinkNew(tmp, name); err != nil {
		dir.Remove(tmp) //nolint:errcheck
		if errors.Is(err, fs.ErrExist) {
			return Commit{}, ErrNameExists
		}
		return Commit{}, ioError("the item cannot be published")
	}
	return settle(dir, tmp), nil
}

// itemRoute is the route a scan read the item by: only an item of the locked section, made by Scan.
func (tx *Tx) itemRoute(it *item.Item) (route, error) {
	r, ok := recall(it)
	if !ok || it.Section != tx.section {
		return route{}, invalidErr("the item is not in the locked section")
	}
	if r.section != tx.section {
		return route{}, invalidErr("the item is not in the locked section")
	}
	return r, nil
}

// openItemDir reaches the item's folder along its scanned route, comparing every folder with the
// identity it had when listed; a link anywhere is refused by the handles themselves.
func (tx *Tx) openItemDir(it *item.Item) (sys.Dir, func(), route, error) {
	r, err := tx.itemRoute(it)
	if err != nil {
		return nil, nil, route{}, err
	}
	dir, done, err := openChain(tx.files, tx.root, r)
	if err != nil {
		return nil, nil, route{}, conflict("the item's folder changed since it was read")
	}
	return dir, done, r, nil
}

// current reads the item's file now: it must still be the file that was listed.
func (tx *Tx) current(dir sys.Dir, r route) ([]byte, item.Version, error) {
	if info, err := tx.files.Stat(dir, r.name); err == nil && !info.Mode.IsRegular() {
		return nil, item.Version{}, invalidErr("the item's file is a link or not a regular file")
	}
	data, info, err := readBounded(tx.files, dir, r.name, MaxItemBytes)
	switch {
	case errors.Is(err, ErrChanged), errors.Is(err, errTooLarge):
		return nil, item.Version{}, conflict("the item changed since it was read")
	case err != nil:
		return nil, item.Version{}, err
	}
	if info.Identity != r.id {
		return nil, item.Version{}, conflict("the item changed since it was read")
	}
	return data, versionOf(info, data), nil
}

// ReadFile reads the item's file under the lock; it must be the version the item was scanned at.
func (tx *Tx) ReadFile(it *item.Item) ([]byte, item.Version, error) {
	dir, done, r, err := tx.openItemDir(it)
	if err != nil {
		return nil, item.Version{}, err
	}
	defer done()
	data, v, err := tx.current(dir, r)
	if err != nil {
		return nil, item.Version{}, err
	}
	if v != it.Version {
		return nil, item.Version{}, conflict("the item changed since it was read")
	}
	return data, v, nil
}

// ReplaceItem writes data over the item, which must still be at version want: the new bytes are
// staged next to it, the version is checked again just before the rename, and the rename commits.
func (tx *Tx) ReplaceItem(it *item.Item, want item.Version, data []byte) (Commit, error) {
	dir, done, r, err := tx.openItemDir(it)
	if err != nil {
		return Commit{}, err
	}
	defer done()
	tmp, err := tx.stage(dir, data)
	if err != nil {
		return Commit{}, err
	}
	if _, v, err := tx.current(dir, r); err != nil || v != want {
		dir.Remove(tmp) //nolint:errcheck
		if err != nil {
			return Commit{}, err
		}
		return Commit{}, conflict("the item changed since it was read")
	}
	if err := dir.Replace(tmp, r.name); err != nil {
		dir.Remove(tmp) //nolint:errcheck
		return Commit{}, ioError("the item cannot be replaced")
	}
	return settle(dir, ""), nil
}

// RemoveItem deletes the item's file, which must still be the scanned version.
func (tx *Tx) RemoveItem(it *item.Item) (Commit, error) {
	dir, done, r, err := tx.openItemDir(it)
	if err != nil {
		return Commit{}, err
	}
	defer done()
	if _, v, err := tx.current(dir, r); err != nil || v != it.Version {
		if err != nil {
			return Commit{}, err
		}
		return Commit{}, conflict("the item changed since it was read")
	}
	if err := dir.Remove(r.name); err != nil {
		return Commit{}, ioError("the item cannot be removed")
	}
	return settle(dir, ""), nil
}

// originals opens the locked section's .pbrain/originals.
func (tx *Tx) originals(create bool) (sys.Dir, error) {
	d, created, err := sub(tx.files, tx.data, "originals", create)
	if err != nil {
		return nil, err
	}
	if created {
		if err := tx.data.Sync(); err != nil {
			d.Close()
			return nil, err
		}
	}
	return d, nil
}

// verifyOriginal checks the file name holds exactly want bytes whose SHA-256 is digest.
func (tx *Tx) verifyOriginal(dir sys.Dir, name string, digest [32]byte, want int64) error {
	f, err := tx.files.OpenRegular(dir, name)
	if err != nil {
		return ioError("the stored original cannot be read")
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil || before.Size != want {
		return ioError("the stored original does not match the input")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, want+1))
	after, serr := f.Stat()
	if err != nil || serr != nil || n != want || after.Size != before.Size || !after.ModTime.Equal(before.ModTime) {
		return ioError("the stored original does not match the input")
	}
	if !bytes.Equal(h.Sum(nil), digest[:]) {
		return ioError("the stored original does not match the input")
	}
	return nil
}

// PutOriginal stores data under its digest in the locked section's originals, or checks the copy
// already there. Any mismatch is io, before the referencing item is published.
func (tx *Tx) PutOriginal(digest [32]byte, data []byte) error {
	if len(data) > MaxInputBytes || sha256.Sum256(data) != digest {
		return ioError("the original does not match its digest")
	}
	dir, err := tx.originals(true)
	if err != nil {
		return ioError("the originals folder cannot be opened")
	}
	defer dir.Close()
	name := hex.EncodeToString(digest[:])
	info, err := tx.files.Stat(dir, name)
	switch {
	case err == nil && !info.Mode.IsRegular():
		return ioError("the stored original is not a regular file")
	case err == nil:
		return tx.verifyOriginal(dir, name, digest, int64(len(data)))
	case !errors.Is(err, fs.ErrNotExist):
		return ioError("the originals folder cannot be checked")
	}
	tmp, err := tx.stage(dir, data)
	if err != nil {
		return err
	}
	lerr := dir.LinkNew(tmp, name)
	dir.Remove(tmp) //nolint:errcheck
	if lerr != nil && !errors.Is(lerr, fs.ErrExist) {
		return ioError("the original cannot be stored")
	}
	if err := dir.Sync(); err != nil {
		return ioError("the stored original cannot be made durable")
	}
	return tx.verifyOriginal(dir, name, digest, int64(len(data)))
}

// RemoveOriginal deletes the original of digest from the locked section. The caller has checked,
// under the lock, that no other item of the section refers to it. removed is true once the file is
// gone, also when the folder sync after it fails.
func (tx *Tx) RemoveOriginal(digest [32]byte) (bool, error) {
	dir, err := tx.originals(false)
	if err != nil {
		return false, ioError("the original does not exist")
	}
	defer dir.Close()
	name := hex.EncodeToString(digest[:])
	info, err := tx.files.Stat(dir, name)
	if err != nil || !info.Mode.IsRegular() {
		return false, ioError("the original does not exist")
	}
	if err := dir.Remove(name); err != nil {
		return false, ioError("the original cannot be removed")
	}
	if err := dir.Sync(); err != nil {
		return true, ioError("the originals folder cannot be synced")
	}
	return true, nil
}

// ReplaceSettings publishes a complete personal.toml in the brain folder. It needs the shared
// lock. A missing file is created exclusively; a file created by someone else meanwhile is conflict.
func (tx *Tx) ReplaceSettings(data []byte) (Commit, error) {
	if tx.section != item.Shared {
		return Commit{}, ioError("settings are written under the shared lock")
	}
	dir := tx.root
	info, err := tx.files.Stat(dir, "personal.toml")
	exists := err == nil
	switch {
	case exists && !info.Mode.IsRegular():
		return Commit{}, &errs.Error{Code: "invalid_config", Message: "personal.toml is not a regular file", Exit: 1}
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return Commit{}, ioError("personal.toml cannot be checked")
	}
	tmp, err := tx.stage(dir, data)
	if err != nil {
		return Commit{}, err
	}
	if exists {
		err = dir.Replace(tmp, "personal.toml")
	} else {
		err = dir.LinkNew(tmp, "personal.toml")
	}
	if err != nil {
		dir.Remove(tmp) //nolint:errcheck // the staged settings must not stay beside the brain
		if errors.Is(err, fs.ErrExist) {
			return Commit{}, conflict("personal.toml was created by another command; try again")
		}
		return Commit{}, ioError("personal.toml cannot be written")
	}
	if exists {
		tmp = "" // the rename consumed it
	}
	return settle(dir, tmp), nil
}
