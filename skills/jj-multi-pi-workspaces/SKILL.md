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
5. **When deleting a workspace, forget it in Jujutsu first, then remove the workspace directory**
6. **Warn that bookmarks are shared across workspaces**

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

If the user explicitly asks to fully clean up a workspace, do both steps in this order:

1. Find the workspace path from:

```bash
jj workspace list
```

2. Forget the workspace mapping:

```bash
jj workspace forget <workspace-name>
```

3. Remove the workspace directory that was created for that workspace:

```bash
rm -rf ../<repo>-pi-a
```

Notes:

- Prefer using the exact path shown or implied by `jj workspace list` / the original `jj workspace add` command.
- Double-check that the path is the workspace directory being removed, not the main repo checkout.
- Never delete directories unless the user explicitly asked for cleanup/removal.

## Safety notes to always mention

- Workspaces have separate `@` working-copy commits.
- Commit graph and bookmarks are shared across workspaces.
- Two agents moving the same bookmark can conflict; coordinate bookmark moves intentionally.

## Response template

When done, summarize with:

1. workspaces created/updated/forgotten
2. paths for each Pi instance
3. any warnings (shared bookmark risk, stale updates applied)
