## High-level summary
- Updated local Pi extensions to match current extension API behavior and remove deprecated session event usage.
- Fixed the custom `web_fetch` tool to report failures using the current tool error contract and added prompt metadata so it is easier for Pi to discover.
- Removed the unused repo-local `pi-agent/settings.json` file and cleaned up symlink/setup documentation to stop linking it.

## Why
- Pi's changelog introduced breaking extension API changes, and this repo still had custom extensions using removed or outdated patterns.
- Keeping those stale patterns around makes future upgrades harder and can cause extension behavior to drift from Pi's documented runtime semantics.
- The repo-local `pi-agent/settings.json` file is no longer used, so continuing to link and document it added unnecessary setup noise.

## Session context
- Scope: audit the repo's Pi extensions against the current Pi changelog and update anything affected by recent extension API changes.
- Inputs/constraints: only include the Pi-related files touched for this task and leave unrelated in-progress Neovim work alone.
- Important context: the audit found deprecated `session_switch` listeners, outdated `isError: true` tool failure handling in `web_fetch`, and stale setup docs for the unused settings file.

## Key decisions made
- Decision: remove `session_switch` listeners from the affected extensions.
  - Why: current Pi uses `session_start` with a reason field instead of the removed post-transition session events.
- Decision: make `web_fetch` throw on invalid input and fetch failures instead of returning `{ isError: true }`.
  - Why: current Pi documents thrown errors as the supported way to mark a tool execution as failed.
- Decision: add a `promptSnippet` to `web_fetch`.
  - Why: current Pi only includes custom tools in the default `Available tools` prompt section when they opt in with `promptSnippet`.
- Decision: remove `pi-agent/settings.json` and stop linking it from setup scripts.
  - Why: the file is no longer used, so keeping it around and symlinking it created stale configuration paths.

## Alternatives considered
- Alternative: leave the deprecated listeners and old tool error handling in place because the extensions still mostly worked.
  - Why not chosen: that would preserve behavior that already diverges from documented Pi APIs and make future breakage more likely.
- Alternative: keep `pi-agent/settings.json` as an empty compatibility stub.
  - Why not chosen: that would keep unnecessary setup and documentation around for a file we do not actually use.

## Trade-offs and risks
- Removing the repo-managed settings symlink means any settings customizations must now come from the user's real Pi settings location instead of this repo.
- The `web_fetch` tool will now fail more explicitly, which is the desired API behavior but may slightly change how failures appear in transcripts.
- This change is intentionally narrow and only updates the extensions and setup/docs touched by the API audit.

## Tests changed
- **Added**:
  - Not available
- **Updated**:
  - Not available
- **Removed**:
  - Not available
- **Not run / blocked**:
  - No test files changed. Re-ran `node --test pi-agent/extensions/auto-qna/question-extractor.test.ts` and `bash -n pi-agent/setup-symlinks.sh` for regression coverage.

## Notes for reviewer
- Focus review on the Pi extension compatibility updates in `pi-agent/extensions/` and the removal of the stale settings symlink/documentation path.
- There is unrelated Neovim work still present in the working copy, but it is not part of this PR.
