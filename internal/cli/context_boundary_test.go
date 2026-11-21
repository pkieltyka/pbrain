package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// contextPage searches a body whose focal line has two neighbours on each side; the farthest one after
// it holds pad code points and the others fill. It returns the printed block's size and line count: the "context" field
// with its comma in JSON, or the numbered lines with their line feeds in text, and the block itself.
func contextPage(t *testing.T, js bool, fill, pad int) (size, lines int, block string) {
	t.Helper()
	rb := newReadBrain(t)
	body := strings.Join([]string{
		"opening line of the note",
		"before two " + strings.Repeat("<", fill),
		"before one " + strings.Repeat("&", fill),
		"the boundaryword stands here",
		"after one " + strings.Repeat(">", fill),
		"after two " + strings.Repeat("~", fill),
		"after three " + strings.Repeat("x", pad),
		"closing line",
	}, "\n") + "\n"
	rb.put("notes/20261101-000000-boundary.md", []string{"kind: note", "title: Boundary", "author: agent:x"}, body)
	args := []string{"search", "boundaryword", "--context", "3", "--max-chars", "100000"}
	if js {
		args = append(args, "--json")
	}
	code, out, errOut := rb.run(args...)
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d: %q", code, errOut)
	}
	if js {
		var page struct {
			Results []struct {
				Context json.RawMessage `json:"context"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(out), &page); err != nil || len(page.Results) != 1 {
			t.Fatalf("%v: %s", err, out)
		}
		raw := page.Results[0].Context
		if !strings.Contains(out, `,"context":`+string(raw)) {
			t.Fatalf("the context field is not printed as decoded: %s", out)
		}
		var block []json.RawMessage
		if err := json.Unmarshal(raw, &block); err != nil {
			t.Fatal(err)
		}
		return utf8.RuneCountInString(`,"context":`) + utf8.RuneCount(raw), len(block), string(raw)
	}
	for _, l := range strings.SplitAfter(out, "\n") {
		if strings.HasPrefix(l, "    ") && len(l) > 4 && l[4] >= '0' && l[4] <= '9' {
			size += utf8.RuneCountInString(l)
			lines++
			block += l
		}
	}
	return size, lines, block
}

// Scenario 40: a block of exactly 600 code points as printed is shown whole; one more code point
// and it loses its outer line, the farthest neighbour after the stretch. JSON escapes nothing in <,
// > and &, so they count one each.
func TestContextBoundary600(t *testing.T) {
	for _, js := range []bool{false, true} {
		// The largest fill whose whole probe block stays under 600 with room for a pad under the cut.
		fill, pad, n0 := 150, 0, 0
		for ; fill > 0; fill -= 10 {
			size, lines, block := contextPage(t, js, fill, 10)
			if strings.Contains(block, "after three") && size < 600 && 10+600-size <= 150 {
				pad, n0 = 10+600-size, lines
				break
			}
		}
		if pad == 0 {
			t.Fatalf("json %v: no fixture gives a 600-code-point block", js)
		}
		if size, lines, block := contextPage(t, js, fill, pad); size != 600 || lines != n0 || !strings.Contains(block, "after three") {
			t.Fatalf("json %v at 600: %d code points in %d lines", js, size, lines)
		}
		if size, lines, block := contextPage(t, js, fill, pad+1); lines != n0-1 || size >= 600 || strings.Contains(block, "after three") {
			t.Fatalf("json %v at 601: %d code points in %d lines, want the outer line dropped: %s", js, size, lines, block)
		}
	}
}
