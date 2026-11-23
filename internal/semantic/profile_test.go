package semantic

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type writerFixture struct {
	embed.Embedder
	fingerprint string
}

func (e writerFixture) Fingerprint() string { return e.fingerprint }

func TestIndexProfilesAcrossSyncs(t *testing.T) {
	e := newEnv(t)
	m := e.m()
	m.CUDAFP16Fingerprint = strings.Repeat("a", 64)
	card := embed.DeviceInfo{ID: "cuda:0", Persistent: "cuda:GPU-A", Backend: "cuda"}
	probes := 0
	var opens []embed.OpenOptions
	fast, wrong := true, false
	e.reg = embed.NewRegistry(embed.Entry{
		Model: m,
		Devices: func() embed.Listing {
			probes++
			return embed.Listing{Devices: []embed.DeviceInfo{card}}
		},
		ResolveFP16: func(snapshot embed.Listing, selector string) (embed.DeviceInfo, string) {
			if selector != "auto" && selector != card.ID {
				t.Fatalf("unexpected selector %q", selector)
			}
			if len(snapshot.Devices) != 1 || snapshot.Devices[0] != card {
				t.Fatal("resolver did not get the current inventory")
			}
			if !fast {
				return embed.DeviceInfo{}, "cuda:0 supports compute capability 7.5; using fp32 CUDA"
			}
			return card, ""
		},
		Open: func(ctx context.Context, a *embed.Artifact, opt embed.OpenOptions) (embed.Embedder, error) {
			opens = append(opens, opt)
			if a.Model.Fingerprint != m.Fingerprint {
				t.Fatal("changed the verified artifact model")
			}
			emb, err := e.kit.Registry.Open(ctx, a, embed.OpenOptions{})
			if err != nil {
				return nil, err
			}
			if opt.Writer.Selector != "" && !wrong {
				if opt.Writer.Selector != embed.CUDAFP16Selector || opt.Writer.DeviceID != card.Persistent {
					t.Fatal("runtime did not receive the reserved selector and UUID")
				}
				return writerFixture{emb, m.CUDAFP16Fingerprint}, nil
			}
			return emb, nil
		},
	})
	shared := newSection(t, item.Shared)
	personal := newSection(t, item.Personal)
	basic(shared)
	basic(personal)
	opts := func(request string) Options {
		o := e.opts()
		o.CUDAPrecision, o.Device = request, "auto"
		o.ShareWriterResolution()
		return o
	}
	read := func(s *section) manifest {
		t.Helper()
		dir, err := indexDir(s.dir, false)
		if err != nil {
			t.Fatal(err)
		}
		defer dir.Close()
		man, state := readManifest(e.opts().Files, dir)
		if state != IndexOK {
			t.Fatalf("missing manifest: %v", state)
		}
		return man
	}
	build := func(o Options, s *section, fp, marker string, embedded, reused int) {
		t.Helper()
		r, err := e.buildWith(t.Context(), o, s)
		if err != nil || r.PassagesEmbedded != embedded || r.PassagesReused != reused {
			t.Fatalf("sync %s: %+v, %v, want %d embedded and %d reused", marker, r, err, embedded, reused)
		}
		man := read(s)
		if man.Fingerprint != indexFingerprint(fp) || man.CUDAPrecisionRequest != marker {
			t.Fatalf("published wrong profile: %+v", man)
		}
	}
	base := opts("fp32")
	build(base, shared, m.Fingerprint, "", 3, 0)
	build(base, personal, m.Fingerprint, "", 3, 0)
	if probes != 0 {
		t.Fatalf("fp32 performed %d early probes", probes)
	}

	upgraded := opts("fp16")
	build(upgraded, shared, m.CUDAFP16Fingerprint, "fp16", 3, 0)
	build(upgraded, personal, m.CUDAFP16Fingerprint, "fp16", 3, 0)
	if x, state := shared.open(m); state != IndexStale {
		if x != nil {
			x.Close()
		}
		t.Fatalf("legacy reader accepted the fp16 fingerprint: %v", state)
	}
	if probes != 1 {
		t.Fatalf("one invocation probed %d times", probes)
	}
	for _, o := range opens[:2] {
		if o.Writer.Selector != "" {
			t.Fatal("base sync received a fast selector")
		}
	}
	// Hardware can change even with no source changes. Shared may reuse
	// its old fp32 cache. Personal must compute its own vectors again.
	fast = false
	fallback := opts("fp16")
	build(fallback, personal, m.Fingerprint, "fp16", 3, 0)
	personal.put("notes/new.md", "Delta", "delta text")
	shared.put("notes/new.md", "Delta", "delta text")
	si, _ := shared.scan(t.Context())
	pi, _ := personal.scan(t.Context())
	beforeShared, beforePersonal := read(shared), read(personal)
	savedProbes := probes
	src := Start(t.Context(), fallback, "alpha", []SectionDir{shared.sectionDir(), personal.sectionDir()})
	src.Attach([]SectionInput{si, pi})
	if err := src.CatchUp(t.Context()); err != nil {
		t.Fatal(err)
	}
	scores, err := src.Score(t.Context(), "alpha", si.Items)
	src.Close()
	if err != nil || scores.Reason != 0 || len(scores.Hits) == 0 {
		t.Fatalf("CPU query did not read mixed profiles: %+v, %v", scores, err)
	}
	if read(shared) != beforeShared || read(personal) == beforePersonal || probes != savedProbes {
		t.Fatal("catch-up wrote fp16, ignored its fp32 section or probed hardware")
	}
	st, err := Status(t.Context(), fallback, []SectionInput{si, pi})
	if err != nil || st.Sections[0].Indexed != 3 || st.Sections[1].Indexed != 4 || probes != savedProbes {
		t.Fatalf("mixed status: %+v, %v, %d probes", st, err, probes)
	}
	build(fallback, shared, m.Fingerprint, "fp16", 1, 3)
	build(opts("fp32"), shared, m.Fingerprint, "", 0, 4)
	previous := read(shared)
	fast, wrong = true, true
	// Existing fp16 vectors can cover the original notes, but the new one
	// cannot be collected from an adapter that opens with the base identity.
	if _, err := e.buildWith(t.Context(), opts("fp16"), shared); errCode(err) != "io" || read(shared) != previous {
		t.Fatalf("published a different writer after open: %v", err)
	}
	wrong = false
	build(opts("fp16"), shared, m.CUDAFP16Fingerprint, "fp16", 1, 3)

	// A search opened the fallback index before sync published fp16.
	// It cannot overwrite the new generation with its CPU catch-up.
	fast = false
	fallback = opts("fp16")
	build(fallback, shared, m.Fingerprint, "fp16", 0, 4)
	shared.put("notes/other.md", "Epsilon", "epsilon text")
	si, _ = shared.scan(t.Context())
	src = Start(t.Context(), fallback, "alpha", []SectionDir{shared.sectionDir()})
	src.Attach([]SectionInput{si})
	<-src.indexes
	<-src.model
	fast = true
	build(opts("fp16"), shared, m.CUDAFP16Fingerprint, "fp16", 1, 4)
	previous = read(shared)
	calls := e.kit.Calls.Load()
	if err := src.CatchUp(t.Context()); err != nil {
		t.Fatal(err)
	}
	src.Close()
	if read(shared) != previous || e.kit.Calls.Load() != calls {
		t.Fatal("CPU catch-up overwrote a concurrently published fp16 index")
	}

	// An emptied section retains its valid effective profile, including its
	// requested policy, without needing the card or trying an old selector.
	for _, k := range []string{"notes/a.md", "notes/b.md", "notes/c.md", "notes/new.md", "notes/other.md"} {
		shared.del(k)
	}
	savedProbes = probes
	build(opts("fp16"), shared, m.CUDAFP16Fingerprint, "fp16", 0, 0)
	if probes != savedProbes {
		t.Fatal("empty sync probed the GPU")
	}

	// Interrupt a switch after its first publication. Resume using the
	// checkpoint's fp16 rows to produce the same rows as a fresh switch.
	resumed, whole := newSection(t, item.Personal), newSection(t, item.Personal)
	for _, s := range []*section{resumed, whole} {
		basic(s)
		build(opts("fp32"), s, m.Fingerprint, "", 3, 0)
	}
	e.clock.step = 30 * time.Second
	e.reg = &windowTestRegistry{Registry: e.reg, width: 1}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, err = Build(ctx, opts("fp16"), resumed.sectionDir(), resumed.scan, e.dl, nil, func(p Progress) {
		if p.Run == nil && p.Done == 1 {
			cancel()
		}
	})
	checkpoint := read(resumed)
	if !errors.Is(err, context.Canceled) || checkpoint.Fingerprint != indexFingerprint(m.CUDAFP16Fingerprint) ||
		checkpoint.CUDAPrecisionRequest != "fp16" {
		t.Fatalf("interrupted switch = %v, checkpoint = %+v", err, checkpoint)
	}
	if r, err := e.buildWith(t.Context(), opts("fp16"), resumed); err != nil || r.PassagesEmbedded != 2 {
		t.Fatalf("resume from fp16 checkpoint = %+v, %v", r, err)
	}
	build(opts("fp16"), whole, m.CUDAFP16Fingerprint, "fp16", 3, 0)
	profileModel := m
	profileModel.Fingerprint = m.CUDAFP16Fingerprint
	left, a := resumed.open(profileModel)
	right, b := whole.open(profileModel)
	if a != IndexOK || b != IndexOK {
		t.Fatalf("index states after precision switch = %v, %v", a, b)
	}
	defer left.Close()
	defer right.Close()
	if !reflect.DeepEqual(left.items, right.items) {
		t.Fatal("resumed precision switch changed the item rows")
	}
	for path, row := range left.items {
		got, err := left.passagesOf(t.Context(), row.id)
		want, otherErr := right.passagesOf(t.Context(), right.items[path].id)
		if err != nil || otherErr != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("resumed precision switch changed the passages at %s: %v, %v", path, err, otherErr)
		}
	}
}

