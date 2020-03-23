# Search a file with fzf inside a Tmux pane and then open it in an editor
fzf_then_open_in_editor() {
  local file=$(fzf-tmux --preview 'bat --style=numbers --color=always {}')
  # Open the file if it exists
  if [ -n "$file" ]; then
    # Use the default editor if it's defined, otherwise Vim
    ${EDITOR:-vim} "$file"
  fi
}

zle -N fzf_then_open_in_editor

# This should work, needs more investigation
# into fzf-tmux and zsh
# bindkey "^T" fzf_then_open_in_editor
bindkey -s '^T' 'fzf_then_open_in_editor\n'
