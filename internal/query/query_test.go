package query

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/item"
)

func TestTokens(t *testing.T) {
	text := "🪵Élan ci-runner, don't CJK中 Σİ2 Ａ９"
	want := []Token{
		{"élan", 1, 5}, {"ci", 6, 8}, {"runner", 9, 15}, {"don", 17, 20}, {"t", 21, 22},
		{"cjk中", 23, 27}, {"σi2", 28, 31}, {"ａ９", 32, 34},
	}
	if got := Tokens(text); !reflect.DeepEqual(got, want) {
		t.Fatalf("Tokens = %#v, want %#v", got, want)
	}
}

func TestQueryUsesItemFold(t *testing.T) {
	for _, text := range []string{"ABC", "İstanbul", "Ελλάδα ΣΣ", "café CAFE\u0301", "漢字 Ａ９ ٣", "a\xffb"} {
		runes := []rune(text)
		var s Scanner
		s.Reset([]byte(text))
		for _, token := range Tokens(text) {
			want := item.Fold(string(runes[token.Start:token.End]))
			got, start, end, ok := s.Next()
			if !ok || string(got) != want || token.Word != want || start != token.Start || end != token.End {
				t.Fatalf("%q: token %+v, scan %q %d:%d, want %q", text, token, got, start, end, want)
			}
		}
		if _, _, _, ok := s.Next(); ok {
			t.Fatalf("%q: scanner had an extra token", text)
		}
	}
}

func TestScannerNoAlloc(t *testing.T) {
	body := []byte(strings.Repeat("élan VITAL ＡＭ ＩΣΣ mixed-case ", 100))
	var s Scanner
	run := func() {
		s.Reset(body)
		for {
			if _, _, _, ok := s.Next(); !ok {
				return
			}
		}
	}
	run()
	if n := testing.AllocsPerRun(20, run); n != 0 {
		t.Fatalf("scanner allocated %v times after warmup", n)
	}
}

func TestParseQuery(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []Clause
	}{
		{`Queue cach* ci-run* a"*Job queue" a*b "" --`, []Clause{
			{Word, []string{"queue"}, "Queue", false}, {Prefix, []string{"cach"}, "cach*", false},
			{Word, []string{"ci"}, "ci", false}, {Prefix, []string{"run"}, "run*", false},
			{Word, []string{"a"}, "a", false}, {Phrase, []string{"job", "queue"}, `"*Job queue"`, true},
			{Word, []string{"b"}, "b", false},
		}},
		{`"durable" Durable "durable" cach-* OR -`, []Clause{
			{Word, []string{"durable"}, `"durable"`, true}, {Prefix, []string{"cach"}, "cach-*", false},
			{Word, []string{"or"}, "OR", false},
		}},
		{`Durable "durable" "a* b" "A B" foo***`, []Clause{
			{Word, []string{"durable"}, "Durable", true}, {Phrase, []string{"a", "b"}, `"a* b"`, true},
			{Prefix, []string{"foo"}, "foo***", false},
		}},
	} {
		got, err := Parse(tc.text)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Parse(%q) = %#v, %v, want %#v", tc.text, got, err, tc.want)
		}
	}
	for _, text := range []string{"", " -- *", `""`, `some "open`} {
		_, err := Parse(text)
		assertUsage(t, err)
	}
}

func assertUsage(t *testing.T, err error) {
	t.Helper()
	var e *errs.Error
	if !errors.As(err, &e) || e.Code != "usage" || e.Exit != 2 {
		t.Errorf("got %v, want a usage error", err)
	}
}

func FuzzParseQuery(f *testing.F) {
	f.Add(`two "same words" partial-*`)
	f.Add(`"word" Элан cach*****`)
	f.Add(`ab"unclosed`)
	f.Fuzz(func(t *testing.T, text string) {
		clauses, err := Parse(text)
		if err != nil {
			assertUsage(t, err)
			return
		}
		if len(clauses) == 0 {
			t.Fatal("accepted empty query")
		}
		seen := make(map[string]bool)
		for _, clause := range clauses {
			if len(clause.Words) == 0 || clause.Label == "" || !strings.Contains(text, clause.Label) {
				t.Fatalf("invalid clause %+v for %q", clause, text)
			}
			key := string(rune(clause.Kind)+1) + strings.Join(clause.Words, "\x00")
			if seen[key] {
				t.Fatalf("duplicate clause %+v", clause)
			}
			seen[key] = true
		}
	})
}
