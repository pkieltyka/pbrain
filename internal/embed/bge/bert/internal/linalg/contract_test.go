// Adapted from aikit linalg/exp_contract_dispatch_test.go.
package linalg

import (
	"math"
	"slices"
	"testing"
)

func TestFP32ContractsMatchScalarTails(t *testing.T) {
	samples := []float32{-104, -86.5, -17.3, -3.25, -1, -0.5, -0.01, 0,
		0.05, 0.226, 0.625, 1.26, 1.77, 2.01, 3, 16, 100}
	sameBits := func(a, b float32) bool { return math.Float32bits(a) == math.Float32bits(b) }
	for n := range len(samples) + 1 {
		src := samples[:n]
		want, got := make([]float32, n), slices.Clone(src)
		geluScalarInto(want, src)
		GELUContractInto(got, got)
		if !slices.EqualFunc(got, want, sameBits) {
			t.Fatalf("%d elements: GELU = %v, want %v", n, got, want)
		}
		const scale float32 = 0.173
		want, got = slices.Clone(src), slices.Clone(src)
		for i := range want {
			want[i] *= scale
		}
		if n > 0 {
			softmaxRowContractWithMax(want, want, slices.Max(want))
		}
		SoftmaxRowScaledContractInto(got, got, scale)
		if !slices.EqualFunc(got, want, sameBits) {
			t.Fatalf("%d elements: softmax = %v, want %v", n, got, want)
		}
	}
}
