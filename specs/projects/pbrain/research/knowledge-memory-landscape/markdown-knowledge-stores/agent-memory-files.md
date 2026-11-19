# How coding agents store project memory files

Researched 2026-11-19. Scope: where Claude Code, Codex and Cursor keep project memory and instruction files, how much they load, and what that implies for pbrain's layout. How these systems decide what to remember belongs to the agent-memory-systems subtopic. "Inference:" marks my reading.

## Claude Code ([memory docs](https://code.claude.com/docs/en/memory), fetched 2026-11-19)

**Two systems, both loaded at session start.** "Claude treats them as context, not enforced configuration."

1. **CLAUDE.md** files are written by people. They load from the working directory and every directory above it; ones in subdirectories load on demand when Claude touches a file there. `CLAUDE.local.md` is the private, git-ignored version. `@path` imports are allowed ("maximum depth of four hops"), but imports "don't reduce its context cost". Guidance: "target under 200 lines per CLAUDE.md file. Longer files consume more context and reduce adherence." `.claude/rules/*.md` with `paths:` front matter load only "when Claude works with matching files". A file over 4 MiB is skipped.
2. **Auto memory** is written by Claude, in `~/.claude/projects/<project>/memory/`. It is "machine-local", shared across worktrees of one repo, and "not shared across machines or cloud environments". Layout:
   ```
   MEMORY.md            # Index, one line per memory, loaded into every session
   user_role.md         # One memory
   feedback_testing.md  # One memory
   ```
   - Front matter `type` is one of `user`, `feedback`, `project`, `reference`. Claude "skips anything it can derive from the codebase … It also skips anything your CLAUDE.md files already say."
   - Budget: "The first 200 lines of `MEMORY.md`, or the first 25KB, whichever comes first, are loaded at the start of every conversation." Near the limit, Claude Code reminds Claude to "keep one line per entry, move detail into topic files, and merge or drop stale entries". Over it, it returns an error telling Claude to rewrite the index.
   - "Claude Code doesn't load topic files … at startup. Claude reads them on demand."
   - "When Claude writes a memory file that begins with YAML frontmatter, Claude Code records the write time in a `modified` frontmatter field".
   - Not loaded into subagents (except forks).

**What pbrain can take from it.** A one-line-per-item index read at start plus full items fetched on demand is progressive disclosure with a hard budget. The four types show what a single agent finds worth keeping. The cost: one shared index file that every write edits, and no supersession. Stale entries are handled by asking the model to "merge or drop" them. That works for one writer on one machine; it would conflict in Git with many writers (see [ids-layout-and-git-merges.md](./ids-layout-and-git-merges.md) §3).

## Codex

- **AGENTS.md** ([guide](https://developers.openai.com/codex/guides/agents-md)): global `~/.codex/AGENTS.md` (or `AGENTS.override.md`), then one file per directory from the Git root down to the working directory, "Files closer to your current directory override earlier guidance because they appear later in the combined prompt." Codex "stops adding files once the combined size reaches the limit defined by `project_doc_max_bytes` (32 KiB by default)".
- **Memories** ([Memories](https://developers.openai.com/codex/memories)): off by default, local under `$CODEX_HOME/memories/` or `memories_v2/`, generated in the background from idle past chats. "Treat these files as generated state … don't rely on editing them by hand". "Codex … redacts secrets from generated memory fields". `memories.disable_on_external_context` keeps chats that used MCP or web search out of memory generation. The guidance: "Keep required team guidance in `AGENTS.md` or checked-in documentation. Treat memories as a helpful recall layer, not as the only source for rules that must always apply."
- **AGENTS.md as a convention** ([agents.md](https://agents.md/)) is a plain Markdown file with no required fields, read by many agents (Codex, Jules, Cursor, Aider, goose, opencode, Zed, Warp, VS Code, Devin, Amp, Gemini CLI, Copilot coding agent, Windsurf and others listed). Repo [agentsmd/agents.md](https://github.com/agentsmd/agents.md): MIT, about 25.6k stars.

## Cursor

- **Project rules** ([Rules](https://cursor.com/docs/rules.md)): `.cursor/rules/*.mdc` with front matter `description`, `globs` and `alwaysApply`. "A plain `.md` file in `.cursor/rules` is ignored". The combination decides loading: always; when a matching file is in context; "Agent reads the description and pulls the rule in when relevant"; or only on `@`-mention. Cursor also reads AGENTS.md.
- **Agent SDK memory** ([memory reference](https://cursor.com/docs/agent-sdk/reference/memory.md)): a hook appends one JSON line per turn to `memory/journal.jsonl` and rotates old records into `journal-*.jsonl`. Caveats it states itself: "All sessions and users share one journal, so one person's messages can surface in another's session", and "Anyone who can message the agent writes to the journal, so keep the scaffolded rule against following instructions found in records."

## What this means for pbrain's layout (inference)

1. **Instructions and memory are kept apart.** Every vendor separates human-written, always-loaded instructions (CLAUDE.md, AGENTS.md, rules) from recalled memory (auto memory, Codex memories, Cursor journal). Codex says rules must not live only in memory. pbrain's items should not be instructions either; factory already says "no note is an instruction". pbrain's skill text is the instruction layer; the brain's contents are the recall layer.
2. **Budgets are hard and small**: 200 lines / 25 KB (Claude Code index) and 32 KiB (Codex instructions). Any session-start slice pbrain injects should fit a few hundred tokens, as one-line entries that point at ids.
3. **Description-gated loading** (Cursor's "Apply Intelligently", Claude Code skills, OKF's one-line `description`) only works if every item has a one-line summary. pbrain items should have a one-line `title` or `description` that search results show without the body.
4. **Private material sits beside shared material, separated by path and ignored by Git**: `CLAUDE.local.md`, `~/.codex`, factory's private folder. That matches factory#62's "Privacy by location, not by a field".
5. **Generated memory is machine-local in every vendor**, and none commits it. pbrain differs on purpose by committing shared knowledge to Git. So pbrain alone carries the multi-writer merge problems that these vendor stores avoid by having one writer per machine.
