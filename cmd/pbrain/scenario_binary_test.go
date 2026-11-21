package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/cli"
)

// A library writing through the process globals after start-up must not write
// through the original descriptors reserved for the command.
func strayCLI(_ context.Context, e cli.Env) int {
	fmt.Printf("stray stdout\n")
	fmt.Fprintln(os.Stderr, "stray stderr")
	fmt.Fprintln(e.Stdout, "command stdout")
	fmt.Fprintln(e.Stderr, "command stderr")
	return 0
}
func TestStrayOutputDiscarded(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), exe)
	cmd.Args[0] = "pbrain-stray"
	var out, diag bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diag
	if err := cmd.Run(); err != nil || out.String() != "command stdout\n" || diag.String() != "command stderr\n" {
		t.Fatalf("exit %v, stdout %q, stderr %q", err, &out, &diag)
	}
}

// Release hosts can supply the exact artifact and an already verified offline
// model folder; ordinary tests build the native executable and exercise its
// streams and its offline, not-ready path without fetching anything.
func TestReleaseBinaryOutput(t *testing.T) {
	bin := os.Getenv("PBRAIN_SCENARIO_BINARY")
	if bin == "" {
		bin = filepath.Join(t.TempDir(), "pbrain")
		cmd := exec.CommandContext(t.Context(), "go", "build", "-p", "2", "-o", bin, ".")
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build: %v: %s", err, out)
		}
	}
	dir := t.TempDir()
	git := exec.CommandContext(t.Context(), "git", "init", "-q", dir)
	if out, err := git.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	cache := t.TempDir()
	run := func(exit int, input string, args ...string) (string, string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), bin, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "HOME="+dir, "PBRAIN_DIR=", "PBRAIN_CACHE="+cache, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		var out, diag bytes.Buffer
		cmd.Stdout, cmd.Stderr, cmd.Stdin = &out, &diag, strings.NewReader(input)
		err := cmd.Run()
		if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != exit {
			t.Fatalf("%v: %v\n%s%s", args, err, &out, &diag)
		}
		if !strings.HasSuffix(out.String(), "\n") || strings.Count(out.String(), "\n") != 1 || !json.Valid(out.Bytes()) {
			t.Fatalf("%v mixed stdout %q", args, &out)
		}
		return out.String(), diag.String()
	}
	clean := func(input string, args ...string) string {
		t.Helper()
		out, diag := run(0, input, args...)
		if diag != "" {
			t.Fatalf("%v mixed stderr %q", args, diag)
		}
		return out
	}
	clean("", "--version", "--json")
	clean("", "init", "--json")
	var paragraphs strings.Builder
	for i := 0; i < 24; i++ {
		fmt.Fprintf(&paragraphs, "# Session record %d\n\nRedis stores this numbered session record.\n\n", i)
	}
	source := paragraphs.String()
	var row struct{ ID string }
	if err := json.Unmarshal([]byte(clean(source, "add", "--json", "--title", "session database", "--author", "human:tester")), &row); err != nil || row.ID == "" {
		t.Fatalf("add: %v %+v", err, row)
	}
	var hit struct{ Results []struct{ ID string } }
	if err := json.Unmarshal([]byte(clean("", "search", "Redis", "--json")), &hit); err != nil || len(hit.Results) != 1 || hit.Results[0].ID != row.ID {
		t.Fatalf("stdin body was not indexed lexically: %+v", hit)
	}
	preview, diag := run(1, "", "delete", row.ID, "--json")
	var deletion struct {
		ID      string
		Deleted bool
	}
	if err := json.Unmarshal([]byte(preview), &deletion); err != nil || deletion.ID != row.ID || deletion.Deleted {
		t.Fatalf("delete preview stdout: %s", preview)
	}
	if !json.Valid([]byte(diag)) || strings.Count(diag, "\n") != 1 || !strings.Contains(diag, `"code":"confirm_required"`) {
		t.Fatalf("delete stderr: %q", diag)
	}
	model := os.Getenv("PBRAIN_SCENARIO_MODEL_PATH")
	if model == "" {
		_, diag = run(0, "", "search", "Redis", "--semantic", "--json")
		if strings.Count(diag, "\n") != 1 || !strings.Contains(diag, `"code":"semantic_not_ready"`) {
			t.Fatalf("search stderr: %q", diag)
		}
		t.Log("release-model indexing requires PBRAIN_SCENARIO_MODEL_PATH")
		return
	}
	// A supplied model_path is the only source for the optional release checks.
	modelName := os.Getenv("PBRAIN_SCENARIO_MODEL")
	if modelName == "" {
		modelName = brain.DefaultModel
	}
	if modelName != "embeddinggemma-2" && modelName != "bge-small-en-v1.5" {
		t.Fatalf("unknown release model %q", modelName)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs/brain/pbrain.toml"), []byte(fmt.Sprintf("semantic.model = %q\n", modelName)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs/brain/personal.toml"), []byte(fmt.Sprintf("model_path = %q\n", model)), 0600); err != nil {
		t.Fatal(err)
	}
	out, diag := run(0, "", "sync", "--json")
	var build struct {
		Sections []struct {
			Passages int `json:"passages_embedded"`
		}
	}
	if err := json.Unmarshal([]byte(out), &build); err != nil || len(build.Sections) != 1 || build.Sections[0].Passages <= 8 {
		t.Fatalf("index: %s", out)
	}
	for _, line := range strings.Split(strings.TrimSuffix(diag, "\n"), "\n") {
		if line != "" && (!json.Valid([]byte(line)) || !strings.Contains(line, `"code":"index_progress"`)) {
			t.Fatalf("index stderr: %q", line)
		}
	}
	clean("", "search", "Redis", "--semantic", "--json")
	testCalibrationOutput(t, modelName, model)
}
func testCalibrationOutput(t *testing.T, modelName, modelPath string) {
	t.Helper()
	tool := os.Getenv("PBRAIN_SCENARIO_CALIBRATE")
	if tool == "" {
		tool = filepath.Join(t.TempDir(), "calibrate")
		build := exec.CommandContext(t.Context(), "go", "build", "-p", "2", "-o", tool, "../../internal/semantic/calibrate")
		build.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("calibration build: %v: %s", err, out)
		}
	}
	cmd := exec.CommandContext(t.Context(), tool, "-model", modelName, "-model-path", modelPath, "../../internal/semantic/calibrate/testdata/retrieval")
	var out, diag bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diag
	if err := cmd.Run(); err != nil || diag.Len() != 0 {
		t.Fatalf("calibration: %v: %s", err, &diag)
	}
	var report struct {
		Model   json.RawMessage
		Partial json.RawMessage `json:"partial_gate"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || len(report.Model) == 0 || len(report.Partial) == 0 {
		t.Fatalf("calibration stream did not contain one complete report: %v", err)
	}
}
