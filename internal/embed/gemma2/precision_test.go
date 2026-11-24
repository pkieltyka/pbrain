package gemma2

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

func TestFP16CapabilitySelection(t *testing.T) {
	gpu0 := embed.DeviceInfo{ID: "cuda:0", Persistent: "cuda:GPU-aaa", Backend: "cuda", FreeBytes: 8 << 30, Memory: "live", MemoryKind: "dedicated"}
	gpu0.ComputeCapability.Major = 12
	gpu0.RunnableSelectors = []string{"cuda", embed.CUDAFP16Selector}
	gpu1 := gpu0
	gpu1.ID, gpu1.Persistent, gpu1.FreeBytes = "cuda:1", "cuda:GPU-bbb", 12<<30
	amd := embed.DeviceInfo{ID: "vulkan:0", Persistent: "vulkan:bbb", Backend: "vulkan"}
	old := gpu1
	old.ComputeCapability.Major, old.ComputeCapability.Minor = 7, 5
	old.RunnableSelectors = []string{"cuda"}
	unknown := gpu1
	unknown.ComputeCapability.Major = 0
	unknown.RunnableSelectors = nil
	ineligible := gpu1
	ineligible.RunnableSelectors = nil
	tooSmall := gpu1
	tooSmall.FreeBytes = cudaWeightFloor - 1
	probeErr := map[string]error{"cuda": errors.New("inventory failed")}
	for _, tc := range []struct {
		name, selector     string
		devices            []embed.DeviceInfo
		failed             map[string]error
		linked             bool
		wantID, wantReason string
	}{
		{"ranked", "auto", []embed.DeviceInfo{gpu0, gpu1, amd}, nil, true, gpu1.Persistent, ""},
		{"explicit alias", gpu0.ID, []embed.DeviceInfo{gpu0, gpu1}, nil, true, gpu0.Persistent, ""},
		{"explicit UUID", gpu0.Persistent, []embed.DeviceInfo{gpu0, gpu1}, nil, true, gpu0.Persistent, ""},
		{"older ranked GPU", "auto", []embed.DeviceInfo{gpu0, old}, nil, true, "", "compute capability 7.5"},
		{"unknown capability", "auto", []embed.DeviceInfo{unknown}, probeErr, true, "", "could not verify"},
		{"capability alone is insufficient", "auto", []embed.DeviceInfo{ineligible}, nil, true, "", "does not report a runnable fp16"},
		{"missing kernel", gpu0.ID, []embed.DeviceInfo{gpu0}, nil, false, "", "no linked fp16"},
		{"too little memory", "auto", []embed.DeviceInfo{tooSmall, gpu0}, nil, true, gpu0.Persistent, ""},
		{"partial inventory", "auto", []embed.DeviceInfo{gpu0}, probeErr, true, gpu0.Persistent, ""},
		{"failed inventory", "auto", nil, probeErr, true, "", ""},
		{"Vulkan", "vulkan", []embed.DeviceInfo{gpu0, amd}, nil, true, "", ""},
		{"CPU", "cpu", []embed.DeviceInfo{gpu0}, nil, true, "", ""},
		{"missing GPU", "cuda:4", []embed.DeviceInfo{gpu0}, nil, true, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, reason := selectFP16(embed.Listing{Devices: tc.devices, Failed: tc.failed}, tc.selector, tc.linked)
			if d.Persistent != tc.wantID || !strings.Contains(reason, tc.wantReason) ||
				(tc.wantReason == "") != (reason == "") {
				t.Fatalf("selected %q, %q", d.Persistent, reason)
			}
		})
	}
}

