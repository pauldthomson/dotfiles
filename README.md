# dotfiles

My environment setup.

## Zsh
- Uses Powerlevel10k with custom `jj` status segment.
- Prompt hides VCS prefix/icon and directory icon.
- `zsh-async` enables async status updates; otherwise prompt uses a synchronous fallback.

## Tmux
- Prefix is `C-Space`.
- `prefix + j`: switch sessions (fzf popup).
- `prefix + k`: switch windows (fzf popup).
- `prefix + X`: kill one-or-many sessions (TAB multi-select).
- `prefix + C-p`: run `ptmux` project/session launcher.
- OSC passthrough is disabled to prevent terminal responses showing in editors.

## Ghostty
- `Shift+Enter` sends a literal newline + carriage return for apps that expect CRLF input.
- `custom-shader-animation = always` keeps cursor shaders animating through focus/mode changes to avoid stuck cursor-outline artifacts.
- Cursor shader stack uses `cursor_tail.glsl` then `ripple_cursor.glsl`; `cursor_tail.glsl` includes an animation-state guard to prevent a lower-left cursor-outline artifact seen when launching full-screen TUIs like `pi`.

## Pi
- Stores shared Pi resources in `pi-agent/` (instead of project `.pi/`) to avoid duplicate extension loading when symlinked into `~/.pi/agent/`.
- Local agent skills are kept in `skills/` (for example `git-clone` and `excalidraw-mcp-app`).
- Adds a `web_fetch` extension for fetching URLs with truncation and temp file fallback.
- Adds a `review` extension (based on mitsuhiko/agent-stuff) for interactive code review flows (`/review`, `/review bookmark <name>`, `/end-review`) using `jj` workflows.
- Adds an `auto-qna` extension that detects multiple user-directed questions in final assistant responses, opens an interactive Q&A TUI, and sends captured answers back as a structured JSON follow-up user message (`/auto-qna [on|off|status]`).
- Adds a `jj-footer` extension that replaces git `(detached)` branch display with jj-aware status (`jj:<bookmark>` or `jj:@<change-id>`), while still falling back to git when outside jj repos.
- Uses a custom `catppuccin` theme (Mocha palette) defined in `pi-agent/themes/catppuccin.json`, with neutral tool-call card backgrounds and softer diff colors to better match the palette.

### Pi symlink setup
Run this after cloning (or anytime you need to re-point your global Pi config):

```bash
./pi-agent/setup-symlinks.sh
```

Agent handoff snippet:

```text
Task: Re-link global Pi resources to this dotfiles repo.
Run: ./pi-agent/setup-symlinks.sh
Expected result:
  ~/.pi/agent/extensions   -> <repo>/pi-agent/extensions
  ~/.pi/agent/settings.json -> <repo>/pi-agent/settings.json
  ~/.pi/agent/themes       -> <repo>/pi-agent/themes
```

Manual fallback:

```bash
mkdir -p ~/.pi/agent
ln -sfn "$PWD/pi-agent/extensions" ~/.pi/agent/extensions
ln -sfn "$PWD/pi-agent/settings.json" ~/.pi/agent/settings.json
ln -sfn "$PWD/pi-agent/themes" ~/.pi/agent/themes
```

## Neovim
- Kotlin LSP is auto-enabled via `mason-lspconfig` (`kotlin_lsp`) and decompiles `jar:`/`jrt:` sources on demand so go-to-definition opens readable buffers.
- `JAVA_HOME` follows the newest installed JDK (via `java_home` with a Homebrew fallback); keep a 21+ JDK installed for JDTLS.
- Terraform LSP prefers the nearest Terraform module directory as its root so monorepo roots (and large folders like `node_modules/`) don’t get indexed unnecessarily.
- Plugin specs follow lazy.nvim guidance: prefer `opts` for setup, and reserve `config` for custom/non-standard initialization.
- Regression checks for lazy loading and Terraform LSP behavior are documented in `nvim/REGRESSION_SPEC.md`.
- Startup-heavy plugins are lazy-loaded (InsertEnter/BufRead/command triggers), and Telescope keymaps defer `require(...)` until invocation to keep empty-start startup fast.
