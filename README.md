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

## Neovim
- Kotlin LSP decompiles `jar:`/`jrt:` sources on demand so go-to-definition opens readable buffers.
- `JAVA_HOME` follows the newest installed JDK (via `java_home` with a Homebrew fallback); keep a 21+ JDK installed for JDTLS.
