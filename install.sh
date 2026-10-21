#!/usr/bin/env bash
# Everything runs inside main, which is called on the last line, so a partial
# download from `curl | bash` never executes a truncated script.
set -euo pipefail

repo_url='https://github.com/cyanxxy/transcription-agent-plugin.git'
# Keep the engine pin in sync with plugins/transcription-agent-cli/scripts/transcriber-cli.
engine_url='https://github.com/cyanxxy/transcription-agent-go.git'
engine_ref='v1.5.0'
engine_sha='ccb5f79fb22d67d57caec30e5c1b781e4cb6e769'

usage() {
  cat <<'USAGE'
Usage: bash install.sh [--agent claude-code|codex|both|none] [--provider gemini|meta|microsoft] [--skip-auth]

Installs or upgrades the transcribe-for-agents skill and Go transcription CLI
for your user account. Re-run it to upgrade; replaced files are backed up.

--agent none installs only the CLI and key; add the skill to any other agent
with: npx skills add cyanxxy/transcription-agent-plugin --skill transcribe-for-agents -g -a <agent>
USAGE
}

fail() { printf 'install: %s\n' "$*" >&2; exit 1; }
require_command() { command -v "$1" >/dev/null 2>&1 || fail "$1 is required"; }
has_tty() { [[ -t 0 ]] || { : < /dev/tty; } 2>/dev/null; }

