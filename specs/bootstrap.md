# Bootstrap Specification

## Overview

The bootstrap process installs the tool binary on a fresh machine and provides the entry point for first-time configuration. The tool is distributed as pre-built binaries via GitHub Releases, with a source-build fallback when no binary is available.

## Binary Distribution

### Naming Convention

Binaries are named: `adze-<os>-<arch>`

### Supported Targets

| Binary Name | OS | Architecture |
|------------|-----|--------------|
| `adze-darwin-arm64` | macOS | Apple Silicon |
| `adze-darwin-amd64` | macOS | Intel |
| `adze-linux-amd64` | Linux | x86_64 |
| `adze-linux-arm64` | Linux | ARM64 |

### Release Assets

Each GitHub release MUST include:
- One binary per supported target
- `checksums.txt` containing SHA256 checksums (one line per binary: `<hash>  <filename>`)
- `install.sh` (the install script)

Binaries are raw executables (not compressed in tar.gz or zip).

## Install Script

### TTY Requirement

The install script MUST check `[[ -t 0 ]]` at the top of execution. If stdin is not a TTY, the script MUST exit with code 1 and print:

```
Error: This script must be run interactively (not piped).
Download it first, then run:

  curl -fsSL https://raw.githubusercontent.com/gregberns/adze/main/install.sh -o install.sh
  bash install.sh
```

The pipe-to-bash pattern (`curl ... | bash`) is not supported because it prevents interactive prompts required by `sudo`, Homebrew, and `xcode-select --install`.

### Install Procedure

The install script (`install.sh`) MUST:

1. Verify stdin is a TTY (see above).
2. Detect the OS via `uname -s` (lowercased).
3. Detect the architecture via `uname -m` (mapped: `x86_64` → `amd64`, `aarch64`/`arm64` → `arm64`).
4. Accept an optional version argument (`$1`); default to `latest`.
5. Resolve `latest` to the actual version tag via the GitHub API.
6. Download the appropriate binary from the GitHub release.
7. Install to `$INSTALL_DIR` (default: `$HOME/.local/bin`).
8. Create `$INSTALL_DIR` if it does not exist: `mkdir -p "$INSTALL_DIR"`.
9. Set the executable bit.
10. If the binary download fails (404, no releases, unsupported target), fall back to the source-build procedure (see below).
11. Configure the shell profile (see below).
12. Print a success message with the installed path and a `--version` command suggestion.

If the architecture is unsupported, the script MUST exit with code 1 and an error message.

The `INSTALL_DIR` environment variable MAY be used to override the default install directory.

### PATH Propagation

After installing any tool that places binaries in a non-default location, the script MUST make that tool available in the current shell session before proceeding. No tool install MAY be followed by a command that uses that tool without an intervening PATH update. Specifically:

- After installing Homebrew on Apple Silicon: `eval "$(/opt/homebrew/bin/brew shellenv)"`.
- After `brew install go` (or any brew package): re-evaluate `eval "$(brew shellenv)"` so newly installed binaries are on PATH.
- After installing adze to `$INSTALL_DIR`: `export PATH="$INSTALL_DIR:$PATH"`.

### Usage

```bash
# Download and run (interactive)
curl -fsSL https://raw.githubusercontent.com/gregberns/adze/main/install.sh -o install.sh
bash install.sh

# Install specific version
bash install.sh v1.2.3
```

## Source Build Fallback

When the binary download fails (404, no releases exist, unsupported platform), `install.sh` MUST fall back to building from source.

The script MUST print: `"No pre-built binary found for <os>/<arch>; building from source..."`.

### Source Build Procedure

1. **Ensure git is available.**
   - On macOS: if `git` is not found, install Xcode Command Line Tools (which provides git).
   - On Linux: `git` MUST already be available; if not, the script MUST exit with instructions to install it.

2. **Ensure Go is available.** If `go` is not found:
   - On macOS: install via `brew install go` (installing Homebrew first if needed — see Homebrew bootstrap below).
   - On Linux (Debian/Ubuntu): install via `apt-get install -y golang-go`.

