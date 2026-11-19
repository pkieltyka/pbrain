# Research: Browser use and permission-gated sources for pbrain import

## Bottom Line

pbrain does not need a browser, a sign-in or a connector of its own. Every gated source studied has at least one route that ends in a faithful local file, and the functional spec already has the hand-off: `pbrain import <file> --source <url>`. What is missing is guidance, so the owner's idea holds: ship a small catalogue of skill topics in the binary, named by source, each listing routes in order of capability. A source's own export beats any capture: Google Docs exports Markdown natively, and Claude Code's Artifact tool saves an artifact's real files. Browser automation is the fallback for what remains. It works through Chrome's user-approved attachment (Chrome 144 and later), but it exposes the whole browser profile to the agent, and generic content extractors fail on app-like pages. Readers that return a summary must never be imported. Nothing here was tested against a live signed-in account, so the routes are documented behaviour, not verified behaviour.

## Key Findings

- **A native export exists for the main sources.** Drive exports a Doc as `text/markdown` through its API, its MCP server (`download_file_content`), and the `gws` and `rclone` CLIs. Slack's MCP server exports canvases as Markdown and reads channels and threads. Claude Code's Artifact tool saves an artifact's published files locally. ([Harness-native readers and skill topics](./harness-readers-and-skill-topics/research_summary.md))
- **Summary readers are the trap.** For an artifact someone else wrote, Claude Code shows the model a summary while saving the full source to a file. Drive's `read_file_content` returns a text representation that "will change over time", and WebFetch paraphrases. The import must use the saved file. ([Harness-native readers and skill topics](./harness-readers-and-skill-topics/research_summary.md))
- **No harness has a universal reader.** Only Claude Code reads Claude artifacts natively. Codex CLI has no browser, and its Drive and Slack plugins are undocumented. agy uses Google's Drive MCP, which needs the person's own Cloud project. pi and OpenCode rely on MCP or a CLI. ([Harness-native readers and skill topics](./harness-readers-and-skill-topics/research_summary.md))
- **Route by capability, not by harness.** Only Claude Code was seen to set an identifying variable, and no variable would reveal which connectors are attached. The agent knows its own tools, so a topic lists routes in order and the agent takes the first it has. ([Harness-native readers and skill topics](./harness-readers-and-skill-topics/research_summary.md))
- **Topics shipped from the binary are the norm.** `todos`, `agent-browser` and `qmd` all print version-matched skills, and agent-browser serves a catalogue of topics. qmd's skill was missing from this machine's build, which argues for embedding. The proposal is a 400-word core plus topics of about 300 words, loaded only when needed. ([Harness-native readers and skill topics](./harness-readers-and-skill-topics/research_summary.md))
- **Attaching to the person's Chrome is supported but broad.** Chrome 136 disabled the old debugging switches on the default profile; Chrome 144 added an in-browser consent flow, which Chrome DevTools MCP uses with `--autoConnect`. That is the most portable route. It grants access to the whole selected profile, so a dedicated signed-in profile is safer for repeated use. ([Browser automation over CDP](./browser-automation-cdp/research_summary.md))
- **Other browser tools are optional.** Agent Browser is a shell-first CLI whose domain allowlist does not work when attached to an existing browser. OpenCode Browse lends one tab at a time through an extension, and is a young tool. ([Browser automation over CDP](./browser-automation-cdp/research_summary.md))
- **Generic extraction fails on app pages.** In offline fixtures, Readability and Defuddle returned the chat text from a conversation shell and toolbar text from a canvas editor, and returned the artifact only when it was supplied as a scoped article. Google Docs renders to a canvas, so its export is the only reliable route. ([Browser automation over CDP](./browser-automation-cdp/research_summary.md))

## Implications

- **The CLI needs almost nothing.** No browser, CDP, OAuth or connector code enters pbrain. Local HTML import, added for artifacts, already covers a saved page.
- **The skill needs topics.** `pbrain skill <topic>` and a topic list, outside the core's 400-word limit, with one line in the core pointing to them.
- **pbrain's HTML floor is not a scope check.** A long conversation shell clears 200 characters. Whoever captures a page must scope it to the document first; the floor only catches empty shells.
- **Audience matters more for gated sources.** Both studies recommend `--personal` unless the content is meant for everyone with the repository. The spec's default is shared, with a "consider" hint.
- **Two formats are undefined:** what `--source` an artifact's extra files carry, and how a Slack thread is written as Markdown.

## Conflicts and Uncertainty

- **Where the person comes in the route order.** The skill study puts "ask the person to export" last, after the browser. The browser study puts the site's own download or copy first and automation after. The evidence leans to the person's export before a browser capture for Google Docs and artifacts, where capture is unreliable and attachment exposes the profile, and to the browser first only for an ordinary rendered page.
- **Topic names.** Topics are named by source, but `browser-use` is a capability. It works as the shared fallback that the source topics point to.
- **Everything moves fast.** Chrome's attachment rules changed twice in two years; DevTools MCP, Agent Browser and Browse all released within the last two weeks. Topics that name tools will drift, which is the argument for keeping them short and linking to vendor docs.

## Gaps

- **No live signed-in test.** No Google, Slack, Claude or Codex account was used, and no browser was launched. Closing it needs one end-to-end run per route on a real account.
- **Fidelity unmeasured** for a live Claude artifact and a live Google Doc, including multi-file artifacts. Needs real exports compared with their sources.
- **Commands taken from documentation**: the `gws` and `rclone` lines were not run.
- **Undocumented**: Codex's Drive and Slack plugin tools; whether reading an artifact needs a claude.ai login; agy's use of the Workspace extension.
- **No route found** for ChatGPT canvases other than a browser or copy and paste.
- **Not tested**: Browse's extension pairing, the Chrome consent dialog with several profiles, and managed-device restrictions.

## Subtopics

- [Browser automation over CDP](./browser-automation-cdp/research_summary.md): attaching to Chrome, the agent browser tools, extraction quality and the security of a signed-in browser. Prefer the site's export; use DevTools MCP when automation is needed.
- [Harness-native readers and skill topics](./harness-readers-and-skill-topics/research_summary.md): what each harness can read and save, the Docs and Slack export routes, and how CLIs ship skills. Recommends source-named topics that route by capability.
