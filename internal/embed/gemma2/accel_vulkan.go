//go:build linux && amd64 && vulkan

package gemma2

import (
	"fmt"
	"math"

	"github.com/pkieltyka/goinfer/backend/vulkan"
	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

const vulkanCompiled = true

func vulkanDevices() ([]embed.DeviceInfo, error) {
	found, err := vulkan.Devices()
	if err != nil {
		return nil, err
	}
	devices := make([]embed.DeviceInfo, 0, len(found))
	for _, d := range found {
		if d.ID == "" || d.Alias == "" {
			continue
		}
		devices = append(devices, embed.DeviceInfo{
			ID: d.Alias, Persistent: d.ID, Backend: "vulkan", Name: d.Name,
			TotalBytes: int64(min(d.TotalLocalBytes, math.MaxInt64)),
			Memory:     "unknown", MemoryKind: "dedicated",
		})
	}
	return devices, nil
}

func attachVulkan(model *embeddinggemma2.Encoder, selector string) error {
	acc, err := model.UseAccelerator(selector)
	if err != nil {
		return fmt.Errorf("%w: attach %s: %w", embed.ErrAccelerator, selector, err)
	}
	d, ok := vulkan.AttachedDevice(acc)
	if !ok || d.ID != selector {
		return fmt.Errorf("%w: Vulkan returned device %q for %q", embed.ErrAccelerator, d.ID, selector)
	}
	return nil
}
