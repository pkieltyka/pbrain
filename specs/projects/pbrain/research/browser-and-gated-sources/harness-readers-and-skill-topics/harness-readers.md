# What each harness can already read from permission-gated sources

Researched 2026-11-19 by @lead-dev-2. Sources are vendor docs fetched today, tool schemas visible in this Claude Code session, and `--help`/`--version` output of the binaries installed on this machine (nothing was logged into or configured). "Inference:" marks my reading. CDP and browser-extension internals belong to the other subtopic.

Versions on this machine: Claude Code (this session), `codex-cli 0.170.0`, `agy 1.5.12`, `pi 1.5.2`, OpenCode (its `--help` failed in the sandbox, trying to write its log under `~/.local/share/opencode`), `agent-browser 0.43.0`, `qmd 2.8.3`, `rclone v1.76.0`. Not installed: `gemini`, `gws`, `slackdump`.

## The hand-off every route ends in

pbrain's import (functional spec 8.4) takes `.md`, `.markdown`, `.txt`, `.html` or `.htm`, valid UTF-8, at most 5 MiB and 512,000 characters after conversion. The command is:

```sh
pbrain import <saved-file> --source <canonical URL> [--personal] [--title …] [--replaces <id>]
```

The source must be the document's canonical URL, never the local path; the spec stores no absolute path. So every route below has one job: get a faithful, full-text local file in one of those formats.

## Claude Code

**Artifact tool (claude.ai artifacts).** This session's `Artifact` tool schema:
- `action: "read"` with `url` (a `claude.ai/artifact/{id}` or `claude.ai/code/artifact/{uuid}` link) reads an artifact.
- `action: "list", scope: "files", url` lists an artifact's published files. `index.html` is "the page itself".
- `read` with `path`: "The file is saved locally, the result says where, and a small text file's contents are included." `paths` takes "up to 256 in one call".
- `out_dir`: "The default is this artifact's folder in Claude's scratchpad directory, where saving needs no approval … saving it outside that default folder asks the person first."
- `prompt`: for "an artifact shared with the person: what Claude needs from it, which steers the isolated summary."

