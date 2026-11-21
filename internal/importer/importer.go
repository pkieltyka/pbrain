// Package importer turns an outside source into the content of one new source item.
package importer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/importer/fetch"
	"github.com/pkieltyka/pbrain/internal/item"
)

type Prepared struct {
	Origin              item.Origin
	Title, Source, Body string
	Original            []byte // exact converter input for file/url, including a valid zero-byte input
	SHA256              string // lowercase hex for file/url; empty for linked
}

func problem(code, message string) error {
	return &errs.Error{Code: code, Message: message, Exit: 1}
}

// FromFile never opens name. The caller supplies bytes from the guarded input handle.
func FromFile(name, source, title string, raw []byte) (Prepared, error) {
	name = filepath.Base(name)
	if source == "" {
		source = name
	}
	var media string
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown":
		media = "text/markdown"
	case ".txt":
		media = "text/plain"
	case ".htm", ".html":
		media = "text/html"
	default:
		return Prepared{}, problem("unsupported_type", "expected a Markdown, text or HTML file")
	}
	base, _ := webURL(source)
	return copied(item.OriginFile, source, title, name, media, base, raw)
}

func FromPage(requestedURL, title string, p *fetch.Page) (Prepared, error) { panic("not implemented") }

// webURL checks form only. The fetch client owns network policy.
func webURL(raw string) (*url.URL, bool) {
	if strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return nil, false
	}
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") || u.Opaque != "" || u.Hostname() == "" || u.User != nil {
		return nil, false
	}
	return u, true
}

func singleLine(s string, limit int) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= limit && !strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsControl(r) || r == '\u2028' || r == '\u2029'
	})
}

func titleFor(explicit, inferred string) (string, error) {
	if explicit != "" {
		if !singleLine(explicit, 120) || strings.TrimSpace(explicit) == "" {
			return "", problem("invalid", "title: expected one nonempty line of at most 120 characters")
		}
		return explicit, nil
	}
	s := strings.Join(strings.Fields(inferred), " ")
	if utf8.RuneCountInString(s) > 120 {
		s = string([]rune(s)[:119]) + "…"
	}
	if s == "" || !singleLine(s, 120) {
		return "", problem("invalid", "could not infer a valid title; use --title")
	}
	return s, nil
}

// Link keeps the requested URL and the supplied summary verbatim.
func Link(rawURL, title string, summary *string) (Prepared, error) {
	if _, ok := webURL(rawURL); !ok || !singleLine(rawURL, 2000) {
		return Prepared{}, problem("invalid", "link: expected an HTTP or HTTPS URL of at most 2,000 characters without credentials")
	}
	name, err := titleFor(title, rawURL)
	if err != nil {
		return Prepared{}, err
	}
	p := Prepared{Origin: item.OriginLink, Source: rawURL, Title: name}
	if summary != nil {
		if *summary == "" || !singleLine(*summary, 300) {
			return Prepared{}, problem("invalid", "summary: expected one line of 1 to 300 characters")
		}
		p.Body = *summary
	}
	return p, nil
}

func copied(origin item.Origin, source, title, fallback, media string, base *url.URL, raw []byte) (Prepared, error) {
	if !singleLine(source, 2000) {
		return Prepared{}, problem("invalid", "source: expected one line of at most 2,000 characters")
	}
	if len(raw) > 5<<20 {
		return Prepared{}, problem("too_large", "the original exceeds 5 MiB")
	}
	if !utf8.Valid(raw) {
		return Prepared{}, problem("unsupported_type", "the source is not UTF-8")
	}
	body := bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	var pageTitle string
	switch media {
	case "text/plain", "text/markdown":
	case "text/html":
		var err error
		body, pageTitle, err = articleMarkdown(body, base)
		if err != nil {
			return Prepared{}, err
		}
	default:
		return Prepared{}, problem("unsupported_type", "expected Markdown, text or HTML")
	}
	if _, _, start, ok, err := item.SplitFrontMatter(body); ok && err == nil {
		body = body[start:]
	}
	body = bytes.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\r' && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, body)
	if utf8.RuneCount(body) > 512000 {
		return Prepared{}, problem("too_large", "the imported body exceeds 512,000 characters")
	}
	if title == "" {
		if heading := item.FirstHeading(body); heading != "" {
			fallback = heading
		} else if strings.TrimSpace(pageTitle) != "" {
			fallback = pageTitle
		}
	}
	name, err := titleFor(title, fallback)
	if err != nil {
		return Prepared{}, err
	}
	sum := sha256.Sum256(raw)
	return Prepared{Origin: origin, Title: name, Source: source, Body: string(body), Original: bytes.Clone(raw), SHA256: hex.EncodeToString(sum[:])}, nil
}
