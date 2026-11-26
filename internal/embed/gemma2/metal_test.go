package gemma2

import (
	"errors"
	"strings"
	"testing"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

type fakeMetal struct {
	fakeSingleAccel
	device  embeddinggemma2.DeviceInfo
	profile string
}

func (a *fakeMetal) AttachedDevice() embeddinggemma2.DeviceInfo { return a.device }
func (a *fakeMetal) AttachedProfile() string                    { return a.profile }

func TestMetalListingAndProfile(t *testing.T) {
	row := embeddinggemma2.DeviceInfo{
		Backend: "metal", Name: "Apple M4", MemoryKind: embeddinggemma2.MemoryShared,
		Memory: embeddinggemma2.DeviceMemory{Basis: embeddinggemma2.MemoryEstimated, AvailableBytes: 17 * gib, TotalBytes: 24 * gib},
	}
	probeErr := errors.New("broken Metal probe")
	arms := []embeddinggemma2.ProbeFailure{{Backend: "metal", Err: probeErr}}
	listing := deviceListing([]embeddinggemma2.DeviceInfo{row}, arms, true, true)
	if len(listing.Devices) != 1 || !errors.Is(listing.Failed["metal"], probeErr) {
		t.Fatalf("Metal discovery = %+v", listing)
	}
	for _, name := range []string{"metal", "metal:0"} {
		d, ok := listing.Match(name)
		if !ok || d.ID != metalDeviceID || d.Persistent != "" || d.Name != row.Name ||
			d.MemoryKind != "shared" || d.Memory != "estimated" || d.FreeBytes != 17*gib || d.TotalBytes != 24*gib {
			t.Fatalf("Metal alias %q = %+v", name, d)
		}
	}
	if _, ok := listing.Match("metal:1"); ok {
		t.Fatal("Metal exposes a second ordinal")
	}
	wrong := row
	wrong.ID = "metal:other"
	for _, rows := range [][]embeddinggemma2.DeviceInfo{{row, row}, {wrong}} {
		if got := deviceListing(rows, nil, true, true); len(got.Devices) != 0 {
			t.Fatalf("Metal lists a non-system-default inventory: %+v", got)
		}
	}
	if got := deviceListing([]embeddinggemma2.DeviceInfo{row}, arms, true, false); len(got.Devices) != 0 || len(got.Failed) != 0 {
		t.Fatalf("Metal-ineligible row exposes Metal inventory: %+v", got)
	}

	q8, q6 := GGUFEntries()[0].Model, GGUFEntries()[1].Model
	prefixes := []string{"c9ee9a472dff", "d5b3af579f21"}
	for i, profile := range []string{metalStockProfile, metalCombinedProfile} {
		got, err := attachedMetalProfile(&fakeMetal{device: row, profile: profile})
		fp := profileFingerprint(q8.Fingerprint, got, goinferRevision)
		if err != nil || got != profile || !strings.HasPrefix(fp, prefixes[i]) ||
			q8.MetalFingerprints[i] != fp || q6.IsMetalFingerprint(fp) || Entry().Model.IsMetalFingerprint(fp) {
			t.Fatalf("attached Metal profile for %q = %q, %q, %v", profile, got, fp, err)
		}
	}
	for _, acc := range []embeddinggemma2.Accelerator{
		&fakeSingleAccel{}, &fakeMetal{device: row}, &fakeMetal{device: wrong, profile: metalStockProfile},
		&fakeMetal{device: row, profile: "future-profile"},
	} {
		if _, err := attachedMetalProfile(acc); !errors.Is(err, embed.ErrAccelerator) {
			t.Fatalf("accepted incomplete Metal attachment: %v", err)
		}
	}
}

func TestMetalSingletonDispatch(t *testing.T) {
	e, m := tinyEncoderForTest(t)
	acc := &fakeSingleAccel{name: "test-metal-singleton", dim: m.Model().Config().EmbeddingDim}
	embeddinggemma2.RegisterAccelerator(acc.Name(), func(*embeddinggemma2.Model) (embeddinggemma2.Accelerator, error) {
		return acc, nil
	})
	if _, err := m.UseAccelerator(acc.Name()); err != nil {
		t.Fatal(err)
	}
	e.info.Device = metalDeviceID
	if e.Window() != 1 {
		t.Fatalf("Metal window = %d", e.Window())
	}
	if v, err := e.Embed(t.Context(), embed.Document, []string{"one", "two"}); err != nil || len(v) != 2 || acc.calls != 2 {
		t.Fatalf("Metal forwards = %d, error = %v", acc.calls, err)
	}
	acc.err = embeddinggemma2.ErrBatchOutOfMemory
	if _, err := e.Embed(t.Context(), embed.Document, []string{"one", "two"}); !errors.Is(err, acc.err) ||
		acc.calls != 3 || e.Info().Device != metalDeviceID {
		t.Fatalf("Metal forward after OOM = %d, %v, device %q", acc.calls, err, e.Info().Device)
	}
	if err := e.Close(); err != nil || acc.closed != 1 {
		t.Fatalf("Metal close = %v, calls = %d", err, acc.closed)
	}
}
