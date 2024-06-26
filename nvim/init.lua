local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"
if not vim.loop.fs_stat(lazypath) then
  vim.fn.system({
    "git",
    "clone",
    "--filter=blob:none",
    "https://github.com/folke/lazy.nvim.git",
    "--branch=stable", -- latest stable release
    lazypath,
  })
end
vim.opt.rtp:prepend(lazypath)

require('lazy').setup({
  -- Package manager
  'wbthomason/packer.nvim',

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
        theme = 'onedark',
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
        direction = "float"
      },
    },
  },
})

-- [[ Setting options ]]
-- See `:help vim.o`

-- Set highlight on search
vim.o.hlsearch = false

-- Make line numbers default
vim.wo.number = true

-- Enable mouse mode
vim.o.mouse = 'a'

-- Enable break indent
vim.o.breakindent = true

-- Save undo history
vim.o.undofile = true

-- Case insensitive searching UNLESS /C or capital in search
vim.o.ignorecase = true
vim.o.smartcase = true

-- Decrease update time
vim.o.updatetime = 250
vim.wo.signcolumn = 'yes'

-- Set colorscheme
vim.o.termguicolors = true
vim.cmd [[colorscheme onedark]]

-- Set completeopt to have a better completion experience
vim.o.completeopt = 'menu,menuone,noselect'

-- Fix splits
vim.o.splitbelow = true
vim.o.splitright = true

-- Fix tab size
vim.o.tabstop = 4
vim.o.softtabstop = 4
vim.o.shiftwidth = 4
vim.o.smarttab = true
vim.o.expandtab = true

vim.g.have_nerd_font = true

-- [[ Basic Keymaps ]]
-- Set <space> as the leader key
-- See `:help mapleader`
--  NOTE: Must happen before plugins are required (otherwise wrong leader will be used)
vim.g.mapleader = ' '
vim.g.maplocalleader = ' '

-- Keymaps for better default experience
-- See `:help vim.keymap.set()`
vim.keymap.set({ 'n', 'v' }, '<Space>', '<Nop>', { silent = true })

-- Remap for dealing with word wrap
vim.keymap.set('n', 'k', "v:count == 0 ? 'gk' : 'k'", { expr = true, silent = true })
vim.keymap.set('n', 'j', "v:count == 0 ? 'gj' : 'j'", { expr = true, silent = true })

-- [[ Highlight on yank ]]
-- See `:help vim.highlight.on_yank()`
local highlight_group = vim.api.nvim_create_augroup('YankHighlight', { clear = true })
vim.api.nvim_create_autocmd('TextYankPost', {
  callback = function()
    vim.highlight.on_yank()
  end,
  group = highlight_group,
  pattern = '*',
})

-- Configure Neotree
require('neo-tree').setup {
  filesystem = {
    follow_current_file = true,
    filtered_items = {
      hide_dotfiles = false
    }
  }
}

-- Enable telescope fzf native, if installed
pcall(require('telescope').load_extension, 'fzf')

-- See `:help telescope.builtin`
vim.keymap.set('n', '<leader>?', require('telescope.builtin').oldfiles, { desc = '[?] Find recently opened files' })
vim.keymap.set('n', '<leader><space>', require('telescope.builtin').buffers, { desc = '[ ] Find existing buffers' })
vim.keymap.set('n', '<leader>/', function()
  -- You can pass additional configuration to telescope to change theme, layout, etc.
  require('telescope.builtin').current_buffer_fuzzy_find(require('telescope.themes').get_dropdown {
    winblend = 10,
    previewer = false,
  })
end, { desc = '[/] Fuzzily search in current buffer]' })

vim.keymap.set('n', '<leader>sf', function()
  require('telescope.builtin').find_files({
    find_command = { "rg", "--files", "--hidden", "--follow", "--glob",
      "!.git/*" }
  })
end, { desc = '[S]earch [F]iles' })
vim.keymap.set('n', '<leader>sh', require('telescope.builtin').help_tags, { desc = '[S]earch [H]elp' })
vim.keymap.set('n', '<leader>sw', require('telescope.builtin').grep_string, { desc = '[S]earch current [W]ord' })
vim.keymap.set('n', '<leader>sg', require('telescope.builtin').live_grep, { desc = '[S]earch by [G]rep' })
vim.keymap.set('n', '<leader>sd', require('telescope.builtin').diagnostics, { desc = '[S]earch [D]iagnostics' })
vim.keymap.set('n', '<leader>sc', require('telescope.builtin').git_commits, { desc = '[S]earch [C]ommits' })

vim.keymap.set("n", "<space>fb", function()
  require("telescope").extensions.file_browser.file_browser({ select_buffer = true, path = '%:p:h' })
end)

