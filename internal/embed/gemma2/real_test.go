package gemma2

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pkieltyka/goinfer/tokenizer"
	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type reference struct {
	Role, Text     string
	IDs            []int
	Vector         []float32
	PreparedIDs    []int     `json:"prepared_ids"`
	PreparedVector []float32 `json:"prepared_vector"`
}

func references(t *testing.T) []reference {
	t.Helper()
	raw, err := os.ReadFile("testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []reference
	if err = json.Unmarshal(raw, &rows); err != nil || len(rows) != 22 {
		t.Fatalf("reference data: %d rows, error %v", len(rows), err)
	}
	return rows
}
func fixtureDir(t testing.TB, variable string) sys.Dir {
	t.Helper()
	name := os.Getenv(variable)
	if name == "" {
		t.Skipf("set %s to run against the pinned local model", variable)
	}
	name, err := filepath.Abs(name)
	if err != nil {
		t.Fatal(err)
	}
	d, err := sys.OpenDir(name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}
func verified(t testing.TB, d sys.Dir, m embed.Model) *embed.Artifact {
	t.Helper()
	a, err := embed.Verify(sys.Native(), d, m)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func realEncoder(t *testing.T) *encoder {
	t.Helper()
	e := Entry()
	r := embed.NewRegistry(e)
	x, err := r.Open(t.Context(), verified(t, fixtureDir(t, "PBRAIN_TEST_GEMMA2_PACK"), e.Model))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { x.Close() })
	return x.(*encoder)
}

func similarity(a, b []float32) float64 {
	var dot, n1, n2 float64
	for i, w := range a {
		x, y := float64(w), float64(b[i])
		dot += x * y
		n1 += x * x
		n2 += y * y
	}
	return dot / math.Sqrt(n1*n2)
}

func TestGemmaParity(t *testing.T) {
	rows := references(t)
	e := realEncoder(t)
	worst := 1.0
	check := func(got, want []float32) {
		t.Helper()
		if len(got) != 768 || len(want) != 768 {
			t.Fatalf("vector shape %d/%d", len(got), len(want))
		}
		for _, n := range []int{512, 768} {
			c := similarity(got[:n], want[:n])
			if !(c >= 0.999) {
				t.Fatalf("cosine[%d]=%.12f", n, c)
			}
			worst = min(worst, c)
		}
	}
	for i, row := range rows {
		role := embed.Document
		if row.Role == "query" {
			role = embed.Query
		}
		ids, err := e.model.Tokenize(row.Text, row.Role)
		if err != nil || !slices.Equal(ids, row.IDs) {
			t.Fatalf("raw ids %d: %v", i, err)
		}
		raw, err := e.model.Model().Embed(ids)
		if err != nil {
			t.Fatal(err)
		}
		check(raw, row.Vector)
		wantIDs, wantVector := row.PreparedIDs, row.PreparedVector
		if wantIDs == nil {
			wantIDs, wantVector = row.IDs, row.Vector
		}
		ids, err = e.tokenIDs(role, row.Text)
		if err != nil || !slices.Equal(ids, wantIDs) {
			t.Fatalf("prepared ids %d: got %d, want %d: %v", i, len(ids), len(wantIDs), err)
		}
		vec, err := e.Embed(t.Context(), role, []string{row.Text})
		if err != nil {
			t.Fatal(err)
		}
		check(vec[0], wantVector)
	}
	t.Logf("22 raw and 22 prepared texts, 768 and 512 dimensions: minimum cosine %.14f", worst)
}

func TestGemmaSpecialTokensAndCut(t *testing.T) {
	e := realEncoder(t)
	const markers = "<bos> <eos> <start_of_turn>"
	ids, err := e.tokenIDs(embed.Document, markers)
	if err != nil {
		t.Fatal(err)
	}
	if ids[0] != e.bos || ids[len(ids)-1] != e.eos {
		t.Fatal("missing outer marker")
	}
	for _, s := range strings.Fields(markers) {
		if id, ok := e.tokenizer.TokenID(s); ok && slices.Contains(ids[1:len(ids)-1], id) {
			t.Fatalf("literal %s became special id %d", s, id)
		}
	}
	long := strings.Repeat("語の", 9500)
	q, err := e.tokenIDs(embed.Query, long)
	if err != nil || len(q) != maxTokens {
		t.Fatalf("query cap: %d %v", len(q), err)
	}
	tail := "preserve the final whole passage"
	ids, err = e.tokenIDs(embed.Document, long+"\n\n"+tail)
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := e.ordinary(tail)
	if len(ids) != maxTokens || !slices.Equal(ids[len(ids)-1-len(plain):len(ids)-1], plain) {
		t.Fatal("short passage lost behind long prefix")
	}
	// The passage beginning must influence the vector even with the longest prefix.
	prefix := strings.Repeat("語", 400) + "\n\n"
	vec, err := e.Embed(t.Context(), embed.Document, []string{prefix + "red", prefix + "blue"})
	if err != nil || slices.Equal(vec[0], vec[1]) {
		t.Fatalf("no evidence from start of passage: %v", err)
	}
	one, err := e.Embed(t.Context(), embed.Document, []string{prefix + "blue"})
	if err != nil || !slices.Equal(vec[1], one[0]) {
		t.Fatal("batching changed vector")
	}
}

func TestGemmaBuildsFromBytes(t *testing.T) {
	e := Entry()
	source := fixtureDir(t, "PBRAIN_TEST_GEMMA2_PACK")
	// Verify a disposable copy on disk and remove it before calling the adapter.
	temp := t.TempDir()
	for _, f := range e.Model.Files {
		input, err := sys.Native().OpenRegular(source, f.Name)
		if err != nil {
			t.Fatal(err)
		}
		dst, err := os.Create(filepath.Join(temp, f.Name))
		if err != nil {
			input.Close()
			t.Fatal(err)
		}
		_, err = dst.ReadFrom(input)
		input.Close()
		cerr := dst.Close()
		if err != nil || cerr != nil {
			t.Fatalf("copy: %v %v", err, cerr)
		}
	}
	d, err := sys.OpenDir(temp)
	if err != nil {
		t.Fatal(err)
	}
	a := verified(t, d, e.Model)
	d.Close()
	if err := os.RemoveAll(temp); err != nil {
		t.Fatal(err)
	}
	x, err := embed.NewRegistry(e).Open(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	if a.Files != nil || x.Dim() != dimension || x.Fingerprint() != e.Model.Fingerprint {
		t.Fatal("artifact handoff or model identity")
	}
	got, err := x.Embed(t.Context(), embed.Query, []string{"where is login state kept"})
	if err != nil || len(got) != 1 {
		t.Fatalf("embed: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err = x.Embed(ctx, embed.Query, nil); err != context.Canceled {
		t.Fatal(err)
	}
	if err := x.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := x.Embed(t.Context(), embed.Query, nil); err == nil {
		t.Fatal("used closed model")
	}
}

func TestDeriveDeterministic(t *testing.T) {
	e := Entry()
	r := embed.NewRegistry(e)
	files := sys.Native()
	src := fixtureDir(t, "PBRAIN_TEST_GEMMA2_SOURCE")
	for range 2 {
		d, err := sys.OpenDir(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		err = embed.Pack(files, r, e.Model, src, d)
		if err == nil {
			_, err = embed.Verify(files, d, e.Model)
		}
		d.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestGemmaTokenizerBinary(t *testing.T) {
	dir := fixtureDir(t, "PBRAIN_TEST_GEMMA2_SOURCE")
	file, err := sys.Native().OpenRegular(dir, "tokenizer.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	b := make([]byte, sourceFiles[1].Size)
	if _, err := file.ReadAt(b, 0); err != nil {
		t.Fatal(err)
	}
	p, err := tokenizer.BuildPack(b)
	if err != nil || sha256.Sum256(p) != packFiles[2].SHA256 {
		t.Fatalf("tokenizer pack: %v", err)
	}
	start := time.Now()
	tok, err := tokenizer.LoadPack(p)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("tokenizer.LoadPack: %s (release threshold 100 ms)", elapsed)
	bos, _ := tok.TokenID("<bos>")
	eos, _ := tok.TokenID("<eos>")
	for _, row := range references(t) {
		prompt := docPrompt
		if row.Role == "query" {
			prompt = queryPrompt
		}
		ids, err := tok.Encode(prompt+row.Text, false)
		if err != nil || row.IDs[0] != bos || row.IDs[len(row.IDs)-1] != eos || !slices.Equal(ids, row.IDs[1:len(row.IDs)-1]) {
			t.Fatal("pack tokenizer ids differ from reference")
		}
	}
}

func BenchmarkGemmaFirstQuery(b *testing.B) {
	d := fixtureDir(b, "PBRAIN_TEST_GEMMA2_PACK")
	e := Entry()
	r := embed.NewRegistry(e)
	for b.Loop() {
		a := verified(b, d, e.Model)
		x, err := r.Open(b.Context(), a)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := x.Embed(b.Context(), embed.Query, []string{"where is login state kept"}); err != nil {
			b.Fatal(err)
		}
		if err := x.Close(); err != nil {
			b.Fatal(err)
		}
	}
}
