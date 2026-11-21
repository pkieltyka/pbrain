# Review: components/audience.md (@lead-dev-2)

Reviewed: `components/audience.md` at sha256 aada16a224fd92a1f74920cc449cd13762253eb83a4224baad5d8f2aa311a927, against functional spec 8.5 (sha256 4622a82f), architecture sections 10 and 12 (sha256 8d40545f), and the owner decisions of 2026-11-20 (`model_path` in `personal.toml`). Standards: `design_review_standards.md`.

**Verdict: changes required.** No critical issue. The canonical form, the decision order and the lookup/disclosure split match the architecture. The issues below are about leaving decisions to the coding agent: what counts as a URL source, an invalid stored table, the error code of each function, and the contract with `brain` and `store` for the settings edit.

## Moderate

1. **"The source is a URL" is not pinned down.** Spec 8.5 gates a write whose `--source` *is* an `http` or `https` URL. The doc recognises one "by its scheme, case-insensitively, even when the rest is malformed", but doesn't say what is tested. State the exact predicate, for example: "after trimming spaces and tabs at both ends, the value begins with `http:` or `https:`, in any case". Then state the consequences:
   - A source such as `task 42, https://docs.google.com/document/d/x` is not a URL source and takes the shared default. That is the spec's rule, but it is a known gap the skill must cover ("put the link alone in `--source`").
   - `HTTPS://…` and a leading space are both caught.
   - `https:foo` (opaque) has no canonical form, so the write is `audience_required`.
   
   Add a test for each.

