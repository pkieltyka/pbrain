package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/audience"
	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

var writeTime = time.Date(2026, 11, 21, 12, 34, 56, 0, time.UTC)

func writeEnv(t *testing.T) Env {
	t.Helper()
	dir := t.TempDir()
	putBytes(t, dir, "pbrain.toml", nil)
	return Env{
		Dir: dir, Files: sys.Native(), Rand: rand.Reader, Now: func() time.Time { return writeTime },
		Getenv: func(name string) string {
			if name == "PBRAIN_AUTHOR" {
				return "agent:default"
			}
			return ""
		},
	}
}

func putBytes(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func getBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustBeMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no %s, got %v", path, err)
	}
}

func storeItem(t *testing.T, root, section, id string, d item.Draft, body string) string {
	t.Helper()
	d.Title, d.Created, d.Author = id, writeTime, "agent:fixture"
	data, err := item.Emit(&d, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return putBytes(t, root, filepath.Join(section, d.Kind.String()+"s", id+".md"), data)
}

func describeWrite(t *testing.T, env Env, args ...string) (writeOutput, item.Header, string) {
	t.Helper()
	args = append(slices.Clone(args), "--json")
	status, stdout, stderr := invoke(t, env, Features{}, args...)
	if status != 0 {
		t.Fatalf("%v: exit %d stdout %q stderr %q", args, status, stdout, stderr)
	}
	var out writeOutput
	oneJSON(t, stdout, &out)
	data := getBytes(t, filepath.Join(env.Dir, out.Path))
	h, start, _, err := item.Parse(out.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	return out, h, string(data[start:])
}

type readerFunc func([]byte) (int, error)

func (r readerFunc) Read(p []byte) (int, error) { return r(p) }

func TestAddBodyAndFlags(t *testing.T) {
	env := writeEnv(t)
	putBytes(t, env.Dir, "personal.toml", []byte("author = 'human:settings'\n"))
	for _, tc := range []struct {
		title  string
		stdin  io.Reader
		flags  []string
		author string
		body   string
	}{
		{"terminal", nil, nil, "agent:default", ""},
		{"redirected empty", strings.NewReader(""), nil, "agent:default", ""},
		{"redirected body", strings.NewReader("text 🌱\r\n"), nil, "agent:default", "text 🌱\r\n"},
		{"explicit body", readerFunc(func([]byte) (int, error) { panic("stdin was read") }),
			[]string{"--body=", "--author=human:flag"}, "human:flag", ""},
		{"largest valid body", strings.NewReader(strings.Repeat("🪵", maxBodyChars)), nil, "agent:default",
			strings.Repeat("🪵", maxBodyChars)},
	} {
		t.Run(tc.title, func(t *testing.T) {
			call := env
			call.Stdin = tc.stdin
			out, h, body := describeWrite(t, call, append([]string{"add", "--title", tc.title}, tc.flags...)...)
			if h.Author != tc.author || body != tc.body || !h.Created.Equal(writeTime) || h.Kind != item.Note ||
				h.Status != item.Provisional || out.Section != "shared" {
				t.Errorf("wrong add header %+v, body bytes %d, section %s", h, len(body), out.Section)
			}
		})
	}
	useSettings := env
	useSettings.Getenv = nil
	_, h, _ := describeWrite(t, useSettings, "add", "--title", "with settings")
	if h.Author != "human:settings" {
		t.Fatalf("wrong settings author %q", h.Author)
	}
	_, h, _ = describeWrite(t, env, "add", "--title", "decision", "--kind", "decision", "--decided", "human:owner")
	if h.Status != item.Verified || h.Decided != "human:owner" {
		t.Fatalf("wrong decision header %+v", h)
	}
	_, h, _ = describeWrite(t, env, "add", "--title", "learning", "--kind", "learning", "--verified", "agent:verifier")
	if h.Status != item.Verified || h.Verified != "agent:verifier" {
		t.Fatalf("wrong learning header %+v", h)
	}
	file := putBytes(t, env.Dir, "input.txt", []byte("exact source\r\né"))
	_, _, body := describeWrite(t, env, "add", "--title", "from file", "--body-file", file)
	if body != "exact source\r\né" {
		t.Errorf("body changed to %q", body)
	}
	putBytes(t, env.Dir, "personal/private.txt", []byte("secret fixture"))
	for _, tc := range []struct {
		args   []string
		stdin  io.Reader
		code   string
		status int
	}{
		{[]string{"--body-file", "personal/private.txt"}, nil, "not_found", 1},
		{[]string{"--body-file", "missing.txt"}, nil, "not_found", 1},
		{nil, strings.NewReader("text\xff"), "invalid", 1},
		{nil, strings.NewReader(strings.Repeat("x", maxBodyChars+1)), "too_large", 1},
		{nil, strings.NewReader(strings.Repeat("🪵", maxBodyChars+1)), "too_large", 1},
		{[]string{"--kind", "decision"}, nil, "usage", 2},
		{[]string{"--kind", "decision", "--decided", "human:x", "--verified", "human:y"}, nil, "usage", 2},
		{[]string{"--kind", "learning", "--decided", "human:x"}, nil, "usage", 2},
		{[]string{"--kind", "source"}, nil, "invalid", 1},
	} {
		call := env
		call.Stdin = tc.stdin
		status, stdout, stderr := invoke(t, call, Features{}, append([]string{"add", "--json", "--title", "refusal"}, tc.args...)...)
		if status != tc.status || stdout != "" || diagnosticCode(t, stderr, "error") != tc.code {
			t.Fatalf("%v: status %d stdout %q stderr %q", tc.args, status, stdout, stderr)
		}
	}
}

func TestWriteViewsAndDisclosure(t *testing.T) {
	env := writeEnv(t)
	putBytes(t, env.Dir, "personal.toml", []byte("[[audience]]\nmatch='example.test/private'\nto='personal'\n"))
	storeItem(t, env.Dir, "", "shared", item.Draft{Kind: item.Note}, "")
	storeItem(t, env.Dir, "personal", "private-replaced", item.Draft{Kind: item.Note}, "")
	storeItem(t, env.Dir, "personal", "private-successor", item.Draft{Kind: item.Note, Replaces: []string{"private-replaced"}}, "")
	putBytes(t, env.Dir, "personal/notes/skipped.md", []byte("not front matter"))
	add := []string{"add", "--title", "routed", "--source", "https://example.test/private/page"}
	var out, diag bytes.Buffer
	env.Stderr, env.Stdout = &diag, &out
	env.Args = append(slices.Clone(add), "--relates", "shared", "--json")
	env.Stdin = readerFunc(func([]byte) (int, error) {
		if !strings.Contains(diag.String(), "destination") || !strings.Contains(diag.String(), "personal") {
			t.Error("input arrived before the destination notice")
		}
		return 0, io.EOF
	})
	if got := Run(context.Background(), env); got != 0 {
		t.Fatalf("routed add failed: %d %s", got, diag.String())
	}
	var result writeOutput
	oneJSON(t, out.String(), &result)
	if result.Section != "personal" || strings.Contains(diag.String(), "skipped_files") {
		t.Fatalf("routed write disclosed private inputs: %s %s", out.String(), diag.String())
	}
	for _, jsonMode := range []bool{false, true} {
		for _, explicit := range []bool{false, true} {
			args := append(slices.Clone(add), "--replaces", "private-replaced", "--body=")
			if explicit {
				args = append(args, "--personal")
			}
			if jsonMode {
				args = append(args, "--json")
			}
			status, stdout, stderr := invoke(t, env, Features{}, args...)
			if status != 1 || stdout != "" || !strings.Contains(stderr, "invalid") ||
				strings.Contains(stderr, "private-successor") != explicit ||
				strings.Contains(stderr, "private-replaced") != explicit ||
				strings.Contains(stderr, "skipped_files") != explicit {
				t.Fatalf("json=%v explicit=%v: %d %q %q", jsonMode, explicit, status, stdout, stderr)
			}
		}
	}
	needsChoice := env
	reads := 0
	needsChoice.Stdin = readerFunc(func([]byte) (int, error) {
		reads++
		return 0, io.EOF
	})
	status, stdout, stderr := invoke(t, needsChoice, Features{}, "add", "--title", "not yet", "--source", "https://docs.google.com/document/d/id", "--json")
	if status != 1 || reads != 0 || stdout != "" || diagnosticCode(t, stderr, "error") != "audience_required" {
		t.Errorf("unrouted mixed source accessed input: %d %d %q %q", status, reads, stdout, stderr)
	}
	// New private sections are created only when a personal write needs them.
	empty := writeEnv(t)
	r, _, _ := describeWrite(t, empty, "add", "--title", "first private note", "--personal")
	if r.Section != "personal" || !strings.HasPrefix(r.Path, "personal/") {
		t.Fatalf("wrong initial personal destination: %+v", r)
	}

	outside := writeEnv(t)
	external := t.TempDir()
	putBytes(t, outside.Dir, "personal.toml", []byte(fmt.Sprintf("path = %q\n", external)))
	status, stdout, stderr = invoke(t, outside, Features{}, "add", "--title", "external", "--personal", "--json")
	result = writeOutput{}
	oneJSON(t, stdout, &result)
	if status != 0 || stderr != "" || result.Section != "personal" || result.Path != "personal:notes/"+result.ID+".md" {
		t.Fatalf("external personal destination: %d %q %q", status, stdout, stderr)
	}
	getBytes(t, filepath.Join(external, "notes", result.ID+".md"))
	mustBeMissing(t, filepath.Join(outside.Dir, "personal"))
}

type unavailableRoot struct {
	sys.Files
	path string
}

func (f unavailableRoot) OpenRoot(path string, create bool) (sys.Dir, error) {
	if path == f.path {
		return nil, os.ErrPermission
	}
	return f.Files.OpenRoot(path, create)
}

func TestSharedWriteWithUnreachablePersonalRoot(t *testing.T) {
	env := writeEnv(t)
	external, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	putBytes(t, env.Dir, "personal.toml", []byte(fmt.Sprintf("path = %q\n", external)))
	env.Files = unavailableRoot{env.Files, external}
	describeWrite(t, env, "add", "--title", "shared only")
	status, stdout, stderr := invoke(t, env, Features{}, "add", "--title", "personal unavailable", "--personal", "--json")
	if status != 1 || stdout != "" || diagnosticCode(t, stderr, "error") != "personal_unavailable" {
		t.Fatalf("unreachable personal section: %d %q %q", status, stdout, stderr)
	}
}

func TestAudienceChangesBeforeCommit(t *testing.T) {
	for _, drift := range []string{"rule", "path"} {
		t.Run(drift, func(t *testing.T) {
			env := writeEnv(t)
			cfg := []byte("[[audience]]\nmatch='example.test'\nto='personal'\n")
			putBytes(t, env.Dir, "personal.toml", cfg)
			var stderr, stdout bytes.Buffer
			started, finishInput := make(chan struct{}), make(chan struct{})
			env.Stdin = readerFunc(func([]byte) (int, error) {
				close(started)
				<-finishInput
				return 0, io.EOF
			})
			env.Args = []string{"add", "--title", "stalled", "--source", "https://example.test/page", "--json"}
			env.Stdout, env.Stderr = &stdout, &stderr
			done := make(chan int, 1)
			go func() { done <- Run(context.Background(), env) }()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("did not reach input")
			}
			other := env
			other.Args = []string{"add", "--title", "second", "--personal", "--body="}
			other.Stdout, other.Stderr = io.Discard, io.Discard
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			otherStatus := Run(ctx, other)
			cancel()
			if drift == "rule" {
				cfg = bytes.ReplaceAll(cfg, []byte("to='personal'"), []byte("to='shared'"))
			} else {
				if err := os.Mkdir(filepath.Join(env.Dir, "other-personal"), 0o700); err != nil {
					t.Error(err)
				}
				cfg = append([]byte("path = 'other-personal'\n"), cfg...)
			}
			putBytes(t, env.Dir, "personal.toml", cfg)
			close(finishInput)
			select {
			case status := <-done:
				if otherStatus != 0 || status != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code":"conflict"`) {
					t.Fatalf("independent write %d, stalled write %d stdout %q stderr %q", otherStatus, status, stdout.String(), stderr.String())
				}
			case <-time.After(3 * time.Second):
				t.Fatal("stalled write did not finish")
			}
			matches, err := filepath.Glob(filepath.Join(env.Dir, "personal", "notes", "*stalled*"))
			if err != nil || len(matches) != 0 {
				t.Fatalf("write escaped its failed plan: %v, %v", matches, err)
			}
		})
	}
}

