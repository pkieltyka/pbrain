package embed_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/embed/embedtest"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type folder struct {
	path string
	dir  sys.Dir
}

func open(t *testing.T, path string) folder {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	d, err := sys.OpenDir(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return folder{path, d}
}

func (f folder) names(t *testing.T) []string {
	t.Helper()
	es, err := os.ReadDir(f.path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

// place writes every file of m into path, from the kit's pinned bytes.
func place(t *testing.T, k *embedtest.Kit, m embed.Model, path string) {
	t.Helper()
	os.MkdirAll(path, 0o755)
	for _, f := range m.Files {
		if err := os.WriteFile(filepath.Join(path, f.Name), k.Bytes[f.URL], 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func code(err error) string {
	var e *errs.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func TestFolderName(t *testing.T) {
	k := embedtest.New(nil)
	a := k.Models[embedtest.ModelA]
	name := embed.FolderName(a)
	if !strings.HasPrefix(name, "test-model-") || len(name) != len("test-model-")+16 || name != embed.FolderName(a) {
		t.Fatalf("FolderName = %q", name)
	}
	// Two models with one name and different files use two folders.
	b := a
	b.Files = slices.Clone(a.Files)
	b.Files[0].SHA256[0] ^= 1
	if embed.FolderName(b) == name {
		t.Fatal("another digest gave the same folder")
	}
	// The URL and the size are not part of it; the file names and their split are.
	c := a
	c.Files = []embed.File{{Name: "ab", SHA256: a.Files[0].SHA256}, {Name: "c"}}
	d := a
	d.Files = []embed.File{{Name: "a", SHA256: a.Files[0].SHA256}, {Name: "bc"}}
	if embed.FolderName(c) == embed.FolderName(d) {
		t.Fatal("two tables that differ in their names hash alike")
	}
	// Each name is framed by its length: a name that swallows the next digest is another table.
	var letters [32]byte
	copy(letters[:], strings.Repeat("A", 32))
	split := a
	split.Files = []embed.File{{Name: "a", SHA256: letters}, {Name: "b", SHA256: a.Files[0].SHA256}}
	joined := a
	joined.Files = []embed.File{{Name: "a" + strings.Repeat("A", 32) + "b", SHA256: a.Files[0].SHA256}}
	if embed.FolderName(split) == embed.FolderName(joined) {
		t.Fatal("a name that swallows the next digest hashes alike")
	}
	e := a
	e.Files = slices.Clone(a.Files)
	e.Files[0].URL, e.Files[0].Size = "https://elsewhere.test/x", 1
	if embed.FolderName(e) != name {
		t.Fatal("the URL or size changed the folder")
	}
}

func TestRegistry(t *testing.T) {
	ctx := context.Background()
	k := embedtest.New(nil)
	if got := k.Registry.Names(); !slices.Equal(got, []string{"test-model", "test-model-b", "test-model-packed", "test-model-pair"}) {
		t.Fatalf("Names = %q", got)
	}
	if m, ok := k.Registry.Lookup(embedtest.ModelB); !ok || m.Fingerprint == k.Models[embedtest.ModelA].Fingerprint || m.Dim != 8 || m.Cutoff != 0.5 {
		t.Fatalf("Lookup: %+v", m)
	}
	if _, ok := k.Registry.Lookup("nope"); ok {
		t.Fatal("an unknown name was found")
	}
	a := k.Models[embedtest.ModelA]
	files := func() [][]byte { return [][]byte{slices.Clone(k.Bytes[a.Files[0].URL])} }

	art := &embed.Artifact{Model: a, Files: files()}
	e, err := k.Registry.Open(ctx, art)
	if err != nil || art.Files != nil {
		t.Fatalf("Open: %v, files kept: %v", err, art.Files != nil)
	}
	vs, err := e.Embed(ctx, embed.Document, []string{"Alpha and beta", "nothing"})
	if err != nil || len(vs) != 2 || vs[0][0] != vs[0][1] || vs[0][0] < 0.7 || vs[1][7] != 1 {
		t.Fatalf("Embed: %v %v", vs, err)
	}
	e.Close()
	e.Close()
	if k.Opens.Load() != 1 || k.Closes.Load() != 1 || k.Calls.Load() != 1 || k.Texts.Load() != 2 {
		t.Fatalf("counts %d %d %d %d", k.Opens.Load(), k.Closes.Load(), k.Calls.Load(), k.Texts.Load())
	}

	// Each failure still takes the bytes over.
	other := a
	other.Fingerprint = strings.Repeat("0", 64)
	unknown := a
	unknown.Name = "nope"
	changed := files()
	changed[0][len(changed[0])-2] ^= 1
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	for name, c := range map[string]struct {
		ctx context.Context
		a   *embed.Artifact
	}{
		"another fingerprint": {ctx, &embed.Artifact{Model: other, Files: files()}},
		"unknown model":       {ctx, &embed.Artifact{Model: unknown, Files: files()}},
		"two files for one":   {ctx, &embed.Artifact{Model: a, Files: append(files(), nil)}},
		"bytes not pinned":    {ctx, &embed.Artifact{Model: a, Files: changed}},
		"opener fails":        {cancelled, &embed.Artifact{Model: a, Files: files()}},
	} {
		if _, err := k.Registry.Open(c.ctx, c.a); err == nil || c.a.Files != nil {
			t.Errorf("%s: %v, files kept: %v", name, err, c.a.Files != nil)
		}
	}
	if _, err := k.Registry.Open(ctx, nil); err == nil {
		t.Error("a nil artifact was opened")
	}
	if _, err := k.Registry.Derive(a, nil); err == nil {
		t.Error("a model without Source was derived")
	}
	defer func() {
		if recover() == nil {
			t.Error("two entries with one name were accepted")
		}
	}()
	embed.NewRegistry(embed.Entry{Model: a}, embed.Entry{Model: a})
}

func TestModelFindVerifyDownload(t *testing.T) {
	ctx := context.Background()
	k := embedtest.New(nil)
	a := k.Models[embedtest.ModelA]
	root := t.TempDir()
	cache := open(t, filepath.Join(root, "cache", "models"))

	// Nothing yet: absent, with or without a cache folder.
	if _, err := embed.Find(sys.Native(), a, nil, cache.dir); !errors.Is(err, embed.ErrAbsent) {
		t.Fatalf("empty cache: %v", err)
	}
	if _, err := embed.Find(sys.Native(), a, nil, nil); !errors.Is(err, embed.ErrAbsent) {
		t.Fatalf("no cache: %v", err)
	}

	// A download into the cache, found and verified.
	dl := k.Downloader()
	dir, err := embed.Download(ctx, sys.Native(), dl, a, cache.dir)
	if err != nil {
		t.Fatal(err)
	}
	dir.Close()
	if dl.Requests() != 1 || !slices.Equal(cache.names(t), []string{embed.FolderName(a)}) {
		t.Fatalf("%d requests, cache %q", dl.Requests(), cache.names(t))
	}
	model := folder{filepath.Join(cache.path, embed.FolderName(a)), nil}
	if got := model.names(t); !slices.Equal(got, []string{"keywords.txt"}) {
		t.Fatalf("the model folder holds %q", got)
	}
	found, err := embed.Find(sys.Native(), a, nil, cache.dir)
	if err != nil {
		t.Fatal(err)
	}
	art, err := embed.Verify(sys.Native(), found, a)
	found.Close()
	if err != nil || len(art.Files) != 1 || !bytes.Equal(art.Files[0], k.Bytes[a.Files[0].URL]) || art.Model.Name != a.Name {
		t.Fatalf("Verify: %v", err)
	}
	// A second process finds the first one's file and asks for nothing.
	dir, err = embed.Download(ctx, sys.Native(), dl, a, cache.dir)
	if err != nil || dl.Requests() != 1 {
		t.Fatalf("a second download: %v, %d requests", err, dl.Requests())
	}
	dir.Close()

	// The file changed in place after Verify returned: the artifact still holds the pinned bytes.
	if err := os.WriteFile(filepath.Join(model.path, "keywords.txt"), bytes.Repeat([]byte("x"), int(a.Files[0].Size)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Registry.Open(ctx, art); err != nil {
		t.Fatalf("the verified bytes no longer open: %v", err)
	}
	found, _ = embed.Find(sys.Native(), a, nil, cache.dir)
	if _, err := embed.Verify(sys.Native(), found, a); !errors.Is(err, embed.ErrWrongModel) {
		t.Fatalf("other bytes of the right size: %v", err)
	}
	found.Close()

	// model_path: only that folder is tried, and it is returned as it is.
	mp := open(t, filepath.Join(root, "mp"))
	if _, err := embed.Find(sys.Native(), a, mp.dir, nil); !errors.Is(err, embed.ErrAbsent) {
		t.Fatalf("empty model_path: %v", err)
	}
	place(t, k, a, mp.path)
	got, err := embed.Find(sys.Native(), a, mp.dir, cache.dir)
	if err != nil || got != mp.dir {
		t.Fatalf("model_path: %v", err)
	}
	if _, err := embed.Verify(sys.Native(), mp.dir, a); err != nil {
		t.Fatal(err)
	}
	kw := filepath.Join(mp.path, "keywords.txt")
	good, _ := os.ReadFile(kw)
	for name, data := range map[string][]byte{
		"a byte longer":  append(slices.Clone(good), 'x'),
		"a byte shorter": good[:len(good)-1],
	} {
		os.WriteFile(kw, data, 0o644)
		if _, err := embed.Find(sys.Native(), a, mp.dir, nil); !errors.Is(err, embed.ErrWrongModel) {
			t.Errorf("Find, %s: %v", name, err)
		}
		if _, err := embed.Verify(sys.Native(), mp.dir, a); !errors.Is(err, embed.ErrWrongModel) {
			t.Errorf("Verify, %s: %v", name, err)
		}
	}
	// A link in the file's place is not the model, even to the right bytes.
	os.Remove(kw)
	os.WriteFile(filepath.Join(root, "elsewhere"), good, 0o644)
	os.Symlink(filepath.Join(root, "elsewhere"), kw)
	if _, err := embed.Find(sys.Native(), a, mp.dir, nil); !errors.Is(err, embed.ErrWrongModel) {
		t.Errorf("Find, a link: %v", err)
	}
	if _, err := embed.Verify(sys.Native(), mp.dir, a); !errors.Is(err, embed.ErrWrongModel) {
		t.Errorf("Verify, a link: %v", err)
	}

	// A two-file model with its companion missing is absent; whole, it verifies.
	pair := k.Models[embedtest.ModelPair]
	pp := open(t, filepath.Join(root, "pair"))
	place(t, k, pair, pp.path)
	os.Remove(filepath.Join(pp.path, "settings.txt"))
	if _, err := embed.Find(sys.Native(), pair, pp.dir, nil); !errors.Is(err, embed.ErrAbsent) {
		t.Errorf("Find, a companion missing: %v", err)
	}
	if _, err := embed.Verify(sys.Native(), pp.dir, pair); !errors.Is(err, embed.ErrAbsent) {
		t.Errorf("Verify, a companion missing: %v", err)
	}
	place(t, k, pair, pp.path)
	art, err = embed.Verify(sys.Native(), pp.dir, pair)
	if err != nil || len(art.Files) != 2 || !bytes.Equal(art.Files[1], []byte("dim 8\n")) {
		t.Fatalf("Verify, the pair: %v", err)
	}
}

// miscount writes what it is given but reports a byte less.
type miscount struct{ embed.Downloader }

func (m miscount) Download(ctx context.Context, url string, dst io.Writer, max int64, idle time.Duration) (int64, error) {
	n, err := m.Downloader.Download(ctx, url, dst, max, idle)
	return n - 1, err
}

func TestDownloadFailures(t *testing.T) {
	ctx := context.Background()
	k := embedtest.New(nil)
	a := k.Models[embedtest.ModelA]
	url := a.Files[0].URL
	good := k.Bytes[url]
	other := slices.Clone(good)
	other[len(other)-2] ^= 1
	for name, body := range map[string][]byte{
		"wrong digest":   other,
		"short body":     good[:len(good)-1],
		"body past size": append(slices.Clone(good), 'x'),
		"empty body":     {},
	} {
		cache := open(t, filepath.Join(t.TempDir(), "models"))
		dl := k.Downloader()
		dl.Serve(url, body)
		if _, err := embed.Download(ctx, sys.Native(), dl, a, cache.dir); code(err) != "fetch_failed" {
			t.Errorf("%s: %v", name, err)
		}
		if got := (folder{filepath.Join(cache.path, embed.FolderName(a)), nil}).names(t); len(got) != 0 {
			t.Errorf("%s left %q", name, got)
		}
		if _, err := embed.Find(sys.Native(), a, nil, cache.dir); !errors.Is(err, embed.ErrAbsent) {
			t.Errorf("%s: then Find gives %v", name, err)
		}
	}

	// A transfer error, a count other than the size, an io error of the destination and a cancelled
	// context.
	cache := open(t, filepath.Join(t.TempDir(), "models"))
	dl := k.Downloader()
	if _, err := embed.Download(ctx, sys.Native(), miscount{dl}, a, cache.dir); code(err) != "fetch_failed" {
		t.Errorf("a count other than the size: %v", err)
	}
	dl.Err = errors.New("connection reset")
	if _, err := embed.Download(ctx, sys.Native(), dl, a, cache.dir); code(err) != "fetch_failed" {
		t.Errorf("a transfer error: %v", err)
	}
	dl.Err = &errs.Error{Code: "io", Message: "could not store downloaded data", Exit: 1}
	if _, err := embed.Download(ctx, sys.Native(), dl, a, cache.dir); code(err) != "io" {
		t.Errorf("an io error: %v", err)
	}
	dl.Err = nil
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := embed.Download(cancelled, sys.Native(), dl, a, cache.dir); !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled download: %v", err)
	}
	if got := (folder{filepath.Join(cache.path, embed.FolderName(a)), nil}).names(t); len(got) != 0 {
		t.Errorf("the failures left %q", got)
	}

	// A name that appears while the file downloads keeps its file; Verify then judges it.
	model := filepath.Join(cache.path, embed.FolderName(a), "keywords.txt")
	planted := bytes.Repeat([]byte("p"), len(good))
	dl.Before = func(string) { os.WriteFile(model, planted, 0o644) }
	dir, err := embed.Download(ctx, sys.Native(), dl, a, cache.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	if got, _ := os.ReadFile(model); !bytes.Equal(got, planted) {
		t.Fatal("the file that appeared was replaced")
	}
	if _, err := embed.Verify(sys.Native(), dir, a); !errors.Is(err, embed.ErrWrongModel) {
		t.Fatalf("Verify of the planted file: %v", err)
	}
	if got := (folder{filepath.Dir(model), nil}).names(t); !slices.Equal(got, []string{"keywords.txt"}) {
		t.Fatalf("the model folder holds %q", got)
	}
}

// The embed half of TestDeriveDeterministic: the source is downloaded beside the pack's folder,
// Pack derives the pinned bytes every time, and nothing is written when a check fails.
func TestPack(t *testing.T) {
	ctx := context.Background()
	k := embedtest.New(nil)
	m := k.Models[embedtest.ModelPacked]
	cache := open(t, filepath.Join(t.TempDir(), "models"))
	dl := k.Downloader()
	src, err := embed.Download(ctx, sys.Native(), dl, m, cache.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	if dl.Requests(m.Source[0].URL) != 1 || dl.Requests() != 1 {
		t.Fatalf("%d requests", dl.Requests())
	}
	if got := cache.names(t); !slices.Equal(got, []string{embed.FolderName(m) + ".source"}) {
		t.Fatalf("the cache holds %q", got)
	}
	if _, err := embed.Find(sys.Native(), m, nil, cache.dir); !errors.Is(err, embed.ErrAbsent) {
		t.Fatalf("the source is not the pack: %v", err)
	}
	pack := open(t, filepath.Join(cache.path, embed.FolderName(m)))
	if err := embed.Pack(sys.Native(), k.Registry, m, src, pack.dir); err != nil {
		t.Fatal(err)
	}
	if got := pack.names(t); !slices.Equal(got, []string{"keywords.txt"}) {
		t.Fatalf("the pack holds %q", got)
	}
	found, err := embed.Find(sys.Native(), m, nil, cache.dir)
	if err != nil {
		t.Fatal(err)
	}
	art, err := embed.Verify(sys.Native(), found, m)
	found.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := k.Registry.Open(ctx, art); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(pack.path, "keywords.txt"))
	// Deriving again gives the same bytes, and the file already named is kept.
	again := open(t, filepath.Join(t.TempDir(), "again"))
	if err := embed.Pack(sys.Native(), k.Registry, m, src, again.dir); err != nil {
		t.Fatal(err)
	}
	if second, _ := os.ReadFile(filepath.Join(again.path, "keywords.txt")); !bytes.Equal(first, second) {
		t.Fatal("two derivations differ")
	}
	if err := embed.Pack(sys.Native(), k.Registry, m, src, pack.dir); err != nil {
		t.Fatal(err)
	}

	// A source with one changed byte: refused, nothing written.
	srcPath := filepath.Join(cache.path, embed.FolderName(m)+".source", "keywords.src")
	good, _ := os.ReadFile(srcPath)
	changed := slices.Clone(good)
	changed[len(changed)-2] ^= 1
	os.WriteFile(srcPath, changed, 0o644)
	empty := open(t, filepath.Join(t.TempDir(), "empty"))
	if err := embed.Pack(sys.Native(), k.Registry, m, src, empty.dir); !errors.Is(err, embed.ErrWrongModel) {
		t.Fatalf("a changed source: %v", err)
	}
	os.Remove(srcPath)
	if err := embed.Pack(sys.Native(), k.Registry, m, src, empty.dir); !errors.Is(err, embed.ErrAbsent) {
		t.Fatalf("a missing source: %v", err)
	}
	os.WriteFile(srcPath, good, 0o644)

	// A result with one changed byte, or a file too many: refused, nothing written.
	for name, derive := range map[string]func([][]byte) ([][]byte, error){
		"a changed result": func(s [][]byte) ([][]byte, error) {
			out, err := embedtest.Derive(s)
			out[0][len(out[0])-2] ^= 1
			return out, err
		},
		"a file too many": func(s [][]byte) ([][]byte, error) {
			out, err := embedtest.Derive(s)
			return append(out, []byte("x")), err
		},
		"an error": func([][]byte) ([][]byte, error) { return nil, errors.New("derive failed") },
	} {
		reg := embed.NewRegistry(embed.Entry{Model: m, Derive: derive})
		if err := embed.Pack(sys.Native(), reg, m, src, empty.dir); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if got := empty.names(t); len(got) != 0 {
		t.Fatalf("a refused pack wrote %q", got)
	}
	if err := embed.Pack(sys.Native(), k.Registry, k.Models[embedtest.ModelA], src, empty.dir); err == nil {
		t.Fatal("a model without Source was packed")
	}
}
