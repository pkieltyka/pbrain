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
)

const maxTokens = 2048

type encoder struct {
	model     *embeddinggemma2.Encoder
	tokenizer *tokenizer.Tokenizer
	bos, eos  int
	info      embed.RunInfo
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
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// The three float32 feed-forward matrices per layer are a lower bound on CUDA's upload.
	weightFloor := 3 * 4 * int64(cfg.Layers) * int64(cfg.Hidden) * int64(cfg.Intermediate)
	info, err := chooseRuntime(ctx, opt.Device, weightFloor, cudaCompiled, cudaDevices, func() error {
		return attachAccelerator(m)
	})
	if err != nil {
		return nil, err
	}
	if info.Device != "cpu" && ctx.Err() == nil {
		var memory embed.DeviceInfo
		if devices := cudaDevices(); len(devices) > 0 {
			memory = devices[0]
		}
		info.Budget = memoryBudget(memory, opt.MemoryBudget)
	}
	bos, _ := tok.TokenID("<bos>")
	eos, _ := tok.TokenID("<eos>") // NewEncoder requires both.
	e := &encoder{model: m, tokenizer: tok, bos: bos, eos: eos, info: info}
	if err := ctx.Err(); err != nil {
		_ = e.Close()
		return nil, err
	}
	return e, nil
}

// chooseRuntime handles the default CUDA device supported by goinfer v0.5.0.
// The registry has already resolved and validated any explicit GPU selector.
func chooseRuntime(ctx context.Context, selector string, weightFloor int64, compiled bool,
	probe func() []embed.DeviceInfo, attach func() error) (embed.RunInfo, error) {
	cpu := embed.RunInfo{Device: "cpu"}
	switch selector {
	case "":
		return cpu, nil
	case "cpu":
		cpu.Fallback = embed.FallbackAsked
		return cpu, nil
	case "auto":
		if !compiled {
			cpu.Fallback = embed.FallbackNoSupport
			return cpu, nil
		}
		devices := probe()
		if len(devices) == 0 {
			cpu.Fallback = embed.FallbackNoDevice
			return cpu, nil
		}
		d := devices[0]
		if d.Memory == "live" && d.FreeBytes < weightFloor {
			cpu.Fallback = embed.FallbackMemory
			return cpu, nil
		}
		if err := ctx.Err(); err != nil {
			return embed.RunInfo{}, err
		}
		if err := attach(); err != nil {
			cpu.Fallback = embed.FallbackFailed
			return cpu, nil
		}
		return embed.RunInfo{Device: d.ID, Name: d.Name}, nil
	default:
		if err := attach(); err != nil {
			return embed.RunInfo{}, err
		}
		return embed.RunInfo{Device: selector}, nil
	}
}

func (e *encoder) Dim() int            { return dimension }
func (e *encoder) Fingerprint() string { return fingerprint }
func (e *encoder) Info() embed.RunInfo { return e.info }

func (e *encoder) Window() int {
	if e.model != nil && e.model.Accelerator() != nil {
		return batchInputs
	}
	return 0
}

func (e *encoder) Close() error {
	if e.model != nil {
		if acc := e.model.Accelerator(); acc != nil {
			if err := acc.Close(); err != nil {
				// Retain handles on failed drain so a retry can close the accelerator cleanly.
				return fmt.Errorf("%w: close %s: %w", embed.ErrAccelerator, acc.Name(), err)
			}
		}
	}
	e.model = nil
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
		ids, err := e.tokenIDs(role, text)
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
	v, err := e.model.EmbedIDs(ids)
	if cerr := ctx.Err(); cerr != nil {
		return nil, cerr
	}
	if err != nil && e.model.Accelerator() != nil {
		return nil, fmt.Errorf("%w: forward: %w", embed.ErrAccelerator, err)
	}
	return v, err
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
