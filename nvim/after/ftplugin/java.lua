local util = require 'lspconfig.util'
local env = {
    HOME = vim.loop.os_homedir(),
    JDTLS_HOME = os.getenv 'JDTLS_HOME',
    WORKSPACE = os.getenv 'WORKSPACE',
}
local project_name = vim.fn.fnamemodify(vim.fn.getcwd(), ':p:h:t')
local function workspace_dir()
    return util.path.join(env.HOME, 'workspace', project_name)
end
local function get_jdtls_script()
    return vim.fn.expand '$JDTLS_HOME/bin/jdtls'
end
local function get_jdtls_config()
    return util.path.join(env.JDTLS_HOME, 'config_mac')
end

local config = {
    -- The command that starts the language server
    -- See: https://github.com/eclipse/eclipse.jdt.ls#running-from-the-command-line
    cmd = {
        -- 💀
        get_jdtls_script(),
             -- ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^                                       ^^^^^^^^^^^^^^
             -- Must point to the                                                     Change this to
             -- eclipse.jdt.ls installation                                           the actual version


        -- 💀
        '-configuration', get_jdtls_config(),
                        -- ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^        ^^^^^^
                        -- Must point to the                      Change to one of `linux`, `win` or `mac`
                        -- eclipse.jdt.ls installation            Depending on your system.


        -- 💀
        -- See `data directory configuration` section in the README
        '-data', workspace_dir()
    },

    -- 💀
    -- This is the default if not provided, you can remove it. Or adjust as needed.
    -- One dedicated LSP server & client will be started per unique root_dir
    root_dir = require('jdtls.setup').find_root({'.git', 'mvnw', 'gradlew'}),

    -- Here you can configure eclipse.jdt.ls specific settings
    -- See https://github.com/eclipse/eclipse.jdt.ls/wiki/Running-the-JAVA-LS-server-from-the-command-line#initialize-request
    -- for a list of options
    settings = {
        java = {
        }
    },

    -- Language server `initializationOptions`
    -- You need to extend the `bundles` with paths to jar files
    -- if you want to use additional eclipse.jdt.ls plugins.
    --
    -- See https://github.com/mfussenegger/nvim-jdtls#java-debug-installation
    --
    -- If you don't plan on using the debugger or other eclipse.jdt.ls plugins you can remove this
    init_options = {
        bundles = {}
    },

    on_attach = on_attach
}
-- This starts a new client & server,
-- or attaches to an existing client & server depending on the `root_dir`.
require('jdtls').start_or_attach(config)
