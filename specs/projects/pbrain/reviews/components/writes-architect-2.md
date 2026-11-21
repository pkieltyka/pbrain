# Write handlers review

Reviewed `676765af0b0527eeb7db7b3e96a13afc637846a22de1cecddf966fd6cc1ad224` against functional §§5.2 and 8, architecture §§7.3 and 10, and the proposed item, graph, store and CLI interfaces. Destination before acquisition, explicit disclosure, URL duplicate equality, derived index independence and committed cleanup outcomes agree.

1. **Blocking — Verify, transaction step 3.** Verifying a provisional learning confirms the content a person selected. A change to the body between the first scan and the locked scan still resolves to the same section and path. Taking the fresh snapshot as the expected version would confirm the changed content. Retain the initially observed `item.Version`, require equality inside the lock before the already-verified shortcut or editing, and use that same version at rename. Return `conflict` with the newer bytes untouched. Add a deterministic test that pauses after initial lookup, changes the body under the same filename, then lets verification acquire its lock; also cover a version change after the locked check.
2. **Should fix — Delete preview.** A preview promises a truthful original disposition for a missing or unsafe original, yet the storage API presented to writers exposes original operations only on a locked transaction. Specify the read-only probe (validated digest, borrowed section root, `.pbrain` and `originals` with `create=false`, no-follow regular-file inspection). No directory or lock may appear. Recompute under the confirmed transaction and base the final output on actual unlink/sync results. Test an absent originals directory, unchanged after preview.
3. **Should fix — Secret warnings.** “Token-like boundaries” leaves the three regexes' behavior open and the string helper decides which diagnostic values may be printed. Define the boundary for each token alphabet, how to use it with the Go regexp engine, and how the line/field projection treats list values. Test punctuation-delimited and embedded values, adjacent tokens on one line, and identical redaction in `Sensitive`, `Scan`, and undeclared-tag output. Never include source lexemes in a diagnostic.

**Verdict: changes required.**

## Re-check

At `4d6e54e7d4f2da820c3d941c9ef12271da58044e8288cb302930e2feb122d48e` all findings are resolved. Verify compares the initial version both inside the lock and at replacement, including before the idempotent success path. Preview probes originals through borrowed handles without creating directories or locks. The scanner defines field lines, token boundaries and the shared diagnostic predicate with specific regression cases.

**Verdict: approve.**

## Consistency delta

At `b204c350773f5b8d7a52d8a01311295631c2d4599da368f431090273070f9a2b`. Every import and rule-routed add emits `destination` before acquiring input. A confirmed shared deletion emits `git_history` after success; a preview returns `confirm_required`. The timing, scope and notice codes agree with functional §§6.3, 8.4–8.6.

**Verdict: approve.**
