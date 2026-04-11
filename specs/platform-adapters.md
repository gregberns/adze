# Platform Adapters Specification

**Status**: Draft  
**Date**: 2026-04-10

---

## Overview

Platform Adapters form the translation layer between abstract operations and
the concrete shell commands required on a given operating system. The system
MUST ship exactly three concrete adapters in v1:

- **Darwin adapter** — Homebrew formulas, Homebrew casks, macOS `defaults`, hostname
- **Ubuntu/Debian adapter** — apt, dpkg, systemctl, hostname
- **Generic adapter** — cross-platform operations: directories, git config, shell, file operations

At runtime, exactly one platform-specific adapter (Darwin or Ubuntu/Debian) MUST
be active. The generic adapter MUST always be active alongside the
platform-specific adapter. All three adapters MUST implement the `Adapter`
interface defined in §2.

---

## Adapter Interface

All adapters MUST implement the following Go interface:

```go
type Adapter interface {
    // Package operations
    PackageInstall(pkg Package) error
    PackageCheck(pkg Package) (bool, error)      // true = already satisfied
    PackageList() ([]InstalledPackage, error)    // leaf/manual installs only
    PackageUpgrade(pkg Package) error
    PackageRemove(pkg Package) error

    // Preference/defaults operations
    DefaultsRead(domain, key string) (DefaultsValue, error)
    DefaultsWrite(domain, key string, value DefaultsValue) error

    // System operations
    ServiceEnable(name string) error
    ServiceDisable(name string) error
    SetHostname(hostname string) error

    // Shell operations
    SetDefaultShell(shell string) error

    // Detection (for status, capture, init)
    ListLeaves() ([]string, error)                      // intentional top-level installs only
    ListAllInstalled() ([]InstalledPackage, error)      // full list including deps
}
```

### Supporting Types

```go
type Package struct {
    Name    string
    Version string  // optional; semantics differ by platform (see §5, §6)
    Pinned  bool
    Cask    bool    // Darwin only
}

type DefaultsValue struct {
    Type string // one of: "string", "boolean", "integer", "float"
    Raw  string // string representation of the value
}

type InstalledPackage struct {
    Name    string
    Version string
    Held    bool
}
```

---

## Platform Detection

Platform detection MUST run once at process startup, before any adapter is
instantiated.

### Detection Sequence

1. Read `runtime.GOOS`.
2. If `runtime.GOOS == "darwin"`: activate the Darwin adapter.
3. If `runtime.GOOS == "linux"`: read `/etc/os-release`.
   - If `ID=ubuntu`, `ID=debian`, or `ID_LIKE` contains the substring `debian`:
     activate the Ubuntu/Debian adapter.
   - All other Linux distributions: return `ErrUnsupportedPlatform` and abort.
4. All other values of `runtime.GOOS`: return `ErrUnsupportedPlatform` and abort.
5. Activate the generic adapter unconditionally alongside the platform-specific adapter.

### Config Platform Field Validation

If the config file declares a `platform:` field, the runtime MUST compare its
value to the detected platform. If they differ, the runtime MUST abort with an
error that names both the declared value and the detected value. The runtime
MUST NOT silently proceed when there is a platform mismatch.

### Adapter Absence Handling

**Darwin adapter — Homebrew absent**: If `brew` is not present in `PATH`, the
methods `PackageInstall`, `PackageCheck`, `PackageList`, `PackageUpgrade`, and
`PackageRemove` MUST return `ErrBrewNotFound`. The adapter MUST NOT silently
succeed on any Homebrew call when Homebrew is absent. Package steps that declare
`require: homebrew` MUST NOT execute until the `homebrew` built-in step has run
successfully.

**Ubuntu/Debian adapter — apt absent**: If `apt` is not present in `PATH` on a
Debian-family system, the adapter MUST return `ErrAptNotFound` and abort. The
absence of `apt` on a Debian-family system is an unrecoverable error in v1.

---

## Darwin Adapter

### Package Operations

#### Check (Formula)

To determine whether a formula is installed:

```sh
brew list --formula <name>
```

- Exit 0: formula is installed.
- Exit 1: formula is not installed.

For versioned formulas, use the versioned formula name verbatim:

