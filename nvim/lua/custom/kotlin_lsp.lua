local M = {}

local function virtual_text_document_ktls(params)
    local bufnr = params.buf
    local actual_path = params.match

    local clients = vim.lsp.get_clients({ name = "kotlin_lsp" })
    if #clients == 0 then
        return
    end

    local client = clients[1]
    local method = "workspace/executeCommand"
    local req_params = { command = "decompile", arguments = { actual_path } }
    local response = client:request_sync(method, req_params, 2000, 0)

    if not response or type(response.result) ~= "table" or type(response.result.code) ~= "string" then
        return
    end

    local lines = vim.split(response.result.code, "\n", { plain = true })

    vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, lines)
    vim.api.nvim_set_option_value("readonly", true, { buf = bufnr })
    vim.api.nvim_set_option_value("modified", false, { buf = bufnr })
    vim.api.nvim_set_option_value("modifiable", false, { buf = bufnr })
    vim.api.nvim_buf_set_name(bufnr, actual_path)

    local filetype = response.result.language or "kotlin"
    vim.api.nvim_set_option_value("filetype", filetype, { buf = bufnr })
end

function M.setup()
    vim.api.nvim_create_autocmd("BufReadCmd", {
        pattern = { "jar:/*", "jrt:/*" },
        callback = virtual_text_document_ktls,
        desc = "Kotlin LSP decompile jar sources",
    })
end

return M
