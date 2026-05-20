package steps

import (
	"testing"

	"github.com/gregberns/adze/internal/config"
)

func includedStepNames(t *testing.T, cfg *config.Config, platform string) map[string]bool {
	t.Helper()
	reg := NewRegistry()
	configs := BuildStepConfigs(cfg, platform, reg)
	names := make(map[string]bool, len(configs))
	for _, sc := range configs {
		names[sc.Name] = true
	}
	return names
}

// R4.1 — empty config on darwin must not include any transitive-only step.
func TestInclusionRule_EmptyConfigDarwin(t *testing.T) {
	cfg := &config.Config{Name: "test", Platform: "darwin"}
	names := includedStepNames(t, cfg, "darwin")

	for _, forbidden := range []string{
		"xcode-cli-tools", "homebrew", "apt-essentials",
		"node-fnm", "python", "go", "rust", "ssh-keys",
	} {
		if names[forbidden] {
			t.Errorf("step %q must NOT appear in plan for empty darwin config; got %v", forbidden, names)
		}
	}
}

// R4.1b — same for ubuntu.
func TestInclusionRule_EmptyConfigUbuntu(t *testing.T) {
	cfg := &config.Config{Name: "test", Platform: "ubuntu"}
	names := includedStepNames(t, cfg, "ubuntu")

	for _, forbidden := range []string{"apt-essentials", "node-fnm", "python", "go", "rust", "ssh-keys"} {
		if names[forbidden] {
			t.Errorf("step %q must NOT appear in plan for empty ubuntu config; got %v", forbidden, names)
		}
	}
}

// R4.2 — packages.brew transitively pulls homebrew + xcode-cli-tools.
func TestInclusionRule_BrewPullsInInfra(t *testing.T) {
	cfg := &config.Config{
		Name:     "test",
		Platform: "darwin",
		Packages: config.PackagesConfig{
			Brew: []config.PackageEntry{{Name: "git"}},
		},
	}
	names := includedStepNames(t, cfg, "darwin")

	for _, expected := range []string{"brew-packages", "homebrew", "xcode-cli-tools"} {
		if !names[expected] {
			t.Errorf("expected %q in plan; got %v", expected, names)
		}
	}
	for _, forbidden := range []string{"rust", "go", "python", "node-fnm", "ssh-keys"} {
		if names[forbidden] {
			t.Errorf("step %q must NOT appear with only brew packages; got %v", forbidden, names)
		}
	}
}

// R4.3 — custom step requires rust → rust pulled in, languages not.
func TestInclusionRule_CustomStepRequiresRust(t *testing.T) {
	cfg := &config.Config{
		Name:     "test",
		Platform: "darwin",
		CustomSteps: map[string]config.CustomStep{
			"build-tool": {
				Description: "needs rust",
				Provides:    []string{"build-tool"},
				Requires:    []string{"rust"},
				Platform:    []string{"darwin"},
				Check:       "command -v build-tool",
				Apply:       map[string]string{"darwin": "echo install"},
			},
		},
	}
	names := includedStepNames(t, cfg, "darwin")

	if !names["rust"] {
		t.Errorf("rust must appear when a custom step requires it; got %v", names)
	}
	for _, forbidden := range []string{"go", "python", "node-fnm", "ssh-keys"} {
		if names[forbidden] {
			t.Errorf("step %q must NOT appear (only rust was required); got %v", forbidden, names)
		}
	}
}

// R4.4 — custom step requires go → transitively pulls homebrew + xcode-cli-tools on darwin (depth 3).
func TestInclusionRule_CustomStepRequiresGoDepth3(t *testing.T) {
	cfg := &config.Config{
		Name:     "test",
		Platform: "darwin",
		CustomSteps: map[string]config.CustomStep{
			"go-tool": {
				Description: "needs go",
				Provides:    []string{"go-tool"},
				Requires:    []string{"go"},
				Platform:    []string{"darwin"},
				Check:       "command -v go-tool",
				Apply:       map[string]string{"darwin": "echo install"},
			},
		},
	}
	names := includedStepNames(t, cfg, "darwin")

	for _, expected := range []string{"go", "homebrew", "xcode-cli-tools"} {
		if !names[expected] {
			t.Errorf("expected %q in plan (depth-3 transitive); got %v", expected, names)
		}
	}
	for _, forbidden := range []string{"rust", "python", "node-fnm"} {
		if names[forbidden] {
			t.Errorf("step %q must NOT appear (only go was required); got %v", forbidden, names)
		}
	}
}

