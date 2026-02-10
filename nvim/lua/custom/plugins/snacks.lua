return {
    'folke/snacks.nvim',
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
