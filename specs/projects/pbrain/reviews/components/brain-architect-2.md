# Brain component review

Reviewed `components/brain.md` at `d6efa38d43785c3405525db76538bc10a9ec801ed65711e84477be3acfd9388b` against functional §§5, 8.5, 9.2 and 10, architecture §§4, 6, 10 and 12 and the accepted init amendment. The nearest-marker walk, nested repository boundary for init, missing default personal folder, literal Git queries, and guard on both the real index and the external worktree agree with their callers.

1. **Should fix — Interfaces and Discovery and configuration.** Unconditional rule validation in `ReadLocal` during `Open` prevents ordinary shared searches when the TOML and personal path parse but an audience rule is invalid. The audience component defers this error until a write or rule-management command. Separate decoding the safety-critical path from validating the rule table. Keep parse and path errors fatal, remember the rule validation error and surface it on every write (including explicit destinations), on listing rules and on edits. Test search and an explicitly shared write against the same invalid rule.
2. **Should fix — Init.** Specify how the edit to an existing `.gitignore` preserves its contents on failure and stays inside the chosen folder. Read a regular file through the target handle and publish the amended bytes through that handle with a temporary sibling and atomic replacement. Refuse a symlink or special file before reading or editing it. Test a symlink to another file and a failed publication over an existing file without a terminal newline. The existing marker and every pre-existing ignore rule must survive a failed attempt, and retry must converge.

**Verdict: changes required.**

## Re-check

At `2b0cfed0d928e069e2a1db74421f68fb1625faf359371c352fa3cf21ed42009b` both findings are resolved. `Open` defers rule errors to `AudienceRules` and strict inside-lock rereads. Init binds the ignore-file edit to the target handle, refuses links, flushes a temporary sibling and preserves existing bytes on publication failure. The test plan covers both regressions.

**Verdict: approve.**
