//go:build !cuda

package gemma2

import (
	"fmt"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

const cudaCompiled = false

func cudaDevices() []embed.DeviceInfo { return nil }

func attachAccelerator(*embeddinggemma2.Encoder) error {
	return fmt.Errorf("%w: CUDA is not compiled in", embed.ErrAccelerator)
}