```sh
brew list --formula node@20
```

#### Install (Formula)

```sh
brew install <name>         # unversioned formula
brew install node@20        # versioned formula (major version only; see §5)
```

- Exit 0: install succeeded.
- Any non-zero exit: install failed; propagate the error.

`brew install` MUST NOT be called with `sudo`. If the process has effective UID
0, the adapter MUST return `ErrBrewCalledAsRoot` without executing the command.

After a successful install, if `brew info --json <name>` reports `"keg_only":
true` in its output, the adapter MUST emit a warning stating that the formula is
keg-only and that `brew link --force <name>` or an explicit `PATH` addition is
required for the binary to be accessible. The adapter MUST determine keg-only
status by parsing the `keg_only` boolean field from `brew info --json <name>`.

#### Upgrade (Formula)

```sh
brew upgrade <name>
```

- Exit 0: formula upgraded or already current.
- Exit 1: formula not installed.

If `Package.Pinned` is `true`, `PackageUpgrade` MUST return `ErrPackagePinned`
without executing any command.

#### Remove (Formula)

```sh
brew uninstall <name>
```

- Exit 0: formula removed.
- Exit 1 when formula was not installed: treat as success (idempotent).

If the formula is pinned, the adapter MUST execute `brew unpin <name>` before
`brew uninstall <name>`.

### Version Pinning

Version pinning for formulas uses `brew pin` and `brew unpin`. Pinning MUST NOT
be applied to cask packages (see §Cask Operations).

#### Pin a Formula

Called after a successful `brew install` when `Package.Pinned` is `true`:

```sh
brew pin <name>
```

- Exit 0 always.

#### Unpin a Formula

Called before `brew uninstall` when the formula is pinned, and before an
explicit targeted upgrade of a pinned formula:

```sh
brew unpin <name>
```

- Exit 0 always.

#### Check Pin Status

```sh
brew list --pinned
```

- Output: one formula name per line; empty output indicates no pinned formulas.
- Exit 0 always, including when output is empty.

#### Pin Behavior — Normative Constraints

- `brew pin` MUST be understood as preventing all upgrade paths: `brew upgrade`,
  `brew upgrade <name>`, `brew upgrade --greedy`, and `brew upgrade --force` all
  skip pinned formulas. There is no flag that overrides a pin without first
  calling `brew unpin`.
- The `--greedy` flag affects only casks that declare `auto_updates true` or
  `version :latest`; it has no interaction with formula pins.
- To upgrade a pinned formula when the user explicitly targets it, the adapter
  MUST execute: `brew unpin <name>`, then `brew upgrade <name>`, then `brew pin <name>`.
  Bulk upgrade operations MUST skip pinned formulas without executing the above
  sequence.
- `brew pin` does not prevent `brew uninstall`. The adapter's `PackageRemove`
  MUST execute `brew unpin <name>` before `brew uninstall <name>` for any pinned
  formula.

### Cask Operations

#### Check (Cask)

```sh
brew list --cask <name>
```

- Exit 0: cask is installed.
- Exit 1: cask is not installed.

#### Install (Cask)

```sh
brew install --cask <name>
```

- Exit 0: cask installed successfully.
- Any non-zero exit: install failed; propagate the error.

The adapter MUST treat exit 0 as success. macOS permission dialogs or system
extension approval prompts that may appear during cask installation are surfaced
directly to the user and are outside the adapter's control.

#### Pinning (Cask)

`brew pin` does not support casks. If `Package.Pinned` is `true` and
`Package.Cask` is `true`, `PackageInstall` MUST return `ErrCaskPinNotSupported`
without executing any command.

#### Upgrade (Cask)

```sh
brew upgrade --cask <name>
```

Casks that declare `auto_updates true` or `version :latest` are skipped by
`brew upgrade --cask <name>` unless `--greedy` is appended. The adapter's
`PackageUpgrade` for casks MUST append `--greedy` only when the caller
explicitly opts in. The default behavior for `PackageUpgrade` on a cask MUST
NOT include `--greedy`.

#### Listing (Cask)

To list all installed casks:

```sh
brew list --cask
```