install_prerequisites() {
  local missing=() answer=''
  command -v go >/dev/null 2>&1 || missing+=(go)
  if ! command -v ffmpeg >/dev/null 2>&1 || ! command -v ffprobe >/dev/null 2>&1; then
    missing+=(ffmpeg)
  fi
  ((${#missing[@]} == 0)) && return
  if command -v brew >/dev/null 2>&1 && has_tty; then
    printf 'Missing %s. Install with Homebrew? [y/N] ' "${missing[*]}" > /dev/tty
    IFS= read -r answer < /dev/tty || true
    if [[ "$answer" == y || "$answer" == Y ]]; then
      # Homebrew and any sudo prompt read from the terminal, never the piped script.
      brew install "${missing[@]}" < /dev/tty
      return
    fi
  fi
  fail "missing ${missing[*]}; install them and run this command again"
}

# is_our_wrapper <path>: true when the file is a transcribe-for-agents wrapper
# from any version of this installer.
is_our_wrapper() {
  [[ -f "$1" && ! -L "$1" ]] && grep -q 'transcribe-for-agents' "$1" && grep -q 'transcriber-cli.bin' "$1"
}

# is_our_skill <dir>: true when the directory holds this skill.
is_our_skill() {
  [[ -d "$1" && ! -L "$1" && -f "$1/SKILL.md" ]] && grep -q '^name: transcribe-for-agents$' "$1/SKILL.md"
}

# The transcription-agent plugin ships the same skill. When it is enabled, a
# standalone copy would make the agent load the skill twice.
plugin_id='transcription-agent@transcription-agent-tools'
claude_plugin_enabled() {
  grep -Eq "\"$plugin_id\"[[:space:]]*:[[:space:]]*true" "$HOME/.claude/settings.json" 2>/dev/null
}
codex_plugin_enabled() {
  awk -v want="[plugins.\"$plugin_id\"]" '
    /^[[:space:]]*\[/ { gsub(/[[:space:]]/, ""); in_section = ($0 == want); next }
    in_section && /^[[:space:]]*enabled[[:space:]]*=[[:space:]]*true/ { found = 1 }
    END { exit !found }
  ' "${CODEX_HOME:-$HOME/.codex}/config.toml" 2>/dev/null
}

# backup <path>: move an installed file or directory out of the way.
# Backups go under the data directory, never beside a skill, so agents do not
# load an old copy as a second skill.
backup() {
  local destination
  destination="$backup_dir/$(date +%Y%m%d-%H%M%S)-$$"
  mkdir -p "$destination"
  mv "$1" "$destination/$2"
  printf 'Backed up previous %s to %s\n' "$2" "$destination/$2"
}

main() {
  local agent='both' provider='gemini' skip_auth=false
  while (($#)); do
    case "$1" in
      --agent) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; agent="$2"; shift 2 ;;
      --provider) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; provider="$2"; shift 2 ;;
      --skip-auth) skip_auth=true; shift ;;
      --help|-h) usage; exit 0 ;;
      *) usage >&2; exit 2 ;;
    esac
  done
  case "$agent" in claude-code|codex|both|none) ;; *) usage >&2; exit 2 ;; esac
  case "$provider" in gemini|meta|microsoft) ;; *) usage >&2; exit 2 ;; esac

  require_command git
  install_prerequisites
  require_command go
  require_command ffmpeg
  require_command ffprobe

  temp_dir="$(mktemp -d)"
  trap 'rm -rf "$temp_dir"' EXIT

  # BASH_SOURCE is empty when the script is piped into bash; only use a local
  # checkout when the installer was run from a file inside one.
  local script_path="${BASH_SOURCE[0]:-}" source_dir="${TRANSCRIBE_INSTALL_SOURCE:-}"
  if [[ -z "$source_dir" && -n "$script_path" && -f "$script_path" &&
        -f "$(dirname "$script_path")/plugins/transcription-agent/skills/transcribe-for-agents/SKILL.md" ]]; then
    source_dir="$(cd "$(dirname "$script_path")" && pwd)"
  fi
  if [[ -z "$source_dir" ]]; then
    printf 'Fetching the skill...\n'
    git clone --quiet --depth 1 "$repo_url" "$temp_dir/plugin"
    source_dir="$temp_dir/plugin"
  fi
  local skill_dir="$source_dir/plugins/transcription-agent/skills/transcribe-for-agents"
  local wrapper="$source_dir/plugins/transcription-agent-cli/scripts/transcriber-cli"
  [[ -f "$skill_dir/SKILL.md" && -f "$wrapper" ]] || fail 'skill or CLI wrapper is missing from the source repository'

  local bin_dir="$HOME/.local/bin"
  local data_dir="${XDG_DATA_HOME:-$HOME/.local/share}/transcribe-for-agents"
  backup_dir="$data_dir/backups"
  local cli="$bin_dir/transcriber-cli"

  # Check every destination before changing anything.
  if [[ -e "$cli" || -L "$cli" ]] && ! cmp -s "$wrapper" "$cli"; then
    is_our_wrapper "$cli" || fail "$cli is not a transcribe-for-agents wrapper (move it before installing)"
  fi
  # targets get the skill; retired paths lose our old copy so the agent does
  # not load the skill twice. Labels name the backups.
  local targets=() target_labels=() retired=() retired_labels=() target i
  local claude_skill="$HOME/.claude/skills/transcribe-for-agents"
  local codex_skill="$HOME/.agents/skills/transcribe-for-agents"
  local codex_legacy_skill="${CODEX_HOME:-$HOME/.codex}/skills/transcribe-for-agents"
  if [[ "$agent" == both || "$agent" == claude-code ]]; then
    if claude_plugin_enabled; then
      printf 'Claude Code: the %s plugin provides the skill; skipping the standalone copy.\n' "$plugin_id"
      retired+=("$claude_skill"); retired_labels+=(claude-skill)
    else
      targets+=("$claude_skill"); target_labels+=(claude-skill)
    fi
  fi
  if [[ "$agent" == both || "$agent" == codex ]]; then
    # Codex reads user skills from ~/.agents/skills; ~/.codex/skills is deprecated.
    retired+=("$codex_legacy_skill"); retired_labels+=(codex-legacy-skill)
    if codex_plugin_enabled; then
      printf 'Codex: the %s plugin provides the skill; skipping the standalone copy.\n' "$plugin_id"
      retired+=("$codex_skill"); retired_labels+=(codex-skill)
    else
      targets+=("$codex_skill"); target_labels+=(codex-skill)
    fi
  fi
  for target in ${targets[@]+"${targets[@]}"}; do
    if [[ -L "$target" ]]; then
      diff -qr "$skill_dir" "$target" >/dev/null ||
        fail "$target is a symlink managed by another installer (update it there, or remove it and run this again)"
    elif [[ -e "$target" ]]; then
      is_our_skill "$target" || fail "$target is not the transcribe-for-agents skill (move it before installing)"
    fi
  done

  local engine_source="${TRANSCRIBE_ENGINE_SOURCE:-}" engine_version="$engine_ref"
  if [[ -z "$engine_source" ]]; then
    printf 'Fetching transcription engine %s...\n' "$engine_ref"
    git -c advice.detachedHead=false clone --quiet --depth 1 --branch "$engine_ref" "$engine_url" "$temp_dir/engine"
    engine_source="$temp_dir/engine"
    [[ "$(git -C "$engine_source" rev-parse HEAD)" == "$engine_sha" ]] || fail 'release tag does not match the pinned source commit'
  else
    engine_version="$(git -C "$engine_source" describe --tags --always --dirty 2>/dev/null || printf 'dev')"
  fi
  [[ -f "$engine_source/go.mod" ]] || fail 'transcription engine source is missing go.mod'

  printf 'Building transcriber-cli %s...\n' "$engine_version"
  (
    cd "$engine_source"
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$engine_version" -o "$temp_dir/transcriber-cli.bin" ./cmd/cli
  )
  # The MCP server that `transcriber-cli mcp` runs.
  local mcp_source="$source_dir/plugins/transcription-agent-cli/mcp-server" mcp_version
  mcp_version="$(sed -n 's/^ *"version": *"\([^"]*\)".*/\1/p' "$mcp_source/../.claude-plugin/plugin.json" | head -n 1)"
  printf 'Building the MCP server %s...\n' "$mcp_version"
  (
    cd "$mcp_source"
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$mcp_version" -o "$temp_dir/transcriber-mcp" .
  )

  mkdir -p "$data_dir" "$bin_dir"
  install -m 755 "$temp_dir/transcriber-cli.bin" "$data_dir/transcriber-cli.bin"
  install -m 755 "$temp_dir/transcriber-mcp" "$data_dir/transcriber-mcp"
  rm -f "$data_dir/transcriber-mcp.source"
  if [[ -e "$cli" ]] && ! cmp -s "$wrapper" "$cli"; then backup "$cli" transcriber-cli; fi
  install -m 755 "$wrapper" "$cli"
  for ((i = 0; i < ${#targets[@]}; i++)); do
    target="${targets[i]}"
    if [[ ! -L "$target" ]]; then
      if [[ -e "$target" ]] && ! diff -qr "$skill_dir" "$target" >/dev/null; then
        backup "$target" "${target_labels[i]}"
      fi
      if [[ ! -e "$target" ]]; then
        mkdir -p "$(dirname "$target")"
        cp -R "$skill_dir" "$target"
      fi
    fi
    printf 'Installed skill: %s\n' "$target"
  done
  for ((i = 0; i < ${#retired[@]}; i++)); do
    target="${retired[i]}"
    if [[ -L "$target" ]]; then
      printf 'Note: %s is a symlink from another installer and duplicates this skill. Remove it if the agent lists the skill twice.\n' "$target"
    elif is_our_skill "$target"; then
      backup "$target" "${retired_labels[i]}"
    fi
  done

  if [[ "$skip_auth" == false ]]; then
    if "$cli" auth status "$provider" >/dev/null 2>&1; then
      "$cli" auth status "$provider"
    elif has_tty; then
      "$cli" auth set "$provider" ||
        printf 'The %s key was not saved. Run %s auth set %s to add it.\n' "$provider" "$cli" "$provider"
    else
      printf 'No terminal available for the API key. Run %s auth set %s later.\n' "$cli" "$provider"
    fi
  fi

  printf '\nReady. Start a new agent session, give it an audio file, and ask it to transcribe it.\n'
  printf 'CLI: %s\n' "$cli"
  if [[ "$agent" == none ]]; then
    printf 'Add the skill to your agent with:\n  npx skills add cyanxxy/transcription-agent-plugin --skill transcribe-for-agents -g -a <agent>\n'
  fi
  # shellcheck disable=SC2016 # print a literal $PATH for the user to paste
  case ":$PATH:" in
    *":$bin_dir:"*) ;;
    *) printf 'Note: %s is not on your PATH. Add it (for example in ~/.zshrc or ~/.bashrc):\n  export PATH="%s:%s"\n' "$bin_dir" "$bin_dir" '$PATH' ;;
  esac
  printf 'ffmpeg and ffprobe must remain on PATH when the agent runs.\n'
}

main "$@"
