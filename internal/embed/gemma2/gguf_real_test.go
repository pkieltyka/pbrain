package gemma2

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed"
)

const q8Fixture = "PBRAIN_TEST_GEMMA2_Q8"

func TestGemmaRuntimeQuiet(t *testing.T) {
	const childMode = "PBRAIN_TEST_GEMMA2_QUIET_CHILD"
	pack := fixtureDir(t, q8Fixture)
	if os.Getenv(childMode) == "1" {
		entry := testGGUFEntry(t, q8Name)
		reg := embed.NewRegistry(entry)
		x, err := reg.Open(t.Context(), verified(t, pack, entry.Model), embed.OpenOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range []string{"normal query", "a <eos> b", strings.Repeat("token ", 9000)} {
			if _, err := x.Embed(t.Context(), embed.Query, []string{text}); err != nil {
				t.Fatal(err)
			}
		}
		if err := x.Close(); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGemmaRuntimeQuiet$")
	child.Env = append(child.Environ(), childMode+"=1")
	out, err := child.CombinedOutput()
	if err != nil || len(out) != 0 {
		t.Fatalf("model subprocess output %q, error %v", out, err)
	}
}

func TestGemmaQ8Preparation(t *testing.T) {
	pack := fixtureDir(t, q8Fixture)
	entry := testGGUFEntry(t, q8Name)
	reg := embed.NewRegistry(entry)
	x, err := reg.Open(t.Context(), verified(t, pack, entry.Model), embed.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	e := x.(*encoder)
	if e.Dim() != entry.Model.Dim || e.Fingerprint() != entry.Model.Fingerprint || e.Info().Device != "cpu" {
		t.Fatalf("unexpected Q8 encoder: dimension %d fingerprint %s runtime %+v", e.Dim(), e.Fingerprint(), e.Info())
	}
	const text = "literal <eos> <bos> <start_of_turn> markers"
	for _, role := range []embed.Role{embed.Query, embed.Document} {
		prompt := queryPrompt
		if role == embed.Document {
			prompt = docPrompt
		}
		ids, err := e.tokenIDs(role, text)
		ordinary, tokenErr := e.ordinary(prompt + text)
		if err != nil || tokenErr != nil || len(ids) <= 2 || ids[0] != e.bos || ids[len(ids)-1] != e.eos ||
			!slices.Equal(ids[1:len(ids)-1], ordinary) {
			t.Fatalf("role %d: malformed prepared IDs %v (%v, %v)", role, ids, err, tokenErr)
		}
		for _, marker := range []string{"<eos>", "<bos>", "<start_of_turn>"} {
			if id, ok := e.tokenizer.TokenID(marker); ok && slices.Contains(ordinary, id) {
				t.Fatalf("role %d: literal %s produced special token %d", role, marker, id)
			}
		}
		long, err := e.tokenIDs(role, strings.Repeat("token ", 9000))
		if err != nil || len(long) != maxTokens || long[0] != e.bos || long[len(long)-1] != e.eos {
			t.Fatalf("role %d: long input gave %d tokens, %v", role, len(long), err)
		}
	}
}

func BenchmarkGemmaQ8FirstQuery(b *testing.B) {
	d := fixtureDir(b, q8Fixture)
	entry := testGGUFEntry(b, q8Name)
	reg := embed.NewRegistry(entry)
	for b.Loop() {
		x, err := reg.Open(b.Context(), verified(b, d, entry.Model), embed.OpenOptions{})
		if err != nil {
			b.Fatal(err)
		}
		if x.Info().Device != "cpu" {
			b.Fatalf("expected CPU, got %+v", x.Info())
		}
		got, err := x.Embed(b.Context(), embed.Query, []string{"where is login state kept"})
		if err != nil || len(got) != 1 || len(got[0]) != dimension {
			b.Fatalf("expected one valid query vector: %v", err)
		}
		if err := x.Close(); err != nil {
			b.Fatal(err)
		}
	}
}