- Output: one bare cask name per line.
- Exit 0 always, including when output is empty.

The adapter MUST NOT call `brew leaves --cask`. Passing `--cask` to `brew
leaves` is not an error but produces no cask output; its behavior is undefined
relative to the intended operation.

### Defaults Operations

The Darwin adapter implements `DefaultsRead` and `DefaultsWrite` using the
macOS `defaults` command-line tool.

#### Supported Types

| `DefaultsValue.Type` | `defaults` write flag       | `defaults read-type` output | `defaults read` output |
|----------------------|-----------------------------|-----------------------------|------------------------|
| `string`             | `-string "value"`           | `Type is string`            | the string value       |
| `boolean`            | `-bool true` / `-bool false`| `Type is boolean`           | `1` or `0`             |
| `integer`            | `-int N`                    | `Type is integer`           | the integer value      |
| `float`              | `-float N`                  | `Type is float`             | the float value        |

No other types are supported in v1.

#### Write Command

```sh
defaults write <domain> <key> -<type> <value>
```

- Exit 0 always, including on type mismatch or missing domain.
- If the domain does not exist, `defaults write` creates
  `~/Library/Preferences/<domain>.plist`.

#### Post-Write Verification (Required)

Because `defaults write` silently accepts any type for any key and exits 0
regardless of type correctness, the adapter MUST execute a post-write
verification sequence after every `DefaultsWrite` call:

1. Execute the write: `defaults write <domain> <key> -<type> <value>`
2. Read back the type:
   ```sh
   defaults read-type <domain> <key> 2>/dev/null
   ```
   Parse the type name as the last word on the output line (e.g., `Type is
   boolean` → `boolean`).
3. Read back the value:
   ```sh
   defaults read <domain> <key> 2>/dev/null
   ```
4. Compare the declared type to the read-back type. If they differ, the adapter
   MUST return `ErrDefaultsTypeMismatch` with a message that names the domain,
   key, declared type, and actual type.
5. Compare the declared value to the read-back value, applying boolean
   normalisation (see below). If they differ, the adapter MUST return
   `ErrDefaultsValueMismatch`.

#### Boolean Normalisation

`defaults read` returns `1` for `true` and `0` for `false`. The adapter MUST
normalise these values when comparing a read-back boolean to a declared value:
a declared value of `"true"` MUST compare equal to a read-back value of `"1"`;
a declared value of `"false"` MUST compare equal to a read-back value of `"0"`.

#### Read Command (Check Phase)

To check whether a key exists and matches the desired value:

```sh
# Step 1: check existence and read value
value=$(defaults read "$domain" "$key" 2>/dev/null)

# Step 2: check type
type=$(defaults read-type "$domain" "$key" 2>/dev/null | awk '{print $NF}')
```

- `defaults read` exit 0: key exists.
- `defaults read` exit 1: key or domain does not exist.

The adapter MUST redirect stderr to `/dev/null` on all `defaults read` and
`defaults read-type` calls in the check phase. The messages `Domain <domain>
does not exist` and `The domain/default pair of (<domain>, <key>) does not
exist` MUST NOT appear in plan or status output.

#### Non-Existent Domain Behavior

- `defaults write` on a non-existent domain: creates the plist, exits 0.
- `defaults read` on a non-existent domain: writes `Domain <domain> does not
  exist` to stderr, exits 1.
- `defaults read` on a missing key in an existing domain: writes `The
  domain/default pair of (<domain>, <key>) does not exist` to stderr, exits 1.

#### Process Restart

Some domains require the affected process to be restarted for changes to take
effect. After a successful `DefaultsWrite`, the adapter MUST issue `killall
<Process>` for the domain if it appears in the following registry:

| Domain                     | Process to restart |
|----------------------------|--------------------|
| `com.apple.dock`           | `Dock`             |
| `com.apple.finder`         | `Finder`           |
| `com.apple.SystemUIServer` | `SystemUIServer`   |
| `NSGlobalDomain`           | (no restart required; system reads on next use) |

The adapter MUST NOT issue a restart for domains not listed above. The restart
command is:

```sh
killall <Process>
```

### Hostname

#### Set

All three of the following commands MUST be executed when `SetHostname` is
called:

