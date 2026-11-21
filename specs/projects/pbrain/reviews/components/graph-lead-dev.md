# Component review: graph

Reviewed `adf99ded0ba88c5522d7dca7bccf5c6eeb1126f95ff720eaef83efda990a0e38` against functional spec `bd0ffc7c`, architecture `8d40545f` and the subsequent confirmation of incoming related-line ordering. The joint SCC stamp pass matches §4.3. `Effect` compares the two graphs, excludes the removed file and uses each holder's own view for newly dangling links.

1. **Blocking · §3.2 step 6, successor order.** With edges from replacing items to the items they replace, the stated reverse topological pass visits A before B and C in C → B → A. `succ[A]` then reads `succ[B]` before it exists. Use forward topological order on that edge orientation, or reverse topological order on the inverted graph, and say which one. Internally a current item's set contains itself, while the public successor query gives an empty slice for a current item as the fuzz test expects. Assert the complete successor sets for the chain, diamond and fork.
2. **Blocking · §§3.1–3.2, resolve a legacy stem only within its section.** Spec §4.3 requires exactly one match in the stamped item's section. An accepted personal stamp can disappear in the personal view just because a shared note has the same stem, since the draft applies cross-section ambiguity before the same-section restriction. Build stamp candidates from the issuing section only. Keep the combined-view ambiguity rule for ordinary links. Test a personal stamp with exactly one personal target and a shared file of the same stem, then with two personal targets.
3. **Should fix · §2, make the prospective-cycle premise explicit.** Another file can already have a dangling `replaces` that would become an incoming edge on publication. `CheckNewReplaces` has no new ID argument, so its “no incoming links” property depends on the caller. `writes.md` reserves IDs in existing replacement fields and legacy stamps at allocation. State that prerequisite and keep a write fixture with a candidate that would close a previously dangling cycle.

**Verdict — Changes required.**

## Re-check

Reviewed `935042aafc56ef6b8e6021ef4e4b0f826f7392060e601c833e9a4a4b7872793d`. The successor pass now explicitly processes a replacer before its predecessors. Stamp lookup in §3.2 is section-local and covered by fixtures, and the prospective-cycle assumption is part of the write contract. Remove “and for a stamp” from §3.1's generic resolved-link paragraph so it agrees with the explicit stamp pass.

**Verdict — Approve.**
