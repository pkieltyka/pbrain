//go:build cuda

// This replay loads either model and times only goinfer's prepared-ID forwards.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/pkieltyka/goinfer/backend/cuda"
	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/goinfer/tokenizer"
)

func main() {
	format := flag.String("format", "pack", "pack (Google) or gguf (Q8_0)")
	model := flag.String("model", "", "pack directory or GGUF file")
	ids := flag.String("ids", "", "TSV prepared IDs grouped by forward")
	device := flag.String("device", "", "CUDA persistent device identifier")
	flag.Parse()
	if os.Getenv("PBRAIN_ALLOW_GEMMA2") != "1" {
		log.Fatal("set PBRAIN_ALLOW_GEMMA2=1 to run the real model")
	}
	log.SetFlags(0)
	if err := run(*format, *model, *ids, *device); err != nil {
		log.Fatal(err)
	}
}

func run(format, path, ids, device string) (err error) {
	groups, err := readGroups(ids)
	if err != nil {
		return err
	}
	e, err := load(format, path)
	if err != nil {
		return err
	}
	acc, err := e.UseDevice(device, "cuda")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, acc.Close()) }()

	out := json.NewEncoder(os.Stdout)
	var total time.Duration
	var inputs, positions int
	for i, group := range groups {
		count, longest := 0, 0
		for _, ids := range group {
			count += len(ids)
			longest = max(longest, len(ids))
		}
		started := time.Now()
		vectors, err := e.EmbedIDsBatch(group)
		elapsed := time.Since(started)
		if err != nil {
			return fmt.Errorf("forward %d: %w", i+1, err)
		}
		if len(vectors) != len(group) {
			return fmt.Errorf("forward %d: returned %d vectors for %d inputs", i+1, len(vectors), len(group))
		}
		if err := out.Encode(map[string]any{
			"forward": i + 1, "inputs": len(group), "total_positions": count,
			"max_positions": longest, "ms": float64(elapsed) / float64(time.Millisecond),
		}); err != nil {
			return err
		}
		total += elapsed
		inputs += len(group)
		positions += count
	}
	return out.Encode(map[string]any{
		"forwards": len(groups), "inputs": inputs, "total_positions": positions,
		"forward_ms":           float64(total) / float64(time.Millisecond),
		"positions_per_second": float64(positions) / total.Seconds(),
		"passages_per_second":  float64(inputs) / total.Seconds(),
	})
}

func readGroups(path string) ([][][]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(nil, 128<<10)
	if !s.Scan() || s.Text() != "batch\tids" {
		return nil, errors.New("expected a batch and IDs TSV header")
	}
	var groups [][][]int
	for row := 2; s.Scan(); row++ {
		values := strings.Fields(s.Text())
		if len(values) < 2 {
			return nil, fmt.Errorf("line %d: expected a batch number and input IDs", row)
		}
		batch, err := strconv.Atoi(values[0])
		if err != nil || batch < max(len(groups), 1) || batch > len(groups)+1 {
			return nil, fmt.Errorf("line %d: nonconsecutive batch number", row)
		}
		if batch > len(groups) {
			groups = append(groups, nil)
		}
		input := make([]int, len(values)-1)
		for i, token := range values[1:] {
			id, err := strconv.Atoi(token)
			if err != nil || id < 0 {
				return nil, fmt.Errorf("line %d: invalid token ID %q", row, token)
			}
			input[i] = id
		}
		groups[batch-1] = append(groups[batch-1], input)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, errors.New("prepared IDs contain no groups")
	}
	return groups, nil
}

func load(format, path string) (*embeddinggemma2.Encoder, error) {
	switch format {
	case "gguf":
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		stat, err := f.Stat()
		if err != nil {
			return nil, err
		}
		return embeddinggemma2.LoadGGUFEncoderReaderAt(f, stat.Size(), nil)
	case "pack":
		return loadPack(path)
	default:
		return nil, fmt.Errorf("unknown model format %q", format)
	}
}

func loadPack(dir string) (*embeddinggemma2.Encoder, error) {
	var conf struct {
		Config  json.RawMessage   `json:"config"`
		Prompts map[string]string `json:"prompts"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &conf); err != nil {
		return nil, err
	}
	cfg, err := embeddinggemma2.LoadConfigBytes(conf.Config)
	if err != nil {
		return nil, err
	}
	tokens, err := os.ReadFile(filepath.Join(dir, "tokens.bf16"))
	if err != nil {
		return nil, err
	}
	layers, err := os.ReadFile(filepath.Join(dir, "layers.safetensors"))
	if err != nil {
		return nil, err
	}
	model, err := embeddinggemma2.NewModel(cfg, tokens, layers)
	if err != nil {
		return nil, err
	}
	pack, err := os.ReadFile(filepath.Join(dir, "tokenizer.bin"))
	if err != nil {
		return nil, err
	}
	tok, err := tokenizer.LoadPack(pack)
	if err != nil {
		return nil, err
	}
	return embeddinggemma2.NewEncoder(model, tok, conf.Prompts)
}
