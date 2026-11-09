-- Read-only collection over one CLI connection. Matching and filesystem checks
-- remain in Go. Include detached blocks; never use session.view for discovery.
local status, err = rex.call("server.status")
if err then error(err, 0) end
local attached
attached, err = rex.call("session.attach_all", { event_scope = "session" })
if err then error(err, 0) end

local result = {
  host_id = status.host_id,
  instance_id = status.instance_id,
  session_count = 0,
  sessions = {},
}
local function inspect(sid, entry)
  local listed, list_err = rex.call("session.list_blocks", { session_id = sid })
  if list_err then
    entry.error = tostring(list_err)
  elseif not listed then
    entry.error = "Rex omitted the block list"
  else
    entry.blocks = {}
    for _, block in ipairs(listed.blocks or {}) do
      local item = { host_id = block.host_id, creator_name = block.creator_name }
      entry.blocks[block.block_id] = item
      -- Do not inspect remote or non-terminal blocks. Go rejects the session
      -- rather than treating a partial view as a verified repository match.
      if block.host_id == status.host_id and block.creator_name == "com.superlogical.terminal" then
        local process, process_err = rex.call("block.method", {
          session_id = sid, block_id = block.block_id, method = "process",
        })
        if process_err then
          item.error = tostring(process_err)
        else
          -- Return only the fields prex needs, not process names or arguments.
          item.process = {
            last_exit = process.last_exit,
            child = process.child and { cwd = process.child.cwd } or nil,
            foreground = process.foreground and { cwd = process.foreground.cwd } or nil,
          }
        end
      end
    end
  end
end
for position, session in ipairs(attached.sessions or {}) do
  local sid = session.session_id
  local entry = { label = session.label, position = position }
  result.session_count = result.session_count + 1
  result.sessions[sid] = entry
  if not rex.args.known_sessions[sid] then inspect(sid, entry) end
end
return result
