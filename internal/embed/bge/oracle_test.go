package bge

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/semantic"
)

type oracleInput struct {
	Key  string `json:"key"`
	Role string `json:"role"`
	Text string `json:"text"`
}

func (in oracleInput) role() embed.Role {
	if in.Role == "query" {
		return embed.Query
	}
	return embed.Document
}

type oracleIDs struct {
	Segments [][]int32 `json:"segments"`
	Wrapped  []int32   `json:"wrapped"`
}

const oracleDir = "testdata/vendor-oracle"

func oracleInputs(t testing.TB, dir string) []oracleInput {
	t.Helper()
	var inputs []oracleInput
	readOracleJSON(t, filepath.Join(dir, "inputs.json"), &inputs)
	return inputs
}

func readOracleJSON(t testing.TB, path string, target any) {
	t.Helper()
	if err := json.Unmarshal(readOracleFile(t, path), target); err != nil {
		t.Fatal(err)
	}
}

func readOracleFile(t testing.TB, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, ".gz") {
		return data
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err = io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// captureOracleInputs assembles the corpus from the tracked fixture and the
// separate sentence-transformers references, then adds tokenizer boundaries.
func captureOracleInputs(t *testing.T, e *embedder) []oracleInput {
	t.Helper()
	repo := "../../.."
	from := filepath.Join(repo, "tools/phase28/evidence/gemma-pax-22/fixture-paths.txt")
	paths := strings.Fields(string(readOracleFile(t, from)))
	slices.Sort(paths)
	var inputs []oracleInput
	for _, path := range paths {
		name := filepath.Join(repo, "internal/semantic/calibrate/testdata/retrieval/brain", path)
		raw := readOracleFile(t, name)
		head, offset, _, err := item.Parse(path, raw)
		if err != nil {
			t.Fatal(err)
		}
		for i, passage := range semantic.Passages(head.Title, raw[offset:]) {
			inputs = append(inputs, oracleInput{fmt.Sprintf("%s:%d", path, i), "document", passage.Text})
		}
	}
	if len(paths) != 32 || len(inputs) != 59 {
		t.Fatalf("retrieval fixture = %d items, %d passages", len(paths), len(inputs))
	}
	var references []oracleInput
	readOracleJSON(t, "testdata/reference.json", &references)
	if len(references) != 22 {
		t.Fatalf("reference fixture = %d texts", len(references))
	}
	for i, ref := range references {
		ref.Key = fmt.Sprintf("reference-%02d", i)
		inputs = append(inputs, ref)
	}

	edges := []struct{ key, text string }{
		{"empty", ""}, {"whitespace", " \t\n\u00a0\u2003\r "},
		{"composed", "Café naïve déjà vu ÉCOLE"}, {"decomposed", "Cafe\u0301 nai\u0308ve de\u0301ja\u0300 vu E\u0301COLE"},
		{"cjk", "我喜欢北京｜東京、한국어。"}, {"punctuation", "a—b…c!! 3.14 hyphen-word @user /x_y.go"},
		{"specials", "[PAD]abc[UNK]def [CLS] [SEP] [MASK]"},
		{"unknown", "qzjqzjqzj\ufffd\u200d🗿"}, {"overlong-word", strings.Repeat("a", 101)},
		{"boundary-word", strings.Repeat("a", 100)}, {"control", "a\x00b\u200bc\tD\r\nE"},
	}
	for _, role := range []string{"document", "query"} {
		for _, edge := range edges {
			inputs = append(inputs, oracleInput{role + "-" + edge.key, role, edge.text})
		}
	}
	queryPrefix := len(e.tokenizer.Encode(queryInstruction))
	for _, n := range []int{509, 510, 511} {
		inputs = append(inputs,
			oracleInput{fmt.Sprintf("document-cut-%d", n), "document", strings.Repeat("a ", n)},
			oracleInput{fmt.Sprintf("query-cut-%d", n), "query", strings.Repeat("a ", n-queryPrefix)},
			oracleInput{fmt.Sprintf("prefix-cut-%d", n), "document", strings.Repeat("a ", 270) + "\n\n" + strings.Repeat("b ", n-270)},
		)
	}
	return append(inputs, oracleInput{"long-prefix-and-body", "document", strings.Repeat("a ", 600) + "\n\n" + strings.Repeat("b ", 700)})
}

func oracleRaw(e *embedder, in oracleInput) oracleIDs {
	role, text := in.role(), neutralize.Replace(in.Text)
	ids := oracleIDs{Wrapped: e.tokenIDs(role, in.Text)}
	if role == embed.Query {
		ids.Segments = [][]int32{e.tokenizer.Encode(queryInstruction + text)}
	} else if title, body, ok := strings.Cut(text, "\n\n"); ok {
		ids.Segments = [][]int32{e.tokenizer.Encode(title), e.tokenizer.Encode(body)}
	} else {
		ids.Segments = [][]int32{e.tokenizer.Encode(text)}
	}
	return ids
}

func oracleVectorBytes(vector []float32) []byte {
	out := make([]byte, 0, len(vector)*4)
	for _, value := range vector {
		out = binary.LittleEndian.AppendUint32(out, math.Float32bits(value))
	}
	return out
}

func TestBgeVendorOracle(t *testing.T) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("oracle is qualified on the two release architectures")
	}
	e := realEmbedder(t)
	arch := runtime.GOARCH
	if dest := os.Getenv("PBRAIN_CAPTURE_BGE_ORACLE"); dest != "" {
		inputs := captureOracleInputs(t, e)
		raw := make([]oracleIDs, len(inputs))
		var vectors []byte
		for i, in := range inputs {
			raw[i] = oracleRaw(e, in)
			got, err := e.Embed(t.Context(), in.role(), []string{in.Text})
			if err != nil || len(got) != 1 {
				t.Fatalf("%s: %v", in.Key, err)
			}
			vectors = append(vectors, oracleVectorBytes(got[0])...)
		}
		for _, file := range []struct {
			name string
			data any
		}{{"inputs.json", inputs}, {"ids.json", raw}} {
			data, err := json.MarshalIndent(file.data, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dest, file.name), append(data, '\n'), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dest, arch+".f32le"), vectors, 0600); err != nil {
			t.Fatal(err)
		}
		t.Logf("captured %d texts, %d vector bytes on %s", len(inputs), len(vectors), arch)
		return
	}

	inputs := oracleInputs(t, oracleDir)
	var wantIDs []oracleIDs
	readOracleJSON(t, filepath.Join(oracleDir, "ids.json.gz"), &wantIDs)
	wantVectors := readOracleFile(t, filepath.Join(oracleDir, arch+".f32le.gz"))
	width := 4 * e.Dim()
	if len(wantIDs) != len(inputs) || len(wantVectors) != width*len(inputs) {
		t.Fatal("oracle lengths do not match inputs and vector dimensions")
	}
	check := func(i int, vector []float32) {
		t.Helper()
		if got := oracleVectorBytes(vector); !bytes.Equal(got, wantVectors[i*width:(i+1)*width]) {
			t.Fatalf("%s: vector bits differ from %s baseline", inputs[i].Key, arch)
		}
	}
	for i, in := range inputs {
		if got := oracleRaw(e, in); !reflect.DeepEqual(got, wantIDs[i]) {
			t.Fatalf("%s: tokenizer IDs differ", in.Key)
		}
		got, err := e.Embed(t.Context(), in.role(), []string{in.Text})
		if err != nil || len(got) != 1 {
			t.Fatalf("%s: %v", in.Key, err)
		}
		check(i, got[0])
	}
	prior := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(prior)
	for _, role := range []embed.Role{embed.Document, embed.Query} {
		var texts []string
		var indices []int
		for i, in := range inputs {
			if in.role() == role {
				texts = append(texts, in.Text)
				indices = append(indices, i)
			}
		}
		got, err := e.Embed(t.Context(), role, texts)
		if err != nil || len(got) != len(texts) {
			t.Fatalf("concurrent embedding: %v", err)
		}
		for j, vector := range got {
			check(indices[j], vector)
		}
	}
	if e.Fingerprint() != "6bd9989750090942b9ff6d86f27ea8a39e255457d0c01659991f87280e181b9b" ||
		semantic.IndexFingerprint(Entry().Model) != "46000384c2fd339d6352b50398ba3a83a7e84cda36e7d53da029f19f92a3363e" {
		t.Fatal("BGE model or index fingerprint changed")
	}
	t.Logf("%d tokenizer and singleton/multiworker vectors match %s", len(inputs), arch)
}

func BenchmarkBgeOraclePassages(b *testing.B) {
	entry := Entry()
	a := verifiedModel(b, modelFolder(b), entry.Model)
	e, err := embed.NewRegistry(entry).Open(b.Context(), a, embed.OpenOptions{})
	if err != nil {
		b.Fatal(err)
	}
	defer e.Close()
	inputs := oracleInputs(b, oracleDir)
	docs := make([]string, 59)
	for i, in := range inputs[:len(docs)] {
		docs[i] = in.Text
	}
	run := func() {
		if vectors, err := e.Embed(b.Context(), embed.Document, docs); err != nil || len(vectors) != len(docs) {
			b.Fatalf("fixture embed = %d vectors, %v", len(vectors), err)
		}
	}
	run()
	for b.Loop() {
		run()
	}
	b.ReportMetric(float64(b.N*len(docs))/b.Elapsed().Seconds(), "passages/s")
}
