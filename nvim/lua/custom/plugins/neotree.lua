return {
    -- File explorer
    "nvim-neo-tree/neo-tree.nvim",
    branch = "v3.x",
    cmd = "Neotree",
    dependencies = {
        "nvim-lua/plenary.nvim",
        "nvim-tree/nvim-web-devicons", -- not strictly required, but recommended
        "MunifTanjim/nui.nvim",
    },
    opts = {
        filesystem = {
            follow_current_file = {
                enabled = true
            },
            filtered_items = {
                hide_dotfiles = false
            }
        },
        window = {
            position = "float",
        }
    }
}
