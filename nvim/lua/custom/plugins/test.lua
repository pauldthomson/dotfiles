return {
    "klen/nvim-test",
    main = "nvim-test",
    cmd = { 'TestNearest', 'TestFile', 'TestSuite', 'TestLast', 'TestVisit' },
    opts = {
        termOpts = {
            direction = "vertical"
        },
    },
}
