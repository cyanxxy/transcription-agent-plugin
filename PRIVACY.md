# Privacy

Transcription Agent Plugin (the skill, plugins, installer, `transcriber-cli`, and MCP server in this repository) runs on your computer. The project has no server of its own and collects no analytics or telemetry.

## What leaves your computer

When you transcribe a recording, the audio is converted with FFmpeg and sent to the speech-to-text provider for the model you choose. The response is written to the output file you name, or returned to your agent.

| Model | Provider | Endpoint |
| --- | --- | --- |
| `gemini-3.5-transcribe` (default) | Google | `generativelanguage.googleapis.com` |
| `muse-voice-transcribe-1.0` | Meta | `api.meta.ai` |
| `MAI-Transcribe-2` | Microsoft | Your Azure Speech resource |

Each provider's own API terms govern how it processes and retains that audio. Your agent's service (for example Anthropic or OpenAI) receives transcript text when the agent reads the output or receives an MCP tool result. MCP results include the saved path, format, model, file size, and a preview of up to 20 KB by default (configurable up to 100 KB). Reading an issued MCP transcript resource returns its full text, up to 16 MiB, to the connected client.

The inline transcript viewer contains no network assets, analytics, or provider credentials. It receives the tool result from its host and can request a full transcript through that host's MCP resource bridge. Transcript resource links are private to the client session that created them, expire when the server stops or after 1,024 newer transcripts displace them, and cannot read arbitrary local files. Transcripts are not published through a public catalog or cache.

The installer and the Claude Code CLI plugin also download source code from GitHub (this repository and [transcription-agent-go](https://github.com/cyanxxy/transcription-agent-go)) and Go modules from the Go module proxy to build the engine.

## What stays on your computer

- **API keys** are stored in owner-only files (mode `600`) in `~/.config/transcribe-for-agents/`, or read from environment variables, or held by your MCP client's secure storage. They are sent only to the matching provider, never to other providers, and are never written to command arguments or the agent chat.
- **Recordings and transcripts** stay where you keep them. Temporary audio chunks are deleted after each run.

## Contact

Open an issue at <https://github.com/cyanxxy/transcription-agent-plugin/issues>.
