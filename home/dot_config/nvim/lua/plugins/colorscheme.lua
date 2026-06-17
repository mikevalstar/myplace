-- Catppuccin Mocha to match the rest of the managed setup
-- (ghostty/bat/starship/zed/delta/fzf/atuin/nano all use Catppuccin Mocha).
return {
  { "catppuccin/nvim", name = "catppuccin", opts = { flavour = "mocha" } },
  { "LazyVim/LazyVim", opts = { colorscheme = "catppuccin-mocha" } },
}
