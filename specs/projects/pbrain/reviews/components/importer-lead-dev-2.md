# Review: components/importer.md (@lead-dev-2)

Reviewed: `components/importer.md` at sha256 d872fc402dcb2b3c76158c1cfc137702b2e56ef0013b776ca3a6380b3475fe20, against functional spec 8.4, 8.5, 2.5 and scenarios 5, 18 to 20, 22 and 28 (sha256 4622a82f), and architecture 10.1, 10.2, 10.7 and 11 (sha256 8d40545f).

**Verdict: changes required.**
- Copied and linked publication, the original's bytes, the fetch loop with one check per hop and pinned addresses, the two size limits and the extraction floor all match the spec.
- The fixture probe is a useful first run.
- Four points need a decision before building. One of them, the TLS host name on a redirect, is a likely defect as worded.

## Moderate

1. **The TLS name and `Host` header on a redirect.** "Keep the HTTP Host and TLS verification hostname from the original hop URL" reads as the first request's URL. On a redirect to another host (`a.example` → `b.example`), pbrain would then:
   - send `Host: a.example` to b's address;
   - verify b's certificate against `a.example`.
   
   Either the fetch fails, or it is answered for the wrong host name. Architecture 11.2 says "TLS verifies the certificate for the URL's host name", meaning each hop's own URL. Reword it as "the current hop's URL host", and add a test: a cross-host redirect presents a certificate for the new host, and the request carries the new `Host`.

2. **How the duplicate check compares URLs.** "Current URL/link items use the stored source URL" doesn't say how two URLs are compared, and scenario 19 depends on it. Options:
   - exact bytes of the stored `source` against the URL as given;
   - a normal form that keeps the query, such as a lower-case scheme and host, the default port dropped, and the fragment kept (an artifact's `#<path>` labels are different files).
   
   Don't use `audience.Canon`: it drops the query, so two different pages would collide. Pick one, state it, and test `HTTPS://Example.com/x` against `https://example.com/x`, and the same path with two different queries.

3. **The 5 MiB file limit and `store.OpenInput`.** "Count bytes before allocating an unbounded buffer" can't happen inside `importer`: architecture 10.2's `OpenInput(b, dest, path) ([]byte, error)` returns the whole file. Either:
   - `OpenInput` takes a maximum, e.g. `OpenInput(b, dest, path string, max int64)`, and fails with `too_large` after an fstat on the opened file and a bounded read; or
   - `store` applies the 5 MiB import limit itself.
   
   `add --body-file` needs a limit too: 512,000 characters is at most about 2 MiB of UTF-8. Agree the signature with @lead-dev (`store.md`), and send @architect the change, since 10.2 is fixed text.

4. **The form check for `--link-only`.** "Validate the HTTP(S) locator" should be an exact predicate, because a linked item's URL is checked "only for form" and the audience component handles canonical form separately. Proposal:
   - `url.Parse` succeeds;
   - the scheme is `http` or `https` in any case;
   - the host is non-empty;
   - there is no user information;
   - there are no spaces or control characters;
   - the whole URL is at most 2,000 code points.
   
   A path with a dot segment passes here, then meets `audience_required` without a flag (architecture 14, "audience decision"). Add the 2,000 and 2,001 cases of scenario 28 to `TestLinkedSource`.

## Mild

5. **Content types.** State what each of these gives:
   - no `Content-Type`;
   - `text/x-markdown`;
   - a `charset` other than UTF-8 on bytes that are valid UTF-8, such as ASCII under `iso-8859-1`;
   - an HTML `<meta charset>` that disagrees with the header.
   
   I'd suggest: a missing type is `unsupported_type`; `charset` must be absent or `utf-8` (any case), else `unsupported_type`; the meta tag is ignored.
6. **Inferred titles.** Collapse runs of whitespace and line breaks to one space before choosing the title (`<title>` often spans lines). Say whether a shortened title ends in "…". Use the same heading rule as `item`'s title fallback: I'll expose it from `item` in `item.md` so both use one function.
7. **Front matter stripping.** Use the same delimiter rule as `item.Parse`: `---` with optional trailing spaces, LF or CRLF, an optional UTF-8 BOM. Then what is stripped on import is exactly what a reader would have taken as front matter. I'll export it from `item` as `item.SplitFrontMatter`. Also state whether a UTF-8 BOM at the start of an imported file is removed. I suggest yes.
8. **Images.** Say whether `<img>` becomes `![alt](url)` or is dropped. Either is safe, since nothing is fetched, but the fixture's expected output depends on it.
9. **Fixtures in the repository.** Commit the static, shell and short-article fixtures under `internal/importer/testdata`, with their expected Markdown, so the probe's result becomes the regression test the architecture asked for.
10. **Tests to add:**
    - scenario 18's "a file import citing a URL is untouched by importing that URL", by name;
    - the cross-host redirect of point 1;
    - a sixth redirect fails with `fetch_failed`;
    - a redirect to `ftp:` fails;
    - a redirect URL with user information fails before any dial;
    - a missing `Content-Type`.

## Checked and fine

- The destination notice comes before any acquisition, from the CLI, and the plan is recomputed inside the lock.
- The original is the converter's exact input: the file bytes, or the decoded entity. It is stored and verified before publishing.
- A linked import stores no original, no `fetched` and no `sha256`, and has no network access.
- The summary is 1 to 300 code points on one line; an empty `--summary` is `invalid` and `--summary` without `--link-only` is `usage`.
- The body limit is 512,000 characters and is never cut.
- `readability.FromDocument` is used, never its network function, and scripts, styles and embeds are removed before extraction.
- The floor is measured on the extracted text.
- One 30-second deadline covers all hops; one resolution per hop, with every address checked, the address pinned, and IPv4-mapped forms classified by the underlying address.
- No proxy, cookies or automatic redirects; gzip is decoded by pbrain with a separate limit.
- `not_found` for a refused path, as in the accepted amendment.
- The `Net` seam matches architecture 13.

## Recheck, sha256 1172fd5dd2e025d8b09bcc4ae539d7de8819a2989798b29738b8167e5b1e1b4f

**Verdict: approve.** Every point is resolved:

1. **Redirects:** TLS and `Host` follow the current hop, tested on a cross-host redirect.
2. **Duplicate URLs** are compared byte for byte, as requested. Case, query and fragment stay distinct, and audience keys are explicitly not used.
3. **Input size:** `OpenInput` keeps its fixed signature and applies a 5 MiB read plus one sentinel byte on the guarded descriptor, after the privacy check. A refused path is `not_found` even when the file is over 5 MiB.
4. **`--link-only`** has an exact predicate, and 2,000 and 2,001 are tested.

The mild points are resolved:
- content types: a missing type, `text/x-markdown` and a non-UTF-8 charset are refused, and meta charset is ignored;
- the title: whitespace collapsed, the "…" cut, and `item.FirstHeading` (exposed in `item.md` at sha256 a12995f8 and later) with no prose fallback;
- the BOM is removed from the cleaned text and kept in the original;
- `item.SplitFrontMatter` is reused;
- `img` is dropped;
- the fixtures are committed under `testdata`;
- the redirect tests are added.

## Recheck of the Download addendum, sha256 02229ba40dbfcda9c196edb95443910467fcc99fa18ed56fcb55593375db3278

**Verdict: approve.** The details are in `reviews/components/cli-lead-dev-2.md`, under "Recheck of the importer addendum".
