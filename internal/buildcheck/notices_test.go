package buildcheck

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestThirdPartyNotices(t *testing.T) {
	repo := root(t)
	cmd := exec.CommandContext(t.Context(), "go", "list", "-m", "-f", "{{.Dir}}", goinferModule)
	cmd.Dir = repo
	cmd.Env = append(cmd.Environ(), "GOWORK=off")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	module := strings.TrimSpace(string(out))
	if !filepath.IsAbs(module) {
		t.Fatalf("go list did not resolve the pinned goinfer module: %q", out)
	}
	for _, tc := range []struct {
		name, source string
		files        []string
	}{
		{"aikit", filepath.Join(module, "internal", "modelcore"), []string{"LICENSE", "NOTICE"}},
		{"goinfer", module, []string{"LICENSE", "NOTICE", "THIRD_PARTY_LICENSES.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(repo, "licenses", tc.name)
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
			if !slices.Equal(names, tc.files) {
				t.Fatalf("license files = %v, want %v", names, tc.files)
			}
			for _, name := range tc.files {
				got, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				want, err := os.ReadFile(filepath.Join(tc.source, name))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("licenses/%s/%s differs from the pinned goinfer copy", tc.name, name)
				}
			}
		})
	}
}
