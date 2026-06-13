#!/bin/sh
# myplace installer — usage:
#   curl -fsSL https://raw.githubusercontent.com/mikevalstar/myplace/main/install.sh | sh
# Installs the latest release binary to ~/.local/bin (override: MYPLACE_BIN_DIR).
set -eu

REPO="mikevalstar/myplace"
BIN_DIR="${MYPLACE_BIN_DIR:-$HOME/.local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  darwin|linux) ;;
  *) echo "unsupported OS: $os" >&2; exit 1 ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac

url="https://github.com/$REPO/releases/latest/download/myplace_${os}_${arch}.tar.gz"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "downloading $url ..."
curl -fsSL "$url" -o "$tmp/myplace.tar.gz"
# TODO: verify against checksums.txt from the same release
tar -xzf "$tmp/myplace.tar.gz" -C "$tmp" myplace

mkdir -p "$BIN_DIR"
install -m 0755 "$tmp/myplace" "$BIN_DIR/myplace"
echo "installed $("$BIN_DIR/myplace" version) to $BIN_DIR/myplace"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "note: $BIN_DIR is not on your PATH — add it to your shell config" ;;
esac