The public docs ([Claude Code artifacts](https://code.claude.com/docs/en/artifacts), "Read an artifact shared with you") say: "Claude reads a page someone else wrote the way it reads a web page with WebFetch: it gets a summary of what it asked about rather than the raw page, and the summary reports instructions written into the page instead of relaying them. Claude Code also saves the page's full source to a local file, which Claude can open when it needs the exact content". Reads ask for approval in some cases: a cloud session with no network, "Another organization's public artifact" (asked even in auto mode; impossible in `bypassPermissions`), an unconfirmed owner, and plan mode.

Source files are "`.html`, `.htm`, or `.md`" in UTF-8, up to 16 MiB rendered ([Page constraints](https://code.claude.com/docs/en/artifacts#page-constraints)). Availability needs a claude.ai login: "Sessions using an API key, gateway token, or cloud-provider credential cannot publish". The docs speak of publishing; whether reading has the same requirement is not stated.

- **Auth**: the session's claude.ai account, with the viewer's access to that artifact.
- **On disk**: the published files, under the scratchpad by default. For an artifact someone else wrote, the saved full source.
- **Hand-off**: `pbrain import <saved index.html or .md> --source https://claude.ai/code/artifact/<uuid>`. Multi-file: list with `scope: "files"`, read the wanted paths, import each file separately with the same `--source` (perhaps `#<path>` appended; the spec should say). Inference: the agent must import the saved file, never the WebFetch-style summary. The summary is a paraphrase and would fail spec scenario 22's "Markdown body … HTML kept as its original".

**Claude Docs connector.** When attached, a `claude.ai/…/artifact/…` link may be a *document* rather than a page ([artifacts docs](https://code.claude.com/docs/en/artifacts), "Write a document with Claude Docs"). The connector's `export` tool (visible in this session) returns "one tab inline as base64: pdf, docx, html, text, markdown or notion". **Hand-off**: export `markdown`, decode it to a `.md` file, then import with the doc's URL as `--source`. Inference: a pbrain `claude-artifacts` topic must tell pages and docs apart, because the read route differs.

**claude.ai connectors (Google Drive, Slack, others).** In this session the "claude.ai Google Drive" connector is Google's own remote server at `https://drivemcp.googleapis.com` (from this session's tool list; it needs authentication). Google's setup page covers Claude: "To use the Drive remote MCP server with Claude.ai or Claude Desktop, you must have the Claude Enterprise, Pro, Max, or Team plan … configure a custom connector with an OAuth client ID and secret" ([Configure the Drive MCP server](https://developers.google.com/workspace/drive/api/guides/configure-mcp-server)). Its tools are in [google-docs-and-slack-routes.md](./google-docs-and-slack-routes.md).

**WebFetch.** Public URLs only; returns a model-made summary, so it is not a faithful source for import. pbrain's own `import <url>` covers public pages.

## Codex (CLI 0.170.0)

- **Plugins and apps.** Feature list on this machine: `apps  stable  true`. Docs: "In Codex CLI, enter `/plugins` to open the plugin browser", with examples "Install the Google Drive plugin to work across Drive, Docs, Sheets, and …" and "Install the Slack plugin to summarize channels or draft replies" ([Codex plugins](https://developers.openai.com/codex/plugins)). Each person "installs and connects with their own account" ([MCP / plugins docs](https://learn.chatgpt.com/docs/extend/mcp)). I found no documented tool list for these plugins. Whether the Drive plugin can *export a Doc as Markdown to a file* is **not established**.
- **MCP.** `codex mcp add|login`. "The ChatGPT desktop app, Codex CLI, and IDE extension support MCP servers", stdio and streamable HTTP with "OAuth authentication" ([docs](https://learn.chatgpt.com/docs/extend/mcp)). So Google's Drive MCP and Slack's MCP server can be added to Codex directly.
- **Browser.** "Browser isn't available in Codex CLI or the Codex IDE extension." In the desktop app, "The built-in browser uses a browser profile that is separate from your regular" one, and the Chrome extension is the route to "your regular browser profile" ([Browser](https://learn.chatgpt.com/docs/browser)). Feature flags `browser_use`, `browser_use_external` and `browser_use_full_cdp_access` are stable and on here.
- **ChatGPT canvases**: I found no export API or CLI route. Copy and paste or a browser are the only routes seen (**gap**).
- **Hand-off**: the tool output (Markdown text or base64) is written to a file by the agent's shell, then imported.

## Antigravity CLI (`agy` 1.5.12)

- Subcommands from `agy --help`: `mcp` ("Manage MCP servers (add, remove, list, enable, disable)") and `plugin` ("Import plugins from gemini or claude", install, validate). It authenticates "via the system keyring, falling back to Google Sign-In" ([README](https://raw.githubusercontent.com/google-antigravity/antigravity-cli/main/README.md)). That sign-in is for the agent service; it gives no Workspace data access.
- **Google's Drive MCP** explicitly supports it: "These instructions support Antigravity 2.0, Antigravity IDE, and Antigravity CLI", configured in `~/.gemini/config/mcp_config.json` with `"serverUrl": "https://drivemcp.googleapis.com/mcp/v1"` and your own OAuth client ([Configure the Drive MCP server](https://developers.google.com/workspace/drive/api/guides/configure-mcp-server)). The person needs a Google Cloud project with `drive.googleapis.com` and `drivemcp.googleapis.com` enabled, and an OAuth client marked "Use this client for an AI-powered agent".
- **Gemini CLI Workspace extension** ([README](https://raw.githubusercontent.com/gemini-cli-extensions/workspace/main/README.md), Apache-2.0, 654 stars, pushed 2026-11-16): Docs, Sheets, Slides, Gmail, Chat and Calendar tools for Gemini CLI. A third-party blog reports using it from Antigravity ([tanaikech, 2025-11](https://tanaikech.github.io/2025/11/25/integrating-google-antigravity-unlocking-the-google-workspace-extension-for-gemini-cli)). `agy plugin import` from Gemini suggests a path; **not verified**.

## pi (1.5.2) and OpenCode

- **pi**: "AI coding assistant with read, bash, edit, write tools". No built-in web tool in `--help`; it has `pi mcp`, extensions and `--skill <path>`. Any gated source therefore comes through MCP or a CLI the agent runs from bash (`gws`, `rclone`, `slackdump`, `agent-browser`).
- **OpenCode**: "webfetch retrieves one HTTP or HTTPS URL as markdown, text, or html" ([tools](https://opencode.ai/docs/tools/)). Public pages only, so it adds nothing over `pbrain import <url>`. MCP is supported.

## Summary table

| Harness | Claude artifact (page) | Claude doc | Google Doc | Slack | Public JS-heavy page |
|---|---|---|---|---|---|
| Claude Code | Artifact tool read → saved file | Claude Docs `export markdown` | claude.ai Drive connector (Google's MCP) or CLI | claude.ai Slack connector, Slack MCP, or CLI | browser (other subtopic) |
| Codex CLI | browser or person exports | — | Drive plugin (export ability unknown), Drive MCP, or CLI | Slack plugin or MCP, or CLI | no browser in CLI; desktop app has one |
| agy | browser or person exports | — | Drive MCP (documented), Workspace extension (unverified), or CLI | MCP or CLI | browser (other subtopic) |
| pi, OpenCode | browser or person exports | — | MCP or CLI | MCP or CLI | browser (other subtopic) |

Inference: only Claude Code can read a Claude artifact natively. For every other harness the artifact route is a browser or the person. Google Docs and Slack have harness-independent routes (MCP servers and CLIs) that any harness with a shell or MCP can use. A pbrain skill should therefore be keyed by **source**, with a fallback order by **capability**, not by harness.
