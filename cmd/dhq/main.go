package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/deployhq/deployhq-cli/internal/commands"
	"github.com/deployhq/deployhq-cli/internal/output"
)

var version = "dev"

func main() {
	cmd := commands.NewRootCmd(version)

	err := cmd.Execute()
	commands.SendTelemetry(err)
	if err != nil {
		exitCode := output.ClassifyError(err)
		if exitCode == 0 {
			exitCode = 1
		}

		// In JSON mode, write structured error to stdout for agents
		if commands.IsJSONMode() {
			writeJSONError(os.Stdout, err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(exitCode)
	}
}

// writeJSONError writes the structured error response for err, unless the
// command already wrote its own — stdout then holds exactly one JSON object.
func writeJSONError(w io.Writer, err error) {
	if output.IsReported(err) {
		return
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(output.ErrorResponseFromErr(err))
}
