package fetch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"testing"
	"time"
)

func TestPinnedModelDownload(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Encoding") != "identity" || r.TLS.ServerName != r.Host {
			t.Error("wrong request encoding or TLS origin")
		}
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "https://second.example.com/progress", 302)
		case "/progress":
			w.Header().Set("Content-Type", "application/octet-stream")
			for i := 0; i < 4; i++ {
				fmt.Fprint(w, "ab")
				w.(http.Flusher).Flush()
				time.Sleep(25 * time.Millisecond)
			}
		case "/downgrade":
			http.Redirect(w, r, "http://example.com/plain", 302)
		case "/private":
			http.Redirect(w, r, "https://[::1]/secret", 302)
		case "/length":
			w.Header().Set("Content-Length", "9")
			fmt.Fprint(w, "123456789")
		case "/chunked":
			w.(http.Flusher).Flush()
			fmt.Fprint(w, "123456789")
		case "/encoded":
			w.Header().Set("Content-Encoding", "gzip")
			fmt.Fprint(w, "abc")
		case "/stall":
			fmt.Fprint(w, "ab")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		case "/headers":
			<-r.Context().Done()
		default:
			fmt.Fprint(w, "abcd")
		}
	})
	c.pageTimeout = 10 * time.Millisecond
	ctx := context.Background()
	var b bytes.Buffer
	n, err := c.Download(ctx, "https://example.com/start", &b, 8, time.Second)
	if err != nil || n != 8 || b.String() != "abababab" {
		t.Fatalf("progressing download: %d %q %v", n, b.String(), err)
	}
	for _, tc := range []struct {
		path    string
		written int64
	}{{"/downgrade", 0}, {"/private", 0}, {"/length", 0}, {"/chunked", 8}, {"/encoded", 0}, {"/stall", 2}, {"/headers", 0}} {
		t.Run(tc.path, func(t *testing.T) {
			b.Reset()
			n, err := c.Download(ctx, "https://example.com"+tc.path, &b, 8, 300*time.Millisecond)
			code(t, err, "fetch_failed")
			if n != tc.written || n != int64(b.Len()) {
				t.Fatalf("written %d to %d-byte sink; want %d", n, b.Len(), tc.written)
			}
		})
	}
	n, err = c.Download(ctx, "https://example.com/plain", shortSink{}, 8, time.Second)
	code(t, err, "io")
	if n != 1 {
		t.Fatalf("failed sink got %d bytes; want 1", n)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	n, err = c.Download(cancelled, "https://example.com/plain", io.Discard, 8, time.Second)
	code(t, err, "fetch_failed")
	if n != 0 {
		t.Fatal("wrote after cancellation")
	}
	for _, tc := range []struct {
		n int64
		d time.Duration
	}{{-1, time.Second}, {8, 0}} {
		_, err := c.Download(ctx, "https://example.com", io.Discard, tc.n, tc.d)
		code(t, err, "invalid")
	}
}

func TestTotalDeadlineAcrossHops(t *testing.T) {
	var deadlines []time.Time
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/wait", 302)
			return
		}
		<-r.Context().Done()
	})
	c.net.Resolver = lookupFunc(func(ctx context.Context, _ string) ([]netip.Addr, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("lookup without article deadline")
		}
		deadlines = append(deadlines, deadline)
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	})
	c.pageTimeout = 350 * time.Millisecond
	_, err := c.Get(context.Background(), "https://example.com/")
	code(t, err, "fetch_failed")
	if len(deadlines) != 2 || !deadlines[0].Equal(deadlines[1]) {
		t.Fatalf("per-hop deadlines: %v", deadlines)
	}
}

func TestModelStartupIdle(t *testing.T) {
	for _, stage := range []string{"lookup", "dial", "tls"} {
		t.Run(stage, func(t *testing.T) {
			c := New(Net{Resolver: lookupFunc(func(ctx context.Context, _ string) ([]netip.Addr, error) {
				if stage == "lookup" {
					<-ctx.Done()
					return nil, ctx.Err()
				}
				return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
			}), Dialer: dialFunc(func(ctx context.Context, _ string, _ netip.AddrPort) (net.Conn, error) {
				if stage == "dial" {
					<-ctx.Done()
					return nil, ctx.Err()
				}
				conn, peer := net.Pipe()
				t.Cleanup(func() { peer.Close() })
				return conn, nil
			})})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, err := c.Download(ctx, "https://example.com", io.Discard, 128, 80*time.Millisecond)
			code(t, err, "fetch_failed")
			if ctx.Err() != nil {
				t.Fatal("only parent deadline interrupted stalled " + stage)
			}
		})
	}
}
