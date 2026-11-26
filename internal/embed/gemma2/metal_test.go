package gemma2

import (
	"errors"
	"fmt"
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
	for _, batch := range []bool{true, false} {
		_, _, _, err := attachMetalRoute(model.UseAccelerator, batch, 0)
		if !errors.Is(err, embed.ErrAccelerator) || !errors.Is(err, embeddinggemma2.ErrBackendUnavailable) ||
			strings.Contains(err.Error(), metalBatchSelector) != batch {
			t.Fatalf("Metal batch=%v attach without a backend: %v", batch, err)
		}
	}
}

type fakeMetalMemory struct {
	*fakeMetal
	err error
}

func (a *fakeMetalMemory) DeviceMemory() (embeddinggemma2.DeviceMemory, error) {
	return embeddinggemma2.DeviceMemory{}, a.err
}

func TestMetalAttachRoutes(t *testing.T) {
	refused := fmt.Errorf("architecture refused: %w", embeddinggemma2.ErrUnsupportedSelector)
	oom := embeddinggemma2.ErrDeviceOutOfMemory
	closeErr := errors.New("Metal drain failed")
	memoryErr := errors.New("Metal memory query failed")
	singletonErr := errors.New("singleton attach failed")
	for _, tc := range []struct {
		name                string
		singleton           bool
		batchErr, plainErr  error
		closeErr, memoryErr error
		batchProfile        string
		batchReports        bool
		plainProfile        string
		selectors           []string
		profile, errorText  string
		cause               error
		budget              int64
		closes              int
		batched             bool
	}{
		{name: "accepted", batchProfile: metalCombinedProfile, batchReports: true,
			selectors: []string{metalBatchSelector}, profile: metalCombinedProfile, budget: gib, batched: true},
		{name: "refusal to stock", batchErr: refused, plainProfile: metalStockProfile,
			selectors: []string{metalBatchSelector, "metal"}, profile: metalStockProfile},
		{name: "refusal to combined", batchErr: refused, plainProfile: metalCombinedProfile,
			selectors: []string{metalBatchSelector, "metal"}, profile: metalCombinedProfile},
		{name: "missing batching", batchProfile: metalCombinedProfile, plainProfile: metalCombinedProfile,
			selectors: []string{metalBatchSelector, "metal"}, profile: metalCombinedProfile, closes: 1},
		{name: "wrong profile", batchProfile: metalStockProfile, batchReports: true, plainProfile: metalStockProfile,
			selectors: []string{metalBatchSelector, "metal"}, profile: metalStockProfile, closes: 1},
		{name: "rejected attachment cannot close", batchProfile: metalCombinedProfile, closeErr: closeErr,
			selectors: []string{metalBatchSelector}, cause: closeErr, errorText: "close metal:0", closes: 1},
		{name: "attachment OOM", batchErr: oom, selectors: []string{metalBatchSelector},
			cause: oom, errorText: "attach metal:0"},
		{name: "memory read failure", batchProfile: metalCombinedProfile, batchReports: true, memoryErr: memoryErr,
			selectors: []string{metalBatchSelector}, cause: memoryErr, errorText: "read memory of metal:0", closes: 1},
		{name: "singleton attachment fails", batchErr: refused, plainErr: singletonErr,
			selectors: []string{metalBatchSelector, "metal"}, cause: singletonErr, errorText: "attach metal:0"},
		{name: "opt out", singleton: true, plainProfile: metalCombinedProfile,
			selectors: []string{"metal"}, profile: metalCombinedProfile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			batch := &fakeMetal{
				device:  embeddinggemma2.DeviceInfo{Backend: "metal", TextBatching: tc.batchReports},
				profile: tc.batchProfile,
			}
			if tc.closeErr != nil {
				batch.closeErrs = []error{tc.closeErr}
			}
			var batchAcc embeddinggemma2.Accelerator = batch
			if tc.memoryErr != nil {
				batchAcc = &fakeMetalMemory{fakeMetal: batch, err: tc.memoryErr}
			}
			plain := &fakeMetal{device: embeddinggemma2.DeviceInfo{Backend: "metal"}, profile: tc.plainProfile}
			var selectors []string
			use := func(selector string) (embeddinggemma2.Accelerator, error) {
				selectors = append(selectors, selector)
				if selector == metalBatchSelector {
					if tc.batchErr != nil {
						return nil, tc.batchErr
					}
					return batchAcc, nil
				}
				if batch.closed != tc.closes {
					t.Fatalf("batch attachment was closed %d times before singleton attach, want %d", batch.closed, tc.closes)
				}
				if tc.plainErr != nil {
					return nil, tc.plainErr
				}
				return plain, nil
			}
			profile, budget, batched, err := attachMetalRoute(use, !tc.singleton, 0)
			if !slices.Equal(selectors, tc.selectors) || profile != tc.profile || budget != tc.budget ||
				batched != tc.batched || batch.closed != tc.closes || plain.closed != 0 {
				t.Fatalf("attach = %q, %d, %v, %v; selectors %v, closes %d/%d",
					profile, budget, batched, err, selectors, batch.closed, plain.closed)
			}
			if tc.cause == nil && err != nil || tc.cause != nil &&
				(!errors.Is(err, embed.ErrAccelerator) || !errors.Is(err, tc.cause) ||
					!strings.Contains(err.Error(), tc.errorText) || strings.Contains(err.Error(), metalBatchSelector)) {
				t.Fatalf("attach error = %v, want %v", err, tc.cause)
			}
			if tc.closeErr != nil {
				var cleanup *attachCleanupError
				if !errors.As(err, &cleanup) || cleanup.acc != batchAcc {
					t.Fatalf("failed close did not retain the rejected handle: %v", err)
				}
			}
		})
	}
}

