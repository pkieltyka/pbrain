// Adapted from aikit embed/safetensors.go.
package bert

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"unsafe"
)

type tensor struct {
	dtype string
	shape []int
	raw   []byte
}

type tensors map[string]tensor

func parseSafetensors(data []byte) (tensors, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("safetensors: missing header length")
	}
	length := binary.LittleEndian.Uint64(data)
	if length > uint64(len(data)-8) {
		return nil, fmt.Errorf("safetensors: header exceeds the file")
	}
	header, payload := data[8:8+length], data[8+length:]
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(header, &entries); err != nil {
		return nil, fmt.Errorf("safetensors: parse header: %w", err)
	}
	weights := make(tensors, len(entries))
	for name, value := range entries {
		if name == "__metadata__" {
			continue
		}
		var entry struct {
			DType   string `json:"dtype"`
			Shape   []int  `json:"shape"`
			Offsets []int  `json:"data_offsets"`
		}
		if err := json.Unmarshal(value, &entry); err != nil {
			return nil, fmt.Errorf("safetensors: parse tensor %q: %w", name, err)
		}
		width := 4
		switch entry.DType {
		case "F32":
		case "I64": // BGE's position_ids buffer is present but not used as weights.
			width = 8
		default:
			return nil, fmt.Errorf("safetensors: tensor %q has unsupported dtype %q", name, entry.DType)
		}
		if len(entry.Offsets) != 2 {
			return nil, fmt.Errorf("safetensors: tensor %q needs two offsets", name)
		}
		start, end := entry.Offsets[0], entry.Offsets[1]
		if start < 0 || start > end || end > len(payload) {
			return nil, fmt.Errorf("safetensors: tensor %q has invalid offsets %v", name, entry.Offsets)
		}
		bytes := end - start
		elements := 1
		for _, dim := range entry.Shape {
			if dim <= 0 || elements > (bytes/width)/dim {
				return nil, fmt.Errorf("safetensors: tensor %q shape %v exceeds its data range", name, entry.Shape)
			}
			elements *= dim
		}
		if bytes%width != 0 || elements != bytes/width {
			return nil, fmt.Errorf("safetensors: tensor %q shape %v does not match its data range", name, entry.Shape)
		}
		weights[name] = tensor{entry.DType, entry.Shape, payload[start:end]}
	}
	return weights, nil
}

func (weights tensors) floats(name string, want []int) ([]float32, error) {
	t, ok := weights[name]
	if !ok {
		return nil, fmt.Errorf("safetensors: missing tensor %q", name)
	}
	if t.dtype != "F32" || !slices.Equal(t.shape, want) {
		return nil, fmt.Errorf("safetensors: tensor %q is %s %v, want F32 %v", name, t.dtype, t.shape, want)
	}
	ptr := unsafe.Pointer(&t.raw[0])
	var one uint16 = 1
	if *(*byte)(unsafe.Pointer(&one)) == 1 && uintptr(ptr)%unsafe.Alignof(float32(0)) == 0 {
		return unsafe.Slice((*float32)(ptr), len(t.raw)/4), nil
	}
	// Decode a misaligned payload (or a big-endian host) into aligned Go floats.
	decoded := make([]float32, len(t.raw)/4)
	for i := range decoded {
		decoded[i] = math.Float32frombits(binary.LittleEndian.Uint32(t.raw[4*i:]))
	}
	return decoded, nil
}
