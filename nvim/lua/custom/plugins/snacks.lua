return {
    'folke/snacks.nvim',
    priority = 100,
    event = 'VeryLazy',
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
