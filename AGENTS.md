# Agent Collaboration Guidelines

## Version Control (Jujutsu)

- Use `jj` for all VCS operations (repos are git-backed and `jj` should be colocated).
- If a repo hasn’t been initialized for `jj`, run `jj git init --colocate` (avoid `git init`).
- Avoid `git` porcelain commands (`git status/add/commit/push`, etc.); use `jj status`, `jj log`, `jj git fetch`, `jj git push` as needed.

- Assume other collaborators may have uncommitted or untracked work in the repository. Leave their changes untouched unless you have explicit agreement to modify them.
- When preparing commits, only include files you personally changed for the current task. Avoid cleaning up or reverting unrelated files, even if they appear in `jj status`.
- If you spot unexpected modifications (for example, untracked documents such as `AGENT_ARCHITECTURE.md`), leave them as-is and inform the relevant collaborator instead of removing or altering them.
- Record any assumptions or coordination notes in the task conversation so other agents stay aligned.
- remember to always create a bookmark/branch first when we commit, push, pr
- When using the `gh` CLI tool to create any new resource (issues, pull requests, etc.) that returns a URL, get the URL directly from the command output and pipe it to pbcopy in a single step. The URL is returned in the gh command response - don't make a separate gh pr view call. IMPORTANT: When piping to pbcopy, the command will show NO output on success (the output goes to clipboard). Don't retry thinking it failed - the lack of output means success.
- Before ANY VCS operations (commit, push, pr), ALWAYS check your current state with `jj status` and your current bookmark(s) with `jj bookmark list`. If you’re working on a bookmark that should exist upstream, run `jj git fetch` and confirm it’s still present; if it’s gone (likely merged), start from the mainline and create a fresh bookmark.
- always update all relevant documentation (README.md etc) whenver making changes
- Make sure commit messages/PR descriptions include why we made the change and not just what was changed. If you're unclear about what the "why" part was, don't make something up and just ask me
- don't include test plan in PR descriptions
- Always use `go mod tidy` instead of manually editing go.mod. It properly resolves transitive dependencies, updates go.sum with correct checksums, and prevents build failures. Run it after making code changes that add/remove imports, or when switching between package versions (e.g., HTTP → gRPC exporters).