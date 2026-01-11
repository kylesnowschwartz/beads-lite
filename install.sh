#!/bin/sh
set -e

# beads-lite installer
# Usage: curl -sSL https://raw.githubusercontent.com/kylesnowschwartz/beads-lite/main/install.sh | sh

REPO="kylesnowschwartz/beads-lite"
BINARY="bl"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
darwin) OS="darwin" ;;
linux) OS="linux" ;;
*) echo "Unsupported OS: $OS" && exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
x86_64 | amd64) ARCH="amd64" ;;
arm64 | aarch64) ARCH="arm64" ;;
*) echo "Unsupported architecture: $ARCH" && exit 1 ;;
esac

# Get latest release tag
VERSION=$(curl -sSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$VERSION" ]; then
  echo "Failed to get latest version"
  exit 1
fi

# Download and install
TARBALL="beads-lite_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$TARBALL"

echo "Downloading $BINARY $VERSION for $OS/$ARCH..."
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

curl -sSL "$URL" -o "$TMPDIR/$TARBALL"
tar -xzf "$TMPDIR/$TARBALL" -C "$TMPDIR"

echo "Installing to $INSTALL_DIR..."
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMPDIR/$BINARY" "$INSTALL_DIR/"
else
  sudo mv "$TMPDIR/$BINARY" "$INSTALL_DIR/"
fi

echo "Installed $BINARY $VERSION to $INSTALL_DIR/$BINARY"
