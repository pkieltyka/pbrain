//go:build arm64

// Adapted from aikit linalg/exp_contract_arm64.go. See LICENSE and NOTICE in the bert directory.
package linalg

func softmaxContractWithMaxImpl(dst, src []float32, m float32) {
	softmaxRowContractNEONWithMax(dst, src, m)
}

// geluContractImpl routes the ERF through the kernel; the wrapper is one scaling
// multiply in and two out, with nothing to fuse into, so it stays in Go. Chunked
// through a stack buffer for the same two reasons as the tanh form: no
// allocation, and correctness when dst aliases src.
func geluContractImpl(dst, src []float32) {
	const invSqrt2 = 0.7071067811865476
	var buf [256]float32
	for off := 0; off < len(src); off += len(buf) {
		n := min(len(buf), len(src)-off)
		s := src[off : off+n]
		for i, v := range s {
			buf[i] = mulRound32(invSqrt2, v)
		}
		if n4 := n &^ 3; n4 > 0 {
			erfF32ContractNEON(&buf[0], &buf[0], n4)
		}
		for i := n &^ 3; i < n; i++ {
			buf[i] = erfF32Contract(buf[i])
		}
		for i, v := range s {
			dst[off+i] = mulRound32(mulRound32(0.5, v), 1+buf[i])
		}
	}
}
