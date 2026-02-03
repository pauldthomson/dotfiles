---
name: git-clone
description: Clone git repositories using standard CLI conventions and directory layout. Use when a user asks to clone a repo or set up a local checkout.
license: Proprietary
compatibility: Requires git, gh (optional), and jj
metadata:
  author: paulthomson
  version: "1.0"
---

# Purpose
Clone repositories in a consistent directory structure and ensure Jujutsu is initialized.

# Default conventions
- Prefer `gh repo clone` when given a GitHub URL or `owner/repo`.
- Otherwise use `git clone`.
- Clone destination: `~/repos/<host>/<org>/<repo>` (e.g., `~/repos/github.com/nc-helix/helix-plugins`).

# Steps
1. Parse the repo identifier (URL or `org/repo`) and determine `host`, `org`, and `repo`.
2. Construct the destination path `~/repos/<host>/<org>/<repo>`.
3. If the parent directories do not exist, create them.
4. Clone:
   - GitHub URL or `org/repo`: `gh repo clone <repo> "<dest>"`
   - Otherwise: `git clone <url> "<dest>"`
5. After cloning, ensure Jujutsu is initialized:
   - If `.jj/` is missing, run `jj git init --colocate` inside the repo.

# Examples
- Input: "Clone nc-helix/helix-plugins"
  - Destination: `~/repos/github.com/nc-helix/helix-plugins`
  - Command: `gh repo clone nc-helix/helix-plugins "~/repos/github.com/nc-helix/helix-plugins"`
- Input: "Clone https://gitlab.com/acme/tools"
  - Destination: `~/repos/gitlab.com/acme/tools`
  - Command: `git clone https://gitlab.com/acme/tools "~/repos/gitlab.com/acme/tools"`

# Edge cases
- If the destination already exists, warn and ask whether to reuse it.
- If `gh` is missing, fall back to `git clone`.
