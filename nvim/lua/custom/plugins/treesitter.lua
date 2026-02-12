return {
    {
        -- Highlight, edit, and navigate code
        'nvim-treesitter/nvim-treesitter',
        branch = 'main',
        lazy = false,
        build = ':TSUpdate',
        config = function()
            -- The rewrite currently exposes configuration only for parser/query install location.
            require('nvim-treesitter').setup {}

            -- Enable treesitter-backed features only when the parser is available.
            local treesitter_augroup = vim.api.nvim_create_augroup('treesitter-compat', { clear = true })
            vim.api.nvim_create_autocmd('FileType', {
                group = treesitter_augroup,
                callback = function()
                    local lang = vim.treesitter.language.get_lang(vim.bo.filetype)
                    if not lang then
                        return
                    end

                    local ok, parser_available = pcall(vim.treesitter.language.add, lang)
                    if not ok or not parser_available then
                        return
                    end

                    -- Start parser + features per buffer/language.
                    pcall(vim.treesitter.start, 0, lang)

                    if vim.bo.filetype ~= 'python' then
                        vim.bo.indentexpr = "v:lua.require'nvim-treesitter'.indentexpr()"
                    end

                    -- Fold options are kept at Neovim defaults (disabled) so files do not open folded.
                end,
            })

            -- Parser installation is now explicitly managed via:
            -- :TSInstall c cpp go lua vim python rust typescript vimdoc java kotlin
            -- or `:TSInstall all`. This avoids parser compilation during every startup.
        end,
    },

    -- Additional text objects via treesitter
    {
        'nvim-treesitter/nvim-treesitter-textobjects',
        -- Use the rewrite-compatible plugin release.
        branch = 'main',
        event = { 'BufReadPost', 'BufNewFile' },
        config = function()
            require('nvim-treesitter-textobjects').setup {
                select = {
                    enable = true,
                    lookahead = true, -- Automatically jump forward to textobj, similar to targets.vim
                    keymaps = {
                        -- You can use the capture groups defined in textobjects.scm
                        ['aa'] = '@parameter.outer',
                        ['ia'] = '@parameter.inner',
                        ['af'] = '@function.outer',
                        ['if'] = '@function.inner',
                        ['ac'] = '@class.outer',
                        ['ic'] = '@class.inner',
                    },
                },
                move = {
                    enable = true,
                    set_jumps = true, -- whether to set jumps in the jumplist
                    goto_next_start = {
                        [']m'] = '@function.outer',
                        [']]'] = '@class.outer',
                    },
                    goto_next_end = {
                        [']M'] = '@function.outer',
                        [']['] = '@class.outer',
                    },
                    goto_previous_start = {
                        ['[m'] = '@function.outer',
                        ['[['] = '@class.outer',
                    },
                    goto_previous_end = {
                        ['[M'] = '@function.outer',
                        ['[]'] = '@class.outer',
                    },
                },
                swap = {
                    enable = true,
                    swap_next = {
                        ['<leader>a'] = '@parameter.inner',
                    },
                    swap_previous = {
                        ['<leader>A'] = '@parameter.inner',
                    },
                },
            }
        end,
    },
}
