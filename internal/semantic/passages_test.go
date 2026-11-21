package semantic

import (
	"crypto/sha256"
	"encoding/binary"
	"hash/crc32"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/outline"
)

func spansTile(t *testing.T, ps []Passage, n int) {
	t.Helper()
	prev := 0
	for i, p := range ps {
		if p.Start < prev || p.End < p.Start || p.End > n {
			t.Fatalf("passage %d [%d,%d) after %d in a body of %d", i, p.Start, p.End, prev, n)
		}
		prev = p.End
		if p.Digest != sha256.Sum256([]byte(p.Text)) {
			t.Fatalf("passage %d digest", i)
		}
	}
}

func TestPassagesSections(t *testing.T) {
	body := "Intro text.\n\n# Top\n\nTop text.\n\n## Empty\n\n## Child\n\nChild text.\n"
	ps := Passages("Title", []byte(body))
	if len(ps) != 3 {
		t.Fatalf("%d passages: %+v", len(ps), ps)
	}
	if ps[0].Text != "Title\n\nIntro text." {
		t.Fatalf("level 0: %q", ps[0].Text)
	}
	if ps[1].Text != "Title\nTop\n\n# Top\n\nTop text." {
		t.Fatalf("top: %q", ps[1].Text)
	}
	if ps[2].Text != "Title\nTop > Child\n\n## Child\n\nChild text." {
		t.Fatalf("child with its ancestor line: %q", ps[2].Text)
	}
	spansTile(t, ps, utf8.RuneCountInString(body))
	// A setext heading with nothing under it gives none.
	if ps := Passages("T", []byte("Head\n====\n\n")); len(ps) != 1 || ps[0].Start != 0 || ps[0].End != 0 || ps[0].Text != "T" {
		t.Fatalf("setext empty: %+v", ps)
	}
	// "#foo" is not an ATX heading: outline reads it as setext text, and its section is empty too.
	if ps := Passages("T", []byte("#foo\n====\n")); len(ps) != 1 || ps[0].Text != "T" || ps[0].End != 0 {
		t.Fatalf("setext #foo: %+v", ps)
	}
	if ps := Passages("T", []byte("  ## H\n\n")); len(ps) != 1 || ps[0].Text != "T" {
		t.Fatalf("an indented ATX heading with nothing under it: %+v", ps)
	}
	// An empty body gives the title passage.
	if ps := Passages("Only title", nil); len(ps) != 1 || ps[0].Text != "Only title" || ps[0].End != 0 {
		t.Fatalf("empty body: %+v", ps)
	}
}

