return {
    {
        'nvim-lualine/lualine.nvim', -- Fancier statusline
        opts = {
            options = {
                theme = 'catppuccin',
                component_separators = '|',
                section_separators = '',
            }
        }
    },

    {
        'lukas-reineke/indent-blankline.nvim', -- Add indentation guides even on blank lines
        main = "ibl",
        opts = {}
    },

    {
        'numToStr/Comment.nvim', -- "gc" to comment visual regions/lines
        lazy = false,
        opts = {}
    },

    'tpope/vim-sleuth', -- Detect tabstop and shiftwidth automatically
}
