# Output amendment review

Reviewed `ad0a4f748f913ba898e99eb09cf5ea5226e0f09b07d6f340b72cfeeaf8f60d02` against the signed joint proposal `1c5a95f0` and the owner's whole-file size decision. The 3,000/5 default, paired coordinates, occurrence rules, cumulative output cap, retained `get --offset` and unchanged list projection are present.

## Blocking

1. **Give unusual paths a safe, exact contract (§§6.2, 7.1, scenario 39).** Quoting with the planned Go JSON encoder still emits literal DEL and C1 controls. A Go probe round-tripped a valid path with U+009B but left that raw control in the text. It also mapped an invalid UTF-8 byte in a Unix path to U+FFFD, which can point at a different real path. Require explicit escapes for Unicode controls in quoted text paths. Decide what happens for invalid UTF-8 before emitting a promised exact locator, such as declining that locator and retaining ID navigation. Test both cases, including a real U+FFFD path that must never stand in for another file. Scope the display change to search and get unless the owner wants to change check, delete and error output too.
2. **Distinguish generated colour from opaque stored bytes (§6.2, scenario 41).** The scenario says no sequence ever prints without the flag. Ordinary get must preserve the stored body, including any escape bytes in an adopted UTF-8 file. State that pbrain adds no styling without the flag and never decorates or alters body bytes. Test equality after removing the renderer's own decorations and preserve a raw escape from the stored body in both branches.

## Should fix

3. **Define location and context by snippet presence (§7.1, scenarios 39–40).** A semantic item with a heading-only body can score on its title with an empty passage. Give every snippet-less hit the path-only anchor form with the actual body and file sizes, including a nonzero body size here, and no snippet coordinates or context. For a nonempty span, use the lines holding its first and last code points. A span ending on LF must not invent an empty focal line or skip a real adjacent line. Add a CRLF case whose selected stretch ends exactly on LF and test a final LF at EOF.

Checks: verified hashes, sample field values and the pathname behavior of Go's encoder in scratch.

Verdict: changes required.

## Recheck

Reviewed `d1ba263730a163d1209dc9a9ab34c88b08aa732fac947c75eaeb950da748e177`. Exact paths now escape nonprinting Unicode and withhold an invalid UTF-8 locator while retaining ID navigation. The rule is limited to search and get. Colour only adds renderer-owned sequences and keeps stored bytes, including a stored escape. A snippet-less hit keeps its actual size and no fictitious snippet coordinate. Context now derives its line endpoints from the selected half-open span. Scenarios 39–41 cover the reproductions.

Verdict: approve.
