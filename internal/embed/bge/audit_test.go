package bge

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

const (
	aikitModule   = "github.com/pkieltyka/aikit"
	goinferModule = "github.com/pkieltyka/goinfer"
	cudaModule    = goinferModule + "/backend/cuda"
	vulkanModule  = goinferModule + "/backend/vulkan"
	decoderPkg    = goinferModule + "/decoder"
	auditedPin    = "v0.6.4"
)

type runtimeSite struct {
	file, scope, symbol, argument string
}

func TestRuntimeEnvAndOutput(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	release, err := os.ReadFile(filepath.Join(root, ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Builds []struct {
			ID, Main           string
			Goos, Goarch, Tags []string
		}
	}
	if err := yaml.Unmarshal(release, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Builds) == 0 {
		t.Fatal("no release builds to audit")
	}
	for _, build := range config.Builds {
		if build.Main == "" || len(build.Goos) == 0 || len(build.Goarch) == 0 {
			t.Fatalf("release build %q has no explicit main or targets", build.ID)
		}
		for _, osName := range build.Goos {
			for _, arch := range build.Goarch {
				t.Run(build.ID+"/"+osName+"/"+arch, func(t *testing.T) {
					auditRuntimeTarget(t, root, build.Main, osName, arch, build.Tags)
				})
			}
		}
	}
}

func auditRuntimeTarget(t *testing.T, root, main, osName, arch string, tags []string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "go", "list", "-p", "2", "-deps",
		"-json=Dir,ImportPath,GoFiles,Module", "-tags", strings.Join(tags, ","), main)
	cmd.Dir = root
	cmd.Env = append(cmd.Environ(), "GOOS="+osName, "GOARCH="+arch, "CGO_ENABLED=0", "GOFLAGS=", "GOWORK=off")
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("go list: %v\n%s", err, exit.Stderr)
		}
		t.Fatal(err)
	}
	list := json.NewDecoder(bytes.NewReader(out))
	checked := map[string]int{}
	seen := map[runtimeSite]bool{}
	knownStartup := slices.Contains(tags, "cuda") && len(knownStartupViolations) > 0
	for {
		var pkg struct {
			Dir, ImportPath string
			GoFiles         []string
			Module          *struct{ Path, Version string }
		}
		if err := list.Decode(&pkg); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if pkg.Module == nil {
			continue
		}
		module := pkg.Module.Path
		if module != aikitModule && module != goinferModule && !strings.HasPrefix(module, goinferModule+"/backend/") {
			continue
		}
		for _, name := range pkg.GoFiles {
			auditRuntimeFile(t, filepath.Join(pkg.Dir, name), pkg.ImportPath+"/"+name, func(line int, site runtimeSite) {
				pinned := pkg.Module.Version == auditedPin
				reviewed := dormantRuntimeSites[site] != "" || allowedRuntimeOutput[site] != ""
				startup := knownStartup && isKnownStartupSite(site)
				if pinned && (reviewed || startup) && !seen[site] {
					seen[site] = true // each recorded source site is allowed one occurrence per build
					return
				}
				t.Errorf("%s:%d (%s) uses %s %s", site.file, line, site.scope, site.symbol, site.argument)
			})
			checked[module]++
		}
	}
	if checked[aikitModule] == 0 || checked[goinferModule] == 0 ||
		(slices.Contains(tags, "cuda") != (checked[cudaModule] > 0)) ||
		(slices.Contains(tags, "vulkan") != (checked[vulkanModule] > 0)) {
		t.Fatalf("incomplete runtime graph for tags %v: %v", tags, checked)
	}
	t.Logf("audited %v with release tags %v", checked, tags)
	if knownStartup {
		for site := range knownStartupViolations {
			if !seen[site] {
				t.Errorf("known startup violation no longer matched: %+v (goinfer #70)", site)
			}
		}
		if !seen[startupHelperRead] {
			t.Error("known startup helper no longer reads the environment (goinfer #70)")
		}
		t.Logf("tracking %d known startup reads from goinfer #70", len(knownStartupViolations))
	}
}

func auditRuntimeFile(t *testing.T, filename, label string, report func(int, runtimeSite)) {
	t.Helper()
	positions := token.NewFileSet()
	f, err := parser.ParseFile(positions, filename, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := map[string]map[string]bool{
		"os":                    {"Getenv": true, "LookupEnv": true, "Environ": true, "ExpandEnv": true, "Stdout": true, "Stderr": true},
		"syscall":               {"Getenv": true, "Environ": true},
		"golang.org/x/sys/unix": {"Getenv": true, "Environ": true},
		"fmt":                   {"Print": true, "Println": true, "Printf": true},
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
	visit := func(n ast.Expr, stack []ast.Node, symbol, argument string) {
		report(positions.Position(n.Pos()).Line, runtimeSite{label, runtimeScope(stack), symbol, argument})
	}
	ast.PreorderStack(f, nil, func(node ast.Node, stack []ast.Node) bool {
		switch n := node.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := n.X.(*ast.Ident); ok {
				p := imports[pkg.Name]
				if forbidden[p][n.Sel.Name] {
					var argument string
					if n.Sel.Name == "Getenv" || n.Sel.Name == "LookupEnv" {
						argument = runtimeArgument(n, stack)
					}
					visit(n, stack, p+"."+n.Sel.Name, argument)
				}
			}
		case *ast.CallExpr:
			if id, ok := ast.Unparen(n.Fun).(*ast.Ident); ok && (id.Name == "print" || id.Name == "println") {
				visit(id, stack, "builtin."+id.Name, "")
			}
		case *ast.Ident:
			// Count both callers of the one known indirect startup read.
			// An additional caller or an escaped reference needs review too.
			if len(knownStartupViolations) > 0 && path.Dir(label) == decoderPkg && n.Name == "envBoolDefault" {
				if decl, ok := stack[len(stack)-1].(*ast.FuncDecl); !ok || decl.Name != n {
					visit(n, stack, "decoder.envBoolDefault", runtimeArgument(n, stack))
				}
			}
		}
		return true
	})
}

func runtimeArgument(fn ast.Expr, stack []ast.Node) string {
	for i := len(stack) - 1; i >= 0; i-- {
		switch n := stack[i].(type) {
		case *ast.ParenExpr:
			continue
		case *ast.CallExpr:
			if ast.Unparen(n.Fun) == fn && len(n.Args) > 0 {
				return expressionText(n.Args[0])
			}
		}
		break
	}
	return "<indirect>"
}

func runtimeScope(stack []ast.Node) string {
	scope := "<package>"
	suffix := ""
	for i := len(stack) - 1; i >= 0; i-- {
		switch n := stack[i].(type) {
		case *ast.FuncLit:
			suffix = " (closure)"
		case *ast.FuncDecl:
			if n.Recv != nil {
				return "(" + expressionText(n.Recv.List[0].Type) + ")." + n.Name.Name + suffix
			}
			return n.Name.Name + suffix
		case *ast.ValueSpec:
			scope = n.Names[0].Name
		}
	}
	return scope + suffix
}

func expressionText(expr ast.Expr) string {
	var out bytes.Buffer
	// All expressions here came from a successfully parsed Go source file.
	if err := format.Node(&out, token.NewFileSet(), expr); err != nil {
		panic(err)
	}
	return out.String()
}
