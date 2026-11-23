package gemma2

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

func TestDeviceSelection(t *testing.T) {
	attachErr := fmt.Errorf("%w: test attach failed", embed.ErrAccelerator)
	device := []embed.DeviceInfo{{ID: "cuda:0", Backend: "cuda", Memory: "live", FreeBytes: 100}}
	vulkan := embed.DeviceInfo{ID: "vulkan:0", Persistent: "vulkan:0123456789abcdef0123456789abcdef",
		Backend: "vulkan", Memory: "unknown", MemoryKind: "dedicated"}
	listingErr := errors.New("vulkan: create instance: VkResult -2")
	for _, test := range []struct {
		name, selector string
		compiled       bool
		devices        []embed.DeviceInfo
		failed         map[string]error
		attachErr      error
		attachErrID    string
		probe, attach  bool
		want           embed.RunInfo
		wantErr        error
		order          []string
	}{
		{name: "CPU query", compiled: true, devices: device, want: embed.RunInfo{Device: "cpu"}},
		{name: "CPU asked for", selector: "cpu", compiled: true, devices: device, want: embed.RunInfo{Device: "cpu", Fallback: embed.FallbackAsked}},
		{name: "untagged", selector: "auto", want: embed.RunInfo{Device: "cpu", Fallback: embed.FallbackNoSupport}},
		{name: "no driver", selector: "auto", compiled: true, probe: true, want: embed.RunInfo{Device: "cpu", Fallback: embed.FallbackNoDevice}},
		{name: "too small", selector: "auto", compiled: true, probe: true,
			devices: []embed.DeviceInfo{{ID: "cuda:0", Backend: "cuda", Memory: "live", FreeBytes: 0}},
			want:    embed.RunInfo{Device: "cpu", Fallback: embed.FallbackMemory}},
		{name: "auto attach fails", selector: "auto", compiled: true, devices: device, attachErr: attachErr,
			probe: true, attach: true, want: embed.RunInfo{Device: "cpu", Fallback: embed.FallbackFailed}},
		{name: "auto success", selector: "auto", compiled: true, devices: device,
			probe: true, attach: true, want: embed.RunInfo{Device: "cuda:0"}},
		{name: "low estimate still tries", selector: "auto", compiled: true, probe: true, attach: true,
			devices: []embed.DeviceInfo{{ID: "cuda:0", Backend: "cuda", Memory: "estimated", FreeBytes: 1}},
			want:    embed.RunInfo{Device: "cuda:0"}},
		{name: "named success", selector: "cuda:0", compiled: true, devices: device, probe: true, attach: true,
			want: embed.RunInfo{Device: "cuda:0"}},
		{name: "named attach fails", selector: "cuda:0", compiled: true, devices: device, probe: true,
			attachErr: attachErr, attach: true, wantErr: attachErr},
		{name: "known free precedes unknown dedicated capacity", selector: "auto", compiled: true,
			devices: []embed.DeviceInfo{vulkan, device[0]}, probe: true, attach: true,
			order: []string{"cuda:0"}, want: embed.RunInfo{Device: "cuda:0"}},
		{name: "auto retries next device", selector: "auto", compiled: true,
			devices: []embed.DeviceInfo{vulkan, device[0]}, probe: true, attach: true, attachErr: attachErr, attachErrID: "cuda:0",
			order: []string{"cuda:0", vulkan.Persistent}, want: embed.RunInfo{Device: "vulkan:0"}},
		{name: "named uses the UUID", selector: vulkan.Persistent, compiled: true,
			devices: []embed.DeviceInfo{device[0], vulkan}, probe: true, attach: true,
			order: []string{vulkan.Persistent}, want: embed.RunInfo{Device: "vulkan:0"}},
		{name: "named cannot change devices after inventory", selector: vulkan.Persistent, compiled: true,
			devices: device, probe: true, wantErr: embed.ErrAccelerator},
		{name: "named second listing failed", selector: vulkan.Persistent, compiled: true, probe: true,
			failed: map[string]error{"vulkan": listingErr}, wantErr: listingErr},
		{name: "auto failed listing", selector: "auto", compiled: true, probe: true,
			failed: map[string]error{"vulkan": listingErr}, want: embed.RunInfo{Device: "cpu", Fallback: embed.FallbackFailed}},
		{name: "auto ignores failed backend after attachment", selector: "auto", compiled: true, probe: true,
			failed: map[string]error{"vulkan": listingErr}, devices: device, attach: true, want: embed.RunInfo{Device: "cuda:0"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			probed, attached := false, false
			var order []string
			info, err := chooseRuntime(t.Context(), test.selector, 50, test.compiled, func() embed.Listing {
				probed = true
				return embed.Listing{Devices: test.devices, Failed: test.failed}
			}, func(d embed.DeviceInfo) error {
				attached = true
				order = append(order, d.Selector())
				if test.attachErrID == "" || test.attachErrID == d.ID {
					return test.attachErr
				}
				return nil
			})
			if info != test.want || !errors.Is(err, test.wantErr) || probed != test.probe || attached != test.attach ||
				(test.order != nil && !slices.Equal(order, test.order)) {
				t.Fatalf("runtime = (%+v, %v), probe=%t attach=%t order=%v", info, err, probed, attached, order)
			}
		})
	}
}

func TestVulkanAbsent(t *testing.T) {
	for _, test := range []struct {
		message string
		want    bool
	}{
		{"vulkan: open libvulkan.so.1: no such file", true},
		{"create instance: VkResult -9", true},
		{"create instance: VkResult -3", true},
		{"create instance: VkResult -6", true},
		{"create instance: VkResult -7", true},
		{"vulkan: runtime unavailable: vulkan: create instance: VK_ERROR_INCOMPATIBLE_DRIVER (-9)", true},
		{"create instance: VK_ERROR_INITIALIZATION_FAILED (-3)", true},
		{"create instance: VK_ERROR_OUT_OF_HOST_MEMORY (-1)", false},
		{"create instance: VK_ERROR_DEVICE_LOST (-4)", false},
		{"create instance: VK_ERROR_INCOMPATIBLE_DRIVER", false},
		{"enumerate physical devices: VK_ERROR_INITIALIZATION_FAILED (-3)", false},
		{"create instance: VkResult -1", false},
		{"create instance: VkResult -2", false},
		{"create instance: VkResult -4", false},
		{"create instance: VkResult unknown", false},
		{"create instance: VkResult ", false},
		{"enumerate physical devices: VkResult -3", false},
	} {
		err := fmt.Errorf("%w: %s", embeddinggemma2.ErrDeviceUnavailable, test.message)
		if got := vulkanAbsent(err); got != test.want {
			t.Errorf("%q absent = %v, want %v", test.message, got, test.want)
		}
	}
}
