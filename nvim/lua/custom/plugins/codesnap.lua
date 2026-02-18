return {
    "mistricky/codesnap.nvim",
    version = "v2.0.0",
    -- v2 downloads a platform-specific generator automatically; running `make`
    -- creates an incompatible lua/generator.so that breaks theme parsing.
    cmd = {
        "CodeSnap",
        "CodeSnapSave",
        "CodeSnapASCII",
        "CodeSnapHighlight",
        "CodeSnapHighlightSave",
    },
}
