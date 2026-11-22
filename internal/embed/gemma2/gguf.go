package gemma2

import (
	"bytes"
	"context"
	"errors"
	"slices"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

const q8Name = "embeddinggemma-2-q8-0"
const q8File = "embeddinggemma-2-Q8_0.gguf"
const ggufURL = "https://huggingface.co/unsloth/embeddinggemma-2-GGUF/resolve/031f0d4b35536f69ab3509d4893c923264fcf253/"

var q8Files = []embed.File{
	file(q8File, ggufURL+q8File, 309855520, "6f1bd4ac6c5df7444f9cca7ca36cafe6cfa34cd6f49fefb1e0b4be8143aed8bc"),
}
var q8Fingerprint = modelFingerprint(q8Name, q8Files)

// GGUFEntries returns the qualified GGUF model rows. Q8_0 supports CPU and CUDA.
func GGUFEntries() []embed.Entry {
	return []embed.Entry{{
		Model: embed.Model{
			Name: q8Name, Runtime: "goinfer", Dim: dimension, Cutoff: 0.72,
			Fingerprint: q8Fingerprint, Files: slices.Clone(q8Files),
		},
		Open: openQ8, Devices: listCUDADevices,
	}}
}

func openQ8(ctx context.Context, a *embed.Artifact, opt embed.OpenOptions) (embed.Embedder, error) {
	files := a.Files
	a.Files = nil
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(files) != 1 {
		return nil, errors.New("gemma2: expected one verified Q8_0 file")
	}
	prompts := map[string]string{"query": queryPrompt, "document": docPrompt}
	reader := bytes.NewReader(files[0])
	m, err := embeddinggemma2.LoadGGUFEncoderReaderAt(reader, reader.Size(), prompts)
	if err != nil {
		return nil, err
	}
	if m.Model().Config().EmbeddingDim != dimension {
		return nil, errors.New("gemma2: unexpected GGUF vector dimension")
	}
	return newRuntime(ctx, m, prompts, opt, q8Fingerprint, false)
}
