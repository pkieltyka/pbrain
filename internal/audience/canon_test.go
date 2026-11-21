package audience

import (
	"reflect"
	"testing"
)

func TestCanonicalRuleSourceParity(t *testing.T) {
	valid := []struct{ input, want string }{
		{"HTTPS://Docs.Google.Com.:00443/docum%65nt/d/X/?q=1#heading", "docs.google.com/document/d/X"},
		{"http://host:80", "host"},
		{"https://host:80", "host:80"},
		{"http://host:443/x", "http://host:443/x"},
		{"http://[2001:0db8::1]:0443/x", "http://[2001:db8::1]:443/x"},
		{"https://[::1]:8443/x", "[::1]:8443/x"},
		{"https://192.0.2.1:8443/", "192.0.2.1:8443"},
		{"https://Bücher.example/café/%7e", "xn--bcher-kva.example/caf%C3%A9/~"},
		{"https://bücher.example/caf%c3%a9/%3a", "xn--bcher-kva.example/caf%C3%A9/%3A"},
		{"https://x/a%23b?path=/a#more", "x/a%23b"},
		{"https://x/a%20b", "x/a%20b"},
		{"https://x/a%252Fb", "x/a%252Fb"},
	}
	for _, tc := range valid {
		k, err := Canon(tc.input, false)
		if err != nil || k.String() != tc.want {
			t.Errorf("Canon(%q) = %q, %v; want %q", tc.input, k.String(), err, tc.want)
			continue
		}
		round, err := Canon(k.String(), true)
		if err != nil || !reflect.DeepEqual(round, k) {
			t.Errorf("stored %q did not round-trip: %#v", k.String(), round)
		}
	}
	for _, input := range []string{"ftp://host/x", "https:foo", "https://", "https://a:b@host/x", "https://host:/x", "https://host:65536", "https://host:0", "https://host:abc", "https://host:1:2", "https://host//", "https://host/a//b", "https://host/%2E./x", "https://host/a%5Cb", "https://host/a\\b", "https://host/café%2fb", "https://host/%xz", "https://not_a_host/path", "https://host..", "https://[not-an-ip]/", "https://host?foo=%ZZ"} {
		if _, err := Canon(input, false); err == nil {
			t.Errorf("accepted invalid source %q", input)
		}
	}
}

func FuzzCanonicalRuleRoundTrip(f *testing.F) {
	for _, s := range []string{"http://[::1]:443", "docs.google.com/document/d/x", "https://host/a%3Ab", "例え.テスト/é", "https://[fe80::1%25eth0]/path", "https://site/a%2fb", "invalid:", "a/b//c", "https://x/!!!"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		k, err := Canon(s, true)
		if err != nil {
			return
		}
		k2, err := Canon(k.String(), true)
		if err != nil || !reflect.DeepEqual(k, k2) {
			t.Fatalf("key %#v round-trips as %#v", k, k2)
		}
	})
}
