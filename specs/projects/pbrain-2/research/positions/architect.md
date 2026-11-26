# brain v2: position of @architect

Written alone, before reading the other positions. Role focus: the overall product and spec shape. Checked against master and the functional spec; nothing was run.

## Position

**Do a variant.** pbrain becomes one tool with three shelves, not two, and the third is what makes agents "get smarter":

| Shelf | What it holds | Where | Authority |
| --- | --- | --- | --- |
| **Memories** | What an agent noticed: handoffs, outcomes, blockers. Cheap to write, attributed, expiring. | Local, per user, outside the repository | None. Advisory. |
| **Items** | Decisions, learnings, observations, notes, sources, as today. | Markdown in Git, with front matter | Standing, lineage, review |
| **Documents** | Any Markdown under configured folders. | Markdown in Git, as people wrote it | None of its own. Reference. |

Agents learn when a useful memory is **promoted** into an item and reviewed. A memory store alone is a log; the loop from memory to reviewed learning is the product. I would not let memories hold decisions: a decision is an item, because it needs a decision maker and a reviewer.

## (a) Any Markdown as a document

Yes for reading, without giving up typed items.

- **The gap is one rule.** The reader is already tolerant: a missing kind reads as note, a missing title is taken from the first heading, a missing status is provisional. What excludes plain Markdown today is spec 2.2: "a .md file without front matter is not an item".
- **Front-matter policy.** None: the file is a document, a note with no author and no date, its title from the first heading. Partial: today's rules. Foreign (Jekyll, Hugo, Obsidian keys): unknown keys are ignored as today, and a known key of the wrong type is treated as missing. Unparseable: index the body as a document and let check warn, where today the file is skipped. **pbrain never writes front matter into a file it did not create.**
- **What breaks: ids.** An id is the file name's stem and must be unique ignoring case. Real documentation trees are full of README.md and index.md, so stems collide and every lookup would answer ambiguous_id. A document's id has to be its path under its root, such as plans/tool-schema. A move then changes the id, which is acceptable for a document and is the reason an item keeps its stem id.
- **What breaks: authority.** A document has no decided, verified or replaces. It never appears in prime and cannot replace or be replaced. In results it must rank after items that have standing, or a long tree of prose will bury the three decisions an agent needs.
- **What breaks: check and the guards.** A file without front matter stops being ignored, so check's findings change, and the secret scan now covers people's documents. Every configured root needs the exposure guard, and the personal section may not lie under a document root.
- **Cost.** Each root multiplies passages. On CPUs the default model indexes about 1.3 passages a second, so a root must be a deliberate choice. Existing brains keep docs/brain; new roots are added in pbrain.toml, never by default.

## (b) Where state lives

- **Derived state in the per-user cache: yes**, as the vector cache already is. The item index must be keyed by checkout, not by project: two worktrees on different branches hold different files.
- **Not derived, so not in the cache:** the originals of imports, which may be the only copy, and section locks, which must sit beside what they protect.
- **Personal state stays physically apart.** A shared read never opens it.
- **Memories:** as the reviewed session-memory plan has them: shared memory in the per-user cache keyed by the repository's common directory, so every worktree of a factory team sees it; personal memory under the personal root; a bounded snapshot replaced under the existing lock.
- **Agents' sandboxes** must be allowed to write that one cache folder. That is an install step and belongs in the README.

## (c) Memories

Take the session-memory plan as reviewed today: explicit store, recall, forget and promote; a lifecycle hook that asks the agent to write a handoff, never a transcript harvester; typed reports shown as unverified; promotion of authored memories only, through the normal writer and normal Git review; seven and fourteen day expiry.

One change for the owner's goal: **recall should answer from memories and items together**, labelled and under one budget, items first. An agent asks "what do we know about X" once. Search stays the items-and-documents command.

## (d) CLI shape

Keep the verbs flat. search, get, add and the rest are the documents-and-items side already, and store, recall, forget and promote sit beside them. A docs namespace would rename fifteen commands for symmetry alone, and aliases would double the tool catalog.

**Timing matters more than taste here.** #80's catalog promises never to rename a tool once it has shipped in a release, and it has not shipped. If the owner wants a docs namespace, the rename must happen before that release. After it, the names are fixed for the major version.

## (e) MEMORY.md or BRAIN.md

Redundant as a tracked file, and against a standing rule: pbrain "never writes a tracked index or log". A generated file goes stale between syncs and conflicts in every merge. The skill says how to use pbrain; prime already says what this brain holds, live and bounded. What is missing is discovery: init should offer three lines for the harness's own instruction file (AGENTS.md, CLAUDE.md) that say "run pbrain skill, then pbrain prime". Extend prime with the task's recent memories and no file is needed.

## (f) First, and refused

Build in this order:
1. Settle the verb names, then ship #80.
2. Memory by hand: store, recall across memories and items, forget. This is the fastest visible gain for a factory.
3. Documents: configured roots, path ids, ranking after items, the check and guard changes, with the indexing cost measured.
4. The lifecycle handoff adapter in factory, then promote.

Refuse: harvesting transcripts; durable shared memory that nobody reviewed acting as the record; an LLM summarizer as a required service; remote sync; rewriting people's Markdown; indexing a whole repository by default; a tracked generated index; renaming verbs after #80 ships.

## (g) Risks

- **Dilution.** Untyped documents and unreviewed memories can outnumber and outrank the few items with standing. Ordering and labels have to be designed, and the retrieval fixture rerun with a document root added.
- **Poisoning.** A wrong memory recalled by five agents becomes five wrong actions. Expiry, the unverified label and review at promotion are the only brakes.
- **Indexing cost** on machines without a GPU, once roots are wide.
- **Two id schemes** in one tool, stem and path.
- **Scope.** This is three projects (#80, documents, memory) and work in the factory repository. Phasing decides whether any of it ships.

## After the Hermes research

Added after reading specs/projects/pbrain-2/research/hermes/research_summary.md. I still have not read the other positions. The research changes three things in the above and confirms the rest.

- **A third promotion target: a document.** Hermes's self-written skills are procedures, and a procedure is neither a decision nor a learning. So promote should be able to turn an authored memory into a document under a configured root: a how-to or a runbook. Three rules come with it. Search the documents first and update an existing one before creating another. Write with a compare-and-swap on the content hash that was read, so a newer edit is never overwritten. Deliver it like any other change, through Git review. A document pbrain creates this way may carry front matter with its provenance; the rule that pbrain never writes front matter into someone else's file stands.
- **The capture rules belong in the skill and in the handoff prompt.** Save lessons, not logs. Save only what this session actually showed. One lesson is one rule; fix the existing one in place. No claims about a person, and no claim that a tool does not work. These are wording, cost nothing to adopt, and are the part of Hermes that its own tracker shows to matter.
- **Refused, more firmly: an unattended writer.** No periodic background pass that writes memories, documents or skills. The research has no controlled evidence that self-written skills help and a documented list of harm: wrong rules that stick, silent rewrites, pollution, secrets quoted back, and 21 to 66 percent more input tokens.

Confirmed: no tracked, agent-written MEMORY.md; a static pointer that a person writes plus a bounded recall at session start is the same thing as my "three lines for the instruction file and an extended prime". Nothing is deleted automatically, and how often something was read never feeds ranking.

Two points for the merge: whether the top document hits join the session-start snapshot is for the evaluation to settle, not for taste; and Hermes's rule of a neutral author name is at odds with the session-memory plan taking the author from the environment, which is a question for @architect-2's sections.
