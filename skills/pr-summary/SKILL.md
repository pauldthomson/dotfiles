---
name: pr-summary
description: Generate reviewer-ready PR bodies with high-level changes, rationale, session context, decisions, alternatives, trade-offs, and test impact. Always use when creating or editing a PR body for agent-authored changes.
license: Proprietary
compatibility: Works in interactive Pi and non-interactive coding harnesses.
metadata:
  author: paulthomson
  version: "1.3"
---

# Purpose
Create consistent, high-signal PR descriptions that explain what changed and why.

# Workflow
Use this skill whenever creating or editing a PR body.

## Mode A: Interview payload available (preferred)
If chat contains a JSON payload with:
- `type: "pr_summary_interview_answers"`
- section answers under `sections`
- diff context under `diff_context`

Then use that payload as the primary source of truth.

## Mode B: No interview payload (interactive Q&A required)
If the payload is not available, run a chat interview instead of guessing:
1. Ask exactly **one** question at a time.
2. Follow the required section order below.
3. For each section, offer a short suggested default inferred from VCS diff output (for this repo, prefer `jj diff`), changed files, and session context.
4. Wait for the user response before asking the next question.
5. If the user says they don't know or wants to skip, record `Not available`.
6. After collecting all sections, render the final PR body.

Never dump all interview questions in a single assistant response.

### Interview turn contract (strict)
- Until all sections are answered, each assistant turn must include **exactly one** section question.
- Do not include additional questions for later sections in the same turn.
- Do not render the PR body until every required section has an answer or explicit `Not available`.

# Required output sections
Fill **all** sections; if data is unavailable, state `Not available` explicitly.

## 1) High-level summary
1–3 bullets that state what changed in non-technical language.

## 2) Why
Explain the problem and the motivation for the change.

## 3) Session context
Capture high-level context from the session, including:
- Scope and assumptions
- Inputs/constraints
- Important prior context/decisions

## 4) Key decisions made
List each decision and rationale.

## 5) Alternatives considered
For each important decision, list alternatives reviewed and why they were rejected.

## 6) Trade-offs and risks
Include explicit trade-offs, compatibility or migration impact, and known risks.

## 7) Tests changed
- **Added**: tests added and what they cover
- **Updated**: tests modified
- **Removed**: tests removed
- **Not run / blocked**: why tests were not run

## 8) Notes for reviewer
Any caveats, follow-ups, or likely review hotspots.

# Rendering rules
- Keep section order exactly as defined above.
- Prefer interview payload when present; otherwise collect answers via one-question-at-a-time interview.
- Preserve explicit `Not available` values.
- Keep language concise and reviewer-friendly.

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
- Why + trade-off rationale is explicit.
- No section is omitted; missing data is explicitly `Not available`.
