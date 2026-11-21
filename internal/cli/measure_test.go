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
	"syscall"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/item"
)

const (
	measureItems         = 5_000
	measurePersonalItems = 500
	measureMeanFileBytes = 10_000
)

// TestMeasureReads runs the architecture §8.6 whole-process benchmark, on
// demand: PBRAIN_MEASURE=1 go test -p 2 -count=1 -run '^TestMeasureReads$' -v ./internal/cli
//
// PBRAIN_MEASURE_BIN can name an already built pbrain binary.
// PBRAIN_MEASURE_PROCS sets GOMAXPROCS for the measured processes (default: up to 4).
// PBRAIN_MEASURE_RUNS sets the number of warm samples per case (default: 10).
// PBRAIN_MEASURE_COLD_PREP can name an absolute executable which prepares the
// cold cache before each first sample. It receives the checkout's absolute path.
// For model cases the second argument is the pinned model directory.
// The caller supplies and documents that method. Without it, the first sample's
// cache state is uncontrolled. The warm samples always follow a priming read.
//
// PBRAIN_TEST_BGE_MODEL or PBRAIN_TEST_GEMMA2_PACK adds model cases using
// the pinned local directory. Select one model per run. Use a test timeout
// long enough for two first index builds. See README.measure.md for details.
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
	harness := readMeasureHarness{bin: pbrain, gitLog: logPath, runs: runs, env: []string{
		"HOME=" + root, "XDG_CONFIG_HOME=" + root, "PATH=" + root + string(os.PathListSeparator) + os.Getenv("PATH"),
		"TZ=UTC", "LC_ALL=C", "GOMAXPROCS=" + strconv.Itoa(procs),
		"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"PBRAIN_MEASURE_GIT=" + git, "PBRAIN_MEASURE_GIT_LOG=" + logPath,
	}}
	harness.coldPrep = os.Getenv("PBRAIN_MEASURE_COLD_PREP")
	cacheLabel := "uncontrolled"
	if harness.coldPrep != "" {
		if !filepath.IsAbs(harness.coldPrep) {
			t.Fatal("PBRAIN_MEASURE_COLD_PREP must be an absolute executable path")
		}
		cacheLabel = "external cold preparation (" + harness.coldPrep + ")"
	}
	t.Logf("%s/%s, %d visible CPUs, GOMAXPROCS=%d, %d warm runs per row", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), procs, runs)
	t.Logf("first sample cache: %s. Timings include the Git-counting shim", cacheLabel)
	help := runMeasureCommand(t, pbrain, root, harness.env, "search", "--help")
	bm25 := bytes.Contains(help, []byte("--bm25"))
	if !bm25 {
		t.Log("this binary does not expose --bm25")
	}
	model := measureModelFromEnv(t)
	if model == nil {
		t.Log("set PBRAIN_TEST_BGE_MODEL or PBRAIN_TEST_GEMMA2_PACK to add index and semantic timings")
	}

	for _, mode := range []string{"shared", "personal", "outside Git"} {
		t.Run(mode, func(t *testing.T) {
			fixture := makeMeasureCheckout(t, root, mode, pbrain, git, harness.env)
			if want := measureItems * measureMeanFileBytes; fixture.size != want {
				t.Fatalf("generated %d item bytes, want %d", fixture.size, want)
			}
			t.Logf("%s: %d items, %d item bytes", mode, measureItems, fixture.size)
			rows := measureCases(fixture.id)
			if mode == "outside Git" {
				rows = []readMeasureCase{rows[0], {name: "get", args: []string{"get", fixture.id}, want: fixture.id}}
			} else if bm25 {
				for _, row := range rows {
					if row.args[0] == "search" {
						rows = append(rows, readMeasureCase{name: row.name + " BM25", args: append(slices.Clone(row.args), "--bm25"), want: row.want})
					}
				}
				rows = append(rows, readMeasureCase{
					name: "search admitted 5/6 BM25", args: []string{"search", measurePartialQuery, "--bm25", "--json"}, want: `"partial":true`,
				})
			}
			harness.timeReads(t, fixture, rows, "")
			if model != nil && fixture.hasGit {
				harness.measureModel(t, fixture, *model, bm25)
			}
		})
	}
}

type readMeasureHarness struct {
	bin, gitLog, coldPrep string
	env                   []string
	runs                  int
}

func (h readMeasureHarness) chill(t *testing.T, fixture readMeasureFixture, modelPath string) {
	t.Helper()
	if h.coldPrep == "" {
		return
	}
	args := []string{fixture.dir}
	if modelPath != "" {
		args = append(args, modelPath)
	}
	runMeasureCommand(t, h.coldPrep, fixture.dir, os.Environ(), args...)
}

