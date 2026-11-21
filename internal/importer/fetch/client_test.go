package fetch

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/pkieltyka/pbrain/internal/errs"
)

type lookupFunc func(context.Context, string) ([]netip.Addr, error)

func (f lookupFunc) LookupNetIP(ctx context.Context, h string) ([]netip.Addr, error) {
	return f(ctx, h)
}

type dialFunc func(context.Context, string, netip.AddrPort) (net.Conn, error)

func (f dialFunc) DialContext(ctx context.Context, n string, a netip.AddrPort) (net.Conn, error) {
	return f(ctx, n, a)
}

type testNetwork struct {
	mu      sync.Mutex
	names   []string
	targets []netip.AddrPort
}

// The dialer records the production endpoint decision and connects only to
// this test's listener. DNS never leaves the test.
func testClient(t *testing.T, handler http.HandlerFunc) (*Client, *testNetwork) {
	t.Helper()
	s := httptest.NewTLSServer(handler)
	t.Cleanup(s.Close)
	log := new(testNetwork)
	ip := netip.MustParseAddr("93.184.216.34")
	c := New(Net{
		Resolver: lookupFunc(func(ctx context.Context, host string) ([]netip.Addr, error) {
			log.mu.Lock()
			defer log.mu.Unlock()
			log.names = append(log.names, host)
			switch host {
			case "mixed.example.com":
				return []netip.Addr{ip, netip.MustParseAddr("10.0.0.1")}, nil
			case "empty.example.com":
				return nil, nil
			case "missing.example.com":
				return nil, errors.New("unpublished resolver detail")
			}
			return []netip.Addr{ip}, nil
		}),
		Dialer: dialFunc(func(ctx context.Context, n string, a netip.AddrPort) (net.Conn, error) {
			log.mu.Lock()
			log.targets = append(log.targets, a)
			log.mu.Unlock()
			var d net.Dialer
			return d.DialContext(ctx, n, s.Listener.Addr().String())
		}),
	})
	c.roots = x509.NewCertPool()
	c.roots.AddCert(s.Certificate())
	return c, log
}
func code(t *testing.T, err error, want string) {
	t.Helper()
	var e *errs.Error
	if !errors.As(err, &e) || e.Code != want || e.Exit != 1 {
		t.Fatalf("want %s, got %#v", want, err)
	}
}

func TestPublicAddresses(t *testing.T) {
	denied := strings.Fields("0.1.2.3 10.3.2.1 100.127.0.1 127.2.3.4 169.254.0.1 172.31.0.1 192.0.0.9 192.0.2.1 192.31.196.1 192.52.193.1 192.88.99.1 192.168.1.1 192.175.48.1 198.19.1.1 198.51.100.1 203.0.113.1 224.1.1.1 255.255.255.255 :: ::1 ::ffff:127.0.0.1 64:ff9b::a00:1 ::ffff:0:a00:1 64:ff9b:1::abc fc00::1 fe80::1%test ff02::1 100::1 100:0:0:1::1 2001:2::1 2001:db8::1 2002::1 2620:4f:8000::1 3fff::1 5f00::1")
	for _, s := range denied {
		if public(netip.MustParseAddr(s)) {
			t.Errorf("%s: accepted", s)
		}
	}
	for _, s := range strings.Fields("8.8.8.8 1.1.1.1 2606:4700:4700::1111 ::ffff:8.8.8.8 64:ff9b::808:808 ::ffff:0:808:808") {
		if !public(netip.MustParseAddr(s)) {
			t.Errorf("%s: refused", s)
		}
	}
	if public(netip.Addr{}) {
		t.Fatal("accepted invalid address")
	}
}

