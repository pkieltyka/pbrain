# skill.md, BM25 amendment delta: review by @architect

Reviewed `components/skill.md` at sha256 fb4cd0c4abad5f36e18623f62ff2179abd7852d5746c58f2fcf3381795a44b20 (base b5269729), against the change list of message 290 and functional spec 079b3d19, section 12 and scenario 38. Scope: the delta only.

**Verdict: approve.**

## Checked

- **The limit.** I counted the shipped core block: 386 words by `strings.Fields`, under 400. The file says 386.
- **The prefix clause.** "When endings vary, quote a prefix such as 'cach*' in the shell." Right, and better than the spec's outline: the quoting keeps a shell from expanding the star.
- **`--bm25`.** "For three or more words, --bm25 broadens matching when listed in search help." The three-word condition matches the admission rule (one- and two-clause queries have no partial match), and "when listed in search help" keeps the text true in a release that holds the flag back.
- **Reading approximate results.** "Read semantic and partial hits before relying on them" carries spec 12's "is not an exact match, so read it before relying on it".
- **The test.** `TestSkillResourcesAndLimits` now also asserts the four things the core must teach.
- Nothing else in the core or the topics changed.

## One suggestion, mild

"A partial 2/3 hit matches two clauses" leaves the three unexplained. "A partial 2/3 hit matches two of three clauses" costs two words (388 of 400). Take it or leave it; no re-review needed.

## Re-check after the output amendment

Reviewed `components/skill.md` at sha256 763d7e76f94ccbff157ff15e003a6d4e87f10208e14769b7778fb055a3ed6421 against functional spec d1ba2637, section 12 and scenarios 23 and 39 to 41.

- **Word count, counted by me.** The core text between its fences splits into exactly 400 fields on white space, the same count `strings.Fields` gives: at the limit of 400, with nothing to spare. The four topics count 219, 101, 79 and 96, each under 300.
- **The new sentence.** "Use path:line in file tools or pbrain get <id> --offset <n> at the snippet's body offset. For headings use get --outline. Try --context 2 for nearby lines when space permits. Leave --color to people." This is what spec 12 asks: the two units are told apart by what takes them, context is optional, and colour is for people.
- **Nothing was lost to make room.** I checked the core against every point of spec 12: when to read, the empty result and its retry, the prefix, `--bm25` and the partial marker, `--semantic`, reading approximate results before relying on them, `pbrain index`, `--all`, the time window, how to read a result, the five kinds with an example each, what not to write, replacing and relating, deleting, personal items and the route to the import topic. All are there.
- **Test.** `TestSkillResourcesAndLimits` now also checks the navigation points, and counts the shipped bytes.

**Verdict: approve.**

A caution for whoever edits this next, not a finding: the core is at its limit, so any addition must come with a removal. @lead-dev-2's note that a `personal:` path cannot be opened by a file tool is right and has no room here; spec 7.1 says it, and `get --personal` by id is what the core already teaches for personal items.
