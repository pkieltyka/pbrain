// Adapted from aikit encoder/scratch.go. See LICENSE and NOTICE in the bert directory.
package bert

import (
	"sync"
)

// scratchPool gives every forward-pass goroutine a private reusable
// scratch arena. EncodeBatch's static-partition design (M3 + M7)
// runs one forward at a time per worker, so a Get/Put pair around
// each forward keeps the scratch private to that goroutine for the
// duration of the pass. Subsequent forwards on the same worker
// (typical for warm-cache rerank workloads) reuse the same buffers,
// avoiding the per-layer / per-head allocations that M3+M7 profiles
// showed dominating GC time.
var scratchPool = sync.Pool{
	New: func() any { return &scratch{} },
}

func getScratch() *scratch { return scratchPool.Get().(*scratch) }

// ensureF32 grows b to capacity n (returning a slice of length n).
// Reuses the underlying array when n ≤ cap(b); allocates a new one
// 25% bigger than n otherwise so subsequent calls with similar
// sizes don't reallocate.
func ensureF32(b []float32, n int) []float32 {
	if cap(b) >= n {
		return b[:n]
	}
	return make([]float32, n, n+n/4)
}

// slabAlign is the float offset every slab view starts on: 1024 floats = 4 KiB. Before
// the slab each large buffer was its own make, which Go page-aligns, so this keeps the
// kernels seeing the alignment they always saw. A latency comparison across the slab
// change then measures the aliasing, not an alignment change.
const slabAlign = 1024

// slabCarver lays views end to end from offset 0, each on a slabAlign boundary.
type slabCarver struct{ end int }

// slabSpan is one view's place in the slab.
type slabSpan struct{ off, n int }

func (c *slabCarver) take(n int) slabSpan {
	off := (c.end + slabAlign - 1) &^ (slabAlign - 1)
	c.end = off + n
	return slabSpan{off, n}
}

// of cuts the view with a full slice expression, so cap == len: a write past the end
// panics on the bounds check instead of landing in the next view, which separate makes
// guaranteed for free.
func (sp slabSpan) of(slab []float32) []float32 {
	return slab[sp.off : sp.off+sp.n : sp.off+sp.n]
}

// headScratch is one attention head's private working set, for the head-parallel
// path (audit M-05). The per-forward scratch holds exactly one of each of these,
// which is all the serial loop needs; a fan-out needs one set per worker.
//
// Pooled rather than sized into scratch as [workers]x: the serial path then pays
// nothing at all, and the parallel path allocates only on its first few calls.
// scores alone is mOut*L floats — 1 MB at L=512 — so multiplying the per-forward
// arena by numCPU would cost real memory on every forward to serve the case that
// fans out.
type headScratch struct {
	qH, kH, vH, ctxHead, scores []float32
}

func (s *scratch) ensureWorkerScratch(w, mOut, headDim, L int) {
	if cap(s.workerScratch) < w {
		s.workerScratch = make([]headScratch, w)
	}
	s.workerScratch = s.workerScratch[:w]
	for i := range w {
		hs := &s.workerScratch[i]
		hs.qH = ensureF32(hs.qH, mOut*headDim)
		hs.kH = ensureF32(hs.kH, L*headDim)
		hs.vH = ensureF32(hs.vH, headDim*L)
		hs.ctxHead = ensureF32(hs.ctxHead, mOut*headDim)
		hs.scores = ensureF32(hs.scores, mOut*L)
		hs.qH, hs.kH = hs.qH[:mOut*headDim], hs.kH[:L*headDim]
		hs.vH, hs.ctxHead = hs.vH[:headDim*L], hs.ctxHead[:mOut*headDim]
		hs.scores = hs.scores[:mOut*L]
	}
}

func putScratch(s *scratch) { scratchPool.Put(s) }

type scratch struct {
	slab                        []float32
	Q, K, V                     []float32
	ctx, out                    []float32
	qH, kH, vH, ctxHead, scores []float32
	val, mid                    []float32
	workerScratch               []headScratch
}

// Reuse the same slab for attention and the MLP. The hidden state is
// allocated outside the slab, so it is the only data carried between phases.
func (s *scratch) ensureLayer(L, D, intermediate, headDim int) {
	ph := L * headDim
	var a slabCarver
	Q, K, V := a.take(L*D), a.take(L*D), a.take(L*D)
	ctx, out := a.take(L*D), a.take(L*D)
	qH, kH, vH, ctxHead := a.take(ph), a.take(ph), a.take(ph), a.take(ph)
	scores := a.take(L * L)

	var m slabCarver
	val, mid := m.take(L*intermediate), m.take(L*D)
	s.slab = ensureF32(s.slab, max(a.end, m.end))
	s.Q, s.K, s.V = Q.of(s.slab), K.of(s.slab), V.of(s.slab)
	s.ctx, s.out = ctx.of(s.slab), out.of(s.slab)
	s.qH, s.kH, s.vH, s.ctxHead = qH.of(s.slab), kH.of(s.slab), vH.of(s.slab), ctxHead.of(s.slab)
	s.scores = scores.of(s.slab)
	s.val, s.mid = val.of(s.slab), mid.of(s.slab)
}

func (s *scratch) mm(a, b, dst []float32, M, K, N int) {
	matmulBTInto(a, b, dst, M, K, N)
}
