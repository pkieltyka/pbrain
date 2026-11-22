// Package gemma2 adapts the text tower used when a project selects embeddinggemma-2.
package gemma2

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"

	"github.com/pkieltyka/pbrain/internal/embed"
)

const sourceURL = "https://huggingface.co/google/embeddinggemma-2/resolve/f9972ac22e696a105c482202646979e33e4b5a1e/"
const dimension = 768
const preparation = "gemma2-1"

func file(name, url string, size int64, digest string) embed.File {
	d, err := hex.DecodeString(digest)
	if err != nil || len(d) != sha256.Size {
		panic("gemma2: invalid release digest for " + name)
	}
	return embed.File{Name: name, URL: url, Size: size, SHA256: [sha256.Size]byte(d)}
}

var sourceFiles = []embed.File{
	file("model.safetensors", sourceURL+"model.safetensors", 1488915288, "197a32965d4b1105faf060417baa899e193fb73cd401f42ec9295234d5553d79"),
	file("tokenizer.json", sourceURL+"tokenizer.json", 32170510, "4d777ef5bdc1aa36227abdfb77c3e49e7b9c892d16e1b6bda41c393504828be4"),
	file("config.json", sourceURL+"config.json", 4455, "b8f1e9931b57fbc054acdb445c41765d55b0074c58d145fa82839941ad1b5bb3"),
	file("config_sentence_transformers.json", sourceURL+"config_sentence_transformers.json", 1565, "031e56a498d33c349ab489a21885bcfe25b4fcba841149dc99e1e90d4a7c28f5"),
}
var packFiles = []embed.File{
	file("tokens.bf16", "", 268435456, "56ebbdbfd706c827f3fae1d97aa39135c7ddc0f1e6d9b12f8565f84bbfbda877"),
	file("layers.safetensors", "", 273620496, "28a9678df941bf3eb7acca4e27ffd2fba6eef24039374d62148d705fc83b6227"),
	file("tokenizer.bin", "", 9581663, "45a7e7d5a1e84ad83ef469a8f6aecc3dd15f51b7c4866008e2d770142d96b709"),
	file("config.json", "", 3442, "65ef03947e006ea2a946fa323c88dd880ad6522d628723e81ddce04c2868db82"),
}
var fingerprint = func() string {
	var s strings.Builder
	s.WriteString("embeddinggemma-2\n")
	for _, f := range packFiles {
		s.WriteString(f.Name + "\n" + hex.EncodeToString(f.SHA256[:]) + "\n")
	}
	s.WriteString(strconv.Itoa(dimension) + "\n" + preparation)
	h := sha256.Sum256([]byte(s.String()))
	return hex.EncodeToString(h[:])
}()

// Entry returns the release model with its cutoff from the frozen retrieval fixture.
func Entry() embed.Entry {
	return embed.Entry{Model: embed.Model{Name: "embeddinggemma-2", Runtime: "goinfer", Dim: dimension, Cutoff: 0.72, Fingerprint: fingerprint,
		Files: slices.Clone(packFiles), Source: slices.Clone(sourceFiles)}, Open: open, Devices: cudaDevices, Derive: derive}
}
