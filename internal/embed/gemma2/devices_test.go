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
		err := fmt.Errorf("%w: %s", embeddinggemma2.ErrBackendUnavailable, test.message)
		if got := vulkanAbsent(err); got != test.want {
			t.Errorf("%q absent = %v, want %v", test.message, got, test.want)
		}
	}
}

func TestUnifiedDeviceListing(t *testing.T) {
	row := func(backend, alias, id string, basis embeddinggemma2.MemoryBasis, free int64) embeddinggemma2.DeviceInfo {
		return embeddinggemma2.DeviceInfo{
			Backend: backend, Alias: alias, ID: id, Name: id,
			MemoryKind: embeddinggemma2.MemoryDedicated, TextBatching: true,
			Memory: embeddinggemma2.DeviceMemory{Basis: basis, TotalBytes: 20 * gib, AvailableBytes: free},
		}
	}
	vk := row("vulkan", "vulkan:0", "vulkan:uuid0", embeddinggemma2.MemoryEstimated, 19*gib)
	cuda0 := row("cuda", "cuda:0", "cuda:GPU-zero", embeddinggemma2.MemoryLive, 0)
	cuda1 := row("cuda", "cuda:1", "cuda:GPU-one", embeddinggemma2.MemoryLive, 8*gib)
	mask := row("cuda", "", "", embeddinggemma2.MemoryUnknown, 0)
	dupe := row("cuda", "cuda:2", "cuda:GPU-two", embeddinggemma2.MemoryLive, 3*gib)
	dupe2 := row("cuda", "cuda:3", "cuda:gpu-two", embeddinggemma2.MemoryLive, 3*gib)
	single := row("vulkan", "vulkan:1", "vulkan:uuid1", embeddinggemma2.MemoryUnknown, 0)
	single.TextBatching = false
	raw := []embeddinggemma2.DeviceInfo{vk, cuda0, cuda1, mask, dupe, dupe2, single}
	closeFailure := errors.New("close temporary context failed")
	listFailure := errors.New("vulkan: enumerate physical devices: VkResult -3")
	absent := fmt.Errorf("%w: create instance: VkResult -9", embeddinggemma2.ErrBackendUnavailable)
	failures := []embeddinggemma2.ProbeFailure{
		{Backend: "cuda", Err: errors.Join(embeddinggemma2.ErrNoDevice, closeFailure)},
		{Backend: "vulkan", Err: errors.Join(absent, listFailure)},
	}
	listing := deviceListing(raw, failures, true)
	devices := listing.Sorted()
	if len(devices) != 3 || devices[0].Persistent != cuda1.ID || devices[1].Persistent != cuda0.ID ||
		devices[1].Memory != "live" || devices[1].FreeBytes != 0 || devices[2].Persistent != vk.ID ||
		devices[2].Memory != "estimated" || devices[2].FreeBytes != 19*gib ||
		devices[2].TotalBytes != 20*gib || devices[2].MemoryKind != "dedicated" {
		t.Fatalf("unexpected listing: %+v", devices)
	}
	if err := listing.Failed["cuda"]; !errors.Is(err, closeFailure) || errors.Is(err, embeddinggemma2.ErrNoDevice) {
		t.Fatalf("CUDA partial failure: %v", err)
	}
	if err := listing.Failed["vulkan"]; !errors.Is(err, listFailure) || errors.Is(err, absent) {
		t.Fatalf("Vulkan partial failure: %v", err)
	}
	q8 := deviceListing(raw, failures, false)
	if len(q8.Devices) != 2 || len(q8.Failed) != 1 || !errors.Is(q8.Failed["cuda"], closeFailure) {
		t.Fatalf("Q8 listing includes an ineligible backend: %+v", q8)
	}
	if err := failedProbe("cuda", fmt.Errorf("%w: missing library", embeddinggemma2.ErrBackendUnavailable)); err != nil {
		t.Fatalf("missing CUDA driver: %v", err)
	}
	if err := failedProbe("cuda", fmt.Errorf("cuda:GPU-one memory: %w", embeddinggemma2.ErrBackendUnavailable)); err == nil {
		t.Fatal("lost a partial memory-probe error")
	}
}