func TestNewLinksAndCurrentDuplicates(t *testing.T) {
	env := writeEnv(t)
	occupied := "20261121-123456-new-aaaa"
	storeItem(t, env.Dir, "", "holder", item.Draft{Kind: item.Note, Replaces: []string{occupied}}, "")
	env.Rand = bytes.NewReader([]byte{0, 0, 0, 0, 0, 0, 0, 1})
	got, _, _ := describeWrite(t, env, "add", "--title", "new")
	if got.ID != "20261121-123456-new-aaab" {
		t.Fatalf("occupied incoming replacement activated: %+v", got)
	}

	env.Rand = rand.Reader
	for _, name := range []string{"first", "second"} {
		storeItem(t, env.Dir, "", name, item.Draft{Kind: item.Source, Origin: item.OriginLink, Source: "https://example.test/article?a=1#here"}, "")
	}
	b, err := brain.Open(context.Background(), brain.Env{Dir: env.Dir, Files: env.Files, Getenv: env.Getenv, Now: env.Now}, brain.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	wc := writeContext{Brain: b, Rand: env.Rand, Now: writeTime}
	link := func(url string, replaces ...string) error {
		r, err := routeWrite(wc, audience.Request{Source: url, Method: audience.LinkedURL})
		if err != nil {
			return err
		}
		_, err = publishNew(context.Background(), wc, newItemInput{route: r, draft: item.Draft{
			Kind: item.Source, Title: "linked", Source: url, Created: writeTime, Author: "agent:test",
			Origin: item.OriginLink, Replaces: replaces,
		}})
		return err
	}
	requireCode(t, link("https://example.test/article?a=1#here", "first"), "duplicate")
	if err := link("https://example.test/article?a=1#here", "first", "second"); err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"https://example.test/article?a=2#here", "https://example.test/article?a=1#there"} {
		if err := link(url); err != nil {
			t.Fatal(err)
		}
	}

	// File imports use their digest even when they cite the linked URL.
	raw := []byte("file bytes")
	digest := sha256.Sum256(raw)
	route, err := routeWrite(wc, audience.Request{Explicit: audience.Shared, Source: "https://example.test/article?a=1#here"})
	if err != nil {
		t.Fatal(err)
	}
	input := newItemInput{route: route, body: raw, original: &copiedOriginal{digest, raw}, draft: item.Draft{
		Kind: item.Source, Title: "copied file", Source: "https://example.test/article?a=1#here",
		Created: writeTime, Author: "agent:test", Origin: item.OriginFile, SHA256: hex.EncodeToString(digest[:]), Fetched: writeTime,
	}}
	r, err := publishNew(context.Background(), wc, input)
	if err != nil || r.ID == "" {
		t.Fatalf("file citation was treated as a URL duplicate: %+v %v", r, err)
	}
	_, err = publishNew(context.Background(), wc, input)
	requireCode(t, err, "duplicate")
}

