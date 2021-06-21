vim.cmd 'packadd paq-nvim'
require 'paq-nvim' {
    {'savq/paq-nvim', opt=true};
    {'nvim-lua/completion-nvim'};
    {'dracula/vim', opt=true, as='dracula'};
    {'junegunn/fzf', run=vim.fn['fzf#install'] };
    {'junegunn/fzf.vim'};
    {'nvim-treesitter/nvim-treesitter', run=vim.fn['TSUpdate'] };
    {'neovim/nvim-lspconfig'};
    {'ms-jpq/chadtree', branch='chad', run='python3 -m chadtree deps'};
    {'vim-airline/vim-airline'};
    {'9mm/vim-closer'};
    {'tpope/vim-commentary'};
    {'ryanoasis/vim-devicons'};
    {'tpope/vim-fugitive'};
    {'airblade/vim-gitgutter'};
    {'christoomey/vim-tmux-navigator'};
    {'tweekmonster/startuptime.vim'};
}

-- Enable built-in modules
require'nvim-treesitter.configs'.setup {
    highlight = {
        enable = true,
    },
    indent = {
        enable = true,
    },
}

-- Map leader to <Space>
vim.g.mapleader = " "

-- Vim built-in config
vim.wo.cursorline = true
vim.wo.number = true
vim.o.encoding = 'utf-8'
vim.o.splitbelow = true
vim.o.splitright = true
vim.o.tabstop = 4
vim.o.expandtab = true
vim.o.shiftwidth = 4
vim.o.smarttab = true
vim.o.showmatch = true
vim.o.hlsearch = true
vim.o.incsearch = true
vim.o.ignorecase = true
vim.o.smartcase = true
vim.o.visualbell = true
vim.o.scrolloff = 3
vim.o.laststatus = 2
vim.o.cot = 'menuone,noinsert,noselect'

vim.cmd('packadd! dracula')
vim.cmd('colorscheme dracula')

-- Use jk to exit insert mode
vim.api.nvim_set_keymap('i', 'jk', '<ESC>', { noremap = true })

-- Use ctrl+hjkl to navigate splits
vim.api.nvim_set_keymap('n', '<C-J>', '<C-W><C-J>', { noremap = true })
vim.api.nvim_set_keymap('n', '<C-K>', '<C-W><C-K>', { noremap = true })
vim.api.nvim_set_keymap('n', '<C-L>', '<C-W><C-L>', { noremap = true })
vim.api.nvim_set_keymap('n', '<C-H>', '<C-W><C-H>', { noremap = true })

-- Save buffer
vim.api.nvim_set_keymap('n', '<leader>w', ':w<CR>', { noremap = true })

-- Copy and past to system clipboard in visual mode
vim.api.nvim_set_keymap('v', '<leader>y', '\"+y', { noremap = false })
vim.api.nvim_set_keymap('n', '<leader>p', '\"+p', { noremap = false })

-- Toggle CHADTree
vim.api.nvim_set_keymap('', '<C-n>', ':CHADopen<CR>', { noremap = false })

-- Italic comments
vim.cmd('highlight Comment cterm=italic')
-- vim.o.t_ZH = '[3m'
-- vim.o.t_ZR = '[23m'
vim.o.termguicolors = true

-- fzf
vim.api.nvim_set_keymap('n', '<leader>f', ':Files<CR>', { noremap = true })
vim.api.nvim_set_keymap('n', '<leader>e', ':Buffers<CR>', { noremap = true })
vim.api.nvim_set_keymap('n', '<leader>r', ':Rg<space>', { noremap = true })

if vim.fn.executable('rg') == 1 then
    vim.env.FZF_DEFAULT_COMMAND = 'rg --files --hidden --follow --glob "!.git/*"'
end

vim.opt.rtp:append('/usr/local/opt/fzf')

-- vim-gitgutter
vim.opt.updatetime = 100
vim.api.nvim_command('highlight GitGutterAdd    guifg=#009900 ctermfg=2')
vim.api.nvim_command('highlight GitGutterChange guifg=#bbbb00 ctermfg=3')
vim.api.nvim_command('highlight GitGutterDelete guifg=#ff2222 ctermfg=1')

