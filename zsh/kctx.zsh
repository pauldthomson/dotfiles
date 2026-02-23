kctx() {
  if ! command -v kubectl >/dev/null 2>&1; then
    echo "kubectl is not installed"
    return 1
  fi

  if [[ -n "$1" ]]; then
    kubectl config use-context "$1"
    return $?
  fi

  if ! command -v fzf >/dev/null 2>&1; then
    echo "fzf is not installed (use: kctx <context-name>)"
    return 1
  fi

  local current_context selected_context
  current_context="$(kubectl config current-context 2>/dev/null)"

  selected_context="$(kubectl config get-contexts -o name 2>/dev/null \
    | awk -v current_context="$current_context" '{print ($0 == current_context ? "* " : "  ") $0}' \
    | fzf --prompt='kube context > ' --height=40% --reverse --header='Enter: switch context · Esc: cancel' \
    | sed 's/^[* ]*//')"

  [[ -z "$selected_context" ]] && return 0

  kubectl config use-context "$selected_context"
}

alias kc='kctx'
