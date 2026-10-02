# Client compatibility review

Checked October 2, 2026. Installed builds and public release notes are separate evidence; a protocol feature does not establish that every client implements it.

| Client or contract | Verified version | Evidence and status |
| --- | --- | --- |
| ChatGPT desktop, installed on this Mac | 26.930.21537, build 12776, production channel | Read from the app bundle; the app updater reported `up_to_date`. |
| ChatGPT desktop, latest exact build in public release notes | macOS 26.924.20706, September 25 | [Official changelog](https://learn.chatgpt.com/docs/changelog); the installed build is newer. |
| Codex CLI | Public stable 0.160.0, October 1; local CLI 0.159.0 | [Official changelog](https://learn.chatgpt.com/docs/changelog). No user-installed runtime was upgraded. |
| Claude Desktop, installed on this Mac | 2.19675.0 | Read from the app bundle. A newest downloadable binary version was not established by official release notes. |
| Claude Code | Public stable 2.1.287; local CLI 2.1.284 | [Official release](https://github.com/anthropics/claude-code/releases/tag/v2.1.287). |
| Claude Agent SDK, TypeScript | 0.3.287 | [Official release](https://github.com/anthropics/claude-agent-sdk-typescript/releases/tag/v0.3.287). The plugin consumes MCP and does not need to embed this SDK. |
| MCP published core | 2026-07-28 | [Latest specification](https://modelcontextprotocol.io/specification/latest), [key changes](https://modelcontextprotocol.io/specification/2026-07-28/changelog). |
| MCP implementation in this repository | Go SDK 1.8.0, latest stable September 14 | [Official releases](https://github.com/modelcontextprotocol/go-sdk/releases); pinned in `plugins/transcription-agent-cli/mcp-server/go.mod`, so no SDK migration was needed. |
| Portable plugin package | Agent Plugins 1.0.0 | [Published specification](https://agent-plugins.org/specification). Native package uses a plugin-relative launcher and root `mcp.json`. |

## Applied improvements

- **Claude Code:** the CLI plugin now registers MCP automatically, offers optional masked provider settings, and allows 31 minutes per call. Empty settings preserve existing environment credentials and saved keys. See [Claude setup](claude.md).
- **Codex and local ChatGPT desktop:** an optional `transcription-agent-mcp` package registers the installed CLI with a desktop-safe path resolver and a 31-minute call timeout. The existing `transcription-agent` package remains skills-only for its public listing. The compatibility configuration follows a relative-launcher pattern present in the installed ChatGPT app's bundled plugins.
- **MCP clients:** precise schemas, provider/dependency status, compact UTF-8 previews, structured execution failures, progress and cancellation, and issued transcript resources let agents recover without shell access or another billed transcription. Concurrent calls to one server using the same `output_path` are rejected; completed transcripts are preserved in a recovery file if another program takes the destination. Artifact registrations are bounded to the latest 1,024 outputs and cannot follow a replaced file into unrelated data. Supported core revisions start at 2025-06-18.
- **MCP Apps clients:** an inline transcript viewer adds search and copy controls. Text and structured results remain available when a client cannot render Apps. Browser checks and SDK tests exercise the implementation; actual rendering inside ChatGPT and Claude Desktop remains unverified.
- **Skills clients:** MCP is preferred when available, with explicit local-file requirements and timeout recovery. Transcript speech stays untrusted input to the agent.

## Opportunities that need another deployment or client proof

ChatGPT [Plugin Extensions](https://developers.openai.com/plugins/build/extensions) provide sidebar entrypoints, conversation panels, file viewers and editors, and forms. The inline viewer is the useful shared foundation. Registering a global file handler for every TXT or JSON file would also take over unrelated documents, so this plugin does not advertise a generic file handler. Dedicated subtitle editing and audio playback need format-specific design and real host checks.

Public ChatGPT MCP submission requires a [remote HTTPS endpoint](https://developers.openai.com/plugins/build/plugins), or arranged local MCP support. This plugin currently transcribes files on the machine running its stdio server. A hosted version needs an upload contract, authentication, private artifact storage, and a deployment destination before it can handle web/mobile attachments. No hosted service or public directory submission was created by this change.

[MCP Events](https://developers.openai.com/plugins/build/mcp-events) require webhook delivery and MCP 2.0 in ChatGPT; the upstream event contract is a draft. This local server has no webhook service or monitored source. Events are appropriate for a future hosted transcription queue rather than repeated local requests.

[MCP Tasks](https://modelcontextprotocol.io/extensions/tasks/overview) are an optional extension. Support must be verified in both the server SDK and consuming client before switching a transcription call to task handles. The current implementation retains ordinary progress and cancellation, plus client timeouts matched to the engine's 30-minute limit. It does not claim task-extension support.

The model used by ChatGPT or Claude to orchestrate tools is separate from the speech model selected by `transcriber-cli`. Changing an agent's model does not add a speech provider to the pinned engine. Provider APIs remain in the shared CLI, so new clients do not duplicate transcription logic.
