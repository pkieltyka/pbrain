package gemma2

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/goinfer/tokenizer"
)

const tokenTable = "language_model.embed_tokens.weight"
const queryPrompt = "task: search result | query: "
const docPrompt = "title: none | text: "

type configuration struct {
	Config  json.RawMessage   `json:"config"`
	Prompts map[string]string `json:"prompts"`
}
type tensor struct {
	DType   string `json:"dtype"`
	Shape   []int  `json:"shape"`
	Offsets []int  `json:"data_offsets"`
}

func derive(source [][]byte) ([][]byte, error) {
	if len(source) != 4 {
		return nil, errors.New("gemma2: expected four source files")
	}
	cfg, err := embeddinggemma2.LoadConfigBytes(source[2])
	if err != nil {
		return nil, err
	}
	if cfg.EmbeddingDim != dimension {
		return nil, errors.New("gemma2: invalid output dimension")
	}
	var st struct {
		Prompts map[string]string `json:"prompts"`
	}
	if err := json.Unmarshal(source[3], &st); err != nil {
		return nil, err
	}
	if st.Prompts["document"] != docPrompt || st.Prompts["query"] != queryPrompt {
		return nil, errors.New("gemma2: unexpected retrieval prompts")
	}
	conf, err := json.Marshal(configuration{source[2], map[string]string{"document": docPrompt, "query": queryPrompt}})
	if err != nil {
		return nil, err
	}
	tok, err := tokenizer.BuildPack(source[1])
	if err != nil {
		return nil, err
	}
	tokens, layers, err := splitWeights(source[0])
	if err != nil {
		return nil, err
	}
	return [][]byte{tokens, layers, tok, conf}, nil
}

// splitWeights keeps the text tensors in their stored representation. The new
// header and payload both use name order; offsets are relative to the payload.
func splitWeights(src []byte) ([]byte, []byte, error) {
	if len(src) < 8 {
		return nil, nil, errors.New("gemma2: missing weights header")
	}
	h := binary.LittleEndian.Uint64(src)
	if h > uint64(len(src)-8) || h > 8<<20 {
		return nil, nil, errors.New("gemma2: invalid weights header length")
	}
	var header map[string]json.RawMessage
	if err := json.Unmarshal(src[8:8+h], &header); err != nil {
		return nil, nil, err
	}
	data := src[8+h:]
	layers := map[string]tensor{}
	var spans [][]byte
	var tokens []byte
	total := 0
	for _, name := range slices.Sorted(maps.Keys(header)) {
		if !strings.HasPrefix(name, "language_model.") {
			continue
		}
		var t tensor
		if err := json.Unmarshal(header[name], &t); err != nil {
			return nil, nil, err
		}
		if t.DType != "BF16" || len(t.Offsets) != 2 || t.Offsets[0] < 0 || t.Offsets[1] < t.Offsets[0] || t.Offsets[1] > len(data) {
			return nil, nil, fmt.Errorf("gemma2: invalid tensor %s", name)
		}
		raw := data[t.Offsets[0]:t.Offsets[1]]
		size := 2 // All the text weights in this checkpoint are BF16.
		for _, width := range t.Shape {
			if width <= 0 || size > len(raw)/width {
				return nil, nil, fmt.Errorf("gemma2: invalid tensor shape for %s", name)
			}
			size *= width
		}
		if size != len(raw) {
			return nil, nil, fmt.Errorf("gemma2: invalid tensor size for %s", name)
		}
		if name == tokenTable {
			tokens = raw
			continue
		}
		if len(raw) > len(data)-total {
			return nil, nil, errors.New("gemma2: invalid cumulative tensor size")
		}
		spans = append(spans, raw)
		t.Offsets = []int{total, total + len(raw)}
		layers[name] = t
		total += len(raw)
	}
	if len(tokens) == 0 || len(layers) == 0 {
		return nil, nil, errors.New("gemma2: missing text tower")
	}
	meta, err := json.Marshal(layers)
	if err != nil {
		return nil, nil, err
	}
	for len(meta)%8 != 0 {
		meta = append(meta, ' ')
	}
	out := make([]byte, 8, 8+len(meta)+total)
	binary.LittleEndian.PutUint64(out, uint64(len(meta)))
	out = append(out, meta...)
	for _, s := range spans {
		out = append(out, s...)
	}
	return bytes.Clone(tokens), out, nil
}
