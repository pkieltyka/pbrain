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

## Gated sources and browser use (owner, 2026-11-19)

1. Slack: use Slack's MCP; it does a good enough job. No raw thread import or transcript format. The point is to record notable Slack content found through the MCP into the brain: links, facts, decisions, ordering and prioritization of a project, as ordinary items citing the Slack message.
2. Routes: use an MCP or CLI when available (including popular ones for Slack and Claude artifacts); a direct HTTP fetch when it works. A browser instance with access to the owner's data is fine: the agent asks permission per domain, with "don't ask again for this domain", then just does it. Less interruption is better; asking the person to export is not preferred over the browser.
3. Shared vs personal for gated imports: open, owner's leaning: the agent asks whether it is shared or personal, and remembers the answer per full domain (with subdomain) and possibly URL path prefix, e.g. a company enterprise Claude artifact with sharing open is shared, a company Google Doc is shared. The owner asked for the team's proposal and can be convinced otherwise.
4. browser-use names Chrome DevTools MCP with Chrome's approval step and a dedicated profile for repeated use, plus "or any browser tool you already have".
5. No Git repository inside a personal folder in v1; a private personal repo is a later feature.

## Audience rules and browser permission (owner adopted, 2026-11-19)

- Audience rules per person only, in the Git-ignored personal.yml: exact host (with subdomain) plus optional path prefix on whole segments, query and fragment ignored, longest prefix wins, a tie resolves to personal, conflicting identical scopes rejected. Only the person authorizes a rule; the agent shows the exact scope and offers "just this time" or "remember". No committed team-wide rules in v1 (later, if asking once per person per scope becomes a burden).
- import and add read the rules when neither --shared nor --personal is given; a matching rule decides and the notice names the rule.
- Content from a sign-in-protected source with no matching rule and no flag fails with audience_required before anything is stored; the agent asks and retries.
- Public pages fetched directly keep the shared default.
- No host-wide shared rule on multi-tenant hosts (docs.google.com, claude.ai and similar): a shared rule there must name a document or an identifiable company path.
- Browser permission stays with the browser tool (Claude in Chrome site permissions, Chrome's approval step); pbrain does not own it. Browser access never by itself makes content shareable.
- Slack: items cite the message permalink with add --source; a decision records the person who decided; DM or private-channel content goes to personal.
