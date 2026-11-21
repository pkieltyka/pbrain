---
status: complete
---

# Importing outside sources

`internal/importer` prepares the content and provenance for one new `source` item. `internal/importer/fetch` retrieves one public resource. The CLI resolves the destination and prints the destination notice **before** acquisition, reads a local file through `store.OpenInput`, and runs the add/publish transaction. Importer never edits a previous item, invokes a model or drives a connector. Model downloads belong to `pbrain sync`.

## Interfaces

```go
package importer

type Prepared struct {
    Origin item.Origin
    Title, Source, Body string
    Original []byte // exact converter input for file/url, including a valid zero-byte input
    SHA256 string   // lowercase hex for file/url; empty for linked
}
func FromFile(name, source, title string, raw []byte) (Prepared, error)
func FromPage(requestedURL, title string, p *fetch.Page) (Prepared, error)
func Link(rawURL, title string, summary *string) (Prepared, error)
```

`name` is the user-supplied file basename, with extension. `source` and `title` are flag values or empty. `Link` distinguishes absent summary from an explicitly empty one. `Prepared` owns immutable original bytes after return. Check `Origin` when deciding whether to store an original. CLI supplies one invocation instant for `fetched` on copied imports. The `source` field on a public fetch is the URL as requested; only conversion uses the final URL. The event, author, tags, links and ids are assigned by the write path. No imported front matter supplies those values.

```go
package fetch

type Resolver interface {
    LookupNetIP(ctx context.Context, host string) ([]netip.Addr, error)
}
type Dialer interface {
    DialContext(ctx context.Context, network string, addr netip.AddrPort) (net.Conn, error)
}
type Net struct {
    Resolver Resolver
    Dialer Dialer
}
type Page struct {
    FinalURL *url.URL
    MediaType string
    Original []byte
}
type Client struct { /* immutable dependencies */ }
func New(net Net) *Client
func (c *Client) Get(ctx context.Context, rawURL string) (*Page, error)
func (c *Client) Download(ctx context.Context, rawURL string, dst io.Writer, maxBytes int64, idle time.Duration) (written int64, err error)
```

`cli.Env.Net` is the injected dependency. The production resolver adapts `net.DefaultResolver.LookupNetIP(ctx, "ip", host)`; the production dialer uses the supplied numeric address with `net.Dialer`. No shared type changes. `Prepared`, `Page` and their constructors are internal Go APIs; item Markdown and CLI output follow the functional spec.

## Acquisition

| Form | Source and validation | Original |
|---|---|---|
| File | `--source` when supplied, else basename. Case-insensitive extension `.md .markdown .txt .htm .html`. At most 5 MiB and valid UTF-8. | File bytes before any cleaning. |
| Public URL | Absolute HTTP(S), host present and no credentials. Handle every redirect through the guarded client. Accept the final 2xx response only. | HTTP entity after content decoding, before UTF-8 cleaning/conversion. |
| `--link-only <url>` | Use `url.Parse`; require absolute hierarchical HTTP(S) (scheme compared without case), a nonempty hostname, no user information, whitespace or controls, and at most 2,000 code points. Preserve the full supplied locator. An otherwise well-formed path with dot segments can be linked with an explicit audience; its missing canonical audience key still requires a flag. Never resolve DNS, fetch, convert or derive a summary. Summary, if given, is 1–300 Unicode code points with no line break or control character; store as the body. | None; no digest or fetch time. |

Check `--source` only for file and `--summary` only for linked imports. Wrong flag combinations give `usage`, bad flag values give `invalid`. Cap the source at 2,000 code points. Validate explicit title under the 120-character item rule. When an inferred title is longer, shorten it on a code-point boundary to fit the field; the complete locator remains in `Source`. Select `item.FirstHeading` from the cleaned body (heading only, no prose fallback), then page title for HTML, then basename or requested URL. Collapse whitespace in inferred titles to one space and cap at 120 code points including a terminal ellipsis when cut. Resolve artifact `#<path>` labels only for audience matching: retain the full supplied URL on the item.

