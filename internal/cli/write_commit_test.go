package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/audience"
	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/item"
)

func TestCommitAndCleanupOutcomes(t *testing.T) {
	for _, stage := range []string{"item unlink", "original unlink", "original sync", "item sync"} {
		t.Run(stage, func(t *testing.T) {
			env := writeEnv(t)
			path, digest := copiedFixture(t, env, true)
			original := filepath.Join(env.Dir, ".pbrain", "originals", digest)
			faults := &writeFaults{Files: env.Files}
			switch stage {
			case "item unlink", "original unlink":
				faults.remove = func(dir, name string) error {
					if stage == "item unlink" && name == filepath.Base(path) ||
						stage == "original unlink" && name == digest {
						return injectedFailure
					}
					return nil
				}
			case "item sync", "original sync":
				faults.sync = func(dir string) error {
					if stage == "item sync" && filepath.Base(dir) == "sources" ||
						stage == "original sync" && filepath.Base(dir) == "originals" {
						return injectedFailure
					}
					return nil
				}
			}
			env.Files = faults
			status, stdout, stderr := invoke(t, env, Features{}, "delete", "copy", "--yes", "--json")
			if stage == "item unlink" {
				if status != 1 || stdout != "" || diagnosticCode(t, stderr, "error") != "io" {
					t.Fatalf("pre-commit failure: %d %q %q", status, stdout, stderr)
				}
				getBytes(t, path)
				getBytes(t, original)
				return
			}
			var out deleteOutput
			oneJSON(t, stdout, &out)
			disposition := "removed"
			if stage == "original unlink" {
				disposition = "left"
				getBytes(t, original)
			} else {
				mustBeMissing(t, original)
			}
			if status != 0 || !out.Deleted || !strings.Contains(stderr, "after_write") || out.Original != disposition {
				t.Fatalf("post-commit outcome: %d %q %q", status, stdout, stderr)
			}
			mustBeMissing(t, path)
		})
	}

	t.Run("verify rename failure", func(t *testing.T) {
		env := writeEnv(t)
		it := storeItem(t, env.Dir, "", "target", item.Draft{Kind: item.Learning}, "original text")
		before := getBytes(t, it)
		env.Files = &writeFaults{Files: env.Files, replace: func(_, _ string) error { return injectedFailure }}
		status, stdout, stderr := invoke(t, env, Features{}, "verify", "target", "--json")
		if status != 1 || stdout != "" || diagnosticCode(t, stderr, "error") != "io" ||
			!bytes.Equal(getBytes(t, it), before) {
			t.Fatalf("failed verify changed original: %d %q %q", status, stdout, stderr)
		}
	})

	t.Run("original persistence failure", func(t *testing.T) {
		env := writeEnv(t)
		env.Files = &writeFaults{Files: env.Files, beforeLink: func(dir, _ string) error {
			if filepath.Base(dir) == "originals" {
				return injectedFailure
			}
			return nil
		}}
		ctx := context.Background()
		b, err := brain.Open(ctx, brain.Env{Dir: env.Dir, Files: env.Files, Getenv: env.Getenv, Now: env.Now}, brain.OpenOptions{})
		if err != nil {
			t.Fatal(err)
		}
		defer b.Close()
		wc := writeContext{Brain: b, Rand: env.Rand, Now: writeTime}
		route, err := routeWrite(wc, audience.Request{Source: "file"})
		if err != nil {
			t.Fatal(err)
		}
		raw := []byte("original bytes")
		sum := sha256.Sum256(raw)
		digest := hex.EncodeToString(sum[:])
		_, err = publishNew(ctx, wc, newItemInput{route: route, body: raw, original: &copiedOriginal{sum, raw}, draft: item.Draft{
			Kind: item.Source, Title: "file", Source: "file", Author: "agent:test", Created: writeTime, Origin: item.OriginFile, SHA256: digest,
		}})
		requireCode(t, err, "io")
		mustBeMissing(t, filepath.Join(env.Dir, "sources"))
		mustBeMissing(t, filepath.Join(env.Dir, ".pbrain", "originals", digest))
	})
}

func TestAddRetriesExclusivePublication(t *testing.T) {
	env := writeEnv(t)
	env.Rand = bytes.NewReader([]byte{0, 0, 0, 0, 0, 0, 0, 1})
	var names []string
	env.Files = &writeFaults{Files: env.Files, beforeLink: func(dir, name string) error {
		if filepath.Base(dir) != "notes" {
			return nil
		}
		names = append(names, name)
		if len(names) == 1 {
			return fs.ErrExist
		}
		return nil
	}}
	result, _, body := describeWrite(t, env, "add", "--title", "collision", "--body", "same body")
	if len(names) != 2 || names[0] != "20261121-123456-collision-aaaa.md" ||
		names[1] != "20261121-123456-collision-aaab.md" ||
		result.ID != "20261121-123456-collision-aaab" || body != "same body" {
		t.Fatalf("exclusive retry: %+v in %v", result, names)
	}
	if matches, err := filepath.Glob(filepath.Join(env.Dir, "notes", "*.md")); err != nil || len(matches) != 1 {
		t.Fatalf("failed publish leaked another item: %v %v", matches, err)
	}
}

func TestWriteMalformedAudienceRuleBeforeInput(t *testing.T) {
	env := writeEnv(t)
	putBytes(t, env.Dir, "personal.toml", []byte("[[audience]]\nmatch='github.com'\nto='shared'\n"))
	env.Stdin = readerFunc(func([]byte) (int, error) {
		return 0, errors.New("input must not be accessed")
	})
	status, stdout, stderr := invoke(t, env, Features{}, "add", "--shared", "--title", "refuse invalid settings", "--json")
	if status != 1 || stdout != "" || diagnosticCode(t, stderr, "error") != "invalid_config" {
		t.Fatalf("invalid rule validated too late: %d %q %q", status, stdout, stderr)
	}
	mustBeMissing(t, filepath.Join(env.Dir, ".pbrain"))
}
