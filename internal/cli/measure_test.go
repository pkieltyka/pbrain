package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/item"
)

// TestMeasureReads runs the architecture §8.6 whole-process benchmark, on
// demand: PBRAIN_MEASURE=1 go test -p 2 -count=1 -run '^TestMeasureReads$' -v ./internal/cli
//
// PBRAIN_MEASURE_BIN can name an already built pbrain binary.
// PBRAIN_MEASURE_PROCS sets GOMAXPROCS for the measured processes (default: up to 4).
// PBRAIN_MEASURE_RUNS sets the number of warm samples per case (default: 10).
// PBRAIN_MEASURE_COLD_PREP can name an absolute executable which prepares the
// cold cache before each first sample. It receives the checkout's absolute path.
// The caller supplies and documents that method. Without it, the first sample's
// cache state is uncontrolled. The warm samples always follow a priming read.
func TestMeasureReads(t *testing.T) {
	if os.Getenv("PBRAIN_MEASURE") != "1" {
		t.Skip("set PBRAIN_MEASURE=1 to time the generated 5,000-item brains")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	procs := measurePositiveEnv(t, "PBRAIN_MEASURE_PROCS", min(4, runtime.GOMAXPROCS(0)))
	runs := measurePositiveEnv(t, "PBRAIN_MEASURE_RUNS", 10)
	pbrain := measureBinary(t, root)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(root, "git")
	if err := os.WriteFile(shim, []byte(measureGitShim), 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "git-count")
	env := []string{
		"HOME=" + root, "XDG_CONFIG_HOME=" + root, "PATH=" + root + string(os.PathListSeparator) + os.Getenv("PATH"),
		"TZ=UTC", "LC_ALL=C", "GOMAXPROCS=" + strconv.Itoa(procs),
		"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"PBRAIN_MEASURE_GIT=" + git, "PBRAIN_MEASURE_GIT_LOG=" + logPath,
	}
	coldPrep := os.Getenv("PBRAIN_MEASURE_COLD_PREP")
	cacheLabel := "uncontrolled"
	if coldPrep != "" {
		if !filepath.IsAbs(coldPrep) {
			t.Fatal("PBRAIN_MEASURE_COLD_PREP must be an absolute executable path")
		}
		cacheLabel = "external cold preparation (" + coldPrep + ")"
	}
	t.Logf("%s/%s, %d visible CPUs, GOMAXPROCS=%d, %d warm runs per row", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), procs, runs)
	t.Logf("first sample cache: %s. Timings include the Git-counting shim", cacheLabel)
	help, _ := runMeasureCommand(t, pbrain, root, env, "search", "--help")
	bm25 := bytes.Contains(help, []byte("--bm25"))
	if !bm25 {
		t.Log("this binary does not expose --bm25")
	}

	for _, mode := range []string{"shared", "personal", "outside Git"} {
		t.Run(mode, func(t *testing.T) {
			fixture := makeMeasureCheckout(t, root, mode, pbrain, git, env)
			if fixture.size != 50_000_000 {
				t.Fatalf("generated %d item bytes, want 50,000,000", fixture.size)
			}
			t.Logf("%s: 5,000 items, %d item bytes", mode, fixture.size)
			rows := measureCases(fixture.id)
			if mode == "outside Git" {
				rows = []readMeasureCase{rows[0], {name: "get", args: []string{"get", fixture.id}, want: fixture.id}}
			} else if bm25 {
				for _, row := range rows {
					if row.args[0] == "search" {
						rows = append(rows, readMeasureCase{row.name + " BM25", append(slices.Clone(row.args), "--bm25"), row.want})
					}
				}
			}
			t.Log("case | first ms | warm median ms [min, max] | Git first | Git warm [min, max]")
			for _, row := range rows {
				args := row.args
				if mode == "personal" {
					args = append([]string{"--personal"}, args...)
				}
				if coldPrep != "" {
					runMeasureCommand(t, coldPrep, fixture.dir, os.Environ(), fixture.dir)
				}
				first := measureRead(t, pbrain, fixture.dir, env, logPath, row, args)
				measureRead(t, pbrain, fixture.dir, env, logPath, row, args) // Prime this row's cache.
				durations := make([]time.Duration, runs)
				minGit, maxGit := math.MaxInt, 0
				for i := range durations {
					sample := measureRead(t, pbrain, fixture.dir, env, logPath, row, args)
					durations[i] = sample.elapsed
					minGit, maxGit = min(minGit, sample.git), max(maxGit, sample.git)
				}
				hasGit := mode != "outside Git"
				if (first.git != 0) != hasGit || (minGit != 0) != hasGit || (maxGit != 0) != hasGit {
					t.Fatalf("%s: Git counting disagrees with checkout mode: first %d, warm %d to %d", row.name, first.git, minGit, maxGit)
				}
				slices.Sort(durations)
				median := (durations[(runs-1)/2] + durations[runs/2]) / 2
				t.Logf("%s | %.1f | %.1f [%.1f, %.1f] | %d | [%d, %d]", row.name,
					millis(first.elapsed), millis(median), millis(durations[0]), millis(durations[runs-1]), first.git, minGit, maxGit)
			}
		})
	}
}

