package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func retainedStatusCommand() string {
	if os.Getenv("PBRAIN_PHASE28_STATUS_COMMAND") == "sync" {
		return "sync"
	}
	return "index"
}

func retainedFixture(t *testing.T) (readMeasureHarness, readMeasureFixture, *measureModelOptions) {
	t.Helper()
	if os.Getenv("PBRAIN_MEASURE") != "1" {
		t.Skip("set PBRAIN_MEASURE=1 to reuse the retained fixture")
	}
	root := os.Getenv("PBRAIN_PHASE28_RETAIN")
	if !filepath.IsAbs(root) {
		t.Fatal("PBRAIN_PHASE28_RETAIN must name the retained fixture root")
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "git-count")
	h := readMeasureHarness{bin: measureBinary(t, root), gitLog: logPath,
		runs: measurePositiveEnv(t, "PBRAIN_MEASURE_RUNS", 10), env: []string{
			"HOME=" + root, "XDG_CONFIG_HOME=" + root, "PATH=" + root + string(os.PathListSeparator) + os.Getenv("PATH"),
			"TZ=UTC", "LC_ALL=C", "GOMAXPROCS=4", "PBRAIN_CACHE=" + filepath.Join(root, "personal-model-cache"),
			"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "PBRAIN_MEASURE_GIT=" + git, "PBRAIN_MEASURE_GIT_LOG=" + logPath,
		}}
	f := readMeasureFixture{dir: filepath.Join(root, "personal"), shared: 4500, personal: 500, hasGit: true}
	f.brain = filepath.Join(f.dir, "docs", "brain")
	onlyID := func(pattern string) string {
		files, err := filepath.Glob(filepath.Join(f.brain, pattern))
		if err != nil || len(files) != 1 {
			t.Fatalf("one fixture file required for %s: %v %v", pattern, files, err)
		}
		return strings.TrimSuffix(filepath.Base(files[0]), ".md")
	}
	f.id = onlyID("personal/decisions/*-personal01f3.md")
	f.sharedID = onlyID("decisions/*-shared1193.md")
	f.semanticID = onlyID("notes/*-shared0000.md")
	model := measureModelFromEnv(t)
	if model == nil {
		t.Fatal("a pinned model path is required")
	}
	return h, f, model
}

func readMeasurementJSON(t *testing.T, path string, into any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatal(err)
	}
}

func timeRetainedViews(t *testing.T, h readMeasureHarness, f readMeasureFixture, model *measureModelOptions) {
	t.Helper()
	shared := f
	shared.id, shared.personal = f.sharedID, 0
	h.timeModelReads(t, shared, model.path, true)
	h.timeModelReads(t, f, model.path, true)
}

func TestMeasureResumeGemma(t *testing.T) {
	h, f, model := retainedFixture(t)
	var paused struct {
		InitialSeconds     float64 `json:"initial_index_wall_seconds"`
		PeakKiB            uint64  `json:"peak_rss_kib"`
		Brain, Cache       string
		CheckpointItems    int `json:"checkpoint_items"`
		CheckpointPassages int `json:"checkpoint_passages"`
	}
	readMeasurementJSON(t, os.Getenv("PBRAIN_PHASE28_PAUSE"), &paused)
	if paused.Brain != f.brain || paused.Cache != filepath.Join(filepath.Dir(f.dir), "personal-model-cache") {
		t.Fatal("resume must use the same retained brain and vector cache")
	}
	var counts map[string]struct{ Passages int }
	readMeasurementJSON(t, os.Getenv("PBRAIN_PHASE28_COUNTS"), &counts)
	status := runMeasureCommand(t, h.bin, f.dir, h.env, f.viewArgs([]string{"index", "--status", "--json"})...)
	state, err := requireMeasuredSections(status, model.name, f.sectionCounts())
	if err != nil {
		t.Fatal(err)
	}
	if !state.ModelPresent || state.Sections[0].Indexed != paused.CheckpointItems || state.Sections[0].Stale || state.Sections[0].Unreadable || state.Sections[1].Indexed != 0 {
		t.Fatalf("the paused checkpoint is unavailable: %s", status)
	}
	t.Logf("resuming the Gemma index with %d shared items and %d passages already published", paused.CheckpointItems, paused.CheckpointPassages)
	got, sample := h.measureCLI(t, f, model.indexTimeout, f.viewArgs([]string{"index", "--json"}))
	if err := indexProgressOnly(got.stderr); err != nil {
		t.Fatal(err)
	}
	built, err := requireMeasuredSections(got.stdout, model.name, f.sectionCounts())
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range built.Sections {
		want := counts[section.Section].Passages
		if section.Section == "shared" {
			want -= paused.CheckpointPassages
		}
		if section.Embedded+section.Reused != want || section.Removed != 0 {
			t.Fatalf("unexpected resumed %s totals: %+v, expected %d remaining passages", section.Section, section, want)
		}
		t.Logf("resumed %s: %d items, %d passages embedded, %d reused", section.Section, section.Items, section.Embedded, section.Reused)
	}
	peak := max(paused.PeakKiB*1024, sample.peakBytes)
	total := paused.InitialSeconds + sample.elapsed.Seconds()
	t.Logf("Gemma index: first portion %.3f s, resumed %.3f s, total active wall %.3f s, %.1f MiB peak RSS", paused.InitialSeconds, sample.elapsed.Seconds(), total, mib(peak))
	summary, err := json.MarshalIndent(map[string]any{"first_seconds": paused.InitialSeconds, "resumed_seconds": sample.elapsed.Seconds(), "total_active_seconds": total, "peak_rss_bytes": peak, "resumed_sections": built.Sections}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("PBRAIN_PHASE28_RESULT"), append(summary, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	h.checkMeasuredIndex(t, f, *model, true)
	timeRetainedViews(t, h, f, model)
	runMeasureCommand(t, h.bin, f.dir, h.env, f.viewArgs([]string{"add", "--author", "agent:readbench", "--title", "One incremental measurement", "--body", "An unindexed item added after the full semantic measurement."})...)
	f.personal++
	h.timeIndex(t, f, *model, true)
	h.checkMeasuredIndex(t, f, *model, true)
}

func TestMeasureRetainedReleaseReads(t *testing.T) {
	h, f, model := retainedFixture(t)
	h.checkMeasuredIndex(t, f, *model, true)
	timeRetainedViews(t, h, f, model)
}
