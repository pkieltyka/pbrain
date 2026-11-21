package bge

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"

	"github.com/pkieltyka/pbrain/internal/embed"
)

const (
	modelURL    = "https://huggingface.co/BAAI/bge-small-en-v1.5/resolve/5c38ec7c405ec4b44b94cc5a9bb96e735b38267a/"
	preparation = "bge-1"
)

var modelFiles = [...]struct {
	name, path string
	size       int64
	sha256     string
}{
	{"model.safetensors", "model.safetensors", 133466304, "3c9f31665447c8911517620762200d2245a2518d6e7208acc78cd9db317e21ad"},
	{"tokenizer.json", "tokenizer.json", 711396, "d241a60d5e8f04cc1b2b3e9ef7a4921b27bf526d9f6050ab90f9267a1f9e5c66"},
	{"config.json", "config.json", 743, "094f8e891b932f2000c92cfc663bac4c62069f5d8af5b5278c4306aef3084750"},
	{"sentence_bert_config.json", "sentence_bert_config.json", 52, "84e39fda68ccbff05bfa723ae9c0e70e23e2ec373b76e0f8c6e71af72a693cbf"},
	{"pooling.json", "1_Pooling/config.json", 190, "d1caf60c96f5fba2157c0c26b76d80818fad6cf0b8eb5e73ec372ff9818eba5c"},
}

var modelRow = makeModel()

func makeModel() embed.Model {
	m := embed.Model{Name: "bge-small-en-v1.5", Runtime: "aikit", Dim: 384}
	h := sha256.New()
	h.Write([]byte(m.Name + "\n"))
	for _, f := range modelFiles {
		digest, err := hex.DecodeString(f.sha256)
		if err != nil || len(digest) != sha256.Size {
			panic("bge: invalid pinned digest for " + f.name)
		}
		m.Files = append(m.Files, embed.File{
			Name: f.name, URL: modelURL + f.path, Size: f.size, SHA256: [sha256.Size]byte(digest),
		})
		h.Write([]byte(f.name + "\n" + f.sha256 + "\n"))
	}
	h.Write([]byte(strconv.Itoa(m.Dim) + "\n" + preparation))
	m.Fingerprint = hex.EncodeToString(h.Sum(nil))
	return m
}

// Entry provides the bge-small-en-v1.5 row and opener. The release registry
// adds this entry only after calibration establishes its cutoff.
func Entry() embed.Entry {
	model := modelRow
	model.Files = slices.Clone(model.Files)
	return embed.Entry{Model: model, Open: open}
}
