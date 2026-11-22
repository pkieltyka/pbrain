//go:build cuda || (linux && amd64 && vulkan)

package gemma2

import (
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/semantic"
)

func sameVectorBits(a, b []float32) bool {
	return slices.EqualFunc(a, b, func(a, b float32) bool {
		return math.Float32bits(a) == math.Float32bits(b)
	})
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
