package item

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pkieltyka/pbrain/internal/outline"
	"go.yaml.in/yaml/v3"
)

// FrontMatterError describes an unreadable block without including file contents.
type FrontMatterError struct {
	Reason FMReason
	Line   int // 1-based file line, 0 if unknown
	Column int
}

type FMReason uint8

const (
	Unclosed FMReason = iota
	Syntax
	NotMapping
	DuplicateKey
)

func (e *FrontMatterError) Error() string {
	reasons := [...]string{"unclosed", "syntax", "not a mapping", "duplicate key"}
	msg := "front matter: " + nameAt(reasons[:], uint8(e.Reason))
	if e.Line > 0 {
		msg += fmt.Sprintf(" at line %d", e.Line)
	}
	return msg
}

type frontMatter struct {
	start, close, body int // byte positions in the original file
}

const yamlLine = 2

// SplitFrontMatter locates the YAML block and the body, with offsets into data.
func SplitFrontMatter(data []byte) (yamlText []byte, line, bodyAt int, ok bool, err error) {
	block, err := splitFrontMatter(data)
	if err == ErrNoFrontMatter {
		return nil, 0, 0, false, nil
	}
	if err != nil {
		return nil, yamlLine, 0, true, err
	}
	return data[block.start:block.close], yamlLine, block.body, true, nil
}

func splitFrontMatter(data []byte) (frontMatter, error) {
	bom := 0
	if bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) {
		bom = 3
	}
	first, end := takeLine(data, bom)
	if !delimiter(first) {
		return frontMatter{}, ErrNoFrontMatter
	}
	block := frontMatter{start: end}
	for start := end; start < len(data); {
		line, next := takeLine(data, start)
		if delimiter(line) {
			block.close, block.body = start, next
			blank, afterBlank := takeLine(data, next)
			if len(blank) == 0 {
				block.body = afterBlank
			}
			return block, nil
		}
		start = next
	}
	return frontMatter{}, &FrontMatterError{Reason: Unclosed, Line: 1, Column: 1}
}

// takeLine returns the contents of a physical line and its next byte position.
func takeLine(data []byte, start int) ([]byte, int) {
	if i := bytes.IndexByte(data[start:], '\n'); i >= 0 {
		end := start + i
		return bytes.TrimSuffix(data[start:end], []byte{'\r'}), end + 1
	}
	return data[start:], len(data)
}

func delimiter(line []byte) bool {
	return bytes.Equal(bytes.TrimRight(line, " \t"), []byte("---"))
}

var syntaxLine = regexp.MustCompile(`\bline ([0-9]+):`)

func readMapping(data []byte, block frontMatter) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data[block.start:block.close], &doc); err != nil {
		line := 0
		if m := syntaxLine.FindStringSubmatch(err.Error()); m != nil {
			line, _ = strconv.Atoi(m[1])
			line += yamlLine - 1
		}
		return nil, &FrontMatterError{Reason: Syntax, Line: line}
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, &FrontMatterError{Reason: NotMapping, Line: root.Line + yamlLine - 1, Column: root.Column}
	}
	seen := make(map[string]bool, len(root.Content)/2)
	for i := 0; i < len(root.Content); i += 2 {
		key := root.Content[i]
		if key.Kind != yaml.ScalarNode {
			continue
		}
		if seen[key.Value] {
			return nil, &FrontMatterError{Reason: DuplicateKey, Line: key.Line + yamlLine - 1, Column: key.Column}
		}
		seen[key.Value] = true
	}
	return root, nil
}

// Parse reads the front matter and returns the body byte offset, even with bad fields.
func Parse(id string, data []byte) (Header, int64, []Problem, error) {
	block, err := splitFrontMatter(data)
	if err != nil {
		return Header{}, 0, nil, err
	}
	root, err := readMapping(data, block)
	if err != nil {
		return Header{}, 0, nil, err
	}
	h, problems := parseHeader(id, data[block.body:], root)
	return h, int64(block.body), problems, nil
}

