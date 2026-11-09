# prex

Project-oriented Rex sessions, starting with **one Pi agent** rather than a predefined editor/shell layout. Independent of `../ptmux/`; no tmux commands, injected keystrokes, or shell-readiness handshakes.

## Install

Requires macOS/Linux, Go 1.26.5+, Rex with `--interactive` support for `new` and `window new` and Lua scripts via `rex do -e`, Pi, Git, Jujutsu, `fzf` for picker tabs (already in the root Brewfile), and `lsof` for clone removal.

```sh
(cd prex && go install .)
```

Keep `$(go env GOPATH)/bin` on your shell PATH (or use your configured `GOBIN`). Start Rex.app and enable **Settings → Rex Server → Remote Control** so `prex` can display sessions. Rex currently advertises app actions even when Remote Control is disabled; if switching fails after creation, the session is retained and its ID printed. Enable Remote Control and switch to that session—don't create another copy.

V1 requires a Unix-socket Rex server belonging to the same local account. Opening projects also requires a connected app; `prex orphans` does not. Remote servers and terminal-only attachment are intentionally unsupported: the inspected Rex version's `attach` is output-only. `--server` can pin an alternate **local** Unix socket. `--client` or `REX_CLIENT` selects an app; otherwise Rex must find exactly one. Flags precede the command.

## Usage

```sh
prex                         # fuzzy project picker
prex my-project              # exact directory name (picker if ambiguous)
prex --client 'my-mac' my-project
prex kill                    # close only the current Rex session
prex picker                  # centred floating Rex picker
prex picker tab              # full-tab fallback
prex picker kill             # close current session; helper tab for clone prompts
prex popup                   # compatibility alias for prex picker
prex recover                 # reconcile/acknowledge an uncertain launch
prex orphans                 # read-only report of recorded clone/session use
```

Projects are directories under `~/repos/<host>/<org>/<repo>`. Symlinked project directories are excluded. Picker rows include host/org/repo. In the native picker, repository names are highlighted, hosts are muted, and green counts mark existing sessions; projects without sessions have no suffix. Unlike `ptmux`, an unknown project is an error, not a placeholder session.

In ordinary terminal invocation, **switch/reopen**, **clone another copy**, and **cancel** appear after selecting a project that still has a Rex session. If Rex.app removed the session after Pi exited, selecting the project directly creates a new Pi session in the same repository. Multiple matching sessions get a session picker first.

The floating picker (`prex picker` or **Cmd+Option+P**) adds a preview and optional shortcuts:

- **Enter:** select a project. If it already has a session, automatically offer **switch/reopen**, **clone a new copy**, or **cancel**. Multiple matches get a session chooser first. If none matches, start Pi in the original repository.
- **Ctrl+N:** clone the highlighted project, then ask for a suffix before creating the copy. This optional shortcut is available even when the original has no session; you do not need it to discover the clone option for an existing session.
- **Tab:** hide/show the preview. It shows the repository path, matching session names/IDs, and available actions; narrow terminals put it below the list.
- **Esc:** cancel. Cancellation performs no launch or clone.

Previews are snapshots, not agent activity indicators. Session matches are refreshed after selection; if Enter's target set or inspection warnings changed, the picker asks you to review and confirm again. Previewing never launches Pi or clones a repository.

| State | Result |
| --- | --- |
| No project session | New session, one `agent` tab running `pi` in the repository |
| Existing empty session | A new Pi tab in the **same session** |
| Existing session with tabs/blocks | Display it without restarting programs or rearranging anything |

Sessions are named `org/repo`, e.g. `nc-helix/helix` or `nc-helix/helix-2` for a clone. The host is omitted to keep names short; repository discovery and ownership still use the full path. New sessions receive that label, and existing sessions selected through prex have it reapplied (including previously generic or manually changed names). Unrelated sessions are not renamed. Long labels retain the end of the path plus a stable hash to fit Rex's limit. Labels are presentation; operations use full IDs, so renaming doesn't lose the recorded project association. Unmanaged sessions can be discovered through **local-host terminal process CWDs**; CWD discovery never grants clone ownership. An uninspectable unmanaged session is skipped with a warning, not treated as a verified match.

