// Package bge adapts bge-small-en-v1.5 to pbrain's embedding interface.
package bge

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	tokenizer "github.com/pkieltyka/aikit/embed"
	"github.com/pkieltyka/aikit/encoder"
	"github.com/pkieltyka/pbrain/internal/embed"
)

const (
	queryInstruction = "Represent this sentence for searching relevant passages: "
	contentLimit     = 510 // CLS and SEP take the remaining two positions.
	embedWindow      = 128
	maxWorkers       = 32
)

var (
	loadBERT   = encoder.LoadBERTFromFS
	neutralize = strings.NewReplacer(
		"[PAD]", "[ PAD]", "[UNK]", "[ UNK]", "[CLS]", "[ CLS]",
		"[SEP]", "[ SEP]", "[MASK]", "[ MASK]",
	)
)

// handoff transfers the verified buffers to aikit. A path is readable once,
// and the weights can continue to alias their buffer after the load.
type handoff struct {
	files   map[string][]byte
	refused bool
}

func (h *handoff) Open(name string) (fs.File, error) {
	h.refused = true
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (h *handoff) ReadFile(name string) ([]byte, error) {
	data, ok := h.files[name]
	if !ok {
		h.refused = true
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	delete(h.files, name)
	return data, nil
}

func open(ctx context.Context, a *embed.Artifact, _ embed.OpenOptions) (embed.Embedder, error) {
	files := a.Files
	a.Files = nil
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(files) != len(modelFiles) {
		return nil, errors.New("bge: artifact must contain five files")
	}
	h := &handoff{files: make(map[string][]byte, len(modelFiles))}
	for i, file := range modelFiles {
		if len(files[i]) == 0 {
			return nil, fmt.Errorf("bge: artifact file %s is empty", file.name)
		}
		h.files[file.path] = files[i]
	}
	// The caller relinquishes the slices on Open. Only the handoff and, once
	// read, aikit own the buffers during the load.
	clear(files)
	model, err := loadBERT(h, ".")
	if err != nil {
		return nil, fmt.Errorf("bge: load model: %w", err)
	}
	if model == nil {
		return nil, errors.New("bge: loader returned no model")
	}
	if h.refused || len(h.files) != 0 {
		return nil, errors.Join(errors.New("bge: loader must read each model file exactly once"), model.Close())
	}
	e, err := newEmbedder(model)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return nil, errors.Join(err, model.Close())
	}
	return e, nil
}

type bertModel interface {
	Embed([]int32) []float32
	Close() error
}

type embedder struct {
	model     bertModel
	tokenizer *tokenizer.Tokenizer
	cls, sep  int32
}

func newEmbedder(model *encoder.BERT) (*embedder, error) {
	t := model.Tokenizer()
	if t == nil {
		return nil, errors.New("bge: model has no tokenizer")
	}
	cls, hasCLS := t.SpecialID("[CLS]")
	sep, hasSEP := t.SpecialID("[SEP]")
	if !hasCLS || !hasSEP {
		return nil, errors.New("bge: tokenizer lacks CLS or SEP")
	}
	return &embedder{model: model, tokenizer: t, cls: cls, sep: sep}, nil
}

func (e *embedder) Dim() int            { return modelRow.Dim }
func (e *embedder) Fingerprint() string { return modelRow.Fingerprint }
func (e *embedder) Info() embed.RunInfo { return embed.RunInfo{Device: "cpu"} }
func (e *embedder) Window() int         { return embedWindow }

func (e *embedder) Close() error {
	if e.model == nil {
		return nil
	}
	err := e.model.Close()
	e.model = nil
	e.tokenizer = nil
	return err
}

func (e *embedder) Embed(ctx context.Context, role embed.Role, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if e.model == nil {
		return nil, errors.New("bge: model is closed")
	}
	if role != embed.Query && role != embed.Document {
		return nil, errors.New("bge: unknown embedding role")
	}
	if workers := min(runtime.GOMAXPROCS(0), maxWorkers, len(texts)); workers > 1 {
		return embedBatch(ctx, len(texts), workers, func(ctx context.Context, i int) ([]float32, error) {
			ids := e.tokenIDs(role, texts[i])
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return e.model.Embed(ids), nil
		})
	}
	vectors := make([][]float32, len(texts))
	for i, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		vectors[i] = e.model.Embed(e.tokenIDs(role, text))
	}
	return vectors, nil
}

// embedBatch gives each worker one input and its output slot at a time. The
// caller can close the model after it returns because all forwards have joined.
func embedBatch(ctx context.Context, count, workers int, forward func(context.Context, int) ([]float32, error)) ([][]float32, error) {
	workCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	vectors := make([][]float32, count)
	var next atomic.Int64
	var group sync.WaitGroup
	for range workers {
		group.Go(func() {
			for workCtx.Err() == nil {
				i := int(next.Add(1) - 1)
				if i >= count {
					return
				}
				vector, err := forward(workCtx, i)
				if err != nil {
					cancel(err)
					return
				}
				vectors[i] = vector
			}
		})
	}
	group.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := context.Cause(workCtx); err != nil {
		return nil, err
	}
	return vectors, nil
}

func (e *embedder) tokenIDs(role embed.Role, text string) []int32 {
	text = neutralize.Replace(text)
	var prefix, passage []int32
	if role == embed.Query {
		passage = e.tokenizer.Encode(queryInstruction + text)
	} else if head, body, ok := strings.Cut(text, "\n\n"); ok {
		prefix = e.tokenizer.Encode(head)
		passage = e.tokenizer.Encode(body)
	} else {
		passage = e.tokenizer.Encode(text)
	}
	prefix = prefix[:min(len(prefix), max(contentLimit/2, contentLimit-len(passage)))]
	passage = passage[:min(len(passage), contentLimit-len(prefix))]
	ids := make([]int32, 1, len(prefix)+len(passage)+2)
	ids[0] = e.cls
	ids = append(ids, prefix...)
	ids = append(ids, passage...)
	return append(ids, e.sep)
}
