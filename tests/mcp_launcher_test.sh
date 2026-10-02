#!/usr/bin/env bash
# Desktop launches do not inherit a login-shell PATH. Exercise real resolution
# and exec behavior using a fake CLI, without provider calls or builds.
set -euo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
launcher="$repo/plugins/transcription-agent-mcp/scripts/launch-mcp.sh"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
task_home="$work/home with spaces"
mkdir -p "$task_home/.local/bin" "$work/path" "$work/override dir"
cat > "$work/fake-cli" <<'CLI'
#!/bin/sh
printf '%s\n' "$0" "$@"
exit "${FAKE_EXIT:-0}"
CLI
chmod +x "$work/fake-cli"
cp "$work/fake-cli" "$task_home/.local/bin/transcriber-cli"
cp "$work/fake-cli" "$work/path/transcriber-cli"
override="$work/override dir/cli; literal"
cp "$work/fake-cli" "$override"

run() { env -i HOME="$task_home" PATH="$work/path:/usr/bin:/bin" "$@" "$launcher"; }
expect() { [[ "$1" == "$2" ]] || { printf 'unexpected output: %s\n' "$2" >&2; exit 1; }; }
expect "$task_home/.local/bin/transcriber-cli"$'\nmcp' "$(run)"
expect "$override"$'\nmcp' "$(run TRANSCRIBER_CLI="$override")"
rm "$task_home/.local/bin/transcriber-cli"
expect "$work/path/transcriber-cli"$'\nmcp' "$(run)"
expect "$work/path/transcriber-cli"$'\nmcp' "$(env -i PATH="$work/path:/usr/bin:/bin" "$launcher")"
if run TRANSCRIBER_CLI=relative > "$work/stdout" 2> "$work/stderr"; then exit 1; fi
[[ ! -s "$work/stdout" ]] && grep -q 'absolute path' "$work/stderr"
if run TRANSCRIBER_CLI="$work/missing" > "$work/stdout" 2> "$work/stderr"; then exit 1; fi
[[ ! -s "$work/stdout" ]] && grep -q 'not executable' "$work/stderr"
if env -i HOME="$task_home" PATH=/usr/bin:/bin "$launcher" > "$work/stdout" 2> "$work/stderr"; then exit 1; fi
[[ ! -s "$work/stdout" ]] && grep -q 'install transcriber-cli' "$work/stderr"
status=0
run FAKE_EXIT=7 >/dev/null || status=$?
[[ "$status" == 7 ]]
printf 'ok   MCP launcher resolution, literal paths, failures, and exit propagation\n'
