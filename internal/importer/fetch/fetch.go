// Package fetch downloads a public page or model file over checked, direct connections.
package fetch

import (
	"context"
	"crypto/x509"
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
type Client struct {
	net         Net
	pageTimeout time.Duration
	roots       *x509.CertPool // nil uses the system trust store
}

func New(n Net) *Client {
	if n.Resolver == nil {
		n.Resolver = systemResolver{}
	}
	if n.Dialer == nil {
		n.Dialer = systemDialer{}
	}
	return &Client{net: n, pageTimeout: 30 * time.Second}
}

type systemResolver struct{}

func (systemResolver) LookupNetIP(ctx context.Context, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

type systemDialer struct{ net.Dialer }

func (d systemDialer) DialContext(ctx context.Context, network string, addr netip.AddrPort) (net.Conn, error) {
	return d.Dialer.DialContext(ctx, network, addr.String())
}
