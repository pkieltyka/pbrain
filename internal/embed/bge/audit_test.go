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
	"strings"
	"testing"
)

// Any approved exception must identify one import-path-qualified source file and one
// fully qualified symbol, with its reason. The same table applies to all targets.
type runtimeException struct{ file, symbol string }

var runtimeExceptions = map[runtimeException]string{}

func TestRuntimeEnvAndOutput(t *testing.T) {
	for _, target := range []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"} {
		t.Run(target, func(t *testing.T) {
			osName, arch, _ := strings.Cut(target, "/")
			auditRuntimeTarget(t, osName, arch)
		})
	}
}

func auditRuntimeTarget(t *testing.T, osName, arch string) {
	t.Helper()
	const aikit = "github.com/pkieltyka/aikit"
	const goinfer = "github.com/pkieltyka/goinfer"
	cmd := exec.CommandContext(t.Context(), "go", "list", "-p", "2", "-deps", "-json=Dir,ImportPath,GoFiles,Module", ".", "../gemma2")
	cmd.Env = append(cmd.Environ(), "GOOS="+osName, "GOARCH="+arch, "CGO_ENABLED=0")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	list := json.NewDecoder(bytes.NewReader(out))
	checked := map[string]int{}
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
		if pkg.Module == nil || (pkg.Module.Path != aikit && pkg.Module.Path != goinfer) {
			continue
		}
		for _, name := range pkg.GoFiles {
			auditRuntimeFile(t, filepath.Join(pkg.Dir, name), pkg.ImportPath+"/"+name)
			checked[pkg.Module.Path]++
		}
	}
	if checked[aikit] == 0 || checked[goinfer] == 0 {
		t.Fatalf("incomplete runtime audit: %v", checked)
	}
	t.Logf("audited %d aikit files and %d goinfer files", checked[aikit], checked[goinfer])
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
	imports := make(map[string]string)
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
			imports[alias] = p
		}
	}
	report := func(pos token.Pos, symbol string) {
		if runtimeExceptions[runtimeException{label, symbol}] == "" {
			t.Errorf("%s:%d uses %s", label, positions.Position(pos).Line, symbol)
		}
	}
	ast.Inspect(f, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := n.X.(*ast.Ident); ok {
				p := imports[pkg.Name]
				if forbidden[p][n.Sel.Name] {
					report(n.Pos(), p+"."+n.Sel.Name)
				}
			}
		case *ast.CallExpr:
			if id, ok := n.Fun.(*ast.Ident); ok && (id.Name == "print" || id.Name == "println") {
				report(n.Pos(), "builtin."+id.Name)
			}
		}
		return true
	})
}
