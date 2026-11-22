package semantic

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/item"
)

type buildDeviceEmbedder struct {
	embed.Embedder
	info  embed.RunInfo
	calls int
}

func (e *buildDeviceEmbedder) Info() embed.RunInfo { return e.info }

func (e *buildDeviceEmbedder) Embed(ctx context.Context, role embed.Role, texts []string) ([][]float32, error) {
	e.calls++
	if e.calls == 2 {
		e.info = embed.RunInfo{Device: "cpu", Fallback: embed.FallbackMemory}
	}
	return e.Embedder.Embed(ctx, role, texts)
}

func TestBuildDeviceReports(t *testing.T) {
	e := newEnv(t)
	section := newSection(t, item.Personal)
	for i := range 17 {
		body := fmt.Sprintf("Test body for note %d", i)
		section.put(fmt.Sprintf("notes/n%02d.md", i), body, body)
	}
	gpu := embed.RunInfo{Device: "cuda:0", Name: "Test GPU", Budget: 1 << 30}
	cpu := embed.RunInfo{Device: "cpu", Fallback: embed.FallbackMemory}
	var runtime *buildDeviceEmbedder
	opt := e.opts()
	opt.Registry = embed.NewRegistry(embed.Entry{Model: e.m(), Open: func(ctx context.Context, a *embed.Artifact, opt embed.OpenOptions) (embed.Embedder, error) {
		base, err := e.reg.Open(ctx, a, opt)
		if err != nil {
			return nil, err
		}
		runtime = &buildDeviceEmbedder{Embedder: base, info: gpu}
		return runtime, nil
	}})
	var reports []embed.RunInfo
	progress := func(p Progress) {
		if p.Run != nil {
			reports = append(reports, *p.Run)
		}
	}
	result, err := Build(t.Context(), opt, section.sectionDir(), section.scan, e.dl, nil, progress)
	if err != nil || result.PassagesEmbedded != 17 || result.Device != "cpu" {
		t.Fatalf("build = (%+v, %v)", result, err)
	}
	if runtime == nil || runtime.calls != 3 || !slices.Equal(reports, []embed.RunInfo{gpu, cpu}) {
		t.Fatalf("runtime = %+v, reports = %+v", runtime, reports)
	}
	reports = nil
	result, err = Build(t.Context(), opt, section.sectionDir(), section.scan, e.dl, nil, progress)
	if err != nil || result.PassagesEmbedded != 0 || result.Device != "" || len(reports) != 0 {
		t.Fatalf("no-op build = (%+v, %v), reports = %+v", result, err, reports)
	}
}
