// Adapted from aikit linalg/linalg.go. See LICENSE and NOTICE in the bert directory.
package linalg

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// Dot4x4 / Dot8x4 are the register-blocked micro-kernels: each computes 4 / 8
// dot products of the shared row `a` against consecutive b-rows. Each row's
// result occupies a 4-lane block of sums; the CALLER horizontal-sums the 4
// lanes of each block to get that row's dot (same contract as Dot2x8). The
// per-lane layout WITHIN a block is arch-specific and must not be relied on —
// arm64 leaves four raw partial sums, the generic/AVX2 path puts the full sum
// in lane 0 and zeros the rest; both reduce to the same value. n4 is len/4 (K
// is a multiple of 4 in the matmul hot path). Exposed for a cache-blocked
// matmul; the decoder uses MatmulBT/Dot.
//
// Output contract: overwrites sums; do not pre-zero. Every element of the fixed-size array is written.
func Dot4x4(a, b0, b1, b2, b3 *float32, n4 int, sums *[16]float32) {
	dotNEON4x4(a, b0, b1, b2, b3, n4, sums)
}

// Dot8x4 reuses the shared row `a` across 8 b-rows, so it beats Dot4x4 at
// small-to-mid K (≈1.7× at K=768). But its 8 live accumulators plus the streamed
// b-rows outgrow the register/cache budget at large K, where it regresses BELOW
// Dot4x4 — measured on amd64 AVX2 at 40.5 vs 51.4 GB/s at K=3072. A cache-blocked
// caller should tile K into ≤~768-element strips and feed Dot8x4 the strips: that
// keeps it in its fast range and is the right cache-blocking move regardless.
// aikit's encoder does exactly this (kBlock=768), so it never hits the cliff; a
// caller that hands Dot8x4 a single large-K row instead should prefer Dot4x4.
//
// Output contract: overwrites sums; do not pre-zero. Every element of the fixed-size array is written.
func Dot8x4(a, b0, b1, b2, b3, b4, b5, b6, b7 *float32, n4 int, sums *[32]float32) {
	dotNEON8x4(a, b0, b1, b2, b3, b4, b5, b6, b7, n4, sums)
}

// Dot2x8 is the MR×NR (2 a-rows × 8 b-rows) register microkernel: it computes the 16
// dot products of two shared rows a0,a1 against eight consecutive b-rows, with 16 live
// 4-lane accumulators held across the K loop. Versus Dot8x4 (1×8), each b-load feeds 2
// FMLAs instead of 1 and the accumulator count rises from 8 to 16 — addressing the 1×8
// kernel's load- and latency-binding (see BenchmarkGEMMPeakFraction). sums holds 16
// 4-lane partial-dot blocks ([a0·b0 … a0·b7, a1·b0 … a1·b7]); the caller sums each
// block's 4 lanes and adds the K%4 scalar tail. arm64 has the NEON kernel; other arches
// get a portable reduction (the encoder wires this in only on arm64, keeping amd64 on
// the AVX2 Dot8x4 path).
//
// Output contract: overwrites sums; do not pre-zero. Every element of the fixed-size array is written.
func Dot2x8(a0, a1, b0, b1, b2, b3, b4, b5, b6, b7 *float32, n4 int, sums *[64]float32) {
	dotNEON2x8(a0, a1, b0, b1, b2, b3, b4, b5, b6, b7, n4, sums)
}

// parThreshold is the MAC count (M*N*K) below which a matmul runs serially —
// under it the goroutine fork/join costs more than the parallelism saves. It is
// set high enough that the M=1 single-token decode projections (≤ ~9M MACs for
// even a fused gate+up on a 0.5B–1.5B model) stay serial: goinfer's profile
// showed those tiny per-call fan-outs spending ~70% of decode CPU in
// pthread_cond park/wake for no speedup. Prompt/prefill (large M) and the
// encoder (large M) clear it comfortably and still parallelize.
//
// SetParallelThreshold tunes it (see SetParallelThreshold).
const parThreshold = 1 << 24

