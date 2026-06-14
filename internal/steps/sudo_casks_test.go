package steps

import (
	"testing"

	"github.com/gregberns/adze/internal/config"
)

func TestSudoStepsForConfig_DockerCask(t *testing.T) {
	cfg := &config.Config{
		Packages: config.PackagesConfig{
			Cask: []config.PackageEntry{{Name: "docker"}},
		},
	}
	notices := SudoStepsForConfig(cfg)
	if len(notices) != 1 {
		t.Fatalf("got %d notices, want 1: %+v", len(notices), notices)
	}
	if notices[0].StepName != "brew-casks" || notices[0].ItemName != "docker" {
		t.Errorf("notice = %+v, want brew-casks[docker]", notices[0])
	}
	if notices[0].Reason == "" {
		t.Error("reason must not be empty")
	}
}

func TestSudoStepsForConfig_MultipleCasks(t *testing.T) {
	cfg := &config.Config{
		Packages: config.PackagesConfig{
			Cask: []config.PackageEntry{
				{Name: "docker"},
				{Name: "iterm2"}, // not sudo
				{Name: "virtualbox"},
			},
		},
	}
	notices := SudoStepsForConfig(cfg)
	if len(notices) != 2 {
		t.Fatalf("got %d notices, want 2: %+v", len(notices), notices)
	}
	got := map[string]bool{}
	for _, n := range notices {
		got[n.ItemName] = true
	}
	if !got["docker"] || !got["virtualbox"] {
		t.Errorf("missing expected notices; got %v", got)
	}
}

func TestSudoStepsForConfig_NoSudoCasks(t *testing.T) {
	cfg := &config.Config{
		Packages: config.PackagesConfig{
			Cask: []config.PackageEntry{{Name: "iterm2"}, {Name: "visual-studio-code"}},
		},
	}
	notices := SudoStepsForConfig(cfg)
	if len(notices) != 0 {
		t.Errorf("expected 0 notices, got %d: %+v", len(notices), notices)
	}
}

func TestSudoStepsForConfig_MachineHostname(t *testing.T) {
	cfg := &config.Config{
		Machine: config.MachineConfig{Hostname: "MyMac"},
	}
	notices := SudoStepsForConfig(cfg)
	if len(notices) != 1 {
		t.Fatalf("got %d notices, want 1", len(notices))
	}
	if notices[0].StepName != "machine-name" {
		t.Errorf("step = %q, want machine-name", notices[0].StepName)
	}
	if notices[0].ItemName != "" {
		t.Errorf("ItemName should be empty for atomic step, got %q", notices[0].ItemName)
	}
}

func TestSudoStepsForConfig_Empty(t *testing.T) {
	cfg := &config.Config{}
	notices := SudoStepsForConfig(cfg)
	if len(notices) != 0 {
		t.Errorf("expected 0 notices for empty config, got %d", len(notices))
	}
}

func TestSudoStepsForConfig_CaskAndHostnameCombined(t *testing.T) {
	cfg := &config.Config{
		Machine: config.MachineConfig{Hostname: "MyMac"},
		Packages: config.PackagesConfig{
			Cask: []config.PackageEntry{{Name: "docker"}},
		},
	}
	notices := SudoStepsForConfig(cfg)
	if len(notices) != 2 {
		t.Fatalf("got %d notices, want 2", len(notices))
	}
}
