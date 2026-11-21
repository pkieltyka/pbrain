package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/gitx"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type noFiles struct{ sys.Files }

func (noFiles) Resolve(string, bool) (sys.ResolvedPath, error) {
	panic("discovery used in a no-brain command")
}

func invoke(t *testing.T, env Env, features Features, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env.Args, env.Stdout, env.Stderr = args, &stdout, &stderr
	status := RunWith(context.Background(), env, features)
	return status, stdout.String(), stderr.String()
}

func oneJSON(t *testing.T, line string, value any) {
	t.Helper()
	if !strings.HasSuffix(line, "\n") || strings.Count(line, "\n") != 1 {
		t.Fatalf("not one JSON line: %q", line)
	}
	if err := json.Unmarshal([]byte(line), value); err != nil {
		t.Fatalf("invalid JSON: %q: %v", line, err)
	}
}

func TestCLINoBrainDispatch(t *testing.T) {
	env := Env{Files: noFiles{}}
	for _, args := range [][]string{
		{"--help"}, {"--personal", "--version"}, {"get", "--help", "--personal"}, {"search", "--help", "--since=bad"},
	} {
		status, stdout, stderr := invoke(t, env, Features{}, args...)
		if status != 0 || stdout == "" || stderr != "" || !strings.HasSuffix(stdout, "\n") {
			t.Errorf("%v: status %d stdout %q stderr %q", args, status, stdout, stderr)
		}
	}
	status, stdout, stderr := invoke(t, env, Features{}, "--help", "--json")
	var help helpOutput
	oneJSON(t, stdout, &help)
	if status != 0 || stderr != "" || !strings.Contains(help.Help, "Usage:") || !strings.Contains(help.Help, "init") {
		t.Errorf("help: %d %+v %q", status, help, stderr)
	}
	status, stdout, stderr = invoke(t, env, Features{}, "--version", "--json")
	var version versionOutput
	oneJSON(t, stdout, &version)
	if status != 0 || stderr != "" || version.Version == "" {
		t.Errorf("version: %d %+v %q", status, version, stderr)
	}
	for _, enabled := range []bool{true, false} {
		status, stdout, stderr := invoke(t, env, Features{BM25: enabled}, "search", "--help")
		if status != 0 || stderr != "" || strings.Contains(stdout, "--bm25") != enabled ||
			!strings.Contains(stdout, "--context") || !strings.Contains(stdout, "--semantic") {
			t.Errorf("help for BM25=%v: %d %q %q", enabled, status, stdout, stderr)
		}
	}
}

func TestSyncAlias(t *testing.T) {
	const message = "pbrain index is now pbrain sync: the old name will be removed in the next release"
	env := Env{Files: noFiles{}}
	status, root, stderr := invoke(t, env, Features{}, "--help")
	if status != 0 || stderr != "" || !strings.Contains(root, "\n  sync ") || strings.Contains(root, "\n  index ") {
		t.Fatalf("root help: %d %q %q", status, root, stderr)
	}
	status, help, stderr := invoke(t, env, Features{}, "sync", "--help")
	if status != 0 || stderr != "" || !strings.HasPrefix(help, "Usage: pbrain sync ") || !strings.Contains(help, "--status") {
		t.Fatalf("sync help: %d %q %q", status, help, stderr)
	}
	if status, out, stderr := invoke(t, env, Features{}, "index", "--help"); status != 0 || out != help || stderr != "" {
		t.Fatalf("alias help: %d %q %q", status, out, stderr)
	}
	status, _, stderr = invoke(t, env, Features{}, "sync", "--bogus", "--json")
	if code, out, diag := invoke(t, env, Features{}, "index", "--bogus", "--json"); status != 2 || code != status || out != "" || diag != stderr {
		t.Fatalf("alias usage: %d %q %q", code, out, diag)
	}
	for _, tc := range []struct {
		args   []string
		notice string
	}{
		{[]string{"--status"}, "command_renamed: " + message + "\n"},
		{[]string{"--status", "--json"}, `{"notice":{"code":"command_renamed","message":"` + message + `"}}` + "\n"},
	} {
		// A valid invocation can fail during discovery. The alias notice comes first.
		status, stdout, stderr := invoke(t, Env{}, Features{}, append([]string{"sync"}, tc.args...)...)
		code, out, diag := invoke(t, Env{}, Features{}, append([]string{"index"}, tc.args...)...)
		if status != 1 || code != status || out != stdout || diag != tc.notice+stderr {
			t.Errorf("alias %v: %d %q %q", tc.args, code, out, diag)
		}
	}
}

