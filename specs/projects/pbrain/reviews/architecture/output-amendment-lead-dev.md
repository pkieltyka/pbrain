# Output amendment architecture review

Reviewed `50c865ad8f373d1e2b20f59027adf37a1e02485f605ec3a2921143b26a6417a1` against spec `ad0a4f74` and the signed output proposal. The same-scan coordinates, page-first alternatives and decoration after all fitting keep result admission, counts and offsets intact.

## Blocking

1. **Publish the item/store count interface before splitting the work (§§4.1, 5.1 and 7.1).** The change list asks for the four values to be set in `item.Parse` and carried by the store. Today's `Parse` returns only `Header`, `bodyAt`, problems and error, and the architecture adds four fields to `Item` without showing how they cross that boundary. Choose and state the common signature now. For example, keep `Parse` and give store one item-owned layout helper over the validated bytes and its returned `bodyAt`, or return a typed layout with the parse. Include the existing callers and retain offsets in bytes internally. This prevents a parser that computes unused counts or a second independent interpretation of the delimiter.
2. **Preserve the line map without a quadratic fitter (§§4.1, 7.4 and 9).** `LineOf` may scan a body prefix once, but it cannot rescan that prefix for every body length the fitter tries. It would turn `get --max-chars` on one 512,000-code-point line into a quadratic operation. Seed the start line once and advance an LF counter during the existing linear prefix fit. Compute the inclusive last line before consuming a trailing LF. Have outline enumerate ascending section starts in one forward pass too. Specify that `LineOf(body, BodyChars)` describes an insertion position and never claims an actual returned character.

## Should fix

3. **Make the edge states part of the search/render handoff (§§8.4–8.5 and 9).** Define the exact context record or fields that carry its file line, focal span and continuation markers. A half-open nonempty snippet covers from the line at Start through the line at End−1, even when it ends on LF; exclude a CR paired with that line's LF. Use no context or snippet coordinates for an empty semantic span, even on a nonempty heading-only body. [Spec review](../functional_spec/output-amendment-lead-dev.md)
4. **Close the colour and path contracts (§§9, 13 and 14).** Define `Style`/enum values alongside the new shared types and say that `TextColor` is accepted by the raw-body fitter. Marks belong to structural text alone and must be nonoverlapping code-point ranges on the selected part. Distinguish renderer-injected sequences from raw stored body bytes in the test. Align path escaping with the corrected spec before propagating it to render and CLI.

Checks: compared the changed interfaces with current item, store, search and render designs. Exercised the pathname encoder against valid control characters and an invalid UTF-8 filename in scratch.

Verdict: changes required.

## Recheck

Reviewed `ed3682b8c013d439d98fd30f6f5f74ae98084f85f332c5b4449066d2bcb45f0c` and the pin-only semantic delta `5f4a23ca0a1d6e703ec7af0fce4891dbd9107d1bc6d12c81585c2ab67c449294`. `Measure` gives the store one count interface over the same bytes and `bodyAt`. `LineCursor` gives the body fitter and outline a forward pass, with the last printed line taken at `b-1`. The context record and endpoints are explicit. `FitBody` uses `Format`, with nonoverlapping marks only on the selected structural parts. Path and stored-byte handling match the revised spec.

Verified that aikit `v1.79.0` resolves locally to `3c0896c0`, declares the intended module path and has both loader methods. The documents distinguish the planned goinfer tokenizer tag from the later release needed to build the EmbeddingGemma 2 adapter. The Item definition matches the signed naming recommendation.

Verdict: approve.
