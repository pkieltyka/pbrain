package embed

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/pkieltyka/pbrain/internal/errs"
)

// DeviceInfo describes a device the registry can use. A live free figure may be zero.
type DeviceInfo struct {
	ID, Backend, Name     string
	Persistent            string // full selector, such as "cuda:GPU-..."
	TotalBytes, FreeBytes int64
	Memory                string // live, estimated or unknown
	MemoryKind            string // dedicated, shared or unknown
}

// Selector returns the persistent selector when the runtime exposes one.
func (d DeviceInfo) Selector() string {
	if d.Persistent != "" {
		return d.Persistent
	}
	return d.ID
}

// Listing keeps an empty inventory distinct from a backend that could not be asked.
type Listing struct {
	Devices []DeviceInfo
	Failed  map[string]error
}

func (l Listing) Sorted() []DeviceInfo {
	devices := slices.Clone(l.Devices)
	slices.SortFunc(devices, compareDevices)
	return devices
}

func (r *registry) Devices() []DeviceInfo {
	var devices []DeviceInfo
	seen := map[string]bool{}
	for _, name := range r.names {
		probe := r.entries[name].Devices
		if probe == nil {
			continue
		}
		for _, d := range probe().Devices {
			if !seen[d.ID] {
				seen[d.ID] = true
				devices = append(devices, d)
			}
		}
	}
	slices.SortFunc(devices, compareDevices)
	return append(devices, DeviceInfo{ID: "cpu", Backend: "cpu"})
}

// namedDevice resolves a device selector only among devices that can run this model.
func namedDevice(e Entry, selector string) (string, error) {
	listing := e.Devices()
	devices := listing.Sorted()
	ids := make([]string, 0, len(devices)+1)
	for _, d := range devices {
		if selector == d.Backend || selector == d.ID || selector == d.Persistent {
			return d.Selector(), nil
		}
		ids = append(ids, d.ID)
	}
	backend, _, _ := strings.Cut(selector, ":")
	if err := listing.Failed[backend]; err != nil {
		return "", fmt.Errorf("%w: %w", ErrAccelerator, err)
	}
	ids = append(ids, "cpu")
	message := fmt.Sprintf("device %q is unavailable; available devices: %s; see pbrain devices",
		selector, strings.Join(ids, ", "))
	return "", &errs.Error{Code: "invalid_config", Message: message, Exit: 1}
}

func compareDevices(a, b DeviceInfo) int {
	if order := cmp.Compare(knownFreeRank(a.Memory), knownFreeRank(b.Memory)); order != 0 {
		return order
	}
	if order := cmp.Compare(memoryKindRank(a.MemoryKind), memoryKindRank(b.MemoryKind)); order != 0 {
		return order
	}
	if order := cmp.Compare(rank(a.Memory, "live", "estimated"), rank(b.Memory, "live", "estimated")); order != 0 {
		return order
	}
	if a.Memory == "live" || a.Memory == "estimated" {
		if order := cmp.Compare(b.FreeBytes, a.FreeBytes); order != 0 {
			return order
		}
	}
	if order := cmp.Compare(rank(a.Backend, "cuda", "vulkan", "metal"), rank(b.Backend, "cuda", "vulkan", "metal")); order != 0 {
		return order
	}
	if order := cmp.Compare(deviceIndex(a.ID), deviceIndex(b.ID)); order != 0 {
		return order
	}
	return cmp.Compare(a.ID, b.ID)
}

func knownFreeRank(memory string) int {
	if memory == "live" || memory == "estimated" {
		return 0
	}
	return 1
}

func memoryKindRank(kind string) int {
	switch kind {
	case "dedicated":
		return 0
	case "shared":
		return 2
	default:
		return 1
	}
}

func rank(value string, values ...string) int {
	if n := slices.Index(values, value); n >= 0 {
		return n
	}
	return len(values)
}

func deviceIndex(id string) int {
	_, suffix, _ := strings.Cut(id, ":")
	index, _ := strconv.Atoi(suffix)
	return index
}
