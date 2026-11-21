package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/sys"
)

func audienceOK(t *testing.T, env Env, args ...string) string {
	t.Helper()
	status, stdout, stderr := invoke(t, env, Features{}, append([]string{"audience"}, args...)...)
	if status != 0 || stderr != "" {
		t.Fatalf("audience %v: exit %d, stdout %q, stderr %q", args, status, stdout, stderr)
	}
	return stdout
}

type noCorpus struct{ sys.Files }

func (f noCorpus) OpenRegular(dir sys.Dir, name string) (sys.RegularFile, error) {
	if strings.HasSuffix(name, ".md") {
		panic("audience scanned an item")
	}
	return f.Files.OpenRegular(dir, name)
}

func TestAudienceListPaging(t *testing.T) {
	env := writeEnv(t)
	putBytes(t, env.Dir, "pbrain.toml", []byte("[list]\nlimit=1\n"))
	putBytes(t, env.Dir, "notes/unread.md", []byte("invalid front matter"))
	env.Files = noCorpus{env.Files}
	mustEqual := func(want string, args ...string) {
		t.Helper()
		if got := audienceOK(t, env, args...); got != want {
			t.Fatalf("audience %v: %q, want %q", args, got, want)
		}
	}
	mustEqual("0 rules.\n")
	mustEqual("{}\n", "--json")
	mustBeMissing(t, filepath.Join(env.Dir, "personal.toml"))
	putBytes(t, env.Dir, "personal.toml", []byte("audience=[{match='z.org',to='personal'},{match='A.org.',to='shared'}]\n"))
	mustEqual("a.org  shared\nz.org  personal\n2 rules.\n")
	mustEqual("a.org  shared\n1 rule. 1 more: --offset 1\n", "--limit=1")
	mustEqual("z.org  personal\n1 rule.\n", "--offset=1", "--limit=1")
	mustEqual("{\"more\":1,\"next\":1,\"rules\":[{\"match\":\"a.org\",\"to\":\"shared\"}]}\n", "--json", "--limit=1")
	mustEqual("{\"rules\":[{\"match\":\"z.org\",\"to\":\"personal\"}]}\n", "--offset=1", "--json", "--limit=1")
	mustEqual("0 rules.\n", "--offset", strconv.Itoa(int(^uint(0)>>1)))
	mustEqual("{}\n", "--offset=2", "--json")
	for _, format := range [][]string{nil, {"--json"}} {
		args := append([]string{"--limit=1"}, format...)
		page := audienceOK(t, env, args...)
		minimum := utf8.RuneCountInString(page)
		mustEqual(page, append(args, "--max-chars", strconv.Itoa(minimum))...)
		status, out, diag := invoke(t, env, Features{}, append([]string{"audience", "--max-chars", strconv.Itoa(minimum - 1)}, args...)...)
		if status != 1 || out != "" || !strings.Contains(diag, "budget_too_small") || !strings.Contains(diag, strconv.Itoa(minimum)) {
			t.Fatalf("minimum %d: %d %q %q", minimum, status, out, diag)
		}
	}
	mustBeMissing(t, filepath.Join(env.Dir, ".pbrain"))
}

func TestAudienceSetCanonicalUnsetAbsent(t *testing.T) {
	env := writeEnv(t)
	path := filepath.Join(env.Dir, "personal.toml")
	audienceOK(t, env, "unset", "absent.org")
	mustBeMissing(t, path)
	// Only the brain folder is needed, even with an explicit personal view.
	outside := filepath.Join(t.TempDir(), "missing")
	initial := fmt.Sprintf("path=%q\nauthor='human:me'\nmodel_path='offline'\n[extra]\ncolor='blue'\n", outside)
	putBytes(t, env.Dir, "personal.toml", []byte(initial))
	if out := audienceOK(t, env, "--personal", "set", "HTTPS://Example.org.:443/%61?q=removed", "shared"); out != "example.org/a  shared\n" {
		t.Fatal(out)
	}
	if out := audienceOK(t, env, "set", "example.org/a/", "personal", "--json"); out != "{\"match\":\"example.org/a\",\"to\":\"personal\"}\n" {
		t.Fatal(out)
	}
	if out := audienceOK(t, env, "--personal"); out != "example.org/a  personal\n1 rule.\n" {
		t.Fatal(out)
	}
	data := append([]byte("# kept on a no-op\n"), getBytes(t, path)...)
	putBytes(t, env.Dir, "personal.toml", data)
	// A no-op must skip publication even when a replacement would fail.
	unchanged := env
	unchanged.Files = &writeFaults{Files: env.Files, beforeTemp: func(string) error { return injectedFailure }}
	audienceOK(t, unchanged, "set", "EXAMPLE.org/a", "personal")
	audienceOK(t, unchanged, "unset", "absent.org")
	if !bytes.Equal(getBytes(t, path), data) {
		t.Fatal("a no-op rewrote personal.toml")
	}
	if out := audienceOK(t, env, "unset", "Example.org/a", "--json"); out != "{\"match\":\"example.org/a\"}\n" {
		t.Fatal(out)
	}
	got := string(getBytes(t, path))
	for _, value := range []string{"human:me", outside, "offline", "[extra]", "blue"} {
		if !strings.Contains(got, value) {
			t.Errorf("edit lost %q", value)
		}
	}
	if out := audienceOK(t, env); out != "0 rules.\n" {
		t.Fatal(out)
	}
	mustBeMissing(t, outside)
}

