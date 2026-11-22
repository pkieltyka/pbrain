package gemma2

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/goinfer/tokenizer"
	"github.com/pkieltyka/pbrain/internal/embed"
)

type fakeSingleAccel struct {
	name      string
	dim       int
	err       error
	closeErrs []error
	onForward func()
	calls     int
	gotIDs    [][]int
	closed    int
}

func (f *fakeSingleAccel) Name() string { return f.name }

func (f *fakeSingleAccel) hiddenFor(ids []int) []float32 {
	out := make([]float32, len(ids)*f.dim)
	for t, id := range ids {
		for j := range f.dim {
			out[t*f.dim+j] = float32((t+1)*(j+3)+id%11) * 0.25
		}
	}
	return out
}

func (f *fakeSingleAccel) Forward(ids []int, _ bool) ([]float32, [][]float32, error) {
	f.calls++
	f.gotIDs = append(f.gotIDs, slices.Clone(ids))
	if f.onForward != nil {
		f.onForward()
	}
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.hiddenFor(ids), nil, nil
}

func (f *fakeSingleAccel) ForwardEmbeds([]float32, int, bool) ([]float32, [][]float32, error) {
	return nil, nil, errors.New("unexpected ForwardEmbeds")
}

func (f *fakeSingleAccel) Close() error {
	f.closed++
	if len(f.closeErrs) > 0 {
		err := f.closeErrs[0]
		f.closeErrs = f.closeErrs[1:]
		return err
	}
	return nil
}

type fakeBatchAccel struct {
	fakeSingleAccel
	batchErr   error
	onBatch    func()
	batchCalls int
	gotBatches [][][]int
}

func (f *fakeBatchAccel) ForwardBatch(ids [][]int) ([][]float32, error) {
	f.batchCalls++
	cloned := make([][]int, len(ids))
	for i, in := range ids {
		cloned[i] = slices.Clone(in)
	}
	f.gotBatches = append(f.gotBatches, cloned)
	if f.onBatch != nil {
		f.onBatch()
	}
	if f.batchErr != nil {
		return nil, f.batchErr
	}
	out := make([][]float32, len(ids))
	for i, in := range ids {
		out[i] = f.hiddenFor(in)
	}
	return out, nil
}

