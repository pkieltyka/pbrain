package semantic

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

func pinnedTestModel(e *env) embed.Model {
	m := e.m()
	m.Files = append([]embed.File(nil), m.Files...)
	for i, f := range m.Files {
		m.Files[i].VeritySHA256 = embed.VerityDigest(e.kit.Bytes[f.URL])
	}
	e.reg = embed.NewRegistry(embed.Entry{Model: m, Open: e.kit.Registry.Open})
	return m
}

type testVerityFS struct {
	sys.Files
	digest     [32]byte
	measureErr error
	probes     int
}

func (f *testVerityFS) MeasureVerity(sys.RegularFile) ([32]byte, error) {
	return f.digest, f.measureErr
}

func (f *testVerityFS) SealTempVerity(_ sys.Dir, _ string, w io.WriteCloser) ([32]byte, error) {
	f.probes++
	_ = w.Close()
	return [32]byte{}, sys.ErrVerityUnavailable
}

func TestManagedStatusAndBuildStayStrict(t *testing.T) {
	e := newEnv(t)
	m := pinnedTestModel(e)
	f := m.Files[0]
	files := &testVerityFS{Files: sys.Native(), digest: f.VeritySHA256}
	e.cache.files = files
	s := newSection(t, item.Shared)
	basic(s)
	opt := e.opts()
	opt.Files = files
	if _, err := e.buildWith(t.Context(), opt, s); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(e.path, "models", embed.FolderName(m), f.Name)
	if err := os.WriteFile(p, bytes.Repeat([]byte("x"), int(f.Size)), 0600); err != nil {
		t.Fatal(err)
	}
	in, _ := s.scan(t.Context())
	st, err := Status(t.Context(), opt, []SectionInput{in})
	if err != nil || st.ModelPresent || files.probes != 0 {
		t.Fatalf("status accepted a raw SHA mismatch or wrote to the model cache: %+v %v", st, err)
	}
	if _, err := e.buildWith(t.Context(), opt, s); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil || !bytes.Equal(got, e.kit.Bytes[f.URL]) || e.dl.Requests() != 2 {
		t.Fatalf("sync did not repair the raw SHA mismatch: %v, %d requests", err, e.dl.Requests())
	}
}

func TestManagedVerityProbesUnsupportedOnce(t *testing.T) {
	e := newEnv(t)
	m := pinnedTestModel(e)
	files := &testVerityFS{Files: sys.Native(), measureErr: sys.ErrVerityUnsealed}
	e.cache.files = files
	opt := e.opts()
	opt.Files = files
	shared := newSection(t, item.Shared)
	personal := newSection(t, item.Personal)
	basic(shared)
	basic(personal)
	for _, section := range []*section{shared, personal, shared} {
		if _, err := e.buildWith(t.Context(), opt, section); err != nil {
			t.Fatal(err)
		}
	}
	if files.probes != 1 || e.dl.Requests() != 1 {
		t.Fatalf("expected one empty probe and one download; got %d, %d", files.probes, e.dl.Requests())
	}
	now := newSection(t, item.Shared)
	basic(now)
	in, _ := now.scan(t.Context())
	if st, err := Status(t.Context(), opt, []SectionInput{in}); err != nil || !st.ModelPresent {
		t.Fatalf("unsupported enable displaced valid files: %+v %v (%s)", st, err, m.Name)
	}
}

type failedPublishDir struct {
	sys.Dir
	err error
}

func (f failedPublishDir) Replace(string, string) error { return f.err }

type sealedTempStub struct {
	sys.Verity
	sum [32]byte
}

func (s sealedTempStub) SealTempVerity(_ sys.Dir, _ string, w io.WriteCloser) ([32]byte, error) {
	return s.sum, w.Close()
}

func TestVerityPublicationFailure(t *testing.T) {
	s := newSection(t, item.Shared)
	p := filepath.Join(s.root, "model")
	old := []byte("old")
	body := []byte("new")
	if err := os.WriteFile(p, old, 0600); err != nil {
		t.Fatal(err)
	}
	pin := embed.VerityDigest(body)
	f := embed.File{Name: "model", VeritySHA256: pin}
	failure := errors.New("injected rename failure")
	for _, tc := range []struct {
		name string
		sum  [32]byte
		err  error
	}{
		{"digest mismatch", [32]byte{1}, embed.ErrWrongModel},
		{"failed rename", pin, failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := failedPublishDir{s.dir, failure}
			err := installSealed(context.Background(), dir, sealedTempStub{sum: tc.sum}, f, body)
			if !errors.Is(err, tc.err) {
				t.Fatalf("prepare err=%v, want %v", err, tc.err)
			}
			entries, err := os.ReadDir(s.root)
			if err != nil || len(entries) != 1 || entries[0].Name() != "model" {
				t.Fatalf("temporary file left after error: %v, %v", entries, err)
			}
			got, err := os.ReadFile(p)
			if err != nil || !bytes.Equal(got, old) {
				t.Fatalf("old file was displaced: %q %v", got, err)
			}
		})
	}
}
