---
name: chrome-devtools-mcp
description: Drive and inspect Chrome pages through the Chrome DevTools MCP server. Use when the user asks to open pages, click/fill UI, inspect console or network activity, take snapshots/screenshots, or run browser performance audits.
license: Proprietary
compatibility: Requires Bun, Node/npm (`npx`), Chrome/Chromium available to DevTools, and network access to install or update `chrome-devtools-mcp@latest`
metadata:
  author: paulthomson
  version: "1.1"
---

# Purpose
Use Chrome DevTools MCP from a local Pi skill without requiring a separate global setup.
The embedded server launches with `--autoConnect` and auto-detects an installed Chrome channel, preferring `stable -> beta -> dev -> canary`.
Override channel selection with `--channel <stable|beta|dev|canary>` or `CHROME_DEVTOOLS_MCP_CHANNEL=<channel>`.

# Tool location
- Script: `scripts/chrome-devtools-mcp.ts`
- Runtime: Bun
- Upstream MCP server: `npx chrome-devtools-mcp@latest --autoConnect --channel=<auto-detected>`

# When to use
Use this skill when the user asks to:
- open or navigate browser pages
- click, type, fill forms, drag, upload, or press keys in a page
- inspect page structure via accessibility snapshots
- take screenshots
- inspect console logs or network requests
- run Lighthouse audits or performance traces
- capture memory snapshots

# Recommended workflow
1. Read the available commands:
   - `./scripts/chrome-devtools-mcp.ts --help`
   - If needed, choose a browser channel before the subcommand, for example:
     - `./scripts/chrome-devtools-mcp.ts --channel stable list-pages --output markdown`
2. Start from browser context:
   - `./scripts/chrome-devtools-mcp.ts list-pages --output markdown`
   - or `./scripts/chrome-devtools-mcp.ts new-page --url https://example.com --output markdown`
3. For page interaction, prefer text snapshots over screenshots:
   - `./scripts/chrome-devtools-mcp.ts take-snapshot --output markdown`
4. Use snapshot `uid` values for actions like `click`, `fill`, `hover`, `drag`, and `upload-file`.
5. For debugging, inspect runtime state:
   - `list-console-messages`
   - `list-network-requests`
   - `get-network-request`
6. For audits, use:
   - `lighthouse-audit`
   - `performance-start-trace`
   - `performance-stop-trace`

# Common command groups
## Navigation and page selection
- `list-pages`
- `new-page --url <url>`
- `select-page --page-id <id>`
- `navigate-page [--type url|back|forward|reload] [--url <url>]`
- `close-page --page-id <id>`
- `resize-page --width <n> --height <n>`

## Interaction
- `take-snapshot [--verbose true]`
- `click --uid <uid>`
- `hover --uid <uid>`
- `fill --uid <uid> --value <text>`
- `fill-form --elements <...>`
- `type-text --text <text>`
- `press-key --key <key>`
- `drag --from-uid <uid> --to-uid <uid>`
- `upload-file --uid <uid> --file-path <path>`
- `wait-for --text <value1,value2>`
- `handle-dialog --action accept|dismiss`

## Inspection and debugging
- `evaluate-script --function '<js-function>'`
- `list-console-messages`
- `get-console-message --msgid <id>`
- `list-network-requests`
- `get-network-request [--reqid <id>]`
- `take-screenshot [--full-page true] [--file-path <path>]`
- `take-memory-snapshot --file-path <path>`

## Performance and emulation
- `emulate [--network-conditions ...] [--cpu-throttling-rate <n>] [--viewport <spec>]`
- `lighthouse-audit [--mode navigation|snapshot] [--device desktop|mobile]`
- `performance-start-trace [--reload true|false] [--auto-stop true|false]`
- `performance-stop-trace`
- `performance-analyze-insight --insight-set-id <id> --insight-name <name>`

# Examples
Open a page and inspect its structure:

```bash
./scripts/chrome-devtools-mcp.ts new-page --url https://example.com --output markdown
./scripts/chrome-devtools-mcp.ts take-snapshot --output markdown
```

Click an element from the latest snapshot:

```bash
./scripts/chrome-devtools-mcp.ts click --uid node-123 --output markdown
```

Check recent console and network activity:

```bash
./scripts/chrome-devtools-mcp.ts list-console-messages --output markdown
./scripts/chrome-devtools-mcp.ts list-network-requests --output markdown
```

Run a Lighthouse audit:

```bash
./scripts/chrome-devtools-mcp.ts lighthouse-audit --mode navigation --device desktop --output markdown
```

# Notes
- First run may be slower because `npx` may need to download the MCP package.
- By default the skill auto-detects a usable Chrome channel in this order: `stable -> beta -> dev -> canary`.
- Use `--channel <channel>` or `CHROME_DEVTOOLS_MCP_CHANNEL=<channel>` when you need to force a specific browser build.
- Prefer `take-snapshot` over `take-screenshot` when you need stable element identifiers.
- Use `--output markdown` when you want more readable structured output in agent logs.
- Use `--raw '<json>'` for complex argument payloads that are awkward to express as flags.

# Troubleshooting
- `bun: command not found`: install Bun and ensure it is on `PATH`.
- `npx: command not found`: install Node.js/npm and ensure `npx` is on `PATH`.
- Requested channel is unavailable: rerun with `--channel stable` or set `CHROME_DEVTOOLS_MCP_CHANNEL=stable`.
- Package download or startup errors: retry and verify network access.
- Tool invocation timeouts: rerun with a larger global timeout, for example `--timeout 60000`.
