# Curator: lifecycle, pruning and consolidation of self-written skills

The curator is Hermes's answer to skill bloat. It came from issue [#7816](https://github.com/NousResearch/hermes-agent/issues/7816) (opened 2026-04-11, closed 2026-04-30), whose problem statement is: "Hermes can create, edit, and patch skills, but it has no way to retire them … Stale or environment-specific guidance keeps influencing the agent after conditions change. Repeated patches cause drift, conflicting instructions, and over-generalization." (These quotes are a summary from WebFetch of the issue page, not verbatim.) First commit: 2026-04-26, `feat(curator): background skill maintenance (issue #7816)`.

Primary sources: [Curator doc](https://hermes-agent.nousresearch.com/docs/user-guide/features/curator), [`agent/curator.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/agent/curator.py), [`tools/skill_usage.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/tools/skill_usage.py), [`hermes_cli/config_defaults.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/hermes_cli/config_defaults.py). Baseline v0.23.4.

## Scope

By default the curator manages only skills marked `created_by: agent` in `.usage.json`, which only the background review fork (or `hermes curator adopt`) sets. Bundled skills are opt-in (`curator.prune_builtins`, archival only). Hub-installed skills are "always off-limits", and external and project dirs are never touched. "The curator also **never auto-deletes** — the worst outcome is archival into `~/.hermes/skills/.archive/`, which is recoverable."

Practical consequence (from the docs): skills the foreground agent wrote at the user's request (`created_by: learn`) are never curated, so a library can "look fully curated while most of it is untouchable". `hermes curator status` reports the unmanaged count.

## When it runs

Not cron. It is an inactivity check run on CLI start, during gateway housekeeping, and on an hourly `hermes serve` timer. It runs only if `interval_hours` (default **168**, 7 days) has passed **and** the agent has been idle `min_idle_hours` (default **2**). The first observation only seeds `last_run_at`, which defers the first real pass a full interval: "This gives you a full interval to review your skill library, pin anything important, or opt out entirely". `hermes curator run --dry-run` previews.

## Phase 1: deterministic transitions (default on, no LLM)

Code defaults (`config_defaults.py`):

```python
"stale_after_days": 14,  # mark "stale" after this many unused days
"archive_after_days": 30,  # move to skills/.archive/ (recoverable) after this many
```

- `active → stale → archived` based on the latest real activity (`last_used_at` / viewed / patched).
- Skipped: pinned skills and skills named by any cron job, even paused ones.
- Grace for never-used skills: "not archived until they are at least `stale_after_days` old. Zero uses is absence of evidence, not proof the skill is disposable."
- `skill_view` counts as use: "Viewing is actively loading the skill to act on it — that counts as use (the curator's stale timer keys off last_used_at)" ([`tools/skills_tool.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/tools/skills_tool.py)).

**Doc inconsistency:** the same Curator page later says "`active` → (30d unused) `stale` → (90d unused) `archived`" and mentions "a fresh 90-day window". Those are the numbers issue #7816 originally proposed (`stale_after_days: 30`, `archive_after_days: 90`). The code and the config section of the same page say 14/30. Trust the code.

## Phase 2: LLM consolidation (opt-in, `curator.consolidate: false` by default)

Off by default "because it costs aux-model tokens on every run and makes broad structural changes to your library". A sweep "typically takes 50–100 API calls". It can be routed to a cheaper model via `auxiliary.curator`. Before it runs, a tar.gz snapshot of the skills tree is taken.

The fork gets `skills_list`, `skill_view` and `skill_manage` only ("You have NO terminal access in this pass — every filesystem mutation goes through skill_manage above so it is ledgered and rollback-able"). Excerpts from `CURATOR_REVIEW_PROMPT`:

> "This is an UMBRELLA-BUILDING consolidation pass, not a passive audit and not a duplicate-finder. The goal of the skill collection is a LIBRARY OF CLASS-LEVEL INSTRUCTIONS AND EXPERIENTIAL KNOWLEDGE. A collection of hundreds of narrow skills where each one captures one session's specific bug is a FAILURE of the library — not a feature. An agent searching skills matches on descriptions, not on exact names …"

> "DO NOT reject consolidation on the grounds that 'each skill has a distinct trigger'. Pairwise distinctness is the wrong bar. The right bar is: 'would a human maintainer write this as N separate skills, or as one skill with N labeled subsections?'"

> "Identify PREFIX CLUSTERS (skills sharing a first word or domain keyword). … Expect 10-25 clusters."

> "If you end the pass with fewer than 10 archives, you stopped too early"

Three consolidation moves: merge into an existing umbrella, create a new umbrella, or demote a sibling into the umbrella's `references/`/`templates/`/`scripts/`. "Package integrity" rules forbid flattening a skill that has support files into a single reference file. Each archive must use `delete` with `absorbed_into=<umbrella>`, which the tool verifies exists. The run must end with a YAML block of `consolidations` / `prunings`. The curator also rewrites cron job skill references to point at the new umbrella.

**Measured outcome (single data point):** in the maintainer's validation run on #7816, a "346-skill collection … produced 249 archives and 18 class-level umbrella skills", with a pinned skill left alone (from the WebFetch summary of the issue thread). This is the only quantitative curator result I found.

**My observation:** the prompt pushes hard toward merging ("fewer than 10 archives … you stopped too early"). That aggression explains why consolidation is opt-in and is surrounded by snapshots, the ledger, `absorbed_into` verification and a rename map in the run summary.

## Reports and recovery

- Each run writes `~/.hermes/logs/curator/<ts>/run.json` and `REPORT.md`. The summary lists every `old-name → new-name` rename.
- `hermes curator restore <skill>`, `rollback` (whole run), `rollback <ledger-entry>` (one mutation), `purge` (explicit only, with `archive_ttl_days`, default 0 = never).
- `hermes curator status` shows counts, pinned/locked skills and the 5 least-recently-used skills.

## Usage telemetry (`.usage.json`)

```json
{ "my-skill": { "use_count": 12, "view_count": 34, "last_used_at": "…", "last_viewed_at": "…",
  "patch_count": 3, "last_patched_at": "…", "created_at": "…", "state": "active",
  "pinned": false, "archived_at": null } }
```

From the docs: "Bundled and hub-installed skills are explicitly excluded from telemetry writes." The curator prompt warns that "The counters are new and often mostly zero. Judge overlap on CONTENT, not on use_count."
