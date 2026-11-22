package gemma2

import (
	"context"
	"fmt"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

const (
	batchInputs      = 256
	bytesPerPosition = 136_000 // measured CUDA batch working set, goinfer v0.5.0
	gib              = 1 << 30
	budgetCeiling    = 6 * gib
)

func memoryBudget(memory embed.DeviceInfo, override int64) int64 {
	free := max(memory.FreeBytes, 0)
	if override > 0 {
		budget := min(override, budgetCeiling)
		if memory.Memory == "live" {
			return min(budget, free)
		}
		return budget
	}
	share := free/10*3 + free%10*3/10 // 30%, rounded down without an overflowing product
	switch memory.Memory {
	case "live":
		return min(share, budgetCeiling)
	case "estimated":
		return min(share, gib)
	default:
		return gib
	}
}

// batchSize takes a prefix of the remaining prepared texts. The first text is
// always admitted, even if its tokens alone exceed the working-set budget.
func batchSize(ids [][]int, positions int) int {
	positions = min(positions, embeddinggemma2.MaxTokens)
	tokens, n := len(ids[0]), 1
	for n < min(batchInputs, len(ids)) && tokens+len(ids[n]) <= positions {
		tokens += len(ids[n])
		n++
	}
	return n
}

func (e *encoder) embedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	ids := make([][]int, len(texts))
	for i, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var err error
		if ids[i], err = e.tokenIDs(embed.Document, text); err != nil {
			return nil, err
		}
	}

	out := make([][]float32, 0, len(ids))
	positions := int(e.info.Budget / bytesPerPosition)
	for len(ids) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n := batchSize(ids, positions)
		if n == 1 {
			v, err := e.embedIDs(ctx, ids[0])
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		} else {
			vs, err := e.model.EmbedIDsBatch(ids[:n])
			if cerr := ctx.Err(); cerr != nil {
				return nil, cerr
			}
			if err != nil {
				return nil, fmt.Errorf("%w: batch forward: %w", embed.ErrAccelerator, err)
			}
			if len(vs) != n {
				return nil, fmt.Errorf("%w: batch forward returned %d vectors for %d inputs", embed.ErrAccelerator, len(vs), n)
			}
			out = append(out, vs...)
		}
		ids = ids[n:]
	}
	return out, nil
}
