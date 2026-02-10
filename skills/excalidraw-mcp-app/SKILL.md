---
name: excalidraw-mcp-app
description: Create and iterate Excalidraw diagrams via a local Excalidraw MCP CLI. Use when a user asks for architecture diagrams, flowcharts, sequence views, or visual system maps.
license: Proprietary
compatibility: Requires Bun and internet access to reach the Excalidraw MCP endpoint
metadata:
  author: paulthomson
  version: "1.0"
---

# Purpose
Generate and refine Excalidraw diagrams from structured element JSON.

# Tool location
- Script: `scripts/excalidraw-mcp-app.ts`
- Runtime: Bun (script shebang uses `bun --install=fallback` for portability)

# When to use
Use this skill when the user asks for:
- architecture diagrams
- sequence/flow diagrams
- service dependency maps
- any Excalidraw visual artifact

# Recommended workflow
1. Read format guidance first:
   - `./scripts/excalidraw-mcp-app.ts read-me --output markdown`
2. Build valid Excalidraw `elements` JSON (strict JSON, no comments/trailing commas).
3. Render a view:
   - `./scripts/excalidraw-mcp-app.ts create-view --elements '<json-array-string>' --output markdown`
4. If needed, export/share:
   - `./scripts/excalidraw-mcp-app.ts export-to-excalidraw --json '<serialized-scene-json>' --output text`
5. For iterative sessions, use checkpoints:
   - `save-checkpoint` and `read-checkpoint`

# Command reference
- `read-me`
- `create-view --elements <elements>`
- `export-to-excalidraw --json <json>`
- `save-checkpoint --id <id> --data <data>`
- `read-checkpoint --id <id>`

Global options:
- `--timeout <ms>`
- `--output <text|markdown|json|raw>`

# JSON handling tips
To avoid escaping issues, keep JSON in a file and compact it:

```bash
cat >/tmp/elements.json <<'EOF'
[
  {
    "type": "rectangle",
    "x": 100,
    "y": 80,
    "width": 220,
    "height": 100,
    "strokeColor": "#1e1e1e",
    "backgroundColor": "transparent",
    "text": "API Service"
  }
]
EOF

ELEMENTS="$(jq -c . /tmp/elements.json)"
./scripts/excalidraw-mcp-app.ts create-view --elements "$ELEMENTS" --output markdown
```

# Expected behavior
- `read-me` returns authoritative element format guidance.
- `create-view` returns render output (often markdown + link/preview details).
- Invalid JSON returns a parsing/validation error; fix JSON and retry.

# Troubleshooting
- `bun: command not found`: install Bun and ensure it is on `PATH`.
- Dependency/module resolution errors: rerun command (auto-install is enabled), verify network access.
- MCP endpoint errors/timeouts: retry with higher `--timeout`, verify endpoint availability.
