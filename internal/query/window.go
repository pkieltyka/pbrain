package query

import (
	"strconv"
	"time"
)

type Window struct {
	Since, Until       time.Time
	HasSince, HasUntil bool
	UntilIncl          bool // false when Until is the local day boundary
}

// ParseWindow parses optional time filters against one instant and one time zone.
func ParseWindow(since, until string, now time.Time, loc *time.Location) (Window, error) {
	var w Window
	if since != "" {
		value, _, ok := when(since, now, loc, false)
		if !ok {
			return Window{}, usage("--since: not a date, instant, today, yesterday or span")
		}
		w.Since, w.HasSince = value, true
	}
	if until != "" {
		value, inclusive, ok := when(until, now, loc, true)
		if !ok {
			return Window{}, usage("--until: not a date, instant, today, yesterday or span")
		}
		w.Until, w.HasUntil, w.UntilIncl = value, true, inclusive
	}
	if w.HasSince && w.HasUntil {
		last := w.Until
		if !w.UntilIncl {
			last = last.Add(-time.Nanosecond)
		}
		if w.Since.After(last) {
			return Window{}, usage("--since is after --until")
		}
	}
	return w, nil
}

func (w Window) Set() bool { return w.HasSince || w.HasUntil }

// Contains reports membership for a known created instant.
func (w Window) Contains(t time.Time) bool {
	return (!w.HasSince || !t.Before(w.Since)) &&
		(!w.HasUntil || t.Before(w.Until) || w.UntilIncl && t.Equal(w.Until))
}

func when(text string, now time.Time, loc *time.Location, upper bool) (value time.Time, inclusive, ok bool) {
	if text == "today" || text == "yesterday" {
		y, m, d := now.In(loc).Date()
		if text == "yesterday" {
			d--
		}
		return civilBound(y, m, d, loc, upper), false, true
	}
	// Parse the date as civil fields. Parsing in loc can normalize away a skipped date.
	if date, err := time.Parse(time.DateOnly, text); err == nil {
		y, m, d := date.Date()
		return civilBound(y, m, d, loc, upper), false, true
	}
	if instant, err := time.Parse(time.RFC3339, text); err == nil {
		return instant, true, true
	}
	if seconds, ok := spanSeconds(text); ok {
		return time.Unix(now.Unix()-seconds, int64(now.Nanosecond())), true, true
	}
	return time.Time{}, false, false
}

func spanSeconds(text string) (int64, bool) {
	if len(text) < 2 || len(text) > 7 {
		return 0, false
	}
	unit := text[len(text)-1]
	text = text[:len(text)-1]
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return 0, false
		}
	}
	n, _ := strconv.ParseInt(text, 10, 64) // at most six digits
	switch unit {
	case 'h':
		return n * 3600, true
	case 'd':
		return n * 86400, true
	case 'w':
		return n * 604800, true
	default:
		return 0, false
	}
}

func civilBound(y int, m time.Month, d int, loc *time.Location, upper bool) time.Time {
	if upper {
		d++
	}
	midnight := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return dayStart(midnight, loc)
}

// dayStart finds the earliest instant in the target civil date or later.
// Walking constant-offset segments also handles rollbacks across midnight.
func dayStart(midnight time.Time, loc *time.Location) time.Time {
	t := midnight.Add(-26 * time.Hour).In(loc)
	for range 64 {
		start, end := t.ZoneBounds()
		_, offset := t.Zone()
		candidate := midnight.Add(-time.Duration(offset) * time.Second).In(loc)
		if candidate.Before(start) {
			candidate = start
		}
		if candidate.Before(t) {
			candidate = t
		}
		if end.IsZero() || candidate.Before(end) {
			y, m, d := candidate.Date()
			if !time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Before(midnight) {
				return candidate
			}
		}
		if end.IsZero() {
			break
		}
		t = end
	}
	y, m, d := midnight.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}