func TestInitDispatch(t *testing.T) {
	env := Env{Dir: t.TempDir(), Files: sys.Native()}
	status, stdout, stderr := invoke(t, env, Features{}, "--personal", "init", "--path", "notes", "--json")
	var init struct{ Path, Selector, Hint string }
	oneJSON(t, stdout, &init)
	if status != 0 || stderr != "" || init.Path != "notes" || init.Selector != "" || !strings.Contains(init.Hint, "--brain") {
		t.Fatalf("init: %d %+v %q", status, init, stderr)
	}
	for _, name := range []string{"pbrain.toml", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(env.Dir, "notes", name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"personal", ".pbrain"} {
		if _, err := os.Stat(filepath.Join(env.Dir, "notes", name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("init created local item data %q, stat %v", name, err)
		}
	}
	marker := filepath.Join(env.Dir, "notes", "pbrain.toml")
	info, err := os.Stat(marker)
	if err != nil {
		t.Fatal(err)
	}
	status, stdout, stderr = invoke(t, env, Features{}, "--brain", "unselected", "init", "--path", "notes")
	if status != 0 || stdout != "initialized notes — outside the brain, use --brain or PBRAIN_DIR (from this directory, use this path)\n" || stderr != "" {
		t.Fatalf("repeated init: %d %q %q", status, stdout, stderr)
	}
	after, err := os.Stat(marker)
	if err != nil || !os.SameFile(info, after) || !after.ModTime().Equal(info.ModTime()) {
		t.Fatalf("repeated init rewrote marker, stat %v", err)
	}
}

func TestInitSelectorOutput(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	for _, tc := range []struct {
		name, path, text, jsonPath, jsonSelector string
	}{
		{"subfolder", "notes", "initialized notes\noutside the brain, use --brain or PBRAIN_DIR (from this directory): ../notes\n", `"notes"`, `"../notes"`},
		{"quoted path", "a \"b\u2028c", "initialized \"a \\\"b\\u2028c\"\noutside the brain, use --brain or PBRAIN_DIR (from this directory): \"../a \\\"b\\u2028c\"\n", `"a \"b\u2028c"`, `"../a \"b\u2028c"`},
		{"withheld path", "bad\xff", "initialized ? — selector unavailable (invalid UTF-8)\n", "", ""},
		{"default", "docs/brain", "initialized docs/brain\n", `"docs/brain"`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "withheld path" && !badNamesKept(t) {
				t.Skip(badNamesSkip)
			}
			root := t.TempDir()
			if out, err := exec.Command(git, "-C", root, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %s: %v", out, err)
			}
			env := Env{Dir: filepath.Join(root, "src"), Files: sys.Native(), Git: gitx.NewRunner(git, os.Environ())}
			if err := os.Mkdir(env.Dir, 0700); err != nil {
				t.Fatal(err)
			}
			status, out, diag := invoke(t, env, Features{}, "init", "--path", tc.path)
			if status != 0 || out != tc.text || diag != "" {
				t.Fatalf("text: %d %q %q", status, out, diag)
			}
			if _, err := os.Stat(filepath.Join(root, tc.path, "pbrain.toml")); err != nil {
				t.Fatal(err)
			}
			status, out, diag = invoke(t, env, Features{}, "init", "--path", tc.path, "--json")
			var fields map[string]json.RawMessage
			oneJSON(t, out, &fields)
			if status != 0 || diag != "" || string(fields["path"]) != tc.jsonPath || string(fields["selector"]) != tc.jsonSelector || strings.Contains(out, root) || strings.Contains(out, "\ufffd") {
				t.Fatalf("json: %d %q %q", status, out, diag)
			}
		})
	}
}

func TestCLISingleInstantAndZone(t *testing.T) {
	base, err := time.Parse(time.RFC3339Nano, "2026-03-29T04:59:59.123Z")
	if err != nil {
		t.Fatal(err)
	}
	est := time.FixedZone("UTC-5", -18000)
	for _, tc := range []struct {
		name, tz string
		point    time.Time
		bound    string
	}{
		{"supplied Toronto", "America/Toronto", base, "2026-03-29T04:00:00Z"},
		{"unset uses effective local", "", base.In(est), "2026-03-28T05:00:00Z"},
		{"explicit empty supplied as UTC by main", "", base.UTC(), "2026-03-29T00:00:00Z"},
		{"colon-prefixed IANA zone", ":Europe/London", base, "2026-03-29T00:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			env := Env{Now: func() time.Time { calls++; return tc.point }}
			env.Getenv = func(name string) string {
				if name != "TZ" {
					t.Fatalf("unexpected environment read %q", name)
				}
				return tc.tz
			}
			in, err := parseOptions([]string{"search", "term", "--since=today"}, Features{})
			if err != nil {
				t.Fatal(err)
			}
			if err := in.captureTime(&env); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || !in.now.Equal(tc.point) || !env.Now().Equal(tc.point) || calls != 1 ||
				in.filter.Window.Since.UTC().Format(time.RFC3339) != tc.bound {
				t.Errorf("invocation lost its snapshot: %v %+v %d", in.now, in.filter.Window, calls)
			}
		})
	}
	env := Env{
		Files: noFiles{},
		Now:   func() time.Time { return base },
		Getenv: func(string) string {
			return "not/a/zone"
		},
	}
	status, stdout, stderr := invoke(t, env, Features{}, "list", "--since=today", "--json")
	if status != 2 || stdout != "" || diagnosticCode(t, stderr, "error") != "usage" ||
		strings.Contains(stderr, "not/a/zone") {
		t.Errorf("invalid TZ: %d %q %q", status, stdout, stderr)
	}
	in, err := parseOptions([]string{"search", "term", "--since=1d"}, Features{})
	if err != nil || in.captureTime(&env) != nil || in.filter.Window.Since.Unix() != base.Unix()-86400 {
		t.Fatalf("absolute span needed a time zone: %+v %v", in.filter.Window, err)
	}
}

