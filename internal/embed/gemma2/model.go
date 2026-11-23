// Package gemma2 adapts the text tower used by the EmbeddingGemma 2 models.
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
	return embed.File{Name: name, URL: url, Size: size, SHA256: releaseDigest(name, digest)}
}

func verityFile(name, url string, size int64, digest, verity string) embed.File {
	f := file(name, url, size, digest)
	f.VeritySHA256 = releaseDigest(name, verity)
	return f
}

func releaseDigest(name, encoded string) [32]byte {
	d, err := hex.DecodeString(encoded)
	if err != nil || len(d) != sha256.Size {
		panic("gemma2: invalid release digest for " + name)
	}
	return [32]byte(d)
}

var sourceFiles = []embed.File{
	file("model.safetensors", sourceURL+"model.safetensors", 1488915288, "197a32965d4b1105faf060417baa899e193fb73cd401f42ec9295234d5553d79"),
	file("tokenizer.json", sourceURL+"tokenizer.json", 32170510, "4d777ef5bdc1aa36227abdfb77c3e49e7b9c892d16e1b6bda41c393504828be4"),
	file("config.json", sourceURL+"config.json", 4455, "b8f1e9931b57fbc054acdb445c41765d55b0074c58d145fa82839941ad1b5bb3"),
	file("config_sentence_transformers.json", sourceURL+"config_sentence_transformers.json", 1565, "031e56a498d33c349ab489a21885bcfe25b4fcba841149dc99e1e90d4a7c28f5"),
}
var packFiles = []embed.File{
	verityFile("tokens.bf16", "", 268435456, "56ebbdbfd706c827f3fae1d97aa39135c7ddc0f1e6d9b12f8565f84bbfbda877", "dac0921910db6362db3544de4a6b1f6196d960a4b373e13ec7d98a9c5faf1891"),
	verityFile("layers.safetensors", "", 273620496, "28a9678df941bf3eb7acca4e27ffd2fba6eef24039374d62148d705fc83b6227", "3735be2e5c7c58bc82eea55ead0084e4254ea3256655c42be2fa3670ccfd1378"),
	verityFile("tokenizer.bin", "", 9581663, "45a7e7d5a1e84ad83ef469a8f6aecc3dd15f51b7c4866008e2d770142d96b709", "43efd96ffb21f2f54082ab07c65c36deca41f7f4d1ba77be04a11a8e8d90b8c1"),
	verityFile("config.json", "", 3442, "65ef03947e006ea2a946fa323c88dd880ad6522d628723e81ddce04c2868db82", "f7172be07f967450c1164cfdbe5f6be3a5fea4ecee664efc31d3a94f521f880c"),
}
var fingerprint = modelFingerprint("embeddinggemma-2", packFiles)

func modelFingerprint(name string, files []embed.File) string {
	var s strings.Builder
	s.WriteString(name + "\n")
	for _, f := range files {
		s.WriteString(f.Name + "\n" + hex.EncodeToString(f.SHA256[:]) + "\n")
	}
	s.WriteString(strconv.Itoa(dimension) + "\n" + preparation)
	h := sha256.Sum256([]byte(s.String()))
	return hex.EncodeToString(h[:])
}

// Entry returns the release model with its cutoff from the frozen retrieval fixture.
func Entry() embed.Entry {
	return embed.Entry{Model: embed.Model{Name: "embeddinggemma-2", Runtime: "goinfer", Dim: dimension, Cutoff: 0.72, Fingerprint: fingerprint,
		Files: slices.Clone(packFiles), Source: slices.Clone(sourceFiles)}, Open: open, Devices: listDevices, Derive: derive}
}
