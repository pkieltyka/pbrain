package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/rogpeppe/go-internal/testscript"
)

// bytefile SRC DEST copies SRC to DEST, whose name is a Go string literal's contents: \xff, \n,
// \u202e and the like give names that are not valid UTF-8 or hold control characters (scenario 39).
// Folders on the way are created.
func scenarioByteFile(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 2 {
		ts.Fatalf("usage: bytefile SRC DEST")
	}
	dest, err := strconv.Unquote(`"` + args[1] + `"`)
	if err != nil {
		ts.Fatalf("bytefile: %q is not a Go string body: %v", args[1], err)
	}
	path := ts.MkAbs(dest)
	ts.Check(os.MkdirAll(filepath.Dir(path), 0o700))
	ts.Check(os.WriteFile(path, []byte(ts.ReadFile(args[0])), 0o600))
}

// byterm NAME removes a file whose name bytefile wrote, its escapes decoded the same way.
func scenarioByteRm(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 1 {
		ts.Fatalf("usage: byterm NAME")
	}
	name, err := strconv.Unquote(`"` + args[0] + `"`)
	if err != nil {
		ts.Fatalf("byterm: %q is not a Go string body: %v", args[0], err)
	}
	ts.Check(os.Remove(ts.MkAbs(name)))
}

// addedSequences are the sequences pbrain adds with --color: bold, cyan and the reset.
var addedSequences = regexp.MustCompile("\x1b\\[(1|36|0)m")

// nocolor SRC DEST writes SRC with the sequences pbrain adds removed (scenario 41). A page at a tight
// limit may hold none.
func scenarioNoColor(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 2 {
		ts.Fatalf("usage: nocolor SRC DEST")
	}
	src := []byte(ts.ReadFile(args[0]))
	ts.Check(os.WriteFile(ts.MkAbs(args[1]), addedSequences.ReplaceAll(src, nil), 0o600))
}

// scriptEnv is the environment a script's commands run with, rebuilt for a child started by hand.
var scriptEnv = []string{"PATH", "HOME", "TMPDIR", "WORK", "TZ", "MODEL", "PBRAIN_CACHE", "PBRAIN_AUTHOR",
	"PBRAIN_TEST_NOW", "PBRAIN_TEST_EMBED_TABLE", "PBRAIN_TEST_BM25", "GIT_CONFIG_NOSYSTEM", "GIT_CONFIG_GLOBAL",
	"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL",
	"TERM", "COLORTERM", "CLICOLOR_FORCE", "FORCE_COLOR", "NO_COLOR"}

