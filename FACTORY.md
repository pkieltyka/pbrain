# Factory

## Project

pbrain ("project brain"): a CLI, and a skill that ships with it, that keeps a project's knowledge (notes, decisions, learnings, specs context and imported outside sources) as Markdown with front matter in Git, so agents and people can record it and query it cheaply. Its first users are factory's agent teams (github.com/pkieltyka/factory, local at `../factory`): agents on a tight context budget who need a few short cited results or one item by id, never a dump; and the owner, who adds outside material, confirms decisions and keeps private notes. Factory adopts it opt-in through a recipe and its brief's Notes section; factory must keep working without it.

Origin and starting recommendations: https://github.com/pkieltyka/factory/issues/62 (read it with `gh issue view 62 -R pkieltyka/factory --comments`). Owner decision 2026-11-19: one brain per project to start. Prior art to study: factory's notes convention (`../factory/skill/references/notes.md`), qmd (github.com/tobi/qmd), beads (github.com/gastownhall/beads), the Hermes agent's learning and context features.

Stack: not chosen yet; the spec decides it. Principles: keep it small and simple; Markdown files in Git are the source of truth and any index is derived and rebuildable; every word an agent reads costs context.

## Tasks

Tasks and bugs are tracked in `TODO.md` at the repo root, one line each: `- [ ] <title> -- <description>`. `## Ready` holds, in the order they will be done, the items the operator may assign without asking the user; only a person puts an item there or reorders it, never the team. Members report discoveries to @pm, who records them in `## Backlog`. An assigned item moves to `## In progress` with its owner at the end of the line, `(@lead-dev)`. Integrated work moves to the top of `## Done` as `- [x]` with its commit. An item's criteria, verdict and next step go on indented lines under it.

## Workflow

1. The operator assigns tasks with acceptance criteria. @pm records owner, criteria, branch or commit, verdict and next step in the tracker, and only @pm updates it.
2. Consult an architect before deciding a shared interface, the storage format or the CLI surface. Architects' plans are advice; the operator decides.
3. Developers agree shared interfaces before parallel work, implement, then ask @reviewer with the task, criteria and exact diff or commit. They fix findings until approved; after three rounds on one task the operator decides how to go on.
4. The operator reviews the result and integrates it into `master`. Work goes straight to `master`; there are no pull requests.
5. One feature or issue is in progress at a time. Its tasks may run in parallel when each writer has its own worktree; in a shared checkout only one task writes at a time.

## Specs

Specifications live in `specs/projects/<project>/` (overview, functional spec, architecture, implementation plan), written with the operator's `/spec` process. Read the active project's spec before a task and follow it; raise a disagreement with the operator instead of departing from it. Check: `ls specs/projects`.

## Notes

Disabled.
