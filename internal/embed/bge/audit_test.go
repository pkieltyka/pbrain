package bge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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

type runtimePin struct{ module, version string }

type runtimeAllowance struct {
	targets runtimeTarget
	count   int
	reason  string
}

type runtimeModule struct {
	Path, Version string
	Main          bool
}

type runtimePackage struct {
	Dir, ImportPath string
	GoFiles         []string
	Module          *runtimeModule
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
	checked, findings := reviewRuntimeGraph(t, bytes.NewReader(out), auditTarget(osName, arch, tags))
	for _, finding := range findings {
		t.Error(finding)
	}
	if checked[aikitModule] == 0 || checked[goinferModule] == 0 ||
		(slices.Contains(tags, "cuda") != (checked[cudaModule] > 0)) ||
		(slices.Contains(tags, "vulkan") != (checked[vulkanModule] > 0)) {
		t.Fatalf("incomplete runtime graph for tags %v: %v", tags, checked)
	}
	t.Logf("audited %v with release tags %v", checked, tags)
}

func reviewRuntimeGraph(t *testing.T, graph io.Reader, target runtimeTarget) (map[string]int, []string) {
	t.Helper()
	var findings []string
	seen := map[runtimeSite]int{}
	knownStartup := target&cudaReleases != 0 && len(knownStartupViolations) > 0
	checked := scanRuntimeGraph(t, graph, func(pin runtimePin, line int, site runtimeSite) {
		limit := 0
		if rule := reviewedDependencySites[pin][site]; rule.targets&target != 0 {
			limit = rule.count
		}
		if pin.version == auditedPin && (pin.module == goinferModule || pin.module == cudaModule) &&
			(dormantRuntimeSites[site] != "" || allowedRuntimeOutput[site] != "" ||
				(knownStartup && isKnownStartupSite(site))) {
			limit = 1
		}
		if seen[site] < limit {
			seen[site]++
			return
		}
		use := strings.TrimSpace(site.symbol + " " + site.argument)
		findings = append(findings, fmt.Sprintf("%s:%d (%s) uses %s", site.file, line, site.scope, use))
	})
	if knownStartup {
		for site := range knownStartupViolations {
			if seen[site] == 0 {
				findings = append(findings, fmt.Sprintf("known startup violation no longer matched: %+v (goinfer #70)", site))
			}
		}
		if seen[startupHelperRead] == 0 {
			findings = append(findings, "known startup helper no longer reads the environment (goinfer #70)")
		}
		t.Logf("tracking %d known startup reads from goinfer #70", len(knownStartupViolations))
	}
	return checked, findings
}

func scanRuntimeGraph(t *testing.T, graph io.Reader, report func(runtimePin, int, runtimeSite)) map[string]int {
	t.Helper()
	list := json.NewDecoder(graph)
	checked := map[string]int{}
	for {
		var pkg runtimePackage
		if err := list.Decode(&pkg); errors.Is(err, io.EOF) {
			return checked
		} else if err != nil {
			t.Fatal(err)
		}
		if pkg.Module == nil || pkg.Module.Main {
			continue
		}
		pin := runtimePin{pkg.Module.Path, pkg.Module.Version}
		for _, name := range pkg.GoFiles {
			auditRuntimeFile(t, filepath.Join(pkg.Dir, name), pkg.ImportPath+"/"+name, func(line int, site runtimeSite) {
				report(pin, line, site)
			})
			checked[pin.module]++
		}
	}
}

func TestRuntimeAuditIncludesNewDependency(t *testing.T) {
	const module, version = "example.com/new-runtime-dependency", "v1.0.0"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "env.go"), []byte(`package probe
import "os"
var setting = os.Getenv("UNREVIEWED_STARTUP")
var temp = os.TempDir()
var expanded = os.Expand("$HOME", os.Getenv)
var mapped = os.Expand("$HOME", func(key string) string { return key })
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var graph bytes.Buffer
	enc := json.NewEncoder(&graph)
	for _, pkg := range []runtimePackage{
		{ImportPath: "os", GoFiles: []string{"not-present.go"}},
		{ImportPath: "pbrain", Module: &runtimeModule{Path: "pbrain", Main: true}, GoFiles: []string{"not-present.go"}},
		{Dir: dir, ImportPath: module, GoFiles: []string{"env.go"}, Module: &runtimeModule{Path: module, Version: version}},
	} {
		if err := enc.Encode(pkg); err != nil {
			t.Fatal(err)
		}
	}
	checked, findings := reviewRuntimeGraph(t, &graph, linuxAMD64CPU)
	want := []string{
		module + `/env.go:3 (setting) uses os.Getenv "UNREVIEWED_STARTUP"`,
		module + "/env.go:4 (temp) uses os.TempDir",
		module + "/env.go:5 (expanded) uses os.Getenv <indirect>",
	}
	if len(checked) != 1 || checked[module] != 1 || !slices.Equal(findings, want) {
		t.Fatalf("scanned %v, found %q, want %q from the new dependency", checked, findings, want)
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
		"os": {
			"Getenv": true, "LookupEnv": true, "Environ": true, "ExpandEnv": true,
			"TempDir": true, "UserHomeDir": true, "UserConfigDir": true, "UserCacheDir": true, "Getwd": true,
			"CreateTemp": true, "MkdirTemp": true, "StartProcess": true,
			"Stdout": true, "Stderr": true,
		},
		// Account fallback can read USER/HOME. Command constructors and
		// LookPath can read PATH; Cmd and StartProcess can inherit Env.
		"os/user": {"Current": true, "Lookup": true, "LookupId": true},
		"os/exec": {"Command": true, "CommandContext": true, "LookPath": true, "Cmd": true},
		"net/http": {
			"ProxyFromEnvironment": true, "DefaultTransport": true, "DefaultClient": true,
			"Get": true, "Head": true, "Post": true, "PostForm": true,
		},
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
