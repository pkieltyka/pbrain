# Factory

## Project

pbrain ("project brain"): a general-purpose CLI, and a skill that ships with it, for storing a project's knowledge and querying it to give agents context: design and research ideas, decisions and facts, learnings, competitor notes, design docs, links, transcripts and Slack messages, of mixed quality. People and agents feed it; agents are its main readers, on a tight context budget, so it returns a few short cited results or one item by id, never a dump. Its primary consumer is factory, the software factory meta-harness that sparked it (github.com/pkieltyka/factory, local at `../factory`), which adopts it opt-in; factory must keep working without it, and pbrain must not hard-wire factory concepts.

Background: https://github.com/pkieltyka/factory/issues/62 (`gh issue view 62 -R pkieltyka/factory --comments`). The approved overview, research and owner decisions are in `specs/projects/pbrain/`.

Stack: Go, shipped as a single binary; qmd is an optional outside tool. Principles: keep it small and simple; Markdown files in Git are the source of truth and any index is derived and rebuildable; shared by default, with a Git-ignored personal section; every word an agent reads costs context.

EmbeddingGemma 2 workloads are off for the team: do not build, download or run it. PBRAIN_ALLOW_GEMMA2=1 turns them back on; only the owner flips it.

## Tasks

Tasks and bugs are GitHub Issues on pkieltyka/pbrain, managed with `todos` (github.com/pkieltyka/todos); the board is pkieltyka/3 (https://github.com/users/pkieltyka/projects/3), columns Backlog, Todo, In progress, Done. Run from the checkout. Learn it once with `todos skill`; agent operations always pass `--json`.

Only a person moves an issue into Todo. That column means the operator may assign it without asking; it is not an assignment to a member. Assigned work moves to In progress.

@pm owns tracker writes, or the operator when there is no @pm. Members report discoveries to that owner, who records them in Backlog. Record owner, criteria, branch or commit, verdict and next step on the issue, as comments. Integrated work is closed (`todos close N --reason completed --comment '<commit>'`) and moves to Done.

Check the board with `todos board --json`; pass `--project pkieltyka/3` if it shows another. Check failures with `todos doctor --json`; reconcile an uncertain write before retrying. Never run `todos rm`; close instead, after confirmed integration. A change counts as integrated only when its GitHub Actions run is green on Linux. macOS is out of scope for now: its CI test jobs are commented out and Mac work (#1, #9) is on the Backlog.

## Workflow

1. The operator assigns tasks with acceptance criteria. @pm records owner, criteria, branch or commit, verdict and next step on the issue, and only @pm updates the tracker.
2. Consult an architect before deciding a shared interface, the storage format or the CLI surface. Architects' plans are advice; the operator decides.
3. Developers agree shared interfaces before parallel work, implement, then ask @reviewer with the task, criteria and exact diff or commit. They fix findings until approved; after three rounds on one task the operator decides how to go on.
4. The operator reviews the result and integrates it into `master`. Work goes straight to `master`; there are no pull requests.
5. One feature or issue is in progress at a time. Its tasks may run in parallel when each writer has its own worktree; in a shared checkout only one task writes at a time.

## Specs

Specifications live in `specs/projects/<project>/` (overview, functional spec, architecture, implementation plan), written with the operator's `/spec` process: an architect drafts each artifact and research summary, another architect and a lead-dev review it, and the owner approves each step. Read the active project's spec before a task and follow it; raise a disagreement with the operator instead of departing from it. Check: `ls specs/projects`.

## Notes

Disabled.
