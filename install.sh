#!/usr/bin/env bash
set -euo pipefail

# adze install script
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/gregberns/adze/main/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/gregberns/adze/main/install.sh | bash -s v1.2.3
#   curl -fsSL ... | INSTALL_DIR=~/.local/bin bash

REPO="gregberns/adze"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# --- Dependency check ---

if ! command -v curl &> /dev/null; then
    echo "Error: curl is required but not found" >&2
    exit 1
fi

# --- Detect OS ---

detect_os() {
    local raw
    raw="$(uname -s)"
    case "$raw" in
        Darwin) echo "darwin" ;;
        Linux)  echo "linux" ;;
        *)
            echo "Error: unsupported operating system: $raw" >&2
            exit 1
            ;;
    esac
}

# --- Detect architecture ---

detect_arch() {
    local raw
    raw="$(uname -m)"
    case "$raw" in
        x86_64)          echo "amd64" ;;
        aarch64|arm64)   echo "arm64" ;;
        *)
            echo "Error: unsupported architecture: $raw" >&2
            exit 1
            ;;
    esac
}

OS="$(detect_os)"
ARCH="$(detect_arch)"

# --- Resolve version ---

VERSION="${1:-latest}"

if [ "$VERSION" = "latest" ]; then
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' \
        | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
    if [ -z "$VERSION" ]; then
        echo "Error: failed to resolve latest version from GitHub API" >&2
        exit 1
    fi
fi

# --- Download binary ---

BINARY_NAME="adze-${OS}-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}"

echo "Installing adze ${VERSION} (${OS}/${ARCH})..."

TEMP_FILE="$(mktemp)"
trap 'rm -f "$TEMP_FILE"' EXIT

HTTP_CODE="$(curl -fsSL -w '%{http_code}' -o "$TEMP_FILE" "$DOWNLOAD_URL" 2>&1)" || {
    echo "Error: failed to download binary from ${DOWNLOAD_URL}" >&2
    echo "Check that version '${VERSION}' exists and the binary '${BINARY_NAME}' is available." >&2
    exit 1
}

# --- Install ---

DEST="${INSTALL_DIR}/adze"

if ! install -m 755 "$TEMP_FILE" "$DEST" 2>/dev/null; then
    echo "Error: permission denied writing to ${INSTALL_DIR}" >&2
    echo "Try: INSTALL_DIR=~/.local/bin bash install.sh ${VERSION}" >&2
    exit 1
fi

echo "Successfully installed adze to ${DEST}"
echo "Run 'adze version' to verify the installation."

# --- PATH check ---

case ":${PATH}:" in
    *":${INSTALL_DIR}:"*)
        ;;
    *)
        echo ""
        echo "Warning: ${INSTALL_DIR} is not in your PATH"
        echo "Add it with: export PATH=\"${INSTALL_DIR}:\$PATH\""
        ;;
esac
