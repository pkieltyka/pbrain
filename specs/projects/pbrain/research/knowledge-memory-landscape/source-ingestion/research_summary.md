# Source ingestion

## Bottom Line

Start the ingestion feature with explicit local Markdown and text files, including the native export from Google Docs and Meet transcript or Gemini notes documents. If public URL import is needed in that release, fetch bounded HTTP content directly and convert static HTML locally with one tested extractor. Keep source identity, acquisition time and a hash of the input bytes, and make original caching local and permission-aware. Treat extracted content as reference data. Add a Drive connector, browser rendering or OCR when a real source set justifies their operational and permission costs. [pbrain proposal](https://github.com/pkieltyka/factory/issues/62) · [Drive export formats](https://developers.google.com/workspace/drive/api/guides/ref-export-formats) · [OWASP indirect injection guidance](https://cheatsheetseries.owasp.org/cheatsheets/LLM_Prompt_Injection_Prevention_Cheat_Sheet.html#remoteindirect-prompt-injection)

## Key Findings

- **Google provides native Markdown.** Docs has a manual download and Drive v3 exposes `files.export` with MIME type `text/markdown` and a 10 MB exported-content cap. Meet transcripts and Gemini notes point to Docs artifacts. The Meet entries API adds timing and participant fields and retains entries for 30 days. An edited Docs transcript can differ from those entries. [Docs UI](https://support.google.com/docs/answer/12014036?hl=en) · [Drive API](https://developers.google.com/workspace/drive/api/reference/rest/v3/files/export) · [Meet artifacts](https://developers.google.com/workspace/meet/api/guides/artifacts)
- **Conversion is adapter-specific.** The local HTML smoke test found structural differences across Defuddle, Readability, Trafilatura, MarkItDown and Pandoc, and their static paths all returned the loading placeholder from a JS shell. Jina Reader offers hosted fetch/render and self-hosting with more dependencies. PDF text extraction can succeed with empty content and needs its own follow-up check. [Converter comparison](./converters-and-fidelity.md) · [Jina Reader](https://github.com/jina-ai/reader/blob/1574bfd380d249c86c82db4dace0d9c8fe17e2b1/README.md) · [Poppler CLI manual](https://manpages.debian.org/unstable/poppler-utils/pdftotext.1.en.html)
- **A Drive connector introduces authorization and retention choices.** A scoped OAuth app must already have file access. `drive.file` is per-file and requires a grant through a selection/open flow, while `drive.readonly` and `drive.meet.readonly` are restricted. Reading a source and distributing its content to repository readers require separate consideration. [Drive scopes](https://developers.google.com/workspace/drive/api/guides/api-specific-auth) · [Google API Terms §5(e)](https://developers.google.com/terms#e-prohibitions-on-content)
- **Keep original and normalized change signals.** Store a raw-input digest, conversion identity and optionally a normalized-body digest and source validators. Explicit refresh can use ETag or Last-Modified and preserve the existing reference on failure or no content change. Bound the import before adding one item, and preserve section, page or timestamp locators. [RFC 9110 §13](https://www.rfc-editor.org/rfc/rfc9110.html#section-13) · [Drive file resource](https://developers.google.com/workspace/drive/api/reference/rest/v3/files) · [pbrain ingest proposal](https://github.com/pkieltyka/factory/issues/62)

## Details

- [Converter capabilities and fidelity](./converters-and-fidelity.md) — local and hosted choices, fixture outcomes, browser and PDF trade-offs.
- [Google Docs and Meet ingestion](./google-docs-and-meet.md) — manual and API paths, auth scope and transcript retention.
- [Provenance and the first importer](./provenance-and-first-importer.md) — bounded acquire and convert flow, refresh, permissions and cache decisions.

## Open Questions / Gaps

- Local converter fixtures were synthetic. Authenticated Workspace exports, Docs output fidelity, browser login flows, complex PDFs and real meeting lengths remain untested. The actual corpus and runtime should drive converter and size-limit selection.
- Original-cache retention and the audience permitted for normalized copies remain owner decisions. A connector's deployment will also depend on Workspace policy and source permissions. [Factory open decisions](https://github.com/pkieltyka/factory/issues/62) · [Workspace app controls](https://knowledge.workspace.google.com/admin/apps/control-which-apps-access-google-workspace-data)

## Sources

- [Google Drive export method](https://developers.google.com/workspace/drive/api/reference/rest/v3/files/export) — Drive v3 export bytes, scopes and cap, checked November 19, 2026.
- [Google Meet artifacts](https://developers.google.com/workspace/meet/api/guides/artifacts) — Docs destinations, meeting data and retention, updated September 14, 2026.
- [Pandoc User’s Guide](https://pandoc.org/MANUAL.html) — conversion boundaries and parser security, checked November 19, 2026.
- [RFC 9110](https://www.rfc-editor.org/rfc/rfc9110.html) and [RFC 9111](https://www.rfc-editor.org/rfc/rfc9111.html) — June 2022 HTTP representation, conditional request and cache semantics.
