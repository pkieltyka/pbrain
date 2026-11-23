package embed_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/sys"
)

func TestNativeVerity(t *testing.T) {
	d := open(t, t.TempDir())
	files := sys.Native()
	verity := files.(sys.Verity)
	for _, n := range []int{0, 17, 4096, 4097, 4096*128 + 99} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			body := make([]byte, n)
			for i := range body {
				body[i] = byte(i/4096 + i)
			}
			f := embed.File{Name: fmt.Sprintf("model-%d", n), Size: int64(n), SHA256: sha256.Sum256(body), VeritySHA256: embed.VerityDigest(body)}
			name, w, err := d.dir.CreateTemp()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write(body); err != nil {
				_ = w.Close()
				t.Fatal(err)
			}
			sum, err := verity.SealTempVerity(d.dir, name, w)
			if errors.Is(err, sys.ErrVerityUnavailable) && os.Getenv("PBRAIN_REQUIRE_FS_VERITY") != "1" {
				t.Skip("temp filesystem cannot enable fs-verity")
			}
			if err != nil || sum != f.VeritySHA256 {
				t.Fatalf("seal digest %x, err %v, want %x", sum, err, f.VeritySHA256)
			}
			if err := d.dir.Replace(name, f.Name); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(d.path, f.Name)
			writable, err := os.OpenFile(path, os.O_WRONLY, 0)
			if writable != nil {
				writable.Close()
			}
			if !errors.Is(err, syscall.EPERM) {
				t.Fatalf("writing a sealed inode: %v", err)
			}
			// Replace the entry after open. The first Artifact must still
			// read the old authenticated inode. A new open sees the change.
			replacement := append(bytes.Clone(body), 1)
			if n != 0 {
				replacement = replacement[:n]
				replacement[0] ^= 1
			}
			swap := verityFiles{files, func(r sys.RegularFile) ([32]byte, error) {
				got, err := verity.MeasureVerity(r)
				if err != nil {
					return got, err
				}
				tmp := path + ".other"
				if err := os.WriteFile(tmp, replacement, 0600); err != nil {
					return got, err
				}
				return got, os.Rename(tmp, path)
			}}
			m := embed.Model{Files: []embed.File{f}}
			a, err := embed.VerifyManaged(swap, d.dir, m, embed.ManagedSearch)
			if err != nil || !bytes.Equal(a.Files[0], body) {
				t.Fatalf("snapshot read after rename: %v", err)
			}
			if a, err := embed.VerifyManaged(files, d.dir, m, embed.ManagedSearch); a != nil || !errors.Is(err, embed.ErrWrongModel) {
				t.Fatalf("different file at the same name: %v", err)
			}
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := embed.VerifyManaged(files, d.dir, m, embed.ManagedSearch); err != nil {
				t.Fatalf("identical unsealed replacement: %v", err)
			}
			if n == 17 {
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, replacement, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
				if _, err := embed.VerifyManaged(files, d.dir, m, embed.ManagedSearch); !errors.Is(err, embed.ErrWrongModel) {
					t.Fatalf("in-place write with restored mtime: %v", err)
				}
			}
		})
	}
}