func (h readMeasureHarness) timeReads(t *testing.T, fixture readMeasureFixture, rows []readMeasureCase, modelPath string) {
	t.Helper()
	t.Log("case | first ms | warm median ms [min, max] | first peak MiB | max warm peak MiB | Git first | Git warm [min, max]")
	for _, row := range rows {
		args := fixture.viewArgs(row.args)
		measure := func() readMeasureSample {
			t.Helper()
			sample, err := h.measureRead(t, fixture, row, args)
			if err != nil {
				t.Fatalf("%s: %v", row.name, err)
			}
			if slices.Contains(args, "--semantic") && sample.elapsed >= time.Second {
				t.Logf("%s exceeded 1 second: %.1f ms", row.name, millis(sample.elapsed))
			}
			return sample
		}
		h.chill(t, fixture, modelPath)
		first := measure()
		measure() // Prime this row's cache.
		durations := make([]time.Duration, h.runs)
		minGit, maxGit := math.MaxInt, 0
		var maxRSS uint64
		for i := range durations {
			sample := measure()
			durations[i] = sample.elapsed
			minGit, maxGit = min(minGit, sample.git), max(maxGit, sample.git)
			maxRSS = max(maxRSS, sample.peakBytes)
		}
		if (first.git != 0) != fixture.hasGit || (minGit != 0) != fixture.hasGit || (maxGit != 0) != fixture.hasGit {
			t.Fatalf("%s: Git counting disagrees with checkout mode: first %d, warm %d to %d", row.name, first.git, minGit, maxGit)
		}
		slices.Sort(durations)
		median := (durations[(h.runs-1)/2] + durations[h.runs/2]) / 2
		t.Logf("%s | %.1f | %.1f [%.1f, %.1f] | %.1f | %.1f | %d | [%d, %d]", row.name,
			millis(first.elapsed), millis(median), millis(durations[0]), millis(durations[h.runs-1]),
			mib(first.peakBytes), mib(maxRSS), first.git, minGit, maxGit)
	}
}

const measureGitShim = `#!/bin/sh
printf x >> "$PBRAIN_MEASURE_GIT_LOG" || exit 125
exec "$PBRAIN_MEASURE_GIT" "$@"
`

type readMeasureCase struct {
	name       string
	args       []string
	want       string
	semanticID string
}

func measureCases(id string) []readMeasureCase {
	return []readMeasureCase{
		{name: "search common", args: []string{"search", "readbenchcommon"}, want: "more: --offset"},
		{name: "search two words JSON", args: []string{"search", "readbenchcommon readbenchaux", "--json"}, want: `"more":`},
		{name: "search rare", args: []string{"search", "readbenchrare"}, want: id},
		{name: "search phrase", args: []string{"search", `"readbenchrare readbenchsignpost"`}, want: id},
		{name: "search prefix", args: []string{"search", "readbenchr*"}, want: id},
		{name: "search all", args: []string{"search", "readbenchcommon", "--all"}, want: "more: --offset"},
		{name: "search window", args: []string{"search", "readbenchcommon", "--since", "2025-01-01", "--until", "2026-01-01"}, want: "more: --offset"},
		{name: "search context", args: []string{"search", "readbenchcommon", "--context", "2"}, want: "more: --offset"},
		{name: "search miss", args: []string{"search", "zzzzreadbenchmissing"}, want: "0 results."},
		{name: "get", args: []string{"get", id}, want: id},
		{name: "get outline", args: []string{"get", id, "--outline"}, want: id},
		{name: "get history", args: []string{"get", id, "--history"}, want: id},
		{name: "list JSON", args: []string{"list", "--json"}, want: `"results":`},
		{name: "prime JSON", args: []string{"prime", "--json"}, want: `"decisions":`},
	}
}

type readMeasureSample struct {
	elapsed   time.Duration
	peakBytes uint64
	git       int
}

func (h readMeasureHarness) measureRead(t *testing.T, fixture readMeasureFixture, row readMeasureCase, args []string) (readMeasureSample, error) {
	t.Helper()
	got, sample := h.measureCLI(t, fixture, 3*time.Minute, args)
	if len(got.stderr) != 0 {
		return sample, fmt.Errorf("unexpected stderr: %s", got.stderr)
	}
	out := got.stdout
	if len(out) == 0 || row.want != "" && !strings.Contains(string(out), row.want) {
		return sample, fmt.Errorf("expected output for %q, got %q", row.want, out)
	}
	if slices.Contains(args, "--json") && !json.Valid(out) {
		return sample, fmt.Errorf("invalid JSON output %q", out)
	}
	if row.semanticID != "" {
		return sample, checkMeasuredHit(out, row.semanticID, true)
	}
	return sample, nil
}

