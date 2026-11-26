package gemma2

import (
	"errors"
	"slices"
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
		got, err := attachedMetalProfile(&fakeMetal{device: row, profile: profile}, false)
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
		if _, err := attachedMetalProfile(acc, false); !errors.Is(err, embed.ErrAccelerator) {
			t.Fatalf("accepted incomplete Metal attachment: %v", err)
		}
	}

	batch := row
	batch.TextBatching = true
	combinedBatch := &fakeMetal{device: batch, profile: metalCombinedProfile}
	if got, err := attachedMetalProfile(combinedBatch, true); err != nil ||
		profileFingerprint(q8.Fingerprint, got, goinferRevision) != q8.MetalFingerprints[1] {
		t.Fatalf("attached Metal batch profile = %q, %v", got, err)
	}
	if _, err := attachedMetalProfile(combinedBatch, false); !errors.Is(err, embed.ErrAccelerator) {
		t.Fatalf("singleton accepted a batch attachment: %v", err)
	}
	if got := deviceListing([]embeddinggemma2.DeviceInfo{batch}, nil, true, true); len(got.Devices) != 0 {
		t.Fatalf("discovery listed a batch device: %+v", got)
	}
	batchWrong := batch
	batchWrong.ID = "metal:other"
	for _, acc := range []embeddinggemma2.Accelerator{
		&fakeSingleAccel{},
		&fakeMetal{device: batch},
		&fakeMetal{device: batch, profile: metalStockProfile},
		&fakeMetal{device: batchWrong, profile: metalCombinedProfile},
		&fakeMetal{device: row, profile: metalCombinedProfile},
	} {
		if _, err := attachedMetalProfile(acc, true); !errors.Is(err, embed.ErrAccelerator) {
			t.Fatalf("accepted incomplete Metal batch attachment: %v", err)
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

func TestMetalBatchDispatch(t *testing.T) {
	e, fake := retryEncoder(t, false)
	e.info.Device = metalDeviceID
	e.metalBatch = true
	if e.Window() != 256 {
		t.Fatalf("Metal batch window = %d", e.Window())
	}
	texts := []string{"one", "two", "three", "four", "five", "six"}
	var want [][]float32
	for _, text := range texts {
		v, err := e.Embed(t.Context(), embed.Document, []string{text})
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, v[0])
	}

	run := func() {
		t.Helper()
		got, err := e.Embed(t.Context(), embed.Document, texts)
		if err != nil || !slices.EqualFunc(got, want, sameVectorBits) || e.Info().Device != metalDeviceID {
			t.Fatalf("Metal dispatch lost order, vectors, or device: %d vectors, %+v, %v", len(got), e.Info(), err)
		}
	}
	fake.attempts = nil
	run()
	if len(fake.attempts) != 2 || len(fake.attempts[0]) != 4 || len(fake.attempts[1]) != 2 {
		t.Fatalf("Metal forward groups = %v, want groups of four and two", fake.attempts)
	}

	fake.attempts = nil
	fake.reject = func(group [][]int) error {
		if len(group) > 1 && len(fake.attempts) == 1 {
			return embeddinggemma2.ErrBatchOutOfMemory
		}
		return nil
	}
	run()
	if len(fake.attempts) < 3 || len(fake.attempts[0]) != 4 || len(fake.attempts[1]) >= 4 {
		t.Fatalf("Metal did not split the refused group: %v", fake.attempts)
	}

	e.retryLimit = nil
	fake.batchErr = embeddinggemma2.ErrUnsupportedBatchShape
	fake.reject = nil
	if got, err := e.Embed(t.Context(), embed.Document, texts); got != nil ||
		!errors.Is(err, embed.ErrAccelerator) || !errors.Is(err, fake.batchErr) {
		t.Fatalf("Metal accepted a refused forward: %v, %v", got, err)
	}
	fake.reject = func([][]int) error { return embeddinggemma2.ErrBatchOutOfMemory }
	if got, err := e.Embed(t.Context(), embed.Document, texts[:1]); got != nil ||
		!errors.Is(err, embeddinggemma2.ErrBatchOutOfMemory) || e.Info().Device != metalDeviceID {
		t.Fatalf("Metal singleton OOM did not fail on the named device: %v, %v, %+v", got, err, e.Info())
	}
}

func TestMetalBatchAttachError(t *testing.T) {
	if metalCompiled {
		t.Skip("requires a build without the real Metal backend")
	}
	_, model := tinyEncoderForTest(t)
	_, _, err := attachMetal(model, true, 0)
	if !errors.Is(err, embed.ErrAccelerator) || !strings.Contains(err.Error(), metalBatchSelector) {
		t.Fatalf("Metal batch attach without a backend = %v", err)
	}
}
