package fetch

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pkieltyka/pbrain/internal/errs"
	"golang.org/x/net/idna"
)

func failure(code, message string) error {
	return &errs.Error{Code: code, Message: message, Exit: 1}
}

func fetchFailed() error { return failure("fetch_failed", "could not fetch the requested resource") }

func (c *Client) endpoint(ctx context.Context, u *url.URL) (netip.AddrPort, error) {
	host := u.Hostname()
	if host == "" || u.User != nil || strings.HasSuffix(u.Host, ":") {
		return netip.AddrPort{}, fetchFailed()
	}
	port := uint16(80)
	if u.Scheme == "https" {
		port = 443
	}
	if p := u.Port(); p != "" {
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil || n == 0 {
			return netip.AddrPort{}, fetchFailed()
		}
		port = uint16(n)
	}
	var ips []netip.Addr
	if ip, err := netip.ParseAddr(host); err == nil {
		ips = []netip.Addr{ip}
	} else {
		if strings.ContainsAny(u.Host, "[]") {
			return netip.AddrPort{}, fetchFailed()
		}
		host, err = idna.Lookup.ToASCII(host)
		if err != nil {
			return netip.AddrPort{}, fetchFailed()
		}
		ips, err = c.net.Resolver.LookupNetIP(ctx, host)
		if err != nil || len(ips) == 0 {
			return netip.AddrPort{}, fetchFailed()
		}
	}
	for _, ip := range ips {
		if !public(ip) {
			return netip.AddrPort{}, failure("fetch_blocked", "destination is outside the public address space")
		}
	}
	return netip.AddrPortFrom(ips[0], port), nil
}

type requestPolicy struct {
	httpsOnly bool
	idle      time.Duration
	encoding  string
}

// open uses RoundTrip directly. Each redirect gets a new DNS check and a
// fresh transport whose only dial target is the numeric endpoint just checked.
func (c *Client) open(ctx context.Context, rawURL string, policy requestPolicy) (*http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fetchFailed()
	}
	for hop := 0; hop <= 5; hop++ {
		if ctx.Err() != nil || (u.Scheme != "https" && (policy.httpsOnly || u.Scheme != "http")) {
			return nil, fetchFailed()
		}
		lookupCtx, done := stageContext(ctx, policy.idle)
		target, err := c.endpoint(lookupCtx, u)
		done()
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, fetchFailed()
		}
		req.Header.Set("Accept-Encoding", policy.encoding)
		tr := &http.Transport{
			DisableKeepAlives: true, DisableCompression: true,
			TLSClientConfig:     &tls.Config{RootCAs: c.roots},
			TLSHandshakeTimeout: policy.idle, ResponseHeaderTimeout: policy.idle,
			DialContext: func(_ context.Context, network, _ string) (net.Conn, error) {
				// A transport may detach its dial context. Preserve the command's
				// deadline and cancellation even when no connection is ready yet.
				dialCtx, cancel := stageContext(ctx, policy.idle)
				defer cancel()
				conn, err := c.net.Dialer.DialContext(dialCtx, network, target)
				if err != nil || policy.idle == 0 {
					return conn, err
				}
				return idleConn{Conn: conn, idle: policy.idle}, nil
			},
		}
		resp, err := tr.RoundTrip(req)
		if err != nil {
			return nil, fetchFailed()
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Request = req
			return resp, nil
		}
		resp.Body.Close()
		switch resp.StatusCode {
		case 301, 302, 303, 307, 308:
			location := resp.Header.Get("Location")
			if hop == 5 || location == "" {
				return nil, fetchFailed()
			}
			u, err = u.Parse(location)
			if err != nil {
				return nil, fetchFailed()
			}
		default:
			return nil, fetchFailed()
		}
	}
	return nil, fetchFailed()
}

func stageContext(ctx context.Context, idle time.Duration) (context.Context, context.CancelFunc) {
	if idle > 0 {
		return context.WithTimeout(ctx, idle)
	}
	return context.WithCancel(ctx)
}

type idleConn struct {
	net.Conn
	idle time.Duration
}

func (c idleConn) Read(p []byte) (int, error) {
	if err := c.SetReadDeadline(time.Now().Add(c.idle)); err != nil {
		return 0, err
	}
	return c.Conn.Read(p)
}

func (c idleConn) Write(p []byte) (int, error) {
	if err := c.SetWriteDeadline(time.Now().Add(c.idle)); err != nil {
		return 0, err
	}
	return c.Conn.Write(p)
}
