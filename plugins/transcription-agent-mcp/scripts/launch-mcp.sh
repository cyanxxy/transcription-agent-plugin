#!/bin/sh
# Resolve an installed CLI without depending on a desktop app's login-shell PATH.
set -eu

if [ -n "${TRANSCRIBER_CLI:-}" ]; then
  case "$TRANSCRIBER_CLI" in
    /*) ;;
    *) printf '%s\n' 'transcription-agent: TRANSCRIBER_CLI must be an absolute path' >&2; exit 1 ;;
  esac
  cli="$TRANSCRIBER_CLI"
elif [ -n "${HOME:-}" ] && [ -x "$HOME/.local/bin/transcriber-cli" ]; then
  cli="$HOME/.local/bin/transcriber-cli"
elif command -v transcriber-cli >/dev/null 2>&1; then
  cli="$(command -v transcriber-cli)"
else
  printf '%s\n' 'transcription-agent: install transcriber-cli first: https://github.com/cyanxxy/transcription-agent-plugin#the-installer' >&2
  exit 1
fi

[ -f "$cli" ] && [ -x "$cli" ] || {
  printf '%s\n' "transcription-agent: CLI is not executable: $cli" >&2
  exit 1
}
exec "$cli" mcp
