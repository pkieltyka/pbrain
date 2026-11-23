package gemma2

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/embed/embedtest"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/semantic"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// testGGUFEntry returns the release row by name, so a new row cannot shift a
// test onto another model.
func testGGUFEntry(tb testing.TB, name string) embed.Entry {
	tb.Helper()
	for _, entry := range GGUFEntries() {
		if entry.Model.Name == name {
			return entry
		}
	}
	tb.Fatalf("no GGUF row named %q", name)
	return embed.Entry{}
}

func TestGemmaGGUFRows(t *testing.T) {
	rows := GGUFEntries()
	if len(rows) != 2 {
		t.Fatalf("GGUF rows = %d, want two admitted models", len(rows))
	}
	for _, want := range []struct {
		name, file, digest, verity       string
		size                             int64
		fingerprint, index, folder, fp16 string
	}{
		{
			name: q8Name, file: q8File,
			size:        309855520,
			digest:      "6f1bd4ac6c5df7444f9cca7ca36cafe6cfa34cd6f49fefb1e0b4be8143aed8bc",
			verity:      "714c9bfa33d80385452f646ed15c1915cb7032e3ff529d862d2aa5fa5d3c32d1",
			fingerprint: "9ebb0de1d547aee1abffec1288639ce596a06b0c32447d502782a474118cf691",
			index:       "c346df62410f98848d85f7a3aa18e084fd8fab8138d97515f8045fcd3988be9d",
			folder:      "embeddinggemma-2-q8-0-49b6c94be1aac068",
			fp16:        "10998f9ecb29a3398281837e1550dd3aaff9004dc0692cb1388c764f27063a7b",
		},
		{
			name: q6Name, file: q6File,
			size:        248812032,
			digest:      "dc84f042bc3ffbe6122b3b38b2fb299d66f5f37b276c9d458db85d4a4f1e6ba3",
			verity:      "f076d236929c1d930d13bf8116f29176ba80db028956f522faf004ad1823f413",
			fingerprint: "16d5093f31a4399589c717804e04d2ad3fd70df5bd42b36bf60424dc4599e2f4",
			index:       "8c964b1108dc4e42f65ea44837ce518911dc462ccb57d8fe1eba862a7e98bfa7",
			folder:      "embeddinggemma-2-ud-q6-k-xl-22e1f6123853c8cb",
			fp16:        "",
		},
	} {
		row := testGGUFEntry(t, want.name)
		m := row.Model
		if m.Name != want.name || m.Runtime != "goinfer" || m.Dim != 768 || m.Cutoff != 0.72 ||
			len(m.Files) != 1 || len(m.Source) != 0 || row.Derive != nil || row.Open == nil || row.Devices == nil {
			t.Fatalf("%s: unexpected row: %+v", want.name, m)
		}
		f := m.Files[0]
		if f.Name != want.file || f.Size != want.size ||
			f.URL != ggufURL+want.file ||
			hex.EncodeToString(f.SHA256[:]) != want.digest ||
			hex.EncodeToString(f.VeritySHA256[:]) != want.verity {
			t.Fatalf("%s: unexpected file pin: %+v", want.name, f)
		}
		if m.Fingerprint != want.fingerprint || m.CUDAFP16Fingerprint != want.fp16 ||
			semantic.IndexFingerprint(m) != want.index ||
			embed.FolderName(m) != want.folder {
			t.Fatalf("%s: unexpected identity: model %s, index %s, folder %s",
				want.name, m.Fingerprint, semantic.IndexFingerprint(m), embed.FolderName(m))
		}
	}

	// A caller must not change the release pins through the returned rows.
	for i := range rows {
		rows[i].Model.Files[0].Name = "changed"
	}
	for _, entry := range GGUFEntries() {
		if got := entry.Model.Files[0].Name; got == "changed" {
			t.Fatalf("%s: a caller changed the release file pin", entry.Model.Name)
		}
	}
}

func TestGGUFInvalidArtifact(t *testing.T) {
	wrong := []byte("invalid GGUF")
	for _, name := range []string{q8Name, q6Name} {
		t.Run(name, func(t *testing.T) {
			entry := testGGUFEntry(t, name)
			a := &embed.Artifact{Model: entry.Model, Files: [][]byte{wrong}}
			if emb, err := entry.Open(t.Context(), a, embed.OpenOptions{}); err == nil || emb != nil || a.Files != nil {
				t.Fatalf("malformed GGUF open = (%v, %v), retained files %d", emb, err, len(a.Files))
			}
			a.Files = [][]byte{wrong}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			if emb, err := entry.Open(ctx, a, embed.OpenOptions{}); !errors.Is(err, context.Canceled) || emb != nil || a.Files != nil {
				t.Fatalf("cancelled GGUF open = (%v, %v), retained files %d", emb, err, len(a.Files))
			}

			// Use a same-size fake transfer so the digest check runs without real weights.
			m := entry.Model
			m.Files[0].Size = int64(len(wrong))
			dl := embedtest.New(nil).Downloader()
			dl.Serve(m.Files[0].URL, wrong)
			path := t.TempDir()
			dest, err := sys.OpenDir(path)
			if err != nil {
				t.Fatal(err)
			}
			defer dest.Close()
			got, err := embed.Download(t.Context(), sys.Native(), dl, m, dest)
			var pe *errs.Error
			if got != nil || !errors.As(err, &pe) || pe.Code != "fetch_failed" {
				t.Fatalf("bad digest download = (%v, %v), want fetch_failed", got, err)
			}
			if _, err := os.Stat(filepath.Join(path, embed.FolderName(m), m.Files[0].Name)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unverified file retained: %v", err)
			}
		})
	}
}

