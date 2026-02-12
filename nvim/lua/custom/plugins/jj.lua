return {
    {
        "avm99963/vim-jjdescription"
    },
    {
        "julienvincent/hunk.nvim",
        cmd = { "DiffEditor" },
        config = function()
            require("hunk").setup()
        end,
    }
}