func parseHeader(id string, body []byte, root *yaml.Node) (Header, []Problem) {
	fields := make(map[string]*yaml.Node)
	if root != nil {
		for i := 0; i < len(root.Content); i += 2 {
			if key := root.Content[i]; key.Kind == yaml.ScalarNode {
				fields[key.Value] = root.Content[i+1]
			}
		}
	}
	var problems []Problem
	problem := func(code, key string, n *yaml.Node) {
		p := Problem{Code: code, Field: key}
		if n != nil {
			p.Line, p.Column = n.Line+yamlLine-1, n.Column
		}
		problems = append(problems, p)
	}
	scalar := func(key string) string {
		n := fields[key]
		if n == nil {
			return ""
		}
		if !singleScalar(n) {
			problem("bad_field", key, n)
			return ""
		}
		return n.Value
	}
	missing := func(key, value string) {
		n := fields[key]
		if value == "" && (n == nil || singleScalar(n)) {
			problem("missing_field", key, nil)
		}
	}
	list := func(key string) []string {
		n := fields[key]
		if n == nil || singleScalar(n) && n.Value == "" {
			return nil
		}
		if n.Kind != yaml.SequenceNode {
			problem("bad_field", key, n)
			return nil
		}
		values := make([]string, 0, len(n.Content))
		for _, value := range n.Content {
			s := value.Value
			ok := value.Kind == yaml.ScalarNode && s != ""
			if ok && key == "tags" {
				s, ok = NormTag(s)
			}
			if !ok {
				problem("bad_field", key, value)
				return nil
			}
			values = append(values, s)
		}
		return values
	}

	var h Header
	h.KindRaw = scalar("kind")
	kind, ok := ParseKind(h.KindRaw)
	h.Kind = kind
	if !ok && h.KindRaw != "" {
		problem("unknown_value", "kind", fields["kind"])
	}
	h.Title = scalar("title")
	if h.Title == "" {
		h.Title = HeadingTitle(body)
		if h.Title == "" {
			h.Title = id
		}
	}
	created := scalar("created")
	if created != "" {
		if t, err := time.Parse(time.RFC3339, created); err == nil {
			h.Created, h.CreatedKnown = t, true
		} else {
			problem("bad_field", "created", fields["created"])
		}
	} else {
		missing("created", created)
	}
	if !h.CreatedKnown {
		h.Created, h.CreatedKnown = IDTime(id)
	}
	h.Author = scalar("author")
	missing("author", h.Author)
	h.Source = scalar("source")
	if h.Kind == Source {
		missing("source", h.Source)
	}
	h.StatusRaw = scalar("status")
	switch h.StatusRaw {
	case "verified":
		h.Status = Verified
	case "", "provisional":
	default:
		if !strings.HasPrefix(h.StatusRaw, stampPrefix) {
			problem("unknown_value", "status", fields["status"])
		}
	}
	h.Verified = scalar("verified")
	h.Decided = scalar("decided")
	if h.Kind == Decision && h.Decided == "" {
		problem("decision_without_decider", "decided", nil)
	}
	h.Replaces = list("replaces")
	h.Relates = list("relates")
	h.Tags = list("tags")
	if stale := scalar("stale_after"); stale != "" {
		h.StaleSet = true
		t, _, ok := ParseStaleAfter(stale)
		h.StaleAfter, h.StaleBad = t, !ok
		if !ok {
			problem("bad_field", "stale_after", fields["stale_after"])
		}
	}
	origin := scalar("origin")
	h.Origin, ok = ParseOrigin(origin)
	if !ok {
		problem("bad_field", "origin", fields["origin"])
	}
	if fetched := scalar("fetched"); fetched != "" {
		t, err := time.Parse(time.RFC3339, fetched)
		h.Fetched = t
		if err != nil {
			problem("bad_field", "fetched", fields["fetched"])
		}
	}
	h.SHA256 = scalar("sha256")
	slices.SortStableFunc(problems, func(a, b Problem) int {
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return a.Column - b.Column
	})
	return h, problems
}

func singleScalar(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) == 0
}

// FirstHeading returns the first named section outside fenced code.
func FirstHeading(body []byte) string {
	for _, section := range outline.Parse(body) {
		if section.Level > 0 && section.Title != "" {
			return section.Title
		}
	}
	return ""
}

// HeadingTitle is FirstHeading or the first non-blank, non-fence line.
func HeadingTitle(body []byte) string {
	if heading := FirstHeading(body); heading != "" {
		return heading
	}
	for line := range bytes.SplitSeq(body, []byte{'\n'}) {
		unindented := line
		for i := 0; i < 3 && len(unindented) > 0 && unindented[0] == ' '; i++ {
			unindented = unindented[1:]
		}
		if bytes.HasPrefix(unindented, []byte("```")) || bytes.HasPrefix(unindented, []byte("~~~")) {
			continue
		}
		if text := bytes.TrimSpace(line); len(text) != 0 {
			return string(text)
		}
	}
	return ""
}
