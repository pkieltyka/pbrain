package semantic

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io/fs"
	"math"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkieltyka/pbrain/internal/gitx"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// Cache is the per-user vector cache (semantic.md, "The vector cache"), and the home of the
// downloaded models. Every folder it opens is tested for a nested repository first.
type Cache struct {
	files sys.Files
	path  string

	mu     sync.Mutex
	dirs   map[string]sys.Dir // tested folders by their path below the root; "" is the root
	verity map[uint64]bool    // fs-verity preparation probes, by destination device
}

var castagnoli = crc32.MakeTable(crc32.Castagnoli)

// OpenCache tests the folder and opens it. With create false a missing folder gives a Cache
// that holds nothing. Every folder a Cache later opens is tested again for a nested repository.
// Errors: ErrCacheExposed; git_required; a file-system error.
func OpenCache(ctx context.Context, opt CacheOptions, create bool) (*Cache, error) {
	if opt.Files == nil || !filepath.IsAbs(opt.Path) {
		return nil, fmt.Errorf("semantic: the cache needs an absolute path")
	}
	rp, err := opt.Files.Resolve(filepath.Clean(opt.Path), true)
	if err != nil {
		return nil, err
	}
	planned := rp.Path
	for _, root := range opt.Forbidden {
		if within(root, planned) {
			return nil, ErrCacheExposed
		}
	}
	start, err := nearestExisting(opt.Files, planned)
	if err != nil {
		return nil, err
	}
	inTree, err := inWorkTree(opt.Files, start)
	if err != nil {
		return nil, err
	}
	if inTree {
		if opt.Git == nil {
			return nil, ErrCacheExposed
		}
		top, err := gitx.Top(ctx, opt.Git, start)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(top, planned)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, ErrCacheExposed
		}
		p := gitx.Path{Rel: rel, Directory: true}
		ignored, err := gitx.Ignored(ctx, opt.Git, top, []gitx.Path{p})
		if err != nil {
			return nil, err
		}
		if rel == "." || !ignored[0] {
			return nil, ErrCacheExposed
		}
		tracked, err := gitx.Tracked(ctx, opt.Git, top, p)
		if err != nil {
			return nil, err
		}
		if tracked {
			return nil, ErrCacheExposed
		}
	}
	c := &Cache{files: opt.Files, path: planned, dirs: map[string]sys.Dir{}}
	if create {
		if _, err := c.dir("", true); err != nil {
			return nil, err
		}
	} else if _, err := c.dir("", false); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return c, nil
}

// Close releases the folders the cache holds open.
func (c *Cache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, d := range c.dirs {
		d.Close()
		delete(c.dirs, k)
	}
	return nil
}

// within reports whether p is root or lies under it.
func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func nearestExisting(files sys.Files, p string) (string, error) {
	for {
		r, err := files.Resolve(p, false)
		if err == nil && r.Exists {
			return r.Path, nil
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", fs.ErrNotExist
		}
		p = parent
	}
}

// inWorkTree reports whether a .git entry lies at dir or above it.
func inWorkTree(files sys.Files, dir string) (bool, error) {
	for p := dir; ; p = filepath.Dir(p) {
		d, err := files.OpenRoot(p, false)
		if err != nil {
			return false, err
		}
		_, err = files.Stat(d, ".git")
		d.Close()
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
		if filepath.Dir(p) == p {
			return false, nil
		}
	}
}

// dir opens the folder at rel below the root ("" for the root itself), each level once, and tests
// each for a .git entry before anything is read, created or written in it. A missing folder is
// fs.ErrNotExist when create is false.
func (c *Cache) dir(rel string, create bool) (sys.Dir, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dirLocked(rel, create)
}

