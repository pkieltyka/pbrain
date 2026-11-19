# Beads and Gas Town

## Bottom Line

Beads is an agent-facing work tracker with a typed dependency graph and a separate persistent-memory facility. Gas Town uses it to recover assignments and render task-specific context after sessions turn over. pbrain can borrow explicit typed links, exact keyed recall, a small startup or recovery payload and a way to look up omitted items. Factory can retain task tracking, while pbrain keeps versioned Markdown evidence and derives which decisions are current. [Beads types](https://github.com/gastownhall/beads/blob/v1.3.4/internal/types/types.go) · [Gas Town prime](https://github.com/gastownhall/gastown/blob/v1.2.1/internal/cmd/prime.go) · [Factory boundary](https://github.com/pkieltyka/factory/issues/62)

## Key Findings

- **Links have explicit effects.** Blocking edges drive `bd ready` and its atomic claim path. Provenance and supersession can be represented as informational edges. `bd supersede OLD --with NEW` records an old-to-new edge and closes OLD. pbrain needs its own authority and direction semantics. [Link implementation](https://github.com/gastownhall/beads/blob/v1.3.4/internal/types/types.go#L1243-L1352) · [Supersede command](https://github.com/gastownhall/beads/blob/v1.3.4/cmd/bd/duplicate.go#L130-L194)
- **Dolt is canonical in Beads 1.3.4.** Classic SQLite and JSONL-in-Git sync are historical. Current JSONL is interchange, and Dolt handles concurrency and versioned sync with a server for multi-writer setups. Migration and audit-history bugs are documented. [Dolt architecture](https://github.com/gastownhall/beads/blob/v1.3.4/docs/architecture/dolt.md) · [1.3.4 release caveats](https://github.com/gastownhall/beads/releases/tag/v1.3.4)
- **Prime's budgeting is a partial precedent.** Beads provides optional whole-entry caps, an omission notice and recall. A local fixture verified the first entry can exceed the byte cap. Gas Town 1.2.1 restores role, assignment and workflow, and uses a light recovery path. Its full memory renderer injects typed categories without a visible output cap. [Beads renderer](https://github.com/gastownhall/beads/blob/v1.3.4/cmd/bd/prime.go#L521-L587) · [Gas Town prime](https://github.com/gastownhall/gastown/blob/v1.2.1/internal/cmd/prime.go#L263-L303) · [Gas Town memory renderer](https://github.com/gastownhall/gastown/blob/v1.2.1/internal/cmd/prime.go#L590-L657)
- **Compatibility is version-specific.** The `gt remember` write path in stable Gas Town still invokes a Beads KV operation that stable Beads now rejects for memory keys. Beads 1.3.4 also adds a numeric version field to the response Gas Town decodes as a string map. Both Beads calls were checked in a local fixture. [Gas Town memory calls](https://github.com/gastownhall/gastown/blob/v1.2.1/internal/cmd/remember.go#L193-L233) · [Beads KV contract](https://github.com/gastownhall/beads/blob/v1.3.4/cmd/bd/kv.go#L26-L128)
- **Decay applies to closed issue content.** Semantic `bd admin compact` can overwrite live prose with a shorter form, with snapshots recoverable in the inspected Tier 1 code. Top-level `bd compact` is Dolt history maintenance. Use pbrain supersession and bounded rendering to preserve evidentiary history. [Compaction and snapshot](https://github.com/gastownhall/beads/blob/v1.3.4/internal/compact/compactor.go#L86-L150) · [History command](https://github.com/gastownhall/beads/blob/v1.3.4/docs/cli-reference/compact.md)

## Details

- [Work graph and storage](./work-graph-and-storage.md) — data shapes, CLI and semantics, storage history, maintenance and operational reports.
- [Session priming](./session-priming.md) — startup format, caps, exact recall, role hooks and workflow checklists.

## Open Questions / Gaps

- The Beads stable Linux binary was tested for readiness, memory priming and recall. Gas Town hook behavior was read from tagged code and the exact Beads KV calls were tested separately. An end-to-end Gas Town run, Dolt multi-writer benchmark and independent before/after study of agent quality remain gaps.

## Sources

- [Beads v1.3.4](https://github.com/gastownhall/beads/releases/tag/v1.3.4) — November 4, 2026 stable release and upgrade caveats.
- [Gas Town v1.2.1](https://github.com/gastownhall/gastown/releases/tag/v1.2.1) — June 6, 2026 stable release.
- [Steve Yegge, Welcome to Gas Town](https://steve-yegge.medium.com/welcome-to-gas-town-4f25ee16dd04) — January 1, 2026 author account of the original orchestration goals.
