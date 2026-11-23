package gemma2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/goinfer/tokenizer"
	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/trace"
)

const maxTokens = 2048

type encoder struct {
	model      *embeddinggemma2.Encoder
	cpu        *embeddinggemma2.Encoder // reserved for an auto-selected accelerator
	tokenizer  *tokenizer.Tokenizer
	bos, eos   int
	info       embed.RunInfo
	fp         string
	retryLimit *int // maximum aggregate positions after a recoverable OOM
}

type attachCleanupError struct {
	err error
	acc embeddinggemma2.Accelerator
}

func (e *attachCleanupError) Error() string { return e.err.Error() }
func (e *attachCleanupError) Unwrap() error { return e.err }

func attachDevice(model *embeddinggemma2.Encoder, device embed.DeviceInfo, override int64) (int64, error) {
	selector := device.Backend
	if selector == "cuda" {
		selector = cudaSelector
	}
	id := device.Selector()
	acc, err := model.UseDevice(id, selector)
	if err != nil {
		return 0, fmt.Errorf("%w: attach %s: %w", embed.ErrAccelerator, id, err)
	}
	budget, err := attachedBudget(acc, id, override)
	if err == nil {
		return budget, nil
	}
	if closeErr := acc.Close(); closeErr != nil {
		return 0, &attachCleanupError{
			err: errors.Join(err, fmt.Errorf("close %s: %w", id, closeErr)),
			acc: acc,
		}
	}
	return 0, err
}

func attachedBudget(acc embeddinggemma2.Accelerator, id string, override int64) (int64, error) {
	var actual string
	if reporter, ok := acc.(embeddinggemma2.AttachedDeviceReporter); ok {
		actual = reporter.AttachedDevice().ID
	}
	if actual != id {
		return 0, fmt.Errorf("%w: attached device %q for %q", embed.ErrAccelerator, actual, id)
	}
	var memory embed.DeviceInfo
	if reporter, ok := acc.(embeddinggemma2.DeviceMemoryReporter); ok {
		reading, err := reporter.DeviceMemory()
		if err != nil {
			return 0, fmt.Errorf("%w: read memory of %s: %w", embed.ErrAccelerator, id, err)
		}
		memory.Memory = reading.Basis.String()
		memory.FreeBytes = reading.AvailableBytes
	}
	return memoryBudget(memory, override), nil
}

