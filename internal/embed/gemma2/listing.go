package gemma2

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

// chooseRuntime leaves CPU requests free of device probes. A named request
// opens only the resolved device, while auto tries the ordered list.
func chooseRuntime(ctx context.Context, selector string, weightFloor int64, compiled bool,
	probe func() embed.Listing, attach func(embed.DeviceInfo) error) (embed.RunInfo, error) {
	cpu := embed.RunInfo{Device: "cpu"}
	switch selector {
	case "":
		return cpu, nil
	case "cpu":
		cpu.Fallback = embed.FallbackAsked
		return cpu, nil
	case "auto":
		if !compiled {
			cpu.Fallback = embed.FallbackNoSupport
			return cpu, nil
		}
	}
	if err := ctx.Err(); err != nil {
		return embed.RunInfo{}, err
	}
	listing := probe()
	if err := ctx.Err(); err != nil {
		return embed.RunInfo{}, err
	}
	if selector != "auto" {
		for _, device := range listing.Devices {
			if selector == device.Selector() {
				if err := attach(device); err != nil {
					return embed.RunInfo{}, err
				}
				return embed.RunInfo{Device: device.ID, Name: device.Name}, nil
			}
		}
		backend, _, _ := strings.Cut(selector, ":")
		if err := listing.Failed[backend]; err != nil {
			return embed.RunInfo{}, fmt.Errorf("%w: %w", embed.ErrAccelerator, err)
		}
		return embed.RunInfo{}, fmt.Errorf("%w: device %q disappeared before attachment", embed.ErrAccelerator, selector)
	}

	cpu.Fallback = embed.FallbackNoDevice
	if len(listing.Failed) > 0 {
		cpu.Fallback = embed.FallbackFailed
	}
	for _, device := range listing.Sorted() {
		if err := ctx.Err(); err != nil {
			return embed.RunInfo{}, err
		}
		if device.Memory == "live" && device.FreeBytes < weightFloor {
			if cpu.Fallback != embed.FallbackFailed {
				cpu.Fallback = embed.FallbackMemory
			}
			continue
		}
		if err := attach(device); err != nil {
			cpu.Fallback = embed.FallbackFailed
			continue
		}
		return embed.RunInfo{Device: device.ID, Name: device.Name}, nil
	}
	return cpu, ctx.Err()
}

func listCUDADevices() embed.Listing {
	return embed.Listing{Devices: cudaDevices()}
}

func listDevices() embed.Listing {
	listing := listCUDADevices()
	devices, err := vulkanDevices()
	if err != nil {
		if !vulkanAbsent(err) {
			listing.Failed = map[string]error{"vulkan": err}
		}
		return listing
	}
	listing.Devices = append(listing.Devices, devices...)
	return listing
}

// The currently pinned Vulkan module uses one sentinel for absent and broken
// discovery. Its error messages distinguish these cases until the API does.
func vulkanAbsent(err error) bool {
	if !errors.Is(err, embeddinggemma2.ErrDeviceUnavailable) {
		return false
	}
	message := err.Error()
	if strings.Contains(message, "open libvulkan.so.1:") {
		return true
	}
	code, ok := createInstanceResult(message)
	if !ok {
		return false
	}
	switch code {
	case -9, -3, -6, -7:
		return true
	}
	return false
}

// createInstanceResult reads the VkResult of a failed vkCreateInstance from the pinned module's
// message, which gives it either as "VkResult -9" or by name as "VK_ERROR_INCOMPATIBLE_DRIVER (-9)".
func createInstanceResult(message string) (int, bool) {
	_, result, found := strings.Cut(message, "create instance: ")
	if !found {
		return 0, false
	}
	fields := strings.Fields(result)
	switch {
	case len(fields) >= 2 && fields[0] == "VkResult":
		code, err := strconv.Atoi(fields[1])
		return code, err == nil
	case len(fields) >= 2 && strings.HasPrefix(fields[0], "VK_") &&
		strings.HasPrefix(fields[1], "(") && strings.HasSuffix(fields[1], ")"):
		code, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(fields[1], "("), ")"))
		return code, err == nil
	}
	return 0, false
}
