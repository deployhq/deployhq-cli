#!/bin/sh
# DeployHQ CLI installer
# Usage: curl -fsSL https://raw.githubusercontent.com/deployhq/deployhq-cli/main/install.sh | sh
set -e

REPO="deployhq/deployhq-cli"
BINARY="deployhq"
INSTALL_DIR="/usr/local/bin"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
    linux|darwin) ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Get latest version
echo "Fetching latest version..."
VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v?([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
    echo "Error: could not determine latest version"
    exit 1
fi

echo "Installing deployhq-cli v$VERSION ($OS/$ARCH)..."

# Download
EXT="tar.gz"
if [ "$OS" = "windows" ]; then
    EXT="zip"
fi

URL="https://github.com/$REPO/releases/download/v$VERSION/${BINARY}_${VERSION}_${OS}_${ARCH}.${EXT}"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "Downloading $URL..."
curl -fsSL "$URL" -o "$TMP/archive.$EXT"

# Extract
cd "$TMP"
if [ "$EXT" = "zip" ]; then
    unzip -q "archive.$EXT"
else
    tar xzf "archive.$EXT"
fi

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "$BINARY" "$INSTALL_DIR/$BINARY"
else
    echo "Installing to $INSTALL_DIR (requires sudo)..."
    sudo mv "$BINARY" "$INSTALL_DIR/$BINARY"
fi

chmod +x "$INSTALL_DIR/$BINARY"

echo ""
echo "deployhq-cli v$VERSION installed to $INSTALL_DIR/$BINARY"
echo ""
echo "Get started:"
echo "  deployhq auth login"
echo "  deployhq projects list"
echo "  deployhq --help"
