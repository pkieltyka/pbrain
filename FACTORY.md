# Factory

## Project

pbrain ("project brain"): a general-purpose CLI, and a skill that ships with it, for storing a project's knowledge and querying it to give agents context: design and research ideas, decisions and facts, learnings, competitor notes, design docs, links, transcripts and Slack messages, of mixed quality. People and agents feed it; agents are its main readers, on a tight context budget, so it returns a few short cited results or one item by id, never a dump. Its primary consumer is factory, the software factory meta-harness that sparked it (github.com/pkieltyka/factory, local at `../factory`), which adopts it opt-in; factory must keep working without it, and pbrain must not hard-wire factory concepts.

Background: https://github.com/pkieltyka/factory/issues/62 (`gh issue view 62 -R pkieltyka/factory --comments`). The approved overview, research and owner decisions are in `specs/projects/pbrain/`.

Stack: Go, shipped as a single binary; qmd is an optional outside tool. Principles: keep it small and simple; Markdown files in Git are the source of truth and any index is derived and rebuildable; shared by default, with a Git-ignored personal section; every word an agent reads costs context.

## Tasks

Tasks and bugs are tracked in `TODO.md` at the repo root. Put the title on a checkbox line. When it needs more detail, add four-space `*` description bullets. Use the same bullets for record fields (`owner`, `criteria`, `work`, `verdict`, `next`) as they become known. Indent nested lists eight spaces. Leave exactly one blank line between items in the same section and none inside an item.

```markdown
- [ ] Title
    * description
    * criteria: ...
    * next: ...
        1. ...

- [ ] Fix the README typo
```

`## Ready` holds, in the order they will be done, the items the operator may assign without asking the user; only a person puts an item there or reorders it, never the team. Members report discoveries to @pm, or the operator when there is no @pm, who records them in `## Backlog`. An assigned item moves to `## In progress` with its owner at the end of the checkbox line, `(@lead-dev)`. Integrated work moves to the top of `## Done` as `- [x]` with its commit. Keep the tracker small: each Done item is one line (title, owner, commit), with short detail only on the three newest; in-progress items carry at most owner, range or branch, and next step. Evidence lives in Git.

## Workflow

1. The operator assigns tasks with acceptance criteria. @pm records owner, criteria, branch or commit, verdict and next step in the tracker, and only @pm updates it.
2. Consult an architect before deciding a shared interface, the storage format or the CLI surface. Architects' plans are advice; the operator decides.
3. Developers agree shared interfaces before parallel work, implement, then ask @reviewer with the task, criteria and exact diff or commit. They fix findings until approved; after three rounds on one task the operator decides how to go on.
4. The operator reviews the result and integrates it into `master`. Work goes straight to `master`; there are no pull requests.
5. One feature or issue is in progress at a time. Its tasks may run in parallel when each writer has its own worktree; in a shared checkout only one task writes at a time.

## Specs

Specifications live in `specs/projects/<project>/` (overview, functional spec, architecture, implementation plan), written with the operator's `/spec` process: an architect drafts each artifact and research summary, another architect and a lead-dev review it, and the owner approves each step. Read the active project's spec before a task and follow it; raise a disagreement with the operator instead of departing from it. Check: `ls specs/projects`.

## Notes

Disabled.
