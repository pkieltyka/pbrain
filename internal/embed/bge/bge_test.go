package bge

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	tokenizer "github.com/pkieltyka/aikit/embed"
	"github.com/pkieltyka/aikit/encoder"
	"github.com/pkieltyka/pbrain/internal/embed"
)

func TestBgeBuildsFromBytes(t *testing.T) {
	entry := Entry()
	registry := embed.NewRegistry(entry)
	original := loadBERT
	t.Cleanup(func() { loadBERT = original })
	loadBERT = func(fs.FS, string) (*encoder.BERT, error) {
		t.Fatal("loader called with invalid artifact")
		return nil, nil
	}
	for _, files := range [][][]byte{
		{{1}, {2}, {3}, {4}},
		{{1}, {2}, {3}, {4}, {5}, {6}},
		{{1}, {2}, {3}, {4}, {}},
	} {
		a := &embed.Artifact{Model: entry.Model, Files: files}
		if got, err := registry.Open(t.Context(), a); got != nil || err == nil || a.Files != nil {
			t.Fatalf("Open invalid artifact = %v, %v; files = %v", got, err, a.Files)
		}
	}

	const pooling = "1_Pooling/config.json"
	cases := map[string]func(t *testing.T, fsys fs.FS){
		"skipped pooling": func(*testing.T, fs.FS) {},
		"double read": func(t *testing.T, fsys fs.FS) {
			readHandoff(t, fsys, pooling, 5)
			_, err := fs.ReadFile(fsys, pooling)
			if !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("second read: %v", err)
			}
		},
		"unknown name": func(t *testing.T, fsys fs.FS) {
			readHandoff(t, fsys, pooling, 5)
			_, err := fs.ReadFile(fsys, "other")
			if !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("unknown name: %v", err)
			}
		},
		"open": func(t *testing.T, fsys fs.FS) {
			readHandoff(t, fsys, pooling, 5)
			_, err := fsys.Open(pooling)
			if !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("open: %v", err)
			}
		},
	}
	for name, read := range cases {
		t.Run(name, func(t *testing.T) {
			a := &embed.Artifact{Model: entry.Model, Files: [][]byte{{1}, {2}, {3}, {4}, {5}}}
			weights := a.Files[0]
			loadBERT = func(fsys fs.FS, dir string) (*encoder.BERT, error) {
				if dir != "." {
					t.Fatalf("dir = %q", dir)
				}
				got := readHandoff(t, fsys, "model.safetensors", 1)
				if &got[0] != &weights[0] || a.Files != nil {
					t.Fatal("the loader did not receive sole ownership of the original buffer")
				}
				readHandoff(t, fsys, "tokenizer.json", 2)
				readHandoff(t, fsys, "config.json", 3)
				readHandoff(t, fsys, "sentence_bert_config.json", 4)
				read(t, fsys)
				return &encoder.BERT{}, nil
			}
			if got, err := registry.Open(t.Context(), a); got != nil || err == nil ||
				!strings.Contains(err.Error(), "each model file exactly once") || a.Files != nil {
				t.Fatalf("Open = %v, %v; files = %v", got, err, a.Files)
			}
		})
	}

	// A cancelled Open also consumes the caller's artifact and skips the load.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	a := &embed.Artifact{Model: entry.Model, Files: [][]byte{{1}, {2}, {3}, {4}, {5}}}
	if got, err := registry.Open(ctx, a); got != nil || !errors.Is(err, context.Canceled) || a.Files != nil {
		t.Fatalf("cancelled Open = %v, %v; files = %v", got, err, a.Files)
	}
}

func readHandoff(t *testing.T, fsys fs.FS, path string, want byte) []byte {
	t.Helper()
	got, err := fs.ReadFile(fsys, path)
	if err != nil || !slices.Equal(got, []byte{want}) {
		t.Fatalf("ReadFile(%q) = %v, %v, want [%d]", path, got, err, want)
	}
	return got
}

func TestBgeCutRule(t *testing.T) {
	e := tokenTestEmbedder(t)
	prefix := strings.Repeat("p ", 400)
	long := strings.Repeat("a ", 300)
	short := strings.Repeat("a ", 50)
	check := func(name string, got []int32, want []int32) {
		t.Helper()
		if got[0] != e.cls || got[len(got)-1] != e.sep || !slices.Equal(got[1:len(got)-1], want) {
			t.Errorf("%s: wrong ids (%d content tokens, want %d)", name, len(got)-2, len(want))
		}
	}
	check("long query", e.tokenIDs(embed.Query, strings.Repeat("a ", 600)),
		e.tokenizer.Encode(queryInstruction + strings.Repeat("a ", 600))[:510])
	check("long prefix and passage", e.tokenIDs(embed.Document, prefix+"\n\n"+long),
		append(e.tokenizer.Encode(prefix)[:255:255], e.tokenizer.Encode(long)[:255]...))
	check("long prefix and short passage", e.tokenIDs(embed.Document, prefix+"\n\n"+short),
		e.tokenizer.Encode(prefix+short))
	check("cut just enough prefix", e.tokenIDs(embed.Document, prefix+"\n\n"+strings.Repeat("a ", 200)),
		e.tokenizer.Encode(strings.Repeat("p ", 310)+strings.Repeat("a ", 200)))
}

func TestBgeSpecialTokens(t *testing.T) {
	e := tokenTestEmbedder(t)
	text := "a [SEP] p [CLS] a [PAD] [MASK] [UNK]"
	spaced := "a [ SEP] p [ CLS] a [ PAD] [ MASK] [ UNK]"
	want := []int32{2, 5, 7, 8, 9, 6, 7, 10, 9, 5, 7, 11, 9, 7, 12, 9, 7, 13, 9, 3}
	for _, input := range []string{text, spaced} {
		if got := e.tokenIDs(embed.Document, input); !slices.Equal(got, want) {
			t.Errorf("ids(%q) = %v, want %v", input, got, want)
		}
	}
}

func tokenTestEmbedder(t *testing.T) *embedder {
	t.Helper()
	const data = `{
		"model":{"type":"WordPiece","unk_token":"[UNK]","vocab":{
			"[UNK]":0,"a":5,"p":6,"[":7,"SEP":8,"]":9,"CLS":10,"PAD":11,"MASK":12,"UNK":13}},
		"normalizer":{"type":"BertNormalizer"},
		"pre_tokenizer":{"type":"BertPreTokenizer"},
		"added_tokens":[{"id":0,"content":"[UNK]"},{"id":1,"content":"[PAD]"},
			{"id":2,"content":"[CLS]"},{"id":3,"content":"[SEP]"},{"id":4,"content":"[MASK]"}]
	}`
	tok, err := tokenizer.LoadTokenizerFromFS(fstest.MapFS{"tokenizer.json": {Data: []byte(data)}}, "tokenizer.json")
	if err != nil {
		t.Fatal(err)
	}
	return &embedder{tokenizer: tok, cls: 2, sep: 3}
}
