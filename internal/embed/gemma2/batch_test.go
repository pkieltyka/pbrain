package gemma2

import (
	"math"
	"slices"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
)

func TestMemoryBudget(t *testing.T) {
	const mib = 1 << 20
	for _, test := range []struct {
		name, figure         string
		free, override, want int64
	}{
		{name: "live", figure: "live", free: 10 * gib, want: 3 * gib},
		{name: "ceiling", figure: "live", free: math.MaxInt64, want: 6 * gib},
		{name: "small", figure: "live", free: 400 * mib, want: 120 * mib},
		{name: "live zero", figure: "live"},
		{name: "estimate", figure: "estimated", free: 400 * mib, want: 120 * mib},
		{name: "estimate ceiling", figure: "estimated", free: 12 * gib, want: gib},
		{name: "unknown", figure: "unknown", want: gib},
		{name: "override 8 MiB", figure: "live", free: 10 * gib, override: 8 * mib, want: 8 * mib},
		{name: "override capped", figure: "live", free: 16 * gib, override: 10 * gib, want: 6 * gib},
		{name: "override capped by free", figure: "live", free: 4 * mib, override: 8 * mib, want: 4 * mib},
		{name: "override with live zero", figure: "live", override: 8 * mib},
		{name: "override with estimated free", figure: "estimated", free: 8 * mib, override: 2 * gib, want: 2 * gib},
	} {
		t.Run(test.name, func(t *testing.T) {
			memory := embed.DeviceInfo{Memory: test.figure, FreeBytes: test.free}
			if got := memoryBudget(memory, test.override); got != test.want {
				t.Fatalf("budget = %d, want %d", got, test.want)
			}
		})
	}
}

func TestGemmaPacker(t *testing.T) {
	for _, test := range []struct {
		name      string
		lengths   []int
		positions int
		want      []int
	}{
		{"in order, exact fill", []int{2, 1, 4, 3, 5}, 7, []int{3, 1, 1}},
		{"oversize alone", []int{4, 2048, 1, 2, 3}, 7, []int{1, 1, 3}},
		{"zero budget", []int{1, 1, 1}, 0, []int{1, 1, 1}},
		{"runtime positions", []int{2048, 2048, 2048, 2048, 2048}, 10000, []int{4, 1}},
		{"input cap", slices.Repeat([]int{1}, 260), 10000, []int{256, 4}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ids := make([][]int, len(test.lengths))
			for i, n := range test.lengths {
				ids[i] = make([]int, n)
			}
			var got []int
			for len(ids) > 0 {
				n := batchSize(ids, test.positions)
				got = append(got, n)
				ids = ids[n:]
			}
			if !slices.Equal(got, test.want) {
				t.Fatalf("batch sizes = %v, want %v", got, test.want)
			}
		})
	}
}
