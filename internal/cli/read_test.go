package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/outline"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// readBrain is a brain on disk for the read commands, at one fixed instant.
type readBrain struct {
	t   *testing.T
	dir string
	env Env
}

func newReadBrain(t *testing.T) *readBrain {
	t.Helper()
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	env := Env{Dir: dir, Files: sys.Native(), Getenv: func(string) string { return "" }, Now: func() time.Time { return now }}
	rb := &readBrain{t: t, dir: dir, env: env}
	if status, out, stderr := invoke(t, env, Features{}, "init"); status != 0 {
		t.Fatalf("init: %d %s", status, stderr)
	} else if out != "initialized docs/brain\n" {
		t.Fatalf("init: %q", out)
	}
	rb.dir = filepath.Join(dir, "docs", "brain")
	rb.env.Dir = rb.dir
	return rb
}

// put writes an item file under the brain: front matter lines, then the body.
func (rb *readBrain) put(rel string, fm []string, body string) {
	rb.t.Helper()
	p := filepath.Join(rb.dir, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	text := "---\n" + strings.Join(fm, "\n") + "\n---\n" + body
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		rb.t.Fatal(err)
	}
}

func (rb *readBrain) run(args ...string) (int, string, string) {
	rb.t.Helper()
	return invoke(rb.t, rb.env, Features{}, args...)
}

const (
	redisID     = "20261119-182109-cache-uses-redis-k3f9"
	memcachedID = "20261001-101500-cache-uses-memcached-x7b2"
	costID      = "20261203-091500-redis-memory-cost-m2x8"
)

func cacheBrain(t *testing.T) *readBrain {
	rb := newReadBrain(t)
	rb.put("decisions/"+redisID+".md", []string{
		"kind: decision", "title: Cache layer uses Redis", "created: 2026-11-19T18:21:09Z",
		"author: agent:lead-dev", "decided: human:pkieltyka", "source: task 42, commit 3f2a9c1", "status: verified",
		"replaces: [" + memcachedID + "]", "relates: [" + costID + "]",
	}, "# Cache\n\nSessions expire through the cache TTL, so Redis replaces memcached for sessions.\n")
	rb.put("decisions/"+memcachedID+".md", []string{
		"kind: decision", "title: Cache layer uses memcached", "created: 2026-10-01T10:15:00Z", "decided: human:pkieltyka", "status: verified",
	}, "memcached for sessions\n")
	rb.put("notes/"+costID+".md", []string{"kind: note", "title: Redis memory cost is high", "created: 2026-11-20T09:15:00Z"}, "")
	return rb
}

func expect(t *testing.T, rb *readBrain, want string, args ...string) {
	t.Helper()
	status, out, errOut := rb.run(args...)
	if status != 0 || errOut != "" || out != want {
		t.Errorf("%v: status %d, stderr %q\n got:\n%s\nwant:\n%s", args, status, errOut, out, want)
	}
}