Each project-discovery snapshot lists sessions and collects unmanaged sessions' terminal directories in one `rex do` invocation, including detached panes. Go determines which recorded associations are valid for the current server instance and sends only their IDs so Lua can skip unnecessary inspections. Lua remains a read-only Rex adapter; it does not read the registry or decide repository matches. Go still validates local-host terminal data and resolves filesystem paths before matching repositories; results retain session-list order. Individual inspection failures skip that session with a warning. A failed collection, incomplete session set, or changed server identity aborts opening rather than risking a duplicate launch. This collection is not reused by recovery or clone-deletion checks.

Rex runs `pi` with `--interactive` for both new sessions and reopened empty sessions, loading the login environment and interactive shell startup files such as `.zshrc`. Picker tabs and cleanup helpers retain their existing startup behavior. Pi exits normally → its pane/tab closes. In the tested Rex.app version, a displayed session is also removed when its last tab closes. An undisplayed/server-only session can remain empty; prex supports reopening it if it still exists. No replacement shell is started and no repository is deleted. Quick failures remain visible under Rex's default terminal policy. Add terminals/editors later using Rex itself.

After creating a Pi pane, prex records the session and displays it immediately, then briefly observes its process for an early exit. An immediate exit reports its code and pane ID; unavailable process data is reported as **unverified**, not success. Prex never closes the diagnostic pane, removes the repository, or retries the launch automatically. Normal-exit behavior remains Rex's default. This check detects early exits, not full UI readiness or a shell startup script waiting for input; terminal output is not copied into the error message.

### Environment

Pi inherits exported configuration from your interactive shell startup (PATH, private-module settings, Java, certificate variables, etc.). Shell aliases and functions do not generally carry into Pi's subprocesses; lazy version-manager functions may still need explicit activation to select a project-specific runtime. Rex loads the shell configuration; `prex` does not copy secrets or reproduce it manually. Keep startup files free of prompts and other side effects that would block a command launch. Picker tabs still need `prex` and its helper tools available through the server/login environment. Verify project-specific tools before relying on unattended agent work.

The live smoke tests cover Pi starting with the currently running local server's environment and exiting with Ctrl+D, followed by reopening an empty session or creating a replacement if Rex.app removed it. The earlier test with Remote Control disabled only exercised the undisplayed/empty-session case; it did not establish the displayed app's lifecycle. A real detached worker also closed its own caller session and wrote a durable completion result. This is not a guarantee that every project's SDK/credential environment is configured, or that a fresh GUI login has identical environment.

## Clones and cleanup

Cloning prompts for a suffix and reserves a sibling directory before running:

```sh
jj git clone --colocate -- <origin-or-local-source> <reserved-directory>
```

A failed clone is **retained** and its path reported. A successful clone is recorded with host, device/inode identity captured before cloning, and a random generation stored both in the registry and `.git/prex-owner` before launching Pi. A failed or uncertain launch also retains the clone.

`prex kill`:

1. Targets only the current Rex session, without a session chooser. No target arguments are accepted; outside Rex or with a missing current session it fails safely. Original/unowned checkouts close directly without a removal prompt.
2. Checks recorded clones for dirty and untracked files, unpushed JJ changes, Git-only local refs/stashes, unsupported submodules/shared worktrees, and failed inspection/fetches. Ignored files do not make a checkout dirty, matching `jj status`; generated dependencies and build output do not block cleanup. Unsafe owned clones block session closure.
3. Asks separately whether to remove the session's owned clone, warning that ignored files will also be deleted. Declining closes the safe session but keeps its directory. Unowned/original checkouts are never deleted.
4. Closes only the current session in a detached worker. Other sessions stay open, including sessions sharing the same clone; a shared clone is retained.
5. Before deletion, checks other sessions on the pinned server (including empty recorded sessions and detached blocks), checks open files/CWDs with `lsof`, and repeats VCS/identity checks. Unknown inspection results retain the directory.
6. Moves the directory into a private, same-filesystem quarantine and verifies the moved inode before deletion, so a replacement at the original name isn't accidentally deleted. On an identity mismatch, the moved entry is **retained** under `.prex-removing-<token>/repo`; the error reports its path. These quarantine directories are excluded from the picker.

