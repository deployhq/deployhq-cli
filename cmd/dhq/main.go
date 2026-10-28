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
		// Give API refusals with a known cause (e.g. unverified email) a
		// specific, actionable message instead of the raw status line.
		err = output.RefineError(err)
		// Name plan/billing refusals (no API access on the plan, account
		// suspended or trial ended, no payment method) and link to the page
		// that resolves them, instead of printing the raw API response.
		err = output.ExplainAccountError(err)
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