func TestReadModeTextTemplates(t *testing.T) {
	rb := cacheBrain(t)
	expect(t, rb, redisID+`  decision  verified  2026-11-19
  Cache layer uses Redis (decided by human:pkieltyka; source: task 42, commit 3f2a9c1)
  decisions/`+redisID+`.md:12  [body L12, offset 0/90; file 14L/374C; 2 occurrences]
  # Cache  Sessions expire through the cache TTL, so Redis replaces memcached for sessions. 
  relates to: `+costID+`, note, provisional, 2026-11-20, Redis memory cost is high
`+costID+`  note  provisional  2026-11-20
  Redis memory cost is high
  notes/`+costID+`.md  [body L6, 0C; file 5L/82C; 1 occurrence]
  newer related: `+redisID+`, decision, decided, 2026-11-19, Cache layer uses Redis
2 results.
`, "search", "redis")
	expect(t, rb, redisID+`  decision  verified  2026-11-19
  Cache layer uses Redis (decided by human:pkieltyka; source: task 42, commit 3f2a9c1)
  decisions/`+redisID+`.md:12  [body L12, offset 0/90; file 14L/374C; 2 occurrences]
    12: # Cache
    13:
    14: Sessions expire through the cache TTL, so Redis replaces memcached for sessions.
  relates to: `+costID+`, note, provisional, 2026-11-20, Redis memory cost is high
1 result. 1 replaced item also matched: --all.
`, "search", "sessions", "--context", "2")
	expect(t, rb, memcachedID+`  decision  verified  2026-10-01  replaced
  Cache layer uses memcached (decided by human:pkieltyka)
  decisions/`+memcachedID+`.md:8  [body L8, offset 0/23; file 8L/152C; 2 occurrences]
  memcached for sessions 
  replacement: `+redisID+`
1 result. 1 replaced item matched.
`, "search", "memcached", "--all", "--offset", "1")
	expect(t, rb, "0 results. Alone: nothing 0, here 0.\n", "search", "nothing", "here")
	expect(t, rb, "0 results.\n", "search", "nothing", "here", "--offset", "1")
	expect(t, rb, "0 results. 1 match outside the time window: drop --since --until. Alone: cache 0.\n", "search", "cache", "--since", "2026-11-21", "--until", "2026-11-21")
	expect(t, rb, redisID+`
decision  verified  2026-11-19T18:21:09Z
title: Cache layer uses Redis
author: agent:lead-dev  decided: human:pkieltyka
source: task 42, commit 3f2a9c1
replaces: `+memcachedID+`
relates: `+costID+`
path: decisions/`+redisID+`.md
body: L12, 90C; file 14L/374C
  relates to: `+costID+`, note, provisional, 2026-11-20, Redis memory cost is high

# Cache

Sessions expire through the cache TTL, so Redis replaces memcached for sessions.
[body 0 to 89 of 90; file L12–L14]
`, "get", redisID)
	expect(t, rb, memcachedID+`
decision  verified  2026-10-01T10:15:00Z  replaced
title: Cache layer uses memcached
decided: human:pkieltyka
replacement: `+redisID+`
current: `+redisID+`
path: decisions/`+memcachedID+`.md
body: L8, 23C; file 8L/152C

memcached for sessions
[body 0 to 22 of 23; file L8–L8]
`, "get", memcachedID)
	expect(t, rb, memcachedID+`  decision  verified  2026-10-01  Cache layer uses memcached (decided by human:pkieltyka)  replaced
`+redisID+`  decision  verified  2026-11-19  Cache layer uses Redis (decided by human:pkieltyka; replaces: `+memcachedID+`)
2 items.
`, "get", redisID, "--history")
	expect(t, rb, memcachedID+`  decision  verified  2026-10-01  Cache layer uses memcached (decided by human:pkieltyka)  replaced
1 item. 1 more: --offset 1.
`, "get", redisID, "--history", "--limit", "1")
	expect(t, rb, redisID+`
outline: 1 sections, body 90
0  90  L12  # Cache
1 section.
`, "get", redisID, "--outline")
	// An empty body: the header and nothing else.
	expect(t, rb, costID+`
note  provisional  2026-11-20T09:15:00Z
title: Redis memory cost is high
path: notes/`+costID+`.md
body: L6, 0C; file 5L/82C
  newer related: `+redisID+`, decision, decided, 2026-11-19, Cache layer uses Redis
`, "get", costID)
}

