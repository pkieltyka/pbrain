package cli

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkieltyka/pbrain/internal/importer/fetch"
	"github.com/pkieltyka/pbrain/internal/item"
)

// Requests travel only to our local server; record every dial and only accept the
// numeric address returned by the injected resolver.
type importNetwork struct {
	t        *testing.T
	local    string
	dials    atomic.Int32
	onLookup func()
}

func importServer(t *testing.T, h http.HandlerFunc) (*importNetwork, fetch.Net) {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	n := &importNetwork{t: t, local: s.Listener.Addr().String()}
	return n, fetch.Net{Resolver: n, Dialer: n}
}

func (n *importNetwork) LookupNetIP(_ context.Context, host string) ([]netip.Addr, error) {
	if n.onLookup != nil {
		n.onLookup()
	}
	public := netip.MustParseAddr("8.8.8.8")
	switch host {
	case "mixed.test":
		return []netip.Addr{public, netip.MustParseAddr("127.0.0.1")}, nil
	case "article.test", "final.test":
		return []netip.Addr{public}, nil
	default:
		n.t.Errorf("unexpected lookup %q", host)
		return nil, fmt.Errorf("unexpected host")
	}
}

func (n *importNetwork) DialContext(ctx context.Context, network string, address netip.AddrPort) (net.Conn, error) {
	n.dials.Add(1)
	if address != netip.MustParseAddrPort("8.8.8.8:80") {
		n.t.Errorf("unexpected dial %v", address)
		return nil, fmt.Errorf("unexpected address")
	}
	var dial net.Dialer
	return dial.DialContext(ctx, network, n.local)
}

func TestURLImportProvenance(t *testing.T) {
	raw := getBytes(t, filepath.Join("..", "importer", "testdata", "article.html"))
	n, network := importServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Host {
		case "article.test":
			http.Redirect(w, r, "http://final.test/guide/part/page", http.StatusFound)
		case "final.test":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Encoding", "gzip")
			g := gzip.NewWriter(w)
			_, _ = g.Write(raw)
			_ = g.Close()
		default:
			t.Errorf("unexpected request %s", r.Host)
		}
	})
	env := writeEnv(t)
	env.Net = network
	// A rule on the redirect destination must not route the source.
	audienceOK(t, env, "set", "final.test", "personal")
	const source = "http://article.test/start?q=1#partA"
	var out, diag bytes.Buffer
	env.Args = []string{"import", source, "--json"}
	env.Stdout, env.Stderr = &out, &diag
	n.onLookup = func() {
		if !strings.Contains(diag.String(), "\"code\":\"destination\"") || !strings.Contains(diag.String(), "copy of the original") {
			t.Error("DNS lookup preceded destination notice")
		}
	}
	if exit := Run(t.Context(), env); exit != 0 || n.dials.Load() != 2 {
		t.Fatalf("import: %d, dials %d: %s", exit, n.dials.Load(), &diag)
	}
	var row writeOutput
	oneJSON(t, out.String(), &row)
	data := getBytes(t, filepath.Join(env.Dir, row.Path))
	h, at, _, err := item.Parse(row.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	if row.Section != "shared" || h.Origin != item.OriginURL || h.Source != source || h.Title != "Assembly notes" || h.SHA256 != digest || !h.Fetched.Equal(writeTime) || !bytes.Contains(data[at:], []byte("http://final.test/guide/manual")) {
		t.Fatalf("row %+v, header %+v, body %s", row, h, data[at:])
	}
	stored := filepath.Join(env.Dir, ".pbrain", "originals", digest)
	if !bytes.Equal(getBytes(t, stored), raw) {
		t.Fatal("original differs from decoded response bytes")
	}
}

