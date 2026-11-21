package outline

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseSections(t *testing.T) {
	parts := []string{
		"préface\r\n\r\n",
		"# One ###\r\n",
		"   Repeated\r\n   ----\r\n  data é\r\n",
		" ### Child \r\n   ~~~~go\r\n# hidden\r\n~~~\r\n  ~~~~~   \r\n",
		"# Two\r\n",
		"## Repeated\r\nlast",
	}
	body := []byte(strings.Join(parts, ""))
	wantLevels := []int{0, 1, 2, 3, 1, 2}
	wantTitles := []string{"", "One", "Repeated", "Child", "Two", "Repeated"}
	wantTreeEnd := []int{1, 4, 4, 4, 6, 6}
	var want []Section
	for i, part := range parts {
		before := strings.Join(parts[:i], "")
		want = append(want, Section{
			Level: wantLevels[i], Title: wantTitles[i],
			Start: utf8.RuneCountInString(before), End: utf8.RuneCountInString(before + part),
			Tree:      utf8.RuneCountInString(strings.Join(parts[:wantTreeEnd[i]], "")),
			ByteStart: len(before), ByteEnd: len(before) + len(part),
		})
	}
	got := Parse(body)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sections = %#v, want %#v", got, want)
	}
	for i, s := range want {
		for _, cp := range []int{s.Start, s.End - 1} {
			if found := At(got, cp); found != i {
				t.Errorf("At(%d) = %d, want %d", cp, found, i)
			}
		}
	}
	for _, cp := range []int{-1, utf8.RuneCount(body)} {
		if At(got, cp) != -1 {
			t.Errorf("At(%d) mapped outside the body", cp)
		}
	}
}

func TestHeadingRules(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		levels     []int
		titles     []string
	}{
		{"blank", " \t\u2003\r\n", nil, nil},
		{"preamble", "\n \n # title", []int{1}, []string{"title"}},
		{"none", "preamble é\ntext", []int{0}, []string{""}},
		{"ATX", "  ######\t日本語 ##\n #\n## ##\n## no###\n##not\n####### also not\n    ## no", []int{6, 1, 2, 2}, []string{"日本語", "", "", "no###"}},
		{"setext", "Alpha\n=\nBeta\n -- \n- item\n---\n* list\n---\n+ list\n---\n> quote\n---\n|table\n---\n1) list\n---\n12. list\n---\n    code\n---\nno\n-\n", []int{1, 2}, []string{"Alpha", "Beta"}},
		{"unclosed", "intro\n````\n```\n~~~\n# hidden\ntext\n-----", []int{0}, []string{""}},
		{"fence closes", "```go\n  ``` extra\n# hidden\n````   \n# shown", []int{0, 1}, []string{"", "shown"}},
		{"only CRLF strips CR", "### title\r\n###\r", []int{3}, []string{"title"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var levels []int
			var titles []string
			for _, s := range Parse([]byte(tc.text)) {
				levels = append(levels, s.Level)
				titles = append(titles, s.Title)
			}
			if !reflect.DeepEqual(levels, tc.levels) || !reflect.DeepEqual(titles, tc.titles) {
				t.Fatalf("got %v %q, want %v %q", levels, titles, tc.levels, tc.titles)
			}
		})
	}
}

func FuzzOutlineSpans(f *testing.F) {
	f.Add("é\n## one\r\n### child\n# end")
	f.Add("  \r\n三\n=\n```\n# literal\n")
	f.Add("~~~\n\t")
	f.Fuzz(func(t *testing.T, body string) {
		if !utf8.ValidString(body) {
			t.Skip()
		}
		sections := Parse([]byte(body))
		if len(sections) == 0 {
			if strings.TrimSpace(body) != "" {
				t.Fatal("lost nonspace body")
			}
			return
		}
		end := utf8.RuneCountInString(body)
		for i, s := range sections {
			if s.ByteStart < 0 || s.ByteStart >= s.ByteEnd || s.ByteEnd > len(body) {
				t.Fatalf("invalid byte range: %#v", s)
			}
			if s.Start != utf8.RuneCountInString(body[:s.ByteStart]) ||
				s.End != utf8.RuneCountInString(body[:s.ByteEnd]) ||
				At(sections, s.Start) != i || At(sections, s.End-1) != i {
				t.Fatalf("invalid offset mapping: %#v", s)
			}
			if s.Tree < s.End || s.Tree > end {
				t.Fatalf("invalid tree end: %#v", s)
			}
			if i > 0 && sections[i-1].ByteEnd != s.ByteStart {
				t.Fatalf("noncontiguous spans at %d: %#v", i, sections)
			}
		}
		if sections[len(sections)-1].End != end || At(sections, end) != -1 {
			t.Fatal("invalid end of body")
		}
	})
}