3. **Clone the repository** to a temporary directory.

4. **Build the binary:** `go build -o "$INSTALL_DIR/adze" ./cmd/adze/`.

5. **Clean up** the cloned repository.

### Homebrew Bootstrap (macOS only)

If Homebrew is required but not installed, the script MUST install it using Homebrew's official install script. After installation, the script MUST immediately propagate PATH (see PATH Propagation above).

## Config-from-URL

The `apply` command MUST accept a URL as the `--config` value:

```bash
adze apply --config https://raw.githubusercontent.com/.../macos-dev.yaml
```

Behavior:
1. Detect that the path is a URL (starts with `http://` or `https://`).
2. Download the file to a temporary location.
3. Validate the config (same validation as local files).
4. If valid: proceed with apply.
5. After the run completes (success or failure): delete the temporary file.

### URL Config Limitations

- URL-sourced configs MUST NOT contain `include` directives (includes require local files). If an include is detected, the tool MUST report an error.
- The URL MUST return YAML content. The tool MUST validate the Content-Type or file content before proceeding.

## Failure Modes

| Failure | Cause | Behavior |
|---------|-------|----------|
| TTY check failed | Script piped via `curl ... \| bash` | Script exits with code 1 and prints download-then-run instructions |
| Download tool missing | `curl` not in PATH | Script exits with error: "curl is required but not found" |
| Network unreachable | No internet, DNS failure | curl fails with its standard error |
| Release not found | Wrong version, 404 | curl fails; script reports the attempted URL and falls back to source build |
| No release binary | No GitHub releases, unsupported target | Fall back to source build; if that also fails, exit with instructions |
| Source build failed | Go build error, clone failure | Script exits with error and prints manual build instructions |
| Config URL unreachable | 404, network error | `apply` reports fetch error with the URL |
| Config invalid | YAML parse or validation error | `apply` reports validation errors (binary is already installed) |

## Render as Fallback

For environments where the binary cannot be installed (air-gapped machines, restricted systems), the `render` command generates a standalone bash script:

1. On a working machine: `adze render --config macos-dev.yaml > setup.sh`
2. Transfer the script to the target machine.
3. Run: `bash setup.sh`

The rendered script operates independently of the tool binary. Bootstrap documentation MUST mention this fallback path.

## PATH Verification

After installation, the install script SHOULD check if `$INSTALL_DIR` is in the user's `$PATH`. If `$INSTALL_DIR` is a non-standard custom path specified by the user via the `INSTALL_DIR` environment variable and is not in PATH, the script MUST print a warning:

```
Warning: <install-dir> is not in your PATH
Add it with: export PATH="<install-dir>:$PATH"
```

For the default `$HOME/.local/bin` directory, shell profile configuration (below) handles PATH persistence and no warning is needed.

## Shell Profile Configuration

After installation, the install script MUST ensure `$INSTALL_DIR` is on PATH in future shell sessions.

### Profile File Selection

The script MUST determine the target profile file:

- macOS (darwin): `~/.zprofile`
- Linux with zsh as `$SHELL`: `~/.zprofile`
- Linux with bash as `$SHELL`: `~/.bashrc`

### PATH Entry

If the profile file does not already contain a PATH entry for `$INSTALL_DIR`, the script MUST append:

```bash
# Added by adze installer
export PATH="$HOME/.local/bin:$PATH"
```

If the profile already contains the entry, the script MUST NOT modify it (idempotent).

### Homebrew Shell Environment (macOS, Apple Silicon only)

On macOS with Homebrew installed at `/opt/homebrew`, the script MUST also ensure the profile contains:

```bash
eval "$(/opt/homebrew/bin/brew shellenv)"
```

This MUST only be added if not already present.

### Output

The script MUST print what was added: `"Added <install-dir> to PATH in <profile-file>"`.

If the profile already had all required entries, the script MUST print nothing (idempotent, silent).
