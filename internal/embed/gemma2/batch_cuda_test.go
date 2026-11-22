//go:build cuda

package gemma2

import (
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/semantic"
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
			if !slices.EqualFunc(vector, single[i], func(a, b float32) bool {
				return math.Float32bits(a) == math.Float32bits(b)
			}) {
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

// batchFixture uses the 32 notes from the Phase 28 parity fixture.
func batchFixture(t *testing.T) []string {
	t.Helper()
	root := "../../semantic/calibrate/testdata/retrieval/brain"
	var texts []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		switch d.Name() {
		case "bge-hugot-old.md", "flat-vectors-old.md", "output-budget-old.md":
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h, offset, _, err := item.Parse(strings.TrimSuffix(d.Name(), ".md"), data)
		if err != nil {
			return err
		}
		for _, p := range semantic.Passages(h.Title, data[offset:]) {
			texts = append(texts, p.Text)
		}
		return nil
	})
	if err != nil || len(texts) != 59 {
		t.Fatalf("batch fixture: %d passages, %v", len(texts), err)
	}
	return texts
}