func TestReadJSONShapes(t *testing.T) {
	rb := cacheBrain(t)
	var res struct {
		Results []map[string]any `json:"results"`
		More    int              `json:"more"`
		Next    int              `json:"next"`
	}
	status, out, _ := rb.run("search", "redis", "--json", "--limit", "1")
	oneJSON(t, out, &res)
	if status != 0 || len(res.Results) != 1 || res.More != 1 || res.Next != 1 {
		t.Fatalf("%s", out)
	}
	r := res.Results[0]
	for key, want := range map[string]any{"id": redisID, "path": "decisions/" + redisID + ".md", "body_line": 12.0, "body_chars": 90.0,
		"file_lines": 14.0, "file_chars": 374.0, "occurrences": 2.0, "snippet_line": 12.0, "snippet_offset": 0.0, "status": "verified"} {
		if r[key] != want {
			t.Errorf("%s = %v, want %v", key, r[key], want)
		}
	}
	for _, absent := range []string{"semantic", "partial", "matched", "clauses", "context", "section", "replaced", "personal"} {
		if _, ok := r[absent]; ok {
			t.Errorf("%s is present", absent)
		}
	}
	var get map[string]any
	_, out, _ = rb.run("get", memcachedID, "--json")
	oneJSON(t, out, &get)
	if fmt.Sprint(get["replacement"]) != "["+redisID+"]" || fmt.Sprint(get["current"]) != "["+redisID+"]" || get["line_from"] != 8.0 || get["offset"] != 0.0 {
		t.Fatalf("get JSON: %s", out)
	}
	var ol map[string]any
	_, out, _ = rb.run("get", redisID, "--outline", "--json")
	oneJSON(t, out, &ol)
	rows, _ := ol["outline"].([]any)
	if ol["sections"] != 1.0 || ol["body_chars"] != 90.0 || len(rows) != 1 || fmt.Sprint(rows[0]) != "map[chars:90 file_line:12 level:1 offset:0 title:Cache]" {
		t.Fatalf("outline JSON: %s", out)
	}
	var hist struct {
		History []map[string]any `json:"history"`
	}
	_, out, _ = rb.run("get", redisID, "--history", "--json")
	oneJSON(t, out, &hist)
	if len(hist.History) != 2 || fmt.Sprint(hist.History[1]["replaces"]) != "["+memcachedID+"]" {
		t.Fatalf("history JSON: %s", out)
	}
}

func TestReadColourAndBudget(t *testing.T) {
	rb := cacheBrain(t)
	status, out, _ := rb.run("search", "memcached", "--color", "--limit", "1")
	if status != 0 || !strings.HasPrefix(out, "\x1b[1m"+redisID+"\x1b[0m") || !strings.Contains(out, "\x1b[36mdecisions/"+redisID+".md:12\x1b[0m  [") {
		t.Fatalf("colour: %q", out)
	}
	status, out, _ = rb.run("get", redisID, "--color")
	if status != 0 || !strings.Contains(out, "path: \x1b[36mdecisions/"+redisID+".md\x1b[0m\n") {
		t.Fatalf("get colour: %q", out)
	}
	status, _, errOut := rb.run("search", "redis", "--max-chars", "50")
	if status != 1 || !strings.HasPrefix(errOut, "budget_too_small: ") {
		t.Fatalf("a tight budget: %d %q", status, errOut)
	}
	// A body sliced by the budget continues with --offset.
	status, out, errOut = rb.run("get", redisID, "--max-chars", "560")
	if status != 0 || !strings.Contains(out, "more: --offset ") {
		t.Fatalf("a sliced body: %q %q", out, errOut)
	}
}

