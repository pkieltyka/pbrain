package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/cli"
	"github.com/pkieltyka/pbrain/internal/sys"
)

func TestReleaseModelSelection(t *testing.T) {
	t.Setenv("PBRAIN_ALLOW_GEMMA2", "")
	models := releaseModels()
	if _, ok := models.Lookup(brain.DefaultModel); !ok {
		t.Fatalf("release registry lacks the default %q", brain.DefaultModel)
	}
	if names := models.Names(); !slices.Equal(names, []string{"bge-small-en-v1.5", "embeddinggemma-2", "embeddinggemma-2-q8-0", "embeddinggemma-2-ud-q6-k-xl"}) {
		t.Fatalf("unexpected release models: %v", names)
	}
	root := t.TempDir()
	cache := filepath.Join(t.TempDir(), "cache")
	const absent = "the model is not on this machine: run pbrain sync, or set model_path"
	for _, tc := range []struct{ config, status, notice string }{
		{"", "bge-small-en-v1.5, not on this machine", absent},
		{`semantic.model = "embeddinggemma-2"`, "embeddinggemma-2, not on this machine", absent},
		{`semantic.model = "embeddinggemma-2-q8-0"`, "embeddinggemma-2-q8-0, not on this machine", absent},
		{`semantic.model = "embeddinggemma-2-ud-q6-k-xl"`, "embeddinggemma-2-ud-q6-k-xl, not on this machine", absent},
		{`semantic.model = "bge-small-en-v1.5"`, "bge-small-en-v1.5, not on this machine", absent},
		{`semantic.model = "none"`, "none", `semantic search is off: pbrain.toml sets model = "none"`},
		{`semantic.model = "None"`, "None, not supported by this pbrain", "this pbrain does not support the model pbrain.toml names: pbrain sync lists the supported names"},
	} {
		if err := os.WriteFile(filepath.Join(root, "pbrain.toml"), []byte(tc.config), 0o600); err != nil {
			t.Fatal(err)
		}
		var out, diag bytes.Buffer
		env := cli.Env{
			Args: []string{"--brain", root, "sync", "--status"}, Dir: root,
			Stdout: &out, Stderr: &diag, Files: sys.Native(), Models: models,
			Getenv: func(key string) string {
				if key == "PBRAIN_CACHE" {
					return cache
				}
				return ""
			},
		}
		code := cli.Run(t.Context(), env)
		if code != 0 || diag.Len() != 0 || out.String() != "model: "+tc.status+"\nshared: 0 of 0 items indexed\n" {
			t.Errorf("settings %q: exit %d, stdout %q, stderr %q", tc.config, code, out.String(), diag.String())
		}
		out.Reset()
		diag.Reset()
		env.Args = []string{"--brain", root, "search", "absent", "--semantic"}
		if code := cli.Run(t.Context(), env); code != 0 || diag.String() != "semantic_not_ready: "+tc.notice+"\n" {
			t.Errorf("semantic search with settings %q: exit %d, stdout %q, stderr %q", tc.config, code, out.String(), diag.String())
		}
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("status or search touched the cache: %v", err)
	}
}

func TestReleaseQ8OfflineRefusal(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(t.TempDir(), "cache")
	model := filepath.Join(root, "model")
	for _, dir := range []string{model, filepath.Join(root, "notes")} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range map[string]string{
		"pbrain.toml":                         "semantic.model = \"embeddinggemma-2-q8-0\"",
		"personal.toml":                       "model_path = \"model\"",
		"notes/20261101-000000-alpha-a1a1.md": "---\nkind: note\ntitle: Alpha\ncreated: 2026-11-01T00:00:00Z\nauthor: agent:test\n---\n\nOffline Q8 test.\n",
		"model/embeddinggemma-2-Q8_0.gguf":    "wrong weights",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, diag bytes.Buffer
	env := cli.Env{
		Args: []string{"--brain", root, "sync"}, Dir: root,
		Stdout: &out, Stderr: &diag, Files: sys.Native(), Models: releaseModels(),
		Getenv: func(key string) string {
			if key == "PBRAIN_CACHE" {
				return cache
			}
			return ""
		},
	}
	if code := cli.Run(t.Context(), env); code != 1 || out.Len() != 0 ||
		!strings.HasPrefix(diag.String(), "invalid_config: the model at model_path is incomplete or is not embeddinggemma-2-q8-0") {
		t.Fatalf("offline Q8 sync = %d, out %q, diag %q", code, out.String(), diag.String())
	}
}
