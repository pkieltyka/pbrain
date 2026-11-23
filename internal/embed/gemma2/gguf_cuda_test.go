//go:build cuda

package gemma2

import (
	"slices"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
)

func TestGemmaQ8CUDAByteLayouts(t *testing.T) {
	pack := fixtureDir(t, q8Fixture)
	if len(listCUDADevices().Devices) == 0 {
		t.Skip("no usable CUDA device")
	}
	entry := testGGUFEntry(t, q8Name)
	reg := embed.NewRegistry(entry)
	x, err := reg.Open(t.Context(), verified(t, pack, entry.Model), embed.OpenOptions{Device: "cuda:0"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = x.Close() })
	window := x.(*encoder).Window()
	if x.Info().Device != "cuda:0" || window != batchInputs {
		t.Fatalf("expected a CUDA batch encoder, got %+v window %d", x.Info(), window)
	}
	texts := batchFixture(t)
	get := func(xs []string) [][]float32 {
		t.Helper()
		v, err := x.Embed(t.Context(), embed.Document, xs)
		if err != nil || len(v) != len(xs) {
			t.Fatalf("got %d vectors for %d documents: %v", len(v), len(xs), err)
		}
		return v
	}
	single := make([][]float32, len(texts))
	for i := range texts {
		single[i] = get(texts[i : i+1])[0]
	}
	check := func(label string, got [][]float32) {
		t.Helper()
		for i, v := range got {
			if !sameVectorBits(v, single[i]) {
				t.Fatalf("%s: passage %d differed from its singleton", label, i)
			}
		}
		t.Logf("%s: all %d vectors match the singleton bits", label, len(got))
	}
	check("packed", get(texts))
	var split [][]float32
	for block := range slices.Chunk(texts, 17) {
		split = append(split, get(block)...)
	}
	check("split into 17-input windows", split)
	reversed := slices.Clone(texts)
	slices.Reverse(reversed)
	back := get(reversed)
	slices.Reverse(back)
	check("reversed and restored", back)
	if err := x.Close(); err != nil {
		t.Fatal(err)
	}
}