func TestMetalRuntimeRoute(t *testing.T) {
	for _, tc := range []struct {
		name, device string
		singleton    bool
		refusesBatch bool
		selectors    []string
		window       int
		budget       int64
	}{
		{name: "named default", device: metalDeviceID, selectors: []string{metalBatchSelector}, window: batchInputs, budget: gib},
		{name: "named opt-out", device: metalDeviceID, singleton: true, selectors: []string{"metal"}, window: 1},
		{name: "auto default", device: "auto", selectors: []string{metalBatchSelector}, window: batchInputs, budget: gib},
		{name: "auto opt-out", device: "auto", singleton: true, selectors: []string{"metal"}, window: 1},
		{name: "auto refusal", device: "auto", refusesBatch: true, selectors: []string{metalBatchSelector, "metal"}, window: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, m := tinyEncoderForTest(t)
			opt := embed.OpenOptions{Device: tc.device, MetalSingleton: tc.singleton,
				Writer: embed.WriteProfile{Inventory: &embed.Listing{Devices: []embed.DeviceInfo{
					{ID: metalDeviceID, Backend: "metal"},
				}}}}
			prompts := map[string]string{"query": queryPrompt, "document": docPrompt}
			var selectors []string
			fake := &fakeMetal{profile: metalCombinedProfile}
			fake.device.Backend = "metal"
			embeddinggemma2.RegisterAccelerator(t.Name(), func(*embeddinggemma2.Model) (embeddinggemma2.Accelerator, error) {
				return fake, nil
			})
			use := func(candidate *embeddinggemma2.Encoder, selector string) (embeddinggemma2.Accelerator, error) {
				selectors = append(selectors, selector)
				if tc.refusesBatch && selector == metalBatchSelector {
					return nil, embeddinggemma2.ErrUnsupportedSelector
				}
				fake.device.TextBatching = selector == metalBatchSelector
				return candidate.UseAccelerator(t.Name())
			}
			e, err := newRuntimeWithMetalAttach(t.Context(), m, prompts, opt, q8Fingerprint, true, use)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := e.Close(); err != nil || fake.closed != 1 {
					t.Errorf("Metal close = %v, calls = %d", err, fake.closed)
				}
			})
			if info := e.Info(); !slices.Equal(selectors, tc.selectors) || info.Device != metalDeviceID ||
				info.Fallback != "" || info.Budget != tc.budget || e.(embed.Windowed).Window() != tc.window ||
				e.Fingerprint() != profileFingerprint(q8Fingerprint, metalCombinedProfile, goinferRevision) {
				t.Fatalf("Metal selectors = %v, run = %+v, window = %d, fingerprint = %s",
					selectors, info, e.(embed.Windowed).Window(), e.Fingerprint())
			}
		})
	}
}
