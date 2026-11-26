# Update safety and governance: how skill writes are validated, gated, audited and undone

This covers what stops Hermes from writing bad, dangerous or unwanted skills, and how changes are versioned and rolled back. Baseline: hermes-agent v0.23.4.

## The single write path: `skill_manage`

All agent skill mutations are supposed to go through one tool ([`tools/skill_manager_tool.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/tools/skill_manager_tool.py)). Its advertised schema is an **atomic operations array**:

> "Create, update or delete skills: your procedural memory for recurring tasks. The operations apply atomically — any failure rolls every touched skill back. Ops: create (… must precede that skill's other ops), patch (targeted old_string/new_string fix — preferred), write_file/remove_file (supporting files), delete (sole op only). Existing skills are modified wherever they live. Description: max {desc_limit} chars, 'Use when <trigger>. <behavior>.', never the category name (the index shows it). Write lessons, not logs: imperative rule + why, no PR numbers/dates/incident narration, one rule per lesson, references/ named by topic (extend before adding)."

| Action | Effect |
|---|---|
| `create` | New skill dir + SKILL.md. Refused if a skill with that name exists anywhere ("A skill named '{name}' already exists at {path}"). This is the only hard duplicate check, and it is by exact name. |
| `patch` (old_string/new_string) | Fuzzy find-and-replace in SKILL.md or a support file, using the same matcher as the file `patch` tool. Unique match unless `replace_all`. The docs call it "preferred". |
| `patch` with `content` (legacy `edit`) | Full SKILL.md rewrite, "last resort — read it via skill_view() first". |
| `write_file` / `remove_file` | Support files under `references/ templates/ scripts/ assets/` only. No traversal. 1 MiB max. |
| `delete` | Foreground: hard `rmtree`, but the ledger can restore it. Background fork: routed to a recoverable **archive**. |

Each action has its own JSON-schema branch with `additionalProperties: false`, because "a 27B model that just used write_file kept emitting file_content on create and the whole batch rolled back (#112677)".

**Important gap (documented):** the gates below apply to `skill_manage` only. The generic `write_file`/`patch`/`terminal` tools can still edit skill files in the foreground. From [`tools/write_approval.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/tools/write_approval.py): "the generic file tools (``write_file``/``patch``) do not consult it (#60440 tracks closing that gap)". The background fork has no write tools, so this gap affects foreground turns only. The **lock** feature is the exception: it does block file tools and terminal writes.

## Guard layers, in the order a write meets them

1. **Background preflight and lock check.** A locked skill refuses every agent write, "in normal turns, cron jobs, subagents, the background self-improvement review and the curator pass alike, and even under `--yolo`" ([Curator doc, Locking](https://hermes-agent.nousresearch.com/docs/user-guide/features/curator#locking-a-skill)).
2. **Write-approval gate** (`skills.write_approval`, default **false**). When true, every write is staged, whatever its origin: "a SKILL.md is too large to review inline, so staging applies regardless of whether the write came from a foreground turn or the background review." Staged writes persist at `~/.hermes/pending/skills/<id>.json`. Review with `/skills pending`, `/skills diff <id>`, `/skills approve|reject <id|all>` ([Skills System doc](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills#gating-agent-skill-writes-skillswrite_approval)). Added 2026-06-10 (#38199). The docs name the use case: "small models that misjudge what they learned, secure environments, or just wanting eyes on the self-improvement loop".
3. **Shape and name validation** (see [skill-format-and-storage.md](./skill-format-and-storage.md)).
4. **Per-skill mutation lock** (`<skills>/.locks/<sha256>.lock`) held across the whole read-modify-write.
5. **Ledger pre-capture** of the skill directory (telemetry, never a gate).
6. **Read-before-write** (background fork and curator only). [`tools/skill_manager_guards.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/tools/skill_manager_guards.py): "The fork must not patch content it only inferred from the transcript: skill_view/read_file call this, and the write guards require the mark." The prompt tells the model: "Content quoted earlier in the conversation transcript does NOT count — the guard requires a fresh load within this review". `skill_view` dedup stubs are turned off inside the fork so the mark always registers (#95976).
7. **Description budget**: a new or changed description must fit the 60-char index.
8. **Atomic write, then security scan.** `_guarded_write` writes the file, scans it, and on a block restores the original (or deletes the new file). The scan is "Opt-in for foreground writes, always on for the background-review fork". The foreground opt-in is `skills.guard_agent_created`, default false, because "terminal() runs the same code ungated". With a baseline, only findings *introduced* by this write count. The scanner ([`tools/skills_guard.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/tools/skills_guard.py), `SCANNER_VERSION = "skills-guard-v16"`) is a regex/AST static scan for exfiltration (e.g. "curl command interpolating secret environment variable", `~/.ssh`, `~/.aws`, the Hermes `.env`), injection, destructive commands, persistence, etc.
9. **Delete guards.** Pinned skills and `ESSENTIAL_SKILLS` refuse `delete` but allow patches. The background fork may only archive skills that are curator-managed (`created_by: agent`), never pinned, external, bundled, hub-installed or protected built-ins. The curator's consolidation deletes need `absorbed_into=<existing umbrella>` ("so the model can't claim one"). Last line before `rmtree`: refuse symlinks, a skills root itself, or anything outside known roots (the comment cites a Kilo Code bug that "wiped the user's entire working directory").
10. **Advisory linter** ([`tools/skill_linter.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/tools/skill_linter.py), added 2026-08-08). Runs on create, on SKILL.md patches and on `references/` writes, and reports only rules this write *introduced*. Findings are returned in the tool result as `lint_warnings` and never block. Rules include `name-format`, `name-dir-mismatch`, `description-length`, `description-marketing` ("powerful", "comprehensive", "seamless"…), `missing-metadata` (version/author/license/tags), `missing-section` (no "## When to Use"), `shell-utility-reference` (say `search_files` not `grep`), `platforms-gating`, `forbidden-file` (README.md, .env, …), `dangling-reference`, and three anti-bloat rules:
    - `oversized-body`: body > 24,000 chars. The comment says "agent-authored skills grew to sit right under" the 100k hard cap.
    - `references-sprawl`: > 60 reference files. "the hoarding shape this catches was 443 one-per-session files".
    - `incident-log-shape`: at least 4 PR/issue refs at ≥0.5 per 1k chars of prose. "the 100k incident-log SKILL.md this targets sat at ~0.7".

    These calibration comments are the best evidence I found of how unconstrained self-written skills actually decay: huge single files, hundreds of per-session reference notes, and incident narration instead of rules.

11. **Post-success side effects**: ledger entry, prompt-cache clear, usage telemetry (`record_created` / `bump_patch`), and an `on_skill_lifecycle` plugin hook.

## Who owns a skill: provenance, pin, lock

- **Write origin** is a ContextVar, `foreground` or `background_review` ([`tools/skill_provenance.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/tools/skill_provenance.py)). Only background-review creates set `created_by: agent`, the marker that puts a skill under the curator. Foreground and `/learn` creates get `created_by: learn`.
- The docs stress that this is policy, not authorship: "`created_by` is a policy flag, not a provenance claim … it is consumed as "may autonomous curation touch this?"" and "Provenance is declared, never inferred … An automatic "looks agent-made, adopt it" heuristic would eventually archive something you hand-wrote." Users can hand skills to the curator with `hermes curator adopt`.
- **Content writes are not ownership-gated.** The review may "patch, rewrite, and add support files to any skill it learns from (hand-written, URL-installed, bundled, hub-installed, external, or pinned)". A bundled or hub skill edited this way counts as locally modified, so "`hermes update` / `hermes skills update` stop overwriting it" ([Curator doc](https://hermes-agent.nousresearch.com/docs/user-guide/features/curator#adopting-unmanaged-skills)). This is a real trade-off: the agent's lessons win over upstream updates.
- **Pin** blocks deletion and archival only. **Lock** blocks every agent write, including terminal writes into the skill dir ("a policy check, not a sandbox"). Both are stored in `.usage.json`, per profile.

## Versioning, audit and rollback

There is **no semantic versioning of agent edits**. `skill_manage` never touches the `version:` frontmatter field (no "version" logic exists in `skill_manager_tool.py`). The `/learn` standard just sets `version: 0.1.0`. Versioning is operational instead:

- **Audit ledger** `~/.hermes/skills/.curator_ledger.jsonl` (added 2026-08-06): every mutation by any actor (`curator`, `agent`, `user`) records an action, evidence (session id, `absorbed_into`) and per-file `{path, sha256}` before/after manifests. Contents are stored content-addressed under `~/.hermes/.curator_backups/blobs/`. `hermes curator rollback <entry-id>` restores exactly one mutation ("can resurrect a hard-deleted skill"). The ledger is size-bounded (5 MB default, then compaction and dropping the oldest entries), "telemetry, never a gate".
- **Snapshots**: a tar.gz of the skills tree before every LLM consolidation pass (keep 2 by default). Whole-run `hermes curator rollback` takes its own pre-rollback snapshot.
- **Bundled/hub drift**: `.bundled_manifest` origin hashes and `.hub/lock.json` content hashes decide whether an upstream update may overwrite a local copy. Edited copies are "skipped forever" unless reset or forced.
- **No git integration.** Nothing commits skill changes to git. `skills.create_dir` can point at a git-tracked directory, but committing is left to the user. **My inference:** for a Git-reviewable design (pbrain), the ledger's before/after hashes and the pending-diff queue are the parts that would map onto commits or PRs.

## Duplicate and bad-skill avoidance, summarised

| Mechanism | Kind | Where |
|---|---|---|
| Exact-name collision refused on create | hard | `_create_skill` |
| Ambiguous same-tier names refuse to load | hard | skill resolution |
| "Update loaded skill → existing umbrella → support file → new umbrella" preference order | prompt | review prompt, `/learn` |
| Class-level naming rules (no PR numbers, error strings, `fix-X`) | prompt | review prompt, curator |
| Do-not-capture list (env failures, negative tool claims, unresolved failures, one-off narratives) | prompt | review prompt |
| Read-before-write on existing files | hard (fork/curator) | guards |
| Security scan | hard (fork), opt-in (foreground) | `skills_guard` |
| Lint warnings for bloat shapes | advisory | linter |
| Human approval queue | opt-in | `write_approval` |
| Curator consolidation of near-duplicates into umbrellas | opt-in LLM pass | curator |
| Inactivity archival | default on | curator |

There is **no semantic or embedding-based similarity check** at create time. Duplicate avoidance before the curator runs depends on the model following the prompt and on the skill index being visible to it. The issue #7816 item "negative-claim revalidation" (TTL on "X is unavailable" claims) was **not built**: the maintainer closed the issue as "No longer needed" on 2026-04-30, so stale negative claims are handled only by the do-not-capture prompt.