func TestBoundedPublicFetch(t *testing.T) {
	ctx := context.Background()
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("NO_PROXY", "")
	c, log := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Host != r.TLS.ServerName || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Errorf("wrong origin or credentials: host=%s, SNI=%s", r.Host, r.TLS.ServerName)
		}
		switch r.URL.Path {
		case "/one":
			w.Header().Set("Set-Cookie", "probe=1")
			http.Redirect(w, r, "/two", 302)
		case "/two":
			http.Redirect(w, r, "https://other.example.com/end", 307)
		case "/end":
			w.Header().Set("Content-Type", "text/html; charset=UTF-8")
			fmt.Fprint(w, "<h1>résumé</h1>")
		case "/blocked":
			http.Redirect(w, r, "https://mixed.example.com/body", 302)
		case "/loop":
			http.Redirect(w, r, "/loop", 302)
		case "/scheme":
			http.Redirect(w, r, "ftp://other.example.com/file", 302)
		case "/auth":
			http.Redirect(w, r, "https://name:secret@other.example.com/file", 302)
		default:
			http.Error(w, "server detail", 404)
		}
	})
	p, err := c.Get(ctx, "https://start.example.com/one")
	if err != nil || p.MediaType != "text/html" || p.FinalURL.Host != "other.example.com" || string(p.Original) != "<h1>résumé</h1>" {
		t.Fatalf("page %#v: %v", p, err)
	}
	if !slices.Equal(log.names, []string{"start.example.com", "start.example.com", "other.example.com"}) || len(log.targets) != 3 {
		t.Fatalf("lookups %v targets %v", log.names, log.targets)
	}
	for _, ep := range log.targets {
		if ep.String() != "93.184.216.34:443" {
			t.Fatalf("unexpected dial %s", ep)
		}
	}
	for _, tc := range []struct {
		url, want string
		dials     int
	}{
		{"https://127.0.0.1/x", "fetch_blocked", 0}, {"https://mixed.example.com/x", "fetch_blocked", 0},
		{"https://empty.example.com", "fetch_failed", 0}, {"https://missing.example.com", "fetch_failed", 0},
		{"https://start.example.com/blocked", "fetch_blocked", 1}, {"https://start.example.com/loop", "fetch_failed", 6},
		{"https://start.example.com/scheme", "fetch_failed", 1}, {"https://start.example.com/auth", "fetch_failed", 1},
		{"https://start.example.com/status", "fetch_failed", 1}, {"https://wrong.test", "fetch_failed", 1},
	} {
		start := len(log.targets)
		p, err := c.Get(ctx, tc.url)
		code(t, err, tc.want)
		if p != nil || len(log.targets)-start != tc.dials {
			t.Errorf("%s got page or wrong dial count: %d", tc.url, len(log.targets)-start)
		}
	}
}

func TestPageDecoding(t *testing.T) {
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	gz.Write(bytes.Repeat([]byte("x"), pageLimit+1))
	gz.Close()
	var short bytes.Buffer
	gz = gzip.NewWriter(&short)
	gz.Write([]byte("small"))
	gz.Close()
	for _, tc := range []struct {
		name, typ, enc, want string
		body                 []byte
	}{
		{"markdown", "text/markdown", "identity", "", []byte("abc")}, {"gzip", "text/plain", "gzip", "", short.Bytes()},
		{"missing", "", "", "unsupported_type", []byte("x")}, {"alias", "text/x-markdown", "", "unsupported_type", nil},
		{"latin", "text/plain; charset=latin1", "", "unsupported_type", []byte("abc")}, {"bytes", "text/plain", "", "unsupported_type", []byte{255}},
		{"wire", "text/plain", "", "too_large", bytes.Repeat([]byte("w"), pageLimit+1)}, {"inflated", "text/plain", "gzip", "too_large", compressed.Bytes()},
		{"encoding", "text/plain", "br", "fetch_failed", nil}, {"invalid-gzip", "text/plain", "gzip", "fetch_failed", []byte("abc")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if tc.typ == "" {
					w.Header()["Content-Type"] = nil
				} else {
					w.Header().Set("Content-Type", tc.typ)
				}
				if tc.enc != "" {
					w.Header().Set("Content-Encoding", tc.enc)
				}
				w.WriteHeader(200)
				w.Write(tc.body)
			})
			p, err := c.Get(context.Background(), "https://example.com")
			if tc.want != "" {
				code(t, err, tc.want)
				return
			}
			want := tc.body
			if tc.enc == "gzip" {
				want = []byte("small")
			}
			if err != nil || !bytes.Equal(p.Original, want) {
				t.Fatalf("page %#v error %v", p, err)
			}
		})
	}
}

type shortSink struct{}

func (shortSink) Write(p []byte) (int, error) { return 1, io.ErrShortWrite }

func TestPlainHTTPNumericEndpoint(t *testing.T) {
	const origin = "8.8.8.8:8443"
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != origin || r.URL.Path != "/notes" {
			t.Errorf("wrong authority or path: %s %s", r.Host, r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/markdown")
		fmt.Fprint(w, "# Note")
	}))
	defer s.Close()
	c := New(Net{
		Resolver: lookupFunc(func(context.Context, string) ([]netip.Addr, error) {
			t.Error("resolver called for a literal address")
			return nil, errors.New("unexpected lookup")
		}),
		Dialer: dialFunc(func(ctx context.Context, network string, ep netip.AddrPort) (net.Conn, error) {
			if ep.String() != origin {
				return nil, errors.New("unexpected endpoint")
			}
			var d net.Dialer
			return d.DialContext(ctx, network, s.Listener.Addr().String())
		}),
	})
	page, err := c.Get(context.Background(), "http://"+origin+"/notes")
	if err != nil || string(page.Original) != "# Note" {
		t.Fatalf("HTTP page %#v, %v", page, err)
	}
}
