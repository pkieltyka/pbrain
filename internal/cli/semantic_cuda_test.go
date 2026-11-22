package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
)

type cliHookRegistry struct {
	embed.Registry
	opens    atomic.Int64
	closes   atomic.Int64
	openErr  error
	queryErr error
	closeErr error
}

type cliHookEmbedder struct {
	embed.Embedder
	r *cliHookRegistry
}

func (r *cliHookRegistry) Open(ctx context.Context, a *embed.Artifact, opt embed.OpenOptions) (embed.Embedder, error) {
	r.opens.Add(1)
	if r.openErr != nil {
		return nil, r.openErr
	}
	emb, err := r.Registry.Open(ctx, a, opt)
	if err != nil {
		return nil, err
	}
	return &cliHookEmbedder{Embedder: emb, r: r}, nil
}

func (h *cliHookEmbedder) Embed(ctx context.Context, role embed.Role, texts []string) ([][]float32, error) {
	if role == embed.Query && h.r.queryErr != nil {
		return nil, h.r.queryErr
	}
	return h.Embedder.Embed(ctx, role, texts)
}

func (h *cliHookEmbedder) Close() error {
	h.r.closes.Add(1)
	_ = h.Embedder.Close()
	return h.r.closeErr
}

func TestCLISemanticCPUClosePreservesResults(t *testing.T) {
	rb, _ := semanticBrain(t)
	if status, _, errOut := rb.run("sync"); status != 0 || errOut != "" {
		t.Fatalf("initial sync: status=%d err=%q", status, errOut)
	}
	args := []string{"search", "beta", "material", "zzz", "--semantic"}
	status, want, errOut := rb.run(args...)
	if status != 0 || errOut != "" || !strings.Contains(want, "beta-b1b1  note  provisional  2026-11-02  semantic\n") {
		t.Fatalf("healthy search: status=%d out=%q err=%q", status, want, errOut)
	}
	hook := &cliHookRegistry{Registry: rb.env.Models, closeErr: errors.New("ordinary CPU close failure")}
	rb.env.Models = hook
	status, out, errOut := rb.run(args...)
	if status != 0 || out != want || errOut != "" {
		t.Fatalf("search with CPU close failure: status=%d out=%q err=%q, want status 0 and output %q", status, out, errOut, want)
	}
	if calls := hook.closes.Load(); calls != 1 {
		t.Fatalf("embedder Close calls = %d, want 1", calls)
	}
}

func TestCLISyncCPUClosePreservesResults(t *testing.T) {
	rb, _ := semanticBrain(t)
	hook := &cliHookRegistry{Registry: rb.env.Models, closeErr: errors.New("ordinary CPU close failure")}
	rb.env.Models = hook
	status, out, errOut := rb.run("sync")
	if status != 0 || errOut != "" || !strings.Contains(out, "shared: 2 items indexed") {
		t.Fatalf("sync with CPU close failure: status=%d out=%q err=%q", status, out, errOut)
	}
	if calls := hook.closes.Load(); calls != 1 {
		t.Fatalf("embedder Close calls = %d, want 1", calls)
	}
	status, out, errOut = rb.run("sync", "--status")
	if status != 0 || errOut != "" || !strings.Contains(out, "shared: 2 of 2 items indexed") {
		t.Fatalf("published index: status=%d out=%q err=%q", status, out, errOut)
	}
}

func TestCLISemanticModelFailures(t *testing.T) {
	// With no index, search returns a lexical hit with the NoIndex notice and keeps the model closed.
	rbNoIdx, _ := semanticBrain(t)
	noIdxHook := &cliHookRegistry{
		Registry: rbNoIdx.env.Models,
		openErr:  fmt.Errorf("%w: missing device", embed.ErrAccelerator),
	}
	rbNoIdx.env.Models = noIdxHook
	status, out, errOut := rbNoIdx.run("search", "alpha", "--semantic")
	if status != 0 || !strings.Contains(out, "alpha-a1a1") || !strings.Contains(errOut, "semantic_not_ready: no index for this model") {
		t.Fatalf("missing index control: status=%d out=%q err=%q", status, out, errOut)
	}
	if noIdxHook.opens.Load() != 0 {
		t.Fatalf("missing index attempted %d opens, want 0", noIdxHook.opens.Load())
	}

	rb, _ := semanticBrain(t)
	if status, _, errOut := rb.run("sync"); status != 0 || errOut != "" {
		t.Fatalf("initial sync: status=%d err=%q", status, errOut)
	}
	status, want, errOut := rb.run("search", "alpha")
	if status != 0 || errOut != "" || !strings.Contains(want, "alpha-a1a1") {
		t.Fatalf("lexical search: status=%d out=%q err=%q", status, want, errOut)
	}

	for _, tc := range []struct {
		name              string
		openErr, queryErr error
	}{
		{name: "CPU open", openErr: errors.New("ordinary CPU model error")},
		{name: "accelerator open", openErr: fmt.Errorf("open failure: %w", embed.ErrAccelerator)},
		{name: "accelerator query", queryErr: fmt.Errorf("query failure: %w", embed.ErrAccelerator)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := rb.env.Models
			rb.env.Models = &cliHookRegistry{Registry: base, openErr: tc.openErr, queryErr: tc.queryErr}
			t.Cleanup(func() { rb.env.Models = base })
			status, out, errOut := rb.run("search", "alpha", "--semantic")
			if status != 0 || out != want || !strings.Contains(errOut, "semantic_not_ready: the model could not be used") {
				t.Fatalf("unavailable model: status=%d out=%q err=%q, want lexical output %q", status, out, errOut, want)
			}
		})
	}
}
