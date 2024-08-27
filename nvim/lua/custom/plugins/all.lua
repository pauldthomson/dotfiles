return {
    { -- LSP Configuration & Plugins
        'neovim/nvim-lspconfig',
        dependencies = {
            -- Automatically install LSPs to stdpath for neovim
            'williamboman/mason.nvim',
            'williamboman/mason-lspconfig.nvim',

            -- Useful status updates for LSP
            'j-hui/fidget.nvim',

            -- Additional lua configuration, makes nvim stuff amazing
            'folke/neodev.nvim',
        },
    },

    { -- Autocompletion
        'hrsh7th/nvim-cmp',
        lazy = false,
        dependencies = {
            'onsails/lspkind-nvim',
            'hrsh7th/cmp-nvim-lsp',
            'hrsh7th/cmp-path',
            'hrsh7th/cmp-buffer',
            'L3MON4D3/LuaSnip',
            'saadparwaiz1/cmp_luasnip'
        },
    },

    { -- Highlight, edit, and navigate code
        'nvim-treesitter/nvim-treesitter',
        build = ':TSUpdate'
    },

    -- Additional text objects via treesitter
    'nvim-treesitter/nvim-treesitter-textobjects',

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

    'navarasu/onedark.nvim', -- Theme inspired by Atom

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

    -- Fuzzy Finder (files, lsp, etc)
    {
        'nvim-telescope/telescope.nvim',
        branch = '0.1.x',
        dependencies = {
            'nvim-lua/plenary.nvim'
        },
        opts = {
            defaults = {
                mappings = {
                    i = {
                        ['<C-u>'] = false,
                        ['<C-d>'] = false,
                    },
                },
            },
        }
    },

    -- Fuzzy Finder Algorithm which requires local dependencies to be built. Only load if `make` is available
    {
        'nvim-telescope/telescope-fzf-native.nvim',
        build = 'make',
        cond = vim.fn.executable 'make' == 1
    },

    -- File explorer
    {
        "nvim-neo-tree/neo-tree.nvim",
        branch = "v2.x",
        dependencies = {
            "nvim-lua/plenary.nvim",
            "nvim-tree/nvim-web-devicons", -- not strictly required, but recommended
            "MunifTanjim/nui.nvim",
        }
    },

    {
        'christoomey/vim-tmux-navigator',
        cmd = {
            "TmuxNavigateLeft",
            "TmuxNavigateDown",
            "TmuxNavigateUp",
            "TmuxNavigateRight",
            "TmuxNavigatePrevious",
        },
        lazy = false,
        keys = {
            { "<C-h>",  "<cmd><C-U>TmuxNavigateLeft<cr>" },
            { "<C-j>",  "<cmd><C-U>TmuxNavigateDown<cr>" },
            { "<C-k>",  "<cmd><C-U>TmuxNavigateUp<cr>" },
            { "<C-l>",  "<cmd><C-U>TmuxNavigateRight<cr>" },
            { "<C-\\>", "<cmd><C-U>TmuxNavigatePrevious<cr>" },
        }
    },

    {
        "folke/trouble.nvim",
        dependencies = "kyazdani42/nvim-web-devicons",
        config = [[ function()
      require("trouble").setup {
        -- your configuration comes here
        -- or leave it empty to use the default settings
        -- refer to the configuration section below
      }
    end ]]
    },

    {
        "klen/nvim-test",
        -- config = function()
        --   require('nvim-test').setup()
        -- end
    },

    'hashivim/vim-terraform',

    {
        "iamcco/markdown-preview.nvim",
        cmd = {
            "MarkdownPreviewToggle",
            "MarkdownPreview",
            "MarkdownPreviewStop"
        },
        ft = {
            "markdown"
        },
        build = function() vim.fn["mkdp#util#install"]() end,
    },
    -- nvim v0.8.0
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
        "nvim-telescope/telescope-file-browser.nvim",
        dependencies = { "nvim-telescope/telescope.nvim", "nvim-lua/plenary.nvim" }
    },
    {
        "windwp/nvim-autopairs",
        -- Optional dependency
        dependencies = { 'hrsh7th/nvim-cmp' },
        config = function()
            require("nvim-autopairs").setup {}
            -- If you want to automatically add `(` after selecting a function or method
            local cmp_autopairs = require('nvim-autopairs.completion.cmp')
            local cmp = require('cmp')
            cmp.event:on(
                'confirm_done',
                cmp_autopairs.on_confirm_done()
            )
        end,
    },
    { "mistricky/codesnap.nvim", build = "make" },
    {
        "klen/nvim-test",
        config = {
            termOpts = {
                direction = "horizontal"
            },
        },
    },
    "tpope/vim-dadbod",
    "kristijanhusak/vim-dadbod-completion",
    "kristijanhusak/vim-dadbod-ui",

    {
        "catppuccin/nvim",
        name = "catppuccin",
        priority = 1000,
    }
}
