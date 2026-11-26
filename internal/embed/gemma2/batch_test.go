package gemma2

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
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
		name    string
		device  string
		lengths []int
		budget  int64
		quality embeddinggemma2.BatchMemoryQuality
		want    []int
	}{
		{"unknown exact fill", "cuda:0", []int{2, 1, 4, 3, 5}, 7 * bytesPerPosition, 0, []int{3, 1, 1}},
		{"unknown oversize singleton", "cuda:0", []int{4, 2048, 1, 2, 3}, 7 * bytesPerPosition, 0, []int{1, 1, 3}},
		{"unknown zero budget", "cuda:0", []int{1, 1, 1}, 0, 0, []int{1, 1, 1}},
		{"unknown CUDA ceiling", "cuda:0", []int{2048, 2048, 2048, 2048, 1}, gib * 2, 0, []int{4, 1}},
		{"unknown Vulkan ceiling", "vulkan:0", []int{2048, 2048, 2048, 2048, 1}, gib * 2, 0, []int{4, 1}},
		{"unknown Vulkan budget", "vulkan:0", []int{4, 3, 2}, 7 * vulkanBytesPerPosition, 0, []int{2, 1}},
		{"input cap", "cuda:0", slices.Repeat([]int{1}, 257), gib, 0, []int{256, 1}},
		{"known additional exact fit", "cuda:0", []int{2, 1, 4}, 14, embeddinggemma2.BatchMemoryUpperBound, []int{3}},
		{"known additional one byte over", "cuda:0", []int{2, 1, 4}, 13, embeddinggemma2.BatchMemoryUpperBound, []int{2, 1}},
		{"known zero when resident", "cuda:0", []int{4, 4, 4}, 0, embeddinggemma2.BatchMemoryUpperBound, []int{3}},
		{"known CUDA retains ceiling", "cuda:0", []int{2048, 2048, 2048, 2048, 1}, 16386, embeddinggemma2.BatchMemoryUpperBound, []int{4, 1}},
		{"known Vulkan remains capped", "vulkan:0", []int{2048, 2048, 2048, 2048, 1}, 16386, embeddinggemma2.BatchMemoryUpperBound, []int{4, 1}},
		{"Metal input cap", metalDeviceID, []int{500, 500, 500, 500, 500}, gib, embeddinggemma2.BatchMemoryApproximate, []int{4, 1}},
		{"Metal position cap", metalDeviceID, []int{1000, 1000, 1000, 10}, gib, embeddinggemma2.BatchMemoryApproximate, []int{2, 2}},
		{"Metal position exact fit", metalDeviceID, []int{2048, 2048, 700}, gib, embeddinggemma2.BatchMemoryApproximate, []int{1, 2}},
		{"Metal one position over", metalDeviceID, []int{700, 2048, 1}, gib, embeddinggemma2.BatchMemoryApproximate, []int{2, 1}},
		{"Metal budget", metalDeviceID, []int{500, 500}, 1999, embeddinggemma2.BatchMemoryApproximate, []int{1, 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			info := embed.RunInfo{Device: test.device, Budget: test.budget}
			lengths := test.lengths
			var got []int
			for len(lengths) > 0 {
				n, err := batchSize(t.Context(), lengths, info, nil, func(prefix []int) (embeddinggemma2.BatchMemoryEstimate, error) {
					b := int64(0)
					for _, l := range prefix {
						if l != 4 || test.budget != 0 {
							b += int64(l) * 2
						}
					}
					return embeddinggemma2.BatchMemoryEstimate{Quality: test.quality, AdditionalPeakBytes: b,
						ResidentWeightBytes: gib}, nil
				})
				if err != nil || n <= 0 {
					t.Fatalf("batchSize = %d, %v", n, err)
				}
				got = append(got, n)
				lengths = lengths[n:]
			}
			if !slices.Equal(got, test.want) {
				t.Fatalf("batch sizes = %v, want %v", got, test.want)
			}
		})
	}
}

func TestGemmaPackerErrors(t *testing.T) {
	info := embed.RunInfo{Device: "cuda:0", Budget: gib}
	unsupported := embeddinggemma2.ErrUnsupportedBatchShape
	estimate := func(lengths []int) (embeddinggemma2.BatchMemoryEstimate, error) {
		if len(lengths) > 2 || lengths[0] == 9 {
			return embeddinggemma2.BatchMemoryEstimate{}, unsupported
		}
		return embeddinggemma2.BatchMemoryEstimate{}, nil
	}
	if n, err := batchSize(t.Context(), []int{1, 2, 3}, info, nil, estimate); n != 2 || err != nil {
		t.Fatalf("unsupported extension = %d, %v", n, err)
	}
	if n, err := batchSize(t.Context(), []int{9, 2}, info, nil, estimate); n != 0 || !errors.Is(err, unsupported) {
		t.Fatalf("unsupported singleton = %d, %v", n, err)
	}
	for _, lengths := range [][]int{{0}, {maxTokens + 1}, {1, -1}} {
		if n, err := batchSize(t.Context(), lengths, info, nil, estimate); n != 0 || err == nil {
			t.Fatalf("invalid shape %v = %d, %v", lengths, n, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	failed := errors.New("closed encoder")
	fail := func([]int) (embeddinggemma2.BatchMemoryEstimate, error) {
		cancel()
		return embeddinggemma2.BatchMemoryEstimate{}, failed
	}
	if n, err := batchSize(ctx, []int{1, 2}, info, nil, fail); n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled estimate = %d, %v", n, err)
	}
	if n, err := batchSize(t.Context(), []int{1, 2}, info, nil, fail); n != 0 || !errors.Is(err, failed) {
		t.Fatalf("failed estimate = %d, %v", n, err)
	}
}
