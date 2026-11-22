package gemma2

import (
	"errors"
	"fmt"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
)

func TestDeviceSelection(t *testing.T) {
	attachErr := fmt.Errorf("%w: test attach failed", embed.ErrAccelerator)
	device := []embed.DeviceInfo{{ID: "cuda:0", Backend: "cuda", Memory: "live", FreeBytes: 100}}
	for _, test := range []struct {
		name, selector string
		compiled       bool
		devices        []embed.DeviceInfo
		attachErr      error
		probe, attach  bool
		want           embed.RunInfo
		wantErr        error
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
		{name: "named success", selector: "cuda:0", compiled: true, attach: true, want: embed.RunInfo{Device: "cuda:0"}},
		{name: "named attach fails", selector: "cuda:0", compiled: true, attachErr: attachErr, attach: true, wantErr: attachErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			probed, attached := false, false
			info, err := chooseRuntime(t.Context(), test.selector, 50, test.compiled, func() []embed.DeviceInfo {
				probed = true
				return test.devices
			}, func() error {
				attached = true
				return test.attachErr
			})
			if info != test.want || !errors.Is(err, test.wantErr) || probed != test.probe || attached != test.attach {
				t.Fatalf("runtime = (%+v, %v), probe=%t attach=%t", info, err, probed, attached)
			}
		})
	}
}
