package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/item"
)

func TestDeletePreviewAndScope(t *testing.T) {
	env := writeEnv(t)
	older := storeItem(t, env.Dir, "", "older", item.Draft{Kind: item.Note}, "")
	target := storeItem(t, env.Dir, "", "target", item.Draft{Kind: item.Note, Replaces: []string{"older"}}, "")
	storeItem(t, env.Dir, "", "old-dangling", item.Draft{Kind: item.Note, Relates: []string{"already-absent"}}, "")
	storeItem(t, env.Dir, "personal", "private-backlink", item.Draft{Kind: item.Note, Relates: []string{"target"}}, "")
	for i := range 12 {
		storeItem(t, env.Dir, "", fmt.Sprintf("ref-%02d", i), item.Draft{Kind: item.Note, Relates: []string{"target"}}, "")
	}
	for _, personal := range []bool{false, true} {
		flags := []string{"delete", "target", "--json"}
		if personal {
			flags = append(flags, "--personal")
		}
		status, stdout, stderr := invoke(t, env, Features{}, flags...)
		var out deleteOutput
		oneJSON(t, stdout, &out)
		extra := 2
		if personal {
			extra++
		}
		if status != 1 || diagnosticCode(t, stderr, "error") != "confirm_required" || out.Deleted ||
			out.ID != "target" || out.Section != "shared" || out.Path != "notes/target.md" ||
			!slices.Equal(out.CurrentAgain, []string{"older"}) || len(out.Dangling) != 10 || out.DanglingMore != extra ||
			strings.Contains(stdout, "old-dangling") || strings.Contains(stdout, "private-backlink") != personal ||
			strings.Contains(stdout, "\"deleted\"") {
			t.Fatalf("wrong preview (%v): %d %s %s", personal, status, stdout, stderr)
		}
		mustBeMissing(t, filepath.Join(env.Dir, ".pbrain"))
		mustBeMissing(t, filepath.Join(env.Dir, "personal", ".pbrain"))
		if _, err := os.Stat(target); err != nil {
			t.Fatal(err)
		}
	}
	status, stdout, stderr := invoke(t, env, Features{}, "delete", "target")
	if status != 1 || !strings.HasPrefix(stdout, "would delete: target  shared  notes/target.md\n") ||
		!strings.Contains(stdout, "left dangling: ref-00,") || !strings.Contains(stdout, "(+2 more)") ||
		!strings.HasPrefix(stderr, "confirm_required: ") {
		t.Errorf("wrong text preview: %d %q %q", status, stdout, stderr)
	}

	// A valid replacement suppresses the stamp, but after deleting it the stamp counts.
	putBytes(t, env.Dir, "notes/older.md", bytes.ReplaceAll(getBytes(t, older), []byte("status: provisional"), []byte("status: superseded-by stamp")))
	storeItem(t, env.Dir, "", "stamp", item.Draft{Kind: item.Note}, "")
	status, stdout, stderr = invoke(t, env, Features{}, "delete", "target", "--yes", "--personal", "--json")
	var out deleteOutput
	oneJSON(t, stdout, &out)
	if status != 0 || !out.Deleted || len(out.CurrentAgain) != 0 || !strings.Contains(stderr, "git_history") {
		t.Errorf("delete did not recompute masked stamp: %d %s %s", status, stdout, stderr)
	}
	mustBeMissing(t, target)
	mustBeMissing(t, filepath.Join(env.Dir, "personal", ".pbrain"))

	storeItem(t, env.Dir, "", "ambiguous", item.Draft{Kind: item.Note}, "")
	storeItem(t, env.Dir, "personal", "ambiguous", item.Draft{Kind: item.Note}, "")
	status, stdout, stderr = invoke(t, env, Features{}, "delete", "ambiguous", "--yes", "--personal", "--json")
	if status != 1 || stdout != "" || diagnosticCode(t, stderr, "error") != "ambiguous_id" {
		t.Fatalf("ambiguous deletion: %d %q %q", status, stdout, stderr)
	}

	// Even preview refuses a protected path holding a nested repository.
	putBytes(t, env.Dir, "personal/nested/.git", nil)
	status, stdout, stderr = invoke(t, env, Features{}, "delete", "ambiguous", "--json")
	if status != 1 || stdout != "" || diagnosticCode(t, stderr, "error") != "personal_exposed" {
		t.Fatalf("preview bypassed the guard: %d %q %q", status, stdout, stderr)
	}
}

