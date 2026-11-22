package bge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/sys"
)

const modelPathEnv = "PBRAIN_TEST_BGE_MODEL"

func TestBgeParity(t *testing.T) {
	data, err := os.ReadFile("testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	type reference struct {
		Role   string
		Text   string
		IDs    []int32
		Vector []float32
	}
	var cases []reference
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 22 {
		t.Fatalf("got %d references, want 22", len(cases))
	}
	e := realEmbedder(t)
	var queries, docs []reference
	for _, test := range cases {
		switch test.Role {
		case "query":
			queries = append(queries, test)
		case "document":
			docs = append(docs, test)
		default:
			t.Fatalf("unknown reference role %q", test.Role)
		}
	}
	for _, group := range []struct {
		role embed.Role
		rows []reference
		name string
	}{
		{embed.Document, docs, "document"},
		{embed.Query, queries, "query"},
	} {
		offset := 0
		worst := 1.0
		for batch := range slices.Chunk(group.rows, 8) {
			texts := make([]string, len(batch))
			for i, test := range batch {
				texts[i] = test.Text
			}
			vectors, err := e.Embed(t.Context(), group.role, texts)
			if err != nil || len(vectors) != len(batch) {
				t.Fatalf("%s batch: got %d vectors, error %v", group.name, len(vectors), err)
			}
			for i, test := range batch {
				if ids := e.tokenIDs(group.role, test.Text); !slices.Equal(ids, test.IDs) {
					t.Errorf("%s %d: token ids differ from reference", group.name, offset+i)
				}
				got := vectors[i]
				checkUnit(t, got, e.Dim())
				cos := cosine(got, test.Vector)
				if !(cos >= 0.999) {
					t.Errorf("%s %d: reference cosine %g", group.name, offset+i, cos)
				}
				worst = min(worst, cos)
				single, err := e.Embed(t.Context(), group.role, []string{test.Text})
				if err != nil || len(single) != 1 || !slices.Equal(single[0], got) {
					t.Errorf("%s %d: batch changed embedding, error %v", group.name, offset+i, err)
				}
			}
			offset += len(batch)
		}
		t.Logf("%s: %d references, lowest cosine %.12f", group.name, offset, worst)
	}
}

func TestBgeWorkersSameBytes(t *testing.T) {
	e := realEmbedder(t)
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)
	texts := make([]string, 64)
	for i := range texts {
		texts[i] = fmt.Sprintf("Note %d\n\n%s", i, strings.Repeat("A decision about café storage and 中文. ", i%31+1))
	}
	baseline, err := e.Embed(t.Context(), embed.Document, texts)
	if err != nil {
		t.Fatal(err)
	}
	sameBits := func(a, b float32) bool { return math.Float32bits(a) == math.Float32bits(b) }
	for _, workers := range []int{4, 32} {
		runtime.GOMAXPROCS(workers)
		got, err := e.Embed(t.Context(), embed.Document, texts)
		if err != nil || len(got) != len(baseline) {
			t.Fatalf("%d workers: got %d vectors, error %v", workers, len(got), err)
		}
		for i, vector := range got {
			if !slices.EqualFunc(vector, baseline[i], sameBits) {
				t.Fatalf("%d workers changed vector bytes for text %d", workers, i)
			}
		}
	}
}

func TestAdapterContract(t *testing.T) {
	e := realEmbedder(t)
	row := Entry().Model
	if e.Dim() != 384 || e.Dim() != row.Dim || e.Fingerprint() != row.Fingerprint || len(row.Fingerprint) != 64 {
		t.Fatal("model row and runtime disagree on identity")
	}
	prefix := strings.Repeat("猫", 400) + "\n\n"
	middle := strings.Repeat("word ", 300)
	texts := []string{
		prefix + "apple " + middle + "branch",
		prefix + "stone " + middle + "branch",
		prefix + "apple " + middle + "leaf",
	}
	vs, err := e.Embed(t.Context(), embed.Document, texts)
	if err != nil || len(vs) != len(texts) {
		t.Fatalf("long documents: %v", err)
	}
	if slices.Equal(vs[0], vs[1]) || !slices.Equal(vs[0], vs[2]) {
		t.Fatal("the passage start must affect the vector and the cut tail must not")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if vs, err := e.Embed(ctx, embed.Query, []string{"cancelled"}); vs != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled embedding: %v, %v", vs, err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Embed(t.Context(), embed.Query, []string{"closed"}); err == nil {
		t.Fatal("embedding after Close succeeded")
	}
}

