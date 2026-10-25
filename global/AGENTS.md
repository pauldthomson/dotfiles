# Global Agent Guidelines

These are cross-repository defaults. Follow repository-specific instructions for
local conventions, commands and workflows.

## Collaboration and scope

- Preserve unrelated edits, untracked files and other collaborators' work. Do not
  clean up, revert or resolve unrelated conflicts without agreement.
- Keep changes focused on the requested task. Discuss material scope or design
  changes before implementing them.
- Keep assumptions, blockers and review findings in the conversation. Do not post
  comments or messages to external systems unless asked.

## Version control

- Prefer Jujutsu (`jj`) in existing jj repositories; avoid Git porcelain commands
  there. Do not initialize or change a repository's VCS setup without agreement.
- Before committing, pushing or opening a PR, inspect the working state and
  bookmarks. Include only changes belonging to the current task.
- Fetch before starting new work and base it on the latest remote mainline unless
  a different base is requested. Use isolated workspaces when sharing a checkout
  with other work or agents.
- Use a task bookmark for PR work. If an old remote bookmark has disappeared,
  check whether it was merged before reusing it. Never assume the mainline is
  named `main` or `master`.

## Validation

- Prefer tests of meaningful behavior and failure modes over assertions that
  merely duplicate implementation details or hardcoded configuration.
- Run relevant checks and report what passed, failed or could not be run. Clearly
  distinguish rendering/schema validation from live provisioning or runtime proof.
- Change generated files through their source and regenerate them using the
  repository's tooling.

## Operational safety

- Never approve deployment gates on the user's behalf. Preparing a change is not
  authorization to apply infrastructure, promote, restore data or enable workloads.
- Prefer authoritative CI infrastructure plans; do not run local plans by default.
  Review observed drift as well as proposed actions, and distinguish pre-existing
  changes from those introduced by the current work.
- Never expose secret values in output, command arguments, Git or infrastructure
  state. Use secure file-based transfers and cleanup when access is authorized.

## Documentation

A README is the entry point for people using or working on a repository. Explain
what the application or project does, how its main parts fit together, and how to
set up, run, validate and operate it. Use concise examples and links to deeper
documentation so a newcomer knows where to start.

A README is not a software specification, configuration inventory, deployment
plan or history of implementation decisions. Do not duplicate resource sizes,
versions, retention settings or manifest fields already defined in code. Link to
those sources of truth instead. Include configuration details when needed to
follow setup or usage instructions.

Put detailed operational procedures in focused runbooks, normally under `docs/`,
and design rationale or specifications in dedicated documents when needed.
Contributor and agent instructions belong in `AGENTS.md`. Keep the README focused
on the current project rather than the current PR. Update documentation when
behavior or usage changes, not merely because code changed.

## Commits and pull requests

- Explain what changes and why. Do not invent rationale; ask when it is unclear.
- Describe the current design, scope and relevant dependencies, not the history of
  iterations. Do not include a test-plan section in PR descriptions.
- Return newly created resource URLs from the creation command's output. On macOS,
  also copy the URL with `pbcopy` when available, while retaining it in the response.
  Do not repeat a creation command just because clipboard piping hid its output.
