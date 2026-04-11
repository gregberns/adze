# Bootstrap Specification

## Overview

The bootstrap process installs the tool binary on a fresh machine and provides the entry point for first-time configuration. The tool is distributed as pre-built binaries via GitHub Releases.

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

The install script (`install.sh`) MUST:

1. Detect the OS via `uname -s` (lowercased).
2. Detect the architecture via `uname -m` (mapped: `x86_64` → `amd64`, `aarch64`/`arm64` → `arm64`).
3. Accept an optional version argument (`$1`); default to `latest`.
4. Resolve `latest` to the actual version tag via the GitHub API.
5. Download the appropriate binary from the GitHub release.
6. Install to `$INSTALL_DIR` (default: `/usr/local/bin`).
7. Set the executable bit.
8. Print a success message with the installed path and a `--version` command suggestion.

If the architecture is unsupported, the script MUST exit with code 1 and an error message.

### Usage

```bash
# Install latest
curl -fsSL https://raw.githubusercontent.com/<owner>/<repo>/main/install.sh | bash

# Install specific version
curl -fsSL https://raw.githubusercontent.com/<owner>/<repo>/main/install.sh | bash -s v1.2.3

# Install to custom directory
curl -fsSL ... | INSTALL_DIR=~/.local/bin bash
```

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
| Download tool missing | `curl` not in PATH | Script exits with error: "curl is required but not found" |
| Network unreachable | No internet, DNS failure | curl fails with its standard error |
| Release not found | Wrong version, 404 | curl fails; script reports the attempted URL |
| Permission denied | Cannot write to install dir | Script reports permission error; suggests `INSTALL_DIR=~/.local/bin` |
| Config URL unreachable | 404, network error | `apply` reports fetch error with the URL |
| Config invalid | YAML parse or validation error | `apply` reports validation errors (binary is already installed) |

## Render as Fallback

For environments where the binary cannot be installed (air-gapped machines, restricted systems), the `render` command generates a standalone bash script:

1. On a working machine: `adze render --config macos-dev.yaml > setup.sh`
2. Transfer the script to the target machine.
3. Run: `bash setup.sh`

The rendered script operates independently of the tool binary. Bootstrap documentation MUST mention this fallback path.

## PATH Verification

After installation, the install script SHOULD check if `$INSTALL_DIR` is in the user's `$PATH`. If not, it MUST print a warning:

```
Warning: <install-dir> is not in your PATH
Add it with: export PATH="<install-dir>:$PATH"
```
