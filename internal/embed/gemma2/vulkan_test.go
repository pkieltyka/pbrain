//go:build linux && amd64 && vulkan

package gemma2

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/goinfer/backend/vulkan"
	"github.com/pkieltyka/pbrain/internal/embed"
)

func TestVulkanAbsentTexts(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "go", "list", "-m", "-f", "{{.Dir}}", "github.com/pkieltyka/goinfer/backend/vulkan")
	dir, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(strings.TrimSpace(string(dir)), "internal", "vk", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var source strings.Builder
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		source.Write(data)
	}
	for _, text := range []string{"open libvulkan.so.1:", `"create instance"`, "vulkan: %s: VkResult %d"} {
		if !strings.Contains(source.String(), text) {
			t.Errorf("pinned Vulkan module no longer provides %q", text)
		}
	}
}

func TestGemmaVulkan(t *testing.T) {
	selector := os.Getenv("PBRAIN_TEST_VULKAN_DEVICE")
	if selector == "" {
		t.Skip("set PBRAIN_TEST_VULKAN_DEVICE to the persistent device ID and use a memory-monitored run")
	}
	folder := fixtureDir(t, "PBRAIN_TEST_GEMMA2_PACK")
	if os.Getenv("PBRAIN_GPU_MEMORY") != "1024" {
		t.Fatal("set PBRAIN_GPU_MEMORY=1024 for the 1 GiB default-budget parity capture")
	}
	entry := Entry()
	reg := embed.NewRegistry(entry)
	devices := reg.Devices()
	at := slices.IndexFunc(devices, func(d embed.DeviceInfo) bool {
		return d.Backend == "vulkan" && d.Persistent == selector
	})
	if at < 0 || devices[at].ID == "" || devices[at].TotalBytes <= 0 {
		t.Fatalf("validated Vulkan device %q with capacity missing: %+v", selector, devices)
	}
	card := devices[at]
	t.Logf("listed %s %s %s, capacity %d bytes", card.ID, card.Persistent, card.Name, card.TotalBytes)
	ctx := t.Context()
	texts := batchFixture(t)
	open := func(device string, budget int64) embed.Embedder {
		t.Helper()
		emb, err := reg.Open(ctx, verified(t, folder, entry.Model), embed.OpenOptions{Device: device, MemoryBudget: budget})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = emb.Close() })
		return emb
	}
	closeModel := func(emb embed.Embedder) {
		t.Helper()
		if err := emb.Close(); err != nil {
			t.Fatal(err)
		}
	}
	embeds := func(emb embed.Embedder, docs []string) [][]float32 {
		t.Helper()
		vectors, err := emb.Embed(ctx, embed.Document, docs)
		if err != nil || len(vectors) != len(docs) {
			t.Fatalf("%s returned %d vectors for %d passages: %v", emb.Info().Device, len(vectors), len(docs), err)
		}
		return vectors
	}
	cpu := open("", 0)
	want := embeds(cpu, texts)
	closeModel(cpu)

	checkAttached := func(emb embed.Embedder, selected string, budget int64) {
		t.Helper()
		actual, ok := vulkan.AttachedDevice(emb.(*encoder).model.Accelerator())
		info := emb.Info()
		if !ok || actual.ID != card.Persistent || info.Device != card.ID || info.Name != card.Name || info.Budget != budget {
			t.Fatalf("selector %q attached %+v, run info %+v", selected, actual, info)
		}
		t.Logf("selector %s attached %s, reports %s, %d MiB a batch", selected, actual.ID, info.Device, info.Budget>>20)
	}
	gpu := open(card.ID, 0)
	checkAttached(gpu, card.ID, gib)
	single := make([][]float32, len(texts))
	worst, passage := 1.0, -1
	for i, text := range texts {
		single[i] = embeds(gpu, []string{text})[0]
		for _, n := range []int{512, 768} {
			cosine := similarity(want[i][:n], single[i][:n])
			if !(cosine >= 0.999) {
				t.Errorf("passage %d cosine at %d dimensions = %.12f", i, n, cosine)
			}
			if cosine < worst {
				worst, passage = cosine, i
			}
		}
	}
	t.Logf("%d CPU/Vulkan passage pairs, minimum cosine %.14f at passage %d", len(single), worst, passage)

	checkBits := func(label string, vectors, single [][]float32) {
		t.Helper()
		var differing int
		for i, vector := range vectors {
			if !sameVectorBits(vector, single[i]) {
				differing++
			}
		}
		t.Logf("%s: %d of %d vectors differ by bits", label, differing, len(vectors))
		if differing != 0 {
			t.Errorf("%s changed vector bytes", label)
		}
	}
	checkBits("default budget", embeds(gpu, texts), single)
	reversed := slices.Clone(texts)
	slices.Reverse(reversed)
	vectors := embeds(gpu, reversed)
	slices.Reverse(vectors)
	checkBits("default budget, reverse order", vectors, single)
	closeModel(gpu)

	const override = 1 << 20
	gpu = open(card.Persistent, override)
	checkAttached(gpu, card.Persistent, override)
	for _, text := range texts {
		ids, err := gpu.(*encoder).tokenIDs(embed.Document, text)
		if err != nil || len(ids) <= override/vulkanBytesPerPosition {
			t.Fatalf("singleton-only fixture: %d prepared tokens, %v", len(ids), err)
		}
	}
	checkBits("1 MiB budget", embeds(gpu, texts), single)
	closeModel(gpu)
}
