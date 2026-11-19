# Functional spec Q&A, round 1 (owner answers, 2026-11-19)

1. Authority: take the recommendation (pbrain records, does not enforce; author, source, status provisional/verified/superseded; decisions record who decided; imported material is reference and never becomes a decision on its own; workflow rules belong to the consumer). But do not overdo recording who did what: Git history or another layer has that. Keep it in the Markdown, tight and simple.
2. Supersession: take the recommendation (the new item's link is authoritative; a command can also mark the old item; a check flags mismatches). The owner is unsure "supersede" is the right word for a CLI command. Operator suggestion for the writer to weigh: a plain-English field and flag, e.g. `replaces:` and `pbrain add --replaces <id>`, instead of a separate verb.
3. Field names and ids: take the recommendation (factory's names, borrowing from Open Knowledge Format only where factory lacks one; ids are date-time + short slug + short random suffix).
4. Location: a folder in the project repo, configurable; one brain per project. Default folder name open; the owner's candidates: `./docs/memory`, `./docs/brain`, `./docs/context`, `./notes`. Writer: recommend one with a reason.
5. Private section: take the recommendation, as either a Git-ignored subfolder, or optionally a Git-ignored marker file (e.g. `PRIVATE.md`) that names where the private data lives elsewhere on the file system, which pbrain reads to find it. It may be called "personal" instead of "private". No Git submodule: too complex.
6. Review dates flag, not hide; usage tracking later.
7. Session-start summary in v1: `pbrain prime` with a hard size cap and a note on what was left out.
8. v1 imports local Markdown and text files, and link items: a URL kept as a bare link, or with its content fetched and indexed plus fetch time; public URLs only. Later: browser use, Google Drive, Slack connectors, PDFs.
9. Cached originals stay local and untracked; the Markdown copy is shared by default; the person adding can mark it private.
10. Runtime: Go, a single binary; qmd optional external tool.

Owner confirmed 8 to 10 as recommended (2026-11-19).
Owner decided (2026-11-19): default folder `docs/brain`; the private section is called "personal".

## Round 1 review decisions (owner, 2026-11-19)

- Q3 ranking: relevance first, standing breaks ties. The owner stresses that the recency of a fact or decision is very important: among matching current items, newer must rank higher. Recency orders results strongly but never hides or replaces an item by itself (only an explicit replaces does); every result shows its date.
- Q5 secret scan: warn, do not block, in v1.

## Ranking and related items (owner, 2026-11-19, after team input)

- Order (option c, @lead-dev's proposal): when the strongest match class contains a current decision, the best such decision is listed first; all remaining results follow the drafted order (match class, recency band, standing, newest). A loosely matching decision cannot jump ahead of a better match.
- Related items, small v1 version, explicit links only: an item that questions or builds on another names it in front matter (`relates`, or a dedicated field such as `challenges`; writer's choice). pbrain never infers relatedness from shared words, tags or a model. A result shows its linked item's id, standing and date beside it when the budget allows (e.g. under the decision: "newer related: <id>, provisional, <date>, <title>"); every result stays its own ranked, cited entry so paging and counts stay predictable. The skill tells agents to add the link whenever they record a doubt about a decision.
