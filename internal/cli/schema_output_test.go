package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"maps"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/cli/schematest"
	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/gitx"
	"github.com/pkieltyka/pbrain/internal/item"
)

var updateSchemaGoldens = flag.Bool("update", false, "refresh schema and output golden files")

func assertSchemaGolden(t *testing.T, name string, data []byte) {
	t.Helper()
	path := filepath.Join("testdata", "schema", name+".json")
	if *updateSchemaGoldens {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if want := getBytes(t, path); !bytes.Equal(data, want) {
		t.Fatalf("output differs from %s\ngot:\n%s\nwant:\n%s", path, data, want)
	}
}

type outputCase struct {
	name string
	env  Env
	exit int
	args []string
}

func walkOutputCases(t *testing.T, visit func(outputCase)) {
	rb := tagBrain(t)
	rb.env.Rand = rand.New(rand.NewSource(80))
	rb.put("notes/broken.md", []string{"kind: invalid", "title: Broken"}, "")
	var old, refs []string
	for i := range 12 {
		id := fmt.Sprintf("predecessor-%02d", i)
		storeItem(t, rb.dir, "", id, item.Draft{Kind: item.Note}, "starting point\n")
		old = append(old, id)
		id = fmt.Sprintf("related-%02d", i)
		storeItem(t, rb.dir, "", id, item.Draft{Kind: item.Note, Relates: []string{"lantern-old"}}, "")
		refs = append(refs, id)
	}
	relations := append(append([]string(nil), refs...), "private", "missing-reference")
	rb.put("notes/lantern-old.md", []string{
		"kind: note", "title: lantern-old", "created: 2026-11-21T12:34:56Z", "author: agent:fixture",
		"verified: human:test", "source: written elsewhere", "stale_after: 2026-11-01", "tags: [cache]",
		"replaces: [" + strings.Join(old, ", ") + "]",
		"relates: [" + strings.Join(relations, ", ") + "]",
	}, "# Lantern\n\nlantern index keywords\n\n## Another heading\n\n"+strings.Repeat("catalog details and transcript ", 60))
	for i := range 12 {
		id := fmt.Sprintf("lantern-next-%02d", i)
		storeItem(t, rb.dir, "", id, item.Draft{Kind: item.Note, Replaces: []string{"lantern-old"}}, "lantern updates\n")
	}
	storeItem(t, rb.dir, "personal", "private", item.Draft{Kind: item.Learning, Status: item.Verified,
		Verified: "human:test", StaleAfter: "2026-11-01", Relates: []string{"lantern-old", "link-old", "private-decision"}},
		"private lantern learning\n")
	storeItem(t, rb.dir, "personal", "private-decision", item.Draft{Kind: item.Decision, Status: item.Verified,
		Decided: "human:test"}, "personal decision\n")
	storeItem(t, rb.dir, "", "link-old", item.Draft{Kind: item.Source, Origin: item.OriginLink,
		Source: "https://example.org/old", Relates: []string{"lantern-old"}}, "lantern reference\n")
	storeItem(t, rb.dir, "", "link-new", item.Draft{Kind: item.Source, Origin: item.OriginLink,
		Source: "https://example.org/new", Replaces: []string{"link-old"}, Relates: []string{"lantern-old"}}, "lantern reference\n")
	storeItem(t, rb.dir, "", "fetched", item.Draft{Kind: item.Source, Origin: item.OriginURL,
		Source: "https://example.org/fetched", Fetched: writeTime, SHA256: strings.Repeat("a", 64)}, "lantern reference\n")
	storeItem(t, rb.dir, "", "partial-match", item.Draft{Kind: item.Note}, "queue retry backoff worker lease\n")
	visitHere := func(name string, exit int, args ...string) { visit(outputCase{name, rb.env, exit, args}) }
	visitHere("search-context", 0, "search", "lantern index keywords", "--context=2", "--all", "--personal")
	visitHere("search-page", 0, "search", "lantern", "--limit=2", "--all", "--since=2026-11-20")
	visitHere("search-empty", 0, "search", "lantern missing")
	visitHere("search-partial", 0, "search", "queue retry timeout backoff worker lease", "--bm25")
	visitHere("search-personal-links", 0, "search", "private lantern learning", "--personal")
	visitHere("search-link", 0, "search", "lantern reference", "--all", "--limit=2")
	visitHere("search-decision", 0, "search", "redis", "--limit=1")
	storeItem(t, rb.dir, "", "long-section", item.Draft{Kind: item.Note},
		"# Intro\n\n"+strings.Repeat("intro "+strings.Repeat("background ", 13)+"\n\n", 70)+"## Lookup\n\nuniquephrase\n")
	visitHere("search-section", 0, "search", "uniquephrase")
	visitHere("get-body", 0, "get", "lantern-old", "--max-chars=1400", "--personal")
	visitHere("get-body-links", 0, "get", "lantern-old", "--max-chars=12000", "--personal")
	visitHere("get-fetched", 0, "get", "fetched")
	visitHere("get-link", 0, "get", "link-old")
	visitHere("get-personal", 0, "get", "private", "--personal")
	visitHere("get-history", 0, "get", "lantern-old", "--history", "--limit=18", "--max-chars=12000")
	visitHere("get-history-link", 0, "get", "link-old", "--history")
	visitHere("get-history-personal", 0, "get", "private", "--history", "--personal")
	visitHere("get-history-actors", 0, "get", redisID, "--history")
	visitHere("get-history-empty", 0, "get", "lantern-old", "--history", "--offset=99")
	visitHere("get-outline", 0, "get", "lantern-old", "--outline", "--limit=1")
	visitHere("get-decision", 0, "get", redisID)
	visitHere("list-page", 0, "list", "--limit=2", "--since=2026-11-20")
	visitHere("list-all", 0, "list", "--all", "--personal", "--limit=50", "--max-chars=20000")
	visitHere("prime", 0, "prime", "--personal")
	visitHere("prime-page", 0, "prime", "--personal", "--max-chars=330")
	visitHere("tags", 0, "tags", "--limit=2")
	visitHere("tags-undeclared", 0, "tags", "--offset=3")
	visitHere("check", 1, "check", "--limit=2")
	visitHere("audience-empty", 0, "audience")
	visitHere("audience-create", 0, "audience", "set", "example.org", "personal")
	visitHere("audience-change", 0, "audience", "set", "example.net", "shared")
	visitHere("audience-list", 0, "audience", "--limit=1")
	visitHere("audience-unset", 0, "audience", "unset", "example.org")
	visitHere("verify", 0, "verify", "partial-match", "--by=human:test")
	visitHere("add", 0, "add", "--author=agent:test", "--title=A new note")
	visitHere("import", 0, "import", "--link-only", "--shared", "--author=agent:test", "https://example.com/schema")

	copiedFixture(t, rb.env, true)
	for _, id := range old {
		// Make each predecessor current again when the copied item is removed.
		storeItem(t, rb.dir, "", id, item.Draft{Kind: item.Source, Origin: item.OriginLink, Source: "https://example.com/" + id}, "")
	}
	// Source predecessors must have their own IDs and paths, without the note copies.
	for _, id := range old {
		if err := os.Remove(filepath.Join(rb.dir, "notes", id+".md")); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(rb.dir, "sources", "copy.md")
	data := getBytes(t, path)
	start, _, _ := bytes.Cut(data, []byte("\n---\n"))
	putBytes(t, rb.dir, "sources/copy.md", []byte(string(start)+"\nreplaces: ["+strings.Join(old, ", ")+"]\n---\nstored text"))
	for _, id := range refs {
		storeItem(t, rb.dir, "", id, item.Draft{Kind: item.Note, Relates: []string{"copy"}}, "")
	}
	visitHere("delete-preview", 1, "delete", "copy")
	visitHere("delete-result", 0, "delete", "copy", "--yes")

	model, kit := semanticBrain(t)
	devices := func() embed.Listing {
		return embed.Listing{Devices: []embed.DeviceInfo{{ID: "cuda:0", Backend: "cuda", Name: "Test GPU",
			Persistent: "cuda:GPU-test", Memory: "live", MemoryKind: "dedicated", TotalBytes: 16384, FreeBytes: 8192}}}
	}
	useDeviceModel(model, kit, devices, func(ctx context.Context, a *embed.Artifact, o embed.OpenOptions) (embed.Embedder, error) {
		enc, err := kit.Registry.Open(ctx, a, embed.OpenOptions{})
		if err != nil {
			return nil, err
		}
		return &deviceEmbedder{Embedder: enc, info: embed.RunInfo{Device: o.Device}}, nil
	})
	visit(outputCase{"devices", model.env, 0, []string{"devices"}})
	visit(outputCase{"sync-index", model.env, 0, []string{"sync", "--device=cuda:0"}})
	visit(outputCase{"search-semantic", model.env, 0, []string{"search", "alpha missing", "--semantic"}})

	manifest := filepath.Join(".pbrain", "index", "manifest")
	valid := getBytes(t, filepath.Join(model.dir, manifest))
	var index struct {
		Fingerprint string
	}
	if err := json.Unmarshal(valid, &index); err != nil || index.Fingerprint == "" {
		t.Fatalf("index fingerprint: %v", err)
	}
	putBytes(t, model.dir, manifest, bytes.Replace(valid, []byte(index.Fingerprint), []byte(strings.Repeat("a", len(index.Fingerprint))), 1))
	visit(outputCase{"sync-stale", model.env, 0, []string{"sync", "--status"}})
	putBytes(t, model.dir, manifest, []byte("broken manifest"))
	visit(outputCase{"sync-unreadable", model.env, 0, []string{"sync", "--status"}})

	env := writeEnv(t)
	parent := t.TempDir()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(git, "-C", parent, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", output, err)
	}
	env.Dir = filepath.Join(parent, "src")
	if err := os.Mkdir(env.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	env.Git = gitx.NewRunner(git, os.Environ())
	// The displayed path is relative to the repository, and the selector
	// is relative to the caller's directory inside it.
	visit(outputCase{"init", env, 0, []string{"init", "--path=outside"}})
}

func TestCatalogOutputs(t *testing.T) {
	seen := map[string]bool{}
	walkOutputCases(t, func(tc outputCase) {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"--json"}, tc.args...)
			code, stdout, stderr := invoke(t, tc.env, Features{BM25: true}, args...)
			if code != tc.exit || stdout == "" {
				t.Fatalf("%v: exit %d, stdout %s, stderr %s", args, code, stdout, stderr)
			}
			in, err := parseOptions(args, Features{BM25: true})
			if err != nil {
				t.Fatal(err)
			}
			variants := outputFor(in.cmd.name)
			variant := selectedOutput(in, variants)
			if err := schematest.Validate(schemaJSON(publishedOutput(variants)), false, []byte(stdout)); err != nil {
				t.Fatalf("published schema: %v", err)
			}
			var doc any
			oneJSON(t, stdout, &doc)
			recordOutputPaths(doc, in.cmd.name+"/"+variant.name, seen)
			// Devices lists the actual Go scheduler width. Pin that value to the
			// invocation's width first, then normalize the golden across hosts.
			if tc.name == "devices" {
				for _, row := range doc.(map[string]any)["devices"].([]any) {
					dev := row.(map[string]any)
					if dev["id"] == "cpu" {
						if dev["threads"] != float64(runtime.GOMAXPROCS(0)) {
							t.Fatal("CPU threads do not match the invocation")
						}
						dev["threads"] = 4
					}
				}
			}
			golden, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			assertSchemaGolden(t, "outputs/"+tc.name, append(golden, '\n'))
		})
	})
	for _, tool := range Catalog(Release) {
		for _, variant := range outputFor(tool.Command) {
			for _, path := range declaredOutputPaths(variant.schema, tool.Command+"/"+variant.name) {
				if !seen[path] {
					t.Errorf("output cases did not exercise %s", path)
				}
			}
		}
	}
}

