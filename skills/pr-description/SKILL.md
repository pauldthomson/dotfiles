---
name: pr-description
description: Write and update pull request titles and descriptions. Use when creating a PR, editing its description, or revising a PR after implementation or review changes.
---

# Pull request descriptions

Treat the description as a materialised view of the final proposed change, not a transcript of the conversation or implementation history.

- Explain what the change does and why it is needed. Include decision rationale that helps a reviewer understand the final design.
- Describe relevant scope, security implications, dependencies and rollout ordering when they materially affect review or deployment.
- Omit intermediate approaches, abandoned proposals and iteration history. Avoid statements such as "we are not doing X" unless the contrast provides necessary context for a reviewer.
- Do not assume the reviewer has read the agent conversation. The description must stand on its own.
- Keep it concise. Prefer a short explanation of why and how over a file-by-file inventory or restatement of the diff.
- Do not include a test-plan section. Keep validation evidence in checks or review comments when needed.
- When the design changes, rewrite the affected description rather than appending a chronological update. Remove stale claims and dependencies.
- Never invent rationale. Ask the user if the reason for a decision is unclear.

Before publishing, check that every statement describes the current change or supplies context necessary to review it. A reviewer should not need to reconstruct the conversation to understand why a sentence is there.
