package bge

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"testing"
)

func TestRuntimeEnvAndOutput(t *testing.T) {
	const runtimeModule = "github.com/pkieltyka/aikit"
	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", "-json", ".").Output()
	if err != nil {
		t.Fatal(err)
	}
	list := json.NewDecoder(bytes.NewReader(out))
	checked := 0
	for {
		var pkg struct {
			Dir, ImportPath string
			GoFiles         []string
			Module          *struct{ Path string }
		}
		if err := list.Decode(&pkg); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if pkg.Module == nil || pkg.Module.Path != runtimeModule {
			continue
		}
		for _, name := range pkg.GoFiles {
			auditRuntimeFile(t, filepath.Join(pkg.Dir, name), pkg.ImportPath+"/"+name)
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("aikit source was not audited")
	}
}

func auditRuntimeFile(t *testing.T, filename, label string) {
	t.Helper()
	positions := token.NewFileSet()
	f, err := parser.ParseFile(positions, filename, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := map[string]map[string]bool{
		"os":      {"Getenv": true, "LookupEnv": true, "Environ": true, "Stdout": true, "Stderr": true},
		"syscall": {"Getenv": true},
		"fmt":     {"Print": true, "Println": true, "Printf": true},
		"log": {
			"Default": true, "Fatal": true, "Fatalf": true, "Fatalln": true, "Output": true,
			"Panic": true, "Panicf": true, "Panicln": true, "Print": true, "Printf": true, "Println": true,
			"SetFlags": true, "SetOutput": true, "SetPrefix": true, "Writer": true,
		},
	}
	imports := make(map[string]map[string]bool)
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if names := forbidden[p]; names != nil {
			alias := path.Base(p)
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			if alias == "." {
				t.Errorf("%s uses a dot import of %s", label, p)
			}
			imports[alias] = names
		}
	}
	ast.Inspect(f, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := n.X.(*ast.Ident); ok && imports[pkg.Name][n.Sel.Name] {
				t.Errorf("%s:%d uses %s.%s", label, positions.Position(n.Pos()).Line, pkg.Name, n.Sel.Name)
			}
		case *ast.CallExpr:
			if id, ok := n.Fun.(*ast.Ident); ok && (id.Name == "print" || id.Name == "println") {
				t.Errorf("%s:%d calls %s", label, positions.Position(n.Pos()).Line, id.Name)
			}
		}
		return true
	})
}
