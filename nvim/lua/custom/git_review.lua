local M = {}

local function trim(text)
  if not text then
    return ""
  end
  return vim.trim(text)
end

local function run(cmd)
  local result = vim.system(cmd, { text = true }):wait()
  if result.code ~= 0 then
    return nil, trim(result.stderr ~= "" and result.stderr or result.stdout)
  end
  return trim(result.stdout), nil
end

local function notify_error(message)
  vim.notify(message, vim.log.levels.ERROR)
end

local function load_plugin(name)
  local ok, lazy = pcall(require, "lazy")
  if ok then
    lazy.load({ plugins = { name } })
  end
end

local function resolve_base_ref(explicit_base)
  if explicit_base and explicit_base ~= "" then
    return explicit_base
  end

  local remote_head = run({ "git", "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD" })
  if remote_head and remote_head ~= "" then
    return remote_head:gsub("^refs/remotes/", "")
  end

  for _, candidate in ipairs({ "origin/main", "origin/master", "main", "master" }) do
    local ok = run({ "git", "rev-parse", "--verify", "--quiet", candidate })
    if ok then
      return candidate
    end
  end

  return nil, "Could not resolve a PR base branch. Try :DiffviewPrReview <base>."
end

local function review_range(base)
  return string.format("%s...HEAD", base)
end

function M.diffview_toggle(base)
  load_plugin("diffview.nvim")

  local lib = require("diffview.lib")
  if lib.get_current_view() then
    vim.cmd("DiffviewClose")
    return
  end

  local resolved_base, err = resolve_base_ref(base)
  if not resolved_base then
    notify_error(err)
    return
  end

  vim.cmd("DiffviewOpen " .. review_range(resolved_base))
end

function M.setup_commands()
  if vim.g.custom_git_review_commands_loaded then
    return
  end
  vim.g.custom_git_review_commands_loaded = true

  vim.api.nvim_create_user_command("DiffviewPrReview", function(opts)
    M.diffview_toggle(opts.args)
  end, {
    nargs = "?",
    desc = "Toggle a Diffview PR review against origin/main or origin/master",
  })
end

return M