const measureGitShim = `#!/bin/sh
printf x >> "$PBRAIN_MEASURE_GIT_LOG" || exit 125
exec "$PBRAIN_MEASURE_GIT" "$@"
`

type readMeasureCase struct {
	name string
	args []string
	want string
}

func measureCases(id string) []readMeasureCase {
	return []readMeasureCase{
		{"search common", []string{"search", "readbenchcommon"}, "more: --offset"},
		{"search two words JSON", []string{"search", "readbenchcommon readbenchaux", "--json"}, `"more":`},
		{"search rare", []string{"search", "readbenchrare"}, id},
		{"search phrase", []string{"search", `"readbenchrare readbenchsignpost"`}, id},
		{"search prefix", []string{"search", "readbenchr*"}, id},
		{"search all", []string{"search", "readbenchcommon", "--all"}, "more: --offset"},
		{"search window", []string{"search", "readbenchcommon", "--since", "2025-01-01", "--until", "2026-01-01"}, "more: --offset"},
		{"search context", []string{"search", "readbenchcommon", "--context", "2"}, "more: --offset"},
		{"search miss", []string{"search", "zzzzreadbenchmissing"}, "0 results."},
		{"get", []string{"get", id}, id},
		{"get outline", []string{"get", id, "--outline"}, id},
		{"get history", []string{"get", id, "--history"}, id},
	}
}

type readMeasureSample struct {
	elapsed time.Duration
	git     int
}

func measureRead(t *testing.T, bin, dir string, env []string, logPath string, row readMeasureCase, args []string) readMeasureSample {
	t.Helper()
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	out, elapsed := runMeasureCommand(t, bin, dir, env, args...)
	if len(out) == 0 || row.want != "" && !strings.Contains(string(out), row.want) {
		t.Fatalf("%s: expected output for %q, got %q", row.name, row.want, out)
	}
	if slices.Contains(args, "--json") && !json.Valid(out) {
		t.Fatalf("%s: invalid JSON output %q", row.name, out)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	return readMeasureSample{elapsed: elapsed, git: len(log)}
}

func runMeasureCommand(t *testing.T, path, dir string, env []string, args ...string) ([]byte, time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir, cmd.Env = dir, env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	start := time.Now()
	out, err := cmd.Output()
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("%s %v: %v\nstdout: %s\nstderr: %s", path, args, err, out, &stderr)
	}
	if stderr.Len() != 0 {
		t.Fatalf("%s %v: unexpected stderr: %s", path, args, &stderr)
	}
	return out, elapsed
}

func measurePositiveEnv(t *testing.T, name string, fallback int) int {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		t.Fatalf("%s must be a positive integer", name)
	}
	return n
}

func millis(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func measureBinary(t *testing.T, root string) string {
	t.Helper()
	if path := os.Getenv("PBRAIN_MEASURE_BIN"); path != "" {
		path, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		return path
	}
	bin := filepath.Join(root, "pbrain")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-p", "2", "-trimpath", "-o", bin, "./cmd/pbrain")
	cmd.Dir, cmd.Env = "../..", append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build pbrain: %v\n%s", err, out)
	}
	return bin
}

type readMeasureFixture struct {
	dir, id string
	size    int
}

