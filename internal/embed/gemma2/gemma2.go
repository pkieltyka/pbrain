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

const (
	maxTokens     = 2048
	pairBatchSize = 2
)

type encoder struct {
	model     *embeddinggemma2.Encoder
	tokenizer *tokenizer.Tokenizer
	bos, eos  int
}

func open(ctx context.Context, a *embed.Artifact) (embed.Embedder, error) {
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
	if err := attachAccelerator(m); err != nil {
		return nil, err
	}
	bos, _ := tok.TokenID("<bos>")
	eos, _ := tok.TokenID("<eos>") // NewEncoder requires both.
	return &encoder{m, tok, bos, eos}, nil
}

func (e *encoder) Dim() int            { return dimension }
func (e *encoder) Fingerprint() string { return fingerprint }
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
	if err := requireAccelerator(e.model); err != nil {
		return nil, err
	}
	out := make([][]float32, len(texts))
	if len(texts) == 0 {
		return out, nil
	}
	hasAccel := e.model.Accelerator() != nil
	for i := 0; i < len(texts); {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if hasAccel && role == embed.Document && len(texts)-i >= pairBatchSize {
			ids0, err := e.tokenIDs(role, texts[i])
			if err != nil {
				return nil, err
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			ids1, err := e.tokenIDs(role, texts[i+1])
			if err != nil {
				return nil, err
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if total := len(ids0) + len(ids1); total > embeddinggemma2.MaxTokens {
				return nil, fmt.Errorf("%w: document pair has %d tokens, over %d cap", embed.ErrAccelerator, total, embeddinggemma2.MaxTokens)
			}
			vecs, err := e.model.EmbedIDsBatch([][]int{ids0, ids1})
			if cerr := ctx.Err(); cerr != nil {
				return nil, cerr
			}
			if err != nil {
				return nil, fmt.Errorf("%w: batch forward: %w", embed.ErrAccelerator, err)
			}
			if len(vecs) != pairBatchSize {
				return nil, fmt.Errorf("%w: batch forward returned %d vectors for %d inputs", embed.ErrAccelerator, len(vecs), pairBatchSize)
			}
			out[i], out[i+1] = vecs[0], vecs[1]
			i += pairBatchSize
			continue
		}
		ids, err := e.tokenIDs(role, texts[i])
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		v, err := e.model.EmbedIDs(ids)
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		if err != nil {
			if hasAccel {
				return nil, fmt.Errorf("%w: forward: %w", embed.ErrAccelerator, err)
			}
			return nil, err
		}
		out[i] = v
		i++
	}
	return out, nil
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
