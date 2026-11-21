package render

import (
	"encoding/json"
	"strings"
	"testing"
)

// The unit half of render.md's TestExactValueProjection: ordinary ids and ids with spaces are bare;
// any other id that holds a rune ExactPath escapes, a double quote or a backslash is one JSON string
// on one line that decodes to it; input that is not valid UTF-8 is not written.
func TestExactValue(t *testing.T) {
	for _, v := range []string{"20261121-000000-plain-a1a1", "Meeting notes 2024", "café ünïcode", "x\uFFFD"} {
		if got, ok := ExactValue(v); !ok || got != v {
			t.Errorf("%q: %q %v, want it bare", v, got, ok)
		}
	}
	for _, v := range []string{"a\nb", "a\rb", "a\tb", "a\x7fb", "a\u0085b", "a\u2028b", "a\u202eb", "a\"b", "a\\b",
		"\"a\\u000ab\"", "a\U000e0001b", " lead\n"} {
		got, ok := ExactValue(v)
		if !ok || !strings.HasPrefix(got, "\"") || strings.ContainsAny(got, "\n\r\u2028\u2029\u0085") {
			t.Errorf("%q: %q %v, want one token on one line", v, got, ok)
			continue
		}
		var back string
		if err := json.Unmarshal([]byte(got), &back); err != nil || back != v {
			t.Errorf("%q: token %q decodes to %q, %v", v, got, back, err)
		}
	}
	if _, ok := ExactValue("x\xff"); ok {
		t.Error("invalid UTF-8 was written")
	}
	if ValueText("x\xff") != "?" || ValueText("a b") != "a b" || ValueText("a\nb") != `"a\u000ab"` {
		t.Errorf("ValueText: %q %q %q", ValueText("x\xff"), ValueText("a b"), ValueText("a\nb"))
	}
}