```sh
sudo scutil --set HostName <name>
sudo scutil --set LocalHostName <name>
sudo scutil --set ComputerName <name>
```

All three require privilege escalation (see §Privilege Escalation).

#### Check

```sh
scutil --get HostName
```

- Exit 0: value is set; output is the current hostname.
- Exit 1: value is not set.

---

## Ubuntu/Debian Adapter

### Package Operations

#### Check

To determine whether a package is installed:

```sh
dpkg-query -W -f='${Status}' <name>
```

- The package is installed if and only if the output contains the substring
  `install ok installed`.
- Exit 0: package is known to dpkg (may or may not be installed; inspect
  the status string).
- Exit 1: package is unknown to dpkg (not installed).

The adapter MUST use `dpkg-query` for package status checks. The adapter MUST
NOT use `apt list` for programmatic checks; `apt list` has an unstable CLI
interface and emits a warning to stderr on every invocation.

#### Install

```sh
sudo apt install -y <name>              # unversioned
sudo apt install -y <name>=<version>    # exact version string
```

The `<version>` value MUST be the exact version string as reported by
`apt-cache madison <name>`, including the Debian/Ubuntu revision suffix (e.g.,
`20.11.0-1nodesource1`). The adapter MUST NOT construct or infer version strings.

Before attempting a versioned install, the adapter MUST validate the version
string by executing:

```sh
apt-cache madison <name>
```

The output format is one line per available version:

```
<name> | <version> | <source>
```

If the configured version string does not appear in the `apt-cache madison`
output, the adapter MUST return `ErrVersionNotAvailable` without executing
`apt install`.

#### Upgrade

```sh
sudo apt install -y --only-upgrade <name>
```

`--only-upgrade` upgrades the package if already installed but does not install
it if absent. The adapter MUST use this form and MUST NOT use `apt upgrade <name>`.

If `Package.Pinned` is `true`, `PackageUpgrade` MUST NOT execute any command
and MUST return `ErrPackagePinned`.

#### Remove

```sh
sudo apt remove -y <name>
```

- Exit 0: package removed, or was already absent (idempotent).

If the package has a hold, `apt remove` will fail. The adapter MUST execute
`sudo apt-mark unhold <name>` before `sudo apt remove -y <name>` when removing
a held package.

### Version Pinning

#### Hold

Called after a successful `apt install` when `Package.Pinned` is `true`:

```sh
sudo apt-mark hold <name>
```

- Exit 0 always.

#### Unhold

```sh
sudo apt-mark unhold <name>
```

- Exit 0 always.

#### Check Hold Status

To list all held packages:

```sh
apt-mark showhold
```

- Output: one package name per line; empty output indicates no held packages.
- Exit 0 always.

To check hold status for a specific package:

```sh
dpkg --get-selections <name>
```

- Output format: `<name>\thold` if the package is held.
- Exit 0: package is installed.
- Exit 1: package is not installed.

#### Hold Behavior — Normative Constraints

- `apt-mark hold` prevents both upgrade and removal. This is more restrictive
  than `brew pin`, which prevents only upgrade.
- `apt upgrade` and `apt dist-upgrade` skip held packages and print `The
  following packages have been kept back:` followed by the package names, but
  exit 0.
- The adapter's `PackageRemove` MUST execute `sudo apt-mark unhold <name>`
  before `sudo apt remove -y <name>` when the package is held.
- Hold state is stored in `/var/lib/dpkg/status` as `Status: hold ok installed`.

### Preferences

The Ubuntu/Debian adapter does not implement `DefaultsRead` or `DefaultsWrite`.
These methods MUST return an error indicating the operation is not supported on
this platform. `gsettings` and `dconf` operations are owned by the `gsettings`
built-in step and are not part of the adapter interface in v1.

### Services

```sh
sudo systemctl enable <name>      # enable at boot
sudo systemctl disable <name>     # disable at boot
sudo systemctl start <name>       # start now
sudo systemctl stop <name>        # stop now
```

Status checks:

```sh
systemctl is-enabled <name>
# Exit 0: service is enabled.
# Exit non-zero: service is disabled or not found.

systemctl is-active <name>
# Exit 0: service is running.
# Exit non-zero: service is stopped.
```

