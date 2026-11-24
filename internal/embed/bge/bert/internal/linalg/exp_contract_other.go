//go:build !amd64 && !arm64

// Adapted from aikit linalg/exp_contract_other.go. See LICENSE and NOTICE in the bert directory.
package linalg

func softmaxContractWithMaxImpl(dst, src []float32, m float32) {
	softmaxRowContractWithMax(dst, src, m)
}

func geluContractImpl(dst, src []float32) { geluScalarInto(dst, src) }
