//go:build arm64

// Adapted from aikit linalg/exp_neon_arm64.go. See LICENSE and NOTICE in the bert directory.
package linalg

// erfF32ContractNEON computes dst[i] = erf(src[i]) for n elements (n a multiple
// of 4), bit-identical to erfF32Contract.
//
//go:noescape
func erfF32ContractNEON(dst, src *float32, n int)
