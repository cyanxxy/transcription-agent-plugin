# Publishing

How to release a version and list it in each directory. Every listing here is a public action on your own account, so the maintainer does these steps; nothing in CI submits to a directory except the MCP registry step in the Release workflow.

## 1. Release

1. Set the same version in:
   - `plugins/transcription-agent/plugin.json`
   - `plugins/transcription-agent/.claude-plugin/plugin.json`
   - `plugins/transcription-agent/.codex-plugin/plugin.json`
   - `plugins/transcription-agent/.cursor-plugin/plugin.json`
   - `plugins/transcription-agent-cli/.claude-plugin/plugin.json`
   - `plugins/transcription-agent-mcp/plugin.json`
   - `plugins/transcription-agent-mcp/.codex-plugin/plugin.json`
   - `server.json` (the Release workflow rewrites the package URLs and hashes)
2. Merge to `main`, then tag and push:

   ```bash
   git tag v1.2.0
   ```

   ```bash
   git push origin v1.2.0
   ```

3. The **Release** workflow checks that the tag matches the CLI plugin's version, builds `transcription-agent-mcp-darwin.mcpb` and `transcription-agent-mcp-linux.mcpb`, creates the GitHub release, and publishes `server.json` to the MCP registry as `io.github.cyanxxy/transcription-agent` using GitHub OIDC. No registry secret is needed because the repository owner matches the `io.github.cyanxxy` namespace.
4. Optional: run the **Plugin evals** workflow (needs the `ANTHROPIC_API_KEY` secret) or `claude plugin eval plugins/transcription-agent --ablation none` locally. The Release workflow also runs the evals after publishing; they skip when the secret isn't set. `--ablation none` keeps the skill-loaded check in the score.

The bundles are unsigned, so Claude Desktop shows an unverified-publisher notice. To sign them, run `npx @anthropic-ai/mcpb sign <file> --cert cert.pem --key key.pem` before uploading.

## 2. Directories

### Anthropic (claude.ai, Cowork, and Claude Code)

- Submit at <https://claude.ai/directory/manage> (needs a paid claude.ai plan). See the [pre-submission checklist](https://claude.com/docs/plugins/pre-submission-checklist).
- Submit **`transcription-agent` only**. It has no top-level `bin/`, so claude.ai and Cowork accept it. `transcription-agent-cli` has a `bin/` folder, which makes claude.ai and Cowork refuse the whole plugin; keep it in this repository's marketplace for Claude Code users.
- State plainly that the skill needs `transcriber-cli` installed on the user's computer, so it works in Claude Code and in Cowork sessions on the user's machine, not in claude.ai chat on the web.
- The MCPB bundle is a separate listing path for Claude Desktop; it comes from the MCP registry entry above.

### OpenAI (ChatGPT and Codex Plugins Directory)

- Submit at <https://platform.openai.com/plugins> as a **Skills only** plugin. You need the Apps Management **Write** permission and a verified individual or business identity.
- Listing fields:

  | Field | Value |
  | --- | --- |
  | Name | Transcription Agent |
  | Short description | Transcribe audio to TXT, SRT, or JSON. |
  | Long description | Use the Transcription Agent CLI to create speaker-labeled, timestamped transcripts from local recordings. Requires the CLI installed on your computer, so it runs in Codex and the ChatGPT desktop app. |
  | Category | Productivity |
  | Website | https://github.com/cyanxxy/transcription-agent-plugin |
  | Support URL | https://github.com/cyanxxy/transcription-agent-plugin/issues |
  | Privacy policy | https://github.com/cyanxxy/transcription-agent-plugin/blob/main/PRIVACY.md |
  | Terms | https://github.com/cyanxxy/transcription-agent-plugin/blob/main/LICENSE |
  | Logo | `plugins/transcription-agent/assets/logo.png` |