func recordOutputPaths(doc any, path string, seen map[string]bool) {
	seen[path] = true
	switch value := doc.(type) {
	case map[string]any:
		for key, property := range value {
			recordOutputPaths(property, path+"."+key, seen)
		}
	case []any:
		for _, row := range value {
			recordOutputPaths(row, path+"[]", seen)
		}
	}
}

func declaredOutputPaths(schema *jsonSchema, path string) []string {
	paths := []string{path}
	for key, property := range schema.Properties {
		paths = append(paths, declaredOutputPaths(property, path+"."+key)...)
	}
	if schema.Items != nil {
		paths = append(paths, declaredOutputPaths(schema.Items, path+"[]")...)
	}
	return paths
}

func TestCatalogHints(t *testing.T) {
	walkOutputCases(t, func(tc outputCase) {
		readOnly := commandNamed(tc.args[0], Release).hints.ReadOnly
		t.Run(tc.name, func(t *testing.T) {
			var before map[string][32]byte
			if readOnly {
				before = outputTree(t, tc.env)
			}
			code, _, stderr := invoke(t, tc.env, Features{BM25: true}, append([]string{"--json"}, tc.args...)...)
			if code != tc.exit {
				t.Fatalf("exit %d, want %d: %s", code, tc.exit, stderr)
			}
			if readOnly && !maps.Equal(before, outputTree(t, tc.env)) {
				t.Fatal("read-only call changed the brain, personal section or cache")
			}
		})
	})
	if search := commandNamed("search", Release).hints; search.ReadOnly || search.Idempotent {
		t.Fatal("bounded semantic search can publish across repeated calls")
	}
	rb, _ := semanticBrain(t)
	expect(t, rb, "shared: 2 items indexed, 0 items removed; 2 passages embedded, 0 reused\n", "sync")
	manifest := filepath.Join(rb.dir, ".pbrain", "index", "manifest")
	before := getBytes(t, manifest)
	storeItem(t, rb.dir, "", "uncovered", item.Draft{Kind: item.Note}, "alpha\n")
	if code, _, stderr := rb.run("search", "alpha", "--semantic", "--json"); code != 0 {
		t.Fatal(stderr)
	}
	if bytes.Equal(before, getBytes(t, manifest)) {
		t.Fatal("semantic search did not advance the manifest")
	}
}

func outputTree(t *testing.T, env Env) map[string][32]byte {
	t.Helper()
	tree := map[string][32]byte{}
	for _, root := range []string{env.Dir, env.Getenv("PBRAIN_CACHE")} {
		if root == "" {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if path == root && os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if d.IsDir() {
				tree[path] = [32]byte{}
			} else {
				tree[path] = sha256.Sum256(getBytes(t, path))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return tree
}
