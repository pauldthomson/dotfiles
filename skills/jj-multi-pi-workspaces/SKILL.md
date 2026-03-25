---
name: jj-multi-pi-workspaces
description: Set up and manage Jujutsu workspaces for running multiple Pi instances on the same repository. Use when users ask to parallelize work across agents, create workspace-per-agent directories, recover stale workspaces, or clean up workspace mappings.
---

# JJ Multi-Pi Workspaces

Use this skill to create a clean, repeatable **one-workspace-per-agent** setup for Pi.

## When to use

Use this skill when the user asks to:

- run multiple Pi instances on one codebase
- set up `jj workspace` directories
- recover stale workspace state
- remove/forget old workspaces

## Core principles

1. **One Pi instance per workspace directory**
2. **Shared repo history, separate working copies**
3. **Workspace names should be stable and human-readable** (for example: `pi-a`, `pi-b`, `integration`)
4. **Do not delete directories unless explicitly asked**
5. **Warn that bookmarks are shared across workspaces**

## Standard setup flow

1. Confirm current repository/workspace context:

```bash
jj root
jj workspace list
```

2. Pick names and destinations. Default convention:

- workspace names: `pi-a`, `pi-b`, `pi-c`, ...
- directories: sibling folders like `../<repo>-pi-a`, `../<repo>-pi-b`

3. Create each workspace:

```bash
jj workspace add ../<repo>-pi-a --name pi-a
jj workspace add ../<repo>-pi-b --name pi-b
```

Optional: create from a specific revision:

```bash
jj workspace add ../<repo>-pi-a --name pi-a -r <revset>
```

4. Verify:

```bash
jj workspace list
```

5. Give launch instructions for separate Pi instances:

```bash
cd ../<repo>-pi-a && pi
cd ../<repo>-pi-b && pi
```

## Stale workspace recovery

If user reports stale working copy warnings, run in affected workspace:

```bash
jj workspace update-stale
```

Then re-check:

```bash
jj st
```

## Cleanup flow

To stop tracking a workspace mapping without deleting files:

```bash
jj workspace forget <workspace-name>
```

Only remove directories if the user explicitly asks.

## Safety notes to always mention

- Workspaces have separate `@` working-copy commits.
- Commit graph and bookmarks are shared across workspaces.
- Two agents moving the same bookmark can conflict; coordinate bookmark moves intentionally.

## Response template

When done, summarize with:

1. workspaces created/updated/forgotten
2. paths for each Pi instance
3. any warnings (shared bookmark risk, stale updates applied)
