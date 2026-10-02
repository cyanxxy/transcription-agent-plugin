#!/usr/bin/env bash
# Build the Transcription Agent MCPB bundle for one OS (amd64 and arm64 inside).
#   bash mcpb/build.sh darwin|linux <out-dir>
# Writes <out-dir>/transcription-agent-mcp-<os>.mcpb and prints its SHA-256.
# Needs Go, git, and Node.js (for npx @anthropic-ai/mcpb). Set
# TRANSCRIBE_ENGINE_SOURCE to build the engine from a local checkout instead of
# the pinned release.
set -euo pipefail

mcpb_cli='@anthropic-ai/mcpb@2.1.2'

fail() { printf 'mcpb/build.sh: %s\n' "$*" >&2; exit 1; }
[[ $# -eq 2 && ( "$1" == darwin || "$1" == linux ) ]] || fail 'usage: bash mcpb/build.sh darwin|linux <out-dir>'
os="$1"
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mkdir -p "$2"
out="$(cd "$2" && pwd)"
cli_plugin="$repo/plugins/transcription-agent-cli"
wrapper="$cli_plugin/bin/transcriber-cli"

pin() { sed -n "s/^$1='\\(.*\\)'\$/\\1/p" "$wrapper"; }
engine_url="$(pin engine_url)" engine_ref="$(pin engine_ref)" engine_sha="$(pin engine_sha)"
version="$(sed -n 's/^ *"version": *"\([^"]*\)".*/\1/p' "$cli_plugin/.claude-plugin/plugin.json" | head -n 1)"
[[ -n "$engine_ref" && -n "$version" ]] || fail 'could not read the engine pin or plugin version'

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
engine_source="${TRANSCRIBE_ENGINE_SOURCE:-}"
if [[ -z "$engine_source" ]]; then
  git -c advice.detachedHead=false clone --quiet --depth 1 --branch "$engine_ref" "$engine_url" "$work/engine"
  engine_source="$work/engine"
  [[ "$(git -C "$engine_source" rev-parse HEAD)" == "$engine_sha" ]] || fail 'release tag does not match the pinned source commit'
fi

bundle="$work/bundle"
mkdir -p "$bundle/server"
for arch in amd64 arm64; do
  printf 'Building %s/%s...\n' "$os" "$arch"
  (cd "$engine_source" && GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
    go build -trimpath -ldflags="-s -w -X main.version=$engine_ref" -o "$bundle/server/transcriber-cli.bin-$arch" ./cmd/cli)
  (cd "$cli_plugin/mcp-server" && GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
    go build -trimpath -ldflags="-s -w -X main.version=$version" -o "$bundle/server/transcriber-mcp-$arch" .)
done
install -m 755 "$wrapper" "$bundle/server/transcriber-cli"
install -m 755 "$repo/mcpb/launch" "$bundle/server/launch"
cp "$repo/plugins/transcription-agent/assets/icon.png" "$bundle/icon.png"
cp "$repo/LICENSE" "$bundle/LICENSE"
sed -e "s/__VERSION__/$version/" -e "s/__PLATFORM__/$os/" "$repo/mcpb/manifest.json" > "$bundle/manifest.json"

npx --yes "$mcpb_cli" validate "$bundle/manifest.json"
file="$out/transcription-agent-mcp-$os.mcpb"
npx --yes "$mcpb_cli" pack "$bundle" "$file"
if command -v sha256sum >/dev/null 2>&1; then sha256sum "$file"; else shasum -a 256 "$file"; fi
