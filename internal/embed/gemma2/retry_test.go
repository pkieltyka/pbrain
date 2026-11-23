package gemma2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/trace"
)

type retryAccel struct {
	fakeBatchAccel
	attempts [][][]int
	reject   func([][]int) error
}

func (a *retryAccel) attempt(ids [][]int) error {
	a.attempts = append(a.attempts, slices.Clone(ids))
	if a.reject != nil {
		return a.reject(ids)
	}
	return nil
}

func (a *retryAccel) Forward(ids []int, layers bool) ([]float32, [][]float32, error) {
	if err := a.attempt([][]int{ids}); err != nil {
		return nil, nil, err
	}
	return a.fakeBatchAccel.Forward(ids, layers)
}

func (a *retryAccel) ForwardBatch(ids [][]int) ([][]float32, error) {
	if err := a.attempt(ids); err != nil {
		return nil, err
	}
	return a.fakeBatchAccel.ForwardBatch(ids)
}

func retryEncoder(t *testing.T, auto bool) (*encoder, *retryAccel) {
	t.Helper()
	e, m := tinyEncoderForTest(t)
	if auto {
		var err error
		e.cpu, err = embeddinggemma2.NewEncoder(m.Model(), m.Tokenizer(), map[string]string{
			"query": queryPrompt, "document": docPrompt,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	fake := &retryAccel{fakeBatchAccel: fakeBatchAccel{
		fakeSingleAccel: fakeSingleAccel{name: t.Name(), dim: m.HiddenDim()},
	}}
	embeddinggemma2.RegisterAccelerator(fake.name, func(*embeddinggemma2.Model) (embeddinggemma2.Accelerator, error) {
		return fake, nil
	})
	if _, err := m.UseAccelerator(fake.name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	e.info = embed.RunInfo{Device: "cuda:0", Budget: gib}
	return e, fake
}

func TestGemmaOOMSplit(t *testing.T) {
	e, fake := retryEncoder(t, false)
	lengths := []int{4, 3, 5, 4, 2}
	ids := make([][]int, len(lengths))
	single := make([][]float32, len(lengths))
	for i, n := range lengths {
		ids[i] = slices.Repeat([]int{100 + i}, n)
		v, err := e.embedIDs(t.Context(), ids[i])
		if err != nil {
			t.Fatal(err)
		}
		single[i] = v
	}
	fake.attempts = nil
	oom := fmt.Errorf("forward allocation: %w", embeddinggemma2.ErrBatchOutOfMemory)
	fake.reject = func(group [][]int) error {
		if len(group) == 5 || len(group) == 2 && group[0][0] == ids[2][0] {
			return oom
		}
		return nil
	}
	out := make([][]float32, len(ids))
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	recording, err := trace.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer recording.Close()
	if err := e.embedPrepared(recording.Context(t.Context()), ids, lengths, out); err != nil {
		t.Fatal(err)
	}
	if err := recording.Close(); err != nil {
		t.Fatal(err)
	}
	groups := [][][]int{ids, ids[:2], ids[2:4], ids[2:3], ids[3:4], ids[4:]}
	if len(fake.attempts) != len(groups) {
		t.Fatalf("attempted %d groups, want %d", len(fake.attempts), len(groups))
	}
	for i, want := range groups {
		if !slices.EqualFunc(fake.attempts[i], want, slices.Equal) {
			t.Fatalf("attempt %d = %v, want %v", i, fake.attempts[i], want)
		}
	}
	if got := tracedForwards(t, path); !slices.Equal(got, []trace.Fields{
		{Device: "cuda:0", Inputs: 5, TotalPositions: 18, MaxPositions: 5, Failed: true},
		{Device: "cuda:0", Inputs: 2, TotalPositions: 7, MaxPositions: 4},
		{Device: "cuda:0", Inputs: 2, TotalPositions: 9, MaxPositions: 5, Failed: true},
		{Device: "cuda:0", Inputs: 1, TotalPositions: 5, MaxPositions: 5},
		{Device: "cuda:0", Inputs: 1, TotalPositions: 4, MaxPositions: 4},
		{Device: "cuda:0", Inputs: 1, TotalPositions: 2, MaxPositions: 2},
	}) {
		t.Fatalf("forward trace did not record actual attempts: %+v", got)
	}
	for i, want := range single {
		if !sameVectorBits(out[i], want) {
			t.Fatalf("passage %d differs from the singleton vector", i)
		}
	}
	if e.retryLimit == nil || *e.retryLimit != 4 || fake.closed != 0 ||
		e.model.Accelerator() != fake {
		t.Fatal("recovery replaced the encoder or lost the reduced budget")
	}
	fake.attempts = nil
	next := [][]int{{105, 105}, {106, 106}, {107, 107}}
	if err := e.embedPrepared(t.Context(), next, []int{2, 2, 2}, make([][]float32, len(next))); err != nil {
		t.Fatal(err)
	}
	if len(fake.attempts) != 2 || len(fake.attempts[0]) != 2 || len(fake.attempts[1]) != 1 {
		t.Fatalf("next window did not retain the reduced cap: %v", fake.attempts)
	}
}

func tracedForwards(t *testing.T, path string) []trace.Fields {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields []trace.Fields
	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		var event struct {
			Span string
			trace.Fields
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.Span == "forward" {
			fields = append(fields, event.Fields)
		}
	}
	return fields
}

func TestGemmaOOMTerminalAndCPUFallback(t *testing.T) {
	oom := fmt.Errorf("out of memory: %w", embeddinggemma2.ErrBatchOutOfMemory)
	generic := fmt.Errorf("out of memory: %w", embeddinggemma2.ErrDeviceOutOfMemory)
	lost := errors.Join(embeddinggemma2.ErrDeviceLost, oom)
	closeErr := errors.New("could not close the GPU")
	for _, test := range []struct {
		name     string
		auto     bool
		forward  error
		closeErr error
		wantErr  error
	}{
		{name: "auto certified singleton", auto: true, forward: oom},
		{name: "explicit certified singleton", forward: oom, wantErr: oom},
		{name: "generic OOM", auto: true, forward: generic, wantErr: generic},
		{name: "untrusted error text", auto: true, forward: errors.New("out of memory"), wantErr: embed.ErrAccelerator},
		{name: "device lost takes precedence", auto: true, forward: lost, wantErr: embed.ErrDeviceLost},
		{name: "failed drain stays on GPU for cleanup", auto: true, forward: oom, closeErr: closeErr, wantErr: closeErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			e, fake := retryEncoder(t, test.auto)
			fake.closeErrs = []error{test.closeErr}
			fake.reject = func([][]int) error { return test.forward }
			texts := []string{"alpha", "beta", "gamma"}
			got, err := e.Embed(t.Context(), embed.Query, texts)
			if test.wantErr != nil {
				if got != nil || !errors.Is(err, test.wantErr) || !errors.Is(err, test.forward) ||
					len(fake.attempts) != 1 || e.model.Accelerator() != fake || e.Info().Device != "cuda:0" {
					t.Fatalf("terminal forward = %v, %v, attempts=%d, info=%+v", got, err, len(fake.attempts), e.Info())
				}
				if test.closeErr != nil && fake.closed != 1 {
					t.Fatalf("failed drain was attempted %d times", fake.closed)
				}
				return
			}
			wantInfo := embed.RunInfo{Device: "cpu", Fallback: embed.FallbackMemory}
			if err != nil || len(got) != len(texts) || e.Info() != wantInfo ||
				len(fake.attempts) != 1 || fake.closed != 1 || e.Window() != batchInputs {
				t.Fatalf("CPU fallback = %v, %v, attempts=%d, closes=%d, info=%+v", got, err, len(fake.attempts), fake.closed, e.Info())
			}
			for i, text := range texts {
				ids, err := e.tokenIDs(embed.Query, text)
				if err != nil {
					t.Fatal(err)
				}
				want, err := e.cpu.EmbedIDs(ids)
				if err != nil || !sameVectorBits(want, got[i]) {
					t.Fatalf("CPU fallback vector %d: %v", i, err)
				}
			}
		})
	}
}

func TestGemmaOOMCanceledChild(t *testing.T) {
	e, fake := retryEncoder(t, false)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	fake.reject = func([][]int) error {
		if len(fake.attempts) == 1 {
			return embeddinggemma2.ErrBatchOutOfMemory
		}
		return nil
	}
	fake.onForward, fake.onBatch = cancel, cancel
	if out, err := e.Embed(ctx, embed.Document, []string{"one", "two", "three", "four"}); out != nil ||
		!errors.Is(err, context.Canceled) || len(fake.attempts) != 2 {
		t.Fatalf("canceled child: %v, %v, attempts=%d", out, err, len(fake.attempts))
	}
}