func TestReadIDsAndNotices(t *testing.T) {
	rb := cacheBrain(t)
	rb.put("notes/20261121-101010-alpha-aaaa.md", []string{"kind: note", "title: Alpha one"}, "x\n")
	rb.put("notes/20261121-101010-alpha-bbbb.md", []string{"kind: note", "title: Alpha two"}, "x\n")
	rb.put("other/20261121-101010-alpha-aaaa.md", []string{"kind: note", "title: Alpha copy"}, "x\n")
	// A mistyped random suffix: the stamp prefix suggests; the whole prefix comes first.
	status, _, errOut := rb.run("get", "20261121-101010-alpha-zzzz")
	if status != 1 || errOut != "not_found: no item has that id in this view; ids that start alike: 20261121-101010-alpha-aaaa, 20261121-101010-alpha-bbbb\n" {
		t.Fatalf("stamp suggestions: %q", errOut)
	}
	_, _, errOut = rb.run("get", "20261119-182109-cache")
	if errOut != "not_found: no item has that id in this view; ids that start alike: "+redisID+"\n" {
		t.Fatalf("prefix suggestions: %q", errOut)
	}
	_, _, errOut = rb.run("get", "zzz")
	if errOut != "not_found: no item has that id in this view\n" {
		t.Fatalf("no suggestion: %q", errOut)
	}
	status, _, errOut = rb.run("get", "20261121-101010-alpha-aaaa")
	if status != 1 || !strings.HasPrefix(errOut, "ambiguous_id: ") {
		t.Fatalf("ambiguous: %d %q", status, errOut)
	}
	// A file the scan cannot use is counted once on stderr.
	os.WriteFile(filepath.Join(rb.dir, "notes", "broken.md"), []byte("---\nkind: [\n---\n"), 0o644)
	status, out, errOut := rb.run("search", "redis", "--json")
	var notice struct {
		Notice struct{ Code, Message string }
	}
	oneJSON(t, errOut, &notice)
	if status != 0 || out == "" || notice.Notice.Code != "skipped_files" || notice.Notice.Message != "1 file skipped: run pbrain check" {
		t.Fatalf("skipped: %d %q", status, errOut)
	}
}