-- Use jk to exit insert mode
vim.keymap.set('i', 'jk', '<ESC>', { noremap = true })

-- Save buffer
vim.keymap.set('n', '<leader>w', ':w<CR>', { noremap = true })

-- Copy and paste to system clipboard in visual mode
vim.keymap.set('v', '<leader>y', '\"+y', { noremap = false })
vim.keymap.set('n', '<leader>p', '\"+p', { noremap = false })

-- Toggle Neotree
vim.keymap.set('n', '<leader>n', ':NeoTreeShowToggle<CR>', { desc = 'Toggle [N]eoTree' })

-- Close quickfix
vim.keymap.set('n', '<leader>cc', ':ccl<CR>', { desc = 'Close quickfix list' })

-- Vim fugitive
vim.keymap.set('n', '<leader>gs', ':Git<CR>', { desc = '[G]it [S]tatus' })
vim.keymap.set('n', '<leader>gt', ':diffget //2<CR>', { desc = '[G]et [T]arget branch' })
vim.keymap.set('n', '<leader>gm', ':diffget //3<CR>', { desc = '[G]et [M]erge branch' })

-- Move visual selection
vim.keymap.set('v', 'J', ":m '>+1<CR>gv=gv")
vim.keymap.set('v', 'K', ":m '<-2<CR>gv=gv")

-- Keep cursor at start when appending lower line
vim.keymap.set('n', 'J', 'mzJ`z')

-- Keep cursor centered when half-page jumping
vim.keymap.set('n', '<C-d>', '<C-d>zz')
vim.keymap.set('n', '<C-u>', '<C-u>zz')

-- Keep cursor centered when searching
vim.keymap.set('n', 'n', 'nzzzv')
vim.keymap.set('n', 'N', 'Nzzzv')

-- Don't nuke paste buffer when pasting over text
vim.keymap.set('x', 'p', '\"_dP')

-- Don't nuke paste buffer when deleting
-- vim.keymap.set('n', 'd', '\"_d')
-- vim.keymap.set('v', 'd', '\"_d')

-- Run ptmux from nvim
vim.keymap.set('n', '<C-f>', '<cmd>silent !tmux neww ptmux<CR>')

-- Launch LazyGit
vim.keymap.set('n', '<leader>lg', ':LazyGit<CR>', { desc = '[L]azy [G]it' })

-- [[ Configure Treesitter ]]
-- See `:help nvim-treesitter`
require('nvim-treesitter.configs').setup {
  -- Add languages to be installed here that you want installed for treesitter
  ensure_installed = { 'c', 'cpp', 'go', 'lua', 'vim', 'python', 'rust', 'typescript', 'vimdoc', 'java', 'kotlin' },

  highlight = { enable = true },
  indent = { enable = true, disable = { 'python' } },
  incremental_selection = {
    enable = true,
    keymaps = {
      -- TODO: clashes with tmux
      --
      -- init_selection = '<c-space>',
      -- node_incremental = '<c-space>',
      -- scope_incremental = '<c-s>',
      -- node_decremental = '<c-backspace>',
      init_selection = "tis",
      node_incremental = "tni",
      scope_incremental = "tsi",
      node_decremental = "tnd",
    },
  },
  textobjects = {
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
  },
}

-- Diagnostic keymaps
vim.keymap.set('n', '[d', vim.diagnostic.goto_prev)
vim.keymap.set('n', ']d', vim.diagnostic.goto_next)
vim.keymap.set('n', '<leader>e', vim.diagnostic.open_float)
vim.keymap.set('n', '<leader>q', vim.diagnostic.setloclist)

