package audience

import (
	"errors"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

var errKey = errors.New("invalid audience key")

// String returns a canonical spelling that can be stored as a scheme-less rule,
// except when HTTPS's default would erase a non-default HTTP port.
func (k Key) String() string {
	host := k.Host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if k.Port != "" {
		host += ":" + k.Port
	}
	u := url.URL{Host: host}
	key := strings.TrimPrefix(u.String(), "//")
	if k.Port == "443" {
		key = "http://" + key
	}
	if len(k.Segments) > 0 {
		key += "/" + strings.Join(k.Segments, "/")
	}
	return key
}

// Canon builds a comparison key. Stored items keep the supplied source locator.
func Canon(raw string, rule bool) (Key, error) {
	raw = strings.Trim(raw, " \t")
	if !utf8.ValidString(raw) {
		return Key{}, errKey
	}
	if rule && !strings.HasPrefix(strings.ToLower(raw), "http://") && !strings.HasPrefix(strings.ToLower(raw), "https://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Hostname() == "" || strings.HasSuffix(u.Host, ":") {
		return Key{}, errKey
	}
	if _, err := url.QueryUnescape(u.RawQuery); err != nil {
		return Key{}, errKey
	}
	host, err := canonicalHost(u)
	if err != nil {
		return Key{}, err
	}
	port := u.Port()
	if port != "" {
		n, err := strconv.ParseUint(port, 10, 16)
		if err != nil || n == 0 {
			return Key{}, errKey
		}
		port = strconv.FormatUint(n, 10)
		if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
			port = ""
		}
	}

	// EscapedPath can fall back to re-escaping the decoded Path when the input
	// mixes raw Unicode with escapes. Preserve the original segment boundaries.
	path := u.RawPath
	if path == "" {
		path = u.EscapedPath()
	}
	key := Key{Host: host, Port: port}
	if path == "" || path == "/" {
		return key, nil
	}
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if segments[len(segments)-1] == "" {
		segments = segments[:len(segments)-1]
	}
	for _, segment := range segments {
		value, err := canonicalSegment(segment)
		if err != nil {
			return Key{}, err
		}
		key.Segments = append(key.Segments, value)
	}
	return key, nil
}

func canonicalHost(u *url.URL) (string, error) {
	host := u.Hostname()
	if strings.ContainsFunc(host, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return "", errKey
	}
	if strings.HasPrefix(u.Host, "[") {
		addr, err := netip.ParseAddr(host)
		if err != nil || !addr.Is6() {
			return "", errKey
		}
		return addr.String(), nil
	}
	if strings.Contains(host, ":") {
		return "", errKey
	}
	ascii, err := idna.Lookup.ToASCII(host)
	host = strings.ToLower(strings.TrimSuffix(ascii, "."))
	if err != nil || len(host) > 253 {
		return "", errKey
	}
	for label := range strings.SplitSeq(host, ".") {
		if label == "" || len(label) > 63 {
			return "", errKey
		}
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.String(), nil
	}
	return host, nil
}

const hexDigits = "0123456789ABCDEF"

func unreserved(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || strings.IndexByte("-._~", b) >= 0
}

func canonicalSegment(raw string) (string, error) {
	if raw == "" {
		return "", errKey
	}
	var out strings.Builder
	for i := 0; i < len(raw); i++ {
		b := raw[i]
		escaped := b == '%'
		if escaped {
			if len(raw)-i < 3 {
				return "", errKey
			}
			value, err := url.PathUnescape(raw[i : i+3])
			if err != nil {
				return "", errKey
			}
			b = value[0]
			i += 2
		}
		if b == '/' || b == '\\' {
			return "", errKey
		}
		if unreserved(b) || (!escaped && strings.IndexByte("!$&'()*+,;=:@", b) >= 0) {
			out.WriteByte(b)
		} else {
			out.WriteByte('%')
			out.WriteByte(hexDigits[b>>4])
			out.WriteByte(hexDigits[b&15])
		}
	}
	value := out.String()
	if value == "." || value == ".." {
		return "", errKey
	}
	return value, nil
}
