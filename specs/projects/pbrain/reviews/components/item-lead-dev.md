# Component review: item

Reviewed `cf430c59f8db6885d22c80f0d5d11762817499c46c16ef9c8bab4847e690af77` against functional spec `bd0ffc7c` and architecture `8d40545f`. The single synthetic separator before emitted bodies gives the stated body offset and round-trip. Recoverable field-shape errors are separated from skipped files. The shared fold and parse interfaces fit the store.

1. **Blocking · §§4.1–4.2, validate what the emitter can publish.** The field rules accept U+FFFE and U+FFFF as valid UTF-8 and non-control code points, but the emitter writes them literally even in a quoted scalar. The pinned Go YAML parser rejects a header containing them. A successful write could disappear from the next read as bad front matter. Escape these code points with YAML Unicode escapes, or reject them in field validation before publishing. A scratch go.yaml.in/yaml/v3 v3.0.6 probe rejected the literal U+FFFE and read the escaped version back correctly. Include both cases in the field round-trip fixture.
2. **Should fix · §5, edit empty fields and preserve comments.** The stated splice fails on an explicitly empty single-line field, for example `verified:` or `verified: # keep me`. With v3.0.6 the empty value has column 10 (immediately after the colon), and its `LineComment` is empty in both cases. Appending the actor there without a separator yields invalid YAML, and relying on the value node loses the comment. Handle the empty scalar as an insertion with a delimiter space and identify the trailing comment from the key/raw line. Treat an indented comment after a one-line scalar as a comment, not an automatic multiline veto. Add LF/CRLF and both comment fixtures and assert every unrelated byte survives.
3. **Should fix · §4.4, retain review-date precision.** RFC 3339 input `2026-11-19T00:00:00.123Z` parses, but reformatting with `time.RFC3339` drops the fractional seconds and can make the item due early. Use `time.RFC3339Nano` to normalize an accepted instant and test the exact boundary.
4. **Should fix · §3.4, share the heading definition.** The semantic amendment now defines Markdown heading and fence handling for the outline, but the title fallback ignores Setext headings. In a body with preamble followed by `Section\n=======`, get's outline finds a section and title fallback instead uses the preamble. Reuse the pure outline parser for the heading lookup, preserving the title fallback to the first content line. Test an ATX or Setext heading after a fenced example. Coordinate the package ordering with the architect.

**Verdict — Changes required.**

## Re-check

Reviewed `6edb8b5cb067bc0841c97f70c562eb0438a37e6cc03e6559929ac11c68319a40`. Escaping the three nonliteral code points closes the unreadable-header case. The raw-line editor covers empty values and comments, `RFC3339Nano` retains the supplied instant, and the heading fallback delegates to outline. The `FirstHeading` API comment still says ATX; add Setext there to match §3.4.

**Verdict — Approve.**

## Output amendment

Reviewed `f81097ee7423c28dd5c63fd9daffde7733d634e910674ace498943102a579d36` against spec `d1ba2637`, architecture `ed3682b8` and store `396c02b7`. The measured-byte contract, virtual empty-body line, BOM and CRLF accounting, and forward cursor match the architecture.

**Should fix · §3.7 and `FuzzMeasure`.** An arbitrary integer clamped to the file bounds can still land inside a UTF-8 rune. In a scratch Go check, `[]byte("éx")` at offset 1 produced one rune start under `Measure` and two runes under the suffix oracle. Document that `Measure` accepts valid UTF-8 and the offset from a successful `Parse`. Drop the arbitrary-offset clamping clause and fuzz the same contract, including multibyte bodies.

**Verdict — Changes required.**

### Output amendment recheck

Reviewed `69970eba83125c3a02d4c19ac0b93e4a065f8408acd75db32ef64259ccdd33bb`. `Measure` requires the offset from a successful parse of the same valid UTF-8 bytes. `FuzzMeasure` uses that offset, so both the measured count and the oracle operate on a valid UTF-8 suffix. The fixtures include multibyte text, BOM and CRLF.

**Verdict — Approve.**