func TestManifestPrecisionMarker(t *testing.T) {
	path := t.TempDir()
	dir, err := sys.OpenDir(path)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	for _, v := range []string{`"fp16"`, `"fp32"`, `""`, `"future"`, "16", "null", "true"} {
		content := fmt.Sprintf(`{"format":3,"fingerprint":"f","model":"m","dim":2,"db":"0123456789abcdef.db","size":1,"crc32c":"abcdabcd","cuda_precision_request":%s}`, v)
		if err := os.WriteFile(filepath.Join(path, manifestName), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		_, state := readManifest(sys.Native(), dir)
		if state == IndexOK && v != `"fp16"` || state != IndexOK && v == `"fp16"` {
			t.Fatalf("manifest marker %s state = %v", v, state)
		}
	}
}

func TestManifestProfileMismatch(t *testing.T) {
	m := embed.Model{Name: "gemma", Dim: 4, Fingerprint: "base", CUDAFP16Fingerprint: "fast"}
	base := manifest{Model: m.Name, Dim: m.Dim, Fingerprint: IndexFingerprint(m)}
	fallback := base
	fallback.CUDAPrecisionRequest = "fp16"
	fast := fallback
	fast.Fingerprint = indexFingerprint(m.CUDAFP16Fingerprint)
	unknown := fast
	unknown.Fingerprint = indexFingerprint("old-fast")
	for _, c := range []struct {
		manifest      manifest
		request, want string
	}{
		{base, "fp32", "base"}, {base, "fp16", ""},
		{fallback, "fp16", "base"}, {fallback, "fp32", ""},
		{fast, "fp16", "fast"}, {fast, "fp32", ""},
		{unknown, "fp16", ""},
	} {
		if got := manifestProfile(c.manifest, m, c.request); got != c.want {
			t.Errorf("%+v (%s) resolved %q, want %q", c.manifest, c.request, got, c.want)
		}
	}
}