func TestURLImportReplacesOnlyNamedDuplicates(t *testing.T) {
	env := writeEnv(t)
	_, env.Net = importServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		fmt.Fprint(w, "# Copied\nbody")
	})
	const url = "http://article.test/source?x#one"
	path := putBytes(t, t.TempDir(), "cited.md", []byte("Local export"))
	file, _, _ := describeWrite(t, env, "import", path, "--source", url, "--shared")
	link, _, _ := describeWrite(t, env, "import", "--link-only", url)
	// A merge can introduce a second current link for the same requested URL.
	const other = "merged-link"
	storeItem(t, env.Dir, "", other, item.Draft{Kind: item.Source, Origin: item.OriginLink, Source: url}, "")
	for _, ids := range [][]string{{}, {file.ID}, {link.ID}} {
		args := []string{url}
		for _, id := range ids {
			args = append(args, "--replaces", id)
		}
		diag := importFailure(t, env, "duplicate", args...)
		if !strings.Contains(diag, other) {
			t.Fatal(diag)
		}
	}
	w, h, _ := describeWrite(t, env, "import", url, "--replaces", link.ID, "--replaces", other)
	if w.ID == link.ID || len(h.Replaces) != 2 || h.Origin != item.OriginURL {
		t.Fatalf("replacement %+v", h)
	}
	if diag := importFailure(t, env, "duplicate", "--link-only", url); !strings.Contains(diag, w.ID) || strings.Contains(diag, file.ID) {
		t.Fatal(diag)
	}
	distinct, _, _ := describeWrite(t, env, "import", url+"-two")
	if distinct.ID == w.ID {
		t.Fatal("lost fragment distinction")
	}
}

func TestURLImportFailureStoresNothing(t *testing.T) {
	fixtures := filepath.Join("..", "importer", "testdata")
	short := getBytes(t, filepath.Join(fixtures, "short.html"))
	shell := getBytes(t, filepath.Join(fixtures, "shell.html"))
	n, network := importServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "http://127.0.0.1/", http.StatusFound)
		case "/large":
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Content-Encoding", "gzip")
			g := gzip.NewWriter(w)
			for range 81 {
				if _, err := io.WriteString(g, strings.Repeat("a", 65536)); err != nil {
					break
				}
			}
			_ = g.Close()
		case "/slow":
			<-r.Context().Done()
		case "/error":
			http.NotFound(w, r)
		case "/type":
			w.Header().Set("Content-Type", "image/png")
		case "/shell":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(shell)
		case "/short":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(short)
		default:
			t.Errorf("unexpected request %s", r.URL)
		}
	})
	for _, tc := range []struct {
		source, code string
		dials        int32
	}{
		{"http://127.0.0.1/", "fetch_blocked", 0}, {"http://mixed.test/", "fetch_blocked", 0},
		{"http://article.test/redirect", "fetch_blocked", 1}, {"http://article.test/large", "too_large", 1},
		{"http://article.test/error", "fetch_failed", 1}, {"http://article.test/type", "unsupported_type", 1},
		{"http://article.test/shell", "no_content", 1}, {"http://article.test/short", "no_content", 1},
		{"http://article.test/slow", "fetch_failed", 1},
	} {
		t.Run(tc.source, func(t *testing.T) {
			env := writeEnv(t)
			env.Net = network
			var out, diag bytes.Buffer
			env.Stdout, env.Stderr = &out, &diag
			env.Args = []string{"--json", "import", tc.source}
			before := n.dials.Load()
			ctx := t.Context()
			if strings.HasSuffix(tc.source, "/slow") {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
				defer cancel()
			}
			exit := Run(ctx, env)
			lines := strings.Split(strings.TrimSuffix(diag.String(), "\n"), "\n")
			if exit != 1 || out.Len() != 0 || len(lines) != 2 || diagnosticCode(t, lines[0]+"\n", "notice") != "destination" || diagnosticCode(t, lines[1]+"\n", "error") != tc.code || n.dials.Load()-before != tc.dials {
				t.Fatalf("exit %d, dials %d: %s", exit, n.dials.Load()-before, &diag)
			}
			for _, dir := range []string{"sources", ".pbrain", "personal"} {
				if _, err := os.Stat(filepath.Join(env.Dir, dir)); !os.IsNotExist(err) {
					t.Fatalf("%s: %v", dir, err)
				}
			}
		})
	}
}

func TestURLImportRuleChangesDuringFetch(t *testing.T) {
	env := writeEnv(t)
	n, network := importServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "short source")
	})
	env.Net = network
	n.onLookup = func() { audienceOK(t, env, "set", "article.test", "personal") }
	diag := importFailure(t, env, "conflict", "http://article.test/edit")
	if !strings.Contains(diag, "Writing to shared") {
		t.Fatal(diag)
	}
	mustBeMissing(t, filepath.Join(env.Dir, "sources"))
	mustBeMissing(t, filepath.Join(env.Dir, ".pbrain", "originals"))
	mustBeMissing(t, filepath.Join(env.Dir, "personal"))
}
