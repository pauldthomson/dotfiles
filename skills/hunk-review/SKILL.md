---
name: hunk-review
description: Review the current changes with Hunk in a new Rex tab or tmux window.
license: Proprietary
compatibility: Requires hunk and either tmux or Rex
metadata:
  author: paulthomson
  version: "1.2"
---

# Prompt
Before opening anything new, check for an existing Hunk review session/window for the current repo:

1. Run `hunk session list --json` and look for an active session whose repo root is the current working directory.
2. If one exists, reuse it. Reload it with `hunk session reload <session-id> -- diff` if the contents may be stale.
3. If no matching Hunk session exists, first detect whether this agent is in a Rex session:
   ```bash
   test -n "${REX_SESSION:-}"
   ```
   - When it succeeds, load the `rex` skill and create a focused Rex tab in that session, preserving the repository working directory:
     ```bash
     rex window new -s "$REX_SESSION" hunk-review -c "$PWD" --json -- hunk diff
     ```
     Save the returned window and block IDs. Do not use `--wait` for this interactive viewer or use tmux in this case. If the session is unavailable, follow the Rex skill's connection recovery guidance rather than retrying blindly.
   - When it fails, run `hunk diff` in a new tmux window in the current session.
4. Run `hunk skill path` to get the Hunk skill path, then load that Hunk skill and use it to review.
