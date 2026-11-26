package semantic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/item"
)

type metalProfileFixture struct {
	writerFixture
	info   embed.RunInfo
	closed func()
}

func (e metalProfileFixture) Info() embed.RunInfo { return e.info }
func (e metalProfileFixture) Window() int         { return 1 }
func (e metalProfileFixture) Close() error {
	e.closed()
	return e.Embedder.Close()
}

func TestMetalIndexProfiles(t *testing.T) {
	e := newEnv(t)
	m := e.m()
	stock, combined := strings.Repeat("b", 64), strings.Repeat("c", 64)
	m.CUDAFP16Fingerprint = strings.Repeat("a", 64)
	m.MetalFingerprints = [2]string{stock, combined}
	fp := stock
	device := embed.DeviceInfo{ID: "metal:0", Backend: "metal", Name: "Apple M4"}
	probes, opens, closes := 0, 0, 0
	var openedSingleton bool
	attachFailure := fmt.Errorf("%w: Metal attach refused", embed.ErrAccelerator)
	var openErr error
	cpuOpen := func(ctx context.Context, a *embed.Artifact, o embed.OpenOptions) (embed.Embedder, error) {
		if o.Device != "" && o.Device != "cpu" {
			t.Fatalf("CPU reader received device %q", o.Device)
		}
		return e.kit.Registry.Open(ctx, a, embed.OpenOptions{})
	}
	e.reg = embed.NewRegistry(embed.Entry{
		Model: m, MetalWriter: true,
		Devices: func() embed.Listing {
			probes++
			return embed.Listing{Devices: []embed.DeviceInfo{device}}
		},
		Open: func(ctx context.Context, a *embed.Artifact, o embed.OpenOptions) (embed.Embedder, error) {
			if o.Device == "" || o.Device == "cpu" {
				return cpuOpen(ctx, a, o)
			}
			if !o.Writer.ResolveMetal || o.Writer.Inventory == nil || (o.Device != "metal:0" && o.Device != "auto") {
				t.Fatalf("Metal open without resolved inventory: %+v", o)
			}
			opens++
			openedSingleton = o.MetalSingleton
			if openErr != nil {
				return nil, openErr
			}
			info := embed.RunInfo{Device: device.ID, Name: device.Name}
			if fp == m.Fingerprint {
				info = embed.RunInfo{Device: "cpu", Fallback: embed.FallbackFailed}
			}
			base, err := cpuOpen(ctx, a, embed.OpenOptions{})
			if err != nil {
				return nil, err
			}
			return metalProfileFixture{writerFixture{base, fp}, info, func() { closes++ }}, nil
		},
	})
	opts := func(sel string) Options {
		o := e.opts()
		o.CUDAPrecision, o.Device = "fp16", sel
		o.ShareWriterResolution()
		return o
	}
	shared, personal := newSection(t, item.Shared), newSection(t, item.Personal)
	basic(shared)
	basic(personal)
	read := func(s *section) manifest {
		t.Helper()
		dir, err := indexDir(s.dir, false)
		if err != nil {
			t.Fatal(err)
		}
		defer dir.Close()
		got, state := readManifest(e.opts().Files, dir)
		if state != IndexOK {
			t.Fatalf("manifest state = %v", state)
		}
		return got
	}
	build := func(o Options, s *section, embedded, reused int, marker string) {
		t.Helper()
		res, err := e.buildWith(t.Context(), o, s)
		if err != nil || res.PassagesEmbedded != embedded || res.PassagesReused != reused {
			t.Fatalf("sync = %+v, %v; want %d embedded, %d reused", res, err, embedded, reused)
		}
		man := read(s)
		if man.Fingerprint != indexFingerprint(fp) || man.CUDAPrecisionRequest != marker {
			t.Fatalf("sync published wrong identity: %+v", man)
		}
	}
	first := opts("metal")
	build(first, shared, 3, 0, "")
	build(first, personal, 3, 0, "")
	if probes != 1 || opens != 2 || closes != 2 {
		t.Fatalf("initial sync probes, opens, closes = %d, %d, %d", probes, opens, closes)
	}
	prev := read(shared)
	build(opts("metal:0"), shared, 0, 0, "")
	if read(shared) != prev || opens != 3 || closes != 3 {
		t.Fatal("no-op did not attach exactly once and keep the existing generation")
	}
	fp = combined
	build(opts("auto"), shared, 3, 0, "")
	fp = stock
	build(opts("metal:0"), shared, 0, 3, "")
	fp = combined
	build(opts("metal"), shared, 0, 3, "")
	prev = read(shared)
	singleton := opts("metal")
	singleton.MetalSingleton = true
	build(singleton, shared, 0, 0, "")
	if !openedSingleton || read(shared) != prev {
		t.Fatal("forcing Metal singleton changed the combined generation or lost the opt-out")
	}
	build(opts("metal"), shared, 0, 0, "")
	if openedSingleton || read(shared) != prev {
		t.Fatal("restoring Metal batching changed the combined generation or retained the opt-out")
	}

	// With both Metal namespaces warm, auto falling back before reuse must
	// still start clean in the base namespace.
	fp = m.Fingerprint
	build(opts("auto"), shared, 3, 0, "fp16")
	fp = combined
	build(opts("auto"), shared, 0, 3, "")
	prev = read(shared)
	openErr = attachFailure
	if _, err := e.buildWith(t.Context(), opts("metal"), shared); !errors.Is(err, attachFailure) ||
		errCode(err) != "io" || openedSingleton || read(shared) != prev {
		t.Fatalf("explicit Metal failure changed the published index: %v", err)
	}
	openErr = nil
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	before := closes
	_, err := Build(ctx, opts("metal"), shared.sectionDir(), shared.scan, e.dl, nil, func(p Progress) {
		if p.Run != nil {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) || read(shared) != prev || closes != before+1 {
		t.Fatalf("canceled attachment changed the index or leaked the encoder: %v", err)
	}

	// A build without Metal opens both completed profiles for CPU search and
	// status. Adding new content does not make either eligible for catch-up.
	shared.put("notes/extra.md", "Extra", "extra content")
	personal.put("notes/extra.md", "Extra", "extra content")
	si, _ := shared.scan(t.Context())
	pi, _ := personal.scan(t.Context())
	reader := e.opts()
	reader.Registry = embed.NewRegistry(embed.Entry{Model: m, Open: cpuOpen})
	reader.CUDAPrecision = "fp32"
	oldPersonal := read(personal)
	src := Start(t.Context(), reader, "alpha", []SectionDir{shared.sectionDir(), personal.sectionDir()})
	src.Attach([]SectionInput{si, pi})
	if err := src.CatchUp(t.Context()); err != nil {
		t.Fatal(err)
	}
	scores, err := src.Score(t.Context(), "alpha", si.Items)
	src.Close()
	status, serr := Status(t.Context(), reader, []SectionInput{si, pi})
	if err != nil || scores.Reason != 0 || len(scores.Hits) == 0 ||
		serr != nil || status.Sections[0].Indexed != 3 || status.Sections[1].Indexed != 3 ||
		read(shared) != prev || read(personal) != oldPersonal {
		t.Fatalf("CPU-only reader changed or failed to open Metal profiles: %+v, %+v, %v, %v", scores, status, err, serr)
	}

	probesBefore, opensBefore := probes, opens
	for _, s := range []*section{shared, personal} {
		for _, note := range []string{"notes/a.md", "notes/b.md", "notes/c.md", "notes/extra.md"} {
			s.del(note)
		}
		build(opts("metal"), s, 0, 0, "")
		fp = stock
	}
	empty := newSection(t, item.Personal)
	if _, err := e.buildWith(t.Context(), opts("metal"), empty); err != nil {
		t.Fatal(err)
	}
	if probes != probesBefore || opens != opensBefore {
		t.Fatal("empty sync probed or opened Metal")
	}
}
