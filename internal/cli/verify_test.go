package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/item"
)

func TestVerifyExistingTargets(t *testing.T) {
	env := writeEnv(t)
	before := []byte("---\r\nkind: learning\r\ncreated: 2026-11-21T12:34:56Z\r\nauthor: person\r\n" +
		"status: provisional # keep\r\nverified: # old verifier\r\nunknown: 'untouched'\r\n---\r\n\r\nexact\r\n")
	path := putBytes(t, env.Dir, "learnings/example.md", before)
	original := storeItem(t, env.Dir, "personal", "private", item.Draft{Kind: item.Observation}, "Personal data.")
	savedPersonal := getBytes(t, original)
	output, h, body := describeWrite(t, env, "verify", "--personal", "EXAMPLE", "--by", "human:reviewer")
	want := bytes.ReplaceAll(before, []byte("status: provisional # keep\r\nverified: # old verifier\r\n"),
		[]byte("status: verified # keep\r\nverified: human:reviewer # old verifier\r\n"))
	if h.Status != item.Verified || h.Verified != "human:reviewer" || body != "exact\r\n" ||
		output.Section != "shared" || !bytes.Equal(getBytes(t, path), want) ||
		!bytes.Equal(getBytes(t, original), savedPersonal) {
		t.Fatal("verify rewrote unrelated bytes or the wrong section")
	}
	mustBeMissing(t, filepath.Join(env.Dir, "personal", ".pbrain"))
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	describeWrite(t, env, "verify", "example", "--by", "human:second")
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(info, after) || !after.ModTime().Equal(info.ModTime()) ||
		!bytes.Equal(getBytes(t, path), want) {
		t.Error("verifying an already verified item replaced it")
	}
	storeItem(t, env.Dir, "", "source", item.Draft{Kind: item.Source, Source: "manual"}, "")
	storeItem(t, env.Dir, "", "old", item.Draft{Kind: item.Note}, "")
	storeItem(t, env.Dir, "", "new", item.Draft{Kind: item.Note, Replaces: []string{"old"}}, "")
	unsupported := putBytes(t, env.Dir, "notes/unsupported.md", []byte("---\nstatus: |\n  provisional\n---\n"))
	saved := getBytes(t, unsupported)
	for _, tc := range []struct{ id, code string }{
		{"source", "invalid"}, {"old", "invalid"}, {"unsupported", "unsupported_edit"}, {"absent", "not_found"},
		{"private", "not_found"},
	} {
		status, stdout, stderr := invoke(t, env, Features{}, "verify", tc.id, "--json")
		for _, line := range strings.Split(strings.TrimSuffix(stderr, "\n"), "\n") {
			if strings.Contains(line, `"error":`) && diagnosticCode(t, line+"\n", "error") != tc.code {
				t.Errorf("wrong error for %s: %s", tc.id, line)
			}
		}
		if status != 1 || stdout != "" || !strings.Contains(stderr, `"code":"`+tc.code+`"`) {
			t.Errorf("rejected verify %s: %d %q %q", tc.id, status, stdout, stderr)
		}
	}
	if !bytes.Equal(getBytes(t, unsupported), saved) {
		t.Fatal("unsupported verify altered its file")
	}
}

func TestVerifyKeepsFirstVersion(t *testing.T) {
	for _, when := range []string{"before lock", "inside replace"} {
		t.Run(when, func(t *testing.T) {
			env := writeEnv(t)
			path := storeItem(t, env.Dir, "", "target", item.Draft{Kind: item.Note}, "first")
			original := getBytes(t, path)
			changed := bytes.ReplaceAll(original, []byte("first"), []byte("later"))
			touched := false
			change := func() error {
				touched = true
				if when == "before lock" {
					// A concurrently verified file must also conflict before the shortcut.
					changed = bytes.ReplaceAll(changed, []byte("status: provisional"), []byte("status: verified"))
				}
				if err := os.WriteFile(path, changed, 0o600); err != nil {
					return err
				}
				return os.Chtimes(path, time.Now(), time.Now())
			}
			files := &writeFaults{Files: env.Files}
			if when == "before lock" {
				files.beforeLock = change
			} else {
				files.beforeTemp = func(dir string) error {
					if filepath.Base(dir) == "notes" {
						return change()
					}
					return nil
				}
			}
			env.Files = files
			status, stdout, stderr := invoke(t, env, Features{}, "verify", "--json", "target")
			if status != 1 || stdout != "" || diagnosticCode(t, stderr, "error") != "conflict" ||
				!touched || !bytes.Equal(getBytes(t, path), changed) {
				t.Errorf("stale verify succeeded or changed the file: %d %q %q", status, stdout, stderr)
			}
		})
	}
}
