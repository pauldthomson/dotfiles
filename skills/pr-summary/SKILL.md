---
name: pr-summary
description: Generate reviewer-ready PR bodies with high-level changes, rationale, session context, decisions, alternatives, trade-offs, and test impact. Always use when creating or editing a PR body for agent-authored changes.
license: Proprietary
compatibility: Requires interactive Pi with the /pr-summary extension command.
metadata:
  author: paulthomson
  version: "1.1"
---

# Purpose
Create consistent, high-signal PR descriptions that explain what changed and why.

# Mandatory workflow (no fallback)
This skill requires structured interview input produced by the `/pr-summary` extension command.

1. User runs `/pr-summary` in interactive Pi.
2. The extension interviews the user section-by-section with suggested defaults.
3. The extension sends a JSON payload in chat with:
   - `type: "pr_summary_interview_answers"`
   - section answers under `sections`
   - diff context under `diff_context`
4. This skill renders the final PR body from that payload.

If that payload is missing, stop and reply:
`Please run /pr-summary in interactive Pi first.`

Do not run free-form fallback interviews in chat.

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
- Use interview payload answers as authoritative source of truth.
- Keep section order exactly as defined above.
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
