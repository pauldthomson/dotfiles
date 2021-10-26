vim.cmd 'packadd paq-nvim'
require 'paq-nvim' {
    {'savq/paq-nvim', opt=true};
    -- {'nvim-lua/completion-nvim'};
    {'dracula/vim', opt=true, as='dracula'};
    {'junegunn/fzf', run=vim.fn['fzf#install'] };
    {'junegunn/fzf.vim'};
    {'nvim-lua/popup.nvim'};
    {'nvim-lua/plenary.nvim'};
    {'nvim-telescope/telescope.nvim'};
    {'nvim-treesitter/nvim-treesitter'};
    {'neovim/nvim-lspconfig'};
    {'ms-jpq/chadtree', branch='chad', run='python3 -m chadtree deps'};
    {'hoob3rt/lualine.nvim'};
    {'tpope/vim-commentary'};
    -- {'ryanoasis/vim-devicons'};
    {'kyazdani42/nvim-web-devicons'};
    {'tpope/vim-fugitive'};
    {'airblade/vim-gitgutter'};
    {'christoomey/vim-tmux-navigator'};
    {'tweekmonster/startuptime.vim'};
    {'iamcco/markdown-preview.nvim', run='cd app && yarn install'};
    {'windwp/nvim-autopairs'};
    {'rinx/lspsaga.nvim'};
    {'hashivim/vim-terraform'};
    {'ms-jpq/coq_nvim', branch = 'coq'};
    {'ms-jpq/coq.artifacts', branch= 'artifacts'};
    {'tsandall/vim-rego'};
}

-- lspsaga settings
require'lspsaga'.init_lsp_saga{
    error_sign = '',
    warn_sign = '',
    hint_sign = '',
    infor_sign = '',
    border_style = "round",
}

