package ui

import (
	"strings"
	"testing"
)

func TestSuccessSymbol_Color(t *testing.T) {
	s := SuccessSymbol(true)
	if !strings.Contains(s, "✓") {
		t.Errorf("SuccessSymbol should contain ✓, got %q", s)
	}
	if !strings.Contains(s, "\033[32m") {
		t.Errorf("SuccessSymbol with color should contain green ANSI code, got %q", s)
	}
}

func TestSuccessSymbol_NoColor(t *testing.T) {
	s := SuccessSymbol(false)
	if s != "✓" {
		t.Errorf("SuccessSymbol without color = %q, want %q", s, "✓")
	}
}

func TestFailureSymbol_Color(t *testing.T) {
	s := FailureSymbol(true)
	if !strings.Contains(s, "✗") {
		t.Errorf("FailureSymbol should contain ✗, got %q", s)
	}
	if !strings.Contains(s, "\033[31m") {
		t.Errorf("FailureSymbol with color should contain red ANSI code, got %q", s)
	}
}

func TestFailureSymbol_NoColor(t *testing.T) {
	s := FailureSymbol(false)
	if s != "✗" {
		t.Errorf("FailureSymbol without color = %q, want %q", s, "✗")
	}
}

func TestWarningSymbol_Color(t *testing.T) {
	s := WarningSymbol(true)
	if !strings.Contains(s, "⚠") {
		t.Errorf("WarningSymbol should contain ⚠, got %q", s)
	}
	if !strings.Contains(s, "\033[33m") {
		t.Errorf("WarningSymbol with color should contain yellow ANSI code, got %q", s)
	}
}

func TestWarningSymbol_NoColor(t *testing.T) {
	s := WarningSymbol(false)
	if s != "⚠" {
		t.Errorf("WarningSymbol without color = %q, want %q", s, "⚠")
	}
}

func TestSkipSymbol_Color(t *testing.T) {
	s := SkipSymbol(true)
	if !strings.Contains(s, "-") {
		t.Errorf("SkipSymbol should contain -, got %q", s)
	}
	if !strings.Contains(s, "\033[33m") {
		t.Errorf("SkipSymbol with color should contain yellow ANSI code, got %q", s)
	}
}

func TestSkipSymbol_NoColor(t *testing.T) {
	s := SkipSymbol(false)
	if s != "-" {
		t.Errorf("SkipSymbol without color = %q, want %q", s, "-")
	}
}
