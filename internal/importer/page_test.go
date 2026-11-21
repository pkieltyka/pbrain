package importer

import (
	"net/url"
	"testing"

	"github.com/pkieltyka/pbrain/internal/importer/fetch"
	"github.com/pkieltyka/pbrain/internal/item"
)

func TestFromPage(t *testing.T) {
	const source = "https://example.org/request?k=v#part"
	final, err := url.Parse("https://example.org/elsewhere/")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ media, text, title, body string }{
		{"text/markdown", "---\nstatus: verified\n---\n## Guide\ntext", "Guide", "## Guide\ntext"},
		{"text/plain", "brief", source, "brief"}, {"text/plain", "", source, ""},
	} {
		p, err := FromPage(source, "", &fetch.Page{FinalURL: final, MediaType: tc.media, Original: []byte(tc.text)})
		if err != nil || p.Origin != item.OriginURL || p.Source != source || p.Title != tc.title || p.Body != tc.body || string(p.Original) != tc.text {
			t.Fatalf("%s: %+v %v", tc.media, p, err)
		}
	}
	_, err = FromPage(source, "", nil)
	wantCode(t, err, "fetch_failed")
	_, err = FromPage(source, "", &fetch.Page{})
	wantCode(t, err, "fetch_failed")
}