All `systemctl` mutating commands (`enable`, `disable`, `start`, `stop`)
require privilege escalation. Status check commands (`is-enabled`, `is-active`)
do not require privilege escalation.

### Hostname

#### Set

```sh
sudo hostnamectl set-hostname <name>
```

- Exit 0: hostname set successfully.

Requires privilege escalation.

#### Check

```sh
hostnamectl status --static
```

- Output: the static hostname.
- Exit 0 always.

---

## Generic Adapter

The generic adapter implements platform-independent operations. It MUST be
active on all platforms alongside the platform-specific adapter.

### Directories

```sh
mkdir -p <path>
```

- Exit 0: directory exists or was created.
- Idempotent.

### Git Config

Set a value:

```sh
git config --global <key> <value>
```

Read a value:

```sh
git config --global --get <key>
```

- Exit 0: key is set; output is the value.
- Exit 1: key is not set.

### Shell Setup

Set the current user's default login shell:

```sh
chsh -s <shell>
```

The `<shell>` value MUST be an absolute path (e.g., `/bin/zsh`, `/opt/homebrew/bin/fish`).

### File Operations

Create or update a symbolic link:

```sh
ln -sf <target> <link>
```

- `-s`: create a symbolic link.
- `-f`: remove any existing file at `<link>` before creating the link.
- Idempotent.

Copy a file:

```sh
cp <src> <dst>
```

None of the generic adapter operations require privilege escalation.

---

## Privilege Escalation

### Operations Requiring sudo

| Adapter        | Operation                                              | Requires sudo |
|----------------|--------------------------------------------------------|---------------|
| Darwin         | `brew install`, `brew upgrade`, `brew uninstall`       | No — MUST NOT be called with sudo; will fail if UID is 0 |
| Darwin         | `brew pin`, `brew unpin`                               | No            |
| Darwin         | `defaults write`, `defaults read`                      | No            |
| Darwin         | `scutil --set HostName/LocalHostName/ComputerName`     | Yes           |
| Darwin         | `brew services start` (system daemons)                 | Yes (user-level agents: No) |
| Ubuntu/Debian  | `apt install`, `apt remove`                            | Yes           |
| Ubuntu/Debian  | `apt-mark hold`, `apt-mark unhold`                     | Yes           |
| Ubuntu/Debian  | `systemctl enable/disable/start/stop`                  | Yes           |
| Ubuntu/Debian  | `hostnamectl set-hostname`                             | Yes           |
| Generic        | All operations                                         | No            |

### Escalation Strategy

At the start of `apply`, if any step in the execution plan requires a sudo
operation, the runtime MUST:

1. Print the following message:
   ```
   This run requires administrator privileges for: <list of operations>.
   ```
2. Execute `sudo -v` to acquire and cache credentials.
   - If `sudo -v` exits non-zero, the runtime MUST abort with exit code 3
     (pre-flight failure) without executing any steps.
3. Launch a keep-alive background process to prevent credential cache expiry:
   ```sh
   ( while true; do sudo -n true; sleep 50; done ) &
   KEEPALIVE_PID=$!
   ```
4. On completion of `apply` (whether success or failure), the runtime MUST
   terminate the keep-alive process.

The 50-second refresh interval MUST be used. macOS sudo cache TTL defaults to 5
minutes; Linux defaults to 15 minutes. The 50-second interval is within both
TTLs.

### brew + sudo Mutual Exclusion

The Darwin adapter MUST never invoke any `brew` command via `sudo` or while the
process has effective UID 0. If the process is running as root (UID 0), all
`brew` method calls MUST return `ErrBrewCalledAsRoot` without executing any
command. This condition is unrecoverable.

### Plan-Phase Privilege Detection

The plan phase MUST determine whether any steps require privileged operations
before `apply` begins. Each adapter method MUST declare whether it requires
privilege. The executor MUST aggregate privilege requirements across the full
plan before calling `sudo -v`.

---

## Status and Capture Support

### Darwin

#### Leaf Formula Listing

```sh
brew leaves
```

- Output: one bare formula name per line; no versions; no casks.
- Exit 0 always, including when output is empty.
- Output format is always one-per-line regardless of terminal or pipe context.

