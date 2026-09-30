# Match Herdr's default dots and Catppuccin status colours.
def status_icon:
  if . == "working" then "\u001b[38;2;249;226;175m●"
  elif . == "blocked" then "\u001b[38;2;243;139;168m●"
  elif . == "done" then "\u001b[38;2;148;226;213m●"
  elif . == "idle" then "\u001b[38;2;166;227;161m○"
  else "\u001b[38;2;108;112;134m·"
  end + "\u001b[0m";

(.result.workspaces // .result.tabs)[] |
[ ((.agent_status | status_icon) + " " + .label),
  (.tab_id // .workspace_id) ] | @tsv
