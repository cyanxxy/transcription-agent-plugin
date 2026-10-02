<div align="center">

<img src="plugins/transcription-agent/assets/logo.png" alt="" width="96" height="96">

# Transcription Agent Plugin

**Coding agents can't hear. This gives them ears.**

Hand Claude Code, Codex in the ChatGPT desktop app, Claude Desktop, or any Agent Skills or MCP client a recording,<br>
and get back a timestamped, speaker-labeled transcript, subtitles, or JSON.

[![CI](https://github.com/cyanxxy/transcription-agent-plugin/actions/workflows/ci.yml/badge.svg)](https://github.com/cyanxxy/transcription-agent-plugin/actions/workflows/ci.yml)
[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)
![Clients](https://img.shields.io/badge/clients-Claude%20Code%20%7C%20Codex%20%7C%20Agent%20Skills%20%7C%20MCP-6D28D9)

[Quick start](#quick-start) · [Usage](#usage) · [MCP server](#mcp-server) · [Models](#supported-models) · [Agents](#supported-agents) · [Credentials](#credentials) · [Troubleshooting](#troubleshooting)

</div>

---

## Why

Recordings are everywhere in real work: standups, customer calls, interviews, podcasts, voice memos. Coding agents can read your code, screenshots, and PDFs, but not your audio.

- **Claude can't open audio files.** Claude Code's Read tool handles text, images, PDFs, and notebooks, it has [no audio tool](https://code.claude.com/docs/en/tools-reference), and the Claude API accepts [no audio file types](https://platform.claude.com/docs/en/build-with-claude/files). [`/voice`](https://code.claude.com/docs/en/voice-dictation) dictates your prompt for up to two minutes; it can't transcribe a file on disk, and it isn't available with an API key, Bedrock, Vertex AI, or Microsoft Foundry.
- **Codex can't either.** You can attach [files, folders, and images](https://learn.chatgpt.com/docs/features), and voice mode is for talking to Codex, not for transcribing a recording.
- **So agents improvise.** Asked for a transcript, an agent typically tries to install a local speech model: a Python environment, gigabytes of weights, slow runs on a laptop CPU, and still no speaker labels. Or it gives up.

Transcription Agent closes that gap with one skill and one command. The agent runs `transcriber-cli`, which sends the audio to a dedicated speech-to-text model and writes the result next to the recording.

| | |
| --- | --- |
| **Speaker labels and timestamps** | On every segment. |
| **TXT, SRT, or JSON** | Ready for notes, subtitles, or code. |
| **Any length, any common format** | FFmpeg converts MP3, WAV, M4A, FLAC, and OGG and splits long audio into chunks each provider accepts. |
| **Your choice of model** | Google Gemini 3.5 Transcribe, Meta Muse Voice Transcribe, or Microsoft MAI-Transcribe-2. |
| **Keys stay out of the chat** | Saved on your machine, never passed as command arguments, and sent only to the provider you picked. |
| **No Python or Node.js** | A Go binary plus FFmpeg. |

## Quick start

Pick the line that matches how you work. Everything runs on macOS and Linux.

| You use | Install with |
| --- | --- |
| Claude Code or Codex in the ChatGPT desktop app | [The installer](#the-installer); optionally add the [native MCP plugin](#native-mcp-plugin) |
| Claude Code, without a separate installer | [The Claude Code CLI plugin](#claude-code-plugins) |
| Claude Desktop | [The MCPB bundle](#claude-desktop) |
| Cursor, GitHub Copilot, OpenCode, or another [Agent Skills agent](#supported-agents) | [The installer, then the Skills CLI](#other-agent-skills-agents) |
| Any other MCP client | [The installer, then register the server](#other-mcp-clients) |

Then start a new session and ask:

> Transcribe `~/Recordings/standup.m4a` and save subtitles.

### The installer

Run this in your own terminal:

```bash
curl -fsSL https://raw.githubusercontent.com/cyanxxy/transcription-agent-plugin/main/install.sh | bash
```

It will:

1. **Check prerequisites.** If Go or FFmpeg is missing and Homebrew is available, it offers to install them.
2. **Install the skill** for Claude Code (`~/.claude/skills`) and Codex (`~/.agents/skills`). It skips an agent where the `transcription-agent` plugin is already enabled, so the skill never loads twice, and it moves aside an older copy in Codex's deprecated `~/.codex/skills`.
3. **Build the CLI** from the pinned `v1.1.0` engine release, verified against its commit hash, into `~/.local/bin/transcriber-cli`, along with the [MCP server](#mcp-server) it runs.
4. **Ask for your API key** without echoing it, or confirm the key you already saved.
5. **Warn you** if `~/.local/bin` isn't on your `PATH`.

To upgrade, run it again. It replaces only what it installed, keeps backups in `~/.local/share/transcribe-for-agents/backups/`, and never touches a `transcriber-cli` or skill it didn't install.

<details>
<summary><b>Installer options</b></summary>

<br>

Prefer to read it first? [Read install.sh](install.sh), then clone this repository and run `bash install.sh` with any of these options.

| Option | Effect |
| --- | --- |
| `--agent claude-code` \| `codex` \| `both` \| `none` | Which agent gets the skill. Default `both`; `none` installs only the CLI and key. |
| `--provider gemini` \| `meta` \| `microsoft` | Which key to set up. Default `gemini`. |
| `--skip-auth` | Skip the key prompt. You need a key before the first transcription. |

With the one-line install, pass options after `bash -s --`:

```bash
curl -fsSL https://raw.githubusercontent.com/cyanxxy/transcription-agent-plugin/main/install.sh | bash -s -- --agent codex --provider meta
```

</details>

### Claude Code plugins

This repository is a Claude Code marketplace with two plugins:

| Plugin | Contains | Use it when |
| --- | --- | --- |
| `transcription-agent-cli` | `transcriber-cli` on Claude's `PATH`, an automatically registered MCP server, and masked provider settings. Builds the engine on first use (needs Go, git, and FFmpeg). Installs `transcription-agent` for the skill. | You don't want to run the installer. |
| `transcription-agent` | The skill only. Also installs in claude.ai and Cowork. | `transcriber-cli` is already installed. |

```bash
claude plugin marketplace add cyanxxy/transcription-agent-plugin
```

```bash
claude plugin install transcription-agent-cli@transcription-agent-tools
```

The CLI plugin's command is on Claude's `PATH`, not your terminal's. Configure a key in the plugin's masked settings, set `GEMINI_API_KEY`, or ask Claude for the command's full path and run `<path> auth set gemini` in your terminal. Its MCP tools are registered automatically, with a 31-minute timeout for long recordings. See [Claude setup and verified versions](docs/claude.md).

### Native MCP plugin

After running the installer, Codex and the ChatGPT desktop app can load the optional `transcription-agent-mcp` plugin:

```bash
codex plugin marketplace add cyanxxy/transcription-agent-plugin
codex plugin add transcription-agent-mcp@transcription-agent-tools
```

This package registers the local MCP server, checks `~/.local/bin` even when a desktop app's `PATH` is limited, and allows 31 minutes per tool call in Codex. Set `TRANSCRIBER_CLI` to an absolute executable path to use another installation. It contains no skill, so it can be installed alongside `transcription-agent` without loading the skill twice. Remove any manually registered copy of the same MCP server to avoid duplicate tools.

The package also includes portable [Agent Plugins 1.0](https://agent-plugins.org/specification) configuration. Clients that support MCP Apps can show an inline transcript preview. Actual ChatGPT and Claude Desktop rendering still needs verification in those apps; other clients receive the normal text and structured result. See [the compatibility review](docs/client-compatibility.md) for current release evidence and cloud limitations.

### Claude Desktop

Download `transcription-agent-mcp-darwin.mcpb` from the [latest release](https://github.com/cyanxxy/transcription-agent-plugin/releases/latest) and open it. The bundle includes the engine for Apple silicon and Intel Macs; you only need FFmpeg (`brew install ffmpeg`). Enter a key in the extension's settings, or leave them empty to use one saved with `transcriber-cli auth set`.

### Other Agent Skills agents

Install the CLI and key without a skill, then add the skill with the [Skills CLI](https://github.com/vercel-labs/skills), replacing `cursor` with [your agent](#supported-agents):

```bash
curl -fsSL https://raw.githubusercontent.com/cyanxxy/transcription-agent-plugin/main/install.sh | bash -s -- --agent none
```

```bash
npx skills add cyanxxy/transcription-agent-plugin --skill transcribe-for-agents -g -a cursor
```

Clients that read [Agent Plugins 1.0](https://agent-plugins.org/specification) can install the plugin from this repository instead:

- **GitHub Copilot CLI:** `copilot plugin marketplace add cyanxxy/transcription-agent-plugin`, then `copilot plugin install transcription-agent@transcription-agent-tools`.
- **VS Code:** run **Chat: Install Plugin From Source** with this repository's URL.
- **Cursor:** reads `.cursor-plugin/marketplace.json` from this repository.
- **Codex:** `codex plugin marketplace add cyanxxy/transcription-agent-plugin`, then `codex plugin add transcription-agent@transcription-agent-tools`.

### Other MCP clients

After the installer, see [MCP server](#mcp-server).

## Usage

### With your agent

Ask in plain language, for example *"transcribe this interview as JSON"*. The agent saves the result next to the audio unless you name another path, checks the file was written, and reports the path, format, and model. Long recordings take several minutes; the skill tells the agent to wait instead of starting a second run.

In Claude Code you can also call the skill directly:

```text
/transcribe-for-agents /absolute/path/to/meeting.m4a
```

With either plugin, the command is `/transcription-agent:transcribe-for-agents`.

> [!NOTE]
> Remote sessions running on your computer can use that computer's CLI, files, and keys. Cloud-only sessions such as Claude Code on the web and Codex cloud don't inherit your local installation; the skill needs the CLI and credentials where the work runs.

### From the terminal

```bash
# Subtitles with the default Gemini model
transcriber-cli -i ~/Recordings/meeting.m4a -o ~/Recordings/meeting.srt -format srt

# Structured JSON with Meta Muse Voice Transcribe
transcriber-cli -model muse-voice-transcribe-1.0 -i interview.wav -o interview.json -format json

# Plain text with Microsoft MAI-Transcribe-2, printed to stdout
transcriber-cli -model MAI-Transcribe-2 -i podcast.mp3
```

> [!WARNING]
> `-o` replaces an existing file. Choose an unused path unless you mean to overwrite it.

### Codex network access

Codex runs commands in a sandbox with network access off, and the CLI needs the network to reach the speech provider. Approve the request when Codex asks to run `transcriber-cli` with network access, or allow it for every sandboxed command in `~/.codex/config.toml`:

```toml
[sandbox_workspace_write]
network_access = true
```

## MCP server

`transcriber-cli mcp` runs a local [Model Context Protocol](https://modelcontextprotocol.io/specification/2026-07-28) server over stdio, so compatible MCP clients can transcribe local files without a skill or shell access. It supports protocols 2025-06-18, 2025-11-25, and 2026-07-28. Clients need at least 2025-06-18 for structured results and transcript resource links.

| Tool | What it does |
| --- | --- |
| `transcribe` | Transcribes `audio_path` to `txt`, `srt`, or `json`, saves it beside the audio or at `output_path`, and returns a bounded preview plus a link to the completed transcript. Reports progress and stops the engine if the client cancels. MCP Apps clients can show the inline viewer. |
| `auth_status` | Shows provider configuration and FFmpeg availability, without showing keys. |

`transcribe` never replaces an existing file unless `overwrite` is true, including a file that appears while the transcription runs, and `output_path` must end in the format's extension.

If another program creates the destination while a transcription is running, the completed transcript is saved to a unique recovery file. The error result includes `error.recovery_output_path` and `error.resource_uri`, so you can recover it without paying for another transcription. Concurrent calls to this server with the same `output_path` are rejected before another engine run starts.

**Claude Desktop** uses the [MCPB bundle](#claude-desktop), which is also listed in the [MCP registry](https://registry.modelcontextprotocol.io/) as `io.github.cyanxxy/transcription-agent`.

**Claude Code and Codex**, after the installer, can register it manually if they aren't using the CLI or native MCP plugin:

```bash
claude mcp add transcription-agent -- transcriber-cli mcp
```

```bash
codex mcp add transcription-agent -- transcriber-cli mcp
```

**Cursor, VS Code, and other clients** that take a JSON config need the full path, because apps started from the Dock may not see `~/.local/bin`:

```json
{
  "mcpServers": {
    "transcription-agent": {
      "command": "/Users/you/.local/bin/transcriber-cli",
      "args": ["mcp"]
    }
  }
}
```

A long recording can take several minutes, and some clients stop waiting before then. The native Codex plugin and Claude Code CLI plugin allow 31 minutes. For a manually registered Codex server, set `tool_timeout_sec = 1860` under `[mcp_servers.transcription-agent]` in `~/.codex/config.toml`. After a timeout, check whether the original run was cancelled or completed before retrying; an overlapping run can incur another provider charge.

The default inline preview is 20 KB. Use `max_inline_bytes` to request a smaller preview or up to 100 KB. When `truncated` is true, read the returned `resource_uri` through MCP `resources/read`; it exposes completed transcripts issued by this server session, up to 16 MiB. Links expire when the server stops or after 1,024 newer transcripts displace them. Larger outputs and expired links remain available at `output_path`. Transcript content is private and is never cached as a public catalog.

## Supported models

Pick a model with `-model` (or the `model` tool argument). The CLI rejects anything else.

| | Gemini 3.5 Transcribe (default) | Meta Muse Voice Transcribe | Microsoft MAI-Transcribe-2 |
| --- | --- | --- | --- |
| **Model ID** | `gemini-3.5-transcribe` | `muse-voice-transcribe-1.0` | `MAI-Transcribe-2` |
| **Languages** | 85+ | 70+ | 60 |
| **Speakers** | Up to 8 (3+ is experimental) | 20+ (non-overlapping speech) | Speaker diarization |
| **Credentials** | `gemini` key | `meta` key | `microsoft` key and Azure Speech endpoint |
| **Docs** | [Google AI](https://ai.google.dev/gemini-api/docs/transcribe) | [Meta](https://dev.meta.ai/models/muse-voice-transcribe/) | [Microsoft Learn](https://learn.microsoft.com/en-us/azure/ai-services/speech-service/mai-transcribe) (public preview) |

Figures come from each provider's documentation as of September 2026. Every input is converted to mono WAV and split into chunks before upload, so any supported format and length works with every model. The CLI accepts files up to 200 MiB and stops a run after 30 minutes.

## Supported agents

The skill follows the open [Agent Skills](https://agentskills.io/specification) format. Any agent that supports it can use the skill, as long as `transcriber-cli` is installed and the agent can run shell commands with network access.

| Agent | Install the skill with | Status |
| --- | --- | --- |
| Claude Code | [The installer](#the-installer) or a [plugin](#claude-code-plugins) | Tested |
| Codex (CLI and app) | [The installer](#the-installer) or the [Codex plugin](#other-agent-skills-agents) | Tested |
| Cursor | `npx skills add … -a cursor` | Compatible |
| GitHub Copilot | `npx skills add … -a github-copilot` | Compatible |
| OpenCode | `npx skills add … -a opencode` | Compatible |
| Amp | `npx skills add … -a amp` | Compatible |
| Goose | `npx skills add … -a goose` | Compatible |
| Windsurf | `npx skills add … -a windsurf` | Compatible |
| Antigravity CLI | `npx skills add … -a antigravity-cli` | Compatible |
| Gemini CLI (Code Assist) | `npx skills add … -a gemini-cli` | Compatible |
| Kiro CLI | `npx skills add … -a kiro-cli` | Compatible |
| Cline | `npx skills add … -a cline` | Compatible |
| Zed | `npx skills add … -a zed` | Compatible |
| Warp | `npx skills add … -a warp` | Compatible |

`…` stands for `cyanxxy/transcription-agent-plugin --skill transcribe-for-agents -g`. See the [Skills CLI](https://github.com/vercel-labs/skills) for every agent it supports. Antigravity CLI [replaced Gemini CLI](https://geminicli.com/) for personal Google accounts in June 2026; Gemini CLI remains for Gemini Code Assist.

## Credentials

Keys live in owner-only files (mode `600`) in `~/.config/transcribe-for-agents/`, or `$XDG_CONFIG_HOME/transcribe-for-agents/` if set. They never appear in command arguments or the agent chat.

```bash
transcriber-cli auth set gemini       # prompt for a key (Microsoft also asks for the endpoint)
transcriber-cli auth status           # show every provider
transcriber-cli auth status gemini    # exit code 1 if not configured
transcriber-cli auth remove gemini    # delete the saved key
```

Environment variables override saved keys. For each run, only the selected model's provider receives its credentials; the others are removed from the engine's environment.

| Provider | Variables |
| --- | --- |
| Gemini | `GEMINI_API_KEY` |
| Meta | `META_API_KEY` |
| Microsoft | `AZURE_SPEECH_KEY` and `AZURE_SPEECH_ENDPOINT` (for example `https://your-resource.cognitiveservices.azure.com`) |

See [PRIVACY.md](PRIVACY.md) for exactly what leaves your machine.

<details>
<summary><b>Advanced environment variables</b></summary>

<br>

| Variable | Purpose |
| --- | --- |
| `TRANSCRIBER_CLI` | Absolute path to the wrapper when it isn't on `PATH` or in `~/.local/bin`. The skill checks this first. |
| `TRANSCRIBER_ENGINE_BIN` | Run a different engine binary. |
| `TRANSCRIBER_MCP_BIN` | Run a different MCP server binary for `transcriber-cli mcp`. |
| `XDG_CONFIG_HOME` / `XDG_DATA_HOME` | Change where keys, the engine, and the MCP server are stored. |

</details>

## Troubleshooting

| Symptom | Fix |
| --- | --- |
| `transcriber-cli: command not found` | Add `~/.local/bin` to your `PATH`, or call `~/.local/bin/transcriber-cli` directly. |
| `transcription engine is missing` | Run the installer again. With the Claude Code CLI plugin, install Go and git so it can build the engine. |
| `MCP server is missing` | Run the installer again. |
| `unsupported model: …` | Use one of the three [supported models](#supported-models). |
| `microsoft: incomplete` | Microsoft needs a key and an endpoint. Run `transcriber-cli auth set microsoft`. |
| `… is not a transcribe-for-agents wrapper` or `… skill` | Another tool already uses that path. Move it, then run the installer again. |
| The agent doesn't see the skill | Start a new agent session after installing. |
| The agent lists the skill twice | Keep one install method per agent. Run the installer again after enabling a plugin to remove its standalone copy. |
| Connection or DNS errors in Codex | The sandbox is blocking the network. See [Codex network access](#codex-network-access). |
| An MCP tool call times out | The client stopped waiting. Transcribe from the terminal or with the skill. |

FFmpeg (`ffmpeg` and `ffprobe`) must stay on your `PATH` when the agent runs.

## Development

```bash
bash tests/install_test.sh                                              # installer and wrapper
(cd plugins/transcription-agent-cli/mcp-server && go test -race ./...)  # MCP server
claude plugin validate . --strict                                       # Claude Code marketplace
claude plugin eval plugins/transcription-agent --ablation none          # skill evals (uses API credit)
bash mcpb/build.sh darwin dist                                          # MCPB bundle
claude --plugin-dir ./plugins/transcription-agent                       # try the plugin from this checkout
```

- **Tests.** The installer tests replace `go`, `git`, `ffmpeg`, and `ffprobe` with fakes, so they need no network or toolchain. CI runs them on Linux and macOS with the MCP server tests, ShellCheck, `claude plugin validate --strict`, schema checks for the Agent Plugins manifest and `server.json`, and both MCPB builds.
- **Evals.** `plugins/transcription-agent/evals/` holds eight [`claude plugin eval`](https://code.claude.com/docs/en/plugin-evals) cases: five where the skill should load and three where it shouldn't. They grant no shell access, so they never call a speech provider. Keep `--ablation none`; without it, the check that the skill loaded is left out of the score. The **Plugin evals** workflow runs them on demand and on each release, given an `ANTHROPIC_API_KEY` secret.
- **Releasing.** Bump the version in the plugin manifests and `server.json`, then push a `v<version>` tag. The **Release** workflow builds the MCPB bundles, attaches them to a GitHub release, and publishes to the MCP registry. [docs/publishing.md](docs/publishing.md) covers the directory listings.

<details>
<summary><b>Repository layout</b></summary>

<br>

```text
install.sh                          One-command installer
plugins/transcription-agent/        Skill-only plugin for every agent
  skills/transcribe-for-agents/     The shared skill (SKILL.md)
    agents/openai.yaml              Codex skill picker name, icon, and prompt
  evals/                            claude plugin eval cases
  assets/                           Plugin logo and icon
  plugin.json                       Agent Plugins 1.0 manifest (Codex, Cursor, Copilot, VS Code, Kiro)
  .claude-plugin/plugin.json        Claude Code manifest
  .codex-plugin/plugin.json         Codex manifest for versions before Agent Plugins
  .cursor-plugin/plugin.json        Cursor manifest
plugins/transcription-agent-cli/    Claude Code plugin with the CLI
  bin/transcriber-cli               Wrapper: loads credentials, runs the engine and MCP server
  mcp-server/                       Go MCP server behind transcriber-cli mcp
mcpb/                               MCPB bundle manifest, launcher, and build script
server.json                         MCP registry entry
.claude-plugin/marketplace.json     Claude Code marketplace
.agents/plugins/marketplace.json    Codex marketplace
.cursor-plugin/marketplace.json     Cursor marketplace
tests/install_test.sh               Hermetic installer and wrapper tests
```

</details>

## License

[Apache 2.0](LICENSE). See also [PRIVACY.md](PRIVACY.md).