func tinyEncoderForTest(t *testing.T) (*encoder, *embeddinggemma2.Encoder) {
	t.Helper()
	const (
		vocab = 270
		H     = 8
		I     = 16
		P     = 4
		L     = 1
		E     = 6
		nH    = 2
		nKV   = 1
		hd    = 4
	)
	cfgJSON := []byte(fmt.Sprintf(`{
		"model_type": "embedding_gemma2",
		"text_config": {
			"vocab_size": %d,
			"hidden_size": %d,
			"intermediate_size": %d,
			"num_hidden_layers": %d,
			"num_attention_heads": %d,
			"num_key_value_heads": %d,
			"head_dim": %d,
			"sliding_window": 3,
			"hidden_size_per_layer_input": %d,
			"embedding_dim": %d,
			"rms_norm_eps": 1e-6,
			"hidden_activation": "gelu_pytorch_tanh",
			"layer_types": ["sliding_attention"],
			"max_position_embeddings": 2048,
			"rope_parameters": {"sliding_attention": {"rope_theta": 10000.0, "rope_type": "default"}}
		}
	}`, vocab, H, I, L, nH, nKV, hd, P, E))
	cfg, err := embeddinggemma2.LoadConfigBytes(cfgJSON)
	if err != nil {
		t.Fatal(err)
	}
	type tspec struct {
		name  string
		shape []int
		val   float32
	}
	specs := []tspec{
		{"language_model.ple.per_layer_model_projection.weight", []int{L * P, H}, 0.1},
		{"language_model.ple.per_layer_projection_norm.weight", []int{P}, 1.0},
		{"language_model.norm.weight", []int{H}, 1.0},
		{"language_model.embedding_projection.weight", []int{E, H}, 0.2},
		{"language_model.layers.0.input_layernorm.weight", []int{H}, 1.0},
		{"language_model.layers.0.self_attn.q_proj.weight", []int{nH * hd, H}, 0.1},
		{"language_model.layers.0.self_attn.k_proj.weight", []int{nKV * hd, H}, 0.1},
		{"language_model.layers.0.self_attn.v_proj.weight", []int{nKV * hd, H}, 0.1},
		{"language_model.layers.0.self_attn.o_proj.weight", []int{H, nH * hd}, 0.1},
		{"language_model.layers.0.self_attn.q_norm.weight", []int{hd}, 1.0},
		{"language_model.layers.0.self_attn.k_norm.weight", []int{hd}, 1.0},
		{"language_model.layers.0.post_attention_layernorm.weight", []int{H}, 1.0},
		{"language_model.layers.0.pre_feedforward_layernorm.weight", []int{H}, 1.0},
		{"language_model.layers.0.mlp.gate_proj.weight", []int{I, H}, 0.1},
		{"language_model.layers.0.mlp.up_proj.weight", []int{I, H}, 0.1},
		{"language_model.layers.0.mlp.down_proj.weight", []int{H, I}, 0.1},
		{"language_model.layers.0.post_feedforward_layernorm.weight", []int{H}, 1.0},
		{"language_model.layers.0.ple_block.per_layer_input_gate.weight", []int{P, H}, 0.1},
		{"language_model.layers.0.ple_block.per_layer_projection.weight", []int{H, P}, 0.1},
		{"language_model.layers.0.ple_block.post_per_layer_input_norm.weight", []int{H}, 1.0},
		{"language_model.layers.0.layer_scalar", []int{1}, 1.0},
	}
	hdr := map[string]tensor{}
	var payload bytes.Buffer
	var b4 [4]byte
	for _, sp := range specs {
		n := 1
		for _, s := range sp.shape {
			n *= s
		}
		start := payload.Len()
		for i := range n {
			v := sp.val + float32(i%5)*0.01
			binary.LittleEndian.PutUint32(b4[:], math.Float32bits(v))
			payload.Write(b4[:])
		}
		hdr[sp.name] = tensor{DType: "F32", Shape: sp.shape, Offsets: []int{start, payload.Len()}}
	}
	meta, err := json.Marshal(hdr)
	if err != nil {
		t.Fatal(err)
	}
	for len(meta)%8 != 0 {
		meta = append(meta, ' ')
	}
	layersFile := make([]byte, 8, 8+len(meta)+payload.Len())
	binary.LittleEndian.PutUint64(layersFile, uint64(len(meta)))
	layersFile = append(layersFile, meta...)
	layersFile = append(layersFile, payload.Bytes()...)

	tokensFile := make([]byte, vocab*H*4)
	for i := range vocab * H {
		binary.LittleEndian.PutUint32(tokensFile[i*4:], math.Float32bits(0.05+float32(i%7)*0.02))
	}
	m, err := embeddinggemma2.NewModel(cfg, tokensFile, layersFile)
	if err != nil {
		t.Fatal(err)
	}

	vocabMap := map[string]int{"<bos>": 0, "<eos>": 1, "<unk>": 2, "<start_of_turn>": 3}
	var added []map[string]any
	for word, id := range vocabMap {
		added = append(added, map[string]any{"id": id, "content": word, "special": true})
	}
	for i := range 256 {
		vocabMap[fmt.Sprintf("<0x%02X>", i)] = 4 + i
	}
	vocabMap["▁"], vocabMap["z"], vocabMap["zz"] = 260, 261, 262
	tokRaw, err := json.Marshal(map[string]any{
		"added_tokens": added,
		"model":        map[string]any{"type": "BPE", "vocab": vocabMap, "byte_fallback": true, "unk_token": "<unk>", "merges": [][]string{{"z", "z"}}},
		"normalizer":   map[string]any{"type": "Replace", "pattern": map[string]string{"String": " "}, "content": "▁"},
		"decoder":      map[string]any{"type": "ByteFallback"},
	})
	if err != nil {
		t.Fatal(err)
	}
	pack, err := tokenizer.BuildPack(tokRaw)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := tokenizer.LoadPack(pack)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := embeddinggemma2.NewEncoder(m, tok, map[string]string{"query": queryPrompt, "document": docPrompt})
	if err != nil {
		t.Fatal(err)
	}
	return &encoder{model: enc, tokenizer: tok, bos: 0, eos: 1}, enc
}

