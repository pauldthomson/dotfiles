return {
    'folke/snacks.nvim',
    priority = 100,
    lazy = false,
    opts = {
        indent = {
            indent = {
                enabled = true,
                char = "┆",
            },
            scope = {
                enabled = true,
                only_current = true,
            },
        },
        scroll = { enabled = true }
    },
}
