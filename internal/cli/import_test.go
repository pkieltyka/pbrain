package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/importer/fetch"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

func importFailure(t *testing.T, env Env, code string, args ...string) string {
	t.Helper()
	status, out, diag := invoke(t, env, Features{}, append([]string{"import", "--json"}, args...)...)
	lines := strings.Split(strings.TrimSuffix(diag, "\n"), "\n")
	if status != 1 || out != "" || diagnosticCode(t, lines[len(lines)-1]+"\n", "error") != code {
		t.Fatalf("%v: exit %d, stdout %q, stderr %q", args, status, out, diag)
	}
	return diag
}

func TestDuplicatesAndAtomicOriginal(t *testing.T) {
	env := writeEnv(t)
	exportDir := t.TempDir()
	raw := []byte("---\nkind: decision\nauthor: forged\nstatus: verified\n---\n\n## Transcript\nCafe\x00 notes[^1]\n[^1]: cited")
	one := putBytes(t, exportDir, "one/transcript.md", raw)
	two := putBytes(t, exportDir, "two/transcript.md", []byte("different"))
	alias := putBytes(t, exportDir, "elsewhere.txt", raw)
	w, h, body := describeWrite(t, env, "import", one)
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	stored := filepath.Join(env.Dir, ".pbrain", "originals", digest)
	if h.Kind != item.Source || h.Origin != item.OriginFile || h.Status != item.Provisional || h.Title != "Transcript" || h.Source != "transcript.md" || h.Author != "agent:default" || h.SHA256 != digest || !h.Created.Equal(h.Fetched) || !h.Fetched.Equal(writeTime) || body != "## Transcript\nCafe notes[^1]\n[^1]: cited" || !bytes.Equal(getBytes(t, stored), raw) {
		t.Fatalf("%+v, body %q", h, body)
	}
	if diag := importFailure(t, env, "duplicate", alias); !strings.Contains(diag, w.ID) {
		t.Fatal(diag)
	}
	other, _, _ := describeWrite(t, env, "import", two)
	if other.ID == w.ID {
		t.Fatal("different files with same basename collapsed")
	}
	_, h, _ = describeWrite(t, env, "import", alias, "--replaces", w.ID)
	if len(h.Replaces) != 1 || h.Replaces[0] != w.ID {
		t.Fatal(h)
	}
	missing := filepath.Join(exportDir, "absent.txt")
	importFailure(t, env, "not_found", missing)
	empty := putBytes(t, exportDir, "empty.txt", nil)
	_, h, body = describeWrite(t, env, "import", empty)
	if body != "" || len(getBytes(t, filepath.Join(env.Dir, ".pbrain", "originals", h.SHA256))) != 0 {
		t.Fatal("empty original missing")
	}
	failed := writeEnv(t)
	failed.Files = &writeFaults{Files: failed.Files, beforeLink: func(dir, name string) error {
		if filepath.Base(dir) == "originals" {
			return injectedFailure
		}
		return nil
	}}
	importFailure(t, failed, "io", one)
	mustBeMissing(t, filepath.Join(failed.Dir, "sources"))
}

type importOpenProbe struct {
	sys.Files
	path   string
	access func()
}

func (p importOpenProbe) Resolve(path string, missing bool) (sys.ResolvedPath, error) {
	if path == p.path {
		p.access()
	}
	return p.Files.Resolve(path, missing)
}

func TestImportAudiencePreflight(t *testing.T) {
	env := writeEnv(t)
	env.Net = fetch.Net{Resolver: struct{ fetch.Resolver }{}, Dialer: struct{ fetch.Dialer }{}}
	raw := []byte("# Workshop\ncontent")
	src := putBytes(t, t.TempDir(), "export.md", raw)
	const url = "https://docs.google.com/document/d/abc"
	var stdout, stderr bytes.Buffer
	accessed := false
	env.Files = importOpenProbe{env.Files, src, func() {
		accessed = true
		if !strings.Contains(stderr.String(), "\"code\":\"destination\"") {
			t.Error("file accessed before notice")
		}
	}}
	env.Stdout, env.Stderr = &stdout, &stderr
	env.Args = []string{"import", src, "--source", url, "--json"}
	if exit := Run(context.Background(), env); exit != 1 || diagnosticCode(t, stderr.String(), "error") != "audience_required" || accessed || stdout.Len() != 0 {
		t.Fatalf("preflight: %d, read=%t, stderr=%s", exit, accessed, &stderr)
	}
	stderr.Reset()
	env.Args = append(env.Args, "--shared")
	if exit := Run(context.Background(), env); exit != 0 || !accessed || !strings.Contains(stderr.String(), "copy of the original") {
		t.Fatalf("shared import: %d, %s", exit, &stderr)
	}
	env.Files = sys.Native()
	audienceOK(t, env, "set", url, "personal")
	w, _, _ := describeWrite(t, env, "import", src, "--source", url)
	if w.Section != "personal" {
		t.Fatal(w)
	}
	diag := importFailure(t, env, "duplicate", src, "--source", url)
	if strings.Contains(diag, w.ID) {
		t.Fatal("revealed existing personal id")
	}
	private := putBytes(t, env.Dir, "personal/large.md", nil)
	if err := os.Truncate(private, (5<<20)+1); err != nil {
		t.Fatal(err)
	}
	importFailure(t, env, "not_found", private, "--shared")
}

func TestImportLinkedCLI(t *testing.T) {
	env := writeEnv(t)
	env.Net = fetch.Net{Resolver: struct{ fetch.Resolver }{}, Dialer: struct{ fetch.Dialer }{}}
	const url = "https://example.org/doc?q=first#A"
	w, h, body := describeWrite(t, env, "import", "--link-only", url, "--summary", "lantern reference")
	if h.Origin != item.OriginLink || h.Source != url || body != "lantern reference" || !h.Fetched.IsZero() || h.SHA256 != "" {
		t.Fatalf("%+v %q", h, body)
	}
	mustBeMissing(t, filepath.Join(env.Dir, ".pbrain", "originals"))
	if diag := importFailure(t, env, "duplicate", "--link-only", url); !strings.Contains(diag, w.ID) {
		t.Fatal(diag)
	}
	for _, other := range []string{"https://Example.org/doc?q=first#A", "https://example.org/doc?q=first#B", "https://example.org/doc?q=second#A"} {
		_, _, body := describeWrite(t, env, "import", "--link-only", other)
		if body != "" {
			t.Fatal("unexpected linked content")
		}
	}
	importFailure(t, env, "audience_required", "--link-only", "https://docs.google.com/document/d/xyz")
	status, out, diag := invoke(t, env, Features{}, "import", "--json", "--link-only", "--shared", "https://docs.google.com/document/d/xyz", "--summary", "gh"+"p_"+strings.Repeat("x", 36))
	if status != 0 || out == "" || !strings.Contains(diag, "title, link, tags and summary") || !strings.Contains(diag, "secret_suspected") || strings.Contains(diag, "gh"+"p_") || strings.Contains(diag, "original") {
		t.Fatalf("linked notices: %d %s", status, diag)
	}
}
