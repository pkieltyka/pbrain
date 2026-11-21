# Store component review

Reviewed `b43b638476a6a3771638f3e269825eb6542ee1508531092cec01c23e4629e1c5` against architecture §§4.3, 7 and 10, the accepted guarded-input byte limit, and the brain/item contracts. The destination guard precedes the bounded descriptor read. The design specifies no-follow child operations, section-local locking, no-overwrite publication and truthful post-unlink outcomes.

- **Should fix — Catalog scan and skipped entries.** The item parser returns `FrontMatterError` with a safe reason, line and column. `Skipped` keeps only the code, section and path, so check cannot distinguish a duplicate key with a known location from an oversized file or invalid UTF-8. Preserve the safe reason and location as optional fields in `Skipped`, or state an equivalent diagnostic path using the original scan. Check must not reread a possibly changed file to reconstruct the diagnostic. Test the parser-to-check handoff with a duplicate key and an oversized file in one page.
- **Should fix — Write primitives and purpose.** Bound verification of an existing original to the expected byte count (maximum 5 MiB) before hashing. A damaged large file at the digest path would otherwise hold the section lock while the whole file is streamed. Refuse the referencing import before committing an item. Test with a large sparse file and a counting reader. Update the opening paragraph and the cache-root fixture to reflect the current amendment: item writes leave the semantic index alone and only explicit index builds update it.

**Verdict: changes required.**

## Re-check

At `f459155f7c4fd6ead179e54751633a77f9251f7eebe33e67398811607d953bda` all findings are resolved. The catalog carries redacted parse reasons and source positions from the same snapshot used by check. Existing original verification checks the expected length before reading and reads at most one sentinel byte above it. The updated storage fixtures cover sparse-file corruption and unchanged source diagnostics. Item writes leave the semantic index to the explicit build.

**Verdict: approve.**

## Consistency delta

At `f6d478335cfd0d3ed3d60900b572ffd8d740a793504d650a262b95725924f99d`. `sys.Files.Lock` follows its caller’s context. `store.Begin` applies the earlier of that deadline and 30 seconds to the item-writer lock. `TestLockWaitPolicies` covers both, allowing a concurrent index build to wait on its separate lock.

**Verdict: approve.**

## Output amendment

**Approve** `396c02b72096a5cfd48e8b78f476468bf65d22fa97dea594dd989ccdec68dec6` against architecture §4.1 at `ed3682b8`. After a successful parse, the scanner passes the same validated whole-file bytes and the parser’s byte offset to `item.Measure`. Both retention modes keep the resulting `Item.Layout`. Thus a `KeepNone` scan can discard the body while preserving the file and body coordinates from that read. Its existing version check still governs a subsequent body read.
