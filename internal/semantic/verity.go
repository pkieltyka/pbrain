package semantic

import (
	"context"
	"errors"
	"io"
	"syscall"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// prepareVerity runs under the model lock, using the bytes from strict verification.
// The existing name can have outside hardlinks, so only a fresh inode is sealed.
func (c *Cache) prepareVerity(ctx context.Context, dir sys.Dir, a *embed.Artifact) error {
	v, ok := c.files.(sys.Verity)
	if !ok {
		return nil
	}
	for i, f := range a.Model.Files {
		if f.VeritySHA256 == [32]byte{} {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		r, err := c.files.OpenRegular(dir, f.Name)
		if err != nil {
			return err
		}
		sum, err := v.MeasureVerity(r)
		_ = r.Close()
		switch {
		case err == nil:
			if sum != f.VeritySHA256 {
				return embed.ErrWrongModel
			}
			continue
		case errors.Is(err, sys.ErrVerityUnavailable):
			continue
		case !errors.Is(err, sys.ErrVerityUnsealed):
			return err
		}

		supported, err := c.canSeal(dir, v)
		if err != nil {
			return err
		}
		if !supported {
			return nil
		}
		if err := installSealed(ctx, dir, v, f, a.Files[i]); err != nil {
			if errors.Is(err, sys.ErrVerityUnavailable) || errors.Is(err, syscall.EROFS) {
				continue
			}
			return err
		}
	}
	return nil
}

// canSeal probes once per destination filesystem in this invocation. ENODATA
// from measurement does not prove that enable works with the fixed profile.
func (c *Cache) canSeal(dir sys.Dir, v sys.Verity) (bool, error) {
	info, err := c.files.DirInfo(dir)
	if err != nil {
		return false, err
	}
	device := info.Identity.Device
	c.mu.Lock()
	defer c.mu.Unlock()
	if supported, ok := c.verity[device]; ok {
		return supported, nil
	}
	name, w, err := dir.CreateTemp()
	if err == nil {
		defer dir.Remove(name)
		_, err = v.SealTempVerity(dir, name, w)
	}
	if err != nil && !errors.Is(err, sys.ErrVerityUnavailable) && !errors.Is(err, syscall.EROFS) {
		return false, err
	}
	if c.verity == nil {
		c.verity = make(map[uint64]bool)
	}
	c.verity[device] = err == nil
	return err == nil, nil
}

func installSealed(ctx context.Context, dir sys.Dir, v sys.Verity, f embed.File, data []byte) error {
	name, w, err := dir.CreateTemp()
	if err != nil {
		return err
	}
	defer dir.Remove(name) // the name is gone after successful publication
	n, err := w.Write(data)
	if err != nil || n != len(data) {
		_ = w.Close()
		if err != nil {
			return err
		}
		return io.ErrShortWrite
	}
	sum, err := v.SealTempVerity(dir, name, w)
	if err != nil {
		return err
	}
	if sum != f.VeritySHA256 {
		return embed.ErrWrongModel
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := dir.Replace(name, f.Name); err != nil {
		return err
	}
	return dir.Sync()
}