**Stop all external writers before confirming removal**, including editors, background agents, and sessions on other Rex servers. Registry locking serializes `prex` operations, not arbitrary programs. CWD/open-file inspection and repeated checks cannot prevent a new external writer racing deletion. Recovery-only Git reflogs, abandoned JJ history, Git/JJ administration, and local repository configuration are not remote backups and are discarded with a successfully removed clone. Ignored files (including `.env` files, dependencies, and build output) are not checked or preserved. Preserve anything you need before approving deletion.

Closing the current session and any confirmed clone cleanup always run in an independent process. The parent exits/releases its lock; the worker is not attached to the dying terminal and runs outside the repository. Results are durable even if the UI disappears.

Closing Pi, closing sessions directly in Rex, and `rex kill` **never invoke prex clone cleanup**. If the corresponding Rex session was already removed outside prex, inspect and remove its orphaned clone manually; prex doesn't infer permission to delete it.

### Finding orphaned clones

Run `prex orphans` to inspect recorded clones without a connected app. It reports:

- **orphan**: ownership identity is valid and no session use was found on the selected Rex server.
- **in-use**: a live session uses the clone or has a current association with it.
- **unknown**: inspection, ownership identity, a pending launch, or a stale surviving association prevents a conclusion.
- **other-host**: ownership belongs to another host and was not inspected locally.

This reads a registry snapshot and inspects all session panes, including detached panes. It does not create state files, refresh associations, fetch repositories, or delete anything. An orphan result is **not permission or proof that deletion is safe**: other Rex servers, external processes, ignored files, and unpushed work are not checked. Review and preserve the directory's contents before any manual removal.

## State and recovery

State lives in `${XDG_STATE_HOME:-$HOME/.local/state}/prex` (private directory):

- `registry.json`: versioned session associations, owned clones, pending launch intent.
- `lock`: interprocess exclusive lock; another active operation is reported, not raced.
- `<token>.job.json`: confirmed, queued cleanup job.
- `<token>.running.json`: claimed job, never automatically replayed.
- `<token>.result.json`: queued/running/complete/failed outcome and completed session IDs.

Keep this directory private. Missing ownership records **never** get reconstructed from suffixes or matching remotes. Corrupt state blocks operations rather than overwriting evidence. Session associations are bound to a server instance; clone ownership is host-bound and still requires its original filesystem identity.

When opening a project after a server restart, prex revalidates surviving recorded session IDs on the same host. It updates an association only when inspection succeeds, at least one process is still live, and every observed working directory is within its previously recorded project; owned clones must still pass identity checks. Empty, mixed-project, inaccessible, or otherwise ambiguous sessions remain unverified. This never infers ownership, matches by label, adopts replacement session IDs, or clears a pending launch.

A timed-out creation/reopen has an unknown outcome, so its durable intent blocks further opening/cleanup. `prex recover` adopts a unique matching session only when its process CWD verifies the association on the original server instance. Otherwise it lists sessions and requires acknowledgement before clearing the intent. After a server restart, it offers explicit abandonment without adopting current sessions or deleting anything.

A worker crash can leave a `running` result. Inspect Rex, the reported clone/quarantine paths, and the result before doing anything manually; **don't replay an old cleanup job**. Filesystem cleanup and Rex are not one transaction.

## Shell and Rex integration

The tracked zsh config adds `prx=prex` and loads `prex/completion.zsh` from its own dotfiles checkout after completion initialization (resolving the `~/.zshrc` symlink). `p=ptmux` and existing tmux/Neovim bindings remain unchanged.

Optional native Rex actions: add this to `~/.config/rex/init.lua` (adjust the checkout path):

```lua
dofile(os.getenv("HOME") .. "/repos/github.com/pauldthomson/dotfiles/prex/rex.lua")
```

Then validate and reload:

```sh
rex config check
rex config reload
```

