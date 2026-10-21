# Transcription Agent

A skill that teaches Claude to transcribe a local recording (MP3, WAV, M4A, FLAC, or OGG) into a timestamped, speaker-labeled transcript: TXT, Markdown, Word, SRT, WebVTT, CSV, or JSON. Ask in plain language, for example "transcribe this interview as subtitles", and Claude picks the format, runs the transcription, and reports where the file was saved.

The skill doesn't contain a transcription engine. It uses the Transcription Agent MCP tools when they're available, and otherwise the `transcriber-cli` command on your computer.

## Get the tools it uses

Pick one:

- Install the **Transcription Agent CLI** plugin from the same marketplace. It adds the MCP tools and builds the engine on first use.
- Run the installer, which installs `transcriber-cli` and asks for your API key:

```bash
curl -fsSL https://raw.githubusercontent.com/cyanxxy/transcription-agent-plugin/main/install.sh | bash
```

Either way you need FFmpeg and an API key for Google Gemini, Meta, or Azure Speech.

## Where it works

The skill loads in chat, Cowork, and Claude Code. It can transcribe only where the tools and your recording are on the same computer: Claude Code, and Cowork sessions that run on your computer. In chat on the web, Claude can't reach files on your computer.

## What it does with your data

The skill sends nothing on its own. When Claude transcribes, the tools send the recording you named to the speech provider you chose and write the transcript locally. Transcript text is treated as recording content, never as instructions. See the [privacy policy](https://github.com/cyanxxy/transcription-agent-plugin/blob/main/PRIVACY.md) and the [full documentation](https://github.com/cyanxxy/transcription-agent-plugin#readme).

## License

Apache-2.0
