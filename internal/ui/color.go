package ui

import (
	"fmt"
	"os"
	"strings"
)

// Color represents an ANSI color/style code.
type Color int

const (
	ColorReset Color = iota
	ColorRed
	ColorGreen
	ColorYellow
	ColorBold
	ColorDim
)

// ANSI escape sequences.
var colorCodes = map[Color]string{
	ColorReset:  "\033[0m",
	ColorRed:    "\033[31m",
	ColorGreen:  "\033[32m",
	ColorYellow: "\033[33m",
	ColorBold:   "\033[1m",
	ColorDim:    "\033[2m",
}

// Colorize wraps text with the given ANSI color code. If enabled is false,
// the text is returned unchanged.
func Colorize(text string, c Color, enabled bool) string {
	if !enabled {
		return text
	}
	code, ok := colorCodes[c]
	if !ok || c == ColorReset {
		return text
	}
	return fmt.Sprintf("%s%s%s", code, text, colorCodes[ColorReset])
}

// ShouldColor determines whether color output should be enabled based on
// environment and flags. Color is enabled when ALL of the following are true:
//   - noColor flag is false
//   - NO_COLOR env var is not set
//   - TERM is not "dumb"
//   - stdout is a TTY
//   - CI env var is not set
//
// FORCE_COLOR env var overrides all checks and enables color.
func ShouldColor(noColorFlag bool, isTTY bool) bool {
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	if noColorFlag {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	if !isTTY {
		return false
	}
	if os.Getenv("CI") != "" {
		return false
	}
	return true
}
