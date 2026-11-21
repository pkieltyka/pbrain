package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/cli"
	"github.com/pkieltyka/pbrain/internal/embed/embedtest"
	"github.com/pkieltyka/pbrain/internal/gitx"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"pbrain":       func() { os.Exit(run()) },
		"pbrain-test":  func() { os.Exit(runWith(scenarioCLI)) },
		"pbrain-stray": func() { os.Exit(runWith(strayCLI)) },
	})
}

// Enter through the same process adapter as the release. The test executable alone
// substitutes a reproducible clock, offline registry, and checked transport.
func scenarioCLI(ctx context.Context, e cli.Env) int {
	if e.Getenv("PBRAIN_TEST_DEADLINE") == "short" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
	}
	now, err := time.Parse(time.RFC3339, e.Getenv("PBRAIN_TEST_NOW"))
	if err != nil {
		panic(err)
	}
	e.Now = func() time.Time { return now }
	kit, err := scenarioModelKit(e.Getenv("PBRAIN_TEST_EMBED_TABLE"))
	if err != nil {
		panic(err)
	}
	e.Models = kit.Registry
	n := newScriptNetwork(e.Getenv("PBRAIN_TEST_HTTP"))
	e.Net.Resolver, e.Net.Dialer = n, n
	if mode := e.Getenv("PBRAIN_TEST_FAULT"); mode != "" {
		e.Files = &scenarioFiles{Files: e.Files, mode: mode}
	}
	if e.Getenv("PBRAIN_TEST_NO_GIT") == "1" {
		e.Git = gitx.NewRunner("/missing-scenario-git", nil)
	}
	code := cli.RunWith(ctx, e, cli.Features{BM25: e.Getenv("PBRAIN_TEST_BM25") != "off"})
	if n.unexpected.Load() {
		fmt.Fprintln(e.Stderr, "scenario: attempted unconfigured network access")
		return 99
	}
	return code
}

func TestScenarios(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:                "testdata/scenarios",
		Setup:              setupScenario,
		RequireUniqueNames: true,
		Condition:          scenarioCondition,
		Cmds: map[string]func(*testscript.TestScript, bool, []string){
			"guard":         assertGuard,
			"pagebudget":    pageBudget,
			"tree-absent":   treeAbsent,
			"crlf":          crlf,
			"status":        scenarioStatus,
			"jsonenv":       jsonEnv,
			"ids":           assertIDs,
			"parallel-add":  parallelAdd,
			"killed-add":    killedAdd,
			"killed-index":  killedIndex,
			"budgets":       scenarioBudgets,
			"testmodel":     scenarioTestModel,
			"damage-index":  scenarioDamageIndex,
			"audience-race": scenarioAudienceRace,
			"bytefile":      scenarioByteFile,
			"byterm":        scenarioByteRm,
			"nocolor":       scenarioNoColor,
			"ptyout":        scenarioPTYOut,
			"fits":          scenarioFits,
			"fileline":      scenarioFileLine,
			"nofinal":       scenarioNoFinal,
			"minbudget":     scenarioMinBudget,
			"sameprefix":    scenarioSamePrefix,
			"countfiles":    scenarioCountFiles,
			"ptyskip":       scenarioPTYSkip,
			"modelpath":     scenarioModelPath,
			"fsnote":        scenarioFSNote,
		},
	})
}

func setupScenario(e *testscript.Env) error {
	repo := filepath.Join(e.WorkDir, "repo")
	home := filepath.Join(e.WorkDir, "home")
	model := filepath.Join(e.WorkDir, "model")
	for _, d := range []string{repo, home, model} {
		if err := os.MkdirAll(d, 0700); err != nil {
			return err
		}
	}
	e.Cd = repo
	e.Setenv("HOME", home)
	e.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	e.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	e.Setenv("GIT_AUTHOR_NAME", "scenario")
	e.Setenv("GIT_COMMITTER_NAME", "scenario")
	e.Setenv("GIT_AUTHOR_EMAIL", "scenario@example.invalid")
	e.Setenv("GIT_COMMITTER_EMAIL", "scenario@example.invalid")
	e.Setenv("PBRAIN_AUTHOR", "human:scenario")
	e.Setenv("PBRAIN_TEST_NOW", "2026-11-21T12:00:00Z")
	e.Setenv("PBRAIN_DEFAULT_MODEL", brain.DefaultModel)
	e.Setenv("TZ", "UTC")
	e.Setenv("PBRAIN_CACHE", filepath.Join(e.WorkDir, "cache"))
	e.Setenv("MODEL", model)
	kit := embedtest.New(nil)
	for _, f := range kit.Models[embedtest.ModelA].Files {
		if err := os.WriteFile(filepath.Join(model, f.Name), kit.Bytes[f.URL], 0600); err != nil {
			return err
		}
	}
	git := exec.Command("git", "init", "-q", repo)
	git.Env = e.Vars
	if out, err := git.CombinedOutput(); err != nil {
		return fmt.Errorf("git init: %w: %s", err, out)
	}
	return nil
}