func TestGemmaAcceleratorDispatchBatchingAndCloseRetry(t *testing.T) {
	e, enc := tinyEncoderForTest(t)

	// 1. CPU path (no accelerator attached): bit-identical to Model.Embed(tokenIDs), empty input returns empty slice.
	emptyOut, err := e.Embed(t.Context(), embed.Document, nil)
	if err != nil || len(emptyOut) != 0 {
		t.Fatalf("CPU empty Embed = (%v, %v), want empty slice, nil", emptyOut, err)
	}
	cpuTexts := []string{"alpha", "beta <eos>", "gamma"}
	cpuVecs, err := e.Embed(t.Context(), embed.Document, cpuTexts)
	if err != nil {
		t.Fatalf("CPU Embed: %v", err)
	}
	for i, txt := range cpuTexts {
		ids, err := e.tokenIDs(embed.Document, txt)
		if err != nil {
			t.Fatal(err)
		}
		want, err := enc.Model().Embed(ids)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(cpuVecs[i], want) {
			t.Fatalf("CPU Embed[%d] differs from Model.Embed(tokenIDs)", i)
		}
	}

	// 2. Attached batch-capable accelerator:
	//    - Document batch of 8 makes 4 native pairs (0 singleton Forward calls)
	//    - Document batch of 5 makes 2 native pairs + 1 singleton tail Forward
	//    - Query batch of 2 makes 2 singleton Forward calls (0 ForwardBatch calls)
	//    - All outputs match single EmbedIDs bit-for-bit and differ from CPU
	fake := &fakeBatchAccel{fakeSingleAccel: fakeSingleAccel{name: "fake-prod-batch", dim: enc.HiddenDim()}}
	embeddinggemma2.RegisterAccelerator(fake.name, func(*embeddinggemma2.Model) (embeddinggemma2.Accelerator, error) {
		return fake, nil
	})
	if _, err := enc.UseAccelerator(fake.name); err != nil {
		t.Fatalf("UseAccelerator: %v", err)
	}

	eight := []string{"p0", "p1", "p2", "p3", "p4", "p5", "p6", "p7"}
	wantIDs := make([][]int, len(eight))
	wantSingle := make([][]float32, len(eight))
	for i, txt := range eight {
		ids, err := e.tokenIDs(embed.Document, txt)
		if err != nil {
			t.Fatal(err)
		}
		wantIDs[i] = ids
		v, err := enc.EmbedIDs(ids)
		if err != nil {
			t.Fatal(err)
		}
		wantSingle[i] = v
	}
	e.info.Budget = int64(len(wantIDs[0])+len(wantIDs[1])) * bytesPerPosition
	fake.calls, fake.batchCalls, fake.gotIDs, fake.gotBatches = 0, 0, nil, nil

	got8, err := e.Embed(t.Context(), embed.Document, eight)
	if err != nil {
		t.Fatalf("accelerated 8-passage Document Embed: %v", err)
	}
	if fake.batchCalls != 4 || fake.calls != 0 {
		t.Fatalf("8-passage Document: batchCalls=%d singleCalls=%d, want 4 and 0", fake.batchCalls, fake.calls)
	}
	for i := range eight {
		if !slices.Equal(got8[i], wantSingle[i]) {
			t.Fatalf("8-passage Document[%d] differs from single EmbedIDs", i)
		}
	}
	if slices.Equal(got8[0], cpuVecs[0]) {
		t.Fatal("accelerated output unexpectedly matched CPU output")
	}

	// 5 passages => 2 pairs + 1 singleton tail.
	fake.calls, fake.batchCalls, fake.gotIDs, fake.gotBatches = 0, 0, nil, nil
	got5, err := e.Embed(t.Context(), embed.Document, eight[:5])
	if err != nil {
		t.Fatalf("accelerated 5-passage Document Embed: %v", err)
	}
	if fake.batchCalls != 2 || fake.calls != 1 {
		t.Fatalf("5-passage Document: batchCalls=%d singleCalls=%d, want 2 and 1", fake.batchCalls, fake.calls)
	}
	if !slices.Equal(fake.gotBatches[0][0], wantIDs[0]) || !slices.Equal(fake.gotBatches[0][1], wantIDs[1]) ||
		!slices.Equal(fake.gotBatches[1][0], wantIDs[2]) || !slices.Equal(fake.gotBatches[1][1], wantIDs[3]) ||
		!slices.Equal(fake.gotIDs[0], wantIDs[4]) {
		t.Fatal("5-passage Document dispatched unexpected prepared IDs")
	}
	for i := range 5 {
		if !slices.Equal(got5[i], wantSingle[i]) {
			t.Fatalf("5-passage Document[%d] differs from single EmbedIDs", i)
		}
	}

	// Query role never pairs: 2 queries => 2 singleton Forward calls, 0 ForwardBatch calls.
	fake.calls, fake.batchCalls, fake.gotIDs, fake.gotBatches = 0, 0, nil, nil
	if _, err := e.Embed(t.Context(), embed.Query, []string{"q1", "q2"}); err != nil {
		t.Fatalf("accelerated Query Embed: %v", err)
	}
	if fake.batchCalls != 0 || fake.calls != 2 {
		t.Fatalf("Query role: batchCalls=%d singleCalls=%d, want 0 and 2", fake.batchCalls, fake.calls)
	}
	budget := e.info.Budget
	e.info.Budget = 0
	fake.calls, fake.batchCalls = 0, 0
	if got, err := e.Embed(t.Context(), embed.Document, eight[:2]); err != nil || len(got) != 2 ||
		fake.calls != 2 || fake.batchCalls != 0 {
		t.Fatalf("zero-budget documents = %d vectors, %v, single=%d batch=%d", len(got), err, fake.calls, fake.batchCalls)
	}
	e.info.Budget = budget

	// 3. ForwardBatch error wraps embed.ErrAccelerator + cause, never retries singletons or CPU, returns nil.
	batchBoom := errors.New("batch device boom")
	fake.batchErr = batchBoom
	fake.calls, fake.batchCalls = 0, 0
	if got, err := e.Embed(t.Context(), embed.Document, []string{"a", "b"}); got != nil || !errors.Is(err, embed.ErrAccelerator) || !errors.Is(err, batchBoom) {
		t.Fatalf("ForwardBatch error = (%v, %v), want nil + ErrAccelerator + %v", got, err, batchBoom)
	}
	if fake.calls != 0 || fake.batchCalls != 1 {
		t.Fatalf("ForwardBatch error retried: single=%d batch=%d", fake.calls, fake.batchCalls)
	}
	fake.batchErr = nil

	// 4. Singleton Forward error wraps embed.ErrAccelerator + cause.
	singleBoom := errors.New("single device boom")
	fake.err = singleBoom
	if got, err := e.Embed(t.Context(), embed.Query, []string{"q"}); got != nil || !errors.Is(err, embed.ErrAccelerator) || !errors.Is(err, singleBoom) {
		t.Fatalf("Forward error = (%v, %v), want nil + ErrAccelerator + %v", got, err, singleBoom)
	}
	fake.err = nil

	// A lost device is reported with its native cause and no further forwards.
	lost := &embeddinggemma2.DeviceLostError{Cause: errors.New("test device reset")}
	fake.err, fake.batchErr = lost, lost
	for _, texts := range [][]string{{"a"}, {"a", "b", "c"}} {
		fake.calls, fake.batchCalls = 0, 0
		if got, err := e.Embed(t.Context(), embed.Document, texts); got != nil ||
			!errors.Is(err, embed.ErrDeviceLost) || !errors.Is(err, embed.ErrAccelerator) || !errors.Is(err, lost) {
			t.Fatalf("lost device for %d texts = (%v, %v), want no vectors and the original loss", len(texts), got, err)
		}
		if fake.calls+fake.batchCalls != 1 {
			t.Fatalf("lost device retried: single=%d batch=%d", fake.calls, fake.batchCalls)
		}
	}
	fake.err, fake.batchErr = nil, nil

	// 5. Context cancellation during forward takes precedence over embed.ErrAccelerator.
	ctx, cancel := context.WithCancel(t.Context())
	fake.onBatch = cancel
	fake.batchErr = batchBoom
	if _, err := e.Embed(ctx, embed.Document, []string{"c1", "c2"}); !errors.Is(err, context.Canceled) || errors.Is(err, embed.ErrAccelerator) {
		t.Fatalf("canceled batch forward = %v, want context.Canceled without ErrAccelerator", err)
	}
	fake.onBatch = nil
	fake.batchErr = nil

	// 6. Close retains handle on failed drain so a retry succeeds, and a third Close is a clean no-op.
	drainErr := errors.New("stream drain failed")
	fake.closeErrs = []error{drainErr, nil}
	if err := e.Close(); !errors.Is(err, embed.ErrAccelerator) || !errors.Is(err, drainErr) {
		t.Fatalf("first Close = %v, want ErrAccelerator + %v", err, drainErr)
	}
	if e.model == nil || fake.closed != 1 {
		t.Fatalf("failed Close cleared model=%v or wrong close count=%d", e.model == nil, fake.closed)
	}
	if err := e.Close(); err != nil {
		t.Fatalf("retry Close = %v, want nil", err)
	}
	if e.model != nil || fake.closed != 2 {
		t.Fatalf("retry Close left model=%v or wrong close count=%d", e.model != nil, fake.closed)
	}
	if err := e.Close(); err != nil || fake.closed != 2 {
		t.Fatalf("third Close = %v (count=%d), want nil and count 2", err, fake.closed)
	}

	// 7. Non-batch accelerator attached fails loud on Document pair with zero Forward calls.
	eNoBatch, encNoBatch := tinyEncoderForTest(t)
	nonBatch := &fakeSingleAccel{name: "fake-prod-nobatch", dim: encNoBatch.HiddenDim()}
	embeddinggemma2.RegisterAccelerator(nonBatch.name, func(*embeddinggemma2.Model) (embeddinggemma2.Accelerator, error) {
		return nonBatch, nil
	})
	if _, err := encNoBatch.UseAccelerator(nonBatch.name); err != nil {
		t.Fatalf("UseAccelerator(nonBatch): %v", err)
	}
	eNoBatch.info.Budget = budget
	if got, err := eNoBatch.Embed(t.Context(), embed.Document, []string{"d1", "d2"}); got != nil || !errors.Is(err, embed.ErrAccelerator) || !strings.Contains(err.Error(), "cannot run a batch") {
		t.Fatalf("non-batch Document pair = (%v, %v), want nil + ErrAccelerator + cannot run a batch", got, err)
	}
	if nonBatch.calls != 0 {
		t.Fatalf("non-batch accelerator called Forward %d times, want 0", nonBatch.calls)
	}
	if err := eNoBatch.Close(); err != nil {
		t.Fatalf("nonBatch Close: %v", err)
	}
}
