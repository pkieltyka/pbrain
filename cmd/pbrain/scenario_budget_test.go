package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rogpeppe/go-internal/testscript"
)

var textNext = regexp.MustCompile(`more: --offset ([0-9]+)`)
var bodyRange = regexp.MustCompile(`(?m)^\[body ([0-9]+) to ([0-9]+) of ([0-9]+);`)

var minimumLimit = regexp.MustCompile("smallest limit that works is ([0-9]+)")

// budgets MODE ARGS... checks the emitted wire form, never the fitter's internal counts.
// A check report may exit 1 when there are errors anywhere in the view.
func scenarioBudgets(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) < 2 {
		ts.Fatalf("usage: budgets ROW-KEY|body ARGS...")
	}
	key, cmd := args[0], args[1:]
	run := func(args []string) string {
		err := ts.Exec("pbrain", args...)
		out := ts.ReadFile("stdout")
		if err != nil && !(len(out) > 0 && cmd[0] == "check") {
			ts.Fatalf("%v: %v %s", args, err, ts.ReadFile("stderr"))
		}
		return out
	}
	for _, jsonMode := range []bool{false, true} {
		argv := append([]string{}, cmd...)
		if jsonMode {
			argv = append(argv, "--json")
		}
		for _, off := range []int{0, 999} {
			base := append(append([]string{}, argv...), "--offset", strconv.Itoa(off))
			if ts.Exec("pbrain", append(base, "--max-chars", "1")...) == nil || ts.ReadFile("stdout") != "" {
				ts.Fatalf("expected empty stdout on budget refusal")
			}
			diag := ts.ReadFile("stderr")
			m := minimumLimit.FindStringSubmatch(diag)
			if m == nil || !strings.Contains(diag, "budget_too_small") {
				ts.Fatalf("missing budget diagnostic: %s", diag)
			}
			limit, err := strconv.Atoi(m[1])
			ts.Check(err)
			out := run(append(base, "--max-chars", m[1]))
			if utf8.RuneCountInString(out) > limit || !strings.HasSuffix(out, "\n") {
				ts.Fatalf("response exceeds %d or lacks newline: %q", limit, out)
			}
			if jsonMode && (!json.Valid([]byte(out)) || strings.Count(out, "\n") != 1 || strings.Contains(out, `"more":0`)) {
				ts.Fatalf("bad JSON envelope: %q", out)
			}
		}
	}
	// Compare a complete response against consecutive JSON pages, including all repeated values.
	base := append(append([]string{}, cmd...), "--json")
	var full map[string]json.RawMessage
	ts.Check(json.Unmarshal([]byte(run(append(base, "--max-chars", "100000"))), &full))
	checkTextPages(ts, cmd, key, full, run)
	if key == "body" {
		var want string
		if raw := full["body"]; raw != nil {
			ts.Check(json.Unmarshal(raw, &want))
		}
		var got strings.Builder
		for off := 0; off <= utf8.RuneCountInString(want); {
			a := append(append([]string{}, base...), "--offset", strconv.Itoa(off))
			_ = ts.Exec("pbrain", append(a, "--max-chars", "1")...)
			m := minimumLimit.FindStringSubmatch(ts.ReadFile("stderr"))
			if m == nil {
				ts.Fatalf("body minimum absent")
			}
			var p struct {
				Body string
				Next int
			}
			ts.Check(json.Unmarshal([]byte(run(append(a, "--max-chars", m[1]))), &p))
			got.WriteString(p.Body)
			if p.Next == 0 {
				break
			}
			if p.Next != off+utf8.RuneCountInString(p.Body) || p.Next <= off {
				ts.Fatalf("bad continuation %+v", p)
			}
			off = p.Next
		}
		if got.String() != want {
			ts.Fatalf("body pages %q, want %q", got.String(), want)
		}
		return
	}
	var want, got []json.RawMessage
	if raw := full[key]; raw != nil {
		ts.Check(json.Unmarshal(raw, &want))
	}
	for off := 0; ; {
		a := append(append([]string{}, base...), "--offset", strconv.Itoa(off))
		if cmd[0] != "prime" {
			a = append(a, "--limit", "1")
		} else {
			a = append(a, "--max-chars", minimumForPage(ts, a))
		}
		var page map[string]json.RawMessage
		ts.Check(json.Unmarshal([]byte(run(a)), &page))
		var rows []json.RawMessage
		if raw := page[key]; raw != nil {
			ts.Check(json.Unmarshal(raw, &rows))
		}
		got = append(got, rows...)
		if page["next"] == nil {
			break
		}
		var next int
		ts.Check(json.Unmarshal(page["next"], &next))
		if next != off+len(rows) || next <= off || next > len(want) {
			ts.Fatalf("nonadvancing or skipped page: %v", page)
		}
		off = next
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		ts.Fatalf("paged rows differ for %v", cmd)
	}
}

