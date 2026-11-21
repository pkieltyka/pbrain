package importer

import (
	"bytes"
	"net/url"
	"strings"
	"unicode/utf8"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func articleMarkdown(raw []byte, baseURL *url.URL) ([]byte, string, error) {
	doc, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return nil, "", problem("no_content", "could not read the HTML document")
	}
	removeEmbeds(doc)
	var title string
	for n := range doc.Descendants() {
		if n.Type == html.ElementNode && n.DataAtom == atom.Title && n.FirstChild != nil {
			title = n.FirstChild.Data
			break
		}
	}
	article, err := readability.FromDocument(doc, baseURL)
	var text strings.Builder
	if err != nil || article.Node == nil || article.RenderText(&text) != nil || utf8.RuneCountInString(strings.TrimSpace(text.String())) < 200 {
		return nil, "", problem("no_content", "the extracted article has fewer than 200 characters; try --link-only")
	}
	conv := converter.NewConverter(converter.WithPlugins(base.NewBasePlugin(), commonmark.NewCommonmarkPlugin(), table.NewTablePlugin()))
	body, err := conv.ConvertNode(article.Node)
	if err != nil {
		return nil, "", problem("no_content", "could not convert the HTML article to Markdown")
	}
	return body, title, nil
}

func removeEmbeds(parent *html.Node) {
	for n := parent.FirstChild; n != nil; {
		next := n.NextSibling
		switch n.DataAtom {
		case atom.Script, atom.Style, atom.Frame, atom.Iframe, atom.Object, atom.Embed, atom.Img, atom.Picture, atom.Audio, atom.Video, atom.Source, atom.Track, atom.Canvas, atom.Svg, atom.Noscript, atom.Template:
			parent.RemoveChild(n)
		default:
			removeEmbeds(n)
		}
		n = next
	}
}
