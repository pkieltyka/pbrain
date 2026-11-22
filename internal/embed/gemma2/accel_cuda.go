//go:build cuda

package gemma2

import (
	"fmt"

	_ "github.com/pkieltyka/goinfer/backend/cuda"
	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

func attachAccelerator(m *embeddinggemma2.Encoder) error {
	acc, err := m.UseAccelerator(cudaSelector)
	if err != nil {
		return fmt.Errorf("%w: attach %s: %w", embed.ErrAccelerator, cudaSelector, err)
	}
	if acc == nil || m.Accelerator() == nil || acc.Name() != "cuda" {
		return fmt.Errorf("%w: %s did not attach as cuda", embed.ErrAccelerator, cudaSelector)
	}
	return nil
}