func (c *Cache) dirLocked(rel string, create bool) (sys.Dir, error) {
	if d, ok := c.dirs[rel]; ok {
		return d, nil
	}
	var d sys.Dir
	var err error
	if rel == "" {
		d, err = c.files.OpenRoot(c.path, create)
	} else {
		parentRel, name := "", rel
		if i := strings.LastIndexByte(rel, '/'); i >= 0 {
			parentRel, name = rel[:i], rel[i+1:]
		}
		parent, perr := c.dirLocked(parentRel, create)
		if perr != nil {
			return nil, perr
		}
		d, err = parent.Sub(name, create)
	}
	if err != nil {
		return nil, err
	}
	if _, err := c.files.Stat(d, ".git"); err == nil {
		d.Close()
		return nil, ErrCacheExposed
	} else if !errors.Is(err, fs.ErrNotExist) {
		d.Close()
		return nil, err
	}
	c.dirs[rel] = d
	return d, nil
}

// exists reports whether the cache folder is there and passed its test, opening it without
// creating it. A catch-up reads and writes the cache only then, so a search never creates it.
func (c *Cache) exists() bool {
	_, err := c.dir("", false)
	return err == nil
}

// models is <cache>/models/, tested.
func (c *Cache) models(create bool) (sys.Dir, error) { return c.dir("models", create) }

// vectorDir is <cache>/vectors/<fingerprint>/<first two hex of the digest>/.
func vectorDir(fingerprint string, digest [32]byte) string {
	h := hex.EncodeToString(digest[:1])
	return "vectors/" + fingerprint + "/" + h
}

// readVector returns the cached vector of a passage, or false. A file that is not exactly dim
// values and their CRC-32C, or whose values are not a unit vector, is removed and counts as absent.
// Only ErrCacheExposed is an error.
func (c *Cache) readVector(fingerprint string, digest [32]byte, dim int) ([]float32, bool, error) {
	if !validFingerprint(fingerprint) {
		return nil, false, nil
	}
	d, err := c.dir(vectorDir(fingerprint, digest), false)
	if errors.Is(err, ErrCacheExposed) {
		return nil, false, err
	}
	if err != nil {
		return nil, false, nil
	}
	name := hex.EncodeToString(digest[:])
	data, _, err := d.Read(name)
	if err != nil {
		return nil, false, nil
	}
	v, ok := decodeVector(data, dim)
	if !ok {
		d.Remove(name) //nolint:errcheck // a bad entry is only ever absent
		return nil, false, nil
	}
	return v, true, nil
}

// writeVector stores a vector under its passage digest, unflushed; an existing name is kept.
// Errors: ErrCacheExposed, or a file-system error.
func (c *Cache) writeVector(fingerprint string, digest [32]byte, v []float32) error {
	if !validFingerprint(fingerprint) {
		return fmt.Errorf("semantic: fingerprint %q", fingerprint)
	}
	d, err := c.dir(vectorDir(fingerprint, digest), true)
	if err != nil {
		return err
	}
	tmp, w, err := d.CreateTemp()
	if err != nil {
		return err
	}
	defer d.Remove(tmp) //nolint:errcheck // after LinkNew the file keeps its name
	if _, err := w.Write(encodeVector(v)); err != nil {
		w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	if err := d.LinkNew(tmp, hex.EncodeToString(digest[:])); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return nil
}

func validFingerprint(s string) bool { return hex64.MatchString(s) }

func encodeVector(v []float32) []byte {
	b := vectorBlob(v)
	return binary.LittleEndian.AppendUint32(b, crc32.Checksum(b, castagnoli))
}

func decodeVector(data []byte, dim int) ([]float32, bool) {
	if len(data) != 4*dim+4 {
		return nil, false
	}
	body := data[:4*dim]
	if crc32.Checksum(body, castagnoli) != binary.LittleEndian.Uint32(data[4*dim:]) {
		return nil, false
	}
	v := make([]float32, dim)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(body[4*i:]))
	}
	return v, validVector(v, dim)
}

// validVector reports whether v has dim finite values and a length within 0.001 of 1.
func validVector(v []float32, dim int) bool {
	if len(v) != dim {
		return false
	}
	var n float64
	for _, f := range v {
		if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
			return false
		}
		n += float64(f) * float64(f)
	}
	return math.Abs(math.Sqrt(n)-1) <= 0.001
}
