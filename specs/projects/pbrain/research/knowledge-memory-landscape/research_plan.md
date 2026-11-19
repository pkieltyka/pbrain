# Research Plan: Knowledge and memory landscape for pbrain

## Goal

Ground the pbrain spec (project `specs/projects/pbrain`) in how existing tools actually work before the functional spec (Step 2) and architecture (Step 4). The decisions waiting on it: store layout and naming, graph vs flat files with typed links, how time/supersession/weighting are represented, query and search approach (qmd or own), ingestion of external sources into Markdown, and what an agent-facing skill should say. Starting recommendations to test, not repeat: pkieltyka/factory#62 (`gh issue view 62 -R pkieltyka/factory --comments`) and factory's notes convention at `/home/peter/Dev/pkieltyka/factory/skill/references/notes.md`.

## Run

- Researchers (factory members, not sub-agents): @lead-dev-2 (claude-opus-5-5) and @lead-dev (gpt-6.2-sol), per the owner. @architect (claude-fable-5-2), the spec writer, writes the cross-subtopic summary.
- Assignment: @lead-dev — qmd, beads and Gas Town, source ingestion. @lead-dev-2 — Markdown knowledge stores, agent memory systems.
- Web tools: WebSearch, WebFetch; `gh` for GitHub repos and issues.

## Subtopics

- [x] qmd — what qmd really does and how pbrain would sit beside it
- [x] beads and Gas Town — data model, memory and agent-priming ideas worth borrowing
- [x] Markdown knowledge stores — layouts, front matter, links and graph-vs-flat in Markdown-native tools
- [x] Agent memory systems — Hermes and others: memory types, staleness, supersession, weighting, retrieval budgets
- [x] Source ingestion — turning URLs, Google Docs and transcripts into normalized Markdown with provenance

## Focus Details

### qmd

github.com/tobi/qmd: its real current capabilities from the repo and docs, not hearsay. Collections and how it indexes a directory of Markdown; index storage (SQLite FTS5, vectors), what is derived vs committed; lexical, vector and hybrid/reranked search modes and which need local or remote models (size, download, latency, privacy of a remote endpoint); CLI and MCP surfaces, output formats (JSON, snippets, ids, budgets), get-by-id or path; front matter and metadata handling; filtering by folder or field; runtime and install footprint (Bun/Node etc.), license, maturity, release cadence. Conclude: can pbrain use qmd as an optional or default query engine over its files, what would pbrain still have to do itself (supersession filtering, privacy exclusion, result budgets), and what are the risks of depending on it. Out of scope: other search tools in general (only name close alternatives briefly), beads, memory theory.

### beads and Gas Town

github.com/gastownhall/beads (and the Gas Town project around it by Steve Yegge): the issue/memory data model, typed links and dependency graph, ready queue, how it primes an agent at session start (budget, format), "memory decay"/compaction of old items, storage (JSONL in Git, Dolt, SQLite — what it uses today and why it changed), the CLI shape agents call, and how agents are told to use it (its AGENTS/skill text). What worked and what users complain about. Conclude with concrete ideas pbrain should borrow and what to avoid, given pbrain is knowledge not a tracker. Out of scope: qmd, general Markdown note tools, general agent-memory research.

### Markdown knowledge stores

Markdown-native, Git-friendly knowledge tools and conventions: Basic Memory (Markdown + semantic links as a knowledge graph for agents), Obsidian/Foam/Dendron conventions (wikilinks, front matter, hierarchical vs flat naming), Architecture Decision Records (ADR, MADR, log4brains: status, supersedes), Zettelkasten-style ids, `nb` and similar CLIs, and how Claude Code / Codex / Cursor store project memory files (CLAUDE.md, AGENTS.md, memory directories). Questions: folder layouts and file naming that stay stable and merge cleanly in Git with many agent writers; id schemes; front matter fields commonly used (type, status, created/updated, supersedes, source, tags, confidence); typed links in front matter vs inline links; whether a derived graph is worth it versus flat files with links; scaling limits. Conclude with a recommended layout pattern and front matter schema shape, with trade-offs. Out of scope: vector search engines, agent memory retrieval theory, ingestion converters.

### Agent memory systems

How agent memory systems model and maintain memory: the Hermes agent (Nous Research) learning loop, skills/memory files, context injection (see pkieltyka/factory#89 comment https://github.com/pkieltyka/factory/issues/89#issuecomment-7010508618 for factory's prior study — build on it, don't redo it); Zep/Graphiti (bi-temporal facts, validity intervals, invalidation on contradiction); mem0; Letta/MemGPT (core vs archival memory); A-MEM and recent papers on memory for coding agents; Claude Code auto-memory and similar. Questions: memory types (fact, decision, learning, observation, preference, episodic), how staleness and contradiction are detected and represented (supersession, validity time, confidence, decay, usage/retrieval signals), who may write and how writes are reviewed or scanned (prompt injection, secrets), session-start priming vs on-demand retrieval, retrieval budgets and citation, evaluation methods. Conclude with which mechanisms suit a Markdown-in-Git store for a multi-agent team and which need a database. Out of scope: specific Markdown layout conventions, ingestion converters, qmd/beads internals.

### Source ingestion

Turning external material into normalized Markdown with provenance: URL-to-Markdown converters (Microsoft markitdown, Jina Reader, trafilatura, defuddle/readability, pandoc) — quality, dependencies, offline vs remote; Google Docs and Drive (export as Markdown via Drive API/`text/markdown` export, Google Meet transcripts and Gemini notes as Docs, auth options for a CLI: OAuth, gcloud, gws-style CLIs); authenticated or JS-heavy pages via browser automation (Playwright, agent browser tools) and when that is worth it; PDFs. Questions: what provenance to keep (source URL, fetch time, content hash, original cached copy), refresh/re-fetch and change detection, handling untrusted content (prompt injection, "data not instructions"), size limits and chunking, licensing/copyright of caching originals. Conclude with a minimal ingestion design for a first version and what to defer. Out of scope: search/indexing, memory models, note layout.
