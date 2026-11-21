package item

import (
	"bytes"
	"reflect"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/errs"
	"go.yaml.in/yaml/v3"
)

type change struct {
	start, end int
	text       []byte
}

// EditVerify edits two top-level lines, preserving the rest of the file.
func EditVerify(data []byte, actor string) ([]byte, error) {
	if actor == "" {
		return nil, invalid("verified", "must not be empty")
	}
	if err := validateText("verified", actor, 200); err != nil {
		return nil, err
	}
	block, err := splitFrontMatter(data)
	if err != nil {
		return nil, unsupported("front matter")
	}
	root, err := readMapping(data, block)
	if err != nil || root != nil && root.Style&yaml.FlowStyle != 0 {
		return nil, unsupported("front matter")
	}

	eol := "\n"
	if bytes.HasSuffix(data[:block.start], []byte("\r\n")) {
		eol = "\r\n"
	}
	var status, verified change
	if root != nil {
		for i := 0; i < len(root.Content); i += 2 {
			key, value := root.Content[i], root.Content[i+1]
			if key.Kind != yaml.ScalarNode {
				continue
			}
			switch key.Value {
			case "status":
				status, err = editLine(data, block, key, value, "verified")
			case "verified":
				verified, err = editLine(data, block, key, value, scalarText(actor))
			}
			if err != nil {
				return nil, err
			}
		}
	}

	if status.text == nil {
		status = change{block.close, block.close, []byte("status: verified" + eol)}
	}
	if verified.text == nil {
		status.text = append(status.text, "verified: "...)
		status.text = append(status.text, scalarText(actor)...)
		status.text = append(status.text, eol...)
	}
	first, second := status, verified
	if second.text != nil && second.start < first.start {
		first, second = second, first
	}
	out := make([]byte, 0, len(data)+len(status.text)+len(verified.text))
	out = append(out, data[:first.start]...)
	out = append(out, first.text...)
	if second.text == nil {
		out = append(out, data[first.end:]...)
	} else {
		out = append(out, data[first.end:second.start]...)
		out = append(out, second.text...)
		out = append(out, data[second.end:]...)
	}

	original, _ := parseHeader("", data[block.body:], root)
	checked, bodyAt, _, err := Parse("", out)
	if err != nil || checked.Status != Verified || checked.Verified != actor ||
		!bytes.Equal(data[block.body:], out[bodyAt:]) {
		return nil, unsupported("front matter")
	}
	checked.Status, checked.StatusRaw, checked.Verified = original.Status, original.StatusRaw, original.Verified
	if !reflect.DeepEqual(checked, original) {
		return nil, unsupported("front matter")
	}
	return out, nil
}

func editLine(data []byte, block frontMatter, key, value *yaml.Node, replacement string) (change, error) {
	if key.Column != 1 || key.Line != value.Line || !singleScalar(value) {
		return change{}, unsupported(key.Value)
	}
	start := block.start
	for l := 1; l < key.Line && start < block.close; l++ {
		_, start = takeLine(data, start)
	}
	if start >= block.close {
		return change{}, unsupported(key.Value)
	}
	line, end := takeLine(data, start)
	for next := end; next < block.close; {
		following, after := takeLine(data, next)
		trimmed := bytes.TrimSpace(following)
		if len(trimmed) != 0 && trimmed[0] != '#' {
			if following[0] == ' ' || following[0] == '\t' {
				return change{}, unsupported(key.Value)
			}
			break
		}
		next = after
	}

	// YAML columns count code points. The raw bytes before the scalar or
	// empty value contain the mapping colon, even for a quoted key.
	at := 0
	for col := 1; col < value.Column && at < len(line); col++ {
		_, size := utf8.DecodeRune(line[at:])
		at += size
	}
	colon := bytes.LastIndexByte(line[:at], ':')
	if colon < 0 {
		return change{}, unsupported(key.Value)
	}
	raw := line[colon+1:]
	comment, ok := trailingComment(raw)
	if !ok {
		return change{}, unsupported(key.Value)
	}

	newLine := append([]byte(nil), line[:colon+1]...)
	newLine = append(newLine, ' ')
	newLine = append(newLine, replacement...)
	newLine = append(newLine, comment...)
	newLine = append(newLine, data[start+len(line):end]...)
	return change{start, end, newLine}, nil
}

func trailingComment(raw []byte) ([]byte, bool) {
	scalar := bytes.TrimLeft(raw, " \t")
	if len(scalar) == 0 {
		return nil, true
	}
	if scalar[0] == '#' {
		return raw, true
	}
	var end int
	switch scalar[0] {
	case '\'', '"':
		var ok bool
		end, ok = quotedEnd(scalar)
		if !ok {
			return nil, false
		}
	default:
		for end = 0; end < len(scalar); end++ {
			if scalar[end] == '#' && end > 0 && (scalar[end-1] == ' ' || scalar[end-1] == '\t') {
				break
			}
		}
		end = len(bytes.TrimRight(scalar[:end], " \t"))
	}
	rest := scalar[end:]
	if after := bytes.TrimLeft(rest, " \t"); len(after) != 0 {
		return rest, after[0] == '#'
	}
	return nil, true
}

func quotedEnd(s []byte) (int, bool) {
	quote := s[0]
	for i := 1; i < len(s); i++ {
		switch {
		case quote == '"' && s[i] == '\\':
			i++
		case s[i] == quote:
			if quote == '\'' && i+1 < len(s) && s[i+1] == quote {
				i++
				continue
			}
			return i + 1, true
		}
	}
	return 0, false
}

func unsupported(field string) error {
	return &errs.Error{Code: "unsupported_edit", Message: "cannot edit " + field + " in place", Exit: 1}
}