- Test cases. These match the eval cases in `plugins/transcription-agent/evals/`; fixture data is any short MP3, WAV, M4A, FLAC, or OGG file and a Gemini API key saved with `transcriber-cli auth set gemini`.

  | # | Prompt | Expected behavior | Expected result |
  | --- | --- | --- | --- |
  | P1 | Transcribe `/path/standup.m4a` and save subtitles next to it. | Loads `transcribe-for-agents`; runs `transcriber-cli -format srt`. | `standup.srt` beside the audio; reply names the path, format, and model. |
  | P2 | Who said what in `~/Downloads/customer-call.mp3`? I need a transcript with speaker labels. | Loads the skill; runs with the default format. | `customer-call.txt` with speaker labels and timestamps. |
  | P3 | Turn `~/Recordings/interview.wav` into JSON segments with timestamps. | Loads the skill; runs with `-format json`. | `interview.json` with segments and metadata. |
  | P4 | Write out the text of this voice memo: `/tmp/memo.ogg` | Loads the skill; runs with the default format. | `memo.txt`; no invented text if the run fails. |
  | P5 | Transcribe `~/Podcasts/episode-12.flac` with Meta's Muse Voice Transcribe model. | Loads the skill; runs with `-model muse-voice-transcribe-1.0`. | `episode-12.txt`, or a clear note that the Meta key is missing. |

  | # | Prompt | Expected behavior | Why the plugin shouldn't act |
  | --- | --- | --- | --- |
  | N1 | Convert `~/Recordings/standup.m4a` to a 128 kbps MP3 with ffmpeg. | Skill stays unused; the agent uses ffmpeg directly. | Audio conversion isn't transcription. |
  | N2 | Write a Python function that parses an SRT subtitle file into a list of cues. | Skill stays unused. | A coding task about subtitles, not a recording. |
  | N3 | Summarize the meeting notes in `notes.md` and list the action items. | Skill stays unused. | The input is text, not audio. |

- Keep the marketplace entry's `products: ["CODEX"]` until a hosted version exists: ChatGPT on the web and mobile can't run a local CLI.
- `transcription-agent-mcp` is an optional local marketplace package. It connects an already installed CLI; keep the public `transcription-agent` listing skills-only. Public MCP submission requires a hosted HTTPS endpoint or arranged local MCP support. Its inline viewer does not remove that transport requirement.

### Cursor Marketplace

- Submit the repository link at <https://cursor.com/marketplace/publish>. Cursor reviews every plugin by hand.
- The repository already has `.cursor-plugin/marketplace.json` (needed for repositories with more than one plugin) and `plugins/transcription-agent/.cursor-plugin/plugin.json` with a relative logo path.
- Cursor doesn't expand the Agent Plugins `${PLUGIN_ROOT}` and `${PLUGIN_DATA}` variables. The skill-only plugin doesn't use them, so nothing changes; if an `mcp.json` is added later, give it a bare `command` such as `transcriber-cli` rather than a path built from those variables.

### GitHub Copilot and VS Code (awesome-copilot)

awesome-copilot doesn't accept pull requests for plugins hosted elsewhere. Open an issue with the [external plugin form](https://github.com/github/awesome-copilot/issues/new?template=external-plugin.yml) after the release tag exists. Automation validates the plugin and runs a Copilot CLI install test; after a maintainer approves, a bot adds it to `plugins/external.json`. Later version bumps can be a pull request to that file.

Get the tag's full commit hash:

```bash
git rev-list -n 1 v1.2.0
```

Form values:

| Field | Value |
| --- | --- |
| Plugin name | transcription-agent |
| Short description | Transcribe local audio to speaker-labeled TXT, SRT, or JSON with Gemini, Meta, or Microsoft speech models. Requires transcriber-cli. |
| GitHub repository | cyanxxy/transcription-agent-plugin |
| Plugin path inside the repository | plugins/transcription-agent |
| Ref to review | v1.2.0 |
| Commit SHA to review | output of the command above (40 characters) |
| Version | 1.2.0 |
| License identifier | Apache-2.0 |
| Author name | cyanxxy |
| Author URL | https://github.com/cyanxxy |
| Homepage URL | https://github.com/cyanxxy/transcription-agent-plugin#readme |
| Keywords | audio, transcription, diarization, subtitles |
| Additional notes for reviewers | Skill-only Agent Plugins 1.0 package. It runs a local CLI (`transcriber-cli`) that the user installs separately; without it the skill explains how to install it. |

### skills.sh

No submission step. The skill is listed after its first `npx skills add cyanxxy/transcription-agent-plugin`, and ranking comes from install counts.
