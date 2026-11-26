//go:build darwin && arm64 && metal

package gemma2

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/trace"
)

type metalLayoutReceipt struct {
	layoutReceipt
	Profile      string         `json:"profile"`
	Fingerprint  string         `json:"fingerprint"`
	Differing512 map[string]int `json:"differing_512"`
	Forwards     int            `json:"packed_forwards"`
	Positions    int            `json:"packed_positions"`
	LargestGroup int            `json:"largest_group"`
	LargestBatch int            `json:"largest_batch_positions"`
}

func TestGemmaMetalBatchLayouts(t *testing.T) {
	pack := fixtureDir(t, q8Fixture)
	var texts []string
	if root := os.Getenv("PBRAIN_TEST_BATCH_NOTES"); root != "" {
		texts = batchFixtureAt(t, root, false)
	} else {
		texts = batchFixture(t)
	}
	ctx := t.Context()
	entry := testGGUFEntry(t, q8Name)
	reg := embed.NewRegistry(entry)
	open := func(options embed.OpenOptions) *encoder {
		t.Helper()
		got, err := reg.Open(ctx, verified(t, pack, entry.Model), options)
		if err != nil {
			t.Fatal(err)
		}
		enc := got.(*encoder)
		t.Cleanup(func() {
			if err := enc.Close(); err != nil {
				t.Errorf("close %q: %v", options.Device, err)
			}
		})
		return enc
	}
	closeEncoder := func(e *encoder) {
		t.Helper()
		if err := e.Close(); err != nil {
			t.Fatal(err)
		}
	}
	embeds := func(ctx context.Context, e *encoder, docs []string) [][]float32 {
		t.Helper()
		out, err := e.Embed(ctx, embed.Document, docs)
		if err != nil || len(out) != len(docs) {
			t.Fatalf("%s produced %d of %d vectors: %v", e.Info().Device, len(out), len(docs), err)
		}
		for i, v := range out {
			if len(v) != dimension {
				t.Fatalf("passage %d: %d dimensions", i, len(v))
			}
		}
		return out
	}
	assertMetal := func(e *encoder, batch bool, window int) {
		t.Helper()
		info := e.Info()
		profile, err := attachedMetalProfile(e.model.Accelerator(), batch)
		if err != nil || profile != metalCombinedProfile || e.Window() != window ||
			info.Device != metalDeviceID || (info.Budget > 0) != batch ||
			e.Fingerprint() != entry.Model.MetalFingerprints[1] {
			t.Fatalf("Metal batch=%v: info %+v, window %d, profile %q, fingerprint %q, %v",
				batch, info, e.Window(), profile, e.Fingerprint(), err)
		}
	}

	singleton := open(embed.OpenOptions{Device: metalDeviceID})
	assertMetal(singleton, false, 1)
	want := embeds(ctx, singleton, texts)
	closeEncoder(singleton)

	receipt := metalLayoutReceipt{
		layoutReceipt: layoutReceipt{
			Model: q8Name, Alias: metalDeviceID, Passages: len(texts), Differing: map[string]int{},
			MinCosine768: 1, MinCosine512: 1, WorstPassage: -1, ReferenceBits: "Metal combined singleton",
		},
		Profile: metalCombinedProfile, Fingerprint: singleton.Fingerprint(), Differing512: map[string]int{},
	}
	check := func(label string, got [][]float32) {
		t.Helper()
		n768, n512 := 0, 0
		for i, v := range got {
			if !sameVectorBits(v, want[i]) {
				n768++
			}
			if !sameVectorBits(normalizedPrefix512(v), normalizedPrefix512(want[i])) {
				n512++
			}
		}
		receipt.Differing[label], receipt.Differing512[label] = n768, n512
		t.Logf("%s: %d/%d vectors differ at 768/512 (of %d)", label, n768, n512, len(got))
		if n768 != 0 || n512 != 0 {
			t.Fatalf("%s did not preserve the Metal singleton bytes", label)
		}
	}

	batch := open(embed.OpenOptions{Device: metalDeviceID, MetalBatch: true})
	assertMetal(batch, true, 256)
	path := filepath.Join(t.TempDir(), "forwards.jsonl")
	recording, err := trace.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recording.Close() })
	packed := embeds(recording.Context(ctx), batch, texts)
	if err := recording.Close(); err != nil {
		t.Fatal(err)
	}
	check("packed", packed)
	totalInputs := 0
	for _, f := range tracedForwards(t, path) {
		if f.Failed || f.Device != metalDeviceID || f.Inputs < 1 || f.Inputs > 4 ||
			f.Inputs > 1 && f.TotalPositions > 2748 {
			t.Fatalf("unqualified Metal forward: %+v", f)
		}
		receipt.Forwards++
		receipt.Positions += f.TotalPositions
		receipt.LargestGroup = max(receipt.LargestGroup, f.Inputs)
		if f.Inputs > 1 {
			receipt.LargestBatch = max(receipt.LargestBatch, f.TotalPositions)
		}
		totalInputs += f.Inputs
	}
	if totalInputs != len(texts) || receipt.LargestGroup < 2 {
		t.Fatalf("packed route forwarded %d/%d passages, largest group %d", totalInputs, len(texts), receipt.LargestGroup)
	}
	t.Logf("packed: %d forwards, %d positions, largest group %d inputs / %d positions",
		receipt.Forwards, receipt.Positions, receipt.LargestGroup, receipt.LargestBatch)

	for _, size := range []int{1, 3} {
		var got [][]float32
		for block := range slices.Chunk(texts, size) {
			got = append(got, embeds(ctx, batch, block)...)
		}
		label := "singleton"
		if size == 3 {
			label = "split3"
		}
		check(label, got)
	}
	reversed := slices.Clone(texts)
	slices.Reverse(reversed)
	back := embeds(ctx, batch, reversed)
	slices.Reverse(back)
	check("reversed", back)
	closeEncoder(batch)

	cpu := open(embed.OpenOptions{})
	if cpu.Info().Device != "cpu" || cpu.model.Accelerator() != nil {
		t.Fatalf("same-file reference opened on %+v", cpu.Info())
	}
	worst := 1.0
	for i, v := range embeds(ctx, cpu, texts) {
		c768 := similarity(v, want[i])
		c512 := similarity(normalizedPrefix512(v), normalizedPrefix512(want[i]))
		if !(c768 >= 0.999 && c512 >= 0.999) {
			t.Fatalf("passage %d: CPU/Metal cosine 768 %.14f / 512 %.14f", i, c768, c512)
		}
		receipt.MinCosine768 = min(receipt.MinCosine768, c768)
		receipt.MinCosine512 = min(receipt.MinCosine512, c512)
		if c := min(c768, c512); c < worst {
			worst, receipt.WorstPassage = c, i
		}
	}
	closeEncoder(cpu)
	t.Logf("%d CPU/Metal passage pairs, minima 768 %.14f / 512 %.14f", len(texts), receipt.MinCosine768, receipt.MinCosine512)
	if dir := os.Getenv("PBRAIN_METAL_LAYOUT_DIR"); dir != "" {
		data, err := json.MarshalIndent(receipt, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, q8Name+".json"), append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func normalizedPrefix512(v []float32) []float32 {
	p := slices.Clone(v[:512])
	var squares float64
	for _, n := range p {
		squares += float64(n) * float64(n)
	}
	norm := math.Sqrt(squares)
	for i, n := range p {
		p[i] = float32(float64(n) / norm)
	}
	return p
}
