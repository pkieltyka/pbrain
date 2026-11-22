package buildcheck

import (
	"go/build"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

const module = "github.com/pkieltyka/pbrain"

// root is the module's root folder, two levels above this package.
func root(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// directives returns the replace and exclude directives in a go.mod text, in either form:
// "replace a => b" on one line, or a "replace (" block with one directive per line.
func directives(gomod string) []string {
	var found []string
	block := ""
	for raw := range strings.SplitSeq(gomod, "\n") {
		line := strings.TrimSpace(raw)
		if i := strings.Index(line, "//"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		if block != "" {
			if line == ")" {
				block = ""
				continue
			}
			if block == "replace" || block == "exclude" {
				found = append(found, block+" "+strings.Join(strings.Fields(line), " "))
			}
			continue
		}
		// Go's lexer splits a directive on any white space, and "(" is a token of its own.
		fields := strings.Fields(strings.Replace(line, "(", " ( ", 1))
		word := fields[0]
		if len(fields) == 2 && fields[1] == "(" {
			block = word
			continue
		}
		if word == "replace" || word == "exclude" {
			found = append(found, word+" "+strings.Join(fields[1:], " "))
		}
	}
	return found
}

// TestGoModHasNoReplaceOrExclude keeps go install at a version working (architecture 15): the
// Go tool refuses it for a module whose go.mod holds either directive.
func TestGoModHasNoReplaceOrExclude(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(root(t), "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if found := directives(string(data)); len(found) > 0 {
		t.Fatalf("go.mod must hold no replace or exclude directive; found %q", found)
	}
}

// The check itself must be able to fail.
func TestDirectivesFindsBothForms(t *testing.T) {
	gomod := "module x\n\ngo 1.27.0\n\nrequire a v1.0.0 // replace in a comment is not a directive\n" +
		"replace a => ../a\nexclude b v1.2.0\nreplace (\n\tc => ../c\n)\nexclude (\n\td v0.1.0\n)\n"
	got := directives(gomod)
	want := []string{"replace a => ../a", "exclude b v1.2.0", "replace c => ../c", "exclude d v0.1.0"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("directives = %q, want %q", got, want)
	}
	// Tabs separate a directive's words as well as spaces do, and a block may open with "replace(".
	gomod = "module x\n\nreplace\ta\t=>\t../a\nexclude\tb v1.2.0\nreplace\t(\n\tc => ../c\n)\nexclude(\n\td\tv0.1.0\n)\n"
	got = directives(gomod)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("directives = %q, want %q", got, want)
	}
	if got := directives("module x\n\ngo 1.27.0\n\nrequire (\n\tgithub.com/a/b v1.0.0\n)\n"); len(got) != 0 {
		t.Fatalf("a go.mod without directives gave %q", got)
	}
}

// allowed is the import table of architecture section 3: for each package, the pbrain packages it
// may import. cli may import all of them. cmd/pbrain also wires native process inputs.
var allowed = map[string][]string{
	"internal/cli":                nil,                                                          // may import every package
	"cmd/pbrain":                  {"cli", "gitx", "sys", "embed", "embed/bge", "embed/gemma2"}, // process adapter in cli.md §1; the release registry (architecture 3: model sub-packages are imported where the registry is built)
	"internal/check":              {"graph", "item", "secretscan"},
	"internal/semantic":           {"search", "embed", "outline", "item", "gitx", "sys", "errs"},
	"internal/semantic/calibrate": {"brain", "cli", "embed", "embed/bge", "embed/gemma2", "graph", "item", "outline", "query", "render", "search", "semantic", "store", "sys"}, // developer tool in semantic.md
	"internal/embed":              {"sys", "errs"},
	"internal/embed/bge":          {"embed", "sys", "errs"},
	"internal/embed/gemma2":       {"embed"},
	"internal/embed/embedtest":    {"embed", "errs"}, // semantic.md "Test plan": the test registry
	"internal/importer":           {"item", "errs", "importer/fetch"},
	"internal/importer/fetch":     {"errs"},
	"internal/search":             {"graph", "item", "query", "outline", "errs"},
	"internal/render":             {"errs"},
	"internal/store":              {"brain", "item", "gitx", "sys", "errs"},    // errs: store.md "Dependencies"
	"internal/brain":              {"audience", "item", "gitx", "sys", "errs"}, // errs: brain.md "Dependencies"
	"internal/graph":              {"item", "errs"},                            // graph.md §4 allows errs for CheckNewReplaces
	"internal/audience":           {"errs"},
	"internal/query":              {"item", "errs"},
	"internal/item":               {"outline", "errs"},
	"internal/outline":            nil,
	"internal/gitx":               {"errs"},
	"internal/secretscan":         nil,
	"internal/skill":              nil,
	"internal/errs":               nil,
	"internal/sys":                nil,
	"internal/buildcheck":         nil,
}

// TestPackageImports holds every package of the module to the table above, and every package
// to being in the table, so that a new package is placed in architecture 3 before it is built.
func TestPackageImports(t *testing.T) {
	top := root(t)
	var dirs []string
	err := filepath.WalkDir(top, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if p != top && (strings.HasPrefix(name, ".") || name == "testdata" || name == "specs") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			dirs = append(dirs, filepath.Dir(p))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, dir := range dirs {
		rel, _ := filepath.Rel(top, dir)
		rel = filepath.ToSlash(rel)
		if seen[rel] {
			continue
		}
		seen[rel] = true
		want, ok := allowed[rel]
		if !ok {
			t.Errorf("%s is not in architecture 3's table (add it to allowed in this test with its permitted imports)", rel)
			continue
		}
		pkg, err := build.ImportDir(dir, 0)
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}
		perm := map[string]bool{}
		for _, w := range want {
			if !strings.HasPrefix(w, "internal/") {
				w = "internal/" + w
			}
			perm[module+"/"+w] = true
		}
		var bad []string
		for _, imp := range pkg.Imports {
			if !strings.HasPrefix(imp, module+"/") {
				continue
			}
			if rel == "internal/cli" || perm[imp] {
				continue
			}
			bad = append(bad, strings.TrimPrefix(imp, module+"/"))
		}
		sort.Strings(bad)
		if len(bad) > 0 {
			t.Errorf("%s imports %v, which architecture 3 does not allow", rel, bad)
		}
	}
}

// TestReleaseConfig checks the CPU and CUDA release build configurations and the version flag.
func TestReleaseConfig(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(root(t), ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		ReportSizes bool `yaml:"report_sizes"`
		Builds      []struct {
			ID, Main, Binary               string
			Env, Goos, Goarch, Tags, Flags []string
			Ldflags                        []string
		}
		Checksum struct{ Algorithm string }
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.ReportSizes || cfg.Checksum.Algorithm != "sha256" {
		t.Error("GoReleaser must report sizes and generate SHA-256 checksums")
	}
	want := map[string]struct{ os, tags []string }{
		"pbrain-cpu":  {os: []string{"linux", "darwin"}},
		"pbrain-cuda": {os: []string{"linux"}, tags: []string{"cuda"}},
	}
	for _, build := range cfg.Builds {
		targets, ok := want[build.ID]
		if !ok {
			t.Errorf("unexpected release build %q", build.ID)
			continue
		}
		delete(want, build.ID)
		if !slices.Equal(build.Goos, targets.os) || !slices.Equal(build.Tags, targets.tags) ||
			!slices.Equal(build.Goarch, []string{"amd64", "arm64"}) {
			t.Errorf("%s: GOOS %v, GOARCH %v, tags %v", build.ID, build.Goos, build.Goarch, build.Tags)
		}
		if build.Main != "./cmd/pbrain" || build.Binary != "pbrain" ||
			!slices.Equal(build.Env, []string{"CGO_ENABLED=0"}) || !slices.Equal(build.Flags, []string{"-trimpath"}) ||
			!strings.Contains(strings.Join(build.Ldflags, " "), "-X github.com/pkieltyka/pbrain/internal/cli.version=") {
			t.Errorf("%s: unexpected build settings %+v", build.ID, build)
		}
	}
	if len(want) != 0 {
		t.Errorf("missing release builds: %v", want)
	}
	for _, banned := range []string{"GOEXPERIMENT", "CGO_ENABLED=1"} {
		if strings.Contains(string(data), banned) {
			t.Errorf(".goreleaser.yaml sets forbidden %q", banned)
		}
	}
	src, err := os.ReadFile(filepath.Join(root(t), "internal", "cli", "run.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "\nvar version string\n") {
		t.Error("internal/cli no longer declares var version string, which the release ldflags set")
	}
}
