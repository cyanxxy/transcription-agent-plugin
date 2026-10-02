#!/usr/bin/env bash
# Hermetic tests for install.sh and the transcriber-cli wrapper. Go, git, ffmpeg
# and ffprobe are replaced with fakes, so no network access or toolchain is needed.
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
wrapper_src="$repo/plugins/transcription-agent-cli/bin/transcriber-cli"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
failures=0

pass() { printf 'ok   %s\n' "$1"; }
flunk() { printf 'FAIL %s\n' "$1"; failures=$((failures + 1)); }
quiet() { "$@" >/dev/null 2>&1; }
not() { ! "$@" >/dev/null 2>&1; }
check() { local name="$1"; shift; if "$@"; then pass "$name"; else flunk "$name"; fi; }

# Fake tools --------------------------------------------------------------
fakes="$work/fakes"
mkdir -p "$fakes" "$work/engine/cmd/cli"
touch "$work/engine/go.mod"
printf '#!/usr/bin/env bash\nexit 0\n' > "$fakes/ffmpeg"
cp "$fakes/ffmpeg" "$fakes/ffprobe"
cat > "$fakes/git" <<'FAKE'
#!/usr/bin/env bash
# "clone" copies $FAKE_GIT_SOURCE to the destination; everything else fails.
for arg; do [[ "$arg" == clone ]] && { cp -R "$FAKE_GIT_SOURCE" "${!#}"; exit 0; }; done
exit 1
FAKE
cat > "$fakes/go" <<'FAKE'
#!/usr/bin/env bash
# "build -o <out>" writes a fake engine that reports which credentials it sees.
while (($#)); do [[ "$1" == -o ]] && out="$2"; shift; done
cat > "$out" <<'ENGINE'
#!/usr/bin/env bash
for v in GEMINI_API_KEY META_API_KEY AZURE_SPEECH_KEY AZURE_SPEECH_ENDPOINT; do
  [[ -z "${!v:-}" ]] || echo "env $v"
done
ENGINE
chmod +x "$out"
FAKE
chmod +x "$fakes"/*

new_home() {
  home="$work/home$((++homes))"
  mkdir -p "$home"
}
homes=0

# run_install <extra env...> -- <args...>: run install.sh from the repository file.
run_install() {
  env -i HOME="$home" PATH="$fakes:/usr/bin:/bin" TRANSCRIBE_ENGINE_SOURCE="$work/engine" \
    bash "$repo/install.sh" "$@" </dev/null
}
cli() {
  env -i HOME="$home" PATH="/usr/bin:/bin" "$@"
}

# Tests -------------------------------------------------------------------
new_home
cd "$work"
if output="$(env -i HOME="$home" PATH="$fakes:/usr/bin:/bin" FAKE_GIT_SOURCE="$repo" \
    TRANSCRIBE_ENGINE_SOURCE="$work/engine" bash -s -- --skip-auth < "$repo/install.sh" 2>&1)"; then
  pass 'piped install (curl | bash) succeeds'
else
  flunk 'piped install (curl | bash) succeeds'; printf '%s\n' "$output"
fi
check 'installs wrapper' test -x "$home/.local/bin/transcriber-cli"
check 'installs engine' test -x "$home/.local/share/transcribe-for-agents/transcriber-cli.bin"
check 'installs Claude Code skill' test -f "$home/.claude/skills/transcribe-for-agents/SKILL.md"
check 'installs Codex skill in ~/.agents/skills' test -f "$home/.agents/skills/transcribe-for-agents/SKILL.md"
check 'does not use the deprecated ~/.codex/skills' test ! -e "$home/.codex/skills/transcribe-for-agents"
check 'warns when bin dir is not on PATH' grep -q 'is not on your PATH' <<<"$output"

check 're-running an identical install succeeds' quiet run_install --skip-auth
check 'identical install makes no backup' test ! -e "$home/.local/share/transcribe-for-agents/backups"

printf 'old line\n' >> "$home/.claude/skills/transcribe-for-agents/SKILL.md"
printf '#!/usr/bin/env bash\n# transcribe-for-agents old wrapper\nexec transcriber-cli.bin\n' > "$home/.local/bin/transcriber-cli"
check 'upgrade over an older install succeeds' quiet run_install --skip-auth
check 'upgrade replaces the skill' diff -q "$repo/plugins/transcription-agent/skills/transcribe-for-agents/SKILL.md" \
  "$home/.claude/skills/transcribe-for-agents/SKILL.md"
check 'upgrade replaces the wrapper' cmp -s "$wrapper_src" "$home/.local/bin/transcriber-cli"
check 'upgrade backs up the old skill' test -n "$(find "$home/.local/share/transcribe-for-agents/backups" -name claude-skill)"
check 'upgrade backs up the old wrapper' test -n "$(find "$home/.local/share/transcribe-for-agents/backups" -name transcriber-cli)"
check 'backups stay out of the skills directory' test "$(find "$home/.claude/skills" -mindepth 1 -maxdepth 1 | wc -l | tr -d ' ')" = 1

new_home
mkdir -p "$home/.local/bin"
printf '#!/bin/sh\necho someone else\n' > "$home/.local/bin/transcriber-cli"
check 'refuses to replace an unrelated transcriber-cli' not run_install --skip-auth
check 'unrelated transcriber-cli is untouched' grep -q 'someone else' "$home/.local/bin/transcriber-cli"

new_home
mkdir -p "$home/.claude/skills/transcribe-for-agents"
printf -- '---\nname: something-else\n---\n' > "$home/.claude/skills/transcribe-for-agents/SKILL.md"
check 'refuses to replace an unrelated skill' not run_install --skip-auth --agent claude-code

skill_src="$repo/plugins/transcription-agent/skills/transcribe-for-agents"
backups() { find "$home/.local/share/transcribe-for-agents/backups" -maxdepth 2 -name "$1" 2>/dev/null; }

new_home
mkdir -p "$home/.codex/skills"
cp -R "$skill_src" "$home/.codex/skills/"
check 'installs over a copy in the deprecated Codex folder' quiet run_install --skip-auth
check 'removes the deprecated Codex copy' test ! -e "$home/.codex/skills/transcribe-for-agents"
check 'backs up the deprecated Codex copy' test -n "$(backups codex-legacy-skill)"
check 'installs the current Codex copy' test -f "$home/.agents/skills/transcribe-for-agents/SKILL.md"

new_home
mkdir -p "$home/.codex/skills" "$home/elsewhere"
cp -R "$skill_src" "$home/elsewhere/"
ln -s "$home/elsewhere/transcribe-for-agents" "$home/.codex/skills/transcribe-for-agents"
check 'warns about a symlinked deprecated Codex copy' \
  grep -q 'is a symlink from another installer' <<<"$(run_install --skip-auth 2>&1)"
check 'leaves the symlinked deprecated Codex copy alone' test -L "$home/.codex/skills/transcribe-for-agents"

new_home
mkdir -p "$home/.claude/skills"
cp -R "$skill_src" "$home/.claude/skills/"
printf '{\n  "enabledPlugins": {\n    "transcription-agent@transcription-agent-tools": true\n  }\n}\n' > "$home/.claude/settings.json"
check 'reports that the Claude Code plugin provides the skill' \
  grep -q 'Claude Code: the transcription-agent@transcription-agent-tools plugin provides the skill' <<<"$(run_install --skip-auth 2>&1)"
check 'no standalone Claude Code copy while the plugin is enabled' test ! -e "$home/.claude/skills/transcribe-for-agents"
check 'backs up the old Claude Code copy' test -n "$(backups claude-skill)"
check 'still installs the Codex copy' test -f "$home/.agents/skills/transcribe-for-agents/SKILL.md"

new_home
mkdir -p "$home/.claude"
printf '{"enabledPlugins":{"transcription-agent@transcription-agent-tools":false}}\n' > "$home/.claude/settings.json"
quiet run_install --skip-auth --agent claude-code
check 'a disabled Claude Code plugin does not skip the skill' test -f "$home/.claude/skills/transcribe-for-agents/SKILL.md"

new_home
mkdir -p "$home/.codex"
printf '[plugins."other@x"]\nenabled = true\n\n[plugins."transcription-agent@transcription-agent-tools"]\nenabled = true\n' > "$home/.codex/config.toml"
quiet run_install --skip-auth --agent codex
check 'no standalone Codex copy while the Codex plugin is enabled' test ! -e "$home/.agents/skills/transcribe-for-agents"
printf '[plugins."transcription-agent@transcription-agent-tools"]\nenabled = false\n[plugins."other@x"]\nenabled = true\n' > "$home/.codex/config.toml"
quiet run_install --skip-auth --agent codex
check 'a disabled Codex plugin does not skip the skill' test -f "$home/.agents/skills/transcribe-for-agents/SKILL.md"

new_home
check '--agent none installs only the CLI' grep -q 'npx skills add' <<<"$(run_install --skip-auth --agent none 2>&1)"
check '--agent none installs the wrapper' test -x "$home/.local/bin/transcriber-cli"
check '--agent none installs no skill' test -z "$(find "$home" -name SKILL.md)"
check 'rejects an unknown --agent' not run_install --skip-auth --agent cursor

new_home
mkdir -p "$home/.claude" "$home/.codex"
printf '{"enabledPlugins":{"transcription-agent@transcription-agent-tools":true}}\n' > "$home/.claude/settings.json"
printf '[plugins."transcription-agent@transcription-agent-tools"]\nenabled = true\n' > "$home/.codex/config.toml"
check 'installs only the CLI when both plugins are enabled' quiet run_install --skip-auth
check 'CLI is installed when both plugins are enabled' test -x "$home/.local/bin/transcriber-cli"

new_home
run_install --skip-auth >/dev/null 2>&1
wrapper="$home/.local/bin/transcriber-cli"
config="$home/.config/transcribe-for-agents"
check 'status: nothing configured' grep -q 'gemini: not configured' <<<"$(cli "$wrapper" auth status)"
check 'status: microsoft key without endpoint is incomplete' \
  grep -q 'microsoft: incomplete' <<<"$(cli AZURE_SPEECH_KEY=k "$wrapper" auth status microsoft || true)"
check 'status <provider> exits 1 when not configured' not cli "$wrapper" auth status meta
check 'status rejects an unknown provider' not cli "$wrapper" auth status openai

mkdir -p "$config" && chmod 700 "$config"
for name in gemini.key meta.key microsoft.key microsoft.endpoint; do printf 'secret' > "$config/$name"; done
check 'status: microsoft from key file plus env endpoint' grep -q 'key via local file, endpoint via environment' \
  <<<"$(rm "$config/microsoft.endpoint"; cli AZURE_SPEECH_ENDPOINT=https://x.example "$wrapper" auth status microsoft)"
printf 'https://x.example' > "$config/microsoft.endpoint"
check 'status <provider> exits 0 when configured' quiet cli "$wrapper" auth status gemini

check 'default model exports only the Gemini key' test "$(cli "$wrapper" -i a.m4a)" = 'env GEMINI_API_KEY'
check 'Meta model exports only the Meta key' \
  test "$(cli "$wrapper" -i a.m4a --model muse-voice-transcribe-1.0)" = 'env META_API_KEY'
check 'Microsoft model exports only Azure settings' \
  test "$(cli "$wrapper" -model=MAI-Transcribe-2 -i a.m4a | tr '\n' ' ')" = 'env AZURE_SPEECH_KEY env AZURE_SPEECH_ENDPOINT '
check 'default model is accepted when named' \
  test "$(cli "$wrapper" -model gemini-3.5-transcribe -i a.m4a)" = 'env GEMINI_API_KEY'
for model in gemini-3.8-flash gemini-3.1-flash-lite muse-voice-transcribe-2.0 MAI-Transcribe-1.5 whisper-1; do
  check "rejects unsupported model $model" not cli "$wrapper" -model "$model" -i a.m4a
done
check 'unsupported model error names the supported models' \
  grep -q 'use gemini-3.5-transcribe, muse-voice-transcribe-1.0, or MAI-Transcribe-2' <<<"$(cli "$wrapper" --model=x -i a.m4a 2>&1 || true)"
check 'unselected provider keys from the environment are removed' \
  test "$(cli GEMINI_API_KEY=g META_API_KEY=m AZURE_SPEECH_KEY=a AZURE_SPEECH_ENDPOINT=https://x.example "$wrapper" -i a.m4a)" = 'env GEMINI_API_KEY'
check 'environment key for the selected provider is kept' \
  test "$(cli META_API_KEY=m GEMINI_API_KEY=g "$wrapper" -model muse-voice-transcribe-1.0 -i a.m4a)" = 'env META_API_KEY'
check 'a flag value that looks like -model is not a model' \
  test "$(cli "$wrapper" -instructions '--model=example' -version)" = 'env GEMINI_API_KEY'
check 'a flag value equal to -model is skipped' \
  test "$(cli "$wrapper" -topic -model -model muse-voice-transcribe-1.0 -i a.m4a)" = 'env META_API_KEY'
check 'boolean flags do not consume the next argument' \
  test "$(cli "$wrapper" -version -judge=false --model muse-voice-transcribe-1.0)" = 'env META_API_KEY'
check 'the last -model wins' \
  test "$(cli "$wrapper" -model MAI-Transcribe-2 -model=muse-voice-transcribe-1.0 -i a.m4a)" = 'env META_API_KEY'
check 'flag parsing stops at the first non-flag argument' \
  test "$(cli "$wrapper" -i a.m4a extra -model MAI-Transcribe-2)" = 'env GEMINI_API_KEY'
check 'flag parsing stops at --' \
  test "$(cli "$wrapper" -i a.m4a -- -model whisper-1)" = 'env GEMINI_API_KEY'
check 'saved key files are owner-only' test -n "$(find "$config/gemini.key" -perm 600)"

check 'installer reports an already saved key instead of prompting' \
  grep -q 'gemini: configured via local file' <<<"$(run_install 2>&1)"

check 'auth remove deletes the saved key' quiet cli "$wrapper" auth remove meta
check 'meta key file is gone' test ! -e "$config/meta.key"

pin() { sed -n "s/^$1='\(.*\)'\$/\1/p" "$2"; }
for name in engine_url engine_ref engine_sha; do
  check "install.sh and the wrapper pin the same $name" \
    test -n "$(pin "$name" "$wrapper_src")" -a "$(pin "$name" "$repo/install.sh")" = "$(pin "$name" "$wrapper_src")"
done

# The Claude Code plugin's bin/transcriber-cli builds the engine on first use
# when install.sh has not installed it.
new_home
plugin_cli() {
  env -i HOME="$home" PATH="$1" TRANSCRIBE_ENGINE_SOURCE="$work/engine" GEMINI_API_KEY=g "$wrapper_src" -i a.m4a
}
check 'first run without Go explains how to get the engine' \
  grep -q 'install Go and git' <<<"$(plugin_cli /usr/bin:/bin 2>&1 || true)"
check 'first run builds the missing engine' test "$(plugin_cli "$fakes:/usr/bin:/bin" 2>/dev/null)" = 'env GEMINI_API_KEY'
check 'first run installs the engine where install.sh does' \
  test -x "$home/.local/share/transcribe-for-agents/transcriber-cli.bin"
check 'later runs keep stdout clean' test "$(plugin_cli "$fakes:/usr/bin:/bin")" = 'env GEMINI_API_KEY'
check 'build progress goes to stderr' \
  grep -q 'first run only' <<<"$(rm "$home/.local/share/transcribe-for-agents/transcriber-cli.bin"; plugin_cli "$fakes:/usr/bin:/bin" 2>&1 >/dev/null)"
check 'a missing TRANSCRIBER_ENGINE_BIN is not built' \
  not env -i HOME="$home" PATH="$fakes:/usr/bin:/bin" TRANSCRIBER_ENGINE_BIN="$work/none" "$wrapper_src" -i a.m4a

# transcriber-cli mcp ------------------------------------------------------
# The fake go writes a script that prints the credentials it sees; here it also
# stands in for the MCP server, so running it proves the wrapper exec'd it.
new_home
run_install --skip-auth >/dev/null 2>&1
server="$home/.local/share/transcribe-for-agents/transcriber-mcp"
check 'installer installs the MCP server' test -x "$server"
check 'installed wrapper runs the installed MCP server' quiet cli "$home/.local/bin/transcriber-cli" mcp
check 'mcp rejects extra arguments' not cli "$home/.local/bin/transcriber-cli" mcp --port 1
# shellcheck disable=SC2016 # the fake server expands $TRANSCRIBER_CLI itself
printf '#!/usr/bin/env bash\necho "cli=$TRANSCRIBER_CLI"\n' > "$server"
check 'mcp passes the wrapper path to the server' \
  test "$(cli "$home/.local/bin/transcriber-cli" mcp)" = "cli=$home/.local/bin/transcriber-cli"

new_home
plugin_mcp() { env -i HOME="$home" PATH="$fakes:/usr/bin:/bin" "$wrapper_src" mcp; }
check 'plugin copy builds the MCP server on first use' quiet plugin_mcp
check 'plugin copy records the MCP server source' test -s "$home/.local/share/transcribe-for-agents/transcriber-mcp.source"
check 'MCP build progress stays off stdout' test -z "$(plugin_mcp 2>/dev/null)"
printf 'stale\n' > "$home/.local/share/transcribe-for-agents/transcriber-mcp.source"
check 'plugin copy rebuilds a stale MCP server' \
  grep -q 'building the MCP server' <<<"$(plugin_mcp 2>&1 >/dev/null)"
check 'an up-to-date MCP server is not rebuilt' \
  not grep -q 'building the MCP server' <<<"$(plugin_mcp 2>&1 >/dev/null)"

# Embedded viewer updates must rebuild the server even when Go sources match.
copy_plugin="$work/plugin with spaces"
cp -R "$repo/plugins/transcription-agent-cli" "$copy_plugin"
new_home
copy_mcp() { env -i HOME="$home" PATH="$fakes:/usr/bin:/bin" "$copy_plugin/bin/transcriber-cli" mcp; }
quiet copy_mcp
printf '\n<!-- viewer update fixture -->\n' >> "$copy_plugin/mcp-server/viewer.html"
check 'viewer-only plugin updates rebuild the MCP server' \
  grep -q 'building the MCP server' <<<"$(copy_mcp 2>&1 >/dev/null)"

printf '\n%d failure(s)\n' "$failures"
((failures == 0))
