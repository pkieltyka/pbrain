package fetch

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"mime"
	"strings"
	"time"
	"unicode/utf8"
)

const pageLimit = 5 << 20

func readPage(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, pageLimit+1))
	if len(b) > pageLimit {
		return nil, failure("too_large", "page exceeds the size limit")
	}
	if err != nil {
		return nil, fetchFailed()
	}
	return b, nil
}

// Get returns the decoded bytes and the final page URL for local conversion.
func (c *Client) Get(ctx context.Context, rawURL string) (*Page, error) {
	ctx, cancel := context.WithTimeout(ctx, c.pageTimeout)
	defer cancel()
	resp, err := c.open(ctx, rawURL, requestPolicy{encoding: "gzip, identity"})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	types := resp.Header.Values("Content-Type")
	if len(types) != 1 {
		return nil, failure("unsupported_type", "expected one UTF-8 text content type")
	}
	media, params, err := mime.ParseMediaType(types[0])
	charset, declared := params["charset"]
	if err != nil || (declared && !strings.EqualFold(charset, "utf-8")) || (media != "text/html" && media != "text/plain" && media != "text/markdown") {
		return nil, failure("unsupported_type", "expected UTF-8 HTML, Markdown or plain text")
	}
	data, err := readPage(resp.Body)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(strings.Join(resp.Header.Values("Content-Encoding"), ",")) {
	case "", "identity":
	case "gzip":
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fetchFailed()
		}
		defer gz.Close()
		data, err = readPage(gz)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fetchFailed()
	}
	if !utf8.Valid(data) {
		return nil, failure("unsupported_type", "expected valid UTF-8")
	}
	if ctx.Err() != nil {
		return nil, fetchFailed()
	}
	return &Page{FinalURL: resp.Request.URL, MediaType: media, Original: data}, nil
}

// Download streams identity-encoded bytes into the caller's temporary file.
// The caller verifies the pinned digest and exact size before publication.
func (c *Client) Download(ctx context.Context, rawURL string, dst io.Writer, maxBytes int64, idle time.Duration) (int64, error) {
	if maxBytes <= 0 || idle <= 0 || dst == nil {
		return 0, failure("invalid", "download requires a destination, positive limit and timeout")
	}
	resp, err := c.open(ctx, rawURL, requestPolicy{httpsOnly: true, idle: idle, encoding: "identity"})
	if err != nil {
		return 0, fetchFailed()
	}
	defer resp.Body.Close()
	encoding := strings.Join(resp.Header.Values("Content-Encoding"), ",")
	if resp.ContentLength > maxBytes || (encoding != "" && !strings.EqualFold(encoding, "identity")) {
		return 0, fetchFailed()
	}

	var total int64
	buf := make([]byte, 32<<10)
	for ctx.Err() == nil {
		remaining := maxBytes - total
		size := int64(len(buf))
		if remaining < size {
			size = remaining + 1
		} // read at most one byte past the limit
		n, readErr := resp.Body.Read(buf[:size])
		allowed := min(int64(n), remaining)
		if allowed > 0 {
			w, writeErr := dst.Write(buf[:allowed])
			total += int64(w)
			if writeErr != nil || int64(w) != allowed {
				return total, failure("io", "could not store downloaded data")
			}
		}
		if int64(n) > remaining || ctx.Err() != nil {
			return total, fetchFailed()
		}
		if readErr == io.EOF {
			return total, nil
		}
		if readErr != nil {
			return total, fetchFailed()
		}
	}
	return total, fetchFailed()
}
