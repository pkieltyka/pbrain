package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	measureBGEName      = "bge-small-en-v1.5"
	measureGemmaName    = "embeddinggemma-2"
	measurePartialQuery = "readbenchrare readbenchsignpost readbenchlease readbenchwork readbenchrenew readbenchabsent"
)

type measureModelOptions struct {
	name, path   string
	indexTimeout time.Duration
}

func measureModelFromEnv(t *testing.T) *measureModelOptions {
	t.Helper()
	name, path, wait := measureBGEName, os.Getenv("PBRAIN_TEST_BGE_MODEL"), 4*time.Hour
	if gemma := os.Getenv("PBRAIN_TEST_GEMMA2_PACK"); gemma != "" {
		if path != "" {
			t.Fatal("select one pinned release model per measurement run")
		}
		name, path, wait = measureGemmaName, gemma, 8*time.Hour
	}
	if path == "" {
		return nil
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		t.Fatal(err)
	}
	if value := os.Getenv("PBRAIN_MEASURE_INDEX_TIMEOUT"); value != "" {
		wait, err = time.ParseDuration(value)
		if err != nil || wait <= 0 {
			t.Fatal("PBRAIN_MEASURE_INDEX_TIMEOUT must be a positive duration")
		}
	}
	return &measureModelOptions{name: name, path: absolute, indexTimeout: wait}
}

func (h readMeasureHarness) measureModel(t *testing.T, f readMeasureFixture, model measureModelOptions, bm25 bool) {
	t.Helper()
	writeMeasureTOML(t, filepath.Join(f.brain, "pbrain.toml"), map[string]map[string]string{
		"semantic": {"model": model.name},
	})
	writeMeasureTOML(t, filepath.Join(f.brain, "personal.toml"), map[string]string{"model_path": model.path})
	// Each view starts with its own cold vector cache. The model is shared
	// from the verified local folder and never downloaded by this harness.
	h.env = append(slices.Clone(h.env), "PBRAIN_CACHE="+f.dir+"-model-cache")
	h.checkMeasuredIndex(t, f, model, false)
	h.chill(t, f, model.path)
	t.Logf("starting first %s index, up to %s", model.name, model.indexTimeout)
	h.timeIndex(t, f, model, false)
	h.checkMeasuredIndex(t, f, model, true)

	rows := []readMeasureCase{
		{"search semantic with exact hit", []string{"search", "readbenchrare", "--semantic", "--json"}, f.id},
		{"search semantic paraphrase", []string{"search", "The worker extends its lease after confirming a retry", "--semantic", "--json"}, ""},
	}
	if bm25 {
		rows = append(rows, readMeasureCase{
			"search semantic and BM25", []string{"search", measurePartialQuery, "--semantic", "--bm25", "--json"}, `"results":`,
		})
	}
	h.timeReads(t, f, rows, model.path)

	// Keep the search measurements at exactly 5,000 items. Add one unique,
	// short note afterwards and time just the rebuild, without chilling.
	runMeasureCommand(t, h.bin, f.dir, h.env, f.viewArgs([]string{
		"add", "--author", "agent:readbench", "--title", "One incremental measurement",
		"--body", "An unindexed item added after the full semantic measurement.",
	})...)
	if f.personal > 0 {
		f.personal++
	} else {
		f.shared++
	}
	h.timeIndex(t, f, model, true)
	h.checkMeasuredIndex(t, f, model, true)
}

