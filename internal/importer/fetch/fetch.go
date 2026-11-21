// Package fetch: exported surface from the pbrain specs; bodies are filled in by later phases.
package fetch

import (
	"context"
	"io"
	"net"
	"net/netip"
	"net/url"
	"time"
)

type Resolver interface {
	LookupNetIP(ctx context.Context, host string) ([]netip.Addr, error)
}
type Dialer interface {
	DialContext(ctx context.Context, network string, addr netip.AddrPort) (net.Conn, error)
}
type Net struct {
	Resolver Resolver
	Dialer   Dialer
}
type Page struct {
	FinalURL  *url.URL
	MediaType string
	Original  []byte
}
type Client struct { /* immutable dependencies */
}

func New(net Net) *Client                                               { panic("not implemented") }
func (c *Client) Get(ctx context.Context, rawURL string) (*Page, error) { panic("not implemented") }
func (c *Client) Download(ctx context.Context, rawURL string, dst io.Writer, maxBytes int64, idle time.Duration) (written int64, err error) {
	panic("not implemented")
}
