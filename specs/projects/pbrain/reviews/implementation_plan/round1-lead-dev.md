# Implementation plan review

Reviewed `9539b3af11e5919f3ec17baaefd1285c46a441942d02cd04158c63251262eaaa` against the complete designs at `24879e6`.

**Verdict — Changes required.**

1. **Blocking · Phases 4 and 10–12.** `add --source` calls `audience.Decide` before acquisition and checks its decision again under the write lock (`writes.md`, “View, preparation and transaction”). Move the pure decision and its tests into 4, with the command and settings edits in 11. The writers and importer can then use the same routing logic without a stub.
2. **Blocking · Phases 17–19.** A test model replaces inference, but `semantic.Build` and the source still need `embed.Find`, `embed.Verify` and the registry. Missing-model recovery also uses `embed.Download` with a fake transfer. Put the model-neutral embed functions before the phases that exercise them, and state which phase wires `fetch.Client.Download` into the real index command. The adapter can proceed independently. Split the current phase 18 at the incremental build and publication boundary so query readiness, scoring and command wiring can be reviewed separately.
3. **Blocking · Phases 15, 20, 22 and release order.** Recompute the BM25 decision against the model set actually shipped. A standalone pass permits a lexical-only BM25 release. If there is no admission rule that also passes the combined gate for every qualifying model, ship the models that passed their individual gates and hold back BM25 (spec §16). Rerun all applicable gates on the released page shape when output changes.
4. **Should fix · Phase 9 and lane assignments.** Keep the first read measurement with the `search` and `get` handlers. Move `list`, `prime`, `tags` and `check` (including `check.Collect`) to a second read ticket. Six command handlers plus the first end-to-end scenarios and benchmark make one large first integration review. Name the actual lead-dev owners and parallel opportunities, including for the gate and measurement phases. Record the operator's two specialist assignments for the fixture and goinfer.

The fixture freeze, separate goinfer tag, and pre-release arm64 Mac measurement are covered. The plan can record the `outline` and `secretscan` order without reopening the architecture.

## Recheck of `11353a98`

The audience prerequisite and combined release policy are fixed. Provisioning now precedes incremental build, and build and query are separate phases. The operator's latest direction covers the remaining first-read split, lane assignments and downloader handoff. Put index status with the source and command phase, or make the provisioning phase a prerequisite for status, since status uses `embed.Find`. Give the command phase an explicit dependency on the phase that delivers `fetch.Client.Download`.

**Verdict — Awaiting the final revision.**

## Final recheck

Reviewed `a2d730494fd34e18e48409f45bfd00994b13eb9ba4c4083771777625b216ee7d`. All 29 phase IDs are ordered and unique. The read integration is split, the lane swap and specialist assignments are recorded, and phase 23 waits for model provisioning and the real downloader. The fixture freeze, per-mode and combined gates, later goinfer tag, and arm64 Mac release measurement remain in place.

**Verdict — Approve.**
