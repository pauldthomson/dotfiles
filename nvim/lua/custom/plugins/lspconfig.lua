return { -- LSP Configuration & Plugins
    {
        'neovim/nvim-lspconfig',
        event = { 'BufReadPre', 'BufNewFile' },
        dependencies = {
            -- Automatically install LSPs to stdpath for neovim
            'williamboman/mason.nvim',
            'williamboman/mason-lspconfig.nvim',

            -- Useful status updates for LSP
            'j-hui/fidget.nvim',

            -- Additional lua configuration, makes nvim stuff amazing
            'folke/neodev.nvim',
        },
        config = function()
            -- vim.lsp.enable('kotlin_lsp')
            require('custom.kotlin_lsp').setup()
        end
    },
    {
        'mason-org/mason.nvim',
        cmd = { 'Mason', 'MasonInstall', 'MasonUninstall', 'MasonUninstallAll', 'MasonLog', 'MasonUpdate' },
        opts = {
            providers = {
                "mason.providers.client",
                "mason.providers.registry-api"
            },
            log_level = vim.log.levels.DEBUG
        }
    },
    {
        'j-hui/fidget.nvim',
        event = 'LspAttach',
        opts = {}
    },
    {
        'mason-org/mason-lspconfig.nvim',
        event = { 'BufReadPre', 'BufNewFile' },
        config = function()
            local servers = {
                -- clangd = {},
                gopls = {
                    completeUnimported = true,
                    usePlaceholders = true,
                    analyses = {
                        unusedparams = true,
                    }
                },
                -- pyright = {},
                -- rust_analyzer = {},
                -- tsserver = {},
                jdtls = {},
                yamlls = {
                    yaml = {
                        keyOrdering = false,
                        schemaStore = {
                            enable = true,
                        },
                    },
                },
                terraformls = {},
                diagnosticls = {},
                lua_ls = {
                    Lua = {
                        workspace = { checkThirdParty = false },
                        telemetry = { enable = false },
                    },
                },
                omnisharp = {},
                omnisharp_mono = {},
                csharp_ls = {},
                bashls = {},
                marksman = {},
                dockerls = {},
                docker_compose_language_service = {},
                gradle_ls = {},
            }

            vim.api.nvim_create_autocmd("LspAttach", {
                callback = function(args)
                    local client = assert(vim.lsp.get_client_by_id(args.data.client_id), "must have valid client")
                    -- NOTE: Remember that lua is a real programming language, and as such it is possible
                    -- to define small helper and utility functions so you don't have to repeat yourself
                    -- many times.
                    --
                    -- In this case, we create a function that lets us more easily define mappings specific
                    -- for LSP related items. It sets the mode, buffer and description for us each time.
                    local nmap = function(keys, func, desc)
                        if desc then
                            desc = 'LSP: ' .. desc
                        end

                        vim.keymap.set('n', keys, func, { buffer = 0, desc = desc })
                    end

                    nmap('<leader>rn', vim.lsp.buf.rename, '[R]e[n]ame')
                    nmap('<leader>ca', vim.lsp.buf.code_action, '[C]ode [A]ction')

                    nmap('gd', require('telescope.builtin').lsp_definitions, '[G]oto [D]efinition')
                    nmap('gr', require('telescope.builtin').lsp_references, '[G]oto [R]eferences')
                    nmap('gI', require('telescope.builtin').lsp_implementations, '[G]oto [I]mplementation')
                    nmap('<leader>D', vim.lsp.buf.type_definition, 'Type [D]efinition')
                    nmap('<leader>ds', require('telescope.builtin').lsp_document_symbols, '[D]ocument [S]ymbols')
                    nmap('<leader>ws', require('telescope.builtin').lsp_dynamic_workspace_symbols,
                        '[W]orkspace [S]ymbols')

                    -- See `:help K` for why this keymap
                    nmap('K', vim.lsp.buf.hover, 'Hover Documentation')
                    vim.keymap.set('i', '<C-k>', vim.lsp.buf.signature_help,
                        { buffer = 0, desc = 'Signature Documentation' })

                    -- Lesser used LSP functionality
                    nmap('gD', vim.lsp.buf.declaration, '[G]oto [D]eclaration')
                    nmap('<leader>wa', vim.lsp.buf.add_workspace_folder, '[W]orkspace [A]dd Folder')
                    nmap('<leader>wr', vim.lsp.buf.remove_workspace_folder, '[W]orkspace [R]emove Folder')
                    nmap('<leader>wl', function()
                        print(vim.inspect(vim.lsp.buf.list_workspace_folders()))
                    end, '[W]orkspace [L]ist Folders')

                    -- Create a command `:Format` local to the LSP buffer
                    vim.api.nvim_buf_create_user_command(0, 'Format', function(_)
                        vim.lsp.buf.format()
                    end, { desc = 'Format current buffer with LSP' })

                    if client.server_capabilities.documentFormattingProvider then
                        vim.api.nvim_command [[augroup Format]]
                        vim.api.nvim_command [[autocmd! * <buffer>]]
                        vim.api.nvim_command [[autocmd BufWritePre <buffer> lua vim.lsp.buf.format()]]
                        vim.api.nvim_command [[augroup END]]
                    end

                    vim.api.nvim_create_autocmd("BufWritePre", {
                        pattern = "*.go",
                        callback = function()
                            local params = vim.lsp.util.make_range_params()
                            params.context = { only = { "source.organizeImports" } }
                            -- buf_request_sync defaults to a 1000ms timeout. Depending on your
                            -- machine and codebase, you may want longer. Add an additional
                            -- argument after params if you find that you have to write the file
                            -- twice for changes to be saved.
                            -- E.g., vim.lsp.buf_request_sync(0, "textDocument/codeAction", params, 3000)
                            local result = vim.lsp.buf_request_sync(0, "textDocument/codeAction", params)
                            for cid, res in pairs(result or {}) do
                                for _, r in pairs(res.result or {}) do
                                    if r.edit then
                                        local enc = (vim.lsp.get_client_by_id(cid) or {}).offset_encoding or "utf-16"
                                        vim.lsp.util.apply_workspace_edit(r.edit, enc)
                                    end
                                end
                            end
                            vim.lsp.buf.format({ async = false })
                        end
                    })
                    -- vim.api.nvim_create_autocmd('BufWritePre', {
                    --     pattern = '*.go',
                    --     callback = function()
                    --         vim.lsp.buf.code_action({ context = { only = { 'source.organizeImports' } }, apply = true })
                    --     end
                    -- })
                end,
            })

            local capabilities = vim.lsp.protocol.make_client_capabilities()
            capabilities = require('cmp_nvim_lsp').default_capabilities(capabilities)

            local mason_lspconfig = require 'mason-lspconfig'

            mason_lspconfig.setup {
                ensure_installed = vim.tbl_keys(servers),
            }
        end
    }
}
