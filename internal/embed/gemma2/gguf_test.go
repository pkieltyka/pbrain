package gemma2

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/embed/embedtest"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/semantic"
	"github.com/pkieltyka/pbrain/internal/sys"
)

func TestGemmaGGUFRows(t *testing.T) {
	rows := GGUFEntries()
	if len(rows) != 1 {
		t.Fatalf("GGUF rows = %d, want one admitted model", len(rows))
	}
	row := rows[0]
	m := row.Model
	if m.Name != "embeddinggemma-2-q8-0" || m.Runtime != "goinfer" || m.Dim != 768 || m.Cutoff != 0.72 || len(m.Files) != 1 || len(m.Source) != 0 || row.Derive != nil {
		t.Fatalf("unexpected Q8 row: %+v", m)
	}
	f := m.Files[0]
	if f.Name != "embeddinggemma-2-Q8_0.gguf" || f.Size != 309855520 ||
		f.URL != "https://huggingface.co/unsloth/embeddinggemma-2-GGUF/resolve/031f0d4b35536f69ab3509d4893c923264fcf253/embeddinggemma-2-Q8_0.gguf" ||
		hex.EncodeToString(f.SHA256[:]) != "6f1bd4ac6c5df7444f9cca7ca36cafe6cfa34cd6f49fefb1e0b4be8143aed8bc" {
		t.Fatalf("unexpected Q8 file: %+v", f)
	}
	if m.Fingerprint != "9ebb0de1d547aee1abffec1288639ce596a06b0c32447d502782a474118cf691" ||
		semantic.IndexFingerprint(m) != "c346df62410f98848d85f7a3aa18e084fd8fab8138d97515f8045fcd3988be9d" ||
		embed.FolderName(m) != "embeddinggemma-2-q8-0-49b6c94be1aac068" {
		t.Fatalf("unexpected Q8 identity: model %s, index %s, folder %s", m.Fingerprint, semantic.IndexFingerprint(m), embed.FolderName(m))
	}
	m.Files[0].Name = "changed"
	if got := GGUFEntries()[0].Model.Files[0]; got != f {
		t.Fatalf("a caller changed the release file pin: %+v", got)
	}
}

func TestQ8InvalidArtifact(t *testing.T) {
	entry := GGUFEntries()[0]
	wrong := []byte("invalid GGUF")
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
}

func TestQ8RejectsVulkan(t *testing.T) {
	entry := GGUFEntries()[0]
	// The registry refuses the selector before attempting to decode the file.
	reg := embed.NewRegistry(entry)
	for _, device := range []string{"vulkan", "vulkan:0", "vulkan:000000000c0000000000000000000000"} {
		a := &embed.Artifact{Model: entry.Model, Files: [][]byte{[]byte("invalid GGUF")}}
		got, err := reg.Open(t.Context(), a, embed.OpenOptions{Device: device})
		var pe *errs.Error
		if got != nil || !errors.As(err, &pe) || pe.Code != "invalid_config" || a.Files != nil {
			t.Fatalf("%s open = (%v, %v), want invalid_config and released artifact", device, got, err)
		}
	}
}
