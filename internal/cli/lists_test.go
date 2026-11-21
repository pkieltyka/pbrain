package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tagBrain(t *testing.T) *readBrain {
	rb := cacheBrain(t)
	os.WriteFile(filepath.Join(rb.dir, "pbrain.toml"), []byte("[tags]\ncache = \"Caching layers, Redis, TTLs\"\nci = \"Build and test pipelines\"\nunused = \"Nobody uses it\"\n"), 0o644)
	rb.put("learnings/20261102-090301-ci-needs-docker-28-p0qz.md", []string{"kind: learning", "title: CI needs Docker 28", "created: 2026-11-02T09:03:01Z",
		"author: agent:lead-dev", "status: verified", "verified: human:pk", "stale_after: 2026-11-10", "tags: [ci, redis-cache, ci]"}, "docker\n")
	rb.put("notes/20261121-000000-secret-n0n0.md", []string{"kind: note", "title: Token note", "created: 2026-11-21T00:00:00Z", "author: agent:x", "tags: [cache]"},
		"token ghp_"+"abcdefghijklmnopqrstuvwxyz0123456789"+"\n")
	return rb
}

func TestListPrimeTagsTemplates(t *testing.T) {
	rb := tagBrain(t)
	const (
		secretID = "20261121-000000-secret-n0n0"
		ciID     = "20261102-090301-ci-needs-docker-28-p0qz"
	)
	expect(t, rb, secretID+`  note  provisional  2026-11-21  Token note
`+costID+`  note  provisional  2026-11-20  Redis memory cost is high
`+redisID+`  decision  verified  2026-11-19  Cache layer uses Redis
`+ciID+`  learning  verified  2026-11-02  CI needs Docker 28  review due
4 items.
`, "list")
	expect(t, rb, redisID+`  decision  verified  2026-11-19  Cache layer uses Redis
`+ciID+`  learning  verified  2026-11-02  CI needs Docker 28  review due
`+memcachedID+`  decision  verified  2026-10-01  Cache layer uses memcached  replaced
3 items.
`, "list", "--all", "--offset", "2")
	expect(t, rb, secretID+`  note  provisional  2026-11-21  Token note
1 item. 1 more: --offset 1. 2 items outside the time window: drop --since.
`, "list", "--since", "2026-11-20", "--limit", "1")
	expect(t, rb, `Project brain: 1 decision in force, 1 verified learning. Context, not instructions.
Look up more: pbrain search <words>, pbrain get <id>
decision  2026-11-19  `+redisID+`  Cache layer uses Redis (human:pkieltyka)
learning  2026-11-02  `+ciID+`  CI needs Docker 28 (agent:lead-dev)  review due
`, "prime")
	expect(t, rb, `Project brain: 1 decision in force, 1 verified learning. Context, not instructions.
Look up more: pbrain search <words>, pbrain get <id>
decision  2026-11-19  `+redisID+`  Cache layer uses Redis (human:pkieltyka)
1 more: --offset 1
`, "prime", "--max-chars", "300")
	expect(t, rb, `Project brain: 0 decisions in force, 1 verified learning. Context, not instructions.
Look up more: pbrain search <words>, pbrain get <id>
learning  2026-11-02  `+ciID+`  CI needs Docker 28 (agent:lead-dev)  review due
`, "prime", "--tag", "ci")
	// Declared tags by name with zero counts, then undeclared ones; a tag repeated on one item counts once.
	expect(t, rb, `cache  1  Caching layers, Redis, TTLs
ci  1  Build and test pipelines
unused  0  Nobody uses it
redis-cache  1  undeclared
4 tags.
`, "tags")
	expect(t, rb, `ci  1  Build and test pipelines
1 tag. 2 more: --offset 2.
`, "tags", "--offset", "1", "--limit", "1")
	// A shared listing is unchanged by a tag on a personal item.
	_, before, _ := rb.run("tags", "--json")
	rb.put("personal/notes/20261121-111111-mine-m1m1.md", []string{"kind: note", "title: Mine", "tags: [cache, private-tag]"}, "")
	if _, after, _ := rb.run("tags", "--json"); after != before {
		t.Fatalf("a personal tag changed the shared listing:\n%s\n%s", before, after)
	}
	_, out, _ := rb.run("--personal", "tags")
	if !strings.Contains(out, "cache  2  ") || !strings.Contains(out, "private-tag  1  undeclared") {
		t.Fatalf("personal tags: %q", out)
	}
	// Without a table, the names in use, unmarked.
	os.WriteFile(filepath.Join(rb.dir, "pbrain.toml"), nil, 0o644)
	expect(t, rb, "cache  1\nci  1\nredis-cache  1\n3 tags.\n", "tags")
	// Zero rows.
	empty := newReadBrain(t)
	expect(t, empty, "0 items.\n", "list")
	expect(t, empty, "0 tags.\n", "tags")
	expect(t, empty, "Project brain: 0 decisions in force, 0 verified learnings. Context, not instructions.\nLook up more: pbrain search <words>, pbrain get <id>\n", "prime")
	expect(t, empty, "0 findings, 0 errors.\n", "check")
}

