return {
    {
        'nvim-lualine/lualine.nvim', -- Fancier statusline
        event = 'VeryLazy',
        opts = {
            options = {
                theme = 'catppuccin',
                component_separators = " ",
                section_separators = { left = "", right = "" },
            },
            sections = {
                lualine_c = {
                    {
                        'filename',
                        path = 1,
                    }
                }
            }
        }
    },
    {
        'numToStr/Comment.nvim', -- "gc" to comment visual regions/lines
        event = { 'BufReadPre', 'BufNewFile' },
        opts = {}
    },

    {
        'tpope/vim-sleuth', -- Detect tabstop and shiftwidth automatically
        event = { 'BufReadPre', 'BufNewFile' },
    },
}
