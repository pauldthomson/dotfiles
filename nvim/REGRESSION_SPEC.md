# Neovim Regression Spec

This document defines regression checks for the Neovim setup in this repo.

Use this after changing:
- `nvim/lua/custom/plugins/*.lua`
- lazy-loading handlers (`event`, `ft`, `cmd`, `keys`)
- LSP wiring (especially `terraformls`)

---

## Preconditions

1. Neovim 0.11+ is installed.
2. Plugins are installed (`:Lazy sync`).
3. `terraform-ls` is installed in Mason (`:MasonInstall terraform-ls`).

---

## A. Static spec checks (fast)

### A1. Plugin specs parse without startup errors

**Command**
```bash
nvim -n --headless '+qa'
```

**Expected**
- Exit code `0`
- No Lua stack traces

---

### A2. We prefer `opts` over `config = { ... }` in plugin specs

**Command**
```bash
rg -n "config\s*=\s*\{" nvim/lua/custom/plugins
```

**Expected**
- No plugin-level `config = { ... }` blocks (nested config tables inside plugin options are fine)

---

### A3. Trouble uses current devicons plugin

**Command**
```bash
rg -n "nvim-web-devicons" nvim/lua/custom/plugins/trouble.lua
```

**Expected**
- Uses `nvim-tree/nvim-web-devicons`
- Does **not** use `kyazdani42/nvim-web-devicons`

---

## B. Runtime lazy-loading checks

### B1. `vim-terraform` does not load for non-terraform buffers

**Command**
```bash
TMP=$(mktemp -t nvimtxt)
echo "hello" > "$TMP"
nvim -n --headless "$TMP" \
  '+lua local p=require("lazy.core.config").plugins["vim-terraform"]; print(p and p._.loaded and true or false)' \
  '+qa'
```

**Expected**
- Prints `false`

---

### B2. `vim-terraform` loads for terraform buffers

**Command**
```bash
TMP=$(mktemp -t nvimtf)
mv "$TMP" "$TMP.tf"
TMP="$TMP.tf"
echo 'terraform {}' > "$TMP"
nvim -n --headless "$TMP" \
  '+lua local p=require("lazy.core.config").plugins["vim-terraform"]; print(p and p._.loaded and true or false)' \
  '+qa'
```

**Expected**
- Prints `true`

---

### B3. FFF native backend initializes without process kill

**Command**
```bash
nvim -n --headless \
  '+lua require("fff.core").ensure_initialized()' \
  '+qa'
```

**Expected**
- Exit code `0`
- No native-backend load crash (no abrupt process kill)

---

## C. Terraform LSP behavior checks

### C1. `terraformls` attaches to `.tf` files

**Command**
```bash
ROOT=$(mktemp -d)
mkdir -p "$ROOT/module"
cat > "$ROOT/module/main.tf" <<'EOF'
terraform {
  required_version = ">= 1.5.0"
}
EOF

nvim -n --headless "$ROOT/module/main.tf" \
  '+lua vim.defer_fn(function() local c=vim.lsp.get_clients({name="terraformls"}); print("clients", #c); vim.cmd("qa") end, 3000)'
```

**Expected**
- Prints `clients 1` (or `>=1`)

---

### C2. `terraformls` root is nearest terraform module dir

**Command**
```bash
ROOT=$(mktemp -d)
mkdir -p "$ROOT/repo/infrastructure"
cat > "$ROOT/repo/infrastructure/main.tf" <<'EOF'
terraform {}
EOF

echo '{}' > "$ROOT/repo/package.json"  # simulate monorepo root

nvim -n --headless "$ROOT/repo/infrastructure/main.tf" \
  '+lua vim.defer_fn(function() local c=vim.lsp.get_clients({name="terraformls"}); if #c==0 then print("NO_CLIENT"); vim.cmd("qa"); return end; print("root", c[1].config.root_dir); vim.cmd("qa") end, 3000)'
```

**Expected**
- Root ends with `/infrastructure` (not `/repo`)

---

### C3. No immediate "session is not initialized ... state: down" for fresh terraform session

**Command**
1. Start a fresh terraform headless session (same as C1).
2. Check latest log entries:

```bash
rg -n "terraform-ls.*session is not initialized|current state: down" ~/.local/state/nvim/lsp.log | tail -n 5
```

**Expected**
- No new entries from the just-run session

Note: historical lines may exist; evaluate only newly appended lines.

---

## D. Manual smoke tests

### D1. Terraform editing UX

1. Open a real terraform file in a project module.
2. Run `:LspInfo`.
3. Use `K`, `gd`, and format (`:Format` or save).

**Expected**
- `terraformls` attached
- Hover/definition work
- No repeated LSP crash notifications

---

### D2. Lazy commands still work

1. `:Telescope find_files`
2. `:Trouble`
3. `:DBUI` (if DB plugins are installed)
4. `:FFFFind`

**Expected**
- Command triggers plugin load
- No missing-command or module errors

---

## Pass/Fail criteria

A change is considered safe when:
- All A/B/C checks pass.
- Manual smoke tests (D) show no regressions in normal usage.