func TestAudienceRefusesInvalidTable(t *testing.T) {
	env := writeEnv(t)
	bad := []byte("audience=[{match='user:secret@private.test',to='shared'}]\n")
	path := putBytes(t, env.Dir, "personal.toml", bad)
	for _, args := range [][]string{{}, {"set", "good.org", "shared"}, {"unset", "bad key"}} {
		status, out, diag := invoke(t, env, Features{}, append([]string{"audience", "--json"}, args...)...)
		if status != 1 || out != "" || diagnosticCode(t, diag, "error") != "invalid_config" ||
			!strings.Contains(diag, "position 1") || !strings.Contains(diag, "by hand") || strings.Contains(diag, "private.test") {
			t.Fatalf("%v: %d %q %q", args, status, out, diag)
		}
	}
	if !bytes.Equal(getBytes(t, path), bad) {
		t.Fatal("invalid table rewritten")
	}
}

func TestAudienceSettingsCommit(t *testing.T) {
	env := writeEnv(t)
	path := putBytes(t, env.Dir, "personal.toml", []byte("# initial\n"))
	for _, scope := range []string{"Docs.Google.com.", "docs.google.com/document", "acme.slack.com/archives"} {
		status, out, diag := invoke(t, env, Features{}, "audience", "set", scope, "shared", "--json")
		if status != 1 || out != "" || diagnosticCode(t, diag, "error") != "invalid" {
			t.Fatalf("%s: %d %q %q", scope, status, out, diag)
		}
	}
	fault := &writeFaults{Files: env.Files, replace: func(string, string) error { return injectedFailure }}
	env.Files = fault
	status, out, diag := invoke(t, env, Features{}, "audience", "set", "example.org", "personal", "--json")
	if status != 1 || out != "" || diagnosticCode(t, diag, "error") != "io" || string(getBytes(t, path)) != "# initial\n" {
		t.Fatalf("before commit: %d %q %q", status, out, diag)
	}
	fault.replace = nil
	fault.sync = func(string) error { return injectedFailure }
	status, out, diag = invoke(t, env, Features{}, "audience", "set", "example.org", "personal", "--json")
	if status != 0 || out != "{\"match\":\"example.org\",\"to\":\"personal\"}\n" || diagnosticCode(t, diag, "notice") != "after_write" {
		t.Fatalf("after commit: %d %q %q", status, out, diag)
	}
	if got := audienceOK(t, env); !strings.Contains(got, "example.org  personal") {
		t.Fatal(got)
	}
}

func TestAudienceRuleRoutesAdd(t *testing.T) {
	env := writeEnv(t)
	const source = "https://docs.google.com/document/d/abc/edit"
	audienceOK(t, env, "set", "https://Docs.Google.com./document/d/abc", "shared")
	args := []string{"add", "--title", "source note", "--source", source}
	written, _, _ := describeWrite(t, env, args...)
	if written.Section != "shared" {
		t.Fatal(written)
	}
	audienceOK(t, env, "unset", "docs.google.com/document/d/abc")
	status, out, diag := invoke(t, env, Features{}, append(args, "--json")...)
	if status != 1 || out != "" || diagnosticCode(t, diag, "error") != "audience_required" {
		t.Fatalf("after unset: %d %q %q", status, out, diag)
	}
}

func TestAudienceValidatesFreshSettings(t *testing.T) {
	env := writeEnv(t)
	initial := putBytes(t, env.Dir, "personal.toml", []byte("author='human:me'\n"))
	corrupt := []byte("audience=[{match='docs.google.com',to='shared'}]\n")
	env.Files = &writeFaults{Files: env.Files, beforeLock: func() error {
		return os.WriteFile(initial, corrupt, 0o600)
	}}
	status, out, diag := invoke(t, env, Features{}, "audience", "set", "ok.org", "shared", "--json")
	if status != 1 || out != "" || diagnosticCode(t, diag, "error") != "invalid_config" || !bytes.Equal(getBytes(t, initial), corrupt) {
		t.Fatalf("fresh settings: %d %q %q", status, out, diag)
	}
}

// Each child opens the same settings snapshot, then waits for its parent before trying the lock.
func TestConcurrentSettingsWriters(t *testing.T) {
	if key := os.Getenv("PBRAIN_TEST_AUDIENCE_CHILD"); key != "" {
		files := &writeFaults{Files: sys.Native(), beforeLock: func() error {
			if _, err := os.Stdout.WriteString("ready\n"); err != nil {
				return err
			}
			var gate [1]byte
			_, err := os.Stdin.Read(gate[:])
			return err
		}}
		env := Env{Files: files, Dir: os.Getenv("PBRAIN_TEST_AUDIENCE_DIR")}
		audienceOK(t, env, "set", key, "personal")
		return
	}
	env := writeEnv(t)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	var children []*exec.Cmd
	var gates []io.WriteCloser
	for _, key := range []string{"first.org", "second.org"} {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestConcurrentSettingsWriters$")
		cmd.Env = append(os.Environ(), "PBRAIN_TEST_AUDIENCE_CHILD="+key, "PBRAIN_TEST_AUDIENCE_DIR="+env.Dir)
		cmd.Stderr = os.Stderr
		in, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		out, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		var ready string
		if _, err := fmt.Fscanln(out, &ready); err != nil || ready != "ready" {
			t.Fatalf("child %s did not reach settings lock: %q, %v", key, ready, err)
		}
		children = append(children, cmd)
		gates = append(gates, in)
	}
	for _, gate := range gates {
		if _, err := gate.Write([]byte{1}); err != nil {
			t.Fatal(err)
		}
		gate.Close()
	}
	for _, cmd := range children {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	if got := audienceOK(t, env); got != "first.org  personal\nsecond.org  personal\n2 rules.\n" {
		t.Fatal(got)
	}
}
