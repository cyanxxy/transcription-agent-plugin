# Claude integration

Checked October 2, 2026. The latest verified [Claude Code release](https://github.com/anthropics/claude-code/releases/tag/v2.1.287) is **2.1.287**, released October 1; the local validation used **2.1.284**. The latest verified [TypeScript Claude Agent SDK release](https://github.com/anthropics/claude-agent-sdk-typescript/releases/tag/v0.3.287) is **0.3.287**. These are separate from the Claude Desktop binary, whose latest version was not established from official sources.

## Claude Code

```bash
claude plugin marketplace add cyanxxy/transcription-agent-plugin
claude plugin install transcription-agent-cli@transcription-agent-tools
```

The CLI plugin installs the shared transcription skill and registers the local MCP server automatically. It provides `transcribe`, `auth_status`, and the command-line fallback through `transcriber-cli`. Go, git, FFmpeg, and network access are required; the MCP server and engine build on first use.

Optional provider keys entered in the plugin configuration are masked and kept in Claude's secure credential storage. They apply to MCP calls. Empty fields preserve exported provider variables and keys saved with `transcriber-cli auth set`; the command-line fallback uses those existing environment variables or saved keys. Plugin options never enter command arguments or skill content. See the official [manifest reference](https://code.claude.com/docs/en/plugins-reference) for configuration and directory metadata.

Use `/mcp` to check the server. On a slow first build, restart with a longer startup allowance:

```bash
MCP_TIMEOUT=120000 claude
```

Then use `/mcp reconnect all` if needed. The plugin sets a 31-minute tool timeout to leave room for the engine's 30-minute limit. Current Claude Code can background long MCP calls; wait for the existing call before retrying. See [MCP lifecycle and timeout guidance](https://code.claude.com/docs/en/mcp).

If you already registered this server manually, keep a single active registration to avoid duplicate tool listings. The skill-only `transcription-agent` plugin still needs a separately installed CLI or MCP server.

## Claude Desktop and Cowork

Use the release's MCPB extension for local Claude Desktop access. The `transcription-agent-cli` plugin contains `bin/`, so Claude's web and Cowork plugin installer rejects it. The shared skill-only package can be listed, but it still requires an execution environment with the CLI and credentials. Installing a skill alone does not give a cloud conversation access to recordings on your computer.

Claude's [September release notes](https://support.claude.com/en/articles/12138966-release-notes) announce the September 25 plugin developer portal and analytics, and the September 16 rollout of Cowork capabilities into conversations. Directory submission remains a separate maintainer action; see [publishing.md](publishing.md).

## Claude Agent SDK

Configure an external stdio server with the absolute installed `transcriber-cli` path and `args: ["mcp"]`. Inspect discovered tools and connection status before transcription. Grant the specific discovered transcription and status tools through `allowedTools`; a file-edit permission mode does not grant MCP permissions. Use the [current Agent SDK MCP guide](https://code.claude.com/docs/en/agent-sdk/mcp), rather than older Claude Code SDK examples. No extra SDK dependency is required in this plugin.

## Verification

The Claude marketplace and both plugin manifests passed `claude plugin validate --strict`. Claude Code's local plugin inventory found the declared MCP server. The launcher passed ShellCheck and hermetic checks for empty options, inherited credentials, overrides, shell characters, paths with spaces, and exit-code propagation. A real launcher/server smoke test passed protocol negotiation, tool discovery, and `auth_status` with a synthetic key, without exposing the key. These checks do not use speech-provider credits or prove a live Desktop/Cowork session.
