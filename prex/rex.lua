-- Optional Rex actions. Load with dofile(...) from ~/.config/rex/init.lua.
-- Uses temporary native tabs; floating layers currently clip in Rex.app.
local binary = os.getenv("HOME") .. "/go/bin/prex"

local function picker(ctx, kill)
  if not ctx.session_id or not ctx.client_id then
    error("Open a Rex session first, or run prex from a terminal", 0)
  end
  local view, err = rex.session.view{ session_id = ctx.session_id }
  if err then error(err, 0) end
  local command = { binary, "--_picker" }
  local caller
  caller, err = rex.call("client.inspect", { client_id = ctx.client_id })
  if err then error(err, 0) end
  -- Keyboard/palette actions identify the app. CLI actions identify a
  -- short-lived CLI connection, which must not be passed as an app target.
  if caller.client.info and caller.client.info.kind == "app" then
    table.insert(command, "--client")
    table.insert(command, ctx.client_id)
  end
  if view.active_window_id then
    table.insert(command, "--_return-window")
    table.insert(command, view.active_window_id)
  end
  if kill then table.insert(command, "kill") end
  local layout = rex.layout.block{
    flavor = "com.superlogical.terminal.shell",
    options = {
      command = command,
      cwd = os.getenv("HOME"),
      -- Default login environment supplies PATH to prex's subprocesses.
      -- No interactive shell, and no wait inside this action.
    },
  }
  -- Normal exit removes the temporary tab. The picker restores the source
  -- tab if it still has focus, but leaves a newly launched agent focused.
  local result
  result, err = rex.session.new_window{
    session_id = ctx.session_id,
    window_label = kill and "Close current session" or "Projects",
    layout = layout,
    focus = true,
  }
  if err then error(err, 0) end
  -- Both picker commands explicitly select their own app tab on startup.
  -- Do not rely on a queued client action or server-side focus alone.
  return result
end

rex.action{
  name = "prex_projects",
  title = "Open Project Agent",
  description = "Switch, reopen Pi, or clone another project copy",
  run = function(ctx) return picker(ctx, false) end,
}
rex.action{
  name = "prex_close",
  title = "Close Current Session",
  description = "Close this Rex session with guarded clone cleanup",
  run = function(ctx) return picker(ctx, true) end,
}

rex.bind("cmd+alt+p", "prex_projects")
rex.bind("cmd+alt+x", "prex_close")
