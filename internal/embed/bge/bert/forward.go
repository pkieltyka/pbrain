// Adapted from aikit encoder/bert.go and encoder/forward.go. See LICENSE and NOTICE in the bert directory.
package bert

import (
	"github.com/pkieltyka/pbrain/internal/embed/bge/bert/internal/linalg"
)

// forward runs the transformer. clsOnly trims the final layer to row 0 — see
// clsHiddenState.
func (b *BERT) forward(ids []int32, clsOnly bool) []float32 {
	enterForward()
	defer leaveForward()

	c := b.cfg
	// The position table bounds the usable sequence length.
	if len(ids) > b.maxSeq {
		ids = ids[:b.maxSeq]
	}
	L, D := len(ids), c.Hidden
	headDim := D / c.Heads
	eps := c.LNEps

	s := getScratch()
	defer putScratch(s)
	s.ensureLayer(L, D, c.Intermediate, headDim)

	// Word and learned-position embeddings, then the token-type row for
	// the single segment used by sentence embeddings.
	h := make([]float32, L*D)
	for i, id := range ids {
		pos := b.posEmb[i*D : (i+1)*D]
		row := h[i*D : (i+1)*D]
		w := b.wordEmb[clampTokenID(id, c.VocabSize)*D:][:D]
		for j := range D {
			row[j] = w[j] + pos[j]
		}
		if len(b.typeEmb) != 0 {
			for j := range D {
				row[j] += b.typeEmb[j]
			}
		}
	}
	layerNorm(h, b.embLNW, b.embLNB, L, D, eps)

	for li := range b.layers {
		l := &b.layers[li]

		// mOut is how many rows of this layer's OUTPUT are needed. It is L
		// everywhere except the final layer of a CLS-only forward, where only row
		// 0 is ever read. K and V stay at full L regardless: attention reads every
		// position, and trimming them would change the result rather than skip
		// unused work.
		mOut := L
		if clsOnly && li == len(b.layers)-1 {
			mOut = 1
		}

		// Self-attention (no RoPE): Q,K,V = hWᵀ + b, into scratch. Q is
		// projected only for the mOut rows actually needed; K,V stay full L
		// since attention reads every position regardless of mOut.
		Q, K, V := s.Q[:mOut*D], s.K[:L*D], s.V[:L*D]
		s.mm(h, l.Wq, Q, mOut, D, D)
		s.mm(h, l.Wk, K, L, D, D)
		s.mm(h, l.Wv, V, L, D, D)
		addBias(Q, l.Bq, mOut, D)
		addBias(K, l.Bk, L, D)
		addBias(V, l.Bv, L, D)

		// Scaled dot-product attention, output projection and residual — shared
		// with the CodeRankEmbed/nomic and GTE forwards via encoder/attention.go.
		// BERT has neither RoPE nor a packed qkv projection, so it builds Q/K/V
		// itself above and calls attentionCore directly rather than selfAttention.
		h = attentionCore(h, Q, K, V, l.Wo, l.Bo, c.Heads, headDim, D, L, mOut, s)
		layerNorm(h, l.AttnLNW, l.AttnLNB, mOut, D, eps)

		// GELU FFN: intermediate → gelu → output, residual, LayerNorm.
		inter := s.val[:mOut*c.Intermediate]
		s.mm(h, l.Wi, inter, mOut, D, c.Intermediate)
		addBias(inter, l.Bi, mOut, c.Intermediate)
		gelu(inter)
		ffn := s.mid[:mOut*D]
		s.mm(inter, l.Wd, ffn, mOut, c.Intermediate, D)
		addBias(ffn, l.Bd, mOut, D)
		for i := range h {
			h[i] += ffn[i] // residual
		}
		layerNorm(h, l.OutLNW, l.OutLNB, mOut, D, eps)
	}
	return h
}

// gelu applies the exact (erf-based) GELU in place: x·Φ(x) = 0.5x(1+erf(x/√2)).
// transformers' "gelu" activation is the erf form, not the tanh approximation.
func gelu(x []float32) {
	// Chunked so a long activation block (BERT's [L,intermediate]) splits across
	// cores; each chunk is expensive enough (math.Erf-class transcendental) that
	// this is one of the largest serial stages once the matmuls are parallel.
	// Elementwise and in place, so the split is numerically inert. Each chunk
	// routes through linalg.GELUInto (vectorized under GOEXPERIMENT=simd — T1,
	// autoresearch round 2) instead of a per-element scalar loop, so this now
	// layers SIMD-within-a-chunk under the existing core-level parallelism —
	// this was the encoder's own gap the autoresearch loop found: GELUInto had
	// real callers (vision/qwen_encoder.go) but not this one, its biggest.
	const chunk = 4096
	chunks := (len(x) + chunk - 1) / chunk
	parallelRows(chunks, len(x), func(start, end int) {
		lo, hi := start*chunk, min(end*chunk, len(x))
		linalg.GELUContractInto(x[lo:hi], x[lo:hi])
	})
}

// clampTokenID maps a token id to a valid row index into a [vocab, D] embedding
// table: id if in range, else 0 (always valid for vocab ≥ 1). The tokenizer
// should never emit an out-of-range id, but a corrupt fixture or a drop-in
// checkpoint could; substituting row 0 avoids an index-out-of-range panic in the
// embedding gather. (The old fallback to id 100 assumed a ≥101-token vocab — the
// repo's own vocab_size:4 fixtures would index WordEmb[400:404] and panic, the
// exact crash this guards — and that 100 is [UNK] for every drop-in checkpoint.)
// Shared by every embed-gather site in the package — CodeRankEmbed/nomic's
// single/batched/f32/Q8/MaxSim-probe paths here, and BERT's and GTE's own word-
// embedding gathers (bert.go, gte.go) despite their otherwise-unshared embedding
// prologues (learned positions + segments for BERT, RoPE-only for GTE) — so the
// fallback can't drift between them again.
func clampTokenID(id int32, vocab int) int {
	if int(id) < 0 || int(id) >= vocab {
		return 0
	}
	return int(id)
}
