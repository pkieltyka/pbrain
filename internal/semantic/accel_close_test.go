package semantic

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/item"
)

type accelHookRegistry struct {
	embed.Registry
	options    []embed.OpenOptions
	closeCalls atomic.Int64
	openErr    error
	docErr     error
	closeErr   error
	onEmbed    func(role embed.Role, texts []string)
	docChunks  []int
}

type accelHookEmbedder struct {
	embed.Embedder
	r *accelHookRegistry
}

func (r *accelHookRegistry) Open(ctx context.Context, a *embed.Artifact, opt embed.OpenOptions) (embed.Embedder, error) {
	r.options = append(r.options, opt)
	if r.openErr != nil {
		return nil, r.openErr
	}
	emb, err := r.Registry.Open(ctx, a, opt)
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
	return h.Embedder.Embed(ctx, role, texts)
}

func (h *accelHookEmbedder) Close() error {
	h.r.closeCalls.Add(1)
	if h.r.closeErr != nil {
		return h.r.closeErr
	}
	return h.Embedder.Close()
}

func TestBuildAcceleratorErrors(t *testing.T) {
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
	if len(hr.docChunks) != 1 || hr.docChunks[0] != 8 || len(hr.options) != 1 {
		t.Fatalf("cold 8-passage chunks=%v opens=%d, want [8] and 1", hr.docChunks, len(hr.options))
	}
	// Repeat fresh sync must not open the model at all.
	hr.options = nil
	hr.docChunks = nil
	res2 := e.mustBuild(s)
	if res2.PassagesEmbedded != 0 || len(hr.options) != 0 {
		t.Fatalf("fresh repeat sync embedded=%d opens=%d, want 0 and 0", res2.PassagesEmbedded, len(hr.options))
	}

	// A close error leaves the published index and counts in force.
	closeCause := fmt.Errorf("%w: close drain failed", embed.ErrAccelerator)
	eClose := newEnv(t)
	sClose := newSection(t, item.Shared)
	sClose.put("notes/a.md", "Title A", "Body A.")
	closeHook := &accelHookRegistry{Registry: eClose.reg, closeErr: closeCause}
	eClose.reg = closeHook
	if r, err := eClose.build(sClose); err != nil || r.Items != 1 || r.PassagesEmbedded != 1 {
		t.Fatalf("build with a close failure = (%+v, %v), want one item and passage", r, err)
	}
	if calls := closeHook.closeCalls.Load(); calls != 1 {
		t.Fatalf("embedder Close calls = %d, want 1", calls)
	}
	if db, state := sClose.open(eClose.m()); state != IndexOK {
		t.Fatalf("index after close failure state=%v, want IndexOK", state)
	} else {
		db.Close()
	}

	// 3. Primary run error + close error preserves primary run error (never masked by close error).
	embedCause := fmt.Errorf("%w: forward failed", embed.ErrAccelerator)
	eBoth := newEnv(t)
	sBoth := newSection(t, item.Shared)
	sBoth.put("notes/b.md", "Title B", "Body B.")
	eBoth.reg = &accelHookRegistry{Registry: eBoth.reg, docErr: embedCause, closeErr: closeCause}
	_, err := eBoth.build(sBoth)
	var pe *errs.Error
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

func TestSearchOpensCPU(t *testing.T) {
	e := newEnv(t)
	probes := 0
	reg := embed.NewRegistry(embed.Entry{Model: e.m(), Open: e.reg.Open, Devices: func() []embed.DeviceInfo {
		probes++
		return []embed.DeviceInfo{{ID: "cuda:0", Backend: "cuda"}}
	}})
	hook := &accelHookRegistry{Registry: reg}
	e.reg = hook
	s := newSection(t, item.Shared)
	s.put("notes/a.md", "Alpha", "alpha text")
	e.mustBuild(s)
	if !slices.Equal(hook.options, []embed.OpenOptions{{Device: "auto"}}) {
		t.Fatalf("Build opens = %v, want auto", hook.options)
	}

	opt := e.opts()
	opt.Device = "cuda:0"
	ctx := t.Context()
	for _, name := range []string{"current", "catch-up"} {
		t.Run(name, func(t *testing.T) {
			var wantDocs []int
			if name == "catch-up" {
				s.put("notes/b.md", "New alpha note", "alpha from an uncovered note")
				wantDocs = []int{1}
			}
			hook.options, hook.docChunks = nil, nil
			scores, err := e.search(ctx, opt, "alpha", s)
			if err != nil || !scores.Ready || scores.Missing != 0 {
				t.Fatalf("search scores = (%+v, %v), want all items covered", scores, err)
			}
			if !slices.Equal(hook.options, []embed.OpenOptions{{}}) {
				t.Fatalf("search opens = %v, want a single open with zero options", hook.options)
			}
			if probes != 0 {
				t.Fatalf("search probed the GPU %d times", probes)
			}
			if !slices.Equal(hook.docChunks, wantDocs) {
				t.Fatalf("document batches = %v, want %v", hook.docChunks, wantDocs)
			}
		})
	}
}
