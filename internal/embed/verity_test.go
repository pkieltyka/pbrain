package embed_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type verityFiles struct {
	sys.Files
	measure func(sys.RegularFile) ([32]byte, error)
}

func (v verityFiles) MeasureVerity(r sys.RegularFile) ([32]byte, error) { return v.measure(r) }

func (verityFiles) SealTempVerity(_ sys.Dir, _ string, w io.WriteCloser) ([32]byte, error) {
	_ = w.Close()
	return [32]byte{}, sys.ErrVerityUnavailable
}

func TestManagedVerification(t *testing.T) {
	checkErr := errors.New("measurement failed")
	body := []byte("good")
	release := embed.File{Name: "model", SHA256: sha256.Sum256(body), VeritySHA256: embed.VerityDigest(body), Size: 4}
	for _, tc := range []struct {
		name, stored string
		measured     [32]byte
		err          error
		search       error
		strict       error
	}{
		{"sealed", "good", release.VeritySHA256, nil, nil, nil},
		{"only search trusts the measured fd", "evil", release.VeritySHA256, nil, nil, embed.ErrWrongModel},
		{"unsealed", "good", [32]byte{}, sys.ErrVerityUnsealed, nil, nil},
		{"unsealed corruption", "evil", [32]byte{}, sys.ErrVerityUnsealed, embed.ErrWrongModel, embed.ErrWrongModel},
		{"unsupported fs", "good", [32]byte{}, sys.ErrVerityUnavailable, nil, nil},
		{"wrong pin with correct bytes", "good", [32]byte{2}, nil, embed.ErrWrongModel, embed.ErrWrongModel},
		{"wrong algorithm or size", "good", [32]byte{}, sys.ErrVerityProfile, embed.ErrWrongModel, embed.ErrWrongModel},
		{"measurement I/O error", "good", [32]byte{}, checkErr, checkErr, checkErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := open(t, t.TempDir())
			if err := os.WriteFile(filepath.Join(d.path, release.Name), []byte(tc.stored), 0600); err != nil {
				t.Fatal(err)
			}
			files := verityFiles{sys.Native(), func(sys.RegularFile) ([32]byte, error) {
				return tc.measured, tc.err
			}}
			m := embed.Model{Files: []embed.File{release}}
			for _, c := range []struct {
				mode embed.ManagedMode
				want error
			}{{embed.ManagedSearch, tc.search}, {embed.ManagedStrict, tc.strict}} {
				a, err := embed.VerifyManaged(files, d.dir, m, c.mode)
				if !errors.Is(err, c.want) || (a != nil) != (c.want == nil) {
					t.Fatalf("mode %v: got artifact=%t err=%v, want %v", c.mode, a != nil, err, c.want)
				}
				if a != nil && !bytes.Equal(a.Files[0], []byte(tc.stored)) {
					t.Fatal("the verifier did not return the read buffer")
				}
			}
			if tc.stored == "good" {
				if _, err := embed.Verify(files, d.dir, m); err != nil {
					t.Fatalf("raw verification consulted the measurement: %v", err)
				}
			}
		})
	}
}

func TestManagedModelRowAndCompanion(t *testing.T) {
	d := open(t, t.TempDir())
	old := []byte("good")
	new := []byte("news")
	a := embed.File{Name: "a", Size: 4, SHA256: sha256.Sum256(old), VeritySHA256: embed.VerityDigest(old)}
	b := a
	b.Name = "b"
	for _, f := range []embed.File{a, b} {
		if err := os.WriteFile(filepath.Join(d.path, f.Name), old, 0600); err != nil {
			t.Fatal(err)
		}
	}
	files := verityFiles{sys.Native(), func(sys.RegularFile) ([32]byte, error) {
		return a.VeritySHA256, nil
	}}
	updated := b
	updated.SHA256 = sha256.Sum256(new)
	updated.VeritySHA256 = embed.VerityDigest(new)
	for _, model := range []embed.Model{
		{Files: []embed.File{updated}},
		{Files: []embed.File{a, updated}},
	} {
		art, err := embed.VerifyManaged(files, d.dir, model, embed.ManagedSearch)
		if art != nil || !errors.Is(err, embed.ErrWrongModel) {
			t.Fatalf("updated release row accepted the old sealed digest: %v", err)
		}
	}
}
