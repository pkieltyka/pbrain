package semantic

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/search"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type accelHookRegistry struct {
	embed.Registry
	opens     atomic.Int64
	openErr   error
	embedErr  error
	queryErr  error
	docErr    error
	closeErr  error
	onEmbed   func(role embed.Role, texts []string)
	docChunks []int
}

type accelHookEmbedder struct {
	embed.Embedder
	r *accelHookRegistry
}

func (r *accelHookRegistry) Open(ctx context.Context, a *embed.Artifact) (embed.Embedder, error) {
	r.opens.Add(1)
	if r.openErr != nil {
		return nil, r.openErr
	}
	emb, err := r.Registry.Open(ctx, a)
	if err != nil {
		return nil, err
	}
	return &accelHookEmbedder{Embedder: emb, r: r}, nil
}

func (h *accelHookEmbedder) Embed(ctx context.Context, role embed.Role, texts []string) ([][]float32, error) {
	if h.r.onEmbed != nil {
		h.r.onEmbed(role, texts)
	}
	if role == embed.Document {
		h.r.docChunks = append(h.r.docChunks, len(texts))
		if h.r.docErr != nil {
			return nil, h.r.docErr
		}
	}
	if role == embed.Query && h.r.queryErr != nil {
		return nil, h.r.queryErr
	}
	if h.r.embedErr != nil {
		return nil, h.r.embedErr
	}
	return h.Embedder.Embed(ctx, role, texts)
}

func (h *accelHookEmbedder) Close() error {
	_ = h.Embedder.Close()
	return h.r.closeErr
}

func TestBuildAcceleratorErrorsAndClosePropagation(t *testing.T) {
	// 1. Cold 8-passage sync passes one 8-text document batch to Embed; fresh repeat sync opens nothing.
	e := newEnv(t)
	s := newSection(t, item.Shared)
	for i := range 8 {
		s.put(fmt.Sprintf("notes/n%02d.md", i), fmt.Sprintf("Title %d", i), fmt.Sprintf("Body %d.", i))
	}
	hr := &accelHookRegistry{Registry: e.reg}
	e.reg = hr
	res := e.mustBuild(s)
	if res.Items != 8 || res.PassagesEmbedded != 8 {
		t.Fatalf("cold 8-passage build = %+v", res)
	}
	if len(hr.docChunks) != 1 || hr.docChunks[0] != 8 || hr.opens.Load() != 1 {
		t.Fatalf("cold 8-passage chunks=%v opens=%d, want [8] and 1", hr.docChunks, hr.opens.Load())
	}
	// Repeat fresh sync must not open the model at all.
	hr.opens.Store(0)
	hr.docChunks = nil
	res2 := e.mustBuild(s)
	if res2.PassagesEmbedded != 0 || hr.opens.Load() != 0 {
		t.Fatalf("fresh repeat sync embedded=%d opens=%d, want 0 and 0", res2.PassagesEmbedded, hr.opens.Load())
	}

	// 2. Close-only error surfaces as typed io error preserving ErrAccelerator, while keeping the published index.
	closeCause := fmt.Errorf("%w: close drain failed", embed.ErrAccelerator)
	eClose := newEnv(t)
	sClose := newSection(t, item.Shared)
	sClose.put("notes/a.md", "Title A", "Body A.")
	eClose.reg = &accelHookRegistry{Registry: eClose.reg, closeErr: closeCause}
	_, err := eClose.build(sClose)
	var pe *errs.Error
	if !errors.As(err, &pe) || pe.Code != "io" || !errors.Is(err, embed.ErrAccelerator) {
		t.Fatalf("close-only failure = %v, want *errs.Error{io} + ErrAccelerator", err)
	}
	// The already-published index must remain readable and intact.
	if db, state := sClose.open(eClose.m()); state != IndexOK {
		t.Fatalf("index after close-only error state=%v, want IndexOK", state)
	} else {
		db.Close()
	}

	// 3. Primary run error + close error preserves primary run error (never masked by close error).
	embedCause := fmt.Errorf("%w: forward failed", embed.ErrAccelerator)
	eBoth := newEnv(t)
	sBoth := newSection(t, item.Shared)
	sBoth.put("notes/b.md", "Title B", "Body B.")
	eBoth.reg = &accelHookRegistry{Registry: eBoth.reg, docErr: embedCause, closeErr: closeCause}
	_, err = eBoth.build(sBoth)
	if !errors.As(err, &pe) || pe.Code != "io" || pe.Message != "the model failed to embed a passage" || !errors.Is(err, embedCause) {
		t.Fatalf("run+close failure = %v, want primary embed error preserved", err)
	}

	// 4. Context cancellation + close error preserves context.Canceled.
	eCancel := newEnv(t)
	sCancel := newSection(t, item.Shared)
	sCancel.put("notes/c.md", "Title C", "Body C.")
	ctx, cancel := context.WithCancel(t.Context())
	eCancel.reg = &accelHookRegistry{
		Registry: eCancel.reg,
		onEmbed:  func(embed.Role, []string) { cancel() },
		docErr:   context.Canceled,
		closeErr: closeCause,
	}
	_, err = eCancel.buildWith(ctx, eCancel.opts(), sCancel)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled run+close = %v, want context.Canceled", err)
	}

	// 5. Open accelerator error preserves typed io + ErrAccelerator + cause.
	openCause := fmt.Errorf("%w: no cuda device", embed.ErrAccelerator)
	eOpen := newEnv(t)
	sOpen := newSection(t, item.Shared)
	sOpen.put("notes/d.md", "Title D", "Body D.")
	eOpen.reg = &accelHookRegistry{Registry: eOpen.reg, openErr: openCause}
	_, err = eOpen.build(sOpen)
	if !errors.As(err, &pe) || pe.Code != "io" || !errors.Is(err, openCause) {
		t.Fatalf("open accelerator error = %v, want *errs.Error{io} + openCause", err)
	}
}

