// Adapted from aikit linalg/shapecheck.go. See LICENSE and NOTICE in the bert directory.
package linalg

import (
	"fmt"
)

// mul returns a*b for non-negative a, b, or -1 if either is negative or the
// product overflows int. The -1 sentinel is reported as an overflow by the
// checkers below.
func mul(a, b int) int {
	if a < 0 || b < 0 {
		return -1
	}
	if a == 0 || b == 0 {
		return 0
	}
	p := a * b
	if p/a != b { // overflow
		return -1
	}
	return p
}

// requireLen panics if have < need, or if need is the -1 overflow sentinel.
// For matmul operands, which only require "at least" the shape's worth.
func requireLen(kernel, operand string, have, need int) {
	if need < 0 {
		panic(fmt.Sprintf("linalg: %s dimension product overflows int (%s)", kernel, operand))
	}
	if have < need {
		panic(fmt.Sprintf("linalg: %s: %s length %d < required %d", kernel, operand, have, need))
	}
}

// checkMatmulBT validates the f32 dst[M,N] = a[M,K]·b[N,K]ᵀ contract before the
// parallel fan-out.
func checkMatmulBT(kernel string, aLen, bLen, dstLen, M, K, N int) {
	if M < 0 || K < 0 || N < 0 {
		panic(fmt.Sprintf("linalg: %s negative dim (M=%d K=%d N=%d)", kernel, M, K, N))
	}
	requireLen(kernel, "a", aLen, mul(M, K))
	requireLen(kernel, "b", bLen, mul(N, K))
	requireLen(kernel, "dst", dstLen, mul(M, N))
}
