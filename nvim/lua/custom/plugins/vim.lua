return {
    {
        'nvim-lualine/lualine.nvim', -- Fancier statusline
        opts = {
            options = {
                theme = 'catppuccin',
                component_separators = " ",
                section_separators = { left = "", right = "" },
            }
        }
    },
    {
        'numToStr/Comment.nvim', -- "gc" to comment visual regions/lines
        lazy = false,
        opts = {}
    },

    'tpope/vim-sleuth', -- Detect tabstop and shiftwidth automatically
}
