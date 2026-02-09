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

## Pi
- Adds a `web_fetch` extension for fetching URLs with truncation and temp file fallback.
- Adds a `review` extension (based on mitsuhiko/agent-stuff) for interactive code review flows (`/review`, `/review bookmark <name>`, `/end-review`) using `jj` workflows.
- Adds a `jj-footer` extension that replaces git `(detached)` branch display with jj-aware status (`jj:<bookmark>` or `jj:@<change-id>`), while still falling back to git when outside jj repos.
- Pi extensions include duplicate-load guards so global + project discovery (common with symlinked dotfiles) does not double-register commands/tools.
- Uses a custom `catppuccin` theme (Mocha palette) defined in `.pi/themes/catppuccin.json`, with neutral tool-call card backgrounds and softer diff colors to better match the palette.

## Neovim
- Kotlin LSP decompiles `jar:`/`jrt:` sources on demand so go-to-definition opens readable buffers.
- `JAVA_HOME` follows the newest installed JDK (via `java_home` with a Homebrew fallback); keep a 21+ JDK installed for JDTLS.
