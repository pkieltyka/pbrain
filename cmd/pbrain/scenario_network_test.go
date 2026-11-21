package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"
)

// A fixture connection is an in-memory HTTP stream. There is no fallback
// to system DNS, a proxy, or a socket, including for model downloads.
type scriptNetwork struct {
	enabled    bool
	unexpected atomic.Bool
}

func newScriptNetwork(mode string) *scriptNetwork { return &scriptNetwork{enabled: mode == "fixture"} }

var scriptIP = netip.MustParseAddr("8.8.8.8")

func (n *scriptNetwork) LookupNetIP(ctx context.Context, host string) ([]netip.Addr, error) {
	if n.enabled {
		switch host {
		case "pages.test":
			return []netip.Addr{scriptIP}, nil
		case "mixed.test":
			return []netip.Addr{scriptIP, netip.MustParseAddr("127.0.0.1")}, nil
		}
	}
	n.unexpected.Store(true)
	return nil, errors.New("unexpected fixture hostname")
}
func (n *scriptNetwork) DialContext(ctx context.Context, network string, addr netip.AddrPort) (net.Conn, error) {
	if !n.enabled || addr != netip.AddrPortFrom(scriptIP, 80) {
		n.unexpected.Store(true)
		return nil, errors.New("unexpected fixture dial")
	}
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		req, err := http.ReadRequest(bufio.NewReader(server))
		if err != nil {
			return
		}
		defer req.Body.Close()
		resp := http.Response{StatusCode: 200, ProtoMajor: 1, ProtoMinor: 1, Header: make(http.Header), Close: true}
		var body []byte
		switch req.URL.Path {
		case "/markdown":
			resp.Header.Set("Content-Type", "text/markdown")
			body = []byte("# Fixture markdown\nA stable answer.\n")
		case "/plain":
			resp.Header.Set("Content-Type", "text/plain")
			body = []byte("Fixture plain text.\n")
		case "/html":
			resp.Header.Set("Content-Type", "text/html")
			body = []byte("<html><head><title>Fixture guide</title></head><body><article><h1>Fixture guide</h1><p>" + strings.Repeat("A factual passage about a fixture. ", 18) + "</p></article></body></html>")
		case "/short", "/shell":
			resp.Header.Set("Content-Type", "text/html")
			body = []byte("<html><body>Short.</body></html>")
			if req.URL.Path == "/shell" {
				body = []byte("<html><body><script>app()</script></body></html>")
			}
		case "/redirect":
			resp.StatusCode = 302
			resp.Header.Set("Location", "http://127.0.0.1/")
		case "/error":
			resp.StatusCode = 503
		case "/type":
			resp.Header.Set("Content-Type", "image/png")
		case "/large":
			resp.Header.Set("Content-Type", "text/plain")
			resp.Header.Set("Content-Encoding", "gzip")
			var buf bytes.Buffer
			z := gzip.NewWriter(&buf)
			block := bytes.Repeat([]byte("x"), 65536)
			for range 81 {
				_, _ = z.Write(block)
			}
			_ = z.Close()
			body = buf.Bytes()
		case "/slow":
			// The test command has a short context deadline; send no response.
			_, _ = io.Copy(io.Discard, server)
			return
		default:
			n.unexpected.Store(true)
			return
		}
		resp.Body = io.NopCloser(bytes.NewReader(body))
		resp.ContentLength = int64(len(body))
		_ = resp.Write(server)
	}()
	return client, nil
}
