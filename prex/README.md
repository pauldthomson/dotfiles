# prex

Project-oriented Rex sessions, starting with **one Pi agent** rather than a predefined editor/shell layout. Independent of `../ptmux/`; no tmux commands, injected keystrokes, or shell-readiness handshakes.

## Install

Requires macOS/Linux, Go 1.26.5+, Rex, Pi, Git, Jujutsu, `fzf` for picker tabs (already in the root Brewfile), and `lsof` for clone removal.

```sh
(cd prex && go install .)
```

Keep `$(go env GOPATH)/bin` on your shell PATH (or use your configured `GOBIN`). Start Rex.app and enable **Settings → Rex Server → Remote Control** so `prex` can display sessions. Rex currently advertises app actions even when Remote Control is disabled; if switching fails after creation, the session is retained and its ID printed. Enable Remote Control and switch to that session—don't create another copy.

V1 requires a Unix-socket Rex server belonging to the same local account and a connected app. Remote servers and terminal-only attachment are intentionally unsupported: the inspected Rex version's `attach` is output-only. `--server` can pin an alternate **local** Unix socket. `--client` or `REX_CLIENT` selects an app; otherwise Rex must find exactly one. Flags precede the command.

## Usage

```sh
prex                         # fuzzy project picker
prex my-project              # exact directory name (picker if ambiguous)
prex --client 'my-mac' my-project
prex kill                    # close only the current Rex session
prex picker                  # temporary native Rex picker tab
prex picker kill             # close current session; helper tab for clone prompts
prex popup                   # compatibility alias for prex picker
prex recover                 # reconcile/acknowledge an uncertain launch
```

Projects are directories under `~/repos/<host>/<org>/<repo>`. Symlinked project directories are excluded. Picker rows include host/org/repo. Unlike `ptmux`, an unknown project is an error, not a placeholder session.

After selecting a project, **switch/reopen**, **clone another copy**, and **cancel** appear only if that project still has a Rex session. If Rex.app removed the session after Pi exited, selecting the project directly creates a new Pi session in the same repository—there is no switch/reopen menu. Multiple matching sessions get a session picker first. Cancellation performs no launch or clone.

| State | Result |
| --- | --- |
| No project session | New session, one `agent` tab running `pi` in the repository |
| Existing empty session | A new Pi tab in the **same session** |
| Existing session with tabs/blocks | Display it without restarting programs or rearranging anything |

Sessions are named `org/repo`, e.g. `nc-helix/helix` or `nc-helix/helix-2` for a clone. The host is omitted to keep names short; repository discovery and ownership still use the full path. New sessions receive that label, and existing sessions selected through prex have it reapplied (including previously generic or manually changed names). Unrelated sessions are not renamed. Long labels retain the end of the path plus a stable hash to fit Rex's limit. Labels are presentation; operations use full IDs, so renaming doesn't lose the recorded project association. Unmanaged sessions can be discovered through **local-host terminal process CWDs**; CWD discovery never grants clone ownership. An uninspectable unmanaged session is skipped with a warning, not treated as a verified match.

Rex runs `pi` through its login-shell environment, **not interactive `.zshrc`**. Pi exits normally → its pane/tab closes. In the tested Rex.app version, a displayed session is also removed when its last tab closes. An undisplayed/server-only session can remain empty; prex supports reopening it if it still exists. No replacement shell is started and no repository is deleted. Quick failures remain visible under Rex's default terminal policy. Add terminals/editors later using Rex itself.

### Environment

Install Pi/Node so they work without interactive NVM initialization. Configuration needed by Pi's tools (PATH, private-module settings, Java, certificate variables, etc.) must be available through the server/login environment. `prex` does not source `.zshrc`, copy secrets, or try to reproduce your interactive shell. Verify project-specific tools before relying on unattended agent work.

The live smoke tests cover Pi starting with the currently running local server's environment and exiting with Ctrl+D, followed by reopening an empty session or creating a replacement if Rex.app removed it. The earlier test with Remote Control disabled only exercised the undisplayed/empty-session case; it did not establish the displayed app's lifecycle. A real detached worker also closed its own caller session and wrote a durable completion result. This is not a guarantee that every project's SDK/credential environment is configured, or that a fresh GUI login has identical environment.

## Clones and cleanup

Cloning prompts for a suffix and reserves a sibling directory before running:

```sh
jj git clone --colocate -- <origin-or-local-source> <reserved-directory>
```

A failed clone is **retained** and its path reported. A successful clone is recorded with host, device/inode identity captured before cloning, and a random generation stored both in the registry and `.git/prex-owner` before launching Pi. A failed or uncertain launch also retains the clone.

`prex kill`:

1. Targets only the current Rex session, without a session chooser. No target arguments are accepted; outside Rex or with a missing current session it fails safely. Original/unowned checkouts close directly without a removal prompt.
2. Checks recorded clones for dirty, untracked **and ignored** files, unpushed JJ changes, Git-only local refs/stashes, unsupported submodules/shared worktrees, and failed inspection/fetches. Unsafe owned clones block session closure, matching `ptmux`'s conservative policy.
3. Asks separately whether to remove the session's owned clone. Declining closes the safe session but keeps its directory. Unowned/original checkouts are never deleted.
4. Closes only the current session in a detached worker. Other sessions stay open, including sessions sharing the same clone; a shared clone is retained.
5. Before deletion, checks other sessions on the pinned server (including empty recorded sessions and detached blocks), checks open files/CWDs with `lsof`, and repeats VCS/identity checks. Unknown inspection results retain the directory.
6. Moves the directory into a private, same-filesystem quarantine and verifies the moved inode before deletion, so a replacement at the original name isn't accidentally deleted. On an identity mismatch, the moved entry is **retained** under `.prex-removing-<token>/repo`; the error reports its path. These quarantine directories are excluded from the picker.

