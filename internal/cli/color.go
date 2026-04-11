package cli

import (
	"os"

	"golang.org/x/term"
)

// ColorEnabled returns true if color output should be enabled.
//
// Color is enabled when ALL of the following are true:
//  1. --no-color flag is not set
//  2. NO_COLOR environment variable is not set
//  3. TERM is not "dumb"
//  4. stdout is a TTY
//  5. CI environment variable is not set
//
// FORCE_COLOR environment variable overrides all checks and enables color.
func ColorEnabled(noColorFlag bool) bool {
	// FORCE_COLOR overrides everything.
	if _, ok := os.LookupEnv("FORCE_COLOR"); ok {
		return true
	}

	if noColorFlag {
		return false
	}

	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}

	if os.Getenv("TERM") == "dumb" {
		return false
	}

	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return false
	}

	if _, ok := os.LookupEnv("CI"); ok {
		return false
	}

	return true
}

// colorEnabledWithEnv is a testable version that accepts environment lookup functions
// and a TTY check function instead of reading the real environment.
func colorEnabledWithEnv(noColorFlag bool, lookupEnv func(string) (string, bool), isTTY bool) bool {
	if _, ok := lookupEnv("FORCE_COLOR"); ok {
		return true
	}

	if noColorFlag {
		return false
	}

	if _, ok := lookupEnv("NO_COLOR"); ok {
		return false
	}

	if val, ok := lookupEnv("TERM"); ok && val == "dumb" {
		return false
	}

	if !isTTY {
		return false
	}

	if _, ok := lookupEnv("CI"); ok {
		return false
	}

	return true
}
