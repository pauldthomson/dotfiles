# Source after compinit. kill closes only the current Rex session.
_prex() {
  local -a values
  local state
  if [[ ${words[2]} == kill || ${words[2]} == orphans ]]; then
    return
  fi
  if [[ ${words[2]} == picker || ${words[2]} == popup ]]; then
    compadd -- tab kill
    return
  fi
  _arguments \
    '--client[connected Rex app ID or label]:client:' \
    '--server[local Rex Unix socket endpoint]:endpoint:' \
    '--help[show usage]' \
    '*:project or command:->projects'
  if [[ $state == projects ]]; then
    values=("${(@f)$(command prex completion 2>/dev/null)}")
    compadd -- kill picker popup recover orphans "${values[@]}"
  fi
}
compdef _prex prex prx
