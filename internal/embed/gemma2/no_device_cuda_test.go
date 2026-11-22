//go:build cuda

package gemma2

import (
	"os"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
)

func TestCUDAFallbackWithoutDevice(t *testing.T) {
	if visible, set := os.LookupEnv("CUDA_VISIBLE_DEVICES"); !set || visible != "" {
		t.Skip("set CUDA_VISIBLE_DEVICES to an empty value before running the test")
	}
	info, err := chooseRuntime(t.Context(), "auto", 1, cudaCompiled, func() embed.Listing {
		return embed.Listing{Devices: cudaDevices()}
	}, func(embed.DeviceInfo) error {
		t.Fatal("attempted to attach CUDA with no visible devices")
		return nil
	})
	if want := (embed.RunInfo{Device: "cpu", Fallback: embed.FallbackNoDevice}); err != nil || info != want {
		t.Fatalf("runtime = (%+v, %v), want %+v", info, err, want)
	}
}
