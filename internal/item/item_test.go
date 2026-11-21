package item

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/errs"
)

func TestStandingTable(t *testing.T) {
	for _, tc := range []struct {
		h    Header
		want int
	}{
		{Header{Kind: Source, Status: Verified, Origin: OriginLink}, 1},
		{Header{Kind: Decision, Decided: "human:owner"}, 3},
		{Header{Kind: Decision, Status: Verified}, 1},
		{Header{Kind: Learning, Status: Verified}, 2},
		{Header{Kind: Observation, Status: Provisional}, 1},
	} {
		if got := tc.h.Standing(); got != tc.want {
			t.Errorf("%#v: standing %d, want %d", tc.h, got, tc.want)
		}
	}
	for origin := OriginNone; origin <= OriginURL; origin++ {
		h := Header{Kind: Source, Origin: origin}
		if h.Linked() != (origin == OriginLink) {
			t.Errorf("origin %s: Linked() = %v", origin, h.Linked())
		}
		h.Kind = Note
		if h.Linked() {
			t.Errorf("note with origin %s cannot be a linked source", origin)
		}
	}
}

func TestReviewDue(t *testing.T) {
	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	h, _, _, err := Parse("id", itemFile(validFields+"stale_after: 2026-11-19\n", ""))
	if err != nil || h.StaleBad || !h.StaleSet || !h.StaleAfter.Equal(now) {
		t.Fatalf("day: %#v, %v", h, err)
	}
	if h.ReviewDue(now.Add(-time.Nanosecond)) || !h.ReviewDue(now) || !h.ReviewDue(now.Add(time.Nanosecond)) {
		t.Fatal("review boundary is wrong")
	}
	h, _, _, err = Parse("id", itemFile(validFields+"stale_after: invalid\n", ""))
	if err != nil || !h.ReviewDue(now) {
		t.Fatalf("bad date: %#v, %v", h, err)
	}
	if (&Header{}).ReviewDue(now) {
		t.Fatal("an absent date is due")
	}
}

func TestLegacyStamp(t *testing.T) {
	for _, tc := range []struct{ text, stem string }{
		{"superseded-by XX", "XX"},
		{"superseded-by ../learnings/X.MD  ", "X"},
		{"superseded-by C:\\learnings\\Y.md", "Y"},
		{"superseded-by .md", ""},
		{"superseded-by  ", ""},
		{"superseded-by", ""},
		{"verified", ""},
	} {
		h := Header{StatusRaw: tc.text}
		stem, ok := h.LegacyStamp()
		if ok != (tc.stem != "") || stem != tc.stem {
			t.Errorf("%q: got %q %v, want %q", tc.text, stem, ok, tc.stem)
		}
	}
}

func TestFold(t *testing.T) {
	for _, tc := range []struct{ word, want string }{
		{"ASCII-Id 42", "ascii-id 42"},
		{"Σİς", "σiς"},
		{"É", "é"},
		{"E\u0301", "e\u0301"},
		{"bad\xffÜ", "bad\uFFFDü"},
		{"\uFFFD", "\uFFFD"},
	} {
		if got := Fold(tc.word); got != tc.want {
			t.Errorf("Fold(%q) = %q, want %q", tc.word, got, tc.want)
		}
	}
	if n := testing.AllocsPerRun(100, func() { _ = Fold("already-lower-123") }); n != 0 {
		t.Errorf("lowercase ASCII allocated %v times", n)
	}
}

func TestSlug(t *testing.T) {
	for _, tc := range []struct{ title, want string }{
		{"Cache uses Memcached", "cache-uses-memcached"},
		{"  Redis!! and?   SQL ", "redis-and-sql"},
		{strings.Repeat("x", 39) + " ! then more", strings.Repeat("x", 39)},
		{strings.Repeat("x", 41), strings.Repeat("x", 40)},
		{"継承 🚕", "item"},
	} {
		if got := Slug(tc.title); got != tc.want {
			t.Errorf("Slug(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
}

func TestNewIDUniqueAndUniform(t *testing.T) {
	now := time.Date(2026, 11, 19, 19, 21, 9, 123, time.FixedZone("one hour ahead", 3600))
	random := bytes.NewReader([]byte{252, 0, 253, 26, 254, 35, 255, 36, 251, 37, 2, 3})
	used := make(map[string]bool)
	draw := func() string {
		t.Helper()
		id, err := NewID(now, "Plan phase!", random, func(id string) bool { return used[id] })
		if err != nil {
			t.Fatal(err)
		}
		used[id] = true
		return id
	}
	const prefix = "20261119-182109-plan-phase-"
	if a, b := draw(), draw(); a != prefix+"a09a" || b != prefix+"9bcd" {
		t.Errorf("ids %q %q", a, b)
	}
	calls := 0
	_, err := NewID(now, "id", bytes.NewReader(make([]byte, 16*4)), func(string) bool {
		calls++
		return true
	})
	requireCode(t, err, "io")
	if calls != 16 {
		t.Errorf("collision attempts = %d, want 16", calls)
	}
	_, err = NewID(now, "id", bytes.NewReader([]byte{0, 1}), func(string) bool { return false })
	requireCode(t, err, "io")
}

func TestIDTime(t *testing.T) {
	for _, id := range []string{"20261119-182109-x", "20261119-182109"} {
		d, ok := IDTime(id)
		if !ok || d.Format(time.RFC3339) != "2026-11-19T18:21:09Z" {
			t.Errorf("%q: got %v %v", id, d, ok)
		}
	}
	for _, id := range []string{"20260229-182109", "20261119-242109", "20261119", "ID", "20261119- 82109", "20261119+182109"} {
		if d, ok := IDTime(id); ok {
			t.Errorf("%q: unexpectedly read %v", id, d)
		}
	}
}

func requireCode(t *testing.T, err error, code string) *errs.Error {
	t.Helper()
	var e *errs.Error
	if !errors.As(err, &e) || e.Code != code || e.Exit != 1 {
		t.Fatalf("got %v, want %s with exit 1", err, code)
	}
	return e
}
