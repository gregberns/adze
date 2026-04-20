#!/usr/bin/env bash
set -euo pipefail

# adze install script
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/gregberns/adze/main/install.sh -o install.sh
#   bash install.sh
#   bash install.sh v1.2.3

# --- TTY check ---

if [[ ! -t 0 ]]; then
    echo "Error: This script must be run interactively (not piped)." >&2
    echo "Download it first, then run:" >&2
    echo "" >&2
    echo "  curl -fsSL https://raw.githubusercontent.com/gregberns/adze/main/install.sh -o install.sh" >&2
    echo "  bash install.sh" >&2
    exit 1
fi

REPO="gregberns/adze"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

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
        | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')" || true
    if [ -z "$VERSION" ]; then
        echo "Warning: failed to resolve latest version from GitHub API; falling back to source build" >&2
    fi
fi

# --- Source build fallback ---

source_build() {
    echo "No pre-built binary found for ${OS}/${ARCH}; building from source..."

    # Ensure git is available
    if ! command -v git &> /dev/null; then
        if [ "$OS" = "darwin" ]; then
            echo "Installing Xcode Command Line Tools (provides git)..."
            xcode-select --install
            echo "Please re-run this script after Xcode Command Line Tools finish installing." >&2
            exit 1
        else
            echo "Error: git is required but not found." >&2
            echo "Install it with: sudo apt-get install -y git" >&2
            exit 1
        fi
    fi

    # Ensure Go is available
    if ! command -v go &> /dev/null; then
        if [ "$OS" = "darwin" ]; then
            # Install Homebrew if needed
            if ! command -v brew &> /dev/null; then
                echo "Installing Homebrew..."
                /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
                # PATH propagation for Homebrew on Apple Silicon
                if [ -x /opt/homebrew/bin/brew ]; then
                    eval "$(/opt/homebrew/bin/brew shellenv)"
                fi
            fi
            echo "Installing Go via Homebrew..."
            brew install go
            eval "$(brew shellenv)"
        else
            echo "Installing Go via apt-get..."
            sudo apt-get update -y
            sudo apt-get install -y golang-go
        fi
    fi

    # Clone, build, clean up
    local tmpdir
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    echo "Cloning adze repository..."
    git clone "https://github.com/${REPO}.git" "$tmpdir/adze"

    echo "Building adze..."
    mkdir -p "$INSTALL_DIR"
    (cd "$tmpdir/adze" && go build -o "$INSTALL_DIR/adze" ./cmd/adze/) || {
        echo "Error: source build failed." >&2
        echo "Manual build instructions:" >&2
        echo "  git clone https://github.com/${REPO}.git" >&2
        echo "  cd adze && go build -o adze ./cmd/adze/" >&2
        exit 1
    }

    rm -rf "$tmpdir"
    # Remove the trap since we cleaned up manually
    trap - EXIT
}

# --- Download binary ---

install_binary() {
    BINARY_NAME="adze-${OS}-${ARCH}"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}"

    echo "Installing adze ${VERSION} (${OS}/${ARCH})..."

    TEMP_FILE="$(mktemp)"
    trap 'rm -f "$TEMP_FILE"' EXIT

    if ! curl -fsSL -o "$TEMP_FILE" "$DOWNLOAD_URL" 2>/dev/null; then
        echo "Warning: failed to download binary from ${DOWNLOAD_URL}" >&2
        rm -f "$TEMP_FILE"
        trap - EXIT
        return 1
    fi

    mkdir -p "$INSTALL_DIR"

    DEST="${INSTALL_DIR}/adze"
    install -m 755 "$TEMP_FILE" "$DEST"
    rm -f "$TEMP_FILE"
    trap - EXIT
    return 0
}

# --- Shell profile configuration ---

configure_shell_profile() {
    local profile_file=""

    # Determine profile file
    if [ "$OS" = "darwin" ]; then
        profile_file="$HOME/.zprofile"
    elif [[ "$SHELL" == */zsh ]]; then
        profile_file="$HOME/.zprofile"
    else
        profile_file="$HOME/.bashrc"
    fi

    local modified=false

    # Add PATH entry for INSTALL_DIR if not present
    local path_line='export PATH="$HOME/.local/bin:$PATH"'
    if [ -f "$profile_file" ] && grep -qF '.local/bin' "$profile_file"; then
        : # Already present
    else
        echo "" >> "$profile_file"
        echo "# Added by adze installer" >> "$profile_file"
        echo "$path_line" >> "$profile_file"
        modified=true
    fi

    # Homebrew shell environment on macOS Apple Silicon
    if [ "$OS" = "darwin" ] && [ -x /opt/homebrew/bin/brew ]; then
        local brew_line='eval "$(/opt/homebrew/bin/brew shellenv)"'
        if [ -f "$profile_file" ] && grep -qF '/opt/homebrew/bin/brew shellenv' "$profile_file"; then
            : # Already present
        else
            echo "" >> "$profile_file"
            echo "$brew_line" >> "$profile_file"
            modified=true
        fi
    fi

    if [ "$modified" = true ]; then
        echo "Added ${INSTALL_DIR} to PATH in ${profile_file}"
    fi
}

# --- Main ---

# Try binary download; fall back to source build
if [ -n "$VERSION" ]; then
    if ! install_binary; then
        source_build
    fi
else
    # No version resolved (GitHub API failed), go straight to source build
    source_build
fi

# PATH propagation for current session
export PATH="$INSTALL_DIR:$PATH"

# Configure shell profile for future sessions
configure_shell_profile

# --- PATH check for custom INSTALL_DIR ---

if [ "$INSTALL_DIR" != "$HOME/.local/bin" ]; then
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*)
            ;;
        *)
            echo ""
            echo "Warning: ${INSTALL_DIR} is not in your PATH"
            echo "Add it with: export PATH=\"${INSTALL_DIR}:\$PATH\""
            ;;
    esac
fi

echo "Successfully installed adze to ${INSTALL_DIR}/adze"
echo "Run 'adze --version' to verify the installation."
