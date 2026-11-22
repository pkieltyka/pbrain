//go:build cuda

package gemma2

import (
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed"
)

func TestGemmaBatchSameBytes(t *testing.T) {
	folder := fixtureDir(t, "PBRAIN_TEST_GEMMA2_PACK")
	if len(cudaDevices()) == 0 {
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
	for _, text := range texts {
		ids, err := emb.(*encoder).tokenIDs(embed.Document, text)
		if err != nil || len(ids) <= override/bytesPerPosition {
			t.Fatalf("singleton fixture: %d prepared tokens, %v", len(ids), err)
		}
	}
	compare("1 MiB budget", emb)
}
