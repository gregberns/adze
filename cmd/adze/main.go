package main

import (
	"fmt"
	"os"
)

// Set via ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("adze %s (commit: %s, built: %s)\n", version, commit, date)
		return
	}

	fmt.Println("adze - machine configuration tool")
	fmt.Println("Run 'adze version' for version info")
	os.Exit(0)
}
