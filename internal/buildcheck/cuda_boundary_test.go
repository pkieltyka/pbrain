package buildcheck

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCUDADriverImportBoundary(t *testing.T) {
	top := root(t)
	const allowed = "internal/embed/gemma2/cuda_capability_linux.go"
	seen := false
	err := filepath.WalkDir(top, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != top && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "dist") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, decl := range parsed.Imports {
			imp, _ := strconv.Unquote(decl.Path.Value)
			if imp != "github.com/eitamring/gocudrv/cuda" && !strings.HasPrefix(imp, "github.com/eitamring/gocudrv/") {
				continue
			}
			rel, err := filepath.Rel(top, path)
			if err != nil {
				return err
			}
			if filepath.ToSlash(rel) != allowed || imp != "github.com/eitamring/gocudrv/cuda" {
				t.Errorf("unexpected direct CUDA driver import %q in %s", imp, rel)
				continue
			}
			seen = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !seen {
		t.Fatalf("missing driver capability probe in %s", allowed)
	}
}