func TestSecretAndTagNotices(t *testing.T) {
	env := writeEnv(t)
	putBytes(t, env.Dir, "pbrain.toml", []byte("[tags]\nknown='Known tag'\n"))
	tag := "xo" + "xb-" + strings.Repeat("x", 24)
	actor := "gh" + "p_" + strings.Repeat("v", 36)
	body := "review line\n-----BEGIN OPENSSH PRIVATE KEY-----\n"
	for _, jsonMode := range []bool{false, true} {
		args := []string{"add", "--title", "warn and write", "--body", body, "--tag", tag, "--tag", "known"}
		if jsonMode {
			args = append(args, "--json")
		}
		status, stdout, stderr := invoke(t, env, Features{}, args...)
		if status != 0 || stdout == "" || strings.Contains(stderr, tag) || strings.Contains(stderr, "BEGIN OPENSSH") ||
			!strings.Contains(stderr, "tags line 1: slack_token") || !strings.Contains(stderr, "body line 2: pem_private_key") ||
			strings.Count(stderr, "undeclared_tag") != 1 {
			t.Fatalf("unexpected notices: %d %q %q", status, stdout, stderr)
		}
		var output writeOutput
		if jsonMode {
			oneJSON(t, stdout, &output)
		} else {
			fields := strings.Fields(stdout)
			output.ID, output.Path = fields[0], fields[2]
		}
		data := getBytes(t, filepath.Join(env.Dir, output.Path))
		h, at, _, err := item.Parse(output.ID, data)
		if err != nil || !slices.Contains(h.Tags, tag) || string(data[at:]) != body {
			t.Fatalf("the warned item was not written intact")
		}
		status, _, stderr = invoke(t, env, Features{}, "verify", output.ID, "--by", actor, "--json")
		if status != 0 || strings.Contains(stderr, actor) || !strings.Contains(stderr, "verified line 1: github_token") {
			t.Fatalf("verify notice: %d %q", status, stderr)
		}
		for _, line := range strings.Split(strings.TrimSuffix(stderr, "\n"), "\n") {
			if !json.Valid([]byte(line)) {
				t.Fatalf("non-JSON diagnostic: %q", line)
			}
		}
	}
}