func copiedFixture(t *testing.T, env Env, withOriginal bool) (path, digest string) {
	t.Helper()
	original := []byte("bytes from outside")
	sum := sha256.Sum256(original)
	digest = hex.EncodeToString(sum[:])
	path = storeItem(t, env.Dir, "", "copy", item.Draft{
		Kind: item.Source, Origin: item.OriginFile, Source: "file.txt", SHA256: digest,
	}, "stored text")
	if withOriginal {
		putBytes(t, env.Dir, ".pbrain/originals/"+digest, original)
	}
	return path, digest
}

func TestDeletePreviewMissingOriginal(t *testing.T) {
	for _, tc := range []string{"absent", "symlink", "malformed"} {
		t.Run(tc, func(t *testing.T) {
			env := writeEnv(t)
			path, digest := copiedFixture(t, env, false)
			outside := putBytes(t, env.Dir, "outside.bin", []byte("outside original bytes"))
			switch tc {
			case "symlink":
				dir := filepath.Join(env.Dir, ".pbrain", "originals")
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(dir, digest)); err != nil {
					t.Fatal(err)
				}
			case "malformed":
				putBytes(t, env.Dir, "sources/copy.md", bytes.ReplaceAll(getBytes(t, path), []byte(digest), []byte("../outside.bin")))
			}
			status, stdout, stderr := invoke(t, env, Features{}, "delete", "copy", "--json")
			var info deleteOutput
			oneJSON(t, stdout, &info)
			if status != 1 || info.Original != "left" || diagnosticCode(t, stderr, "error") != "confirm_required" {
				t.Fatalf("preview: %d %q %q", status, stdout, stderr)
			}
			mustBeMissing(t, filepath.Join(env.Dir, ".pbrain", "lock"))
			if tc == "absent" {
				mustBeMissing(t, filepath.Join(env.Dir, ".pbrain"))
			}
			status, stdout, stderr = invoke(t, env, Features{}, "delete", "copy", "--yes", "--json")
			info = deleteOutput{}
			oneJSON(t, stdout, &info)
			if status != 0 || !info.Deleted || info.Original != "left" || !strings.Contains(stderr, "after_write") {
				t.Fatalf("confirmed: %d %q %q", status, stdout, stderr)
			}
			mustBeMissing(t, path)
			if string(getBytes(t, outside)) != "outside original bytes" {
				t.Fatal("outside original altered")
			}
		})
	}
}

func TestDeleteOriginalRefcountAndIsolation(t *testing.T) {
	for _, sameSection := range []bool{true, false} {
		t.Run(fmt.Sprint(sameSection), func(t *testing.T) {
			env := writeEnv(t)
			_, digest := copiedFixture(t, env, true)
			personal := putBytes(t, env.Dir, "personal/.pbrain/originals/"+digest, []byte("personal original"))
			if sameSection {
				storeItem(t, env.Dir, "", "other-copy", item.Draft{
					Kind: item.Source, Origin: item.OriginURL, Source: "https://example.test/items", SHA256: digest,
				}, "still refers to the original")
				storeItem(t, env.Dir, "", "replacer", item.Draft{
					Kind: item.Source, Origin: item.OriginLink, Source: "https://example.test/replacement",
					Replaces: []string{"other-copy"},
				}, "")
			}
			status, stdout, stderr := invoke(t, env, Features{}, "delete", "copy", "--yes", "--personal", "--json")
			var out deleteOutput
			oneJSON(t, stdout, &out)
			want := "removed"
			if sameSection {
				want = "kept"
			}
			if status != 0 || out.Original != want || !strings.Contains(stderr, "git_history") || strings.Contains(stderr, "after_write") {
				t.Fatalf("unexpected original state: %d %q %q", status, stdout, stderr)
			}
			path := filepath.Join(env.Dir, ".pbrain", "originals", digest)
			if sameSection {
				getBytes(t, path)
			} else {
				mustBeMissing(t, path)
			}
			if string(getBytes(t, personal)) != "personal original" {
				t.Fatal("shared deletion touched the personal original")
			}
			mustBeMissing(t, filepath.Join(env.Dir, "personal", ".pbrain", "lock"))
		})
	}
}