func TestPassagesSplit(t *testing.T) {
	var b strings.Builder
	b.WriteString("# S\n\n")
	for i := range 25 {
		b.WriteString(strings.Repeat("word ", 39) + "end" + string(rune('a'+i%26)) + ".\n\n")
	}
	b.WriteString("```\n" + strings.Repeat("code line\n\n", 40) + "```\n")
	body := b.String()
	ps := Passages("T", []byte(body))
	rs := []rune(body)
	spansTile(t, ps, len(rs))
	for _, p := range ps {
		if p.End-p.Start > partMax {
			t.Fatalf("a part of %d", p.End-p.Start)
		}
		seg := string(rs[p.Start:p.End])
		if strings.Count(seg, "```")%2 != 0 && len([]rune(seg)) < partMax {
			t.Fatalf("a fenced block was cut by a paragraph break: %q", seg[:min(80, len(seg))])
		}
	}
	if len(ps) < 3 {
		t.Fatalf("a long section gave %d parts", len(ps))
	}
	// A closing line with a tab after its marker does not close the fence (only spaces may follow),
	// and an indented closer does: in both cases the paragraphs agree with outline's fence.
	long := func(c string) string { return strings.Repeat(c, 499) + "\n" }
	tabbed := "# S\n\n```\n" + long("c") + "```\t\n\n" + long("p") + "\n" + long("q") + "\n" + long("r")
	if secs := outline.Parse([]byte(tabbed)); len(secs) != 1 {
		t.Fatalf("outline found %d sections", len(secs))
	}
	ps = Passages("T", []byte(tabbed))
	trs := []rune(tabbed)
	// The heading alone is closed first (filling would pass 1,400); the open fence is then one
	// paragraph, cut at its last line feed before 1,800.
	if len(ps) < 2 || ps[0].End != 5 || ps[1].End < 1400 || ps[1].End-ps[1].Start > partMax || trs[ps[1].End-1] != '\n' {
		t.Fatalf("a tab after the closing marker closed the fence: parts end at %d and %d", ps[0].End, ps[1].End)
	}
	indented := "# S\n\n```\n" + long("c") + "   ```  \n\n" + long("p") + "\n" + long("q") + "\n" + long("r")
	ps = Passages("T", []byte(indented))
	irs := []rune(indented)
	// Closed, the fence is a paragraph of its own: heading, fence and p fill the first part.
	if len(ps) < 2 || !strings.Contains(string(irs[ps[0].Start:ps[0].End]), "ppp") || string(irs[ps[1].Start:ps[1].Start+3]) != "qqq" {
		t.Fatalf("an indented closer did not close the fence: parts %+v", [][2]int{{ps[0].Start, ps[0].End}, {ps[1].Start, ps[1].End}})
	}
	// One 4,000-code-point line with spaces is cut after a space; without spaces at 1,800.
	line := strings.Repeat("abcdefghi ", 400)
	ps = Passages("T", []byte(line))
	if ps[0].End-ps[0].Start > partMax || []rune(line)[ps[0].End-1] != ' ' {
		t.Fatalf("a long line was not cut after a space: %d", ps[0].End)
	}
	ps = Passages("T", []byte(strings.Repeat("x", 4000)))
	if ps[0].End != 1800 || ps[1].End != 3600 || ps[2].End != 4000 {
		t.Fatalf("no spaces: %+v", []int{ps[0].End, ps[1].End})
	}
	// Multibyte text keeps code-point offsets.
	mb := strings.Repeat("ü", 2000)
	ps = Passages("T", []byte(mb))
	if ps[0].End != 1800 || ps[1].End != 2000 {
		t.Fatalf("multibyte offsets: %d %d", ps[0].End, ps[1].End)
	}
}

func TestPassagesPrefixCap(t *testing.T) {
	title := strings.Repeat("t", 600)
	var b strings.Builder
	for l := 1; l <= 6; l++ {
		b.WriteString(strings.Repeat("#", l) + " " + strings.Repeat("h", 60) + string(rune('0'+l)) + "\n\n")
	}
	b.WriteString("distinct body passage\n")
	ps := Passages(title, []byte(b.String()))
	last := ps[len(ps)-1]
	prefix, rest, _ := strings.Cut(last.Text, "\n\n")
	lines := strings.Split(prefix, "\n")
	if utf8.RuneCountInString(lines[0]) != 200 || utf8.RuneCountInString(lines[1]) > 200 {
		t.Fatalf("prefix lines %d and %d", utf8.RuneCountInString(lines[0]), utf8.RuneCountInString(lines[1]))
	}
	if !strings.HasSuffix(lines[1], "6") || !strings.Contains(rest, "distinct body passage") {
		t.Fatalf("the innermost heading or the passage is missing: %q", last.Text)
	}
}