The adapter MUST use `brew leaves` (not `brew list`) for drift detection and
`capture` output. `brew leaves` excludes transitive dependencies and returns
only formulas that nothing else depends on.

To retrieve versions for leaf formulas:

```sh
brew leaves | xargs brew list --versions
```

- Output format: `<name> <version>` per line.

#### Cask Listing

```sh
brew list --cask
```

- Output: one bare cask name per line.
- Exit 0 always.

There is no `brew leaves --cask` equivalent; all casks are treated as leaves.
The adapter MUST use `brew list --cask` for cask drift detection and `capture`
output.

#### Drift Detection Algorithm (Darwin)

1. Execute `brew leaves` → set L (leaf formulas on machine).
2. Read `packages.brew[]` from config → set C (configured formulas).
3. `L − C` = packages on machine but not in config (untracked drift).
4. `C − L` = packages in config but not on machine (unsatisfied).
5. Repeat with `brew list --cask` → set LC and `packages.cask[]` → set CC.
6. `LC − CC` = casks on machine but not in config.
7. `CC − LC` = casks in config but not on machine.

#### capture Output (Darwin)

Packages in `L − C` (formulas) and `LC − CC` (casks) MUST be formatted as
config entries suitable for appending to `packages.brew[]` and
`packages.cask[]` respectively.

### Ubuntu/Debian

#### Manual Package Listing

```sh
apt-mark showmanual
```

- Output: one manually-installed package name per line.
- Exit 0 always, including when output is empty.

`apt-mark showmanual` is the Ubuntu/Debian equivalent of `brew leaves` for
`capture` and `init` purposes. The adapter MUST use `apt-mark showmanual` for
drift detection against `packages.apt[]`.

#### Full Installed Package Listing

```sh
dpkg-query -W -f='${Package}\t${Version}\t${db:Status-Abbrev}\n'
```

- Output: tab-separated lines of `<name>`, `<version>`, `<status-flags>`.
- Status flags: `ii` = installed, `hi` = held+installed, `rc` = removed but config retained.
- The adapter MUST filter for lines where the status field starts with `i` or `h`
  to identify installed packages.

The adapter MUST NOT use `apt list --installed` in scripts.

#### Drift Detection Algorithm (Ubuntu/Debian)

1. Execute `apt-mark showmanual` → set M (manually-installed packages).
2. Read `packages.apt[]` from config → set C.
3. `M − C` = packages on machine but not in config (untracked drift).
4. `C − M` = packages in config but not on machine (unsatisfied).

---

## Error Types

The following error constants MUST be defined and returned in the specified
circumstances:

| Error constant             | Returned when |
|----------------------------|---------------|
| `ErrBrewNotFound`          | Any Homebrew adapter method is called and `brew` is not present in `PATH`. |
| `ErrAptNotFound`           | Any Ubuntu/Debian adapter method is called and `apt` is not present in `PATH`. |
| `ErrBrewCalledAsRoot`      | Any Homebrew adapter method is called and the process has effective UID 0. |
| `ErrPackagePinned`         | `PackageUpgrade` is called with a `Package` where `Pinned` is `true`. |
| `ErrCaskPinNotSupported`   | `PackageInstall` is called with a `Package` where both `Cask` and `Pinned` are `true`. |
| `ErrNoVersionedFormula`    | The `Version` field on a Darwin `Package` contains a non-major version string (e.g., `"20.11.0"` instead of `"20"`). |
| `ErrVersionNotAvailable`   | The configured `apt` version string does not appear in `apt-cache madison` output. |
| `ErrDefaultsTypeMismatch`  | Post-write `defaults read-type` returns a type that differs from the declared `DefaultsValue.Type`. The error message MUST name the domain, key, declared type, and actual type. |
| `ErrDefaultsValueMismatch` | Post-write `defaults read` returns a value that differs from the declared `DefaultsValue.Raw` after boolean normalisation. |
| `ErrPrivilegeRequired`     | `sudo -v` exits non-zero during pre-flight privilege acquisition. |
| `ErrUnsupportedPlatform`   | The detected OS has no available adapter in v1. |