-- vim-fugitive
vim.opt.diffopt:append('vertical')
vim.api.nvim_set_keymap('n', '<leader>gs', ':G<CR>', { noremap = true })
vim.api.nvim_set_keymap('n', '<leader>gt', ':diffget //2', { noremap = false })
vim.api.nvim_set_keymap('n', '<leader>gm', ':diffget //3', { noremap = false })

local nvim_lsp = require('lspconfig')
-- Use an on_attach function to only map the following keys 
-- after the language server attaches to the current buffer
local on_attach = function(client, bufnr)
  local function buf_set_keymap(...) vim.api.nvim_buf_set_keymap(bufnr, ...) end
  local function buf_set_option(...) vim.api.nvim_buf_set_option(bufnr, ...) end

  --Enable completion triggered by <c-x><c-o>
  buf_set_option('omnifunc', 'v:lua.vim.lsp.omnifunc')

  -- Mappings.
  local opts = { noremap=true, silent=true }

  -- See `:help vim.lsp.*` for documentation on any of the below functions
  buf_set_keymap('n', 'gD', '<Cmd>lua vim.lsp.buf.declaration()<CR>', opts)
  buf_set_keymap('n', 'gd', '<Cmd>lua vim.lsp.buf.definition()<CR>', opts)
  buf_set_keymap('n', 'K', '<Cmd>lua vim.lsp.buf.hover()<CR>', opts)
  buf_set_keymap('n', 'gi', '<cmd>lua vim.lsp.buf.implementation()<CR>', opts)
  buf_set_keymap('n', '<C-k>', '<cmd>lua vim.lsp.buf.signature_help()<CR>', opts)
  buf_set_keymap('n', '<space>wa', '<cmd>lua vim.lsp.buf.add_workspace_folder()<CR>', opts)
  buf_set_keymap('n', '<space>wr', '<cmd>lua vim.lsp.buf.remove_workspace_folder()<CR>', opts)
  buf_set_keymap('n', '<space>wl', '<cmd>lua print(vim.inspect(vim.lsp.buf.list_workspace_folders()))<CR>', opts)
  buf_set_keymap('n', '<space>D', '<cmd>lua vim.lsp.buf.type_definition()<CR>', opts)
  buf_set_keymap('n', '<space>rn', '<cmd>lua vim.lsp.buf.rename()<CR>', opts)
  buf_set_keymap('n', '<space>ca', '<cmd>lua vim.lsp.buf.code_action()<CR>', opts)
  buf_set_keymap('n', 'gr', '<cmd>lua vim.lsp.buf.references()<CR>', opts)
  --buf_set_keymap('n', '<space>e', '<cmd>lua vim.lsp.diagnostic.show_line_diagnostics()<CR>', opts)
  buf_set_keymap('n', '[d', '<cmd>lua vim.lsp.diagnostic.goto_prev()<CR>', opts)
  buf_set_keymap('n', ']d', '<cmd>lua vim.lsp.diagnostic.goto_next()<CR>', opts)
  buf_set_keymap('n', '<space>q', '<cmd>lua vim.lsp.diagnostic.set_loclist()<CR>', opts)
  --buf_set_keymap("n", "<space>f", "<cmd>lua vim.lsp.buf.formatting()<CR>", opts)

  require'completion'.on_attach()
end

-- Use a loop to conveniently call 'setup' on multiple servers and
-- map buffer local keybindings when the language server attaches
local servers = { "gopls" }
for _, lsp in ipairs(servers) do
  nvim_lsp[lsp].setup { on_attach = on_attach }
end

nvim_lsp.jdtls.setup { cmd = {'jdt-ls'}, on_attach = on_attach }
nvim_lsp.kotlin_language_server.setup { cmd = {'kotlin-ls'}, on_attach = on_attach }

-- vim-airline
vim.g['airline#extensions#tabline#enabled'] = 1
