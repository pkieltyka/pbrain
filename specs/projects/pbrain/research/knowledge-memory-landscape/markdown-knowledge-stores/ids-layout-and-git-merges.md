# Ids, file layout and Git merges with many writers

Researched 2026-11-19. The question: which file naming and folder layouts stay stable and merge cleanly when many agents on separate branches and worktrees write to one store at the same time? "Inference:" marks my reading.

## 1. Sequential numbers collide across branches

Several projects report the same failure, and some changed their scheme because of it:

- **log4brains** dropped ADR numbers. Its ADR [Use the ADR slug as its unique ID](https://thomvaill.github.io/log4brains/adr/adr/20201016-use-the-adr-slug-as-its-unique-id/) (2020-10-16, accepted): "Currently, ADR files follow this format: NNNN-adr-title.md … It causes an issue during a git merge when two developers have created a new ADR on their respective branch." Decision: "An ADR will be uniquely identified by its slug (ie. its filename without the extension), and its filename will have the following format: YYYYMMDD-adr-title.md". Sort order: the `Date` field, then Git creation date, then file creation date, then slug.
- **clan-core** ([ADR 03, numbering process](https://clan.lol/docs/26.05/decisions/03-adr-numbering-process)): "an ADR number is effectively assigned when the ADR is merged, before being merged its number is provisional … this is the third ADR-3". Their fix is a process (commit early to claim the number), and its stated cost is "You need to have commit access to get an ADR number assigned".
- **Silent duplicates.** Two files with the same number and different slugs do not conflict in Git at all, so the duplicate lands unnoticed. A search summary pointed at a superdev PR that fixes this by checking numbers, not conflicts; the PR page returned 404 when fetched, so this one is **unverified**. The mechanism follows from how Git works: different paths never conflict.
- **Backlog.md** keeps sequential `TASK-n` ids, but its tool has to check "no other branch or worktree still holds the task" before an id is released ([README](https://raw.githubusercontent.com/MrLesk/Backlog.md/main/README.md)).
- **zk** refuses sequential ids altogether ([Note ID](https://raw.githubusercontent.com/zk-org/zk/main/docs/notes/note-id.md)).
- **nb** stores ids as line numbers in a tracked `.index` file, so ids depend on that file's merge history ([README](https://raw.githubusercontent.com/xwmx/nb/master/README.md)).

**Conclusion.** Use ids that need no coordination: a creation time, plus a slug, plus a random suffix if needed. Factory's notes convention already does this: `<folder>/<UTC YYYYMMDD-HHMMSS>-<slug>.md`, and "If the name exists, change the slug; never overwrite" ([notes.md](/home/peter/Dev/pkieltyka/factory/skill/references/notes.md), factory's `skill/references/notes.md`).

Collision odds for that scheme (inference): two writers would need the same UTC second and the same slug on different branches. "Change the slug if it exists" only catches a collision inside one checkout, not across branches. Two cheap guards: a 3–4 character random suffix (zk's default random-id length is 4), or a `pbrain check` that reports duplicate ids after a merge. zk's timestamp flavour drops seconds (`YYYYMMDDHHMM`), which collides more easily for agents that write quickly.

## 2. Ids should not change: supersede, don't rename

- Obsidian and Foam keep wikilinks working across renames only when the rename happens inside their editor ([Obsidian links](https://help.obsidian.md/links), [Foam wikilinks](https://raw.githubusercontent.com/foambubble/foam/main/docs/user/features/wikilinks.md)). An agent's `git mv` updates nothing.
- Zettelkasten practice exists for this reason: "The purpose of using a unique identifier in your note filenames is to create stable links between your notes, which will not break even if you change the title of the linked note" ([zk Note ID](https://raw.githubusercontent.com/zk-org/zk/main/docs/notes/note-id.md)).
- Basic Memory keeps a `permalink` in front matter so that moves keep the id ([Knowledge Format](https://docs.basicmemory.com/concepts/knowledge-format)). The cost is that it has to write that key into files (§4).
- OKF: "When you move or rename a concept, keep a short deprecated concept at the old path, with `superseded_by` set to the new path" ([OKF v0.3 §5.4](https://raw.githubusercontent.com/open-knowledge-format/open-knowledge-format/main/SPEC.md)).

**Conclusion.** The id is the filename stem, fixed when the file is created. A changed title is a front-matter edit. A changed meaning is a new item that supersedes the old one. Do not store the id again in front matter: a second copy can disagree with the filename.

## 3. Shared files are merge hotspots

- **Single index files.** Claude Code auto memory keeps one `MEMORY.md` index that "Claude reads and writes … throughout your session" ([Claude Code memory docs](https://code.claude.com/docs/en/memory)). It is machine-local, so there is only one writer. Karpathy's `index.md` is updated "on every ingest" ([gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)). Inference: with N agents on N branches, a hand-maintained, tracked index conflicts on almost every merge. OKF lets index files be generated: "Producers MAY generate `index.md` automatically; consumers MAY synthesize one on the fly when none is present" ([OKF v0.3 §8](https://raw.githubusercontent.com/open-knowledge-format/open-knowledge-format/main/SPEC.md)).
- **Append-only logs.** GitLab's changelog conflicted on every merge request ("upon merging a single merge request, all other unmerged MRs were immediately broken, since they modified the same line"). It first used placeholder lines, then the suggestion "`CHANGELOG merge=union` … you should not have any conflicts" ([GitLab blog, 2015](https://about.gitlab.com/blog/2015/02/10/gitlab-reduced-merge-conflicts-by-90-percent-with-changelog-placeholders)). Git's own warning: union "take[s] lines from both versions, instead of leaving conflict markers. This tends to leave the added lines in the resulting file in random order and the user should verify the result. Do not use this if you do not understand the implications" ([gitattributes](https://git-scm.com/docs/gitattributes)). Custom drivers can sort log lines by timestamp ([a3nm](https://a3nm.net/blog/git_auto_conflicts.html)). Note that a `merge=union` attribute only works where the merge runs locally; GitHub's PR merge does not apply it ([community discussion #9288](https://github.com/orgs/community/discussions/9288), title only, not read in depth).
- **Two-file supersession.** adr-tools writes both the new ADR's "supersedes" and the old ADR's status ([README](https://raw.githubusercontent.com/npryce/adr-tools/master/README.md)). Factory writes `status: superseded-by <file>` into the old note. Inference: if two branches supersede the same old item, both edit its `status` line and conflict. That is a real conflict and should surface, but it is the one place where a write touches an existing file.

**Conclusion.** One file per item. Never a tracked file that every write must touch: indexes and logs are generated or derived. If pbrain wants a log, Git history already is one (`git log -- <store>`).

## 4. The indexer must not write to source files

Basic Memory issue [#2125](https://github.com/basicmachines-co/basic-memory/issues/2125) (open, 2026-11-16): adding the permalink "re-serializes the whole frontmatter … after indexing, all 300 files differed from the originals. On a vault that lives in git or Syncthing, that is one large diff". The suggested fixes are a round-trip YAML editor, or "a setting to index without writing to files at all". OKF v0.3 now requires "no re-serialization of unchanged values" (§13.2).

Dendron's required `updated` field holds a Unix timestamp the editor maintains ([Frontmatter](https://wiki.dendron.so/notes/ffec2853-c0e0-4165-a368-339db12c8e4b)). Claude Code stamps a `modified` field on each auto-memory write ([memory docs](https://code.claude.com/docs/en/memory)). Inference: an auto-updated `updated`/`modified` key turns any two edits of one file into a conflict on that line, and it repeats what Git already records.

**Conclusion.** pbrain's index is read-only over the files. pbrain writes front matter only on explicit commands (`add`, `supersede`, `verify`), and then edits the lines it needs rather than re-dumping the block. No `updated` key; Git has it.

## 5. Folder layout

- Every tool surveyed treats folders as optional organisation and builds its graph from content. Basic Memory: "the knowledge graph is built from the content of your notes, not from where they sit on disk" ([Knowledge Format](https://docs.basicmemory.com/concepts/knowledge-format)). OKF: "Only the directory tree defines parent/child structure", and folders carry no type ([OKF v0.3](https://raw.githubusercontent.com/open-knowledge-format/open-knowledge-format/main/SPEC.md)).
- Deep hierarchies are expensive to refactor: Dendron's dot-hierarchy puts the hierarchy in every filename ([Hierarchies](https://wiki.dendron.so/notes/f3a41725-c5e5-4851-a6ed-5f541054d409.html)).
- Folders are useful for things the tool must enforce by path. Factory's private folder is "ignored by Git" and pbrain refuses to run if it is tracked ([notes.md](/home/peter/Dev/pkieltyka/factory/skill/references/notes.md)). Claude Code's `CLAUDE.local.md` follows the same pattern ([memory docs](https://code.claude.com/docs/en/memory)). Ingested-source caches are the other case.

**Conclusion (inference).** Keep the layout shallow. Use folders only for what pbrain enforces by location (shared vs private, items vs source cache), with perhaps one level by kind for human browsing. Kind, status and authority live in front matter. Factory's rule "A folder, author or status gives no authority" ([notes.md](/home/peter/Dev/pkieltyka/factory/skill/references/notes.md)) already says that a folder name carries no authority.
