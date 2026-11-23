package gemma2

import (
	"os"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
)

func TestFP16CapabilitySelection(t *testing.T) {
	gpu0 := embed.DeviceInfo{ID: "cuda:0", Persistent: "cuda:GPU-aaa", Backend: "cuda", FreeBytes: 8 << 30, Memory: "live", MemoryKind: "dedicated"}
	gpu1 := embed.DeviceInfo{ID: "cuda:1", Persistent: "cuda:GPU-bbb", Backend: "cuda", FreeBytes: 12 << 30, Memory: "live", MemoryKind: "dedicated"}
	amd := embed.DeviceInfo{ID: "vulkan:0", Persistent: "vulkan:bbb", Backend: "vulkan"}
	list := embed.Listing{Devices: []embed.DeviceInfo{gpu0, gpu1, amd}}
	for _, tc := range []struct {
		name, selector     string
		linked, usable     bool
		major, minor       int
		wantID, wantReason string
		wantProbes         int
	}{
		{"ranked", "auto", true, true, 8, 0, gpu1.Persistent, "", 1},
		{"explicit", gpu0.Persistent, true, true, 8, 0, gpu0.Persistent, "", 1},
		{"old GPU", "auto", true, true, 7, 5, "", "compute capability 7.5", 1},
		{"unknown UUID or driver", "auto", true, false, 0, 0, "", "could not verify", 1},
		{"missing kernel", gpu0.ID, false, true, 8, 0, "", "no linked fp16", 0},
		{"Vulkan", "vulkan", true, true, 8, 0, "", "", 0},
		{"CPU", "cpu", true, true, 8, 0, "", "", 0},
		{"missing GPU", "cuda:4", true, true, 8, 0, "", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			d, reason := selectFP16(list, tc.selector, tc.linked, func(d embed.DeviceInfo) (int, int, bool) {
				calls++
				want := gpu1
				if tc.selector == gpu0.Persistent {
					want = gpu0
				}
				if d != want {
					t.Fatalf("probed %+v, want %+v", d, want)
				}
				return tc.major, tc.minor, tc.usable
			})
			if d.Persistent != tc.wantID || calls != tc.wantProbes || !strings.Contains(reason, tc.wantReason) ||
				(tc.wantReason == "") != (reason == "") {
				t.Fatalf("selected %q, %q, %d probes", d.Persistent, reason, calls)
			}
		})
	}
	for _, tc := range []struct {
		id, uuid string
		want     bool
	}{
		{"cuda:GPU-abcdef", "GPU-ABCDEF", true},
		{"cuda:GPU-a", "GPU-b", false},
		{"cuda:MIG-abcdef", "GPU-abcdef", false},
		{"GPU-abcdef", "GPU-abcdef", false},
	} {
		if sameCUDAUUID(tc.id, tc.uuid) != tc.want {
			t.Errorf("sameCUDAUUID(%q, %q), want %t", tc.id, tc.uuid, tc.want)
		}
	}
}

func TestCUDAProfileFingerprints(t *testing.T) {
	const qualifiedRevision = "897943c7a6bba158a74eaa3e2c43b1d11b24c445"
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
		"github.com/pkieltyka/goinfer":              "cLTCqfbEOHGK2eSwiUVspxOpilgVUErLuzHu3AAbgkw=",
		"github.com/pkieltyka/goinfer/backend/cuda": "sJKvWBaFWQBtGyw3f8ZiXiNFjYILvqKZ2dt6kbQanIo=",
	} {
		if !strings.Contains(string(mod), "\t"+path+" v0.6.5\n") ||
			!strings.Contains(string(sums), path+" v0.6.5 h1:"+sum+"\n") {
			t.Errorf("requalify the fp16 identity when %s changes", path)
		}
	}
	for _, model := range []struct {
		row        embed.Model
		base, fp16 string
	}{
		{Entry().Model, "79cd8cb6d605c915fb0ea74c3680816bd15df66e5db70ad0de1bf13df463de48", "59699e22cef7fac07877a0993172a09be79c857777c454a58f2d2ad8312c1686"},
		{GGUFEntries()[0].Model, "9ebb0de1d547aee1abffec1288639ce596a06b0c32447d502782a474118cf691", "a324c317d5433fc2bb667134b7c7de66ae2f6f593956a20f389d643c59c7fd89"},
	} {
		if model.row.Fingerprint != model.base || model.row.CUDAFP16Fingerprint != model.fp16 {
			t.Errorf("%s has an unexpected identity: %s %s", model.row.Name, model.row.Fingerprint, model.row.CUDAFP16Fingerprint)
		}
		updated := model.row
		updated.CUDAFP16Fingerprint = fp16Fingerprint(updated.Fingerprint, "next revision")
		if updated.CUDAFP16Fingerprint == model.fp16 || embed.FolderName(updated) != embed.FolderName(model.row) {
			t.Errorf("%s revision did not isolate vector identity from the file folder", model.row.Name)
		}
	}
}
