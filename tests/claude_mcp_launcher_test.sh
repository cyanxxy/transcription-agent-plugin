#!/usr/bin/env bash
# No speech-provider requests or real credentials: exercise the Claude launcher
# against an asserting wrapper in a plugin path containing spaces.
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
python3 - "$repo_root/plugins/transcription-agent-cli/mcp/launch" <<'PY'
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

source = Path(sys.argv[1])
providers = ['GEMINI_API_KEY', 'META_API_KEY', 'AZURE_SPEECH_KEY', 'AZURE_SPEECH_ENDPOINT']
options = ['TRANSCRIBER_PLUGIN_' + name for name in providers]

with tempfile.TemporaryDirectory(prefix='claude launcher test ') as directory:
    root = Path(directory)
    (root / 'mcp').mkdir()
    (root / 'bin').mkdir()
    shutil.copy2(source, root / 'mcp' / 'launch')
    wrapper = root / 'bin' / 'transcriber-cli'
    wrapper.write_text('''#!/usr/bin/env bash
set -euo pipefail
[[ "$#" -eq 1 && "$1" == mcp ]]
[[ "${GEMINI_API_KEY:-}" == "${EXPECT_GEMINI:-}" ]]
[[ "${META_API_KEY:-}" == "${EXPECT_META:-}" ]]
[[ "${AZURE_SPEECH_KEY:-}" == "${EXPECT_AZURE:-}" ]]
[[ "${AZURE_SPEECH_ENDPOINT:-}" == "${EXPECT_ENDPOINT:-}" ]]
[[ -z "${TRANSCRIBER_PLUGIN_GEMINI_API_KEY+x}" ]]
[[ -z "${TRANSCRIBER_PLUGIN_META_API_KEY+x}" ]]
[[ -z "${TRANSCRIBER_PLUGIN_AZURE_SPEECH_KEY+x}" ]]
[[ -z "${TRANSCRIBER_PLUGIN_AZURE_SPEECH_ENDPOINT+x}" ]]
printf 'ok\\n'
exit "${EXPECT_EXIT:-0}"
''')
    wrapper.chmod(0o755)
    base = {key: value for key, value in os.environ.items()
            if key not in providers and not key.startswith('TRANSCRIBER_PLUGIN_')
            and not key.startswith('EXPECT_')}
    inherited = dict(zip(providers, ['inherited-gemini', 'inherited-meta', 'inherited-azure', 'https://inherited.invalid']))
    expected = dict(zip(['EXPECT_GEMINI', 'EXPECT_META', 'EXPECT_AZURE', 'EXPECT_ENDPOINT'], inherited.values()))
    configured = dict(zip(options, ['configured-gemini', 'configured-meta', 'configured-azure', 'https://configured.invalid']))
    configured_expected = dict(zip(expected, configured.values()))
    marker = root / 'must-not-execute'
    literal = f'literal $(touch "{marker}") `touch "{marker}"` " quote'
    cases = [
        ('unset options', {}),
        ('empty options preserve all inherited credentials', inherited | expected | dict.fromkeys(options, '')),
        ('configured options override all inherited credentials', inherited | configured | configured_expected),
        ('partial options preserve other inherited credentials', inherited | expected | {
            'TRANSCRIBER_PLUGIN_GEMINI_API_KEY': 'partial-gemini', 'EXPECT_GEMINI': 'partial-gemini'}),
        ('shell characters stay literal and exit code propagates', {
            'TRANSCRIBER_PLUGIN_GEMINI_API_KEY': literal, 'EXPECT_GEMINI': literal, 'EXPECT_EXIT': '13'}),
    ]
    for label, overrides in cases:
        result = subprocess.run([str(root / 'mcp' / 'launch')], env=base | overrides,
                                capture_output=True, text=True, timeout=10)
        assert result.returncode == int(overrides.get('EXPECT_EXIT', 0)), f'{label}: unexpected exit {result.returncode}'
        assert result.stdout == 'ok\n', f'{label}: unexpected stdout'
        assert result.stderr == '', f'{label}: unexpected stderr'
        assert not marker.exists(), f'{label}: shell value executed'
        print(f'PASS: {label}')

print('Claude MCP launcher tests passed (plugin path contains spaces).')
PY