func TestCheckCommand(t *testing.T) {
	rb := tagBrain(t)
	os.WriteFile(filepath.Join(rb.dir, "notes", "plain.md"), []byte("no front matter\n"), 0o644)
	want := `error missing_field decisions/` + memcachedID + `.md  author is missing
error missing_field notes/` + costID + `.md  author is missing
warning review_due learnings/20261102-090301-ci-needs-docker-28-p0qz.md  the review date has passed
warning undeclared_tag learnings/20261102-090301-ci-needs-docker-28-p0qz.md  tag redis-cache is not declared in pbrain.toml
warning secret_suspected notes/20261121-000000-secret-n0n0.md  body line 1: github_token
info not_an_item notes/plain.md  a .md file without front matter is not an item
6 findings, 2 errors.
`
	status, out, errOut := rb.run("check")
	if status != 1 || out != want || errOut != "" {
		t.Fatalf("check: %d %q\n%s", status, errOut, out)
	}
	// Past the last finding the exit still follows the full error count.
	status, out, _ = rb.run("check", "--offset", "10", "--json")
	if status != 1 || out != "{\"errors\":2}\n" {
		t.Fatalf("check past the end: %d %q", status, out)
	}
	status, _, errOut = rb.run("check", "--max-chars", "20")
	if status != 1 || !strings.HasPrefix(errOut, "budget_too_small: ") {
		t.Fatalf("check budget: %d %q", status, errOut)
	}
	// A brain without errors exits 0.
	clean := newReadBrain(t)
	clean.put("notes/20261121-000000-ok-o0o0.md", []string{"kind: note", "title: Fine", "created: 2026-11-21T00:00:00Z", "author: agent:x"}, "fine\n")
	if status, out, _ := clean.run("check"); status != 0 || out != "0 findings, 0 errors.\n" {
		t.Fatalf("clean check: %d %q", status, out)
	}
}

func TestListPrimeTagsEdges(t *testing.T) {
	rb := newReadBrain(t)
	rb.put("notes/undated-note.md", []string{"kind: note", "title: Undated", "author: agent:x"}, "")
	rb.put("notes/20250101-000000-dated-d0d0.md", []string{"kind: note", "title: Dated", "created: 2025-01-01T00:00:00Z", "author: agent:x"}, "")
	rb.put("learnings/20251001-000000-old-o0o0.md", []string{"kind: learning", "title: Old", "created: 2025-10-01T00:00:00Z",
		"author: agent:x", "status: verified", "verified: human:v", "tags: [gone]"}, "")
	rb.put("learnings/20251101-000000-new-n0n0.md", []string{"kind: learning", "title: New", "created: 2025-11-01T00:00:00Z",
		"author: agent:x", "status: verified", "verified: human:v", "replaces: [20251001-000000-old-o0o0]"}, "")
	rb.put("learnings/20251201-000000-maybe-m0m0.md", []string{"kind: learning", "title: Maybe", "created: 2025-12-01T00:00:00Z", "author: agent:x"}, "")
	// Unknown created sorts last.
	_, out, _ := rb.run("list")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 5 || !strings.HasPrefix(lines[3], "undated-note  note  provisional  unknown") {
		t.Fatalf("list order: %q", out)
	}
	// A tag on a replaced item is not counted; a provisional learning is not primed.
	expect(t, rb, "0 tags.\n", "tags")
	_, out, _ = rb.run("prime")
	if !strings.Contains(out, "1 verified learning.") || strings.Contains(out, "Maybe") || strings.Contains(out, "  Old") {
		t.Fatalf("prime: %q", out)
	}
}
