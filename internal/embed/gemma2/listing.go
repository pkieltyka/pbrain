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
			var cleanup *attachCleanupError
			if errors.As(err, &cleanup) {
				return embed.RunInfo{}, err
			}
			cpu.Fallback = embed.FallbackFailed
			continue
		}
		return embed.RunInfo{Device: device.ID, Name: device.Name}, nil
	}
	return cpu, ctx.Err()
}

func listCUDADevices() embed.Listing {
	return probeDevices(false)
}

func listDevices() embed.Listing {
	return probeDevices(true)
}

func probeDevices(allowVulkan bool) embed.Listing {
	devices, failures := embeddinggemma2.Devices(embeddinggemma2.ProbeOptions{IncludeMemory: true})
	return deviceListing(devices, failures, allowVulkan)
}

func deviceListing(devices []embeddinggemma2.DeviceInfo, failures []embeddinggemma2.ProbeFailure, allowVulkan bool) embed.Listing {
	allowed := func(backend string) bool { return backend == "cuda" || allowVulkan && backend == "vulkan" }
	ids := make(map[string]int, len(devices))
	aliases := make(map[string]int, len(devices))
	for _, device := range devices {
		ids[strings.ToLower(device.ID)]++
		aliases[device.Alias]++
	}
	var listing embed.Listing
	for _, device := range devices {
		prefix := device.Backend + ":"
		if !allowed(device.Backend) || !device.TextBatching ||
			!strings.HasPrefix(device.ID, prefix) || !strings.HasPrefix(device.Alias, prefix) ||
			device.ID == prefix || device.Alias == prefix ||
			ids[strings.ToLower(device.ID)] != 1 || aliases[device.Alias] != 1 {
			continue
		}
		memoryKind := "unknown"
		switch device.MemoryKind {
		case embeddinggemma2.MemoryDedicated:
			memoryKind = "dedicated"
		case embeddinggemma2.MemoryShared:
			memoryKind = "shared"
		}
		listing.Devices = append(listing.Devices, embed.DeviceInfo{
			ID: device.Alias, Persistent: device.ID, Backend: device.Backend, Name: device.Name,
			TotalBytes: device.Memory.TotalBytes, FreeBytes: device.Memory.AvailableBytes,
			Memory: device.Memory.Basis.String(), MemoryKind: memoryKind,
			ComputeCapability: device.ComputeCapability, RunnableSelectors: device.RunnableSelectors(),
		})
	}
	for _, failure := range failures {
		if !allowed(failure.Backend) {
			continue
		}
		if err := failedProbe(failure.Backend, failure.Err); err != nil {
			if listing.Failed == nil {
				listing.Failed = make(map[string]error)
			}
			listing.Failed[failure.Backend] = errors.Join(listing.Failed[failure.Backend], err)
		}
	}
	return listing
}

// failedProbe discards absent components of joined inventory errors. A typed
// classification from fmt.Errorf can wrap a category and its cause together;
// keep that pair together rather than treating the native cause as a failure.
func failedProbe(backend string, err error) error {
	if err == nil {
		return nil
	}
	if multi, ok := err.(interface{ Unwrap() []error }); ok {
		children := multi.Unwrap()
		messages := make([]string, len(children))
		for i, child := range children {
			messages[i] = child.Error()
		}
		if err.Error() == strings.Join(messages, "\n") {
			var failed []error
			for _, child := range children {
				failed = append(failed, failedProbe(backend, child))
			}
			return errors.Join(failed...)
		}
	}
	// CUDA's per-device failures are contextualized with the device ID, even
	// when the driver reports no device during a memory or cleanup operation.
	if backend == "cuda" && strings.HasPrefix(err.Error(), "cuda:") {
		return err
	}
	if errors.Is(err, embeddinggemma2.ErrNoDevice) ||
		backend == "cuda" && errors.Is(err, embeddinggemma2.ErrBackendUnavailable) ||
		backend == "vulkan" && vulkanAbsent(err) {
		return nil
	}
	return err
}

// The currently pinned Vulkan module uses one sentinel for absent and broken
// discovery. Its error messages distinguish these cases until the API does.
func vulkanAbsent(err error) bool {
	if !errors.Is(err, embeddinggemma2.ErrBackendUnavailable) {
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
