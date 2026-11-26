package gemma2

import (
	"errors"
	"fmt"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

const (
	metalDeviceID        = "metal:0"
	metalStockProfile    = "metal-eg2-stock-v1"
	metalCombinedProfile = "metal-eg2-combined-v1"
	metalBatchSelector   = "metal-eg2-batch"
	metalBatchInputs     = 4
	metalBatchPositions  = 2748
)

func metalDefault(device embeddinggemma2.DeviceInfo) bool {
	return device.Backend == "metal" && device.ID == "" && device.Alias == ""
}

func systemDefaultMetal(device embeddinggemma2.DeviceInfo) bool {
	return metalDefault(device) && !device.TextBatching
}

func attachMetal(model *embeddinggemma2.Encoder, batch bool, override int64) (string, int64, error) {
	selector := "metal"
	if batch {
		selector = metalBatchSelector
	}
	acc, err := model.UseAccelerator(selector)
	if batch && errors.Is(err, embeddinggemma2.ErrUnsupportedSelector) {
		return "", 0, fmt.Errorf("%w: device %s cannot run selector %q: %w", embed.ErrAccelerator, metalDeviceID, selector, err)
	}
	if err != nil {
		return "", 0, fmt.Errorf("%w: attach %s: %w", embed.ErrAccelerator, metalDeviceID, err)
	}
	profile, err := attachedMetalProfile(acc, batch)
	var budget int64
	if err == nil && batch {
		budget, err = deviceBudget(acc, metalDeviceID, override)
	}
	return profile, budget, closeRejectedAttachment(acc, metalDeviceID, err)
}

func attachedMetalProfile(acc embeddinggemma2.Accelerator, batch bool) (string, error) {
	reporter, ok := acc.(embeddinggemma2.AttachedDeviceReporter)
	var device embeddinggemma2.DeviceInfo
	if ok {
		device = reporter.AttachedDevice()
	}
	if !metalDefault(device) || device.TextBatching != batch {
		return "", fmt.Errorf("%w: attached device is not the Metal system default", embed.ErrAccelerator)
	}
	var profile string
	if reporter, ok := acc.(embeddinggemma2.AttachedProfileReporter); ok {
		profile = reporter.AttachedProfile()
	}
	if profile == metalCombinedProfile || !batch && profile == metalStockProfile {
		return profile, nil
	}
	return "", fmt.Errorf("%w: unrecognized Metal vector profile %q", embed.ErrAccelerator, profile)
}
