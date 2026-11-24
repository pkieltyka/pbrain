// Adapted from aikit encoder/bert_fs_test.go and encoder/bert_test.go.
package bert

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"math"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

const tinyTokenizer = `{
  "added_tokens":[{"id":0,"content":"[PAD]","special":true},{"id":1,"content":"[UNK]","special":true},
                  {"id":2,"content":"[CLS]","special":true},{"id":3,"content":"[SEP]","special":true}],
  "normalizer":{"type":"BertNormalizer","clean_text":true,"handle_chinese_chars":true,"lowercase":true},
  "pre_tokenizer":{"type":"BertPreTokenizer"},
  "model":{"type":"WordPiece","unk_token":"[UNK]","continuing_subword_prefix":"##","max_input_chars_per_word":100,
           "vocab":{"[PAD]":0,"[UNK]":1,"[CLS]":2,"[SEP]":3,"hello":4,"world":5,"##s":6,"model":7,"files":8}}
}`

// A small, generated checkpoint keeps FS and forward tests independent of
// downloaded weights. It has the same F32 weights and I64 metadata as BGE.
func tinyBERTFiles(t *testing.T) fstest.MapFS {
	t.Helper()
	const D, I, L, V, P = 16, 32, 2, 9, 64
	type metadata struct {
		DType   string `json:"dtype"`
		Shape   []int  `json:"shape"`
		Offsets [2]int `json:"data_offsets"`
	}
	header := map[string]metadata{}
	var data []byte
	add := func(name string, shape ...int) {
		start := len(data)
		n := 1
		for _, d := range shape {
			n *= d
		}
		for i := range n {
			v := float32((i+start/4)%53-26) / 61
			if strings.HasSuffix(name, "LayerNorm.weight") {
				v += 1
			}
			data = binary.LittleEndian.AppendUint32(data, math.Float32bits(v))
		}
		header[name] = metadata{"F32", shape, [2]int{start, len(data)}}
	}
	header["embeddings.position_ids"] = metadata{"I64", []int{1, P}, [2]int{0, P * 8}}
	data = make([]byte, P*8)
	add("embeddings.word_embeddings.weight", V, D)
	add("embeddings.position_embeddings.weight", P, D)
	add("embeddings.token_type_embeddings.weight", 2, D)
	add("embeddings.LayerNorm.weight", D)
	add("embeddings.LayerNorm.bias", D)
	for i := range L {
		prefix := fmt.Sprintf("encoder.layer.%d.", i)
		for _, line := range []string{"attention.self.query", "attention.self.key", "attention.self.value", "attention.output.dense"} {
			add(prefix+line+".weight", D, D)
			add(prefix+line+".bias", D)
		}
		for _, norm := range []string{"attention.output.LayerNorm", "output.LayerNorm"} {
			add(prefix+norm+".weight", D)
			add(prefix+norm+".bias", D)
		}
		add(prefix+"intermediate.dense.weight", I, D)
		add(prefix+"intermediate.dense.bias", I)
		add(prefix+"output.dense.weight", D, I)
		add(prefix+"output.dense.bias", D)
	}
	raw, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	for len(raw)%8 != 0 {
		raw = append(raw, ' ')
	}
	weights := binary.LittleEndian.AppendUint64(nil, uint64(len(raw)))
	weights = append(weights, raw...)
	weights = append(weights, data...)
	return fstest.MapFS{
		"m/config.json": {Data: []byte(fmt.Sprintf(`{"vocab_size":%d,"hidden_size":%d,"num_hidden_layers":%d,
			"num_attention_heads":4,"intermediate_size":%d,"max_position_embeddings":%d,
			"type_vocab_size":2,"model_type":"bert","hidden_act":"gelu","position_embedding_type":"absolute"}`, V, D, L, I, P))},
		"m/model.safetensors":         {Data: weights},
		"m/sentence_bert_config.json": {Data: []byte(`{"max_seq_length":40}`)},
		"m/1_Pooling/config.json":     {Data: []byte(`{"pooling_mode_cls_token":true}`)},
		"m/tokenizer.json":            {Data: []byte(tinyTokenizer)},
	}
}