// R4.5 — ssh-keys gated by identity.generate_ssh_key.
func TestInclusionRule_SSHKeysFlag(t *testing.T) {
	t.Run("flag_false_default", func(t *testing.T) {
		cfg := &config.Config{
			Name:     "test",
			Platform: "darwin",
			Identity: config.IdentityConfig{
				GitName:  "Test User",
				GitEmail: "test@example.com",
			},
		}
		names := includedStepNames(t, cfg, "darwin")
		if names["ssh-keys"] {
			t.Errorf("ssh-keys must NOT appear when generate_ssh_key is false; got %v", names)
		}
	})

	t.Run("flag_true_includes_ssh_keys", func(t *testing.T) {
		cfg := &config.Config{
			Name:     "test",
			Platform: "darwin",
			Identity: config.IdentityConfig{
				GitName:        "Test User",
				GitEmail:       "test@example.com",
				GenerateSSHKey: true,
			},
		}
		names := includedStepNames(t, cfg, "darwin")
		if !names["ssh-keys"] {
			t.Errorf("ssh-keys must appear when generate_ssh_key is true; got %v", names)
		}
	})

	t.Run("custom_step_requires_ssh_keys", func(t *testing.T) {
		cfg := &config.Config{
			Name:     "test",
			Platform: "darwin",
			CustomSteps: map[string]config.CustomStep{
				"deploy": {
					Description: "needs ssh",
					Provides:    []string{"deploy"},
					Requires:    []string{"ssh-keys"},
					Platform:    []string{"darwin"},
					Check:       "true",
					Apply:       map[string]string{"darwin": "echo"},
				},
			},
		}
		names := includedStepNames(t, cfg, "darwin")
		if !names["ssh-keys"] {
			t.Errorf("ssh-keys must appear when a custom step requires it (even without the flag); got %v", names)
		}
	})
}

// R4.6 — property: every candidate is absent from an empty-config plan.
func TestInclusionRule_Property_AllCandidatesGated(t *testing.T) {
	reg := NewRegistry()
	for _, platform := range []string{"darwin", "ubuntu"} {
		t.Run(platform, func(t *testing.T) {
			cfg := &config.Config{Name: "test", Platform: platform}
			names := includedStepNames(t, cfg, platform)

			for _, def := range reg.ForPlatform(platform) {
				if !isCandidate(def) {
					continue
				}
				if names[def.Name] {
					t.Errorf("candidate %q is present in empty-config plan on %s; "+
						"candidate steps must be transitive-only", def.Name, platform)
				}
			}
		})
	}
}

// R4.7 — property: every candidate is included when something requires it.
func TestInclusionRule_Property_TransitiveResolves(t *testing.T) {
	reg := NewRegistry()
	for _, platform := range []string{"darwin", "ubuntu"} {
		for _, def := range reg.ForPlatform(platform) {
			if !isCandidate(def) {
				continue
			}
			if len(def.Provides) == 0 {
				continue
			}
			capability := def.Provides[0]

			t.Run(platform+"/"+def.Name, func(t *testing.T) {
				cfg := &config.Config{
					Name:     "test",
					Platform: platform,
					CustomSteps: map[string]config.CustomStep{
						"consumer": {
							Description: "consumes " + capability,
							Provides:    []string{"consumer-cap"},
							Requires:    []string{capability},
							Platform:    []string{platform},
							Check:       "true",
							Apply:       map[string]string{platform: "echo"},
						},
					},
				}
				names := includedStepNames(t, cfg, platform)
				if !names[def.Name] {
					t.Errorf("candidate %q (provides %q) was not pulled in by a custom step requiring %q; plan: %v",
						def.Name, capability, capability, names)
				}
			})
		}
	}
}

// Idempotency — running BuildStepConfigs twice yields identical sets.
func TestInclusionRule_Idempotent(t *testing.T) {
	cfg := &config.Config{
		Name:     "test",
		Platform: "darwin",
		Packages: config.PackagesConfig{
			Brew: []config.PackageEntry{{Name: "git"}},
		},
	}
	a := includedStepNames(t, cfg, "darwin")
	b := includedStepNames(t, cfg, "darwin")

	if len(a) != len(b) {
		t.Fatalf("non-idempotent: %d vs %d steps", len(a), len(b))
	}
	for name := range a {
		if !b[name] {
			t.Errorf("step %q present in run 1 but not run 2", name)
		}
	}
}
