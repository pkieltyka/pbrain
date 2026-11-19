# Research Plan: Browser use and permission-gated sources for pbrain import

## Goal

Decide how pbrain's import can get content that a plain public fetch cannot: pages behind a sign-in (Claude artifacts, Google Docs, Slack, internal tools) and JavaScript-rendered pages. The owner's idea: pbrain stays small and does not sign in itself; instead `pbrain skill` gains topics (e.g. `pbrain skill browser-use`) that teach an agent to use a browser over CDP, ideally the person's existing Chrome session, or a harness's own reader, to save the content locally and then `pbrain import <file> --source <url>`. Feeds a functional spec addition (skill topics, import hand-off) and the architecture step.

## Run

- Researchers (factory members): @lead-dev (gpt-6.2-sol) and @lead-dev-2 (claude-opus-5-5). @architect writes the cross-subtopic summary.
- Web tools: each researcher's own web search and fetch; `gh` for repositories.

## Subtopics

- [x] Browser automation over CDP (@lead-dev) — driving Chrome, ideally the person's existing session, to save rendered or signed-in pages
- [x] Harness-native readers and skill topics (@lead-dev-2) — what Claude Code, Codex, agy and others can already read and save, and how `pbrain skill <topic>` should hand off

## Focus Details

### Browser automation over CDP

How an agent on the person's machine can drive Chrome to read a page the person can see, and save it as HTML or Markdown for `pbrain import`. Cover: attaching to the person's existing Chrome session vs a separate profile (remote debugging port, Chrome's recent restrictions on remote debugging of the default profile and any new opt-in flow, `chrome-devtools-mcp` options such as connecting to a running browser, Playwright `connectOverCDP`); the `agent-browser` CLI (what it is, install footprint, how agents use it, maturity); the "Browse" browser extension from the anomaly / OpenCode team (what it is, how agents drive it, maturity, platforms); Claude in Chrome and similar extensions only as comparison points; extracting main content from the rendered DOM to Markdown (readability, defuddle, others) and how that behaves on app-like pages such as Claude artifacts and Google Docs; security of exposing a signed-in browser to an agent (cookie and session exposure, prompt injection from page content, scoping to one tab or origin). Conclude: the most portable recommendation for "everyone has Chrome", the setup steps an agent would follow, and what to avoid. Out of scope: harness-native artifact or Docs readers and pbrain's skill command design (the other subtopic).

### Harness-native readers and skill topics

What the agent harnesses factory runs (Claude Code, Codex, Antigravity CLI `agy`, pi, OpenCode) can already read and save from permission-gated sources without a browser: Claude Code's artifact tool (what it can read and save, including multi-file artifacts), any Codex equivalent (ChatGPT canvases, connectors, MCP), agy and Gemini tooling for Google Docs and Drive (Workspace access, export as Markdown), Google Workspace or Drive MCP servers and CLIs that export Docs as Markdown, and Slack export routes. For each: auth model, what lands on disk, and how an agent would then run `pbrain import <file> --source <url>`. Then the skill side: how CLIs ship agent instructions via a `skill` command (look at the `todos` CLI's `todos skill` on this machine, `todos skill --help`, and comparable tools), whether `pbrain skill <topic>` subcommands (e.g. `browser-use`, `claude-artifacts`, `google-docs`) are a good pattern, how big each topic should be, and how the skill should pick a route based on what the agent's harness has. Conclude with a recommended set of skill topics and their outline. Out of scope: CDP and browser extension internals (the other subtopic).
