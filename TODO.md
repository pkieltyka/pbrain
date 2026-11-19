TODO
====

## Ready

## In progress

- [ ] factory setup: refresh to the latest factory skill (@operator)
    * description: refresh the factory setup guidance and portable sample to match the latest factory skill
    * criteria: update FACTORY.md Tasks, Project, and Specs sections and make `factory.toml.sample` portable
    * work: changes complete and uncommitted
    * verdict: implementation complete; commit awaits owner authorization
    * next: commit when the owner authorizes

- [ ] pbrain spec: functional spec (@architect)
    * description: gather the owner's round-1 answers, then draft the functional spec
    * criteria: Q&A answers recorded in `specs/projects/pbrain/qa_round1.md`; `specs/projects/pbrain/functional_spec.md` drafted; @architect-2 and @lead-dev review it, with findings in `specs/projects/pbrain/reviews/functional_spec/round1-<name>.md`
    * work: draft complete and uncommitted; both round-1 reviews complete with 6 blocking, 8 should-fix, and 1 nit from @architect-2, and 6 blocking, 9 should-fix, and 1 nit from @lead-dev
    * verdict: changes required; @architect is revising for round 2
    * next: @architect addresses every required fix in both review reports; owner decisions remain pending on Q3 ranking and Q5 secret scan
        * @architect-2's full findings and fixes: `specs/projects/pbrain/reviews/functional_spec/round1-architect-2.md` (6 blocking, 8 should-fix, 1 nit)
        * @lead-dev's full findings and fixes: `specs/projects/pbrain/reviews/functional_spec/round1-lead-dev.md` (6 blocking, 9 should-fix, 1 nit)

## Backlog

## Done

- [x] pbrain spec: project overview rewrite (@architect) `4f1988e`
    * description: rewrite the brain-dump intro into a clean overview
    * criteria: the owner's criteria sent to @architect; deliver `specs/projects/pbrain/project_overview.md`
    * verdict: complete; owner approved; no peer review needed for the owner's document
    * next: none

- [x] pbrain spec: research, knowledge-memory-landscape (@lead-dev, @lead-dev-2, @architect) `4f1988e`
    * description: five subtopics and the cross-subtopic `research_summary.md`
    * criteria: @lead-dev covers qmd, beads and Gas Town, and source ingestion; @lead-dev-2 covers Markdown knowledge stores and agent memory systems; @architect writes `specs/projects/pbrain/research/knowledge-memory-landscape/research_summary.md`
    * verdict: complete; feeds the functional spec
    * next: none