`store.OpenInput(b,dest,path)` checks the privacy guard and opened descriptor identity first. Only then check that descriptor with a 5 MiB bounded read plus one sentinel byte. A refused path gives `not_found` even if its target exceeds 5 MiB. In-memory `FromFile` independently checks the supplied length. Map an unsupported extension, media type or non-UTF-8 text to `unsupported_type`; a raw or cleaned body exceeding its limit gives `too_large`. A missing or refused local path gives `not_found` via the guarded opener. Empty text/Markdown and a short valid text are allowed. Compute SHA-256 over original bytes. Remove an initial UTF-8 BOM from cleaned text, retaining it in the original. Reuse `item.SplitFrontMatter` on the text to find complete opening and closing delimiters (including permitted trailing spaces). With no complete block keep the text. Imported front matter is discarded as bytes without ever invoking YAML field parsing. Handle LF and CRLF, preserve ordinary body text, headings and footnotes, remove control characters except line breaks and tabs. Keep the cleaned body iff it is at most 512,000 code points; never truncate a copied body to make it fit.

## HTML and the fixture result

1. Validate UTF-8 before parsing. Parse from already supplied bytes using `x/net/html.Parse`; remove scripts, style, frame/iframe, object/embed and embedded media elements, including img (omit the element and its resource URL). Keep anchor text/links and structure. No resource loader.
2. Use `readability.FromDocument(doc, baseURL)`, never its network convenience function. For a fetch use the response's final URL. For local HTML use a valid URL citation when present, else nil (relative citations stay relative). Take the page title from the document before extraction for fallback. No extraction result or fewer than 200 Unicode code points of trimmed extracted plain text (`Article.RenderText`) gives `no_content`.
3. Convert `Article.Node` via `converter.NewConverter(converter.WithPlugins(base.NewBasePlugin(), commonmark.NewCommonmarkPlugin(), table.NewTablePlugin()))` and `ConvertNode`. Apply front matter/control cleanup and the body limit to the output. Use a fresh converter per acquisition.

On Linux/amd64 with Go 1.27.3 and cgo disabled, an isolated probe used `go-readability/v2 v2.1.3`, `html-to-markdown/v2 v2.5.2` and `x/net v0.63.0`. A static fixture containing six sentences, an H2, relative link, list, fenced-code input and two-column table yielded 529 extracted characters. The Markdown retained heading, list, code and table. The link `../manual` under `https://example.org/ops/start` became `https://example.org/manual`; navigation and script text were absent. A script shell produced eight characters (“Loading…”), and a real short article produced 23. Both fall below the floor. These synthetic checks justify the library choices; they do not establish extraction quality for every website.

## Fetch contract

Use one total deadline of 30 seconds, respecting any earlier caller deadline. At each of at most six request attempts (initial plus five redirects), validate the current URL; resolve once, validate **every** returned address against the checked-in public-address policy in architecture §11.2, and pin one approved endpoint to the connection. Literal IPs receive the same check. Classify mapped/translated addresses by the underlying address. Treat unapproved destinations as `fetch_blocked` before dialing. DNS failures or no answers give `fetch_failed`. Validate and bind at the request boundary even on a redirect to the same name. Do not connect to a proxy, inherit environment proxy settings, use cookies, forward credentials or follow redirects automatically. Use the current hop URL’s hostname for TLS verification and its authority for the Host header. On a redirect to a different host both change before the next request. Close each response and per-hop transport after use.

Request gzip or identity explicitly with automatic transport decoding disabled. Read at most 5 MiB + 1 of the success response body; independently bound gzip decoding to that same maximum. Overflow in either stage gives `too_large`. An unsupported content encoding or malformed compressed stream fails with `fetch_failed`. Parse Content-Type using `mime.ParseMediaType`; admit `text/markdown`, `text/plain`, `text/html` only and require valid UTF-8. A missing type, `text/x-markdown`, or a declared charset other than UTF-8 (case-insensitive) gives `unsupported_type`, including ASCII bytes declared as Latin-1. Ignore HTML meta charset for decoding: there is no transcoding. Resolve relative Location values against the current URL. Timeout, unhandled HTTP status, invalid redirect or exhaustion gives `fetch_failed`. Messages use safe error codes and no credentials, response bodies or private inputs. Fetch downloads only the requested page; HTML processing makes zero requests.