func scenarioStatus(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) < 3 {
		ts.Fatalf("usage: status EXIT ARGS...")
	}
	want, err := strconv.Atoi(args[0])
	ts.Check(err)
	err = ts.Exec(args[1], args[2:]...)
	got := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			ts.Fatalf("%v", err)
		}
		got = ee.ExitCode()
	}
	if got != want {
		ts.Fatalf("exit %d, want %d", got, want)
	}
}
func jsonEnv(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 2 {
		ts.Fatalf("usage: jsonenv VAR key.path.0")
	}
	var value any
	ts.Check(json.Unmarshal([]byte(ts.ReadFile("stdout")), &value))
	for _, key := range strings.Split(args[1], ".") {
		switch obj := value.(type) {
		case map[string]any:
			value = obj[key]
		case []any:
			i, err := strconv.Atoi(key)
			ts.Check(err)
			if i < 0 || i >= len(obj) {
				ts.Fatalf("bad array index %s", key)
			}
			value = obj[i]
		default:
			ts.Fatalf("missing JSON field %s", args[1])
		}
	}
	switch v := value.(type) {
	case string:
		ts.Setenv(args[0], v)
	case float64:
		ts.Setenv(args[0], strconv.FormatFloat(v, 'f', -1, 64))
	default:
		ts.Fatalf("%s is not a string or number", args[1])
	}
}

// ids checks the entire ordered projection, including an empty/omitted array.
func assertIDs(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) < 1 {
		ts.Fatalf("usage: ids ARRAY [ID...]")
	}
	// Envelopes also contain scalar fields. Decode only the requested row array.
	var root map[string]json.RawMessage
	ts.Check(json.Unmarshal([]byte(ts.ReadFile("stdout")), &root))
	var rows []struct{ ID string }
	if raw, ok := root[args[0]]; ok {
		ts.Check(json.Unmarshal(raw, &rows))
	}
	if len(rows) != len(args)-1 {
		ts.Fatalf("wanted %v, got rows %v", args[1:], rows)
	}
	for i, r := range rows {
		if r.ID != args[i+1] {
			ts.Fatalf("wanted %v, got rows %v", args[1:], rows)
		}
	}
}
func pageBudget(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 2 {
		ts.Fatalf("usage: pagebudget VAR FILE")
	}
	ts.Setenv(args[0], strconv.Itoa(utf8.RuneCountInString(ts.ReadFile(args[1]))))
}
func crlf(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 2 {
		ts.Fatalf("usage: crlf SOURCE DEST")
	}
	ts.Check(os.WriteFile(ts.MkAbs(args[1]), []byte(strings.ReplaceAll(ts.ReadFile(args[0]), "\n", "\r\n")), 0600))
}
func treeAbsent(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 2 {
		ts.Fatalf("usage: tree-absent DIRECTORY TEXT")
	}
	root := ts.MkAbs(args[0])
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return
	}
	ts.Check(filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(path, args[1]) {
			return fmt.Errorf("found private value in a filename")
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(b, []byte(args[1])) {
			return fmt.Errorf("found private value in a file")
		}
		return nil
	}))
}

// guard checks the exposure result at each command entry. index uses the
// offline test executable so even a broken guard cannot download a model.
func assertGuard(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 0 {
		ts.Fatalf("usage: guard")
	}
	commands := [][]string{{"init"}, {"list"}, {"search", "probe"}, {"get", "missing"}, {"tags"}, {"prime"}, {"check"}, {"audience"}, {"add", "--title", "probe"}, {"delete", "missing"}, {"verify", "missing"}, {"import", "missing.md"}, {"sync", "--status"}, {"sync"}}
	for _, a := range commands {
		err := ts.Exec("pbrain-test", append(a, "--json")...)
		var ee *exec.ExitError
		var e struct{ Error struct{ Code string } }
		if !errors.As(err, &ee) || ee.ExitCode() != 1 || ts.ReadFile("stdout") != "" || json.Unmarshal([]byte(ts.ReadFile("stderr")), &e) != nil || e.Error.Code != "personal_exposed" {
			ts.Fatalf("%v bypassed exposure guard: %v %s %s", a, err, ts.ReadFile("stdout"), ts.ReadFile("stderr"))
		}
	}
}
