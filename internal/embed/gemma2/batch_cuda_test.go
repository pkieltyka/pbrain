//go:build cuda

package gemma2

import (
	"fmt"
	"testing"
	"time"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

func TestGemmaBatchSameBytes(t *testing.T) {
	folder := fixtureDir(t, "PBRAIN_TEST_GEMMA2_PACK")
	if len(listCUDADevices().Devices) == 0 {
		t.Skip("no usable CUDA device")
	}
	texts := batchFixture(t)
	ctx := t.Context()
	entry := Entry()
	reg := embed.NewRegistry(entry)
	openGPU := func(budget int64) embed.Embedder {
		t.Helper()
		emb, err := reg.Open(ctx, verified(t, folder, entry.Model), embed.OpenOptions{Device: "cuda:0", MemoryBudget: budget})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = emb.Close() })
		if emb.Info().Device != "cuda:0" {
			t.Fatalf("opened device = %+v, want cuda:0", emb.Info())
		}
		return emb
	}
	emb := openGPU(0)
	single := make([][]float32, len(texts))
	start := time.Now()
	for i, text := range texts {
		v, err := emb.Embed(ctx, embed.Document, []string{text})
		if err != nil {
			t.Fatal(err)
		}
		single[i] = v[0]
	}
	t.Logf("singleton %.2f passages/s", float64(len(texts))/time.Since(start).Seconds())

	compare := func(label string, emb embed.Embedder) {
		t.Helper()
		start := time.Now()
		vectors, err := emb.Embed(ctx, embed.Document, texts)
		elapsed := time.Since(start)
		if err != nil || len(vectors) != len(single) {
			t.Fatalf("%s: %d vectors, %v", label, len(vectors), err)
		}
		for i, vector := range vectors {
			// The bit pattern check also distinguishes signed zero.
			if !sameVectorBits(vector, single[i]) {
				t.Fatalf("%s: vector %d differs from its singleton", label, i)
			}
		}
		t.Logf("%s: %d equal vectors, budget %d bytes, %.2f passages/s",
			label, len(vectors), emb.Info().Budget, float64(len(texts))/elapsed.Seconds())
		if err := emb.Close(); err != nil {
			t.Fatal(err)
		}
	}
	compare("default budget", emb)
	const override = 1 << 20
	emb = openGPU(override)
	if emb.Info().Budget != override {
		t.Fatalf("override budget = %d, want %d", emb.Info().Budget, override)
	}
	compare("1 MiB budget", emb)
}

// splitCUDA owns one real device and injects one certified failure before
// dispatch. All successful calls still execute on that same device.
type splitCUDA struct {
	embeddinggemma2.Accelerator
	batch interface {
		ForwardBatch([][]int) ([][]float32, error)
	}
	memory   embeddinggemma2.BatchMemoryEstimator
	failNext bool
	shapes   []int
}

func (a *splitCUDA) BatchMemory(lengths []int) (embeddinggemma2.BatchMemoryEstimate, error) {
	return a.memory.BatchMemory(lengths)
}

func (a *splitCUDA) Forward(ids []int, keepLayers bool) ([]float32, [][]float32, error) {
	a.shapes = append(a.shapes, len(ids))
	return a.Accelerator.Forward(ids, keepLayers)
}

func (a *splitCUDA) ForwardBatch(ids [][]int) ([][]float32, error) {
	positions := 0
	for _, input := range ids {
		positions += len(input)
	}
	a.shapes = append(a.shapes, positions)
	if a.failNext {
		a.failNext = false
		return nil, fmt.Errorf("test forward before dispatch: %w", embeddinggemma2.ErrBatchOutOfMemory)
	}
	return a.batch.ForwardBatch(ids)
}

func TestGemmaCUDAForcedSplitSameBytes(t *testing.T) {
	for _, test := range []struct {
		entry embed.Entry
		pack  string
	}{
		{Entry(), "PBRAIN_TEST_GEMMA2_PACK"},
		{testGGUFEntry(t, q8Name), q8Fixture},
	} {
		t.Run(test.entry.Model.Name, func(t *testing.T) {
			folder := fixtureDir(t, test.pack)
			if len(listCUDADevices().Devices) == 0 {
				t.Skip("no usable CUDA device")
			}
			reg := embed.NewRegistry(test.entry)
			emb, err := reg.Open(t.Context(), verified(t, folder, test.entry.Model), embed.OpenOptions{Device: "cuda:0"})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = emb.Close() })
			e := emb.(*encoder)
			raw := e.model.Accelerator()
			wrapper := &splitCUDA{
				Accelerator: raw,
				batch: raw.(interface {
					ForwardBatch([][]int) ([][]float32, error)
				}),
				memory: raw.(embeddinggemma2.BatchMemoryEstimator),
			}
			embeddinggemma2.RegisterAccelerator(t.Name(), func(*embeddinggemma2.Model) (embeddinggemma2.Accelerator, error) {
				return wrapper, nil
			})
			if _, err := e.model.UseAccelerator(t.Name()); err != nil {
				t.Fatal(err)
			}
			texts := batchFixture(t)
			embedTexts := func(texts []string) [][]float32 {
				t.Helper()
				out, err := e.Embed(t.Context(), embed.Document, texts)
				if err != nil || len(out) != len(texts) {
					t.Fatalf("embedded %d of %d: %v", len(out), len(texts), err)
				}
				return out
			}
			single := make([][]float32, len(texts))
			for i := range texts {
				single[i] = embedTexts(texts[i : i+1])[0]
			}
			check := func(label string) {
				t.Helper()
				got := embedTexts(texts)
				for i := range got {
					if !sameVectorBits(got[i], single[i]) {
						t.Fatalf("%s changed passage %d", label, i)
					}
				}
				t.Logf("%s: %d bit-identical vectors, forward attempts by aggregate positions %v", label, len(got), wrapper.shapes)
			}
			wrapper.shapes = nil
			check("default budget")
			if len(wrapper.shapes) != 2 || wrapper.shapes[0] <= batchPositions/2 ||
				wrapper.shapes[0] > batchPositions || wrapper.shapes[1] > batchPositions {
				t.Fatalf("wanted two capped forwards for the 59 passages, got %v", wrapper.shapes)
			}
			first := wrapper.shapes[0]
			total := first + wrapper.shapes[1]
			wrapper.shapes = nil
			wrapper.failNext = true
			check("forced split")
			if wrapper.failNext || len(wrapper.shapes) < 3 || wrapper.shapes[0] != first ||
				e.retryLimit == nil || *e.retryLimit != first/2 || e.model.Accelerator() != wrapper {
				t.Fatalf("did not split on the same encoder: %v", wrapper.shapes)
			}
			positions := 0
			for _, size := range wrapper.shapes[1:] {
				if size > first/2 {
					t.Fatalf("forward exceeded the reduced retry limit: %v", wrapper.shapes)
				}
				positions += size
			}
			if positions != total {
				t.Fatalf("retry forwarded %d positions, want %d", positions, total)
			}
			if err := e.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
