package main

import (
	"os"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/sys"
)

func TestModelVerityPins(t *testing.T) {
	paths := map[string]string{
		"bge-small-en-v1.5":           "PBRAIN_TEST_BGE_MODEL",
		"embeddinggemma-2":            "PBRAIN_TEST_GEMMA2_PACK",
		"embeddinggemma-2-q8-0":       "PBRAIN_TEST_GEMMA2_Q8",
		"embeddinggemma-2-ud-q6-k-xl": "PBRAIN_TEST_GEMMA2_Q6",
	}
	checkBytes := os.Getenv("PBRAIN_TEST_MODEL_VERITY") == "1"
	reg := releaseModels()
	for _, name := range reg.Names() {
		t.Run(name, func(t *testing.T) {
			m, _ := reg.Lookup(name)
			for _, f := range m.Source {
				if f.VeritySHA256 != [32]byte{} {
					t.Errorf("download-only %s must not have a verity pin", f.Name)
				}
			}
			for _, f := range m.Files {
				if f.SHA256 == [32]byte{} || f.VeritySHA256 == [32]byte{} || f.Size <= 0 {
					t.Errorf("invalid release pins for %s", f.Name)
				}
			}
			if !checkBytes {
				return
			}
			key, ok := paths[name]
			path := os.Getenv(key)
			if !ok || path == "" {
				t.Fatalf("release pin check needs the real %s fixture (%s)", name, key)
			}
			d, err := sys.OpenDir(path)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			artifact, err := embed.Verify(sys.Native(), d, m)
			if err != nil {
				t.Fatal(err)
			}
			for i, f := range m.Files {
				if got := embed.VerityDigest(artifact.Files[i]); got != f.VeritySHA256 {
					t.Errorf("%s verity digest = %x, pinned %x", f.Name, got, f.VeritySHA256)
				}
			}
		})
	}
}