func minimumForPage(ts *testscript.TestScript, args []string) string {
	_ = ts.Exec("pbrain", append(append([]string{}, args...), "--max-chars", "1")...)
	m := minimumLimit.FindStringSubmatch(ts.ReadFile("stderr"))
	if m == nil {
		ts.Fatalf("missing budget_too_small minimum for %q", args)
	}
	return m[1]
}

// Reassemble the text pages too. Prime has a fixed header and no --limit,
// so its single-row pages are fitted at their advertised minimum.
func checkTextPages(ts *testscript.TestScript, cmd []string, key string, full map[string]json.RawMessage, run func([]string) string) {
	argsAt := func(off int) []string {
		return append(append([]string{}, cmd...), "--offset", strconv.Itoa(off))
	}
	if key == "body" {
		var body string
		if full[key] != nil {
			ts.Check(json.Unmarshal(full[key], &body))
		}
		chars := []rune(body)
		for off := 0; off < len(chars); {
			a := argsAt(off)
			out := run(append(a, "--max-chars", minimumForPage(ts, a)))
			loc := bodyRange.FindStringSubmatchIndex(out)
			if loc == nil {
				ts.Fatalf("missing printed body range: %q", out)
			}
			start, _ := strconv.Atoi(out[loc[2]:loc[3]])
			last, _ := strconv.Atoi(out[loc[4]:loc[5]])
			total, _ := strconv.Atoi(out[loc[6]:loc[7]])
			if start != off || last < off || last >= len(chars) || total != len(chars) {
				ts.Fatalf("wrong body range at %d: %s", off, out)
			}
			part := string(chars[start : last+1])
			if !strings.HasSuffix(part, "\n") {
				part += "\n"
			}
			if !strings.HasSuffix(out[:loc[0]], "\n\n"+part) {
				ts.Fatalf("wrong text at offset %d: %q", off, out)
			}
			off = last + 1
			requireNext(ts, out[loc[0]:], off, len(chars))
		}
		return
	}
	var rows []json.RawMessage
	if full[key] != nil {
		ts.Check(json.Unmarshal(full[key], &rows))
	}
	// All row commands end in a footer, except prime on its last page.
	payload := func(out string, more bool) string {
		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if cmd[0] != "prime" || more {
			lines = lines[:len(lines)-1]
		}
		if cmd[0] == "prime" || key == "outline" {
			if len(lines) < 2 {
				ts.Fatalf("missing header for %v: %q", cmd, out)
			}
			lines = lines[2:]
		}
		return strings.Join(lines, "\n")
	}
	complete := payload(run(append(append([]string{}, cmd...), "--max-chars", "100000")), false)
	var pages []string
	for off := range len(rows) {
		a := argsAt(off)
		if cmd[0] == "prime" {
			a = append(a, "--max-chars", minimumForPage(ts, a))
		} else {
			a = append(a, "--limit", "1")
		}
		out := run(a)
		requireNext(ts, out, off+1, len(rows))
		pages = append(pages, payload(out, off+1 < len(rows)))
	}
	if strings.Join(pages, "\n") != complete {
		ts.Fatalf("text pages differ for %q: %q against %q", cmd, pages, complete)
	}
}

func requireNext(ts *testscript.TestScript, out string, next, total int) {
	m := textNext.FindStringSubmatch(out)
	if next < total {
		if m == nil || m[1] != strconv.Itoa(next) {
			ts.Fatalf("expected --offset %d: %q", next, out)
		}
	} else if m != nil {
		ts.Fatalf("unexpected continuation at end: %q", out)
	}
}
