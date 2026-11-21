# Implementation plan review

**Changes required** on `9539b3af11e5919f3ec17baaefd1285c46a441942d02cd04158c63251262eaaa`, reviewed against the complete designs at `24879e6`.

## Blocking

1. **Phases 15, 20 and 22, release table.** The plan enables `Release.BM25` after its standalone pass, then says the model enters the table only after the combined gate passes. Consider 40 held-out questions with 38 empty pages per mode alone and 36 with both. Assuming the individual gain and regression gates also pass, spec §16 says to ship the qualifying model and hold back BM25 if no tested admission rule passes the combination. Make phase 15’s decision valid for that release’s model set. At phases 20 and 22, recheck combined abstention and every regression for each listed model, and say which feature stays out on a combined-only failure. Rerun the rendered-page gates when output changes.
2. **Phases 4, 10–12.** Phase 10’s `add --source` needs the real `audience.Decide` before acquiring input and again when validating the write plan under lock. Phase 12’s linked imports need it too. The plan assigns the decision to phase 11, after 10 and alongside 12. Move the pure decision with its tests into phase 4, leaving the management command and settings update in 11. This lets both worktrees exercise the actual routing and disclosure rules independently.

## Should fix

3. **Phases 17–19.** A test registry substitutes the inference model. Build and query still call the concrete `embed.Find` and `embed.Verify`, and the missing-model build uses `embed.Download` with an injected downloader. Phase 18 therefore cannot finish scenarios 33–37 using only the signatures from phase 0 while phase 19 independently builds those functions. Land the common provisioning functions first, or make them an explicit merge prerequisite for 18. The bge adapter can still run alongside it. For a reviewable boundary within 18, split incremental build/publication and recovery tests from query readiness, scoring and CLI integration.

## Note

Phase 0 should name both `replace` and `exclude` in the module check, as architecture §15 does. The fixture freeze, later goinfer tag, outline and secret-scan ordering, and pre-release arm64 Mac measurement otherwise follow the approved design. No change to those designs is needed.

## Re-check

**Approve** `a2d730494fd34e18e48409f45bfd00994b13eb9ba4c4083771777625b216ee7d`. Phase 3 supplies audience selection to both write paths. Phases 21–23 put provisioning ahead of build, status and query, and phase 23 wires the real downloader after the fetch client lands. Phases 18, 25 and 27 distinguish a standalone pass from eligibility to ship the combined mode. The operator’s two worktree lanes and independent fixture and goinfer assignments are reflected in the checklist. The split reading-command reviews, fixture freeze and release measurements have explicit places.