// MatmulBT computes dst[M,N] = a[M,K] · b[N,K]ᵀ — the PyTorch [out,in] weight
// layout the safetensors checkpoints store, so no transpose copy is needed.
// Each output is a Dot of an a-row against a b-row; work is parallelized over
// the N output columns (always large in the transformer projections, and the
// only dimension with parallelism on the M=1 single-token decode path).
//
// M-INVARIANT: each output dst[i,j] is bit-identical regardless of M — a row
// computed alone (M=1) equals the same row computed inside a batch (M>1), via
// the one blocked-kernel reduction order (gated by TestMatmulBT_MConsistent).
// Consumers depend on this: same-model speculative decoding accepts 100% only
// if the target's batched-verify logits (M=K) match the draft's one-at-a-time
// logits (M=1), and batched-prefill must match sequential-decode. The kernel is
// not M-PARALLEL-dependent either — output columns shard 8-aligned, so the
// fan-out width is also numerically inert (TestParallelWidth_bitIdentical).
//
// (Historically small matmuls below a MAC-count threshold took a naive
// dot-per-output span; it used a different reduction order than the blocked
// kernel, so the per-output result flipped at the M=1↔M=K threshold and broke the
// invariant above. The threshold is gone: all M route through blockedFill, which
// — measured — is also faster than the naive span at small-M decode/attention
// shapes, so M-invariance costs nothing here.)
//
// Output contract: zeroes dst internally; do not pre-zero. Covers dst[:M*N]; it clears that span, then accumulates into it.
func MatmulBT(a, b, dst []float32, M, K, N int) {
	checkMatmulBT("MatmulBT", len(a), len(b), len(dst), M, K, N)
	zeroSpanF32(dst[:M*N])
	parallelCols(M*N*K, N, func(j0, j1 int) {
		blockedFill(a, b, dst, M, K, N, j0, j1, mBlockDefault, nBlockDefault, kBlockDefault)
	})
}

// parallelCols runs fn over the [0,N) output columns, split into one chunk per
// worker. Serial below parThreshold MACs (work) where the goroutine fan-out
// would cost more than it saves. Both the f32 and int8 matmuls share it.
func parallelCols(work, N int, fn func(j0, j1 int)) {
	if work < parThreshold || N < 2 {
		fn(0, N)
		return
	}
	parallelSpawnCols(N, runtime.GOMAXPROCS(0), fn)
}

// parallelSpawnCols splits [0,N) into one chunk per worker and runs fn on each
// in its own goroutine. The caller has already decided parallelism is worth it
// (so the fn closure's heap escape here is paid only on the parallel path —
// callers that want a zero-alloc serial path call their span function directly
// when below threshold, rather than routing a closure through parallelCols).
// workers is the resolved fan-out (see resolveWidth); it is clamped to [1, N].
func parallelSpawnCols(N, workers int, fn func(j0, j1 int)) {
	if workers > N {
		workers = N
	}
	if workers < 1 {
		workers = 1
	}
	chunk := (N + workers - 1) / workers
	// Round the shard width up to a multiple of 8 (the blocked f32 kernel's column-
	// group size) so an 8-column group is never split across workers. That keeps
	// MatmulBT's per-element result independent of the fan-out width — the
	// numerically-inert width contract (TestParallelWidth_bitIdentical). Harmless for
	// the int8 matmuls, whose per-column dot doesn't depend on column grouping.
	if r := chunk % 8; r != 0 {
		chunk += 8 - r
	}

	// DYNAMIC CHUNKING (audit M-08). The workers used to take one fixed shard
	// each, so the slowest-STARTING worker set the barrier for all of them. S-02
	// measured the mechanism on decode: a start spread of 92.6 µs against a
	// 57.7 µs median shard — the goroutines do not begin together, and with a
	// static split an early-woken worker finishes its shard and then idles at the
	// barrier while a late-woken one still has its whole shard to do. Handing out
	// chunks from an atomic counter lets the early workers absorb the late ones'
	// share. Measured 1.135x at six workers when S-02 evaluated it; it was
	// recorded as "complementary rather than a substitute" for the batch form
	// that shipped in v1.34.0, and then never built.
	//
	// NUMERICALLY INERT, for the same reason the width contract is: every output
	// column is still computed in full by exactly one worker, and the chunks stay
	// 8-aligned, so no 8-column group is ever split. WHICH worker takes a chunk
	// does not enter the arithmetic.
	//
	// grain is smaller than chunk so there is something left to steal — one shard
	// per worker would reduce to the static split. 8-aligned for the contract
	// above; clamped so a tiny N does not spawn a counter per column.
	grain := chunk / 4
	if r := grain % 8; r != 0 {
		grain += 8 - r
	}
	if grain < 8 {
		grain = 8
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				j0 := int(next.Add(int64(grain))) - grain
				if j0 >= N {
					return
				}
				fn(j0, min(j0+grain, N))
			}
		}()
	}
	wg.Wait()
}