func TestBgeOpensAfterModelFilesAreRemoved(t *testing.T) {
	src := modelFolder(t)
	dst := filepath.Join(t.TempDir(), "model")
	if err := os.Mkdir(dst, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, f := range modelFiles {
		copyModelFile(t, filepath.Join(src, f.name), filepath.Join(dst, f.name))
	}
	entry := Entry()
	a := verifiedModel(t, dst, entry.Model)
	if err := os.RemoveAll(dst); err != nil {
		t.Fatal(err)
	}
	e, err := embed.NewRegistry(entry).Open(t.Context(), a)
	if err != nil || a.Files != nil {
		t.Fatalf("Open after removing files: %v", err)
	}
	defer e.Close()
	got, err := e.Embed(t.Context(), embed.Query, []string{"model loaded from memory"})
	if err != nil || len(got) != 1 {
		t.Fatalf("Embed after removing files: %v", err)
	}
	checkUnit(t, got[0], e.Dim())
}

func TestRuntimeQuiet(t *testing.T) {
	if os.Getenv("PBRAIN_TEST_BGE_QUIET_CHILD") == "1" {
		e := realEmbedder(t)
		for _, text := range []string{"normal query", "a [SEP] b", strings.Repeat("long ", 600)} {
			if _, err := e.Embed(t.Context(), embed.Query, []string{text}); err != nil {
				t.Fatal(err)
			}
		}
		if err := e.Close(); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	}
	modelFolder(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRuntimeQuiet$")
	child.Env = append(os.Environ(), "PBRAIN_TEST_BGE_QUIET_CHILD=1")
	out, err := child.CombinedOutput()
	if err != nil || len(out) != 0 {
		t.Fatalf("model subprocess output %q, error %v", out, err)
	}
}

func BenchmarkBgeFirstQuery(b *testing.B) {
	path := modelFolder(b)
	entry := Entry()
	registry := embed.NewRegistry(entry)
	for b.Loop() {
		a := verifiedModel(b, path, entry.Model)
		e, err := registry.Open(b.Context(), a)
		if err != nil {
			b.Fatal(err)
		}
		_, embedErr := e.Embed(b.Context(), embed.Query, []string{"which conditions are needed to rebuild the index"})
		if err := errors.Join(embedErr, e.Close()); err != nil {
			b.Fatal(err)
		}
	}
}

func modelFolder(t testing.TB) string {
	t.Helper()
	path := os.Getenv(modelPathEnv)
	if path == "" {
		t.Skipf("set %s to the folder containing the five verified bge model files", modelPathEnv)
	}
	return path
}

func realEmbedder(t *testing.T) *embedder {
	t.Helper()
	entry := Entry()
	a := verifiedModel(t, modelFolder(t), entry.Model)
	e, err := embed.NewRegistry(entry).Open(t.Context(), a)
	if err != nil || a.Files != nil {
		t.Fatalf("Open model: %v", err)
	}
	t.Cleanup(func() {
		if err := e.Close(); err != nil {
			t.Error(err)
		}
	})
	return e.(*embedder)
}

func verifiedModel(t testing.TB, path string, model embed.Model) *embed.Artifact {
	t.Helper()
	files := sys.Native()
	dir, err := sys.OpenDir(path)
	if err != nil {
		t.Fatal(err)
	}
	a, verifyErr := embed.Verify(files, dir, model)
	if err := errors.Join(verifyErr, dir.Close()); err != nil {
		t.Fatal(err)
	}
	return a
}

func copyModelFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(out, in)
	if err := errors.Join(copyErr, out.Close()); err != nil {
		t.Fatal(err)
	}
}

func checkUnit(t *testing.T, v []float32, dim int) {
	t.Helper()
	if len(v) != dim || !(math.Abs(magnitude(v)-1) <= 0.001) {
		t.Fatalf("expected dimension %d and a unit vector; got dimension %d and norm %g", dim, len(v), magnitude(v))
	}
}

func magnitude(v []float32) float64 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	return math.Sqrt(sum)
}

func cosine(a, b []float32) float64 {
	if len(a) != len(b) {
		return math.NaN()
	}
	var dot float64
	for i, x := range a {
		dot += float64(x) * float64(b[i])
	}
	return dot / (magnitude(a) * magnitude(b))
}
