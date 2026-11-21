# Output amendment review

Reviewed `50c865ad8f373d1e2b20f59027adf37a1e02485f605ec3a2921143b26a6417a1`. The fields derive from one scan, occurrence counts use the existing field-local BM25 rules and context is optional after the base page fits. The architecture preserves the ordinary page and minimum under colour.

1. **Should-fix — §§6.2 (functional spec), 9 and 13.** Carry the path and stdout resolutions from the [spec review](../functional_spec/output-amendment-architect-2.md) through the architecture. The CLI still inspects stdin for body-less add. Preserve the distinction between source bytes and colour marks. A text or JSON path must not silently substitute a different directory name.
2. **Should-fix — §§9 and 14, ordinary get.** Complete the handoff for the existing `render.FitBody` entry point. Its current boolean argument cannot distinguish plain text from colour. Use the same `render.Format` as `Fit`, size the *plain* body prefix including the real line fields, then decorate the retained header. For a nonempty printed slice [a,b), use `LineOf(a)` and `LineOf(b-1)`, not `LineOf(b)`. In a body starting on L9 with `A\r\nB`, printing the first three code points covers only L9, and the next page begins on L10. Add a case that ends exactly on LF or between CR/LF, plus an offset at/past EOF with no line range. Across tight budgets, coloured get must return the identical body slice, range, continuation and minimum. Name this explicitly in the render component handoff so existing callers compile.

**Verdict: changes required.**

## Output amendment re-check

**Approve** `ed3682b8c013d439d98fd30f6f5f74ae98084f85f332c5b4449066d2bcb45f0c`. Sections 9 and 13 carry the exact-path and stdout rules into the renderer and process adapter. Marks belong to the selected part and leave stored body bytes alone.

`FitBody` takes the same format as `Fit`. It accounts for the real closing line before adding colour. For a body beginning on L9, the slice containing `A\r\n` prints L9 through L9. The next character begins on L10. An empty slice has no printed-line range. Section 14 covers the split between CR and LF, EOF, and equality of the coloured and plain continuations and minimum.

The §4.1 definition agrees with the signed naming text, including a linked source with an optional summary. The library table distinguishes the available aikit tag from the planned goinfer tokenizer release and the later release needed for the bytes loader.
