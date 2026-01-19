package main

import (
	"fmt"
	"os"

	"github.com/RevCBH/choo/internal/cli"
)

// Build-time variables (set via ldflags)
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	app := cli.New()
	app.SetVersion(version, commit, date)

	if err := app.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
