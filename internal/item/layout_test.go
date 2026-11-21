package item

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestMeasure(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       Layout
	}{
		{"LF", "---\ntitle: café\n---\n\nα\n🙂\n", Layout{BodyLine: 5, BodyChars: 4, FileLines: 6, FileChars: 25}},
		{"BOM, CRLF", "\ufeff---\r\ntitle: café\r\n---\r\n\r\nα\r\n🙂", Layout{BodyLine: 5, BodyChars: 4, FileLines: 6, FileChars: 30}},
		{"no final feed", "---\n---\na", Layout{BodyLine: 3, BodyChars: 1, FileLines: 3, FileChars: 9}},
		{"empty with LF", "---\n---\n", Layout{BodyLine: 3, FileLines: 2, FileChars: 8}},
		{"empty without LF", "---\n---", Layout{BodyLine: 3, FileLines: 2, FileChars: 7}},
		{"empty with separator", "---\n---\n\n", Layout{BodyLine: 4, FileLines: 3, FileChars: 9}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(tc.text)
			_, at, _, err := Parse("id", data)
			if err != nil {
				t.Fatal(err)
			}
			if got := Measure(data, at); got != tc.want {
				t.Errorf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestLineOf(t *testing.T) {
	for _, text := range []string{
		"\ufeff---\r\ntitle: café\r\n---\r\n\r\né\r\n三\r\n",
		"---\n---\n\n🙂line\nline2",
		"---\n---",
		"---\n---\n",
	} {
		data := []byte(text)
		_, at, _, err := Parse("id", data)
		if err != nil {
			t.Fatal(err)
		}
		it := Item{Layout: Measure(data, at)}
		body := data[at:]
		for cp := -2; cp <= it.Layout.BodyChars+2; cp++ {
			want := oracleLine(data, int(at), cp)
			if got := it.LineOf(body, cp); got != want {
				t.Errorf("%q at %d: got %d, want %d", text, cp, got, want)
			}
		}
	}
	if got := (&Item{}).LineOf([]byte("x\n"), 2); got != 0 {
		t.Errorf("unmeasured line = %d, want 0", got)
	}
}

func TestLineCursorOnePass(t *testing.T) {
	for _, body := range []string{strings.Repeat("x", 512000), strings.Repeat("é\r\n", 100000)} {
		data := itemFile("title: test\n", body)
		_, at, _, err := Parse("id", data)
		if err != nil {
			t.Fatal(err)
		}
		it := Item{Layout: Measure(data, at)}
		cursor := it.Lines(data[at:])
		for cp := 0; cp < it.Layout.BodyChars; cp += 1009 {
			if got, want := cursor.At(cp), it.LineOf(data[at:], cp); got != want {
				t.Fatalf("at %d got %d, want %d", cp, got, want)
			}
		}
		cursor.At(it.Layout.BodyChars)
		if cursor.bytesStepped != len(body) {
			t.Fatalf("read %d bytes for %d bytes of body", cursor.bytesStepped, len(body))
		}
		if got := cursor.At(1); got != it.Layout.BodyLine || cursor.bytesStepped != len(body)+len(string([]rune(body)[0])) {
			t.Fatalf("backwards seek: line %d, bytes stepped %d", got, cursor.bytesStepped)
		}
	}
}

func FuzzMeasure(f *testing.F) {
	f.Add("---\né: 三\n---\n\n🙂\né", 4)
	f.Add("\ufeff---\r\ntitle: é\r\n---\r\n\r\né\r\n三\r\n", 1)
	f.Add("---\n---", 0)
	f.Fuzz(func(t *testing.T, text string, cp int) {
		if !utf8.ValidString(text) {
			t.Skip()
		}
		data := []byte(text)
		_, at, _, err := Parse("id", data)
		if err != nil {
			t.Skip()
		}
		lines := bytes.Split(data, []byte{'\n'})
		physical := len(lines)
		if len(lines[physical-1]) == 0 {
			physical--
		}
		want := Layout{FileLines: physical, FileChars: utf8.RuneCount(data), BodyChars: utf8.RuneCount(data[at:]), BodyLine: oracleLine(data, int(at), 0)}
		it := Item{Layout: Measure(data, at)}
		if it.Layout != want || it.LineOf(data[at:], cp) != oracleLine(data, int(at), cp) {
			t.Errorf("measure = %#v, want %#v", it.Layout, want)
		}
	})
}

func oracleLine(data []byte, at, cp int) int {
	if at == len(data) {
		n := len(bytes.Split(data, []byte{'\n'}))
		if len(data) != 0 && data[len(data)-1] != '\n' {
			n++
		}
		return n
	}
	runes := []rune(string(data[at:]))
	return bytes.Count(data[:at], []byte{'\n'}) + strings.Count(string(runes[:min(max(cp, 0), len(runes))]), "\n") + 1
}
