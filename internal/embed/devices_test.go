package embed_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/errs"
)

func TestRegistryDevices(t *testing.T) {
	cuda := func(id string, free int64) embed.DeviceInfo {
		return embed.DeviceInfo{ID: id, Backend: "cuda", MemoryKind: "dedicated", Memory: "live", FreeBytes: free}
	}
	list := []embed.DeviceInfo{
		{ID: "metal:0", Backend: "metal", MemoryKind: "shared", Memory: "live", FreeBytes: 500},
		{ID: "cuda:1", Backend: "cuda", MemoryKind: "dedicated", Memory: "estimated", FreeBytes: 50},
		{ID: "cuda:3", Backend: "cuda", MemoryKind: "dedicated", Memory: "unknown", FreeBytes: 9999},
		{ID: "vulkan:0", Backend: "vulkan", MemoryKind: "dedicated", Memory: "live", FreeBytes: 60},
		{ID: "vulkan:9", Backend: "vulkan", Memory: "live", FreeBytes: 400},
		cuda("cuda:10", 60),
		cuda("cuda:4", 30),
		cuda("cuda:2", 60),
	}
	list[7].Persistent = "cuda:GPU-test"
	before := slices.Clone(list)
	probes := 0
	var failed map[string]error
	probe := func() embed.Listing {
		probes++
		return embed.Listing{Devices: list, Failed: failed}
	}
	e := embed.Entry{Model: embed.Model{Name: "accelerated"}, Devices: probe}
	var opts []embed.OpenOptions
	e.Open = func(_ context.Context, _ *embed.Artifact, o embed.OpenOptions) (embed.Embedder, error) {
		opts = append(opts, o)
		return nil, nil
	}
	other := embed.Entry{Model: embed.Model{Name: "other"}, Devices: probe}
	cpu := embed.Entry{Model: embed.Model{Name: "cpu-only"}, Open: e.Open}
	reg := embed.NewRegistry(e, other, cpu)
	open := func(m embed.Model, device string) error {
		_, err := reg.Open(t.Context(), &embed.Artifact{Model: m}, embed.OpenOptions{Device: device, MemoryBudget: 42})
		return err
	}

	for _, selector := range []string{"", "cpu", "auto"} {
		if err := open(e.Model, selector); err != nil {
			t.Fatal(err)
		}
		if got := opts[len(opts)-1]; got != (embed.OpenOptions{Device: selector, MemoryBudget: 42}) {
			t.Fatalf("opener options = %+v, selector %q and budget 42 expected", got, selector)
		}
	}
	if probes != 0 {
		t.Fatalf("registry probed %d times on a CPU or adapter-managed open", probes)
	}
	devices := reg.Devices()
	var ids []string
	for _, d := range devices {
		ids = append(ids, d.ID)
	}
	wantIDs := []string{"cuda:2", "cuda:10", "vulkan:0", "cuda:4", "cuda:1", "vulkan:9", "metal:0", "cuda:3", "cpu"}
	if !slices.Equal(ids, wantIDs) {
		t.Fatalf("devices = %v, want %v", ids, wantIDs)
	}

	for _, selection := range [][2]string{{"cuda", "cuda:GPU-test"}, {"cuda:2", "cuda:GPU-test"},
		{"cuda:1", "cuda:1"}, {"vulkan", "vulkan:0"}, {"cuda:GPU-test", "cuda:GPU-test"}} {
		if err := open(e.Model, selection[0]); err != nil {
			t.Fatal(err)
		}
		if got := opts[len(opts)-1]; got.Device != selection[1] {
			t.Fatalf("%q resolved to %q, want %q", selection[0], got.Device, selection[1])
		}
	}
	cause := errors.New("vulkan instance failed")
	failed = map[string]error{"vulkan": cause}
	if err := open(e.Model, "vulkan:99"); !errors.Is(err, embed.ErrAccelerator) || !errors.Is(err, cause) {
		t.Fatalf("failed listing selection = %v, want accelerator failure and listing cause", err)
	}
	if err := open(e.Model, "vulkan:0"); err != nil {
		t.Fatalf("a listed matching device was rejected due to a probe failure: %v", err)
	}
	if !slices.Equal(reg.Devices(), devices) {
		t.Fatal("public inventory changed when a backend's probe failed")
	}
	if !slices.Equal(list, before) {
		t.Fatal("registry reordered the adapter's device list")
	}
	calls := len(opts)
	for _, selector := range []string{"cuda:99", "GPU-test"} {
		err := open(e.Model, selector)
		var config *errs.Error
		if !errors.As(err, &config) || config.Code != "invalid_config" || len(opts) != calls {
			t.Fatalf("missing selection opens = %v, error = %v", opts, err)
		}
	}
	priorProbes := probes
	if err := open(cpu.Model, "cuda:99"); err != nil || probes != priorProbes {
		t.Fatalf("CPU-only model consulted the device selector: %v", err)
	}
}
