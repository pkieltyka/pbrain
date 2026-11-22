//go:build !linux || !amd64 || !vulkan

package gemma2

import (
	"fmt"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

const vulkanCompiled = false

func vulkanDevices() ([]embed.DeviceInfo, error) { return nil, nil }

func attachVulkan(*embeddinggemma2.Encoder, string) error {
	return fmt.Errorf("%w: Vulkan is not compiled in", embed.ErrAccelerator)
}
