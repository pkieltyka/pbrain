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
	"github.com/pkieltyka/pbrain/internal/embed/embedtest"
)

const (
	measureBGEName       = "bge-small-en-v1.5"
	measureGemmaName     = "embeddinggemma-2"
	measurePartialQuery  = "readbenchrare readbenchsignpost readbenchlease readbenchwork readbenchrenew readbenchabsent"
	measureSemanticBody  = "Login state is kept in the local session store. The session cookie preserves the login state.\n"
	measureSemanticQuery = "where is login state kept"
)

type measureModelOptions struct {
	name, path   string
	indexTimeout time.Duration
}

func measureModelFromEnv(t *testing.T) *measureModelOptions {
	t.Helper()
	name, path, wait := measureBGEName, os.Getenv("PBRAIN_TEST_BGE_MODEL"), 4*time.Hour
	if gemma := os.Getenv("PBRAIN_TEST_GEMMA2_PACK"); gemma != "" {
		embedtest.SkipGemma2(t)
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

	if f.personal > 0 {
		shared := f
		shared.id, shared.personal = f.sharedID, 0
		h.timeModelReads(t, shared, model.path, bm25)
	}
	h.timeModelReads(t, f, model.path, bm25)

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

func (h readMeasureHarness) timeModelReads(t *testing.T, f readMeasureFixture, modelPath string, bm25 bool) {
	t.Helper()
	if f.semanticID == "" {
		t.Fatal("missing semantic control item")
	}
	strict := runMeasureCommand(t, h.bin, f.dir, h.env, f.viewArgs([]string{"search", measureSemanticQuery, "--json"})...)
	if err := checkMeasuredHit(strict, f.semanticID, false); err != nil {
		t.Fatal(err)
	}
	view := "shared"
	if f.personal > 0 {
		view = "personal"
	}
	t.Logf("%s model search: %d visible items", view, f.shared+f.personal)
	rows := []readMeasureCase{{name: "search semantic with exact hit", args: []string{"search", "readbenchrare", "--semantic", "--json"}, want: f.id}}
	rows = append(rows, measuredSemanticRows(f.semanticID, bm25)...)
	h.timeReads(t, f, rows, modelPath)
}

func measuredSemanticRows(id string, bm25 bool) []readMeasureCase {
	rows := []readMeasureCase{{name: "search semantic paraphrase", args: []string{"search", measureSemanticQuery, "--semantic", "--json"}, semanticID: id}}
	if bm25 {
		rows = append(rows, readMeasureCase{name: "search semantic and BM25", args: []string{"search", measureSemanticQuery, "--semantic", "--bm25", "--json"}, semanticID: id})
	}
	return rows
}

func checkMeasuredHit(out []byte, id string, semantic bool) error {
	var page struct {
		Results []struct {
			ID       string `json:"id"`
			Semantic bool   `json:"semantic"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out, &page); err != nil {
		return fmt.Errorf("decode search results: %w", err)
	}
	for _, hit := range page.Results {
		if hit.ID != id {
			continue
		}
		if !semantic {
			return fmt.Errorf("strict search unexpectedly found %s", id)
		}
		if !hit.Semantic {
			return fmt.Errorf("hit %s lacks the semantic marker", id)
		}
		return nil
	}
	if semantic {
		return fmt.Errorf("search did not return semantic hit %s", id)
	}
	return nil
}

func TestMeasureSemanticRowsRejectStrippedFlag(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "wrapper")
	// The stand-in release returns a semantic or lexical hit according to argv.
	// Wrap it by removing the flag on every invocation or on the combined path.
	const wrapper = `#!/bin/sh
combined=false
for arg do
    if [ "$arg" = --bm25 ]; then combined=true; fi
done
for arg do
    shift
    if [ "$arg" = --semantic ] && { [ "$STRIP" = all ] || { [ "$STRIP" = combined ] && "$combined"; }; }; then
        continue
    fi
    set -- "$@" "$arg"
done
exec "$RELEASE" "$@"
`
	const release = `#!/bin/sh
for arg do
    if [ "$arg" = --semantic ]; then
        printf '%s\n' '{"results":[{"id":"control","semantic":true}]}'
        exit 0
    fi
done
printf '%s\n' '{"results":[{"id":"control"}]}'
`
	for name, script := range map[string]string{bin: wrapper, filepath.Join(dir, "release"): release} {
		if err := os.WriteFile(name, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		strip string
		fails [2]bool
	}{
		{"none", [2]bool{false, false}},
		{"all", [2]bool{true, true}},
		{"combined", [2]bool{false, true}},
	} {
		t.Run(tc.strip, func(t *testing.T) {
			h := readMeasureHarness{bin: bin, gitLog: filepath.Join(dir, "git-count"),
				env: []string{"STRIP=" + tc.strip, "RELEASE=" + filepath.Join(dir, "release")}}
			for i, row := range measuredSemanticRows("control", true) {
				_, err := h.measureRead(t, readMeasureFixture{dir: dir}, row, row.args)
				if (err != nil) != tc.fails[i] {
					t.Errorf("%s: got error %v, want failure %t", row.name, err, tc.fails[i])
				} else if err != nil && err.Error() != "hit control lacks the semantic marker" {
					t.Errorf("%s failed for the wrong reason: %v", row.name, err)
				}
			}
		})
	}
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
