# Routes for Google Docs and Slack that don't need a browser

Researched 2026-11-19. These routes work from any harness that has a shell or MCP. "Inference:" marks my reading. Nothing was installed or signed in.

## Google Docs and Drive

### The underlying API: Drive `files.export` to `text/markdown`

Google's export table maps **Documents → "Markdown | text/markdown | .md"** ([Export MIME types](https://developers.google.com/workspace/drive/api/guides/ref-export-formats)). `files.export` "Exports a Google Workspace document to the requested MIME type and returns exported byte content … Note that the exported content is limited to 10 MB" ([files.export](https://developers.google.com/workspace/drive/api/reference/rest/v3/files/export)). Every route below is a wrapper around this call, so this is the faithful format to ask for. The 10 MB cap is above pbrain's 5 MiB file limit, so a very large Doc can export successfully and still be refused by `import` with `too_large`.

### Google's remote Drive MCP server (`drivemcp.googleapis.com`)

- **Tools** ([MCP reference](https://developers.google.com/workspace/drive/api/reference/mcp)): `search_files`, `list_recent_files`, `get_file_metadata`, `get_file_permissions`, `read_file_content`, `download_file_content`, `create_file`, `copy_file`.
- **`read_file_content`** returns "a natural language representation of a Drive file, and optionally, its comments … The file content may be incomplete for very large files. The text representation will change over time, so don't make assumptions about the particular format" ([tool page](https://developers.google.com/workspace/drive/api/reference/mcp/tools_list/read_file_content)). Not a faithful export: avoid it for import.
- **`download_file_content`** returns "the content of a Drive file as a base64 encoded string. If the file is a Google Drive first-party mime type, the exportMimeType field is required … Defaults to text if not specified" ([tool page](https://developers.google.com/workspace/drive/api/reference/mcp/tools_list/download_file_content)). **Use `exportMimeType: "text/markdown"`**, base64-decode to `<name>.md`, then import. Inference: the base64 string passes through the model's context, about 1.33× the file size in tokens of opaque text. That is costly for big Docs; a CLI route keeps the bytes out of context.
- **Auth**: your own Google Cloud project with `drive.googleapis.com` and `drivemcp.googleapis.com` enabled, an OAuth consent screen with `drive.readonly` (and `drive.file`), and an OAuth client marked "Use this client for an AI-powered agent". The setup page documents Antigravity (2.0, IDE and CLI) and Claude.ai / Claude Desktop custom connectors ([Configure the Drive MCP server](https://developers.google.com/workspace/drive/api/guides/configure-mcp-server)). In Claude Code, the "claude.ai Google Drive" connector points at this same server (seen in this session's tool list).

### `gws`: Google Workspace CLI (googleworkspace/cli)

- Apache-2.0, about 32k stars, created 2026-03-02, last push 2026-11-03, latest release v0.22.5 (2026-03-31) ([repo](https://github.com/googleworkspace/cli)). README: "One CLI for all of Google Workspace — built for humans and AI agents … Structured JSON output." It is generated from Google's Discovery documents. Install from a release binary, `npm install -g @googleworkspace/cli`, cargo or Homebrew.
- **Auth**: `gws auth setup` (needs `gcloud`, creates a project) or manual OAuth; `GOOGLE_WORKSPACE_CLI_TOKEN=$(gcloud auth print-access-token)`; or service accounts. Unverified apps in testing mode are limited "to ~25 scopes".
- **Export** (inference from the generated surface and `gws-shared` skill flags: `-o, --output <PATH>` "Save binary responses to file"):
  ```sh
  gws drive files export --params '{"fileId":"<id>","mimeType":"text/markdown"}' -o doc.md
  pbrain import doc.md --source https://docs.google.com/document/d/<id>/edit
  ```
  I did not run this. The exact flag spelling should be checked against `gws schema drive.files.export`.
- **Prompt-injection screening**: `--sanitize <template>` runs responses through Google Cloud Model Armor, mode `warn` or `block`. It ships "100+ Agent Skills (`SKILL.md` files)", installed with `npx skills add`.

### rclone

`rclone` (v1.76.0 installed here) exports Docs on copy according to `--drive-export-formats`, whose table includes "`md` | `text/markdown` | Markdown Text Format" ([rclone Drive](https://rclone.org/drive/)). A one-time `rclone config` OAuth is needed. Inference: `rclone copyto "drive:Path/Doc" doc.md --drive-export-formats md` gives a file; untested.

### Gemini CLI Workspace extension

Docs, Sheets, Slides, Gmail, Chat, Calendar and Drive tools for Gemini CLI, with a headless login whose "Credentials are read securely from `/dev/tty` and are never exposed to the AI model" ([README](https://raw.githubusercontent.com/gemini-cli-extensions/workspace/main/README.md)). I did not confirm whether it exports a Doc as Markdown to a file.

### Meet transcripts and Gemini notes

These arrive as Google Docs ("A Google Doc or meeting transcript is exported as Markdown, then imported", spec 8.4), so the Docs routes cover them.

## Slack

### Slack's official MCP server

- Tools ([Slack MCP server](https://docs.slack.dev/ai/slack-mcp-server)): search messages, files, channels and users; "Read channels — grab the complete message history of channels"; "Read threads — grab complete message thread conversations"; "Read a canvas — export canvases as markdown files"; and others.
- **Constraint**: "Only directory-published apps or internal apps may use MCP … unlisted apps are prohibited". "MCP clients must be backed by a registered Slack app with a fixed app ID". Confidential OAuth with `client_id` and `client_secret`, admin approval. Rate limits are per tool (for example Tier 3 for reading a channel or thread).
- **Hand-off**: a canvas comes back as Markdown, so write it to `.md` and import with the canvas URL. A channel or thread comes back as messages, which the agent must render to a `.md` transcript (author, time, text) and then import with the thread permalink as `--source`. Inference: that rendering step should be specified in the skill topic (format: one line per message, `**name** (ISO time): text`). Otherwise each agent invents its own format and duplicate detection by digest stops meaning anything.

### slackdump (rusq/slackdump)

"archive your private and public Slack messages … Generate Slack Export without admin privileges" (AGPL-3.0, about 2.9k stars, pushed 2026-11-14) ([README](https://raw.githubusercontent.com/rusq/slackdump/master/README.md)). It logs in through its "EZ-Login 3000" browser flow. Outputs are a Slack Export (JSON), an archive (SQLite or JSON+gz), or `slackdump convert -f html` to "a browsable static HTML archive". It also has a read-only MCP server over its archives. Inference: pbrain cannot import JSON, so the routes are `convert -f html` (an HTML page per channel, `.html` import) or an agent rendering Markdown from the archive. Its login drives the person's session (a browser-type risk), and AGPL licensing applies to whoever ships it, not to pbrain calling it.

### Browser route

`agent-browser` ships a `slack` skill ("Interact with Slack workspaces using browser automation … `agent-browser connect 9222`"). That belongs to the CDP subtopic.

## Size and safety notes for any route

- Imports must stay ≤5 MiB and ≤512,000 characters (spec 8.4). Long Slack channels need a date range or one thread per item.
- All gated content is untrusted (the "data, not instructions" rule). `gws --sanitize` and the Claude artifact read's instruction-reporting summary are mitigations at fetch time; pbrain's own rules still apply on import.
- Gated content is often not for everyone with the repo. Spec 12 already says "consider `import --personal`". Inference: for Slack DMs, private channels and Claude artifacts shared to the person, the topic should make `--personal` the default suggestion and shared the deliberate choice.
