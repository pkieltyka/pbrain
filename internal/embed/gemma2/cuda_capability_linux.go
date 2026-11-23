//go:build cuda

package gemma2

import (
	"strconv"
	"strings"

	"github.com/eitamring/gocudrv/cuda"
	"github.com/pkieltyka/pbrain/internal/embed"
)

func cudaCapability(device embed.DeviceInfo) (int, int, bool) {
	index, ok := strings.CutPrefix(device.ID, "cuda:")
	ordinal, err := strconv.Atoi(index)
	if !ok || err != nil || ordinal < 0 || cuda.Init() != nil {
		return 0, 0, false
	}
	card, err := cuda.GetDevice(ordinal)
	if err != nil {
		return 0, 0, false
	}
	uuid, err := card.UUID()
	if err != nil || !sameCUDAUUID(device.Persistent, uuid) {
		return 0, 0, false
	}
	major, minor, err := card.ComputeCapability()
	return major, minor, err == nil
}
