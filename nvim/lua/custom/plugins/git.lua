return {
    -- Git related plugins
    'tpope/vim-fugitive',
    'tpope/vim-rhubarb',
    {
        'lewis6991/gitsigns.nvim',
        opts = {
            signs = {
                add = { text = '+' },
                change = { text = '~' },
                delete = { text = '_' },
                topdelete = { text = '‾' },
                changedelete = { text = '~' },
            },
            on_attach = function()
                vim.keymap.set({ 'n', 'v' }, '<leader>hs', ':Gitsigns stage_hunk<cr>')
                vim.keymap.set({ 'n', 'v' }, '<leader>hr', ':Gitsigns reset_hunk<cr>')
                vim.keymap.set({ 'n', 'v' }, '<leader>hn', ':Gitsigns next_hunk<cr>')
                vim.keymap.set({ 'n', 'v' }, '<leader>hp', ':Gitsigns prev_hunk<cr>')
                vim.keymap.set({ 'n', 'v' }, '<leader>ph', ':Gitsigns preview_hunk<cr>')
            end
        },
        config = true
    },
    {
        "kdheepak/lazygit.nvim",
        cmd = {
            "LazyGit",
            "LazyGitConfig",
            "LazyGitCurrentFile",
            "LazyGitFilter",
            "LazyGitFilterCurrentFile",
        },
        -- optional for floating window border decoration
        dependencies = {
            "nvim-lua/plenary.nvim",
        },
    },
}
