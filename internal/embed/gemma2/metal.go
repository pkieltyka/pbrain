package gemma2

import (
	"fmt"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

const (
	metalDeviceID        = "metal:0"
	metalStockProfile    = "metal-eg2-stock-v1"
	metalCombinedProfile = "metal-eg2-combined-v1"
)

func systemDefaultMetal(device embeddinggemma2.DeviceInfo) bool {
	return device.Backend == "metal" && device.ID == "" && device.Alias == "" && !device.TextBatching
}

func attachMetal(model *embeddinggemma2.Encoder) (string, error) {
	acc, err := model.UseAccelerator("metal")
	if err != nil {
		return "", fmt.Errorf("%w: attach %s: %w", embed.ErrAccelerator, metalDeviceID, err)
	}
	profile, err := attachedMetalProfile(acc)
	return profile, closeRejectedAttachment(acc, metalDeviceID, err)
}

func attachedMetalProfile(acc embeddinggemma2.Accelerator) (string, error) {
	device, ok := acc.(embeddinggemma2.AttachedDeviceReporter)
	if !ok || !systemDefaultMetal(device.AttachedDevice()) {
		return "", fmt.Errorf("%w: attached device is not the Metal system default", embed.ErrAccelerator)
	}
	var profile string
	if reporter, ok := acc.(embeddinggemma2.AttachedProfileReporter); ok {
		profile = reporter.AttachedProfile()
	}
	switch profile {
	case metalStockProfile, metalCombinedProfile:
		return profile, nil
	default:
		return "", fmt.Errorf("%w: unrecognized Metal vector profile %q", embed.ErrAccelerator, profile)
	}
}