func open(ctx context.Context, a *embed.Artifact, opt embed.OpenOptions) (embed.Embedder, error) {
	files := a.Files
	a.Files = nil
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(files) != 4 {
		return nil, errors.New("gemma2: expected four verified pack files")
	}
	decode := trace.Start(ctx, "model_decode")
	defer decode.End()
	var conf configuration
	if err := json.Unmarshal(files[3], &conf); err != nil {
		return nil, err
	}
	cfg, err := embeddinggemma2.LoadConfigBytes(conf.Config)
	if err != nil {
		return nil, err
	}
	if cfg.EmbeddingDim != dimension || conf.Prompts["query"] != queryPrompt || conf.Prompts["document"] != docPrompt {
		return nil, errors.New("gemma2: unexpected pack configuration")
	}
	var tok *tokenizer.Tokenizer
	var tokErr error
	done := make(chan struct{})
	go func() { defer close(done); tok, tokErr = tokenizer.LoadPack(files[2]) }()
	model, err := embeddinggemma2.NewModel(cfg, files[0], files[1])
	<-done
	if err != nil {
		return nil, err
	}
	if tokErr != nil {
		return nil, tokErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m, err := embeddinggemma2.NewEncoder(model, tok, conf.Prompts)
	decode.End()
	if err != nil {
		return nil, err
	}
	return newRuntime(ctx, m, conf.Prompts, opt, fingerprint, true)
}

func newRuntime(ctx context.Context, m *embeddinggemma2.Encoder, prompts map[string]string, opt embed.OpenOptions, fp string, allowVulkan bool) (embed.Embedder, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	model, tok := m.Model(), m.Tokenizer()
	cfg := model.Config()
	compiled, list := cudaCompiled, listCUDADevices
	if allowVulkan {
		compiled, list = cudaCompiled || vulkanCompiled, listDevices
	}
	// The three float32 feed-forward matrices per layer are a lower bound on CUDA's upload.
	weightFloor := 3 * 4 * int64(cfg.Layers) * int64(cfg.Hidden) * int64(cfg.Intermediate)
	var attached *embeddinggemma2.Encoder
	var budget int64
	info, err := chooseRuntime(ctx, opt.Device, weightFloor, compiled, list, func(device embed.DeviceInfo) error {
		candidate, err := embeddinggemma2.NewEncoder(model, tok, prompts)
		if err != nil {
			return err
		}
		attach := trace.Start(ctx, trace.DeviceAttach)
		budget, err = attachDevice(candidate, device, opt.MemoryBudget)
		attach.EndWith(trace.Fields{Device: device.ID, Failed: err != nil})
		if err != nil {
			return err
		}
		attached = candidate
		return nil
	})
	if err != nil {
		var cleanup *attachCleanupError
		if errors.As(err, &cleanup) {
			// The failed candidate cannot be used or replaced until its handle
			// has been released. Retain the original failure even if cleanup
			// succeeds on the second attempt.
			if closeErr := cleanup.acc.Close(); closeErr != nil {
				return nil, errors.Join(err, closeErr)
			}
		}
		return nil, err
	}
	bos, _ := tok.TokenID("<bos>")
	eos, _ := tok.TokenID("<eos>") // NewEncoder requires both.
	e := &encoder{model: m, tokenizer: tok, bos: bos, eos: eos, info: info, fp: fp}
	if attached != nil {
		e.model = attached
		e.info.Budget = budget
		if opt.Device == "auto" {
			e.cpu = m
		}
	}
	if err := ctx.Err(); err != nil {
		_ = e.Close()
		return nil, err
	}
	return e, nil
}

func (e *encoder) Dim() int            { return dimension }
func (e *encoder) Fingerprint() string { return e.fp }
func (e *encoder) Info() embed.RunInfo { return e.info }

func (e *encoder) Window() int {
	if e.model != nil && (e.model.Accelerator() != nil || e.cpu != nil) {
		return batchInputs
	}
	return 0
}

func (e *encoder) Close() error {
	if e.model != nil {
		if err := e.closeAccelerator(); err != nil {
			// Retain handles on failed drain so a retry can close the accelerator cleanly.
			return err
		}
	}
	e.model = nil
	e.cpu = nil
	e.tokenizer = nil
	return nil
}

func (e *encoder) Embed(ctx context.Context, role embed.Role, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if e.model == nil {
		return nil, errors.New("gemma2: encoder closed")
	}
	if role != embed.Query && role != embed.Document {
		return nil, errors.New("gemma2: unknown embedding role")
	}
	if role == embed.Document && len(texts) > 1 && e.model.Accelerator() != nil {
		return e.embedDocuments(ctx, texts)
	}
	out := make([][]float32, len(texts))
	for i, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		prepare := trace.Start(ctx, "token_prepare")
		ids, err := e.tokenIDs(role, text)
		prepare.EndWith(trace.Fields{Inputs: 1, Failed: err != nil})
		if err != nil {
			return nil, err
		}
		v, err := e.embedIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (e *encoder) embedIDs(ctx context.Context, ids []int) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	v, err := e.forwardSingle(ctx, ids)
	if cerr := ctx.Err(); cerr != nil {
		return nil, cerr
	}
	if err != nil && e.model.Accelerator() != nil {
		if e.cpu == nil || !retryableForward(err) {
			return nil, forwardError("forward", err)
		}
		closeErr := e.closeAccelerator()
		if closeErr != nil {
			if cerr := ctx.Err(); cerr != nil {
				return nil, cerr
			}
			return nil, forwardError("forward", errors.Join(err, closeErr))
		}
		e.model = e.cpu
		e.info = embed.RunInfo{Device: "cpu", Fallback: embed.FallbackMemory}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		v, err = e.forwardSingle(ctx, ids)
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
	}
	return v, err
}

func (e *encoder) forwardSingle(ctx context.Context, ids []int) ([]float32, error) {
	forward := trace.Start(ctx, "forward")
	v, err := e.model.EmbedIDs(ids)
	forward.EndWith(trace.Fields{
		Device: e.info.Device, Inputs: 1, TotalPositions: len(ids), MaxPositions: len(ids), Failed: err != nil,
	})
	return v, err
}

func retryableForward(err error) bool {
	return !errors.Is(err, embeddinggemma2.ErrDeviceLost) && errors.Is(err, embeddinggemma2.ErrBatchOutOfMemory)
}

func (e *encoder) closeAccelerator() error {
	acc := e.model.Accelerator()
	if acc == nil {
		return nil
	}
	if err := acc.Close(); err != nil {
		return fmt.Errorf("%w: close %s: %w", embed.ErrAccelerator, acc.Name(), err)
	}
	return nil
}

func forwardError(stage string, err error) error {
	category := embed.ErrAccelerator
	if errors.Is(err, embeddinggemma2.ErrDeviceLost) {
		category = embed.ErrDeviceLost
	}
	return fmt.Errorf("%w: %s: %w", category, stage, err)
}

func (e *encoder) ordinary(s string) ([]int, error) {
	return e.tokenizer.EncodeSegments([]tokenizer.Segment{{Text: s, Special: false}}, false)
}
func (e *encoder) tokenIDs(role embed.Role, text string) ([]int, error) {
	prompt := docPrompt
	if role == embed.Query {
		prompt = queryPrompt
	}
	ids, err := e.ordinary(prompt + text)
	if err != nil {
		return nil, err
	}
	const capacity = maxTokens - 2
	if len(ids) > capacity {
		head, body, split := strings.Cut(text, "\n\n")
		if role == embed.Document && split {
			prefix, err := e.ordinary(prompt + head + "\n\n")
			if err != nil {
				return nil, err
			}
			passage, err := e.ordinary(body)
			if err != nil {
				return nil, err
			}
			instruction, err := e.ordinary(prompt)
			if err != nil {
				return nil, err
			}
			room := capacity - len(instruction)
			keep := min(len(prefix), len(instruction)+max(room/2, room-len(passage)))
			ids = append(prefix[:keep], passage[:min(len(passage), capacity-keep)]...)
		} else {
			ids = ids[:capacity]
		}
	}
	out := make([]int, 1, len(ids)+2)
	out[0] = e.bos
	out = append(out, ids...)
	return append(out, e.eos), nil
}
