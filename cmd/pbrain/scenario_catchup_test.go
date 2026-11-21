package main

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/sys"
	"github.com/rogpeppe/go-internal/testscript"
)

// heldLocks are the locks holdlock took, by folder. The test process holds them, so to a
// pbrain-test child they are held by another process.
var heldLocks sync.Map

// holdlock DIR takes DIR/lock until releaselock DIR or the end of the script.
func scenarioHoldLock(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 1 {
		ts.Fatalf("usage: holdlock DIR")
	}
	path := ts.MkAbs(args[0])
	d, err := sys.OpenDir(path)
	ts.Check(err)
	defer d.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	l, err := sys.Native().Lock(ctx, d, "lock")
	ts.Check(err)
	heldLocks.Store(path, l)
	ts.Defer(func() { releaseHeld(path) })
}

// releaselock DIR releases what holdlock DIR took.
func scenarioReleaseLock(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 1 {
		ts.Fatalf("usage: releaselock DIR")
	}
	if !releaseHeld(ts.MkAbs(args[0])) {
		ts.Fatalf("no lock held on %s", args[0])
	}
}

func releaseHeld(path string) bool {
	v, ok := heldLocks.LoadAndDelete(path)
	if ok {
		v.(io.Closer).Close()
	}
	return ok
}

// slowRegistry delays every document embedding call, so that a script can run a search's
// catch-up out of time (PBRAIN_TEST_EMBED_DELAY).
type slowRegistry struct {
	embed.Registry
	delay time.Duration
}

type slowEmbedder struct {
	embed.Embedder
	delay time.Duration
}

func (r slowRegistry) Open(ctx context.Context, a *embed.Artifact) (embed.Embedder, error) {
	e, err := r.Registry.Open(ctx, a)
	if err != nil {
		return nil, err
	}
	return slowEmbedder{e, r.delay}, nil
}

func (e slowEmbedder) Embed(ctx context.Context, role embed.Role, texts []string) ([][]float32, error) {
	if role == embed.Document {
		time.Sleep(e.delay)
	}
	return e.Embedder.Embed(ctx, role, texts)
}
