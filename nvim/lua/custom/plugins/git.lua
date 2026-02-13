return {
    -- Git related plugins
    {
        'tpope/vim-fugitive',
        cmd = { 'Git', 'G', 'GBrowse', 'Gdiffsplit', 'Gvdiffsplit' },
        dependencies = {
            'tpope/vim-rhubarb',
        },
    },
    {
        'tpope/vim-rhubarb',
    },
    {
        'lewis6991/gitsigns.nvim',
        event = { 'BufReadPre', 'BufNewFile' },
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
    {
        "aaronhallaert/advanced-git-search.nvim",
        cmd = { "AdvancedGitSearch" },
        config = function()
            require("telescope").load_extension("advanced_git_search")
        end,
        dependencies = {
            "nvim-telescope/telescope.nvim",
            -- to show diff splits and open commits in browser
            "tpope/vim-fugitive",
            -- to open commits in browser with fugitive
            "tpope/vim-rhubarb",
            -- optional: to replace the diff from fugitive with diffview.nvim
            -- (fugitive is still needed to open in browser)
            -- "sindrets/diffview.nvim",
        },
    },
    {
        'sindrets/diffview.nvim',
        cmd = { 'DiffviewOpen', 'DiffviewClose', 'DiffviewFileHistory' },
    }
}