// TestSearchGetOutlineSectionAlignment is render.md's alignment fixture, lexical half: the location,
// the get slice at the snippet's offset and the outline rows agree, with LF, CRLF and a BOM.
func TestSearchGetOutlineSectionAlignment(t *testing.T) {
	for _, variant := range []string{"lf", "crlf", "bom"} {
		t.Run(variant, func(t *testing.T) {
			rb := newReadBrain(t)
			os.WriteFile(filepath.Join(rb.dir, "pbrain.toml"), []byte("[get]\nmax_chars = 1000\n"), 0o644)
			filler := func(word string, n int) string { return strings.TrimSpace(strings.Repeat(word+" ", n)) }
			body := "Préambule ünïcode " + filler("intro", 30) + "\n\n" +
				"# Alpha\n\n" + filler("alpha", 40) + "\n\n## Setup\n\n" + filler("first", 60) + "\n\n" +
				"```\n# not a heading\n```\n\n" +
				"# Beta\n\nzq " + filler("beta", 40) + "\n\n## Setup\n\n" + filler("second", 60) + " uniqueword " + filler("tail", 10) + "\n"
			fm := []string{"kind: note", "title: Alignment", "created: 2026-11-01T00:00:00Z"}
			text := "---\n" + strings.Join(fm, "\n") + "\n---\n" + body
			if variant == "crlf" {
				text = strings.ReplaceAll(text, "\n", "\r\n")
			}
			if variant == "bom" {
				text = "\ufeff" + text
			}
			id := "20261101-000000-alignment-a1b2"
			os.MkdirAll(filepath.Join(rb.dir, "notes"), 0o755)
			os.WriteFile(filepath.Join(rb.dir, "notes", id+".md"), []byte(text), 0o644)

			var res struct {
				Results []struct {
					SnippetLine   int    `json:"snippet_line"`
					SnippetOffset int    `json:"snippet_offset"`
					Snippet       string `json:"snippet"`
					Section       string `json:"section"`
					BodyLine      int    `json:"body_line"`
				} `json:"results"`
			}
			status, out, errOut := rb.run("search", "uniqueword", "--json")
			if status != 0 {
				t.Fatal(errOut)
			}
			oneJSON(t, out, &res)
			if len(res.Results) != 1 || res.Results[0].Section != "Setup" {
				t.Fatalf("search: %s", out)
			}
			hit := res.Results[0]
			// The text locator names the same line, offset and section.
			_, textOut, _ := rb.run("search", "uniqueword")
			if !strings.Contains(textOut, fmt.Sprintf(".md:%d  [body L%d, offset %d/", hit.SnippetLine, hit.BodyLine, hit.SnippetOffset)) || !strings.Contains(textOut, "] [in Setup]") {
				t.Fatalf("text locator: %s", textOut)
			}
			// get at the offset starts on the snippet's line with the snippet's text.
			var got struct {
				Body     string `json:"body"`
				LineFrom int    `json:"line_from"`
			}
			_, out, _ = rb.run("get", id, "--offset", fmt.Sprint(hit.SnippetOffset), "--json", "--max-chars", "2000")
			oneJSON(t, out, &got)
			flat := strings.Join(strings.Fields(got.Body), " ")
			want := strings.Join(strings.Fields(strings.Trim(hit.Snippet, "…")), " ")
			if got.LineFrom != hit.SnippetLine || !strings.HasPrefix(flat, want[:40]) {
				t.Fatalf("get at %d: line %d (want %d), %q vs %q", hit.SnippetOffset, got.LineFrom, hit.SnippetLine, flat[:60], want[:40])
			}
			// The outline: the fenced pseudo-heading has no row, and each row's offset opens its heading.
			var ol struct {
				Outline []struct {
					Offset   int    `json:"offset"`
					FileLine int    `json:"file_line"`
					Level    int    `json:"level"`
					Title    string `json:"title"`
				} `json:"outline"`
			}
			_, out, _ = rb.run("get", id, "--outline", "--json")
			if err := json.Unmarshal([]byte(out), &ol); err != nil {
				t.Fatal(err)
			}
			raw, _ := os.ReadFile(filepath.Join(rb.dir, "notes", id+".md"))
			_ = raw
			var titles []string
			for _, row := range ol.Outline {
				titles = append(titles, row.Title)
			}
			if strings.Join(titles, ",") != ",Alpha,Setup,Beta,Setup" {
				t.Fatalf("outline rows: %q", titles)
			}
			stored := body
			if variant == "crlf" {
				stored = strings.ReplaceAll(body, "\n", "\r\n")
			}
			secs := outline.Parse([]byte(stored))
			second := ol.Outline[4]
			if hit.SnippetOffset < second.Offset-200 || outline.At(secs, second.Offset) != 4 {
				t.Fatalf("the second Setup row: %+v", second)
			}
			_, out, _ = rb.run("get", id, "--offset", fmt.Sprint(second.Offset), "--json", "--max-chars", "800")
			oneJSON(t, out, &got)
			if !strings.HasPrefix(got.Body, "## Setup") || got.LineFrom != second.FileLine {
				t.Fatalf("get at the second Setup: line %d (want %d) %q", got.LineFrom, second.FileLine, got.Body[:20])
			}
			// A sole match ending at S + 10, S the start of Beta: the snippet starts at S - 190, in the
			// section before it, and that offset opens there in get.
			beta := ol.Outline[3].Offset
			status, out, _ = rb.run("search", "zq", "--json")
			oneJSON(t, out, &res)
			// With CRLF the match ends at S + 12: the position is always its end minus 200.
			end := len([]rune(stored[:strings.Index(stored, "zq")])) + 2
			if end-beta > 12 || status != 0 || len(res.Results) != 1 || res.Results[0].SnippetOffset != end-200 || res.Results[0].Section != "Setup" {
				t.Fatalf("a match at the top of Beta (S = %d): %s", beta, out)
			}
			_, out, _ = rb.run("get", id, "--offset", fmt.Sprint(end-200), "--json", "--max-chars", "2000")
			oneJSON(t, out, &got)
			if got.LineFrom != res.Results[0].SnippetLine {
				t.Fatalf("get at S - 190: line %d, snippet line %d", got.LineFrom, res.Results[0].SnippetLine)
			}
		})
	}
}