func TestSourceAcceleratorFailLoudAndCPUControls(t *testing.T) {
	setupIndexed := func(t *testing.T) (*env, *section, SectionInput) {
		t.Helper()
		e := newEnv(t)
		s := newSection(t, item.Shared)
		s.put("notes/a.md", "Alpha", "Body alpha.")
		e.mustBuild(s)
		in, err := s.scan(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		return e, s, in
	}

	// 1. Explicit accelerator open failure surfaces from Score as typed io + ErrAccelerator (no ModelFailed fallback).
	e1, s1, in1 := setupIndexed(t)
	openCause := fmt.Errorf("%w: missing cuda device", embed.ErrAccelerator)
	e1.reg = &accelHookRegistry{Registry: e1.reg, openErr: openCause}
	src1 := Start(t.Context(), e1.opts(), "query", []SectionDir{s1.sectionDir()})
	defer src1.Close()
	src1.Attach([]SectionInput{in1})
	if _, err := src1.Score(t.Context(), "query", in1.Items); !errors.Is(err, openCause) {
		t.Fatalf("Score with accelerator open error = %v, want %v", err, openCause)
	}

	// 2. Explicit accelerator query forward failure surfaces from Score as typed io + ErrAccelerator.
	e2, s2, in2 := setupIndexed(t)
	qCause := fmt.Errorf("%w: query forward boom", embed.ErrAccelerator)
	e2.reg = &accelHookRegistry{Registry: e2.reg, queryErr: qCause}
	src2 := Start(t.Context(), e2.opts(), "query", []SectionDir{s2.sectionDir()})
	defer src2.Close()
	src2.Attach([]SectionInput{in2})
	if _, err := src2.Score(t.Context(), "query", in2.Items); !errors.Is(err, qCause) {
		t.Fatalf("Score with accelerator query error = %v, want %v", err, qCause)
	}

	// 3. Explicit accelerator catch-up passage forward failure surfaces from CatchUp instead of best-effort break.
	e3, s3, _ := setupIndexed(t)
	s3.put("notes/new.md", "New note", "Uncovered passage for catch-up.")
	in3, err := s3.scan(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	cuCause := fmt.Errorf("%w: catch-up passage boom", embed.ErrAccelerator)
	e3.reg = &accelHookRegistry{Registry: e3.reg, docErr: cuCause}
	src3 := Start(t.Context(), e3.opts(), "query", []SectionDir{s3.sectionDir()})
	defer src3.Close()
	src3.Attach([]SectionInput{in3})
	if err := src3.CatchUp(t.Context()); !errors.Is(err, cuCause) {
		t.Fatalf("CatchUp with accelerator doc error = %v, want %v", err, cuCause)
	}

	// 4. Close-only error on Source surfaces from Source.Close() as typed io + ErrAccelerator.
	e4, s4, in4 := setupIndexed(t)
	closeCause := fmt.Errorf("%w: close failed", embed.ErrAccelerator)
	e4.reg = &accelHookRegistry{Registry: e4.reg, closeErr: closeCause}
	src4 := Start(t.Context(), e4.opts(), "query", []SectionDir{s4.sectionDir()})
	src4.Attach([]SectionInput{in4})
	if scores, err := src4.Score(t.Context(), "query", in4.Items); err != nil || !scores.Ready {
		t.Fatalf("Score before close error = (%+v, %v)", scores, err)
	}
	var pe *errs.Error
	if err := src4.Close(); !errors.As(err, &pe) || pe.Code != "io" || !errors.Is(err, closeCause) {
		t.Fatalf("Source.Close error = %v, want *errs.Error{io} + %v", err, closeCause)
	}

	// 5. Control: ordinary non-accelerator open failure keeps ModelFailed lexical degradation.
	e5, s5, in5 := setupIndexed(t)
	e5.reg = &accelHookRegistry{Registry: e5.reg, openErr: errors.New("ordinary cpu open failure")}
	src5 := Start(t.Context(), e5.opts(), "query", []SectionDir{s5.sectionDir()})
	defer src5.Close()
	src5.Attach([]SectionInput{in5})
	if scores, err := src5.Score(t.Context(), "query", in5.Items); err != nil || scores.Ready || scores.Reason != search.ModelFailed {
		t.Fatalf("CPU open failure Score = (%+v, %v), want nil error + ModelFailed", scores, err)
	}

	// 6. Control: missing index (with model present on disk) never attempts Registry.Open.
	e6 := newEnv(t)
	mp := filepath.Join(t.TempDir(), "model")
	if err := os.MkdirAll(mp, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range e6.m().Files {
		if err := os.WriteFile(filepath.Join(mp, f.Name), e6.kit.Bytes[f.URL], 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mdir, err := sys.OpenDir(mp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mdir.Close() })
	s6 := newSection(t, item.Shared)
	s6.put("notes/unindexed.md", "Unindexed", "Body.")
	in6, err := s6.scan(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	hr6 := &accelHookRegistry{Registry: e6.reg, openErr: openCause}
	e6.reg = hr6
	opt6 := e6.opts()
	opt6.ModelDir = mdir
	src6 := Start(t.Context(), opt6, "query", []SectionDir{s6.sectionDir()})
	defer src6.Close()
	src6.Attach([]SectionInput{in6})
	if scores, err := src6.Score(t.Context(), "query", in6.Items); err != nil || scores.Reason != search.NoIndex {
		t.Fatalf("missing index Score = (%+v, %v), want NoIndex", scores, err)
	}
	if hr6.opens.Load() != 0 {
		t.Fatalf("missing index opened model %d times, want 0", hr6.opens.Load())
	}
}