2. **An invalid stored table has no way out through the command.** "A corrupt stored table gives `invalid_config` before any write." If `Set` and `Unset` validate the stored table first (the doc doesn't say), `pbrain audience unset <bad match>` cannot remove the rule that breaks every write, and the only fix is a hand edit. Decide and state:
   - (a) `unset` also removes a stored rule whose raw `match` equals the operand byte for byte, even when the table is invalid; `set` still refuses until the table is valid. Or:
   - (b) both refuse, and the `invalid_config` message names the fix: edit `personal.toml` by hand, giving the rule's position, not its value.
   
   Also say what `pbrain audience` (the listing) does with an invalid table: list what parses and mark the invalid rows, or fail with `invalid_config`. I recommend (a), and listing with the invalid rows marked, so a person can see what to remove.

3. **The error codes of each function are missing.** The depth bar asks for full signatures with error codes. Today the codes are scattered through the prose, and `Set` and `Unset` mix two sources of error. Give a table:
   - `Canon`: a typed error with a reason and no code; the caller maps it.
   - `ValidateRules`: `invalid_config`.
   - `Decide`: `audience_required` or `invalid_config`.
   - `Set`: `invalid` for the operand, `invalid_config` for the stored table.
   - `Unset`: per point 2.
   
   Also say which values each message may name. The `audience_required` message names only the host, and only when the source has a canonical form; a rule-scope refusal names the host and the required prefix shape, never the rest of the path.

4. **The settings edit relies on functions nobody has agreed.** `brain.EditAudience(dir, edit)` and "the store's atomic settings replacement through the same brain directory handle" have no signatures here, and they belong to @lead-dev's `brain.md` and `store.md`. Package order also matters: `store` imports `brain`, so `brain` cannot call into `store`. Write down:
   - the exact signatures, e.g. `brain.EditLocal(d sys.Dir, edit func(*LocalConfig) (changed bool, err error)) ([]byte, bool, error)` and `store.ReplaceSettings(d sys.Dir, data []byte) error`, or whatever @lead-dev settles;
   - who takes the settings lock (`<brain>/.pbrain/lock`, architecture 10.4) and who creates `.pbrain/` on the first `set`;
   - the order: lock, read through the handle, edit, encode, replace, release.
   
   Then confirm the contract with @lead-dev before either doc is approved.

5. **The text form of `pbrain audience` is not given.** "List canonical keys in sorted order with destination, using the list budget" leaves the line format, the closing line and the defaults to the builder. I need these for `render.md`. Proposal:
   - each line is `<match>  <to>`;
   - the closing line is `N rules.`, with `M more: --offset K` when the page is cut;
   - the defaults are `list`'s (`--limit` 20, 1 to 200; `--max-chars` 2,000);
   - an empty table prints `0 rules.`;
   - JSON is `{"rules": [...], "more", "next"}` under the presence rules.
   
   Confirm or replace this, and I'll mirror it.

## Mild

6. **A rule's canonical form changes after it is saved.** `Key.Port` is "empty for the effective scheme's default", but `Key.String` drops the scheme, and a scheme-less rule is read as HTTPS. So `http://host:443/x` becomes `host:443/x` (443 is not HTTP's default), and once that string is saved and read back, its port is dropped: the rule matches different sources before and after the save.
   - Either refuse such a rule, or define the key so that `Canon(k.String(), true) == k` always holds.
   - Add that property as a fuzz test.
   - Also state whether `personal.toml` stores the canonical key or the person's spelling. The doc implies the canonical key; say so.
7. **Name the path accessor.** Use `url.URL.EscapedPath()`, not `Path`: `Path` has already decoded `%2F`, which would let an encoded slash pass. Add a test where a raw `é` and `%C3%A9` give the same key.
8. **Host names `idna.Lookup` refuses.** Lookup applies STD3 rules, so a host with `_` has no canonical form and needs a flag. That is acceptable; state it and test it.
9. **Rewriting `personal.toml` and unknown keys.** "Preserves other settings" should say whether unknown top-level keys survive a rewrite. Architecture 12 keeps known keys only, so a newer pbrain's key would be dropped by an older one. Either decode to a map and keep every key, or state the loss next to the comment loss in architecture 17.
10. **Redirects don't change the decision.** For a fetched URL, the decision uses the requested URL, before and inside the lock; a redirect to another host is not decided again. That follows the spec (a fetched page is public), but say it.
11. **Tests to add:**
    - the five refused shared spellings of scenario 24, by name (`docs.google.com`, `docs.google.com/document`, `Docs.Google.com.`, `acme.slack.com`, `acme.slack.com/archives`);
    - the round-trip property of point 6;
    - reads still succeed while the stored table is invalid;
    - an embedded-URL source falls to the shared default;
    - the unset or list behaviour chosen in point 2.

## Checked and fine

- The decision order (flag, then a URL with no canonical form, then the longest rule, then a gated write, then shared) matches architecture 12.
- The mixed-audience table is looked up by host without the port.
- D and G Slack ids can never match a shared rule.
- Matching is by whole segments.
- The plan is recomputed inside the lock, with `conflict` if it changed.
- Only an explicit `--personal` widens what an error or notice may name, as in architecture 10.1.
- The guard is required for `audience`, with no scan.
- The concurrent-writers test is present.
- The package boundary: no I/O, no `brain` or `store` import.

## Recheck, sha256 58070db6c2b16ee1555318633eb2878f1a0c32fbe5b4670c285ebddf8142b382

**Verdict: approve.** Every point is resolved:

1. **The URL test** is pinned: trim, then `http:` or `https:` in any case. `https:foo` and a URL embedded in text are both tested.
2. **An invalid table** is refused by the listing, `set` and `unset`, with a one-based position and the remedy, a hand edit. Reads continue. This is option (b), and it is tested.
3. **Error codes** are given for each function, with what each message may name.
4. **The settings transaction:**
   - `store.Begin` takes the shared lock;
   - `brain.EditAudience(dir sys.Dir, edit)` returns the payload;
   - `tx.ReplaceSettings` runs only when the table changed;
   - the dependency direction is kept, and unknown keys survive.
5. **The listing:** its line, closing line, defaults and JSON form are given. @lead-dev now owns `render.md` and should mirror them.
6. to 11. **The mild points:**
   - an explicit HTTP port 443 is written as `http://`, and the `Canon(k.String())` round trip is fuzzed;
   - `EscapedPath`, with raw and escaped `é` comparing equal;
   - IDNA with an underscore is refused;
   - redirects never change the audience;
   - all five refused scopes are tested.
