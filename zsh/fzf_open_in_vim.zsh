# Search a file with fzf inside a Tmux pane and then open it in an editor
fzf_then_open_in_editor() {
  local file=$(fzf-tmux)
  # Open the file if it exists
  if [ -n "$file" ]; then
    # Use the default editor if it's defined, otherwise Vim
    ${EDITOR:-vim} "$file"
  fi
}

zle -N fzf_then_open_in_editor

bindkey "^T" fzf_then_open_in_editor
