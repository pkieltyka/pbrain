// Adapted from aikit linalg/exp_contract_dispatch.go. See LICENSE and NOTICE in the bert directory.
package linalg

// SoftmaxRowScaledContractInto is SoftmaxRowContractInto fused with a scalar
// multiply on the input — the attention shape, where the caller would otherwise
// run its own `scores[i] *= 1/sqrt(headDim)` pass over an O(L^2) score matrix
// immediately before the softmax.
//
// The scale is applied as its own pass into dst and the contract softmax then
// runs in place, rather than folding the multiply into the kernel. That keeps
// the contract EXACTLY as softmaxContractImpl defines it — same max-subtraction,
// same pinned f64 lane-partial fold — so this stays bit-identical across
// architectures for free, which a second fused kernel would have had to re-earn.
// The extra pass is a multiply over a buffer already in cache, against an
// exponential per element.
//
// Output contract: overwrites dst; do not pre-zero. Covers dst[:len(src)]; dst may alias src (that is the in-place form), which is one more reason
// not to pre-zero it.
func SoftmaxRowScaledContractInto(dst, src []float32, scale float32) {
	if len(dst) != len(src) {
		panic("linalg: SoftmaxRowScaledContractInto length mismatch")
	}
	if len(src) == 0 {
		return
	}
	n := len(src)
	_ = src[n-1]
	_ = dst[n-1]
	m := src[0] * scale
	dst[0] = m
	i := 1
	for ; i+3 < n; i += 4 {
		v0 := src[i+0] * scale
		v1 := src[i+1] * scale
		v2 := src[i+2] * scale
		v3 := src[i+3] * scale
		dst[i+0] = v0
		dst[i+1] = v1
		dst[i+2] = v2
		dst[i+3] = v3
		if v0 > m {
			m = v0
		}
		if v1 > m {
			m = v1
		}
		if v2 > m {
			m = v2
		}
		if v3 > m {
			m = v3
		}
	}
	for ; i < n; i++ {
		v := src[i] * scale
		dst[i] = v
		if v > m {
			m = v
		}
	}
	softmaxContractWithMaxImpl(dst, dst, m)
}

// GELUContractInto writes the EXACT GELU — x·Φ(x), HF's "gelu" — elementwise
// under the contract. A different function from GELUTanhContractInto, not a
// spelling of it: the two differ by up to 4.73e-4, worst near x ≈ −2.7.
//
// Output contract: overwrites dst; do not pre-zero. Covers dst[:len(src)]; dst may alias src (that is the in-place form), which is one more reason
// not to pre-zero it.
func GELUContractInto(dst, src []float32) {
	if len(dst) != len(src) {
		panic("linalg: GELUContractInto length mismatch")
	}
	geluContractImpl(dst, src)
}

// geluScalarInto is the reference loop and the non-arm64 implementation.
func geluScalarInto(dst, src []float32) {
	for i, v := range src {
		dst[i] = geluF32Contract(v)
	}
}
