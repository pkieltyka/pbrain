package semantic

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

func TestSyncMigratesAndRepairsSealedModel(t *testing.T) {
	e := newEnv(t)
	m := pinnedTestModel(e)
	f := m.Files[0]
	body := e.kit.Bytes[f.URL]
	dir, err := e.cache.dir("models/"+embed.FolderName(m), true)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "offline")
	p := filepath.Join(e.path, "models", embed.FolderName(m), f.Name)
	if err := os.WriteFile(outside, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, p); err != nil {
		t.Fatal(err)
	}
	v := sys.Native().(sys.Verity)
	measure := func() ([32]byte, error) {
		r, err := e.cache.files.OpenRegular(dir, f.Name)
		if err != nil {
			return [32]byte{}, err
		}
		defer r.Close()
		return v.MeasureVerity(r)
	}
	s := newSection(t, item.Shared)
	basic(s)
	in, _ := s.scan(t.Context())
	st, err := Status(t.Context(), e.opts(), []SectionInput{in})
	if err != nil || !st.ModelPresent {
		t.Fatalf("status of pre-upgrade model: %+v %v", st, err)
	}
	if _, err := measure(); err == nil {
		t.Fatal("status sealed a file")
	}
	personal := newSection(t, item.Personal)
	basic(personal)
	done := make(chan error, 2)
	for _, section := range []*section{s, personal} {
		go func() {
			_, err := e.build(section)
			done <- err
		}()
	}
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	sum, err := measure()
	if (errors.Is(err, sys.ErrVerityUnavailable) || errors.Is(err, sys.ErrVerityUnsealed)) && os.Getenv("PBRAIN_REQUIRE_FS_VERITY") != "1" {
		t.Skip("temp filesystem cannot enable fs-verity")
	}
	if err != nil || sum != f.VeritySHA256 {
		t.Fatalf("sync migration did not seal the verified bytes: %v, %x", err, sum)
	}
	bad := bytes.Repeat([]byte("x"), len(body))
	if err := os.WriteFile(outside, bad, 0600); err != nil {
		t.Fatalf("offline hardlink was sealed: %v", err)
	}
	first, _ := e.cache.files.Stat(dir, f.Name)
	e.mustBuild(s)
	second, _ := e.cache.files.Stat(dir, f.Name)
	if first.Identity != second.Identity || e.dl.Requests() != 0 {
		t.Fatal("repeated sync replaced or fetched a valid sealed file")
	}
	s.queryFromCache(e, true)
	tmp, w, err := dir.CreateTemp()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(bad); err != nil {
		_ = w.Close()
		t.Fatal(err)
	}
	if _, err := v.SealTempVerity(dir, tmp, w); err != nil {
		t.Fatal(err)
	}
	if err := dir.Replace(tmp, f.Name); err != nil {
		t.Fatal(err)
	}
	opens := e.kit.Opens.Load()
	s.queryFromCache(e, false)
	if e.kit.Opens.Load() != opens {
		t.Fatal("search reached the adapter with a mismatched fs-verity pin")
	}
	st, err = Status(t.Context(), e.opts(), []SectionInput{in})
	if err != nil || st.ModelPresent {
		t.Fatalf("status accepted the mismatched file: %+v %v", st, err)
	}
	e.mustBuild(s)
	if sum, err = measure(); err != nil || sum != f.VeritySHA256 || e.dl.Requests() != 1 {
		t.Fatalf("sync did not reprovision the bad cache: %x %v, %d requests", sum, err, e.dl.Requests())
	}
	s.queryFromCache(e, true)
}

func (s *section) queryFromCache(e *env, want bool) {
	s.t.Helper()
	src := Start(s.t.Context(), e.opts(), "alpha", []SectionDir{s.sectionDir()})
	defer src.Close()
	<-src.model
	if got := len(src.query) != 0; got != want {
		s.t.Fatalf("cache query ready=%t, want %t", got, want)
	}
}