func writeMeasureTOML(t *testing.T, path string, config any) {
	t.Helper()
	content, err := toml.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

type measuredSection struct {
	Section    string `json:"section"`
	Items      int    `json:"items"`
	Removed    int    `json:"items_removed"`
	Embedded   int    `json:"passages_embedded"`
	Reused     int    `json:"passages_reused"`
	Indexed    int    `json:"indexed"`
	Stale      bool   `json:"stale"`
	Unreadable bool   `json:"unreadable"`
}

type measuredIndex struct {
	Model        string            `json:"model"`
	ModelPresent bool              `json:"model_present"`
	Sections     []measuredSection `json:"sections"`
}

func (f readMeasureFixture) sectionCounts() map[string]int {
	counts := map[string]int{"shared": f.shared}
	if f.personal > 0 {
		counts["personal"] = f.personal
	}
	return counts
}

func requireMeasuredSections(out []byte, model string, expected map[string]int) (measuredIndex, error) {
	var index measuredIndex
	if err := json.Unmarshal(out, &index); err != nil {
		return index, err
	}
	if index.Model != model || len(index.Sections) != len(expected) {
		return index, fmt.Errorf("wrong model or incomplete sections: %s", out)
	}
	seen := map[string]bool{}
	for _, s := range index.Sections {
		if count, ok := expected[s.Section]; !ok || seen[s.Section] || count != s.Items {
			return index, fmt.Errorf("wrong item counts: %s", out)
		}
		seen[s.Section] = true
	}
	return index, nil
}

func (h readMeasureHarness) checkMeasuredIndex(t *testing.T, f readMeasureFixture, model measureModelOptions, indexed bool) {
	t.Helper()
	got := runMeasured(t, h.bin, f.dir, h.env, model.indexTimeout, f.viewArgs([]string{"index", "--status", "--json"})...)
	if len(got.stderr) > 0 {
		t.Fatalf("unexpected index status diagnostics: %s", got.stderr)
	}
	state, err := requireMeasuredSections(got.stdout, model.name, f.sectionCounts())
	if err != nil {
		t.Fatal(err)
	}
	if !state.ModelPresent {
		t.Fatalf("binary has no available %s at the pinned path: %s", model.name, got.stdout)
	}
	for _, s := range state.Sections {
		want := 0
		if indexed {
			want = s.Items
		}
		if s.Indexed != want || s.Stale || s.Unreadable {
			t.Fatalf("expected %d indexed items for %s: %s", want, s.Section, got.stdout)
		}
	}
}

func indexProgressOnly(stderr []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(stderr))
	for {
		var line struct {
			Notice struct{ Code string } `json:"notice"`
		}
		if err := decoder.Decode(&line); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if line.Notice.Code != "index_progress" {
			return fmt.Errorf("unexpected index diagnostic %q", line.Notice.Code)
		}
	}
}

func (h readMeasureHarness) timeIndex(t *testing.T, f readMeasureFixture, model measureModelOptions, incremental bool) {
	t.Helper()
	got, sample := h.measureCLI(t, f, model.indexTimeout, f.viewArgs([]string{"index", "--json"}))
	if err := indexProgressOnly(got.stderr); err != nil {
		t.Fatalf("measured index: %v", err)
	}
	if sample.peakBytes == 0 || sample.git == 0 {
		t.Fatal("index sample lacks peak RSS or Git process accounting")
	}
	built, err := requireMeasuredSections(got.stdout, model.name, f.sectionCounts())
	if err != nil {
		t.Fatal(err)
	}
	phase, changed := "first index", "shared"
	if f.personal > 0 {
		changed = "personal"
	}
	if incremental {
		phase = "index after one note"
	}
	for _, s := range built.Sections {
		if err := checkMeasuredBuild(s, incremental, changed); err != nil {
			t.Fatalf("%s: %v", phase, err)
		}
		t.Logf("%s %s: %d items, %d passages embedded, %d reused", phase, s.Section, s.Items, s.Embedded, s.Reused)
	}
	t.Logf("%s | %.1f ms | %.1f MiB peak RSS | %d Git calls", phase, millis(sample.elapsed), mib(sample.peakBytes), sample.git)
}

func checkMeasuredBuild(s measuredSection, incremental bool, changedSection string) error {
	if s.Removed != 0 {
		return fmt.Errorf("%s unexpectedly removed %d items", s.Section, s.Removed)
	}
	if !incremental {
		if s.Embedded == 0 {
			return fmt.Errorf("%s first build embedded no passages", s.Section)
		}
		return nil
	}
	want := 0
	if s.Section == changedSection {
		want = 1
	}
	if s.Embedded != want || s.Reused != 0 {
		return fmt.Errorf("%s incremental build embedded %d and reused %d; expected %d new and no reused passages", s.Section, s.Embedded, s.Reused, want)
	}
	return nil
}