require'lualine'.setup{
    options = {
        theme = 'dracula'
    },
    extensions = { 
        'chadtree', 
        -- 'fzf', 
        'quickfix',
        'fugitive'
    },
    tabline = {
      lualine_a = {{'filename', path = 1}},
      lualine_b = {},
      lualine_c = {},
      lualine_x = {},
      lualine_y = {},
      lualine_z = {}
}
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

local npairs = require('nvim-autopairs')
npairs.setup({
    check_ts = true
})

-- nvim-autopairs
-- map <CR> to be in between inserted bracket etc
_G.MUtils= {}

-- vim.g.completion_confirm_key = ""
--
MUtils.CR = function()
  if vim.fn.pumvisible() ~= 0 then
    if vim.fn.complete_info({ 'selected' }).selected ~= -1 then
      return npairs.esc('<c-y>')
    else
      return npairs.esc('<c-e>') .. npairs.autopairs_cr()
    end
  else
    return npairs.autopairs_cr()
  end
end
vim.api.nvim_set_keymap('i', '<cr>', 'v:lua.MUtils.CR()', { expr = true, noremap = true })

MUtils.BS = function()
  if vim.fn.pumvisible() ~= 0 and vim.fn.complete_info({ 'mode' }).mode == 'eval' then
    return npairs.esc('<c-e>') .. npairs.autopairs_bs()
  else
    return npairs.autopairs_bs()
  end
end
vim.api.nvim_set_keymap('i', '<bs>', 'v:lua.MUtils.BS()', { expr = true, noremap = true })

-- MUtils.completion_confirm=function()
--   if vim.fn.pumvisible() ~= 0  then
--     if vim.fn.complete_info()["selected"] ~= -1 then
--       require'completion'.confirmCompletion()
--       return npairs.esc("<c-y>")
--     else
--       vim.api.nvim_select_popupmenu_item(0 , false , false ,{})
--       require'completion'.confirmCompletion()
--       return npairs.esc("<c-n><c-y>")
--     end
--   else
--     return npairs.autopairs_cr()
--   end
-- end

-- vim.api.nvim_set_keymap('i' , '<CR>','v:lua.MUtils.completion_confirm()', {expr = true , noremap = true})

-- TODO: dies scheint funkioniert nicht
local parser_config = require "nvim-treesitter.parsers".get_parser_configs()
parser_config.hcl = {
    install_info = {
    url = "~/repos/github.com/mitchellh/tree-sitter-hcl",
    files = {"src/parser.c"}
  },
  used_by = {"terraform", "tf"}
}

vim.g.coq_settings = { 
    auto_start = 'shut-up', 
    keymap = { 
        jump_to_mark = '' ,
        manual_complete = '<c-x><c-o>'
    }
}

-- Map leader to <Space>
vim.g.mapleader = " "

-- Vim built-in config
vim.wo.cursorline = true
vim.wo.number = true
vim.wo.rnu = true
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

-- Close quickfix window
vim.api.nvim_set_keymap('n', '<leader>cc', ':ccl<CR>', { noremap = true })

-- Copy and past to system clipboard in visual mode
vim.api.nvim_set_keymap('v', '<leader>y', '\"+y', { noremap = false })
vim.api.nvim_set_keymap('n', '<leader>p', '\"+p', { noremap = false })

-- Toggle CHADTree
vim.api.nvim_set_keymap('', '<C-n>', ':CHADopen<CR>', { noremap = false })

-- Italic comments
-- vim.cmd('highlight Comment cterm=italic')
-- vim.o.t_ZH = '[3m'
-- vim.o.t_ZR = '[23m'
vim.o.termguicolors = true

-- fzf
-- vim.api.nvim_set_keymap('n', '<leader>f', ':Files<CR>', { noremap = true })
-- vim.api.nvim_set_keymap('n', '<leader>e', ':Buffers<CR>', { noremap = true })
-- vim.api.nvim_set_keymap('n', '<leader>r', ':Rg<space>', { noremap = true })

if vim.fn.executable('rg') == 1 then
    vim.env.FZF_DEFAULT_COMMAND = 'rg --files --hidden --follow --glob "!.git/*"'
end

-- vim.opt.rtp:append('/usr/local/opt/fzf')

-- telescope
vim.api.nvim_set_keymap('n', '<leader>f', '<cmd>lua require(\'telescope.builtin\').find_files({ find_command = {"rg", "--files", "--hidden", "--follow", "--glob", "!.git/*"}})<cr>', { noremap = true })
vim.api.nvim_set_keymap('n', '<leader>e', '<cmd>lua require(\'telescope.builtin\').buffers()<cr>', { noremap = true })
vim.api.nvim_set_keymap('n', '<leader>r', '<cmd>lua require(\'telescope.builtin\').live_grep()<cr>', { noremap = true })
vim.api.nvim_set_keymap('n', 'gi', '<cmd>lua require(\'telescope.builtin\').lsp_implementations()<cr>', { noremap = true })

require('telescope').setup{
    defaults = {
        color_devicons = true,
    }
}

-- vim-gitgutter
vim.opt.updatetime = 100
vim.api.nvim_command('highlight GitGutterAdd    guifg=#009900 ctermfg=2')
vim.api.nvim_command('highlight GitGutterChange guifg=#bbbb00 ctermfg=3')
vim.api.nvim_command('highlight GitGutterDelete guifg=#ff2222 ctermfg=1')

-- vim-fugitive
vim.opt.diffopt:append('vertical')
vim.api.nvim_set_keymap('n', '<leader>gs', ':G<CR>', { noremap = true })
vim.api.nvim_set_keymap('n', '<leader>gt', ':diffget //2<CR>', { noremap = false })
vim.api.nvim_set_keymap('n', '<leader>gm', ':diffget //3<CR>', { noremap = false })

-- completion-nvim
local function t(str)
    return vim.api.nvim_replace_termcodes(str, true, true, true)
end

function _G.smart_tab()
    return vim.fn.pumvisible() == 1 and t'<C-n>' or t'<Tab>'
end

function _G.s_smart_tab()
    return vim.fn.pumvisible() == 1 and t'<C-p>' or t'<S-Tab>'
end

vim.api.nvim_set_keymap('i', '<Tab>', 'v:lua.smart_tab()', { expr = true, noremap = true})
vim.api.nvim_set_keymap('i', '<S-Tab>', 'v:lua.s_smart_tab()', { expr = true, noremap = true})

local nvim_lsp = require('lspconfig')
local protocol = require'vim.lsp.protocol'

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
  -- buf_set_keymap('n', 'K', '<Cmd>lua vim.lsp.buf.hover()<CR>', opts)
  buf_set_keymap('n', 'K', '<Cmd>Lspsaga hover_doc<CR>', opts)
  -- buf_set_keymap('n', 'gi', '<cmd>lua vim.lsp.buf.implementation()<CR>', opts)
  -- buf_set_keymap('n', '<C-k>', '<cmd>lua vim.lsp.buf.signature_help()<CR>', opts)
  buf_set_keymap('i', '<C-k>', '<cmd>Lspsaga signature_help<CR>', opts)
  buf_set_keymap('n', '<C-G>', '<cmd>Lspsaga lsp_finder<CR>', opts)
  -- buf_set_keymap('n', '<space>wa', '<cmd>lua vim.lsp.buf.add_workspace_folder()<CR>', opts)
  -- buf_set_keymap('n', '<space>wr', '<cmd>lua vim.lsp.buf.remove_workspace_folder()<CR>', opts)
  -- buf_set_keymap('n', '<space>wl', '<cmd>lua print(vim.inspect(vim.lsp.buf.list_workspace_folders()))<CR>', opts)
  buf_set_keymap('n', '<space>D', '<cmd>lua vim.lsp.buf.type_definition()<CR>', opts)
  buf_set_keymap('n', '<space>rn', '<cmd>lua vim.lsp.buf.rename()<CR>', opts)
  -- buf_set_keymap('n', '<space>ca', '<cmd>lua vim.lsp.buf.code_action()<CR>', opts)
  buf_set_keymap('n', '<space>ca', '<cmd>Lspsaga code_action<CR>', opts)
  buf_set_keymap('n', 'gr', '<cmd>lua vim.lsp.buf.references()<CR>', opts)
  buf_set_keymap('n', '<space>d', '<cmd>lua vim.lsp.diagnostic.show_line_diagnostics()<CR>', opts)
  buf_set_keymap('n', '[d', '<cmd>lua vim.lsp.diagnostic.goto_prev()<CR>', opts)
  buf_set_keymap('n', ']d', '<cmd>lua vim.lsp.diagnostic.goto_next()<CR>', opts)
  buf_set_keymap('n', '<space>q', '<cmd>lua vim.lsp.diagnostic.set_loclist()<CR>', opts)
  -- buf_set_keymap("n", "<space>fm", "<cmd>lua vim.lsp.buf.formatting()<CR>", opts)

  -- require'completion'.on_attach()

  --protocol.SymbolKind = { }
  protocol.CompletionItemKind = {
    '', -- Text
    '', -- Method
    '', -- Function
    '', -- Constructor
    '', -- Field
    '', -- Variable
    '', -- Class
    'ﰮ', -- Interface
    '', -- Module
    '', -- Property
    '', -- Unit
    '', -- Value
    '', -- Enum
    '', -- Keyword
    '﬌', -- Snippet
    '', -- Color
    '', -- File
    '', -- Reference
    '', -- Folder
    '', -- EnumMember
    '', -- Constant
    '', -- Struct
    '', -- Event
    'ﬦ', -- Operator
    '', -- TypeParameter
  }

  -- format on save
  if client.resolved_capabilities.document_formatting then
      vim.api.nvim_command [[augroup Format]]
      vim.api.nvim_command [[autocmd! * <buffer>]]
      vim.api.nvim_command [[autocmd BufWritePre <buffer> lua vim.lsp.buf.formatting_seq_sync()]]
      vim.api.nvim_command [[augroup END]]
  end

  require'coq'.lsp_ensure_capabilities()
end

-- Use a loop to conveniently call 'setup' on multiple servers and
-- map buffer local keybindings when the language server attaches
local servers = { "kotlin_language_server" }
for _, lsp in ipairs(servers) do
  nvim_lsp[lsp].setup { on_attach = on_attach }
end

nvim_lsp.jdtls.setup { cmd = {'jdt-ls'}, on_attach = on_attach }
nvim_lsp.yamlls.setup { settings = { 
    yaml = { 
        schemas = { 
            ['kubernetes'] = '/**/*.yaml'
        }
    } 
}, on_attach = on_attach }
nvim_lsp.terraformls.setup { filetypes = { 'terraform', 'tf' }, on_attach = on_attach }
nvim_lsp.diagnosticls.setup {
    on_attach = on_attach,
    filetypes = { 'go', 'java', 'yaml', 'kotlin', 'markdown'},
}
nvim_lsp.gopls.setup {
    settings = {
	    gopls = {
            buildFlags = {"-tags=test"}
        }
    },
    on_attach = on_attach
}
require'lspsaga'.init_lsp_saga()
-- require'lspsaga'.setup {
--     debug = false,
--     use_saga_diagnostic_sign = true,
--     -- diagnostic sign
--     error_sign = "",
--     warn_sign = "",
--     hint_sign = "",
--     infor_sign = "",
--     diagnostic_header_icon = "   ",
--     -- code action title icon
--     code_action_icon = " ",
--     code_action_prompt = {
--         enable = true,
--         sign = true,
--         sign_priority = 40,
--         virtual_text = true,
--     },
--     finder_definition_icon = "  ",
--     finder_reference_icon = "  ",
--     max_preview_lines = 10,
--     finder_action_keys = {
--         open = "o",
--         vsplit = "s",
--         split = "i",
--         quit = "q",
--         scroll_down = "<C-f>",
--         scroll_up = "<C-b>",
--     },
--     code_action_keys = {
--         quit = "q",
--         exec = "<CR>",
--     },
--     rename_action_keys = {
--         quit = "<C-c>",
--         exec = "<CR>",
--     },
--     definition_preview_icon = "  ",
--     border_style = "single",
--     rename_prompt_prefix = "➤",
--     server_filetype_map = {},
-- }

-- vim-airline
-- vim.g['airline#extensions#tabline#enabled'] = 1