// A personal item is invisible to shared reads: not listed, not counted, not suggested, not related.
func TestReadPersonalView(t *testing.T) {
	rb := cacheBrain(t)
	_, sharedBefore, _ := rb.run("search", "redis", "--json")
	const mine = "20261121-090000-redis-private-p1p1"
	rb.put("personal/notes/"+mine+".md", []string{"kind: note", "title: Redis private note", "created: 2026-11-21T09:00:00Z",
		"relates: [" + redisID + "]"}, "redis secrets of mine\n")
	status, sharedAfter, errOut := rb.run("search", "redis", "--json")
	if status != 0 || sharedAfter != sharedBefore || errOut != "" {
		t.Fatalf("a personal item changed the shared search:\n%s\n%s %s", sharedBefore, sharedAfter, errOut)
	}
	_, out, _ := rb.run("search", "nothingatall", "redis")
	if out != "0 results. Alone: nothingatall 0, redis 2.\n" {
		t.Fatalf("shared alone counts: %q", out)
	}
	status, _, errOut = rb.run("get", mine)
	if status != 1 || errOut != "not_found: no item has that id in this view\n" {
		t.Fatalf("shared get of a personal id: %q", errOut)
	}
	_, _, errOut = rb.run("get", "20261121-090000-redis")
	if strings.Contains(errOut, mine) {
		t.Fatalf("a shared suggestion named a personal id: %q", errOut)
	}
	status, out, _ = rb.run("--personal", "search", "private")
	if status != 0 || !strings.HasPrefix(out, mine+"  note  provisional  2026-11-21  personal\n") {
		t.Fatalf("personal search: %q", out)
	}
	_, out, _ = rb.run("--personal", "get", redisID)
	if !strings.Contains(out, "newer related: "+mine+", note, provisional, 2026-11-21, Redis private note, personal") {
		t.Fatalf("personal related row: %q", out)
	}
}

// A context block's focal line shows "…" where its line goes on before or after the stretch.
func TestReadContextEllipsis(t *testing.T) {
	rb := newReadBrain(t)
	line := strings.Repeat("lorem ", 80) + "needle " + strings.Repeat("ipsum ", 80)
	rb.put("notes/20261101-000000-long-line-a1a1.md", []string{"kind: note", "title: Long line"}, "before\n"+line+"\nafter\n")
	status, out, _ := rb.run("search", "needle", "--context", "1")
	if status != 0 || !strings.Contains(out, "    5: before\n    6: …m lorem") || !strings.Contains(out, " needle…\n    7: after\n") {
		t.Fatalf("focal ellipses: %q", out)
	}
}

// changingFiles rewrites one file just before the opens it is told to.
type changingFiles struct {
	sys.Files
	name   string
	path   string
	opens  int
	change map[int]bool
}

func (c *changingFiles) OpenRegular(dir sys.Dir, name string) (sys.RegularFile, error) {
	if name == c.name {
		c.opens++
		if c.change[c.opens] {
			f, _ := os.OpenFile(c.path, os.O_APPEND|os.O_WRONLY, 0)
			f.WriteString(fmt.Sprintf("changed %d\n", c.opens))
			f.Close()
		}
	}
	return c.Files.OpenRegular(dir, name)
}

// A body that changed since the scan is scanned again once; a second change is conflict.
func TestReadChangedBody(t *testing.T) {
	rb := cacheBrain(t)
	name := redisID + ".md"
	path := filepath.Join(rb.dir, "decisions", name)
	// Learn how often one get opens the file when nothing changes: once to scan, once to read.
	spy := &changingFiles{Files: sys.Native(), name: name, path: path}
	rb.env.Files = spy
	if status, _, errOut := rb.run("get", redisID); status != 0 || spy.opens != 2 {
		t.Fatalf("an unchanged get: %d %q, %d opens", status, errOut, spy.opens)
	}
	rb.env.Files = &changingFiles{Files: sys.Native(), name: name, path: path, change: map[int]bool{2: true}}
	status, out, errOut := rb.run("get", redisID)
	if status != 0 || !strings.Contains(out, "changed 2") {
		t.Fatalf("one change: %d %q %q", status, out, errOut)
	}
	rb.env.Files = &changingFiles{Files: sys.Native(), name: name, path: path, change: map[int]bool{2: true, 4: true}}
	status, out, errOut = rb.run("get", redisID, "--json")
	var e struct{ Error errs.Notice }
	oneJSON(t, errOut, &e)
	if status != 1 || out != "" || e.Error.Code != "conflict" {
		t.Fatalf("two changes: %d %q %q", status, out, errOut)
	}
}
