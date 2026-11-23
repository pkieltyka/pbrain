package gemma2

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/trace"
)

const (
	batchInputs            = 256
	batchPositions         = 8192       // qualified aggregate ceiling on both GPU backends
	bytesPerPosition       = 136_000    // measured CUDA batch working set, goinfer v0.5.0
	vulkanBytesPerPosition = 4 * 26_882 // float32 buffers in the Vulkan forward graph
	gib                    = 1 << 30
	budgetCeiling          = 6 * gib
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

// batchSize admits increasing prefixes on the currently attached encoder.
// A valid singleton runs even if its workspace exceeds the budget.
func batchSize(ctx context.Context, lengths []int, info embed.RunInfo, retryLimit *int,
	estimate func([]int) (embeddinggemma2.BatchMemoryEstimate, error)) (int, error) {
	vulkan := strings.HasPrefix(info.Device, "vulkan:")
	positions := 0
	for i, length := range lengths[:min(batchInputs, len(lengths))] {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if length <= 0 || length > maxTokens {
			return 0, fmt.Errorf("invalid prepared length %d at input %d", length, i)
		}
		positions += length
		if i > 0 && (positions > batchPositions || retryLimit != nil && positions > *retryLimit) {
			return i, nil
		}
		memory, err := estimate(lengths[:i+1])
		if cerr := ctx.Err(); cerr != nil {
			return 0, cerr
		}
		if i > 0 && errors.Is(err, embeddinggemma2.ErrUnsupportedBatchShape) {
			return i, nil
		}
		if err != nil {
			return 0, err
		}
		bytes := int64(positions) * bytesPerPosition
		if vulkan {
			bytes = int64(positions) * vulkanBytesPerPosition
		}
		switch memory.Quality {
		case embeddinggemma2.BatchMemoryUpperBound, embeddinggemma2.BatchMemoryApproximate:
			bytes = memory.AdditionalPeakBytes
		case embeddinggemma2.BatchMemoryUnknown:
		default:
			return 0, fmt.Errorf("unrecognized batch memory estimate quality %d", memory.Quality)
		}
		if i > 0 && bytes > info.Budget {
			return i, nil
		}
	}
	return min(batchInputs, len(lengths)), nil
}

func (e *encoder) embedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	prepare := trace.Start(ctx, "token_prepare")
	defer prepare.End()
	ids := make([][]int, len(texts))
	lengths := make([]int, len(texts))
	for i, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var err error
		if ids[i], err = e.tokenIDs(embed.Document, text); err != nil {
			return nil, err
		}
		lengths[i] = len(ids[i])
	}
	prepare.EndWith(trace.Fields{Inputs: len(ids)})

	out := make([][]float32, len(ids))
	if err := e.embedPrepared(ctx, ids, lengths, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (e *encoder) embedPrepared(ctx context.Context, ids [][]int, lengths []int, out [][]float32) error {
	for len(ids) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		n := 1
		if e.model.Accelerator() != nil {
			var err error
			pack := trace.Start(ctx, "pack")
			n, err = batchSize(ctx, lengths, e.info, e.retryLimit, e.model.BatchMemory)
			pack.EndWith(trace.Fields{Inputs: n, Failed: err != nil})
			if err != nil {
				if cerr := ctx.Err(); cerr != nil {
					return cerr
				}
				return forwardError("batch memory", err)
			}
		}
		if n == 1 {
			v, err := e.embedIDs(ctx, ids[0])
			if err != nil {
				return err
			}
			out[0] = v
		} else if err := e.embedBatch(ctx, ids[:n], lengths[:n], out[:n]); err != nil {
			return err
		}
		ids, lengths, out = ids[n:], lengths[n:], out[n:]
	}
	return nil
}

func (e *encoder) embedBatch(ctx context.Context, ids [][]int, lengths []int, out [][]float32) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fields := trace.Fields{Device: e.info.Device, Inputs: len(ids)}
	if trace.Active(ctx) {
		for _, n := range lengths {
			fields.TotalPositions += n
			fields.MaxPositions = max(fields.MaxPositions, n)
		}
	}
	forward := trace.Start(ctx, "forward")
	vectors, err := e.model.EmbedIDsBatch(ids)
	fields.Failed = err != nil
	forward.EndWith(fields)
	if cerr := ctx.Err(); cerr != nil {
		return cerr
	}
	if retryableForward(err) {
		// Scope the retry to the failed range. Later siblings get a fresh
		// estimate and retain any smaller limit learned by earlier children.
		positions := 0
		for _, length := range lengths {
			positions += length
		}
		limit := positions / 2
		if e.retryLimit != nil {
			limit = min(limit, *e.retryLimit)
		}
		e.retryLimit = &limit
		return e.embedPrepared(ctx, ids, lengths, out)
	}
	if err != nil {
		return forwardError("batch forward", err)
	}
	if len(vectors) != len(ids) {
		return fmt.Errorf("%w: batch forward returned %d vectors for %d inputs", embed.ErrAccelerator, len(vectors), len(ids))
	}
	copy(out, vectors)
	return nil
}