type fakeInventoryAccel struct {
	fakeSingleAccel
	id        string
	memory    embeddinggemma2.DeviceMemory
	memoryErr error
	reads     int
}

func (a *fakeInventoryAccel) AttachedDevice() embeddinggemma2.DeviceInfo {
	return embeddinggemma2.DeviceInfo{ID: a.id, Alias: "testgpu:9"}
}

func (a *fakeInventoryAccel) DeviceMemory() (embeddinggemma2.DeviceMemory, error) {
	a.reads++
	return a.memory, a.memoryErr
}

func TestAttachedBudget(t *testing.T) {
	const id = "testgpu:persistent-id"
	device := embed.DeviceInfo{
		ID: "testgpu:0", Persistent: id, Backend: "testgpu", Memory: "live", FreeBytes: 20 * gib,
	}
	readErr := errors.New("driver memory query failed")
	closeErr := errors.New("driver close failed")
	for _, test := range []struct {
		name, attachedID  string
		memoryErr         error
		closeErr          error
		want, override    int64
		wantReads, closes int
	}{
		{name: "reads attached device", attachedID: id, want: 600 * (1 << 20), wantReads: 1},
		{name: "clamps to attached live figure", attachedID: id, override: 4 * gib, want: 2000 * (1 << 20), wantReads: 1},
		{name: "rejects remapped identity", attachedID: "testgpu:other", closes: 1},
		{name: "read fails with cleanup", attachedID: id, memoryErr: readErr, wantReads: 1, closes: 1},
		{name: "read fails and retains handle", attachedID: id, memoryErr: readErr, closeErr: closeErr, wantReads: 1, closes: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, model := tinyEncoderForTest(t)
			acc := &fakeInventoryAccel{
				fakeSingleAccel: fakeSingleAccel{name: device.Backend, closeErrs: []error{test.closeErr}},
				id:              test.attachedID, memory: embeddinggemma2.DeviceMemory{
					Basis: embeddinggemma2.MemoryLive, AvailableBytes: 2000 * (1 << 20),
				}, memoryErr: test.memoryErr,
			}
			embeddinggemma2.RegisterDeviceAccelerator(device.Backend, device.Backend,
				func(_ *embeddinggemma2.Model, token string) (embeddinggemma2.Accelerator, error) {
					if token != "persistent-id" {
						t.Fatalf("attached by movable alias: %q", token)
					}
					return acc, nil
				})
			got, err := attachDevice(model, device, test.override, "")
			if got != test.want || acc.reads != test.wantReads || acc.closed != test.closes ||
				(test.closes > 0) != errors.Is(err, embed.ErrAccelerator) ||
				test.memoryErr != nil && !errors.Is(err, test.memoryErr) {
				t.Fatalf("attach = %d, %v, reads=%d closes=%d", got, err, acc.reads, acc.closed)
			}
			if test.closeErr != nil {
				var cleanup *attachCleanupError
				if !errors.As(err, &cleanup) || !errors.Is(err, test.closeErr) || cleanup.acc != acc {
					t.Fatalf("failed close lost its handle or cause: %v", err)
				}
				if _, err := chooseRuntime(t.Context(), "auto", 0, true, func() embed.Listing {
					return embed.Listing{Devices: []embed.DeviceInfo{device, {ID: "testgpu:1"}}}
				}, func(d embed.DeviceInfo) error {
					if d != device {
						t.Fatal("attempted another device while cleanup was pending")
					}
					return cleanup
				}); !errors.Is(err, test.closeErr) {
					t.Fatalf("auto ignored failed cleanup: %v", err)
				}
			}
			if err == nil {
				if err := acc.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
