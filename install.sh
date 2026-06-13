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

archive="myplace_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/latest/download"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "downloading $base/$archive ..."
curl -fsSL "$base/$archive" -o "$tmp/$archive"

# Verify the archive against checksums.txt from the same release before
# extracting — TLS trust alone shouldn't gate a binary we're about to run.
echo "verifying checksum ..."
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"
expected=$(awk -v f="$archive" '$2 == f {print $1}' "$tmp/checksums.txt")
[ -n "$expected" ] || { echo "no checksum for $archive in checksums.txt" >&2; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$archive" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')
else
  echo "no sha256 tool found (need sha256sum or shasum)" >&2; exit 1
fi
[ "$expected" = "$actual" ] || { echo "checksum mismatch: expected $expected, got $actual" >&2; exit 1; }

tar -xzf "$tmp/$archive" -C "$tmp" myplace

mkdir -p "$BIN_DIR"
install -m 0755 "$tmp/myplace" "$BIN_DIR/myplace"
echo "installed $("$BIN_DIR/myplace" version) to $BIN_DIR/myplace"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "note: $BIN_DIR is not on your PATH — add it to your shell config" ;;
esac