func makeMeasureCheckout(t *testing.T, root, mode, pbrain, git string, env []string) readMeasureFixture {
	t.Helper()
	dir := filepath.Join(root, strings.ReplaceAll(mode, " ", "-"))
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	hasGit := mode != "outside Git"
	if hasGit {
		runMeasureCommand(t, git, dir, env, "init", "-q", "--template=")
	}
	runMeasureCommand(t, pbrain, dir, env, "init")
	brain := filepath.Join(dir, "docs", "brain")
	shared := 5_000
	if mode == "personal" {
		shared -= 500
	}
	id, size := genBrain(t, brain, shared, 10_000, "shared")
	if hasGit {
		runMeasureCommand(t, git, dir, env, "add", "--", "docs/brain")
		runMeasureCommand(t, git, dir, env, "-c", "user.name=Read Bench", "-c", "user.email=readbench@example.invalid",
			"commit", "-qm", "Generate shared benchmark brain", "--no-gpg-sign")
	}
	if mode == "personal" {
		var extra int
		id, extra = genBrain(t, filepath.Join(brain, "personal"), 500, 10_000, "personal")
		size += extra
	}
	return readMeasureFixture{dir: dir, id: id, size: size}
}

// genBrain builds a reproducible section with varying item lengths and the
// requested mean file size, including the header. It returns a current id
// with a short replacement history for the get and rare-search cases.
func genBrain(t testing.TB, dir string, n, meanBytes int, prefix string) (id string, size int) {
	t.Helper()
	rng := rand.New(rand.NewPCG(7, uint64(len(prefix))))
	vocab := make([]string, 20_000)
	for i := range vocab {
		word := make([]byte, 3+rng.IntN(8))
		for j := range word {
			word[j] = 'a' + byte(rng.IntN(26))
		}
		vocab[i] = string(word)
	}
	word := func() string {
		return vocab[int(float64(len(vocab))*rng.Float64()*rng.Float64()*rng.Float64())]
	}
	kinds := []item.Kind{item.Note, item.Note, item.Learning, item.Observation, item.Decision, item.Source}
	for _, kind := range kinds {
		if err := os.MkdirAll(filepath.Join(dir, kind.String()+"s"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	var previous string
	spread := 0
	for i := range n {
		kind := kinds[rng.IntN(len(kinds))]
		created := start.Add(time.Duration(i*5+rng.IntN(5)) * time.Hour)
		id = fmt.Sprintf("%s-%s%04x", created.Format("20060102-150405"), prefix, i)
		draft := item.Draft{
			Kind: kind, Created: created, Title: strings.Join([]string{word(), word(), word(), word()}, " "),
			Author: "agent:readbench", Tags: []string{word(), word()},
		}
		if i >= n-3 {
			draft.Kind = item.Decision
			if previous != "" {
				draft.Replaces = []string{previous}
			}
			previous = id
		} else if kind == item.Source {
			draft.Source = fmt.Sprintf("https://example.invalid/%s", id)
		}
		if draft.Kind == item.Decision {
			draft.Decided, draft.Status = "human:readbench", item.Verified
		}
		data, err := item.Emit(&draft, nil)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, '\n')
		switch {
		case i%2 != 0:
			spread = -spread
		case i+1 < n:
			spread = rng.IntN(meanBytes / 2)
		default:
			spread = 0
		}
		budget := meanBytes + spread - len(data)
		data = append(data, measureBody(rng, word, budget, i == n-1)...)
		path := filepath.Join(dir, draft.Kind.String()+"s", id+".md")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		size += len(data)
	}
	return id, size
}

func measureBody(rng *rand.Rand, word func() string, budget int, anchor bool) string {
	var body strings.Builder
	body.WriteString("## Introduction\n\nreadbenchcommon café 日本語.\n\n")
	if anchor {
		body.WriteString("The readbenchrare readbenchsignpost marks this item.\n\n")
	}
	aux := false
	for i := 0; body.Len() < budget; i++ {
		if i%5 == 0 {
			fmt.Fprintf(&body, "## Section %d\n\n", i/5+1)
		}
		if !aux && body.Len() >= budget/2 {
			body.WriteString("readbenchaux ")
			aux = true
		}
		for range 20 + rng.IntN(60) {
			body.WriteString(word())
			body.WriteByte(' ')
		}
		body.WriteString("\n\n")
	}
	return body.String()[:budget-1] + "\n"
}
