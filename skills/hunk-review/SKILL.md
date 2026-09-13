---
name: hunk-review
description: Review the current changes with Hunk in a new Herdr tab or tmux window.
license: Proprietary
compatibility: Requires hunk and either tmux or Herdr
metadata:
  author: paulthomson
  version: "1.1"
---

# Prompt
Before opening anything new, check for an existing Hunk review session/window for the current repo:

1. Run `hunk session list --json` and look for an active session whose repo root is the current working directory.
2. If one exists, reuse it. Reload it with `hunk session reload <session-id> -- diff` if the contents may be stale.
3. If no matching Hunk session exists, first detect whether this agent is in a Herdr-managed pane:
   ```bash
   test "${HERDR_ENV:-}" = 1
   ```
   - When it succeeds, create a focused Herdr tab in the current workspace, preserving the repository working directory:
     ```bash
     herdr tab create --workspace "$HERDR_WORKSPACE_ID" --cwd "$PWD" --label hunk-review --focus
     ```
     Read the new root-pane ID from the JSON response, then run `hunk diff` in that pane with `herdr pane run <root-pane-id> "hunk diff"`. Do not use tmux in this case.
   - When it fails, run `hunk diff` in a new tmux window in the current session.
4. Run `hunk skill path` to get the Hunk skill path, then load that Hunk skill and use it to review.