// ptyout FILE COMMAND ARGS... runs a registered command with its standard output on a pseudo-terminal
// with output translation off, and writes what it printed there to FILE (scenario 41). It fails on a
// nonzero exit or any standard error; a script guards it with the [pty] condition.
func scenarioPTYOut(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) < 2 {
		ts.Fatalf("usage: ptyout FILE COMMAND ARGS...")
	}
	var path string
	for _, dir := range filepath.SplitList(ts.Getenv("PATH")) {
		if p := filepath.Join(dir, args[1]); fileExists(p) {
			path = p
			break
		}
	}
	if path == "" {
		ts.Fatalf("ptyout: no command %s", args[1])
	}
	cmd := exec.Command(path, args[2:]...)
	cmd.Dir = ts.MkAbs(".")
	for _, k := range scriptEnv {
		if v := ts.Getenv(k); v != "" {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	var diag bytes.Buffer
	cmd.Stderr = &diag
	out, skip, err := runOnPTY(cmd)
	if skip != "" {
		ts.Fatalf("ptyout without a pseudo-terminal (%s): guard it with [pty]", skip)
	}
	if err != nil || diag.Len() > 0 {
		ts.Fatalf("ptyout %v: %v %q", args[1:], err, diag.String())
	}
	ts.Check(os.WriteFile(ts.MkAbs(args[0]), out, 0o600))
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// ptyskip logs why the script's [pty] lines are skipped, so the skip is never silent; a script runs
// it as [!pty] ptyskip.
func scenarioPTYSkip(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 0 {
		ts.Fatalf("usage: [!pty] ptyskip")
	}
	ts.Logf("skipped: %s", ptyUnavailable())
}

// scenarioCondition adds [pty], a pseudo-terminal can be opened here, and the file system's traits
// some scenarios depend on: [badnames], it keeps a name that is not valid UTF-8 (APFS refuses one);
// [nfdistinct], it keeps "café" in NFC and in NFD as two names (APFS takes them as one); and
// [casedistinct], it keeps "Mixed" and "mixed" as two names (APFS by default does not).
func scenarioCondition(cond string) (bool, error) {
	switch cond {
	case "pty":
		return ptyAvailable(), nil
	case "root":
		return os.Geteuid() == 0, nil
	case "badnames", "nfdistinct", "casedistinct":
		fsTraitsOnce.Do(probeFSTraits)
		return fsTraits[cond], nil
	}
	return false, fmt.Errorf("unknown condition %q", cond)
}

var (
	fsTraitsOnce sync.Once
	fsTraits     = map[string]bool{}
)

// probeFSTraits tries each trait once in a temporary folder of the file system the scripts run on.
func probeFSTraits() {
	dir, err := os.MkdirTemp("", "pbrain-fs-traits-")
	if err != nil {
		return
	}
	defer os.RemoveAll(dir)
	fsTraits["badnames"] = os.WriteFile(filepath.Join(dir, "x\xff"), nil, 0o600) == nil
	distinct := func(a, b string) bool {
		if os.WriteFile(filepath.Join(dir, a), nil, 0o600) != nil {
			return false
		}
		_, err := os.Stat(filepath.Join(dir, b))
		return errors.Is(err, fs.ErrNotExist)
	}
	fsTraits["nfdistinct"] = distinct("caf\u00e9", "cafe\u0301")
	fsTraits["casedistinct"] = distinct("Mixed", "mixed")
}

// fits FILE N fails when FILE holds more than N code points.
func scenarioFits(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: fits FILE N")
	}
	n, err := strconv.Atoi(args[1])
	ts.Check(err)
	got := utf8.RuneCountInString(ts.ReadFile(args[0]))
	if (got <= n) == neg {
		ts.Fatalf("%s holds %d code points; limit %d", args[0], got, n)
	}
}

// fileline FILE N REGEXP checks line N of a stored file, its carriage return and a leading byte-order
// mark set aside: the line a location names holds what the snippet starts with (scenario 39).
func scenarioFileLine(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 3 {
		ts.Fatalf("usage: fileline FILE N REGEXP")
	}
	n, err := strconv.Atoi(args[1])
	ts.Check(err)
	lines := strings.Split(strings.TrimPrefix(ts.ReadFile(args[0]), "\ufeff"), "\n")
	if n < 1 || n > len(lines) {
		ts.Fatalf("%s has no line %d", args[0], n)
	}
	line := strings.TrimSuffix(lines[n-1], "\r")
	re, err := regexp.Compile(args[2])
	ts.Check(err)
	if !re.MatchString(line) {
		ts.Fatalf("line %d of %s is %q, not matching %s", n, args[0], line, args[2])
	}
}

// nofinal SRC DEST copies SRC without its final line feed: a file that ends without one.
func scenarioNoFinal(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 2 {
		ts.Fatalf("usage: nofinal SRC DEST")
	}
	ts.Check(os.WriteFile(ts.MkAbs(args[1]), []byte(strings.TrimSuffix(ts.ReadFile(args[0]), "\n")), 0o600))
}

var statedMinimum = regexp.MustCompile("smallest limit that works is ([0-9]+)")

// minbudget VAR sets VAR to the minimum the last command's budget_too_small error states.
func scenarioMinBudget(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 1 {
		ts.Fatalf("usage: minbudget VAR")
	}
	m := statedMinimum.FindStringSubmatch(ts.ReadFile("stderr"))
	if m == nil {
		ts.Fatalf("no stated minimum in %q", ts.ReadFile("stderr"))
	}
	ts.Setenv(args[0], m[1])
}

// sameprefix PLAIN WITH checks that the page PLAIN, without its closing line, is how WITH begins:
// a flag that only adds results changes nothing before them (scenarios 33 and 38).
func scenarioSamePrefix(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 2 {
		ts.Fatalf("usage: sameprefix PLAIN WITH")
	}
	plain := strings.TrimSuffix(ts.ReadFile(args[0]), "\n")
	cut := strings.LastIndex(plain, "\n")
	if cut < 0 {
		ts.Fatalf("%s has no rows", args[0])
	}
	if with := ts.ReadFile(args[1]); !strings.HasPrefix(with, plain[:cut+1]) {
		ts.Fatalf("%s does not begin with the rows of %s:\n%s", args[1], args[0], with)
	}
}

// countfiles DIR N checks that DIR holds N regular files, at any depth; a missing DIR holds none.
func scenarioCountFiles(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 2 {
		ts.Fatalf("usage: countfiles DIR N")
	}
	want, err := strconv.Atoi(args[1])
	ts.Check(err)
	n := 0
	root := ts.MkAbs(args[0])
	if _, err := os.Stat(root); err == nil {
		ts.Check(filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
			if err == nil && d.Type().IsRegular() {
				n++
			}
			return err
		}))
	}
	if n != want {
		ts.Fatalf("%s holds %d files, want %d", args[0], n, want)
	}
}

// modelpath PATH writes docs/brain/personal.toml naming PATH as model_path; script files cannot
// expand $WORK, so a script that needs a path of its own (a link, say) sets it this way.
func scenarioModelPath(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 1 {
		ts.Fatalf("usage: modelpath PATH")
	}
	ts.Check(os.WriteFile(ts.MkAbs("docs/brain/personal.toml"), []byte("model_path = "+strconv.Quote(args[0])+"\n"), 0o600))
}

// fsnote MESSAGE logs why the lines a file-system condition leaves out did not run, so the skip is
// never silent; a script runs it as, say, [!nfdistinct] fsnote '...'.
func scenarioFSNote(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 1 {
		ts.Fatalf("usage: fsnote MESSAGE")
	}
	ts.Logf("skipped: %s", args[0])
}
