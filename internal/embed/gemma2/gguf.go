package gemma2

import (
	"bytes"
	"context"
	"errors"
	"slices"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/trace"
)

const q8Name = "embeddinggemma-2-q8-0"
const q8File = "embeddinggemma-2-Q8_0.gguf"
const q6Name = "embeddinggemma-2-ud-q6-k-xl"
const q6File = "embeddinggemma-2-UD-Q6_K_XL.gguf"
const ggufURL = "https://huggingface.co/unsloth/embeddinggemma-2-GGUF/resolve/031f0d4b35536f69ab3509d4893c923264fcf253/"

var q8Files = []embed.File{
	verityFile(q8File, ggufURL+q8File, 309855520, "6f1bd4ac6c5df7444f9cca7ca36cafe6cfa34cd6f49fefb1e0b4be8143aed8bc", "714c9bfa33d80385452f646ed15c1915cb7032e3ff529d862d2aa5fa5d3c32d1"),
}
var q6Files = []embed.File{
	verityFile(q6File, ggufURL+q6File, 248812032, "dc84f042bc3ffbe6122b3b38b2fb299d66f5f37b276c9d458db85d4a4f1e6ba3", "f076d236929c1d930d13bf8116f29176ba80db028956f522faf004ad1823f413"),
}
var q8Fingerprint = modelFingerprint(q8Name, q8Files)
var q6Fingerprint = modelFingerprint(q6Name, q6Files)

// GGUFEntries returns the qualified GGUF rows. Q8 additionally admits Metal.
func GGUFEntries() []embed.Entry {
	q8 := ggufEntry(q8Name, q8Files, q8Fingerprint, listQ8Devices)
	q8.MetalWriter = metalCompiled
	q8.Model.CUDAFP16Fingerprint = fp16Fingerprint(q8Fingerprint, goinferRevision)
	q8.Model.MetalFingerprints = [2]string{
		profileFingerprint(q8Fingerprint, metalStockProfile, goinferRevision),
		profileFingerprint(q8Fingerprint, metalCombinedProfile, goinferRevision),
	}
	return []embed.Entry{q8, ggufEntry(q6Name, q6Files, q6Fingerprint, listDevices)}
}

func ggufEntry(name string, files []embed.File, fingerprint string, devices func() embed.Listing) embed.Entry {
	return embed.Entry{
		Model: embed.Model{
			Name: name, Runtime: "goinfer", Dim: dimension, Cutoff: 0.72,
			Fingerprint: fingerprint, Files: slices.Clone(files),
		},
		Open: openGGUF(fingerprint, name == q8Name && metalCompiled), Devices: devices, ResolveFP16: fp16Resolver(),
	}
}

func openGGUF(fingerprint string, allowMetal bool) func(context.Context, *embed.Artifact, embed.OpenOptions) (embed.Embedder, error) {
	return func(ctx context.Context, a *embed.Artifact, opt embed.OpenOptions) (embed.Embedder, error) {
		files := a.Files
		a.Files = nil
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(files) != 1 {
			return nil, errors.New("gemma2: expected one verified GGUF file")
		}
		prompts := map[string]string{"query": queryPrompt, "document": docPrompt}
		reader := bytes.NewReader(files[0])
		decode := trace.Start(ctx, "model_decode")
		m, err := embeddinggemma2.LoadGGUFEncoderReaderAt(reader, reader.Size(), prompts)
		decode.End()
		if err != nil {
			return nil, err
		}
		if m.Model().Config().EmbeddingDim != dimension {
			return nil, errors.New("gemma2: unexpected GGUF vector dimension")
		}
		return newRuntime(ctx, m, prompts, opt, fingerprint, allowMetal)
	}
}
