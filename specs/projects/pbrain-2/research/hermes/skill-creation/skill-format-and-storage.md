# Skill format and storage

How a Hermes Agent skill is laid out on disk, what its frontmatter holds, where skills live, and how Hermes's format relates to the agentskills.io / Anthropic Agent Skills format.

Source baseline: `NousResearch/hermes-agent` tag **v0.23.4** (commit `8fc03744`, 2026-11-23; release published 2026-11-24). I read the code from a shallow clone of that tag. Docs quoted are the in-repo `website/docs` at the same tag, which is what [the docs site](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills) serves.

## A skill is a directory with a SKILL.md

From [`tools/skill_manager_tool.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/tools/skill_manager_tool.py) (module docstring):

> "Skills are the agent's procedural memory (narrow "how to do X"; MEMORY.md/USER.md are broad, declarative). New skills land in ~/.hermes/skills/ (or ``skills.create_dir``); existing skills (bundled, hub, user) are modified in place. Layout: ``<skills>/[category/]<skill>/SKILL.md`` + optional ``references/ templates/ scripts/ assets/``."

The [Skills System doc](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills) shows the tree:

```text
~/.hermes/skills/                  # Single source of truth
├── mlops/                         # Category directory
│   ├── axolotl/
│   │   ├── SKILL.md               # Main instructions (required)
│   │   ├── references/            # Additional docs
│   │   ├── templates/             # Output formats
│   │   ├── scripts/               # Helper scripts callable from the skill
│   │   ├── examples/              # Referenced example outputs
│   │   └── assets/                # Supplementary files
├── devops/
│   └── deploy-k8s/                # Agent-created skill
├── .hub/                          # Skills Hub state (lock.json, quarantine/, audit.log)
└── .bundled_manifest              # Tracks seeded bundled skills
```

Sidecar state also lives under the skills dir (from the [Curator doc](https://hermes-agent.nousresearch.com/docs/user-guide/features/curator)): `.usage.json` (per-skill counters, state, pinned/locked), `.archive/` (archived skills), `.curator_ledger.jsonl` (audit ledger), `.curator_backups/` (tarball snapshots), `.locks/` (per-skill mutation locks). Usage counters are deliberately kept **out** of SKILL.md: issue [#7816](https://github.com/NousResearch/hermes-agent/issues/7816) lists "storing counters in frontmatter" as a rejected alternative, to avoid noisy edits to user-authored files.

### What the agent may write

`skill_manage` only writes `SKILL.md` and files under four subdirectories:

```python
MAX_NAME_LENGTH = 64
MAX_DESCRIPTION_LENGTH = 1024
MAX_SKILL_CONTENT_CHARS = 100_000   # ~36k tokens at 2.75 chars/token
MAX_SKILL_FILE_BYTES = 1_048_576    # 1 MiB per supporting file
VALID_NAME_RE = re.compile(r'^[a-z0-9][a-z0-9._-]*$')  # filesystem-safe, URL-friendly
ALLOWED_SUBDIRS = {"references", "templates", "scripts", "assets"}  # for write_file/remove_file
```

(Note: the docs tree also lists `examples/`, which hub installs copy, but `skill_manage write_file` rejects it. `examples/` is not in `ALLOWED_SUBDIRS`.)

Hard validation (`_validate_frontmatter`) requires: the file starts with `---`, the YAML frontmatter closes, it is a mapping, it has `name` and `description`, the description is at most 1024 chars, and there is a non-empty body. A separate check rejects a **new or changed** description longer than the skill-index width (`skills.index_desc_limit`, default **60** chars, set by `SKILL_PROMPT_DESC_LIMIT = 60` in [`agent/skill_utils.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/agent/skill_utils.py)):

> "Description is {len} chars; a skill description must fit the {limit}-char skill index (one sentence, trigger first, ends with a period, without repeating the category name). The index clips anything longer. Move detail into the skill body."

An old over-limit description stays editable, so a body patch is not blocked by legacy frontmatter.