func TestGGUFDevicePolicy(t *testing.T) {
	const gib = int64(1 << 30)
	row := func(backend, alias, id string, basis embeddinggemma2.MemoryBasis, free int64) embeddinggemma2.DeviceInfo {
		return embeddinggemma2.DeviceInfo{
			Backend: backend, Alias: alias, ID: id, Name: id,
			MemoryKind: embeddinggemma2.MemoryDedicated, TextBatching: true,
			Memory: embeddinggemma2.DeviceMemory{Basis: basis, TotalBytes: 20 * gib, AvailableBytes: free},
		}
	}
	raw := []embeddinggemma2.DeviceInfo{
		row("cuda", "cuda:0", "cuda:GPU-zero", embeddinggemma2.MemoryLive, 8*gib),
		row("vulkan", "vulkan:0", "vulkan:uuid0", embeddinggemma2.MemoryEstimated, 19*gib),
	}
	withVulkan := deviceListing(raw, nil, true)
	if len(withVulkan.Devices) != 2 {
		t.Fatalf("Vulkan-enabled listing = %+v", withVulkan)
	}
	if got := withVulkan.Devices[1]; got.Backend != "vulkan" || got.ID != "vulkan:0" ||
		got.Persistent != "vulkan:uuid0" || got.FreeBytes != 19*gib {
		t.Fatalf("unexpected Vulkan row: %+v", got)
	}
	cudaOnly := deviceListing(raw, nil, false)
	if len(cudaOnly.Devices) != 1 || cudaOnly.Devices[0].Backend != "cuda" {
		t.Fatalf("CUDA-only listing = %+v", cudaOnly)
	}

	// Both rows accept a Vulkan inventory injected through the shared builder.
	listing := embed.Listing{Devices: []embed.DeviceInfo{
		{ID: "cuda:0", Persistent: "cuda:GPU-zero", Backend: "cuda", Name: "CUDA"},
		{ID: "vulkan:0", Persistent: "vulkan:uuid0", Backend: "vulkan", Name: "Radeon"},
	}}
	for _, entry := range ggufEntries(func() embed.Listing { return listing }) {
		got := entry.Devices()
		if len(got.Devices) != 2 || got.Devices[1].Backend != "vulkan" {
			t.Fatalf("%s does not list the Vulkan inventory: %+v", entry.Model.Name, got)
		}
	}

	// Production rows use the Vulkan-enabled inventory, not the CUDA-only one.
	for _, entry := range GGUFEntries() {
		if got, want := reflect.ValueOf(entry.Devices).Pointer(), reflect.ValueOf(listDevices).Pointer(); got != want {
			t.Fatalf("%s does not use listDevices", entry.Model.Name)
		}
	}
}

func TestGGUFNamedDeviceResolution(t *testing.T) {
	entry := testGGUFEntry(t, q8Name)
	reg := embed.NewRegistry(entry)
	cudaOnly := embed.Listing{Devices: []embed.DeviceInfo{
		{ID: "cuda:0", Persistent: "cuda:GPU-zero", Backend: "cuda", Name: "CUDA"},
	}}
	withVulkan := embed.Listing{Devices: append(cudaOnly.Devices, embed.DeviceInfo{
		ID: "vulkan:0", Persistent: "vulkan:uuid0", Backend: "vulkan", Name: "Radeon",
	})}
	selectors := []string{"vulkan", "vulkan:0", "vulkan:uuid0"}

	for _, selector := range selectors {
		a := &embed.Artifact{Model: entry.Model, Files: [][]byte{[]byte("invalid GGUF")}}
		got, err := reg.Open(t.Context(), a, embed.OpenOptions{
			Device: selector, Writer: embed.WriteProfile{Inventory: &cudaOnly},
		})
		var pe *errs.Error
		if got != nil || !errors.As(err, &pe) || pe.Code != "invalid_config" || a.Files != nil {
			t.Fatalf("%s on a CUDA-only inventory = (%v, %v), want invalid_config", selector, got, err)
		}
	}

	for _, selector := range selectors {
		a := &embed.Artifact{Model: entry.Model, Files: [][]byte{[]byte("invalid GGUF")}}
		got, err := reg.Open(t.Context(), a, embed.OpenOptions{
			Device: selector, Writer: embed.WriteProfile{Inventory: &withVulkan},
		})
		var pe *errs.Error
		if got != nil || err == nil || a.Files != nil ||
			errors.As(err, &pe) && pe.Code == "invalid_config" {
			t.Fatalf("%s on a Vulkan inventory = (%v, %v), want the selector accepted", selector, got, err)
		}
	}
}
