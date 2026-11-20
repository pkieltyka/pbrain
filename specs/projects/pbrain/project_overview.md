---
status: complete
---

# pbrain

pbrain ("project brain") is a CLI and product for storing a project's knowledge (notes, memory, docs) and querying it, to give agents extra context. It is general purpose: anything that needs a queryable store of project knowledge can use it.

What sparked the need: we are building a software factory meta-harness ([factory](https://github.com/pkieltyka/factory)), a team of agents across several harnesses and model families, run in herdr. The factory needs a memory for the project it works on, and it is pbrain's primary consumer. The ticket [pkieltyka/factory#62](https://github.com/pkieltyka/factory/issues/62) has the background.

## What it is for

pbrain is the context and memory of a project. We want an efficient way for our agents to reference this growing knowledge base. The whole point is to engineer a really great system and build a great product that meets our users' needs.

Our agents in the factory use it to record notes and memory as they work; other agents and people can do the same.

It could be a bit of a knowledge graph or it could be simple, but either way it needs a sense of time and some weighting: old facts and old decisions go stale.

We will probably have a pbrain skill file as well, so agents know how to use the CLI, when is the right time to record each type of memory, and how to do it.

## What goes in it

Project context, in engineering, is all of this:

- Design ideas, research ideas, things we thought about and initial discussions.
- Decisions and facts.
- Notes on competitors.
- Design docs, ours and the ones other human team members bring once we start designing and implementing.
- Links sourced from third parties, and lists of URLs.
- Meeting transcripts and Slack messages.
- Learnings from running tests and from production environments.
- Our mistakes: we will get things wrong and have to learn from them.

Specs belong somewhere else.

The quality is mixed. We will have noisy data, correct data and incorrect data, all of it.

## Who uses it

pbrain is multiplayer: many humans, and the agents working for them, feed data into it, and many access it.

Agents are the real consumers. We are not really going to read it manually as humans. We feed it data and give it to our agents (the software factory team, the design team, the architect team), who use it as context to make the best decisions when architecting, programming, implementing, testing, delivering, deploying and so forth.

## Storage

Markdown is a great start: structured Markdown on the file system, in the project's own repository or in a different folder. The notes get committed, and that is also how they are shared.

I'm not sure yet of the best folder structure, nesting or naming conventions.

## Shared by default, with a private section

By default the contents are shared: the brain goes into a Git repository, so everyone with the repository has it. It would also be good to have a private section, for when someone wants a private doc or private notes. A Git-ignored folder is one way; a Git submodule or an external folder that is linked in are others worth considering. Bottom line: shared is the default, and private notes and a private section are a good addition.

## External sources

A project also has external documents and sources. We want to keep pbrain simple, but it can certainly hold URLs and links to other places.

A link can be kept in two ways. In some cases we index its contents, and we could record when we indexed them, in case we want to search them. Other times we leave the link flat and traverse it on demand, when we need it.

We would also like to bring content in. For example, take the contents of a Google Doc, such as a transcript, and add it to pbrain converted to Markdown. That probably takes a combination of things, browser use among them.

## Prior art

- [qmd](https://github.com/tobi/qmd), by Tobi Lütke of Shopify.
- [beads](https://github.com/gastownhall/beads), from Gas Town.
- The Hermes agent, which has interesting learning and context features that may help us.

## Where it may grow

We start with a Markdown backend. Later we might expand into SQLite, some remote store or an API, and get more sophisticated. But I love the idea of keeping it really simple as Markdown files and Git, and qmd would let us query that data more easily, so the two are a good match.

## The open design question

I'm not sure whether a graph structure or flat files is the better idea. That is what I want to design here.

## From the issue ([pkieltyka/factory#62](https://github.com/pkieltyka/factory/issues/62))

**The owner's questions.** How a project manages its knowledge: notes, specs and context. Open questions: the backend (Markdown in Git, beads, or something else); querying and searching it (for example qmd); tools that import knowledge from the web and other sources into one normalized format (Markdown); and a privacy level per piece of knowledge (personal, or shared with the owner's team of human colleagues, and so on). A dedicated CLI for project knowledge, named something like `memory`, `mem` or `brain`.

**Owner decision (2026-11-19).** One brain per project to start.

**Recommendations from factory's architect and lead-dev** (a starting point for the spec, not decisions):

- *Users.* Agents on a context budget: sessions start near 40,000 tokens and cache reads are 97% of all tokens, so whatever pbrain returns is paid for on every later response. Return a few short cited results, or one item by id, never a dump. The operator needs the decisions in force, who made them and their limits. Writing roles need what a later task would otherwise get wrong. Read-only roles (architect, reviewer, product, designer) and pm propose note text in their reports. The owner adds outside material, confirms decisions and keeps private notes.
- *Today's factory notes convention* (`skill/references/notes.md` in factory). One Markdown file per note with front matter (`created`, `author`, `source`, `status`: provisional, verified or superseded-by) and a kind (observation, learning, decision). A decision or verified learning binds until an explicit newer one replaces it, and freshness alone does not replace it. No note is an instruction. A private folder is ignored by Git and nothing from it leaves the machine.
- *What the spec should settle:*
  1. Source of truth: Markdown with front matter in the project's Git repo. Indexes are derived, rebuildable and uncommitted. Existing notes are adopted unchanged.
  2. No graph database. A few typed links in front matter (`supersedes`, `source`, `relates`), and decisions as a first-class kind that records who decided.
  3. Explicit supersession and authority first, recency only as a tie-break. Superseded items are hidden by default, with history on request.
  4. Privacy by location, not by a field. Shared output never reveals a private id, file name, link or text. Refuse to run if the private directory is tracked or not ignored. Processing private material locally is a separate question from sending it to a remote model.
  5. Ingestion: an explicit `add` of a URL or file writes one normalized item with its source, fetch time and content hash, and keeps the original in a cache. It is reference, never a decision, and data, never instructions.
  6. One CLI for agents and people: bounded search with JSON output, and an exact get. Lexical search first; semantic search optional, later.
  7. Agents propose; whoever has the authority decides; the item records who.
  8. Factory integration is opt-in, through a recipe, the brief's Notes section and pbrain's skill. No new role, and factory works without pbrain.
  9. Tasks stay in the tracker and specs in spec files; pbrain links to them.
- *On prior art.* beads is a tracker first, so it is not the backend; borrow its budgeted session-start priming and typed links. Dolt solves versioned structured data, but pbrain's data is prose that Git already versions. qmd is an optional reader over pbrain's files, not a dependency.
- *Suggested first milestone.* A local Markdown store in factory's note format, with `add`, bounded `search` and `get`, supersession read from metadata, and an opt-in skill and recipe. It is accepted on a fixture of a decision superseded by its successor, a newer provisional note that disagrees, and a private note holding a unique term:
  - An agent asking why the current position holds gets the successor, who decided and the source, within the result budget.
  - History finds the older item.
  - No shared search, get or index reveals the private note.
  - Rebuilding the index gives the same answers.
- *Not in the first milestone:* importers, embeddings, graph views, more sharing levels, MCP, any UI.
- *Still open for the owner:* whether import caches are tracked, and whether any audience narrower than the repo's readers is needed.

**From the Hermes study** ([#89](https://github.com/pkieltyka/factory/issues/89#issuecomment-7010508618)), to adapt for pbrain: a usage signal (which knowledge is actually retrieved), scanning content at write time, and injecting a frozen, size-bounded slice per session.

**Approach.** Spec-driven, with research and thinking to design and architect the best solution.
