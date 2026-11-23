//go:build cuda && eg2_combined && eg2_fp16

package gemma2

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/trace"
)

func TestLinkedCUDAPrecisions(t *testing.T) {
	linked := embeddinggemma2.DeviceSelectors("cuda")
	for _, want := range []string{"cuda", "cuda-eg2-combined", embed.CUDAFP16Selector} {
		if !slices.Contains(linked, want) {
			t.Errorf("linked selectors %v lack %s", linked, want)
		}
	}
}

func TestGemmaFP16ByteLayouts(t *testing.T) {
	for _, tc := range []struct {
		row  embed.Entry
		pack string
	}{{Entry(), "PBRAIN_TEST_GEMMA2_PACK"}, {testGGUFEntry(t, q8Name), q8Fixture}} {
		t.Run(tc.row.Model.Name, func(t *testing.T) {
			folder := fixtureDir(t, tc.pack)
			ctx := t.Context()
			if dir := os.Getenv("PBRAIN_FP16_TRACE_DIR"); dir != "" {
				log, err := trace.Open(filepath.Join(dir, tc.row.Model.Name+".jsonl"))
				if err != nil {
					t.Fatal(err)
				}
				ctx = log.Context(ctx)
				t.Cleanup(func() {
					if err := log.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			reg := embed.NewRegistry(tc.row)
			profile := reg.ResolveWriter(tc.row.Model, "fp16", "cuda:0")
			if profile.Selector != embed.CUDAFP16Selector {
				t.Skipf("no qualified device: %s", profile.Reason)
			}
			emb, err := reg.Open(ctx, verified(t, folder, tc.row.Model), embed.OpenOptions{
				Device: "cuda:0", Writer: profile,
			})
			if err != nil {
				t.Fatal(err)
			}
			e := emb.(*encoder)
			t.Cleanup(func() { _ = e.Close() })
			if e.cpu != nil || e.Info().Device != "cuda:0" || e.Fingerprint() != tc.row.Model.CUDAFP16Fingerprint {
				t.Fatalf("did not open a bound fp16 writer: %+v", e.Info())
			}
			t.Logf("selector=%s device=%s budget=%d", profile.Selector, profile.DeviceID, e.Info().Budget)
			raw := e.model.Accelerator()
			memory, err := raw.(embeddinggemma2.DeviceMemoryReporter).DeviceMemory()
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("post-attach device memory: %+v", memory)
			wrapper := &splitCUDA{Accelerator: raw,
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
			singletons := make([][]float32, len(texts))
			embedAll := func(ctx context.Context, input []string) [][]float32 {
				t.Helper()
				vectors, err := e.Embed(ctx, embed.Document, input)
				if err != nil || len(vectors) != len(input) {
					t.Fatalf("embed: %d vectors, %v", len(vectors), err)
				}
				return vectors
			}
			for i, text := range texts {
				singletons[i] = embedAll(trace.WithWindow(ctx, 1), []string{text})[0]
			}
			check := func(label string, window int, input []string, reverse bool) []int {
				t.Helper()
				wrapper.shapes = nil
				got := embedAll(trace.WithWindow(ctx, window), input)
				if reverse {
					slices.Reverse(got)
				}
				for i, vector := range got {
					if !sameVectorBits(vector, singletons[i]) {
						t.Fatalf("%s changed vector %d", label, i)
					}
				}
				for _, positions := range wrapper.shapes {
					if positions > batchPositions {
						t.Fatalf("%s exceeded 8,192 positions: %v", label, wrapper.shapes)
					}
				}
				t.Logf("%s: %d bit-identical vectors, aggregate calls %v", label, len(got), wrapper.shapes)
				return slices.Clone(wrapper.shapes)
			}
			ordinary := check("default", 2, texts, false)
			reversed := slices.Clone(texts)
			slices.Reverse(reversed)
			check("reordered", 3, reversed, true)
			budget := e.info.Budget
			e.info.Budget = 1 << 20
			check("1 MiB", 4, texts, false)
			e.info.Budget = budget
			wrapper.failNext = true
			split := check("certified OOM split", 5, texts, false)
			if len(ordinary) == 0 || len(split) <= len(ordinary) || split[0] != ordinary[0] ||
				wrapper.failNext || e.retryLimit == nil || *e.retryLimit != ordinary[0]/2 || e.model.Accelerator() != wrapper {
				t.Fatalf("the certified retry did not shrink the same encoder's batches: %v", split)
			}
			if err := e.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
