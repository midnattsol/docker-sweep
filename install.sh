#!/bin/bash
set -e

# docker-sweep installer
# Usage: curl -sSL https://raw.githubusercontent.com/midnattsol/docker-sweep/main/install.sh | bash

REPO="midnattsol/docker-sweep"
BINARY="docker-sweep"
PLUGIN_DIR="${HOME}/.docker/cli-plugins"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case $ARCH in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

case $OS in
    linux|darwin) ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

# Get latest version
echo "Fetching latest version..."
VERSION=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)

if [ -z "$VERSION" ]; then
    echo "Failed to get latest version"
    exit 1
fi

echo "Installing ${BINARY} ${VERSION} for ${OS}/${ARCH}..."

# Create plugin directory
mkdir -p "$PLUGIN_DIR"

# Download and extract
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}-${OS}-${ARCH}.tar.gz"
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

curl -sSL "$DOWNLOAD_URL" | tar -xz -C "$TMP_DIR"

# Install
cp "${TMP_DIR}/${BINARY}" "${PLUGIN_DIR}/${BINARY}"
chmod +x "${PLUGIN_DIR}/${BINARY}"

echo ""
echo "Installed ${BINARY} ${VERSION} to ${PLUGIN_DIR}/${BINARY}"
echo ""
echo "Verify installation:"
echo "  docker sweep --version"
echo ""
echo "Usage:"
echo "  docker sweep              # Interactive cleanup of all resources"
echo "  docker sweep containers   # Clean up containers only"
echo "  docker sweep images       # Clean up images only"
echo "  docker sweep --yes        # Non-interactive, delete suggested"
