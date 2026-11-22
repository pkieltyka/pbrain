package bge

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/pkieltyka/pbrain/internal/embed"
)

type fakeBERT struct {
	forward func([]int32) []float32
	close   func() error
}

func (m fakeBERT) Embed(ids []int32) []float32 { return m.forward(ids) }
func (m fakeBERT) Close() error                { return m.close() }

func TestBgeWorkersKeepInputOrder(t *testing.T) {
	previous := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(previous)
	synctest.Test(t, func(t *testing.T) {
		e := tokenTestEmbedder(t)
		second := make(chan struct{})
		e.model = fakeBERT{forward: func(ids []int32) []float32 {
			if len(ids) == 3 {
				<-second
			} else {
				close(second)
			}
			return []float32{float32(len(ids))}
		}}
		vectors, err := e.Embed(t.Context(), embed.Document, []string{"a", "a p"})
		if err != nil || !slices.EqualFunc(vectors, [][]float32{{3}, {4}}, slices.Equal) {
			t.Fatalf("embeddings in input order = %v, %v", vectors, err)
		}
	})
}

func TestBgeWorkersCancelAndJoin(t *testing.T) {
	previous := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(previous)
	synctest.Test(t, func(t *testing.T) {
		e := tokenTestEmbedder(t)
		ctx, cancel := context.WithCancelCause(t.Context())
		defer cancel(nil)
		started := make(chan struct{}, 2)
		release := make(chan struct{})
		var forwards atomic.Int32
		e.model = fakeBERT{
			forward: func([]int32) []float32 {
				forwards.Add(1)
				started <- struct{}{}
				<-release
				return []float32{1}
			},
			close: func() error { return nil },
		}
		type result struct {
			vectors [][]float32
			err     error
		}
		done := make(chan result, 1)
		go func() {
			vectors, err := e.Embed(ctx, embed.Document, []string{"a", "p", "a p"})
			done <- result{vectors, err}
		}()
		<-started
		<-started
		cancel(errors.New("caller cancel cause"))
		synctest.Wait()
		if len(done) != 0 {
			t.Error("Embed returned while forwards were still running")
		}
		close(release)
		got := <-done
		if got.vectors != nil || !errors.Is(got.err, context.Canceled) || forwards.Load() != 2 {
			t.Fatalf("cancelled batch = %v, %v after %d forwards", got.vectors, got.err, forwards.Load())
		}
		if err := e.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestBgeWorkersDiscardResultsOnFailure(t *testing.T) {
	sentinel := errors.New("failed input")
	vectors, err := embedBatch(t.Context(), 2, 2, func(_ context.Context, i int) ([]float32, error) {
		if i == 1 {
			return nil, sentinel
		}
		return []float32{1}, nil
	})
	if vectors != nil || err != sentinel {
		t.Fatalf("failed batch = %v, %v", vectors, err)
	}
}

func TestBgeSingleForwardAndClose(t *testing.T) {
	previous := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(previous)
	e := tokenTestEmbedder(t)
	forwards, closes := 0, 0
	e.model = fakeBERT{
		forward: func([]int32) []float32 {
			forwards++
			return []float32{1}
		},
		close: func() error {
			closes++
			return nil
		},
	}
	vectors, err := e.Embed(t.Context(), embed.Query, []string{"a"})
	if err != nil || !slices.EqualFunc(vectors, [][]float32{{1}}, slices.Equal) || forwards != 1 {
		t.Fatalf("singleton = %v, %v after %d forwards", vectors, err, forwards)
	}
	if err := e.Close(); err != nil || closes != 1 {
		t.Fatalf("Close = %v after %d closes", err, closes)
	}
	if _, err := e.Embed(t.Context(), embed.Document, []string{"a", "p"}); err == nil || forwards != 1 {
		t.Fatalf("Embed after Close = %v after %d forwards", err, forwards)
	}
}