## Frontmatter

From the [Skills System doc](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills#skillmd-format) and [Creating Skills](https://hermes-agent.nousresearch.com/docs/developer-guide/creating-skills):

```yaml
---
name: my-skill
description: Brief description (shown in skill search results)
version: 1.0.0
author: Your Name
license: MIT
platforms: [macos, linux]          # Optional; hides the skill on other OSes
metadata:
  hermes:
    tags: [Category, Subcategory, Keywords]
    related_skills: [other-skill-name]
    requires_toolsets: [web]            # show only when these toolsets are active
    requires_tools: [web_search]
    fallback_for_toolsets: [browser]    # hide when these toolsets are active
    fallback_for_tools: [browser_exec]
    config:                              # non-secret settings stored in config.yaml
      - key: my.setting
        description: "What this setting controls"
        default: "sensible-default"
        prompt: "Display prompt for setup"
    blueprint:                           # optional: makes the skill a suggested cron job
      schedule: "0 9 * * *"
required_environment_variables:          # secrets; prompted on load in the local CLI only
  - name: MY_API_KEY
    prompt: "Enter your API key"
required_credential_files:               # OAuth token files etc., mounted into sandboxes
  - path: google_token.json
---
```

Other fields the code reads: `deps:` (package-manager tool dependencies ensured at `skill_view` time) and `compatibility` (agentskills.io optional field, passed through), both in [`tools/skills_tool.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/tools/skills_tool.py).

### Body conventions

The docs' recommended body is `## When to Use` / `## Procedure` / `## Pitfalls` / `## Verification`. The `/learn` prompt ([`agent/learn_prompt.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/agent/learn_prompt.py)) enforces a longer house order (Title + intro, When to Use, Prerequisites, How to Run, Quick Reference, Procedure, Pitfalls, Verification) and size targets: "~100 lines for a simple skill, ~200 for a complex one".

Two rules in `/learn` matter for privacy and portability:

> "author: always the literal value `Hermes`. NEVER fill it from the host environment — the OS/login username ..., git config, or any identity you can probe must not be written. Skills get shared and published, so an environment-derived name is a privacy leak the user never opted into"

> "Prefer exact commands, endpoint URLs, function signatures, and config keys that appear VERBATIM in the source. NEVER invent flags, paths, or APIs"

### Scripts and dynamic content

- Scripts live in `scripts/` and are run through the `terminal` tool. On load Hermes substitutes `${HERMES_SKILL_DIR}` and `${HERMES_SESSION_ID}` in the body (`skills.template_vars`, default on) ([Creating Skills](https://hermes-agent.nousresearch.com/docs/developer-guide/creating-skills); [`agent/skill_preprocessing.py`](https://github.com/NousResearch/hermes-agent/blob/v0.23.4/agent/skill_preprocessing.py)).
- Inline shell: `` !`cmd` `` snippets in SKILL.md are run and their output inlined. This is **off by default** (`skills.inline_shell: false`), and community hub installs never expand them even when it is on: "any snippet in a SKILL.md runs on the host without approval".

## Where skills live, and precedence

Four tiers, resolved the same way on every surface ([Skills System doc](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills#external-skill-directories)):

> "`project → local (~/.hermes/skills/) → skills.create_dir → external_dirs`. The hidden copy is logged as shadowed."

- **Local** `~/.hermes/skills/` (per profile): "the primary directory and source of truth. On fresh install, bundled skills are copied from the repo. Hub-installed and agent-created skills also go here. The agent can modify or delete any skill."
- **`skills.create_dir`**: redirects *new* agent-created skills elsewhere, e.g. "a shared "brain" directory, a git-tracked repo, or a fleet-wide skills volume". The tool description and prompt text render the configured path. Existing skills are still edited in place.
- **`skills.external_dirs`** (e.g. `~/.agents/skills`): scanned and indexed, but "**External dirs are not a write-protection boundary**: If an external skill directory is writable by the Hermes process, agent-managed skill updates can change files in that directory." The background curator refuses to *archive* external skills.
- **Project-local** `<repo>/.hermes/skills/` and `<repo>/.agents/skills/`: loaded only after `hermes skills trust`. Each skill is security-scanned before it enters the index, and dangerous ones are quarantined. Tagged `[project]` in the index. The curator never modifies them, and "new agent-created skills always go to `~/.hermes/skills/`".
- Duplicate names within one tier are never guessed between: "loading the bare name fails with `Ambiguous skill name dup-demo: use one of a/one, b/two`".

Other sources: bundled (`skills/` in the repo, re-synced on `hermes update` unless the user modified them; tracked by an origin hash in `.bundled_manifest`), official optional (`optional-skills/`), hub installs (skills.sh, GitHub taps including `anthropics/skills` and `openai/skills`, well-known endpoints, ClawHub, LobeHub, browse.sh, direct URL), and plugin skills (`plugin:skill`).

## Comparison with the agentskills.io / Anthropic Agent Skills format

Hermes says skills "are compatible with the [agentskills.io](https://agentskills.io/specification) open standard" ([Skills System doc](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills)). In practice Hermes is a **superset with stricter house rules**:

| Aspect | agentskills.io spec | Anthropic (API / Claude Code) | Hermes |
|---|---|---|---|
| Required fields | `name`, `description` | `name`, `description` (API); all optional in Claude Code ("`description` is the one recommended field") | `name`, `description` (hard-validated) |
| `name` | "Max 64 characters. Lowercase letters, numbers, and hyphens only", must match parent dir | Max 64, lowercase/numbers/hyphens, no `anthropic`/`claude` | `^[a-z0-9][a-z0-9._-]*$`, ≤64 (allows `.` and `_`; dir-name mismatch is only a lint error) |
| `description` | ≤1024 chars | ≤1024 (API); listing truncated at 1,536 chars with `when_to_use` (Claude Code) | ≤1024 hard cap, but a new description must fit the **60-char** index window (configurable 60..1024) |
| Extra metadata | `metadata` is "a map from string keys to string values" | Claude Code adds `allowed-tools`, `disable-model-invocation`, `context: fork`, `paths`, `hooks`, … | Nested `metadata.hermes.{tags, related_skills, requires_*, fallback_for_*, config, blueprint}` plus top-level `platforms`, `version`, `author`, `required_environment_variables`. Strictly, nested maps break the spec's "string to string" metadata rule. |
| Dirs | `scripts/`, `references/`, `assets/` | Any files, e.g. `FORMS.md`, `scripts/` | `references/`, `templates/`, `scripts/`, `assets/` (agent writes restricted to these) |
| Size guidance | "Keep your main `SKILL.md` under 500 lines"; body "< 5000 tokens recommended" | Same: Level 2 "Under 5k tokens" | ~100–200 lines target; lint warning above 24k chars body; hard cap 100k chars |
| Who writes skills | Author | User or tooling. Claude Code has narrow self-writing cases (`/verify`, `/run-skill-generator`), not general auto-creation | **The agent itself**, in the foreground and from a background review fork (see [creation-triggers-and-review.md](./creation-triggers-and-review.md)) |

Sources: [agentskills.io specification](https://agentskills.io/specification) (fetched 2026-11-26, undated); [Anthropic Agent Skills overview](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview) (fetched 2026-11-26); [Claude Code skills docs](https://code.claude.com/docs/en/skills) (fetched 2026-11-26, references Claude Code v2.1.331).

**My inference:** Hermes skills are readable by any agentskills-compatible client (the core is name + description + markdown + subdirs). Hermes-specific gating (`requires_toolsets`, `platforms`, `config`) is ignored elsewhere. Moving the other way, Claude Code skills that use `allowed-tools`, `context: fork` or `$ARGUMENTS` lose those semantics under Hermes. Hermes's 60-char description budget is far tighter than Anthropic's guidance that the description should say "both what the Skill does and when to use it". That is a deliberate token trade-off, and the index width is configurable.