-- LSP settings.
--  This function gets run when an LSP connects to a particular buffer.
local on_attach = function(client, bufnr)
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

    vim.keymap.set('n', keys, func, { buffer = bufnr, desc = desc })
  end

  nmap('<leader>rn', vim.lsp.buf.rename, '[R]e[n]ame')
  nmap('<leader>ca', vim.lsp.buf.code_action, '[C]ode [A]ction')

  nmap('gd', require('telescope.builtin').lsp_definitions, '[G]oto [D]efinition')
  nmap('gr', require('telescope.builtin').lsp_references, '[G]oto [R]eferences')
  nmap('gI', require('telescope.builtin').lsp_implementations, '[G]oto [I]mplementation')
  nmap('<leader>D', vim.lsp.buf.type_definition, 'Type [D]efinition')
  nmap('<leader>ds', require('telescope.builtin').lsp_document_symbols, '[D]ocument [S]ymbols')
  nmap('<leader>ws', require('telescope.builtin').lsp_dynamic_workspace_symbols, '[W]orkspace [S]ymbols')

  -- See `:help K` for why this keymap
  nmap('K', vim.lsp.buf.hover, 'Hover Documentation')
  vim.keymap.set('i', '<C-k>', vim.lsp.buf.signature_help, { buffer = bufnr, desc = 'Signature Documentation' })

  -- Lesser used LSP functionality
  nmap('gD', vim.lsp.buf.declaration, '[G]oto [D]eclaration')
  nmap('<leader>wa', vim.lsp.buf.add_workspace_folder, '[W]orkspace [A]dd Folder')
  nmap('<leader>wr', vim.lsp.buf.remove_workspace_folder, '[W]orkspace [R]emove Folder')
  nmap('<leader>wl', function()
    print(vim.inspect(vim.lsp.buf.list_workspace_folders()))
  end, '[W]orkspace [L]ist Folders')

  -- Create a command `:Format` local to the LSP buffer
  vim.api.nvim_buf_create_user_command(bufnr, 'Format', function(_)
    vim.lsp.buf.format()
  end, { desc = 'Format current buffer with LSP' })

  if client.server_capabilities.documentFormattingProvider then
    vim.api.nvim_command [[augroup Format]]
    vim.api.nvim_command [[autocmd! * <buffer>]]
    vim.api.nvim_command [[autocmd BufWritePre <buffer> lua vim.lsp.buf.format()]]
    vim.api.nvim_command [[augroup END]]
  end

  vim.api.nvim_create_autocmd('BufWritePre', {
    pattern = '*.go',
    callback = function()
      vim.lsp.buf.code_action({ context = { only = { 'source.organizeImports' } }, apply = true })
    end
  })
end

-- Setup mason so it can manage external tooling
require('mason').setup {
  providers = {
    "mason.providers.client",
    "mason.providers.registry-api"
  },
  log_level = vim.log.levels.DEBUG
}
-- Enable the following language servers
--  Feel free to add/remove any LSPs that you want here. They will automatically be installed.
--
--  Add any additional override configuration in the following tables. They will be passed to
--  the `settings` field of the server config. You must look up that documentation yourself.
local servers = {
  -- clangd = {},
  gopls = {},
  -- pyright = {},
  -- rust_analyzer = {},
  -- tsserver = {},
  jdtls = {},
  kotlin_language_server = {},
  yamlls = {
    yaml = {
      keyOrdering = false,
      schemaStore = {
        enable = true,
      },
      schemas = {
        ["https://raw.githubusercontent.com/instrumenta/kubernetes-json-schema/master/master/all.json"] = "/*",
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
}

-- Setup neovim lua configuration
require('neodev').setup()

-- nvim-cmp supports additional completion capabilities, so broadcast that to servers
local capabilities = vim.lsp.protocol.make_client_capabilities()
capabilities = require('cmp_nvim_lsp').default_capabilities(capabilities)

-- Setup mason so it can manage external tooling
require('mason').setup()

-- Ensure the servers above are installed
local mason_lspconfig = require 'mason-lspconfig'

mason_lspconfig.setup {
  ensure_installed = vim.tbl_keys(servers),
}

mason_lspconfig.setup_handlers {
  function(server_name)
    require('lspconfig')[server_name].setup {
      capabilities = capabilities,
      on_attach = on_attach,
      settings = servers[server_name],
    }
  end,
}

-- Turn on lsp status information
require('fidget').setup()

-- nvim-cmp setup
local cmp = require 'cmp'
local luasnip = require 'luasnip'

cmp.setup {
  snippet = {
    expand = function(args)
      luasnip.lsp_expand(args.body)
    end,
  },
  mapping = cmp.mapping.preset.insert {
    ['<C-d>'] = cmp.mapping.scroll_docs(-4),
    ['<C-f>'] = cmp.mapping.scroll_docs(4),
    ['<C-Space>'] = cmp.mapping.complete(),
    ['<CR>'] = cmp.mapping.confirm {
      behavior = cmp.ConfirmBehavior.Replace,
      select = true,
    },
    ['<Tab>'] = cmp.mapping(function(fallback)
      if cmp.visible() then
        cmp.select_next_item()
      elseif luasnip.expand_or_jumpable() then
        luasnip.expand_or_jump()
      else
        fallback()
      end
    end, { 'i', 's' }),
    ['<S-Tab>'] = cmp.mapping(function(fallback)
      if cmp.visible() then
        cmp.select_prev_item()
      elseif luasnip.jumpable(-1) then
        luasnip.jump(-1)
      else
        fallback()
      end
    end, { 'i', 's' }),
  },
  sources = {
    { name = 'nvim_lsp' },
    { name = 'luasnip' },
  },
}

-- The line beneath this is called `modeline`. See `:help modeline`
-- vim: ts=2 sts=2 sw=2 et
