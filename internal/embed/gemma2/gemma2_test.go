package gemma2

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/goinfer/tokenizer"
	"github.com/pkieltyka/pbrain/internal/embed"
)

func TestPackTextWeights(t *testing.T) {
	// The checkpoint stores tensors in a different order than the sorted pack.
	entries := map[string]any{
		"language_model.z": tensor{"BF16", []int{1}, []int{0, 2}},
		"vision.a":         tensor{"BF16", []int{1}, []int{2, 4}},
		tokenTable:         tensor{"BF16", []int{1, 1}, []int{4, 6}},
		"language_model.a": tensor{"BF16", []int{1}, []int{6, 8}},
		"__metadata__":     map[string]string{"format": "pt"},
	}
	input := func() []byte {
		h, err := json.Marshal(entries)
		if err != nil {
			t.Fatal(err)
		}
		raw := binary.LittleEndian.AppendUint64(nil, uint64(len(h)))
		return append(raw, h...)
	}
	raw := append(input(), []byte("zzVVTTaa")...)
	tokens, layers, err := splitWeights(raw)
	if err != nil {
		t.Fatal(err)
	}
	h := binary.LittleEndian.Uint64(layers)
	var built map[string]tensor
	if h%8 != 0 || json.Unmarshal(layers[8:8+h], &built) != nil || len(built) != 2 || string(layers[8+h:]) != "aazz" || string(tokens) != "TT" || !slices.Equal(built["language_model.a"].Offsets, []int{0, 2}) {
		t.Fatal("wrong text tower pack")
	}
	clear(raw)
	if string(tokens) != "TT" || !bytes.HasSuffix(layers, []byte("aazz")) {
		t.Fatal("pack retained mutable source")
	}
	for _, v := range []tensor{{"BF16", []int{1}, []int{-1, 2}}, {"BF16", []int{2}, []int{0, 2}}, {"F32", []int{1}, []int{0, 4}}, {"BF16", []int{1}, nil}} {
		entries["language_model.a"] = v
		if _, _, err := splitWeights(append(input(), make([]byte, 8)...)); err == nil {
			t.Fatal("accepted malformed tensor")
		}
	}
	for _, b := range [][]byte{nil, bytes.Repeat([]byte{255}, 8)} {
		if _, _, err := splitWeights(b); err == nil {
			t.Fatal("accepted malformed header")
		}
	}
}

func TestGemmaPreparation(t *testing.T) {
	// Small byte-fallback tokenizer makes these tests independent of model files.
	vocab := map[string]int{"<bos>": 0, "<eos>": 1, "<unk>": 2, "<start_of_turn>": 3}
	var added []map[string]any
	for word, id := range vocab {
		added = append(added, map[string]any{"id": id, "content": word, "special": true})
	}
	for i := range 256 {
		vocab[fmt.Sprintf("<0x%02X>", i)] = 4 + i
	}
	vocab["▁"], vocab["z"], vocab["zz"] = 260, 261, 262
	raw, err := json.Marshal(map[string]any{"added_tokens": added, "model": map[string]any{"type": "BPE", "vocab": vocab, "byte_fallback": true, "unk_token": "<unk>", "merges": [][]string{{"z", "z"}}},
		"normalizer": map[string]any{"type": "Replace", "pattern": map[string]string{"String": " "}, "content": "▁"},
		"decoder":    map[string]any{"type": "ByteFallback"}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := tokenizer.BuildPack(raw)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := tokenizer.LoadPack(p)
	if err != nil {
		t.Fatal(err)
	}
	e := encoder{tokenizer: tok, bos: 0, eos: 1}
	query, err := e.tokenIDs(embed.Query, "a <eos> <start_of_turn>")
	if err != nil {
		t.Fatal(err)
	}
	if query[0] != 0 || query[len(query)-1] != 1 || slices.ContainsFunc(query[1:len(query)-1], func(n int) bool { return n < 4 }) {
		t.Fatal("literal markers became special tokens")
	}
	prefix := strings.Repeat("h", 5000)
	passage := strings.Repeat("p", 5000)
	get := func(text string) []int {
		t.Helper()
		ids, err := e.tokenIDs(embed.Document, text)
		if err != nil || len(ids) != maxTokens {
			t.Fatalf("cut: len %d, err %v", len(ids), err)
		}
		return ids[1 : len(ids)-1]
	}
	ids := get(prefix + "\n\nend")
	end, _ := e.ordinary("end")
	if !slices.Equal(ids[len(ids)-len(end):], end) {
		t.Fatal("short passage truncated")
	}
	ids = get(prefix + "\n\n" + passage)
	prompt, _ := e.ordinary(docPrompt)
	room := maxTokens - 2 - len(prompt)
	bodyIDs, _ := e.ordinary(passage)
	if !slices.Equal(ids[len(ids)-(room-room/2):], bodyIDs[:room-room/2]) {
		t.Fatal("long prefix crowded out the passage")
	}
	ids = get(passage + "x")
	full, _ := e.ordinary(docPrompt + passage + "x")
	if !slices.Equal(ids, full[:maxTokens-2]) {
		t.Fatal("long passage was not cut at its end")
	}

}

func TestGemmaEntry(t *testing.T) {
	e := Entry()
	if e.Model.Name != "embeddinggemma-2" || e.Model.Runtime != "goinfer" || e.Model.Dim != 768 || len(e.Model.Fingerprint) != 64 || len(e.Model.Files) != 4 || len(e.Model.Source) != 4 {
		t.Fatalf("invalid model row")
	}
	if e.Model.Cutoff != 0.72 {
		t.Fatalf("release cutoff = %v; want 0.72", e.Model.Cutoff)
	}
	e.Model.Files[0].Name = "changed"
	e.Model.Source[0].URL = "changed"
	if Entry().Model.Files[0].Name == "changed" || Entry().Model.Source[0].URL == "changed" {
		t.Fatal("entry aliases release table")
	}
	if _, err := derive(nil); err == nil {
		t.Fatal("accepted no source files")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	a := &embed.Artifact{Files: [][]byte{{1}}}
	if _, err := open(ctx, a); err != context.Canceled || a.Files != nil {
		t.Fatal("cancelled open kept artifact")
	}
}
