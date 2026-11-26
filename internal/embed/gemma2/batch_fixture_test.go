//go:build cuda || (linux && amd64 && vulkan) || (darwin && arm64 && metal)

package gemma2

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/semantic"
)

// layoutReceipt records the same-device byte comparison and same-file CPU parity.
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

// batchFixture uses the 32 notes from the Phase 28 parity fixture.
func batchFixture(t *testing.T) []string {
	t.Helper()
	return batchFixtureAt(t, "../../semantic/calibrate/testdata/retrieval/brain", true)
}

func batchFixtureAt(t *testing.T, root string, phase28 bool) []string {
	t.Helper()
	var texts []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		if phase28 {
			switch d.Name() {
			case "bge-hugot-old.md", "flat-vectors-old.md", "output-budget-old.md":
				return nil
			}
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
	if err != nil || len(texts) == 0 || phase28 && len(texts) != 59 {
		t.Fatalf("batch fixture: %d passages, %v", len(texts), err)
	}
	return texts
}
