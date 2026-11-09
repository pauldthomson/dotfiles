-- Observe only the newly created terminal. This is not a readiness handshake:
-- no input is sent, no terminal output is copied, and no resource is closed.
local status, err = rex.call("server.status")
if err then error(err, 0) end
local result = { host_id = status.host_id, instance_id = status.instance_id, state = "unknown" }
local _, attach_err = rex.call("session.attach", { session_id = rex.args.session_id })
if attach_err then return result end
for attempt = 1, 11 do
  local process, process_err = rex.call("block.method", {
    session_id = rex.args.session_id, block_id = rex.args.block_id, method = "process",
  })
  if process_err or not process then return result end
  if process.last_exit then
    result.state = "exited"
    result.exit_code = process.last_exit.exit_code
    return result
  end
  if attempt == 11 then
    if process.child or process.foreground then result.state = "observed" end
    return result
  end
  rex.sleep(0.1)
end
return result
