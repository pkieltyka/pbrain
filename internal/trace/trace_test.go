package trace

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestDisabled(t *testing.T) {
	ctx := context.Background()
	if got := testing.AllocsPerRun(100, func() {
		window := WithWindow(WithSection(ctx, "shared"), 3)
		span := Start(window, "forward")
		span.EndWith(Fields{Inputs: 1})
		if window != ctx || Active(window) || !span.started.IsZero() {
			t.Fatal("disabled tracing changed the context or started a timer")
		}
	}); got != 0 {
		t.Fatalf("disabled tracing allocated: %v", got)
	}
}

func TestConcurrentSpans(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithWindow(WithSection(r.Context(t.Context()), "shared"), 3)
	root := Start(ctx, Sync)
	var workers sync.WaitGroup
	for range 10 {
		workers.Go(func() {
			span := Start(ctx, "forward")
			span.EndWith(Fields{Device: "cuda:0", Inputs: 2, TotalPositions: 61, MaxPositions: 33})
			span.End()
		})
	}
	workers.Wait()
	root.End()
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var sum float64
	for i := range 12 {
		if !scanner.Scan() {
			t.Fatalf("missing row %d: %v", i, scanner.Err())
		}
		var row struct {
			Span    string
			Section string
			Window  int
			Run     int64
			StartMS float64 `json:"start_ms"`
			MS      float64
			Spans   map[string]total
			Fields
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		switch {
		case i < 10:
			want := Fields{Device: "cuda:0", Inputs: 2, TotalPositions: 61, MaxPositions: 33}
			if row.Span != "forward" || row.Section != "shared" || row.Window != 3 || row.Run == 0 ||
				row.StartMS < 0 || row.MS < 0 || row.Fields != want {
				t.Fatalf("bad forward: %+v", row)
			}
			sum += row.MS
		case i == 10:
			if row.Span != Sync {
				t.Fatalf("bad root span: %+v", row)
			}
		default:
			want := total{Calls: 10, MS: sum, Share: sum / row.MS}
			if row.Span != "summary" || row.Spans["forward"] != want {
				t.Fatalf("bad summary: %+v", row)
			}
		}
	}
	if scanner.Scan() || scanner.Err() != nil {
		t.Fatalf("extra trace data: %q %v", scanner.Text(), scanner.Err())
	}
}