**Stop all external writers before confirming removal**, including editors, background agents, and sessions on other Rex servers. Registry locking serializes `prex` operations, not arbitrary programs. CWD/open-file inspection and repeated checks cannot prevent a new external writer racing deletion. Recovery-only Git reflogs, abandoned JJ history, Git/JJ administration, and local repository configuration are not remote backups and are discarded with a successfully removed clone. Preserve anything you need before approving deletion.

Closing the current session and any confirmed clone cleanup always run in an independent process. The parent exits/releases its lock; the worker is not attached to the dying terminal and runs outside the repository. Results are durable even if the UI disappears.

Closing Pi, closing sessions directly in Rex, and `rex kill` **never invoke prex clone cleanup**. If the corresponding Rex session was already removed outside prex, inspect and remove its orphaned clone manually; prex doesn't infer permission to delete it.

## State and recovery

State lives in `${XDG_STATE_HOME:-$HOME/.local/state}/prex` (private directory):

- `registry.json`: versioned session associations, owned clones, pending launch intent.
- `lock`: interprocess exclusive lock; another active operation is reported, not raced.
- `<token>.job.json`: confirmed, queued cleanup job.
- `<token>.running.json`: claimed job, never automatically replayed.
- `<token>.result.json`: queued/running/complete/failed outcome and completed session IDs.

Keep this directory private. Missing ownership records **never** get reconstructed from suffixes or matching remotes. Corrupt state blocks operations rather than overwriting evidence. Session associations are bound to a server instance; clone ownership is host-bound and still requires its original filesystem identity.

A timed-out creation/reopen has an unknown outcome, so its durable intent blocks further opening/cleanup. `prex recover` adopts a unique matching session only when its process CWD verifies the association on the original server instance. Otherwise it lists sessions and requires acknowledgement before clearing the intent. After a server restart, it offers explicit abandonment without adopting current sessions or deleting anything.

A worker crash can leave a `running` result. Inspect Rex, the reported clone/quarantine paths, and the result before doing anything manually; **don't replay an old cleanup job**. Filesystem cleanup and Rex are not one transaction.

## Shell and Rex integration

The tracked zsh config adds `prx=prex` and loads `prex/completion.zsh` from `~/.dotfiles` after completion initialization. `p=ptmux` and existing tmux/Neovim bindings remain unchanged.

Optional native Rex actions: add this to `~/.config/rex/init.lua` (adjust the checkout path):

```lua
dofile(os.getenv("HOME") .. "/repos/github.com/pauldthomson/dotfiles/prex/rex.lua")
```

Then validate and reload:

```sh
rex config check
rex config reload
```

The file registers **Open Project Agent** / **Close Current Session** and binds **Cmd+Option+P** / **Cmd+Option+X** directly (no leader). The project action opens a temporary native picker **tab**, avoiding the clipping observed with Rex.app's floating layers. The close action targets the current session immediately, without a session picker; a temporary helper tab hosts any clone-removal confirmation or safety errors. Picker tabs use `fzf` with Catppuccin colors, a rounded border, title, and navigation hints; plain prompts use the same background. Personal `FZF_DEFAULT_OPTS`/options files are ignored for these embedded pickers. Ordinary `prex` in a terminal retains its Go fuzzy picker. Both project-picker and close-helper processes explicitly select their own window ID in Rex.app (`session.select`) before reading input. This does not depend on the Lua action's queued client actions or server-side `focus`. The picker tab closes normally on cancellation or completion. Before exiting, it restores the originating tab if the picker still has focus and that tab still exists. It does not switch away from the selected project or override focus on a newly launched agent. Keyboard/palette actions pass the initiating app client ID explicitly. When invoked from the CLI, the transient CLI connection is not used as an app target; the picker resolves an unambiguous app instead. Errors wait for Enter. Edit `binary` in `rex.lua` if `prex` isn't installed in `~/go/bin`.

These actions are opt-in; installing the binary does not overwrite or reload your Rex configuration. Native shortcuts work from Neovim too, without replacing its existing tmux launcher binding.

## Tests

```sh
(cd prex && go test -race ./... && go vet ./...)
rex config check "$PWD/prex/rex.lua"
zsh -n prex/completion.zsh zsh/zshrc
```

Tests use mocked Rex calls and disposable real Git/JJ repositories; they do not change your sessions. Coverage includes native launch/reopen/switch, failures and uncertain outcomes, exact IDs, stale/corrupt/concurrent state, clone generation/replacement/quarantine, ignored files, stashes, meaningful empty JJ changes, caller-last cleanup, and a subprocess regression for parent death without unlocking.

Optional live smoke (creates/removes disposable sessions and may change the selected app session; sends Pi no prompts):

```sh
(cd prex && PREX_LIVE=1 PREX_LIVE_SERVER="$REX_SERVER" go test -run '^TestLive' -v)
```

Pass `PREX_LIVE_CLIENT` when several apps are connected. With Remote Control disabled, the test reports that app switching remains unverified while checking server-side lifecycle.