The file registers **Open Project Agent** / **Close Current Session** and binds **Cmd+Option+P** / **Cmd+Option+X** directly (no leader). The project action opens a centred **floating picker** over the current tab. Use `prex picker tab` for the full-tab fallback if a Rex app renders floating layers incorrectly. Creation failures are not automatically retried: inspect for an existing picker before using the fallback. The close action targets the current session immediately, without a session picker; a temporary helper tab hosts any clone-removal confirmation or safety errors. Pickers use `fzf` with Catppuccin colors, a rounded border, title, and navigation hints; plain prompts use the same background. Personal `FZF_DEFAULT_OPTS`/options files are ignored for these embedded pickers. Ordinary `prex` in a terminal retains its Go fuzzy picker. Both project-picker and close-helper processes explicitly select their own window ID in Rex.app (`session.select`) before reading input. This does not depend on the Lua action's queued client actions or server-side `focus`. The temporary picker closes normally on cancellation or completion, leaving the underlying tab intact. Before exiting, it restores the originating tab if the picker still has focus and that tab still exists. It does not switch away from the selected project or override focus on a newly launched agent. Keyboard/palette actions pass the initiating app client ID explicitly. When invoked from the CLI, the transient CLI connection is not used as an app target; the picker resolves an unambiguous app instead. Errors wait for Enter; if a failed launch already selected its new session, the picker returns to its own tab to show the error while retaining the diagnostic pane. Edit `binary` in `rex.lua` if `prex` isn't installed in `~/go/bin`.

These actions are opt-in; installing the binary does not overwrite or reload your Rex configuration. Native shortcuts work from Neovim too, without replacing its existing tmux launcher binding.

## Tests

```sh
(cd prex && go test -race ./... && go vet ./...)
rex config check "$PWD/prex/rex.lua"
zsh -n prex/completion.zsh zsh/zshrc
```

Tests use mocked Rex calls and disposable real Git/JJ repositories; they do not change your sessions. Reliability coverage includes early exits versus unknown startup outcomes, read-only orphan reporting, and revalidation against simulated stale server-instance associations. Coverage includes native launch/reopen/switch, failures and uncertain outcomes, exact IDs, stale/corrupt/concurrent state, clone generation/replacement/quarantine, ignored files, stashes, meaningful empty JJ changes, caller-last cleanup, and a subprocess regression for parent death without unlocking.

For real fzf keyboard and wide/narrow layout checks without launching Pi or changing app selection:

```sh
(cd prex && PREX_LIVE_PICKER=1 PREX_LIVE_SERVER="$REX_SERVER" go test -run '^TestLiveProjectPicker$' -v)
```

This uses fixture previews in disposable Rex sessions; it checks filtering, Enter, Ctrl+N, Tab, and Esc. Unit tests cover action routing, changed session snapshots, cancellation, clone ownership, and safe preview rendering.

Optional live smoke (creates/removes disposable sessions and may change the selected app session; sends Pi no prompts):

```sh
(cd prex && PREX_LIVE=1 PREX_LIVE_SERVER="$REX_SERVER" go test -run '^TestLive' -v)
```

For a launch-only check without switching the app's selected session, use `-run '^TestLiveInteractiveAgentLaunch$'` instead. It starts Pi in a disposable session, waits for raw terminal input mode, exits with Ctrl+D, and repeats after reopening the empty session; no prompts are sent to Pi.

`TestLiveImmediateStartupExit` creates a disposable terminal that exits with a known failure code and checks that the diagnostic pane remains available. Neither this test nor the launch-only test changes app selection. Restart association tests use controlled stale metadata; they do not restart your server.

A separate read-only check compares the collected session list and paths with individual Rex inspections, and verifies that recorded sessions skip process collection (run while terminal working directories are stable):

```sh
(cd prex && PREX_LIVE_LOOKUP=1 PREX_LIVE_SERVER="$REX_SERVER" go test -run '^TestLiveLookupCollection$' -v)
```

Pass `PREX_LIVE_CLIENT` when several apps are connected. With Remote Control disabled, the app lifecycle test reports that app switching remains unverified while checking server-side lifecycle.
