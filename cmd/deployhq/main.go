package main

import (
	"fmt"
	"os"

	"github.com/deployhq/deployhq-cli/internal/commands"
	"github.com/deployhq/deployhq-cli/internal/output"
)

var version = "dev"

func main() {
	cmd := commands.NewRootCmd(version)

	if err := cmd.Execute(); err != nil {
		exitCode := output.ClassifyError(err)
		if exitCode == 0 {
			exitCode = 1
		}
		// Error already printed by command's RunE via envelope
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}
}
