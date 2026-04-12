package scan

import (
	"fmt"
	"strings"
	"testing"
)

// mockRunner creates a CommandRunner that returns predefined outputs.
func mockRunner(responses map[string]string) CommandRunner {
	return func(name string, args ...string) (string, error) {
		key := name + " " + strings.Join(args, " ")
		if out, ok := responses[key]; ok {
			return out, nil
		}
		return "", fmt.Errorf("command not found: %s", key)
	}
}

func TestScanMachineWithDarwin(t *testing.T) {
	run := mockRunner(map[string]string{
		"scutil --get ComputerName":          "My MacBook",
		"brew leaves":                        "git\nwget\ncurl",
		"brew list --cask":                   "firefox\niterm2",
		"defaults read NSGlobalDomain AppleShowAllExtensions":             "1",
		"defaults read NSGlobalDomain NSAutomaticSpellingCorrectionEnabled": "0",
		"defaults read NSGlobalDomain NSAutomaticCapitalizationEnabled":     "0",
		"defaults read com.apple.dock autohide":      "1",
		"defaults read com.apple.dock tilesize":      "48",
		"defaults read com.apple.dock magnification": "0",
		"defaults read com.apple.finder AppleShowAllFiles":    "1",
		"defaults read com.apple.finder ShowPathbar":          "1",
		"defaults read com.apple.finder ShowStatusBar":        "0",
		"defaults read com.apple.finder FXPreferredViewStyle": "Nlsv",
		"defaults read com.apple.screencapture location": "~/Desktop",
		"defaults read com.apple.screencapture type":     "png",
		"dockutil --list":                    "Safari\t/Applications/Safari.app\nMail\t/Applications/Mail.app",
		"git config --global user.name":  "Test User",
		"git config --global user.email": "test@example.com",
		"git config --global github.user": "testuser",
	})

	result, err := scanMachineWith("darwin", run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Platform != "darwin" {
		t.Errorf("platform = %q, want %q", result.Platform, "darwin")
	}
	if result.Hostname != "My MacBook" {
		t.Errorf("hostname = %q, want %q", result.Hostname, "My MacBook")
	}
	if len(result.Packages.Brew) != 3 {
		t.Errorf("brew packages = %d, want 3", len(result.Packages.Brew))
	}
	if len(result.Packages.Cask) != 2 {
		t.Errorf("cask packages = %d, want 2", len(result.Packages.Cask))
	}
	if result.Identity.GitName != "Test User" {
		t.Errorf("git_name = %q, want %q", result.Identity.GitName, "Test User")
	}
	if result.Identity.GitEmail != "test@example.com" {
		t.Errorf("git_email = %q, want %q", result.Identity.GitEmail, "test@example.com")
	}
	if result.Identity.GithubUser != "testuser" {
		t.Errorf("github_user = %q, want %q", result.Identity.GithubUser, "testuser")
	}

	// Check defaults were detected
	if _, ok := result.Defaults["NSGlobalDomain"]; !ok {
		t.Error("expected NSGlobalDomain defaults")
	}
	if _, ok := result.Defaults["com.apple.dock"]; !ok {
		t.Error("expected com.apple.dock defaults")
	}
	if _, ok := result.Defaults["com.apple.finder"]; !ok {
		t.Error("expected com.apple.finder defaults")
	}
	if _, ok := result.Defaults["com.apple.screencapture"]; !ok {
		t.Error("expected com.apple.screencapture defaults")
	}

	// Check dock apps
	if len(result.Dock) != 2 {
		t.Errorf("dock apps = %d, want 2", len(result.Dock))
	}
	if result.DockError != "" {
		t.Errorf("dock error = %q, want empty", result.DockError)
	}
}

func TestScanMachineDockutilMissing(t *testing.T) {
	// dockutil not in mock responses = command fails
	run := mockRunner(map[string]string{
		"scutil --get ComputerName": "My MacBook",
	})

	result, err := scanMachineWith("darwin", run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Dock) != 0 {
		t.Errorf("dock apps = %d, want 0 when dockutil missing", len(result.Dock))
	}
	if result.DockError == "" {
		t.Error("expected dock error when dockutil missing")
	}
}

func TestDetectDock(t *testing.T) {
	run := mockRunner(map[string]string{
		"dockutil --list": "Safari\t/Applications/Safari.app\nChrome\t/Applications/Google Chrome.app\nTerminal\t/Applications/Utilities/Terminal.app",
	})

	apps, errMsg := detectDock(run)
	if errMsg != "" {
		t.Errorf("unexpected error: %s", errMsg)
	}
	if len(apps) != 3 {
		t.Errorf("dock apps = %d, want 3", len(apps))
	}
	if apps[0] != "Safari" {
		t.Errorf("first app = %q, want Safari", apps[0])
	}
}

func TestScanMachineWithUbuntu(t *testing.T) {
	run := mockRunner(map[string]string{
		"hostnamectl status --static":        "my-server",
		"apt-mark showmanual":                "build-essential\ncurl\ngit",
		"git config --global user.name":      "Linux User",
		"git config --global user.email":     "linux@example.com",
		"git config --global github.user":    "linuxuser",
	})

	result, err := scanMachineWith("ubuntu", run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Platform != "ubuntu" {
		t.Errorf("platform = %q, want %q", result.Platform, "ubuntu")
	}
	if result.Hostname != "my-server" {
		t.Errorf("hostname = %q, want %q", result.Hostname, "my-server")
	}
	if len(result.Packages.Apt) != 3 {
		t.Errorf("apt packages = %d, want 3", len(result.Packages.Apt))
	}
	if len(result.Packages.Brew) != 0 {
		t.Errorf("brew packages = %d, want 0", len(result.Packages.Brew))
	}

	// No defaults on ubuntu
	if len(result.Defaults) != 0 {
		t.Errorf("defaults = %d, want 0 on ubuntu", len(result.Defaults))
	}
}

func TestScanMachineGracefulFailure(t *testing.T) {
	// All commands fail — should still produce a result with empty fields.
	run := mockRunner(map[string]string{})

	result, err := scanMachineWith("darwin", run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Platform != "darwin" {
		t.Errorf("platform = %q, want %q", result.Platform, "darwin")
	}
	// Hostname falls back to os.Hostname which should work
	// Packages should be empty
	if len(result.Packages.Brew) != 0 {
		t.Errorf("brew packages should be empty when brew fails")
	}
	if result.Identity.GitName != "" {
		t.Errorf("git_name should be empty when git fails")
	}
}

func TestToYAML(t *testing.T) {
	// Fix date for deterministic output
	origDate := currentDate
	currentDate = func() string { return "2026-04-10" }
	defer func() { currentDate = origDate }()

	result := &ScanResult{
		Platform: "darwin",
		Hostname: "My MacBook",
		Packages: ScanPackages{
			Brew: []string{"git", "wget"},
			Cask: []string{"firefox"},
		},
		Shell: ScanShell{
			Default: "zsh",
			OhMyZsh: true,
			Plugins: []string{"git", "docker"},
		},
		Identity: ScanIdentity{
			GitName:  "Test User",
			GitEmail: "test@example.com",
		},
		Dock:        []string{"Safari", "Mail"},
		Directories: []string{"~/Projects"},
		Defaults:    map[string]map[string]interface{}{},
	}

	data, err := result.ToYAML()
	if err != nil {
		t.Fatalf("ToYAML error: %v", err)
	}

	yamlStr := string(data)

	// Check required fields are present
	checks := []string{
		"name: my-macbook",
		"platform: darwin",
		"hostname: My MacBook",
		"git_name: Test User",
		"git_email: test@example.com",
		"oh_my_zsh: true",
		"default: zsh",
		"- git",
		"- wget",
		"- firefox",
		"- ~/Projects",
		"- Safari",
		"- Mail",
	}
	for _, check := range checks {
		if !strings.Contains(yamlStr, check) {
			t.Errorf("YAML missing %q\n---\n%s", check, yamlStr)
		}
	}

	// Check comments are present
	commentChecks := []string{
		"# Generated by adze init",
		"# Platform: darwin",
		"# Date: 2026-04-10",
		"# Detected from: git config --global",
		"# Detected from: brew leaves (2 packages)",
		"# Detected from: brew list --cask (1 packages)",
		"# Detected from: dockutil --list (2 apps)",
		"# Detected from: $SHELL and ~/.oh-my-zsh",
		"# Detected from: scanning ~/ (1 directories)",
	}
	for _, check := range commentChecks {
		if !strings.Contains(yamlStr, check) {
			t.Errorf("YAML missing comment %q\n---\n%s", check, yamlStr)
		}
	}
}

func TestToYAMLDockError(t *testing.T) {
	origDate := currentDate
	currentDate = func() string { return "2026-04-10" }
	defer func() { currentDate = origDate }()

	result := &ScanResult{
		Platform:  "darwin",
		Hostname:  "My MacBook",
		DockError: "dockutil not installed",
		Defaults:  map[string]map[string]interface{}{},
	}

	data, err := result.ToYAML()
	if err != nil {
		t.Fatalf("ToYAML error: %v", err)
	}

	yamlStr := string(data)
	if !strings.Contains(yamlStr, "dockutil not installed") {
		t.Errorf("YAML should contain dock error comment\n---\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "dock:") {
		t.Errorf("YAML should contain dock key\n---\n%s", yamlStr)
	}
}

func TestToYAMLMinimal(t *testing.T) {
	origDate := currentDate
	currentDate = func() string { return "2026-04-10" }
	defer func() { currentDate = origDate }()

	result := &ScanResult{
		Platform: "ubuntu",
		Defaults: map[string]map[string]interface{}{},
	}

	data, err := result.ToYAML()
	if err != nil {
		t.Fatalf("ToYAML error: %v", err)
	}

	yamlStr := string(data)
	if !strings.Contains(yamlStr, "name: my-ubuntu-machine") {
		t.Errorf("expected fallback name, got:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "platform: ubuntu") {
		t.Errorf("expected platform: ubuntu, got:\n%s", yamlStr)
	}
	// Should not have empty sections
	if strings.Contains(yamlStr, "packages:") {
		t.Errorf("should not have empty packages section:\n%s", yamlStr)
	}
	// Should have header comment
	if !strings.Contains(yamlStr, "# Generated by adze init") {
		t.Errorf("expected header comment, got:\n%s", yamlStr)
	}
}

func TestMachineName(t *testing.T) {
	tests := []struct {
		hostname string
		platform string
		want     string
	}{
		{"My MacBook Pro", "darwin", "my-macbook-pro"},
		{"server01", "ubuntu", "server01"},
		{"", "darwin", "my-darwin-machine"},
		{"", "ubuntu", "my-ubuntu-machine"},
	}

	for _, tt := range tests {
		t.Run(tt.hostname+"/"+tt.platform, func(t *testing.T) {
			r := &ScanResult{Hostname: tt.hostname, Platform: tt.platform}
			got := machineName(r)
			if got != tt.want {
				t.Errorf("machineName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseLines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"a\nb\nc", 3},
		{"a\n\nb\n", 2},
		{"", 0},
		{"\n\n\n", 0},
		{"single", 1},
	}

	for _, tt := range tests {
		got := parseLines(tt.input)
		if len(got) != tt.want {
			t.Errorf("parseLines(%q) = %d items, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestParseZshPlugins(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			"standard",
			"plugins=(git docker kubectl)",
			[]string{"git", "docker", "kubectl"},
		},
		{
			"with comments",
			"# plugins=(old stuff)\nplugins=(git zsh-autosuggestions)",
			[]string{"git", "zsh-autosuggestions"},
		},
		{
			"no plugins line",
			"source ~/.oh-my-zsh/oh-my-zsh.sh",
			nil,
		},
		{
			"empty plugins",
			"plugins=()",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseZshPlugins(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("parseZshPlugins() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseZshPlugins()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseDefaultsValue(t *testing.T) {
	tests := []struct {
		input string
		want  interface{}
	}{
		{"1", true},
		{"true", true},
		{"0", false},
		{"false", false},
		{"48", 48},
		{"some-string", "some-string"},
	}

	for _, tt := range tests {
		got := parseDefaultsValue(tt.input)
		if got != tt.want {
			t.Errorf("parseDefaultsValue(%q) = %v (%T), want %v (%T)", tt.input, got, got, tt.want, tt.want)
		}
	}
}

func TestSortedCopy(t *testing.T) {
	input := []string{"z", "a", "m"}
	got := sortedCopy(input)
	if got[0] != "a" || got[1] != "m" || got[2] != "z" {
		t.Errorf("sortedCopy = %v, want [a m z]", got)
	}
	// Original should be unchanged
	if input[0] != "z" {
		t.Errorf("original modified")
	}

	// Nil/empty
	if sortedCopy(nil) != nil {
		t.Errorf("sortedCopy(nil) should be nil")
	}
	if sortedCopy([]string{}) != nil {
		t.Errorf("sortedCopy([]) should be nil")
	}
}
