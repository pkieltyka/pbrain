//go:build cuda

package gemma2

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/trace"
)

// TestTraceSlice is built by slice.sh as an overlay on an existing test.
func TestTraceSlice(t *testing.T) {
	raw, err := os.ReadFile(os.Getenv("PBRAIN_SLICE"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Text      string
		Positions int
	}
	if err := json.Unmarshal(raw, &rows); err != nil || len(rows) < 200 {
		t.Fatalf("expected at least 200 passages: %d, %v", len(rows), err)
	}
	rows = rows[:200]
	entry := Entry()
	switch os.Getenv("PBRAIN_SLICE_ARM") {
	case "google":
	case "q8":
		entry = GGUFEntries()[0]
	default:
		t.Fatal("unknown slice arm")
	}

	r, err := trace.Open(os.Getenv("PBRAIN_TRACE"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	}()
	ctx := r.Context(t.Context())
	total := trace.Start(ctx, "slice")
	defer total.End()
	pack := fixtureDir(t, "PBRAIN_TEST_GEMMA2_PACK")
	verify := trace.Start(ctx, "model_verify")
	artifact := verified(t, pack, entry.Model)
	verify.End()
	open := trace.Start(ctx, trace.ModelOpen)
	emb, err := embed.NewRegistry(entry).Open(ctx, artifact, embed.OpenOptions{Device: os.Getenv("PBRAIN_SLICE_DEVICE")})
	open.End()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := emb.Close(); err != nil {
			t.Error(err)
		}
	}()
	e := emb.(*encoder)
	if e.Info().Device != "cuda:0" || e.Info().Budget != 6<<30 {
		t.Fatalf("expected CUDA with the default 6 GiB budget, got %+v", e.Info())
	}
	texts := make([]string, len(rows))
	for i, row := range rows {
		texts[i] = row.Text
	}
	window := trace.Start(ctx, "slice_embed")
	vectors, err := e.Embed(trace.WithWindow(ctx, 1), embed.Document, texts)
	window.End()
	total.End()
	if err != nil || len(vectors) != len(rows) {
		t.Fatalf("embedded %d of %d: %v", len(vectors), len(rows), err)
	}

	// Export the exact pbrain-prepared IDs and ordered default-budget groups
	// after the timer, for replay without pbrain.
	ids := make([][]int, len(rows))
	lengths := make([]int, len(rows))
	positions := 0
	for i, row := range rows {
		ids[i], err = e.tokenIDs(embed.Document, row.Text)
		if err != nil || len(ids[i]) != row.Positions {
			t.Fatalf("input %d: %d IDs, expected %d: %v", i, len(ids[i]), row.Positions, err)
		}
		lengths[i] = len(ids[i])
		positions += lengths[i]
	}
	if positions != 100066 {
		t.Fatalf("expected 100066 positions, got %d", positions)
	}
	f, err := os.OpenFile(os.Getenv("PBRAIN_SLICE_IDS"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	fmt.Fprintln(w, "batch\tids")
	batch := 0
	for len(ids) > 0 {
		batch++
		n, err := batchSize(ctx, lengths, e.Info(), nil, e.model.BatchMemory)
		if err != nil {
			t.Fatal(err)
		}
		for _, input := range ids[:n] {
			fmt.Fprintf(w, "%d\t", batch)
			for i, id := range input {
				if i > 0 {
					w.WriteByte(' ')
				}
				w.WriteString(strconv.Itoa(id))
			}
			w.WriteByte('\n')
		}
		ids, lengths = ids[n:], lengths[n:]
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
}