## Publication and duplicates

Prepare outside the section lock. Inside it the write path recomputes audience and duplicate state, validates the output directory and links, and enforces `Disclose` on notices/errors. Current `origin:file` duplicates in the destination use SHA-256 regardless of filename. Current URL/link items compare the complete stored source with the newly requested URL, byte for byte, including scheme spelling, query and fragment. Audience keys are unsuitable here because they drop locator components; a file citing a URL is still a file. All conflicting ids must be named by `--replaces` to lift duplicate refusal; otherwise `duplicate`. New imports always get a new id. Store and verify a copied original in the destination section before publication. Linked items skip this step, even with a summary. The normal shared-field scan warns on a suspected secret; its value never appears in a notice. Failures before publication expose no new item; after publication follow the common `after_write` contract.

## Streaming a pinned model file

`embed` calls `Download` only from an explicit index build, passing the release-table file's HTTPS URL, pinned size as the byte ceiling, a positive idle timeout and a temporary-file writer. The caller supplies its deadline or cancellation, validates exact size and SHA-256 over the stored bytes, and owns temporary-file cleanup and atomic publication. A successful transfer alone cannot publish a model. Import's deadline, type checks, decompression and body-in-memory limit remain in `Get`. Both routes reuse one guarded per-hop request implementation.

Require HTTPS at every hop, no user info, no proxy or credentials, independent address checks and current-host certificate validation on each connection. At most five redirects. Request identity encoding, reject another encoding, and accept only a 2xx response; Content-Type is immaterial. Apply idle to DNS/connect, TLS, response headers and a stalled body, resetting the body-read deadline after progress. Respect cancellation throughout. There is no 30-second total cap on this method. Check declared length when present, then stream in bounded chunks with a one-byte overflow probe; never write beyond maxBytes or buffer the complete payload. Return the count actually written. Network/policy/status/overflow/timeouts yield a fixed `fetch_failed` model-download error, destination write failure yields `io`. A bad internal limit or timeout is `invalid`. Diagnostics exclude response bodies and full URLs.

`TestPinnedModelDownload` uses a slow, progressing multi-chunk response longer than an article's total timeout, an idle stall, HTTPS downgrade, cross-host redirect, oversized data with no Content-Length, cancellation and failing sink. Assert digest mismatch prevents publication at the embed caller. Scenarios 35 and 37.

## Tests

| Test | Expected outcome |
|---|---|
| `TestImportAsData` (5, 22, 26) | Raw digest survives HTML/Markdown cleanup; forged leading fields do not change the item; Markdown footnotes survive. Local imports never use the network. |
| `TestHTMLArticleAndShell` (20, 22) | Ship the static article, script shell and short article under `internal/importer/testdata` with expected Markdown or error. Use structural assertions and zero auxiliary requests. Verify final-URL and local-URL link resolution, 199/200 threshold, content anchors and the script-shell refusal. |
| `TestLinkedSource` (19, 28) | No original, clock stamp, connection or digest. Empty summary flag, 301 runes and control characters fail. Absence succeeds. A 2,000-code-point locator succeeds, 2,001 fails with `invalid`. The whole locator is kept and the stored summary is searchable and scanned. |
| `TestDuplicatesAndAtomicOriginal` (14–15, 18–19, 29) | Same bytes/different name; different bytes/same name; file citation untouched by import of the URL; differently cased source and distinct query/fragment stay distinct; linked-to-copied replacement; multiple duplicate ids; original write failure before publish; shared original cleanup on delete. |
| `TestBoundedPublicFetch` (20) | Fake controlled DNS and transport verify one resolution per hop, no unapproved dial or proxy request, new Host and TLS certificate name after cross-host redirect, sixth redirect, FTP/credentialed redirect and missing Content-Type refused, deadline across redirects, separate encoded/decoded limits, 2xx/type/charset/UTF-8 checks. No public network in CI. |
| `TestImportAudiencePreflight` (6, 24–26) | No acquisition before audience/notice and path guard; private-rule duplicate output stays confined. |
