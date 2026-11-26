package semantic

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/item"
)

type windowTestRegistry struct {
	embed.Registry
	batches []int
	width   int
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

func (e *windowTestEmbedder) Window() int { return e.registry.width }

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
	r := &windowTestRegistry{Registry: e.reg, width: 128}
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
