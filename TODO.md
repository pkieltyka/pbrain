TODO
====

## Ready

## In progress

- [ ] pbrain spec: functional spec (@architect)
    * description: gather the owner's round-1 answers, then draft the functional spec
    * criteria: Q&A answers recorded in `specs/projects/pbrain/qa_round1.md`; `specs/projects/pbrain/functional_spec.md` drafted; @architect-2 and @lead-dev review it each round, with findings in `specs/projects/pbrain/reviews/functional_spec/roundN-<name>.md`
    * work: round-3 revision complete; owner ranking decision recorded in `specs/projects/pbrain/qa_round1.md` and applied; @architect-2 and @lead-dev are assigned final review; draft SHA-256 `3b526d45d8a3f1fb339741134d2940b5af462cecc7833ef98d3ca089bb0a30f7`
    * verdict: round-3 final review pending
    * next: @architect-2 and @lead-dev review the final draft and record findings in `specs/projects/pbrain/reviews/functional_spec/round3-<name>.md`; then the operator decides

## Backlog

## Done

- [x] factory setup: refresh to the latest factory skill (@operator) `9ea029e`
    * description: refresh the factory setup guidance and portable sample to match the latest factory skill
    * criteria: update FACTORY.md Tasks, Project, and Specs sections and make `factory.toml.sample` portable
    * verdict: complete; committed in `9ea029e`
    * next: none

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
