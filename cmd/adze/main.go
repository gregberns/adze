package main

import (
	"os"

	"github.com/gregberns/adze/internal/cli"
)

// Set via ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd := cli.NewRootCmd(version, commit, date)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
