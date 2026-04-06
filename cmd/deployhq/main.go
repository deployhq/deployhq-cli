package main

import (
	"encoding/json"
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

		// In JSON mode, write structured error to stdout for agents
		if commands.IsJSONMode() {
			errResp := output.ErrorResponseFromErr(err)
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(errResp)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(exitCode)
	}
}