// A synthetic inventory exercises goinfer's linked CUDA predicates without
// calling Devices or opening a driver. The same fixture covers the plain CUDA
// build (no FP16 factory) and the release CUDA build.
func TestCUDAInventoryProfileEligibility(t *testing.T) {
	resolve := fp16Resolver()
	if !cudaCompiled {
		if resolve != nil {
			t.Fatal("CPU build enables CUDA writer resolution")
		}
		return
	}
	linked := slices.Contains(embeddinggemma2.DeviceSelectors("cuda"), embed.CUDAFP16Selector)
	for _, cc := range []embeddinggemma2.CUDAComputeCapability{{Major: 7, Minor: 5}, {Major: 12}} {
		const uuid = "cuda:GPU-01234567-89ab-cdef-0123-456789abcdef"
		raw := embeddinggemma2.DeviceInfo{
			Backend: "cuda", ID: uuid, Alias: "cuda:0", TextBatching: true, ComputeCapability: cc,
		}
		listing := deviceListing([]embeddinggemma2.DeviceInfo{raw}, nil, false)
		if len(listing.Devices) != 1 {
			t.Fatalf("lost the CUDA fixture: %+v", listing)
		}
		card := listing.Devices[0]
		if card.ID != raw.Alias || card.Persistent != uuid || card.ComputeCapability != cc ||
			!slices.Equal(card.RunnableSelectors, raw.RunnableSelectors()) {
			t.Fatalf("did not carry the inventory fields: %+v", card)
		}
		wantFP16 := linked && cc.Major == 12
		if slices.Contains(card.RunnableSelectors, embed.CUDAFP16Selector) != wantFP16 {
			t.Fatalf("CC %d.%d selectors = %v", cc.Major, cc.Minor, card.RunnableSelectors)
		}
		selected, reason := resolve(listing, card.ID)
		if wantFP16 {
			if selected.Persistent != uuid || reason != "" {
				t.Fatalf("FP16 selection = %+v, %q", selected, reason)
			}
		} else {
			wantReason := "no linked fp16"
			if linked {
				wantReason = "compute capability 7.5"
			}
			if selected.Persistent != "" || !strings.Contains(reason, wantReason) {
				t.Fatalf("FP16 rejection = %+v, %q, want %q", selected, reason, wantReason)
			}
		}
	}
}

func TestCUDAProfileFingerprints(t *testing.T) {
	const qualifiedRevision = "c26c69ed44717d86cf2218370054d5e89886b694"
	if goinferRevision != qualifiedRevision {
		t.Fatalf("requalify the fp16 identity for goinfer %s", goinferRevision)
	}
	sums, err := os.ReadFile("../../../go.sum")
	if err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile("../../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for path, sum := range map[string]string{
		"github.com/pkieltyka/goinfer":              "oxahCIZevBqtTJTXE6DIsh0i32U2jNZ4Pw6Kp9LifwU=",
		"github.com/pkieltyka/goinfer/backend/cuda": "q73SRYrPxf+hi+Olg/A/pPhuBFfwssH6I3+ha2GfY34=",
	} {
		if !strings.Contains(string(mod), "\t"+path+" v0.6.7\n") ||
			!strings.Contains(string(sums), path+" v0.6.7 h1:"+sum+"\n") {
			t.Errorf("requalify the fp16 identity when %s changes", path)
		}
	}
	for _, model := range []struct {
		row        embed.Model
		base, fp16 string
	}{
		{Entry().Model, "79cd8cb6d605c915fb0ea74c3680816bd15df66e5db70ad0de1bf13df463de48", "a002b1e0bde5678c269e19debcbeee527dc206a1549f7835a8d733b22f8481f2"},
		{testGGUFEntry(t, q8Name).Model, "9ebb0de1d547aee1abffec1288639ce596a06b0c32447d502782a474118cf691", "4c1ace74bf64e6f53f4401a1031007847ec815aed25727b457d4547ff897ca28"},
	} {
		if model.row.Fingerprint != model.base || model.row.CUDAFP16Fingerprint != model.fp16 {
			t.Errorf("%s has an unexpected identity: %s %s", model.row.Name, model.row.Fingerprint, model.row.CUDAFP16Fingerprint)
		}
		updated := model.row
		updated.CUDAFP16Fingerprint = fp16Fingerprint(updated.Fingerprint, "c0a5159bbff1c2fe5c8624e1399aa7438c820677")
		if updated.CUDAFP16Fingerprint == model.fp16 || embed.FolderName(updated) != embed.FolderName(model.row) {
			t.Errorf("%s revision did not isolate vector identity from the file folder", model.row.Name)
		}
	}
}
