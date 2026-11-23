//go:build linux && amd64 && vulkan

package gemma2

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pkieltyka/goinfer/backend/vulkan"
	"github.com/pkieltyka/pbrain/internal/embed"
)

const q6Fixture = "PBRAIN_TEST_GEMMA2_Q6"

// layoutReceipt is what the device acceptance runner verifies without parsing
// test output. It records same-device layout differences and CPU/Vulkan cosine
// for one quant.
type layoutReceipt struct {
	Model         string         `json:"model"`
	Alias         string         `json:"alias"`
	Persistent    string         `json:"persistent"`
	Passages      int            `json:"passages"`
	Differing     map[string]int `json:"differing"`
	MinCosine768  float64        `json:"min_cpu_cosine_768"`
	MinCosine512  float64        `json:"min_cpu_cosine_512"`
	WorstPassage  int            `json:"worst_passage"`
	ReferenceBits string         `json:"reference_bits"`
}

// TestGemmaVulkanLayouts checks the Q8_0 and UD-Q6_K_XL rows on one named
// Vulkan device: same-device layouts must be identical by bits, and CPU/Vulkan
// vectors must meet rule 2. It writes a receipt when PBRAIN_VULKAN_LAYOUT_DIR
// is set and does no work when the device or fixtures are absent.
func TestGemmaVulkanLayouts(t *testing.T) {
	selector := os.Getenv("PBRAIN_TEST_VULKAN_DEVICE")
	if selector == "" {
		t.Skip("set PBRAIN_TEST_VULKAN_DEVICE to the persistent device ID")
	}
	if os.Getenv("PBRAIN_GPU_MEMORY") != "1024" {
		t.Fatal("set PBRAIN_GPU_MEMORY=1024 for the 1 GiB default-budget parity capture")
	}
	receiptDir := os.Getenv("PBRAIN_VULKAN_LAYOUT_DIR")
	if receiptDir != "" {
		if err := os.MkdirAll(receiptDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	texts := batchFixture(t)

	for _, tc := range []struct {
		name, variable string
	}{
		{q8Name, q8Fixture},
		{q6Name, q6Fixture},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pack := fixtureDir(t, tc.variable)
			entry := testGGUFEntry(t, tc.name)
			reg := embed.NewRegistry(entry)
			devices := reg.Devices()
			at := slices.IndexFunc(devices, func(d embed.DeviceInfo) bool {
				return d.Backend == "vulkan" && d.Persistent == selector
			})
			if at < 0 {
				t.Fatalf("validated Vulkan device %q is not listed: %+v", selector, devices)
			}
			card := devices[at]
			open := func(device string, budget int64) embed.Embedder {
				t.Helper()
				x, err := reg.Open(t.Context(), verified(t, pack, entry.Model),
					embed.OpenOptions{Device: device, MemoryBudget: budget})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := x.Close(); err != nil {
						t.Errorf("close %q: %v", device, err)
					}
				})
				return x
			}
			embeds := func(x embed.Embedder, docs []string) [][]float32 {
				t.Helper()
				vectors, err := x.Embed(t.Context(), embed.Document, docs)
				if err != nil || len(vectors) != len(docs) {
					t.Fatalf("%s returned %d vectors for %d passages: %v", x.Info().Device, len(vectors), len(docs), err)
				}
				return vectors
			}

			cpu := open("", 0)
			if cpu.Info().Device != "cpu" {
				t.Fatalf("reference is %+v, want cpu", cpu.Info())
			}
			want := embeds(cpu, texts)
			if err := cpu.Close(); err != nil {
				t.Fatalf("close CPU: %v", err)
			}

			gpu := open(card.Persistent, gib)
			if gpu.Info().Device != card.ID || gpu.Info().Name != card.Name || gpu.(*encoder).Window() != batchInputs {
				t.Fatalf("Vulkan run is %+v, window %d", gpu.Info(), gpu.(*encoder).Window())
			}
			actual, ok := vulkan.AttachedDevice(gpu.(*encoder).model.Accelerator())
			if !ok || actual.ID != card.Persistent {
				t.Fatalf("attached %+v, want %s", actual, card.Persistent)
			}

			single := make([][]float32, len(texts))
			for i, text := range texts {
				single[i] = embeds(gpu, []string{text})[0]
			}
			worst768, worst512, overall, passage := 1.0, 1.0, 1.0, -1
			for i := range single {
				c768 := similarity(want[i][:768], single[i][:768])
				c512 := similarity(want[i][:512], single[i][:512])
				if c768 < 0.999 || c512 < 0.999 {
					t.Errorf("passage %d cosine 768/512 = %.14f / %.14f, below 0.999", i, c768, c512)
				}
				if c768 < worst768 {
					worst768 = c768
				}
				if c512 < worst512 {
					worst512 = c512
				}
				if v := min(c768, c512); v < overall {
					overall, passage = v, i
				}
			}
			t.Logf("%d CPU/Vulkan passage pairs, minima 768 %.14f / 512 %.14f, worst passage %d",
				len(single), worst768, worst512, passage)

			differing := map[string]int{}
			check := func(label string, vectors [][]float32) {
				t.Helper()
				count := 0
				for i, vector := range vectors {
					if !sameVectorBits(vector, single[i]) {
						count++
					}
				}
				differing[label] = count
				if count != 0 {
					t.Errorf("%s: %d of %d vectors differ by bits", label, count, len(vectors))
				}
				t.Logf("%s: %d of %d vectors differ by bits", label, count, len(vectors))
			}
			check("packed", embeds(gpu, texts))
			var split [][]float32
			for block := range slices.Chunk(texts, 17) {
				split = append(split, embeds(gpu, block)...)
			}
			check("split17", split)
			reversed := slices.Clone(texts)
			slices.Reverse(reversed)
			back := embeds(gpu, reversed)
			slices.Reverse(back)
			check("reversed", back)
			if err := gpu.Close(); err != nil {
				t.Fatalf("close Vulkan: %v", err)
			}

			const tiny = 1 << 20
			tinyGPU := open(card.Persistent, tiny)
			for _, text := range texts {
				ids, err := tinyGPU.(*encoder).tokenIDs(embed.Document, text)
				if err != nil || len(ids) <= tiny/vulkanBytesPerPosition {
					t.Fatalf("singleton-only fixture: %d prepared tokens, %v", len(ids), err)
				}
			}
			check("tiny-budget", embeds(tinyGPU, texts))
			if err := tinyGPU.Close(); err != nil {
				t.Fatalf("close 1 MiB Vulkan: %v", err)
			}

			if receiptDir == "" {
				return
			}
			receipt := layoutReceipt{
				Model: tc.name, Alias: card.ID, Persistent: card.Persistent,
				Passages: len(texts), Differing: differing,
				MinCosine768: worst768, MinCosine512: worst512, WorstPassage: passage,
				// The reference is the CPU packed pass; layouts compare to the
				// Vulkan singleton vectors.
				ReferenceBits: "vulkan singleton",
			}
			raw, err := json.MarshalIndent(receipt, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(receiptDir, tc.name+".json"), append(raw, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
		})
	}
}
