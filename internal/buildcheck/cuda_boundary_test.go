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
	const driver = "github.com/eitamring/gocudrv"
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
			if imp != driver && !strings.HasPrefix(imp, driver+"/") {
				continue
			}
			rel, err := filepath.Rel(top, path)
			if err != nil {
				return err
			}
			t.Errorf("direct CUDA driver import %q in %s; use goinfer's inventory", imp, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
