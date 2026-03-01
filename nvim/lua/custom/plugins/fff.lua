return {
    'dmtrKovalenko/fff.nvim',
    build = function()
        -- This plugin ships a Rust backend in a native module. If the prebuilt binary is missing or invalid,
        -- this will download a fresh one (or build it locally when rustup is available).
        require('fff.download').download_or_build_binary()
    end,
    -- Keep lazy to avoid loading the native backend during startup.
    lazy = true,
    -- Safety wrapper: validate the native module before invoking FFF.
    -- If the backend is corrupted or not loadable, disable the command instead of letting Neovim crash.
    keys = {
        {
            'ff',
            function()
                local ok, loader, load_err = pcall(function()
                    local binary_path = require('fff.download').get_binary_path()
                    return package.loadlib(binary_path, 'luaopen_fff_nvim')
                end)

                if not ok then
                    vim.notify('FFF backend is unavailable: ' .. tostring(loader), vim.log.levels.ERROR)
                    return
                end

                if loader == nil then
                    vim.notify('FFF backend is unavailable: ' .. tostring(load_err), vim.log.levels.ERROR)
                    return
                end

                require('fff').find_files()
            end,
            desc = 'FFFind files',
        },
    },
}
