package semantic

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/embed/bge"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type windowTestRegistry struct {
	embed.Registry
	batches []int
}

func (r *windowTestRegistry) Open(ctx context.Context, a *embed.Artifact, opt embed.OpenOptions) (embed.Embedder, error) {
	e, err := r.Registry.Open(ctx, a, opt)
	if err != nil {
		return nil, err
	}
	return &windowTestEmbedder{Embedder: e, registry: r}, nil
}

type windowTestEmbedder struct {
	embed.Embedder
	registry *windowTestRegistry
}

func (*windowTestEmbedder) Window() int { return 128 }

func (e *windowTestEmbedder) Embed(ctx context.Context, role embed.Role, texts []string) ([][]float32, error) {
	e.registry.batches = append(e.registry.batches, len(texts))
	return e.Embedder.Embed(ctx, role, texts)
}

func windowTestSection(t *testing.T) *section {
	t.Helper()
	s := newSection(t, item.Personal)
	for i := range 130 {
		s.put(fmt.Sprintf("notes/%03d.md", i), fmt.Sprintf("Note %d", i), "A decision about local storage.\n")
	}
	return s
}

func TestBuildUsesEmbedderWindow(t *testing.T) {
	e := newEnv(t)
	r := &windowTestRegistry{Registry: e.reg}
	e.reg = r
	s := windowTestSection(t)
	got := e.mustBuild(s)
	if got.Items != 130 || got.PassagesEmbedded != 130 || !slices.Equal(r.batches, []int{128, 2}) {
		t.Fatalf("Build = %+v, embed batches = %v", got, r.batches)
	}
	// A no-op build still verifies its model without opening the runtime.
	e.mustBuild(s)
	if e.kit.Opens.Load() != 1 {
		t.Fatalf("runtime opened %d times", e.kit.Opens.Load())
	}
}

func TestBuildWorkersSameDatabase(t *testing.T) {
	const env = "PBRAIN_TEST_BGE_MODEL"
	path := os.Getenv(env)
	if path == "" {
		t.Skipf("set %s to the verified BGE model folder", env)
	}
	modelDir, err := sys.OpenDir(path)
	if err != nil {
		t.Fatal(err)
	}
	defer modelDir.Close()
	entry := bge.Entry()
	opt := Options{
		Files: sys.Native(), Registry: embed.NewRegistry(entry), ModelName: entry.Model.Name,
		ModelDir: modelDir, Rand: rand.Reader, Now: func() time.Time { return time.Unix(1_800_000_000, 0) },
	}
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)
	want := BuildResult{Section: item.Personal, Items: 130, PassagesEmbedded: 130}
	var baseline []byte
	for _, workers := range []int{1, 4, 32} {
		runtime.GOMAXPROCS(workers)
		s := windowTestSection(t)
		got, err := Build(t.Context(), opt, s.sectionDir(), s.scan, nil, nil, nil)
		if err != nil || got != want {
			t.Fatalf("%d workers: Build = %+v, %v, want %+v", workers, got, err, want)
		}
		data, err := os.ReadFile(filepath.Join(s.index(), s.manifestDB()))
		if err != nil {
			t.Fatal(err)
		}
		if workers == 1 {
			baseline = data
		} else if !bytes.Equal(data, baseline) {
			t.Fatalf("%d workers changed published database bytes", workers)
		}
	}
}
