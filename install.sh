#!/bin/sh
# Install script for Cartograph
# Usage: curl -sSL https://raw.githubusercontent.com/dan-strohschein/Cartograph/main/install.sh | sh
set -e

REPO="dan-strohschein/Cartograph"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

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

# Get latest release tag
TAG=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
if [ -z "$TAG" ]; then
  echo "Error: could not determine latest release"
  exit 1
fi

echo "Installing cartograph ${TAG} for ${OS}/${ARCH}..."

SUFFIX="${OS}-${ARCH}"
if [ "$OS" = "windows" ]; then
  ARCHIVE="cartograph-${SUFFIX}.zip"
else
  ARCHIVE="cartograph-${SUFFIX}.tar.gz"
fi

URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${URL}..."
curl -sSL -o "${TMPDIR}/${ARCHIVE}" "$URL"

cd "$TMPDIR"
if [ "$OS" = "windows" ]; then
  unzip -q "$ARCHIVE"
else
  tar xzf "$ARCHIVE"
fi

BIN="cartograph-${SUFFIX}"
if [ "$OS" = "windows" ]; then BIN="${BIN}.exe"; fi

chmod +x "$BIN"
if [ -w "$INSTALL_DIR" ]; then
  mv "$BIN" "${INSTALL_DIR}/cartograph"
else
  sudo mv "$BIN" "${INSTALL_DIR}/cartograph"
fi

echo "Installed cartograph to ${INSTALL_DIR}/cartograph"
