package ui

import (
	"testing"
)

func TestColorize_Enabled(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		color    Color
		expected string
	}{
		{"green", "hello", ColorGreen, "\033[32mhello\033[0m"},
		{"red", "error", ColorRed, "\033[31merror\033[0m"},
		{"yellow", "warn", ColorYellow, "\033[33mwarn\033[0m"},
		{"bold", "title", ColorBold, "\033[1mtitle\033[0m"},
		{"dim", "muted", ColorDim, "\033[2mmuted\033[0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Colorize(tt.text, tt.color, true)
			if got != tt.expected {
				t.Errorf("Colorize(%q, %d, true) = %q, want %q", tt.text, tt.color, got, tt.expected)
			}
		})
	}
}

func TestColorize_Disabled(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		color Color
	}{
		{"green", "hello", ColorGreen},
		{"red", "error", ColorRed},
		{"yellow", "warn", ColorYellow},
		{"bold", "title", ColorBold},
		{"dim", "muted", ColorDim},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Colorize(tt.text, tt.color, false)
			if got != tt.text {
				t.Errorf("Colorize(%q, %d, false) = %q, want %q", tt.text, tt.color, got, tt.text)
			}
		})
	}
}

func TestColorize_Reset(t *testing.T) {
	// ColorReset should return text unchanged even when enabled.
	got := Colorize("text", ColorReset, true)
	if got != "text" {
		t.Errorf("Colorize with ColorReset should return plain text, got %q", got)
	}
}

func TestColorize_EmptyText(t *testing.T) {
	got := Colorize("", ColorGreen, true)
	expected := "\033[32m\033[0m"
	if got != expected {
		t.Errorf("Colorize empty string = %q, want %q", got, expected)
	}
}

func TestShouldColor_ForceColor(t *testing.T) {
	t.Setenv("FORCE_COLOR", "1")
	// FORCE_COLOR overrides everything, even noColorFlag and non-TTY.
	if !ShouldColor(true, false) {
		t.Error("FORCE_COLOR should override all checks")
	}
}

func TestShouldColor_NoColorFlag(t *testing.T) {
	if ShouldColor(true, true) {
		t.Error("noColor flag should disable color")
	}
}

func TestShouldColor_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if ShouldColor(false, true) {
		t.Error("NO_COLOR env should disable color")
	}
}

func TestShouldColor_DumbTerm(t *testing.T) {
	t.Setenv("TERM", "dumb")
	if ShouldColor(false, true) {
		t.Error("TERM=dumb should disable color")
	}
}

func TestShouldColor_NonTTY(t *testing.T) {
	if ShouldColor(false, false) {
		t.Error("non-TTY should disable color")
	}
}

func TestShouldColor_CI(t *testing.T) {
	t.Setenv("CI", "true")
	if ShouldColor(false, true) {
		t.Error("CI env should disable color")
	}
}

func TestShouldColor_AllConditionsMet(t *testing.T) {
	// Clear env vars that would interfere.
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("CI", "")
	if !ShouldColor(false, true) {
		t.Error("all conditions met, should enable color")
	}
}
