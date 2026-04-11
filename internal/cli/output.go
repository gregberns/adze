package cli

import (
	"os"

	"golang.org/x/term"
)

// OutputMode indicates whether output is formatted for humans or as JSON.
type OutputMode int

const (
	// OutputHuman produces colored, human-readable output.
	OutputHuman OutputMode = iota
	// OutputJSON produces machine-readable JSON output.
	OutputJSON
)

// DetectOutputMode returns OutputJSON when the --json flag is set,
// otherwise OutputHuman.
func DetectOutputMode(jsonFlag bool) OutputMode {
	if jsonFlag {
		return OutputJSON
	}
	return OutputHuman
}

// IsInteractive returns true when stdout is connected to a terminal (TTY).
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