func TestPassagesStable(t *testing.T) {
	var b strings.Builder
	for i := range 12 {
		b.WriteString(strings.Repeat("paragraph ", 30) + string(rune('A'+i)) + "\n\n")
	}
	body := b.String()
	a, again := Passages("T", []byte(body)), Passages("T", []byte(body))
	for i := range a {
		if a[i] != again[i] {
			t.Fatal("not deterministic")
		}
	}
	edited := strings.Replace(body, "paragraph K", "PARAGRAPH K", 1)
	c := Passages("T", []byte(edited))
	if len(c) != len(a) {
		t.Fatal("an edit changed the number of parts")
	}
	changed := 0
	for i := range a {
		if a[i].Digest != c[i].Digest {
			changed++
			if !strings.Contains(c[i].Text, "PARAGRAPH K") {
				t.Fatal("a part without the edit changed")
			}
		}
	}
	if changed != 1 {
		t.Fatalf("%d parts changed", changed)
	}
}

func TestContentDigestFraming(t *testing.T) {
	pairs := [][2]string{{"ab", "c"}, {"a", "bc"}, {"a\x00", "b"}, {"a", "\x00b"}, {"", "ab"}, {"ab", ""}}
	seen := map[[32]byte]int{}
	for i, p := range pairs {
		d := ContentDigest(p[0], []byte(p[1]))
		if j, ok := seen[d]; ok {
			t.Fatalf("pair %d and %d collide", i, j)
		}
		seen[d] = i
	}
	if first, again := ContentDigest("t", []byte("b")), ContentDigest("t", []byte("b")); first != again {
		t.Fatal("not deterministic")
	}

	// ContentCheck is the CRC-32C and count of the same framed bytes.
	for i, p := range pairs {
		framed := binary.LittleEndian.AppendUint64(nil, uint64(len(p[0])))
		framed = append(append(framed, p[0]...), p[1]...)
		want := Check{CRC: crc32.Checksum(framed, crc32.MakeTable(crc32.Castagnoli)), Size: int64(len(framed))}
		if got := ContentCheck(p[0], []byte(p[1])); got != want {
			t.Fatalf("pair %d: %+v, want %+v", i, got, want)
		}
	}
	// Moving bytes between the title's end and the body's start changes the CRC, not the size; one
	// added byte changes the size.
	a, b := ContentCheck("ab", []byte("c")), ContentCheck("a", []byte("bc"))
	if a.Size != b.Size || a.CRC == b.CRC {
		t.Fatalf("a moved byte: %+v and %+v", a, b)
	}
	if c := ContentCheck("ab", []byte("cd")); c.Size != a.Size+1 {
		t.Fatalf("an added byte: %+v", c)
	}
}

// Every span lies inside one section of outline.Parse and none starts at a fenced line that looks
// like a heading; two sections with one title under different parents give different heading paths.
func TestPassageAlignment(t *testing.T) {
	body := "# Alpha\n\nαβγ text.\n\n## Setup\n\nAlpha setup.\n\n```\n# not a heading\n```\n\n# Beta\n\nmore.\n\n## Setup\n\nBeta setup.\n"
	secs := outline.Parse([]byte(body))
	ps := Passages("T", []byte(body))
	var setupPrefixes []string
	for _, p := range ps {
		i := outline.At(secs, p.Start)
		if i < 0 || p.End > secs[i].End {
			t.Fatalf("passage [%d,%d) is not inside one section", p.Start, p.End)
		}
		if strings.HasPrefix(string([]rune(body)[p.Start:]), "# not a heading") {
			t.Fatal("a passage starts at the fenced line")
		}
		if secs[i].Title == "Setup" {
			prefix, _, _ := strings.Cut(p.Text, "\n\n")
			setupPrefixes = append(setupPrefixes, prefix)
		}
		// The body from Start begins with the passage text that was embedded.
		_, text, _ := strings.Cut(p.Text, "\n\n")
		if !strings.HasPrefix(strings.TrimSpace(string([]rune(body)[p.Start:])), text[:min(10, len(text))]) {
			t.Fatalf("the body at %d does not start with the passage", p.Start)
		}
	}
	if len(setupPrefixes) != 2 || setupPrefixes[0] == setupPrefixes[1] {
		t.Fatalf("the two Setup sections: %q", setupPrefixes)
	}
}
