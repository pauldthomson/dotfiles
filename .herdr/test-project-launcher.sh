#!/bin/bash
# Exercise duplicate launches without touching the running Herdr server.
set -euo pipefail
script_dir=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d)
tmp=$(cd "$tmp" && pwd -P)
trap 'rm -rf -- "$tmp"' EXIT
export HOME="$tmp/home"
export TEST_PROJECT="$HOME/repos/example.test/org/project" TEST_LOG="$tmp/actions"
mkdir -p "$tmp/bin" "${TEST_PROJECT%/*}"
jj git init --colocate "$TEST_PROJECT" >/dev/null
export HERDR_BIN_PATH="$tmp/bin/herdr" PATH="$tmp/bin:$PATH"
cat >"$tmp/bin/fzf" <<'SH'
#!/bin/bash
# Consume the whole input, like real fzf.
while IFS= read -r line; do :; done
printf 'example.test/org/project\n'
SH
cat >"$tmp/bin/herdr" <<'SH'
#!/bin/bash
printf '%s\n' "$*" >>"$TEST_LOG"
case "$1 $2" in
  'workspace list') printf '{"result":{"workspaces":[{"workspace_id":"existing"}]}}\n' ;;
  'pane list') printf '{"result":{"panes":[{"cwd":"%s"}]}}\n' "$TEST_PROJECT" ;;
  'workspace create')
    if [ "${TEST_FAIL_CREATE:-0}" = 1 ]; then
      echo 'simulated Herdr creation failure' >&2
      exit 42
    fi
    printf '{"result":{"workspace":{"workspace_id":"new"},"tab":{"tab_id":"nvim"},"root_pane":{"pane_id":"editor"}}}\n' ;;
  'tab create') printf '{"result":{"root_pane":{"pane_id":"agent"}}}\n' ;;
  *) : ;;
esac
SH
chmod +x "$tmp/bin/"*
printf -- '-evals\n' | bash "$script_dir/project-workspace" open
[ -d "$TEST_PROJECT-evals/.jj" ]
[ ! -e "$TEST_PROJECT--evals" ]
grep -Fxq 'workspace focus new' "$TEST_LOG"
grep -Fxq 'pane run agent pi' "$TEST_LOG"
# An existing suffix must produce a useful error, not create another workspace.
: >"$TEST_LOG"
if printf 'evals\n' | bash "$script_dir/project-workspace" open 2>"$tmp/error"; then
  echo 'Existing checkout was accepted' >&2; exit 1
fi
grep -Fq 'already exists:' "$tmp/error"
! grep -q '^workspace create' "$TEST_LOG"
# CLI failures survive popup exit in the persistent log; the checkout is kept.
export TEST_FAIL_CREATE=1
if printf 'failure\n' | bash "$script_dir/project-workspace" open 2>"$tmp/error"; then
  echo 'Herdr failure was ignored' >&2; exit 1
fi
[ -d "$TEST_PROJECT-failure/.jj" ]
# tee runs asynchronously; wait briefly for its final log write.
for ((i=0; i<50; i++)); do
  if grep -Fq 'command failed:' "$HOME/.config/herdr/workspace-launcher.log"; then break; fi
  sleep 0.02
done
grep -Fq 'simulated Herdr creation failure' "$HOME/.config/herdr/workspace-launcher.log"
grep -Fq 'command failed:' "$HOME/.config/herdr/workspace-launcher.log"
printf 'Duplicate launch and visible-error regression: OK\n'
