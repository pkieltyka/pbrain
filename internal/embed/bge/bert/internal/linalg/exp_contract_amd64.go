//go:build amd64

// Adapted from aikit linalg/exp_contract_amd64.go. See LICENSE and NOTICE in the bert directory.
package linalg

func softmaxContractWithMaxImpl(dst, src []float32, m float32) {
	if !hasAVX2 || len(src) == 0 {
		softmaxRowContractWithMax(dst, src, m)
		return
	}
	// dst doubles as scratch. Safe when dst aliases src: each index is read
	// before it is written.
	for i, v := range src {
		d := v - m
		if d < expClampLoF32 {
			d = expClampLoF32
		}
		dst[i] = d
	}
	n8 := len(dst) &^ 7
	if n8 > 0 {
		expF32ContractAVX2(&dst[0], &dst[0], n8)
	}
	for i := n8; i < len(dst); i++ {
		dst[i] = expF32Contract(dst[i])
	}
	// The pinned order, reproduced exactly: element i into partials[i&3], folded
	// by the fixed tree. This is the half that would silently diverge if it were
	// left to a plain sequential sum.
	var p [4]float64
	for i, e := range dst {
		p[i&3] += float64(e)
	}
	sum := (p[0] + p[1]) + (p[2] + p[3])
	if sum == 0 {
		u := float32(1) / float32(len(src))
		for i := range dst {
			dst[i] = u
		}
		return
	}
	inv := float32(1 / sum)
	d := 0
	for ; d+3 < len(dst); d += 4 {
		dst[d+0] *= inv
		dst[d+1] *= inv
		dst[d+2] *= inv
		dst[d+3] *= inv
	}
	for ; d < len(dst); d++ {
		dst[d] *= inv
	}
}

func geluContractImpl(dst, src []float32) {
	if !hasAVX2 {
		geluScalarInto(dst, src)
		return
	}
	const invSqrt2 = 0.7071067811865476
	var buf [256]float32
	for off := 0; off < len(src); off += len(buf) {
		n := min(len(buf), len(src)-off)
		s := src[off : off+n]
		for i, v := range s {
			buf[i] = mulRound32(invSqrt2, v)
		}
		if n8 := n &^ 7; n8 > 0 {
			erfF32ContractAVX2(&buf[0], &buf[0], n8)
		}
		for i := n &^ 7; i < n; i++ {
			buf[i] = erfF32Contract(buf[i])
		}
		for i, v := range s {
			dst[off+i] = mulRound32(mulRound32(0.5, v), 1+buf[i])
		}
	}
}
