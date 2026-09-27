#!/bin/bash
# Regression: JJ may leave the canonical workspace root blank in workspace list.
set -euo pipefail
script_dir=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
jj git init --colocate "$tmp/main" >/dev/null
jj -R "$tmp/main" workspace add "$tmp/main-herdr-test" --name main-herdr-test >/dev/null
mkdir "$tmp/bin"
# Simulate older repositories where JJ reports `default` without a root path.
export REAL_JJ=$(command -v jj)
cat >"$tmp/bin/jj" <<'SH'
#!/bin/bash
if [[ " $* " = *' workspace list '* ]]; then
  "$REAL_JJ" "$@" | awk -F '\t' 'BEGIN {OFS="\t"} $1 == "default" {$2=""} {print}'
else
  "$REAL_JJ" "$@"
fi
SH
chmod +x "$tmp/bin/jj"
# Avoid touching the running Herdr session or invoking a real fzf popup.
export TEST_MAIN="$tmp/main" TEST_SECONDARY="$tmp/main-herdr-test" TEST_LOG="$tmp/actions"
export HERDR_ACTIVE_WORKSPACE_ID=wTest HERDR_BIN_PATH="$tmp/bin/herdr"
export PATH="$tmp/bin:$PATH"
[ -z "$(jj -R "$tmp/main-herdr-test" workspace list -T 'name ++ "\t" ++ root ++ "\n"' | awk -F '\t' '$1 == "default" {print $2}')" ]
cat >"$tmp/bin/herdr" <<'SH'
#!/bin/bash
case "$1 $2" in
  'pane list')
    printf '{"result":{"panes":[{"cwd":"%s"}]}}\n' "$TEST_SECONDARY" ;;
  'workspace list')
    printf '{"result":{"workspaces":[{"workspace_id":"wTest"}]}}\n' ;;
  'workspace close')
    printf 'close:%s\n' "$3" >>"$TEST_LOG" ;;
  *) exit 1 ;;
esac
SH
cat >"$tmp/bin/fzf" <<'SH'
#!/bin/bash
IFS= read -r first
printf 'choices:%s\n' "$first" >>"$TEST_LOG"
printf '%s\n' "$first"
SH
chmod +x "$tmp/bin/herdr" "$tmp/bin/fzf"
bash "$script_dir/project-workspace" close
grep -Fxq 'choices:Keep checkout' "$TEST_LOG" || { echo 'Clean checkout prompt was skipped' >&2; exit 1; }
grep -Fxq 'close:wTest' "$TEST_LOG"
[ -d "$TEST_SECONDARY/.jj" ]
# A nonempty JJ working-copy commit must not even offer deletion.
: >"$TEST_LOG"
printf 'unsaved work\n' >"$TEST_SECONDARY/important.txt"
bash "$script_dir/project-workspace" close
grep -Fxq 'choices:Keep checkout (dirty; deletion disabled)' "$TEST_LOG"
grep -Fxq 'close:wTest' "$TEST_LOG"
[ -f "$TEST_SECONDARY/important.txt" ]
: >"$TEST_LOG"
if bash "$script_dir/project-workspace" cleanup "$TEST_MAIN" "$TEST_SECONDARY" main-herdr-test wTest 2>/dev/null; then
  echo 'Detached cleanup accepted a dirty checkout' >&2
  exit 1
fi
[ ! -s "$TEST_LOG" ] && [ -f "$TEST_SECONDARY/important.txt" ]
printf 'Cleanup prompt and dirty-check regression: OK\n'
