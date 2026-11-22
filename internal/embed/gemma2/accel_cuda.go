//go:build cuda

package gemma2

import (
	"fmt"

	_ "github.com/pkieltyka/goinfer/backend/cuda"
	"github.com/pkieltyka/goinfer/decoder"
	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

const cudaCompiled = true

func cudaDevices() []embed.DeviceInfo {
	if free, ok := decoder.FreeBytesFor("cuda"); ok {
		return []embed.DeviceInfo{{ID: "cuda:0", Backend: "cuda", FreeBytes: free, Memory: "live", MemoryKind: "unknown"}}
	}
	return nil
}

func attachAccelerator(m *embeddinggemma2.Encoder) error {
	acc, err := m.UseAccelerator(cudaSelector)
	if err != nil {
		return fmt.Errorf("%w: attach %s: %w", embed.ErrAccelerator, cudaSelector, err)
	}
	if acc == nil {
		return fmt.Errorf("%w: %s did not attach as cuda", embed.ErrAccelerator, cudaSelector)
	}
	return nil
}
