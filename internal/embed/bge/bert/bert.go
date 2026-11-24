// Package bert contains the verified-byte BERT path used by bge-small.
// Adapted from aikit encoder/bert.go and encoder/pooling.go.
package bert

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"path"
)

type bertConfig struct {
	VocabSize    int     `json:"vocab_size"`
	Hidden       int     `json:"hidden_size"`
	Layers       int     `json:"num_hidden_layers"`
	Heads        int     `json:"num_attention_heads"`
	Intermediate int     `json:"intermediate_size"`
	MaxPos       int     `json:"max_position_embeddings"`
	TypeVocab    int     `json:"type_vocab_size"`
	LNEps        float64 `json:"layer_norm_eps"`
	Act          string  `json:"hidden_act"`
	PosType      string  `json:"position_embedding_type"`
	ModelType    string  `json:"model_type"`
}

type bertLayer struct {
	Wq, Wk, Wv       []float32
	Bq, Bk, Bv       []float32
	Wo, Bo           []float32
	AttnLNW, AttnLNB []float32
	Wi, Bi           []float32
	Wd, Bd           []float32
	OutLNW, OutLNB   []float32
}

// BERT holds immutable weights and the tokenizer. Concurrent Embed calls
// use independent scratch buffers. Close follows the last call.
type BERT struct {
	cfg                     bertConfig
	wordEmb, posEmb         []float32
	typeEmb, embLNW, embLNB []float32
	layers                  []bertLayer
	tok                     *Tokenizer
	maxSeq                  int
}

// LoadBERTFromFS reads the five BGE files once from fsys, rooted at dir.
// Weight slices can alias the safetensors buffer, which must stay immutable.
func LoadBERTFromFS(fsys fs.FS, dir string) (*BERT, error) {
	var c bertConfig
	if err := readJSON(fsys, path.Join(dir, "config.json"), &c); err != nil {
		return nil, err
	}
	if c.ModelType != "bert" || c.Act != "gelu" || c.PosType != "" && c.PosType != "absolute" {
		return nil, fmt.Errorf("bert: unsupported config (%s, %s, %s)", c.ModelType, c.Act, c.PosType)
	}
	if c.Hidden <= 0 || c.Heads <= 0 || c.Hidden%c.Heads != 0 ||
		c.VocabSize <= 0 || c.Layers <= 0 || c.Intermediate <= 0 || c.MaxPos <= 0 || c.TypeVocab < 0 ||
		c.LNEps < 0 || math.IsNaN(c.LNEps) || math.IsInf(c.LNEps, 0) {
		return nil, fmt.Errorf("bert: invalid model dimensions or layer norm epsilon")
	}
	if c.LNEps == 0 {
		c.LNEps = 1e-12
	}

	data, err := fs.ReadFile(fsys, path.Join(dir, "model.safetensors"))
	if err != nil {
		return nil, fmt.Errorf("bert: read weights: %w", err)
	}
	weights, err := parseSafetensors(data)
	if err != nil {
		return nil, err
	}
	if c.Layers > len(weights) {
		return nil, fmt.Errorf("bert: %d layers exceed the available tensors", c.Layers)
	}

	D, I := c.Hidden, c.Intermediate
	b := &BERT{cfg: c, layers: make([]bertLayer, c.Layers)}
	get := func(name string, want ...int) []float32 {
		if err != nil {
			return nil
		}
		var v []float32
		v, err = weights.floats(name, want)
		return v
	}
	b.wordEmb = get("embeddings.word_embeddings.weight", c.VocabSize, D)
	b.posEmb = get("embeddings.position_embeddings.weight", c.MaxPos, D)
	if c.TypeVocab > 0 {
		b.typeEmb = get("embeddings.token_type_embeddings.weight", c.TypeVocab, D)
	}
	b.embLNW = get("embeddings.LayerNorm.weight", D)
	b.embLNB = get("embeddings.LayerNorm.bias", D)
	for i := range b.layers {
		p := fmt.Sprintf("encoder.layer.%d.", i)
		l := &b.layers[i]
		l.Wq, l.Bq = get(p+"attention.self.query.weight", D, D), get(p+"attention.self.query.bias", D)
		l.Wk, l.Bk = get(p+"attention.self.key.weight", D, D), get(p+"attention.self.key.bias", D)
		l.Wv, l.Bv = get(p+"attention.self.value.weight", D, D), get(p+"attention.self.value.bias", D)
		l.Wo, l.Bo = get(p+"attention.output.dense.weight", D, D), get(p+"attention.output.dense.bias", D)
		l.AttnLNW, l.AttnLNB = get(p+"attention.output.LayerNorm.weight", D), get(p+"attention.output.LayerNorm.bias", D)
		l.Wi, l.Bi = get(p+"intermediate.dense.weight", I, D), get(p+"intermediate.dense.bias", I)
		l.Wd, l.Bd = get(p+"output.dense.weight", D, I), get(p+"output.dense.bias", D)
		l.OutLNW, l.OutLNB = get(p+"output.LayerNorm.weight", D), get(p+"output.LayerNorm.bias", D)
	}
	if err != nil {
		return nil, err
	}

	var sentence struct {
		MaxSeqLength int `json:"max_seq_length"`
	}
	if err := readJSON(fsys, path.Join(dir, "sentence_bert_config.json"), &sentence); err != nil {
		return nil, err
	}
	if sentence.MaxSeqLength <= 0 {
		return nil, fmt.Errorf("bert: max_seq_length must be positive")
	}
	b.maxSeq = min(sentence.MaxSeqLength, c.MaxPos)
	b.tok, err = LoadTokenizerFromFS(fsys, path.Join(dir, "tokenizer.json"))
	if err != nil {
		return nil, err
	}

	var pooling struct {
		CLS          bool `json:"pooling_mode_cls_token"`
		Mean         bool `json:"pooling_mode_mean_tokens"`
		Max          bool `json:"pooling_mode_max_tokens"`
		MeanSqrt     bool `json:"pooling_mode_mean_sqrt_len_tokens"`
		WeightedMean bool `json:"pooling_mode_weightedmean_tokens"`
		Last         bool `json:"pooling_mode_lasttoken"`
	}
	if err := readJSON(fsys, path.Join(dir, "1_Pooling/config.json"), &pooling); err != nil {
		return nil, err
	}
	if !pooling.CLS || pooling.Mean || pooling.Max || pooling.MeanSqrt || pooling.WeightedMean || pooling.Last {
		return nil, fmt.Errorf("bert: expected CLS pooling")
	}
	return b, nil
}

func readJSON(fsys fs.FS, name string, dst any) error {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("bert: read %s: %w", name, err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("bert: parse %s: %w", name, err)
	}
	return nil
}

// Tokenizer is the WordPiece tokenizer read with the model weights.
func (b *BERT) Tokenizer() *Tokenizer { return b.tok }

// Close drops the references to the provided buffers and is idempotent.
func (b *BERT) Close() error {
	*b = BERT{}
	return nil
}

// Embed returns the L2-normalized CLS vector for wrapped token IDs.
func (b *BERT) Embed(ids []int32) []float32 {
	if len(ids) == 0 || b.maxSeq == 0 {
		return nil
	}
	// The last layer computes only row zero. Its K and V still read the
	// whole sequence, so the result agrees with row zero of a full forward.
	return l2Normalize(b.forward(ids, true))
}
