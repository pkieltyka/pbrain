package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncTrace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	control, _ := semanticBrain(t)
	test, kit := semanticBrain(t)
	getenv := test.env.Getenv
	test.env.Getenv = func(k string) string {
		if k == "PBRAIN_TRACE" {
			return path
		}
		return getenv(k)
	}

	controlCode, controlOut, controlErr := control.run("sync", "--json")
	code, out, stderr := test.run("sync", "--json")
	if controlCode != 0 || code != controlCode || out != controlOut || stderr != controlErr {
		t.Fatalf("trace changed sync output: %d %q %q vs %d %q %q", code, out, stderr, controlCode, controlOut, controlErr)
	}
	if a, b := traceDB(t, control.dir), traceDB(t, test.dir); !bytes.Equal(a, b) {
		t.Fatal("trace changed the published database")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(raw), []byte{0x0a})
	var summary struct {
		Span     string
		MS       float64
		Opens    int `json:"model_opens"`
		Attaches int `json:"accelerator_attaches"`
		Spans    map[string]struct {
			Calls int
			MS    float64
			Share float64
		}
	}
	if err := json.Unmarshal(lines[len(lines)-1], &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Span != "summary" || summary.Opens != 1 || kit.Opens.Load() != 1 || summary.Attaches != 0 {
		t.Fatalf("unexpected summary: %+v, actual opens %d", summary, kit.Opens.Load())
	}
	for name, count := range map[string]int{
		"sync": 1, "brain_scan_read": 2, "chunk": 2, "model_verify": 1, "model_open": 1,
		"cache_lookup": 2, "cache_write": 2, "sqlite_write": 2, "sqlite_commit": 2, "sqlite_checkpoint": 1, "publish": 1,
	} {
		tot := summary.Spans[name]
		if tot.Calls != count || tot.MS < 0 || tot.Share != tot.MS/summary.MS {
			t.Errorf("invalid %s total: %+v", name, tot)
		}
	}
	for _, args := range [][]string{{"sync", "--status"}, {"sync", "--help"}, {"search", "alpha", "--semantic"}} {
		if code, _, stderr := test.run(args...); code != 0 {
			t.Fatalf("%v: %d %s", args, code, stderr)
		}
	}
	if after, err := os.ReadFile(path); err != nil || !bytes.Equal(raw, after) {
		t.Fatalf("commands outside sync changed the trace: %v", err)
	}
	if code, _, stderr := test.run("sync"); code != 0 {
		t.Fatalf("no-op sync: %d %s", code, stderr)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.HasPrefix(after, raw) {
		t.Fatalf("second sync did not append: %v", err)
	}
	lines = bytes.Split(bytes.TrimSpace(after[len(raw):]), []byte{0x0a})
	if err := json.Unmarshal(lines[len(lines)-1], &summary); err != nil || summary.Span != "summary" || summary.Opens != 0 {
		t.Fatalf("no-op sync summary: %+v, %v", summary, err)
	}
}

func traceDB(t *testing.T, brain string) []byte {
	t.Helper()
	index := filepath.Join(brain, ".pbrain", "index")
	raw, err := os.ReadFile(filepath.Join(index, "manifest"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct{ DB string }
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(index, manifest.DB))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
