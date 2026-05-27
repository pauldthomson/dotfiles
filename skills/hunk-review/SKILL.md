---
name: hunk-review
description: Review the current changes with Hunk in a new tmux window.
license: Proprietary
compatibility: Requires tmux and hunk
metadata:
  author: paulthomson
  version: "1.0"
---

# Prompt
Before opening anything new, check for an existing Hunk review session/window for the current repo:

1. Run `hunk session list --json` and look for an active session whose repo root is the current working directory.
2. If one exists, reuse it. Reload it with `hunk session reload <session-id> -- diff` if the contents may be stale.
3. If no matching Hunk session exists, run `hunk diff` in a new tmux window in the current session.
4. Run `hunk skill path` to get the Hunk skill path, then load that Hunk skill and use it to review.
