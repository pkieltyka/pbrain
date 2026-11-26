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

func attachMetal(model *embeddinggemma2.Encoder, batch bool, override int64) (string, int64, bool, error) {
	return attachMetalRoute(model.UseAccelerator, batch, override)
}

func attachMetalRoute(use func(string) (embeddinggemma2.Accelerator, error), batch bool, override int64) (string, int64, bool, error) {
	if batch {
		acc, err := use(metalBatchSelector)
		switch {
		case errors.Is(err, embeddinggemma2.ErrUnsupportedSelector):
			// The system-default device can still run the singleton selector.
		case err != nil:
			return "", 0, false, fmt.Errorf("%w: attach %s: %w", embed.ErrAccelerator, metalDeviceID, err)
		default:
			profile, err := attachedMetalProfile(acc, true)
			if err == nil {
				budget, err := deviceBudget(acc, metalDeviceID, override)
				if err != nil {
					return "", 0, false, closeRejectedAttachment(acc, metalDeviceID, err)
				}
				return profile, budget, true, nil
			}
			var cleanup *attachCleanupError
			if reject := closeRejectedAttachment(acc, metalDeviceID, err); errors.As(reject, &cleanup) {
				return "", 0, false, reject
			}
		}
	}
	acc, err := use("metal")
	if err != nil {
		return "", 0, false, fmt.Errorf("%w: attach %s: %w", embed.ErrAccelerator, metalDeviceID, err)
	}
	profile, err := attachedMetalProfile(acc, false)
	return profile, 0, false, closeRejectedAttachment(acc, metalDeviceID, err)
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
