package steps

import (
	"github.com/gregberns/adze/internal/config"
)

// SudoCasks lists Homebrew casks known to require sudo (admin password)
// during install. Used to surface a pre-flight warning so users aren't
// surprised mid-run by a Password: prompt hidden behind the spinner.
//
// Maintenance: add entries as new sudo-requiring casks are discovered.
// Removal: if a cask no longer prompts for sudo on current macOS, remove
// the entry — over-warning erodes trust just as much as under-warning.
var SudoCasks = map[string]string{
	"docker":             "Docker Desktop installer requires admin password",
	"virtualbox":         "VirtualBox installs a kernel extension (admin password required)",
	"parallels":          "Parallels installs a kernel extension (admin password required)",
	"vmware-fusion":      "VMware Fusion installs a kernel extension (admin password required)",
	"wireshark":          "Wireshark configures BPF permissions (admin password required)",
	"tuxera-ntfs":        "Tuxera NTFS installs a kernel extension (admin password required)",
	"microsoft-teams":    "Microsoft Teams installer may request admin password",
	"google-drive":       "Google Drive helper installer requires admin password",
	"little-snitch":      "Little Snitch installs a system extension (admin password required)",
	"karabiner-elements": "Karabiner-Elements installs a driver/system extension (admin password required)",
}

// SudoNotice describes one step or item that may prompt for sudo at runtime.
type SudoNotice struct {
	StepName string `json:"step"`           // e.g. "brew-casks", "machine-name"
	ItemName string `json:"item,omitempty"` // e.g. "docker"; empty for atomic steps
	Reason   string `json:"reason"`
}

// SudoStepsForConfig inspects the user's config and returns notices for
// steps or items known to require sudo at runtime. Used by plan and apply
// to print a pre-flight warning so the user can decide whether to continue
// with the laptop plugged in / admin password ready.
func SudoStepsForConfig(cfg *config.Config) []SudoNotice {
	var notices []SudoNotice

	// Casks from packages.cask matching the allow-list.
	for _, pkg := range cfg.Packages.Cask {
		if reason, ok := SudoCasks[pkg.Name]; ok {
			notices = append(notices, SudoNotice{
				StepName: "brew-casks",
				ItemName: pkg.Name,
				Reason:   reason,
			})
		}
	}

	// Built-in steps known to require sudo (macOS).
	if cfg.Machine.Hostname != "" {
		notices = append(notices, SudoNotice{
			StepName: "machine-name",
			Reason:   "Setting the computer hostname requires admin password (scutil)",
		})
	}

	return notices
}