func (h readMeasureHarness) measureCLI(t *testing.T, f readMeasureFixture, timeout time.Duration, args []string) (readMeasureCommand, readMeasureSample) {
	t.Helper()
	if err := os.WriteFile(h.gitLog, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got := runMeasured(t, h.bin, f.dir, h.env, timeout, args...)
	log, err := os.ReadFile(h.gitLog)
	if err != nil {
		t.Fatal(err)
	}
	return got, readMeasureSample{elapsed: got.elapsed, peakBytes: got.peakBytes, git: len(log)}
}

func runMeasureCommand(t *testing.T, path, dir string, env []string, args ...string) []byte {
	t.Helper()
	got := runMeasured(t, path, dir, env, 3*time.Minute, args...)
	if len(got.stderr) != 0 {
		t.Fatalf("%s %v: unexpected stderr: %s", path, args, got.stderr)
	}
	return got.stdout
}

type readMeasureCommand struct {
	stdout, stderr []byte
	elapsed        time.Duration
	peakBytes      uint64
}

func runMeasured(t *testing.T, path, dir string, env []string, timeout time.Duration, args ...string) readMeasureCommand {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
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
	usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		t.Fatal("child peak RSS is unavailable")
	}
	peak := uint64(usage.Maxrss)
	if runtime.GOOS == "linux" {
		peak *= 1024 // Linux reports KiB. Darwin reports bytes.
	}
	return readMeasureCommand{stdout: out, stderr: stderr.Bytes(), elapsed: elapsed, peakBytes: peak}
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
func mib(n uint64) float64           { return float64(n) / (1024 * 1024) }

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
	dir, brain, id, sharedID, semanticID string
	size                                 int
	shared, personal                     int
	hasGit                               bool
}

func (f readMeasureFixture) viewArgs(args []string) []string {
	if f.personal > 0 {
		return append([]string{"--personal"}, args...)
	}
	return args
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
	shared := measureItems
	personal := 0
	if mode == "personal" {
		personal = measurePersonalItems
		shared -= personal
	}
	sharedID, semanticID, size := genBrain(t, brain, shared, measureMeanFileBytes, "shared")
	id := sharedID
	if hasGit {
		runMeasureCommand(t, git, dir, env, "add", "--", "docs/brain")
		runMeasureCommand(t, git, dir, env, "-c", "user.name=Read Bench", "-c", "user.email=readbench@example.invalid",
			"commit", "-qm", "Generate shared benchmark brain", "--no-gpg-sign")
	}
	if personal > 0 {
		var extra int
		id, _, extra = genBrain(t, filepath.Join(brain, "personal"), personal, measureMeanFileBytes, "personal")
		size += extra
	}
	return readMeasureFixture{dir: dir, brain: brain, id: id, sharedID: sharedID, semanticID: semanticID, size: size, shared: shared, personal: personal, hasGit: hasGit}
}

// genBrain builds a reproducible section with varying item lengths and the
// requested mean file size, including the header. It returns the current ID
// for get and rare search, and the shared semantic control's ID when present.
func genBrain(t testing.TB, dir string, n, meanBytes int, prefix string) (id, semanticID string, size int) {
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
	spread, extra := 0, 0
	for i := range n {
		kind := kinds[rng.IntN(len(kinds))]
		created := start.Add(time.Duration(i*5+rng.IntN(5)) * time.Hour)
		id = fmt.Sprintf("%s-%s%04x", created.Format("20060102-150405"), prefix, i)
		draft := item.Draft{
			Kind: kind, Created: created, Title: strings.Join([]string{word(), word(), word(), word()}, " "),
			Author: "agent:readbench", Tags: []string{word(), word()},
		}
		if i == 0 && n > 3 && prefix == "shared" {
			draft.Kind, draft.Title = item.Note, "Login session storage"
			semanticID = id
		}
		if i >= n-3 {
			draft.Kind = item.Decision
			if previous != "" {
				draft.Replaces = []string{previous}
			}
			previous = id
		} else if draft.Kind == item.Source {
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
		budget := meanBytes + spread - len(data) + extra
		extra = 0
		if id == semanticID {
			data = append(data, measureSemanticBody...)
			extra = budget - len(measureSemanticBody)
		} else {
			data = append(data, measureBody(rng, word, budget, i == n-1)...)
		}
		path := filepath.Join(dir, draft.Kind.String()+"s", id+".md")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		size += len(data)
	}
	return id, semanticID, size
}

func measureBody(rng *rand.Rand, word func() string, budget int, anchor bool) string {
	var body strings.Builder
	body.WriteString("## Introduction\n\nreadbenchcommon café 日本語.\n\n")
	if anchor {
		body.WriteString("The readbenchrare readbenchsignpost readbenchlease readbenchwork readbenchrenew marks this item.\n")
		body.WriteString("A worker renews its lease after confirming a retry.\n\n")
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
