return {
    -- Fuzzy Finder (files, lsp, etc)
    {
        "nvim-telescope/telescope.nvim",
        -- tag = "0.2.x",
        dependencies = {
            "nvim-lua/plenary.nvim",
        },
        opts = {
            defaults = {
                mappings = {
                    i = {
                        ["<C-u>"] = false,
                        ["<C-d>"] = false,
                        ["<C-r>"] = require("telescope.actions").to_fuzzy_refine,
                    },
                },
            },
            pickers = {
                live_grep = {
                    additional_args = function()
                        return { "--hidden" }
                    end,
                },
            },
            extensions = {
                advanced_git_search = {
                    -- Browse command to open commits in browser. Default fugitive GBrowse.
                    -- {commit_hash} is the placeholder for the commit hash.
                    browse_command = "GBrowse {commit_hash}",
                    -- when {commit_hash} is not provided, the commit will be appended to the specified command seperated by a space
                    -- browse_command = "GBrowse",
                    -- => both will result in calling `:GBrowse commit`

                    -- fugitive or diffview
                    diff_plugin = "fugitive",
                    -- customize git in previewer
                    -- e.g. flags such as { "--no-pager" }, or { "-c", "delta.side-by-side=false" }
                    git_flags = {},
                    -- customize git diff in previewer
                    -- e.g. flags such as { "--raw" }
                    git_diff_flags = {},
                    git_log_flags = {},
                    -- Show builtin git pickers when executing "show_custom_functions" or :AdvancedGitSearch
                    show_builtin_git_pickers = false,
                    entry_default_author_or_date = "author", -- one of "author" or "date"
                    keymaps = {
                        -- following keymaps can be overridden
                        toggle_date_author = "<C-w>",
                        open_commit_in_browser = "<C-o>",
                        copy_commit_hash = "<C-y>",
                        show_entire_commit = "<C-e>",
                    },

                    -- Telescope layout setup
                    telescope_theme = {
                        function_name_1 = {
                            -- Theme options
                        },
                        function_name_2 = "dropdown",
                        -- e.g. realistic example
                        show_custom_functions = {
                            layout_config = { width = 0.4, height = 0.4 },
                        },
                    },
                    file_browser = {},
                },
            },
        },
    },

    -- Fuzzy Finder Algorithm which requires local dependencies to be built. Only load if `make` is available
    {
        "nvim-telescope/telescope-fzf-native.nvim",
        build = "make",
        cond = vim.fn.executable("make") == 1,
    },
    {
        "nvim-telescope/telescope-file-browser.nvim",
        dependencies = { "nvim-telescope/telescope.nvim", "nvim-lua/plenary.nvim" },
    },
}
