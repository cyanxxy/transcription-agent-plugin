# Transcription Agent CLI

Local MCP tools that let Claude transcribe a recording on your computer into a timestamped, speaker-labeled transcript: TXT, Markdown, SRT, WebVTT, CSV, or JSON. Clients that support MCP Apps show an interactive transcript with speaker filters, search, and copy.

Installing this plugin also installs the **Transcription Agent** skill plugin, which tells Claude when and how to use the tools.

## Where it works

| Surface | Works |
| --- | --- |
| Claude Code (terminal, IDE, desktop Code tab) | Yes |
| Cowork sessions that run on your computer | Yes |
| Chat on claude.ai, desktop, or mobile | No: chat doesn't start local MCP servers |

## Requirements

- macOS or Linux
- Go and git, to build the transcription engine on first use
- FFmpeg (`ffmpeg` and `ffprobe` on your `PATH`)
- An API key for Google Gemini (default), Meta, or Azure Speech

## Set up a key

In Claude Code, enter a key in the plugin's masked settings when you enable it. Cowork doesn't prompt for plugin settings, so save a key on your computer instead, either with the installer, which also prompts for the key without echoing it:

```bash
curl -fsSL https://raw.githubusercontent.com/cyanxxy/transcription-agent-plugin/main/install.sh | bash -s -- --agent none
```

or by running `scripts/transcriber-cli auth set gemini` from this plugin's folder in your own terminal. Never paste a key into a chat.

## What it runs and sends

- **First use:** clones the pinned [transcription-agent-go](https://github.com/cyanxxy/transcription-agent-go) release from GitHub, checks it against its commit hash, downloads its Go modules from the Go module proxy, and builds the engine and MCP server into `~/.local/share/transcribe-for-agents/`.
- **Each transcription:** sends the recording you name, and only that file, to the speech provider you chose: Google, Meta, or Microsoft. Your key goes only to that provider.
- **Output:** writes the transcript beside the recording, or to the path you give, and never replaces an existing file unless you ask.

The MCP server reads only transcripts it wrote itself. See the [privacy policy](https://github.com/cyanxxy/transcription-agent-plugin/blob/main/PRIVACY.md) and the [full documentation](https://github.com/cyanxxy/transcription-agent-plugin#readme).

## License

Apache-2.0
