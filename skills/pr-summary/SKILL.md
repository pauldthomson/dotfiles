---
name: pr-summary
description: Generate reviewer-ready PR bodies with high-level changes, rationale, session context, decisions, alternatives, trade-offs, and test impact. Always use when creating or editing a PR body for agent-authored changes.
license: Proprietary
compatibility: Requires access to git/jj diff context and the agent session notes.
metadata:
  author: paulthomson
  version: "1.0"
---

# Purpose
Create consistent, high-signal PR descriptions that help a reviewer understand not only what changed, but also how/why it was changed.

# When to use
Use this skill when:
- You are preparing a PR body from agent work.
- You are creating a PR (`gh pr create`) or editing an existing PR body (`gh pr edit`) for agent-authored changes.
- The user asks for "make a PR", "open a PR", or similar phrasing.
- You need to explain implementation choices and alternatives considered.
- You need to report test changes (added/changed/removed/not run).

# Required output sections
Fill **all** sections; if data is unavailable, state `Not available` explicitly.

## 1) High-level summary
1–3 bullets that state what changed in non-technical language.

## 2) Why
Explain the problem and the motivation for the change.

## 3) Session context
Capture the high-level context from the agent session, including:
- Scope and assumptions
- Inputs/constraints
- Any important context from prior decisions or discussion

## 4) Key decisions made
List each decision and rationale.

## 5) Alternatives considered
For each important decision, list the alternatives reviewed and why they were rejected.

## 6) Trade-offs and risks
Include explicit trade-offs, compatibility or migration impact, and any known risks.

## 7) Tests changed
- **Added**: tests added and what they cover
- **Updated**: tests modified
- **Removed**: tests removed
- **Not run / blocked**: why tests were not run

## 8) Notes for reviewer
- Any caveats, follow-ups, or likely review hotspots.

# Recommended workflow
1. Collect changed files and summary of code changes.
2. Capture rationale from the issue/goal and session notes.
3. Capture decisions, alternatives considered, and trade-offs.
4. Capture test status: added/updated/removed and anything not run.
5. Render the PR body in the exact section order above.
6. If a PR was already created with a short/incomplete body, regenerate with this skill and immediately update via `gh pr edit --body-file`.

# PR body template
```markdown
## High-level summary
- ...

## Why
- ...

## Session context
- ...

## Key decisions made
- Decision: ...
  - Why: ...

## Alternatives considered
- Alternative: ...
  - Why not chosen: ...

## Trade-offs and risks
- ...

## Tests changed
- **Added**:
  - ...
- **Updated**:
  - ...
- **Removed**:
  - ...
- **Not run / blocked**:
  - ...

## Notes for reviewer
- ...
```

# Success criteria
- Every required section is present.
- Why + trade-off rationale is explicit (no vague claims).
- If no data exists for a section, explicitly state that it is missing.