func diagnosticCode(t *testing.T, stderr, field string) string {
	t.Helper()
	var got map[string]diagnostic
	oneJSON(t, stderr, &got)
	if len(got) != 1 || got[field].Code == "" {
		t.Fatalf("wrong diagnostic: %q", stderr)
	}
	return got[field].Code
}

func TestCLIExitStreams(t *testing.T) {
	env := Env{Dir: t.TempDir(), Files: sys.Native()}
	for _, tc := range []struct {
		args []string
		exit int
		json bool
		code string
	}{
		{[]string{"search", "query", "--bad", "--json"}, 2, true, "usage"},
		{[]string{"--bad", "search", "query", "--json"}, 2, true, "usage"},
		{[]string{"add", "--title", "x", "--bad", "--body", "--json"}, 2, false, "usage"},
		{[]string{"add", "--title", "x", "--bad", "--body=--json", "--json"}, 2, true, "usage"},
		{[]string{"search", "query", "--bad", "--", "--json"}, 2, false, "usage"},
		{[]string{"add", "--title=", "--json"}, 1, true, "invalid"},
		{[]string{"list", "--since=not-a-date", "--json"}, 2, true, "usage"},
		{[]string{"list", "--since=", "--json"}, 2, true, "usage"},
		{[]string{"get", "id", "--color", "--json"}, 2, true, "usage"},
		{[]string{"--json", "get", "--color", "id"}, 2, true, "usage"},
		{[]string{"search", "query", "--bm25"}, 2, false, "usage"},
		{[]string{"search", "query"}, 1, false, "no_brain"},
	} {
		status, stdout, stderr := invoke(t, env, Features{}, tc.args...)
		if status != tc.exit || stdout != "" {
			t.Errorf("%v: got %d %q %q", tc.args, status, stdout, stderr)
		}
		if tc.json {
			if diagnosticCode(t, stderr, "error") != tc.code {
				t.Errorf("%v: stderr = %q, want %q", tc.args, stderr, tc.code)
			}
		} else if !strings.HasPrefix(stderr, tc.code+": ") || strings.Count(stderr, "\n") != 1 {
			t.Errorf("%v: expected text %s, got %q", tc.args, tc.code, stderr)
		}
	}
	panicEnv := env
	panicEnv.Now = func() time.Time { panic("do not publish this detail") }
	status, stdout, stderr := invoke(t, panicEnv, Features{}, "--json", "--version")
	if status != 1 || stdout != "" || diagnosticCode(t, stderr, "error") != "io" ||
		strings.Contains(stderr, "do not publish") {
		t.Errorf("unexpected panic handling: %d %q %q", status, stdout, stderr)
	}
}

