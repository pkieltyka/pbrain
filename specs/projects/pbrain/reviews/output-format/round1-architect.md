# Output format, round 1: @architect

Written alone, before reading the other papers. Source: `owner-question.md`; spec 6.1, 6.2, 7.1 and 7.2 at sha256 6c458ba7.

## Answer in short

Keep the output as it is for v1: plain text by default, stored Markdown for bodies, `--json` on every command. Add one sentence to the spec that makes the JSON safe to build an API on. Everything else the owner asks about is a per-result addition, and each of those is paid by every agent on every search, so it should wait for the figure that shows it helps: the "reaching the evidence" measurement that is now in the test plan.

One number frames all of it. A result today is about 330 to 480 characters: an id line of about 70, a title line of up to about 150, a snippet of up to 200 and, on a long body, a bracket of about 35. Five of the largest already do not fit the default 2,000. Whatever is added per result is taken from the number of results on the page.

## The six points

| | Recommendation | Cost | When |
|---|---|---|---|
| (f) Markdown or plain text for agents | Plain lines for everything pbrain writes; the body as stored, which is Markdown. No Markdown markup around results. | None. Markup would be counted in the budget, and a wrapper such as a code fence breaks on a body that holds one. JSON costs more than text for a body: every line break and quotation mark is escaped. | v1, as is |
| (a) Line numbers | Not as an address. Keep the code-point position, which is what `get --offset` takes. | About 10 characters per result. | Later, for people only (see d) |
| (b) Occurrence count per item | Not now. | About 10 characters per result. The pass that picks the snippet already finds every occurrence, so the code is cheap; the page space is not. | Later, if measured |
| (c) Context lines | No `--context N` in lines. If more context is wanted, a snippet length in characters. | Unbounded in lines; see below. | Later, as `--snippet <chars>`, if measured |
| (d) Friendly terminal mode | The text form is the human form. Colour only by an explicit switch, never by detecting a terminal. No Markdown rendering inside pbrain. | None for the agent contract if it is opt-in. | Later |
| (e) JSON for an API | `--json` is enough, with one written rule: fields are only ever added. No version field in the output. | One sentence in spec 6.2. | v1 |

## Reasons

**(a) Lines.** A line is not a usable unit in Markdown prose: a paragraph is often one line of several hundred or several thousand characters, and a heading is a line of twenty. A line number cannot point inside a long line, and `get` is budgeted in characters, so the position would still be needed. Two coordinates for one place is a cost with no gain for an agent. Where a line does help is a person: `path:line` is what terminals and editors turn into a link. That belongs to the human mode, not to the agent's output. Today the position and the section heading appear only for bodies longer than `get`'s limit, which is right: a shorter body comes back whole.

**(b) Counts.** A count says how often, not whether the item answers the question, and long sources win it by length. The order already uses where the match is and the item's standing. If the evidence figures show that the reader's first follow-up read often opens the wrong result, a count is one candidate to try, on the fixture, as an additive field.

**(c) Context.** Two things are wrong with context in lines. First the budget: plus or minus two lines is 40 characters under a heading and 3,000 in prose, and results are never cut in half, so one result could miss the limit and fail with `budget_too_small`. Any such flag ends up capped in characters, which is what the snippet length is. Second, the one-line rule of spec 6.2 is a safety rule: a flattened snippet cannot imitate pbrain's own lines. Raw lines from outside material could print a line that looks like a result or like `0 results.`; they would need a prefix on every line. The agent's way to context exists: `get <id> --offset <position>`, one call. The evidence measurement counts exactly what that call costs. If it is too dear, the lever is a longer snippet on request, for example `--snippet 600`: about two results in 2,000 characters instead of five.

**(d) People.** Detecting a terminal is the usual way to switch colour on, and it is wrong here: some agent harnesses run commands on a pseudo-terminal, and escape codes would then reach the agent and change the size of an output whose limit is exact. So: a flag or a personal setting, never with `--json`, with the budget counted on the plain form. Highlighting the matched words and printing `path:line` are the two things worth having. Rendering Markdown is not: spec 6.2 says pbrain prints stored text and never renders it, and `pbrain get <id> | glow -` or `| less` works today.

**(e) API.** Every command has `--json`, errors are one JSON object and notices are JSON lines, so the answer to the owner's fourth point is that it is there. What an API needs beyond that is a promise, not a format: fields are added and never renamed, removed or changed in meaning, and a reader ignores fields it does not know. A version field would cost characters in every response to say something `pbrain --version` says; a version belongs in the API's own address when there is an API. In the code, an API would call the same document model that `render` prints, so nothing has to be reserved for it now.

## For the owner

- **Proposed spec change, v1:** one sentence in spec 6.2: JSON fields are only added within a major version; none is renamed, removed or given another meaning, and readers must ignore unknown fields.
- **Not proposed for v1:** line numbers, occurrence counts, context lines, a terminal mode. Each is additive later and none is blocked by today's shape.
- **What would change my answer:** the evidence figures. If, on long sources, the 200-character snippet seldom shows the answer and the follow-up read is costly, `--snippet <chars>` moves to v1.
