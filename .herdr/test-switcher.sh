#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

python3 - <<'PY'
import json
import subprocess
import tomllib

with open('config.toml', 'rb') as f:
    config = tomllib.load(f)
for key in ('prefix+j', 'prefix+k'):
    command = next(c['command'] for c in config['keys']['command'] if c['key'] == key)
    assert 'fzf --ansi ' in command
    assert '/switcher-rows.jq' in command
    subprocess.run(['bash', '-n'], input=command, text=True, check=True)

states = {
    'working': ('●', '249;226;175'),
    'blocked': ('●', '243;139;168'),
    'done': ('●', '148;226;213'),
    'idle': ('○', '166;227;161'),
    'unknown': ('·', '108;112;134'),
    None: ('·', '108;112;134'),
}
for collection in ('workspaces', 'tabs'):
    rows = []
    for i, state in enumerate(states):
        row = {'label': f'repo {i} · evals', 'workspace_id': f'w{i}', 'agent_status': state}
        if collection == 'tabs':
            row['tab_id'] = f'w{i}:t2'
        rows.append(row)
    result = subprocess.run(
        ['jq', '-r', '-f', 'switcher-rows.jq'],
        input=json.dumps({'result': {collection: rows}}),
        capture_output=True, text=True, check=True,
    )
    for i, (line, (icon, color)) in enumerate(zip(result.stdout.splitlines(), states.values(), strict=True)):
        label, identifier = line.split('\t')
        assert label == f'\x1b[38;2;{color}m{icon}\x1b[0m repo {i} · evals'
        assert identifier == (f'w{i}:t2' if collection == 'tabs' else f'w{i}')
print('Switcher status icons, labels, IDs, and command syntax: OK')
PY
