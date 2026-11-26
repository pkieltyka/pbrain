---
status: complete
---

# pbrain-2

pbrain is a utility that helps our software factory of agents learn: they store what they find and get smarter over time. Two contexts matter, both first class and different: **memory and docs**.

## Memory

Agents working together in a software factory need automatic session memory, which is what Mnemosyne does well (`specs/projects/pbrain/research/mnemosyne-comparison.md`). It is not the same as spec and doc search, but the commands are pretty close.

- `pbrain store` and `pbrain recall` are the simple memory commands.
- Memories can hold facts, decisions and the like.
- The pbrain skill tells agents to use memory and to store things.
- Maybe a file such as `MEMORY.md`, `KNOWLEDGE.md` or `BRAIN.md` describes pbrain memory and docs for agents.

## Docs

Doc indexing and search are just as important. Brains hold long documents, not short notes, and need at least EmbeddingGemma 2 quality.

- A project's `pbrain.toml` names the docs folder or folders; `./docs` is the default. Any folders with docs or notes can be configured.
- Anyone can add a Markdown document or note under `./docs/**` (or whatever the rule is) and it gets indexed. Move a file into `./docs/**` and pbrain indexes it, if that is the rule, and it becomes searchable.
- Front matter is important; how to handle it is an open question.
- The derived storage (the SQL files) lives in `~/.cache/pbrain/`.
- The CLI gets a namespace for the docs portion, for example `pbrain docs add` and `pbrain docs search`.

## Decisions so far

- Build memory into pbrain, not as a separate project, and do not adopt Mnemosyne (#82).
- Memory and docs are both first-class and distinct.

## Research to do

- **Hermes.** Many people say they like Hermes for its automatic skill creation and updates and its ability to "auto learn": "It's the first system that feels like working with a co-worker, learns by doing." Research how Hermes does this and what pbrain-2's memory should take from it.

## Inputs

- The team's independent positions on this direction and the merged proposal (in progress).
- The approved session-memory design (#83, `docs/plans/session-memory.md`).
- The Mnemosyne research (#82).
- The first iteration's specs (`specs/projects/pbrain/`), which pbrain-2 changes.
