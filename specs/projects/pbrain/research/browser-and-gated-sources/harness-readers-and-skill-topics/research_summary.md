# Harness-native readers and skill topics

## Bottom Line

No harness gives one universal reader for permission-gated sources, but every source has at least one route that ends in a faithful local file and then `pbrain import <file> --source <canonical URL>`. Claude artifacts are readable natively only in Claude Code: the Artifact tool saves an artifact's published files locally, and a shared artifact's full source is saved alongside the summary. Google Docs and Slack have harness-independent routes that any harness with MCP or a shell can use: Google's own Drive MCP server and the `gws` and `rclone` CLIs export Docs as `text/markdown`, and Slack's MCP server exports canvases as Markdown and reads channels and threads. The routes to avoid are the "summary" readers (WebFetch, Drive's `read_file_content`), whose text is a paraphrase or changes format over time. Recommendation: keep the 400-word core skill and add a small embedded catalogue. `pbrain skill list`, `import`, `claude-artifacts`, `google-docs`, `slack` and `browser-use` each run about 300 words, are named by **source**, and route by **capability** (native reader → MCP export → CLI → browser → ask the person) rather than by detecting the harness.

## Key Findings

- **Claude Code's Artifact tool saves files**: `list scope:"files"` then `read path|paths` saves each published file locally, by default in the artifact's scratchpad folder, without approval. For an artifact someone else wrote, "Claude Code also saves the page's full source to a local file", while the model sees only a WebFetch-style summary ([artifacts docs](https://code.claude.com/docs/en/artifacts)). The import must use the saved file, never the summary.
- **Claude Docs links are a different route**: the connector's `export` gives `markdown` as base64. A pbrain topic must tell artifact pages and docs apart.
- **Codex CLI has no browser** ("Browser isn't available in Codex CLI"), but it does have `/plugins` (Google Drive, Slack) and MCP with OAuth ([plugins](https://developers.openai.com/codex/plugins), [MCP](https://learn.chatgpt.com/docs/extend/mcp)). Whether its Drive plugin exports Markdown to a file is undocumented.
- **agy uses Google's Drive MCP** (documented for "Antigravity 2.0, Antigravity IDE, and Antigravity CLI"), which needs your own Cloud project and an OAuth client marked "Use this client for an AI-powered agent" ([setup](https://developers.google.com/workspace/drive/api/guides/configure-mcp-server)). pi and OpenCode have no gated reader of their own (OpenCode's `webfetch` is public-only), so they rely on MCP or a CLI.
- **Drive export to Markdown is first-class**: Docs → `text/markdown` (.md) in `files.export`, capped at 10 MB ([formats](https://developers.google.com/workspace/drive/api/guides/ref-export-formats)). Through MCP, use `download_file_content` with `exportMimeType: "text/markdown"` (base64), not `read_file_content` ("The text representation will change over time"). `gws` (Apache-2.0, ~32k stars) and `rclone --drive-export-formats md` export to a file without the bytes passing through the model.
- **Slack's MCP server** reads channels and threads and "export[s] canvases as markdown files", but "Only directory-published apps or internal apps may use MCP" ([Slack MCP](https://docs.slack.dev/ai/slack-mcp-server)). `slackdump` makes exports "without admin privileges", in JSON or SQLite, convertible to HTML. Channel and thread messages need rendering to a Markdown transcript before import; pbrain should fix that format.
- **Shipping skills from the binary is the norm**: `todos skill` (one ~2,100-word SKILL.md), `agent-browser skills list/get <topic> [--full|--ref]` (10 topics; "always version-matched … rather than relying on cached copies"), `qmd skill show/install` + `skills list/get`. The Agent Skills spec gives ~100 tokens of metadata, an instructions body under 5,000 tokens and under 500 lines, and references one level deep. qmd's skill was missing in this machine's build, which argues for embedding topics as pbrain does its core.
- **Harness detection is not a sound router**: Claude Code sets `CLAUDECODE=1` and `AI_AGENT` (seen here). I found no documented equivalent for the others, and it would not reveal attached connectors anyway. The agent knows its tools, so topics list capabilities in order.

## Details

- [Harness readers](./harness-readers.md): per harness (Claude Code, Codex, agy, pi, OpenCode), what it can read, its auth, what lands on disk and the exact hand-off; summary matrix by source.
- [Google Docs and Slack routes](./google-docs-and-slack-routes.md): Drive `files.export`, the Drive MCP tools, `gws`, rclone, the Gemini Workspace extension; Slack MCP, slackdump; size and safety notes.
- [Skill topics design](./skill-topics-design.md): how todos, agent-browser, qmd and gws ship skills; the Agent Skills budget; why topics, route order, the recommended topic set with sizes, a topic template, and spec questions.

## Open Questions / Gaps

- I did not run any export or read end to end: no Google, Slack or Codex account was signed in, by instruction. The `gws` and `rclone` command lines are inferred from their docs and generated surfaces.
- Not established whether reading (as opposed to publishing) artifacts needs a claude.ai login, or works with an API-key session.
- Codex's Google Drive and Slack plugin tool lists, and whether they can save Markdown to a file: no documentation found. There is no route for ChatGPT canvases except a browser or copy and paste.
- The Gemini CLI Workspace extension's Markdown export, and its use from agy through `agy plugin import`: not verified.
- `qmd skill show` failed in this Nix build, so qmd's skill content and size were not measured.
- No documented harness-identifying environment variables for Codex, agy, pi or OpenCode were found.
- For the spec: the `--source` form for extra artifact files, whether `--personal` should be the default suggestion for gated imports, and a fixed Slack transcript format.

## Sources

- [Claude Code: Share session output as artifacts](https://code.claude.com/docs/en/artifacts): reading shared artifacts, page constraints, availability; fetched 2026-11-19. Also this session's `Artifact` and `claude.ai Claude Docs` tool schemas.
- [Google Drive export MIME types](https://developers.google.com/workspace/drive/api/guides/ref-export-formats) and [files.export](https://developers.google.com/workspace/drive/api/reference/rest/v3/files/export): `text/markdown`, 10 MB cap; updated 2026-11
- [Drive MCP reference](https://developers.google.com/workspace/drive/api/reference/mcp), [read_file_content](https://developers.google.com/workspace/drive/api/reference/mcp/tools_list/read_file_content), [download_file_content](https://developers.google.com/workspace/drive/api/reference/mcp/tools_list/download_file_content), [Configure the Drive MCP server](https://developers.google.com/workspace/drive/api/guides/configure-mcp-server): tools, auth, Antigravity and Claude setup
- [googleworkspace/cli README](https://github.com/googleworkspace/cli) and its `gws-shared`, `gws-drive`, `gws-docs` skills: auth, `-o`, `--sanitize`, skills; v0.22.5
- [rclone Google Drive](https://rclone.org/drive/): `--drive-export-formats md`
- [Gemini CLI Workspace extension](https://github.com/gemini-cli-extensions/workspace): tools, headless auth
- [Antigravity CLI README](https://github.com/google-antigravity/antigravity-cli): auth; `agy --help` (1.5.12) on this machine
- [Codex plugins](https://developers.openai.com/codex/plugins), [MCP](https://learn.chatgpt.com/docs/extend/mcp), [Browser](https://learn.chatgpt.com/docs/browser): CLI plugins, MCP OAuth, no browser in CLI; `codex features list` (0.170.0) on this machine
- [OpenCode tools](https://opencode.ai/docs/tools/): webfetch; [pi](https://github.com/badlogic/pi-mono): `pi --help` (1.5.2)
- [Slack MCP server](https://docs.slack.dev/ai/slack-mcp-server): tools, app restrictions, rate limits
- [slackdump](https://github.com/rusq/slackdump): export without admin, formats, MCP; AGPL-3.0
- [Agent Skills specification](https://agentskills.io/specification): metadata, instructions and resources budget
- Local: `todos skill` / `todos skill --help`; `agent-browser skills --help`, `skills list`, `skills get core|slack` (0.43.0); `qmd skill --help` (2.8.3)