type countingFS struct {
	files fstest.MapFS
	reads map[string]int
}

func (c *countingFS) Open(name string) (fs.File, error) {
	c.reads[name]++
	return c.files.Open(name)
}

type countingReadFileFS struct{ *countingFS }

func (c countingReadFileFS) ReadFile(name string) ([]byte, error) {
	c.reads[name]++
	return c.files.ReadFile(name)
}

func TestLoadBERTFromFSReadsEachFileOnce(t *testing.T) {
	for _, viaReadFile := range []bool{false, true} {
		counts := &countingFS{files: tinyBERTFiles(t), reads: map[string]int{}}
		var view fs.FS = counts
		if viaReadFile {
			view = countingReadFileFS{counts}
		}
		model, err := LoadBERTFromFS(view, "m")
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]int{}
		for name := range counts.files {
			want[name] = 1
		}
		if !maps.Equal(counts.reads, want) {
			t.Fatalf("reads = %v, want %v", counts.reads, want)
		}
		if ids := model.Tokenizer().Encode("Hello worlds"); !slices.Equal(ids, []int32{4, 5, 6}) {
			t.Fatalf("tokenizer IDs = %v", ids)
		}
		if err := errors.Join(model.Close(), model.Close()); err != nil || model.Tokenizer() != nil {
			t.Fatalf("Close = %v, tokenizer %v", err, model.Tokenizer())
		}
	}
}

func TestLoadBERTFromFSRejectsMissingAndUnsupportedFiles(t *testing.T) {
	files := tinyBERTFiles(t)
	for name := range files {
		copy := maps.Clone(files)
		delete(copy, name)
		if model, err := LoadBERTFromFS(copy, "m"); model != nil || !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("missing %s: model %v, err %v", name, model, err)
		}
	}
	for _, tc := range []struct{ file, before, after, want string }{
		{"m/config.json", `"hidden_size":16`, `"hidden_size":12`, "want F32"},
		{"m/config.json", `"num_attention_heads":4`, `"num_attention_heads":-4`, "dimensions"},
		{"m/tokenizer.json", `"WordPiece"`, `"BPE"`, "expected WordPiece"},
		{"m/1_Pooling/config.json", "true", "false", "expected CLS"},
	} {
		copy := maps.Clone(files)
		body := string(copy[tc.file].Data)
		changed := strings.Replace(body, tc.before, tc.after, 1)
		if changed == body {
			t.Fatalf("invalid test fixture for %s", tc.file)
		}
		copy[tc.file] = &fstest.MapFile{Data: []byte(changed)}
		if model, err := LoadBERTFromFS(copy, "m"); model != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("invalid %s: model %v, err %v", tc.file, model, err)
		}
	}
}

func TestCLSHiddenStateMatchesFullRowZero(t *testing.T) {
	b, err := LoadBERTFromFS(tinyBERTFiles(t), "m")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	sameBits := func(a, b float32) bool { return math.Float32bits(a) == math.Float32bits(b) }
	for _, length := range []int{1, 2, 3, 7, 16, 31, 40, 57} {
		ids := make([]int32, length)
		for i := range ids {
			ids[i] = int32(i % b.cfg.VocabSize)
		}
		full := b.forward(ids, false)[:b.cfg.Hidden]
		got := b.forward(ids, true)
		if !slices.EqualFunc(got, full, sameBits) {
			t.Fatalf("%d tokens: CLS row differs from full forward", length)
		}
		if vector := b.Embed(ids); !slices.EqualFunc(vector, l2Normalize(full), sameBits) {
			t.Fatalf("%d tokens: normalized vector differs", length)
		}
	}
}

func TestWordPieceConcurrentMemo(t *testing.T) {
	tok, err := LoadTokenizerFromFS(tinyBERTFiles(t), "m/tokenizer.json")
	if err != nil {
		t.Fatal(err)
	}
	for i := range 32 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			ids := tok.Encode("HÉLLO worlds[SEP] " + strings.Repeat("a", 101))
			if !slices.Equal(ids, []int32{4, 5, 6, 3, 1}) {
				t.Fatalf("tokenizer IDs = %v", ids)
			}
			ids[0] = 99
		})
	}
}