type shortWriter struct{}

func (shortWriter) Write([]byte) (int, error) { return 1, io.ErrShortWrite }

func TestCommandOutputDelivery(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := Env{Stdout: &stdout, Stderr: &stderr}
	secret := "gh" + "p_" + strings.Repeat("s", 36)
	type expect struct {
		exit int
		out  string
		key  string
		code string
	}
	for _, tc := range []struct {
		cmd string
		out outcome
		expect
	}{
		{"check", outcome{data: []byte("{}\n"), exit: 1}, expect{1, "{}\n", "", ""}},
		{"delete", outcome{data: []byte("{}\n"), err: &errs.Error{Code: "confirm_required", Message: "confirm", Exit: 1}}, expect{1, "{}\n", "error", "confirm_required"}},
		{"get", outcome{data: []byte("{}\n"), err: errors.New("private path")}, expect{1, "", "error", "io"}},
		{"add", outcome{data: []byte("{}\n"), committed: true, err: ioFailure("cleanup failed")}, expect{0, "{}\n", "notice", "after_write"}},
		{"add", outcome{err: ioFailure(secret)}, expect{1, "", "error", "io"}},
	} {
		stdout.Reset()
		stderr.Reset()
		status := finish(env, &invocation{cmd: command{name: tc.cmd}, json: true}, tc.out)
		if status != tc.exit || stdout.String() != tc.expect.out || strings.Contains(stderr.String(), secret) ||
			strings.Contains(stderr.String(), "private path") {
			t.Errorf("deliver %s: %d %q %q", tc.cmd, status, stdout.String(), stderr.String())
		}
		if tc.key != "" {
			if diagnosticCode(t, stderr.String(), tc.key) != tc.code {
				t.Errorf("wrong diagnostic %q", stderr.String())
			}
		} else if stderr.Len() != 0 {
			t.Errorf("unexpected stderr %q", stderr.String())
		}
	}
	env.Stdout = shortWriter{}
	for _, committed := range []bool{true, false} {
		stderr.Reset()
		got := finish(env, &invocation{json: true}, outcome{data: []byte("{}\n"), committed: committed})
		if committed {
			if got != 0 || diagnosticCode(t, stderr.String(), "notice") != "after_write" || !strings.Contains(stderr.String(), "Look up the item") {
				t.Errorf("committed write delivery: %d %q", got, stderr.String())
			}
		} else if got != 1 || diagnosticCode(t, stderr.String(), "error") != "io" {
			t.Errorf("read delivery: %d %q", got, stderr.String())
		}
	}
}

func TestDiagnosticLineShape(t *testing.T) {
	var stderr bytes.Buffer
	emitNotice(&stderr, true, errs.Notice{Code: "destination", Message: "personal <other>\nconfigured"})
	var m map[string]diagnostic
	oneJSON(t, stderr.String(), &m)
	want := map[string]diagnostic{"notice": {Code: "destination", Message: "personal <other> configured"}}
	if !reflect.DeepEqual(m, want) || strings.Contains(stderr.String(), `\u003c`) {
		t.Errorf("unexpected notice %s", stderr.String())
	}
}
