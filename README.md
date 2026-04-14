# dotfiles

My environment setup.

## Fresh machine bootstrap
- Curated Homebrew manifests live at `Brewfile` (baseline) and `Brewfile.optional` (feature-specific extras).
- They intentionally exclude tools managed by Neovim plugins/package managers such as Mason.nvim, Lazy.nvim, TPM, and Krew.
- Install the baseline packages with:

```bash
brew bundle --file="$PWD/Brewfile"
```

- Install the optional extras with:

```bash
brew bundle --file="$PWD/Brewfile.optional"
```

- No npm globals are required for the tracked config itself. If you want the Pi CLI used by the tmux/agent workflow, install it with:

```bash
npm install -g @mariozechner/pi-coding-agent
```

- The local `ptmux` helper is referenced by tmux, zsh, and Neovim bindings; build/install it after Homebrew bootstrap:

```bash
(cd ptmux && go install .)
```

## Zsh
- Uses Powerlevel10k with custom `jj` status segment.
- Prompt hides VCS prefix/icon and directory icon.
- `zsh-async` enables async status updates; otherwise prompt uses a synchronous fallback.
- Startup is tuned to avoid expensive per-shell work: the Oh My Zsh `nvm` plugin is lazy-loaded (including `pi` as an explicit lazy trigger), kubectl completions are cached at `${XDG_CACHE_HOME:-$HOME/.cache}/zsh/kubectl-completion.zsh`, omz completion dumps are cached at `${XDG_CACHE_HOME:-$HOME/.cache}/oh-my-zsh/.zcompdump-${ZSH_VERSION}`, auto-update checks are disabled, and prompt config is sourced once.
- `kctx` (defined in `zsh/kctx.zsh`) opens an `fzf` picker for kube contexts (marks current context with `*`) and runs `kubectl config use-context` on selection. It is loaded via Oh My Zsh custom config (`~/.oh-my-zsh/custom/kctx.zsh` symlink). You can also run `kctx <context-name>` directly. `kc` is an alias for `kctx`.

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

## Delta
- Includes a Catppuccin Mocha theme at `delta/themes.config` (palette-aligned with https://catppuccin.com/palette/).

## Pi
- Stores shared Pi resources in `pi-agent/` (instead of project `.pi/`) to avoid duplicate extension loading when symlinked into `~/.pi/agent/`.
- Local agent skills are kept in `skills/` (for example `git-clone`, `excalidraw-mcp-app`, `chrome-devtools-mcp`, and `pr-summary`).
- Adds a `web_fetch` extension for fetching URLs with truncation and temp file fallback.
- Adds a `review` extension (based on mitsuhiko/agent-stuff) for interactive code review flows (`/review`, `/review bookmark <name>`, `/end-review`) using `jj` workflows.
- Adds an `auto-qna` extension that detects explicit user-directed clarification questions in final assistant responses (e.g. prompts containing “you/your” or “should I…”), groups numbered answer choices under the same question when present, opens an interactive Q&A TUI with selectable options or freeform overrides, and sends captured answers back as a structured JSON follow-up user message (`/auto-qna [on|off|status]`).
- Includes regression tests for auto-qna question extraction in `pi-agent/extensions/auto-qna/question-extractor.test.ts` (run with `node --test pi-agent/extensions/auto-qna/question-extractor.test.ts`).
- Adds a `pr-summary-interview` extension command (`/pr-summary`) that runs an interactive interview with suggested defaults for each PR section, then sends structured answers to `pr-summary` skill generation (the command itself requires interactive Pi UI; without payload, the skill falls back to one-question-at-a-time chat interview).
- Includes a `chrome-devtools-mcp` skill that wraps `npx chrome-devtools-mcp@latest --autoConnect --channel=beta` for browser automation, inspection, and performance analysis from Pi.
- Adds a `jj-footer` extension that replaces git `(detached)` branch display with jj-aware status (`jj:<bookmark>` or `jj:@<change-id>`) while keeping the default footer layout/metrics (session name, token/cache totals, cost, context usage, model, extension statuses).
- Uses a custom `catppuccin` theme (Mocha palette) defined in `pi-agent/themes/catppuccin.json`, with neutral tool-call card backgrounds and softer diff colors to better match the palette.

### Pi symlink setup
Run this after cloning (or anytime you need to re-point your global Pi config):

```bash
./pi-agent/setup-symlinks.sh
```

Agent handoff snippet:

```text
Task: Re-link global Pi resources and local agent skills to this dotfiles repo.
Run: ./pi-agent/setup-symlinks.sh
Expected result:
  ~/.pi/agent/extensions            -> <repo>/pi-agent/extensions
  ~/.pi/agent/themes                -> <repo>/pi-agent/themes
  ~/.agents/skills/<skill-name>     -> <repo>/skills/<skill-name> (for each skill dir with SKILL.md)
  (removes stale ~/.agents/skills links that no longer exist in <repo>/skills/)

```

Manual fallback:

```bash
mkdir -p ~/.pi/agent
mkdir -p ~/.agents/skills
ln -sfn "$PWD/pi-agent/extensions" ~/.pi/agent/extensions
ln -sfn "$PWD/pi-agent/themes" ~/.pi/agent/themes
for skill_dir in "$PWD/skills"/*/; do
  [ -f "${skill_dir}SKILL.md" ] || continue
  skill_name="$(basename "$skill_dir")"
  ln -sfn "$skill_dir" "$HOME/.agents/skills/$skill_name"
done
```


## Neovim
- Kotlin LSP is auto-enabled via `mason-lspconfig` (`kotlin_lsp`) and decompiles `jar:`/`jrt:` sources on demand so go-to-definition opens readable buffers.
- `JAVA_HOME` follows the newest installed JDK (via `java_home` with a Homebrew fallback); keep a 21+ JDK installed for JDTLS.
- Terraform LSP prefers the nearest Terraform module directory as its root so monorepo roots (and large folders like `node_modules/`) don’t get indexed unnecessarily.
- Plugin specs follow lazy.nvim guidance: prefer `opts` for setup, and reserve `config` for custom/non-standard initialization.
- `nvim-treesitter` is pinned to the rewrite (`main`) branch and uses `require('nvim-treesitter').setup(...)`; parser/query installs follow upstream guidance and require the `tree-sitter` CLI (`brew install tree-sitter-cli`). A curated set (`c`, `cpp`, `go`, `lua`, `vim`, `python`, `rust`, `typescript`, `vimdoc`, `java`, `kotlin`) is auto-installed, and core treesitter features are enabled via a FileType autocommand (highlight/folds/indent).
- Regression checks for lazy loading and Terraform LSP behavior are documented in `nvim/REGRESSION_SPEC.md`.
- Startup-heavy plugins are lazy-loaded (InsertEnter/BufRead/command triggers), and Telescope keymaps defer `require(...)` until invocation to keep empty-start startup fast.
- `fff.nvim` is now intentionally lazy-loaded and validates its native backend on-demand to avoid startup crashes when the Rust binary is missing/corrupt.
- The Markdown preview plugin (`iamcco/markdown-preview.nvim`) builds with `cd app && npm install` via Lazy, so run `:Lazy sync` after initial setup or when updating the plugin.
