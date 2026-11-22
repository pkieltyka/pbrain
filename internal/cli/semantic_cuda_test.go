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
	docErr   error
	closeErr error
}

type cliHookEmbedder struct {
	embed.Embedder
	r *cliHookRegistry
}

func (r *cliHookRegistry) Open(ctx context.Context, a *embed.Artifact) (embed.Embedder, error) {
	r.opens.Add(1)
	if r.openErr != nil {
		return nil, r.openErr
	}
	emb, err := r.Registry.Open(ctx, a)
	if err != nil {
		return nil, err
	}
	return &cliHookEmbedder{Embedder: emb, r: r}, nil
}

func (h *cliHookEmbedder) Embed(ctx context.Context, role embed.Role, texts []string) ([][]float32, error) {
	if role == embed.Query && h.r.queryErr != nil {
		return nil, h.r.queryErr
	}
	if role == embed.Document && h.r.docErr != nil {
		return nil, h.r.docErr
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

func TestCLISemanticAcceleratorFailLoudAndControls(t *testing.T) {
	// 1. Missing index control: search --semantic never opens model, returns lexical hit + NoIndex notice.
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

	// Build one indexed brain for the remaining search/sync failure checks.
	setupIndexed := func(t *testing.T) (*readBrain, embed.Registry) {
		t.Helper()
		rb, _ := semanticBrain(t)
		baseReg := rb.env.Models
		if st, _, eOut := rb.run("sync"); st != 0 || eOut != "" {
			t.Fatalf("initial sync failed: status=%d err=%q", st, eOut)
		}
		return rb, baseReg
	}

	// 2. Explicit accelerator attach failure (missing device) on search --semantic exits nonzero with io error.
	rbAttach, baseReg := setupIndexed(t)
	rbAttach.env.Models = &cliHookRegistry{
		Registry: baseReg,
		openErr:  fmt.Errorf("%w: attach cuda: no device", embed.ErrAccelerator),
	}
	status, out, errOut = rbAttach.run("search", "beta", "material", "zzz", "--semantic")
	if status != 1 || out != "" || !strings.HasPrefix(errOut, "io: the model cannot be opened") {
		t.Fatalf("search missing device: status=%d out=%q err=%q", status, out, errOut)
	}

	// 3. Explicit accelerator query forward failure on search --semantic exits nonzero with io error.
	rbQuery, baseReg := setupIndexed(t)
	rbQuery.env.Models = &cliHookRegistry{
		Registry: baseReg,
		queryErr: fmt.Errorf("%w: query forward failed", embed.ErrAccelerator),
	}
	status, out, errOut = rbQuery.run("search", "beta", "material", "zzz", "--semantic")
	if status != 1 || out != "" || !strings.HasPrefix(errOut, "io: the model failed to embed a query") {
		t.Fatalf("search failed query: status=%d out=%q err=%q", status, out, errOut)
	}

	// 4. Explicit accelerator catch-up passage forward failure on search --semantic exits nonzero with io error.
	rbCatchUp, baseReg := setupIndexed(t)
	rbCatchUp.put("notes/20261103-000000-gamma-c1c1.md", []string{"kind: note", "title: Third", "created: 2026-11-03T00:00:00Z", "author: agent:x"}, "gamma passage\n")
	rbCatchUp.env.Models = &cliHookRegistry{
		Registry: baseReg,
		docErr:   fmt.Errorf("%w: catch-up passage failed", embed.ErrAccelerator),
	}
	status, out, errOut = rbCatchUp.run("search", "beta", "material", "zzz", "--semantic")
	if status != 1 || out != "" || !strings.HasPrefix(errOut, "io: the model failed to embed a passage") {
		t.Fatalf("search failed catch-up: status=%d out=%q err=%q", status, out, errOut)
	}

	// 5. Close-only error during search --semantic propagates via deferred close as nonzero io error with empty stdout.
	rbSearchClose, baseReg := setupIndexed(t)
	rbSearchClose.env.Models = &cliHookRegistry{
		Registry: baseReg,
		closeErr: fmt.Errorf("%w: close drain failed", embed.ErrAccelerator),
	}
	status, out, errOut = rbSearchClose.run("search", "beta", "material", "zzz", "--semantic")
	if status != 1 || out != "" || !strings.HasPrefix(errOut, "io: the model cannot be closed") {
		t.Fatalf("search close-only error: status=%d out=%q err=%q", status, out, errOut)
	}

	// 6. Close-only error during sync propagates as nonzero io error while keeping the published index usable.
	rbSyncClose, _ := semanticBrain(t)
	baseSyncReg := rbSyncClose.env.Models
	rbSyncClose.env.Models = &cliHookRegistry{
		Registry: baseSyncReg,
		closeErr: fmt.Errorf("%w: sync close drain failed", embed.ErrAccelerator),
	}
	status, out, errOut = rbSyncClose.run("sync")
	if status != 1 || out != "" || !strings.HasPrefix(errOut, "io: the model cannot be closed") {
		t.Fatalf("sync close-only error: status=%d out=%q err=%q", status, out, errOut)
	}
	// Verify the published index from sync is still intact.
	rbSyncClose.env.Models = baseSyncReg
	status, out, errOut = rbSyncClose.run("sync", "--status")
	if status != 0 || errOut != "" || !strings.Contains(out, "shared: 2 of 2 items indexed") {
		t.Fatalf("status after sync close error: status=%d out=%q err=%q", status, out, errOut)
	}

	// 7. Control: ordinary CPU (non-accelerator) open failure during search --semantic still degrades to lexical + ModelFailed notice.
	rbCPUFail, baseReg := setupIndexed(t)
	rbCPUFail.env.Models = &cliHookRegistry{
		Registry: baseReg,
		openErr:  errors.New("ordinary cpu model error"),
	}
	status, out, errOut = rbCPUFail.run("search", "alpha", "--semantic")
	if status != 0 || !strings.Contains(out, "alpha-a1a1") || !strings.Contains(errOut, "semantic_not_ready: the model could not be used") {
		t.Fatalf("CPU ModelFailed control: status=%d out=%q err=%q", status, out, errOut)
	}
}
