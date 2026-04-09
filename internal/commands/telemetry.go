package commands

import (
	"os"
	"path/filepath"

	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/internal/telemetry"
	"github.com/spf13/cobra"
)

func newTelemetryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telemetry",
		Short: "Manage anonymous usage telemetry",
		Long: `Control anonymous telemetry that helps improve the DeployHQ CLI.

What we collect: command name, exit code, duration, CLI version, OS, arch, agent flag.
What we never collect: account, email, project, arguments, file paths, error messages.`,
	}

	cmd.AddCommand(
		newTelemetryStatusCmd(),
		newTelemetryEnableCmd(),
		newTelemetryDisableCmd(),
	)

	return cmd
}

func newTelemetryStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current telemetry status",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope
			enabled := telemetry.IsEnabled()
			source := telemetry.EnabledSource()
			token := telemetry.Token()
			environment := telemetry.Environment()

			// Resolve distinct ID
			distinctID := ""
			home, err := os.UserHomeDir()
			if err == nil {
				dir := filepath.Join(home, ".deployhq")
				if telemetry.HasIdentity(dir) {
					distinctID = telemetry.DistinctID(dir)
				}
			}

			hasToken := token != ""

			if env.JSONMode {
				return env.WriteJSON(map[string]interface{}{
					"enabled":     enabled,
					"source":      source,
					"distinct_id": distinctID,
					"has_token":   hasToken,
					"environment": environment,
				})
			}

			enabledStr := "yes"
			if !enabled {
				enabledStr = "no"
			}
			tokenStr := "yes (injected at build)"
			if !hasToken {
				tokenStr = "no (dev build — telemetry inactive)"
			}
			if distinctID == "" {
				distinctID = "(not yet generated)"
			}

			env.WriteTable(
				[]string{"Setting", "Value"},
				[][]string{
					{"Enabled", enabledStr},
					{"Source", source},
					{"Distinct ID", distinctID},
					{"Has Token", tokenStr},
					{"Environment", environment},
				},
			)

			env.Status("\nDisable: dhq telemetry disable")
			env.Status("Enable:  dhq telemetry enable")
			env.Status("Env var: DEPLOYHQ_NO_TELEMETRY=1")
			return nil
		},
	}
}

func newTelemetryEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Enable anonymous telemetry",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := telemetry.SetEnabled(true); err != nil {
				return &output.InternalError{Message: "enable telemetry", Cause: err}
			}
			cliCtx.Envelope.Status("Telemetry enabled. Saved to %s", config.GlobalConfigPath())
			return nil
		},
	}
}

func newTelemetryDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable anonymous telemetry",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := telemetry.SetEnabled(false); err != nil {
				return &output.InternalError{Message: "disable telemetry", Cause: err}
			}
			cliCtx.Envelope.Status("Telemetry disabled. Saved to %s", config.GlobalConfigPath())
			return nil
		},
	}
}
