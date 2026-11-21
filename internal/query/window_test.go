package query

import (
	"testing"
	"time"
)

func instant(t *testing.T, text string) time.Time {
	t.Helper()
	when, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		t.Fatal(err)
	}
	return when
}

func zone(t *testing.T, text string) *time.Location {
	t.Helper()
	z, err := time.LoadLocation(text)
	if err != nil {
		t.Fatal(err)
	}
	return z
}

func TestParseWindow(t *testing.T) {
	now := instant(t, "2026-11-21T05:00:01.123Z")
	toronto := zone(t, "America/Toronto")
	for _, tc := range []struct {
		name, since, until  string
		wantSince, wantTill string
		inclusive           bool
	}{
		{"unset", "", "", "", "", false},
		{"civil date", "2026-11-20", "2026-11-20", "2026-11-20T05:00:00Z", "2026-11-21T05:00:00Z", false},
		{"today", "today", "today", "2026-11-21T05:00:00Z", "2026-11-22T05:00:00Z", false},
		{"yesterday", "yesterday", "yesterday", "2026-11-20T05:00:00Z", "2026-11-21T05:00:00Z", false},
		{"RFC3339", "2026-11-21T00:00:00-05:00", "2026-11-21T05:00:00Z", "2026-11-21T05:00:00Z", "2026-11-21T05:00:00Z", true},
		{"spans", "1w", "1d", "2026-11-14T05:00:01.123Z", "2026-11-20T05:00:01.123Z", true},
		{"exact hour", "1h", "0d", "2026-11-21T04:00:01.123Z", "2026-11-21T05:00:01.123Z", true},
		{"until alone", "", "2026-11-20", "", "2026-11-21T05:00:00Z", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, err := ParseWindow(tc.since, tc.until, now, toronto)
			if err != nil || w.Set() != (tc.since != "" || tc.until != "") || w.UntilIncl != tc.inclusive {
				t.Fatalf("got %+v, %v", w, err)
			}
			if tc.wantSince != "" {
				a := instant(t, tc.wantSince)
				if !w.HasSince || !w.Since.Equal(a) || !w.Contains(a) || w.Contains(a.Add(-time.Nanosecond)) {
					t.Fatalf("wrong since bound: %+v", w)
				}
			}
			if tc.wantTill != "" {
				b := instant(t, tc.wantTill)
				if !w.HasUntil || !w.Until.Equal(b) || w.Contains(b) != tc.inclusive || w.Contains(b.Add(time.Nanosecond)) {
					t.Fatalf("wrong until bound: %+v", w)
				}
			}
		})
	}
	for _, text := range []string{"soon", "3x", "-3d", "3D", "3 d", "1000000d", "2026-13-01", "2026-11-20T14:01:00"} {
		_, err := ParseWindow(text, "", now, toronto)
		assertUsage(t, err)
		_, err = ParseWindow("", text, now, toronto)
		assertUsage(t, err)
	}
	_, err := ParseWindow("2026-11-21", "2026-11-20", now, toronto)
	assertUsage(t, err)

	for _, span := range []string{"999999w", "999999d"} {
		w, err := ParseWindow(span, "", now, toronto)
		if err != nil || !w.Since.Before(time.Date(1000, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("%s did not go far into the past: %+v, %v", span, w, err)
		}
	}
	before := instant(t, "2026-11-21T04:59:59Z")
	w, err := ParseWindow("today", "", before, toronto)
	if err != nil || !w.Since.Equal(instant(t, "2026-11-20T05:00:00Z")) {
		t.Fatalf("just before the Toronto boundary: %+v %v", w, err)
	}
	w, err = ParseWindow("today", "", before, time.UTC)
	if err != nil || !w.Since.Equal(instant(t, "2026-11-21T00:00:00Z")) {
		t.Fatalf("just before the same boundary in UTC: %+v %v", w, err)
	}
}

func TestParseWindowTransitions(t *testing.T) {
	for _, tc := range []struct {
		zone, date, start, included string
	}{
		{"Europe/London", "2026-03-29", "2026-03-29T00:00:00Z", "2026-03-29T22:59:59Z"},
		{"America/Sao_Paulo", "2018-11-04", "2018-11-04T03:00:00Z", "2018-11-04T03:30:00Z"},
		{"America/Goose_Bay", "1988-10-30", "1988-10-30T02:00:00Z", "1988-10-30T02:00:30Z"},
		{"America/Havana", "2026-11-01", "2026-11-01T04:00:00Z", "2026-11-01T04:00:01Z"},
		{"America/Havana", "2026-03-08", "2026-03-08T05:00:00Z", "2026-03-08T05:00:30Z"},
	} {
		w, err := ParseWindow(tc.date, tc.date, time.Time{}, zone(t, tc.zone))
		start := instant(t, tc.start)
		if err != nil || !w.Since.Equal(start) || !w.Contains(instant(t, tc.included)) || w.Contains(start.Add(-1)) {
			t.Errorf("%s %s: got %+v, err %v", tc.zone, tc.date, w, err)
		}
	}
	apia := zone(t, "Pacific/Apia")
	_, err := ParseWindow("2011-12-30", "2011-12-30", time.Time{}, apia)
	assertUsage(t, err)
	w, err := ParseWindow("2011-12-30", "", time.Time{}, apia)
	if err != nil || !w.Since.Equal(instant(t, "2011-12-30T10:00:00Z")) {
		t.Fatalf("skipped day did not advance: %+v, %v", w, err)
	}
}
